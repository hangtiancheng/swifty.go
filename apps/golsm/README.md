<p align="center">
  <b>golsm: an LSM tree implemented in pure Go</b>
</p>

## Introduction

While studying LSM tree implementations, this project borrowed heavily from the
simple-raft project.

## Overview

golsm is an LSM tree engine written in 100% pure Go (standard library only,
zero third-party dependencies). It is well suited for write-heavy key-value
storage workloads.

## Features

- Write-ahead log (WAL) protecting every write before it reaches the memtable.
- Skiplist based memtable with read-only memtable staging.
- SSTables with prefix compressed records, per-block bloom filters and block
  indexes.
- Background compaction: memtable flushing, level sorted merges and multi-level
  size based triggers.
- Crash recovery: the tree is rebuilt from the SST files and WAL files on disk.

## Project layout

```
golsm/
├── tree.go               # Tree: the public engine API (Put/Get/Close)
├── tree_compact.go       # background compaction (memtable flush, level merges)
├── tree_restore.go       # recovery from SST and WAL files
├── config.go             # Config, NewConfig and the With* options
└── internal/
    ├── filter/           # Filter interface + bloom filter (hash/fnv based)
    ├── memtable/         # MemTable interface + skiplist
    ├── wal/              # write-ahead log writer and reader
    ├── sst/              # SSTable writer, reader, block and tree node
    └── util/             # small byte slice helpers
```

The root package `golsm` is the public API of the engine. Everything under
`internal/` is private implementation detail.

## Usage

```go
package main

import "github.com/hangtiancheng/swifty.go/apps/golsm"

func main() {
	// 1 Build the configuration.
	conf, err := golsm.NewConfig("./lsm", // directory that stores the sstable files
		golsm.WithMaxLevel(7),               // 7 level lsm tree
		golsm.WithSSTSize(1024*1024),        // each level 0 sstable is 1MB
		golsm.WithSSTDataBlockSize(16*1024), // each block inside an sstable is 16KB
		golsm.WithSSTNumPerLevel(10),        // each level stores 10 sstable files
	)
	if err != nil {
		panic(err)
	}

	// 2 Create an lsm tree instance.
	lsmTree, err := golsm.NewTree(conf)
	if err != nil {
		panic(err)
	}
	defer lsmTree.Close()

	// 3 Write data.
	if err := lsmTree.Put([]byte{1}, []byte{2}); err != nil {
		panic(err)
	}

	// 4 Read data.
	value, ok, err := lsmTree.Get([]byte{1})
	if err != nil {
		panic(err)
	}
	if ok {
		println(string(value))
	}
}
```

Custom filters and memtables can be plugged in through `golsm.WithFilter` and
`golsm.WithMemtableConstructor` by implementing the `golsm.Filter` and
`golsm.MemTable` interfaces.
