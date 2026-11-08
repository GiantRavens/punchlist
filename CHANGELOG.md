# Changelog

## 1.3.10
- **`pin renumber` repairs duplicate task ids** (pin #49). `pin renumber --collisions` finds every id claimed by more than one task file; in each group the earliest-created task keeps the id and the others move to fresh ids above both `next_id` and the highest id on disk. `pin renumber <task-file>` moves one named file (by path, or bare filename inside this scope's `tasks/`; an ambiguous id cannot name one). Frontmatter and filename change together, the task's `## Log` records old -> new and why, and `--dry-run` prints the plan without changing anything. Why: 1.3.5's disk floor prevents most shared-store collisions, but not when sync is stalled, because neither host can see the other's file. Observed 2026-10-23 in the notebook root, where the Mac's 271 (a DONE log entry, written while Mac Syncthing was down) collided with futhark's 271 (an open ticket), and every by-id command on 271 refused as ambiguous for two weeks. `pin doctor` (`duplicate_id`) and the fleet `pin_scope_integrity` sensor already detected it; this is the repair. The ambiguous-id error and the doctor finding now name the command. It renames first, then rewrites the frontmatter, so an interrupted run leaves an `id_mismatch` that `pin doctor` reports, never a silent second copy. Gated by the write lock.
  - Tests: `cmd/renumber_test.go` (keeps the oldest, log entry, next_id advance, doctor clean afterwards, idempotent second run, dry run changes nothing, named-file form, refused arguments).

## 1.3.9
- **Fill a ticket in the command that creates it** (pin #21). `pin todo --ticket "title" --problem "…" --approach "…" --acceptance "…" [--acceptance "…"]` writes all three sections at once, and `--body-file PATH` (or `--body-file -` for stdin) takes the body as markdown: prose before the first `##` is the description, `## Problem`/`## Approach` fill those sections, and `## Acceptance` list items become checkbox criteria. The fill flags imply `--ticket`; `--body-file` alone adds plain prose. Why: the notebook requires substantive tasks to be tickets with Problem, Approach and testable Acceptance, and forbids direct task-file edits, but an agent could only fill the sections through `pin edit` (an editor it cannot drive) or the 1.3.6 `pin problem` commands one argument at a time. Long prose through shell quoting is where that failed (quotes, `$`, backticks), and agents fell back to `## Notes`, where `pin acceptance`/`pin check` cannot see the criteria. Recurrences: pod#26 (2026-11-05) and weft #222-244 (2026-11-07). The body is read and validated before any id is allocated or the lock is taken, so a refused body (a `# ` title line, `## Notes`/`## Log`, a repeated heading, a non-list Acceptance line) creates nothing. Fenced code is never parsed for headings.
  - `pin section set/add`, `pin problem` and `pin approach` take `--file PATH` (or `-`) in place of the text argument. Section text containing a `#`/`##` heading line is refused, since it would split the section.
  - The first real criterion replaces the scaffold's placeholder criterion (`pin acceptance add` and the fill flags), and `pin section add` on a placeholder replaces it, so a filled ticket never keeps a "fill me in" line beside real content.
  - Create routing strips the creation flags before deciding "new task or state change", so a value that looks like an id (`--body-file -`, `--acceptance "5"`) no longer turns `pin todo` into "invalid task id"; a malformed flag reports the flag.
  - MCP: `punchlist_create` takes `ticket`, `problem`, `approach` and `acceptance` (a list) and passes them as the flags above, as an argument list with no shell quoting. It refuses them with a version message when the `pin` it finds is older than 1.3.9.
  - Lightweight use is unchanged: with none of the new flags, creation takes the old path and writes a byte-identical file (checked against 1.3.5 for plain, done and `--ticket` tasks; the bare scaffold's placeholder wording is the 1.3.6 text). `stripTicketFlag` is replaced by `parseCreateFlags`.
  - Hardened after an adversarial review (2026-11-07), each with a regression test: placeholder replacement matches the seeded hint exactly (current and pre-1.3.6 wording), never by substring, so authored text mentioning "fill me in" is kept; `--body-file`/`--file` sources are read in the pre-run hook before the write lock, so a slow or interactive stdin no longer holds the scope; help takes no lock (a pipe reader closing early, `pin todo --help | head`, had left it held); a criterion must be one line (`--acceptance` and `pin acceptance add`), since a newline could write headings into the task; a heading line is refused even inside a code fence, because `splitSection` does not track fences; nested Acceptance items, empty sections and duplicate headings are refused; `## problem` in any case fills `## Problem`; a BOM is dropped; `--problem -h` is text, not a help request.
  - Tests: `cmd/ticket_fill_test.go`, one test per acceptance criterion of pin #21 plus routing, flag-parsing, refusal and review-regression cases.

## 1.3.8
- The MCP server (`mcp/server.py`) no longer writes task files itself — every mutation (`punchlist_create`, `punchlist_update`) shells out to the `pin` CLI. It had been a *second, hand-mirrored* writer: it allocated ids straight from `next_id` with no on-disk floor (the exact dup-id-on-sync bug the CLI fixed in 1.3.5 — see that entry), took no project write lock, and serialized frontmatter through Python's `frontmatter` library instead of pin's serializer. The three protections were maintained in parallel in two languages, so they drifted. Now writes go through `pin` (resolving scope by `cwd`, which the MCP sets to the domain directory), inheriting the disk-floor allocation, the write lock, and the one canonical serializer by construction. The parallel write path is deleted — ~130 lines of Python section-surgery, slug/truncate, and timestamp helpers that duplicated the Go side. Reads stay native (read-only, token-efficient). Verified end-to-end: a store with a stale `next_id` beside a higher on-disk id now allocates above the floor through the MCP, where it previously reissued a colliding id.
  - `add_log` is dropped from `punchlist_update`. `## Log` is auto-maintained (state/priority changes auto-log through pin); AGENTS.md already tells agents never to write it directly, so a raw log-injection parameter was the same second-writer hazard. Free-form entries belong in `add_note` (→ `## Notes`), which remains.
  - A state validated against the domain's catalog before shelling out still fails with a domain-aware message; an unknown state never reaches the CLI.
- `pin tag set <ids> <tags>` (replace a task's tags with exactly the set given) and `pin tag rm <ids> <tags>` (remove the given tags; idempotent). Bare `pin tag` still ADDS/merges, unchanged. These exist because the MCP supports replace and remove but the CLI could only add — the gap had been papered over by the MCP's own writer, so removing that writer surfaced it. Both are gated by the write lock (added to `mutatesPunchlist` and locked by `TestMutatorsRequireWriteLock`). Removal logic is a pure `removeTags` helper with unit coverage; replace reuses `dedupeTags`.

## 1.3.7
- `pin note ls <id>` and `pin note rm <id> <index>` — list a task's notes with indices and remove one. `## Notes` was append-only with no CLI path to prune a superseded or mistaken note, forcing a banned direct file edit; `rm` drops the nth `- ` item (and any indented continuation lines of a wrapped note), leaving the `## Log` journal untouched. `note` without a subcommand still adds, as before.
- `pin acceptance set <id> <index> <text>` — edit a criterion's text in place, preserving its checked/unchecked marker. Correcting a criterion previously meant `rm` + `add`, which lost the checked state and moved the item to the end of the list. The indent of the original line is preserved.
- All task-mutating commands — including `pin section set/add`, `pin problem`, `pin approach`, `pin acceptance set`, and `pin note rm` — are gated by the project write lock (`mutatesPunchlist`, cmd/write_lock.go). A new `TestMutatorsRequireWriteLock` walks the command tree and fails if any known mutator is ungated, so a future write command can't silently ship unlocked and race the MCP server or a second `pin`.
- `pin doctor` learns `scaffold_unfilled` (info severity, report-only) — flags a `--ticket` task whose `## Problem`, `## Approach`, or `## Acceptance` still carries seeded placeholder text ("fill me in" / the self-documenting fill hints). It is the shift-left partner to the stingy-ticket class: a ticket scaffolded but never authored is detectable rather than silently shipped. Only sections that exist are considered, so non-ticket tasks report nothing, and each section clears independently as it is filled. Info findings stay cosmetic — they never drive the non-zero exit status.

## 1.3.6
- `pin section set/add <id> <heading> <text>`, with `pin problem` and `pin approach` shortcuts — set or append the free-prose body of a task section without opening an editor. Closes the gap the `--ticket` scaffold opened: it seeds `## Problem` and `## Approach` as prose (not checkboxes), and until now the only way to fill them was interactive `pin edit` or a direct file write — the latter banned for the same reason as every other field (bypasses frontmatter validation, the state-change log, and `next_id`). `pin note` already fronted `## Log`/`## Notes` and `pin acceptance add` the checkboxes; prose sections had no CLI path, which blocked agents authoring tickets non-interactively (the audience the ticket scaffold exists for). A missing section is created above the structured trailing sections (`## Acceptance`/`## Notes`/`## Log`) so the description stays contiguous, matching scaffold order; `set` replaces the body, `add` appends a paragraph preserving what's there. The managed sections `Acceptance`, `Notes`, and `Log` are refused with a pointer to their dedicated commands, so their structure (parseAcceptance checkboxes, append-only journals) can't be clobbered as prose. The scaffold's placeholder hints now name these commands, so the fill path is self-documenting. Reuses the existing `splitSection`/`joinBlocks`/`indexOfHeadingLine` section machinery; 7 unit tests cover replace, append-preserves-existing, create-above-Acceptance, create-when-no-anchors, heading normalization, and the reserved-section guard.

## 1.3.5
- Task ids now take their floor from disk, so a SHARED store stops handing out the same id twice. `next_id` was the sole authority, which makes `config.yaml` a single contended counter: when a `.punchlist` is shared between machines (Syncthing, a network share, a restored backup) both hosts allocate from their own view of it, the file conflicts, and two tasks claim one id. Observed in the wild in the forge fleet scope — `config.yaml` at `next_id: 13` beside a `config.sync-conflict-…yaml` at `12`, ids 10 and 11 each held by two tasks, and one filename mangled to `011-010-<slug>.md`. Creation now reads the highest id present in `tasks/` and `.trash/` (filename prefixes only — one readdir, no file parsing) and allocates above both that and `next_id`, then advances the counter past the id actually used. `pin doctor` already NAMED this as `next_id_drift` ("future id collision"); it is now prevented at the point of allocation rather than only reported afterwards, and a diverged store heals on its next create. Closes the common window, where the task file has synced but the counter is stale; two hosts creating in the same instant, before either file lands, still needs distinct scopes.

## 1.3.4
- Symlinked stores now resolve to their real location: `findPunchlistDir` runs `EvalSymlinks` on the `.punchlist` it finds, and task creation (`FindAcceptingRootFrom`) returns the resolved root. Previously a scope reached through a convenience symlink (e.g. `code/forge/<forge>/.punchlist` → `forge/state/<forge>/.punchlist`) listed ZERO tasks — `filepath.WalkDir` never descends a symlinked `tasks/` — while trash and creation quietly forked parallel real `tasks/`/`.trash/` dirs beside the links (two tasks were stranded that way in the forge fleet before this fix). All nine WalkDir call sites inherit the resolution from the single discovery point. Regression test covers the symlinked-store case.
- MCP discovery dedupes symlink-aliased stores: a `.punchlist` that resolves elsewhere under the served root is skipped as an alias — the store registers once, under its canonical path (a store resolving *outside* the root keeps its alias entry). Previously each aliased store surfaced as two domains with identical counts, double-counting in cross-domain rollups.

## 1.3.3
- `pin doctor` learns `loose_list` — a new info-severity, fixable check detecting Log/Notes sections whose entries are separated by blank lines (the pre-1.3.2 loose emission). `--fix` tightens them in place, preserving every entry and indented continuation lines, via the same byte-preserving section rewrite as duplicate merges (now unified as `tightenSection`, idempotent across combined defects). Info findings are cosmetic: they never drive the non-zero exit status, which now fires only for remaining error/warning findings. Report ordering is error > warning > info.

## 1.3.2
- Log and Notes entries are now emitted as TIGHT markdown lists — consecutive `- ` lines with a blank line only between the heading and the first item. The previous emission put a blank line between every entry (a "loose" CommonMark list), which wraps each item in `<p>` when rendered — extra vertical spacing on web surfaces (forge bridge, Obsidian, GitHub) and two lines of file growth per entry. Applied in both the Go CLI (`appendEntry`) and the Python MCP server (`append_entry`, verified byte-for-byte against Go); `doctor --fix` section merges also rewrite tight, so legacy loose files converge opportunistically as they're touched. Existing loose files remain valid and are not flagged; parsers accept both forms.

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
