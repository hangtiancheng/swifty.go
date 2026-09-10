package config

type SchedulerAppConf struct {
	SchedulersNum int `json:"schedulersNum"`
	WorkersNum    int `json:"workersNum"`
	// One extra bucket for every additional 200 tasks on top of the default bucket count.
	BucketsNum             int `json:"bucketsNum"`
	TryLockSeconds         int `json:"tryLockSeconds"`
	TryLockGapMilliSeconds int `json:"tryLockGapMilliSeconds"`
	SuccessExpireSeconds   int `json:"successExpireSeconds"`
}

var defaultSchedulerAppConfProvider *SchedulerAppConfProvider

type SchedulerAppConfProvider struct {
	conf *SchedulerAppConf
}

func NewSchedulerAppConfProvider(conf *SchedulerAppConf) *SchedulerAppConfProvider {
	return &SchedulerAppConfProvider{conf: conf}
}

func (s *SchedulerAppConfProvider) Get() *SchedulerAppConf {
	return s.conf
}

func DefaultSchedulerAppConfProvider() *SchedulerAppConfProvider {
	return defaultSchedulerAppConfProvider
}
