// Copyright (c) 2025 Reliant Labs
package cmd

import (
	"github.com/spf13/cobra"
)

// mergeServerRoles completes the generated `server` command (roots_gen.go:
// the api, worker and gateway roots' parent, with ServerProcessConfig's and
// DatabaseConfig's flags persistent on it) with its help text.
func mergeServerRoles(cmd *cobra.Command) {
	cmd.Short = "Run cloud server components"
	cmd.Long = `Run Reliant cloud server components. Each subcommand starts a specific
server role for split-deployment mode.`
}
