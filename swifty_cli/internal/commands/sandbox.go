package commands

// SandboxMode defines three sandbox operating modes
type SandboxMode int

const (
	SandboxAutoAllow SandboxMode = iota // Sandbox + auto-allow (recommended)
	SandboxRegular                      // Sandbox + regular permission confirmation
	SandboxOff                          // Disable sandbox
)

// SandboxModeLabels returns display labels for the three modes
func SandboxModeLabels() []string {
	return []string{
		"Enable Sandbox + Auto-Allow (Recommended)",
		"Enable Sandbox + Regular Permissions",
		"Disable Sandbox",
	}
}

// SandboxModeDescriptions returns explanatory text for each mode
func SandboxModeDescriptions() []string {
	return []string{
		"Commands execute inside a sandbox automatically, no confirmation needed. Explicit deny rules still apply.",
		"Commands execute inside a sandbox, but still require permission confirmation.",
		"No OS-level isolation; relies solely on application-layer permissions.",
	}
}
