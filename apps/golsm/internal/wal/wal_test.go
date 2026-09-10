package wal

import (
	"bytes"
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
