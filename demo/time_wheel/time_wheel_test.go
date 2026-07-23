package time_wheel

import (
	"context"
	"testing"
	"time"

	time_wheel_http "github.com/hangtiancheng/swifty.go/demo/time_wheel/pkg/http"
	"github.com/hangtiancheng/swifty.go/demo/time_wheel/pkg/redis"
)

func Test_timeWheel(t *testing.T) {
	timeWheel := NewTimeWheel(10, 500*time.Millisecond)
	defer timeWheel.Stop()

	timeWheel.AddTask("test1", func() {
		t.Errorf("test1, %v", time.Now())
	}, time.Now().Add(time.Second))
	timeWheel.AddTask("test2", func() {
		t.Errorf("test2, %v", time.Now())
	}, time.Now().Add(5*time.Second))
	timeWheel.AddTask("test2", func() {
		t.Errorf("test2, %v", time.Now())
	}, time.Now().Add(3*time.Second))

	<-time.After(6 * time.Second)
}

const (
	// redis server info
	network  = "tcp"
	address  = "please fill in redis address"
	password = "please fill in redis password"
)

var (
	// scheduled task callback info
	callbackURL    = "please fill in callback url"
	callbackMethod = "POST"
	callbackReq    interface{}
	callbackHeader map[string]string
)

func Test_redis_timeWheel(t *testing.T) {
	rTimeWheel := NewRTimeWheel(
		redis.NewClient(network, address, password),
		time_wheel_http.NewClient(),
	)
	defer rTimeWheel.Stop()

	ctx := context.Background()
	if err := rTimeWheel.AddTask(ctx, "test1", &RTaskElement{
		CallbackURL: callbackURL,
		Method:      callbackMethod,
		Req:         callbackReq,
		Header:      callbackHeader,
	}, time.Now().Add(time.Second)); err != nil {
		t.Error(err)
		return
	}

	if err := rTimeWheel.AddTask(ctx, "test2", &RTaskElement{
		CallbackURL: callbackURL,
		Method:      callbackMethod,
		Req:         callbackReq,
		Header:      callbackHeader,
	}, time.Now().Add(4*time.Second)); err != nil {
		t.Error(err)
		return
	}

	if err := rTimeWheel.RemoveTask(ctx, "test2", time.Now().Add(4*time.Second)); err != nil {
		t.Error(err)
		return
	}

	<-time.After(5 * time.Second)
	t.Log("ok")
}
