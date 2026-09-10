package app

import (
	"go.uber.org/dig"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/app/migrator"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/app/monitor"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/app/scheduler"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/app/webserver"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/bloom"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/cron"
	taskdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/task"
	timerdao "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/dao/timer"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/hash"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/mysql"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/prometheus"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
	executorservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/executor"
	migratorservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/migrator"
	monitorservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/monitor"
	schedulerservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/scheduler"
	triggerservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/trigger"
	webservice "github.com/hangtiancheng/swifty.go/apps/xtimer/internal/service/webserver"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/xhttp"
)

var (
	container *dig.Container
)

func init() {
	container = dig.New()

	provideConfig(container)
	providePKG(container)
	provideDAO(container)
	provideService(container)
	provideApp(container)
}

func provideConfig(c *dig.Container) {
	c.Provide(config.DefaultMysqlConfProvider)
	c.Provide(config.DefaultSchedulerAppConfProvider)
	c.Provide(config.DefaultTriggerAppConfProvider)
	c.Provide(config.DefaultWebServerAppConfProvider)
	c.Provide(config.DefaultRedisConfigProvider)
	c.Provide(config.DefaultMigratorAppConfProvider)
}

func providePKG(c *dig.Container) {
	c.Provide(bloom.NewFilter)
	c.Provide(hash.NewMurmur3Encryptor)
	c.Provide(hash.NewSHA1Encryptor)
	c.Provide(redis.GetClient)
	c.Provide(mysql.GetClient)
	c.Provide(cron.NewCronParser)
	c.Provide(xhttp.NewJSONClient)
	c.Provide(prometheus.GetReporter)
}

func provideDAO(c *dig.Container) {
	c.Provide(timerdao.NewTimerDAO)
	c.Provide(taskdao.NewTaskDAO)
	c.Provide(taskdao.NewTaskCache)
}

func provideService(c *dig.Container) {
	c.Provide(migratorservice.NewWorker)
	c.Provide(webservice.NewTaskService)
	c.Provide(webservice.NewTimerService)
	c.Provide(executorservice.NewTimerService)
	c.Provide(executorservice.NewWorker)
	c.Provide(triggerservice.NewWorker)
	c.Provide(triggerservice.NewTaskService)
	c.Provide(schedulerservice.NewWorker)
	c.Provide(monitorservice.NewWorker)
}

func provideApp(c *dig.Container) {
	c.Provide(migrator.NewMigratorApp)
	c.Provide(webserver.NewTaskApp)
	c.Provide(webserver.NewTimerApp)
	c.Provide(webserver.NewServer)
	c.Provide(scheduler.NewWorkerApp)
	c.Provide(monitor.NewMonitorApp)
}

func GetSchedulerApp() *scheduler.WorkerApp {
	var schedulerApp *scheduler.WorkerApp
	if err := container.Invoke(func(s *scheduler.WorkerApp) {
		schedulerApp = s
	}); err != nil {
		panic(err)
	}
	return schedulerApp
}

func GetWebServer() *webserver.Server {
	var server *webserver.Server
	if err := container.Invoke(func(s *webserver.Server) {
		server = s
	}); err != nil {
		panic(err)
	}
	return server
}

func GetMigratorApp() *migrator.MigratorApp {
	var migratorApp *migrator.MigratorApp
	if err := container.Invoke(func(m *migrator.MigratorApp) {
		migratorApp = m
	}); err != nil {
		panic(err)
	}
	return migratorApp
}

func GetMonitorApp() *monitor.MonitorApp {
	var monitorApp *monitor.MonitorApp
	if err := container.Invoke(func(m *monitor.MonitorApp) {
		monitorApp = m
	}); err != nil {
		panic(err)
	}
	return monitorApp
}
