package main

import (
	"fmt"
	"net"
)

/**
出现粘包问题，“粘包”可发生在发送端也可发生在接收端：
*/
func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:3000")
	if err != nil {
		fmt.Println("dial failed, err", err)
		return
	}
	defer conn.Close()
	//for i := 0; i < 20; i++ {
	msg := `Hello, How are you!`
	_, err = conn.Write([]byte(msg))
	var buf = make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("re err:", err)
		return
	}
	fmt.Println("buf:", string(buf[:n]))
	//}
}
