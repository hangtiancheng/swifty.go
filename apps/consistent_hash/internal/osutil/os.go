// Package osutil provides small helpers built on os and runtime to identify
// the current process and goroutine.
package osutil

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// GetCurrentProcessAndGoroutineIDStr returns an identifier composed of the
// current process id and the current goroutine id, e.g. "42_17".
func GetCurrentProcessAndGoroutineIDStr() string {
	return fmt.Sprintf("%d_%s", GetCurrentProcessID(), GetCurrentGoroutineID())
}

// GetCurrentGoroutineID returns the id of the current goroutine. It parses the
// "goroutine <id> [running]..." header that runtime.Stack always emits.
func GetCurrentGoroutineID() string {
	buf := make([]byte, 128)
	buf = buf[:runtime.Stack(buf, false)]
	stackInfo := string(buf)

	// Drop the "goroutine " prefix and keep the id up to the next space.
	_, rest, _ := strings.Cut(stackInfo, " ")
	id, _, _ := strings.Cut(rest, " ")
	return id
}

// GetCurrentProcessID returns the id of the current process.
func GetCurrentProcessID() int {
	return os.Getpid()
}
