package main

import (
	"fmt"
	pb "geeCache/geeCachePb"
	"geeCache/singleflight"
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
	peers     PeerPicker
	loader    *singleflight.Group
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
		loader:    &singleflight.Group{},
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

func (g *Group) RegisterPeerPicker(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")

	}
	g.peers = peers
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

func (g *Group) load(key string) (value ByteView, err error) {

	view, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}

				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}
		return g.getLocally(key)
	})

	if err == nil {
		return view.(ByteView), nil
	}
	return

}

func (g *Group) getFromPeer(getter PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := getter.Get(req, res)

	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
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
