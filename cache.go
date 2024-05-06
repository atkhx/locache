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

	items *list.List
	index map[Key]*list.Element
}

func New[Key comparable, Value any](ctx context.Context, ttl time.Duration) *Cache[Key, Value] {
	return &Cache[Key, Value]{
		ctx:   ctx,
		ttl:   ttl,
		items: list.New(),
		index: make(map[Key]*list.Element),
	}
}

func (c *Cache[Key, Value]) Get(key Key) (Value, bool) {
	var val Value

	c.mtx.RLock()
	defer c.mtx.RUnlock()

	element, found := c.index[key]
	if !found {
		return val, false
	}

	if item := element.Value.(*Item[Key, Value]); item.IsValid() {
		return item.val, true
	}
	return val, false
}

func (c *Cache[Key, Value]) Set(key Key, value Value) {
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
	element := c.getOrCreateElement(key)

	item := element.Value.(*Item[Key, Value])
	item.mtx.Lock()

	if item.IsValid() {
		val := item.val
		item.mtx.Unlock()
		return val, nil
	}

	val, err := refresh()
	if err != nil {
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
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for element := c.items.Front(); element != nil; {
		item := element.Value.(*Item[Key, Value])
		if item.exp.Before(now()) {
			remove := element
			element = element.Next()
			c.items.Remove(remove)
			delete(c.index, item.key)
		} else {
			element = element.Next()
		}
	}
}
