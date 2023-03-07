package main

import (
	"encoding/json"
	"fmt"
	geerpc "geeRpc"
	"geeRpc/codec"
	"log"
	"net"
	"time"
)

func startServer(addr chan string) {
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	geerpc.Accept(l)
}

func main() {
	addr := make(chan string)
	// use chan to ensure client will be established after server
	go startServer(addr)

	// in fact, following code is like a simple geerpc client
	conn, _ := net.Dial("tcp", <-addr)
	defer func() { _ = conn.Close() }()

	time.Sleep(1 * time.Second)
	// send options
	_ = json.NewEncoder(conn).Encode(geerpc.DefaultOption)
	cc := codec.NewGobCodec(conn)
	//send request & receive response
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		// client send message
		_ = cc.Write(h, fmt.Sprintf("geerpc req %d", h.Seq))
		// client get h from server
		// 这里要先读h的原因是 写入conn中的数据是header+body的形式,如果不先把header读出来,body就会读取header的部分,而因为结构不一致的原因,最终会读出0
		_ = cc.ReadHeader(h)
		var reply string
		// client get body from server
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}
