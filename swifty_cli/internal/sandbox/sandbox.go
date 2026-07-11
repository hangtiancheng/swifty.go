package sandbox

// Sandbox defines the unified interface for OS-level sandboxing.
// Each platform (macOS seatbelt / Linux bubblewrap) provides its own implementation.
type Sandbox interface {
	// Wrap wraps the original command into a sandbox-executed command string.
	Wrap(command string, config Config) (string, error)
	// Available checks whether the platform's sandbox tool is available.
	Available() bool
}

// Config controls the sandbox's read, write, and network permissions.
type Config struct {
	AllowWrite     []string // paths where writing is permitted
	DenyWrite      []string // paths that are always read-only (priority over AllowWrite)
	NetworkEnabled bool     // whether network access is permitted
}

// New returns the sandbox implementation for the current platform, or nil if
// the platform is unsupported.
func New() Sandbox {
	return newPlatformSandbox()
}
