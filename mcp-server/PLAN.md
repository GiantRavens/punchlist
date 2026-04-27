# Punchlist MCP Server — Architecture & Plan

## What This Is

An MCP (Model Context Protocol) server that exposes punchlist task domains to AI assistants (Claude Desktop, Claude Code, etc.) with **typed, schema-aware** access. Instead of grepping markdown files, the AI navigates structured data: domains, configs, task metadata, filtered queries.

This is the "Serena for tasks" idea — structured access that prevents token-wasting full-text scans.

## Core Insight

Punchlist already has the structure an MCP needs:

```
<root>/
  .punchlist/config.yaml    ← root config (optional, fallback defaults)
  quantum/
    .punchlist/config.yaml   ← domain config (states, id_width, etc.)
    tasks/
      001-some-task.md       ← structured md+yaml files
      002-another-task.md
  career/
    .punchlist/config.yaml
    tasks/
      001-first-task.md
```

The `.punchlist/config.yaml` per domain defines:
- `next_id` — auto-increment counter
- `id_width` — zero-pad width for filenames
- `ls_state_order` — **the valid states for this domain** (varies per domain!)
- Display/editor preferences (title_max_len, etc.)

Each task file has YAML frontmatter:
- `id` (int), `title` (string), `state` (string from domain's state list)
- `priority` (int, optional), `tags` (list, optional)
- `created_at`, `updated_at`, `started_at`, `completed_at` (ISO 8601)

Body has markdown with `## Notes` and `## Log` sections.

## Design Principles

1. **Read-heavy, write-light.** Primary use: AI queries tasks to build context. Writes happen but are secondary.
2. **Domain-aware.** Every operation is scoped to a domain. States, schemas, and conventions are per-domain.
3. **Config-driven.** The MCP discovers domains by walking the filesystem for `.punchlist/` dirs. No hardcoded paths.
4. **Token-efficient.** Metadata-first: list operations return frontmatter only. Full content on explicit request.
5. **Flexible states.** No hardcoded state machine. Each domain defines its own states in config. The MCP reads them and validates against them.

## MCP Tools

### Discovery

#### `punchlist_discover`
Returns all discovered domains with their configs.

**Parameters:** none (uses configured root path)

**Returns:**
```json
{
  "root": "/path/to/workspace",
  "domains": {
    "quantum": {
      "task_count": 604,
      "states": ["BEGUN", "BLOCK", "TODO", "CONFIRM", "DONE", "NOTDO"],
      "next_id": 605,
      "id_width": 3
    },
    "career": {
      "task_count": 12,
      "states": ["TODO", "DOING", "DONE", "DEFERRED", "NOTDO"],
      "next_id": 13,
      "id_width": 3
    }
  }
}
```

### Query

#### `punchlist_list`
List tasks in a domain with optional filters. Returns **metadata only** (no body content) for token efficiency.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | yes | Domain name (e.g., "quantum") |
| `state` | string or list | no | Filter by state(s) |
| `tag` | string or list | no | Filter by tag(s) |
| `priority` | int | no | Filter by priority |
| `search` | string | no | Substring match on title |
| `since` | string | no | Tasks updated after this ISO date |
| `limit` | int | no | Max results (default 50) |
| `offset` | int | no | Pagination offset |
| `sort` | string | no | Sort field (default: "updated_at") |
| `reverse` | bool | no | Reverse sort order |

**Returns:** Array of task metadata objects (frontmatter only, no body).

#### `punchlist_get`
Get full task content including body, notes, and log.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | yes | Domain name |
| `id` | int | yes | Task ID |

**Returns:** Full task object with parsed frontmatter + body sections.

#### `punchlist_search`
Full-text search across task bodies (not just titles). For when you need semantic-ish search without embeddings.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | no | Scope to domain (omit for cross-domain) |
| `query` | string | yes | Search text |
| `include_body` | bool | no | Include matching body excerpts (default false) |
| `limit` | int | no | Max results (default 20) |

**Returns:** Matching tasks with relevance context.

### Mutation

#### `punchlist_create`
Create a new task.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | yes | Domain name |
| `title` | string | yes | Task title |
| `state` | string | no | Initial state (default: first in config's state order, typically TODO) |
| `priority` | int | no | Priority level |
| `tags` | list | no | Tags |
| `body` | string | no | Initial body content |

**Behavior:** Reads `next_id` from config, creates file `{id}-{kebab-title}.md`, increments `next_id`, writes config back.

#### `punchlist_update`
Update task metadata and/or content.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | yes | Domain name |
| `id` | int | yes | Task ID |
| `state` | string | no | New state (validated against domain's state list) |
| `priority` | int | no | New priority |
| `tags` | list | no | Replace tags |
| `add_tags` | list | no | Append tags |
| `remove_tags` | list | no | Remove specific tags |
| `add_note` | string | no | Append a timestamped note |
| `add_log` | string | no | Append a timestamped log entry |

**Behavior:** State changes auto-log. Timestamps auto-update. Validates state against domain config.

### Aggregation

#### `punchlist_summary`
Dashboard-style overview of a domain or all domains.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | no | Specific domain (omit for all) |

**Returns:**
```json
{
  "quantum": {
    "total": 604,
    "by_state": {"DONE": 175, "TODO": 153, "CONFIRM": 133, ...},
    "by_priority": {"1": 45, "2": 120, "3": 89, "unset": 350},
    "recent_activity": [...],
    "top_tags": [{"tag": "contactlog", "count": 42}, ...]
  }
}
```

#### `punchlist_cross_domain`
Query across all domains simultaneously.

**Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `state` | string or list | no | Filter by state(s) |
| `tag` | string or list | no | Filter by tag(s) |
| `search` | string | no | Title substring |
| `since` | string | no | Updated after date |
| `limit` | int | no | Max results |

**Returns:** Tasks from all domains, each tagged with its domain name.

## Implementation Stack

- **Language:** Python 3.10+
- **MCP SDK:** `mcp` (official Anthropic Python SDK for MCP servers)
- **YAML parsing:** `PyYAML` or `ruamel.yaml` (ruamel preserves formatting on write)
- **Frontmatter parsing:** `python-frontmatter`
- **Transport:** stdio (for Claude Code / Claude Desktop)
- **No database.** Filesystem IS the database. Read on demand, no caching layer (files are small, OS cache handles it).

## File Structure

```
notebook/code/punchlist/
  PLAN.md              ← this file
  CLAUDE.md            ← implementation guide for Claude CLI
  server.py            ← MCP server implementation
  pyproject.toml       ← package config
  README.md            ← user-facing docs
```

## Configuration

The server needs one piece of config: the **root path** to walk for `.punchlist/` directories.

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "python",
      "args": ["/path/to/notebook/code/punchlist/server.py"],
      "env": {
        "PUNCHLIST_ROOT": "/path/to/workspace"
      }
    }
  }
}
```

Or pass as CLI arg: `python server.py --root /path/to/workspace`

## State Validation — The Flexible States Design

This is critical. Each domain defines its own state vocabulary:

```yaml
# quantum/.punchlist/config.yaml
ls_state_order:
  - BEGUN
  - BLOCK
  - TODO
  - CONFIRM
  - DONE
  - NOTDO
```

```yaml
# career/.punchlist/config.yaml  (hypothetical)
ls_state_order:
  - TODO
  - DOING
  - DONE
  - DEFERRED
  - NOTDO
```

The MCP server:
1. Reads `ls_state_order` from each domain's config
2. Uses it for validation on writes (`punchlist_update` rejects invalid states)
3. Exposes it in `punchlist_discover` so the AI knows what states are valid
4. Uses it for ordering in `punchlist_summary` (states appear in config order)

If a domain config doesn't define `ls_state_order`, fall back to root config. If neither defines it, use a sensible default: `["TODO", "DOING", "DONE"]`.

## Token Efficiency Analysis

**Without MCP (grep-based):**
- "What quantum tasks are blocked?" → grep all 604 files → ~100K tokens of raw markdown scanned
- "Create a task" → AI must know the schema, file naming, config update logic

**With MCP:**
- `punchlist_list(domain="quantum", state="BLOCK")` → ~500 tokens response (just matching frontmatter)
- `punchlist_create(domain="quantum", title="...", state="TODO")` → ~200 tokens response
- **Estimated 95%+ token reduction** for typical task operations

## Future Extensions (Not in V1)

- **Semantic search via embeddings** — index task bodies with local embeddings for fuzzy search
- **Task dependencies / links** — frontmatter field for `depends_on: [42, 67]`
- **Archiving** — move completed tasks to `tasks/.archive/` to keep the active set small
- **Webhooks / file watching** — notify on task changes (for Obsidian sync)
- **Obsidian integration** — expose as an Obsidian plugin that reads the same punchlist format
- **Generalized entity types** — same pattern for concepts, decisions, references (the `.concepts` idea)
