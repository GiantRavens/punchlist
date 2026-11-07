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
import shutil
import logging
import subprocess
import time
from datetime import datetime
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

# ---------------------------------------------------------------------------
# Writes delegate to the `pin` CLI — the MCP is NOT a second writer.
#
# The Go CLI owns every task-file mutation: disk-floor id allocation (so a
# Syncthing-shared store cannot hand out a duplicate id), the per-project
# write lock (so concurrent writers serialize), and the one canonical
# frontmatter serializer. A Python reimplementation of any of that drifts the
# moment either side changes — which is exactly the bug this server used to
# carry. So mutations shell out to `pin`; only reads stay native here (they
# are read-only and token-efficient). See mcp/CLAUDE.md "Writes shell out".
# ---------------------------------------------------------------------------


def _pin_bin() -> str:
    """Locate the `pin` binary. An explicit PUNCHLIST_PIN_BIN wins (handy for
    pointing at an unreleased build); otherwise PATH, then common install
    locations, then a bare name so subprocess surfaces a clear error."""
    override = os.environ.get("PUNCHLIST_PIN_BIN")
    if override:
        return override
    found = shutil.which("pin")
    if found:
        return found
    for cand in (
        "/opt/homebrew/bin/pin",
        "/usr/local/bin/pin",
        os.path.expanduser("~/.local/bin/pin"),
    ):
        if os.path.exists(cand):
            return cand
    return "pin"


_PIN_VERSION: tuple | None = None


def _pin_version() -> tuple:
    """The installed pin's version as a tuple, read once (`pin version` prints
    "pin 1.3.9"). (0,) when it cannot be read."""
    global _PIN_VERSION
    if _PIN_VERSION is None:
        try:
            out = subprocess.run([_pin_bin(), "version"], capture_output=True, text=True).stdout
            m = re.search(r"(\d+)\.(\d+)\.(\d+)", out)
            _PIN_VERSION = tuple(int(x) for x in m.groups()) if m else (0,)
        except FileNotFoundError:
            _PIN_VERSION = (0,)
    return _PIN_VERSION


def _run_pin(domain: "PunchlistDomain", argv: list[str]) -> str:
    """Run one `pin` mutation inside the domain's directory, returning stdout.

    `pin` resolves its scope by walking up from the working directory to the
    nearest .punchlist/, so the domain directory (which holds that store) IS
    the scope selector — no path token, no PUNCHLIST_ROOT env needed. On a
    non-zero exit `pin` has already written the reason to stderr; surface it.
    """
    cmd = [_pin_bin(), *argv]
    try:
        proc = subprocess.run(
            cmd, cwd=str(domain.root), capture_output=True, text=True
        )
    except FileNotFoundError as e:
        raise RuntimeError(
            f"could not run `pin` ({cmd[0]}): {e}. "
            "Put pin on PATH or set PUNCHLIST_PIN_BIN."
        )
    if proc.returncode != 0:
        detail = (proc.stderr or proc.stdout or "").strip()
        raise RuntimeError(f"`pin {' '.join(argv)}` failed: {detail}")
    return proc.stdout


def _tag_brace(tags: list[str]) -> str:
    """Render a tag list as pin's `{a,b,c}` brace literal."""
    return "{" + ",".join(tags) + "}"


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


class PunchlistWorkspace:
    """Discovers and manages all punchlist domains under a root."""

    # Discovery is depth-bounded. An unbounded walk of the notebook (229K dirs,
    # bulk forge/batch state) took >170s and blew Claude's 30s MCP connect
    # timeout; every real store sits <=4 levels down (2026-10-21 census), so 5
    # leaves one level of headroom at ~0.5s. Override with PUNCHLIST_MAX_DEPTH.
    DEFAULT_MAX_DEPTH = 5
    # refresh() runs on every tool call; rescan at most this often.
    REFRESH_TTL_SECS = 30.0

    def __init__(self, root: Path):
        self.root = root
        self.domains: dict[str, PunchlistDomain] = {}
        self.max_depth = int(
            os.environ.get("PUNCHLIST_MAX_DEPTH", self.DEFAULT_MAX_DEPTH)
        )
        self._last_discover = 0.0
        self._discover()

    def _discover(self):
        """Walk the filesystem (to max_depth) for .punchlist/ directories."""
        self.domains.clear()
        started = time.monotonic()
        base_depth = len(self.root.parts)

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
            if len(dp.parts) - base_depth >= self.max_depth:
                dirnames[:] = []  # check this dir, but descend no further
            if dp == self.root:
                # Root itself: check if it has tasks/ (makes it a domain too)
                if (self.root / "tasks").exists():
                    self.domains["_root"] = PunchlistDomain(
                        "_root", self.root, root_config
                    )
                continue

            domain_config_path = dp / ".punchlist" / "config.yaml"
            if domain_config_path.exists():
                # Dedupe symlink-aliased stores (engine-side convenience links
                # into the state tier): if .punchlist resolves to a store that
                # really lives elsewhere under the root, this path is an alias —
                # skip it; the canonical location registers under its real path.
                store_root = (dp / ".punchlist").resolve().parent
                if store_root != dp.resolve():
                    try:
                        store_root.relative_to(self.root.resolve())
                        dirnames[:] = [
                            d for d in dirnames
                            if d not in ("tasks", ".punchlist", ".trash")
                        ]
                        continue
                    except ValueError:
                        pass  # store lives outside the root: keep the alias
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

        self._last_discover = time.monotonic()
        logger.info(
            f"Discovered {len(self.domains)} domain(s) in "
            f"{self._last_discover - started:.2f}s (max_depth={self.max_depth}): "
            f"{list(self.domains.keys())}"
        )

    def refresh(self):
        """Re-scan for domains, at most once per REFRESH_TTL_SECS."""
        if time.monotonic() - self._last_discover >= self.REFRESH_TTL_SECS:
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
                    "Create a new task in a domain. Delegates to the `pin` CLI, so the "
                    "ID is allocated from the on-disk floor (sync-safe), the project "
                    "write lock is held, and the file is written by pin's own serializer."
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
                        "ticket": {
                            "type": "boolean",
                            "description": (
                                "Create a ticket: seed ## Problem, ## Approach and "
                                "## Acceptance. Implied by problem, approach or acceptance."
                            ),
                        },
                        "problem": {
                            "type": "string",
                            "description": "Ticket ## Problem: the fault, its root cause and the evidence (needs pin 1.3.9+)",
                        },
                        "approach": {
                            "type": "string",
                            "description": "Ticket ## Approach: the fix (needs pin 1.3.9+)",
                        },
                        "acceptance": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": (
                                "Ticket ## Acceptance: testable post-conditions, one "
                                "checkbox each (needs pin 1.3.9+)"
                            ),
                        },
                    },
                    "required": ["domain", "title"],
                },
            ),
            Tool(
                name="punchlist_update",
                description=(
                    "Update a task's state, priority, tags, and/or append a note. "
                    "Delegates to the `pin` CLI (write-locked, single serializer). "
                    "State changes are validated against the domain's state list; pin "
                    "auto-logs them to ## Log (which is maintained automatically — there "
                    "is no raw log-injection; use add_note for free-form entries)."
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
                            "description": "Timestamped note to append to ## Notes",
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

            # Validate the state against THIS domain's catalog before shelling
            # out, so a bad state fails with a domain-aware message rather than
            # a raw pin error.
            state_token = args.get(
                "state", domain.states[0] if domain.states else "TODO"
            )
            state = domain.validate_state(state_token)

            # `pin <state> "<title>" [pri:N] [tags:{a,b}]` — pin truncates the
            # title, slugifies the filename, floors the id off disk, and writes
            # under the project lock. We reconstruct none of that here.
            argv = [state, args["title"]]
            if args.get("priority") is not None:
                argv.append(f"pri:{int(args['priority'])}")
            if args.get("tags"):
                argv.append(f"tags:{_tag_brace(args['tags'])}")

            # Ticket sections are filled by pin in the create command itself
            # (pin #21), so the task is never written with placeholders. The
            # arguments go through subprocess as a list: no shell quoting.
            fills = [("--problem", args.get("problem")), ("--approach", args.get("approach"))]
            criteria = [c for c in (args.get("acceptance") or []) if str(c).strip()]
            if args.get("ticket") or any(v for _, v in fills) or criteria:
                if any(v for _, v in fills) or criteria:
                    if _pin_version() < (1, 3, 9):
                        raise RuntimeError(
                            "problem/approach/acceptance need pin 1.3.9 or later; "
                            f"{_pin_bin()} reports {'.'.join(map(str, _pin_version()))}"
                        )
                argv.append("--ticket")
                for flag, value in fills:
                    if value and str(value).strip():
                        argv += [flag, str(value)]
                for c in criteria:
                    argv += ["--acceptance", str(c)]

            out = _run_pin(domain, argv)
            m = re.search(r"Created task (\d+): (.+)", out)
            if not m:
                raise RuntimeError(
                    f"could not parse created-task id from pin output: {out!r}"
                )
            task_id = int(m.group(1))
            filename = os.path.basename(m.group(2).strip())

            # The MCP's `body` is a free-form note — route it through `pin note`
            # (which appends to ## Notes), not into ## Log.
            if args.get("body"):
                _run_pin(domain, ["note", str(task_id), args["body"]])

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

            task_id = str(args["id"])
            changes: list[str] = []

            # Each field maps to one `pin` mutation. They run sequentially;
            # each one takes and releases the project write lock, so concurrent
            # updates serialize. pin auto-logs state/priority to ## Log.

            # State — validated against this domain's catalog first. pin also
            # stamps started_at (BEGUN) / completed_at (DONE) itself.
            if "state" in args:
                new_state = domain.validate_state(args["state"])
                _run_pin(domain, [new_state, task_id])
                changes.append(f"State set to {new_state}")

            # Priority (pin treats 0 as clear).
            if args.get("priority") is not None:
                _run_pin(domain, ["pri", task_id, str(int(args["priority"]))])
                changes.append(f"Priority set to {args['priority']}")

            # Tags: replace (`tag set`), add (`tag`), remove (`tag rm`).
            if "tags" in args:
                new_tags = args["tags"] or []
                if new_tags:
                    _run_pin(domain, ["tag", "set", task_id, _tag_brace(new_tags)])
                    changes.append(f"Tags set to {new_tags}")
                else:
                    # Replace-with-empty == clear: drop whatever is present.
                    current = parse_task_metadata(path).get("tags") or []
                    if current:
                        _run_pin(
                            domain, ["tag", "rm", task_id, _tag_brace(current)]
                        )
                    changes.append("Tags cleared")
            if args.get("add_tags"):
                _run_pin(domain, ["tag", task_id, _tag_brace(args["add_tags"])])
                changes.append(f"Added tags: {args['add_tags']}")
            if args.get("remove_tags"):
                _run_pin(
                    domain, ["tag", "rm", task_id, _tag_brace(args["remove_tags"])]
                )
                changes.append(f"Removed tags: {args['remove_tags']}")

            # Free-form note → ## Notes.
            if args.get("add_note"):
                _run_pin(domain, ["note", task_id, args["add_note"]])
                changes.append("Added note")

            return {
                "updated": bool(changes),
                "id": args["id"],
                "changes": changes,
            }

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
