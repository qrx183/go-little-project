package main

import (
	"geeCache/lru"
	"sync"
)

type Cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	CacheBytes int64
}

func (c *Cache) Add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lru == nil {
		// 懒加载
		c.lru = lru.New(c.CacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *Cache) Get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	// 这里需要加锁,因为可能涉及到lru中节点的删除操作
	defer c.mu.Unlock()
	if c.lru == nil {
		// 返回默认值: byteview.ByteView{},false
		return
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}

	return
}
