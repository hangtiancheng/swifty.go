package golsm

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// newTestTree builds an lsm tree for tests. The thresholds are small so that
// memtable flushes and compactions happen quickly.
func newTestTree(t *testing.T, dir string) *Tree {
	t.Helper()

	conf, err := NewConfig(dir,
		WithMaxLevel(4),           // 4 level lsm tree
		WithSSTSize(2*1024),       // each level 0 sstable is 2KB
		WithSSTDataBlockSize(256), // each block inside an sstable is 256B
		WithSSTNumPerLevel(3),     // each level stores 3 sst files
	)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := NewTree(conf)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// testKey builds the key of pair i, testValue builds its expected value for
// the given round of writes.
func testKey(i int) []byte {
	return fmt.Appendf(nil, "key-%04d", i)
}

func testValue(round int) []byte {
	return bytes.Repeat([]byte{byte(round%251 + 1)}, 64)
}

// assertPairs reads every pair of the range [start, end) and compares it with
// the value it holds in the given round of writes.
func assertPairs(t *testing.T, tree *Tree, start, end, round int) {
	t.Helper()
	for i := start; i < end; i++ {
		got, ok, err := tree.Get(testKey(i))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("key: %s does not exist", testKey(i))
		}
		if expect := testValue(round); !bytes.Equal(got, expect) {
			t.Fatalf("key: %s, expect value: %v, got: %v", testKey(i), expect, got)
		}
	}
}

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

	for i := range gotEntries {
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

// Test_Tree_RestoreRoundTrip verifies that data written into a tree survives
// a close and a reopen, including pairs that were not flushed into sstables
// yet and pairs of read only memtables that were not compacted yet.
func Test_Tree_RestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// 1 Write the first batch of pairs.
	tree := newTestTree(t, dir)
	const firstBatch = 200
	for i := range firstBatch {
		if err := tree.Put(testKey(i), testValue(0)); err != nil {
			t.Fatal(err)
		}
	}
	tree.Close()

	// 2 Reopen the tree: every pair must still be readable.
	tree = newTestTree(t, dir)
	assertPairs(t, tree, 0, firstBatch, 0)

	// 3 Overwrite the first half of the pairs and append new ones, then
	// repeat the round trip.
	const overwrites = 100
	const appended = 50
	for i := range overwrites {
		if err := tree.Put(testKey(i), testValue(1)); err != nil {
			t.Fatal(err)
		}
	}
	for i := firstBatch; i < firstBatch+appended; i++ {
		if err := tree.Put(testKey(i), testValue(0)); err != nil {
			t.Fatal(err)
		}
	}
	tree.Close()

	// 4 Reopen the tree: the overwritten pairs must show the newest value and
	// every other pair must keep its value.
	tree = newTestTree(t, dir)
	defer tree.Close()
	assertPairs(t, tree, 0, overwrites, 1)
	assertPairs(t, tree, overwrites, firstBatch+appended, 0)
}

// Test_Tree_RestoreAfterCrash verifies that a wal tail torn by a crash and a
// partial temporary sst file do not break the next start, and that writes
// appended after the recovery keep the wal file parseable.
func Test_Tree_RestoreAfterCrash(t *testing.T) {
	dir := t.TempDir()

	// 1 Write a batch of pairs.
	tree := newTestTree(t, dir)
	const firstBatch = 50
	for i := range firstBatch {
		if err := tree.Put(testKey(i), testValue(0)); err != nil {
			t.Fatal(err)
		}
	}
	tree.Close()

	// 2 Simulate a crash: append a torn record to the active wal file and
	// leave a partial temporary sst file behind. The active wal file is the
	// one carrying the biggest memtable index.
	walDir := path.Join(dir, "walfile")
	entries, err := os.ReadDir(walDir)
	if err != nil {
		t.Fatal(err)
	}
	activeWAL := ""
	activeIndex := -1
	for _, entry := range entries {
		index, ok := parseMemTableIndexFromWALFile(entry.Name())
		if ok && index > activeIndex {
			activeIndex = index
			activeWAL = entry.Name()
		}
	}
	if activeWAL == "" {
		t.Fatal("expect at least one wal file")
	}
	f, err := os.OpenFile(path.Join(walDir, activeWAL), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	// A key length whose bytes are never followed by the key itself.
	if _, err = f.Write([]byte{0x7f, 0x01}); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path.Join(dir, "0_99.sst.tmp"), []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3 The tree must start and every complete pair must be readable.
	tree = newTestTree(t, dir)
	assertPairs(t, tree, 0, firstBatch, 0)

	// 4 Write more pairs and close: they are appended to the wal file that
	// was truncated at its valid prefix.
	const secondBatch = 30
	for i := firstBatch; i < firstBatch+secondBatch; i++ {
		if err := tree.Put(testKey(i), testValue(0)); err != nil {
			t.Fatal(err)
		}
	}
	tree.Close()

	// 5 A third start must still see every pair, which proves that the wal
	// file stayed consistent across the recovery.
	tree = newTestTree(t, dir)
	defer tree.Close()
	assertPairs(t, tree, 0, firstBatch+secondBatch, 0)
}

// Test_Tree_CompactionKeepsNewestValue verifies that repeatedly overwriting
// the same keys keeps the newest value visible while the background
// compaction merges the sstables: stale values must never resurface.
func Test_Tree_CompactionKeepsNewestValue(t *testing.T) {
	dir := t.TempDir()

	tree := newTestTree(t, dir)
	defer tree.Close()

	const keys = 60
	const rounds = 6
	for round := range rounds {
		for i := range keys {
			if err := tree.Put(testKey(i), testValue(round)); err != nil {
				t.Fatal(err)
			}
		}
		// While the sstables pile up, the newest round value must always win.
		assertPairs(t, tree, 0, keys, round)
	}

	// Give the background compaction time to merge every level, then verify
	// the newest values again.
	time.Sleep(2 * time.Second)
	assertPairs(t, tree, 0, keys, rounds-1)
}

// Test_Tree_ConcurrentGetPut exercises concurrent reads and writes. Run with
// -race it verifies that the tree is safe for concurrent use.
func Test_Tree_ConcurrentGetPut(t *testing.T) {
	dir := t.TempDir()

	tree := newTestTree(t, dir)
	defer tree.Close()

	const writers = 4
	const keysPerWriter = 100

	var wg sync.WaitGroup
	// Every writer fills its own key range.
	for w := range writers {
		wg.Go(func() {
			for i := range keysPerWriter {
				key := fmt.Appendf(nil, "key-%d-%03d", w, i)
				if err := tree.Put(key, bytes.Repeat([]byte{byte(w + 1)}, 64)); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	// Readers walk the key space while the writers are running.
	for range writers {
		wg.Go(func() {
			for i := range 4 * keysPerWriter {
				key := fmt.Appendf(nil, "key-%d-%03d", i%writers, i%keysPerWriter)
				if _, _, err := tree.Get(key); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()

	// Every pair written by every writer must be readable.
	for w := range writers {
		for i := range keysPerWriter {
			key := fmt.Appendf(nil, "key-%d-%03d", w, i)
			value, ok, err := tree.Get(key)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatalf("key: %s does not exist", key)
			}
			if expect := bytes.Repeat([]byte{byte(w + 1)}, 64); !bytes.Equal(value, expect) {
				t.Fatalf("key: %s, expect value: %v, got: %v", key, expect, value)
			}
		}
	}
}

// Test_Tree_CloseLeak verifies that closing a tree right after memtable
// switches, with pending flushes still queued, leaves no goroutine behind.
func Test_Tree_CloseLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	dir := t.TempDir()

	tree := newTestTree(t, dir)
	const keys = 300
	for i := range keys {
		if err := tree.Put(testKey(i), testValue(0)); err != nil {
			t.Fatal(err)
		}
	}
	tree.Close()
}
