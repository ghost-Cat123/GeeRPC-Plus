package client

import (
	"GrowRPC"
	"GrowRPC/xclient"
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerInterceptor 按地址维护独立熔断器
// 需要在调用链中透传目标地址（通过 context）
func CircuitBreakerInterceptor(threshold int, timeout time.Duration) GrowRPC.ClientInterceptor {
	// 默认赋值
	if threshold <= 0 {
		threshold = 5
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	breakers := &sync.Map{} // key: rpcAddr → *CircuitBreaker

	return func(next GrowRPC.Invoker) GrowRPC.Invoker {
		return func(ctx context.Context, method string, req, reply interface{}) error {
			// 从 ctx 提取目标地址（由 XClient.Call 注入）
			addr, _ := ctx.Value(xclient.ContextKeyTargetAddr).(string)

			cb := getOrCreateBreaker(breakers, addr, threshold, timeout)
			if !cb.Allow() {
				return fmt.Errorf("circuit breaker open for %s", addr)
			}

			err := next(ctx, method, req, reply)
			cb.Record(err)
			return err
		}
	}
}

func getOrCreateBreaker(m *sync.Map, addr string, threshold int, timeout time.Duration) *CircuitBreaker {
	v, _ := m.LoadOrStore(addr, &CircuitBreaker{
		state:     StateClosed,
		threshold: threshold,
		timeout:   timeout,
	})
	return v.(*CircuitBreaker)
}
