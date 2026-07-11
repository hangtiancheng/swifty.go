package consolidation

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const lockFileName = ".consolidate-lock"

// holderStaleMs is the maximum lock hold time; after this duration the lock is
// considered stale even if the holder PID is still alive (guards against PID reuse).
const holderStaleMs = 60 * 60 * 1000

func lockPath(memoryDir string) string {
	return filepath.Join(memoryDir, lockFileName)
}

// ReadLastConsolidatedAt returns the timestamp of the last completed consolidation
// (derived from the lock file's mtime). Returns 0 if the lock file does not exist.
// Per-turn cost: a single stat call.
func ReadLastConsolidatedAt(memoryDir string) (int64, error) {
	info, err := os.Stat(lockPath(memoryDir))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.ModTime().UnixMilli(), nil
}

// TryAcquireLock attempts to acquire the consolidation lock. On success it returns
// the mtime prior to acquisition (used for rollback on failure). On failure it
// returns -1 (lock is held by another process).
//
// Acquisition flow:
//  1. Read the lock file's mtime and PID.
//  2. If the file exists, mtime is within 1 hour, and the PID is alive — give up.
//  3. Otherwise, write own PID.
//  4. Re-read to verify; if PID is not ours — lost the race.
func TryAcquireLock(memoryDir string) (priorMtime int64, err error) {
	path := lockPath(memoryDir)

	var mtimeMs int64
	var holderPid int
	var fileExists bool

	if info, statErr := os.Stat(path); statErr == nil {
		fileExists = true
		mtimeMs = info.ModTime().UnixMilli()
		if raw, readErr := os.ReadFile(path); readErr == nil {
			if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw))); parseErr == nil {
				holderPid = pid
			}
		}
	}

	if fileExists && time.Now().UnixMilli()-mtimeMs < holderStaleMs {
		if holderPid > 0 && isProcessRunning(holderPid) {
			return -1, nil
		}
	}

	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return -1, fmt.Errorf("write lock: %w", err)
	}

	verify, err := os.ReadFile(path)
	if err != nil {
		return -1, fmt.Errorf("verify lock: %w", err)
	}
	if strings.TrimSpace(string(verify)) != strconv.Itoa(os.Getpid()) {
		return -1, nil
	}

	if fileExists {
		return mtimeMs, nil
	}
	return 0, nil
}

// RollbackLock reverts the lock file's mtime to its pre-acquisition value.
// Used to restore state after a failed consolidation. When priorMtime is 0,
// the lock file is deleted entirely.
func RollbackLock(memoryDir string, priorMtime int64) {
	path := lockPath(memoryDir)
	if priorMtime == 0 {
		os.Remove(path)
		return
	}
	// Clear the PID to prevent our own PID from being mistaken as still holding the lock.
	os.WriteFile(path, []byte(""), 0o644)
	t := time.UnixMilli(priorMtime)
	os.Chtimes(path, t, t)
}

// isProcessRunning is implemented in platform-specific files (lock_unix.go / lock_windows.go).
