// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
