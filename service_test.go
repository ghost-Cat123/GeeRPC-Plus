package GrowRPC

import (
	"context"
	"fmt"
	"testing"
)

type Foo int

type Args struct{ Num1, Num2 int }

func (f *Foo) Sum(_ context.Context, args *Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestRegisterMethod(t *testing.T) {
	server := NewServer()
	var foo Foo
	RegisterMethod[Args, int](server, "Foo.Sum", foo.Sum)
	_, ok := server.serviceMap.Load("Foo.Sum")
	_assert(ok, "service Method should be registered")
}

func TestMethodHandler_Call(t *testing.T) {
	server := NewServer()
	var foo Foo
	RegisterMethod[Args, int](server, "Foo.Sum", foo.Sum)
	entryI, ok := server.serviceMap.Load("Foo.Sum")
	_assert(ok, "service Method should be registered")

	entry := entryI.(*handlerEntry)

	// 模拟主循环：用 newReq 创建实例，填充值（代替 cc.ReadBody）
	reqVal := entry.newReq()
	args := reqVal.(*Args)
	args.Num1 = 1
	args.Num2 = 3

	// 模拟 handleRequest 的 goroutine 调用业务函数
	respI, err := entry.handler(context.Background(), reqVal)
	_assert(err == nil, "handler should not error")
	reply := respI.(*int)
	_assert(*reply == 4, "expected 4, got %d", *reply)
}
