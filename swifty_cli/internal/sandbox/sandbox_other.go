//go:build !darwin && !linux

package sandbox

// newPlatformSandbox returns nil on unsupported platforms.
// Callers must check for nil before use.
func newPlatformSandbox() Sandbox {
	return nil
}
