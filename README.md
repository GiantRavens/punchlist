# punchlist

Punchlist is an open, markdown-native task system for people who prefer simple, powerful tools. Every task is a markdown file, easily parsed and edited with any tools like Obsidian.

## How to build locally for testing

Build a local binary for testing:

```bash
./scripts/build_binary.sh
```

Install it for system-wide use to `/usr/local/bin/pin`:

```bash
./scripts/install_binary.sh
```

## Make any folder a scoped task system.

From within any folder, such as 'work' or 'home projects' initialize punchlist, then optionally hook punchlist task alerts in your shell.

Initialize punchlist tasks in the current folder. This command will build a .punchlist directory that contains a basic config.yaml file, and a tasks/ folder that will contain markdown files, one markdown file per task. Each markdown task has YAML front-matter, and is easily editable and configurable in any editor, or via punchlists 'pin' command.

```bash
pin init
```

Punchlists 'pin' command grammer is meant to be natural and tolerant.

Examples of creating tasks:

```bash
pin todo "write release plan" pri:1 by:2026-01-09 tags:{launch,pr}
pin "default state is todo"
```

Listing and inspecting tasks:

```bash
pin ls
pin ls todo --tag launch
pin show 12
```

Updating task 'states':

```bash
pin start 12
pin done 12
pin block 12
pin confirm 12
pin notdo 12
```

Add notes and log entries to existing tasks:

```bash
pin note 12 "call vendor and confirm timeline"
pin log 12 "reviewed draft and sent feedback"
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

## Selecting multiple tasks:

You can pass multiple ids and ranges:

```bash
pin done 2 3 6-9
pin del "[2-3, 7]"
```

note: zsh treats `[]` as glob patterns, so quote bracket selectors or use `noglob`.

## Hooking punchlist into shell completion

If you like, you can be alerted when CWD into a directory that is punchlist enabled - here's a simple starter example that give an old school mail alert on entering a punchlist directory with the task count:

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

## Development

run tests:

```bash
go test ./...
```

for command grammar details, see `docs/grammar.md`.


## Project

Punchlist is open source software.

- Author: Skip Levens
- Organization: Giant Ravens
- License: MIT
- Project home: https://github.com/giant-ravens/punchlist
