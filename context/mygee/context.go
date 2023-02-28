package mygee

import (
	"encoding/json"
	"fmt"
	"net/http"
)

/*
	主要提供了获取参数的两种方法query和postForm 和 发送数据的几种方式:json,string,html,data
	设置H 方便后续构建json数据
*/
type H map[string]interface{}

type Context struct {
	W          http.ResponseWriter
	Req        *http.Request
	Path       string
	Method     string
	StatusCode int
}

func NewContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		W:      w,
		Req:    req,
		Path:   req.URL.Path,
		Method: req.Method,
	}
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.W.WriteHeader(code)
}

func (c *Context) SetHeader(key string, val string) {
	c.W.Header().Set(key, val)
}
func (c *Context) PostForm(key string) string {
	return c.Req.FormValue(key)
}

func (c *Context) Query(key string) string {
	return c.Req.URL.Query().Get(key)
}

func (c *Context) String(code int, format string, value ...interface{}) {
	c.SetHeader("Content-type", "text/plain")
	c.Status(code)
	c.W.Write([]byte(fmt.Sprintf(format, value)))
}

func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	c.W.Write(data)
}

func (c *Context) JSON(code int, obj interface{}) {
	c.SetHeader("Content-type", "application/json")
	c.Status(code)
	encode := json.NewEncoder(c.W)

	if err := encode.Encode(obj); err != nil {
		http.Error(c.W, err.Error(), 500)
	}

}

func (c *Context) HTML(code int, html string) {
	c.SetHeader("Content-type", "text/html")
	c.Status(code)
	c.W.Write([]byte(html))
}
