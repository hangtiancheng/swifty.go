package vo

import "fmt"

const (
	successCode = 0
)

type Errorer interface {
	Error() error
}

func NewCodeMsg(code int32, msg string) CodeMsg {
	cm := CodeMsg{
		Code: code,
		Msg:  msg,
	}
	cm.Err = cm.Error()
	return cm
}

func NewCodeMsgWithErr(err error) CodeMsg {
	return CodeMsg{Err: err}
}

type CodeMsg struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
	Err  error  `json:"err"`
}

func (c *CodeMsg) Error() error {
	if c.Code == successCode {
		return nil
	}
	return fmt.Errorf("code: %d, msg: %s", c.Code, c.Msg)
}

// PageLimiter is the common pagination parameters.
type PageLimiter struct {
	Index int `json:"pageIndex" query:"pageIndex"`
	Size  int `json:"pageSize" query:"pageSize"`
}

func (p *PageLimiter) Get() (offset, limit int) {
	if p.Index <= 0 {
		p.Index = 1
	}

	if p.Size <= 0 {
		p.Size = 10
	}

	return (p.Index - 1) * p.Size, p.Size
}
