package golsm

import (
	"fmt"
	"os"
	"path"

	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/filter"
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/memtable"
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/sst"
)

// Filter is the filter interface accepted by the engine. It is an alias of
// the interface provided by internal/filter so that custom implementations
// can be injected through WithFilter.
type Filter = filter.Filter

// MemTable is the ordered in memory table interface accepted by the engine.
// It is an alias of the interface provided by internal/memtable.
type MemTable = memtable.MemTable

// MemTableConstructor builds a new empty MemTable. It is an alias of the type
// provided by internal/memtable.
type MemTableConstructor = memtable.MemTableConstructor

// Config aggregates the configuration of an lsm tree.
type Config struct {
	Dir      string // directory that stores the sst files
	MaxLevel int    // total number of levels of the lsm tree

	// sst related options
	SSTSize          uint64 // size limit of each level 0 sstable, default 1MB
	SSTNumPerLevel   int    // expected number of sstables per level, default 10
	SSTDataBlockSize int    // size limit of each block inside an sstable, default 16KB
	SSTFooterSize    int    // size of the sstable footer. Fixed at 32B

	Filter              Filter              // filter used by the sstables, defaults to the built-in bloom filter
	MemTableConstructor MemTableConstructor // memtable constructor, defaults to the built-in skiplist
}

// NewConfig builds a configuration.
func NewConfig(dir string, opts ...ConfigOption) (*Config, error) {
	c := Config{
		Dir:           dir, // directory that stores the sstable files
		SSTFooterSize: 32,  // 4 uint64 values, 32 bytes in total
	}

	// Load the configuration options.
	for _, opt := range opts {
		opt(&c)
	}

	// Apply the default values.
	applyDefaults(&c)

	// Validate the configuration. The directories storing the sst files and
	// the wal files are created here if they are missing.
	return &c, c.check()
}

// check validates the configuration. It mainly checks the directories that
// store the sst files and the wal files, and creates them if missing.
func (c *Config) check() error {
	// Make sure the sstable directory exists.
	if err := ensureDir(c.Dir); err != nil {
		return err
	}

	// Make sure the wal directory exists.
	return ensureDir(path.Join(c.Dir, "walfile"))
}

// ensureDir creates dir if it does not exist yet.
func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", dir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, os.ModePerm)
}

// ConfigOption mutates a Config.
type ConfigOption func(*Config)

// WithMaxLevel sets the number of levels of the lsm tree. Default: 7.
func WithMaxLevel(maxLevel int) ConfigOption {
	return func(c *Config) {
		c.MaxLevel = maxLevel
	}
}

// WithSSTSize sets the size limit of each level 0 sstable, in bytes.
// Default: 1MB. Every deeper level multiplies the size limit by 10.
func WithSSTSize(sstSize uint64) ConfigOption {
	return func(c *Config) {
		c.SSTSize = sstSize
	}
}

// WithSSTDataBlockSize sets the size limit of each block inside an sstable.
// Default: 16KB.
func WithSSTDataBlockSize(sstDataBlockSize int) ConfigOption {
	return func(c *Config) {
		c.SSTDataBlockSize = sstDataBlockSize
	}
}

// WithSSTNumPerLevel sets the expected maximum number of sstable files stored
// per level. Default: 10.
func WithSSTNumPerLevel(sstNumPerLevel int) ConfigOption {
	return func(c *Config) {
		c.SSTNumPerLevel = sstNumPerLevel
	}
}

// WithFilter injects the filter implementation. Default: the bloom filter
// implemented in internal/filter.
func WithFilter(f Filter) ConfigOption {
	return func(c *Config) {
		c.Filter = f
	}
}

// WithMemtableConstructor injects the ordered table constructor. Default: the
// skiplist implemented in internal/memtable.
func WithMemtableConstructor(memtableConstructor MemTableConstructor) ConfigOption {
	return func(c *Config) {
		c.MemTableConstructor = memtableConstructor
	}
}

// applyDefaults fills the configuration with default values.
func applyDefaults(c *Config) {
	// The lsm tree has 7 levels by default.
	if c.MaxLevel <= 1 {
		c.MaxLevel = 7
	}

	// Each level 0 sstable is limited to 1MB by default.
	// Every deeper level multiplies the size limit by 10.
	if c.SSTSize <= 0 {
		c.SSTSize = 1024 * 1024
	}

	// Each block inside an sstable is limited to 16KB by default.
	if c.SSTDataBlockSize <= 0 {
		c.SSTDataBlockSize = 16 * 1024 // 16KB
	}

	// Each level is expected to store at most 10 sstable files by default.
	if c.SSTNumPerLevel <= 0 {
		c.SSTNumPerLevel = 10
	}

	// The filter defaults to the bloom filter implemented in internal/filter.
	if c.Filter == nil {
		c.Filter, _ = filter.NewBloomFilter(1024)
	}

	// The memtable constructor defaults to the skiplist implemented in
	// internal/memtable.
	if c.MemTableConstructor == nil {
		c.MemTableConstructor = memtable.NewSkiplist
	}
}

// sstOptions converts the configuration into the options consumed by the sst
// layer.
func (c *Config) sstOptions() *sst.Options {
	return &sst.Options{
		Dir:              c.Dir,
		SSTFooterSize:    c.SSTFooterSize,
		SSTDataBlockSize: c.SSTDataBlockSize,
		Filter:           c.Filter,
	}
}
