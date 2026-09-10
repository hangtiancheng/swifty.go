package timewheel

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	thttp "github.com/hangtiancheng/swifty.go/apps/timewheel/internal/http"
	"github.com/hangtiancheng/swifty.go/apps/timewheel/internal/redis"
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
