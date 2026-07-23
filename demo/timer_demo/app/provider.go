package app

import (
	"go.uber.org/dig"

	"github.com/hangtiancheng/swifty.go/demo/timer_demo/app/migrator"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/app/monitor"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/app/scheduler"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/app/webserver"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/common/conf"
	taskdao "github.com/hangtiancheng/swifty.go/demo/timer_demo/dao/task"
	timerdao "github.com/hangtiancheng/swifty.go/demo/timer_demo/dao/timer"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/bloom"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/cron"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/hash"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/mysql"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/promethus"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/redis"
	"github.com/hangtiancheng/swifty.go/demo/timer_demo/pkg/xhttp"
	executorservice "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/executor"
	migratorservice "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/migrator"
	monitorservice "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/monitor"
	schedulerservice "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/scheduler"
	triggerservice "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/trigger"
	webservice "github.com/hangtiancheng/swifty.go/demo/timer_demo/service/webserver"
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
	c.Provide(conf.DefaultMysqlConfProvider)
	c.Provide(conf.DefaultSchedulerAppConfProvider)
	c.Provide(conf.DefaultTriggerAppConfProvider)
	c.Provide(conf.DefaultWebServerAppConfProvider)
	c.Provide(conf.DefaultRedisConfigProvider)
	c.Provide(conf.DefaultMigratorAppConfProvider)
}

func providePKG(c *dig.Container) {
	c.Provide(bloom.NewFilter)
	c.Provide(hash.NewMurmur3Encryptor)
	c.Provide(hash.NewSHA1Encryptor)
	c.Provide(redis.GetClient)
	c.Provide(mysql.GetClient)
	c.Provide(cron.NewCronParser)
	c.Provide(xhttp.NewJSONClient)
	c.Provide(promethus.GetReporter)
}

func provideDAO(c *dig.Container) {
	c.Provide(timerdao.NewTimerDAO)
	c.Provide(taskdao.NewTaskDAO)
	c.Provide(taskdao.NewTaskCache)
}

func provideService(c *dig.Container) {
	c.Provide(migratorservice.NewWorker)
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
	if err := container.Invoke(func(_s *scheduler.WorkerApp) {
		schedulerApp = _s
	}); err != nil {
		panic(err)
	}
	return schedulerApp
}

func GetWebServer() *webserver.Server {
	var server *webserver.Server
	if err := container.Invoke(func(_s *webserver.Server) {
		server = _s
	}); err != nil {
		panic(err)
	}
	return server
}

func GetMigratorApp() *migrator.MigratorApp {
	var migratorApp *migrator.MigratorApp
	if err := container.Invoke(func(_m *migrator.MigratorApp) {
		migratorApp = _m
	}); err != nil {
		panic(err)
	}
	return migratorApp
}

func GetMonitorApp() *monitor.MonitorApp {
	var monitorApp *monitor.MonitorApp
	if err := container.Invoke(func(_m *monitor.MonitorApp) {
		monitorApp = _m
	}); err != nil {
		panic(err)
	}
	return monitorApp
}
