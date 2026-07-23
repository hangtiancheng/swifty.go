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

func Test_set_crud(t *testing.T) {
	set := newSetEntity("")
	s := make(map[int]struct{}, 1000)
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))

	t.Run("add", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			member := randInst.Intn(1000)
			_, ok := s[member]
			success := set.Add(strconv.Itoa(member))
			s[member] = struct{}{}
			if ok != (success == 0) {
				t.Errorf("add member: %d, expect already exists: %v, got success: %d", member, ok, success)
			}
		}
	})

	t.Run("rem", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			member := randInst.Intn(1000)
			_, ok := s[member]
			exist := set.Rem(strconv.Itoa(member))
			delete(s, member)
			if ok != (exist == 1) {
				t.Errorf("rem member: %d, expect existed: %v, got: %d", member, ok, exist)
			}
		}
	})

	t.Run("exist", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			member := randInst.Intn(1000)
			_, ok := s[member]
			exist := set.Exist(strconv.Itoa(member))
			if ok != (exist == 1) {
				t.Errorf("exist member: %d, expect: %v, got: %d", member, ok, exist)
			}
		}
	})
}

func Test_set_to_cmd(t *testing.T) {
	set := newSetEntity("")
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	s := make(map[int]struct{}, 1000)
	// Insert 1000 entries.
	for i := 0; i < 1000; i++ {
		member := randInst.Intn(1000)
		set.Add(strconv.Itoa(member))
		s[member] = struct{}{}
	}

	cmd := set.ToCmd()
	t.Run("length", func(t *testing.T) {
		if len(s)+2 != len(cmd) {
			t.Errorf("length, expect: %d, got: %d", len(s)+2, len(cmd))
		}
	})
	t.Run("command", func(t *testing.T) {
		if database.CmdTypeSAdd != database.CmdType(cmd[0]) {
			t.Errorf("command, expect: %s, got: %s", database.CmdTypeSAdd, database.CmdType(cmd[0]))
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

	expect := make([]int, 0, len(s))
	for member := range s {
		expect = append(expect, member)
	}
	sort.Slice(expect, func(i, j int) bool {
		return expect[i] < expect[j]
	})

	t.Run("member", func(t *testing.T) {
		if !reflect.DeepEqual(expect, actual) {
			t.Errorf("member, expect: %v, got: %v", expect, actual)
		}
	})
}
