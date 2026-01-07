#!/usr/bin/env bash
set -euo pipefail

# resolve repo root from this script location
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_path="/usr/local/bin/pin"

"$repo_dir/scripts/build_binary.sh"
sudo cp "$repo_dir/pin" "$bin_path"

echo "Installed $bin_path"
