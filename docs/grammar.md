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
- `state:<token>`

examples of the kinds of actions you can take as quick one liners from the command line:

Fire off a complete TODO with priority, due date, and helpful tags:

`pin todo "draft qbr outline" pri:1 by:2026-01-15 tags:{qbr,launch}`

Add a quick todo that you'll fill in later - note that its status as a 'todo' is implied:

`pin "build quick outline for fidgetspinner launch"`

And maybe you just want to record a quick action of something completed - this might be helpful to include in a weekly or monthly rollup of accomplishments for example:

`pin done "quick win you want to record and refer to later"`

## Listing Tasks

You can list/dump all tasks with a simple:

`pin ls`

and you'll see every task by state order.

If you just want to see open todos filter with:

`pin ls --status TODO`

and you'll just see all open TODO tickets.

```
pin ls [path] [state] [flags]
```

Note that --status and --state are aliases, and you can list any configured state (states/aliases live in `.punchlist/config.yaml`).

path is optional and must start with `.` or `/`.

flags:
- `--pri <int>`
- `--tag <tag>` (repeatable)
- `--state <state>`
- `--status <state>` (alias)
- `--order state|id|priority|date`
- `--reverse`
- `--by-priority`
- `--by-date`
- `--by-date-reverse`
- `--chunk <n>`
- `--page <n>` (with `--chunk`)

## Search Tasks

You can also quickly search for a key word across all tasks like:

`pin search "widget"`

```
pin search [path] <query> [flags]
```

Search is case-insensitive across frontmatter, title, and body/notes (logs excluded).

flags:
- `--pri <int>`
- `--tag <tag>` (repeatable)
- `--state <state>`
- `--status <state>` (alias)
- `--order state|id|priority|date`
- `--reverse`

## Show All Tasks

Sometimes you just want to drill down into a single task like:

`pin show 23`

```
pin show <id>
```

## Browsing all Tasks, TUI style

And sometimes you want to page through all tasks and get hotkey actions to edit, mark tasks etc. 

To do that issue:

`pin browse`

You'll be taken to a TUI style interface with these hotkeys (state hotkeys are configured in `.punchlist/config.yaml`):

- left/K: previous ticket
- right/J/space: next ticket
- state hotkey: apply state and advance to next ticket
- n: add a note
- e: edit in your editor
- 1-9: set priority 1-9
- 0: set priority 10
- q: quit

## Modify Task 'State'

You can quickly mark the 'state' of a task like take it from todo to begun, begun to done, etc.

```
pin start <ids>
pin todo <ids>
pin done <ids>
pin block <ids>
pin confirm <ids>
pin followup <ids>
pin notdo <ids>
```

`<ids>` can be:
- `12`
- `12 13 14`
- `12-15`
- `"[12, 13, 15-20]"`

If you pass non-id text instead, a new task is created in that state:

```
pin begun "triage the backlog"
pin block "waiting on vendor response"
```

State names and aliases are configurable in `.punchlist/config.yaml` (single-word tokens). You can use `pin <state> <ids>` for any state token.

## Edit a Task

Editing a task to add more detailed notes, etc. fires up your default editor.

```
pin edit <id>
```

## Add Informaton to a Task (notes, duedate)

You can also quickly fire off one-liners to augment a task like add a note, change the duedate, etc.

```
pin note <id> <message>
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
- `edit_goyo`: when true, add `+Goyo` for vim/nvim (default false)
- `browse_margin`: columns of left/right margin in `pin browse` (default 12)
- `title_max_len`: max stored title length before truncation (default 80)
- `ls_title_max_len`: max title length shown in `pin ls` (default 80)
