# Changelog

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
