package memtable

// MemTableConstructor builds a new empty MemTable.
type MemTableConstructor func() MemTable

// MemTable is an ordered in memory table used to buffer writes before they
// are flushed into sstables.
type MemTable interface {
	Put(key, value []byte)         // writes a key value pair
	Get(key []byte) ([]byte, bool) // reads a key; the second flag tells whether the key exists
	All() []*KV                    // returns all key value pairs in order
	Size() int                     // size of the data held by the table, in bytes
	EntriesCnt() int               // number of key value pairs
}

// KV is a single key value pair.
type KV struct {
	Key, Value []byte
}
