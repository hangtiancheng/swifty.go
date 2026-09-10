package sst

import (
	"bytes"
	"fmt"
	"testing"
)

func Test_SSTReader(t *testing.T) {
	opts := newTestOptions(t)
	opts.SSTDataBlockSize = 16

	// Build an sst writer and write data into it.
	sstWriter, err := NewSSTWriter("test_write_read.sst", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer sstWriter.Close()

	// datablock1: record: [0 1 1 a b] [1 1 2 b c d] [0 1 1 e f]
	// datablock2: record: [0 2 1 e f g h]
	// filter: 0 -> bitmap1  16 -> bitmap2
	// index: [` 0 0] [e 0 16] [ef 16 7]
	// footer: ...
	expectKVs := []*KV{
		{Key: []byte("a"), Value: []byte("b")},
		{Key: []byte("ab"), Value: []byte("cd")},
		{Key: []byte("e"), Value: []byte("f")},
		{Key: []byte("ef"), Value: []byte("gh")},
	}

	for _, kv := range expectKVs {
		sstWriter.Append(kv.Key, kv.Value)
	}

	_, expectBlockToFilter, expectIndex := sstWriter.Finish()

	// Build an sst reader and read the data back.
	sstReader, err := NewSSTReader("test_write_read.sst", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer sstReader.Close()

	gotBlockToFilter, err := sstReader.ReadFilter()
	if err != nil {
		t.Fatal(err)
	}

	gotIndex, err := sstReader.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}

	if err = assertFilterEqual(expectBlockToFilter, gotBlockToFilter); err != nil {
		t.Fatal(err)
	}

	if err = assertIndexEqual(expectIndex, gotIndex); err != nil {
		t.Fatal(err)
	}

	gotKVs, err := sstReader.ReadData()
	if err != nil {
		t.Fatal(err)
	}

	if err = assertDataEqual(expectKVs, gotKVs); err != nil {
		t.Fatal(err)
	}
}

func assertFilterEqual(expect, got map[uint64][]byte) error {
	if len(expect) != len(got) {
		return fmt.Errorf("expect len: %d, got len: %d", len(expect), len(got))
	}

	for expectK, expectV := range expect {
		gotV := got[expectK]
		if !bytes.Equal(expectV, gotV) {
			return fmt.Errorf("key: %d, expect value: %v, got value: %v", expectK, expectV, gotV)
		}
	}

	return nil
}

func assertIndexEqual(expect, got []*Index) error {
	if len(expect) != len(got) {
		return fmt.Errorf("expect len: %d, got len: %d", len(expect), len(got))
	}

	for i := range expect {
		if !bytes.Equal(expect[i].Key, got[i].Key) {
			return fmt.Errorf("index: %d, expect key: %s, got key: %s", i, expect[i].Key, got[i].Key)
		}

		if expect[i].PrevBlockOffset != got[i].PrevBlockOffset {
			return fmt.Errorf("index: %d, expect offset: %d, got offset: %d", i, expect[i].PrevBlockOffset, got[i].PrevBlockOffset)
		}

		if expect[i].PrevBlockSize != got[i].PrevBlockSize {
			return fmt.Errorf("index: %d, expect size: %d, got size: %d", i, expect[i].PrevBlockSize, got[i].PrevBlockSize)
		}
	}
	return nil
}

func assertDataEqual(expect, got []*KV) error {
	if len(expect) != len(got) {
		return fmt.Errorf("expect len: %d, got len: %d", len(expect), len(got))
	}

	for i := range expect {
		if !bytes.Equal(expect[i].Key, got[i].Key) {
			return fmt.Errorf("data: %d, expect key: %s, got key: %s", i, expect[i].Key, got[i].Key)
		}

		if !bytes.Equal(expect[i].Value, got[i].Value) {
			return fmt.Errorf("data: %d, expect value: %s, got value: %s", i, expect[i].Value, got[i].Value)
		}
	}
	return nil
}
