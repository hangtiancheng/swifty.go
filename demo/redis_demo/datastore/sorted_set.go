package datastore

import (
	"math"
	"math/rand"
	"strconv"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
)

func (k *KVStore) getAsSortedSet(key string) (SortedSet, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	zset, ok := v.(SortedSet)
	if !ok {
		return nil, handler.NewWrongTypeErrReply()
	}

	return zset, nil
}

func (k *KVStore) putAsSortedSet(key string, zset SortedSet) {
	k.data[key] = zset
}

type SortedSet interface {
	Add(score int64, member string)
	Rem(member string) int64
	Range(score1, score2 int64) []string
	database.CmdAdapter
}

type skiplist struct {
	key           string
	scoreToNode   map[int64]*skipNode
	memberToScore map[string]int64
	head          *skipNode
	randInst      *rand.Rand
}

func newSkiplist(key string) SortedSet {
	return &skiplist{
		key:           key,
		memberToScore: make(map[string]int64),
		scoreToNode:   make(map[int64]*skipNode),
		head:          newSkipNode(0, 0),
		randInst:      rand.New((rand.NewSource(lib.TimeNow().UnixNano()))),
	}
}

func (s *skiplist) Add(score int64, member string) {
	// Already exists; remove the old entry first.
	oldScore, ok := s.memberToScore[member]
	if ok {
		if oldScore == score {
			return
		}
		s.rem(oldScore, member)
	}

	s.memberToScore[member] = score
	node, ok := s.scoreToNode[score]
	if ok {
		node.members[member] = struct{}{}
		return
	}

	// New insert; roll a height.
	height := s.roll()
	for int64(len(s.head.nexts)) < height+1 {
		s.head.nexts = append(s.head.nexts, nil)
	}

	inserted := newSkipNode(score, height+1)
	inserted.members[member] = struct{}{}
	s.scoreToNode[score] = inserted

	move := s.head
	for i := height; i >= 0; i-- {
		for move.nexts[i] != nil && move.nexts[i].score < score {
			move = move.nexts[i]
			continue
		}

		inserted.nexts[i] = move.nexts[i]
		move.nexts[i] = inserted
	}
}

func (s *skiplist) Rem(member string) int64 {
	// Already exists; remove the old entry first.
	score, ok := s.memberToScore[member]
	if !ok {
		return 0
	}
	s.rem(score, member)
	return 1
}

// [score1,score2]
func (s *skiplist) Range(score1, score2 int64) []string {
	if score2 == -1 {
		score2 = math.MaxInt64
	}

	if score1 > score2 {
		return []string{}
	}

	move := s.head
	for i := len(s.head.nexts) - 1; i >= 0; i-- {
		for move.nexts[i] != nil && move.nexts[i].score < score1 {
			move = move.nexts[i]
		}
	}

	// At level 0, move.nexts[0] (if present) is the first element >= score1.
	if len(move.nexts) == 0 || move.nexts[0] == nil {
		return []string{}
	}

	res := []string{}
	for move.nexts[0] != nil && move.nexts[0].score >= score1 && move.nexts[0].score <= score2 {
		for member := range move.nexts[0].members {
			res = append(res, member)
		}
		move = move.nexts[0]
	}
	return res
}

func (s *skiplist) roll() int64 {
	var level int64
	for s.randInst.Intn(2) > 0 {
		level++
	}
	return level
}

func (s *skiplist) rem(score int64, member string) {
	delete(s.memberToScore, member)
	node := s.scoreToNode[score]

	delete(node.members, member)
	if len(node.members) > 0 {
		return
	}

	delete(s.scoreToNode, score)
	move := s.head
	for i := len(s.head.nexts) - 1; i >= 0; i-- {
		for move.nexts[i] != nil && move.nexts[i].score < score {
			move = move.nexts[i]
		}

		if move.nexts[i] == nil || move.nexts[i].score > score {
			continue
		}

		remVal := move.nexts[i]
		move.nexts[i] = move.nexts[i].nexts[i]
		remVal.nexts[i] = nil
	}
}

func (s *skiplist) ToCmd() [][]byte {
	args := make([][]byte, 0, 2+2*len(s.memberToScore))
	args = append(args, []byte(database.CmdTypeZAdd), []byte(s.key))
	for member, score := range s.memberToScore {
		scoreStr := strconv.FormatInt(score, 10)
		args = append(args, []byte(scoreStr), []byte(member))
	}
	return args
}

type skipNode struct {
	score   int64
	members map[string]struct{}
	nexts   []*skipNode
}

func newSkipNode(score, height int64) *skipNode {
	return &skipNode{
		score:   score,
		members: make(map[string]struct{}),
		nexts:   make([]*skipNode, height),
	}
}
