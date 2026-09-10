package executor

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/bloom"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/consts"
	taskdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/task"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/prometheus"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/utils"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/xhttp"
)

type Worker struct {
	timerService *TimerService
	taskDAO      *taskdao.TaskDAO
	httpClient   *xhttp.JSONClient
	bloomFilter  *bloom.Filter
	reporter     *prometheus.Reporter
}

func NewWorker(timerService *TimerService, taskDAO *taskdao.TaskDAO, httpClient *xhttp.JSONClient, bloomFilter *bloom.Filter, reporter *prometheus.Reporter) *Worker {
	return &Worker{
		timerService: timerService,
		taskDAO:      taskDAO,
		httpClient:   httpClient,
		bloomFilter:  bloomFilter,
		reporter:     reporter,
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.timerService.Start(ctx)
}

func (w *Worker) Work(ctx context.Context, timerIDUnixKey string) error {
	// Parse the message and load the full timer definition.
	timerID, unix, err := utils.SplitTimerIDUnix(timerIDUnixKey)
	if err != nil {
		return err
	}

	bloomKey := utils.GetTaskBloomFilterKey(utils.GetDayStr(time.UnixMilli(unix)))
	if exist, err := w.bloomFilter.Exist(ctx, bloomKey, timerIDUnixKey); err != nil || exist {
		log.WarnContextf(ctx, "bloom filter check failed, start to check db, bloom key: %s, timerIDUnixKey: %s, err: %v, exist: %t",
			bloomKey, timerIDUnixKey, err, exist)
		// Check the task status in the database.
		task, err := w.taskDAO.GetTask(ctx, taskdao.WithTimerID(timerID), taskdao.WithRunTimer(time.UnixMilli(unix)))
		if err == nil && task.Status != consts.NotRun.ToInt() {
			// The task has already been executed.
			log.WarnContextf(ctx, "task is already executed, timerID: %d, exec_time: %v", timerID, task.RunTimer)
			return nil
		}
	}

	return w.executeAndPostProcess(ctx, timerID, unix)
}

func (w *Worker) executeAndPostProcess(ctx context.Context, timerID uint, unix int64) error {
	// The task has not run yet, load the full timer definition and run the callback.
	timer, err := w.timerService.GetTimer(ctx, timerID)
	if err != nil {
		return fmt.Errorf("get timer failed, id: %d, err: %w", timerID, err)
	}

	// A disabled timer does not need its tasks processed.
	if timer.Status != consts.Enabled {
		log.WarnContextf(ctx, "timer has already been disabled, timerID: %d", timerID)
		return nil
	}

	execTime := time.Now()
	resp, err := w.execute(ctx, timer)
	return w.postProcess(ctx, resp, err, timer.App, timerID, unix, execTime)
}

func (w *Worker) execute(ctx context.Context, timer *vo.Timer) (map[string]interface{}, error) {
	var (
		resp map[string]interface{}
		err  error
	)
	switch strings.ToUpper(timer.NotifyHTTPParam.Method) {
	case nethttp.MethodGet:
		err = w.httpClient.Get(ctx, timer.NotifyHTTPParam.URL, timer.NotifyHTTPParam.Header, nil, &resp)
	case nethttp.MethodPatch:
		err = w.httpClient.Patch(ctx, timer.NotifyHTTPParam.URL, timer.NotifyHTTPParam.Header, timer.NotifyHTTPParam.Body, &resp)
	case nethttp.MethodDelete:
		err = w.httpClient.Delete(ctx, timer.NotifyHTTPParam.URL, timer.NotifyHTTPParam.Header, timer.NotifyHTTPParam.Body, &resp)
	case nethttp.MethodPost:
		err = w.httpClient.Post(ctx, timer.NotifyHTTPParam.URL, timer.NotifyHTTPParam.Header, timer.NotifyHTTPParam.Body, &resp)
	default:
		err = fmt.Errorf("invalid http method: %s, timer: %s", timer.NotifyHTTPParam.Method, timer.Name)
	}

	return resp, err
}

func (w *Worker) postProcess(ctx context.Context, resp map[string]interface{}, execErr error, app string, timerID uint, unix int64, execTime time.Time) error {
	go w.reportMonitorData(app, unix, execTime)
	bloomKey := utils.GetTaskBloomFilterKey(utils.GetDayStr(time.UnixMilli(unix)))
	if err := w.bloomFilter.Set(ctx, bloomKey, utils.UnionTimerIDUnix(timerID, unix), consts.BloomFilterKeyExpireSeconds); err != nil {
		log.ErrorContextf(ctx, "set bloom filter failed, key: %s, err: %v", bloomKey, err)
	}

	task, err := w.taskDAO.GetTask(ctx, taskdao.WithTimerID(timerID), taskdao.WithRunTimer(time.UnixMilli(unix)))
	if err != nil {
		return fmt.Errorf("get task failed, timerID: %d, runTimer: %v, err: %w", timerID, time.UnixMilli(unix), err)
	}

	respBody, _ := json.Marshal(resp)
	task.Output = string(respBody)

	if execErr != nil {
		task.Status = consts.Failed.ToInt()
	} else {
		task.Status = consts.Succeeded.ToInt()
	}

	return w.taskDAO.UpdateTask(ctx, task)
}

func (w *Worker) reportMonitorData(app string, expectExecTimeUnix int64, actualExecTime time.Time) {
	w.reporter.ReportExecRecord(app)
	// Report the delay in milliseconds.
	w.reporter.ReportTimerDelayRecord(app, float64(actualExecTime.UnixMilli()-expectExecTimeUnix))
}
