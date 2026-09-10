package timewheel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	thttp "github.com/hangtiancheng/swifty.go/apps/timewheel/internal/http"
	"github.com/hangtiancheng/swifty.go/apps/timewheel/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/timewheel/internal/timex"
)

// RTaskElement is a distributed time wheel task that triggers an HTTP
// callback when it becomes due.
type RTaskElement struct {
	Key         string            `json:"key"`
	CallbackURL string            `json:"callback_url"`
	Method      string            `json:"method"`
	Req         any               `json:"req"`
	Header      map[string]string `json:"header"`
}

// RTimeWheel is a distributed timing wheel backed by redis zsets: tasks are
// stored in minute-level zset shards and fetched once per second for
// execution. Create one with NewRTimeWheel and stop it with Stop.
type RTimeWheel struct {
	sync.Once
	redisClient *redis.Client
	httpClient  *thttp.Client
	stopc       chan struct{}
	ticker      *time.Ticker
}

// NewRTimeWheel creates and starts a redis-backed timing wheel using the
// given redis client to store tasks and the given HTTP client to deliver
// callbacks.
func NewRTimeWheel(redisClient *redis.Client, httpClient *thttp.Client) *RTimeWheel {
	r := RTimeWheel{
		ticker:      time.NewTicker(time.Second),
		redisClient: redisClient,
		httpClient:  httpClient,
		stopc:       make(chan struct{}),
	}

	go r.run()
	return &r
}

// Stop stops the wheel. It is idempotent.
func (r *RTimeWheel) Stop() {
	r.Do(func() {
		close(r.stopc)
		r.ticker.Stop()
	})
}

// AddTask schedules an HTTP callback task for the given executeAt. Tasks are
// stored in the minute-level zset shard of executeAt, keyed by key; re-adding
// a key clears its deletion flag first.
func (r *RTimeWheel) AddTask(ctx context.Context, key string, task *RTaskElement, executeAt time.Time) error {
	if err := r.addTaskPrecheck(task); err != nil {
		return err
	}

	task.Key = key
	taskBody, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	keys := []string{
		// Minute-level zset shard the task belongs to.
		r.getMinuteSlice(executeAt),
		// Set holding the keys flagged as deleted.
		r.getDeleteSetKey(executeAt),
	}
	args := []any{
		// The second-level unix timestamp of the execution time is the
		// zset score.
		executeAt.Unix(),
		// Serialized task body.
		string(taskBody),
		// Task key, stored in the delete set when removed.
		key,
	}
	return goredis.NewScript(LuaAddTasks).Run(ctx, r.redisClient, keys, args).Err()
}

// RemoveTask flags the task registered under key as deleted, so it is skipped
// when it becomes due.
func (r *RTimeWheel) RemoveTask(ctx context.Context, key string, executeAt time.Time) error {
	keys := []string{r.getDeleteSetKey(executeAt)}
	args := []any{key}
	return goredis.NewScript(LuaDeleteTask).Run(ctx, r.redisClient, keys, args).Err()
}

func (r *RTimeWheel) run() {
	for {
		select {
		case <-r.stopc:
			return
		case <-r.ticker.C:
			// Fetch and run the due tasks on every tick.
			go r.executeTasks()
		}
	}
}

func (r *RTimeWheel) executeTasks() {
	defer func() {
		// A panic here must not crash the process.
		_ = recover()
	}()

	// Bound the whole fetch-and-execute round to 30 seconds.
	tctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	tasks, err := r.getExecutableTasks(tctx)
	if err != nil {
		// TODO: log the fetch error.
		return
	}

	// Execute the tasks concurrently.
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		task := task // capture the loop variable
		go func() {
			defer wg.Done()
			// TODO: log the execution error.
			_ = r.executeTask(tctx, task)
		}()
	}
	wg.Wait()
}

func (r *RTimeWheel) executeTask(ctx context.Context, task *RTaskElement) error {
	return r.httpClient.JSONDo(ctx, task.Method, task.CallbackURL, task.Header, task.Req, nil)
}

func (r *RTimeWheel) addTaskPrecheck(task *RTaskElement) error {
	if task.Method != http.MethodGet && task.Method != http.MethodPost {
		return fmt.Errorf("invalid method: %s", task.Method)
	}
	if !strings.HasPrefix(task.CallbackURL, "http://") && !strings.HasPrefix(task.CallbackURL, "https://") {
		return fmt.Errorf("invalid url: %s", task.CallbackURL)
	}
	return nil
}

// getExecutableTasks fetches the tasks whose score (execution second) falls
// into the current second, excluding the ones flagged as deleted.
func (r *RTimeWheel) getExecutableTasks(ctx context.Context) ([]*RTaskElement, error) {
	now := time.Now()
	minuteSlice := r.getMinuteSlice(now)
	deleteSetKey := r.getDeleteSetKey(now)
	nowSecond := timex.TruncateToSecond(now)
	score1 := nowSecond.Unix()
	score2 := nowSecond.Add(time.Second).Unix()

	keys := []string{minuteSlice, deleteSetKey}
	args := []any{score1, score2}
	rawReply, err := goredis.NewScript(LuaZrangeTasks).Run(ctx, r.redisClient, keys, args).Result()
	if err != nil {
		return nil, fmt.Errorf("fetch executable tasks: %w", err)
	}

	replies, ok := rawReply.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected reply type %T from LuaZrangeTasks", rawReply)
	}
	if len(replies) == 0 {
		// No delete set and no due tasks.
		return nil, nil
	}

	// replies[0] holds the delete set members.
	deletedMembers, _ := replies[0].([]any)
	deletedSet := make(map[string]struct{}, len(deletedMembers))
	for _, member := range deletedMembers {
		deletedSet[redisReplyString(member)] = struct{}{}
	}

	tasks := make([]*RTaskElement, 0, len(replies)-1)
	for i := 1; i < len(replies); i++ {
		var task RTaskElement
		if err := json.Unmarshal([]byte(redisReplyString(replies[i])), &task); err != nil {
			// TODO: log the unmarshal error and skip the malformed task.
			continue
		}

		if _, ok := deletedSet[task.Key]; ok {
			continue
		}
		tasks = append(tasks, &task)
	}

	return tasks, nil
}

// redisReplyString converts a redis reply element to its string form.
func redisReplyString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (r *RTimeWheel) getMinuteSlice(executeAt time.Time) string {
	return fmt.Sprintf("xiaoxu_timewheel_task_{%s}", timex.GetTimeMinuteStr(executeAt))
}

func (r *RTimeWheel) getDeleteSetKey(executeAt time.Time) string {
	return fmt.Sprintf("xiaoxu_timewheel_delset_{%s}", timex.GetTimeMinuteStr(executeAt))
}
