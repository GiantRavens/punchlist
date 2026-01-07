#!/usr/bin/env bash
set -euo pipefail

repo_dir="$HOME/Desktop/notebook/code/punchlist"
bin_path="/usr/local/bin/pin"

"$repo_dir/scripts/build_binary.sh"
sudo cp "$repo_dir/pin" "$bin_path"

echo "Installed $bin_path"
