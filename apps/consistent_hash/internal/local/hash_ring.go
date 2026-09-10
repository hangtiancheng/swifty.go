package local

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/consistent_hash/internal/osutil"
)

// SkiplistHashRing is an in-memory HashRing backed by a skiplist of virtual
// nodes. It also owns the data-key bookkeeping of the ring.
type SkiplistHashRing struct {
	LockEntity
	root *virtualNode
	// nodeToReplicas maps every registered node id to its number of virtual
	// nodes; the entry also marks the node as registered.
	nodeToReplicas map[string]int
	// nodeToDataKey maps every registered node id to the set of data keys
	// assigned to it.
	nodeToDataKey map[string]map[string]struct{}
}

// LockEntity is the lock of the hash ring. Unlock is idempotent: unlocking an
// already released or expired lock is reported as an error instead of crashing
// on a double mutex unlock.
type LockEntity struct {
	lock sync.Mutex
	// doubleLock guards the unlock path so that an expired lock released by
	// the guardian goroutine and a concurrent explicit unlock stay exclusive.
	doubleLock sync.Mutex
	cancel     context.CancelFunc
	owner      atomic.Value
}

// NewSkiplistHashRing creates an empty local hash ring.
func NewSkiplistHashRing() *SkiplistHashRing {
	return &SkiplistHashRing{
		root:           &virtualNode{},
		nodeToReplicas: make(map[string]int),
		nodeToDataKey:  make(map[string]map[string]struct{}),
	}
}

type virtualNode struct {
	score int32
	// nodeIDs holds the raw virtual node keys registered under the score.
	nodeIDs []string
	nexts   []*virtualNode
}

// Lock acquires the ring lock. When expireSeconds is positive, a guardian
// goroutine releases the lock automatically once the expiry elapsed, so the
// lock is never held longer than configured.
//
// The mutex is taken before doubleLock: a goroutine waiting for a held lock
// must not hold doubleLock, because the holder's unlock path needs it too.
// doubleLock only guards the short owner/cancel bookkeeping window.
func (s *SkiplistHashRing) Lock(ctx context.Context, expireSeconds int) error {
	s.lock.Lock()

	s.doubleLock.Lock()
	defer s.doubleLock.Unlock()

	token := osutil.GetCurrentProcessAndGoroutineIDStr()
	s.owner.Store(token)
	if expireSeconds <= 0 {
		return nil
	}

	// The lock was acquired; schedule its automatic release. The guardian
	// only unlocks when the owner is still itself, so it never releases a
	// lock that already changed hands.
	cctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		// The guardian exits as soon as the lock is unlocked explicitly.
		select {
		case <-cctx.Done():
			return
		case <-time.After(time.Duration(expireSeconds) * time.Second):
			_ = s.unlock(ctx, token)
		}
	}()
	return nil
}

// unlock releases the ring lock on behalf of the given token. Unlocking a lock
// that is not owned (anymore) fails instead of panicking.
func (s *SkiplistHashRing) unlock(_ context.Context, token string) error {
	s.doubleLock.Lock()
	defer s.doubleLock.Unlock()

	owner, _ := s.owner.Load().(string)
	if owner != token {
		return errors.New("not your lock")
	}
	s.owner.Store("")

	// Stop the guardian goroutine of this lock before releasing the mutex.
	if s.cancel != nil {
		s.cancel()
	}

	s.lock.Unlock()
	return nil
}

// Unlock releases the ring lock acquired by the current goroutine.
func (s *SkiplistHashRing) Unlock(ctx context.Context) error {
	token := osutil.GetCurrentProcessAndGoroutineIDStr()
	return s.unlock(ctx, token)
}

// Add registers a raw virtual node key under the given score. When the score
// already exists, the key joins the existing node list.
func (s *SkiplistHashRing) Add(_ context.Context, score int32, nodeID string) error {
	targetNode, ok := s.get(score)
	if ok {
		if slices.Contains(targetNode.nodeIDs, nodeID) {
			return nil
		}
		targetNode.nodeIDs = append(targetNode.nodeIDs, nodeID)
		return nil
	}

	rLevel := s.roll()
	if len(s.root.nexts) < rLevel+1 {
		grown := make([]*virtualNode, rLevel+1-len(s.root.nexts))
		s.root.nexts = append(s.root.nexts, grown...)
	}

	newNode := virtualNode{
		score:   score,
		nexts:   make([]*virtualNode, rLevel+1),
		nodeIDs: []string{nodeID},
	}

	// Link the new node from the top level down to level 0.
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

// Ceiling returns the smallest score >= score, wrapping around to the first
// score of the ring when the requested score is beyond the last one. It
// returns -1 when the ring is empty.
func (s *SkiplistHashRing) Ceiling(_ context.Context, score int32) (int32, error) {
	target, ok := s.ceiling(score)
	if ok {
		return target, nil
	}

	first, _ := s.first()
	return first, nil
}

// Floor returns the largest score <= score, wrapping around to the last score
// of the ring when the requested score is before the first one. It returns -1
// when the ring is empty.
func (s *SkiplistHashRing) Floor(_ context.Context, score int32) (int32, error) {
	target, ok := s.floor(score)
	if ok {
		return target, nil
	}

	last, _ := s.last()
	return last, nil
}

// Rem removes a raw virtual node key from the given score. When it is the last
// key under the score, the whole virtual node is unlinked from the skiplist.
func (s *SkiplistHashRing) Rem(_ context.Context, score int32, nodeID string) error {
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

	if len(targetNode.nodeIDs) > 1 {
		targetNode.nodeIDs = append(targetNode.nodeIDs[:index], targetNode.nodeIDs[index+1:]...)
		return nil
	}

	// Unlink the node from every level, top level first.
	move := s.root
	for level := range slices.Backward(s.root.nexts) {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}
		if move.nexts[level] == nil || move.nexts[level].score > score {
			continue
		}
		move.nexts[level] = move.nexts[level].nexts[level]
	}

	// Shrink the root when top levels became empty.
	for level := 0; level < len(s.root.nexts); level++ {
		if s.root.nexts[level] != nil {
			continue
		}
		s.root.nexts = s.root.nexts[:level]
		break
	}

	return nil
}

// Nodes returns a copy of the mapping from node id to virtual node count.
func (s *SkiplistHashRing) Nodes(_ context.Context) (map[string]int, error) {
	nodes := make(map[string]int, len(s.nodeToReplicas))
	maps.Copy(nodes, s.nodeToReplicas)
	return nodes, nil
}

// AddNodeToReplica records the number of virtual nodes of a node, marking the
// node as registered on the ring.
func (s *SkiplistHashRing) AddNodeToReplica(_ context.Context, nodeID string, replicas int) error {
	s.nodeToReplicas[nodeID] = replicas
	return nil
}

// DeleteNodeToReplica drops the registration of a node.
func (s *SkiplistHashRing) DeleteNodeToReplica(_ context.Context, nodeID string) error {
	delete(s.nodeToReplicas, nodeID)
	return nil
}

// Node returns a copy of the raw virtual node keys registered under the given
// score.
func (s *SkiplistHashRing) Node(_ context.Context, score int32) ([]string, error) {
	targetNode, ok := s.get(score)
	if !ok {
		return nil, fmt.Errorf("score: %d not exist", score)
	}
	nodeIDs := make([]string, len(targetNode.nodeIDs))
	copy(nodeIDs, targetNode.nodeIDs)
	return nodeIDs, nil
}

// DataKeys returns a copy of the data keys currently assigned to the node.
func (s *SkiplistHashRing) DataKeys(_ context.Context, nodeID string) (map[string]struct{}, error) {
	dataKeys := make(map[string]struct{}, len(s.nodeToDataKey[nodeID]))
	for dataKey := range s.nodeToDataKey[nodeID] {
		dataKeys[dataKey] = struct{}{}
	}
	return dataKeys, nil
}

// AddNodeToDataKeys assigns data keys to the node.
func (s *SkiplistHashRing) AddNodeToDataKeys(_ context.Context, nodeID string, dataKeys map[string]struct{}) error {
	oldDataKeys := s.nodeToDataKey[nodeID]
	if oldDataKeys == nil {
		oldDataKeys = make(map[string]struct{})
	}
	for dataKey := range dataKeys {
		oldDataKeys[dataKey] = struct{}{}
	}
	s.nodeToDataKey[nodeID] = oldDataKeys
	return nil
}

// DeleteNodeToDataKeys removes data keys from the node and drops the node
// entry entirely when no key is left.
func (s *SkiplistHashRing) DeleteNodeToDataKeys(_ context.Context, nodeID string, dataKeys map[string]struct{}) error {
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

// roll randomly draws the level of a new skiplist node (a geometric
// distribution with p = 0.5).
func (s *SkiplistHashRing) roll() int {
	var level int
	for rand.IntN(2) == 1 {
		level++
	}
	return level
}

// ceiling finds the smallest score >= score.
func (s *SkiplistHashRing) ceiling(score int32) (int32, bool) {
	if len(s.root.nexts) == 0 {
		return -1, false
	}

	move := s.root
	for level := range slices.Backward(s.root.nexts) {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}
	}

	if move.nexts[0] == nil {
		return -1, false
	}

	return move.nexts[0].score, true
}

// first returns the smallest score of the ring.
func (s *SkiplistHashRing) first() (int32, bool) {
	if len(s.root.nexts) == 0 {
		return -1, false
	}

	return s.root.nexts[0].score, true
}

// floor finds the largest score <= score.
func (s *SkiplistHashRing) floor(score int32) (int32, bool) {
	if len(s.root.nexts) == 0 {
		return -1, false
	}

	move := s.root
	for level := range slices.Backward(s.root.nexts) {
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

// last returns the largest score of the ring.
func (s *SkiplistHashRing) last() (int32, bool) {
	move := s.root
	for level := range slices.Backward(s.root.nexts) {
		for move.nexts[level] != nil {
			move = move.nexts[level]
		}
	}

	if move == s.root {
		return -1, false
	}

	return move.score, true
}

// get finds the virtual node registered under the given score.
func (s *SkiplistHashRing) get(score int32) (*virtualNode, bool) {
	move := s.root
	for level := range slices.Backward(s.root.nexts) {
		for move.nexts[level] != nil && move.nexts[level].score < score {
			move = move.nexts[level]
		}

		if move.nexts[level] != nil && move.nexts[level].score == score {
			return move.nexts[level], true
		}
	}

	return nil, false
}
