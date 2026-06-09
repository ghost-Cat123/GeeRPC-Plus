package server

import (
	. "GrowRPC"
	"fmt"
	"log"
)

func RecoveryInterceptor(next HandlerFunc) HandlerFunc {
	return func(i *CallInfo) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[RPC Panic Recovered] Method: %s | Panic: %v", i.ServiceMethod, r)
				err = fmt.Errorf("internal panic: %v", r)
			}
		}()
		err = next(i)
		// 命名返回值defer return 前修改返回值
		return err
	}
}
