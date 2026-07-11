//go:build linux

package sandbox

import (
	"fmt"
	"os/exec"
	"strings"
)

// linuxSandbox implements sandbox isolation using bubblewrap (bwrap).
// bwrap leverages Linux user namespaces to create lightweight isolated environments.
type linuxSandbox struct{}

func newPlatformSandbox() Sandbox {
	return &linuxSandbox{}
}

func (s *linuxSandbox) Available() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

func (s *linuxSandbox) Wrap(command string, config Config) (string, error) {
	var args []string

	// Isolate user and PID namespaces.
	args = append(args, "bwrap", "--unshare-user", "--unshare-pid")

	// Mount the root filesystem as read-only.
	args = append(args, "--ro-bind", "/", "/")

	// Grant write access per path via writable bind mounts.
	for _, path := range config.AllowWrite {
		args = append(args, "--bind", path, path)
	}

	// Enforce read-only (overrides writable sub-paths under the root mount).
	for _, path := range config.DenyWrite {
		args = append(args, "--ro-bind", path, path)
	}

	// Network isolation.
	if !config.NetworkEnabled {
		args = append(args, "--unshare-net")
	}

	// Mount /proc, required by many commands.
	args = append(args, "--proc", "/proc")

	// Append the command to execute.
	args = append(args, "--", "bash", "-c", command)

	// Assemble the full command string, ensuring proper shell escaping.
	var sb strings.Builder
	for i, arg := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		// Wrap arguments containing whitespace or special characters in single quotes.
		if strings.ContainsAny(arg, " \t\n\"'\\$`!") {
			sb.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\\''")))
		} else {
			sb.WriteString(arg)
		}
	}
	return sb.String(), nil
}
