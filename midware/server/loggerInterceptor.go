package server

import (
	. "GrowRPC"
	"log"
	"time"
)

// LoggerInterceptor 1. 日志与请求耗时拦截器
func LoggerInterceptor(next HandlerFunc) HandlerFunc {
	return func(i *CallInfo) error {
		start := time.Now()
		log.Printf("[RPC Call Start] Method: %s | Argv: %v", i.ServiceMethod, i.ReqArgs)
		err := next(i)
		log.Printf("[RPC Call End] Method: %s | Cost: %v | Error: %v", i.ServiceMethod, time.Since(start), err)
		return err
	}
}
