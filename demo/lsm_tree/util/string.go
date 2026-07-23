package util

func SharedPrefixLen(a, b []byte) int {
	var i int
	for ; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			break
		}
	}
	return i
}

// GetSeparatorBetween returns x such that a <= x < b. The caller must ensure a < b.
func GetSeparatorBetween(a, b []byte) []byte {
	// If a is empty, return a value smaller than b.
	if len(a) == 0 {
		separator := make([]byte, len(b))
		copy(separator, b)
		return append(separator[:len(b)-1], separator[len(b)-1]-1)
	}

	// Return a itself.
	return a
}
