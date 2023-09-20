// sig 是接收信号通知的包

package sig

import (
	"context"
	"github.com/gogf/gf/v2/os/grpool"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var once sync.Once

var server *SignalInstance

type SignalFlag struct {
	flag os.Signal
}

// String 获取信号量转字符串
func (s *SignalFlag) String() string {
	switch s.flag {
	case os.Interrupt:
		return "os.Interrupt"
	case syscall.SIGTERM:
		return "syscall.SIGTERM"
	}
	return "unknown signal"
}

// GetFlag 获取信号量
func (s *SignalFlag) GetFlag() os.Signal {
	return s.flag
}

// Func 所有方法必须实现优雅退出,已保证退出后所做的事
type Func interface {
	GracefulClose(ctx context.Context, signal *SignalFlag)
	RecoverFunc(ctx context.Context, err error)
}

// SignalInstance 是一个信号抽象
type SignalInstance struct {
	// 方法集合
	funcArr []Func
	// 系统信号
	signalChan chan os.Signal
	// 当接收到退出信号时,将直接取消
	quickCtx context.Context
	// 快速退出方法
	quickCancel context.CancelFunc
	// 等待全体退出
	waitContext context.Context
	// 取消方法
	cancelFunc context.CancelFunc
}

// GracefulClose 注册关闭方法
func (s *SignalInstance) RegisterClose(f Func) context.Context {
	server.funcArr = append(server.funcArr, f)
	return server.quickCtx
}

// Close 主动触发关闭信号,触发优雅停止方法
func (s *SignalInstance) Close() {
	s.signalChan <- os.Interrupt
}

// Get 获取单例
func Get() *SignalInstance {
	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		quickCtx, quickCancel := context.WithCancel(context.Background())
		server = &SignalInstance{
			funcArr:     make([]Func, 0),
			signalChan:  make(chan os.Signal),
			quickCtx:    quickCtx,
			quickCancel: quickCancel,
			waitContext: ctx,
			cancelFunc:  cancel,
		}

		go func() {
			signal.Notify(server.signalChan, os.Interrupt, syscall.SIGTERM)
			sig := <-server.signalChan
			wg := new(sync.WaitGroup)
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				server.quickCancel()
				for i := range server.funcArr {
					wg.Add(1)
					doFunc := server.funcArr[i]
					_ = grpool.AddWithRecover(ctx, func(ctx context.Context) {
						defer wg.Done()
						doFunc.GracefulClose(ctx, &SignalFlag{
							flag: sig,
						})
					}, func(ctx context.Context, err error) {
						doFunc.RecoverFunc(ctx, err)
					})
				}
				wg.Wait()
				server.cancelFunc()
			}
		}()
	})
	return server
}

// WaitClosePending 等待最终关闭
func WaitClosePending() {
	<-Get().waitContext.Done()
}
