package filter

import (
	"bytes"
	"math/bits"
	"testing"
)

func Test_BloomFilter_Add_Exist(t *testing.T) {
	m := 16
	bf, err := NewBloomFilter(m)
	if err != nil {
		t.Fatal(err)
	}

	bf.Add([]byte("a"))
	bf.Add([]byte("b"))
	bf.Add([]byte("c"))
	bf.Add([]byte("d"))

	bitmap := bf.Hash()
	for _, key := range []string{"a", "b", "c", "d"} {
		if ok := bf.Exist(bitmap, []byte(key)); !ok {
			t.Errorf("key: %s, expect: true, got: false", key)
		}
	}

	// "e" maps to bit 0 twice, which is not set by any added key.
	if ok := bf.Exist(bitmap, []byte("e")); ok {
		t.Errorf("key: e, expect: false, got: true")
	}
}

func Test_BloomFilter_Hash(t *testing.T) {
	m := 8
	bf, err := NewBloomFilter(m)
	if err != nil {
		t.Fatal(err)
	}

	bf.Add([]byte("a"))
	bf.Add([]byte("b"))

	// k := 2
	// hashedKey1 (fnv64a of "a"): 12638187200555641996, delta (rotate right 17): 17745967803416331008
	// hashedKey2 (fnv64a of "b"): 12638190499090526629, delta (rotate right 17): 17929630225745199872
	// bitmap bits: 00110000, last byte stores k
	expect := []byte{
		uint8(48),
		uint8(2),
	}

	if got := bf.Hash(); !bytes.Equal(got, expect) {
		t.Errorf("expect: %v, got: %v", expect, got)
	}
}

func Test_HashBitOperation(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expectH1 uint64
		expectH2 uint64
	}{
		{
			name:     "a",
			key:      "a",
			expectH1: 12638187200555641996,
			expectH2: 17745967803416331008,
		},
		{
			name:     "b",
			key:      "b",
			expectH1: 12638190499090526629,
			expectH2: 17929630225745199872,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h1 := hashKey([]byte(test.key))
			if h1 != test.expectH1 {
				t.Errorf("key: %s, expect h1: %d, got: %d", test.key, test.expectH1, h1)
			}
			if h2 := hashDelta(h1); h2 != test.expectH2 {
				t.Errorf("key: %s, expect h2: %d, got: %d", test.key, test.expectH2, h2)
			}
			if h2 := hashDelta(h1); h2 != bits.RotateLeft64(h1, -17) {
				t.Errorf("key: %s, h2 is not h1 rotated right by 17 bits: %d", test.key, h2)
			}
		})
	}
}
