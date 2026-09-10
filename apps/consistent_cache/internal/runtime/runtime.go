// Package runtime provides helpers to identify the current process and goroutine.
package runtime

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// GetCurrentProcessAndGoroutineIDStr returns an identifier string composed of
// the current process id and the current goroutine id.
func GetCurrentProcessAndGoroutineIDStr() string {
	pid := GetCurrentProcessID()
	goroutineID := GetCurrentGoroutineID()
	return fmt.Sprintf("%d_%s", pid, goroutineID)
}

// GetCurrentGoroutineID returns the id of the current goroutine.
func GetCurrentGoroutineID() string {
	buf := make([]byte, 128)
	buf = buf[:runtime.Stack(buf, false)]
	stackInfo := string(buf)
	return strings.TrimSpace(strings.Split(strings.Split(stackInfo, "[running]")[0], "goroutine")[1])
}

// GetCurrentProcessID returns the id of the current process.
func GetCurrentProcessID() int {
	return os.Getpid()
}
