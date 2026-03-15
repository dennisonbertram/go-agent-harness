package main

import (
	"container/list"
	"sync"
)

type entry struct {
	key   string
	value string
}

type LRU struct {
	capacity int
	mutex    sync.Mutex
	cache    map[string]*list.Element
	list     *list.List
}

func NewLRU(capacity int) *LRU {
	if capacity <= 0 {
		panic("capacity must be positive")
	}
	return &LRU{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

func (l *LRU) Get(key string) (string, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	e, ok := l.cache[key]
	if !ok {
		return "", false
	}
	l.list.MoveToFront(e)
	return e.Value.(*entry).value, true
}

func (l *LRU) Put(key, value string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if e, ok := l.cache[key]; ok {
		e.Value.(*entry).value = value
		l.list.MoveToFront(e)
		return
	}
	// New entry
	e := l.list.PushFront(&entry{key: key, value: value})
	l.cache[key] = e
	if l.list.Len() > l.capacity {
		last := l.list.Back()
		if last != nil {
			l.list.Remove(last)
			delEntry := last.Value.(*entry)
			delete(l.cache, delEntry.key)
		}
	}
}
