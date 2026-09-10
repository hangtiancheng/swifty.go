package util

import (
	"bytes"
	"testing"
)

func Test_SharedPrefixLen(t *testing.T) {
	tests := []struct {
		name string
		a, b []byte
		want int
	}{
		{name: "nil b", a: []byte("a"), b: nil, want: 0},
		{name: "a is prefix of b", a: []byte("ab"), b: []byte("abc"), want: 2},
		{name: "no shared prefix", a: []byte("ab"), b: []byte("c"), want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SharedPrefixLen(test.a, test.b); got != test.want {
				t.Errorf("SharedPrefixLen(%q, %q) = %d, want %d", test.a, test.b, got, test.want)
			}
		})
	}
}

func Test_GetSeparatorBetween(t *testing.T) {
	tests := []struct {
		name       string
		a, b, want []byte
	}{
		{name: "empty a", a: nil, b: []byte("b"), want: []byte("a")},
		// The separator must stay smaller than b even when b ends with zero
		// bytes: decrementing the last byte unconditionally would underflow.
		{name: "empty a, b ends with zero byte", a: nil, b: []byte{1, 0}, want: []byte{0}},
		{name: "empty a, b is a single zero byte", a: nil, b: []byte{0}, want: nil},
		{name: "empty a, b is only zero bytes", a: nil, b: []byte{0, 0, 0}, want: nil},
		{name: "a is prefix of b", a: []byte("abcd"), b: []byte("abcde"), want: []byte("abcd")},
		{name: "a shares prefix with b", a: []byte("abcd"), b: []byte("abce"), want: []byte("abcd")},
		{name: "empty b", a: nil, b: nil, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetSeparatorBetween(test.a, test.b)
			if !bytes.Equal(got, test.want) {
				t.Errorf("GetSeparatorBetween(%q, %q) = %q, want %q", test.a, test.b, got, test.want)
			}
		})
	}
}
