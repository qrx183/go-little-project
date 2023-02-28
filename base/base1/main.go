package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// 使用net包实现简单web接口,通过HandleFunc去实现单一逻辑的路由
	http.HandleFunc("/hello", helloHandle)

	log.Fatal(http.ListenAndServe("localhost:9080", nil))
}

func helloHandle(w http.ResponseWriter, req *http.Request) {
	for k, v := range req.Header {
		fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
	}
}
