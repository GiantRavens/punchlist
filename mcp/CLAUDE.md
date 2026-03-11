# Punchlist MCP — Implementation & Setup Guide

This is a guide for configuring and extending the Punchlist MCP server.

## Quick Start

### 1. Install dependencies

```bash
cd mcp/
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
```

### 2. Test the server standalone

```bash
source .venv/bin/activate
python3 server.py --root /path/to/your/workspace
# Logs to stderr: "Discovered N domain(s): ['work/quantum', ...]"
# Then waits for MCP stdio input (ctrl-c to exit)
```

### 3. Configure for Claude Code

Add to your project's `.mcp.json` or `~/.claude.json`:

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "/absolute/path/to/mcp/.venv/bin/python3",
      "args": ["/absolute/path/to/mcp/server.py", "--root", "/absolute/path/to/workspace"]
    }
  }
}
```

Or using env var:

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "/absolute/path/to/mcp/.venv/bin/python3",
      "args": ["/absolute/path/to/mcp/server.py"],
      "env": {
        "PUNCHLIST_ROOT": "/absolute/path/to/workspace"
      }
    }
  }
}
```

### 4. Configure for Claude Desktop

Same config format, placed in:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

## Architecture

```
workspace/                    <-- PUNCHLIST_ROOT
+-- .punchlist/config.yaml    <-- root fallback config
+-- home/
|   +-- .punchlist/config.yaml  <-- domain: "home"
|   +-- tasks/
+-- work/
|   +-- .punchlist/config.yaml  <-- domain: "work"
|   +-- quantum/
|       +-- .punchlist/config.yaml  <-- domain: "work/quantum"
|       +-- tasks/
+-- omni/
    +-- .punchlist/config.yaml  <-- domain: "omni"
    +-- tasks/
```

The server walks PUNCHLIST_ROOT recursively for `.punchlist/` directories. Each one becomes a "domain" named by its relative path (e.g., `work/quantum`). The filesystem IS the database — no SQLite, no cache, no sync issues.

### Domain discovery rules

- Root config at `PUNCHLIST_ROOT/.punchlist/config.yaml` provides fallback defaults.
- Each domain config is merged: `{**root_config, **domain_config}`.
- Domains nest to arbitrary depth (`work/quantum`, `code/wargame_engine`, etc.).
- If root itself has a `tasks/` directory, it becomes the `_root` domain.
- Discovery re-runs on every tool call (just a directory walk — fast).

## Tools Reference

| Tool | Purpose | Token Cost |
|------|---------|------------|
| `punchlist_discover` | List all domains + configs + states | Very low |
| `punchlist_list` | Filtered task metadata (no body) | Low |
| `punchlist_get` | Full task with body/notes/log | Medium |
| `punchlist_search` | Full-text across titles+bodies | Medium |
| `punchlist_create` | Create task, auto-ID, update config | Low |
| `punchlist_update` | Change state/priority/tags, add notes | Low |
| `punchlist_summary` | Dashboard: counts by state/priority/tags | Medium |
| `punchlist_cross_domain` | Query across all domains | Medium-High |

### Typical AI Workflow

1. `punchlist_discover` — orient: what domains exist, what states are valid
2. `punchlist_summary(domain="work/quantum")` — get the lay of the land
3. `punchlist_list(domain="work/quantum", state="BEGUN")` — what's in flight
4. `punchlist_get(domain="work/quantum", id=42)` — drill into a specific task
5. `punchlist_update(domain="work/quantum", id=42, add_note="...")` — update it

## Key Design Decisions

### Go CLI Compatibility

The MCP server matches the Go CLI (`pin`) behavior exactly:

- **File naming:** `{id_width-padded}-{slugified-truncated-title}.md`
- **Slug generation:** `[^a-z0-9]+` → `-`, strip edges (matches Go's `slugifyRegex`)
- **Title truncation:** Truncated with `...` at `title_max_len` before slugifying
- **Task ID lookup:** Parse numeric prefix as int (not zero-padded prefix match)
- **Config write-back:** Preserves field order with `sort_keys=False`, atomic via `.tmp`
- **Timestamps:** RFC3339 UTC (`2026-01-07T15:29:31Z`)
- **State timestamps:** `started_at` only for BEGUN, `completed_at` only for DONE
- **Section handling:** `split_section`/`append_entry`/`join_blocks` match Go exactly

### Flexible States Per Domain

Each domain defines its own states. The MCP supports both config formats:

Simple (just ordering):
```yaml
ls_state_order:
  - BEGUN
  - TODO
  - DONE
```

Rich (with aliases and hotkeys):
```yaml
states:
  - name: TODO
    aliases: [todo]
    tui_hotkey: t
  - name: BEGUN
    aliases: [begun, STARTED, started]
    tui_hotkey: b
```

State validation resolves aliases (e.g., "confirm" → FOLLOWUP, "QA" → TEST).

### Metadata-First Listing

`punchlist_list` returns frontmatter ONLY. This is critical for token efficiency: 600 tasks x ~100 bytes metadata = ~60KB vs 600 tasks x ~2KB full content = ~1.2MB. The AI calls `punchlist_get` only when it needs the full story.

### Auto-Logging

State changes, priority changes, and tag changes automatically append to `## Log` with timestamps. Notes go to `## Notes`. `## Log` is always the last section (matching Go's invariant).

### No Caching

Files are small, the OS filesystem cache is fast, and the data changes outside the MCP (via punchlist CLI, direct editing in nvim/Obsidian). A cache would create stale-data bugs. The server re-discovers domains on every tool call.

## Testing without MCP client

```python
from pathlib import Path
from server import PunchlistWorkspace

ws = PunchlistWorkspace(Path("/path/to/workspace"))
for name, domain in ws.domains.items():
    print(f"{name}: {len(domain.task_files())} tasks, states={domain.states}")
```

## Adding a new domain

Create the directory structure — the MCP discovers it automatically:

```bash
mkdir -p career/.punchlist career/tasks
cat > career/.punchlist/config.yaml << 'EOF'
next_id: 1
id_width: 3
ls_state_order:
  - TODO
  - DOING
  - DONE
EOF
```
