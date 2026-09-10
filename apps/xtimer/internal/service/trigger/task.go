package trigger

import (
	"context"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/consts"
	taskdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/task"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/po"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
)

type TaskService struct {
	confProvider *config.SchedulerAppConfProvider
	cache        *taskdao.TaskCache
	dao          taskDAO
}

func NewTaskService(dao *taskdao.TaskDAO, cache *taskdao.TaskCache, confProvider *config.SchedulerAppConfProvider) *TaskService {
	return &TaskService{
		confProvider: confProvider,
		dao:          dao,
		cache:        cache,
	}
}

func (t *TaskService) GetTasksByTime(ctx context.Context, key string, bucket int, start, end time.Time) ([]*vo.Task, error) {
	// Try the cache first.
	if tasks, err := t.cache.GetTasksByTime(ctx, key, start.UnixMilli(), end.UnixMilli()); err == nil && len(tasks) > 0 {
		return vo.NewTasks(tasks), nil
	}

	// Fall back to the database on a cache miss.
	tasks, err := t.dao.GetTasks(ctx, taskdao.WithStartTime(start), taskdao.WithEndTime(end), taskdao.WithStatus(int32(consts.NotRun.ToInt())))
	if err != nil {
		return nil, err
	}

	maxBucket := t.confProvider.Get().BucketsNum
	var validTask []*po.Task
	for _, task := range tasks {
		if task.TimerID%uint(maxBucket) != uint(bucket) {
			continue
		}
		validTask = append(validTask, task)
	}

	return vo.NewTasks(validTask), nil
}

type taskDAO interface {
	GetTasks(ctx context.Context, opts ...taskdao.Option) ([]*po.Task, error)
}
