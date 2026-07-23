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

func (k *KVStore) getAsString(key string) (String, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	str, ok := v.(String)
	if !ok {
		return nil, handler.NewWrongTypeErrReply()
	}

	return str, nil
}

func (k *KVStore) put(key, value string, insertStrategy bool) int64 {
	if _, ok := k.data[key]; ok && insertStrategy {
		return 0
	}

	k.data[key] = NewString(key, value)
	return 1
}

type String interface {
	Bytes() []byte
	database.CmdAdapter
}

type stringEntity struct {
	key, str string
}

func NewString(key, str string) String {
	return &stringEntity{key: key, str: str}
}

func (s *stringEntity) Bytes() []byte {
	return []byte(s.str)
}

func (s *stringEntity) ToCmd() [][]byte {
	return [][]byte{[]byte(database.CmdTypeSet), []byte(s.key), []byte(s.str)}
}
