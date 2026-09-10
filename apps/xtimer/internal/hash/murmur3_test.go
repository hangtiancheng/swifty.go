package hash

import "testing"

// Golden values generated from the reference spaolacci/murmur3 implementation
// (murmur3.Sum32, seed 0) to guarantee the internal implementation is
// bit-for-bit compatible.
func TestSum32Golden(t *testing.T) {
	cases := map[string]uint64{
		"":                            0,
		"hello":                       613153351,
		"xtimer":                      2923741163,
		"task_bloom_2026-09-01 10:00": 22203869,
		"1_1785000000000":             2510795045,
		"a":                           1009084850,
		"ab":                          2613040991,
		"abc":                         3017643002,
		"abcd":                        1139631978,
		"The quick brown fox jumps over the lazy dog": 776992547,
	}
	for input, want := range cases {
		if got := Sum32([]byte(input)); uint64(got) != want {
			t.Errorf("Sum32(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestMurmur3Encryptor(t *testing.T) {
	e := NewMurmur3Encryptor()
	if got := e.Encrypt("hello"); got != uint64(613153351) {
		t.Errorf("Encrypt(\"hello\") = %d, want %d", got, uint64(613153351))
	}
}

func TestSHA1EncryptorDeterministic(t *testing.T) {
	s := NewSHA1Encryptor()
	a, b := s.Encrypt("xtimer"), s.Encrypt("xtimer")
	if a != b {
		t.Fatalf("SHA1Encryptor is not deterministic: %d != %d", a, b)
	}
	if a == 0 {
		t.Fatal("SHA1Encryptor returned 0 for a non-empty input")
	}
}
