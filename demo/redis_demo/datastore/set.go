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

func (k *KVStore) getAsSet(key string) (Set, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	set, ok := v.(Set)
	if !ok {
		return nil, handler.NewWrongTypeErrReply()
	}

	return set, nil
}

func (k *KVStore) putAsSet(key string, set Set) {
	k.data[key] = set
}

type Set interface {
	Add(value string) int64
	Exist(value string) int64
	Rem(value string) int64
	database.CmdAdapter
}

type setEntity struct {
	key       string
	container map[string]struct{}
}

func newSetEntity(key string) Set {
	return &setEntity{
		key:       key,
		container: make(map[string]struct{}),
	}
}

func (s *setEntity) Add(value string) int64 {
	if _, ok := s.container[value]; ok {
		return 0
	}
	s.container[value] = struct{}{}
	return 1
}

func (s *setEntity) Exist(value string) int64 {
	if _, ok := s.container[value]; ok {
		return 1
	}
	return 0
}

func (s *setEntity) Rem(value string) int64 {
	if _, ok := s.container[value]; ok {
		delete(s.container, value)
		return 1
	}
	return 0
}

func (s *setEntity) ToCmd() [][]byte {
	args := make([][]byte, 0, 2+len(s.container))
	args = append(args, []byte(database.CmdTypeSAdd), []byte(s.key))
	for k := range s.container {
		args = append(args, []byte(k))
	}

	return args
}
