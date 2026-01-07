# punchlist

*A text-native, AI-friendly task and ticket system*

---

## 1. Overview

**punchlist** is a Markdown-first task and ticket system designed for people who think and work in text: Obsidian users, developers, writers, product leaders, and anyone who wants durable, non-proprietary artifacts.

Each task is a single Markdown file with YAML frontmatter. The CLI provides a concise, human-readable grammar for creating, updating, querying, and annotating tasks. AI is a first-class participant, but never the owner of the data: prompts, outputs, and iterations are captured explicitly and stored alongside the task as plain files.

punchlist is intentionally **UI-agnostic**. It does not ship a web app or Kanban board. Instead, it produces clean artifacts that can be consumed by:

- Obsidian
- Text editors
- Git
- Future web or TUI frontends
- AI agents (local or remote)

The project name is **punchlist**.  
The primary CLI verb is **`pin`** (with `punchlist` as an optional long form).

---

## 2. Design Intent

### Primary goals

- **Non-proprietary output**  
	All state lives in Markdown + YAML. If punchlist disappears, your data remains usable.

- **Round-trippable editing**  
	Users can safely edit tickets in any text editor. The CLI must tolerate and preserve manual edits.

- **Low cognitive overhead**  
	Creating and updating tasks should feel like typing a sentence, not filling out a form.

- **AI-friendly by construction**  
	Tickets are structured so AI tools can:
	- Read context deterministically
	- Append notes or outputs safely
	- Generate new artifacts without destroying human intent

- **Composable, not monolithic**  
	punchlist is a backend and grammar, not a product UI.

### Explicit non-goals

- No proprietary database
- No mandatory cloud service
- No enforced UI or workflow ideology
- No scraping of arbitrary notes without opt-in

---

## 3. Core Concepts

### 3.1 Scope

- Scope is defined by **folder**, not metadata.
- punchlist operates within the nearest directory containing a config file.
- Typical structure:

```
work/
	hugeco/
		tasks/
			000041-do-the-thing.md
			000042-call-bob.md
		.punchlist/
			next_id
			ai/
		.config.yaml
```

This aligns naturally with Obsidian vaults and Git repositories.

---

### 3.2 Tickets

- One task = one Markdown file
- Filenames are deterministic and sortable
- YAML frontmatter is the canonical API

#### Ticket frontmatter (proposed v1)

```yaml
---
id: 41
title: Do the thing
state: todo
priority: 1
due: 2025-02-01T09:00:00-06:00

tags: [hot, hugeco]

created_at: 2026-01-06T10:12:33-06:00
updated_at: 2026-01-06T10:12:33-06:00
started_at:
completed_at:

external_refs:
	- obsidian:work/hugeco/notes.md:123
---
```

---

## 4. Configuration

5. CLI Grammar (Proposed)

The grammar is designed to be:
	•	human-readable
	•	easily parseable
	•	extensible

5.1 Create

```
pin (or punchlist) todo "do the thing" pri:1 by:2025-02-01T09:00 tags:{hot,quantum}
```

Grammar rules:
	•	First positional token = state
	•	Quoted string = title
	•	Remaining tokens = key:value modifiers

Supported modifiers (v1):
	•	pri:<int> → priority
	•	by:<date|datetime> → due
	•	tags:{a,b,c} → tags

⸻

5.2 State transitions

```
punchlist start 41
punchlist done 41
punchlist defer 41
```

Effects:
	•	state updated
	•	appropriate timestamp set (started_at, completed_at)
	•	updated_at refreshed

⸻

5.3 Listing

```
punchlist ls todo
punchlist ls doing
punchlist ls done --since 7d
punchlist ls todo --tag hot
```

Output is stable, line-oriented, and pipe-friendly:

```
41  pri:1  by:2025-02-01  {hot,quantum}  Do the thing
```

`punchlist log 40 "finished outline - sent to bob"`

Appends a timestamped entry under ## Log in the ticket body.

⸻

6. AI Integration (First-Class)

6.1 Philosophy

AI is a participant, not an authority.
	•	Prompts are stored
	•	Outputs are stored
	•	Nothing is implicit or ephemeral
	•	Humans remain in control of state changes

6.2 Ticket structure for AI

Standard sections (optional but recommended):

```
## Description
## Context
## Constraints
## Acceptance
## Log
## AI
```


6.3 AI command

`punchlist ai 41 "Draft a concise outline for this task."`

Behavior:
	1.	Collect context (frontmatter + selected sections)
	2.	Run model (local or remote)
	3.	Write result to:
	•	## AI/Output in the ticket
	•	and/or a new Markdown file
	4.	Record a log entry


6.4 Recipes

Reusable prompt templates stored as files:

```
.punchlist/ai/recipes/
  summarize.md
  email_draft.md
```


Example Recipe:

```
Summarize the task below.

Return:
- 1 sentence summary
- 5 bullet points
- Suggested next step

Task:
{{ticket}}

Recent log:
{{log_last_10}}
```

Invocation

```
punchlist ai 41 --recipe summarize
punchlist ai 41 --recipe email_draft --write-file
```

6.5 AI history and audit trail

Full AI runs are stored as sidecar files:

`.punchlist/ai/000041/2026-01-06T16-41-12Z.json`

Contents:
	•	prompt
	•	model / provider
	•	response
	•	context hash
	•	timestamps

The ticket links to these files but remains readable.

⸻

7. Obsidian Interop
	•	Tickets live in visible folders (tasks/)
	•	No dot-prefixed ticket directories
	•	Optional importer scans for TODO: or - [ ] patterns
	•	Imported tickets include stable external_refs so re-imports are idempotent

⸻

8. Future-Facing (Out of Scope for v1, Enabled by Design)
	•	Kanban renderer (Markdown / HTML / TUI)
	•	Web UI
	•	Multi-scope dashboards
	•	AI agents that suggest state transitions
	•	Dataset export for fine-tuning or RAG

⸻

9. Summary

punchlist is:
	•	a grammar
	•	a file format
	•	a CLI

It deliberately avoids being “yet another app” and instead becomes a durable substrate for human and AI collaboration over tasks.

If the project succeeds, it will be because:
	•	people trust the artifacts
	•	tools can come and go
	•	the data remains legible in 10+ years
