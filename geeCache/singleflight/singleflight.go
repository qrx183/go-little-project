package singleflight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 保证key相同的大量并发请求只有1个抢占到锁向远程节点获取缓存值或更新缓存,其他的请求共享结果
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	g.mu.Lock()
	c := new(call)
	g.m[key] = c
	c.wg.Add(1)
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	// 请求完后应该把key删除掉,缓存只在lru中存储,这里不删除既会占用内存,也可能会导致缓存对应的非最新数据
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
