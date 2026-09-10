package golsm

import (
	"bytes"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/memtable"
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/sst"
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/wal"
)

// Tree is an lsm tree.
//
//  1. Build a tree from a Config and the on disk file layout.
//  2. Write key value pairs into it.
//  3. Read key value pairs from it.
type Tree struct {
	conf *Config

	// lock guarding data reads and writes
	dataLock sync.RWMutex

	// one read write lock per level of nodes
	levelLocks []sync.RWMutex

	// the active (read write) memtable
	memTable memtable.MemTable

	// read only memtables waiting to be flushed
	rOnlyMemTable []*memTableCompactItem

	// writer of the write ahead log
	walWriter *wal.WALWriter

	// the lsm tree topology, one node slice per level
	nodes [][]*sst.Node

	// channel used to signal that a memtable has reached its threshold and
	// must be flushed to disk
	memCompactC chan *memTableCompactItem

	// channel used to signal that the sst files of a level have reached their
	// size threshold and must be compacted
	levelCompactC chan int

	// channel closed when the lsm tree stops
	stopc chan struct{}

	// channel closed by the compaction goroutine when it exits
	compactDone chan struct{}

	// index of the active memtable. It maps one to one to a wal file.
	memTableIndex int

	// seq of the sstable files of each level. Files are named level_seq.sst.
	levelToSeq []atomic.Int32
}

// NewTree builds an lsm tree.
func NewTree(conf *Config) (*Tree, error) {
	// 1 Build the lsm tree instance.
	t := Tree{
		conf:          conf,
		memCompactC:   make(chan *memTableCompactItem),
		levelCompactC: make(chan int),
		stopc:         make(chan struct{}),
		compactDone:   make(chan struct{}),
		levelToSeq:    make([]atomic.Int32, conf.MaxLevel),
		nodes:         make([][]*sst.Node, conf.MaxLevel),
		levelLocks:    make([]sync.RWMutex, conf.MaxLevel),
	}

	// 2 Restore the whole tree from the sst files.
	if err := t.constructTree(); err != nil {
		return nil, err
	}

	// 3 Start the goroutine responsible for compaction.
	go t.compact()

	// 4 Restore the memtables from the wal files.
	if err := t.constructMemtable(); err != nil {
		// Stop the compaction goroutine and release every resource acquired
		// so far before reporting the failure.
		close(t.stopc)
		<-t.compactDone
		t.closeNodes()
		return nil, err
	}

	// 5 Return the lsm tree instance.
	return &t, nil
}

// Close stops the tree and releases all resources.
func (t *Tree) Close() {
	close(t.stopc)
	// Wait until the compaction goroutine finished its current work, so no
	// node can be inserted anymore while the readers are being closed.
	<-t.compactDone
	t.walWriter.Close()
	t.closeNodes()
}

// closeNodes closes the sst reader of every node of the lsm tree.
func (t *Tree) closeNodes() {
	for i := 0; i < len(t.nodes); i++ {
		for j := 0; j < len(t.nodes[i]); j++ {
			t.nodes[i][j].Close()
		}
	}
}

// Put writes a key value pair into the lsm tree. The pair goes directly into
// the active memtable.
func (t *Tree) Put(key, value []byte) error {
	// 1 Take the write lock.
	t.dataLock.Lock()
	defer t.dataLock.Unlock()

	// 2 Write the data into the write ahead log first, so the memtable data
	// survives a crash.
	if err := t.walWriter.Write(key, value); err != nil {
		return err
	}

	// 3 Write the data into the active skiplist.
	t.memTable.Put(key, value)

	// 4 If the size of the active skiplist has not reached the size threshold
	// of the level 0 sstables, we are done. The size is amplified by 5/4
	// because flushing into an sstable needs some auxiliary metadata.
	if uint64(t.memTable.Size()*5/4) <= t.conf.SSTSize {
		return nil
	}

	// 5 The active skiplist has reached its limit, so switch to a new one.
	return t.refreshMemTableLocked()
}

// Get reads the value of a key.
func (t *Tree) Get(key []byte) ([]byte, bool, error) {
	t.dataLock.RLock()
	// 1 Read the active memtable first.
	value, ok := t.memTable.Get(key)
	if ok {
		t.dataLock.RUnlock()
		return value, true, nil
	}

	// 2 Read the read only memtables. Iterate by index in reverse order: the
	// bigger the index, the later the data was written and the fresher it is.
	for _, v := range slices.Backward(t.rOnlyMemTable) {
		value, ok = v.memTable.Get(key)
		if ok {
			t.dataLock.RUnlock()
			return value, true, nil
		}
	}
	t.dataLock.RUnlock()

	// 3 Read the level 0 sstables. Iterate by index in reverse order: the
	// bigger the index, the later the data was written and the fresher it is.
	var err error
	t.levelLocks[0].RLock()
	for _, v := range slices.Backward(t.nodes[0]) {
		if value, ok, err = v.Get(key); err != nil {
			t.levelLocks[0].RUnlock()
			return nil, false, err
		}
		if ok {
			t.levelLocks[0].RUnlock()
			return value, true, nil
		}
	}
	t.levelLocks[0].RUnlock()

	// 4 Read the sstables of levels 1 ~ i. At most one sstable per level needs
	// to be consulted, because the sstables of these levels hold no duplicated
	// data and are globally ordered.
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

	// 5 No data was read anywhere, so the key does not exist.
	return nil, false, nil
}

// refreshMemTableLocked turns the active memtable into a read only memtable
// and builds a new active one. The data lock must be held by the caller.
func (t *Tree) refreshMemTableLocked() error {
	// Welcome the new memtable first: build a new active memtable together
	// with its matching wal file. If this fails the old memtable and its wal
	// file stay in place, so the data remains safe.
	oldWalFile := t.walFile()
	oldMemTable := t.memTable
	oldWALWriter := t.walWriter
	t.memTableIndex++
	if err := t.newMemTable(); err != nil {
		t.memTableIndex--
		return err
	}

	// Retire the old memtable: close its wal file, turn it into a read only
	// memtable, append it to the slice and hand it over to the compact
	// goroutine through the channel. The compact goroutine is responsible for
	// flushing it into a level 0 sst file.
	oldWALWriter.Close()
	oldItem := memTableCompactItem{
		walFile:  oldWalFile,
		memTable: oldMemTable,
	}
	t.rOnlyMemTable = append(t.rOnlyMemTable, &oldItem)
	go func() {
		select {
		case t.memCompactC <- &oldItem:
		case <-t.stopc:
		}
	}()
	return nil
}

// levelBinarySearch looks up the node of a level whose key range may contain
// the key. The nodes of levels above 0 are sorted and do not overlap.
func (t *Tree) levelBinarySearch(level int, key []byte, start, end int) (*sst.Node, bool) {
	if start > end {
		return nil, false
	}

	mid := start + (end-start)>>1
	if bytes.Compare(t.nodes[level][mid].End(), key) < 0 {
		return t.levelBinarySearch(level, key, mid+1, end)
	}

	if bytes.Compare(t.nodes[level][mid].Start(), key) > 0 {
		return t.levelBinarySearch(level, key, start, mid-1)
	}

	return t.nodes[level][mid], true
}

// newMemTable builds a new active memtable together with its wal file.
func (t *Tree) newMemTable() error {
	walWriter, err := wal.NewWALWriter(t.walFile())
	if err != nil {
		return err
	}
	t.walWriter = walWriter
	t.memTable = t.conf.MemTableConstructor()
	return nil
}
