package sst

import (
	"github.com/hangtiancheng/swifty.go/apps/golsm/internal/filter"
)

// Options carries the configuration required by the sst layer.
type Options struct {
	// Dir is the directory that stores the sstable files.
	Dir string
	// SSTFooterSize is the fixed size of the sstable footer, in bytes.
	SSTFooterSize int
	// SSTDataBlockSize is the size limit of a single data block, in bytes.
	SSTDataBlockSize int
	// Filter assists reads in skipping blocks that cannot contain a key.
	Filter filter.Filter
}
