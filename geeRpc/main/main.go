package main

import (
	"context"
	"fmt"
	geerpc "geeRpc"
	"log"
	"net"
	"sync"
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

	client, _ := geerpc.Dial("tcp", <-addr)

	defer func() {
		_ = client.Close()
	}()

	time.Sleep(1 * time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("geerpc req %d", i)
			var reply string
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			if err := client.Call(ctx, "Foo.sum", args, &reply); err != nil {
				log.Fatal("call foo.sum err:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
