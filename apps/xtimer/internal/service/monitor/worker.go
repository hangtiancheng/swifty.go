package monitor

import (
	"context"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/consts"
	taskdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/task"
	timerdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/timer"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/prometheus"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/utils"
)

type Worker struct {
	lockService *redis.Client
	taskDAO     *taskdao.TaskDAO
	timerDAO    *timerdao.TimerDAO
	reporter    *prometheus.Reporter
}

func NewWorker(taskDAO *taskdao.TaskDAO, timerDAO *timerdao.TimerDAO, lockService *redis.Client, reporter *prometheus.Reporter) *Worker {
	return &Worker{
		taskDAO:     taskDAO,
		timerDAO:    timerDAO,
		lockService: lockService,
		reporter:    reporter,
	}
}

// Start reports the number of failed timers every minute.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		now := time.Now()
		lock := w.lockService.GetDistributionLock(utils.GetMonitorLockKey(now))
		if err := lock.Lock(ctx, 2*int64(time.Minute/time.Second)); err != nil {
			continue
		}

		// Query the timers of the previous minute.
		minute := utils.GetMinute(now)
		go w.reportUnexecutedTasksCnt(ctx, minute)
		go w.reportEnabledTimersCnt(ctx)
	}
}

func (w *Worker) reportUnexecutedTasksCnt(ctx context.Context, minute time.Time) {
	unexecutedTasksCnt, err := w.taskDAO.Count(ctx, taskdao.WithStartTime(minute.Add(-time.Minute)), taskdao.WithEndTime(minute), taskdao.WithStatus(int32(consts.NotRun)))
	if err != nil {
		log.ErrorContextf(ctx, "[monitor] get unexecuted tasks cnt failed, err: %v", err)
		return
	}
	w.reporter.ReportTimerUnexecutedRecord(float64(unexecutedTasksCnt))
	log.InfoContextf(ctx, "[monitor] report unexecuted tasks cnt success, cnt: %d", unexecutedTasksCnt)
}

func (w *Worker) reportEnabledTimersCnt(ctx context.Context) {
	enabledTimerCnt, err := w.timerDAO.Count(ctx, timerdao.WithStatus(int32(consts.Enabled)))
	if err != nil {
		log.ErrorContextf(ctx, "[monitor] get enabled timer cnt failed, err: %v", err)
		return
	}
	w.reporter.ReportTimerEnabledRecord(float64(enabledTimerCnt))
	log.InfoContextf(ctx, "[monitor] report enabled timer cnt success, cnt: %d", enabledTimerCnt)
}
