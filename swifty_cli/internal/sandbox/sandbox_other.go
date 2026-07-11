//go:build !darwin && !linux

package sandbox

// Unsupported platforms return nil; callers must check for nil.
func newPlatformSandbox() Sandbox {
	return nil
}
