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
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
)

func Test_skiplist_add_rem_range(t *testing.T) {
	skiplist := newSkiplist("")
	// Add 1000 entries.
	for i := 0; i < 1000; i++ {
		skiplist.Add(int64(i), fmt.Sprintf("%d_0", i))
		skiplist.Add(int64(i), fmt.Sprintf("%d_1", i))
	}

	// Randomly remove 1000 members.
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	remSet := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		score := randInst.Intn(1000)
		index := randInst.Intn(2)
		member := fmt.Sprintf("%d_%d", score, index)
		remSet[member] = struct{}{}
		skiplist.Rem(member)
	}

	t.Run("single_score", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			score := int64(randInst.Intn(1000))
			member := skiplist.Range(score, score)
			sort.Slice(member, func(i, j int) bool {
				return member[i] < member[j]
			})
			expected := make([]string, 0, 2)
			member1 := fmt.Sprintf("%d_0", score)
			member2 := fmt.Sprintf("%d_1", score)
			if _, ok := remSet[member1]; !ok {
				expected = append(expected, member1)
			}
			if _, ok := remSet[member2]; !ok {
				expected = append(expected, member2)
			}

			if !reflect.DeepEqual(expected, member) {
				t.Errorf("score: %d, expect: %v, got: %v", score, expected, member)
			}
		}
	})

	t.Run("normal_score_range", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			leftScore := int64(randInst.Intn(501))
			rightScore := leftScore + int64(randInst.Intn(500))
			member := skiplist.Range(leftScore, rightScore)
			sort.Slice(member, func(i, j int) bool {
				arr := strings.Split(member[i], "_")
				arr2 := strings.Split(member[j], "_")
				if arr[0] == arr2[0] {
					v1, _ := strconv.Atoi(arr[1])
					v2, _ := strconv.Atoi(arr2[1])
					return v1 < v2
				}
				v1, _ := strconv.Atoi(arr[0])
				v2, _ := strconv.Atoi(arr2[0])
				return v1 < v2
			})

			expected := make([]string, 0, 2*(rightScore-leftScore+1))
			for j := leftScore; j <= rightScore; j++ {
				member1 := fmt.Sprintf("%d_0", j)
				member2 := fmt.Sprintf("%d_1", j)
				if _, ok := remSet[member1]; !ok {
					expected = append(expected, member1)
				}
				if _, ok := remSet[member2]; !ok {
					expected = append(expected, member2)
				}
			}
			if !reflect.DeepEqual(expected, member) {
				t.Errorf("range [%d:%d], expect: %v, got: %v", leftScore, rightScore, expected, member)
			}
		}
	})

	t.Run("with_maximum_right_range", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			leftScore := int64(randInst.Intn(1000))
			rightScore := int64(-1)
			member := skiplist.Range(leftScore, rightScore)
			sort.Slice(member, func(i, j int) bool {
				arr := strings.Split(member[i], "_")
				arr2 := strings.Split(member[j], "_")
				if arr[0] == arr2[0] {
					v1, _ := strconv.Atoi(arr[1])
					v2, _ := strconv.Atoi(arr2[1])
					return v1 < v2
				}
				v1, _ := strconv.Atoi(arr[0])
				v2, _ := strconv.Atoi(arr2[0])
				return v1 < v2
			})

			expected := make([]string, 0, 2*(1000-leftScore))
			for j := leftScore; j < 1000; j++ {
				member1 := fmt.Sprintf("%d_0", j)
				member2 := fmt.Sprintf("%d_1", j)
				if _, ok := remSet[member1]; !ok {
					expected = append(expected, member1)
				}
				if _, ok := remSet[member2]; !ok {
					expected = append(expected, member2)
				}
			}
			if !reflect.DeepEqual(expected, member) {
				t.Errorf("range [%d:-1], expect: %v, got: %v", leftScore, expected, member)
			}
		}
	})
}

func Test_skiplist_upsert_member_with_dif_score(t *testing.T) {
	skiplist := newSkiplist("")
	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	scoreToMembers := make(map[int64][]string)
	memberSet := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		score1 := int64(randInst.Intn(1000))
		member := strconv.FormatInt(score1, 10)
		if _, ok := memberSet[member]; ok {
			continue
		}
		memberSet[member] = struct{}{}
		skiplist.Add(score1, member)
		score2 := int64(randInst.Intn(1000))
		skiplist.Add(score2, member)
		scoreToMembers[score2] = append(scoreToMembers[score2], member)
	}

	t.Run("score_to_members", func(t *testing.T) {
		for score, members := range scoreToMembers {
			sort.Slice(members, func(i, j int) bool {
				v1, _ := strconv.Atoi(members[i])
				v2, _ := strconv.Atoi(members[j])
				return v1 < v2
			})

			actualMembers := skiplist.Range(score, score)
			sort.Slice(actualMembers, func(i, j int) bool {
				v1, _ := strconv.Atoi(actualMembers[i])
				v2, _ := strconv.Atoi(actualMembers[j])
				return v1 < v2
			})

			if !reflect.DeepEqual(members, actualMembers) {
				t.Errorf("score: %d, expect: %v, got: %v", score, members, actualMembers)
			}

			// The member's previous score must not return the member.
			for _, member := range members {
				oldScore, _ := strconv.ParseInt(member, 10, 64)
				if oldScore == score {
					continue
				}
				for _, gotMember := range skiplist.Range(oldScore, oldScore) {
					if gotMember == member {
						t.Errorf("old score: %d, member: %s should not be found", oldScore, gotMember)
					}
				}
			}
		}
	})
}

func Test_skiplist_to_cmd(t *testing.T) {
	skiplist := newSkiplist("")

	randInst := rand.New(rand.NewSource(lib.TimeNow().UnixNano()))
	memberToScore := make(map[int]int, 1000)
	// Insert 1000 entries.
	for i := 0; i < 1000; i++ {
		score := randInst.Intn(1000)
		member := randInst.Intn(1000)
		skiplist.Add(int64(score), strconv.Itoa(member))
		memberToScore[member] = score
	}

	cmd := skiplist.ToCmd()
	t.Run("length", func(t *testing.T) {
		if 2*len(memberToScore)+2 != len(cmd) {
			t.Errorf("length, expect: %d, got: %d", 2*len(memberToScore)+2, len(cmd))
		}
	})
	t.Run("command", func(t *testing.T) {
		if database.CmdTypeZAdd != database.CmdType(cmd[0]) {
			t.Errorf("command, expect: %s, got: %s", database.CmdTypeZAdd, database.CmdType(cmd[0]))
		}
	})
	t.Run("key", func(t *testing.T) {
		if "" != string(cmd[1]) {
			t.Errorf("key, expect: empty, got: %s", string(cmd[1]))
		}
	})

	type scoreToMember struct {
		score, member int
	}
	actual := make([]scoreToMember, 0, 1000)
	for i := 2; i < len(cmd); i += 2 {
		score, _ := strconv.Atoi(string(cmd[i]))
		member, _ := strconv.Atoi(string(cmd[i+1]))
		actual = append(actual, scoreToMember{score: score, member: member})
	}

	sort.Slice(actual, func(i, j int) bool {
		if actual[i].score == actual[j].score {
			return actual[i].member < actual[j].member
		}
		return actual[i].score < actual[j].score
	})

	expect := make([]scoreToMember, 0, 2*len(memberToScore))
	for member, score := range memberToScore {
		expect = append(expect, scoreToMember{score: score, member: member})
	}
	sort.Slice(expect, func(i, j int) bool {
		if expect[i].score == expect[j].score {
			return expect[i].member < expect[j].member
		}
		return expect[i].score < expect[j].score
	})

	t.Run("member", func(t *testing.T) {
		if !reflect.DeepEqual(expect, actual) {
			t.Errorf("member, expect: %v, got: %v", expect, actual)
		}
	})
}
