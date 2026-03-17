package main

import "sync"

type Cache struct {
	mu   sync.Mutex
	data map[string]string
}

func NewCache() *Cache {
	return &Cache{data: map[string]string{}}
}

func (c *Cache) Set(k, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[k] = v
}

func (c *Cache) GetOrSet(k, defaultVal string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.data[k]; ok {
		return v
	}
	c.data[k] = defaultVal
	return defaultVal
}
