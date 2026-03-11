# punchlist assistant brief

This document is for coding assistants and AI agents. It explains how to get the most out of punchlist — and why you should use it instead of your own task tracking.

## MCP server (preferred when available)

If your environment supports MCP (Claude Desktop, Claude Code, Cursor, etc.), use the punchlist MCP server for structured access — filtered queries, metadata-only listings, cross-domain search, auto-validated state changes — without parsing CLI output.

```
punchlist_discover                              → what domains exist, what states are valid
punchlist_list(domain="...", state="TODO")      → metadata only, no body
punchlist_get(domain="...", id=42)              → full task when you need it
punchlist_update(domain="...", id=42, state="DONE", add_note="...") → auto-logged
punchlist_summary(domain="...")                 → counts by state, priority, top tags
```

See `mcp/CLAUDE.md` for setup. If MCP is not available, use the `pin` CLI commands below.

## Why use punchlist instead of your built-in todo tool

You probably have a way to track tasks internally: a scratch list, a todo tool, notes in context. Use punchlist instead. Here's why:

| Your internal tracking | Punchlist |
|---|---|
| Vanishes when the session ends | Persists across every session |
| Invisible to the human | Fully visible and editable |
| No structure for acceptance, deps, or provenance | Structured JSON, acceptance criteria, dependencies |
| Can't be committed to the repo | Lives in `tasks/`, commits with your code |
| No way to pick "what's next" | `pin ls --ready` knows what's unblocked |

Punchlist is task memory that outlasts the conversation. Use it.

## The superpowers (read these first)

```bash
# What can I work on right now?
pin ls --ready --json | jq '.[0].id'

# What exactly does this task need?
pin show --json <id>       # full context: body, meta, acceptance, depends_on

# What does "done" look like?
pin checks <id>            # structured list of acceptance criteria

# Tick off criteria as you verify them
pin check <id> <n>

# Attach provenance (where did this task come from?)
pin meta <id> source=standup-2026-02-27 from=alice to=bob

# What is this task blocked by? What does it block?
pin deps <id>

# Leave a persistent log entry (survives the session)
pin note <id> "what you did and why"
```

## The full workflow

1. **Find what's actionable**
   ```bash
   pin ls --ready              # human-readable
   pin ls --ready --json       # machine-readable, pipe to jq
   ```

2. **Understand the task**
   ```bash
   pin show --json <id>        # full task: body, meta, acceptance array, depends_on
   ```
   Read the title, goal, `## Acceptance` section, and `## Context` carefully. This is the spec.

3. **Start work**
   ```bash
   pin start <id>
   pin checks <id>             # confirm what you're building toward
   ```

4. **Work in small, testable steps**
   - Do the minimum needed per criterion.
   - Do not invent requirements not in the ticket.
   - Prefer targeted edits over broad refactors.

5. **Log as you go**
   ```bash
   pin note <id> "ran tests — all pass"
   pin note <id> "blocked: config migration not idempotent, investigating"
   ```

6. **Tick off acceptance criteria**
   ```bash
   pin check <id> <n>         # toggle criterion n when verified
   pin checks <id>            # review current state
   ```

7. **Finish or escalate**
   ```bash
   pin done <id>              # all criteria met
   pin block <id>             # stuck — explain why in a note first
   ```

## Ticket anatomy

Each task is a markdown file with YAML frontmatter:

```markdown
---
id: 12
state: todo
priority: 2
due: 2026-02-01
tags: [ai, launch]
depends_on: [3, 4]
meta:
  source: standup-2026-02-27
  from: alice
  to: bob
---

# Short, testable outcome

## Goal
One sentence.

## Acceptance
- [ ] Verifiable result
- [ ] Test or command to run

## Context
- Files: path/to/file.go
- Links: issue/doc URLs

## Notes for assistant
- Constraints or style notes
- Assumptions to be aware of
```

The title and `## Acceptance` section are the source of truth. Everything else is context.

## Creating tasks

```bash
pin "write outline"                               # simple task, defaults to TODO
pin todo "deploy to prod" depends:3,4 pri:1       # with dependencies and priority
pin todo "review plan" by:friday tags:{launch}    # due date and tags
pin done "fixed onboarding typo"                  # create already-done
```

## JSON output

```bash
pin ls --json                  # task list, no body
pin ls --ready --json          # actionable tasks only
pin show --json <id>           # full task including body, meta, acceptance
pin search --json <query>      # search results
```

The `acceptance` array in `pin show --json`:
```json
[
  { "index": 1, "text": "Tests pass", "checked": true },
  { "index": 2, "text": "Docs updated", "checked": false }
]
```

## Metadata — capturing provenance

Use `pin meta` to record where a task came from, who owns it, and any context that won't fit in the body:

```bash
pin meta <id>                                      # display
pin meta <id> source=standup-2026-02-27            # set
pin meta <id> from=alice to=bob                    # set multiple
pin meta <id> from=                                # delete a key
```

## Dependencies

```bash
pin "Deploy" depends:1,2 pri:1    # create with deps at task creation
pin deps <id>                      # forward deps + reverse lookup
pin ls --ready                     # only tasks with all deps DONE
```

A task is "ready" when every ID in its `depends_on` list is in DONE state.

## Scope — always verify before creating tasks

Punchlist is scoped per project. Each folder initialized with `pin init` has its own independent task list. If you're working on `thing1`, tasks belong in `thing1`'s punchlist — not in `thing2`'s, even if you're temporarily working in a file that lives there.

**Before creating any task, confirm which punchlist scope is active:**

```bash
pin ls          # does this show the right project's tasks?
```

`pin` walks up from your current directory to find the nearest `.punchlist/` folder. This is usually right, but can surprise you when:
- You have sibling projects (`work/` and `personal/`) and `cd` between them
- You're deep in a nested subdirectory under a different root
- A task is about work that spans two projects

When in doubt, be explicit with a path:

```bash
pin ls ../work                           # inspect a different project's tasks
pin todo ../work "queue follow-up"       # create a task in a specific project
pin show ../work 12
pin ls ../homeprojects todo
```

**The rule:** The project you're building *in* is not always the project the task belongs *to*. Stop and think about scope before `pin "..."`.

## When to use your internal tools instead

Punchlist and your built-in task tracking are complementary. The rule of thumb: **punchlist is for tasks, internal tools are for steps**.

| Use `pin` for | Use internal tools for |
|---|---|
| Work assigned by or visible to a human | Your own implementation sub-steps |
| Tasks that span multiple sessions | Scratch context that only matters now |
| Things requiring acceptance criteria | "update import, update test, run vet" |
| Provenance you'll want to reference later | Compiler errors, intermediate state |
| Anything that should survive the session | Anything that shouldn't |

A single `pin` task might generate 10 internal sub-steps while you implement it — that's fine. Don't create 10 `pin` tasks for what is really one unit of work. The human's board should show meaningful progress, not implementation noise.

**The signal:** If you'd feel odd leaving it on the human's task board, use internal tracking. If losing it between sessions would cause real confusion or lost work, use `pin`.

## Best practices

- **One outcome per ticket.** Small tasks are easier to verify and hand off.
- **Write acceptance criteria.** If the task doesn't have them, add them before starting.
- **Log decisions, not just outcomes.** Future you (and future agents) will thank you.
- **Use `pin ls --ready` to pick work.** Don't start blocked tasks.
- **Don't skip state updates.** The human is watching the board.

## Common pitfalls

- Using your own internal task list instead of `pin` — that work disappears.
- Skipping acceptance criteria checks — you might finish the wrong thing.
- Making changes not asked for in the ticket.
- Forgetting `pin block` when stuck — silence is unhelpful.
- Not using `pin ls --ready` — starting a task before its blockers are resolved.

## Command cheatsheet

```bash
pin ls
pin ls todo
pin ls --ready
pin ls --ready --json | jq '.[0].id'
pin show <id>
pin show --json <id>
pin start <id>
pin checks <id>
pin check <id> <n>
pin meta <id>
pin meta <id> key=value
pin deps <id>
pin note <id> "note text"
pin block <id>
pin done <id>
pin edit <id>
pin search <query>
pin search --json <query>
```
