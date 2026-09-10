package sst

import (
	"bytes"
	"os"
	"path"
)

// Node is a node of the lsm tree. It corresponds to one sstable.
type Node struct {
	opts          *Options          // sst layer options
	file          string            // sstable file name, without the directory path
	level         int               // level the sstable belongs to
	seq           int32             // seq of the sstable, the seq part of its file name level_seq.sst
	size          uint64            // size of the sstable, in bytes
	blockToFilter map[uint64][]byte // filter bitmap of each block
	index         []*Index          // index entry of each block
	startKey      []byte            // smallest key in the sstable
	endKey        []byte            // largest key in the sstable
	sstReader     *SSTReader        // entry point used to read the sst file
}

// NewNode creates a node from an sst reader.
func NewNode(opts *Options, file string, sstReader *SSTReader, level int, seq int32, size uint64, blockToFilter map[uint64][]byte, index []*Index) *Node {
	return &Node{
		opts:          opts,
		file:          file,
		sstReader:     sstReader,
		level:         level,
		seq:           seq,
		size:          size,
		blockToFilter: blockToFilter,
		index:         index,
		startKey:      index[0].Key,
		endKey:        index[len(index)-1].Key,
	}
}

// GetAll returns all key value pairs stored in the sstable.
func (n *Node) GetAll() ([]*KV, error) {
	return n.sstReader.ReadData()
}

// Get reads the value of the key from the sstable.
func (n *Node) Get(key []byte) ([]byte, bool, error) {
	// Locate the block the key may belong to through the index.
	index, ok := n.binarySearchIndex(key, 0, len(n.index)-1)
	if !ok {
		return nil, false, nil
	}

	// Let the bloom filter help decide whether the key exists.
	bitmap := n.blockToFilter[index.PrevBlockOffset]
	if ok = n.opts.Filter.Exist(bitmap, key); !ok {
		return nil, false, nil
	}

	// Read the matching block.
	block, err := n.sstReader.ReadBlock(index.PrevBlockOffset, index.PrevBlockSize)
	if err != nil {
		return nil, false, err
	}

	// Parse the block data into key value pairs.
	kvs, err := n.sstReader.ReadBlockData(block)
	if err != nil {
		return nil, false, err
	}

	for _, kv := range kvs {
		if bytes.Equal(kv.Key, key) {
			return kv.Value, true, nil
		}
	}

	return nil, false, nil
}

// Size returns the size of the sstable, in bytes.
func (n *Node) Size() uint64 {
	return n.size
}

// Start returns the smallest key in the sstable.
func (n *Node) Start() []byte {
	return n.startKey
}

// End returns the largest key in the sstable.
func (n *Node) End() []byte {
	return n.endKey
}

// Index returns the level and the seq of the sstable.
func (n *Node) Index() (level int, seq int32) {
	level, seq = n.level, n.seq
	return
}

// Destroy closes the sst reader and deletes the sst file from disk.
func (n *Node) Destroy() {
	n.sstReader.Close()
	_ = os.Remove(path.Join(n.opts.Dir, n.file))
}

// Close closes the sst reader.
func (n *Node) Close() {
	n.sstReader.Close()
}

// binarySearchIndex searches for the block index the key may belong to.
func (n *Node) binarySearchIndex(key []byte, start, end int) (*Index, bool) {
	if start == end {
		return n.index[start], bytes.Compare(n.index[start].Key, key) >= 0
	}

	// The target block guarantees key <= index[i].key && key > index[i-1].key.
	mid := start + (end-start)>>1
	if bytes.Compare(n.index[mid].Key, key) < 0 {
		return n.binarySearchIndex(key, mid+1, end)
	}

	return n.binarySearchIndex(key, start, mid)
}
