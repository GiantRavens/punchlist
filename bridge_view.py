"""Interactive Punchlist deck for the Forge bridge.

The bridge owns presentation; `pin` remains the only task mutation authority.
Every context is resolved through punchlist_board's discovered allowlist, every
write is an argv-only subprocess, and every successful mutation is re-read.
"""
from __future__ import annotations

import html
import json
import logging
import os
import re
import secrets
import shutil
import subprocess
import time
from pathlib import Path
from urllib.parse import quote

from bridge import lcars
from bridge.punchlist_domain import load_board

punchlist_board = load_board()

log = logging.getLogger("forge.punchlist")
CSRF_TOKEN = secrets.token_urlsafe(24)
WRITE_ENABLED = os.environ.get("FORGE_PUNCHLIST_WRITE", "1").lower() not in {"0", "false", "no"}
_STATE_RE = re.compile(r"^\s*- name:\s*([^\s#]+)", re.MULTILINE)


def _contexts() -> dict[str, dict]:
    return {c["id"]: c for c in punchlist_board.contexts()}


def context(context_id: str) -> dict | None:
    return _contexts().get(context_id)


def _run(ctx: dict, args: list[str], timeout: float = 8.0) -> subprocess.CompletedProcess:
    configured = os.environ.get("PIN_BIN", "")
    user_bin = Path.home() / ".local" / "bin" / "pin"
    pin = configured or (str(user_bin) if user_bin.is_file() and os.access(user_bin, os.X_OK)
                         else shutil.which("pin"))
    if not pin:
        raise RuntimeError("pin is not installed on the bridge host")
    started = time.monotonic()
    result = subprocess.run([pin, *args], cwd=ctx["root"], capture_output=True,
                            text=True, timeout=timeout)
    log.info("punchlist action=%s context=%s rc=%d duration_ms=%d", args[0],
             ctx["id"], result.returncode, int((time.monotonic() - started) * 1000))
    if result.returncode:
        msg = (result.stderr or result.stdout or "pin failed").strip().splitlines()[-1]
        raise RuntimeError(msg[:240])
    return result


def _json(ctx: dict, args: list[str]) -> object:
    result = _run(ctx, args)
    if result.stdout.strip() == "No matches found.":
        return []
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"pin returned invalid JSON: {exc}") from exc


def _states(ctx: dict) -> list[str]:
    try:
        text = (ctx["root"] / ".punchlist" / "config.yaml").read_text(errors="replace")
        states = _STATE_RE.findall(text)
        if states:
            return states
    except OSError:
        pass
    return ["TODO", "BEGUN", "FOLLOWUP", "DEFER", "NOTDO", "DONE"]


def _task_list(ctx: dict, state: str = "", q: str = "") -> list[dict]:
    tasks = _json(ctx, ["ls", "--json"])
    if not isinstance(tasks, list):
        return []
    ql = q.casefold().strip()
    return [t for t in tasks
            if (not state or str(t.get("state", "")).upper() == state.upper())
            and (not ql or ql in str(t.get("title", "")).casefold())]


def _shell(title: str, body: str) -> str:
    css = """
body{background:#000;color:#d6dae6;font:14px/1.5 "Helvetica Neue",Arial,sans-serif}
a{color:#f6ef95}.notice{border-left:5px solid #ffce63;background:#111018;padding:7px 12px;margin:8px 0}
.err{border-left-color:#ce6363;color:#ffb4b4}.muted{color:#777f92}.deck{display:grid;grid-template-columns:minmax(270px,38%) 1fr;gap:14px}
.tasks,.ticket{background:#0c0c0f;border-left:5px solid #f6ef95;padding:10px 14px}.task{display:block;text-decoration:none;padding:7px;border-bottom:1px solid #20222b;color:#cfd4e2}.task:hover{background:#151722}.task b{color:#f6ef95}.state{color:#ffce63;font-size:11px;text-transform:uppercase}.meta{color:#747d91;font-size:11px}.body{white-space:pre-wrap;font:13px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace;background:#08090c;padding:12px;overflow:auto}.acts{display:flex;gap:10px;flex-wrap:wrap;margin:10px 0}.acts form{display:flex;gap:7px;align-items:center}.acts input,.acts select,.create input{background:#12141f;color:#d6dae6;border:1px solid #303447;border-radius:5px;padding:6px}.acts button,.create button{background:none;border:0;color:#ffce63;font-weight:700;text-transform:uppercase;cursor:pointer}.create{display:flex;gap:8px;flex-wrap:wrap;margin:10px 0}.create input[name=title]{min-width:280px;flex:1}.top{display:flex;align-items:center;gap:12px}.top .launch{margin-left:auto;text-decoration:none;text-transform:uppercase;font-weight:700;color:#ce9cce}
@media(max-width:760px){.deck{grid-template-columns:1fr}}
"""
    return (f"<!doctype html><html><head><meta charset=utf-8><meta name=viewport content='width=device-width,initial-scale=1'>"
            f"<title>{html.escape(title)}</title><style>{css}</style>{lcars.CSS_LINK}</head><body>"
            f"{lcars.shell_open('Punchlists', lcars.PALETTE['pale'], active='/punchlists')}"
            f"{body}{lcars.SHELL_CLOSE}</body></html>")


def render_index(notice: str = "") -> str:
    cards = "".join(
        f'<a class=task href="/punchlists/{html.escape(c["id"])}"><b>{html.escape(c["label"])}</b>'
        f'<div class=meta>{html.escape(c["rel"] or "notebook")}</div></a>'
        for c in punchlist_board.contexts())
    msg = f'<div class=notice>{html.escape(notice)}</div>' if notice else ""
    return _shell("Punchlists", f"{msg}<h1>Punchlist contexts</h1><div class=tasks>{cards}</div>")


def render_context(ctx: dict, state: str = "", q: str = "", notice: str = "",
                   selected: int | None = None) -> str:
    e = html.escape
    tasks = _task_list(ctx, state=state, q=q)
    states = _states(ctx)
    opts = '<option value="">open + closed</option>' + "".join(
        f'<option value="{e(s)}"{(" selected" if s == state else "")}>{e(s)}</option>' for s in states)
    rows = "".join(
        f'<a class=task href="/punchlists/{e(ctx["id"])}/{int(t["id"])}?state={quote(state)}&q={quote(q)}">'
        f'<span class=state>{e(str(t.get("state", "")))}</span> <b>#{int(t["id"])}</b> {e(str(t.get("title", "")))}'
        f'<div class=meta>priority {int(t.get("priority") or 0)} · updated {e(str(t.get("updated_at", ""))[:16])}</div></a>'
        for t in tasks)
    launch_rel = quote(ctx["rel"], safe="")
    top = (f'<div class=top><h1>{e(ctx["label"])}</h1><a class=launch '
           f'href="hammerspoon://pin-browse?context={launch_rel}">open in TUI</a></div>')
    filt = (f'<form class=bar method=get><input name=q value="{e(q)}" placeholder="filter titles">'
            f'<select name=state>{opts}</select><button>engage</button></form>')
    create = ""
    if WRITE_ENABLED:
        create = (f'<form class=create method=post action=/punchlists/action>'
                  f'<input type=hidden name=csrf value="{CSRF_TOKEN}"><input type=hidden name=context value="{e(ctx["id"])}">'
                  f'<input type=hidden name=do value=create><input name=title required placeholder="new task">'
                  f'<input name=priority type=number min=0 max=10 placeholder=priority><input name=tags placeholder="tags: comma separated">'
                  f'<button>add pin</button></form>')
    msg = f'<div class=notice>{e(notice)}</div>' if notice else ""
    detail = '<div class=ticket><span class=muted>Select a ticket to review it.</span></div>'
    if selected is not None:
        detail = render_ticket(ctx, selected, states)
    return _shell(f'{ctx["label"]} Punchlist', f'{msg}{top}{filt}{create}<div class=deck><div class=tasks>{rows or "<span class=muted>No matches.</span>"}</div>{detail}</div>')


def render_ticket(ctx: dict, task_id: int, states: list[str] | None = None) -> str:
    e = html.escape
    task = _json(ctx, ["show", "--json", str(task_id)])
    if not isinstance(task, dict):
        raise RuntimeError("pin show returned no task")
    states = states or _states(ctx)
    controls = ""
    if WRITE_ENABLED:
        common = (f'<input type=hidden name=csrf value="{CSRF_TOKEN}"><input type=hidden name=context value="{e(ctx["id"])}">'
                  f'<input type=hidden name=id value="{task_id}">')
        state_opts = "".join(f'<option{(" selected" if s == task.get("state") else "")}>{e(s)}</option>' for s in states)
        controls = (f'<div class=acts><form method=post action=/punchlists/action>{common}<input type=hidden name=do value=state>'
                    f'<select name=state>{state_opts}</select><button>set state</button></form>'
                    f'<form method=post action=/punchlists/action>{common}<input type=hidden name=do value=priority>'
                    f'<input name=priority type=number min=0 max=10 value="{int(task.get("priority") or 0)}"><button>set priority</button></form></div>'
                    f'<form class=create method=post action=/punchlists/action>{common}<input type=hidden name=do value=note>'
                    f'<input name=note required placeholder="add a review note"><button>comment</button></form>')
    tags = ", ".join(task.get("tags") or [])
    return (f'<div class=ticket><span class=state>{e(str(task.get("state", "")))}</span>'
            f'<h2>#{task_id} {e(str(task.get("title", "")))}</h2>'
            f'<div class=meta>priority {int(task.get("priority") or 0)} · {e(tags)}</div>{controls}'
            f'<div class=body>{e(str(task.get("body", "")))}</div></div>')


def mutate(form: dict[str, str]) -> tuple[str, str]:
    if not WRITE_ENABLED:
        raise PermissionError("Punchlist web writes are disabled")
    if not secrets.compare_digest(form.get("csrf", ""), CSRF_TOKEN):
        raise PermissionError("invalid CSRF token")
    ctx = context(form.get("context", ""))
    if not ctx:
        raise ValueError("unknown Punchlist context")
    action = form.get("do", "")
    task_id = int(form.get("id") or 0)
    manifest = {"context": ctx["id"], "action": action, "task_id": task_id or None}
    log.info("punchlist mutation manifest=%s", json.dumps(manifest, sort_keys=True))
    if action == "create":
        title = form.get("title", "").strip()
        if not title:
            raise ValueError("title is required")
        args = ["todo", title]
        if form.get("priority", "").strip():
            args.append("pri:" + str(int(form["priority"])))
        tags = [x.strip() for x in form.get("tags", "").split(",") if x.strip()]
        if tags:
            args.append("tags:{" + ",".join(tags) + "}")
        result = _run(ctx, args)
        match = re.search(r"Created task (\d+)", result.stdout)
        if not match:
            raise RuntimeError("pin did not report the created task ID")
        task_id = int(match.group(1))
        _json(ctx, ["show", "--json", str(task_id)])
        return f"Created task #{task_id}", f"/punchlists/{ctx['id']}/{task_id}"
    if task_id < 1:
        raise ValueError("task ID is required")
    if action == "note":
        note = form.get("note", "").strip()
        if not note:
            raise ValueError("note is required")
        _run(ctx, ["note", str(task_id), note])
    elif action == "priority":
        _run(ctx, ["pri", str(task_id), str(int(form.get("priority") or 0))])
    elif action == "state":
        state = form.get("state", "")
        if state not in _states(ctx):
            raise ValueError("invalid state")
        state_cmd = {"BEGUN": "start", "DONE": "done", "TODO": "todo",
                     "FOLLOWUP": "confirm", "NOTDO": "notdo"}.get(state, state)
        _run(ctx, [state_cmd, str(task_id)])
    else:
        raise ValueError("unknown Punchlist action")
    verified = _json(ctx, ["show", "--json", str(task_id)])
    if not isinstance(verified, dict) or int(verified.get("id") or 0) != task_id:
        raise RuntimeError("post-write verification failed")
    return f"Updated task #{task_id}", f"/punchlists/{ctx['id']}/{task_id}"
