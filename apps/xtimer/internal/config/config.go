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
	validate()
	defaultMigratorAppConfProvider = NewMigratorAppConfProvider(gConf.Migrator)
	defaultMysqlConfProvider = NewMysqlConfProvider(gConf.Mysql)
	defaultRedisConfProvider = NewRedisConfigProvider(gConf.Redis)
	defaultTriggerAppConfProvider = NewTriggerAppConfProvider(gConf.Trigger)
	defaultSchedulerAppConfProvider = NewSchedulerAppConfProvider(gConf.Scheduler)
	defaultWebServerAppConfProvider = NewWebServerAppConfProvider(gConf.WebServer)
}

// load reads conf.json from the current working directory and merges it on top
// of the fallback defaults below, field by field. Values that are absent from
// the file keep their default values. A missing file is not fatal: the
// built-in defaults are used and the app fails later with a clear message if a
// required address (e.g. the Redis address) is still missing.
func load() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	raw, err := os.ReadFile(filepath.Join(wd, configFileName))
	if err != nil {
		fmt.Printf("config: %v, falling back to built-in defaults\n", err)
		return
	}

	if err := merge(raw); err != nil {
		panic(fmt.Sprintf("failed to unmarshal %s: %v", configFileName, err))
	}
}

// merge applies conf.json on top of the built-in defaults. Each section is
// decoded into the pre-filled default struct, so a section present in the file
// only overrides the fields it explicitly sets.
func merge(raw []byte) error {
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sections); err != nil {
		return err
	}

	targets := map[string]any{
		"migrator":  gConf.Migrator,
		"mysql":     gConf.Mysql,
		"redis":     gConf.Redis,
		"trigger":   gConf.Trigger,
		"scheduler": gConf.Scheduler,
		"webServer": gConf.WebServer,
	}
	for name, section := range sections {
		target, ok := targets[name]
		if !ok || string(section) == "null" {
			continue
		}
		if err := json.Unmarshal(section, target); err != nil {
			return fmt.Errorf("section %q: %w", name, err)
		}
	}
	return nil
}

// validate fails fast on settings that would break arithmetic done on them
// (bucket sharding, tickers) with a cryptic error later at runtime.
func validate() {
	if gConf.Scheduler.BucketsNum <= 0 {
		panic(fmt.Sprintf("scheduler.bucketsNum must be positive, got %d", gConf.Scheduler.BucketsNum))
	}
	if gConf.Scheduler.TryLockGapMilliSeconds <= 0 {
		panic(fmt.Sprintf("scheduler.tryLockGapMilliSeconds must be positive, got %d", gConf.Scheduler.TryLockGapMilliSeconds))
	}
	if gConf.Trigger.ZRangeGapSeconds <= 0 {
		panic(fmt.Sprintf("trigger.zrangeGapSeconds must be positive, got %d", gConf.Trigger.ZRangeGapSeconds))
	}
	if gConf.Migrator.MigrateStepMinutes <= 0 {
		panic(fmt.Sprintf("migrator.migrateStepMinutes must be positive, got %d", gConf.Migrator.MigrateStepMinutes))
	}
	if gConf.Migrator.TimerDetailCacheMinutes <= 0 {
		panic(fmt.Sprintf("migrator.timerDetailCacheMinutes must be positive, got %d", gConf.Migrator.TimerDetailCacheMinutes))
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
