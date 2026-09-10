package sst

import (
	"bytes"
	"os"
	"path"
	"testing"
)

func Test_SSTWriter(t *testing.T) {
	opts := newTestOptions(t)
	opts.SSTDataBlockSize = 16

	sstWriter, err := NewSSTWriter("test.sst", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer sstWriter.Close()

	sstWriter.Append([]byte("a"), []byte("b"))
	sstWriter.Append([]byte("ab"), []byte("cd"))
	sstWriter.Append([]byte("e"), []byte("f"))
	sstWriter.Append([]byte("ef"), []byte("gh"))

	// datablock1: record: [0 1 1 a b] [1 1 2 b c d] [0 1 1 e f], size: 16
	// datablock2: record: [0 2 2 ef gh], size: 7
	// filter: 0 -> bitmap1  16 -> bitmap2
	// index: [` 0 0] [e 0 16] [ef 16 7]
	// footer: ...
	_, blockToFilter, index, err := sstWriter.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if len(blockToFilter) != 2 {
		t.Errorf("unexpected filter len: %d", len(blockToFilter))
	}

	if _, ok := blockToFilter[0]; !ok {
		t.Error("miss filter key: 0")
	}

	if _, ok := blockToFilter[16]; !ok {
		t.Error("miss filter key: 16")
	}

	if len(index) != 3 {
		t.Errorf("unexpected index len: %d", len(index))
	}

	if string(index[0].Key) != "`" || index[0].PrevBlockOffset != 0 || index[0].PrevBlockSize != 0 {
		t.Errorf("invalid index0: %+v, key: %s", index[0], index[0].Key)
	}

	if string(index[1].Key) != "e" || index[1].PrevBlockOffset != 0 || index[1].PrevBlockSize != 16 {
		t.Errorf("invalid index1: %+v", index[1])
	}

	if string(index[2].Key) != "ef" || index[2].PrevBlockOffset != 16 || index[2].PrevBlockSize != 7 {
		t.Errorf("invalid index2: %+v", index[2])
	}
}

// Test_SSTWriter_FinishPublishesFile verifies that Finish publishes the
// sstable under its final name and that no temporary file is left behind.
func Test_SSTWriter_FinishPublishesFile(t *testing.T) {
	opts := newTestOptions(t)

	sstWriter, err := NewSSTWriter("test_publish.sst", opts)
	if err != nil {
		t.Fatal(err)
	}

	sstWriter.Append([]byte("a"), []byte("b"))
	if _, _, _, err = sstWriter.Finish(); err != nil {
		t.Fatal(err)
	}
	sstWriter.Close()

	if _, err = os.Stat(path.Join(opts.Dir, "test_publish.sst")); err != nil {
		t.Fatalf("final sstable is missing: %v", err)
	}
	if _, err = os.Stat(path.Join(opts.Dir, "test_publish.sst.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temporary file was not removed: %v", err)
	}
}

// Test_SSTWriter_UnfinishedWriterLeavesNoSstable verifies that a writer
// dropped without Finish leaves no sstable behind, so a crash in the middle
// of a write cannot publish a partial file.
func Test_SSTWriter_UnfinishedWriterLeavesNoSstable(t *testing.T) {
	opts := newTestOptions(t)

	sstWriter, err := NewSSTWriter("test_unfinished.sst", opts)
	if err != nil {
		t.Fatal(err)
	}
	sstWriter.Append([]byte("a"), []byte("b"))
	sstWriter.Close()

	if _, err = os.Stat(path.Join(opts.Dir, "test_unfinished.sst")); !os.IsNotExist(err) {
		t.Fatalf("unfinished sstable was published: %v", err)
	}
	if _, err = os.Stat(path.Join(opts.Dir, "test_unfinished.sst.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temporary file was not removed: %v", err)
	}
}

// noopFilter is a filter that never holds any key, as a custom implementation
// with an empty KeyLen would behave.
type noopFilter struct{}

func (noopFilter) Add(key []byte)                {}
func (noopFilter) Exist(bitmap, key []byte) bool { return true }
func (noopFilter) Hash() []byte                  { return nil }
func (noopFilter) Reset()                        {}
func (noopFilter) KeyLen() int                   { return 0 }

// Test_SSTWriter_EmptyFilterKeepsData verifies that blocks are flushed even
// when the filter holds no keys, so a filter that contributes no bitmap never
// silently drops the data of an sstable.
func Test_SSTWriter_EmptyFilterKeepsData(t *testing.T) {
	opts := newTestOptions(t)
	opts.Filter = noopFilter{}
	opts.SSTDataBlockSize = 8

	sstWriter, err := NewSSTWriter("test_noop_filter.sst", opts)
	if err != nil {
		t.Fatal(err)
	}

	expectKVs := []*KV{
		{Key: []byte("a"), Value: []byte("b")},
		{Key: []byte("abc"), Value: []byte("d")},
		{Key: []byte("e"), Value: []byte("f")},
	}
	for _, kv := range expectKVs {
		sstWriter.Append(kv.Key, kv.Value)
	}

	_, blockToFilter, index, err := sstWriter.Finish()
	if err != nil {
		t.Fatal(err)
	}
	sstWriter.Close()

	if len(blockToFilter) != 0 {
		t.Errorf("expect no filter bitmap, got: %d", len(blockToFilter))
	}
	if len(index) == 0 {
		t.Fatal("expect at least one index entry")
	}

	sstReader, err := NewSSTReader("test_noop_filter.sst", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer sstReader.Close()

	gotKVs, err := sstReader.ReadData()
	if err != nil {
		t.Fatal(err)
	}
	if len(gotKVs) != len(expectKVs) {
		t.Fatalf("expect %d pairs, got: %d", len(expectKVs), len(gotKVs))
	}
	for i := range expectKVs {
		if !bytes.Equal(gotKVs[i].Key, expectKVs[i].Key) || !bytes.Equal(gotKVs[i].Value, expectKVs[i].Value) {
			t.Errorf("index: %d, expect: %v, got: %v", i, expectKVs[i], gotKVs[i])
		}
	}
}
