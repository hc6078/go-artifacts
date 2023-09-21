package crontab

import (
	"context"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/robfig/cron/v3"
	"github.com/vrieske/go-artifacts/sig"
	"log"
	"sync"
)

type (
	// cronTask 一个定时任务
	cronTask struct {
		f           func(ctx context.Context)
		Spec        string
		immediately bool
		closeCtx    context.Context
	}
	Crontab struct {
		cron          *cron.Cron
		tasks         []*cronTask
		closeCtx      context.Context
		immediatelyWg *sync.WaitGroup
	}
)

var crontabInstance *Crontab

var crontabOnce sync.Once

// GetFunc 获取执行函数
func (s *cronTask) GetFunc() func() {
	return func() {
		s.f(s.closeCtx)
	}
}

// NewCrontab 实例化定时任务
func NewCrontab() *Crontab {
	crontabOnce.Do(func() {
		crontabInstance = &Crontab{
			cron: cron.New(cron.WithSeconds()), tasks: make([]*cronTask, 0),
			immediatelyWg: new(sync.WaitGroup),
		}
		crontabInstance.closeCtx = sig.Get().RegisterClose(crontabInstance)
	})
	return crontabInstance
}

// GracefulClose 实现优雅退出方法
func (s *Crontab) GracefulClose(ctx context.Context, signal *sig.SignalFlag) {
	glog.Info(ctx, "Crontab waiting close")
	<-s.cron.Stop().Done()
	s.immediatelyWg.Wait()
	glog.Info(ctx, "Crontab close")
}

func (s *Crontab) RecoverFunc(ctx context.Context, err error) {
	glog.Error(ctx, "Crontab Recover ", err)
}

// AddFunc 新增定时func
func (s *Crontab) AddFunc(spec string, f func(ctx context.Context), immediately bool) {
	s.tasks = append(s.tasks, &cronTask{
		closeCtx:    s.closeCtx,
		f:           f,
		Spec:        spec,
		immediately: immediately,
	})
}

// Start 启动
func (s *Crontab) Start() {
	doImmediately := func(task *cronTask) {
		s.immediatelyWg.Add(1)
		defer s.immediatelyWg.Done()
		task.GetFunc()()
	}
	for _, item := range s.tasks {
		if _, err := s.cron.AddFunc(item.Spec, item.GetFunc()); err != nil {
			log.Fatal(err.Error())
		}
		// 立刻执行
		if item.immediately {
			go doImmediately(item)
		}
	}
	s.cron.Start()
}
