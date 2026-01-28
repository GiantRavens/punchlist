# punchlist assistant brief

This document explains how to use punchlist with coding assistants. It is written for an assistant that can run commands and edit files.

## What punchlist is

Punchlist is a markdown-native ticket system. Each task is a markdown file with YAML front matter. The `pin` command creates, edits, and changes task state.

## Quick start for assistants

1) Find the project root (a folder with `.punchlist/` or `tasks/`).
2) List tasks: `pin ls` or `pin ls todo`.
3) Pick a task, open it: `pin show <id>`.
4) Start work: `pin start <id>`.
5) Implement and add notes: `pin log <id> "did X and Y"`.
6) Finish: `pin done <id>` or set another state like `pin block <id>`.

## Ticket anatomy

Each task file is a markdown doc with YAML front matter:

- `state`: todo, started, done, blocked, etc.
- `pri`: priority (1 high, larger is lower)
- `by`: due date
- `tags`: labels
- Title and body: the actual spec and acceptance checks

Use the title and "Acceptance" section as the source of truth.

## Working style for assistants

- Use tasks to drive work. Do not invent requirements that are not in the ticket.
- Prefer small, testable edits over broad refactors.
- Always update task state as work progresses.
- Record key decisions and commands in `pin log`.
- If blocked, set `pin block` and explain why.

## Best practices for tickets

Good tickets are:

- Small and specific: one outcome per ticket.
- Testable: includes concrete checks or commands.
- Actionable: lists file paths and constraints.
- Honest: capture risks or unknowns early.

## Command cheatsheet

```bash
pin ls
pin ls todo
pin show 12
pin start 12
pin log 12 "ran tests: go test ./..."
pin note 12 "need design input for UI copy"
pin block 12
pin done 12
pin edit 12
```

## Example assistant flow

```text
pin ls todo
pin show 12
pin start 12
...edit code and run tests...
pin log 12 "updated config parser, added tests"
pin done 12
```

## Common pitfalls

- Skipping the task acceptance checks.
- Making wide, speculative changes not asked for.
- Forgetting to update task state or log important work.
