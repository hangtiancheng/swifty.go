package sandbox

// Sandbox defines a unified interface for OS-level sandboxing.
// Each platform (macOS seatbelt / Linux bubblewrap) provides its own implementation.
type Sandbox interface {
	// Wrap wraps the original command string so it executes inside the sandbox.
	Wrap(command string, config Config) (string, error)
	// Available reports whether the sandbox tool for the current platform is available.
	Available() bool
}

// Config controls the sandbox's write and network permissions.
type Config struct {
	AllowWrite     []string // Paths permitted for write access.
	DenyWrite      []string // Paths that are always read-only (takes precedence over AllowWrite).
	NetworkEnabled bool     // Whether network access is allowed.
}

// New returns the sandbox implementation for the current platform.
// Returns nil on unsupported platforms.
func New() Sandbox {
	return newPlatformSandbox()
}
