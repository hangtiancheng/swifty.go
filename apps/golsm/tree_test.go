package golsm

import (
	"bytes"
	"os"
	"path"
	"testing"
	"time"
)

func Test_LSM_UseCase(t *testing.T) {
	// 1 Build the configuration.
	conf, err := NewConfig(t.TempDir(), // directory that stores the sstable files
		WithMaxLevel(7),               // 7 level lsm tree
		WithSSTSize(1024*1024),        // each level 0 sstable is 1M
		WithSSTDataBlockSize(16*1024), // each block inside an sstable is 16KB
		WithSSTNumPerLevel(10),        // each level stores 10 sstable files
	)
	if err != nil {
		t.Fatal(err)
	}

	// 2 Create the lsm tree instance.
	lsmTree, err := NewTree(conf)
	if err != nil {
		t.Fatal(err)
	}
	defer lsmTree.Close()

	// 3 Write data.
	if err = lsmTree.Put([]byte{1}, []byte{2}); err != nil {
		t.Fatal(err)
	}

	// 4 Read data.
	v, ok, err := lsmTree.Get([]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("key [1] does not exist")
	}
	if !bytes.Equal(v, []byte{2}) {
		t.Fatalf("expect value: %v, got: %v", []byte{2}, v)
	}
}

func Test_LSM(t *testing.T) {
	// Build the configuration.
	conf, err := NewConfig(t.TempDir(), // directory that stores the sstable files
		WithMaxLevel(7),              // 7 level lsm tree
		WithSSTSize(32*1024),         // each level 0 sstable is 32KB
		WithSSTDataBlockSize(2*1024), // each block inside an sstable is 2KB
		WithSSTNumPerLevel(4),        // each level stores 4 sstable files
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create the lsm tree instance.
	lsmTree, err := NewTree(conf)
	if err != nil {
		t.Fatal(err)
	}
	defer lsmTree.Close()

	kvs := []struct {
		key []byte
		val []byte
	}{}

	for i := 65; i <= 122; i++ {
		for j := 65; j <= 122; j++ {
			for k := 65; k <= 85; k++ {
				kvs = append(kvs, struct {
					key []byte
					val []byte
				}{
					key: []byte{uint8(i), uint8(j), uint8(k)},
					val: []byte{uint8(i), uint8(j), uint8(k)},
				})
			}
		}
	}

	for i := 0; i < len(kvs); i++ {
		if err = lsmTree.Put(kvs[i].key, kvs[i].val); err != nil {
			t.Fatal(err)
		}
	}

	for _, kv := range kvs {
		v, ok, err := lsmTree.Get(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("key: %s does not exist", kv.key)
		}
		if !bytes.Equal(v, kv.val) {
			t.Fatalf("key: %s, expect value: %s, got: %s", kv.key, kv.val, v)
		}
	}

	// Give the background compaction goroutine some time to settle, then
	// re-read a subset of the keys. The subset covers the whole key space and
	// therefore exercises the compacted sstables of the upper levels.
	time.Sleep(time.Second)
	for i := 0; i < len(kvs); i += 257 {
		kv := kvs[i]
		v, ok, err := lsmTree.Get(kv.key)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("key: %s does not exist after compaction", kv.key)
		}
		if !bytes.Equal(v, kv.val) {
			t.Fatalf("key: %s, expect value: %s, got: %s after compaction", kv.key, kv.val, v)
		}
	}
}

func Test_Tree_getSortedSSTEntries(t *testing.T) {
	dir := t.TempDir()

	files := []string{"1_1.sst", "1_2.ab", "10_0.sst", "2_3.sst", "1_5.sst", "10_10.sst", "10_5.sst", "test.sst"}
	for _, file := range files {
		fd, err := os.Create(path.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if err = fd.Close(); err != nil {
			t.Fatal(err)
		}
	}

	tree := Tree{
		conf: &Config{
			Dir: dir,
		},
	}

	expectEntries := []string{
		"1_1.sst", "1_5.sst", "2_3.sst", "10_0.sst", "10_5.sst", "10_10.sst",
	}

	gotEntries, err := tree.getSortedSSTEntries()
	if err != nil {
		t.Fatal(err)
	}

	if len(gotEntries) != len(expectEntries) {
		t.Fatalf("got len: %d, expect: %d", len(gotEntries), len(expectEntries))
	}

	for i := 0; i < len(gotEntries); i++ {
		if gotEntries[i].Name() != expectEntries[i] {
			t.Errorf("index: %d, got entry: %s, expect: %s", i, gotEntries[i].Name(), expectEntries[i])
		}
	}
}

func Test_parseLevelSeqFromSSTFile(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		expectLevel int
		expectSeq   int32
		expectOK    bool
	}{
		{name: "valid file name", file: "1_2.sst", expectLevel: 1, expectSeq: 2, expectOK: true},
		{name: "big level and seq", file: "10_10.sst", expectLevel: 10, expectSeq: 10, expectOK: true},
		{name: "missing seq", file: "test.sst", expectOK: false},
		{name: "non numeric level", file: "a_1.sst", expectOK: false},
		{name: "non numeric seq", file: "1_b.sst", expectOK: false},
		{name: "too many parts", file: "1_2_3.sst", expectOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			level, seq, ok := parseLevelSeqFromSSTFile(test.file)
			if ok != test.expectOK {
				t.Fatalf("file: %s, expect ok: %t, got: %t", test.file, test.expectOK, ok)
			}
			if !ok {
				return
			}
			if level != test.expectLevel || seq != test.expectSeq {
				t.Errorf("file: %s, expect level: %d seq: %d, got level: %d seq: %d", test.file, test.expectLevel, test.expectSeq, level, seq)
			}
		})
	}
}

func Test_PathJoin(t *testing.T) {
	tests := []struct {
		elems []string
		want  string
	}{
		{elems: []string{"./", "wal", "1.sst"}, want: "wal/1.sst"},
		{elems: []string{"/root/", "/wal", "1.sst"}, want: "/root/wal/1.sst"},
		{elems: []string{"/root", "/wal", "1.sst"}, want: "/root/wal/1.sst"},
		{elems: []string{"/root", "wal", "1.sst"}, want: "/root/wal/1.sst"},
	}

	for _, test := range tests {
		if got := path.Join(test.elems...); got != test.want {
			t.Errorf("path.Join(%v) = %q, want %q", test.elems, got, test.want)
		}
	}
}
