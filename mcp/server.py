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
# Logging — stderr only (stdout is MCP stdio transport)
# ---------------------------------------------------------------------------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    stream=sys.stderr,
)
logger = logging.getLogger("punchlist-mcp")

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Match Go's DefaultLsStateOrder
DEFAULT_STATES = ["TODO", "BEGUN", "FOLLOWUP", "DEFER", "NOTDO", "DONE"]
DEFAULT_ID_WIDTH = 3
DEFAULT_TITLE_MAX_LEN = 80

# Match Go's slugifyRegex: [^a-z0-9]+
_SLUG_RE = re.compile(r"[^a-z0-9]+")


def slugify(text: str) -> str:
    """Convert title to filename slug — matches Go's slugify exactly."""
    s = text.lower()
    s = _SLUG_RE.sub("-", s)
    s = s.strip("-")
    return s


def truncate_with_ellipsis(text: str, max_len: int) -> str:
    """Truncate title with '...' suffix — matches Go's truncateWithEllipsis."""
    if max_len <= 0:
        return text
    if len(text) <= max_len:
        return text
    if max_len <= 3:
        return text[:max_len]
    return text[: max_len - 3] + "..."


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


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
# Section manipulation — matches Go's splitSection/appendEntry/joinBlocks
# ---------------------------------------------------------------------------


def split_section(body: str, heading: str) -> tuple[str, str, str, bool]:
    """Split markdown body into (before, section, after, found) for a heading."""
    idx = body.find(heading)
    if idx == -1:
        return body, "", "", False
    before = body[:idx]
    rest = body[idx:]
    search_start = len(heading)
    next_idx = rest.find("\n## ", search_start)
    if next_idx == -1:
        return before, rest, "", True
    return before, rest[:next_idx], rest[next_idx:], True


def append_entry(section: str, entry: str) -> str:
    """Append a list entry to a section with spacing."""
    section = section.rstrip("\n")
    if not section:
        return entry + "\n\n"
    return section + "\n\n" + entry + "\n\n"


def join_blocks(*blocks: str) -> str:
    """Join markdown blocks with blank lines, trimming edges."""
    cleaned = []
    for b in blocks:
        b = b.strip("\n")
        if b:
            cleaned.append(b)
    return "\n\n".join(cleaned)


def add_note_to_body(body: str, message: str) -> str:
    """Append a timestamped note to ## Notes section — matches Go's addNote."""
    note_entry = f"- {now_iso()}: {message}"
    # Extract log section first (it goes at the end)
    pre_body, log_section, after_log, log_found = split_section(body, "## Log")
    if log_found:
        pre_body = join_blocks(pre_body, after_log)

    before_notes, notes_section, after_notes, notes_found = split_section(
        pre_body, "## Notes"
    )
    if not notes_found:
        notes_section = "## Notes"

    notes_section = append_entry(notes_section, note_entry)
    new_pre_body = join_blocks(before_notes, notes_section, after_notes)
    if log_found:
        return join_blocks(new_pre_body, log_section)
    return new_pre_body


def add_log_to_body(body: str, message: str) -> str:
    """Append a timestamped log entry to ## Log section — matches Go's addLog."""
    log_entry = f"- {now_iso()}: {message}"
    pre_body, log_section, after_log, log_found = split_section(body, "## Log")
    if not log_found:
        log_section = "## Log"
    log_section = append_entry(log_section, log_entry)
    return join_blocks(pre_body, after_log, log_section)


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
        """Get valid states — check rich 'states' config first, then ls_state_order."""
        rich_states = self.config.get("states")
        if rich_states and isinstance(rich_states, list):
            names = []
            for s in rich_states:
                if isinstance(s, dict) and "name" in s:
                    names.append(s["name"])
                elif isinstance(s, str):
                    names.append(s)
            if names:
                return names
        return self.config.get("ls_state_order", DEFAULT_STATES)

    @property
    def state_aliases(self) -> dict[str, str]:
        """Build alias→canonical name map from rich states config."""
        aliases = {}
        rich_states = self.config.get("states")
        if rich_states and isinstance(rich_states, list):
            for s in rich_states:
                if isinstance(s, dict) and "name" in s:
                    name = s["name"]
                    aliases[name.upper()] = name
                    for alias in s.get("aliases", []):
                        aliases[alias.upper()] = name
        else:
            for s in self.states:
                aliases[s.upper()] = s
        return aliases

    def resolve_state(self, token: str) -> Optional[str]:
        """Resolve a state token (name or alias) to canonical name."""
        aliases = self.state_aliases
        key = token.upper().strip()
        return aliases.get(key)

    def validate_state(self, token: str) -> str:
        """Validate and resolve state, raising ValueError if invalid."""
        resolved = self.resolve_state(token)
        if resolved:
            return resolved
        raise ValueError(
            f"Invalid state '{token}' for domain '{self.name}'. "
            f"Valid states: {self.states}"
        )

    @property
    def id_width(self) -> int:
        return self.config.get("id_width", DEFAULT_ID_WIDTH)

    @property
    def next_id(self) -> int:
        return self.config.get("next_id", 1)

    @property
    def title_max_len(self) -> int:
        return self.config.get("title_max_len", DEFAULT_TITLE_MAX_LEN)

    def task_files(self) -> list[Path]:
        if not self.tasks_dir.exists():
            return []
        return sorted(self.tasks_dir.glob("*.md"))

    def find_task_file(self, task_id: int) -> Optional[Path]:
        """Find task file by ID — matches Go's findTaskFile (parse prefix as int)."""
        if not self.tasks_dir.exists():
            return None
        for f in self.tasks_dir.iterdir():
            if not f.suffix == ".md":
                continue
            parts = f.name.split("-", 1)
            if not parts[0]:
                continue
            try:
                parsed_id = int(parts[0])
            except ValueError:
                continue
            if parsed_id == task_id:
                return f
        return None

    def increment_id(self):
        """Increment next_id and write config back preserving field order."""
        self.config["next_id"] = self.next_id + 1
        config_path = self.root / ".punchlist" / "config.yaml"
        # Atomic write via tmp file — matches Go's SaveConfig pattern
        tmp_path = str(config_path) + ".tmp"
        with open(tmp_path, "w") as f:
            yaml.dump(self.config, f, default_flow_style=False, sort_keys=False)
        os.rename(tmp_path, str(config_path))


class PunchlistWorkspace:
    """Discovers and manages all punchlist domains under a root."""

    def __init__(self, root: Path):
        self.root = root
        self.domains: dict[str, PunchlistDomain] = {}
        self._discover()

    def _discover(self):
        """Walk filesystem recursively for .punchlist/ directories."""
        self.domains.clear()

        # Load root config as fallback
        root_config_path = self.root / ".punchlist" / "config.yaml"
        root_config = {}
        if root_config_path.exists():
            with open(root_config_path) as f:
                root_config = yaml.safe_load(f) or {}

        # Recursive walk — find all dirs containing .punchlist/config.yaml
        for dirpath, dirnames, _filenames in os.walk(self.root):
            # Skip hidden directories and common noise
            dirnames[:] = [
                d
                for d in dirnames
                if not d.startswith(".")
                and d not in ("node_modules", "__pycache__", ".git")
            ]

            dp = Path(dirpath)
            if dp == self.root:
                # Root itself: check if it has tasks/ (makes it a domain too)
                if (self.root / "tasks").exists():
                    self.domains["_root"] = PunchlistDomain(
                        "_root", self.root, root_config
                    )
                continue

            domain_config_path = dp / ".punchlist" / "config.yaml"
            if domain_config_path.exists():
                with open(domain_config_path) as f:
                    domain_config = yaml.safe_load(f) or {}
                # Merge: domain config overrides root config
                merged = {**root_config, **domain_config}
                # Domain name = relative path from root with / separators
                rel = dp.relative_to(self.root)
                domain_name = str(rel)
                self.domains[domain_name] = PunchlistDomain(
                    domain_name, dp, merged
                )
                # Skip tasks/ and .punchlist/ but keep descending for nested domains
                dirnames[:] = [
                    d for d in dirnames
                    if d not in ("tasks", ".punchlist", ".trash")
                ]

        logger.info(
            f"Discovered {len(self.domains)} domain(s): {list(self.domains.keys())}"
        )

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
                        "domain": {
                            "type": "string",
                            "description": "Domain name (e.g. 'work/quantum')",
                        },
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
                        "priority": {
                            "type": "integer",
                            "description": "Filter by priority",
                        },
                        "search": {
                            "type": "string",
                            "description": "Substring match on title",
                        },
                        "since": {
                            "type": "string",
                            "description": "ISO date — tasks updated after this",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Max results (default 50)",
                        },
                        "offset": {
                            "type": "integer",
                            "description": "Pagination offset",
                        },
                        "sort": {
                            "type": "string",
                            "description": "Sort field (default: updated_at)",
                        },
                        "reverse": {
                            "type": "boolean",
                            "description": "Reverse sort order",
                        },
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
                        "query": {
                            "type": "string",
                            "description": "Search text",
                        },
                        "domain": {
                            "type": "string",
                            "description": "Scope to domain (omit for all)",
                        },
                        "include_body": {
                            "type": "boolean",
                            "description": "Include body excerpts (default false)",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Max results (default 20)",
                        },
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
                        "domain": {
                            "type": "string",
                            "description": "Domain name",
                        },
                        "title": {
                            "type": "string",
                            "description": "Task title",
                        },
                        "state": {
                            "type": "string",
                            "description": "Initial state (default: domain's first state)",
                        },
                        "priority": {
                            "type": "integer",
                            "description": "Priority level",
                        },
                        "tags": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "Tags",
                        },
                        "body": {
                            "type": "string",
                            "description": "Initial body/notes content",
                        },
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
                        "domain": {
                            "type": "string",
                            "description": "Domain name",
                        },
                        "id": {"type": "integer", "description": "Task ID"},
                        "state": {
                            "type": "string",
                            "description": "New state",
                        },
                        "priority": {
                            "type": "integer",
                            "description": "New priority",
                        },
                        "tags": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "Replace tags",
                        },
                        "add_tags": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "Append tags",
                        },
                        "remove_tags": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "Remove tags",
                        },
                        "add_note": {
                            "type": "string",
                            "description": "Timestamped note to append",
                        },
                        "add_log": {
                            "type": "string",
                            "description": "Timestamped log entry to append",
                        },
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
                        "domain": {
                            "type": "string",
                            "description": "Domain (omit for all)",
                        },
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
                        "search": {
                            "type": "string",
                            "description": "Title substring",
                        },
                        "since": {
                            "type": "string",
                            "description": "Updated after date",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Max results (default 50)",
                        },
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

        # Resolve state aliases to canonical names for matching
        resolved_states = set()
        for s in states:
            canonical = domain.resolve_state(s)
            if canonical:
                resolved_states.add(canonical.upper())
            else:
                resolved_states.add(s.upper())

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
            if resolved_states:
                task_state = (meta.get("state") or "").upper()
                if task_state not in resolved_states:
                    continue
            if tags:
                task_tags = [t.lower() for t in (meta.get("tags") or [])]
                if not any(t.lower() in task_tags for t in tags):
                    continue
            if priority is not None and meta.get("priority") != priority:
                continue
            if search:
                title_match = search.lower() in (meta.get("title") or "").lower()
                body_match = (
                    include_body
                    and search.lower() in (meta.get("_body") or "").lower()
                )
                if not title_match and not body_match:
                    continue
            if since:
                updated = meta.get("updated_at", "")
                if isinstance(updated, datetime):
                    updated = updated.isoformat()
                if str(updated) < since:
                    continue

            # Strip internal fields for output
            out = {k: v for k, v in meta.items() if not k.startswith("_")}
            if include_body and "_body" in meta:
                out["body"] = meta["_body"]
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
            out: dict[str, Any] = {"root": str(workspace.root), "domains": {}}
            for dname, domain in sorted(workspace.domains.items()):
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
                raise FileNotFoundError(
                    f"Task {args['id']} not found in {args['domain']}"
                )
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
            results: list[dict] = []
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
                        out = {
                            k: v for k, v in task.items() if not k.startswith("_")
                        }
                        out["_domain"] = domain.name
                        if include_body:
                            out["body"] = task.get("_body", "")
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
            state_token = args.get(
                "state", domain.states[0] if domain.states else "TODO"
            )
            state = domain.validate_state(state_token)

            # Truncate title for storage — matches Go behavior
            full_title = title
            title = truncate_with_ellipsis(full_title, domain.title_max_len)

            # Build frontmatter
            meta: dict[str, Any] = {
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

            # Build body — matches Go: "# {fullTitle}\n" then addLog
            body = f"# {full_title}"
            if args.get("body"):
                body = add_note_to_body(body, args["body"])
            body = add_log_to_body(body, "Created task")

            # Build filename — matches Go: fmt.Sprintf("%0*d-%s.md", idWidth, id, slug)
            slug = slugify(title)
            filename = f"{str(task_id).zfill(domain.id_width)}-{slug}.md"
            filepath = domain.tasks_dir / filename
            domain.tasks_dir.mkdir(parents=True, exist_ok=True)

            post = frontmatter.Post(body, **meta)
            # Atomic write
            tmp_path = str(filepath) + ".tmp"
            with open(tmp_path, "w") as f:
                f.write(frontmatter.dumps(post))
            os.rename(tmp_path, str(filepath))

            # Update config
            domain.increment_id()

            return {
                "created": True,
                "id": task_id,
                "file": filename,
                "domain": domain.name,
            }

        # -- punchlist_update --
        if name == "punchlist_update":
            domain = workspace.get_domain(args["domain"])
            if not domain:
                raise ValueError(f"Unknown domain: {args['domain']}")

            path = domain.find_task_file(args["id"])
            if not path:
                raise FileNotFoundError(
                    f"Task {args['id']} not found in {args['domain']}"
                )

            post = frontmatter.load(str(path))
            changes: list[str] = []
            body = post.content

            # State change
            if "state" in args:
                new_state = domain.validate_state(args["state"])
                old_state = post.metadata.get("state", "unknown")
                post.metadata["state"] = new_state
                changes.append(f"State changed from {old_state} to {new_state}")

                # Match Go: started_at only for BEGUN, completed_at only for DONE
                if new_state.upper() == "BEGUN" and not post.metadata.get(
                    "started_at"
                ):
                    post.metadata["started_at"] = now_iso()
                if new_state.upper() == "DONE":
                    post.metadata["completed_at"] = now_iso()

            # Priority
            if "priority" in args:
                old_p = post.metadata.get("priority", "unset")
                post.metadata["priority"] = args["priority"]
                changes.append(
                    f"Priority changed from {old_p} to {args['priority']}"
                )

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
                post.metadata["tags"] = [
                    t for t in existing if t not in args["remove_tags"]
                ]
                changes.append(f"Removed tags: {args['remove_tags']}")

            # Append note — uses section manipulation
            if "add_note" in args:
                body = add_note_to_body(body, args["add_note"])
                changes.append("Added note")

            # Auto-log changes + explicit log
            for change in changes:
                body = add_log_to_body(body, change)
            if "add_log" in args:
                body = add_log_to_body(body, args["add_log"])

            post.content = body
            post.metadata["updated_at"] = now_iso()

            # Atomic write
            tmp_path = str(path) + ".tmp"
            with open(tmp_path, "w") as f:
                f.write(frontmatter.dumps(post))
            os.rename(tmp_path, str(path))

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
            for dname, domain in sorted(domains_to_summarize.items()):
                if domain is None:
                    continue
                by_state: dict[str, int] = {}
                by_priority: dict[str, int] = {}
                tags_count: dict[str, int] = {}
                recent: list[dict] = []

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

                    recent.append(
                        {
                            "id": meta.get("id"),
                            "title": meta.get("title"),
                            "state": state,
                            "updated_at": meta.get("updated_at"),
                        }
                    )

                # Sort recent by updated_at descending
                recent.sort(
                    key=lambda x: str(x.get("updated_at", "")), reverse=True
                )

                # Order states by domain config order
                ordered_states: dict[str, int] = {}
                for s in domain.states:
                    if s in by_state:
                        ordered_states[s] = by_state.pop(s)
                ordered_states.update(by_state)

                top_tags = sorted(
                    tags_count.items(), key=lambda x: x[1], reverse=True
                )[:15]

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
            all_results: list[dict] = []
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

            all_results.sort(
                key=lambda x: str(x.get("updated_at", "")), reverse=True
            )
            return {
                "count": len(all_results[:limit]),
                "tasks": all_results[:limit],
            }

        raise ValueError(f"Unknown tool: {name}")

    return server


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


async def _async_main():
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
        await server.run(
            read_stream, write_stream, server.create_initialization_options()
        )


def main():
    """Sync entry point for console_scripts."""
    import asyncio

    asyncio.run(_async_main())


if __name__ == "__main__":
    main()
