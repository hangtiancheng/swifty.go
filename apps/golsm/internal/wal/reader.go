package wal

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"os"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/memtable"
)

// WALReader reads a wal file.
type WALReader struct {
	file      string        // name of the wal file, including its directory path
	src       *os.File      // the wal file
	reader    *bufio.Reader // buffered reader wrapping the wal file
	validSize int64         // size of the valid prefix of the wal file, in bytes
}

// NewWALReader creates a wal reader.
func NewWALReader(file string) (*WALReader, error) {
	// Open the wal file in read only mode. The file must exist.
	src, err := os.OpenFile(file, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &WALReader{
		file:   file,
		src:    src,
		reader: bufio.NewReader(src),
	}, nil
}

// RestoreToMemTable replays the whole wal file into the memtable to restore
// the in memory data. A trailing record left incomplete by a crash ends the
// replay instead of failing it; the incomplete bytes are reported by
// ValidSize so the caller can drop them.
func (w *WALReader) RestoreToMemTable(memTable memtable.MemTable) error {
	// Read the full content of the wal file.
	body, err := io.ReadAll(w.reader)
	if err != nil {
		return err
	}

	// Parse the content read from the file into a list of key value pairs.
	kvs, validSize := readAll(body)

	// Inject all key value pairs into the memtable.
	for _, kv := range kvs {
		memTable.Put(kv.Key, kv.Value)
	}

	w.validSize = validSize
	return nil
}

// ValidSize returns the size of the valid prefix of the wal file, in bytes.
// Everything past it was left incomplete by a crash and must be dropped
// before new records are appended to the file.
func (w *WALReader) ValidSize() int64 {
	return w.validSize
}

// readAll parses the raw content into a list of key value pairs. It returns
// the pairs together with the size of the valid prefix they form: a record
// that was left incomplete by a crash ends the replay instead of failing it.
func readAll(body []byte) ([]*memtable.KV, int64) {
	var (
		kvs       []*memtable.KV
		validSize int64
	)

	reader := bytes.NewReader(body)
	for {
		// Read the first uvarint as the key length. Reaching the end of the
		// content means the file has been fully consumed.
		keyLen, err := binary.ReadUvarint(reader)
		if err != nil {
			break
		}

		// Read the next uvarint as the value length.
		valLen, err := binary.ReadUvarint(reader)
		if err != nil {
			break
		}

		// A length reaching past the end of the content means the tail of the
		// file was left incomplete by a crash. Never allocate from such a
		// length; just stop the replay at the last complete record.
		if keyLen > uint64(reader.Len()) {
			break
		}
		keyBuf := make([]byte, keyLen)
		if _, err = io.ReadFull(reader, keyBuf); err != nil {
			break
		}

		if valLen > uint64(reader.Len()) {
			break
		}
		valBuf := make([]byte, valLen)
		if _, err = io.ReadFull(reader, valBuf); err != nil {
			break
		}

		kvs = append(kvs, &memtable.KV{
			Key:   keyBuf,
			Value: valBuf,
		})
		// The record is complete, so it belongs to the valid prefix.
		validSize = int64(len(body) - reader.Len())
	}

	return kvs, validSize
}

// Close closes the underlying wal file.
func (w *WALReader) Close() {
	w.reader.Reset(w.src)
	_ = w.src.Close()
}
