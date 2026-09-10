package filter

// Filter assists sstables in quickly deciding whether a key may exist inside
// a given data block.
type Filter interface {
	Add(key []byte)                // adds a key to the filter
	Exist(bitmap, key []byte) bool // reports whether the key may exist, false positives are possible
	Hash() []byte                  // generates the bitmap of the filter
	Reset()                        // resets the filter
	KeyLen() int                   // number of keys currently held by the filter
}
