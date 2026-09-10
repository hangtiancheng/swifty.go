// Package uuid provides a minimal RFC 4122 version 4 UUID generator built on
// crypto/rand. It exists so that gotcc does not need to depend on
// github.com/google/uuid.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a new random (version 4) UUID formatted as
// xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx, where y is one of 8, 9, a or b.
func New() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand failing is unrecoverable; panic like other stdlib
		// randomness consumers would.
		panic(fmt.Errorf("uuid: failed to read random bytes: %w", err))
	}

	// Set the version (4) and variant (RFC 4122) bits.
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	dst := make([]byte, 36)
	hex.Encode(dst[0:8], buf[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], buf[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], buf[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], buf[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], buf[10:16])

	return string(dst)
}
