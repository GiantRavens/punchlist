```
  o
+--\----+
|   \   |
| punch |
|  list |
+-------+
```

# punchlist

A lightweight, local, markdown-native task and ticket system for humans and AI agents — no database, no app, no account required.

## Why punchlist?

Most task tools are apps: databases behind a UI, locked to a service, requiring context-switches away from your work. GitHub Issues are powerful but heavy — and don't travel with your local project. Plain text files are too loose to query or automate.

Punchlist sits in the middle: **every task is a plain markdown file with YAML frontmatter**, committed alongside your code, editable in any text editor, queryable with `--json`, and legible to both humans and AI agents without translation.

- **No database.** Tasks are `.md` files in a `tasks/` folder.
- **No app.** Open any task in nvim, VS Code, Obsidian, or anything else.
- **No lock-in.** Plain markdown and YAML. If you stop using `pin`, your files are still there.
- **Travels with your project.** Commit your tasks alongside your code.
- **Works with your AI assistant.** JSON output, structured metadata, dependency tracking, and acceptance criteria make punchlist a first-class context layer for AI agents and coding tools.

---

## Quick start

```bash
pin init                              # initialize any folder as a punchlist project
pin "write release plan" pri:1        # create a task (defaults to TODO)
pin ls                                # list all tasks
pin browse                            # interactive TUI browser
```

---

## Creating tasks

The `pin` command grammar is natural and tolerant. State is optional; if omitted, the task defaults to TODO.

```bash
pin "write outline"
pin todo "draft messaging brief" pri:1
pin todo "send pr draft" by:2026-01-15
pin todo "ship notes" by:tomorrow
pin todo "review plan" by:friday
pin todo "write release plan" pri:1 by:2026-01-09 tags:{launch,pr}
pin todo "Deploy to prod" depends:3,4 pri:1
pin done "shipped quick fix for onboarding typo"
pin begun "triage the backlog"
pin block "waiting on vendor response"
pin todo ../work "queue follow-up"    # create in another project folder
```

Each task gets a numbered markdown file (`tasks/001-write-outline.md`) with YAML frontmatter and a body you can freely edit.

**Scope:** Each initialized folder has its own independent task list. Use an explicit path to target a different project:

```bash
pin ls ../work                        # list tasks in another project
pin todo ../work "queue follow-up"    # create a task in a specific project
```

`pin` walks up from the current directory to find the nearest `.punchlist/` folder, so it's usually correct — but worth checking when working across sibling projects.

---

## Listing, searching, and inspecting tasks

```bash
pin ls                        # all tasks, grouped by state
pin ls todo                   # filter by state
pin ls todo --tag launch      # filter by tag
pin ls --order id             # sort by id instead of state
pin ls todo --chunk 10        # show first 10 matches
pin ls todo --chunk 10 --page 2
pin ls todo --by-priority
pin ls todo --by-date-reverse
pin search "release"          # full-text search across title, body, frontmatter
pin show 12                   # full task detail
```

### Machine-readable JSON output

All read commands support `--json` for use in scripts and AI pipelines:

```bash
pin ls --json
pin ls todo --json
pin show --json 12
pin search --json "keyword"
pin ls --ready --json | jq '.[0].id'    # next actionable task
```

`pin ls --json` returns a task array without body. `pin show --json` includes the full body, metadata, and acceptance criteria.

---

## Task states

States are fully configurable. Default states from `pin init`:

| State | Aliases | Browse hotkey |
|---|---|---|
| `TODO` | todo | `t` |
| `BEGUN` | begun, started, inprogress | `s` |
| `BLOCKED` | blocked, block | `b` |
| `FOLLOWUP` | followup, confirm | `f` |
| `DEFER` | defer | `z` |
| `NOTDO` | notdo | `x` |
| `DONE` | done, complete, completed | `d` |

Existing projects whose `config.yaml` spells out `states:` keep their configured hotkeys. A configured hotkey that collides with a browse navigation key (e.g. the old default `l` for DEFER, now next-task) is shadowed — navigation wins and the hotkey is dropped from the browse help line. Update the `tui_hotkey:` entries in your config to adopt the new scheme.

Change state with any state token:

```bash
pin start 12
pin done 12
pin block 12
pin todo 12
pin REDLIGHT 12           # any custom state token works
pin begun "triage the backlog"    # state + new task title creates a task in that state
```

Add or rename states by editing `states:` in `.punchlist/config.yaml`. All commands respect the configured ordering.

---

## Browse

```bash
pin browse
pin browse todo
```

`pin browse` opens a keyboard-driven TUI viewer with vim-respecting keys:

| Key | Action |
|---|---|
| `h` / `l` (or `←`/`→`, `space`) | previous / next task |
| `j` / `k` (or `↑`/`↓`, mouse wheel) | scroll task body down / up |
| `pgup`/`pgdn`, `ctrl+u`/`ctrl+d`, `home`/`end` | page / jump within body |
| `tab` / `shift+tab` | jump to the head of the next / previous state group |
| `gg` / `G` | first / last task |
| `/` | filter tasks (title, state, tags, body); `esc` clears |
| state hotkeys (`t`,`s`,`b`,`f`,`z`,`x`,`d`) | set state and advance — uppercase also works (`F` = followup) |
| `n` | create a new task — input accepts the pin grammar (`fix the thing pri:1 tags:{x}`, leading state token ok) |
| `N` | add a note to the current task |
| `e` | open in `$EDITOR` |
| `1`–`9`, `0` | set priority (0 = 10) |
| `q` / `ctrl+c` | quit |

When a state hotkey is used, the state is applied and the cursor advances to the next task. Mouse wheel scrolling is on by default (hold Shift to select text in the terminal; in tmux, wheel pass-through requires `set -g mouse on`).
When a state hotkey is used in browse, the state is applied and the cursor advances to the next task.

---

## Task details

```bash
pin note 12 "call vendor and confirm timeline"    # append a note
pin tag 12 15 "today, blocked"                    # add tags to one or more tasks
pin due 12 "next tuesday"                         # set or change due date
pin due 12 2026-01-15
pin edit 12                                       # open in $EDITOR
```

### Multiple task selection

```bash
pin done 2 3 6-9
pin del "[2-3, 7]"     # quote brackets in zsh to avoid glob expansion
```

---

## Health checking — pin doctor

Task files are plain markdown, which means hand edits, LLM sessions, migrations, and typo'd commands can leave files that parse cleanly but are semantically wrong. `pin doctor` audits the whole scope:

```bash
pin doctor              # report: unknown states, duplicated sections, duplicate ids,
                        # dangling deps, title/H1 divergence, next_id drift, ...
pin doctor --json       # stable check names, for scripts and sensors
pin doctor --fix        # apply only mechanically safe repairs (merge duplicated
                        # Log/Notes sections, preserving every entry)
```

Judgment-shaped findings (an unknown state that might be intentional, a diverged title) are always report-only — fix those with normal pin commands (`pin todo 46`, `pin title 46 "..."`).

Exit status is lint-style: `0` when the scope is clean (or `--fix` repaired everything), `1` when findings remain — so `pin doctor && deploy`-style gating and sensor scripts work without parsing output.

---

## AI agent workflow

Punchlist is designed as a first-class AI agent substrate: structured enough to be queried and automated, simple enough to edit by hand.

### Task metadata — provenance and context

Capture where a task came from, who assigned it, and to whom:

```bash
pin meta 1 source=standup-2026-02-27 from=alice to=bob
pin meta 1                      # display all metadata
pin meta 1 from=                # delete a key (empty value)
pin show --json 1 | jq .meta   # read in JSON
```

Metadata lives in `meta:` YAML frontmatter — invisible to human users who don't need it, fully accessible to agents that do.

### Acceptance criteria

Structured checkboxes from a `## Acceptance` section in the task body:

```bash
pin acceptance 1       # list criteria with indices (alias: pin checks)
pin acceptance add 1 "Tests pass"   # append a new criterion (creates the section if missing)
pin acceptance rm 1 2  # remove item 2 (alias: remove)
pin check 1 2          # toggle item 2 checked/unchecked
pin show --json 1 | jq .acceptance    # structured array: text, checked, index
```

### Dependencies and planning

```bash
pin "Deploy" depends:1,2 pri:1         # create task with dependencies
pin deps 5                              # forward deps + reverse lookup
pin ls --ready                          # only tasks whose deps are all DONE
pin ls --ready --json | jq '.[0].id'   # pick next actionable task
```

### Suggested task template

```markdown
---
state: todo
pri: 2
by: 2026-02-01
tags: [ai, assistant]
depends_on: [3, 4]
meta:
  source: standup-2026-02-27
  from: alice
  to: bob
---

# <short, testable outcome>

## Goal
<one sentence>

## Acceptance
- [ ] <verifiable result>
- [ ] <test or command to run>

## Context
- Files: <paths>
- Links: <issues/docs>

## Notes for assistant
- Constraints: <style/tech>
- Assumptions: <if any>
```

### Workflow tips

- Keep tasks small and specific — one outcome per ticket.
- Include concrete acceptance checks; use `pin check` to toggle them as you go.
- Use `pin meta` to capture provenance: source meeting, who assigned it, and to whom.
- Use `depends:` and `pin ls --ready` so agents always pick the right next task.
- Treat `pin note` as a running log — decisions, commands run, blockers hit.
- See `AGENTS.md` for machine-facing guidance and `docs/assistant-brief.md` for an extended agent workflow guide.

---

## MCP server — structured AI access

The punchlist MCP server gives AI assistants (Claude Desktop, Claude Code, Cursor, or any MCP-compatible client) direct, structured access to your tasks. Instead of grepping markdown files and burning tokens on raw text, the AI navigates typed data: domains, configs, filtered queries, metadata-only listings.

The MCP server is a standalone Python process that reads and writes the same files `pin` does. They coexist — use `pin` from the terminal, use MCP from your AI tools, edit tasks in Obsidian or nvim. The filesystem is the shared contract.

### Why use it?

| Without MCP | With MCP |
|---|---|
| AI greps 600 task files to find blocked items | `punchlist_list(state="BLOCK")` — metadata only, ~500 tokens |
| AI guesses at file naming and config format | `punchlist_create(title="...")` — correct by construction |
| AI reads entire task bodies to answer summary questions | `punchlist_summary` — counts by state, priority, tags |
| Context window fills with raw markdown | Metadata-first: bodies only on explicit `punchlist_get` |

### Setup

```bash
cd mcp/
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
```

### Configure for Claude Code

Add to your project's `.mcp.json` or `~/.claude.json`:

```json
{
  "mcpServers": {
    "punchlist": {
      "command": "/path/to/punchlist/mcp/.venv/bin/python3",
      "args": ["/path/to/punchlist/mcp/server.py", "--root", "/path/to/workspace"]
    }
  }
}
```

### Configure for Claude Desktop

Same JSON format, placed in:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

### Available tools

| Tool | Purpose | Token cost |
|---|---|---|
| `punchlist_discover` | List all domains, states, task counts | Very low |
| `punchlist_list` | Filtered metadata (no body) — state, tag, priority, search, date | Low |
| `punchlist_get` | Full task with body, notes, log | Medium |
| `punchlist_search` | Full-text across titles and bodies | Medium |
| `punchlist_create` | Create task — auto-ID, correct filename, config update | Low |
| `punchlist_update` | Change state/priority/tags, add notes — auto-logged | Low |
| `punchlist_summary` | Dashboard: counts by state, priority, top tags | Medium |
| `punchlist_cross_domain` | Query across all domains at once | Medium–High |

### Multi-domain workspaces

Point `--root` at a parent directory and the server discovers all nested punchlist domains automatically:

```
workspace/
  .punchlist/config.yaml    ← root config (shared defaults)
  home/
    .punchlist/config.yaml  ← domain: "home"
    tasks/
  work/
    .punchlist/config.yaml  ← domain: "work"
    quantum/
      .punchlist/config.yaml ← domain: "work/quantum" (nested)
      tasks/
```

Each domain has its own states, ID counter, and task set. The root config provides fallback defaults.

### Works alongside Obsidian

Task files are plain markdown with YAML frontmatter — Obsidian reads them natively. Point an Obsidian vault at your workspace, edit tasks in Obsidian's editor, and the MCP server sees changes immediately (no caching, no sync). The MCP server adds structured query and mutation that Obsidian doesn't have; Obsidian adds rich editing and linking that the MCP doesn't need.

### Typical AI workflow

```
1. punchlist_discover        → orient: what domains, what states
2. punchlist_summary         → lay of the land: counts, recent activity
3. punchlist_list(state=...) → filtered metadata
4. punchlist_get(id=...)     → drill into one task
5. punchlist_update(id=...)  → change state, add notes
```

For implementation details, see `mcp/PLAN.md` (architecture) and `mcp/CLAUDE.md` (setup guide).

---

## Configuration

`.punchlist/config.yaml` supports:

- `states`: list of state definitions with `name`, `aliases`, and `tui_hotkey`
- `ls_state_order`: custom state ordering for `pin ls`
- `next_id`: next task id (auto-managed)
- `id_width`: zero padding width for filenames (default 3)
- `title_max_len`: max stored title length before truncation (default 80)
- `ls_title_max_len`: max title length shown in `pin ls` (default 80)
- `edit_start_insert`: add `+startinsert` for vim/nvim (default true)
- `edit_goyo`: add `+Goyo` for vim/nvim (default false)
- `browse_margin`: columns of left/right margin in `pin browse` (default 12)
- `accepting`: when false, the scope is closed to new tasks — `pin todo` walks up to the nearest accepting parent scope, while `ls`/`done`/`note` still serve the scope (default true)

```bash
pin config          # open config in $EDITOR
pin config migrate  # backfill new config fields without overwriting yours
```

State config rules: `name` and aliases must be single-word tokens; `tui_hotkey` must be a single character and cannot conflict with reserved keys (`n`, `q`, `j`, `k`, `J`, `K`, space, `e`, `0–9`).

---

## Data layout

```
your-project/
  .punchlist/
    config.yaml       # project config
  tasks/
    001-write-outline.md
    002-review-plan.md
    ...
  .trash/             # deleted tasks land here (not permanently removed)
```

The punchlist source tree includes:

```
punchlist/
  cmd/                # Go CLI commands
  config/             # config loading and state catalog
  task/               # task parsing and serialization
  mcp/                # MCP server (Python, optional)
    server.py         # MCP server implementation
    pyproject.toml    # Python package config
    .venv/            # Python virtual environment (after setup)
  docs/               # extended documentation
  AGENTS.md           # machine-facing agent instructions
```

Tasks are plain markdown files with YAML frontmatter. Config is plain YAML. Nothing is hidden, encoded, or proprietary.

---

## Shell integration (optional)

Get notified when you `cd` into a punchlist-enabled folder:

```bash
# find nearest parent with .punchlist
_punchlist_root() {
  local d="$PWD"
  while [[ "$d" != "/" ]]; do
    [[ -d "$d/.punchlist" ]] && { print -r -- "$d"; return 0 }
    d="${d:h}"
  done
  return 1
}

_punchlist_task_count() {
  local root tasks_dir
  root="$(_punchlist_root)" || return 1
  if [[ -d "$root/tasks" ]]; then
    tasks_dir="$root/tasks"
  elif [[ -d "$root/.punchlist/tasks" ]]; then
    tasks_dir="$root/.punchlist/tasks"
  else
    return 1
  fi
  local -a files
  files=("$tasks_dir"/*.md(N))
  print -r -- "${#files[@]}"
}

typeset -g _PUNCHLIST_LAST_COUNT=""
_punchlist_maybe_notice() {
  [[ -o interactive ]] || return 0
  local count
  count="$(_punchlist_task_count)" || { _PUNCHLIST_LAST_COUNT=""; return 0 }
  if [[ "$count" != "$_PUNCHLIST_LAST_COUNT" ]]; then
    local plural=""
    (( count != 1 )) && plural="s"
    print -r -- "${count} task${plural}. Use \`pin ls\` to review."
    _PUNCHLIST_LAST_COUNT="$count"
  fi
}
```

---

## Development

```bash
make check    # go test ./... && go vet ./...
make build    # builds ./pin with version from VERSION file
make install  # go install with version ldflags
```

For command grammar details, see `docs/grammar.md`.

---

## Project

Punchlist is open source software.

- Author: Skip Levens
- Organization: Giant Ravens
- License: MIT
- Project home: https://github.com/GiantRavens/punchlist
