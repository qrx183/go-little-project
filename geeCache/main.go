package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *Group {
	return NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

func startCacheServer(addr string, addrs []string, gee *Group) {
	peer := NewHTTPPool(addr)
	peer.Set(addrs...)
	gee.RegisterPeerPicker(peer)
	log.Println("geeCache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peer))
}

func startAPIServer(addr string, gee *Group) {

	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			key := req.URL.Query().Get("key")
			view, err := gee.Get(key)

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.byteSlice())
		}))
	log.Println("fontend server is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], nil))
}

func main() {

	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()
	fmt.Println("port:", port)
	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	gee := createGroup()
	if api {
		go startAPIServer(apiAddr, gee)
	}
	startCacheServer(addrMap[port], []string(addrs), gee)
}
