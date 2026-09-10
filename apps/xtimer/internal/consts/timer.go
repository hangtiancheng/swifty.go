package consts

const (
	MinuteFormat = "2006-01-02 15:04"
	SecondFormat = "2006-01-02 15:04:00"
	HourFormat   = "2006-01-02 15"
	DayFormat    = "2006-01-02"
	// Bloom filter keys expire after one day by default.
	BloomFilterKeyExpireSeconds = 24 * 60 * 60
)

type TaskStatus int

func (t TaskStatus) ToInt() int {
	return int(t)
}

type TimerStatus int

func (t TimerStatus) ToInt() int {
	return int(t)
}

const (
	NotRun TaskStatus = 0
	// Running means the task has been picked up by an executor but has not finished yet.
	Running TaskStatus = 1
	// Succeeded means the task finished successfully.
	Succeeded TaskStatus = 2
	// Failed means the task finished with an error.
	Failed TaskStatus = 3

	Disabled TimerStatus = 1
	Enabled  TimerStatus = 2
)
