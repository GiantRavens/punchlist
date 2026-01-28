#!/usr/bin/env bash
set -euo pipefail

# resolve repo root from this script location
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

go build -o pin .
./scripts/gen_help_docs.sh
echo "Built $repo_dir/pin"
