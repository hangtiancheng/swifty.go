package golsm

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/memtable"
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/sst"
)

type memTableCompactItem struct {
	walFile  string
	memTable memtable.MemTable
}

// compact runs the background compaction goroutine.
func (t *Tree) compact() {
	for {
		select {
		// The stop signal of the lsm tree arrived, exit the goroutine.
		case <-t.stopc:
			return
		// A read only memtable arrived and must be flushed into a level 0
		// sstable file.
		case memCompactItem := <-t.memCompactC:
			t.compactMemTable(memCompactItem)
		// A level compact signal arrived: run the level sorted merge between
		// level and level+1.
		case level := <-t.levelCompactC:
			t.compactLevel(level)
		}
	}
}

// compactLevel runs the sorted merge of a given level.
func (t *Tree) compactLevel(level int) {
	// Pick the nodes of level and level+1 taking part in this merge.
	pickedNodes := t.pickCompactNodes(level)

	// Write into the target sst writer of level+1.
	seq := t.levelToSeq[level+1].Load() + 1
	sstWriter, err := sst.NewSSTWriter(t.sstFile(level+1, seq), t.conf.sstOptions())
	if err != nil {
		return
	}

	// Size limit of each sst file in level+1.
	sstLimit := t.conf.SSTSize * uint64(math.Pow10(level+1))
	// Collect all key value pairs covered by this merge.
	pickedKVs, err := t.pickedNodesToKVs(pickedNodes)
	if err != nil {
		sstWriter.Close()
		return
	}

	// Nothing to merge, so no sstable needs to be created.
	if len(pickedKVs) == 0 {
		sstWriter.Close()
		return
	}

	// Iterate over every key value pair to merge.
	for i := range pickedKVs {
		// If the new level+1 sst file has reached its size limit, flush it to
		// disk and start a new one.
		if sstWriter.Size() > sstLimit {
			// Flush the sst file to disk.
			size, blockToFilter, index := sstWriter.Finish()
			sstWriter.Close()
			// Insert the node matching the sst file into the lsm tree.
			t.insertNode(level+1, seq, size, blockToFilter, index)
			// Build a new level+1 sst writer.
			seq = t.levelToSeq[level+1].Load() + 1
			sstWriter, err = sst.NewSSTWriter(t.sstFile(level+1, seq), t.conf.sstOptions())
			if err != nil {
				return
			}
		}

		// Append the key value pair to the sst writer.
		sstWriter.Append(pickedKVs[i].Key, pickedKVs[i].Value)
		// The last pair is responsible for flushing the sst writer to disk and
		// inserting the matching node into the lsm tree.
		if i == len(pickedKVs)-1 {
			size, blockToFilter, index := sstWriter.Finish()
			sstWriter.Close()
			t.insertNode(level+1, seq, size, blockToFilter, index)
		}
	}

	// Remove the old nodes that have been merged.
	t.removeNodes(level, pickedNodes)

	// Try to trigger the compaction of the next level.
	t.tryTriggerCompact(level + 1)
}

// pickCompactNodes picks all nodes taking part in this round of compaction,
// covering level and level+1.
func (t *Tree) pickCompactNodes(level int) []*sst.Node {
	// Each merge covers the first half of the nodes of the current level.
	startKey := t.nodes[level][0].Start()
	endKey := t.nodes[level][0].End()

	mid := len(t.nodes[level]) >> 1
	if bytes.Compare(t.nodes[level][mid].Start(), startKey) < 0 {
		startKey = t.nodes[level][mid].Start()
	}

	if bytes.Compare(t.nodes[level][mid].End(), endKey) > 0 {
		endKey = t.nodes[level][mid].End()
	}

	var pickedNodes []*sst.Node
	// Merge the nodes of level and level+1 overlapping the [start, end] range.
	for i := level + 1; i >= level; i-- {
		for j := 0; j < len(t.nodes[i]); j++ {
			if bytes.Compare(endKey, t.nodes[i][j].Start()) < 0 || bytes.Compare(startKey, t.nodes[i][j].End()) > 0 {
				continue
			}

			// Append every node whose range overlaps.
			pickedNodes = append(pickedNodes, t.nodes[i][j])
		}
	}

	return pickedNodes
}

// pickedNodesToKVs collects all key value pairs covered by this round of
// compaction. Duplicated keys keep only the freshest value.
func (t *Tree) pickedNodesToKVs(pickedNodes []*sst.Node) ([]*sst.KV, error) {
	// The smaller the index, the older the data. The bigger the index, the
	// fresher the data. So data from bigger indexes overwrites data from
	// smaller ones.
	mergeTable := t.conf.MemTableConstructor()
	for _, node := range pickedNodes {
		kvs, err := node.GetAll()
		if err != nil {
			return nil, err
		}
		for _, kv := range kvs {
			mergeTable.Put(kv.Key, kv.Value)
		}
	}

	// Reuse the memtable to keep the pairs ordered.
	rawKVs := mergeTable.All()
	kvs := make([]*sst.KV, 0, len(rawKVs))
	for _, kv := range rawKVs {
		kvs = append(kvs, &sst.KV{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}

	return kvs, nil
}

// removeNodes removes all old nodes that finished the compaction process.
func (t *Tree) removeNodes(level int, nodes []*sst.Node) {
	// Remove the old nodes from the nodes of the lsm tree.
outer:
	for k := range nodes {
		node := nodes[k]
		for i := level + 1; i >= level; i-- {
			for j := 0; j < len(t.nodes[i]); j++ {
				if node != t.nodes[i][j] {
					continue
				}

				t.levelLocks[i].Lock()
				t.nodes[i] = append(t.nodes[i][:j], t.nodes[i][j+1:]...)
				t.levelLocks[i].Unlock()
				continue outer
			}
		}
	}

	go func() {
		// Destroy the old nodes: close their sst readers and delete the
		// matching sst files from disk.
		for _, node := range nodes {
			node.Destroy()
		}
	}()
}

// compactMemTable flushes a read only memtable into a level 0 sstable file.
func (t *Tree) compactMemTable(memCompactItem *memTableCompactItem) {
	// Flush the memtable:
	// 1 flush the memtable into the level 0 sstables.
	if err := t.flushMemTable(memCompactItem.memTable); err != nil {
		// Keep the wal file so the data can be restored on the next start.
		return
	}

	// 2 recycle the matching memtable from the read only slice.
	t.dataLock.Lock()
	for i := 0; i < len(t.rOnlyMemTable); i++ {
		if t.rOnlyMemTable[i].memTable != memCompactItem.memTable {
			continue
		}
		t.rOnlyMemTable = t.rOnlyMemTable[i+1:]
		break
	}
	t.dataLock.Unlock()

	// 3 delete the matching wal file. Once the memtable is on disk the data is
	// safe and cannot be lost anymore.
	_ = os.Remove(memCompactItem.walFile)
}

// flushMemTable flushes the data of a memtable into a new level 0 sst file.
func (t *Tree) flushMemTable(memTable memtable.MemTable) error {
	kvs := memTable.All()
	// An empty memtable has nothing to flush. Flushing it anyway would create
	// a corrupt sstable without any index entry.
	if len(kvs) == 0 {
		return nil
	}

	// Write the memtable into a level 0 sstable.
	seq := t.levelToSeq[0].Load() + 1

	// Create the sst writer.
	sstWriter, err := sst.NewSSTWriter(t.sstFile(0, seq), t.conf.sstOptions())
	if err != nil {
		return err
	}

	// Iterate over the memtable and write the data into the sst writer.
	for _, kv := range kvs {
		sstWriter.Append(kv.Key, kv.Value)
	}

	// Flush the sstable to disk.
	size, blockToFilter, index := sstWriter.Finish()
	sstWriter.Close()

	// Build the node and add it into the tree.
	t.insertNode(0, seq, size, blockToFilter, index)
	// Try to trigger a round of compaction.
	t.tryTriggerCompact(0)
	return nil
}

func (t *Tree) tryTriggerCompact(level int) {
	// The last level never compacts.
	if level == len(t.nodes)-1 {
		return
	}

	var size uint64
	for _, node := range t.nodes[level] {
		size += node.Size()
	}

	if size <= t.conf.SSTSize*uint64(math.Pow10(level))*uint64(t.conf.SSTNumPerLevel) {
		return
	}

	go func() {
		t.levelCompactC <- level
	}()
}

// insertNodeWithReader inserts a node built from an sst reader into the given
// level.
func (t *Tree) insertNodeWithReader(sstReader *sst.SSTReader, level int, seq int32, size uint64, blockToFilter map[uint64][]byte, index []*sst.Index) {
	file := t.sstFile(level, seq)
	// Record the current seq of the level (monotonically increasing).
	t.levelToSeq[level].Store(seq)

	// Build the new node.
	newNode := sst.NewNode(t.conf.sstOptions(), file, sstReader, level, seq, size, blockToFilter, index)
	// Level 0 only needs to append the node.
	if level == 0 {
		t.levelLocks[0].Lock()
		t.nodes[level] = append(t.nodes[level], newNode)
		t.levelLocks[0].Unlock()
		return
	}

	// Levels 1 ~ level k keep their nodes ordered by key, so insert the node
	// in order.
	for i := 0; i < len(t.nodes[level]); i++ {
		// Walk the nodes from the smallest key to the biggest one and insert
		// the new node before the first node whose start key is bigger than
		// the start key of the new node.
		if bytes.Compare(newNode.Start(), t.nodes[level][i].Start()) < 0 {
			t.levelLocks[level].Lock()
			t.nodes[level] = append(t.nodes[level][:i], append([]*sst.Node{newNode}, t.nodes[level][i:]...)...)
			t.levelLocks[level].Unlock()
			return
		}
	}

	// The new node was not inserted while walking the level, which means it
	// holds the biggest keys of the level, so append it at the end.
	t.levelLocks[level].Lock()
	t.nodes[level] = append(t.nodes[level], newNode)
	t.levelLocks[level].Unlock()
}

func (t *Tree) insertNode(level int, seq int32, size uint64, blockToFilter map[uint64][]byte, index []*sst.Index) {
	file := t.sstFile(level, seq)
	sstReader, err := sst.NewSSTReader(file, t.conf.sstOptions())
	if err != nil {
		return
	}

	t.insertNodeWithReader(sstReader, level, seq, size, blockToFilter, index)
}

func (t *Tree) sstFile(level int, seq int32) string {
	return fmt.Sprintf("%d_%d.sst", level, seq)
}

func (t *Tree) walFile() string {
	return path.Join(t.conf.Dir, "walfile", fmt.Sprintf("%d.wal", t.memTableIndex))
}

func walFileToMemTableIndex(walFile string) int {
	rawIndex := strings.ReplaceAll(walFile, ".wal", "")
	index, _ := strconv.Atoi(rawIndex)
	return index
}
