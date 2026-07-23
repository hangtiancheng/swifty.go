package local

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/consistent_hash/pkg/os"
	"github.com/hangtiancheng/swifty.go/demo/redis_lock/utils"
)

type LockEntityV2 struct {
	locked   bool
	mutex    sync.Mutex
	expireAt time.Time
	owner    string
}

func NewLockEntityV2() *LockEntityV2 {
	return &LockEntityV2{}
}

func (l *LockEntityV2) Lock(ctx context.Context, expireSeconds int) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	now := time.Now()
	if !l.locked || l.expireAt.Before(now) {
		l.locked = true
		l.expireAt = now.Add(time.Duration(expireSeconds) * time.Second)
		l.owner = utils.GetProcessAndGoroutineIDStr()
		return nil
	}

	return errors.New("accquire by others")
}

func (l *LockEntityV2) Unlock(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if !l.locked || l.expireAt.Before(time.Now()) {
		return errors.New("not locked")
	}

	if l.owner != utils.GetProcessAndGoroutineIDStr() {
		return errors.New("not your lock")
	}

	l.locked = false
	return nil
}

// SkiplistHashRing is a local in-memory hash ring backed by a skip list.
type SkiplistHashRing struct {
	LockEntity
	root *virtualNode
	// nodeToReplicas maps each node to its virtual-node count.
	nodeToReplicas map[string]int
	nodeToDataKey  map[string]map[string]struct{}
}

type LockEntity struct {
	lock sync.Mutex
	// doubleLock supports idempotent unlock.
	doubleLock sync.Mutex
	cancel     context.CancelFunc
	owner      atomic.Value
}

func NewSkiplistHashRing() *SkiplistHashRing {
	return &SkiplistHashRing{
		root:           &virtualNode{},
		nodeToReplicas: make(map[string]int),
		nodeToDataKey:  make(map[string]map[string]struct{}),
	}
}

type virtualNode struct {
	score int32
	// nodeIDs stores the node ids at this score.
	nodeIDs []string
	nexts   []*virtualNode
}

// Lock acquires the hash-ring lock with the given TTL. The lock auto-releases on expiry.
func (s *SkiplistHashRing) Lock(ctx context.Context, expireSeconds int) error {
	// Hold the lock for the specified duration only.
	s.doubleLock.Lock()
	defer s.doubleLock.Unlock()

	s.lock.Lock()
	token := os.GetCurrentProcessAndGogroutineIDStr()
	s.owner.Store(token)
	if expireSeconds <= 0 {
		return nil
	}

	// Lock now, schedule unlock after the TTL. The unlock must verify ownership to avoid unlocking someone else's lock.
	cctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		// This goroutine exits once an unlock is performed.
		select {
		case <-cctx.Done():
			return
		case <-time.After(time.Duration(expireSeconds) * time.Second):
			s.unlock(ctx, token)
		}
	}()
	return nil
}

func (s *SkiplistHashRing) unlock(ctx context.Context, token string) error {
	// Unlock. If already unlocked, return immediately to avoid a fatal error.
	s.doubleLock.Lock()
	defer s.doubleLock.Unlock()
	// If the lock is not ours, bail out.
	owner, _ := s.owner.Load().(string)
	if owner != token {
		return errors.New("not your lock")
	}
	s.owner.Store("")

	// Our lock: cancel the watchdog goroutine.
	if s.cancel != nil {
		s.cancel()
	}

	s.lock.Unlock()
	return nil
}

func (s *SkiplistHashRing) Unlock(ctx context.Context) error {
	token := os.GetCurrentProcessAndGogroutineIDStr()
	return s.unlock(ctx, token)
}

func (s *SkiplistHashRing) Add(ctx context.Context, score int32, nodeID string) error {
	targetNode, ok := s.get(score)
	if ok {
		for _, _nodeID := range targetNode.nodeIDs {
			if _nodeID == nodeID {
				return nil
			}
		}
		targetNode.nodeIDs = append(targetNode.nodeIDs, nodeID)
		return nil
	}

	rLevel := s.roll()
	if len(s.root.nexts) < rLevel+1 {
		difs := make([]*virtualNode, rLevel+1-len(s.root.nexts))
		s.root.nexts = append(s.root.nexts, difs...)
	}

	newNode := virtualNode{
		score:   score,
		nexts:   make([]*virtualNode, rLevel+1),
		nodeIDs: []string{nodeID},
	}

	// Top-down level traversal.
	move := s.root
	for level := rLevel; level >= 0; level-- {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}
		newNode.nexts[level] = move.nexts[level]
		move.nexts[level] = &newNode
	}

	return nil
}

func (s *SkiplistHashRing) Ceiling(ctx context.Context, score int32) (int32, error) {
	target, ok := s.ceiling(score)
	if ok {
		return target, nil
	}

	first, _ := s.first()
	return first, nil
}

func (s *SkiplistHashRing) Floor(ctx context.Context, score int32) (int32, error) {
	target, ok := s.floor(score)
	if ok {
		return target, nil
	}

	last, _ := s.last()
	return last, nil
}

func (s *SkiplistHashRing) Rem(ctx context.Context, score int32, nodeID string) error {
	targetNode, ok := s.get(score)
	if !ok {
		return fmt.Errorf("score: %d not exist", score)
	}

	index := -1
	for i := 0; i < len(targetNode.nodeIDs); i++ {
		if targetNode.nodeIDs[i] == nodeID {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("node: %s not exist in score: %d", nodeID, score)
	}

	delete(s.nodeToDataKey, nodeID)

	if len(targetNode.nodeIDs) > 1 {
		targetNode.nodeIDs = append(targetNode.nodeIDs[:index], targetNode.nodeIDs[index+1:]...)
		return nil
	}

	// Top-down level traversal to unlink the node.
	move := s.root
	for level := len(s.root.nexts) - 1; level >= 0; level-- {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}
		if move.nexts[level] == nil || move.nexts[level].score > score {
			continue
		}
		move.nexts[level] = move.nexts[level].nexts[level]
	}

	for level := 0; level < len(s.root.nexts); level++ {
		if s.root.nexts[level] != nil {
			continue
		}
		s.root.nexts = s.root.nexts[:level]
		break
	}

	return nil
}

func (s *SkiplistHashRing) Nodes(ctx context.Context) (map[string]int, error) {
	return s.nodeToReplicas, nil
}

func (s *SkiplistHashRing) AddNodeToReplica(ctx context.Context, nodeID string, replicas int) error {
	s.nodeToReplicas[nodeID] = replicas
	return nil
}

func (s *SkiplistHashRing) DeleteNodeToReplica(ctx context.Context, nodeID string) error {
	delete(s.nodeToReplicas, nodeID)
	return nil
}

func (s *SkiplistHashRing) Node(ctx context.Context, score int32) ([]string, error) {
	targetNode, ok := s.get(score)
	if !ok {
		return nil, fmt.Errorf("score: %d not exist", score)
	}
	return targetNode.nodeIDs, nil
}

func (s *SkiplistHashRing) DataKeys(ctx context.Context, nodeID string) (map[string]struct{}, error) {
	return s.nodeToDataKey[nodeID], nil
}

func (s *SkiplistHashRing) AddNodeToDataKeys(ctx context.Context, nodeID string, dataKeys map[string]struct{}) error {
	oldDataKeys := s.nodeToDataKey[nodeID]
	if oldDataKeys == nil {
		oldDataKeys = make(map[string]struct{})
	}
	for _dataKey := range dataKeys {
		oldDataKeys[_dataKey] = struct{}{}
	}
	s.nodeToDataKey[nodeID] = oldDataKeys
	return nil
}

func (s *SkiplistHashRing) DeleteNodeToDataKeys(ctx context.Context, nodeID string, dataKeys map[string]struct{}) error {
	oldDataKeys := s.nodeToDataKey[nodeID]
	if oldDataKeys == nil {
		return nil
	}
	for dataKey := range dataKeys {
		delete(oldDataKeys, dataKey)
	}
	if len(oldDataKeys) == 0 {
		delete(s.nodeToDataKey, nodeID)
	}
	return nil
}

func (s *SkiplistHashRing) roll() int {
	rander := rand.New(rand.NewSource(time.Now().UnixNano()))
	var level int
	for rander.Intn(2) == 1 {
		level++
	}
	return level
}

// ceiling returns the smallest score >= the given score.
func (s *SkiplistHashRing) ceiling(score int32) (int32, bool) {
	if len(s.root.nexts) == 0 {
		return -1, false
	}

	move := s.root
	for level := len(s.root.nexts) - 1; level >= 0; level-- {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}
	}

	if move.nexts[0] == nil {
		return -1, false
	}

	return move.nexts[0].score, true
}

func (s *SkiplistHashRing) first() (int32, bool) {
	if len(s.root.nexts) == 0 {
		return -1, false
	}

	return s.root.nexts[0].score, true
}

func (s *SkiplistHashRing) floor(score int32) (int32, bool) {
	if len(s.root.nexts) == 0 {
		return -1, false
	}

	move := s.root
	for level := len(s.root.nexts) - 1; level >= 0; level-- {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}
	}

	if move.nexts[0] != nil && move.nexts[0].score == score {
		return score, true
	}

	if move == s.root {
		return -1, false
	}

	return move.score, true
}

// last returns the largest score in the ring.
func (s *SkiplistHashRing) last() (int32, bool) {
	// Top-down level traversal.
	move := s.root
	for level := len(s.root.nexts) - 1; level >= 0; level-- {
		for move.nexts[level] != nil {
			move = move.nexts[level]
		}
	}

	if move == s.root {
		return -1, false
	}

	return move.score, true
}

func (s *SkiplistHashRing) get(score int32) (*virtualNode, bool) {
	move := s.root
	for level := len(s.root.nexts) - 1; level >= 0; level-- {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}

		if move.nexts[level] != nil && move.nexts[level].score == score {
			return move.nexts[level], true
		}
	}

	return nil, false
}
