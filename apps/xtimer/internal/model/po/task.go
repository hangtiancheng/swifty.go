package po

import (
	"time"

	"gorm.io/gorm"
)

// Task is the run-history record of a timer execution.
type Task struct {
	gorm.Model
	App      string    `gorm:"column:app;NOT NULL"`           // app the timer belongs to
	TimerID  uint      `gorm:"column:timer_id;NOT NULL"`      // timer definition ID
	Output   string    `gorm:"column:output;default:null"`    // execution result
	RunTimer time.Time `gorm:"column:run_timer;default:null"` // execution time
	CostTime int       `gorm:"column:cost_time"`              // execution cost in milliseconds
	Status   int       `gorm:"column:status;NOT NULL"`        // current status
}

func (t *Task) TableName() string {
	return "task"
}

type MinuteTaskCnt struct {
	Minute string `gorm:"column:minute"`
	Cnt    int64  `gorm:"column:cnt"`
}
