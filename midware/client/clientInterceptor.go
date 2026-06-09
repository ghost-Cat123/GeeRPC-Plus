package client

import (
	"GrowRPC"
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig 指数退避进行重试 客户端中间件
type RetryConfig struct {
	MaxAttempts   int           // 最大尝试次数（含首次），默认 3
	BaseBackoff   time.Duration // 基础退避，默认 100ms
	MaxBackoff    time.Duration // 退避上限，默认 3s
	BackoffFactor float64       // 退避倍率，默认 2.0
}

func RetryInterceptor(cfg RetryConfig) GrowRPC.ClientInterceptor {
	// 填充默认值
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 100 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 3 * time.Second
	}
	if cfg.BackoffFactor <= 0 {
		cfg.BackoffFactor = 2.0
	}

	return func(next GrowRPC.Invoker) GrowRPC.Invoker {
		return func(ctx context.Context, method string, req, reply interface{}) error {
			var lastErr error
			for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
				// 每次重试前检查 ctx 是否已取消
				if ctx.Err() != nil {
					return ctx.Err()
				}

				err := next(ctx, method, req, reply)
				if err == nil {
					return nil
				}

				// 不可重试的错误（业务错误如 "参数非法"）直接返回
				if !GrowRPC.IsRetryable(err) {
					return err
				}

				lastErr = err

				// 最后一次不等待
				if attempt < cfg.MaxAttempts-1 {
					backoff := time.Duration(float64(cfg.BaseBackoff) * math.Pow(cfg.BackoffFactor, float64(attempt)))
					if backoff > cfg.MaxBackoff {
						backoff = cfg.MaxBackoff
					}
					select {
					case <-time.After(backoff):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			return fmt.Errorf("retry exhausted after %d attempts: %w", cfg.MaxAttempts, lastErr)
		}
	}
}
