package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/ide"
	"github.com/stack-bound/workflow/internal/picker"
	"github.com/stack-bound/workflow/internal/workspace"
)

func newEditCmd() *cobra.Command {
	var project string
	var pick bool
	var list bool
	cmd := &cobra.Command{
		Use:   "edit [branch]",
		Short: "Open a workspace in an editor or IDE (interactive picker)",
		Long: "Open an editor on a workspace. With no argument it opens the current " +
			"directory; pass a branch to target another workspace. A picker lists the " +
			"editors detected on this machine (the repo's default first); pick one with " +
			"the arrow keys and Enter. When the repo has autolaunch set, its default " +
			"opens straight away — use --pick to choose anyway.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, g, err := manager()
			if err != nil {
				return err
			}
			ides := ide.Detect(g)

			if list {
				return printIDEs(cmd, ides)
			}

			dir, proj, err := resolveEditTarget(m, args, project)
			if err != nil {
				return err
			}

			defaultID, autolaunch := "", false
			if proj != "" {
				defaultID, autolaunch, _ = m.ProjectIDEPrefs(proj)
			}
			if defaultID == "" {
				defaultID = g.DefaultIDE
			}

			// Autolaunch: open the default without the picker (unless --pick).
			if !pick && autolaunch && defaultID != "" {
				if i, ok := ide.Find(ides, defaultID); ok {
					return launchIDE(cmd, i, dir)
				}
			}

			res, err := picker.Run(ides, defaultID)
			if err != nil {
				return err
			}
			switch res.Action {
			case picker.Launch:
				return launchIDE(cmd, res.IDE, dir)
			case picker.SetDefault:
				return persistDefaultIDE(cmd, m, g, proj, res.IDE, false)
			case picker.SetDefaultAutolaunch:
				return persistDefaultIDE(cmd, m, g, proj, res.IDE, true)
			default: // picker.Cancel
				return nil
			}
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	cmd.Flags().BoolVarP(&pick, "pick", "i", false, "always show the picker, even when autolaunch is set")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list the editors detected on this machine and exit")
	return cmd
}

// resolveEditTarget returns the directory to open and the project that owns it.
// With a branch argument it resolves that workspace; otherwise it opens the
// current directory and finds its owning project (empty when unregistered).
func resolveEditTarget(m *workspace.Manager, args []string, project string) (dir, proj string, err error) {
	if len(args) == 1 {
		wt, err := m.Resolve(args[0], project)
		if err != nil {
			return "", "", err
		}
		return wt.Path, wt.Project, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	proj, _ = m.ProjectForDir(cwd)
	return cwd, proj, nil
}

// launchIDE opens dir in the editor: a GUI app detached so the shell returns
// immediately, a terminal editor attached so it can take over the terminal.
func launchIDE(cmd *cobra.Command, i ide.IDE, dir string) error {
	c := ide.LaunchCmd(i, dir)
	if i.GUI {
		if err := ide.RunDetached(c); err != nil {
			return fmt.Errorf("launch %s: %w", i.Name, err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Launched %s\n", i.Name)
		return nil
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("open %s: %w", i.Name, err)
	}
	return nil
}

// persistDefaultIDE records the picker's choice. With an owning project it writes
// to that repo's .workFlow.yaml; otherwise it falls back to the global default.
// alsoAutolaunch toggles autolaunch (off when the editor is already the
// autolaunching default, on otherwise).
func persistDefaultIDE(cmd *cobra.Command, m *workspace.Manager, g *config.Global, project string, i ide.IDE, alsoAutolaunch bool) error {
	out := cmd.OutOrStdout()
	if project == "" {
		g.DefaultIDE = i.ID
		if err := config.SaveGlobal(g); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Set %s as the global default editor\n", i.Name)
		return nil
	}
	curDefault, curAuto, _ := m.ProjectIDEPrefs(project)
	autolaunch := curAuto
	if alsoAutolaunch {
		// Toggle: turn autolaunch off only when this editor is already the
		// autolaunching default, otherwise turn it on.
		autolaunch = !curAuto || curDefault != i.ID
	}
	if err := m.SetProjectDefaultIDE(project, i.ID, autolaunch); err != nil {
		return err
	}
	switch {
	case alsoAutolaunch && autolaunch:
		_, _ = fmt.Fprintf(out, "%s is now the default for %s, autolaunch on\n", i.Name, project)
	case alsoAutolaunch && !autolaunch:
		_, _ = fmt.Fprintf(out, "Autolaunch off for %s\n", project)
	default:
		_, _ = fmt.Fprintf(out, "%s is now the default editor for %s\n", i.Name, project)
	}
	return nil
}

// printIDEs lists detected editors and their ids (for default_ide / config).
func printIDEs(cmd *cobra.Command, ides []ide.IDE) error {
	out := cmd.OutOrStdout()
	if len(ides) == 0 {
		_, _ = fmt.Fprintln(out, "No editors detected. Add one under `ides:` in your global config.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tKIND")
	for _, i := range ides {
		kind := "terminal"
		if i.GUI {
			kind = "gui"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", i.ID, i.Name, kind)
	}
	return tw.Flush()
}
