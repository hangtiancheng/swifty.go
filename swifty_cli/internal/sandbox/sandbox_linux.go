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

	// Isolate user and pid namespaces.
	args = append(args, "bwrap", "--unshare-user", "--unshare-pid")

	// Read-only bind mount of the root filesystem.
	args = append(args, "--ro-bind", "/", "/")

	// Allow write per path (writable bind mounts).
	for _, path := range config.AllowWrite {
		args = append(args, "--bind", path, path)
	}

	// Force read-only (overrides writable sub-paths under root).
	for _, path := range config.DenyWrite {
		args = append(args, "--ro-bind", path, path)
	}

	// Network isolation.
	if !config.NetworkEnabled {
		args = append(args, "--unshare-net")
	}

	// Mount /proc; many commands depend on it.
	args = append(args, "--proc", "/proc")

	// Append the command to execute.
	args = append(args, "--", "bash", "-c", command)

	// Assemble the full command string; shell-special characters must be
	// properly escaped.
	var sb strings.Builder
	for i, arg := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		// Wrap arguments containing spaces or special characters in single quotes.
		if strings.ContainsAny(arg, " \t\n\"'\\$`!") {
			sb.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\\''")))
		} else {
			sb.WriteString(arg)
		}
	}
	return sb.String(), nil
}
