package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd prints the wf version. The version itself is set on the root
// command (see newRootCmd) from the embedded VERSION file, so `wf version` and
// `wf --version` always agree.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the wf version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "wf version %s\n", cmd.Root().Version)
			return nil
		},
	}
}
