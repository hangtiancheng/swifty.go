package sst

import (
	"bytes"
	"encoding/binary"
	"os"
	"path"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/util"
)

// Index is used to locate a block inside an sstable quickly.
type Index struct {
	Key             []byte // key of the index. It is >= the largest key of the previous block and < the smallest key of the next block
	PrevBlockOffset uint64 // offset of the previous block inside the sstable
	PrevBlockSize   uint64 // size of the previous block, in bytes
}

// SSTWriter writes one sstable of the lsm tree.
type SSTWriter struct {
	opts          *Options          // sst layer options
	dest          *os.File          // the sstable file
	file          string            // final name of the sstable file, without the directory path
	dataBuf       *bytes.Buffer     // data block buffer: key -> value
	filterBuf     *bytes.Buffer     // filter block buffer: previous block offset -> filter bitmap
	indexBuf      *bytes.Buffer     // index block buffer: index key -> previous block offset, previous block size
	blockToFilter map[uint64][]byte // previous block offset -> filter bitmap
	index         []*Index          // index key -> previous block offset, previous block size

	dataBlock     *Block   // data block
	filterBlock   *Block   // filter block
	indexBlock    *Block   // index block
	assistScratch [20]byte // scratch buffer used while writing the index block

	prevKey         []byte // key of the most recently written record
	prevBlockOffset uint64 // starting offset of the previous data block
	prevBlockSize   uint64 // size of the previous data block
}

// NewSSTWriter creates an sst writer. The data goes into a temporary file
// first and is published under the final name by Finish, so a crash in the
// middle of a write never leaves a partial sstable behind that would fail to
// load on the next start.
func NewSSTWriter(file string, opts *Options) (*SSTWriter, error) {
	// Open the temporary file, truncating a stale one left behind by a crash.
	dest, err := os.OpenFile(path.Join(opts.Dir, file+".tmp"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	return &SSTWriter{
		opts:          opts,
		dest:          dest,
		file:          file,
		dataBuf:       bytes.NewBuffer([]byte{}),
		filterBuf:     bytes.NewBuffer([]byte{}),
		indexBuf:      bytes.NewBuffer([]byte{}),
		blockToFilter: make(map[uint64][]byte),
		dataBlock:     NewBlock(),
		filterBlock:   NewBlock(),
		indexBlock:    NewBlock(),
		prevKey:       []byte{},
	}, nil
}

// Finish completes the whole sstable: it flushes the buffered data to disk,
// publishes the file under its final name and returns the information the
// upper lsm layer needs to cache.
func (s *SSTWriter) Finish() (size uint64, blockToFilter map[uint64][]byte, index []*Index, err error) {
	// Handle the last data block.
	s.refreshBlock()
	// Complete the last index entry.
	s.insertIndex(s.prevKey)

	// Write the filter block into its buffer.
	_, _ = s.filterBlock.FlushTo(s.filterBuf)
	// Write the index block into its buffer.
	_, _ = s.indexBlock.FlushTo(s.indexBuf)

	// Build the footer, recording the offset and the size of the filter block
	// and of the index block.
	footer := make([]byte, s.opts.SSTFooterSize)
	size = uint64(s.dataBuf.Len())
	n := binary.PutUvarint(footer[0:], size)
	filterBufLen := uint64(s.filterBuf.Len())
	n += binary.PutUvarint(footer[n:], filterBufLen)
	size += filterBufLen
	n += binary.PutUvarint(footer[n:], size)
	indexBufLen := uint64(s.indexBuf.Len())
	binary.PutUvarint(footer[n:], indexBufLen)
	size += indexBufLen

	// Write everything to the file in order.
	if _, err = s.dest.Write(s.dataBuf.Bytes()); err != nil {
		return
	}
	if _, err = s.dest.Write(s.filterBuf.Bytes()); err != nil {
		return
	}
	if _, err = s.dest.Write(s.indexBuf.Bytes()); err != nil {
		return
	}
	if _, err = s.dest.Write(footer); err != nil {
		return
	}

	// Publish the sstable under its final name.
	if err = os.Rename(path.Join(s.opts.Dir, s.file+".tmp"), path.Join(s.opts.Dir, s.file)); err != nil {
		return
	}

	blockToFilter = s.blockToFilter
	index = s.index
	return
}

// Append adds one record to the sstable.
func (s *SSTWriter) Append(key, value []byte) {
	// A new data block needs an index entry.
	if s.dataBlock.entriesCnt == 0 {
		s.insertIndex(key)
	}

	// Write the record into the data block.
	s.dataBlock.Append(key, value)
	// Add the key to the bloom filter of the block.
	s.opts.Filter.Add(key)
	// Track the most recent key.
	s.prevKey = key

	// If the data block reached its size limit, move it into the buffer and
	// start a new one.
	if s.dataBlock.Size() >= s.opts.SSTDataBlockSize {
		s.refreshBlock()
	}
}

// Size returns the amount of data buffered for the data blocks, in bytes.
func (s *SSTWriter) Size() uint64 {
	return uint64(s.dataBuf.Len())
}

// Close closes the sstable file and releases the buffers. If the sstable was
// never finished its temporary file is removed; after a successful Finish the
// temporary file no longer exists and the removal fails silently.
func (s *SSTWriter) Close() {
	_ = s.dest.Close()
	s.dataBuf.Reset()
	s.indexBuf.Reset()
	s.filterBuf.Reset()
	_ = os.Remove(path.Join(s.opts.Dir, s.file+".tmp"))
}

func (s *SSTWriter) insertIndex(key []byte) {
	// Build the key of the index entry.
	indexKey := util.GetSeparatorBetween(s.prevKey, key)
	n := binary.PutUvarint(s.assistScratch[0:], s.prevBlockOffset)
	n += binary.PutUvarint(s.assistScratch[n:], s.prevBlockSize)

	s.indexBlock.Append(indexKey, s.assistScratch[:n])
	s.index = append(s.index, &Index{
		Key:             indexKey,
		PrevBlockOffset: s.prevBlockOffset,
		PrevBlockSize:   s.prevBlockSize,
	})
}

func (s *SSTWriter) refreshBlock() {
	// An empty block has nothing to flush and contributes no index entry.
	if s.dataBlock.entriesCnt == 0 {
		return
	}

	// The block being flushed starts at the current end of the data buffer.
	s.prevBlockOffset = uint64(s.dataBuf.Len())
	// Move the data of the block into the buffer.
	s.prevBlockSize, _ = s.dataBlock.FlushTo(s.dataBuf)

	// A filter holding no keys contributes no bitmap. The data of the block
	// is flushed either way, otherwise its records would be lost.
	if s.opts.Filter.KeyLen() == 0 {
		return
	}

	// Store the bloom filter bitmap.
	filterBitmap := s.opts.Filter.Hash()
	s.blockToFilter[s.prevBlockOffset] = filterBitmap
	n := binary.PutUvarint(s.assistScratch[0:], s.prevBlockOffset)
	s.filterBlock.Append(s.assistScratch[:n], filterBitmap)
	// Reset the bloom filter.
	s.opts.Filter.Reset()
}
