# Installation

WorkFlow ships as a single self-contained binary, `wf`. There's no runtime to
install — just the executable and (optionally) `git` and a clipboard helper.

## One-line install

```sh
curl -sSfL https://raw.githubusercontent.com/stack-bound/workflow/master/install.sh | sh
```

This script:

- detects your **OS and architecture** (Linux/macOS, amd64/arm64),
- downloads the **latest release** from GitHub,
- installs the `wf` binary to `~/.local/bin` (**no sudo** required),
- adds that directory to your `PATH` if it isn't already (updating your shell
  rc file), and
- installs **shell completions** for bash, zsh, or fish.

### Install somewhere else

Set `INSTALL_DIR` to choose the install location:

```sh
INSTALL_DIR=~/bin curl -sSfL https://raw.githubusercontent.com/stack-bound/workflow/master/install.sh | sh
```

## From source

If you have a Go toolchain, install directly with `go install`:

```sh
go install github.com/stack-bound/workflow/cmd/wf@latest
```

This puts `wf` in your `$GOBIN` (usually `~/go/bin`) — make sure that's on your
`PATH`.

::: tip Building locally
Cloning the repo? `make build` produces `./wf`, and `make install` drops it into
`~/.local/bin` (override with `INSTALL_DIR`), the same place the install script
uses — so there's no duplicate `wf` on your `PATH`.
:::

## Requirements

| Dependency | Required? | Used for |
| --- | --- | --- |
| **git** | Yes | All worktree, branch, status, and merge operations |
| **tmux** | Optional | Real windows per workspace, jump-to-window, sidebar, resurrect |
| **A clipboard helper** | Optional | `wf copy` — `xclip`, `xsel`, `wl-clipboard` (Linux), or `pbcopy` (macOS) |

WorkFlow shells out to `git` and `tmux` rather than reimplementing them, so it
uses whatever versions you already have.

## Verify the install

```sh
wf version
```

You should see the installed version printed. If `wf` isn't found, confirm the
install directory is on your `PATH` (open a new shell, or `source` your rc file).

## Shell completions

The install script sets these up for you. To (re)install or manage them
yourself, see [Shell Integration](/guide/shell-integration#completions).

## Next steps

Head to [Getting Started](/guide/getting-started) to register your first project
and create a workspace.
