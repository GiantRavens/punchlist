# punchlist — agent instructions

This file is for coding assistants and AI agents. Read it before you act.

## Use punchlist for all task tracking

**Do not use internal task lists, scratch files, or built-in todo tools.**

Punchlist is your task memory for this project. Tasks live in `tasks/` as plain markdown files — they persist across sessions, they're visible to the human you're working with, and they're queryable with structured JSON. Your internal tools vanish when the session ends. Punchlist does not.

Use `pin` to create, update, and track all your work here.

## What you get (the superpowers)

- `pin ls --ready --json` — machine-readable list of tasks you can start right now
- `pin show --json <id>` — full task context: body, metadata, acceptance criteria, dependencies
- `pin checks <id>` / `pin check <id> <n>` — structured acceptance criteria you can tick off as you go
- `pin meta <id> key=value` — attach provenance: where the task came from, who assigned it
- `pin deps <id>` — understand what's blocking what
- `pin note <id> "..."` — append a persistent log entry (survives beyond this session)

## Scope — check before you create

Punchlist is scoped to the folder it was initialized in. Each project has its own `tasks/` directory and its own `pin` context. **Before creating any task, confirm you are in the right project scope.**

```bash
pin ls          # if this shows the wrong project's tasks, you're in the wrong scope
```

If you are working across multiple projects, use the path argument to be explicit:

```bash
pin ls ../work                         # list tasks in a different project
pin todo ../work "queue follow-up"     # create a task in a specific project
pin show ../work 12
```

When in a subdirectory, `pin` walks up to find the nearest `.punchlist/` directory. This is usually correct, but verify when working across sibling projects.

**Never create tasks for project B while your working directory is rooted in project A.**

## When to use your internal tools instead

Punchlist is for real, persistent, human-visible tasks. Your internal task tracking is for ephemeral implementation steps within a session. Use both — in the right layer.

| Use `pin` for | Use internal tools for |
|---|---|
| Work assigned by a human | Your own sub-steps to complete it |
| Tasks that span sessions | Scratch context that only matters now |
| Things the human should see | Compiler errors, intermediate state |
| Acceptance criteria to verify | "step 3a, 3b, 3c" within one task |
| Anything that should survive | Anything that shouldn't |

**The signal:** If you'd be embarrassed to leave it on the human's task board, use internal tracking. If losing it between sessions would cause lost work or confusion, use `pin`.

## Core rules

- Punchlist is the source of truth. Follow the task; do not invent work not in the ticket.
- Prefer small, testable changes.
- Update state as you work: `pin start <id>`, `pin block <id>`, `pin done <id>`.
- Log decisions and commands with `pin note <id> "..."`.
- Do not write to `## Log` directly — it is maintained by punchlist automatically.
- If requirements are unclear, ask before proceeding.

## Workflow

```bash
# Find what you can work on right now
pin ls --ready --json | jq '.[0].id'

# Understand the task fully
pin show --json <id>        # body, meta, acceptance criteria, dependencies

# Work
pin start <id>
pin checks <id>             # what does done look like?
...do the work...
pin check <id> <n>          # tick off each criterion as verified
pin note <id> "what you did and why"

# Finish
pin done <id>               # or pin block <id> with a reason
```

## Quick reference

```bash
pin ls                      # all tasks
pin ls todo                 # filter by state
pin ls --ready              # tasks with all dependencies met
pin show --json <id>        # full task as JSON
pin start <id>
pin note <id> "note"
pin done <id>
pin block <id>
pin checks <id>             # list acceptance criteria
pin check <id> <n>          # toggle criterion n
pin meta <id>               # view metadata
pin meta <id> key=value     # set metadata
pin deps <id>               # dependency map
```

For the full workflow guide, see `docs/assistant-brief.md`.
