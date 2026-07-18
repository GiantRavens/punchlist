# Punchlist MCP — Implementation and Setup

The Punchlist MCP server exposes punchlist task domains to AI assistants over the Model Context Protocol. It is the "Serena for tasks" idea — structured, typed, schema-aware access so an LLM can navigate domains and task metadata without grepping markdown files.

The Go CLI (`pin`) and this MCP are the two sanctioned ways to interact with punchlist task files. **Never edit task `.md` files directly** — both paths preserve frontmatter, `next_id`, and the `## Log` invariant; direct edits do not.

This server is the canonical implementation. An earlier parallel `code/punchlist/mcp-server/` directory was consolidated into this one on 2026-05-22 and archived under `.trash/punchlist/`. The original design rationale lives at `code/punchlist/design/mcp-server.md`.

## Quick start (Linux, primary host)

The server is installed and wired up at session time on futhark. If you need to rebuild:

```bash
cd /home/skip/Desktop/notebook/code/punchlist/mcp
uv venv .venv --python 3.11
uv pip install -p .venv/bin/python -e .
```

Smoke test (the server discovers all `.punchlist/` directories under `--root`, logs to stderr, then waits on stdio):

```bash
.venv/bin/python -c "
from pathlib import Path
import sys; sys.path.insert(0, '.')
from server import PunchlistWorkspace
ws = PunchlistWorkspace(Path('/home/skip/Desktop/notebook'))
print(len(ws.domains), 'domains:', list(ws.domains.keys()))
"
```

Expected: 22 domains across `forge/`, `work/`, `code/`, `cook/`, `write/`, `projects/`, plus the `_root` domain.

## Wiring

Configured in `~/.claude.json` under the top-level `mcpServers` key (user scope — every Claude session on this machine sees it, regardless of cwd):

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "/home/skip/Desktop/notebook/code/punchlist/mcp/.venv/bin/python",
      "args": [
        "/home/skip/Desktop/notebook/code/punchlist/mcp/server.py",
        "--root",
        "/home/skip/Desktop/notebook"
      ]
    }
  }
}
```

The same block also holds `knowledge-forge` and `media-forge`. To add the MCP for other LLM clients (Codex, Cursor, Claude Desktop), use the same `command`/`args` shape — the server is stdio-only and client-agnostic.

Mac note: the `.venv/` is excluded from Syncthing (`.stignore`), so if you ever drive sessions from the Mac side, rebuild the venv locally there. Linux is the primary host; cross-host concerns are a fallback path.

## Architecture

```
PUNCHLIST_ROOT/                       <-- --root argument (or PUNCHLIST_ROOT env)
+-- .punchlist/config.yaml             <-- root fallback config (optional)
+-- forge/knowledge-forge/
|   +-- .punchlist/config.yaml         <-- domain: "forge/knowledge-forge"
|   +-- tasks/
+-- work/quantum/
|   +-- .punchlist/config.yaml         <-- domain: "work/quantum"
|   +-- tasks/
+-- code/punchlist/
    +-- .punchlist/config.yaml         <-- domain: "code/punchlist"
    +-- tasks/
```

The server walks the root recursively for `.punchlist/` directories. Each one becomes a domain named by its path relative to the root (e.g., `work/quantum`). The filesystem IS the database — no SQLite, no cache, no sync issues. Discovery re-runs on every tool call.

If the root itself has a `tasks/` directory, it becomes the `_root` domain.

## Tools

| Tool | Purpose | Token cost |
|---|---|---|
| `punchlist_discover` | List all domains + configs + valid states | Very low |
| `punchlist_list` | Filtered task metadata only (no body) | Low |
| `punchlist_get` | Full task with body, notes, log | Medium |
| `punchlist_search` | Full-text across titles and bodies | Medium |
| `punchlist_create` | Create task, auto-ID, update domain config | Low |
| `punchlist_update` | Change state/priority/tags, add notes, auto-log | Low |
| `punchlist_summary` | Dashboard: counts by state, priority, tags | Medium |
| `punchlist_cross_domain` | Query across all domains at once | Medium-High |

Typical LLM flow: `discover` to orient → `summary` for the lay of the land → `list` filtered → `get` to drill in → `update` to record progress.

## Design decisions worth knowing

**Go CLI parity.** The MCP matches `pin` behavior exactly: file naming `{padded-id}-{slug-from-title}.md`, slug regex `[^a-z0-9]+` → `-`, title truncated with `...` at `title_max_len`, RFC3339 UTC timestamps, atomic config writes via `.tmp`. Either tool can edit a task; the other reads it identically.

**Metadata-first listing.** `punchlist_list` returns frontmatter only. 600 tasks × ~100 bytes ≈ 60 KB instead of ~1.2 MB. `punchlist_get` is on-demand.

**Auto-logging.** State, priority, and tag changes auto-append to `## Log` with timestamps. Notes go to `## Notes`. `## Log` is always last (matches the Go CLI invariant).

**State aliases per domain.** Each domain can define its own states with aliases. `resolve_state("confirm")` maps to `FOLLOWUP` if the domain config declares that alias. Validation runs before any write.

**No caching.** Files are small, the OS page cache is fast, and the data changes outside the MCP (via `pin`, nvim, Obsidian). Re-discovery on every call avoids stale-data bugs.

## Adding a new domain

```bash
mkdir -p some/scope/.punchlist some/scope/tasks
cat > some/scope/.punchlist/config.yaml <<'EOF'
next_id: 1
id_width: 3
ls_state_order:
  - TODO
  - BEGUN
  - DONE
EOF
```

No MCP restart needed — discovery runs on every tool call.
