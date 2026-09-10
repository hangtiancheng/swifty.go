package vo

import (
	"encoding/json"
	"errors"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/consts"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/po"
)

type GetAppTimersReq struct {
	PageLimiter
	App string `json:"app" query:"app"`
}

type GetTimersByNameReq struct {
	PageLimiter
	App       string `json:"app" query:"app"`
	FuzzyName string `json:"fuzzyName" query:"fuzzyName"`
}

type GetTimersResp struct {
	CodeMsg
	Data  []*Timer `json:"data"`
	Total int64    `json:"total"`
}

func NewGetTimersResp(timers []*Timer, total int64, codeMsg CodeMsg) *GetTimersResp {
	return &GetTimersResp{
		Data:    timers,
		Total:   total,
		CodeMsg: codeMsg,
	}
}

type CreateTimerResp struct {
	CodeMsg
	ID uint `json:"id"`
}

func NewCreateTimerResp(id uint, codeMsg CodeMsg) *CreateTimerResp {
	return &CreateTimerResp{
		ID:      id,
		CodeMsg: codeMsg,
	}
}

type TimerReq struct {
	App string `json:"app" query:"app"`
	ID  uint   `json:"id" query:"id"`
}

type GetTimerResp struct {
	CodeMsg
	Data *Timer `json:"data"`
}

func NewGetTimerResp(timer *Timer, codeMsg CodeMsg) *GetTimerResp {
	return &GetTimerResp{
		CodeMsg: codeMsg,
		Data:    timer,
	}
}

type Timer struct {
	ID              uint               `json:"id,omitempty"`
	App             string             `json:"app,omitempty"`             // app the timer belongs to
	Name            string             `json:"name,omitempty"`            // timer definition name
	Status          consts.TimerStatus `json:"status"`                    // timer definition status, 1: disabled, 2: enabled
	Cron            string             `json:"cron,omitempty"`            // timer cron configuration
	NotifyHTTPParam *NotifyHTTPParam   `json:"notifyHTTPParam,omitempty"` // HTTP callback parameters
}

type NotifyHTTPParam struct {
	Method string            `json:"method,omitempty"` // POST, GET method
	URL    string            `json:"url,omitempty"`    // URL path
	Header map[string]string `json:"header,omitempty"` // request headers
	Body   string            `json:"body,omitempty"`   // request body
}

func NewTimer(timer *po.Timer) (*Timer, error) {
	var param NotifyHTTPParam
	if err := json.Unmarshal([]byte(timer.NotifyHTTPParam), &param); err != nil {
		return nil, err
	}

	return &Timer{
		ID:              timer.ID,
		App:             timer.App,
		Name:            timer.Name,
		Status:          consts.TimerStatus(timer.Status),
		Cron:            timer.Cron,
		NotifyHTTPParam: &param,
	}, nil
}

func NewTimers(timers []*po.Timer) ([]*Timer, error) {
	vTimers := make([]*Timer, 0, len(timers))
	for _, timer := range timers {
		vTimer, err := NewTimer(timer)
		if err != nil {
			return nil, err
		}
		vTimers = append(vTimers, vTimer)
	}
	return vTimers, nil
}

func (t *Timer) Check() error {
	if t.NotifyHTTPParam == nil {
		return errors.New("empty notify http params")
	}
	return nil
}

func (t *Timer) ToPO() (*po.Timer, error) {
	if err := t.Check(); err != nil {
		return nil, err
	}

	param, err := json.Marshal(t.NotifyHTTPParam)
	if err != nil {
		return nil, err
	}

	timer := po.Timer{
		App:             t.App,
		Name:            t.Name,
		Status:          t.Status.ToInt(),
		Cron:            t.Cron,
		NotifyHTTPParam: string(param),
	}
	if timer.Status == 0 {
		timer.Status = consts.Disabled.ToInt()
	}
	return &timer, nil
}

type MinuteBucket struct {
	Minute string
	Bucket int
}
