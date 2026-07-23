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

func Test_hashmap_crud(t *testing.T) {
	hashmap := newHashMapEntity("")
	mp := make(map[int]int, 1000)

	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	for i := 0; i < 1000; i++ {
		k := randInst.Intn(1000)
		v := randInst.Intn(1000)
		hashmap.Put(strconv.Itoa(k), []byte(strconv.Itoa(v)))
		mp[k] = v
	}

	t.Run("delete", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			k := randInst.Intn(1000)
			_, ok := mp[k]
			exist := hashmap.Del(strconv.Itoa(k))
			if ok != (exist == 1) {
				t.Errorf("key: %d, expect exist: %v, got: %d", k, ok, exist)
			}
			delete(mp, k)
		}
	})

	t.Run("get", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			k := randInst.Intn(1000)
			value := hashmap.Get(strconv.Itoa(k))
			v, ok := mp[k]
			if ok != (value != nil) {
				t.Errorf("key: %d, expect exist: %v, got: %v", k, ok, value != nil)
			}
			if ok {
				got, _ := strconv.Atoi(string(value))
				if v != got {
					t.Errorf("key: %d, expect: %d, got: %d", k, v, got)
				}
			}
		}
	})
}

func Test_hashmap_to_cmd(t *testing.T) {
	hashmap := newHashMapEntity("")
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	mp := make(map[int]int, 1000)
	// Insert 1000 entries.
	for i := 0; i < 1000; i++ {
		k := randInst.Intn(1000)
		v := randInst.Intn(1000)
		hashmap.Put(strconv.Itoa(k), []byte(strconv.Itoa(v)))
		mp[k] = v
	}

	cmd := hashmap.ToCmd()
	t.Run("length", func(t *testing.T) {
		if 2*len(mp)+2 != len(cmd) {
			t.Errorf("length, expect: %d, got: %d", 2*len(mp)+2, len(cmd))
		}
	})
	t.Run("command", func(t *testing.T) {
		if database.CmdTypeHSet != database.CmdType(cmd[0]) {
			t.Errorf("command, expect: %s, got: %s", database.CmdTypeHSet, database.CmdType(cmd[0]))
		}
	})
	t.Run("key", func(t *testing.T) {
		if "" != string(cmd[1]) {
			t.Errorf("key, expect: empty, got: %s", string(cmd[1]))
		}
	})

	type kv struct {
		k, v int
	}
	actual := make([]kv, 0, 1000)
	for i := 2; i < len(cmd); i += 2 {
		k, _ := strconv.Atoi(string(cmd[i]))
		v, _ := strconv.Atoi(string(cmd[i+1]))
		actual = append(actual, kv{k: k, v: v})
	}

	sort.Slice(actual, func(i, j int) bool {
		if actual[i].k == actual[j].k {
			return actual[i].v < actual[j].v
		}
		return actual[i].k < actual[j].k
	})

	expect := make([]kv, 0, 2*len(mp))
	for k, v := range mp {
		expect = append(expect, kv{k: k, v: v})
	}
	sort.Slice(expect, func(i, j int) bool {
		if expect[i].k == expect[j].k {
			return expect[i].v < expect[j].v
		}
		return expect[i].k < expect[j].k
	})

	t.Run("member", func(t *testing.T) {
		if !reflect.DeepEqual(expect, actual) {
			t.Errorf("member, expect: %v, got: %v", expect, actual)
		}
	})
}
