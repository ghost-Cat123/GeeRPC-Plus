package client

import (
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

func (cb *CircuitBreaker) Record(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.failures = 0
		cb.state = StateClosed
		return
	}

	cb.failures++
	cb.lastFail = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.threshold {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		// 试探失败，立即重新熔断
		cb.state = StateOpen
	case StateOpen:
	}
}
