// Package app intentionally no longer exposes the DebugUnixSocket global or
// ApplyConfig helper that used to live here. The source of truth for the
// debug socket path is now cfg.Debug.UnixSocket (see app_config.go); CLI
// commands that need to connect to a running operator bind their
// --debug-unix-socket flag against the helper in package debug instead.
//
// This file is kept (empty) so external code that imports the package keeps
// compiling; new code should not add package-level mutable state here.
package app
