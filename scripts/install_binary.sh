#!/usr/bin/env bash
set -euo pipefail

# resolve repo root from this script location
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_path="/usr/local/bin/pin"

"$repo_dir/scripts/build_binary.sh"
sudo cp "$repo_dir/pin" "$bin_path"
sudo chmod 755 "$bin_path"
sudo xattr -d com.apple.provenance "$bin_path" 2>/dev/null || true
sudo xattr -d com.apple.quarantine "$bin_path" 2>/dev/null || true

echo "Installed $bin_path"
