#!/usr/bin/env bash
set -euo pipefail

repo_dir="$HOME/Desktop/notebook/code/punchlist"
cd "$repo_dir"

go build -o pin .
echo "Built $repo_dir/pin"
