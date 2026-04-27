#!/usr/bin/env python3
"""
Punchlist MCP Server

Exposes punchlist task domains to AI assistants via MCP (Model Context Protocol).
Provides typed, schema-aware access to md+yaml task files organized by domain.

Each domain has:
  - .punchlist/config.yaml  (states, id_width, next_id)
  - tasks/*.md              (frontmatter + body)

Usage:
  python server.py --root /path/to/workspace
  # or set PUNCHLIST_ROOT env var
"""

import argparse
import os
import re
import sys
import json
import logging
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

import yaml
import frontmatter
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import Tool, TextContent

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("punchlist-mcp")

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

DEFAULT_STATES = ["TODO", "DOING", "DONE"]
DEFAULT_ID_WIDTH = 3


def kebab(text: str) -> str:
    """Convert a title string to kebab-case for filenames."""
    s = text.lower().strip()
    s = re.sub(r"[^a-z0-9\s-]", "", s)
    s = re.sub(r"[\s_]+", "-", s)
    s = re.sub(r"-+", "-", s)
    return s.strip("-")[:80]


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def parse_task_file(path: Path) -> dict[str, Any]:
    """Parse a task .md file into structured data."""
    post = frontmatter.load(str(path))
    meta = dict(post.metadata)
    meta["_body"] = post.content
    meta["_path"] = str(path)
    meta["_filename"] = path.name
    return meta


def parse_task_metadata(path: Path) -> dict[str, Any]:
    """Parse only frontmatter (no body) for token efficiency."""
    post = frontmatter.load(str(path))
    meta = dict(post.metadata)
    meta["_filename"] = path.name
    return meta


def serialize(obj: Any) -> Any:
    """Make objects JSON-serializable."""
    if isinstance(obj, datetime):
        return obj.isoformat()
    if isinstance(obj, Path):
        return str(obj)
    if isinstance(obj, dict):
        return {k: serialize(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [serialize(i) for i in obj]
    return obj


# ---------------------------------------------------------------------------
# Domain discovery and config
# ---------------------------------------------------------------------------

class PunchlistDomain:
    """Represents a single punchlist domain."""

    def __init__(self, name: str, root: Path, config: dict):
        self.name = name
        self.root = root
        self.config = config
        self.tasks_dir = root / "tasks"

    @property
    def states(self) -> list[str]:
        return self.config.get("ls_state_order", DEFAULT_STATES)

    @property
    def id_width(self) -> int:
        return self.config.get("id_width", DEFAULT_ID_WIDTH)

    @property
    def next_id(self) -> int:
        return self.config.get("next_id", 1)

    def task_files(self) -> list[Path]:
        if not self.tasks_dir.exists():
            return []
        return sorted(self.tasks_dir.glob("*.md"))

    def find_task_file(self, task_id: int) -> Optional[Path]:
        prefix = str(task_id).zfill(self.id_width) + "-"
        for f in self.tasks_dir.iterdir():
            if f.name.startswith(prefix) and f.suffix == ".md":
                return f
        return None

    def increment_id(self):
        self.config["next_id"] = self.next_id + 1
        config_path = self.root / ".punchlist" / "config.yaml"
        with open(config_path, "w") as f:
            yaml.dump(self.config, f, default_flow_style=False)


class PunchlistWorkspace:
    """Discovers and manages all punchlist domains under a root."""

    def __init__(self, root: Path):
        self.root = root
        self.domains: dict[str, PunchlistDomain] = {}
        self._discover()

    def _discover(self):
        """Walk filesystem for .punchlist/ directories."""
        self.domains.clear()

        # Check root level
        root_config_path = self.root / ".punchlist" / "config.yaml"
        root_config = {}
        if root_config_path.exists():
            with open(root_config_path) as f:
                root_config = yaml.safe_load(f) or {}

        # Walk one level deep for domain directories
        for entry in sorted(self.root.iterdir()):
            if not entry.is_dir() or entry.name.startswith("."):
                continue
            domain_config_path = entry / ".punchlist" / "config.yaml"
            if domain_config_path.exists():
                with open(domain_config_path) as f:
                    domain_config = yaml.safe_load(f) or {}
                # Merge: domain config overrides root config
                merged = {**root_config, **domain_config}
                self.domains[entry.name] = PunchlistDomain(entry.name, entry, merged)

        # Also check root itself as a domain (if it has tasks/)
        if (self.root / "tasks").exists():
            self.domains["_root"] = PunchlistDomain("_root", self.root, root_config)

        logger.info(f"Discovered {len(self.domains)} domain(s): {list(self.domains.keys())}")

    def refresh(self):
        """Re-scan for domains (cheap operation)."""
        self._discover()

    def get_domain(self, name: str) -> Optional[PunchlistDomain]:
        return self.domains.get(name)


# ---------------------------------------------------------------------------
# MCP Server
# ---------------------------------------------------------------------------

def build_server(workspace: PunchlistWorkspace) -> Server:
    server = Server("punchlist-mcp")

    # -- Tool definitions --------------------------------------------------

    @server.list_tools()
    async def list_tools() -> list[Tool]:
        return [
            Tool(
                name="punchlist_discover",
                description=(
                    "Discover all punchlist domains, their configs, states, and task counts. "
                    "Call this first to understand what domains exist and what states each uses."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {},
                    "required": [],
                },
            ),
            Tool(
                name="punchlist_list",
                description=(
                    "List tasks in a domain. Returns metadata only (no body) for token efficiency. "
                    "Supports filtering by state, tag, priority, title search, and date range."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "domain": {"type": "string", "description": "Domain name (e.g. 'quantum')"},
                        "state": {
                            "description": "Filter by state(s). Single string or array.",
                            "oneOf": [
                                {"type": "string"},
                                {"type": "array", "items": {"type": "string"}},
                            ],
                        },
                        "tag": {
                            "description": "Filter by tag(s). Single string or array.",
                            "oneOf": [
                                {"type": "string"},
                                {"type": "array", "items": {"type": "string"}},
                            ],
                        },
                        "priority": {"type": "integer", "description": "Filter by priority"},
                        "search": {"type": "string", "description": "Substring match on title"},
                        "since": {"type": "string", "description": "ISO date — tasks updated after this"},
                        "limit": {"type": "integer", "description": "Max results (default 50)"},
                        "offset": {"type": "integer", "description": "Pagination offset"},
                        "sort": {"type": "string", "description": "Sort field (default: updated_at)"},
                        "reverse": {"type": "boolean", "description": "Reverse sort order"},
                    },
                    "required": ["domain"],
                },
            ),
            Tool(
                name="punchlist_get",
                description=(
                    "Get full task content including body, notes, and log. "
                    "Use after punchlist_list to drill into a specific task."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "domain": {"type": "string", "description": "Domain name"},
                        "id": {"type": "integer", "description": "Task ID"},
                    },
                    "required": ["domain", "id"],
                },
            ),
            Tool(
                name="punchlist_search",
                description=(
                    "Full-text search across task titles and bodies. "
                    "Searches within a domain or across all domains."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "query": {"type": "string", "description": "Search text"},
                        "domain": {"type": "string", "description": "Scope to domain (omit for all)"},
                        "include_body": {"type": "boolean", "description": "Include body excerpts (default false)"},
                        "limit": {"type": "integer", "description": "Max results (default 20)"},
                    },
                    "required": ["query"],
                },
            ),
            Tool(
                name="punchlist_create",
                description=(
                    "Create a new task in a domain. Auto-assigns ID, creates file, updates config."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "domain": {"type": "string", "description": "Domain name"},
                        "title": {"type": "string", "description": "Task title"},
                        "state": {"type": "string", "description": "Initial state (default: domain's first state)"},
                        "priority": {"type": "integer", "description": "Priority level"},
                        "tags": {"type": "array", "items": {"type": "string"}, "description": "Tags"},
                        "body": {"type": "string", "description": "Initial body/notes content"},
                    },
                    "required": ["domain", "title"],
                },
            ),
            Tool(
                name="punchlist_update",
                description=(
                    "Update a task's metadata and/or add notes/log entries. "
                    "State changes are validated against the domain's state list and auto-logged."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "domain": {"type": "string", "description": "Domain name"},
                        "id": {"type": "integer", "description": "Task ID"},
                        "state": {"type": "string", "description": "New state"},
                        "priority": {"type": "integer", "description": "New priority"},
                        "tags": {"type": "array", "items": {"type": "string"}, "description": "Replace tags"},
                        "add_tags": {"type": "array", "items": {"type": "string"}, "description": "Append tags"},
                        "remove_tags": {"type": "array", "items": {"type": "string"}, "description": "Remove tags"},
                        "add_note": {"type": "string", "description": "Timestamped note to append"},
                        "add_log": {"type": "string", "description": "Timestamped log entry to append"},
                    },
                    "required": ["domain", "id"],
                },
            ),
            Tool(
                name="punchlist_summary",
                description=(
                    "Dashboard overview: task counts by state, priority, top tags, recent activity. "
                    "Scope to a domain or get all domains."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "domain": {"type": "string", "description": "Domain (omit for all)"},
                    },
                    "required": [],
                },
            ),
            Tool(
                name="punchlist_cross_domain",
                description=(
                    "Query across ALL domains simultaneously. "
                    "Each result includes its domain name for context."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "state": {
                            "description": "Filter by state(s)",
                            "oneOf": [
                                {"type": "string"},
                                {"type": "array", "items": {"type": "string"}},
                            ],
                        },
                        "tag": {
                            "description": "Filter by tag(s)",
                            "oneOf": [
                                {"type": "string"},
                                {"type": "array", "items": {"type": "string"}},
                            ],
                        },
                        "search": {"type": "string", "description": "Title substring"},
                        "since": {"type": "string", "description": "Updated after date"},
                        "limit": {"type": "integer", "description": "Max results (default 50)"},
                    },
                    "required": [],
                },
            ),
        ]

    # -- Tool implementations ----------------------------------------------

    def _filter_tasks(
        domain: PunchlistDomain,
        state=None,
        tag=None,
        priority=None,
        search=None,
        since=None,
        limit=50,
        offset=0,
        sort="updated_at",
        reverse=False,
        include_body=False,
    ) -> list[dict]:
        """Core filtering logic used by list and cross_domain."""
        tasks = []

        # Normalize filters
        states = [state] if isinstance(state, str) else (state or [])
        tags = [tag] if isinstance(tag, str) else (tag or [])
        states_upper = [s.upper() for s in states]

        for path in domain.task_files():
            try:
                if include_body:
                    meta = parse_task_file(path)
                else:
                    meta = parse_task_metadata(path)
            except Exception as e:
                logger.warning(f"Failed to parse {path}: {e}")
                continue

            # Apply filters
            if states_upper and meta.get("state", "").upper() not in states_upper:
                continue
            if tags:
                task_tags = [t.lower() for t in (meta.get("tags") or [])]
                if not any(t.lower() in task_tags for t in tags):
                    continue
            if priority is not None and meta.get("priority") != priority:
                continue
            if search and search.lower() not in (meta.get("title", "") or "").lower():
                if not include_body or search.lower() not in (meta.get("_body", "") or "").lower():
                    continue
            if since:
                updated = meta.get("updated_at", "")
                if isinstance(updated, datetime):
                    updated = updated.isoformat()
                if str(updated) < since:
                    continue

            # Strip internal fields for output (keep _body if requested)
            out = {k: v for k, v in meta.items() if not k.startswith("_") or (include_body and k == "_body")}
            if include_body and "_body" in meta:
                out["body"] = meta["_body"]
                out.pop("_body", None)
            tasks.append(out)

        # Sort
        def sort_key(t):
            val = t.get(sort, "")
            if isinstance(val, datetime):
                return val.isoformat()
            return str(val) if val is not None else ""

        tasks.sort(key=sort_key, reverse=not reverse)

        # Paginate
        return tasks[offset : offset + limit]

    @server.call_tool()
    async def call_tool(name: str, arguments: dict) -> list[TextContent]:
        workspace.refresh()  # re-scan domains each call (cheap)

        try:
            result = _handle_tool(name, arguments)
        except ValueError as e:
            result = {"error": str(e)}
        except FileNotFoundError as e:
            result = {"error": str(e)}
        except Exception as e:
            logger.exception(f"Error in tool {name}")
            result = {"error": f"Internal error: {e}"}

        return [TextContent(type="text", text=json.dumps(serialize(result), indent=2))]

    def _handle_tool(name: str, args: dict) -> Any:

        # -- punchlist_discover --
        if name == "punchlist_discover":
            out = {"root": str(workspace.root), "domains": {}}
            for dname, domain in workspace.domains.items():
                out["domains"][dname] = {
                    "task_count": len(domain.task_files()),
                    "states": domain.states,
                    "next_id": domain.next_id,
                    "id_width": domain.id_width,
                }
            return out

        # -- punchlist_list --
        if name == "punchlist_list":
            domain = workspace.get_domain(args["domain"])
            if not domain:
                raise ValueError(f"Unknown domain: {args['domain']}")
            tasks = _filter_tasks(
                domain,
                state=args.get("state"),
                tag=args.get("tag"),
                priority=args.get("priority"),
                search=args.get("search"),
                since=args.get("since"),
                limit=args.get("limit", 50),
                offset=args.get("offset", 0),
                sort=args.get("sort", "updated_at"),
                reverse=args.get("reverse", False),
            )
            return {"domain": args["domain"], "count": len(tasks), "tasks": tasks}

        # -- punchlist_get --
        if name == "punchlist_get":
            domain = workspace.get_domain(args["domain"])
            if not domain:
                raise ValueError(f"Unknown domain: {args['domain']}")
            path = domain.find_task_file(args["id"])
            if not path:
                raise FileNotFoundError(f"Task {args['id']} not found in {args['domain']}")
            task = parse_task_file(path)
            out = {k: v for k, v in task.items() if not k.startswith("_")}
            out["body"] = task.get("_body", "")
            return out

        # -- punchlist_search --
        if name == "punchlist_search":
            query = args["query"]
            target_domain = args.get("domain")
            include_body = args.get("include_body", False)
            limit = args.get("limit", 20)
            results = []
            domains_to_search = (
                [workspace.get_domain(target_domain)]
                if target_domain
                else list(workspace.domains.values())
            )
            for domain in domains_to_search:
                if domain is None:
                    continue
                for path in domain.task_files():
                    try:
                        task = parse_task_file(path)
                    except Exception:
                        continue
                    title = (task.get("title") or "").lower()
                    body = (task.get("_body") or "").lower()
                    q = query.lower()
                    if q in title or q in body:
                        out = {k: v for k, v in task.items() if not k.startswith("_")}
                        out["_domain"] = domain.name
                        if include_body:
                            out["body"] = task.get("_body", "")
                        # Add a relevance hint: title match > body match
                        out["_match"] = "title" if q in title else "body"
                        results.append(out)
                        if len(results) >= limit:
                            break
                if len(results) >= limit:
                    break
            return {"query": query, "count": len(results), "results": results}

        # -- punchlist_create --
        if name == "punchlist_create":
            domain = workspace.get_domain(args["domain"])
            if not domain:
                raise ValueError(f"Unknown domain: {args['domain']}")

            task_id = domain.next_id
            title = args["title"]
            state = args.get("state", domain.states[0] if domain.states else "TODO")

            # Validate state
            if state.upper() not in [s.upper() for s in domain.states]:
                raise ValueError(
                    f"Invalid state '{state}' for domain '{domain.name}'. "
                    f"Valid states: {domain.states}"
                )

            # Build frontmatter
            meta = {
                "id": task_id,
                "title": title,
                "state": state,
                "created_at": now_iso(),
                "updated_at": now_iso(),
            }
            if "priority" in args:
                meta["priority"] = args["priority"]
            if "tags" in args:
                meta["tags"] = args["tags"]

            # Build body
            body_parts = [f"# {title}", "", "## Notes", ""]
            if args.get("body"):
                body_parts.append(f"- {now_iso()}: {args['body']}")
                body_parts.append("")
            body_parts.extend(["## Log", "", f"- {now_iso()}: Created task", ""])
            body = "\n".join(body_parts)

            # Write file
            filename = f"{str(task_id).zfill(domain.id_width)}-{kebab(title)}.md"
            filepath = domain.tasks_dir / filename
            domain.tasks_dir.mkdir(parents=True, exist_ok=True)

            post = frontmatter.Post(body, **meta)
            with open(filepath, "w") as f:
                f.write(frontmatter.dumps(post))

            # Update config
            domain.increment_id()

            return {"created": True, "id": task_id, "file": filename, "domain": domain.name}

        # -- punchlist_update --
        if name == "punchlist_update":
            domain = workspace.get_domain(args["domain"])
            if not domain:
                raise ValueError(f"Unknown domain: {args['domain']}")

            path = domain.find_task_file(args["id"])
            if not path:
                raise FileNotFoundError(f"Task {args['id']} not found in {args['domain']}")

            post = frontmatter.load(str(path))
            changes = []

            # State change
            if "state" in args:
                new_state = args["state"]
                if new_state.upper() not in [s.upper() for s in domain.states]:
                    raise ValueError(
                        f"Invalid state '{new_state}' for domain '{domain.name}'. "
                        f"Valid states: {domain.states}"
                    )
                old_state = post.metadata.get("state", "unknown")
                post.metadata["state"] = new_state
                changes.append(f"State changed from {old_state} to {new_state}")

                # Set started_at on first non-TODO state
                if new_state.upper() in ("BEGUN", "DOING") and not post.metadata.get("started_at"):
                    post.metadata["started_at"] = now_iso()

                # Set completed_at on terminal states
                if new_state.upper() in ("DONE", "NOTDO"):
                    post.metadata["completed_at"] = now_iso()

            # Priority
            if "priority" in args:
                old_p = post.metadata.get("priority", "unset")
                post.metadata["priority"] = args["priority"]
                changes.append(f"Priority changed from {old_p} to {args['priority']}")

            # Tags
            if "tags" in args:
                post.metadata["tags"] = args["tags"]
                changes.append(f"Tags set to {args['tags']}")
            if "add_tags" in args:
                existing = post.metadata.get("tags") or []
                post.metadata["tags"] = list(set(existing + args["add_tags"]))
                changes.append(f"Added tags: {args['add_tags']}")
            if "remove_tags" in args:
                existing = post.metadata.get("tags") or []
                post.metadata["tags"] = [t for t in existing if t not in args["remove_tags"]]
                changes.append(f"Removed tags: {args['remove_tags']}")

            # Append note
            if "add_note" in args:
                note_line = f"\n- {now_iso()}: {args['add_note']}\n"
                if "## Notes" in post.content:
                    post.content = post.content.replace("## Notes\n", f"## Notes\n{note_line}", 1)
                else:
                    post.content += f"\n## Notes\n{note_line}"
                changes.append("Added note")

            # Append log (auto-log state changes + explicit log)
            log_lines = []
            for change in changes:
                log_lines.append(f"\n- {now_iso()}: {change}")
            if "add_log" in args:
                log_lines.append(f"\n- {now_iso()}: {args['add_log']}")

            if log_lines:
                log_text = "".join(log_lines) + "\n"
                if "## Log" in post.content:
                    post.content = post.content.replace("## Log\n", f"## Log\n{log_text}", 1)
                else:
                    post.content += f"\n## Log\n{log_text}"

            # Update timestamp
            post.metadata["updated_at"] = now_iso()

            # Write back
            with open(path, "w") as f:
                f.write(frontmatter.dumps(post))

            return {"updated": True, "id": args["id"], "changes": changes}

        # -- punchlist_summary --
        if name == "punchlist_summary":
            target = args.get("domain")
            domains_to_summarize = (
                {target: workspace.get_domain(target)}
                if target
                else workspace.domains
            )
            summaries = {}
            for dname, domain in domains_to_summarize.items():
                if domain is None:
                    continue
                by_state: dict[str, int] = {}
                by_priority: dict[str, int] = {}
                tags_count: dict[str, int] = {}
                recent = []

                for path in domain.task_files():
                    try:
                        meta = parse_task_metadata(path)
                    except Exception:
                        continue

                    state = meta.get("state", "unknown")
                    by_state[state] = by_state.get(state, 0) + 1

                    p = str(meta.get("priority", "unset"))
                    by_priority[p] = by_priority.get(p, 0) + 1

                    for t in meta.get("tags") or []:
                        tags_count[t] = tags_count.get(t, 0) + 1

                    recent.append({
                        "id": meta.get("id"),
                        "title": meta.get("title"),
                        "state": state,
                        "updated_at": meta.get("updated_at"),
                    })

                # Sort recent by updated_at descending
                recent.sort(key=lambda x: str(x.get("updated_at", "")), reverse=True)

                # Order states by domain config
                ordered_states = {}
                for s in domain.states:
                    if s in by_state:
                        ordered_states[s] = by_state.pop(s)
                ordered_states.update(by_state)  # any remaining

                top_tags = sorted(tags_count.items(), key=lambda x: x[1], reverse=True)[:15]

                summaries[dname] = {
                    "total": sum(ordered_states.values()),
                    "by_state": ordered_states,
                    "by_priority": by_priority,
                    "top_tags": [{"tag": t, "count": c} for t, c in top_tags],
                    "recent_activity": recent[:10],
                }

            return summaries

        # -- punchlist_cross_domain --
        if name == "punchlist_cross_domain":
            limit = args.get("limit", 50)
            all_results = []
            for dname, domain in workspace.domains.items():
                tasks = _filter_tasks(
                    domain,
                    state=args.get("state"),
                    tag=args.get("tag"),
                    search=args.get("search"),
                    since=args.get("since"),
                    limit=limit,
                )
                for t in tasks:
                    t["_domain"] = dname
                all_results.extend(tasks)

            # Sort by updated_at across all domains
            all_results.sort(
                key=lambda x: str(x.get("updated_at", "")), reverse=True
            )
            return {"count": len(all_results[:limit]), "tasks": all_results[:limit]}

        raise ValueError(f"Unknown tool: {name}")

    return server


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

async def main():
    parser = argparse.ArgumentParser(description="Punchlist MCP Server")
    parser.add_argument(
        "--root",
        type=str,
        default=os.environ.get("PUNCHLIST_ROOT", "."),
        help="Root directory to scan for .punchlist domains",
    )
    args = parser.parse_args()

    root = Path(args.root).resolve()
    if not root.exists():
        logger.error(f"Root path does not exist: {root}")
        sys.exit(1)

    logger.info(f"Punchlist MCP starting with root: {root}")
    workspace = PunchlistWorkspace(root)
    server = build_server(workspace)

    async with stdio_server() as (read_stream, write_stream):
        await server.run(read_stream, write_stream, server.create_initialization_options())


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
