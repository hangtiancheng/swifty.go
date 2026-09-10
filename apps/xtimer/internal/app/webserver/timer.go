package webserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
	webservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/webserver"
)

type TimerApp struct {
	service timerService
}

func NewTimerApp(service *webservice.TimerService) *TimerApp {
	return &TimerApp{service: service}
}

// CreateTimer creates a timer definition.
func (t *TimerApp) CreateTimer(c fiber.Ctx) error {
	var req vo.Timer
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[create timer] bind req failed, err: %v", err)))
	}

	id, err := t.service.CreateTimer(c.RequestCtx(), &req)
	if err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewCreateTimerResp(id, vo.NewCodeMsgWithErr(nil)))
}

// GetAppTimers returns all timers under an app.
func (t *TimerApp) GetAppTimers(c fiber.Ctx) error {
	var req vo.GetAppTimersReq
	if err := c.Bind().Query(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[get app timers] bind req failed, err: %v", err)))
	}

	timers, total, err := t.service.GetAppTimers(c.RequestCtx(), &req)
	if err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewGetTimersResp(timers, total, vo.NewCodeMsgWithErr(nil)))
}

func (t *TimerApp) GetTimersByName(c fiber.Ctx) error {
	var req vo.GetTimersByNameReq
	if err := c.Bind().Query(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[get timers by name] bind req failed, err: %v", err)))
	}

	timers, total, err := t.service.GetTimersByName(c.RequestCtx(), &req)
	if err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewGetTimersResp(timers, total, vo.NewCodeMsgWithErr(nil)))
}

func (t *TimerApp) DeleteTimer(c fiber.Ctx) error {
	var req vo.TimerReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[delete timer] bind req failed, err: %v", err)))
	}

	if err := t.service.DeleteTimer(c.RequestCtx(), req.App, req.ID); err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewCodeMsgWithErr(nil))
}

func (t *TimerApp) UpdateTimer(c fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(nil)
}

func (t *TimerApp) GetTimer(c fiber.Ctx) error {
	var req vo.TimerReq
	if err := c.Bind().Query(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[get timer] bind req failed, err: %v", err)))
	}

	timer, err := t.service.GetTimer(c.RequestCtx(), req.ID)
	if err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewGetTimerResp(timer, vo.NewCodeMsgWithErr(nil)))
}

// EnableTimer activates a timer definition.
func (t *TimerApp) EnableTimer(c fiber.Ctx) error {
	var req vo.TimerReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[enable timer] bind req failed, err: %v", err)))
	}

	if err := t.service.EnableTimer(c.RequestCtx(), req.App, req.ID); err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewCodeMsgWithErr(nil))
}

// UnableTimer deactivates a timer definition.
func (t *TimerApp) UnableTimer(c fiber.Ctx) error {
	var req vo.TimerReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[unable timer] bind req failed, err: %v", err)))
	}

	if err := t.service.UnableTimer(c.RequestCtx(), req.App, req.ID); err != nil {
		return c.Status(http.StatusOK).JSON(vo.NewCodeMsg(-1, err.Error()))
	}
	return c.Status(http.StatusOK).JSON(vo.NewCodeMsgWithErr(nil))
}

type timerService interface {
	CreateTimer(ctx context.Context, timer *vo.Timer) (uint, error)
	DeleteTimer(ctx context.Context, app string, id uint) error
	UpdateTimer(ctx context.Context, timer *vo.Timer) error
	GetTimer(ctx context.Context, id uint) (*vo.Timer, error)
	EnableTimer(ctx context.Context, app string, id uint) error
	UnableTimer(ctx context.Context, app string, id uint) error
	GetAppTimers(ctx context.Context, req *vo.GetAppTimersReq) ([]*vo.Timer, int64, error)
	GetTimersByName(ctx context.Context, req *vo.GetTimersByNameReq) ([]*vo.Timer, int64, error)
}
