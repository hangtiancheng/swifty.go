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

package redis_lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func Test_blockingLock(t *testing.T) {
	// Fill in the redis node address and password.
	addr := "xxxx:xx"
	passwd := ""

	client := NewClient("tcp", addr, passwd)
	lock1 := NewRedisLock("test_key", client, WithExpireSeconds(1))
	lock2 := NewRedisLock("test_key", client, WithBlock(), WithBlockWaitingSeconds(2))

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock1.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock2.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Wait()

	t.Log("success")
}

func Test_nonBlockingLock(t *testing.T) {
	// Fill in the redis node address and password.
	addr := "xxxx:xx"
	passwd := ""

	client := NewClient("tcp", addr, passwd)
	lock1 := NewRedisLock("test_key", client, WithExpireSeconds(1))
	lock2 := NewRedisLock("test_key", client)

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock1.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock2.Lock(ctx); err == nil || !errors.Is(err, ErrLockAcquiredByOthers) {
			t.Errorf("got err: %v, expect: %v", err, ErrLockAcquiredByOthers)
			return
		}
	}()

	wg.Wait()
	t.Log("success")
}

func Test_redLock(t *testing.T) {
	// Fill in the addresses and passwords of three redis nodes.
	addr1 := "xxxx:xx"
	passwd1 := ""

	addr2 := "yyyy:yy"
	passwd2 := ""

	addr3 := "zzzz:zz"
	passwd3 := ""

	// Three locks, one per direct redis node.
	confs := []*SingleNodeConf{
		{
			Network:  "tcp",
			Address:  addr1,
			Password: passwd1,
		},
		{
			Network:  "tcp",
			Address:  addr2,
			Password: passwd2,
		},
		{
			Network:  "tcp",
			Address:  addr3,
			Password: passwd3,
		},
	}

	redLock, err := NewRedLock("test_key", confs, WithRedLockExpireDuration(10*time.Second), WithSingleNodesTimeout(100*time.Millisecond))
	if err != nil {
		return
	}

	ctx := context.Background()
	if err = redLock.Lock(ctx); err != nil {
		t.Error(err)
		return
	}

	if err = redLock.Unlock(ctx); err != nil {
		t.Error(err)
		return
	}

	t.Log("success")
}
