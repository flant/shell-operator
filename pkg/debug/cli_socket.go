// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package debug

import (
	"github.com/spf13/cobra"
)

// DefaultSocketPath is the path the debug CLI client (DefaultClient) connects
// to when no explicit socket has been configured. It is a CLI concern only —
// library consumers should build a debug.Client with NewClient(path) directly.
var DefaultSocketPath = "/var/run/shell-operator/debug.socket"

// DefineSocketFlag binds the --debug-unix-socket flag on cmd to the
// process-level DefaultSocketPath used by DefaultClient(). Called by debug
// sub-commands (queue, hook, config, raw).
func DefineSocketFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&DefaultSocketPath, "debug-unix-socket", DefaultSocketPath, "A path to a unix socket for a debug endpoint.")
	_ = cmd.Flags().MarkHidden("debug-unix-socket")
}
