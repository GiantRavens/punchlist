# punchlist guidance for coding assistants

This file is machine-facing guidance for coding assistants. It should be read and followed by tools/agents before they act in this repo. Keep it short, explicit, and task-focused.

## When to use this

- Use this file when an assistant is operating in this repo and needs rules of engagement.
- Treat it as higher priority than README or other human docs for assistant behavior.

## When not to use this

- Do not use it as a human tutorial or product documentation.
- Do not duplicate large docs here; link or refer to `docs/` instead.

Use punchlist as the source of truth for work. Tasks are markdown files in `tasks/` (or `.punchlist/tasks/`).

## Core rules

- Read the task and follow its acceptance checks.
- Prefer small, testable changes.
- Update state as you work: `pin start`, `pin block`, `pin done`.
- Log decisions and commands with `pin log`.
- If available, run `pin --help` to confirm command grammar.
- If requirements are unclear, ask before proceeding.

## Quick commands

```bash
pin ls
pin ls todo
pin show <id>
pin start <id>
pin log <id> "note"
pin done <id>
```
