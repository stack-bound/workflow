#!/bin/sh
set -e

# Installs the wf binary without sudo. By default it goes to ~/.local/bin and
# that directory is added to your PATH if it isn't already.
REPO="mattnelsonuk/workflow"
BINARY="wf"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)
    echo "Error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# Get latest version
echo "Fetching latest release..."
VERSION="$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')"
if [ -z "$VERSION" ]; then
  echo "Error: could not determine latest version" >&2
  exit 1
fi
echo "Latest version: v${VERSION}"

# Download
FILENAME="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -sSfL -o "${TMPDIR}/${FILENAME}" "$URL"

# Extract
tar -xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

# Install to a user-writable directory (no sudo)
mkdir -p "$INSTALL_DIR"
mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"
echo "${BINARY} v${VERSION} installed to ${INSTALL_DIR}/${BINARY}"

# Make sure INSTALL_DIR is on PATH, adding it to the shell's rc file if needed.
ensure_on_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*)
      return 0  # already on PATH
      ;;
  esac

  SHELL_NAME="$(basename "${SHELL:-}")"
  case "$SHELL_NAME" in
    fish)
      rc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish"
      line="fish_add_path \"$INSTALL_DIR\""
      reload="exec fish"
      ;;
    zsh)
      rc="$HOME/.zshrc"
      line="export PATH=\"$INSTALL_DIR:\$PATH\""
      reload="source $rc"
      ;;
    bash)
      rc="$HOME/.bashrc"
      line="export PATH=\"$INSTALL_DIR:\$PATH\""
      reload="source $rc"
      ;;
    *)
      rc="$HOME/.profile"
      line="export PATH=\"$INSTALL_DIR:\$PATH\""
      reload="source $rc"
      ;;
  esac

  mkdir -p "$(dirname "$rc")"
  if [ -f "$rc" ] && grep -qF "$INSTALL_DIR" "$rc" 2>/dev/null; then
    : # an entry for this directory is already present
  else
    printf '\n# Added by the workflow (wf) installer\n%s\n' "$line" >> "$rc"
    echo "Added ${INSTALL_DIR} to your PATH in ${rc}"
  fi
  echo "Restart your shell, or run: ${reload}"
}

ensure_on_path

# Install shell completions using wf's own installer (auto-detects your shell).
echo "Installing shell completions..."
if ! "${INSTALL_DIR}/${BINARY}" completions install --force; then
  echo "Could not auto-install completions. Run '${BINARY} completions install' after restarting your shell."
fi
