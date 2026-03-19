#!/bin/sh
# Canopy installer — https://github.com/isacssw/canopy
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/isacssw/canopy/main/install.sh | sh
#
# Environment variables:
#   CANOPY_VERSION  — install a specific version (e.g. "0.3.1"), default: latest
#   INSTALL_DIR     — where to place the binary, default: ~/.local/bin

set -eu

# ── Helpers ──────────────────────────────────────────────────────────────────

BOLD="" DIM="" RED="" GREEN="" YELLOW="" CYAN="" RESET=""
if [ -t 1 ]; then
  BOLD='\033[1m'  DIM='\033[2m'  RED='\033[31m'  GREEN='\033[32m'
  YELLOW='\033[33m'  CYAN='\033[36m'  RESET='\033[0m'
fi

info()  { printf "${CYAN}info${RESET}  %s\n" "$*"; }
warn()  { printf "${YELLOW}warn${RESET}  %s\n" "$*"; }
error() { printf "${RED}error${RESET} %s\n" "$*" >&2; }
die()   { error "$@"; exit 1; }

TMPDIR_CLEANUP=""
cleanup() {
  if [ -n "$TMPDIR_CLEANUP" ] && [ -d "$TMPDIR_CLEANUP" ]; then
    rm -rf "$TMPDIR_CLEANUP"
  fi
}
trap cleanup EXIT INT TERM

usage() {
  cat <<EOF
Canopy installer

Usage:
  install.sh [options]

Options:
  --help    Show this help message

Environment variables:
  CANOPY_VERSION   Install a specific version (default: latest)
  INSTALL_DIR      Installation directory (default: ~/.local/bin)
EOF
}

# ── Argument parsing ─────────────────────────────────────────────────────────

for arg in "$@"; do
  case "$arg" in
    --help|-h) usage; exit 0 ;;
  esac
done

# ── Platform detection ───────────────────────────────────────────────────────

detect_platform() {
  OS="$(uname -s)"
  ARCH="$(uname -m)"

  case "$OS" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)      die "Unsupported OS: $OS (only Linux and macOS are supported)" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)       ARCH="amd64" ;;
    aarch64|arm64)      ARCH="arm64" ;;
    *)                  die "Unsupported architecture: $ARCH (only amd64 and arm64 are supported)" ;;
  esac
}

# ── HTTP client ──────────────────────────────────────────────────────────────

has_cmd() { command -v "$1" >/dev/null 2>&1; }

download() {
  url="$1" dest="$2"
  if has_cmd curl; then
    curl -fsSL -o "$dest" "$url"
  elif has_cmd wget; then
    wget -qO "$dest" "$url"
  else
    die "Neither curl nor wget found. Please install one and retry."
  fi
}

fetch() {
  url="$1"
  if has_cmd curl; then
    curl -fsSL "$url"
  elif has_cmd wget; then
    wget -qO- "$url"
  else
    die "Neither curl nor wget found. Please install one and retry."
  fi
}

# ── Version resolution ───────────────────────────────────────────────────────

resolve_version() {
  if [ -n "${CANOPY_VERSION:-}" ]; then
    VERSION="$CANOPY_VERSION"
    info "Using requested version: v${VERSION}"
    return
  fi

  info "Fetching latest release..."
  LATEST_URL="https://api.github.com/repos/isacssw/canopy/releases/latest"
  TAG="$(fetch "$LATEST_URL" | grep '"tag_name"' | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"
  [ -z "$TAG" ] && die "Could not determine latest version from GitHub API"
  VERSION="${TAG#v}"
  info "Latest version: v${VERSION}"
}

# ── Download & verify ────────────────────────────────────────────────────────

download_and_verify() {
  TMPDIR_CLEANUP="$(mktemp -d)"
  ARCHIVE="canopy_${OS}_${ARCH}.tar.gz"
  ARCHIVE_URL="https://github.com/isacssw/canopy/releases/download/v${VERSION}/${ARCHIVE}"
  CHECKSUMS_URL="https://github.com/isacssw/canopy/releases/download/v${VERSION}/checksums.txt"

  info "Downloading canopy v${VERSION} (${OS}/${ARCH})..."
  download "$ARCHIVE_URL" "${TMPDIR_CLEANUP}/${ARCHIVE}"
  download "$CHECKSUMS_URL" "${TMPDIR_CLEANUP}/checksums.txt"

  info "Verifying checksum..."
  EXPECTED="$(grep -F "  ${ARCHIVE}" "${TMPDIR_CLEANUP}/checksums.txt" | awk '{print $1}')"
  [ -z "$EXPECTED" ] && die "Checksum not found for ${ARCHIVE} in checksums.txt"

  if has_cmd sha256sum; then
    ACTUAL="$(sha256sum "${TMPDIR_CLEANUP}/${ARCHIVE}" | awk '{print $1}')"
  elif has_cmd shasum; then
    ACTUAL="$(shasum -a 256 "${TMPDIR_CLEANUP}/${ARCHIVE}" | awk '{print $1}')"
  else
    die "Neither sha256sum nor shasum found — cannot verify download integrity"
  fi

  if [ "$EXPECTED" != "$ACTUAL" ]; then
    die "Checksum mismatch!\n  Expected: ${EXPECTED}\n  Actual:   ${ACTUAL}"
  fi
}

# ── Install ──────────────────────────────────────────────────────────────────

install_binary() {
  INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

  info "Extracting..."
  tar -xzf "${TMPDIR_CLEANUP}/${ARCHIVE}" -C "${TMPDIR_CLEANUP}" --no-same-owner
  [ -f "${TMPDIR_CLEANUP}/canopy" ] || die "Archive did not contain expected 'canopy' binary"

  mkdir -p "$INSTALL_DIR"
  mv "${TMPDIR_CLEANUP}/canopy" "${INSTALL_DIR}/canopy"
  chmod +x "${INSTALL_DIR}/canopy"

  # macOS: remove quarantine attribute
  if [ "$OS" = "darwin" ]; then
    xattr -d com.apple.quarantine "${INSTALL_DIR}/canopy" 2>/dev/null || true
  fi

  info "Installed to ${INSTALL_DIR}/canopy"
}

# ── Post-install checks ─────────────────────────────────────────────────────

check_tmux() {
  if ! has_cmd tmux; then
    printf "\n"
    warn "tmux is required but not found on your PATH."
    printf "  Install it with your package manager:\n"
    if [ "$OS" = "darwin" ]; then
      printf "    ${DIM}brew install tmux${RESET}\n"
    else
      printf "    ${DIM}# Debian/Ubuntu${RESET}\n"
      printf "    ${DIM}sudo apt install tmux${RESET}\n\n"
      printf "    ${DIM}# Fedora${RESET}\n"
      printf "    ${DIM}sudo dnf install tmux${RESET}\n\n"
      printf "    ${DIM}# Arch${RESET}\n"
      printf "    ${DIM}sudo pacman -S tmux${RESET}\n"
    fi
  fi
}

check_path() {
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      printf "\n"
      warn "${INSTALL_DIR} is not on your PATH."
      printf "  Add the following to your shell config (~/.bashrc, ~/.zshrc, etc.):\n"
      printf "    ${DIM}export PATH=\"%s:\$PATH\"${RESET}\n" "$INSTALL_DIR"
      ;;
  esac
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
  printf "\n  ${BOLD}Canopy Installer${RESET}\n\n"

  detect_platform
  resolve_version
  download_and_verify
  install_binary
  check_tmux
  check_path

  printf "\n  ${GREEN}✓${RESET} ${BOLD}canopy v${VERSION}${RESET} installed successfully!\n"
  printf "    ${DIM}Run 'canopy --help' to get started.${RESET}\n\n"
}

main
