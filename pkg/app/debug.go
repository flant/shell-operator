package app

import (
"gopkg.in/alecthomas/kingpin.v2"
)

// DebugUnixSocket is the default path for the debug unix socket.
// It is used as the binding target for the --debug-unix-socket flag on debug
// sub-commands (queue, hook, etc.) that connect to a running operator.
// For the start command, cfg.Debug.UnixSocket is preferred; see flags.go.
var DebugUnixSocket = "/var/run/shell-operator/debug.socket"

// DefineDebugUnixSocketFlag registers the --debug-unix-socket flag on cmd,
// binding it to the DebugUnixSocket global. Called by debug sub-commands that
// need to locate the operator's debug socket.
func DefineDebugUnixSocketFlag(cmd *kingpin.CmdClause) {
cmd.Flag("debug-unix-socket", "A path to a unix socket for a debug endpoint.").
Hidden().
Default(DebugUnixSocket).
StringVar(&DebugUnixSocket)
}
