package time_wheel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	time_wheel_http "github.com/hangtiancheng/swifty.go/demo/time_wheel/pkg/http"
	"github.com/hangtiancheng/swifty.go/demo/time_wheel/pkg/redis"
	"github.com/hangtiancheng/swifty.go/demo/time_wheel/pkg/util"
)

type RTaskElement struct {
	Key         string            `json:"key"`
	CallbackURL string            `json:"callback_url"`
	Method      string            `json:"method"`
	Req         interface{}       `json:"req"`
	Header      map[string]string `json:"header"`
}

type RTimeWheel struct {
	sync.Once
	redisClient *redis.Client
	httpClient  *time_wheel_http.Client
	stopChan    chan struct{}
	ticker      *time.Ticker
}

func NewRTimeWheel(redisClient *redis.Client, httpClient *time_wheel_http.Client) *RTimeWheel {
	r := RTimeWheel{
		ticker:      time.NewTicker(time.Second),
		redisClient: redisClient,
		httpClient:  httpClient,
		stopChan:    make(chan struct{}),
	}

	go r.run()
	return &r
}

func (r *RTimeWheel) Stop() {
	r.Do(func() {
		close(r.stopChan)
		r.ticker.Stop()
	})
}

func (r *RTimeWheel) AddTask(ctx context.Context, key string, task *RTaskElement, executeAt time.Time) error {
	if err := r.addTaskPreCheck(task); err != nil {
		return err
	}

	task.Key = key
	taskBody, _ := json.Marshal(task)
	_, err := r.redisClient.Eval(ctx, LuaAddTasks, 2, []interface{}{
		// Minute-level zset time slice.
		r.getMinuteSlice(executeAt),
		// Set marking tasks for deletion.
		r.getDeleteSetKey(executeAt),
		// The execution-time second-level unix timestamp serves as the zset score.
		executeAt.Unix(),
		// Task body.
		string(taskBody),
		// Task key, stored in the delete set.
		key,
	})
	return err
}

func (r *RTimeWheel) RemoveTask(ctx context.Context, key string, executeAt time.Time) error {
	// Mark the task as deleted.
	_, err := r.redisClient.Eval(ctx, LuaDeleteTask, 1, []interface{}{
		r.getDeleteSetKey(executeAt),
		key,
	})
	return err
}

func (r *RTimeWheel) run() {
	for {
		select {
		case <-r.stopChan:
			return
		case <-r.ticker.C:
			// Fetch tasks on each tick.
			go r.executeTasks()
		}
	}
}

func (r *RTimeWheel) executeTasks() {
	defer func() {
		if err := recover(); err != nil {
			// log
		}
	}()

	// Concurrency control: 30s timeout.
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	tasks, err := r.getExecutableTasks(ctxWithTimeout)
	if err != nil {
		// log
		return
	}

	// Execute tasks concurrently.
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		// shadow
		task := task
		go func() {
			defer func() {
				if err := recover(); err != nil {
				}
				wg.Done()
			}()
			if err := r.executeTask(ctxWithTimeout, task); err != nil {
				// log
			}
		}()
	}
	wg.Wait()
}

func (r *RTimeWheel) executeTask(ctx context.Context, task *RTaskElement) error {
	return r.httpClient.JSONDo(ctx, task.Method, task.CallbackURL, task.Header, task.Req, nil)
}

func (r *RTimeWheel) addTaskPreCheck(task *RTaskElement) error {
	if task.Method != http.MethodGet && task.Method != http.MethodPost {
		return fmt.Errorf("invalid method: %s", task.Method)
	}
	if !strings.HasPrefix(task.CallbackURL, "http://") && !strings.HasPrefix(task.CallbackURL, "https://") {
		return fmt.Errorf("invalid url: %s", task.CallbackURL)
	}
	return nil
}

func (r *RTimeWheel) getExecutableTasks(ctx context.Context) ([]*RTaskElement, error) {
	now := time.Now()
	minuteSlice := r.getMinuteSlice(now)
	deleteSetKey := r.getDeleteSetKey(now)
	nowSecond := util.GetTimeSecond(now)
	score1 := nowSecond.Unix()
	score2 := nowSecond.Add(time.Second).Unix()
	rawReply, err := r.redisClient.Eval(ctx, LuaZrangeTasks, 2, []interface{}{
		minuteSlice, deleteSetKey, score1, score2,
	})
	if err != nil {
		return nil, err
	}

	replies, ok := rawReply.([]interface{})
	if !ok || len(replies) == 0 {
		return nil, fmt.Errorf("invalid replies: %v", replies)
	}

	delArr, _ := replies[0].([]interface{})
	deletedSet := make(map[string]struct{}, len(delArr))
	for _, deleted := range delArr {
		deletedSet[toString(deleted)] = struct{}{}
	}

	tasks := make([]*RTaskElement, 0, len(replies)-1)
	for i := 1; i < len(replies); i++ {
		var task RTaskElement
		if err := json.Unmarshal([]byte(toString(replies[i])), &task); err != nil {
			// log
			continue
		}

		if _, ok := deletedSet[task.Key]; ok {
			continue
		}
		tasks = append(tasks, &task)
	}

	return tasks, nil
}

func (r *RTimeWheel) getMinuteSlice(executeAt time.Time) string {
	return fmt.Sprintf("swifty_time_wheel_task_{%s}", util.GetTimeMinuteStr(executeAt))
}

func (r *RTimeWheel) getDeleteSetKey(executeAt time.Time) string {
	return fmt.Sprintf("swifty_time_wheel_delete_set_{%s}", util.GetTimeMinuteStr(executeAt))
}

func toString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}
