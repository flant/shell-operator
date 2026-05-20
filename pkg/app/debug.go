package app

import (
	"github.com/spf13/cobra"
)

// DebugUnixSocket is the default path for the debug unix socket.
// It is used as the binding target for the --debug-unix-socket flag on debug
// sub-commands (queue, hook, etc.) that connect to a running operator.
// For the start command, cfg.Debug.UnixSocket is preferred; see flags.go.
var DebugUnixSocket = "/var/run/shell-operator/debug.socket"

// ApplyConfig copies values from cfg into package-level globals that pre-date
// the Config struct and therefore cannot be populated by caarlos0/env (today
// just DebugUnixSocket).
//
// Call it whenever cfg may have been built outside the CLI flow — most
// importantly when an outer program (e.g. addon-operator) assembles its own
// *Config and hands it to shell-operator without going through BindFlags.
// After this call, debug sub-commands that bind --debug-unix-socket against
// the global will see cfg.Debug.UnixSocket as their default.
//
// A nil cfg is a no-op so callers don't need to guard. ApplyConfig is also
// invoked from BindFlags and shell_operator.Init, so most users get the
// override for free; calling it again is safe and idempotent.
func ApplyConfig(cfg *Config) {
	if cfg == nil {
		return
	}
	DebugUnixSocket = cfg.Debug.UnixSocket
}

// DefineDebugUnixSocketFlag registers the --debug-unix-socket flag on cmd,
// binding it to the DebugUnixSocket global. Called by debug sub-commands that
// need to locate the operator's debug socket.
func DefineDebugUnixSocketFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&DebugUnixSocket, "debug-unix-socket", DebugUnixSocket, "A path to a unix socket for a debug endpoint.")
	_ = cmd.Flags().MarkHidden("debug-unix-socket")
}
