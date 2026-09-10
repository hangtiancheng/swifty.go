package scheduler

import (
	"context"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/pool"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/trigger"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/utils"
)

type Worker struct {
	pool            pool.WorkerPool
	appConfProvider appConfProvider
	trigger         *trigger.Worker
	lockService     lockService
}

func NewWorker(trigger *trigger.Worker, redisClient *redis.Client, appConfProvider *config.SchedulerAppConfProvider) *Worker {
	return &Worker{
		pool:            pool.NewGoWorkerPool(appConfProvider.Get().WorkersNum),
		trigger:         trigger,
		lockService:     redisClient,
		appConfProvider: appConfProvider,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.trigger.Start(ctx)

	ticker := time.NewTicker(time.Duration(w.appConfProvider.Get().TryLockGapMilliSeconds) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.WarnContext(ctx, "stopped")
			return nil
		case <-ticker.C:
		}

		w.handleSlices(ctx)
	}
}

func (w *Worker) handleSlices(ctx context.Context) {
	for i := range w.appConfProvider.Get().BucketsNum {
		w.handleSlice(ctx, i)
	}
}

func (w *Worker) handleSlice(ctx context.Context, bucketID int) {
	now := time.Now()
	if err := w.pool.Submit(func() {
		w.asyncHandleSlice(ctx, now.Add(-time.Minute), bucketID)
	}); err != nil {
		log.ErrorContextf(ctx, "[handle slice] submit task failed, err: %v", err)
	}
	if err := w.pool.Submit(func() {
		w.asyncHandleSlice(ctx, now, bucketID)
	}); err != nil {
		log.ErrorContextf(ctx, "[handle slice] submit task failed, err: %v", err)
	}
}

func (w *Worker) asyncHandleSlice(ctx context.Context, t time.Time, bucketID int) {
	lockKey := utils.GetTimeBucketLockKey(t, bucketID)
	locker := w.lockService.GetDistributionLock(lockKey)
	if err := locker.Lock(ctx, int64(w.appConfProvider.Get().TryLockSeconds)); err != nil {
		return
	}

	log.InfoContextf(ctx, "get scheduler lock success, key: %s", lockKey)

	ack := func() {
		if err := locker.ExpireLock(ctx, int64(w.appConfProvider.Get().SuccessExpireSeconds)); err != nil {
			log.ErrorContextf(ctx, "expire lock failed, lock key: %s, err: %v", lockKey, err)
		}
	}

	if err := w.trigger.Work(ctx, utils.GetSliceMsgKey(t, bucketID), ack); err != nil {
		log.ErrorContextf(ctx, "trigger work failed, err: %v", err)
	}
}

type appConfProvider interface {
	Get() *config.SchedulerAppConf
}

type lockService interface {
	GetDistributionLock(key string) redis.DistributeLocker
}
