#!/usr/bin/env bash
set -euo pipefail

# resolve repo root from this script location
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

ensure_writable_dir_env() {
  local var_name="$1"
  local fallback="$2"
  local current="${!var_name:-}"

  if [[ -n "$current" && -d "$current" && -w "$current" ]]; then
    return
  fi

  mkdir -p "$fallback"
  export "$var_name=$fallback"
}

resolve_go() {
  if [[ -n "${GO_BIN:-}" ]]; then
    if [[ -x "$GO_BIN" ]]; then
      echo "$GO_BIN"
      return
    fi
    echo "error: GO_BIN is set but is not executable: $GO_BIN" >&2
    exit 127
  fi

  if command -v go >/dev/null 2>&1; then
    command -v go
    return
  fi

  local os_name
  os_name="$(uname -s 2>/dev/null || echo unknown)"
  local candidates=()

  case "$os_name" in
    Darwin)
      candidates+=(
        "/opt/homebrew/bin/go"
        "/usr/local/go/bin/go"
        "/usr/local/bin/go"
        "/opt/local/bin/go"
      )
      ;;
    Linux)
      candidates+=(
        "/usr/local/go/bin/go"
        "/usr/local/bin/go"
        "/usr/bin/go"
        "/bin/go"
        "/opt/go/bin/go"
        "/snap/bin/go"
      )
      ;;
    *)
      candidates+=(
        "/usr/local/go/bin/go"
        "/usr/local/bin/go"
        "/usr/bin/go"
        "/opt/go/bin/go"
      )
      ;;
  esac

  candidates+=(
    "$HOME/.asdf/shims/go"
    "$HOME/.local/share/mise/shims/go"
    "$HOME/.mise/shims/go"
  )

  local candidate
  for candidate in "${candidates[@]}"; do
    if [[ -x "$candidate" ]]; then
      echo "$candidate"
      return
    fi
  done

  echo "error: Go compiler not found." >&2
  echo "Install Go from https://go.dev/dl/ or add its bin directory to PATH." >&2
  echo "If Go is provided by a dev shell or unusual install, rerun with GO_BIN=/absolute/path/to/go." >&2
  echo "Checked PATH plus common $os_name locations." >&2
  echo "Current PATH: ${PATH:-<empty>}" >&2
  exit 127
}

uid_value="$(id -u 2>/dev/null || echo user)"
ensure_writable_dir_env TMPDIR "/tmp/punchlist-${uid_value}/tmp"
ensure_writable_dir_env GOTMPDIR "$TMPDIR"
ensure_writable_dir_env GOCACHE "/tmp/punchlist-${uid_value}/go-build"

VERSION="$(tr -d '[:space:]' < VERSION)"
GO_BIN="$(resolve_go)"
"$GO_BIN" build -ldflags "-X punchlist/cmd.Version=${VERSION}" -o pin .
bash "$repo_dir/scripts/gen_help_docs.sh"
echo "Built $repo_dir/pin"
