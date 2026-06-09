# GrowRPC

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

GrowRPC 是一个从零实现的轻量级 Go RPC 框架，参考 `net/rpc` 与 gRPC 的设计思想，提供完整的服务通信、服务治理与高可用能力。

## 功能特性

### 核心通信

| 特性 | 说明 |
|------|------|
| **多协议支持** | Gob / JSON / Protobuf 三种序列化协议，Option 握手自动协商 |
| **单连接多路复用** | Seq 序号 + pending map，单条 TCP 连接承载多个并发请求，类 HTTP/2 Stream ID |
| **自定义 TLV 帧格式** | 4 字节大端长度前缀 + Protobuf Payload，解决 TCP 粘包/半包 |
| **HTTP CONNECT 隧道** | `http.Hijacker` 劫持连接，使 RPC 流量可穿透 HTTP 代理 |
| **TLS 传输加密** | 通过 `Option.TLSConfig` 零代码改动启用 mTLS |

### 高性能设计

| 特性 | 说明 |
|------|------|
| **泛型零反射路由** | Go 1.18+ 泛型 + 闭包工厂消除 `reflect`，handler 调用 10 ns/op（vs 反射 471 ns，47x） |
| **客户端连接池** | `sync.Mutex` + LIFO 栈 + `active` 精准计数 + 后台清理 goroutine |
| **并发安全优化** | `sending` 细粒度写锁 + `mu` 读锁分离，消除锁竞争 |

### 服务治理

| 特性 | 说明 |
|------|------|
| **ETCD 注册中心** | 基于 Lease 租约 + KeepAlive 心跳 + Watch 实时推送，秒级故障感知 |
| **负载均衡** | Random / Round-Robin / 一致性哈希（CRC32 + 虚拟节点） |
| **Context 级联超时** | 客户端 Deadline 穿越网络透传至服务端，下游 DB/Redis 可感知并提前熔断 |
| **令牌桶限流** | 无锁 CAS 实现，支持全局限流 + 按方法粒度精细化限流 |

### 中间件系统

| 特性 | 位置 | 说明 |
|------|------|------|
| **Logger** | 服务端 | 记录请求耗时、参数与错误 |
| **Recovery** | 服务端 | panic 全局兜底，防止服务崩溃 |
| **RateLimit** | 服务端 | 令牌桶限流，QPS 保护 |
| **Retry** | 客户端 | 指数退避重试，自动区分业务错误与网络错误 |
| **CircuitBreaker** | 客户端 | 三态熔断器（Closed → Open → HalfOpen），按节点隔离 |
| **Degradation** | 客户端 | 熔断后自动降级到 fallback 回调 |

### 代码生成

| 特性 | 说明 |
|------|------|
| **`protoc-gen-growrpc`** | 自定义 protoc 插件，从 `.proto` 文件自动生成 Server 接口 + Client Stub |

## 项目结构

```
GrowRPC/
├── codec/                     # 编解码器层
│   ├── pb/                    # Protobuf 定义与生成代码
│   ├── codec.go               # Codec 接口 + Header 定义
│   ├── gob.go                 # Gob 编解码
│   ├── json.go                # JSON 编解码
│   ├── protobuf.go            # TLV 帧 + Protobuf 编解码
│   └── header.proto           # 网络传输头定义
├── midware/
│   ├── server/                # 服务端拦截器
│   │   ├── loggerInterceptor.go
│   │   ├── recoveryInterceptor.go
│   │   ├── ratelimit.go       # 令牌桶算法
│   │   └── rateLimitInterceptor.go
│   └── client/                # 客户端拦截器
│       ├── clientInterceptor.go # 重试拦截器
│       ├── breaker.go           # 熔断器状态机
│       ├── circuitBreakerInterceptor.go
│       └── degradationInterceptor.go
├── registry/
│   ├── registry.go            # HTTP 注册中心（Deprecated）
│   └── etcd.go                # ETCD 注册中心
├── xclient/
│   ├── consistent_hash.go     # 一致性哈希环
│   ├── discovery.go           # Discovery 接口 + 手工维护实现
│   ├── discovery_gee.go       # HTTP 注册中心发现
│   ├── discovery_etcd.go      # ETCD Watch 发现
│   └── xclient.go             # 负载均衡客户端
├── pool/
│   └── pool.go                # 连接池
├── cmd/
│   └── protoc-gen-growrpc/
│       └── main.go            # protoc 代码生成插件
├── benchmark/                 # 泛型 vs 反射性能对比
├── docs/                      # 技术文档
├── server.go                  # RPC 服务端
├── client.go                  # RPC 客户端（拦截器链 + RpcError）
├── service.go                 # 泛型方法注册
├── debug.go                   # Debug HTTP 端点
└── main/main.go               # 完整示例
```

## 快速开始

### 安装

```bash
go get github.com/ghost-Cat123/GrowRPC
```

### 定义服务（.proto 文件）

```protobuf
// math.proto
syntax = "proto3";
package math;
option go_package = "./pb";

message MathArgs {
  int32 a = 1;
  int32 b = 2;
}

message MathReply {
  int32 result = 1;
}

service MathService {
  rpc Add(MathArgs) returns (MathReply);
  rpc Divide(MathArgs) returns (MathReply);
}
```

### 生成代码

```bash
# 安装插件
go install ./cmd/protoc-gen-growrpc

# 生成 pb + growrpc stub
protoc --go_out=. --growrpc_out=. math.proto
```

生成产物：
- `math.pb.go` — 消息类型定义
- `math_growrpc.pb.go` — Server interface + Register 函数 + Client Stub

### 服务端

```go
package main

import (
	"GrowRPC"
	"GrowRPC/midware/server"
	"GrowRPC/registry"
	"context"
	"net"
	pb "yourproject/pb"
)

// 实现生成的接口
type mathImpl struct {
	pb.UnimplementedMathServiceServer
}

func (m *mathImpl) Add(ctx context.Context, req *pb.MathArgs, resp *pb.MathReply) error {
	resp.Result = req.A + req.B
	return nil
}

func main() {
	// 注册服务端中间件
	bucket := server.NewTokenBucket(1000, 2000)
	GrowRPC.Use(
		server.RateLimitInterceptor(bucket, nil),
		server.LoggerInterceptor,
		server.RecoveryInterceptor,
	)

	// 一行注册
	pb.RegisterMathServiceServer(GrowRPC.DefaultServer, &mathImpl{})

	// ETCD 注册
	reg, _ := registry.NewEtcdRegistry([]string{"localhost:2379"}, 10)
	reg.Register("MathService", "192.168.1.5:8080", nil)
	go reg.KeepAlive(context.Background())

	// 启动监听
	lis, _ := net.Listen("tcp", ":8080")
	GrowRPC.Accept(lis)
}
```

### 客户端

```go
package main

import (
    "GrowRPC"
    "GrowRPC/xclient"
    "GrowRPC/midware/client"
    clientv3 "go.etcd.io/etcd/client/v3"
    pb "yourproject/pb"
    "context"
    "time"
)

func main() {
    // ETCD 发现
    etcdCli, _ := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
    discovery := xclient.NewEtcdDiscovery(etcdCli, "MathService")

    // 创建负载均衡客户端
    opt := &GrowRPC.Option{CodecType: "application/protobuf"}
    xc := xclient.NewXClient(discovery, xclient.RoundRobinSelect, opt)

    // 注册客户端中间件（外层先执行）
    xc.Use(
        client.DegradationInterceptor(/* fallbacks */),
        client.CircuitBreakerInterceptor(5, 30*time.Second),
        client.RetryInterceptor(client.RetryConfig{MaxAttempts: 3}),
    )

    // 类型安全的调用
    mathClient := pb.NewMathServiceClient(xc)
    reply, err := mathClient.Add(context.Background(), &pb.MathArgs{A: 10, B: 20})
    // reply.Result == 30
}
```

### 不使用代码生成（直接注册）

```go
type MathArgs struct{ A, B int }
type MathReply struct{ Result int }
type MathService struct{}

func (m *MathService) Add(_ context.Context, args *MathArgs, reply *MathReply) error {
    reply.Result = args.A + args.B
    return nil
}

GrowRPC.RegisterMethod[MathArgs, MathReply](
    GrowRPC.DefaultServer,
    "MathService.Add",
    (&MathService{}).Add,
)
```

## 中间件组装顺序

### 服务端（`server.Use`）

```
RateLimit → Logger → Recovery → handler
```

越晚注册越靠外层。

### 客户端（`xc.Use`）

```
Degradation → CircuitBreaker → Retry → rawInvoker
```

执行流程：
1. Degradation 兜底，捕获所有下层错误
2. CircuitBreaker：Open 时快速失败 → Degradation 降级
3. Retry：网络抖动指数退避重试，业务错误直接返回
4. rawInvoker：discovery.Get → pool.Get → Client.Call

## 错误分类

| 错误类型 | `IsRetryable` | 说明 |
|---------|:---:|------|
| `RpcError`（服务端业务错误） | ❌ | 请求被正确处理但被拒绝 |
| `context.Canceled` / `DeadlineExceeded` | ❌ | 用户主动取消或超时 |
| `net.OpError`（连接拒绝/超时/重置） | ✅ | 网络基础设施故障，换节点可恢复 |
| `io.EOF` / `io.ErrUnexpectedEOF` | ✅ | 对端异常断开 |

## 配置项

```go
type Option struct {
    MagicNumber    int              // 协议魔数
    CodecType      codec.Type       // "application/gob" | "application/json" | "application/protobuf"
    ConnectTimeout time.Duration    // 连接超时
    HandleTimeout  time.Duration    // 服务端处理超时
    TLSConfig      *tls.Config      // TLS 配置（nil = 明文）
}
```

## TODO

- [x] Context 级联超时透传
- [x] 泛型零反射服务路由 + Benchmark
- [x] 客户端连接池（LIFO + active 精准计数）
- [x] ETCD 注册中心（Lease + KeepAlive + Watch）
- [x] protoc 代码生成插件（`protoc-gen-growrpc`）
- [x] 服务端中间件：Logger / Recovery / RateLimit
- [x] 客户端拦截器链 + Retry / CircuitBreaker / Degradation
- [x] 错误分类（`RpcError` + `IsRetryable`）
- [x] TLS 传输加密
- [x] 优雅关闭（GracefulShutdown）
- [x] ETCD Watch 自动重连 + KeepAlive 循环重建
- [ ] 流式 RPC（Server Streaming / Client Streaming / Bidi）
- [ ] 客户端限流拦截器
- [ ] OpenTelemetry Tracing
- [ ] Reactor 网络模型（gnet）

## 性能数据

| 场景 | 泛型方案 | 反射方案 | 提升 |
|------|---------|---------|:---:|
| handler 调用开销 | 10 ns / 1 alloc | 471 ns / 6 allocs | 47x |
| 实例创建开销 | 12 ns / 1 alloc | 18 ns / 1 alloc | 1.5x |
| 完整路由分发 | 34 ns / 2 allocs | 41 ns / 2 allocs | 1.2x |

## License

MIT
