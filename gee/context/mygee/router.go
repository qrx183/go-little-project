package mygee

import (
	"log"
	"net/http"
	"strings"
)

type router struct {
	root     map[string]*node
	handlers map[string]HandlerFunc
}

func newRouter() *router {
	return &router{
		root:     make(map[string]*node),
		handlers: make(map[string]HandlerFunc),
	}
}

func (r *router) parsePatterns(pattern string) []string {
	parts := strings.Split(pattern, "/")

	res := make([]string, 0)
	for _, part := range parts {
		if part == "" {
			continue
		}

		res = append(res, part)

		if part[0] == '*' {
			break
		}
	}
	return res
}

// addRoute是启动服务时调用的，生成当前路由的前缀树
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	log.Printf("Route %4s - %s", method, pattern)
	parts := r.parsePatterns(pattern)

	_, ok := r.root[method]

	if !ok {
		r.root[method] = &node{}
	}
	r.root[method].insert(pattern, parts, 0)

	key := method + "-" + pattern
	r.handlers[key] = handler
}

// getRoute 是调用接口时调用的,利用传入的具体路由路径来匹配合适的前缀树
func (r *router) getRoute(method string, pattern string) (*node, map[string]string) {
	searchParts := r.parsePatterns(pattern)
	params := make(map[string]string)
	n, ok := r.root[method]
	if !ok {
		return nil, nil
	}
	n = n.search(searchParts, 0)
	if n == nil {
		return nil, nil
	}

	parts := r.parsePatterns(n.pattern)

	for index, part := range parts {
		if part[0] == ':' && len(part) > 1 {
			params[part[1:]] = searchParts[index]
		}

		if part[0] == '*' && len(part) > 1 {
			params[part[1:]] = strings.Join(searchParts[index:], "/")
		}
	}

	return n, params

}

func (r *router) handle(c *Context) {

	n, params := r.getRoute(c.Method, c.Path)
	if n != nil {
		c.Params = params
		// 注意这里要用前缀树中的pattern,因为addRoute时存到r的handler中的key是前缀树中的pattern
		// 不能用c.Path
		key := c.Method + "-" + n.pattern
		c.handlers = append(c.handlers, r.handlers[key])
	} else {
		c.handlers = append(c.handlers, func(c *Context) {
			c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
		})
	}
	c.Next()
}
