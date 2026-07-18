"""Punchlist-owned Bridge task deck (captain 2026-07-04).

Tasks are the command-deck signal the bridge didn't show. One compact card per
punchlist CONTEXT (any dir with .punchlist/ + tasks/ under the notebook),
answering the captain's three questions plus two doctrine wants:

  - open counts by status (todo · begun — begun highlighted, WIP should be small)
  - STALE-BEGUN flag (begun >7d = the classic started-and-abandoned leak, amber)
  - 7-day FLOW trend in the fleet grammar ("↓−5/7d" closing faster than filing;
    derived purely from created_at/completed_at — no history files)
  - the top-priority open task title (the next action, one line)
  - a fleet-wide recent-activity strip (last task events with ages)

Contexts with nothing open and no 14-day activity do not render (earn-its-keep).
READ-ONLY over the task .md frontmatter; parsing is regex-cheap (no yaml dep)
and cached ~120s — the board refreshes at 60s and tasks change at human speed.
"""
from __future__ import annotations

import html as _html
import hashlib
import os
import re
import time
from pathlib import Path

# Context discovery root. Overridable so tests can point the board at a
# scratch punchlist tree (pin #475) — production never sets the env var.
_NB = Path(os.environ.get("FORGE_PUNCHLIST_NB") or (Path.home() / "Desktop/notebook"))
STALE_BEGUN_DAYS = 7
_SKIP_PARTS = {
    ".stversions", "node_modules", ".trash", ".git", ".venv", "venv",
    "__pycache__", ".cache",
}
# friendly short labels for the busiest contexts
_LABELS = {
    "": "notebook", "forge/document-forge": "df", "forge/knowledge-forge": "kf",
    "forge/signal-forge": "sf", "forge/media-forge": "mf", "forge/contact-forge": "cf",
    "forge/research-forge": "rf", "forge/index-forge": "if",
    "code/forge/document-forge": "df", "forge/state/document-forge": "df",
    "code/forge/knowledge-forge": "kf", "forge/state/knowledge-forge": "kf",
    "code/forge/signal-forge": "sf", "forge/state/signal-forge": "sf",
    "code/forge/media-forge": "mf", "forge/state/media-forge": "mf",
    "code/forge/contact-forge": "cf", "forge/state/contact-forge": "cf",
    "code/forge/research-forge": "rf", "forge/state/research-forge": "rf",
    "code/forge/index-forge": "if", "forge/state/index-forge": "if",
    "code/punchlist": "punchlist", "work/quantum": "quantum",
}

_FM_RE = re.compile(
    r"^(id|title|state|priority|created_at|updated_at|started_at|completed_at):\s*(.*)$")

_cache: dict = {"ts": 0.0, "data": None}
_contexts_cache: dict = {"ts": 0.0, "root": None, "data": None}
CONTEXT_CACHE_SECONDS = 60


def _parse_task(p: Path) -> dict | None:
    try:
        head = p.read_text(errors="replace")[:1200]
    except OSError:
        return None
    if not head.startswith("---"):
        return None
    t: dict = {}
    for ln in head.splitlines()[1:30]:
        if ln.strip() == "---":
            break
        m = _FM_RE.match(ln)
        if m:
            v = m.group(2).strip().strip("'\"")
            t[m.group(1)] = v
    return t if t.get("state") else None


def _ts(v: str | None) -> float:
    if not v:
        return 0.0
    try:
        # 2026-07-03T10:22:49.040334054-05:00 -> trim sub-seconds + offset colon quirks
        v = re.sub(r"\.\d+", "", v)
        return time.mktime(time.strptime(v[:19], "%Y-%m-%dT%H:%M:%S"))
    except ValueError:
        return 0.0


def _contexts() -> list[tuple[str, Path]]:
    """Discover contexts without crossing the depth boundary or cargo trees.

    ``Path.glob("**/.punchlist")`` walks the entire notebook before callers can
    reject deep matches. On the lifetime corpus that made the interactive index
    a 25-second request. ``os.walk`` lets us prune before descent and never enter
    task payload, dependency, version-history, or hidden VCS trees.
    """
    out = []
    root = _NB.resolve()
    for current, dirnames, _filenames in os.walk(root, followlinks=False):
        directory = Path(current)
        depth = len(directory.relative_to(root).parts)
        if ".punchlist" in dirnames:
            tasks = directory / "tasks"
            if tasks.is_dir():
                rel = str(directory.relative_to(root))
                rel = "" if rel == "." else rel
                out.append((_LABELS.get(rel, rel.split("/")[-1]), tasks))
        # Context roots may be at depth three (marker depth four). Never enter
        # their task files or configuration, and prune ignored cargo *before*
        # os.walk has a chance to traverse it.
        dirnames[:] = [
            name for name in dirnames
            if name not in _SKIP_PARTS and name not in {".punchlist", "tasks"}
        ]
        if depth >= 3:
            dirnames[:] = []
    # code-tree markers symlink their tasks/ into the data tree — dedupe by
    # resolved dir, keeping the shortest label (df beats document-forge)
    by_dir: dict = {}
    for label, tasks in out:
        key = tasks.resolve()
        if key not in by_dir or len(label) < len(by_dir[key][0]):
            by_dir[key] = (label, tasks)
    return sorted(by_dir.values())


def contexts() -> list[dict]:
    """Public, allowlisted context registry for the interactive task deck."""
    now = time.monotonic()
    root = _NB.resolve()
    if (_contexts_cache["data"] is not None
            and _contexts_cache["root"] == root
            and now - _contexts_cache["ts"] < CONTEXT_CACHE_SECONDS):
        return list(_contexts_cache["data"])
    out = []
    for label, tasks in _contexts():
        root = tasks.parent.resolve()
        rel = str(root.relative_to(_NB.resolve()))
        rel = "" if rel == "." else rel
        slug = re.sub(r"[^a-z0-9]+", "-", label.lower()).strip("-") or "notebook"
        digest = hashlib.sha256(rel.encode()).hexdigest()[:8]
        out.append({"id": f"{slug}-{digest}", "label": label, "root": root,
                    "tasks": tasks.resolve(), "rel": rel})
    _contexts_cache.update(ts=now, root=_NB.resolve(), data=tuple(out))
    return out


def gather() -> dict:
    """Stale-while-revalidate: the full scan costs seconds (a **/.punchlist glob over the
    whole notebook + thousands of task files), so an expired cache is served IMMEDIATELY
    while one daemon thread rebuilds — the 60s-refresh bridge never blocks on this
    auxiliary deck. COLD is non-blocking too (df#464: zero walks on the request path —
    the first render after a service restart paid ~4s here, worse under IO load): kick
    the build and return a 'building' placeholder; the next refresh has the board."""
    now = time.time()
    if _cache["data"] is not None:
        if now - _cache["ts"] >= 120 and not _cache.get("refreshing"):
            _cache["refreshing"] = True
            import threading
            threading.Thread(target=_rebuild, daemon=True).start()
        return _cache["data"]
    if not _cache.get("refreshing"):
        _cache["refreshing"] = True
        import threading
        threading.Thread(target=_rebuild, daemon=True).start()
    return {"cards": [], "recent": [], "now": now, "quiet_n": 0, "quiet_open": 0,
            "building": True}


def _rebuild() -> dict:
    now = time.time()
    week = now - 7 * 86400
    fortnight = now - 14 * 86400
    cards, recent = [], []
    for ctx in contexts():
        label, tasks_dir = ctx["label"], ctx["tasks"]
        todo = begun = stale = done7 = opened7 = 0
        top = None          # highest-priority open task (priority 1 best)
        for p in tasks_dir.glob("*.md"):
            t = _parse_task(p)
            if not t:
                continue
            st = t["state"].upper()
            created = _ts(t.get("created_at"))
            updated = _ts(t.get("updated_at")) or created
            if created >= week:
                opened7 += 1
            if st == "DONE":
                if _ts(t.get("completed_at")) >= week:
                    done7 += 1
            elif st in ("TODO", "BEGUN", "BACKLOG"):
                if st == "BEGUN":
                    begun += 1
                    if (_ts(t.get("started_at")) or updated) < now - STALE_BEGUN_DAYS * 86400:
                        stale += 1
                elif st == "TODO":
                    todo += 1
                pri = int(t.get("priority") or 5)
                if st != "BACKLOG" and (top is None or pri < top[0]):
                    top = (pri, t.get("title", "?"))
            if updated >= fortnight - 12 * 86400 and updated >= now - 48 * 3600:
                recent.append({"ts": updated, "label": label, "id": t.get("id", "?"),
                               "state": st.lower(),
                               "title": (t.get("title") or "?")[:46]})
        if todo + begun == 0 and done7 == 0 and opened7 == 0:
            continue        # earn-its-keep: dormant contexts stay off the bridge
        # LIVE contexts get cards; quiet-but-open ones fold into one summary line
        # (a bridge deck of 24 cards is noise — the debt stays visible, not loud)
        alive = begun or done7 or opened7
        delta = opened7 - done7
        trend = (f"↓{delta:+d}/7d" if delta < 0 else
                 (f"↑{delta:+d}/7d" if delta > 0 else "→/7d"))
        cards.append({"id": ctx["id"], "label": label, "todo": todo, "begun": begun, "stale": stale,
                      "done7": done7, "trend": trend, "delta": delta,
                      "top": top[1] if top else "", "alive": bool(alive)})
    quiet = [c for c in cards if not c["alive"]]
    cards = [c for c in cards if c["alive"]]
    cards.sort(key=lambda c: -(c["todo"] + c["begun"]))
    recent.sort(key=lambda r: -r["ts"])
    data = {"cards": cards, "recent": recent[:6], "now": now,
            "quiet_n": len(quiet), "quiet_open": sum(c["todo"] + c["begun"] for c in quiet)}
    _cache.update(ts=now, data=data, refreshing=False)
    return data


def _ago(ts: float, now: float) -> str:
    m = int((now - ts) / 60)
    if m < 60:
        return f"{m}m"
    if m < 48 * 60:
        return f"{m // 60}h"
    return f"{m // 1440}d"


def section() -> str:
    """The bridge section html (styles live in status.py's sheet)."""
    e = _html.escape
    d = gather()
    if d.get("building"):
        return ("<div class=sec id=tasks>punchlists <span class=secnum>03-1701</span></div>"
                "<div class=dim>task board building in background — next refresh has it</div>")
    if not d["cards"]:
        return ""
    cards = []
    for c in d["cards"]:
        tcls = "down" if c["delta"] < 0 else ("up" if c["delta"] > 0 else "")
        stale = (f" <span class=pstale>{c['stale']} stale-begun⚠</span>" if c["stale"] else "")
        begun = (f" · <b class=pbegun>{c['begun']} begun</b>" if c["begun"] else "")
        top = (f"<div class=ptop title=\"{e(c['top'])}\">→ {e(c['top'][:52])}</div>"
               if c["top"] else "")
        cards.append(
            f"<a class=pcard href=\"/punchlists/{e(c['id'])}\"><div class=pname>{e(c['label'])}"
            f" <span class='trend {tcls}'>{c['trend']}</span></div>"
            f"<div class=pcnt><b>{c['todo']}</b> todo{begun}{stale}"
            f" · <span class=pdone>{c['done7']} done/7d</span></div>{top}</a>")
    strip = " &nbsp;·&nbsp; ".join(
        f"<span class=page>{_ago(r['ts'], d['now'])}</span> {e(r['label'])}#{e(str(r['id']))} "
        f"<span class='pst {r['state']}'>{e(r['state'])}</span> {e(r['title'])}"
        for r in d["recent"])
    strip_html = f"<div class=pstrip>{strip}</div>" if strip else ""
    quiet = (f"<div class=pquiet>+ {d['quiet_n']} quiet context(s) holding "
             f"{d['quiet_open']} open task(s) — <code>pin ls</code> in each</div>"
             if d.get("quiet_n") else "")
    return (f"<div class=sec id=tasks>punchlists <span class=secnum>03-1701</span></div>"
            f"<div class=pgrid>{''.join(cards)}</div>{quiet}{strip_html}")
