package locache

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"
)

var now = time.Now

type Item[Key comparable, Value any] struct {
	mtx sync.Mutex
	key Key
	val Value
	exp time.Time
	set bool
}

func (i *Item[Key, Value]) IsExpired() bool {
	return i.exp.Before(now())
}

func (i *Item[Key, Value]) IsValid() bool {
	return i.set && !i.IsExpired()
}

type Cache[Key comparable, Value any] struct {
	ctx context.Context
	ttl time.Duration
	mtx sync.RWMutex
	mtr Metrics

	items *list.List
	index map[Key]*list.Element
}

func New[Key comparable, Value any](
	ctx context.Context,
	ttl time.Duration,
	mtr Metrics,
) *Cache[Key, Value] {
	return &Cache[Key, Value]{
		ctx: ctx,
		ttl: ttl,
		mtr: mtr,

		items: list.New(),
		index: make(map[Key]*list.Element),
	}
}

func (c *Cache[Key, Value]) Get(key Key) (Value, bool) {
	startTime := now()
	defer c.mtr.ObserveRequest(MethodGet, startTime)

	var val Value

	c.mtx.RLock()
	defer c.mtx.RUnlock()

	element, found := c.index[key]
	if !found {
		c.mtr.IncMisses(MethodGet)
		return val, false
	}

	if item := element.Value.(*Item[Key, Value]); item.IsValid() {
		c.mtr.IncHits(MethodGet)
		return item.val, true
	}

	c.mtr.IncMisses(MethodGet)
	return val, false
}

func (c *Cache[Key, Value]) Set(key Key, value Value) {
	startTime := now()
	defer c.mtr.ObserveRequest(MethodSet, startTime)

	c.mtx.Lock()
	defer c.mtx.Unlock()

	if element, found := c.index[key]; found {
		item := element.Value.(*Item[Key, Value])
		item.set = true
		item.val = value
		item.exp = now().Add(c.ttl)

		c.items.MoveToBack(element)
		return
	}

	c.index[key] = c.items.PushBack(&Item[Key, Value]{
		set: true,
		key: key,
		val: value,
		exp: now().Add(c.ttl),
	})
}

func (c *Cache[Key, Value]) Del(key Key) {
	startTime := now()
	defer c.mtr.ObserveRequest(MethodDel, startTime)

	c.mtx.Lock()
	defer c.mtx.Unlock()

	if element, found := c.index[key]; found {
		c.items.Remove(element)
		delete(c.index, key)
	}
}

func (c *Cache[Key, Value]) getOrCreateElement(key Key) *list.Element {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	element, found := c.index[key]
	if !found {
		element = c.items.PushBack(&Item[Key, Value]{
			key: key,
			exp: now().Add(c.ttl),
		})
		c.index[key] = element
	}
	return element
}

func (c *Cache[Key, Value]) GetOrRefresh(key Key, refresh func() (Value, error)) (Value, error) {
	startTime := now()
	defer c.mtr.ObserveRequest(MethodGetOrRefresh, startTime)

	element := c.getOrCreateElement(key)

	item := element.Value.(*Item[Key, Value])
	item.mtx.Lock()

	if item.IsValid() {
		c.mtr.IncHits(MethodGetOrRefresh)
		val := item.val
		item.mtx.Unlock()
		return val, nil
	}

	val, err := refresh()
	if err != nil {
		c.mtr.IncErrors(MethodGetOrRefresh)
		item.mtx.Unlock()
		var emptyVal Value
		return emptyVal, fmt.Errorf("refresh val: %w", err)
	}

	item.set = true
	item.val = val
	item.exp = now().Add(c.ttl)
	item.mtx.Unlock()

	c.mtx.Lock()
	c.items.MoveToBack(element)
	c.mtx.Unlock()

	return val, nil
}

func (c *Cache[Key, Value]) SchedulePurge(purgeInterval time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(purgeInterval):
				c.Purge()
			}
		}
	}()
	return done
}

func (c *Cache[Key, Value]) Purge() {
	startTime := now()
	defer c.mtr.ObserveRequest(MethodPurge, startTime)

	c.mtx.Lock()
	defer c.mtx.Unlock()

	for element := c.items.Front(); element != nil; {
		item := element.Value.(*Item[Key, Value])
		if !item.mtx.TryLock() {
			element = element.Next()
			continue
		}
		if item.exp.Before(now()) {
			remove := element
			element = element.Next()
			c.items.Remove(remove)
			delete(c.index, item.key)
		} else {
			element = element.Next()
		}
		item.mtx.Unlock()
	}

	c.mtr.SetItemsCount(c.items.Len())
}
