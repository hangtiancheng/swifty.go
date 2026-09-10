package migrator

import (
	"context"
	"sync"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	migratorservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/migrator"
)

// MigratorApp periodically loads task records derived from the timer table
// and inserts them into the task table.
type MigratorApp struct {
	sync.Once
	ctx    context.Context
	stop   func()
	worker *migratorservice.Worker
}

func NewMigratorApp(worker *migratorservice.Worker) *MigratorApp {
	m := MigratorApp{
		worker: worker,
	}

	m.ctx, m.stop = context.WithCancel(context.Background())
	return &m
}

func (m *MigratorApp) Start() {
	m.Do(func() {
		log.InfoContext(m.ctx, "migrator is starting")
		go func() {
			if err := m.worker.Start(m.ctx); err != nil {
				log.ErrorContextf(m.ctx, "start worker failed, err: %v", err)
			}
		}()
	})
}

func (m *MigratorApp) Stop() {
	m.stop()
}
