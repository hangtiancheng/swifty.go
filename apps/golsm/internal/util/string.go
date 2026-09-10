package util

import "slices"

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
		// Decrement the last non-zero byte of b and truncate after it, so the
		// result stays strictly smaller than b even when b ends with zero
		// bytes. Decrementing the last byte unconditionally would underflow
		// and produce a value bigger than b.
		for i, c := range slices.Backward(b) {
			if c > 0 {
				separator := make([]byte, i+1)
				copy(separator, b[:i+1])
				separator[i]--
				return separator
			}
		}
		// b consists only of zero bytes, so the empty key is the only value
		// smaller than b.
		return nil
	}

	// a itself satisfies a <= a < b.
	return a
}
