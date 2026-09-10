package webserver

import (
	"context"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/consts"
	taskdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/task"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
)

type TaskService struct {
	dao *taskdao.TaskDAO
}

func NewTaskService(dao *taskdao.TaskDAO) *TaskService {
	return &TaskService{dao: dao}
}

func (t *TaskService) GetTask(ctx context.Context, id uint) (*vo.Task, error) {
	task, err := t.dao.GetTask(ctx, taskdao.WithTaskID(id))
	if err != nil {
		return nil, err
	}
	return vo.NewTask(task), nil
}

func (t *TaskService) GetTasks(ctx context.Context, req *vo.GetTasksReq) ([]*vo.Task, int64, error) {
	statuses := []int32{
		int32(consts.Running),
		int32(consts.Succeeded),
		int32(consts.Failed),
	}

	total, err := t.dao.Count(ctx, taskdao.WithTimerID(req.TimerID), taskdao.WithStatuses(statuses))
	if err != nil {
		return nil, -1, err
	}

	offset, limit := req.Get()
	if total <= int64(offset) {
		return []*vo.Task{}, total, nil
	}
	tasks, err := t.dao.GetTasks(ctx, taskdao.WithTimerID(req.TimerID), taskdao.WithPageLimit(offset, limit), taskdao.WithStatuses(statuses), taskdao.WithDesc())
	if err != nil {
		return nil, -1, err
	}

	return vo.NewTasks(tasks), total, nil
}
