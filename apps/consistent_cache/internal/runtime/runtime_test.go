package runtime

import (
	"os"
	"strconv"
	"testing"
)

// TestGetCurrentProcessID verifies the process id helper.
func TestGetCurrentProcessID(t *testing.T) {
	if got := GetCurrentProcessID(); got != os.Getpid() {
		t.Errorf("GetCurrentProcessID() = %d, want %d", got, os.Getpid())
	}
}

// TestGetCurrentGoroutineID verifies that the goroutine id is a positive number.
func TestGetCurrentGoroutineID(t *testing.T) {
	got := GetCurrentGoroutineID()
	id, err := strconv.Atoi(got)
	if err != nil {
		t.Fatalf("GetCurrentGoroutineID() = %q, want a numeric id", got)
	}
	if id <= 0 {
		t.Errorf("GetCurrentGoroutineID() = %q, want a positive id", got)
	}
}

// TestGetCurrentProcessAndGoroutineIDStr verifies the combined identifier format.
func TestGetCurrentProcessAndGoroutineIDStr(t *testing.T) {
	want := strconv.Itoa(os.Getpid()) + "_" + GetCurrentGoroutineID()
	if got := GetCurrentProcessAndGoroutineIDStr(); got != want {
		t.Errorf("GetCurrentProcessAndGoroutineIDStr() = %q, want %q", got, want)
	}
}
