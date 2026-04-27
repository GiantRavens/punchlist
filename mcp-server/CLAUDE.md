# Punchlist MCP — Implementation & Setup Guide

This is a guide for implementing, testing, and configuring the Punchlist MCP server.

## Quick Start

### 1. Install dependencies

```bash
cd notebook/code/punchlist
pip install -e .
# or just:
pip install mcp pyyaml python-frontmatter
```

### 2. Test the server standalone

```bash
# Quick smoke test — does it start and discover domains?
python server.py --root /path/to/your/workspace
# Should log: "Discovered N domain(s): ['quantum', ...]"
# Then wait for MCP stdio input (ctrl-c to exit)
```

### 3. Configure for Claude Code

Add to your Claude Code MCP config (`~/.claude/claude_desktop_config.json` or the project-level `.mcp.json`):

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "python",
      "args": ["/absolute/path/to/notebook/code/punchlist/server.py", "--root", "/absolute/path/to/workspace"],
      "env": {}
    }
  }
}
```

Or using env var:

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "python",
      "args": ["/absolute/path/to/notebook/code/punchlist/server.py"],
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

## Architecture at a Glance

```
workspace/                    ← PUNCHLIST_ROOT
├── .punchlist/config.yaml    ← root fallback config
├── quantum/
│   ├── .punchlist/config.yaml  ← domain config (states, id_width, etc.)
│   └── tasks/
│       ├── 001-some-task.md
│       └── 002-another.md
├── career/
│   ├── .punchlist/config.yaml
│   └── tasks/
└── ...
```

The server walks PUNCHLIST_ROOT looking for `.punchlist/` directories. Each one becomes a "domain." The filesystem IS the database — no SQLite, no cache, no sync issues.

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
2. `punchlist_summary(domain="quantum")` — get the lay of the land
3. `punchlist_list(domain="quantum", state="BEGUN")` — what's in flight
4. `punchlist_get(domain="quantum", id=42)` — drill into a specific task
5. `punchlist_update(domain="quantum", id=42, add_note="...")` — update it

## Key Design Decisions

### Flexible States Per Domain

Each domain defines its own states in `.punchlist/config.yaml`:

```yaml
# quantum — complex workflow
ls_state_order:
  - BEGUN
  - BLOCK
  - TODO
  - CONFIRM
  - DONE
  - NOTDO

# career — simple kanban
ls_state_order:
  - TODO
  - DOING
  - DONE

# code — developer workflow
ls_state_order:
  - TODO
  - DOING
  - DONE
  - DEFERRED
  - NOTDO
```

The MCP validates state changes against the domain's list and rejects invalid ones. No hardcoded state machine.

### Metadata-First Listing

`punchlist_list` returns frontmatter ONLY. This is critical for token efficiency: 604 tasks × ~100 bytes metadata = ~60KB vs 604 tasks × ~2KB full content = ~1.2MB. The AI calls `punchlist_get` only when it needs the full story.

### Auto-Logging

State changes, priority changes, and tag changes automatically append to `## Log` with timestamps. This preserves the audit trail that already exists in the punchlist format.

### No Caching

Files are small, the OS filesystem cache is fast, and the data changes outside the MCP (via punchlist CLI, direct editing in nvim/Obsidian). A cache would create stale-data bugs. The server re-discovers domains on every tool call (just a directory walk — microseconds).

## Implementation Notes for Claude CLI

### Things to validate/fix during implementation:

1. **Test with the real quantum domain** — 604 tasks is a good stress test. Make sure `punchlist_list` with no filters returns quickly and respects the limit.

2. **Frontmatter edge cases** — some tasks may have unusual YAML (single-quoted titles with colons, multiline values, missing fields). The `python-frontmatter` library handles most of this but watch for:
   - Titles with special chars: `title: plan HYCU partnership 'launch'`
   - Truncated titles with `...`: `title: 'Bruno: Set up meeting with IBM and HP executives (Mark Hill, George) to discu...'`

3. **File naming on create** — the `kebab()` function truncates to 80 chars. Make sure it matches whatever the existing punchlist CLI does.

4. **Config write-back** — `increment_id` writes the config back with PyYAML. Verify it preserves existing fields (like `ls_state_order`) and doesn't reorder them. If formatting matters, switch to `ruamel.yaml`.

5. **Cross-domain search performance** — with only a few hundred tasks per domain, linear scan is fine. If any domain grows to thousands, consider adding a simple in-memory index that rebuilds on each call.

6. **MCP SDK version** — the `mcp` Python package is evolving. As of early 2026, the stdio transport and Tool/TextContent types are stable. Check `pip install mcp` gets a compatible version.

### Testing without MCP client

You can test the core logic without an MCP client by importing the workspace directly:

```python
from pathlib import Path
from server import PunchlistWorkspace

ws = PunchlistWorkspace(Path("/path/to/workspace"))
for name, domain in ws.domains.items():
    print(f"{name}: {len(domain.task_files())} tasks, states={domain.states}")
```

### Adding a new domain

Just create the directory structure:

```bash
mkdir -p career/.punchlist career/tasks
cat > career/.punchlist/config.yaml << 'EOF'
next_id: 1
id_width: 3
ls_state_order:
  - TODO
  - DOING
  - DONE
  - DEFERRED
  - NOTDO
EOF
```

The MCP will discover it automatically on next call.

## Future: Generalizing Beyond Tasks

The same pattern works for any entity type. The server could be extended to look for other dot-config directories:

```
quantum/
  .punchlist/config.yaml → tasks/
  .concepts/config.yaml  → concepts/
  .decisions/config.yaml → decisions/
```

Each config defines a schema (required fields, valid states/types, etc.) and the MCP exposes typed operations for each. This would turn the whole workspace into a structured knowledge base with zero database overhead.
