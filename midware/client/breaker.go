package client

import (
	"GrowRPC"
	"sync"
	"time"
)

type State int32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu        sync.Mutex
	state     State
	failures  int
	lastFail  time.Time
	threshold int           // 连续失败次数阈值
	timeout   time.Duration // Open→HalfOpen 冷却时间
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if now.Sub(cb.lastFail) > cb.timeout {
			cb.state = StateHalfOpen
			return true // 放行试探
		}
		return false
	case StateHalfOpen:
		return true // 放行试探（由 record 决定恢复还是重新熔断）
	}
	return true
}

func (cb *CircuitBreaker) Record(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err == nil {
		cb.failures = 0
		cb.state = StateClosed
		return
	}

	// 业务错误不计数（服务还活着）
	if !GrowRPC.IsRetryable(err) {
		return
	}

	// 网络错误才计数
	cb.failures++
	cb.lastFail = time.Now()

	if cb.state == StateClosed && cb.failures >= cb.threshold {
		cb.state = StateOpen
	} else if cb.state == StateHalfOpen {
		cb.state = StateOpen
	}
}
