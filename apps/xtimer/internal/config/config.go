package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configFileName = "conf.json"

func init() {
	load()
	defaultMigratorAppConfProvider = NewMigratorAppConfProvider(gConf.Migrator)
	defaultMysqlConfProvider = NewMysqlConfProvider(gConf.Mysql)
	defaultRedisConfProvider = NewRedisConfigProvider(gConf.Redis)
	defaultTriggerAppConfProvider = NewTriggerAppConfProvider(gConf.Trigger)
	defaultSchedulerAppConfProvider = NewSchedulerAppConfProvider(gConf.Scheduler)
	defaultWebServerAppConfProvider = NewWebServerAppConfProvider(gConf.WebServer)
}

// load reads conf.json from the current working directory and merges it on top
// of the fallback defaults below. Values that are absent from the file keep
// their default values.
func load() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	raw, err := os.ReadFile(filepath.Join(wd, configFileName))
	if err != nil {
		panic(fmt.Sprintf("failed to read %s from working directory %s: %v", configFileName, wd, err))
	}

	if err := json.Unmarshal(raw, &gConf); err != nil {
		panic(fmt.Sprintf("failed to unmarshal %s: %v", configFileName, err))
	}
}

// gConf holds the fallback configuration. It is pre-filled with sensible
// defaults and then overridden by whatever conf.json provides.
var gConf = GlobalConf{
	Migrator: &MigratorAppConf{
		// Number of concurrent goroutines on a single node.
		WorkersNum: 1000,
		// Time interval between two data migrations, in minutes.
		MigrateStepMinutes: 60,
		// Lock expiry time updated after a successful migration, in minutes.
		MigrateSuccessExpireMinutes: 120,
		// Initial expiry time of the lock acquired by the migrator, in minutes.
		MigrateTryLockMinutes: 20,
		// How long the migrator pre-caches timer details in memory, in minutes.
		TimerDetailCacheMinutes: 2,
	},

	Scheduler: &SchedulerAppConf{
		// Number of concurrent goroutines on a single node.
		WorkersNum: 100,
		// Number of buckets.
		BucketsNum: 10,
		// Initial expiry time of the distributed lock acquired by the scheduler, in seconds.
		TryLockSeconds: 70,
		// Interval between two attempts to acquire the distributed lock, in milliseconds.
		TryLockGapMilliSeconds: 100,
		// Distributed lock time updated after a time slice is executed successfully, in seconds.
		SuccessExpireSeconds: 130,
	},

	Trigger: &TriggerAppConf{
		// Interval between two polls of the timer task zset, in seconds.
		ZRangeGapSeconds: 1,
		// Number of concurrent goroutines.
		WorkersNum: 10000,
	},

	WebServer: &WebServerAppConf{
		Port: 8092,
	},
	Redis: &RedisConfig{
		Network: "tcp",
		// Maximum number of idle connections.
		MaxIdle: 2000,
		// Idle connection timeout, in seconds.
		IdleTimeoutSeconds: 30,
		// Maximum number of connections kept alive in the pool.
		MaxActive: 1000,
		// When the connection limit is reached, whether new requests wait or fail immediately.
		Wait: true,
	},
	Mysql: &MySQLConfig{
		MaxOpenConns: 100,
		MaxIdleConns: 50,
	},
}

type GlobalConf struct {
	Migrator  *MigratorAppConf  `json:"migrator"`
	Mysql     *MySQLConfig      `json:"mysql"`
	Redis     *RedisConfig      `json:"redis"`
	Trigger   *TriggerAppConf   `json:"trigger"`
	Scheduler *SchedulerAppConf `json:"scheduler"`
	WebServer *WebServerAppConf `json:"webServer"`
}
