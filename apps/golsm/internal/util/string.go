package util

// SharedPrefixLen returns the length of the prefix shared by a and b.
func SharedPrefixLen(a, b []byte) int {
	var i int
	for ; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			break
		}
	}
	return i
}

// GetSeparatorBetween returns a value x that guarantees a <= x < b. The caller
// must ensure a < b.
func GetSeparatorBetween(a, b []byte) []byte {
	// If a is empty, any value smaller than b works. An empty b has no
	// smaller value, so return nil.
	if len(b) == 0 {
		return nil
	}
	if len(a) == 0 {
		separator := make([]byte, len(b))
		copy(separator, b)
		return append(separator[:len(b)-1], separator[len(b)-1]-1)
	}

	// a itself satisfies a <= a < b.
	return a
}
