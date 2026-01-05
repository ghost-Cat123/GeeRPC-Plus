package main

import (
	"GeeRPC"
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Foo int

type Args struct{ Num1, Num2 int }

// Sum 远端服务器方法
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// 服务端 发送请求 监听服务器
func startServer(addrCh chan string) {
	var foo Foo
	// 监听 启动RPC服务器
	l, _ := net.Listen("tcp", ":9999")
	// 注册服务
	_ = GeeRPC.Register(&foo)
	// 发送请求 geerpc.Accept() 替换为 GeeRPC.HandleHTTP()
	GeeRPC.HandleHTTP()
	addrCh <- l.Addr().String()
	_ = http.Serve(l, nil)
}

// 客户端 接受请求 调用服务器端方法
func call(addrCh chan string) {
	addr := <-addrCh
	// 重点：必须检查DialHTTP的错误，不能忽略！
	client, err := GeeRPC.DialHTTP("tcp", addr)
	if err != nil {
		log.Fatalf("dial HTTP failed: %v", err) // 错误直接退出，避免使用nil Client
	}
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: i, Num2: i * i}
			var reply int
			err := client.Call(context.Background(), "Foo.Sum", args, &reply)
			if err != nil {
				log.Printf("call Foo.Sum(%d, %d) error: %v", args.Num1, args.Num2, err)
				return // 单个请求失败不退出，仅打印
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	ch := make(chan string)
	// 客户端异步调用请求
	go call(ch)
	// 服务器端等待请求 监听端口
	startServer(ch)
}
