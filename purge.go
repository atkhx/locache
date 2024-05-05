package locache

import (
	"container/list"
	"sync"
	"time"
)

type (
	PurgeItem[Key comparable] struct {
		key Key
		exp time.Time
	}
	PurgeLog[Key comparable] struct {
		sync.RWMutex
		list *list.List
	}
)

func NewPurgeLog[Key comparable]() *PurgeLog[Key] {
	return &PurgeLog[Key]{
		list: list.New(),
	}
}

func (p *PurgeLog[Key]) Add(key Key, exp time.Time) {
	p.Lock()
	p.list.PushBack(PurgeItem[Key]{key, exp})
	p.Unlock()
}

func (p *PurgeLog[Key]) Purge(now time.Time, purge func(key Key)) {
	var head *list.Element
	for element := p.list.Front(); element != nil; element = element.Next() {
		walItem := element.Value.(PurgeItem[Key])
		if walItem.exp.After(now) {
			break
		}
		purge(walItem.key)
		head = element
	}

	if head != nil {
		p.Lock()
		p.list.MoveToFront(head)
		p.Unlock()
	}
}
