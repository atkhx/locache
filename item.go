package locache

import (
	"sync"
	"time"
)

type Item[Value any] struct {
	sync.RWMutex
	val Value
	exp time.Time
}

func NewItem[Value any](val Value, exp time.Time) *Item[Value] {
	return &Item[Value]{val: val, exp: exp}
}

func (i *Item[Value]) IsValid() bool {
	return !(i.exp.IsZero() || i.IsExpired())
}

func (i *Item[Value]) IsExpired() bool {
	return now().After(i.exp)
}
