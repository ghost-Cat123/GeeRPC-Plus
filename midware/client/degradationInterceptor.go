package client

import (
	"GrowRPC"
	"context"
	"reflect"
)

type FallbackFunc func(ctx context.Context, req interface{}) (interface{}, error)

// DegradationInterceptor 降级拦截器
func DegradationInterceptor(fallbacks map[string]FallbackFunc) GrowRPC.ClientInterceptor {
	return func(next GrowRPC.Invoker) GrowRPC.Invoker {
		return func(ctx context.Context, method string, req, reply interface{}) error {
			err := next(ctx, method, req, reply)
			if err == nil {
				return nil
			}

			// 有配置 fallback 且不是 ctx 取消 → 走降级
			fb, ok := fallbacks[method]
			if !ok || ctx.Err() != nil {
				return err
			}

			fbResp, fbErr := fb(ctx, req)
			if fbErr != nil {
				return err // 降级也失败，返回原始错误
			}

			// 用降级结果填充 reply
			if reply != nil && fbResp != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(fbResp).Elem())
			}
			return nil // 降级成功，视作正常
		}
	}
}
