```
  o
+--\----+
|   \   |
| punch |
|  list |
+-------+
```

# punchlist

Punchlist is an open, transparent, markdown-native, AI-friendly task ticket system. Every TODO/task is a markdown file, easily parsed and edited with tools like nvim or Obsidian, or wired into your coding assistant workflow.

## Make any folder a scoped task system

From within any folder, such as 'work' or 'home projects' use `pin init` to initialize it as a punchlist project home. 

```bash
pin init
```

`pin init` simply builds a .punchlist directory with a basic config.yaml file, adds a tasks/ folder that holds markdown files, one markdown file per task. 

Each markdown task has YAML front-matter, and is easily editable and configurable in any editor, or modified with punchlist's 'pin' command.

Punchlist's 'pin' command grammar is meant to be natural and tolerant.

Examples of creating tasks:

```bash
pin todo "write release plan" pri:1 by:2026-01-09 tags:{launch,pr}
pin todo ../homeprojects "draft release email"
pin done "shipped quick fix for onboarding typo"
```

Listing and inspecting tasks:

```bash
pin ls
pin ls ../homeprojects
pin ls todo
pin ls done
pin ls todo --tag launch
pin ls --state todo
pin ls --status todo
pin search exagrid
pin show 12
```

Search scans frontmatter, title, and body/notes (logs excluded) with case-insensitive matching.

Browse tasks interactively:

```bash
pin browse
pin browse todo
```

`pin browse` opens a keyboard-driven viewer with the current task, plus quick actions for adding notes
and updating state.

Updating task 'states':

```bash
pin start 12
pin todo 12
pin done 12
pin block 12
pin confirm 12
pin followup 12
pin notdo 12
pin begun "triage the backlog"
pin block "waiting on vendor response"
```

State commands accept uppercase aliases and a few synonyms (for example: `pin TODO 12`, `pin DONE 12`, `pin REVIEW 12`, `pin WAITING 12`, `pin FOLLOWUP 12`).

Add notes and log entries to existing tasks:

```bash
pin note 12 "call vendor and confirm timeline"
pin log 12 "reviewed draft and sent feedback"
```

Tag existing tasks:

```bash
pin tag 12 15 "today, blocked"
```

Open a task in your editor:

```bash
pin edit 12
```

Add a due date:

```bash
pin due 12 2026-01-15
pin due 12 "next tuesday"
```

Delete a Task (moves to `.trash/`):

```bash
pin del 12
```

Compact task IDs down (renumber all tasks to avoid large id gaps):

```bash
pin compact
```

## Select multiple tasks:

You can pass multiple ids and ranges:

```bash
pin done 2 3 6-9
pin del "[2-3, 7]"
```

note: zsh treats `[]` as glob patterns, so quote bracket selectors or use `noglob`.

## Automatically showing task counts when entering a punchlist capable folder:

If you like, you can be alerted when you move into a directory that is punchlist enabled - here's a simple starter example that gives an old school mail alert on entering a punchlist-enabled directory:

```bash
# punchlist notifier
# find nearest parent with .punchlist (project root)
_punchlist_root() {
  local d="$PWD"
  while [[ "$d" != "/" ]]; do
    [[ -d "$d/.punchlist" ]] && { print -r -- "$d"; return 0 }
    d="${d:h}"
  done
  return 1
}

# count markdown tasks (prefer ./tasks, fallback to .punchlist/tasks)
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
  files=("$tasks_dir"/*.md(N))   # nullglob
  print -r -- "${#files[@]}"
}

# last-seen task count
typeset -g _PUNCHLIST_LAST_COUNT=""

# print notice before prompt (mail-style)
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

## Data Layout

- tasks live in `tasks/` as markdown files with yaml frontmatter.
- config lives in `.punchlist/config.yaml`.
- deleted tasks move to `.trash/`.
- compacted tasks have their filenames renumbered, but a log entry is added noting the original and new id's

## Config

`.punchlist/config.yaml` supports:

- `next_id`: next task id
- `id_width`: zero padding width for filenames (default 3)
- `ls_state_order`: custom state ordering for `pin ls`
- `edit_start_insert`: when true, add `+startinsert` for vim/nvim (default true)
- `edit_goyo`: when true, add `+Goyo` for vim/nvim (default false)
- `browse_margin`: columns of left/right margin in `pin browse` (default 12)
- `title_max_len`: max stored title length before truncation (default 80)
- `ls_title_max_len`: max title length shown in `pin ls` (default 80)

## Using punchlist with AI coding assistants

Punchlist works best with assistants when tasks are small, explicit, and easy to verify. Use tickets as the source of truth, and have the assistant update task state as work progresses.

If you use agent instruction files, see `AGENTS.md` at the repo root for machine-facing guidance on how assistants should operate in this codebase.

Suggested ticket template (markdown file content):

```markdown
---
state: todo
pri: 2
by: 2026-02-01
tags: [ai, assistant]
---

# <short, testable outcome>

## Goal
<one sentence>

## Acceptance
- [ ] <verifiable result>
- [ ] <test or command to run, if any>

## Context
- Files: <paths>
- Links: <issues/docs>

## Notes for assistant
- Constraints: <style/tech>
- Assumptions: <if any>
```

Workflow tips:

- Keep tasks tiny and specific; one outcome per ticket.
- Include concrete acceptance checks (files changed, behavior, tests).
- Add file paths and constraints so the assistant can act without guessing.
- Track status with `pin start`, `pin block`, `pin done`, and add `pin log` notes as you go.

## Docs

- `docs/assistant-brief.md` for assistant workflows and best practices.
- `docs/help-docs-build-generated.md` for offline `pin --help` output (auto-generated during build).

## Development

run tests:

```bash
go test ./...
```

For command grammar details, see `docs/grammar.md`.

## Project

Punchlist is open source software.

- Author: Skip Levens
- Organization: Giant Ravens
- License: MIT
- Project home: https://github.com/GiantRavens/punchlist
