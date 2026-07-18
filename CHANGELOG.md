# Changelog

## 1.3.1
- State catalog fix: an explicit `ls_state_order` now registers its states in the catalog even when explicit `states:` are also present. Previously the ordering was silently discarded in that case, so order-declared states (BACKLOG, TEST, BLOCK, DUPLICATE) resolved nowhere: `pin ls` grouped by them but `pin doctor` flagged every use as `unknown_state` — 2,058 false findings in one scope. Regression test locks the contract: order-only states resolve (case-insensitively) and hold their declared position.

## 1.3.0
- `findTaskFile` now walks `tasks/` recursively, matching `pin ls` — by-id commands (`show`, `note`, `pri`, `due`, `tag`, `meta`, `edit`, `del`, acceptance, deps) previously failed on tasks in subfolders that `ls` had just listed (surfaced by the futhark bridge punchlist deck). Ambiguous ids (same id in two files) are refused with both paths named.
- Error paths now exit non-zero. New `failf` helper records the failure and `Execute` applies it after the write-lock post-run releases (never `os.Exit` mid-command). Previously `pin show <missing-id>` printed to stderr and exited 0, which callers could not distinguish from success. In-walk parse messages in `ls`/`search` remain warnings (rc 0 — the listing still succeeds).
- `pin pri <id> [<id> ...] <priority>` — set or clear the priority on existing tasks. Pass `0` as the priority to clear (priority is `yaml:omitempty`). Mirrors `pin due` and `pin tag` shape: load task, mutate field, append timestamped log entry, write back. Closes the gap that previously forced direct file edits for any post-creation priority change.
- Aliases: `PRI`, `priority`.
- Pure helper `changePriority(tsk, newPri, now)` extracted for testability; 5 unit-test cases cover set / clear / change / no-op / clear-when-zero.
- `pin browse` rendering fixed for long tasks and narrow terminals (tmux panes). Root cause: the View modeled the screen in logical lines while the terminal renders visual rows — any line wider than the pane wrapped visually, the View overflowed the terminal height, and bubbletea's renderer silently dropped lines from the *top* ("long tasks don't start at the top, and scrolling does nothing"). Three paths produced over-wide lines: blocks over 20KB skipped word-wrapping entirely (the removed `browseWrapLimit` escape), the help footer was never wrapped, and tabs/long tokens miscounted. All content and footer text now flows through lipgloss wrapping (which hard-wraps long tokens and expands tabs), so logical lines == visual rows by construction.
- `pin browse` supports mouse wheel scrolling (`tea.WithMouseCellMotion`) — 3 lines per notch, clamped by the same logic as the keyboard. Terminal text selection inside browse now requires holding Shift (standard mouse-tracking trade-off).
- New width-invariant regression tests (`cmd/browse_width_test.go`): every View line must fit the terminal width and the View must never emit more lines than the terminal height, exercised over an oversize task (>20KB, long lines, unbroken URL, tabs) at three geometries including a pathological 34×9 pane. Verified to fail under the old unwrapped regime.
- `pin browse` keymap reworked to respect vim: `h`/`l` move between tasks, `j`/`k` scroll the body (previously j/k moved between tasks and only arrows scrolled). `tab`/`shift+tab` jump to the head of the next/previous state group. `gg`/`G`, arrows, `space`, and `J`/`K` keep their old meanings.
- `/` filters tasks in browse (case-insensitive substring over title, state, tags, body); `esc` clears; the footer shows the active filter.
- `n` now creates a new task from inside browse (previously `n` was note; note moved to `N`). The input accepts the pin creation grammar — leading state token and inline modifiers (`pri:1`, `tags:{x}`, `by:friday`) are parsed; everything else becomes the title. Creation goes through the same locked `createTaskInCwd` core as the CLI (extracted from `createTaskFromArgsInDir`, now print-free with `Path` set).
- Default state hotkeys reworked: BEGUN moves `b`→`s` (start), DEFER moves `l`→`z` (snooze), and a new default state **BLOCKED** (aliases blocked/block) takes `b`. Default `ls` order becomes TODO, BEGUN, BLOCKED, FOLLOWUP, DEFER, NOTDO, DONE. State hotkeys now case-fold: `F` applies followup, `D` done, etc.
- Legacy configs keep working: `validateHotkey`'s reserved list is deliberately NOT widened (that would hard-fail catalog loading for configs carrying old defaults like `l:defer`). Instead browse shadows nav-colliding config hotkeys — navigation wins, and the dead hotkey is omitted from the help line.
- Browse errors (e.g. failed creation) now display in the footer instead of being silently swallowed; any keypress dismisses them.
- Keymap regression tests in `cmd/browse_keymap_test.go`: hjkl nav, tab group jumps with wrapping, filter apply/clear/no-match, new-default and case-folded state hotkeys, legacy-hotkey shadowing, and end-to-end in-TUI task creation against a real temp scope.
- **Data-corruption fix**: `splitSection` matched section headings with a raw substring search, so a task body whose *prose* merely mentioned a heading (e.g. an acceptance criterion describing "## Notes or ## Log") was carved apart at that point by the next mutation — fragments became fake headings and later appends landed in the wrong section. Heading detection is now line-anchored (`indexOfHeadingLine`: start-of-body or after a newline, followed by newline or EOF), mirrored in the Python MCP server (`_index_of_heading_line`). Regression tests include the exact corrupting command sequence (pin #28; specimen was tasks/021 in this repo's own scope).
- `pin version` is now a real subcommand. Previously the bare word fell through the tolerant creation grammar and created a task titled "version" — the origin of this scope's mystery tasks 5 and 14 (reproduced live during 1.3.0 development). The same footgun class for other mistyped words is tracked in task 31.
- **`pin doctor [--json] [--fix]`** — semantic health check for a scope's task substrate. `pin ls` only rejects hard parse failures; doctor finds files that parse cleanly but are wrong: `unknown_state` (typo'd commands swallowed by the tolerant grammar, e.g. `state: PRI`), `duplicate_section`, `duplicate_id`, `id_mismatch` (filename vs frontmatter), `dangling_dependency`, `zero_timestamp`, `title_divergence` (H1 vs frontmatter), `log_noise`, `bad_checkbox`, and `next_id_drift` (future id collision). `--json` emits stable check names for sensor pipelines. `--fix` applies only mechanically safe repairs: merging duplicated Log/Notes sections with every entry preserved in encounter order, via raw text surgery that keeps the frontmatter byte-identical (Acceptance duplicates stay report-only because criteria are addressed by index). Lint-style exit status per bettercli.org exit-code guidance: 0 when clean or every finding fixed, 1 when findings remain — findings print to stdout as the message, no redundant stderr line. First live run: knowledge-forge scope, 154 files — 16 findings, 15 auto-merged (migration-incident artifacts), 1 judgment call surfaced.

## 1.2.1
- `pin browse` now keeps the task detail pane scrollable for long tasks while preserving the fixed footer/key command area.
- Added browse keys for long task bodies: `↑`/`↓`, `pgup`/`pgdn`, `home`, and `end`; moving to another task resets the detail view to the top.

## 1.2.0
- **MCP server** (`mcp/`) — Model Context Protocol server for AI assistants (Claude Desktop, Claude Code, Cursor, any MCP client).
- 8 tools: discover, list, get, search, create, update, summary, cross_domain.
- Metadata-first listing — returns frontmatter only for token efficiency.
- Recursive domain discovery — finds nested domains (e.g., `work/quantum`) at any depth.
- State alias resolution — validates against rich `states:` config including aliases.
- Auto-logging — state, priority, and tag changes append timestamped entries to `## Log`.
- File naming, slug generation, and section handling match the Go CLI exactly.
- Atomic writes (write-to-temp-then-rename) for both tasks and config.
- Python 3.10+, installable via `pip install -e .` from `mcp/` directory.

## 1.1.0
- JSON output (`--json`) for `pin ls`, `pin show`, and `pin search` — machine-readable task data for AI agents and scripts.
- `pin meta <id> [key=value ...]` — freeform metadata for provenance: source, from, to, meeting context, etc.
- `pin acceptance <id>` / `pin checks <id>` — list `## Acceptance` checkboxes from a task body.
- `pin check <id> <index>` — toggle an acceptance criterion checkbox; state persists to disk.
- `pin deps <id>` — show forward dependencies and reverse dependents for a task.
- `pin ls --ready` — filter to tasks whose dependencies are all DONE (for agent task planning).
- `depends:` creation modifier — `pin "Deploy" depends:1,2 pri:1` wires up task dependencies at creation time.
- `DependsOn` and `Meta` fields added to task YAML frontmatter (both `omitempty`, zero user impact for existing tasks).
- Acceptance criteria appear in `pin show --json` output as a structured array.

## 1.0.0
- Atomic file writes for tasks and config (write-to-temp-then-rename).
- All error output routed to stderr; stdout reserved for data.
- Priority bounds validation (0-10) on task creation.
- Consolidated duplicate state-change and note-addition logic into shared helpers.
- Removed ParseState fallback from CLI path; state resolution uses StateCatalog exclusively.
- Slugify regex compiled once at package level.
- Build-time version injection via ldflags; Makefile added.
- New tests for sections, modifiers, compact, and state changes.

## 0.3.0
- Configurable states/aliases/hotkeys with freeform state updates.
- `pin config migrate` to backfill missing config defaults.

## 0.2.1
- AI assistant guidance, ls/truncation tweaks, and browse behavior refinements.

## 0.2.0
- Search feature plus docs updates.

## 0.1.4
- Browse refinements, edit integration, and title truncation.

## 0.1.3
- Browse TUI added.

## 0.1.2
- Path handling tweaks and error cleanup.

## 0.1.1
- `pin compact` added.

## 0.1.0
- Initial CLI and docs foundation.
