package teams

import (
	"os"
	"runtime"
)

// detectBackend mirrors Claude Code: the pane backend is used only when already
// inside a tmux / iTerm2 session; otherwise it falls back to in-process. The
// decision is based on environment variables — tmux and iTerm2 automatically
// set TMUX / ITERM_SESSION_ID for processes in their sessions, so no manual
// configuration is required.
func detectBackend() TeamMode {
	// Windows guardrail: spawning a tmux pane would run POSIX commands through
	// pwsh and fail, so always stay in-process.
	if runtime.GOOS == "windows" {
		return ModeInProcess
	}
	return detectBackendFromEnv()
}

// detectBackendFromEnv decides purely from environment variables; it is split
// out for unit testing (independent of the host platform).
func detectBackendFromEnv() TeamMode {
	if os.Getenv("TMUX") != "" {
		return ModeTmux
	}
	if os.Getenv("ITERM_SESSION_ID") != "" {
		return ModeITerm
	}
	return ModeInProcess
}
