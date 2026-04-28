#!/usr/bin/env bash
set -euo pipefail

# resolve repo root from this script location
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
prefix="${PREFIX:-/usr/local}"
bin_dir="${BINDIR:-${prefix}/bin}"
target_bin_dir="${DESTDIR:-}${bin_dir}"
bin_path="${target_bin_dir}/pin"

bash "$repo_dir/scripts/build_binary.sh"

run_install() {
  if [[ -w "$target_bin_dir" ]]; then
    install -m 755 "$repo_dir/pin" "$bin_path"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "$repo_dir/pin" "$bin_path"
    return
  fi

  echo "error: $target_bin_dir is not writable and sudo is not available" >&2
  echo "hint: rerun with BINDIR=/path/to/writable/bin or PREFIX=/path/to/prefix" >&2
  exit 1
}

mkdir_with_privilege() {
  if [[ -d "$target_bin_dir" ]]; then
    return
  fi
  if mkdir -p "$target_bin_dir" 2>/dev/null; then
    return
  fi
  if command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$target_bin_dir"
    return
  fi
  echo "error: could not create $target_bin_dir and sudo is not available" >&2
  exit 1
}

mkdir_with_privilege
run_install

if [[ "$(uname -s)" == "Darwin" ]] && command -v xattr >/dev/null 2>&1; then
  if [[ -w "$bin_path" ]]; then
    xattr -d com.apple.provenance "$bin_path" 2>/dev/null || true
    xattr -d com.apple.quarantine "$bin_path" 2>/dev/null || true
  elif command -v sudo >/dev/null 2>&1; then
    sudo xattr -d com.apple.provenance "$bin_path" 2>/dev/null || true
    sudo xattr -d com.apple.quarantine "$bin_path" 2>/dev/null || true
  fi
fi

echo "Installed $bin_path"
