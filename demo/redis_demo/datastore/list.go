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

func (k *KVStore) getAsList(key string) (List, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	list, ok := v.(List)
	if !ok {
		return nil, handler.NewWrongTypeErrReply()
	}

	return list, nil
}

func (k *KVStore) putAsList(key string, list List) {
	k.data[key] = list
}

type List interface {
	LPush(value []byte)
	LPop(cnt int64) [][]byte
	RPush(value []byte)
	RPop(cnt int64) [][]byte
	Len() int64
	Range(start, stop int64) [][]byte
	database.CmdAdapter
}

type listEntity struct {
	key  string
	data [][]byte
}

func newListEntity(key string, elements ...[]byte) List {
	return &listEntity{
		key:  key,
		data: elements,
	}
}

func (l *listEntity) LPush(value []byte) {
	l.data = append([][]byte{value}, l.data...)
}

func (l *listEntity) LPop(cnt int64) [][]byte {
	if int64(len(l.data)) < cnt {
		return nil
	}

	popped := l.data[:cnt]
	l.data = l.data[cnt:]
	return popped
}

func (l *listEntity) RPush(value []byte) {
	l.data = append(l.data, value)
}

func (l *listEntity) RPop(cnt int64) [][]byte {
	if int64(len(l.data)) < cnt {
		return nil
	}

	popped := l.data[int64(len(l.data))-cnt:]
	l.data = l.data[:int64(len(l.data))-cnt]
	return popped
}

func (l *listEntity) Len() int64 {
	return int64(len(l.data))
}

func (l *listEntity) Range(start, stop int64) [][]byte {
	if stop == -1 {
		stop = int64(len(l.data) - 1)
	}

	if start < 0 || start >= int64(len(l.data)) {
		return nil
	}

	if stop < 0 || stop >= int64(len(l.data)) || stop < start {
		return nil
	}

	return l.data[start : stop+1]
}

func (l *listEntity) ToCmd() [][]byte {
	args := make([][]byte, 0, 2+l.Len())
	args = append(args, []byte(database.CmdTypeRPush), []byte(l.key))
	args = append(args, l.data...)
	return args
}
