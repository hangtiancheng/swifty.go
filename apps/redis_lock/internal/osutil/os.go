// Package osutil provides small helpers around the operating system and the
// Go runtime.
package osutil

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// GetCurrentProcessID returns the process id of the current process.
func GetCurrentProcessID() string {
	return strconv.Itoa(os.Getpid())
}

// GetCurrentGoroutineID returns the id of the goroutine that calls it.
func GetCurrentGoroutineID() string {
	buf := make([]byte, 128)
	buf = buf[:runtime.Stack(buf, false)]
	stackInfo := string(buf)
	return strings.TrimSpace(strings.Split(strings.Split(stackInfo, "[running]")[0], "goroutine")[1])
}

// GetProcessAndGoroutineIDStr returns a "<process id>_<goroutine id>" string,
// usable as a unique lock owner token inside a single process.
func GetProcessAndGoroutineIDStr() string {
	return fmt.Sprintf("%s_%s", GetCurrentProcessID(), GetCurrentGoroutineID())
}
