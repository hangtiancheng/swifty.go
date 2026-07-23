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
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
)

func (k *KVStore) getAsHashMap(key string) (HashMap, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	hashmap, ok := v.(HashMap)
	if !ok {
		return nil, handler.NewWrongTypeErrReply()
	}

	return hashmap, nil
}

func (k *KVStore) putAsHashMap(key string, hashmap HashMap) {
	k.data[key] = hashmap
}

type HashMap interface {
	Put(key string, value []byte)
	Get(key string) []byte
	Del(key string) int64
	database.CmdAdapter
}

type hashMapEntity struct {
	key  string
	data map[string][]byte
}

func newHashMapEntity(key string) HashMap {
	return &hashMapEntity{
		key:  key,
		data: make(map[string][]byte),
	}
}

func (h *hashMapEntity) Put(key string, value []byte) {
	h.data[key] = value
}

func (h *hashMapEntity) Get(key string) []byte {
	return h.data[key]
}

func (h *hashMapEntity) Del(key string) int64 {
	if _, ok := h.data[key]; !ok {
		return 0
	}
	delete(h.data, key)
	return 1
}

func (h *hashMapEntity) ToCmd() [][]byte {
	args := make([][]byte, 0, 2+2*len(h.data))
	args = append(args, []byte(database.CmdTypeHSet), []byte(h.key))
	for k, v := range h.data {
		args = append(args, []byte(k), v)
	}
	return args
}
