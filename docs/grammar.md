# Command Grammar

the cli uses a small, sentence-like grammar.

## Creating a Task

```
pin [state] [path] <title> [modifiers...]
```

state defaults to `TODO` when omitted.
path is optional and must start with `.` or `/` (example: `../work`).

modifiers:
- `pri:<int>` or `priority:<int>`
- `by:<date>` or `due:<date>`
- `tags:{a,b,c}`

examples:

```
pin todo "draft qbr outline" pri:1 by:2026-01-15 tags:{qbr,launch}
pin "default todo task"
pin done "quick win you want to record"
```

## Listing Tasks

```
pin ls [path] [state] [flags]
```

state can be any task state (TODO, BEGUN, BLOCK, CONFIRM, DONE, NOTDO).
path is optional and must start with `.` or `/`.

flags:
- `--pri <int>`
- `--tag <tag>` (repeatable)
- `--order state|id`
- `--reverse`

## Show All Tasks

```
pin show <id>
```

## Modify Task 'State'

```
pin start <ids>
pin done <ids>
pin block <ids>
pin confirm <ids>
pin notdo <ids>
```

`<ids>` can be:
- `12`
- `12 13 14`
- `12-15`
- `"[12, 13, 15-20]"`

If you pass non-id text instead, a task is created in that state:

```
pin begun "triage the backlog"
pin block "waiting on vendor response"
```

## Edit a Task

```
pin edit <id>
```

## Add Infomraiton to a Task (notes, log, duedate)

```
pin note <id> <message>
pin log <id> <message>
pin due <id> <date>
pin tag <ids> <tags>
```

dates accept:
- `today`, `tomorrow`
- weekdays (`mon`, `tuesday`, `next fri`)
- `YYYY-MM-DD`
- `YYYY-MM-DDTHH:MM`
- rfc3339 timestamps

## Delete a Task(s)

```
pin del <ids>
```

moves tasks to `.trash/` with a collision-safe filename.

## Compact IDs

```
pin compact
```

reassigns task ids into a contiguous sequence and updates filenames and ids.
each changed task gets a log entry noting the old and new id.

## Config

`.punchlist/config.yaml` supports:
- `next_id`: next task id
- `id_width`: zero padding width for filenames (default 3)
- `ls_state_order`: custom state ordering for `pin ls`
- `edit_start_insert`: when true, add `+startinsert` for vim/nvim (default true)
- `title_max_len`: max stored title length before truncation (default 80)
- `ls_title_max_len`: max title length shown in `pin ls` (default 80)
