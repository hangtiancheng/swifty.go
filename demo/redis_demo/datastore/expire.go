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

package datastore

import (
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
)

func (k *KVStore) GC() {
	// Find all expired keys and reclaim them in batch.
	nowUnix := lib.TimeNow().Unix()
	for _, expiredKey := range k.expireTimeWheel.Range(0, nowUnix) {
		k.expireProcess(expiredKey)
	}
}

func (k *KVStore) ExpirePreprocess(key string) {
	expiredAt, ok := k.expiredAt[key]
	if !ok {
		return
	}

	if expiredAt.After(lib.TimeNow()) {
		return
	}

	k.expireProcess(key)
}

func (k *KVStore) expireProcess(key string) {
	delete(k.expiredAt, key)
	delete(k.data, key)
	k.expireTimeWheel.Rem(key)
}

func (k *KVStore) expire(key string, expiredAt time.Time) {
	if _, ok := k.data[key]; !ok {
		return
	}
	k.expiredAt[key] = expiredAt
	k.expireTimeWheel.Add(expiredAt.Unix(), key)
}
