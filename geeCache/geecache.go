package main

import (
	"fmt"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

type Group struct {
	name      string
	getter    Getter
	mainCache *Cache
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("getter is required")
	}
	mu.Lock()
	defer mu.Unlock()
	newGroup := &Group{
		name:      name,
		getter:    getter,
		mainCache: &Cache{CacheBytes: cacheBytes},
	}
	groups[name] = newGroup
	return newGroup
}

func GetGroups(name string) *Group {
	mu.RLock()
	defer mu.RUnlock()
	m := groups[name]
	return m
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	// 查到缓存获取缓存的value
	if value, ok := g.mainCache.Get(key); ok {
		log.Printf("[GeeCache] hit")
		return value, nil
	}

	// 没有查到缓存,通过回调getter方法获得数据后存入缓存中
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	return g.getLocally(key)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)

	if err != nil {
		return ByteView{}, err
	}

	g.populateCache(key, ByteView{b: cloneBytes(bytes)})

	return ByteView{b: cloneBytes(bytes)}, err
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.Add(key, value)
}
