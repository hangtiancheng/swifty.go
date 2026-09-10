package wal

import (
	"encoding/binary"
	"os"
)

// WALWriter is the writer of the write ahead log.
type WALWriter struct {
	file         string   // name of the wal file, including its directory path
	dest         *os.File // the wal file
	assistBuffer [30]byte // scratch buffer used to stage data
}

// NewWALWriter creates a wal writer.
func NewWALWriter(file string) (*WALWriter, error) {
	// Open the wal file, creating it if it does not exist. New records are
	// always appended: a wal file that is being restored from is reopened by
	// the lsm tree and its existing records must be preserved.
	dest, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &WALWriter{
		file: file,
		dest: dest,
	}, nil
}

// Write appends a key value pair to the wal file.
func (w *WALWriter) Write(key, value []byte) error {
	// First fill the key and value lengths into the scratch buffer.
	n := binary.PutUvarint(w.assistBuffer[0:], uint64(len(key)))
	n += binary.PutUvarint(w.assistBuffer[n:], uint64(len(value)))

	// Then stage key length, value length, key and value in order.
	buf := make([]byte, 0, n+len(key)+len(value))
	buf = append(buf, w.assistBuffer[:n]...)
	buf = append(buf, key...)
	buf = append(buf, value...)
	// Write the staged content to the wal file.
	_, err := w.dest.Write(buf)
	return err
}

// Close closes the underlying wal file.
func (w *WALWriter) Close() {
	_ = w.dest.Close()
}
