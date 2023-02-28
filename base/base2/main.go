package main

import (
	"fmt"
	"log"
	"net/http"
)

type Engine struct {
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/":
		fmt.Fprintf(w, "URL.path = %q", req.URL.Path)
	case "/hello":
		for k, v := range req.Header {
			fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
		}
	default:
		fmt.Fprintf(w, "404 NOT FOUND")
	}
}

func main() {
	// 通过实现Handler接口自定义Handler来对一系列逻辑进行统一处理
	engine := new(Engine)
	log.Fatal(http.ListenAndServe(":9090", engine))
}
