// Command wf is WorkFlow's CLI: a local-first cockpit for git worktree
// workspaces. The CLI is the engine; the dashboard, tmux integration, and
// future agents are layers over the same commands.
package main

import "github.com/mattnelsonuk/workflow/internal/cli"

func main() {
	cli.Execute()
}
