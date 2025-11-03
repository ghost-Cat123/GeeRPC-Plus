package main

import (
	"GeeRPC"
	"GeeRPC/codec"
	"encoding/json"
	"fmt"
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
	GeeRPC.Accept(l)
}

func main() {
	// 使用addr信道
	addr := make(chan string)
	go startServer(addr)

	// in fact, following code is like a simple geerpc client
	conn, _ := net.Dial("tcp", <-addr)
	defer func() { _ = conn.Close() }()

	time.Sleep(time.Second)
	// send options
	_ = json.NewEncoder(conn).Encode(GeeRPC.DefaultOption)
	cc := codec.NewGobCodec(conn)
	// send request & receive response
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		// 发送消息头和消息体
		_ = cc.Write(h, fmt.Sprintf("geerpc req %d", h.Seq))
		_ = cc.ReadHeader(h)
		var reply string
		// 解析服务端响应reply
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}
