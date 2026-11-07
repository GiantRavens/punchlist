# Punchlist MCP — Implementation and Setup

The Punchlist MCP server exposes punchlist task domains to AI assistants over the Model Context Protocol. It is the "Serena for tasks" idea — structured, typed, schema-aware access so an LLM can navigate domains and task metadata without grepping markdown files.

The Go CLI (`pin`) and this MCP are the two sanctioned ways to interact with punchlist task files. **Never edit task `.md` files directly** — both paths preserve frontmatter, `next_id`, and the `## Log` invariant; direct edits do not.

**Writes shell out to `pin` (since 1.3.8).** The MCP is a thin orchestration layer, not a second writer. `punchlist_create` and `punchlist_update` run the `pin` CLI with the working directory set to the domain's folder (pin resolves its scope by walking up from `cwd`). Only reads are implemented natively here. This is deliberate: task-file mutation has three invariants — disk-floor id allocation (sync-safe), the per-project write lock, and one canonical frontmatter serializer — and a reimplementation in Python drifts from the Go source the moment either side changes. Before 1.3.8 the MCP *did* reimplement all three and carried the pre-1.3.5 dup-id bug as a result. Requirements for the host running the MCP: `pin` must be on `PATH` (or set `PUNCHLIST_PIN_BIN` to its path).

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

Expected (2026-10-21 census): 48 domains across `forge/`, `work/`, `code/`, `cook/`, `write/`, `projects/`, `archive/`, plus the `_root` domain, discovered in about 1s. If startup takes more than a few seconds, the depth bound has been lost. Claude's MCP connect timeout is 30s.

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

The server walks the root for `.punchlist/` directories, **bounded to depth 5** (`PUNCHLIST_MAX_DEPTH` overrides it). Each one becomes a domain named by its path relative to the root (e.g., `work/quantum`). The filesystem IS the database — no SQLite, no cache, no sync issues. Discovery re-runs on tool calls, at most once every 30s.

**Why the depth bound (2026-10-21).** An unbounded walk of the notebook (229K dirs, including bulk forge/batch state) took over 170s. It blew Claude's 30s MCP connect timeout, so the server silently never loaded. Every real store sits at most 4 levels down, so depth 5 leaves a level of headroom and costs about 1s. To check that the bound still covers every store, compare against the full census: `find . \( -name .git -o -name node_modules \) -prune -o -type d -name .punchlist -print`.

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

**Go CLI parity is structural, not mirrored (since 1.3.8).** Writes run `pin`, so file naming, slugging, title truncation, timestamps, id allocation, and the write lock are pin's — there is nothing to keep in sync. Before 1.3.8 these were reimplemented in Python and asserted to "match `pin` exactly"; that promise drifted (the dup-id floor added to the CLI in 1.3.5 was never ported). The create/update argument mapping is the only MCP-side logic: `punchlist_create` → `pin <state> "<title>" [pri:N] [tags:{…}] [--ticket --problem … --approach … --acceptance …]` (the ticket fields need pin 1.3.9+; plus `pin note` for the `body` field); `punchlist_update` → one `pin` call per field (`<state> <id>`, `pri`, `tag`/`tag set`/`tag rm`, `note`).

**Metadata-first listing.** `punchlist_list` returns frontmatter only. 600 tasks × ~100 bytes ≈ 60 KB instead of ~1.2 MB. `punchlist_get` is on-demand.

**Auto-logging.** `pin` auto-appends state and priority changes to `## Log` (tag changes are not logged — they are low-ceremony metadata, matching the CLI). Notes go to `## Notes` via `pin note`. `## Log` is auto-maintained and append-only; the MCP exposes no raw log-injection (the former `add_log` parameter was removed in 1.3.8) — use `add_note` for free-form entries.

**State aliases per domain.** Each domain can define its own states with aliases. `resolve_state("confirm")` maps to `FOLLOWUP` if the domain config declares that alias. Validation runs before any write.

**No task caching.** Files are small, the OS page cache is fast, and the data changes outside the MCP (via `pin`, nvim, Obsidian). Task reads always hit disk. Only the *domain list* is rate-limited, to one rescan per 30s.

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

No MCP restart needed. The new domain appears within 30s, provided it is no more than 5 levels below the root.
