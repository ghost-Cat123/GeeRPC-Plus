package server

import (
	"GrowRPC"
	"fmt"
)

// RateLimitInterceptor 服务端限流中间件
// 支持全局限流 + 按方法级别的精细化限流
func RateLimitInterceptor(global *TokenBucket, perMethod map[string]*TokenBucket) GrowRPC.Interceptor {
	return func(next GrowRPC.HandlerFunc) GrowRPC.HandlerFunc {
		return func(i *GrowRPC.CallInfo) error {
			// 先检查方法级别
			if tb := perMethod[i.ServiceMethod]; tb != nil {
				if !tb.Allow() {
					return fmt.Errorf("rpc rate limit exceeded: %s", i.ServiceMethod)
				}
			} else if global != nil {
				// 再检查全局
				if !global.Allow() {
					return fmt.Errorf("rpc rate limit exceeded")
				}
			}
			return next(i)
		}
	}
}
