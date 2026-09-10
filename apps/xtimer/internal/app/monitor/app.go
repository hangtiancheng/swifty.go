package monitor

import (
	"context"
	"sync"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	monitorservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/monitor"
)

type MonitorApp struct {
	sync.Once
	ctx    context.Context
	stop   func()
	worker *monitorservice.Worker
}

func NewMonitorApp(worker *monitorservice.Worker) *MonitorApp {
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
