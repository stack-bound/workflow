# Shell Integration

Two small touches make WorkFlow feel native in your shell: a `cd` helper for
jumping into workspaces, and tab completions for every command.

## The `cd` helper {#cd-helper}

A shell can't change *your* current directory from a subprocess, so `wf` can't
`cd` for you directly. Instead, `wf path <branch>` prints a workspace's path, and
a one-line shell function turns that into a jump:

::: code-group

```sh [bash / zsh]
# add to ~/.bashrc or ~/.zshrc
wfd() { cd "$(wf path "$1")"; }
```

```fish [fish]
# add to ~/.config/fish/config.fish
function wfd
    cd (wf path $argv[1])
end
```

:::

Then:

```sh
wfd feature-x      # cd into the feature-x workspace
```

Scope to a project when a branch name is ambiguous by extending the helper to
pass `--project`, or just call `wf path feature-x --project myrepo` directly.

## Completions

WorkFlow generates completion scripts for **bash**, **zsh**, **fish**, and
**PowerShell**. The [install script](/guide/installation) sets these up
automatically; you can also manage them yourself.

### Install

```sh
wf completions install          # auto-detect $SHELL and install
wf completions install zsh      # or name the shell: bash, zsh, fish
wf completions install --force  # overwrite an existing completion file
```

`install` writes to the standard per-user location for your shell:

| Shell | Installed to | Notes |
| --- | --- | --- |
| **bash** | `$XDG_DATA_HOME/bash-completion/completions/wf` | Auto-loaded; start a new shell |
| **zsh** | `$XDG_DATA_HOME/zsh/site-functions/_wf` | Ensure the dir is on your `fpath` before `compinit` |
| **fish** | `$XDG_CONFIG_HOME/fish/completions/wf.fish` | Auto-loaded; start a new shell |

For zsh, the command prints the one-time lines to add to `~/.zshrc`:

```sh
fpath=($XDG_DATA_HOME/zsh/site-functions $fpath)
autoload -U compinit && compinit
```

### Print to stdout

To place the script yourself (or for PowerShell, which `install` doesn't handle),
print it:

```sh
wf completions zsh > _wf
wf completions bash > /etc/bash_completion.d/wf
wf completions powershell      # print the PowerShell script
```

(`$XDG_DATA_HOME` defaults to `~/.local/share`; `$XDG_CONFIG_HOME` to
`~/.config`.)
