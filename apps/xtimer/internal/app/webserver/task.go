package webserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
	webservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/webserver"
)

type TaskApp struct {
	service taskService
}

func NewTaskApp(service *webservice.TaskService) *TaskApp {
	return &TaskApp{service: service}
}

func (t *TaskApp) GetTasks(c fiber.Ctx) error {
	var req vo.GetTasksReq
	if err := c.Bind().Query(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(vo.NewCodeMsg(-1, fmt.Sprintf("[get tasks] bind req failed, err: %v", err)))
	}

	tasks, total, err := t.service.GetTasks(c.RequestCtx(), &req)
	return c.Status(http.StatusOK).JSON(vo.NewGetTasksResp(tasks, total, vo.NewCodeMsgWithErr(err)))
}

type taskService interface {
	GetTask(ctx context.Context, id uint) (*vo.Task, error)
	GetTasks(ctx context.Context, req *vo.GetTasksReq) ([]*vo.Task, int64, error)
}
