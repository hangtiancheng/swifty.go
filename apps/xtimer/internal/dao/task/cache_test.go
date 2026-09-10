package task

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/po"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
)

func newTestCache(t *testing.T) (*TaskCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.GetClient(config.NewRedisConfigProvider(&config.RedisConfig{Address: mr.Addr()}))
	conf := config.NewSchedulerAppConfProvider(&config.SchedulerAppConf{BucketsNum: 10})
	return NewTaskCache(client, conf), mr
}

// nextMinute returns a minute in the future, matching the production flow
// where only upcoming tasks are cached (past tasks expire immediately).
func nextMinute() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()+1, 0, 0, now.Location())
}

func testTasks(minute time.Time) []*po.Task {
	tasks := make([]*po.Task, 0, 3)
	for _, id := range []uint{1, 12, 23} {
		tasks = append(tasks, &po.Task{
			TimerID:  id,
			Status:   0,
			RunTimer: minute.Add(30 * time.Second),
		})
	}
	return tasks
}

func TestGetTableName(t *testing.T) {
	cache, _ := newTestCache(t)

	task := &po.Task{
		TimerID:  123, // 123 % 10 = 3
		RunTimer: time.Date(2026, 9, 1, 10, 0, 30, 0, time.Local),
	}
	if got, want := cache.GetTableName(task), "2026-09-01 10:00_3"; got != want {
		t.Fatalf("GetTableName = %q, want %q", got, want)
	}
}

func TestBatchCreateTasksAndGetTasksByTime(t *testing.T) {
	cache, mr := newTestCache(t)
	ctx := context.Background()

	minute := nextMinute()
	tasks := testTasks(minute)

	if err := cache.BatchCreateTasks(ctx, tasks, minute, minute.Add(time.Minute)); err != nil {
		t.Fatalf("BatchCreateTasks: %v", err)
	}

	// Every task lands in the zset of its own bucket and can be read back.
	for _, task := range tasks {
		table := cache.GetTableName(task)
		if !mr.Exists(table) {
			t.Fatalf("zset %q not created", table)
		}
		if ttl := mr.TTL(table); ttl <= 0 {
			t.Fatalf("zset %q has no expiry", table)
		}

		got, err := cache.GetTasksByTime(ctx, table, minute.UnixMilli(), minute.Add(time.Minute).UnixMilli())
		if err != nil {
			t.Fatalf("GetTasksByTime: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d tasks, want 1", len(got))
		}
		if got[0].TimerID != task.TimerID || !got[0].RunTimer.Equal(task.RunTimer) {
			t.Fatalf("got timerID=%d runTimer=%v, want timerID=%d runTimer=%v",
				got[0].TimerID, got[0].RunTimer, task.TimerID, task.RunTimer)
		}
	}
}

func TestGetTasksByTimeExcludesEnd(t *testing.T) {
	cache, _ := newTestCache(t)
	ctx := context.Background()

	minute := nextMinute()
	if err := cache.BatchCreateTasks(ctx, testTasks(minute), minute, minute.Add(time.Minute)); err != nil {
		t.Fatalf("BatchCreateTasks: %v", err)
	}

	// The task runs at minute+30s; querying with end == run time must exclude it.
	runTimer := minute.Add(30 * time.Second)
	table := cache.GetTableName(&po.Task{TimerID: 1, RunTimer: runTimer})
	got, err := cache.GetTasksByTime(ctx, table, minute.UnixMilli(), runTimer.UnixMilli())
	if err != nil {
		t.Fatalf("GetTasksByTime: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("tasks at the end boundary must be excluded, got %v", got)
	}

	// Moving the end boundary one millisecond forward includes it again.
	got, err = cache.GetTasksByTime(ctx, table, minute.UnixMilli(), runTimer.Add(time.Millisecond).UnixMilli())
	if err != nil {
		t.Fatalf("GetTasksByTime: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d tasks, want 1", len(got))
	}
}
