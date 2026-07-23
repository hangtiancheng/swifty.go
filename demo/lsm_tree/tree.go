package lsm_tree

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/hangtiancheng/swifty.go/demo/lsm_tree/memtable"
	"github.com/hangtiancheng/swifty.go/demo/lsm_tree/wal"
)

// Tree is an lsm tree backed by config and on-disk sst files.
// It supports writing and reading key-value pairs.
type Tree struct {
	conf *Config

	// Lock for read/write operations.
	dataLock sync.RWMutex

	// Per-level read/write locks.
	levelLocks []sync.RWMutex

	// Active read-write memtable.
	memTable memtable.MemTable

	// Read-only memtables awaiting flush.
	rOnlyMemTable []*memTableCompactItem

	// WAL writer.
	walWriter *wal.WALWriter

	// lsm tree node topology (level -> nodes).
	nodes [][]*Node

	// Signals memtable flush when the memtable reaches the size threshold.
	memCompactC chan *memTableCompactItem

	// Signals level compaction when a level reaches the size threshold.
	levelCompactC chan int

	// Signals tree shutdown.
	stopc chan struct{}

	// memtable index; maps 1:1 to the wal file name.
	memTableIndex int

	// Per-level sstable seq counters. sst files are named level_seq.sst.
	levelToSeq []atomic.Int32
}

// NewTree constructs an lsm tree.
func NewTree(conf *Config) (*Tree, error) {
	// 1. Build the tree instance.
	t := Tree{
		conf:          conf,
		memCompactC:   make(chan *memTableCompactItem),
		levelCompactC: make(chan int),
		stopc:         make(chan struct{}),
		levelToSeq:    make([]atomic.Int32, conf.MaxLevel),
		nodes:         make([][]*Node, conf.MaxLevel),
		levelLocks:    make([]sync.RWMutex, conf.MaxLevel),
	}

	// 2. Read sst files and reconstruct the tree.
	if err := t.constructTree(); err != nil {
		return nil, err
	}

	// 3. Start the compaction goroutine.
	go t.compact()

	// 4. Read wal files and reconstruct the memtable.
	if err := t.constructMemtable(); err != nil {
		return nil, err
	}

	// 5. Return the tree.
	return &t, nil
}

func (t *Tree) Close() {
	close(t.stopc)
	for i := 0; i < len(t.nodes); i++ {
		for j := 0; j < len(t.nodes[i]); j++ {
			t.nodes[i][j].Close()
		}
	}
}

// Put writes a key-value pair to the lsm tree. The pair goes directly into the active memtable.
func (t *Tree) Put(key, value []byte) error {
	// 1. Acquire the write lock.
	t.dataLock.Lock()
	defer t.dataLock.Unlock()

	// 2. Write to the WAL first to avoid memtable data loss on crash.
	if err := t.walWriter.Write(key, value); err != nil {
		return err
	}

	// 3. Write to the active memtable.
	t.memTable.Put(key, value)

	// 4. If the memtable has not reached the level-0 sstable threshold, return.
	// Account for sst metadata overhead by using 5/4 of the raw size.
	if uint64(t.memTable.Size()*5/4) <= t.conf.SSTSize {
		return nil
	}

	// 5. Rotate the memtable.
	t.refreshMemTableLocked()
	return nil
}

// Get reads the value for a key.
func (t *Tree) Get(key []byte) ([]byte, bool, error) {
	t.dataLock.RLock()
	// 1. Check the active memtable first.
	value, ok := t.memTable.Get(key)
	if ok {
		t.dataLock.RUnlock()
		return value, true, nil
	}

	// 2. Check read-only memtables in reverse index order (newest first).
	for i := len(t.rOnlyMemTable) - 1; i >= 0; i-- {
		value, ok = t.rOnlyMemTable[i].memTable.Get(key)
		if ok {
			t.dataLock.RUnlock()
			return value, true, nil
		}
	}
	t.dataLock.RUnlock()

	// 3. Check level-0 sstables in reverse index order (newest first).
	var err error
	t.levelLocks[0].RLock()
	for i := len(t.nodes[0]) - 1; i >= 0; i-- {
		if value, ok, err = t.nodes[0][i].Get(key); err != nil {
			t.levelLocks[0].RUnlock()
			return nil, false, err
		}
		if ok {
			t.levelLocks[0].RUnlock()
			return value, true, nil
		}
	}
	t.levelLocks[0].RUnlock()

	// 4. Check levels 1..N. Each level requires at most one sstable interaction because
	// these levels are sorted and non-overlapping.
	for level := 1; level < len(t.nodes); level++ {
		t.levelLocks[level].RLock()
		node, ok := t.levelBinarySearch(level, key, 0, len(t.nodes[level])-1)
		if !ok {
			t.levelLocks[level].RUnlock()
			continue
		}
		if value, ok, err = node.Get(key); err != nil {
			t.levelLocks[level].RUnlock()
			return nil, false, err
		}
		if ok {
			t.levelLocks[level].RUnlock()
			return value, true, nil
		}
		t.levelLocks[level].RUnlock()
	}

	// 5. Key not found.
	return nil, false, nil
}

// refreshMemTableLocked rotates the active memtable to read-only and builds a new active memtable.
func (t *Tree) refreshMemTableLocked() {
	// Rotate: move the active memtable to the read-only slice and send it to the compaction goroutine.
	oldItem := memTableCompactItem{
		walFile:  t.walFile(),
		memTable: t.memTable,
	}
	t.rOnlyMemTable = append(t.rOnlyMemTable, &oldItem)
	t.walWriter.Close()
	go func() {
		t.memCompactC <- &oldItem
	}()

	// Build a new active memtable and its WAL.
	t.memTableIndex++
	t.newMemTable()
}

func (t *Tree) levelBinarySearch(level int, key []byte, start, end int) (*Node, bool) {
	if start > end {
		return nil, false
	}

	mid := start + (end-start)>>1
	if bytes.Compare(t.nodes[level][start].endKey, key) < 0 {
		return t.levelBinarySearch(level, key, mid+1, end)
	}

	if bytes.Compare(t.nodes[level][start].startKey, key) > 0 {
		return t.levelBinarySearch(level, key, start, mid-1)
	}

	return t.nodes[level][mid], true
}

func (t *Tree) newMemTable() {
	t.walWriter, _ = wal.NewWALWriter(t.walFile())
	t.memTable = t.conf.MemTableConstructor()
}
