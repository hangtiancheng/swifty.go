package trigger

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/concurrency"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/pool"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/executor"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/utils"
)

type Worker struct {
	task         taskService
	confProvider confProvider
	pool         pool.WorkerPool
	executor     *executor.Worker
	lockService  *redis.Client
}

func NewWorker(executor *executor.Worker, task *TaskService, lockService *redis.Client, confProvider *config.TriggerAppConfProvider) *Worker {
	return &Worker{
		executor:     executor,
		task:         task,
		lockService:  lockService,
		pool:         pool.NewGoWorkerPool(confProvider.Get().WorkersNum),
		confProvider: confProvider,
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.executor.Start(ctx)
}

// Work processes one minute of tasks for a bucket, polling the zset every
// ZRangeGapSeconds until the minute elapses.
func (w *Worker) Work(ctx context.Context, minuteBucketKey string, ack func()) error {
	startTime, err := getStartMinute(minuteBucketKey)
	if err != nil {
		return err
	}

	conf := w.confProvider.Get()
	gap := time.Duration(conf.ZRangeGapSeconds) * time.Second
	ticker := time.NewTicker(gap)
	defer ticker.Stop()

	endTime := startTime.Add(time.Minute)

	notifier := concurrency.NewSafeChan(int(time.Minute/gap) + 1)
	defer notifier.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		if err := w.handleBatch(ctx, minuteBucketKey, startTime, startTime.Add(gap)); err != nil {
			notifier.Put(err)
		}
	})
	for range ticker.C {
		select {
		case e := <-notifier.GetChan():
			err, _ = e.(error)
			return err
		default:
		}

		if startTime = startTime.Add(gap); startTime.Equal(endTime) || startTime.After(endTime) {
			break
		}

		wg.Add(1)
		go func(startTime time.Time) {
			defer wg.Done()
			if err := w.handleBatch(ctx, minuteBucketKey, startTime, startTime.Add(gap)); err != nil {
				notifier.Put(err)
			}
		}(startTime)
	}

	wg.Wait()
	select {
	case e := <-notifier.GetChan():
		err, _ = e.(error)
		return err
	default:
	}

	ack()
	log.InfoContextf(ctx, "ack success, key: %s", minuteBucketKey)
	return nil
}

func (w *Worker) handleBatch(ctx context.Context, key string, start, end time.Time) error {
	bucket, err := getBucket(key)
	if err != nil {
		return err
	}

	tasks, err := w.task.GetTasksByTime(ctx, key, bucket, start, end)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if err := w.pool.Submit(func() {
			if err := w.executor.Work(ctx, utils.UnionTimerIDUnix(task.TimerID, task.RunTimer.UnixMilli())); err != nil {
				log.ErrorContextf(ctx, "executor work failed, err: %v", err)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

func getStartMinute(slice string) (time.Time, error) {
	timeBucket := strings.Split(slice, "_")
	if len(timeBucket) != 2 {
		return time.Time{}, fmt.Errorf("invalid format of msg key: %s", slice)
	}

	return utils.GetStartMinute(timeBucket[0])
}

func getBucket(slice string) (int, error) {
	timeBucket := strings.Split(slice, "_")
	if len(timeBucket) != 2 {
		return -1, fmt.Errorf("invalid format of msg key: %s", slice)
	}
	return strconv.Atoi(timeBucket[1])
}

type taskService interface {
	GetTasksByTime(ctx context.Context, key string, bucket int, start, end time.Time) ([]*vo.Task, error)
}

type confProvider interface {
	Get() *config.TriggerAppConf
}
