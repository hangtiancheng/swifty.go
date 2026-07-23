package monitor

import (
	"context"
	"sync"

	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/log"
	service "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/monitor"
)

type MonitorApp struct {
	sync.Once
	ctx    context.Context
	stop   func()
	worker *service.Worker
}

func NewMonitorApp(worker *service.Worker) *MonitorApp {
	m := MonitorApp{
		worker: worker,
	}

	m.ctx, m.stop = context.WithCancel(context.Background())
	return &m
}

func (m *MonitorApp) Start() {
	m.Do(func() {
		log.InfoContext(m.ctx, "monitor is starting")
		go m.worker.Start(m.ctx)
	})
}

func (m *MonitorApp) Stop() {
	m.stop()
}
