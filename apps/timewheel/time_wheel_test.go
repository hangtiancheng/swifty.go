package timewheel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"

	thttp "github.com/hangtiancheng/swifty.go/apps/timewheel/internal/http"
	"github.com/hangtiancheng/swifty.go/apps/timewheel/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/timewheel/internal/timex"
)

// redisTestAddress is the address used by the live-redis integration test.
const redisTestAddress = "127.0.0.1:6379"

// skipWithoutRedis skips the test when no redis instance is reachable, so the
// test suite still passes on machines without a running redis.
func skipWithoutRedis(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", redisTestAddress, time.Second)
	if err != nil {
		t.Skipf("skipping: no redis reachable at %s: %v", redisTestAddress, err)
	}
	_ = conn.Close()
}

func Test_timeWheel(t *testing.T) {
	timeWheel := NewTimeWheel(10, 100*time.Millisecond)
	defer timeWheel.Stop()

	executed := make(chan string, 8)
	timeWheel.AddTask("test1", func() { executed <- "test1" }, time.Now().Add(300*time.Millisecond))
	// Re-adding a key replaces the pending task: only the 500ms one runs.
	timeWheel.AddTask("test2", func() { executed <- "test2-stale" }, time.Now().Add(700*time.Millisecond))
	timeWheel.AddTask("test2", func() { executed <- "test2" }, time.Now().Add(500*time.Millisecond))

	want := map[string]bool{"test1": false, "test2": false}
	deadline := time.After(5 * time.Second)
	for want["test1"] == false || want["test2"] == false {
		select {
		case key := <-executed:
			if _, ok := want[key]; !ok {
				t.Errorf("unexpected task executed: %s", key)
				continue
			}
			if want[key] {
				t.Errorf("task executed more than once: %s", key)
				continue
			}
			want[key] = true
		case <-deadline:
			t.Fatalf("timed out waiting for tasks, executed so far: %v", want)
		}
	}

	// No further tasks should fire (in particular not the stale "test2").
	select {
	case key := <-executed:
		t.Errorf("unexpected extra task executed: %s", key)
	case <-time.After(600 * time.Millisecond):
	}
}

func Test_timeWheelRemoveTask(t *testing.T) {
	timeWheel := NewTimeWheel(10, 100*time.Millisecond)
	defer timeWheel.Stop()

	executed := make(chan string, 4)
	timeWheel.AddTask("doomed", func() { executed <- "doomed" }, time.Now().Add(250*time.Millisecond))
	timeWheel.AddTask("kept", func() { executed <- "kept" }, time.Now().Add(250*time.Millisecond))
	timeWheel.RemoveTask("doomed")

	select {
	case key := <-executed:
		if key == "doomed" {
			t.Fatal("removed task was executed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("control task was not executed; wheel seems broken")
	}

	select {
	case key := <-executed:
		t.Errorf("unexpected task executed after removal: %s", key)
	case <-time.After(600 * time.Millisecond):
	}
}

func Test_timeWheelOverdueTask(t *testing.T) {
	// Regression test: a task with an execution time in the past used to
	// produce a negative slot index and crash the wheel goroutine. It must
	// be executed on the next tick instead.
	timeWheel := NewTimeWheel(10, 100*time.Millisecond)
	defer timeWheel.Stop()

	executed := make(chan struct{}, 1)
	timeWheel.AddTask("overdue", func() { executed <- struct{}{} }, time.Now().Add(-time.Hour))

	select {
	case <-executed:
	case <-time.After(2 * time.Second):
		t.Fatal("task with an overdue execution time was not executed")
	}
}

const (
	// Live redis server used by the integration test below.
	redisNetwork  = "tcp"
	redisAddress  = redisTestAddress
	redisPassword = ""
)

func Test_redis_timeWheel(t *testing.T) {
	skipWithoutRedis(t)

	// The tasks deliver an HTTP callback; capture the deliveries with a
	// local test server instead of relying on an external endpoint.
	var mu sync.Mutex
	delivered := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		var payload struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return
		}
		mu.Lock()
		delivered[payload.Key]++
		mu.Unlock()
	}))
	defer srv.Close()

	rTimeWheel := NewRTimeWheel(
		redis.NewClient(redisNetwork, redisAddress, redisPassword),
		thttp.NewClient(),
	)
	defer rTimeWheel.Stop()

	ctx := context.Background()
	now := time.Now()
	if err := rTimeWheel.AddTask(ctx, "test1", &RTaskElement{
		CallbackURL: srv.URL,
		Method:      http.MethodPost,
		Req:         map[string]string{"key": "test1"},
	}, now.Add(time.Second)); err != nil {
		t.Fatalf("add task test1: %v", err)
	}

	if err := rTimeWheel.AddTask(ctx, "test2", &RTaskElement{
		CallbackURL: srv.URL,
		Method:      http.MethodPost,
		Req:         map[string]string{"key": "test2"},
	}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("add task test2: %v", err)
	}

	if err := rTimeWheel.RemoveTask(ctx, "test2", now.Add(2*time.Second)); err != nil {
		t.Fatalf("remove task test2: %v", err)
	}

	// Wait until test1 has been delivered.
	deadline := time.Now().Add(20 * time.Second)
	for {
		mu.Lock()
		got1 := delivered["test1"]
		mu.Unlock()
		if got1 > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if delivered["test1"] == 0 {
		t.Fatalf("task test1 was not delivered within the deadline; delivered: %v", delivered)
	}
	if delivered["test2"] != 0 {
		t.Errorf("removed task test2 was delivered %d time(s)", delivered["test2"])
	}
}

func Test_timeWheelAddAfterStop(t *testing.T) {
	timeWheel := NewTimeWheel(10, 50*time.Millisecond)
	timeWheel.Stop()
	timeWheel.Stop() // must stay idempotent

	done := make(chan struct{})
	go func() {
		defer close(done)
		// The run goroutine is gone at this point; both calls must return
		// promptly instead of blocking forever on their channels.
		timeWheel.AddTask("late", func() {}, time.Now().Add(time.Hour))
		timeWheel.RemoveTask("late")
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AddTask/RemoveTask blocked forever after Stop (goroutine leak)")
	}
}

func Test_timeWheelPanickingTask(t *testing.T) {
	timeWheel := NewTimeWheel(4, 20*time.Millisecond)
	defer timeWheel.Stop()

	fired := make(chan struct{}, 1)
	timeWheel.AddTask("boom", func() { panic("boom") }, time.Now().Add(30*time.Millisecond))
	timeWheel.AddTask("ok", func() { fired <- struct{}{} }, time.Now().Add(120*time.Millisecond))

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("wheel stopped delivering tasks after a panicking task")
	}
}

func Test_timeWheelFullRotation(t *testing.T) {
	// Regression test for the cycle arithmetic: a task due after exactly one
	// full wheel rotation must fire on that rotation's boundary visit, not a
	// whole rotation later (and not on the very first tick).
	const (
		slotNum  = 5
		interval = 50 * time.Millisecond
	)
	timeWheel := NewTimeWheel(slotNum, interval)
	defer timeWheel.Stop()

	fired := make(chan time.Time, 1)
	start := time.Now()
	timeWheel.AddTask("rotation", func() { fired <- time.Now() }, start.Add(slotNum*interval))

	select {
	case at := <-fired:
		// The wheel always runs a task on the tick after its due time, so the
		// delay lands in (n, 2n) with n = slotNum*interval. A broken cycle
		// count fires one rotation early or late, outside that interval.
		if delay := at.Sub(start); delay <= slotNum*interval || delay >= 2*slotNum*interval {
			t.Errorf("task fired after %v, want a delay between %v and %v", delay, slotNum*interval, 2*slotNum*interval)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("task not executed")
	}
}

func Test_timeWheelConcurrentAddRemove(t *testing.T) {
	// Hammer AddTask/RemoveTask from many goroutines while the wheel ticks;
	// run with -race to verify the wheel state stays race-free, and Stop while
	// tasks are still pending.
	timeWheel := NewTimeWheel(8, time.Millisecond)

	executed := make(chan struct{}, 1024)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := range 50 {
				key := fmt.Sprintf("task-%d-%d", i, j)
				timeWheel.AddTask(key, func() { executed <- struct{}{} },
					time.Now().Add(time.Duration(j%7)*time.Millisecond))
				if j%2 == 0 {
					timeWheel.RemoveTask(key)
				}
			}
		}(i)
	}
	wg.Wait()

	// Every key fires at most once, so at most 800 sends fit in the channel
	// buffer and task goroutines never block, even after Stop.
	time.Sleep(50 * time.Millisecond)
	timeWheel.Stop()
	if len(executed) == 0 {
		t.Error("no tasks were executed")
	}
}

// newDirectRTimeWheel returns an RTimeWheel whose background loop is not
// running, so tests can drive getExecutableTasks with a fixed clock without
// the ticker racing them for due tasks.
func newDirectRTimeWheel(t *testing.T, addr string) *RTimeWheel {
	t.Helper()

	rw := &RTimeWheel{
		redisClient: redis.NewClient("tcp", addr, ""),
		httpClient:  thttp.NewClient(),
		stopc:       make(chan struct{}),
		ticker:      time.NewTicker(time.Hour),
	}
	t.Cleanup(func() {
		rw.Stop()
		_ = rw.redisClient.Close()
	})
	return rw
}

func Test_redis_timeWheelFetchWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rw := newDirectRTimeWheel(t, mr.Addr())

	ctx := context.Background()
	base := timex.TruncateToSecond(time.Now())
	// Keep the whole test window inside one minute shard: the previous-second
	// retry never crosses a minute boundary, because the task of the previous
	// second lives in the previous minute's shard.
	switch s := base.Second(); {
	case s < 5:
		base = base.Add(time.Duration(5-s) * time.Second)
	case s > 50:
		base = base.Add(time.Duration(65-s) * time.Second)
	}

	add := func(key string, at time.Time) {
		t.Helper()
		err := rw.AddTask(ctx, key, &RTaskElement{
			CallbackURL: "http://127.0.0.1:1/callback",
			Method:      http.MethodPost,
			Req:         map[string]string{"key": key},
		}, at)
		if err != nil {
			t.Fatalf("add task %s: %v", key, err)
		}
	}
	fetch := func(now time.Time) map[string]bool {
		t.Helper()
		tasks, err := rw.getExecutableTasks(ctx, now)
		if err != nil {
			t.Fatalf("getExecutableTasks: %v", err)
		}
		got := make(map[string]bool, len(tasks))
		for _, task := range tasks {
			got[task.Key] = true
		}
		return got
	}

	add("prev", base.Add(-time.Second))
	add("due", base)
	add("next", base.Add(time.Second))
	add("later", base.Add(2*time.Second))

	// A fetch during the due second returns the current and the previous
	// second's tasks; it must not return the next second's tasks a round
	// early.
	got := fetch(base)
	if !got["prev"] || !got["due"] {
		t.Errorf("fetch at the due second returned %v, want prev and due", got)
	}
	if got["next"] || got["later"] {
		t.Errorf("fetch at the due second returned future tasks: %v", got)
	}

	// Fetched tasks are removed from the zset: a second fetch is empty, and
	// the future tasks stay behind for their own round.
	if again := fetch(base); len(again) != 0 {
		t.Errorf("second fetch at the same second returned %v, want none", again)
	}
	remaining, err := rw.redisClient.ZCard(ctx, rw.getMinuteSlice(base)).Result()
	if err != nil {
		t.Fatalf("zcard: %v", err)
	}
	if remaining != 2 {
		t.Errorf("%d tasks left in the shard, want 2 (next and later)", remaining)
	}

	// The next second's task is fetched on its own round, on time.
	if got = fetch(base.Add(time.Second)); !got["next"] || got["later"] {
		t.Errorf("fetch one second later returned %v, want next only", got)
	}

	// A removed task is skipped when it becomes due...
	add("removed", base.Add(time.Second))
	if err := rw.RemoveTask(ctx, "removed", base.Add(time.Second)); err != nil {
		t.Fatalf("remove task: %v", err)
	}
	ttl, err := rw.redisClient.TTL(ctx, rw.getDeleteSetKey(base.Add(time.Second))).Result()
	if err != nil {
		t.Fatalf("ttl: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("delete set has no ttl set: %v", ttl)
	}
	if got := fetch(base.Add(time.Second)); got["removed"] {
		t.Error("removed task was fetched")
	}

	// ...but re-adding the same key clears the deletion flag.
	add("removed", base.Add(time.Second))
	if got := fetch(base.Add(time.Second)); !got["removed"] {
		t.Error("re-added task was still treated as deleted")
	}
}
