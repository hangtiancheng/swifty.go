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
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
)

func Test_list_crud(t *testing.T) {
	list := newListEntity("")
	l := make([][]byte, 0, 1000)
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	for i := 0; i < 1000; i++ {
		member1 := randInst.Intn(1000)
		member2 := randInst.Intn(1000)
		list.LPush([]byte(strconv.Itoa(member1)))
		list.RPush([]byte(strconv.Itoa(member2)))
		l = append([][]byte{[]byte(strconv.Itoa(member1))}, l...)
		l = append(l, []byte(strconv.Itoa(member2)))
	}

	t.Run("range", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			start := randInst.Intn(1001)
			end := start + randInst.Intn(1000)
			actual := list.Range(int64(start), int64(end))
			expect := l[start : end+1]
			if !reflect.DeepEqual(expect, actual) {
				t.Errorf("range [%d:%d], expect: %v, got: %v", start, end, expect, actual)
			}
		}
	})

	t.Run("pop", func(t *testing.T) {
		for i := 0; i < 500; i++ {
			actual := list.LPop(2)
			expect := l[:2]
			l = l[2:]
			if !reflect.DeepEqual(expect, actual) {
				t.Errorf("lpop, expect: %v, got: %v", expect, actual)
			}

			actual = list.RPop(2)
			expect = l[len(l)-2:]
			l = l[:len(l)-2]
			if !reflect.DeepEqual(expect, actual) {
				t.Errorf("rpop, expect: %v, got: %v", expect, actual)
			}
		}
	})
}

func Test_list_to_cmds(t *testing.T) {
	list := newListEntity("")
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	l := make([]int, 0, 1000)
	// Insert 1000 entries.
	for i := 0; i < 1000; i++ {
		member := randInst.Intn(1000)
		list.LPush([]byte(strconv.Itoa(member)))
		l = append(l, member)
	}

	cmd := list.ToCmd()
	t.Run("length", func(t *testing.T) {
		if len(l)+2 != len(cmd) {
			t.Errorf("length, expect: %d, got: %d", len(l)+2, len(cmd))
		}
	})
	t.Run("command", func(t *testing.T) {
		if database.CmdTypeRPush != database.CmdType(cmd[0]) {
			t.Errorf("command, expect: %s, got: %s", database.CmdTypeRPush, database.CmdType(cmd[0]))
		}
	})
	t.Run("key", func(t *testing.T) {
		if "" != string(cmd[1]) {
			t.Errorf("key, expect: empty, got: %s", string(cmd[1]))
		}
	})

	actual := make([]int, 0, len(cmd)-2)
	for i := 2; i < len(cmd); i++ {
		v, _ := strconv.Atoi(string(cmd[i]))
		actual = append(actual, v)
	}
	sort.Slice(actual, func(i, j int) bool {
		return actual[i] < actual[j]
	})

	expect := l
	sort.Slice(expect, func(i, j int) bool {
		return expect[i] < expect[j]
	})

	t.Run("member", func(t *testing.T) {
		if !reflect.DeepEqual(expect, actual) {
			t.Errorf("member, expect: %v, got: %v", expect, actual)
		}
	})
}
