package sst

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path"
	"sync"
)

// KV is a single key value pair.
type KV struct {
	Key   []byte
	Value []byte
}

// SSTReader reads one sstable of the lsm tree.
type SSTReader struct {
	opts         *Options      // sst layer options
	src          *os.File      // the sstable file
	reader       *bufio.Reader // buffered reader wrapping the file
	filterOffset uint64        // offset of the filter block inside the sstable
	filterSize   uint64        // size of the filter block, in bytes
	indexOffset  uint64        // offset of the index block inside the sstable
	indexSize    uint64        // size of the index block, in bytes

	// mu guards src and reader. Both reads and compaction walk the same file
	// through the same reader, and every read is a seek followed by a read, so
	// the sequences must not interleave.
	mu sync.Mutex
}

// NewSSTReader creates an sst reader.
func NewSSTReader(file string, opts *Options) (*SSTReader, error) {
	src, err := os.OpenFile(path.Join(opts.Dir, file), os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &SSTReader{
		opts:   opts,
		src:    src,
		reader: bufio.NewReader(src),
	}, nil
}

// Size returns the size of the sstable data, in bytes.
func (s *SSTReader) Size() (uint64, error) {
	if s.indexOffset == 0 {
		if err := s.ReadFooter(); err != nil {
			return 0, err
		}
	}
	return s.indexOffset + s.indexSize, nil
}

// Close closes the sstable file.
func (s *SSTReader) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reader.Reset(s.src)
	_ = s.src.Close()
}

// ReadFooter reads the sstable footer into the fields of the reader.
func (s *SSTReader) ReadFooter() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Seek backwards from the end of the file by the sst footer size.
	if _, err := s.src.Seek(-int64(s.opts.SSTFooterSize), io.SeekEnd); err != nil {
		return err
	}

	s.reader.Reset(s.src)

	var err error
	if s.filterOffset, err = binary.ReadUvarint(s.reader); err != nil {
		return err
	}

	if s.filterSize, err = binary.ReadUvarint(s.reader); err != nil {
		return err
	}

	if s.indexOffset, err = binary.ReadUvarint(s.reader); err != nil {
		return err
	}

	if s.indexSize, err = binary.ReadUvarint(s.reader); err != nil {
		return err
	}

	return nil
}

// ReadFilter reads the filter block.
func (s *SSTReader) ReadFilter() (map[uint64][]byte, error) {
	// If the footer has not been read yet, load it first.
	if s.filterOffset == 0 || s.filterSize == 0 {
		if err := s.ReadFooter(); err != nil {
			return nil, err
		}
	}

	// Read the content of the filter block.
	filterBlock, err := s.ReadBlock(s.filterOffset, s.filterSize)
	if err != nil {
		return nil, err
	}

	// Parse the content of the filter block.
	return s.readFilter(filterBlock)
}

// ReadIndex reads the index block.
func (s *SSTReader) ReadIndex() ([]*Index, error) {
	// If the footer has not been read yet, load it first.
	if s.indexOffset == 0 || s.indexSize == 0 {
		if err := s.ReadFooter(); err != nil {
			return nil, err
		}
	}

	// Read the content of the index block.
	indexBlock, err := s.ReadBlock(s.indexOffset, s.indexSize)
	if err != nil {
		return nil, err
	}

	// Parse the content of the index block.
	return s.readIndex(indexBlock)
}

// ReadData reads all key value pairs stored in the sstable.
func (s *SSTReader) ReadData() ([]*KV, error) {
	// If the footer has not been read yet, load it first.
	if s.indexOffset == 0 || s.indexSize == 0 || s.filterOffset == 0 || s.filterSize == 0 {
		if err := s.ReadFooter(); err != nil {
			return nil, err
		}
	}

	// Read the content of all data blocks.
	dataBlock, err := s.ReadBlock(0, s.filterOffset)
	if err != nil {
		return nil, err
	}

	// Parse the content of all data blocks.
	return s.ReadBlockData(dataBlock)
}

// ReadBlock reads the content of one block.
func (s *SSTReader) ReadBlock(offset, size uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Seek to the starting offset of the block.
	if _, err := s.src.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, err
	}
	s.reader.Reset(s.src)

	// Read the requested size.
	buf := make([]byte, size)
	_, err := io.ReadFull(s.reader, buf)
	return buf, err
}

// readFilter parses the content of the filter block.
func (s *SSTReader) readFilter(block []byte) (map[uint64][]byte, error) {
	blockToFilter := make(map[uint64][]byte)
	// Wrap the content of the filter block into a buffer.
	buf := bytes.NewBuffer(block)
	var prevKey []byte
	for {
		// Each record is a block filter entry: the key is the offset of the
		// block and the value is the filter bitmap.
		key, value, err := s.ReadRecord(prevKey, buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		blockOffset, _ := binary.Uvarint(key)
		blockToFilter[blockOffset] = value
		prevKey = key
	}

	return blockToFilter, nil
}

// readIndex parses the content of the index block.
func (s *SSTReader) readIndex(block []byte) ([]*Index, error) {
	var (
		index   []*Index
		prevKey []byte
	)

	// Wrap the content of the index block into a buffer.
	buf := bytes.NewBuffer(block)
	for {
		// Each record is an index entry: the key is the separator key between
		// two blocks and the value is the offset and the size of the previous
		// block.
		key, value, err := s.ReadRecord(prevKey, buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		blockOffset, n := binary.Uvarint(value)
		blockSize, _ := binary.Uvarint(value[n:])
		index = append(index, &Index{
			Key:             key,
			PrevBlockOffset: blockOffset,
			PrevBlockSize:   blockSize,
		})

		prevKey = key
	}
	return index, nil
}

// ReadBlockData parses the content of one data block.
func (s *SSTReader) ReadBlockData(block []byte) ([]*KV, error) {
	// The previous key must be tracked temporarily.
	var prevKey []byte
	// Wrap the block data into a buffer.
	buf := bytes.NewBuffer(block)
	var data []*KV

	for {
		// Read one key value pair at a time.
		key, value, err := s.ReadRecord(prevKey, buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		data = append(data, &KV{
			Key:   key,
			Value: value,
		})
		// Update prevKey.
		prevKey = key
	}
	return data, nil
}

// ReadRecord reads one record. The key is stored with prefix compression, so
// prevKey is required to restore the full key.
func (s *SSTReader) ReadRecord(prevKey []byte, buf *bytes.Buffer) (key, value []byte, err error) {
	// Length of the prefix shared with prevKey.
	sharedPrefixLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, nil, err
	}

	// Length of the remaining key part.
	keyLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, nil, err
	}

	// Length of the value.
	valLen, err := binary.ReadUvarint(buf)
	if err != nil {
		return nil, nil, err
	}

	// Read the remaining key part.
	key = make([]byte, keyLen)
	if _, err = io.ReadFull(buf, key); err != nil {
		return nil, nil, err
	}

	// Read the value.
	value = make([]byte, valLen)
	if _, err = io.ReadFull(buf, value); err != nil {
		return nil, nil, err
	}

	// Join the shared prefix and the remaining key part.
	sharedPrefix := make([]byte, sharedPrefixLen)
	copy(sharedPrefix, prevKey[:sharedPrefixLen])
	key = append(sharedPrefix, key...)
	// Return the full key and the value.
	return
}
