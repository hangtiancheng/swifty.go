package sst

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/util"
)

// Block is a data block inside an sstable. Blocks map one to one to index
// entries and filter bitmaps.
type Block struct {
	buffer     [30]byte      // scratch buffer used to stage varints
	record     *bytes.Buffer // buffer holding the staged data
	entriesCnt int           // number of key value pairs in the block
	prevKey    []byte        // key of the most recently written record
}

// NewBlock creates a data block.
func NewBlock() *Block {
	return &Block{
		record: bytes.NewBuffer([]byte{}),
	}
}

// Append adds a key value pair to the block.
func (b *Block) Append(key, value []byte) {
	// Always runs: set prevKey to the key just written and count the entry.
	defer func() {
		b.prevKey = append(b.prevKey[:0], key...)
		b.entriesCnt++
	}()

	// Length of the key prefix shared with the previous key.
	sharedPrefixLen := util.SharedPrefixLen(b.prevKey, key)

	// Stage sharedPrefixLen || remainingKeyLen || valueLen.
	n := binary.PutUvarint(b.buffer[0:], uint64(sharedPrefixLen))
	n += binary.PutUvarint(b.buffer[n:], uint64(len(key)-sharedPrefixLen))
	n += binary.PutUvarint(b.buffer[n:], uint64(len(value)))

	// Write sharedPrefixLen || remainingKeyLen || valueLen into the record buffer.
	_, _ = b.record.Write(b.buffer[:n])
	// Write the remaining key part and the value into the record buffer.
	b.record.Write(key[sharedPrefixLen:])
	b.record.Write(value)
}

// Size returns the size of the block, in bytes.
func (b *Block) Size() int {
	return b.record.Len()
}

// FlushTo writes the data of the block into the dest writer.
func (b *Block) FlushTo(dest io.Writer) (uint64, error) {
	defer b.clear()
	n, err := dest.Write(b.ToBytes())
	return uint64(n), err
}

// ToBytes returns the data of the block as a byte slice.
func (b *Block) ToBytes() []byte {
	return b.record.Bytes()
}

// clear removes the data held by the block.
func (b *Block) clear() {
	b.entriesCnt = 0
	b.prevKey = b.prevKey[:0]
	b.record.Reset()
}
