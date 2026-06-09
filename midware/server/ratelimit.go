package server

import (
	"sync/atomic"
	"time"
)

type TokenBucket struct {
	rate       float64      // 每秒生成令牌数（QPS）
	burst      int64        // 桶容量（最大突发请求数）
	tokens     atomic.Int64 // 当前令牌数
	lastRefill atomic.Int64 // 上次填充时间（纳秒）
}

func NewTokenBucket(rate float64, burst int64) *TokenBucket {
	tb := &TokenBucket{rate: rate, burst: burst}
	tb.tokens.Store(burst) // 初始满桶
	tb.lastRefill.Store(time.Now().UnixNano())
	return tb
}

func (tb *TokenBucket) Allow() bool {
	now := time.Now().UnixNano()
	last := tb.lastRefill.Load()

	// 阶段1：计算新令牌数，尝试 CAS 更新 lastRefill + tokens
	// 注意：只有成功 CAS 的那个 goroutine 才做 refill，其他竞争失败的跳过
	elapsed := float64(now-last) / 1e9
	newTokens := int64(elapsed * tb.rate)
	if newTokens > 0 {
		if tb.lastRefill.CompareAndSwap(last, now) {
			current := tb.tokens.Load()
			current += newTokens
			if current > tb.burst {
				current = tb.burst
			}
			tb.tokens.Store(current)
		}
	}

	// 阶段2：CAS 扣减 1 个令牌
	for {
		current := tb.tokens.Load()
		if current <= 0 {
			return false
		}
		if tb.tokens.CompareAndSwap(current, current-1) {
			return true
		}
	}
}
