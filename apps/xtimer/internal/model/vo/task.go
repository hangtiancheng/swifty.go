package vo

import (
	"time"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/po"
)

type GetTasksReq struct {
	PageLimiter
	TimerID uint `json:"timerID" query:"timerID"`
}

type GetTasksResp struct {
	CodeMsg
	Total int64   `json:"total"`
	Data  []*Task `json:"data"`
}

func NewGetTasksResp(tasks []*Task, total int64, codeMsg CodeMsg) *GetTasksResp {
	return &GetTasksResp{
		CodeMsg: codeMsg,
		Total:   total,
		Data:    tasks,
	}
}

// Task is the run-history record of a timer execution.
type Task struct {
	ID       uint      `json:"id"`       // task ID
	App      string    `json:"app"`      // app the task belongs to
	TimerID  uint      `json:"timerID"`  // timer definition ID
	Output   string    `json:"output"`   // execution result
	RunTimer time.Time `json:"runTimer"` // execution time
	CostTime int       `json:"costTime"` // execution cost
	Status   int       `json:"status"`   // current status
}

func NewTask(task *po.Task) *Task {
	return &Task{
		ID:       task.ID,
		App:      task.App,
		TimerID:  task.TimerID,
		Output:   task.Output,
		RunTimer: task.RunTimer,
		CostTime: task.CostTime,
		Status:   task.Status,
	}
}

func NewTasks(tasks []*po.Task) []*Task {
	vTasks := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		vTasks = append(vTasks, NewTask(task))
	}
	return vTasks
}

func (t *Task) ToPO() *po.Task {
	return &po.Task{
		App:      t.App,
		TimerID:  t.TimerID,
		Output:   t.Output,
		RunTimer: t.RunTimer,
		CostTime: t.CostTime,
		Status:   t.Status,
	}
}
