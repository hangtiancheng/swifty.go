package golsm

import (
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/sst"
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/wal"
)

// constructTree restores the whole tree from the sst files.
func (t *Tree) constructTree() error {
	// List the sst files stored in the sst directory.
	sstEntries, err := t.getSortedSSTEntries()
	if err != nil {
		return err
	}

	// Load every sst file as a node into the in memory nodes of the lsm tree.
	for _, sstEntry := range sstEntries {
		if err = t.loadNode(sstEntry); err != nil {
			return err
		}
	}

	return nil
}

func (t *Tree) getSortedSSTEntries() ([]fs.DirEntry, error) {
	allEntries, err := os.ReadDir(t.conf.Dir)
	if err != nil {
		return nil, err
	}

	sstEntries := make([]fs.DirEntry, 0, len(allEntries))
	for _, entry := range allEntries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".sst") {
			continue
		}

		// Skip sst files whose name does not follow the level_seq.sst layout.
		if _, _, ok := parseLevelSeqFromSSTFile(entry.Name()); !ok {
			continue
		}

		sstEntries = append(sstEntries, entry)
	}

	sort.Slice(sstEntries, func(i, j int) bool {
		levelI, seqI, _ := parseLevelSeqFromSSTFile(sstEntries[i].Name())
		levelJ, seqJ, _ := parseLevelSeqFromSSTFile(sstEntries[j].Name())
		if levelI == levelJ {
			return seqI < seqJ
		}
		return levelI < levelJ
	})
	return sstEntries, nil
}

// loadNode loads one sst file as a node into the topology of the lsm tree.
func (t *Tree) loadNode(sstEntry fs.DirEntry) error {
	// Create the reader of the sst file.
	sstReader, err := sst.NewSSTReader(sstEntry.Name(), t.conf.sstOptions())
	if err != nil {
		return err
	}

	// Read the filter information of each block.
	blockToFilter, err := sstReader.ReadFilter()
	if err != nil {
		return err
	}

	// Read the index information.
	index, err := sstReader.ReadIndex()
	if err != nil {
		return err
	}

	// Read the size of the sst file, in bytes.
	size, err := sstReader.Size()
	if err != nil {
		return err
	}

	// Parse the sst file name to learn the level and the seq of the file.
	level, seq, _ := parseLevelSeqFromSSTFile(sstEntry.Name())
	// Insert the sst file as a node into the lsm tree.
	t.insertNodeWithReader(sstReader, level, seq, size, blockToFilter, index)
	return nil
}

// parseLevelSeqFromSSTFile parses the level and the seq carried by an sst file
// name such as 1_2.sst. The third result reports whether the name follows the
// expected level_seq.sst layout.
func parseLevelSeqFromSSTFile(file string) (level int, seq int32, ok bool) {
	file = strings.Replace(file, ".sst", "", -1)
	splitted := strings.Split(file, "_")
	if len(splitted) != 2 {
		return 0, 0, false
	}

	_level, err := strconv.Atoi(splitted[0])
	if err != nil {
		return 0, 0, false
	}

	_seq, err := strconv.Atoi(splitted[1])
	if err != nil {
		return 0, 0, false
	}

	return _level, int32(_seq), true
}

// constructMemtable restores the memtables from the wal files.
func (t *Tree) constructMemtable() error {
	// 1 Read the wal directory to get all wal files.
	raw, err := os.ReadDir(path.Join(t.conf.Dir, "walfile"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// 2 Filter out everything that is not a wal file.
	var wals []fs.DirEntry
	for _, entry := range raw {
		if entry.IsDir() {
			continue
		}

		// The file must be a .wal file.
		if !strings.HasSuffix(entry.Name(), ".wal") {
			continue
		}

		wals = append(wals, entry)
	}

	// 3 If the wal directory or the wal files are missing, build a new
	// memtable.
	if len(wals) == 0 {
		return t.newMemTable()
	}

	// 4 Restore the memtables one by one. The freshest memtable becomes the
	// active one, while the earlier memtables become read only memtables. They
	// are added to the in memory slice and to the channel.
	return t.restoreMemTable(wals)
}

// restoreMemTable restores a list of read only memtables plus the single
// active memtable from the wal files.
func (t *Tree) restoreMemTable(wals []fs.DirEntry) error {
	// 1 Sort the wal files. Their indexes grow monotonically, and so does the
	// freshness of the data.
	sort.Slice(wals, func(i, j int) bool {
		indexI := walFileToMemTableIndex(wals[i].Name())
		indexJ := walFileToMemTableIndex(wals[j].Name())
		return indexI < indexJ
	})

	// 2 Restore the memtables one by one and add them to memory and channel.
	for i := 0; i < len(wals); i++ {
		name := wals[i].Name()
		file := path.Join(t.conf.Dir, "walfile", name)

		// Build the wal reader matching the wal file.
		walReader, err := wal.NewWALReader(file)
		if err != nil {
			return err
		}
		defer walReader.Close()

		// Read the content of the wal file through the reader and inject the
		// data into the memtable.
		memTable := t.conf.MemTableConstructor()
		if err = walReader.RestoreToMemTable(memTable); err != nil {
			return err
		}

		if i == len(wals)-1 { // The last wal file restores the active memtable.
			t.memTable = memTable
			t.memTableIndex = walFileToMemTableIndex(name)
			t.walWriter, err = wal.NewWALWriter(file)
			if err != nil {
				return err
			}
		} else { // Every earlier wal file restores a read only memtable, appended to the read only slice and to the channel so its flush continues in the background.
			memTableCompactItem := memTableCompactItem{
				walFile:  file,
				memTable: memTable,
			}

			t.rOnlyMemTable = append(t.rOnlyMemTable, &memTableCompactItem)
			t.memCompactC <- &memTableCompactItem
		}
	}
	return nil
}
