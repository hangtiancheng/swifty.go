package memtable

import (
	"bytes"
	"math/rand"
	"time"
)

// Skiplist is an ordered skiplist. It takes no locks and is not safe for
// concurrent use.
type Skiplist struct {
	head       *skipNode  // head node of the skiplist
	entriesCnt int        // number of key value pairs in the skiplist
	size       int        // size of the data held by the skiplist, in bytes
	rand       *rand.Rand // random source used to roll node heights
}

// skipNode is a node of the skiplist.
type skipNode struct {
	nexts      []*skipNode // multi level next pointers forming the tower of the node
	key, value []byte      // key value pair stored in the node
}

// NewSkiplist creates a skiplist instance.
func NewSkiplist() MemTable {
	return &Skiplist{
		head: &skipNode{}, // the head node must be initialized
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Put writes a key value pair into the skiplist. If the key does not exist it
// is inserted; if the key already exists the value is overwritten.
func (s *Skiplist) Put(key, value []byte) {
	// If the key already exists.
	if node := s.getNode(key); node != nil {
		// Adjust the data size by the difference between the new and the old value.
		s.size += len(value) - len(node.value)
		// Overwrite the value.
		node.value = value
		return
	}

	// The key does not exist, so this is an insertion. Add both the key and
	// the value size to the skiplist size.
	s.size += len(key) + len(value)
	s.entriesCnt++
	// Roll the height of the new node.
	newNodeHeight := s.roll()

	// If the skiplist is not tall enough, grow its height.
	if len(s.head.nexts) < newNodeHeight {
		pad := make([]*skipNode, newNodeHeight-len(s.head.nexts))
		s.head.nexts = append(s.head.nexts, pad...)
	}

	// Build the new node.
	newNode := skipNode{
		nexts: make([]*skipNode, newNodeHeight),
		key:   key,
		value: value,
	}

	// Walk the levels from high to low and insert the node in order on every level.
	move := s.head
	for level := newNodeHeight - 1; level >= 0; level-- {
		// Keep moving right on the level until the next node is missing or its key is greater.
		for move.nexts[level] != nil && bytes.Compare(move.nexts[level].key, key) < 0 {
			move = move.nexts[level]
		}

		// Insert the node.
		newNode.nexts[level] = move.nexts[level]
		move.nexts[level] = &newNode
	}
}

// Get reads a key value pair from the skiplist.
func (s *Skiplist) Get(key []byte) ([]byte, bool) {
	// If the key exists, return the matching value.
	if node := s.getNode(key); node != nil {
		return node.value, true
	}

	return nil, false
}

// All returns all key value pairs held by the skiplist.
func (s *Skiplist) All() []*KV {
	if len(s.head.nexts) == 0 {
		return nil
	}

	kvs := make([]*KV, 0, s.entriesCnt)
	// Walk the bottom level from left to right.
	for move := s.head; move.nexts[0] != nil; move = move.nexts[0] {
		kvs = append(kvs, &KV{
			Key:   move.nexts[0].key,
			Value: move.nexts[0].value,
		})
	}

	return kvs
}

// Size returns the size of the data held by the skiplist, in bytes.
func (s *Skiplist) Size() int {
	return s.size
}

// EntriesCnt returns the number of key value pairs in the skiplist.
func (s *Skiplist) EntriesCnt() int {
	return s.entriesCnt
}

// getNode returns the node matching the key, or nil if the key does not exist.
func (s *Skiplist) getNode(key []byte) *skipNode {
	move := s.head
	// Walk the levels from high to low.
	for level := len(s.head.nexts) - 1; level >= 0; level-- {
		// Keep moving right until the next node is missing or its key is not
		// smaller than the key being searched for.
		for move.nexts[level] != nil && bytes.Compare(move.nexts[level].key, key) < 0 {
			move = move.nexts[level]
		}
		// If the next node matches the key the target is found; otherwise go
		// down one level.
		if move.nexts[level] != nil && bytes.Equal(move.nexts[level].key, key) {
			return move.nexts[level]
		}
	}

	return nil
}

// roll rolls the height of a new node. The minimum height is 1 and every
// additional level halves the probability.
func (s *Skiplist) roll() int {
	var level int
	for s.rand.Intn(2) == 1 {
		level++
	}
	return level + 1
}
