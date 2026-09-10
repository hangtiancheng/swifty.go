package sst

import (
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
	_, blockToFilter, index := sstWriter.Finish()
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
