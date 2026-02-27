# Changelog

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
