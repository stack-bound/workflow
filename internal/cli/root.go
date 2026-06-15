// Package cli wires WorkFlow's command surface (cobra) onto the engine. Every
// capability is a subcommand; the TUI and future agents drive the same engine.
package cli

import (
	"fmt"
	"os"

	"github.com/mattnelsonuk/workflow"
	"github.com/mattnelsonuk/workflow/internal/config"
	"github.com/mattnelsonuk/workflow/internal/workspace"
	"github.com/spf13/cobra"
)

// Execute runs the root command. It is the single entrypoint from main.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wf: "+err.Error())
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "wf",
		Short:         "WorkFlow — orchestrate git worktree workspaces from one cockpit",
		Long:          "WorkFlow (wf) manages an isolated git worktree per piece of work,\nshows them all with live git status, and lets you review, merge, and clean up.",
		Version:       workflow.Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
		// No subcommand: show the ledger (the dashboard arrives in M2).
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, false)
		},
	}
	// Replace cobra's default "completion" command with our own "completions"
	// (generate + install). The hidden __complete runtime command remains, so
	// generated scripts still work.
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newProjectCmd(),
		newAddCmd(),
		newListCmd(),
		newPathCmd(),
		newOpenCmd(),
		newCopyCmd(),
		newRmCmd(),
		newMergeCmd(),
		newInitCmd(),
		newConfigCmd(),
		newCompletionsCmd(),
		newVersionCmd(),
	)
	return root
}

// manager builds a workspace.Manager from the resolved config and registry.
func manager() (*workspace.Manager, *config.Global, error) {
	g, err := config.LoadGlobal()
	if err != nil {
		return nil, nil, err
	}
	rp, err := config.RegistryPath()
	if err != nil {
		return nil, nil, err
	}
	return workspace.New(rp, g), g, nil
}
