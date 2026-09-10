package webserver

import (
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v3"
	fiberadaptor "github.com/gofiber/fiber/v3/middleware/adaptor"
	fiberlogger "github.com/gofiber/fiber/v3/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
)

type Server struct {
	sync.Once
	app *fiber.App

	timerApp *TimerApp
	taskApp  *TaskApp

	timerRouter fiber.Router
	taskRouter  fiber.Router
	mockRouter  fiber.Router

	confProvider *config.WebServerAppConfProvider
}

func NewServer(timer *TimerApp, task *TaskApp, confProvider *config.WebServerAppConfProvider) *Server {
	s := Server{
		app:          fiber.New(),
		timerApp:     timer,
		taskApp:      task,
		confProvider: confProvider,
	}

	s.app.Use(fiberrecover.New())
	s.app.Use(corsMiddleware())
	s.app.Use(fiberlogger.New())

	s.timerRouter = s.app.Group("/api/timer/v1")
	s.taskRouter = s.app.Group("/api/task/v1")
	s.mockRouter = s.app.Group("/api/mock/v1")
	s.RegisterMockRouter()
	s.RegisterTimerRouter()
	s.RegisterTaskRouter()
	s.RegisterMonitorRouter()
	return &s
}

func (s *Server) Start() {
	s.Do(s.start)
}

func (s *Server) start() {
	conf := s.confProvider.Get()
	go func() {
		if err := s.app.Listen(fmt.Sprintf(":%d", conf.Port)); err != nil {
			panic(err)
		}
	}()
}

func (s *Server) RegisterTimerRouter() {
	s.timerRouter.Get("/def", s.timerApp.GetTimer)
	s.timerRouter.Post("/def", s.timerApp.CreateTimer)
	s.timerRouter.Delete("/def", s.timerApp.DeleteTimer)
	s.timerRouter.Patch("/def", s.timerApp.UpdateTimer)

	s.timerRouter.Get("/defs", s.timerApp.GetAppTimers)
	s.timerRouter.Get("/defsByName", s.timerApp.GetTimersByName)

	s.timerRouter.Post("/enable", s.timerApp.EnableTimer)
	s.timerRouter.Post("/unable", s.timerApp.UnableTimer)
}

func (s *Server) RegisterTaskRouter() {
	s.taskRouter.Get("/records", s.taskApp.GetTasks)
}

func (s *Server) RegisterMockRouter() {
	s.mockRouter.All("/mock", func(c fiber.Ctx) error {
		return c.Status(200).JSON(struct {
			Word string `json:"word"`
		}{
			Word: "hello world!",
		})
	})
}

// RegisterMonitorRouter exposes the prometheus metrics endpoint.
func (s *Server) RegisterMonitorRouter() {
	s.app.All("/metrics", fiberadaptor.HTTPHandler(promhttp.Handler()))
}
