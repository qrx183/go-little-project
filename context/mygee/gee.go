package mygee

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

type HandlerFunc func(c *Context)

type RouterGroup struct {
	prefix      string
	parent      *RouterGroup
	middlewares []HandlerFunc
	engine      *Engine
}

// Engine
// Engine和RouterGroup是双向对应关系
type Engine struct {
	*RouterGroup  //通过将group设置为属性从而实现继承的关系,进而调用该属性对应的方法
	router        *router
	routerGroups  []*RouterGroup
	htmlTemplates *template.Template
	funcMap       template.FuncMap
}

func New() *Engine {
	engine := &Engine{router: newRouter()}

	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.routerGroups = []*RouterGroup{engine.RouterGroup}
	return engine
}

func (e *Engine) SetFuncMap(funcMap template.FuncMap) {
	e.funcMap = funcMap
}

// Default
// 默认引擎支持日志打印和错误处理
func Default() *Engine {
	engine := New()
	engine.Use(Logger(), Recovery())
	return engine
}

// LoadHtmlGlob
// 加载所有静态文件到模板中
func (e *Engine) LoadHtmlGlob(pattern string) {
	e.htmlTemplates = template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern))
}

func (g *RouterGroup) Use(middlewares ...HandlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

func (g *RouterGroup) Group(prefix string) *RouterGroup {
	engine := g.engine

	newGroup := &RouterGroup{
		prefix:      g.prefix + prefix,
		middlewares: g.middlewares,
		parent:      g,
		engine:      engine,
	}
	engine.routerGroups = append(engine.routerGroups, newGroup)
	return newGroup
}

func (g *RouterGroup) addRoute(method string, pattern string, handler HandlerFunc) {
	g.engine.router.addRoute(method, pattern, handler)
}
func (g *RouterGroup) GET(pattern string, handler HandlerFunc) {
	g.addRoute("GET", pattern, handler)
}

func (g *RouterGroup) POST(pattern string, handler HandlerFunc) {
	g.addRoute("POST", pattern, handler)
}

func (g *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	absolutePath := g.prefix + relativePath

	// 把请求中url的absolutePath部分去掉
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))

	return func(c *Context) {
		file := c.Param("filepath")

		if _, err := fs.Open(file); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		fileServer.ServeHTTP(c.W, c.Req)
	}

}

func (g *RouterGroup) Static(relativePath string, root string) {
	handler := g.createStaticHandler(relativePath, http.Dir(root))

	urlPattern := path.Join(relativePath, "/*filepath")

	g.GET(urlPattern, handler)
}

// Logger
// 记录一个请求的耗时
func Logger() HandlerFunc {
	return func(c *Context) {
		t := time.Now()

		c.Next()

		log.Printf("[%d] %s in %v\n", c.StatusCode, c.Req.RequestURI, time.Since(t))
	}
}

func (e *Engine) Run(addr string) error {
	return http.ListenAndServe(addr, e)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc

	for _, group := range e.routerGroups {
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}

	}

	c := NewContext(w, req)
	c.e = e
	c.handlers = middlewares
	e.router.handle(c)
}
