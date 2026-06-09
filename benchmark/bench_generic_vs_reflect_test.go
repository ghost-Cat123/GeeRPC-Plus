package benchmark

import (
	"GrowRPC"
	"context"
	"reflect"
	"testing"
)

// 自包含的类型定义，避免依赖 GrowRPC 包内的 _test.go 文件
type Foo int
type Args struct{ Num1, Num2 int }

func (f *Foo) Sum(_ context.Context, args *Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// reflectEntry 模拟旧版反射时代在 serviceMap 中存储的结构
type reflectEntry struct {
	reqType reflect.Type
	handler func(ctx context.Context, req interface{}) (interface{}, error)
}

func newReflectEntry() *reflectEntry {
	var foo Foo
	reqType := reflect.TypeOf(Args{})

	return &reflectEntry{
		reqType: reqType,
		handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			respVal := reflect.New(reflect.TypeOf(0))
			err := foo.Sum(ctx, req.(*Args), respVal.Interface().(*int))
			return respVal.Interface(), err
		},
	}
}

// ─────────────── Benchmark 1: 实例创建开销 ───────────────

func BenchmarkNew_Generic(b *testing.B) {
	server := GrowRPC.NewServer()
	var foo Foo
	GrowRPC.RegisterMethod[Args, int](server, "Foo.Sum", foo.Sum)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = new(Args)
	}
}

func BenchmarkNew_Reflect(b *testing.B) {
	re := newReflectEntry()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = reflect.New(re.reqType).Interface()
	}
}

// ─────────────── Benchmark 2: handler 调用开销 ───────────────

func BenchmarkCall_Generic(b *testing.B) {
	server := GrowRPC.NewServer()
	var foo Foo
	GrowRPC.RegisterMethod[Args, int](server, "Foo.Sum", foo.Sum)
	ctx := context.Background()
	req := &Args{Num1: 10, Num2: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var resp int
		_ = foo.Sum(ctx, req, &resp)
	}
}

func BenchmarkCall_Reflect(b *testing.B) {
	var foo Foo
	ctx := context.Background()

	fooVal := reflect.ValueOf(&foo)
	method := fooVal.MethodByName("Sum")
	argType := reflect.TypeOf(Args{})
	replyType := reflect.TypeOf(0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reqVal := reflect.New(argType)
		reqVal.Elem().Field(0).SetInt(10)
		reqVal.Elem().Field(1).SetInt(20)
		replyVal := reflect.New(replyType)

		method.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reqVal,
			replyVal,
		})
	}
}

// ─────────────── Benchmark 3: 完整路由流程 ───────────────

func BenchmarkFullDispatch_Generic(b *testing.B) {
	server := GrowRPC.NewServer()
	var foo Foo
	GrowRPC.RegisterMethod[Args, int](server, "Foo.Sum", foo.Sum)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		args := &Args{Num1: i, Num2: i + 1}
		var resp int
		_ = foo.Sum(ctx, args, &resp)
	}
}

func BenchmarkFullDispatch_Reflect(b *testing.B) {
	re := newReflectEntry()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reqVal := reflect.New(re.reqType).Interface()
		args := reqVal.(*Args)
		args.Num1 = i
		args.Num2 = i + 1
		_, _ = re.handler(ctx, reqVal)
	}
}

// ─────────────── Benchmark 4: 并发场景 ───────────────

func BenchmarkParallel_Generic(b *testing.B) {
	var foo Foo
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			args := &Args{Num1: 1, Num2: 2}
			var resp int
			_ = foo.Sum(ctx, args, &resp)
		}
	})
}

func BenchmarkParallel_Reflect(b *testing.B) {
	re := newReflectEntry()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			reqVal := reflect.New(re.reqType).Interface()
			args := reqVal.(*Args)
			args.Num1 = 1
			args.Num2 = 2
			_, _ = re.handler(ctx, reqVal)
		}
	})
}
