package locache

import (
	"context"
	"fmt"
	"time"
)

var now = time.Now

type Cache[Key comparable, Value any] struct {
	ctx context.Context

	itemsTTL time.Duration
	itemsMap *ItemsMap[Key, Value]
	purgeLog *PurgeLog[Key]
}

func New[Key comparable, Value any](
	ctx context.Context,
	ttl time.Duration,
) *Cache[Key, Value] {
	cache := &Cache[Key, Value]{
		ctx:      ctx,
		itemsTTL: ttl,
		itemsMap: NewItems[Key, Value](),
		purgeLog: NewPurgeLog[Key](),
	}
	return cache
}

func (c *Cache[Key, Value]) Get(key Key) (value Value, ok bool) {
	if item, found := c.itemsMap.Get(key); found {
		item.RLock()
		if item.IsValid() {
			value, ok = item.val, true
		}
		item.RUnlock()
	}
	return
}

func (c *Cache[Key, Value]) GetOrRefresh(key Key, refresh func() (Value, error)) (Value, error) {
	item, itemIsNew := c.itemsMap.GetOrCreate(key)
	item.Lock()

	if item.IsValid() {
		val := item.val
		item.Unlock()
		return val, nil
	}

	val, err := refresh()
	if err != nil {
		if itemIsNew {
			c.itemsMap.Delete(key)
		}
		item.Unlock()
		var emptyValue Value
		return emptyValue, fmt.Errorf("get val: %w", err)
	}

	item.val = val
	item.exp = now().Add(c.itemsTTL)

	c.itemsMap.Set(key, item)
	c.purgeLog.Add(key, item.exp)

	item.Unlock()
	return val, nil
}

func (c *Cache[Key, Value]) Set(key Key, value Value) {
	item := NewItem(value, now().Add(c.itemsTTL))
	item.Lock()
	c.itemsMap.Set(key, item)
	c.purgeLog.Add(key, item.exp)
	item.Unlock()
}

func (c *Cache[Key, Value]) Update(key Key, update func(value Value, exists bool) (Value, bool, error)) error {
	item, itemIsNew := c.itemsMap.GetOrCreate(key)
	item.Lock()

	val, set, err := update(item.val, !itemIsNew)

	if itemIsNew && (err != nil || !set) {
		c.itemsMap.Delete(key)
	}

	if err != nil {
		item.Unlock()
		return fmt.Errorf("update val: %w", err)
	}

	if set {
		item.val = val
		item.exp = now().Add(c.itemsTTL)

		c.itemsMap.Set(key, item)
	}

	item.Unlock()
	return nil
}

func (c *Cache[Key, Value]) Delete(key Key) {
	if item, ok := c.itemsMap.Get(key); ok {
		item.Lock()
		c.itemsMap.Delete(key)
		item.Unlock()
	}
}

func (c *Cache[Key, Value]) DeleteExpired(key Key) {
	if item, ok := c.itemsMap.Get(key); ok {
		item.Lock()
		if item.IsExpired() {
			c.itemsMap.Delete(key)
		}
		item.Unlock()
	}
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
	c.purgeLog.Purge(now(), c.DeleteExpired)
}
