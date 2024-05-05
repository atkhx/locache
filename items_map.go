package locache

import "sync"

type ItemsMap[Key comparable, Value any] struct {
	sync.RWMutex
	data map[Key]*Item[Value]
}

func NewItems[Key comparable, Value any]() *ItemsMap[Key, Value] {
	return &ItemsMap[Key, Value]{data: make(map[Key]*Item[Value])}
}

func (i *ItemsMap[Key, Value]) Get(key Key) (item *Item[Value], ok bool) {
	i.RLock()
	item, ok = i.data[key]
	i.RUnlock()
	return
}

func (i *ItemsMap[Key, Value]) GetOrCreate(key Key) (*Item[Value], bool) {
	i.Lock()
	item, ok := i.data[key]
	if !ok {
		item = &Item[Value]{}
		i.data[key] = item
	}
	i.Unlock()
	return item, !ok
}

func (i *ItemsMap[Key, Value]) Set(key Key, item *Item[Value]) {
	i.Lock()
	i.data[key] = item
	i.Unlock()
}

func (i *ItemsMap[Key, Value]) Delete(key Key) {
	i.Lock()
	delete(i.data, key)
	i.Unlock()
}
