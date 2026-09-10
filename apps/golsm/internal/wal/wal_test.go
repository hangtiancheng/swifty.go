package wal

import (
	"bytes"
	"os"
	"path"
	"testing"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/memtable"
)

func Test_WAL(t *testing.T) {
	walFile := path.Join(t.TempDir(), "test.wal")

	walWriter, err := NewWALWriter(walFile)
	if err != nil {
		t.Fatal(err)
	}
	defer walWriter.Close()

	skiplist := memtable.NewSkiplist()

	kvs := make([]*memtable.KV, 0, 100)
	for i := range 100 {
		kvs = append(kvs, &memtable.KV{
			Key:   []byte{'a' + uint8(i)},
			Value: []byte{'b' + uint8(i)},
		})
	}

	for _, kv := range kvs {
		skiplist.Put(kv.Key, kv.Value)
		if err = walWriter.Write(kv.Key, kv.Value); err != nil {
			t.Fatal(err)
		}
	}

	walReader, err := NewWALReader(walFile)
	if err != nil {
		t.Fatal(err)
	}
	defer walReader.Close()

	restoredSkiplist := memtable.NewSkiplist()
	if err = walReader.RestoreToMemTable(restoredSkiplist); err != nil {
		t.Fatal(err)
	}

	originKVs := skiplist.All()
	restoredKVs := restoredSkiplist.All()

	if len(originKVs) != len(restoredKVs) {
		t.Fatalf("expect len: %d, got: %d", len(originKVs), len(restoredKVs))
	}

	for i := range originKVs {
		if !bytes.Equal(originKVs[i].Key, restoredKVs[i].Key) {
			t.Errorf("index: %d, expect key: %s, got: %s", i, originKVs[i].Key, restoredKVs[i].Key)
		}
		if !bytes.Equal(originKVs[i].Value, restoredKVs[i].Value) {
			t.Errorf("index: %d, expect value: %s, got: %s", i, originKVs[i].Value, restoredKVs[i].Value)
		}
	}
}

// Test_WAL_AppendAfterReopen verifies that reopening a wal file for writing
// appends to its existing records instead of overwriting them.
func Test_WAL_AppendAfterReopen(t *testing.T) {
	walFile := path.Join(t.TempDir(), "test.wal")

	// Write the first batch of records.
	walWriter, err := NewWALWriter(walFile)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 50 {
		if err = walWriter.Write([]byte{uint8(i)}, []byte{uint8(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	walWriter.Close()

	// Reopen the same file, as the lsm tree does when it restores the active
	// memtable, and append a second batch of records.
	walWriter, err = NewWALWriter(walFile)
	if err != nil {
		t.Fatal(err)
	}
	defer walWriter.Close()
	for i := 50; i < 100; i++ {
		if err = walWriter.Write([]byte{uint8(i)}, []byte{uint8(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}

	// All 100 records must be replayed.
	walReader, err := NewWALReader(walFile)
	if err != nil {
		t.Fatal(err)
	}
	defer walReader.Close()

	memTable := memtable.NewSkiplist()
	if err = walReader.RestoreToMemTable(memTable); err != nil {
		t.Fatal(err)
	}

	if got := memTable.EntriesCnt(); got != 100 {
		t.Fatalf("expect 100 entries, got: %d", got)
	}
	for i := range 100 {
		v, ok := memTable.Get([]byte{uint8(i)})
		if !ok {
			t.Fatalf("key: %d does not exist", i)
		}
		if len(v) != 1 || v[0] != uint8(i+1) {
			t.Fatalf("key: %d, expect value: %d, got: %v", i, i+1, v)
		}
	}
}

// Test_WAL_TornTail verifies that a record left incomplete by a crash ends
// the replay instead of failing it, and that ValidSize reports the size of
// the valid prefix.
func Test_WAL_TornTail(t *testing.T) {
	walFile := path.Join(t.TempDir(), "test.wal")

	walWriter, err := NewWALWriter(walFile)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		if err = walWriter.Write([]byte{uint8(i)}, []byte{uint8(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	walWriter.Close()

	// Simulate a crash in the middle of a record write: the key length was
	// written but the key and the value are missing.
	f, err := os.OpenFile(walFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte{0x05}); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	tornSize, err := os.Stat(walFile)
	if err != nil {
		t.Fatal(err)
	}

	// Replay stops after the last complete record.
	walReader, err := NewWALReader(walFile)
	if err != nil {
		t.Fatal(err)
	}
	defer walReader.Close()

	memTable := memtable.NewSkiplist()
	if err = walReader.RestoreToMemTable(memTable); err != nil {
		t.Fatal(err)
	}

	if got := memTable.EntriesCnt(); got != 10 {
		t.Fatalf("expect 10 entries, got: %d", got)
	}
	if got := walReader.ValidSize(); got != tornSize.Size()-1 {
		t.Fatalf("expect valid size: %d, got: %d", tornSize.Size()-1, got)
	}
}
