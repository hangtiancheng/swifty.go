package webserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/consts"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/cron"
	taskdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/task"
	timerdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/timer"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/po"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/mysql"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/utils"
)

const defaultEnableGapSeconds = 3

type TimerService struct {
	dao                 timerDAO
	confProvider        confProvider
	migrateConfProvider *config.MigratorAppConfProvider
	cronParser          cronParser
	taskCache           taskCache
	lockService         *redis.Client
}

func NewTimerService(dao *timerdao.TimerDAO, taskCache *taskdao.TaskCache, lockService *redis.Client,
	confProvider *config.WebServerAppConfProvider, migrateConfProvider *config.MigratorAppConfProvider, parser *cron.CronParser) *TimerService {
	return &TimerService{
		dao:                 dao,
		confProvider:        confProvider,
		migrateConfProvider: migrateConfProvider,
		taskCache:           taskCache,
		cronParser:          parser,
		lockService:         lockService,
	}
}

func (t *TimerService) CreateTimer(ctx context.Context, timer *vo.Timer) (uint, error) {
	lock := t.lockService.GetDistributionLock(utils.GetCreateLockKey(timer.App))
	if err := lock.Lock(ctx, defaultEnableGapSeconds); err != nil {
		return 0, errors.New("create/delete operations are too frequent, please try again later")
	}
	// Validate the cron expression.
	if !t.cronParser.IsValidCronExpr(timer.Cron) {
		return 0, fmt.Errorf("invalid cron expression: %s", timer.Cron)
	}

	pTimer, err := timer.ToPO()
	if err != nil {
		return 0, err
	}
	return t.dao.CreateTimer(ctx, pTimer)
}

func (t *TimerService) DeleteTimer(ctx context.Context, app string, id uint) error {
	lock := t.lockService.GetDistributionLock(utils.GetCreateLockKey(app))
	if err := lock.Lock(ctx, defaultEnableGapSeconds); err != nil {
		return errors.New("create/delete operations are too frequent, please try again later")
	}
	return t.dao.DeleteTimer(ctx, id)
}

func (t *TimerService) UpdateTimer(ctx context.Context, timer *vo.Timer) error {
	pTimer, err := timer.ToPO()
	if err != nil {
		return err
	}
	pTimer.ID = timer.ID
	return t.dao.UpdateTimer(ctx, pTimer)
}

func (t *TimerService) GetTimer(ctx context.Context, id uint) (*vo.Timer, error) {
	pTimer, err := t.dao.GetTimer(ctx, timerdao.WithID(id))
	if err != nil {
		return nil, err
	}

	return vo.NewTimer(pTimer)
}

func (t *TimerService) EnableTimer(ctx context.Context, app string, id uint) error {
	// Rate limit enable/disable operations per app.
	lock := t.lockService.GetDistributionLock(utils.GetEnableLockKey(app))
	if err := lock.Lock(ctx, defaultEnableGapSeconds); err != nil {
		return errors.New("enable/disable operations are too frequent, please try again later")
	}

	do := func(ctx context.Context, dao *timerdao.TimerDAO, timer *po.Timer) error {
		// Validate the status.
		if timer.Status != consts.Disabled.ToInt() {
			return fmt.Errorf("not disabled status, enable failed, timer id: %d", id)
		}

		// Compute the upcoming execution times.
		// end is the right boundary of the next two time slices.
		start := time.Now()
		end := utils.GetForwardTwoMigrateStepEnd(start, 2*time.Duration(t.migrateConfProvider.Get().MigrateStepMinutes)*time.Minute)
		executeTimes, err := t.cronParser.NextsBefore(timer.Cron, end)
		if err != nil {
			log.ErrorContextf(ctx, "get executeTimes failed, err: %v", err)
			return err
		}

		// Persist the execution times.
		tasks := timer.BatchTasksFromTimer(executeTimes)
		// The unique key on (timer_id, run_timer) prevents duplicate task inserts.
		if err := dao.BatchCreateRecords(ctx, tasks); err != nil && !mysql.IsDuplicateEntryErr(err) {
			return err
		}

		// Push the execution times into the redis zsets.
		if err := t.taskCache.BatchCreateTasks(ctx, tasks, start, end); err != nil {
			return err
		}

		// Switch the timer to the enabled status.
		timer.Status = consts.Enabled.ToInt()
		return dao.UpdateTimer(ctx, timer)
	}

	return t.dao.DoWithLock(ctx, id, do)
}

func (t *TimerService) UnableTimer(ctx context.Context, app string, id uint) error {
	// Rate limit enable/disable operations per app.
	lock := t.lockService.GetDistributionLock(utils.GetEnableLockKey(app))
	if err := lock.Lock(ctx, defaultEnableGapSeconds); err != nil {
		return errors.New("enable/disable operations are too frequent, please try again later")
	}

	do := func(ctx context.Context, dao *timerdao.TimerDAO, timer *po.Timer) error {
		// Validate the status.
		if timer.Status != consts.Enabled.ToInt() {
			return fmt.Errorf("not enabled status, unable failed, timer id: %d", id)
		}

		// Switch the timer to the disabled status.
		timer.Status = consts.Disabled.ToInt()
		return dao.UpdateTimer(ctx, timer)
	}

	return t.dao.DoWithLock(ctx, id, do)
}

func (t *TimerService) GetAppTimers(ctx context.Context, req *vo.GetAppTimersReq) ([]*vo.Timer, int64, error) {
	total, err := t.dao.Count(ctx, timerdao.WithApp(req.App))
	if err != nil {
		return nil, -1, err
	}

	offset, limit := req.Get()
	if total <= int64(offset) {
		return []*vo.Timer{}, total, nil
	}

	timers, err := t.dao.GetTimers(ctx, timerdao.WithApp(req.App), timerdao.WithPageLimit(offset, limit), timerdao.WithDesc())
	if err != nil {
		return nil, -1, err
	}

	sort.Slice(timers, func(i, j int) bool {
		return timers[i].ID > timers[j].ID
	})

	vTimers, err := vo.NewTimers(timers)
	return vTimers, total, err
}

func (t *TimerService) GetTimersByName(ctx context.Context, req *vo.GetTimersByNameReq) ([]*vo.Timer, int64, error) {
	total, err := t.dao.Count(ctx, timerdao.WithApp(req.App), timerdao.WithFuzzyName(req.FuzzyName))
	if err != nil {
		return nil, -1, err
	}

	offset, limit := req.Get()
	if total <= int64(offset) {
		return []*vo.Timer{}, total, nil
	}

	timers, err := t.dao.GetTimers(ctx, timerdao.WithApp(req.App), timerdao.WithPageLimit(offset, limit), timerdao.WithFuzzyName(req.FuzzyName))
	if err != nil {
		return nil, -1, err
	}

	sort.Slice(timers, func(i, j int) bool {
		return timers[i].ID > timers[j].ID
	})

	vTimers, err := vo.NewTimers(timers)
	return vTimers, total, err
}

type timerDAO interface {
	CreateTimer(ctx context.Context, timer *po.Timer) (uint, error)
	DeleteTimer(ctx context.Context, id uint) error
	UpdateTimer(ctx context.Context, timer *po.Timer) error
	GetTimer(ctx context.Context, opts ...timerdao.Option) (*po.Timer, error)
	BatchCreateRecords(ctx context.Context, tasks []*po.Task) error
	DoWithLock(ctx context.Context, id uint, do func(ctx context.Context, dao *timerdao.TimerDAO, timer *po.Timer) error) error
	GetTimers(ctx context.Context, opts ...timerdao.Option) ([]*po.Timer, error)
	Count(ctx context.Context, opts ...timerdao.Option) (int64, error)
}

type confProvider interface {
	Get() *config.WebServerAppConf
}

type taskCache interface {
	BatchCreateTasks(ctx context.Context, tasks []*po.Task, start, end time.Time) error
}

type cronParser interface {
	NextsBefore(cron string, end time.Time) ([]time.Time, error)
	IsValidCronExpr(cron string) bool
}
