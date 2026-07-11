package commands

// SandboxMode defines three operating modes for the sandbox
type SandboxMode int

const (
	SandboxAutoAllow SandboxMode = iota // Sandbox + Auto-allow (Recommended)
	SandboxRegular                      // Sandbox + Regular permission confirmation
	SandboxOff                          // Disable sandbox
)

// SandboxModeLabels returns the display labels for the three modes
func SandboxModeLabels() []string {
	return []string{
		"Enable Sandbox + Auto-allow (Recommended)",
		"Enable Sandbox + Regular Permissions",
		"Disable Sandbox",
	}
}

// SandboxModeDescriptions returns the description text for each mode
func SandboxModeDescriptions() []string {
	return []string{
		"Commands are automatically executed within the sandbox without confirmation. Explicit deny rules still apply.",
		"Commands are executed within the sandbox but still require permission confirmation.",
		"No OS-level isolation is used; relies only on application-layer permissions.",
	}
}
