package adblock

import "container/list"

type lruEntry struct {
	v   bool
	key pair
	e   *list.Element
}

type LRU struct {
	m        map[pair]*lruEntry
	l        *list.List
	capacity int
}

func NewLRU(capacity int) *LRU {
	return &LRU{
		m:        make(map[pair]*lruEntry),
		l:        list.New(),
		capacity: capacity,
	}
}

func (lru *LRU) promote(e *lruEntry) {
	lru.l.MoveToFront(e.e)
}

func (lru *LRU) prune() {
	// prune 5%
	n := lru.capacity / 20
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		if end := lru.l.Back(); end != nil {
			e := lru.l.Remove(end).(*lruEntry)
			delete(lru.m, e.key)
		} else {
			break
		}
	}
}

func (lru *LRU) Set(key pair, value bool) {
	e, ok := lru.m[key]
	if ok {
		e.v = value
		lru.promote(e)
		return
	}

	if len(lru.m) > lru.capacity {
		lru.prune()
	}
	e = &lruEntry{v: value, key: key}
	e.e = lru.l.PushFront(e)
	lru.m[key] = e
}

func (lru *LRU) Get(key pair) (value bool, ok bool) {
	e, ok := lru.m[key]
	if !ok {
		return false, false
	}
	lru.promote(e)
	return e.v, true
}
