#!/usr/bin/env python3
"""kiroku — Claude Code と Kiro の作業履歴を週カレンダーで可視化する。

読むもの（あるものだけ）:
  Claude Code : ~/.claude/projects/*/*.jsonl
  Kiro IDE    : ~/.kiro/sessions/<hash>/sess_*/{session.json,messages.jsonl}   (v1.0 以降)
                <globalStorage>/kiro.kiroagent/workspace-sessions/*/            (v1.0 より前)
  Kiro CLI    : ~/.kiro/sessions/cli/<id>.{json,jsonl}
1 ファイルの HTML を書き出す。依存なし（Python 3.8+ 標準ライブラリだけ）。

使い方:
    python3 kiroku.py                 # ./kiroku.html を作ってブラウザで開く
    python3 kiroku.py --sources kiro  # Kiro だけ
    python3 kiroku.py -o out.html --no-open --gap 20
"""
import argparse
import json
import os
import platform
import re
import sys
import webbrowser
from datetime import datetime, timedelta, timezone
from pathlib import Path


def parse_ts(v):
    """ISO 文字列 / UNIX 秒 / UNIX ミリ秒 をすべて UNIX 秒に。"""
    if v is None or v == "":
        return None
    if isinstance(v, str) and v.strip().lstrip("-").isdigit():
        v = int(v)
    if isinstance(v, (int, float)):
        return v / 1000 if v > 1e12 else float(v)
    try:
        return datetime.fromisoformat(str(v).replace("Z", "+00:00")).timestamp()
    except ValueError:
        return None


def text_of(content):
    """content（文字列 or ブロック配列）から人が書いた文字だけ取り出す。"""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for b in content:
            if not isinstance(b, dict):
                continue
            if b.get("type") == "text":
                parts.append(b.get("text", ""))
            elif b.get("kind") == "text" and isinstance(b.get("data"), str):  # Kiro CLI
                parts.append(b["data"])
        return "\n".join(p for p in parts if p)
    return ""


def is_noise(text):
    t = text.lstrip()
    return (not t) or t.startswith("<") or t.startswith("Caveat:") or t.startswith("[Request interrupted")


def read_jsonl(path):
    with open(path, encoding="utf-8", errors="replace") as f:
        for line in f:
            try:
                yield json.loads(line)
            except ValueError:
                continue


def read_json(path):
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


# ── 料金（API 換算の目安） ──────────────────────────────────────────
# USD / 100 万トークン: 入力, 出力, キャッシュ書き込み(5分), キャッシュ書き込み(1時間), キャッシュ読み込み
# 出典: https://platform.claude.com/docs/en/about-claude/pricing （2026-10 時点）。--prices で上書きできる。
# モデル ID の先頭一致で引く（長いキーが優先）。サブスクリプションの請求額とは別物。
PRICES = {
    "claude-fable-5-1":  (10, 50, 12.5, 20, 0.25),
    "claude-mythos-5-1": (10, 50, 12.5, 20, 0.25),
    "claude-fable-5":    (10, 50, 12.5, 20, 1.0),
    "claude-mythos-5":   (10, 50, 12.5, 20, 1.0),
    "claude-opus-5-5":   (4, 20, 5, 8, 0.20),
    "claude-opus-5":     (5, 25, 6.25, 10, 0.50),
    "claude-opus-4-8":   (5, 25, 6.25, 10, 0.50),
    "claude-opus-4-7":   (5, 25, 6.25, 10, 0.50),
    "claude-opus-4-6":   (5, 25, 6.25, 10, 0.50),
    "claude-opus-4-5":   (5, 25, 6.25, 10, 0.50),
    "claude-opus-4-1":   (15, 75, 18.75, 30, 1.50),
    "claude-opus-4":     (15, 75, 18.75, 30, 1.50),
    "claude-sonnet-5-5": (2, 10, 2.5, 4, 0.20),
    "claude-sonnet-5":   (2, 10, 2.5, 4, 0.20),
    "claude-sonnet-4-6": (3, 15, 3.75, 6, 0.30),
    "claude-sonnet-4-5": (3, 15, 3.75, 6, 0.30),
    "claude-sonnet-4":   (3, 15, 3.75, 6, 0.30),
    "claude-haiku-4-5":  (1, 5, 1.25, 2, 0.10),
    "claude-3-5-haiku":  (0.8, 4, 1, 1.6, 0.08),
}
USAGE_KEYS = ("in", "out", "cw", "cw1h", "cr")


def model_name(m):
    """claude-haiku-4-5-20251001 → claude-haiku-4-5（日付の版を落としてまとめる）"""
    return re.sub(r"-\d{8}$", "", m) if m else m


def price_of(model):
    m = (model or "").lower().replace("anthropic.", "")
    keys = [k for k in PRICES if m.startswith(k)]
    return PRICES[max(keys, key=len)] if keys else None


def cost_of(model, u):
    p = price_of(model)
    return None if p is None else sum(u.get(k, 0) * p[i] for i, k in enumerate(USAGE_KEYS)) / 1e6


def read_usage(raw):
    """message.usage → {in, out, cw, cw1h, cr}。1時間キャッシュの内訳があれば分ける。"""
    raw = raw if isinstance(raw, dict) else {}
    n = lambda v: v if isinstance(v, (int, float)) else 0
    cw = n(raw.get("cache_creation_input_tokens"))
    cc = raw.get("cache_creation") if isinstance(raw.get("cache_creation"), dict) else {}
    cw1h = n(cc.get("ephemeral_1h_input_tokens"))
    return {"in": n(raw.get("input_tokens")), "out": n(raw.get("output_tokens")),
            "cw": max(0, cw - cw1h), "cw1h": cw1h, "cr": n(raw.get("cache_read_input_tokens"))}


class Usage:
    """1 つの応答が複数行に分かれて記録されるので、メッセージ ID ごとに項目別の最大値をとってから足す。"""
    def __init__(self):
        self.by_msg = {}  # mid -> [t, model, usage]

    def add(self, mid, t, model, raw):
        u, model = read_usage(raw), model_name(model)
        if not mid:
            mid = f"_{len(self.by_msg)}"
        cur = self.by_msg.get(mid)
        if cur is None:
            self.by_msg[mid] = [t, model, u]
        else:
            cur[0] = cur[0] or t
            cur[1] = cur[1] or model
            for k in USAGE_KEYS:
                cur[2][k] = max(cur[2][k], u[k])

    def events(self):
        """[(t, model, usage, cost)]"""
        return [(t, m, u, cost_of(m, u)) for t, m, u in self.by_msg.values() if m != "<synthetic>"]


def sum_usage(evs):
    tot = {k: 0 for k in USAGE_KEYS}
    cost, unknown = 0.0, 0
    for _, _, u, c in evs:
        for k in USAGE_KEYS:
            tot[k] += u[k]
        if c is None:
            unknown += u["in"] + u["out"] + u["cw"] + u["cw1h"] + u["cr"]
        else:
            cost += c
    tot["cost"] = round(cost, 4)
    tot["unpriced"] = unknown
    return tot


# 言い直し・差し戻しっぽい依頼（こじれたセッションの目印）
CORRECTION = re.compile(r"違う|ちがう|そうじゃな|やり直|戻して|元に戻|取り消|じゃなくて|"
                        r"\b(?:wrong|revert|undo|not what|that's not|try again)\b", re.I)


class Session:
    EDIT_TOOLS = {"Edit", "Write", "MultiEdit", "NotebookEdit",  # Claude Code
                  "fsWrite", "fsAppend", "strReplace", "fs_write", "write", "editCode"}  # Kiro

    def __init__(self, source, sid):
        self.source, self.id = source, sid
        self.project = self.branch = self.title = self.resume = None
        self.prompts, self.times, self.tools, self.files = [], [], {}, set()
        self.agent_times = []  # AI が動いていた時刻（待たせ時間の計算用）
        self.interrupts = 0
        self.usage = Usage()      # このセッション本体のトークン
        self.models = {}          # モデル -> 応答数
        self.subagents = []       # サブエージェントの実行
        self.credits = []         # Kiro: [(t, クレジット)]

    def tick(self, ts):
        if ts is not None:
            self.times.append(ts)

    def agent(self, ts):
        self.tick(ts)
        if ts is not None:
            self.agent_times.append(ts)

    def prompt(self, ts, text):
        if text and text.lstrip().startswith("[Request interrupted"):
            self.interrupts += 1
        if text and not is_noise(text):
            self.prompts.append({"t": ts, "text": text.strip()[:400]})

    def waits(self):
        """AI が最後に動いてから、人が次の依頼を出すまでの秒数（30 分以内のものだけ）。"""
        agent, out, prev = sorted(self.agent_times), [], None
        for p in sorted(x["t"] for x in self.prompts if x["t"]):
            last = max((a for a in agent if (prev is None or a > prev) and a < p), default=None)
            if last is not None and 0 < p - last <= 1800:
                out.append([p, round(p - last)])
            prev = p
        return out

    def corrections(self):
        return sum(1 for x in self.prompts if CORRECTION.search(x["text"]))

    def tool(self, name, args=None):
        name = name or "?"
        self.tools[name] = self.tools.get(name, 0) + 1
        args = args if isinstance(args, dict) else {}
        fp = args.get("file_path") or args.get("path") or args.get("targetFile")
        if isinstance(fp, str) and name in self.EDIT_TOOLS:
            self.files.add(fp)


# ── Claude Code ─────────────────────────────────────────────────────

SUBAGENT_TOOLS = ("Task", "Agent")  # v2.1.63 で Task から Agent に名前が変わった


def load_subagent_file(path):
    """<sessionId>/subagents/agent-<id>.jsonl を 1 本読む。"""
    u, times, models, tools = Usage(), [], {}, {}
    for e in read_jsonl(path):
        ts = parse_ts(e.get("timestamp"))
        if ts is not None:
            times.append(ts)
        msg = e.get("message") or {}
        if e.get("type") == "assistant":
            m = model_name(msg.get("model"))
            if m and m != "<synthetic>":
                models[m] = models.get(m, 0) + 1
            u.add(msg.get("id") or e.get("requestId"), ts, m, msg.get("usage"))
            for b in msg.get("content") or []:
                if isinstance(b, dict) and b.get("type") == "tool_use":
                    tools[b.get("name") or "?"] = tools.get(b.get("name") or "?", 0) + 1
    aid = path.stem[len("agent-"):] if path.stem.startswith("agent-") else path.stem
    return {"agentId": aid, "start": min(times) if times else None, "end": max(times) if times else None,
            "events": u.events(), "models": models, "tools": tools}


def load_claude(root):
    for path in sorted(Path(root).glob("*/*.jsonl")):
        s = Session("Claude Code", path.stem)
        summaries, calls, side = [], {}, Usage()
        for e in read_jsonl(path):
            typ = e.get("type")
            if typ == "summary" and e.get("summary"):
                summaries.append(e["summary"])
                continue
            if typ in ("custom-title", "ai-title") and (e.get("title") or e.get("customTitle")):
                s.title = e.get("customTitle") or e.get("title")
                continue
            ts = parse_ts(e.get("timestamp"))
            if ts is None:
                continue
            s.agent(ts) if typ == "assistant" else s.tick(ts)
            s.project = s.project or e.get("cwd")
            s.branch = s.branch or e.get("gitBranch")
            msg = e.get("message") or {}
            if typ == "assistant":
                m = model_name(msg.get("model"))
                # 古い版はサブエージェントの発言も同じファイルに isSidechain つきで混ざる
                (side if e.get("isSidechain") else s.usage).add(msg.get("id") or e.get("requestId"), ts, m, msg.get("usage"))
                if m and m != "<synthetic>" and not e.get("isSidechain"):
                    s.models[m] = s.models.get(m, 0) + 1
            if typ == "user" and not e.get("isMeta") and not e.get("isSidechain"):
                s.prompt(ts, text_of(msg.get("content")))
                for b in msg.get("content") if isinstance(msg.get("content"), list) else []:
                    if isinstance(b, dict) and b.get("type") == "tool_result" and b.get("tool_use_id") in calls:
                        c = calls[b["tool_use_id"]]
                        c["end"] = ts
                        r = e.get("toolUseResult")
                        if isinstance(r, dict):
                            c["agentId"] = r.get("agentId") or c.get("agentId")
                            c["reported"] = {k: r.get(k) for k in ("totalTokens", "totalDurationMs", "totalToolUseCount") if isinstance(r.get(k), (int, float))}
                            if isinstance(r.get("usage"), dict):
                                c["reportedUsage"] = read_usage(r["usage"])
            elif typ == "assistant" and isinstance(msg.get("content"), list) and not e.get("isSidechain"):
                for b in msg["content"]:
                    if isinstance(b, dict) and b.get("type") == "tool_use":
                        s.tool(b.get("name"), b.get("input"))
                        if b.get("name") in SUBAGENT_TOOLS:
                            inp = b.get("input") if isinstance(b.get("input"), dict) else {}
                            calls[b.get("id")] = {"type": inp.get("subagent_type") or "general-purpose",
                                                  "desc": (inp.get("description") or "")[:120],
                                                  "bg": bool(inp.get("run_in_background")), "start": ts, "end": None}
        # サブエージェントの別ファイル（新しい版）を、agentId か時刻でつなぐ
        files = [load_subagent_file(f) for f in sorted((path.parent / path.stem / "subagents").glob("*.jsonl"))]
        order = sorted(calls.values(), key=lambda c: c["start"] or 0)
        for f in files:
            c = next((c for c in order if c.get("agentId") and c["agentId"] == f["agentId"]), None)
            if c is None and f["start"]:
                c = next((c for c in order if "file" not in c and c["start"] and c["start"] - 5 <= f["start"] <= (c["end"] or f["start"]) + 5), None)
            if c is None:
                c = {"type": "subagent", "desc": "", "bg": False, "start": f["start"], "end": f["end"]}
                order.append(c)
            c["file"] = f
        side_evs = side.events() if not files else []
        for ev in list(side_evs):  # 古い形式: isSidechain の行を、呼び出しの時間帯で振り分ける
            c = next((c for c in order if ev[0] and c["start"] and c["start"] - 5 <= ev[0] <= (c["end"] or c["start"]) + 5), None)
            if c is not None:
                c.setdefault("side", []).append(ev)
                side_evs.remove(ev)
        for c in order:
            f = c.get("file")
            evs = f["events"] if f else c.get("side", [])
            if not evs and c.get("reportedUsage"):
                evs = [(c["start"], None, c["reportedUsage"], None)]
            tot = sum_usage(evs)
            if not f and not c.get("reportedUsage") and c.get("reported", {}).get("totalTokens"):
                tot["reportedTokens"] = c["reported"]["totalTokens"]
            end = (f and f["end"]) or c["end"]
            if not end and c.get("reported", {}).get("totalDurationMs") and c["start"]:
                end = c["start"] + c["reported"]["totalDurationMs"] / 1000
            s.subagents.append({"type": c["type"], "desc": c["desc"], "bg": c["bg"], "start": c["start"], "end": end,
                                "model": max(f["models"], key=f["models"].get) if f and f["models"] else
                                         (c["side"][0][1] if c.get("side") else None),
                                "tools": (f and sum(f["tools"].values())) or c.get("reported", {}).get("totalToolUseCount", 0),
                                "usage": tot, "_events": evs})
        if side_evs:  # 古い形式: どの呼び出しにも入らなかった分は 1 つにまとめる
            evs = side_evs
            s.subagents.append({"type": "sidechain", "desc": "", "bg": False, "start": min((t for t, *_ in evs if t), default=None),
                                "end": max((t for t, *_ in evs if t), default=None), "model": None, "tools": 0,
                                "usage": sum_usage(evs), "_events": evs})
        s.title = s.title or (summaries[-1] if summaries else None)
        s.project = s.project or path.parent.name
        s.resume = f"cd {s.project} && claude --resume {s.id}"
        yield s


# ── Kiro ────────────────────────────────────────────────────────────

def kiro_home():
    return Path(os.environ.get("KIRO_HOME") or Path.home() / ".kiro")


def kiro_global_storage():
    sysname = platform.system()
    if sysname == "Darwin":
        bases = [Path.home() / "Library/Application Support/Kiro"]
    elif sysname == "Windows":
        bases = [Path(os.environ.get("APPDATA", Path.home() / "AppData/Roaming")) / "Kiro"]
    else:
        bases = [Path.home() / ".config/Kiro", Path.home() / ".kiro-server/data"]
    return [b / "User/globalStorage/kiro.kiroagent" for b in bases]


def load_kiro_ide(home):
    """Kiro IDE v1.0 以降: sess_<id>/session.json + messages.jsonl（行ごとに timestamp あり）"""
    base = home / "sessions"
    if not base.is_dir():
        return
    for meta_path in sorted(base.glob("*/*/session.json")):
        if meta_path.parent.parent.name == "cli":
            continue
        meta = read_json(meta_path) or {}
        s = Session("Kiro IDE", meta.get("id") or meta_path.parent.name)
        s.title = meta.get("title")
        s.project = ((meta.get("workspacePaths") or meta.get("rootPaths") or [None])[0])
        model = meta.get("modelId")
        s.tick(parse_ts(meta.get("createdAt")))
        msgs = meta_path.parent / "messages.jsonl"
        if msgs.exists():
            for e in read_jsonl(msgs):
                ts = parse_ts(e.get("timestamp"))
                p = e.get("payload") or {}
                s.tick(ts) if p.get("type") == "user" else s.agent(ts)
                if p.get("type") == "user":
                    s.prompt(ts, text_of(p.get("content")))
                elif p.get("type") == "tool_call":
                    s.tool(p.get("toolName"), p.get("args"))
                elif p.get("type") == "usage_summary":
                    used = sum((x or {}).get("usage") or 0 for x in p.get("promptTurnSummaries") or []
                               if (x or {}).get("unit", "credit").startswith("credit"))
                    if used:
                        s.credits.append((ts, used))
                    if model:
                        s.models[model] = s.models.get(model, 0) + 1
        yield s


def load_kiro_cli(home):
    """Kiro CLI: sessions/cli/<id>.json（メタ）+ <id>.jsonl（Prompt/AssistantMessage）"""
    base = home / "sessions" / "cli"
    if not base.is_dir():
        return
    for meta_path in sorted(base.glob("*.json")):
        meta = read_json(meta_path) or {}
        sid = meta.get("session_id") or meta.get("id") or meta_path.stem
        s = Session("Kiro CLI", sid)
        s.title, s.project = meta.get("title"), meta.get("cwd")
        s.tick(parse_ts(meta.get("created_at")))
        s.tick(parse_ts(meta.get("updated_at")))
        turns = (((meta.get("session_state") or {}).get("conversation_metadata") or {})
                 .get("user_turn_metadatas") or [])
        default_model = ((((meta.get("session_state") or {}).get("rts_model_state") or {}).get("model_info") or {})
                         .get("model_id"))
        for t in turns:
            t = t or {}
            te = parse_ts(t.get("end_timestamp"))
            s.agent(te)
            used = sum((m or {}).get("value") or 0 for m in t.get("metering_usage") or []
                       if str((m or {}).get("unit", "credit")).startswith("credit"))
            if used:
                s.credits.append((te, used))
            m = t.get("model") or default_model
            if m:
                s.models[m] = s.models.get(m, 0) + 1
        log = meta_path.with_suffix(".jsonl")
        if log.exists():
            for e in read_jsonl(log):
                data = e.get("data") or {}
                ts = parse_ts(e.get("timestamp") or (data.get("meta") or {}).get("timestamp"))
                s.tick(ts) if e.get("kind") == "Prompt" else s.agent(ts)
                if e.get("kind") == "Prompt":
                    s.prompt(ts, text_of(data.get("content")))
                elif e.get("kind") == "AssistantMessage":
                    for c in data.get("content") or []:
                        if isinstance(c, dict) and c.get("kind") == "toolUse":
                            d = c.get("data") or {}
                            s.tool(d.get("name"), d.get("input"))
        if s.project:
            s.resume = f"cd {s.project} && kiro-cli chat --resume-id {sid}"
        yield s


def load_kiro_ide_legacy(storages):
    """Kiro IDE v1.0 より前: workspace-sessions/<ws>/sessions.json + <sessionId>.json。
    発言ごとの時刻がないので、開始 = dateCreated、終了 = ファイル更新時刻 のざっくり表示。"""
    for gs in storages:
        for index in sorted(gs.glob("workspace-sessions/*/sessions.json")):
            entries = read_json(index)
            if not isinstance(entries, list):
                continue
            for ent in entries:
                if not isinstance(ent, dict) or ent.get("hidden") or not ent.get("sessionId"):
                    continue
                f = index.parent / f"{ent['sessionId']}.json"
                data = read_json(f) or {}
                s = Session("Kiro IDE (旧)", ent["sessionId"])
                s.title = ent.get("title") or data.get("title")
                s.project = data.get("workspacePath") or data.get("workspaceDirectory") or ent.get("workspaceDirectory")
                start = parse_ts(ent.get("dateCreated"))
                s.tick(start)
                for h in data.get("history") or []:
                    m = (h or {}).get("message") or {}
                    if m.get("role") == "user":
                        s.prompt(start, text_of(m.get("content")))
                if f.exists():
                    s.tick(f.stat().st_mtime)
                yield s


# ── まとめ ──────────────────────────────────────────────────────────

def segments(times, gap_sec):
    """イベント時刻の列を、gap 以上あいたところで帯に切る。"""
    segs, start, prev, n = [], times[0], times[0], 1
    for t in times[1:]:
        if t - prev > gap_sec:
            segs.append([start, max(prev, start + 60), n])
            start, n = t, 0
        prev, n = t, n + 1
    segs.append([start, max(prev, start + 60), n])
    return segs


def collect(args):
    loaders = []
    if "claude" in args.sources:
        root = Path(args.root)
        if root.is_dir():
            loaders.append(("Claude Code", root, load_claude(root)))
    if "kiro" in args.sources:
        home = kiro_home()
        loaders.append(("Kiro IDE", home / "sessions", load_kiro_ide(home)))
        loaders.append(("Kiro CLI", home / "sessions/cli", load_kiro_cli(home)))
        storages = [g for g in kiro_global_storage() if g.is_dir()]
        loaders.append(("Kiro IDE (旧)", " / ".join(map(str, storages)) or "なし", load_kiro_ide_legacy(storages)))
    for name, where, gen in loaders:
        n = 0
        try:
            for s in gen:
                n += 1
                yield s
        except OSError as e:
            print(f"  {name}: 読めないファイルがあったのでスキップ ({e})", file=sys.stderr)
        print(f"  {name}: {n} セッション ({where})", file=sys.stderr)


def build(args):
    out = []
    for s in collect(args):
        times = sorted(t for t in s.times if t)
        if not times:
            continue
        project = s.project or "(不明)"
        title = s.title or (s.prompts[0]["text"].splitlines()[0][:80] if s.prompts else "(無題)")
        out.append({
            "id": s.id,
            "source": s.source,
            "project": os.path.basename(project.rstrip("/\\")) or project,
            "projectPath": project,
            "branch": s.branch,
            "title": title,
            "start": times[0], "end": times[-1],
            "events": len(times),
            "segs": segments(times, args.gap * 60),
            "prompts": s.prompts[:50],
            "nPrompts": len(s.prompts),
            "tools": sorted(s.tools.items(), key=lambda kv: -kv[1])[:8],
            "files": sorted(s.files)[:30],
            "nFiles": len(s.files),
            "resume": s.resume,
            "waits": s.waits(),
            "interrupts": s.interrupts,
            "corrections": s.corrections(),
            "models": sorted(s.models.items(), key=lambda kv: -kv[1]),
            "usage": sum_usage(s.usage.events()),
            "subagents": [{k: v for k, v in a.items() if not k.startswith("_")} for a in s.subagents],
            "credits": round(sum(c for _, c in s.credits), 3),
            "cost": round((sum_usage(s.usage.events())["cost"] + sum(a["usage"]["cost"] for a in s.subagents)), 4),
            # 週ごとの集計用（HTML には入れない）
            "_uev": s.usage.events() + [ev for a in s.subagents for ev in a["_events"]],
            "_cev": s.credits,
        })
    out.sort(key=lambda x: x["start"])
    return out


# ── 週のふりかえり ──────────────────────────────────────────────────
# 時刻はすべてこのマシンのローカル時刻で数える。分単位で「誰が動いていたか」を並べて集計する。

FOCUS_MIN = 60      # これ以上続いたら「集中ブロック」
FOCUS_BRIDGE = 5    # この分数までの切れ目はつながっているとみなす
NIGHT = (22, 6)     # 深夜の時間帯


def monday_of(ts):
    d = datetime.fromtimestamp(ts)
    return datetime(d.year, d.month, d.day) - timedelta(days=d.weekday())


def week_stats(data, ws_dt):
    ws = ws_dt.timestamp()
    we = (ws_dt + timedelta(days=7)).timestamp()
    n = int((we - ws) // 60)
    mins = [[] for _ in range(n)]  # 分ごとに動いていた (セッション, プロジェクト)
    ai_min = 0
    for d in data:
        if d["end"] < ws or d["start"] >= we:
            continue
        for a, b, _ in d["segs"]:
            m0, m1 = max(0, int((a - ws) // 60)), min(n, int(-(-(b - ws) // 60)))
            ai_min += max(0, m1 - m0)
            for m in range(m0, m1):
                mins[m].append((d["id"], d["project"]))
    active = [m for m in range(n) if mins[m]]
    if not active:
        return None

    projects = {}
    for m in active:
        names = {p for _, p in mins[m]}
        for p in names:
            projects[p] = projects.get(p, 0) + 1 / len(names)
    parallel = sum(1 for m in active if len({sid for sid, _ in mins[m]}) >= 2)
    max_conc = max(len({sid for sid, _ in mins[m]}) for m in active)
    day_of = lambda m: min(6, m // 1440)  # 夏時間の週は 7 日 ± 1 時間あるので 0〜6 に収める
    is_night = lambda m: (m % 1440) // 60 >= NIGHT[0] or (m % 1440) // 60 < NIGHT[1]
    night = sum(1 for m in active if is_night(m))
    weekend = sum(1 for m in active if day_of(m) >= 5)

    # 集中ブロック: 途切れ（FOCUS_BRIDGE 分まで）を許して続いた作業
    blocks, start, last = [], active[0], active[0]
    for m in active[1:] + [None]:
        if m is not None and m - last <= FOCUS_BRIDGE + 1:
            last = m
            continue
        if last - start + 1 >= FOCUS_MIN:
            share = {}
            for k in range(start, last + 1):
                for p in {p for _, p in mins[k]}:
                    share[p] = share.get(p, 0) + 1
            top = max(share, key=share.get)
            blocks.append({"t": ws + start * 60, "min": last - start + 1, "project": top,
                           "share": round(share[top] / (last - start + 1), 2)})
        if m is not None:
            start = last = m

    # 切り替え: 依頼を出した順に並べて、前の依頼とプロジェクトが変わった回数（日ごと）
    prompts = sorted((p["t"], d["project"]) for d in data for p in d["prompts"] if p["t"] and ws <= p["t"] < we)
    days = [{"active": 0, "night": 0, "switches": 0, "prompts": 0} for _ in range(7)]
    for m in active:
        days[day_of(m)]["active"] += 1
        days[day_of(m)]["night"] += is_night(m)
    prev = None
    for t, proj in prompts:
        di = day_of(int((t - ws) // 60))
        days[di]["prompts"] += 1
        if prev and prev[0] == di and prev[1] != proj:
            days[di]["switches"] += 1
        prev = (di, proj)
    active_days = [x for x in days if x["active"]]

    waits = sorted(w for d in data for t, w in d["waits"] if ws <= t < we)
    pick = lambda q: waits[min(len(waits) - 1, int(len(waits) * q))] if waits else None

    friction = []
    for d in data:
        if not (ws <= d["start"] < we):
            continue
        why, score = [], 0
        if d["corrections"]:
            why.append(f"言い直し {d['corrections']} 回"); score += 2 * d["corrections"]
        if d["interrupts"]:
            why.append(f"中断 {d['interrupts']} 回"); score += 2 * d["interrupts"]
        if d["nPrompts"] >= 15:
            why.append(f"依頼 {d['nPrompts']} 回"); score += (d["nPrompts"] - 15) // 5 + 1
        if score:
            friction.append({"id": d["id"], "title": d["title"], "project": d["project"], "start": d["start"],
                             "score": score, "why": why})
    friction.sort(key=lambda x: -x["score"])

    # AI の使い方: トークン・目安コスト・クレジット・モデル・サブエージェント
    tok = lambda u: u["in"] + u["out"] + u["cw"] + u["cw1h"] + u["cr"]
    models, by_proj, heavy, tot = {}, {}, {}, {k: 0 for k in USAGE_KEYS}
    cost = credits = 0.0
    for d in data:
        for t, m, u, c in d["_uev"]:
            t = t or d["start"]
            if not (ws <= t < we):
                continue
            for k in USAGE_KEYS:
                tot[k] += u[k]
            mm = models.setdefault(m or "（不明）", {"tokens": 0, "cost": 0.0, "msgs": 0})
            mm["tokens"] += tok(u); mm["msgs"] += 1; mm["cost"] += c or 0
            cost += c or 0
            by_proj[d["project"]] = by_proj.get(d["project"], 0) + (c or 0)
            heavy[d["id"]] = heavy.get(d["id"], 0) + (c or 0)
        for t, c in d["_cev"]:
            t = t or d["start"]
            if ws <= t < we:
                credits += c
                by_proj.setdefault(d["project"], 0)
    subs = [a for d in data for a in d["subagents"] if a["start"] and ws <= a["start"] < we]
    sub_types = {}
    for a in subs:
        sub_types[a["type"]] = sub_types.get(a["type"], 0) + 1
    sub_min = sum(max(0, (a["end"] or a["start"]) - a["start"]) for a in subs) / 60
    reads = tot["cr"] + tot["cw"] + tot["cw1h"] + tot["in"]
    usage = {
        "tokens": sum(tot.values()), "out": tot["out"], "cost": round(cost, 2), "credits": round(credits, 2),
        "cacheHit": round(tot["cr"] / reads, 3) if reads else None,
        "models": sorted(([m, round(v["cost"], 2), v["tokens"], v["msgs"]] for m, v in models.items()), key=lambda r: (-r[1], -r[2])),
        "projects": sorted(([k, round(v, 2)] for k, v in by_proj.items() if v), key=lambda kv: -kv[1]),
        "subagents": len(subs), "subMin": round(sub_min), "subTypes": sorted(sub_types.items(), key=lambda kv: -kv[1]),
        "heavy": [{"id": d["id"], "title": d["title"], "project": d["project"], "start": d["start"], "cost": round(heavy[d["id"]], 2),
                   "subagents": len(d["subagents"])}
                  for d in sorted((d for d in data if heavy.get(d["id"])), key=lambda d: -heavy[d["id"]])[:3]],
    }

    return {
        "usage": usage,
        "week": ws_dt.strftime("%Y-%m-%d"),
        "sessions": sum(1 for d in data if d["end"] >= ws and d["start"] < we),
        "prompts": len(prompts),
        "active": len(active), "ai": ai_min,
        "parallel": parallel, "maxConc": max_conc,
        "night": night, "weekend": weekend,
        "focus": blocks,
        "switchesAvg": round(sum(x["switches"] for x in active_days) / len(active_days), 1),
        "switchesMax": max(x["switches"] for x in days),
        "waitMedian": pick(0.5), "waitP90": pick(0.9), "waitCount": len(waits),
        "projects": sorted(([k, round(v)] for k, v in projects.items()), key=lambda kv: -kv[1]),
        "days": days,
        "friction": friction[:3],
    }


def all_weeks(data):
    out, seen = {}, set()
    for d in data:
        w = monday_of(d["start"])
        while w.timestamp() <= d["end"]:
            if w not in seen:
                seen.add(w)
                st = week_stats(data, w)
                if st:
                    out[st["week"]] = st
            w += timedelta(days=7)
    return out


def hm(minutes):
    minutes = int(round(minutes))
    return f"{minutes // 60}時間{minutes % 60:02d}分" if minutes >= 60 else f"{minutes}分"


def secs(v):
    return "—" if v is None else (f"{v}秒" if v < 60 else f"{v // 60}分{v % 60:02d}秒")


def delta(cur, prev, key, fmt=hm):
    if not prev:
        return ""
    d = cur[key] - prev[key]
    return f"（先週から {'+' if d >= 0 else '−'}{fmt(abs(d))}）"


def weekly_markdown(st, prev):
    ws = datetime.strptime(st["week"], "%Y-%m-%d")
    we = ws + timedelta(days=6)
    longest = max((b["min"] for b in st["focus"]), default=0)
    L = [f"# kiroku 週次ふりかえり {ws:%Y/%m/%d}〜{we:%m/%d}", "",
         "> 自分のふりかえり用の数字です。人と比べたり評価に使ったりするためのものではありません。", "",
         "## 注意の使い方", "",
         "| 指標 | 今週 | メモ |", "|---|---|---|",
         f"| 作業していた時間 | {hm(st['active'])} | {delta(st, prev, 'active')} どれかのセッションが動いていた時間 |",
         f"| AI の延べ稼働 | {hm(st['ai'])} | 並列で動かした分も足した合計 |",
         f"| 集中ブロック（{FOCUS_MIN}分以上） | {len(st['focus'])} 回 | 最長 {hm(longest)} |",
         f"| 1 日の切り替え | 平均 {st['switchesAvg']} 回 | 最大 {st['switchesMax']} 回 |",
         f"| 並列で動かしていた時間 | {hm(st['parallel'])} | 最大 {st['maxConc']} 本同時 |",
         f"| 深夜（{NIGHT[0]}〜{NIGHT[1]}時） | {hm(st['night'])} | |",
         f"| 週末 | {hm(st['weekend'])} | |",
         f"| 待たせ時間 | 中央値 {secs(st['waitMedian'])} | 90%点 {secs(st['waitP90'])}（{st['waitCount']} 回） |",
         f"| セッション / 依頼 | {st['sessions']} / {st['prompts']} | |", "",
         "## プロジェクト別の配分", ""]
    total = sum(v for _, v in st["projects"]) or 1
    L += [f"- {k}: {hm(v)}（{round(v * 100 / total)}%）" for k, v in st["projects"]]
    L += ["", "## 日ごと", "", "| 日 | 作業 | 深夜 | 依頼 | 切り替え |", "|---|---|---|---|---|"]
    for i, x in enumerate(st["days"]):
        d = ws + timedelta(days=i)
        z = lambda v, f=str: f(v) if v else "—"
        L.append(f"| {d:%m/%d}({'月火水木金土日'[i]}) | {z(x['active'], hm)} | {z(x['night'], hm)} | {z(x['prompts'])} | {z(x['switches'])} |")
    u = st["usage"]
    cost_delta = ""
    if prev:
        dc = u["cost"] - prev["usage"]["cost"]
        cost_delta = f"先週から {'+' if dc >= 0 else '−'}${abs(dc):,.2f}・"
    L += ["", "## AI の使い方", "",
          "| 指標 | 今週 | メモ |", "|---|---|---|",
          f"| 目安コスト（API 換算） | ${u['cost']:,.2f} | {cost_delta}サブスクの請求額とは別 |",
          f"| トークン | {u['tokens']:,} | うち出力 {u['out']:,} |",
          f"| キャッシュから読んだ割合 | {'—' if u['cacheHit'] is None else str(round(u['cacheHit'] * 100)) + '%'} | 入力のうち |",
          f"| サブエージェント | {u['subagents']} 回 | 延べ {hm(u['subMin'])} |"]
    if u["credits"]:
        L.append(f"| Kiro クレジット | {u['credits']:,} | |")
    L += ["", "モデル別:", ""]
    L += [f"- {m}: ${c:,.2f}・{t:,} トークン・{n} 応答" for m, c, t, n in u["models"]] or ["- なし"]
    if u["heavy"]:
        L += ["", "重かったセッション:", ""]
        L += [f"- {datetime.fromtimestamp(h['start']):%m/%d %H:%M} [{h['project']}] {h['title']} — ${h['cost']:,.2f}" for h in u["heavy"]]
    L += ["", "## 集中ブロック", ""]
    L += [f"- {datetime.fromtimestamp(b['t']):%m/%d %H:%M} から {hm(b['min'])}・{b['project']}（{round(b['share'] * 100)}%）"
          for b in st["focus"]] or ["- なし"]
    L += ["", "## こじれたかもしれないセッション", ""]
    L += [f"- {datetime.fromtimestamp(f['start']):%m/%d %H:%M} [{f['project']}] {f['title']} — {'、'.join(f['why'])}"
          for f in st["friction"]] or ["- なし"]
    L += ["", "## ふりかえりメモ", "", "- よかったこと：", "- 詰まったこと：", "- 来週ためすこと：", ""]
    return "\n".join(L)


HTML = r"""<!doctype html>
<html lang="ja"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<meta name="color-scheme" content="light dark">
<title>kiroku</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect x='2' y='2' width='28' height='28' rx='5' fill='%23c8402f'/%3E%3Ctext x='16' y='23' font-size='19' text-anchor='middle' fill='%23fff' font-family='serif'%3E記%3C/text%3E%3C/svg%3E">
<style>
/* ── tokens ─────────────────────────────────────────────── */
:root{
  color-scheme:light;
  --paper:#f6f3ec; --paper-2:#efebe2; --card:#fbf9f5; --ink:#1d1b18; --ink-2:#55514a; --ink-3:#6b665c;
  --rule:#e2ddd2; --rule-2:#d3cdc0; --shu:#c8402f; --shu-ink:#a8321f; --badge:#c8402f; --night:rgba(60,64,110,.032); --weekend:rgba(29,27,24,.018); --fillk:1;
  --shadow:0 1px 2px rgba(40,30,10,.06),0 8px 24px -12px rgba(40,30,10,.18);
  --grain:.07; --c0:#2f5d9e; --c1:#d9572b; --c2:#1f9b7a; --c3:#e0a100; --c4:#d6699a; --c5:#4f8a1e; --c6:#6b5bb5; --c7:#b8433f; --other:#9a958b;
  --serif:"Shippori Mincho","Hiragino Mincho ProN","Yu Mincho","YuMincho","Noto Serif JP","Noto Serif CJK JP",serif;
  --sans:"Hiragino Sans","Hiragino Kaku Gothic ProN","Noto Sans JP","Noto Sans CJK JP","Yu Gothic UI",system-ui,sans-serif;
  --mono:ui-monospace,"SFMono-Regular",Menlo,Consolas,monospace;
  --ease:cubic-bezier(.2,.7,.1,1);
}
@media (prefers-color-scheme:dark){:root:not([data-theme="light"]){
  color-scheme:dark;
  --paper:#141318; --paper-2:#1b1a20; --card:#1d1c22; --ink:#ece8df; --ink-2:#b5afa3; --ink-3:#8b857a;
  --rule:#2a2830; --rule-2:#36343d; --shu:#e0614c; --shu-ink:#ef7a65; --badge:#c4503d; --night:rgba(140,150,255,.035); --weekend:rgba(255,255,255,.015); --fillk:1.7;
  --shadow:0 1px 2px rgba(0,0,0,.4),0 12px 32px -12px rgba(0,0,0,.6);
  --grain:.0; --c0:#4f7fcc; --c1:#e0683a; --c2:#2aa889; --c3:#bf8a00; --c4:#d0628f; --c5:#5f9c2a; --c6:#8f80dc; --c7:#d65a56; --other:#77736b;
}}
:root[data-theme="dark"]{
  color-scheme:dark;
  --paper:#141318; --paper-2:#1b1a20; --card:#1d1c22; --ink:#ece8df; --ink-2:#b5afa3; --ink-3:#8b857a;
  --rule:#2a2830; --rule-2:#36343d; --shu:#e0614c; --shu-ink:#ef7a65; --badge:#c4503d; --night:rgba(140,150,255,.035); --weekend:rgba(255,255,255,.015); --fillk:1.7;
  --shadow:0 1px 2px rgba(0,0,0,.4),0 12px 32px -12px rgba(0,0,0,.6);
  --grain:.0; --c0:#4f7fcc; --c1:#e0683a; --c2:#2aa889; --c3:#bf8a00; --c4:#d0628f; --c5:#5f9c2a; --c6:#8f80dc; --c7:#d65a56; --other:#77736b;
}

/* ── base ───────────────────────────────────────────────── */
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body::before{content:"";position:fixed;inset:0;pointer-events:none;z-index:50;opacity:var(--grain);mix-blend-mode:multiply;
  background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2' stitchTiles='stitch'/%3E%3CfeColorMatrix values='0 0 0 0 .3 0 0 0 0 .25 0 0 0 0 .2 0 0 0 .6 0'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E")}
body{background:var(--paper);color:var(--ink);font:13px/1.6 var(--sans);-webkit-font-smoothing:antialiased;display:flex;flex-direction:column;overflow:hidden;
  font-feature-settings:"palt" 1}
button,input{font:inherit;color:inherit}
button{cursor:pointer;background:none;border:0;padding:0}
:focus-visible{outline:2px solid var(--shu);outline-offset:2px;border-radius:6px}
.num{font-family:var(--serif);font-variant-numeric:tabular-nums lining-nums;letter-spacing:-.01em}
.muted{color:var(--ink-3)}
.sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
svg.i{width:16px;height:16px;stroke:currentColor;fill:none;stroke-width:1.6;stroke-linecap:round;stroke-linejoin:round;flex:none}

/* ── header ─────────────────────────────────────────────── */
header{display:grid;grid-template-columns:1fr auto 1fr;align-items:center;gap:18px;padding:14px 24px 12px;border-bottom:1px solid var(--rule);background:var(--paper);position:relative;z-index:5}
.brand{display:flex;align-items:center;gap:10px;min-width:0}
.seal{width:30px;height:30px;border-radius:7px;background:var(--badge);color:#fff;display:grid;place-items:center;font:600 18px/1 var(--serif);transform:rotate(-4deg);
  box-shadow:inset 0 0 0 1.5px rgba(255,255,255,.28),0 2px 6px -2px rgba(200,64,47,.6)}
.word{font:600 19px/1 var(--serif);letter-spacing:.06em}
.tag{font-size:11px;color:var(--ink-3);letter-spacing:.08em;margin-top:3px}
.weeknav{display:flex;align-items:center;gap:6px}
.weeknav .range{min-width:232px;text-align:center;padding:0 6px}
.range .y{display:block;font-size:10.5px;letter-spacing:.2em;color:var(--ink-3)}
.range .d{font:500 18px/1.25 var(--serif);letter-spacing:.02em;white-space:nowrap}
.iconbtn{width:34px;height:34px;display:grid;place-items:center;border-radius:9px;color:var(--ink-2);transition:background .2s var(--ease),color .2s}
.iconbtn:hover{background:var(--paper-2);color:var(--ink)}
.pill{height:30px;padding:0 12px;border:1px solid var(--rule-2);border-radius:999px;font-size:12px;color:var(--ink-2);transition:all .2s var(--ease);white-space:nowrap}
.pill:hover{border-color:var(--ink-3);color:var(--ink)}
.tools{display:flex;align-items:center;gap:8px;justify-self:end}
.seg{display:inline-flex;padding:3px;border-radius:10px;background:var(--paper-2);gap:2px}
.seg button{height:26px;padding:0 10px;border-radius:7px;font-size:12px;color:var(--ink-2);transition:all .2s var(--ease);white-space:nowrap}
.seg button:hover{color:var(--ink)}
.seg button[aria-pressed="true"]{background:var(--card);color:var(--ink);box-shadow:0 1px 2px rgba(0,0,0,.08)}
.search{position:relative}
.search input{width:200px;height:32px;border-radius:9px;border:1px solid var(--rule-2);background:var(--card);padding:0 30px 0 32px;font-size:12.5px;transition:border-color .2s,width .3s var(--ease)}
.search input:focus{outline:none;border-color:var(--ink-3);width:240px}
.search svg{position:absolute;left:10px;top:8px;color:var(--ink-3)}
.search kbd{position:absolute;right:8px;top:7px}
kbd{font:11px/1 var(--mono);padding:3px 5px;border-radius:5px;border:1px solid var(--rule-2);color:var(--ink-3);background:var(--card)}

/* ── legend ─────────────────────────────────────────────── */
.legend{display:flex;align-items:center;gap:6px;padding:10px 24px;border-bottom:1px solid var(--rule);overflow-x:auto;scrollbar-width:none}
.legend::-webkit-scrollbar{display:none}
.legend .lab{font-size:11px;letter-spacing:.14em;color:var(--ink-3);margin-right:6px;white-space:nowrap}
.chip{display:inline-flex;align-items:center;gap:7px;height:26px;padding:0 10px 0 8px;border-radius:999px;font-size:12px;color:var(--ink-2);border:1px solid transparent;transition:all .2s var(--ease);white-space:nowrap}
.chip:hover{background:var(--paper-2)}
.chip .dot{width:9px;height:9px;border-radius:3px;background:var(--c)}
.chip .n{font:11px var(--sans);color:var(--ink-3);font-variant-numeric:tabular-nums}
.chip[aria-pressed="false"]{opacity:.42}.chip[aria-pressed="false"] .dot{background:transparent;box-shadow:inset 0 0 0 1.5px var(--c)}
.legend .count{margin-left:auto;font-size:12px;color:var(--ink-3);white-space:nowrap;padding-left:12px}

/* ── layout ─────────────────────────────────────────────── */
main{flex:1;display:grid;grid-template-columns:minmax(0,1fr) 400px;min-height:0}
#cal{overflow:auto;position:relative;scroll-behavior:smooth;scrollbar-width:thin}
aside{border-left:1px solid var(--rule);background:var(--card);overflow:auto;position:relative;scrollbar-width:thin}

/* ── calendar ───────────────────────────────────────────── */
.heads{display:grid;grid-template-columns:56px repeat(7,minmax(0,1fr));position:sticky;top:0;z-index:4;background:color-mix(in oklab,var(--paper) 88%,transparent);backdrop-filter:blur(8px);border-bottom:1px solid var(--rule)}
.head{padding:10px 10px 9px;display:flex;align-items:baseline;gap:7px;border-left:1px solid transparent}
.head .dn{font:500 22px/1 var(--serif);font-variant-numeric:tabular-nums}
.head .dw{font-size:11px;color:var(--ink-3);letter-spacing:.1em}
.head .dm{font-size:10.5px;color:var(--shu-ink);letter-spacing:.06em;font-variant-numeric:tabular-nums;order:-1;margin-right:-2px}
.head.today .dn{color:#fff;background:var(--badge);border-radius:999px;padding:4px 7px 5px;margin:-4px 0 -5px -2px}
.head.today .dw{color:var(--shu-ink)}
.grid{display:grid;grid-template-columns:56px repeat(7,minmax(0,1fr));position:relative}
.hours{position:relative}
.hours span{position:absolute;right:10px;transform:translateY(-50%);font-size:10.5px;color:var(--ink-3);font-variant-numeric:tabular-nums;letter-spacing:.04em}
.day{position:relative;border-left:1px solid var(--rule);
  background-image:linear-gradient(var(--rule) 1px,transparent 1px),linear-gradient(90deg,transparent 0 50%,transparent 50%);
  background-size:100% var(--hh)}
.day.we{background-color:var(--weekend)}
.day.today{background-color:color-mix(in oklab,var(--shu) 4%,transparent)}
.nightband{position:absolute;left:0;right:0;background:var(--night);pointer-events:none}
.half{position:absolute;left:0;right:0;border-top:1px dashed color-mix(in oklab,var(--rule) 70%,transparent);pointer-events:none}
.blk{position:absolute;border-radius:2px 6px 6px 2px;padding:3px 6px 0 7px;overflow:hidden;cursor:pointer;
  background:color-mix(in oklab,var(--c) calc(var(--fill) * var(--fillk)),var(--paper));box-shadow:inset 3px 0 0 var(--c);
  font-size:11px;line-height:1.35;color:var(--ink);text-align:left;
  transition:transform .18s var(--ease),box-shadow .18s var(--ease),opacity .25s var(--ease),filter .25s;
  animation:rise .5s var(--ease) both;animation-delay:var(--delay,0ms)}
.blk.thin{border-radius:1px 3px 3px 1px;padding:0}
.blk .t{display:block;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.blk .m{display:block;color:var(--ink-2);font-size:10.5px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-variant-numeric:tabular-nums}
.blk:hover{transform:translateY(-1px);box-shadow:inset 3px 0 0 var(--c),var(--shadow);z-index:3}
.grid.focus .blk{opacity:.28;filter:saturate(.6)}
.grid.focus .blk.sel{opacity:1;filter:none;box-shadow:inset 3px 0 0 var(--c),0 0 0 1.5px var(--c),var(--shadow);z-index:3}
@keyframes rise{from{opacity:0;transform:translateY(6px)}}
.now{position:absolute;left:-5px;right:0;height:0;border-top:1.5px solid var(--shu);z-index:3;pointer-events:none}
.now::before{content:"";position:absolute;left:0;top:-5px;width:9px;height:9px;border-radius:50%;background:var(--shu);box-shadow:0 0 0 3px color-mix(in oklab,var(--shu) 25%,transparent)}
.nowlab{position:absolute;right:6px;transform:translateY(-50%);font:600 10px/1 var(--sans);color:#fff;background:var(--badge);padding:3px 5px;border-radius:4px;z-index:3;font-variant-numeric:tabular-nums}
.empty{position:sticky;top:38%;height:0;margin-left:56px;text-align:center;pointer-events:none;z-index:2}
.empty div{transform:translateY(-50%)}
.empty .k{width:72px;height:72px;margin:0 auto;border-radius:16px;border:2px solid var(--rule-2);color:var(--rule-2);display:grid;place-items:center;font:500 40px/1 var(--serif);transform:rotate(-4deg)}
.empty p{color:var(--ink-3);margin:12px 0 0}
.daytabs{display:none}

/* tooltip */
.tip{position:fixed;z-index:20;pointer-events:none;max-width:280px;background:var(--ink);color:var(--paper);border-radius:10px;padding:9px 11px;font-size:12px;line-height:1.5;
  box-shadow:0 10px 30px -10px rgba(0,0,0,.5);opacity:0;transform:translateY(4px);transition:opacity .15s,transform .15s var(--ease)}
.tip.on{opacity:1;transform:none}
.tip b{display:block;font-weight:600;margin-bottom:2px}
.tip .r{display:flex;align-items:center;gap:6px;color:color-mix(in oklab,var(--paper) 72%,transparent);font-variant-numeric:tabular-nums}
.tip .r i{width:8px;height:8px;border-radius:2px;background:var(--c)}

/* ── panel ──────────────────────────────────────────────── */
.pane{padding:26px 28px 40px;animation:fade .35s var(--ease) both}
@keyframes fade{from{opacity:0;transform:translateX(8px)}}
.eyebrow{font-size:10.5px;letter-spacing:.22em;color:var(--ink-3);text-transform:uppercase;display:flex;align-items:center;gap:8px}
.eyebrow .dot{width:8px;height:8px;border-radius:2px;background:var(--c)}
h2{font:500 21px/1.45 var(--serif);margin:6px 0 4px;letter-spacing:.01em;overflow-wrap:anywhere}
h3{font:600 11px/1 var(--sans);letter-spacing:.18em;color:var(--ink-3);margin:30px 0 12px;display:flex;align-items:center;gap:10px}
h3::after{content:"";flex:1;height:1px;background:var(--rule)}
.hero{margin:18px 0 6px}
.hero .big{display:flex;align-items:baseline;gap:2px}
.hero .big .num{font-size:52px;line-height:1;font-weight:500}
.hero .big .u{font:500 15px var(--serif);color:var(--ink-2);margin:0 6px 0 3px}
.hero .cap{color:var(--ink-2);margin-top:8px;font-size:12.5px}
.delta{display:inline-flex;align-items:center;gap:4px;font-size:11.5px;color:var(--ink-2);background:var(--paper-2);border-radius:999px;padding:2px 9px;margin-left:8px;font-variant-numeric:tabular-nums}
.stats{display:grid;grid-template-columns:1fr 1fr;border-top:1px solid var(--rule);margin-top:20px}
.stat{padding:14px 0 13px;border-bottom:1px solid var(--rule)}
.stat:nth-child(odd){padding-right:14px;border-right:1px solid var(--rule)}
.stat:nth-child(even){padding-left:16px}
.stat:last-child:nth-child(odd){grid-column:1/-1;border-right:0}
.stat .k{font-size:11.5px;color:var(--ink-3)}
.stat .v{font:500 22px/1.25 var(--serif);font-variant-numeric:tabular-nums;margin-top:3px}
.stat .v small{font:500 12px var(--serif);color:var(--ink-2);margin-left:2px}
.stat .s{font-size:11px;color:var(--ink-3);font-variant-numeric:tabular-nums}
.rhythm{display:grid;grid-template-columns:repeat(7,1fr);gap:8px;align-items:end;height:132px;padding-top:6px}
.col{display:flex;flex-direction:column;align-items:center;gap:6px;height:100%;justify-content:flex-end;cursor:default;min-width:0}
.bar{width:100%;max-width:30px;border-radius:4px 4px 1px 1px;background:var(--ink);opacity:.82;display:flex;flex-direction:column-reverse;overflow:hidden;min-height:2px;transition:opacity .2s}
.bar .nt{background:repeating-linear-gradient(135deg,var(--ink) 0 2px,var(--ink-3) 2px 4px)}
.col:hover .bar{opacity:1}
.col .l{font-size:11px;color:var(--ink-3)}
.col.today .l{color:var(--shu-ink);font-weight:600}
.col .v{font-size:10.5px;color:var(--ink-2);font-variant-numeric:tabular-nums;height:14px}
.keyrow{display:flex;gap:14px;font-size:11px;color:var(--ink-3);margin-top:14px;justify-content:flex-end}
.keyrow i{display:inline-block;width:10px;height:10px;border-radius:2px;vertical-align:-1px;margin-right:5px;background:var(--ink);opacity:.82}
.keyrow i.nt{background:repeating-linear-gradient(135deg,var(--ink) 0 2px,var(--ink-3) 2px 4px)}
.stack{display:flex;gap:2px;height:10px;border-radius:5px;overflow:hidden;margin-bottom:12px}
.stack span{background:var(--c);min-width:2px}
.prow{display:grid;grid-template-columns:12px 1fr auto 42px;align-items:center;gap:8px;padding:5px 0;font-size:12.5px}
.prow i{width:9px;height:9px;border-radius:3px;background:var(--c)}
.prow .nm{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.prow .tm{color:var(--ink-2);font-variant-numeric:tabular-nums}
.prow .pc{text-align:right;color:var(--ink-3);font-variant-numeric:tabular-nums}
.focusrow{display:grid;grid-template-columns:86px 1fr 62px;gap:10px;align-items:center;padding:5px 0;font-size:12px}
.focusrow .when{color:var(--ink-2);font-variant-numeric:tabular-nums}
.focusrow .track{height:6px;border-radius:3px;background:var(--paper-2);overflow:hidden}
.focusrow .track span{display:block;height:100%;background:var(--c);border-radius:3px}
.focusrow .len{text-align:right;font-variant-numeric:tabular-nums}
.card{display:block;width:100%;text-align:left;padding:11px 13px;border:1px solid var(--rule);border-radius:10px;margin-bottom:8px;transition:all .2s var(--ease);background:var(--card)}
.card:hover{border-color:var(--rule-2);box-shadow:var(--shadow);transform:translateY(-1px)}
.card .ti{font-weight:600;display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.card .me{font-size:11.5px;color:var(--ink-3);display:flex;gap:8px;align-items:center;margin-top:3px;flex-wrap:wrap}
.tagx{font-size:11px;color:var(--shu-ink);background:color-mix(in oklab,var(--shu) 10%,transparent);border-radius:4px;padding:0 6px}
.none{color:var(--ink-3);font-size:12.5px}
.foot{margin-top:30px;padding-top:16px;border-top:1px solid var(--rule);font-size:11.5px;color:var(--ink-3);line-height:1.75}
.code{display:flex;align-items:center;gap:8px;margin-top:10px;background:var(--paper-2);border-radius:8px;padding:7px 8px 7px 11px;font:11.5px/1.5 var(--mono);color:var(--ink-2)}
.code code{flex:1;overflow-x:auto;white-space:nowrap;scrollbar-width:none}
.copy{font:11px var(--sans);color:var(--ink-2);border:1px solid var(--rule-2);border-radius:6px;padding:3px 8px;background:var(--card);flex:none}
.copy:hover{color:var(--ink);border-color:var(--ink-3)}
.back{display:inline-flex;align-items:center;gap:6px;font-size:12px;color:var(--ink-3);margin-bottom:18px}
.back:hover{color:var(--ink)}
.meta{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}
.meta span{font-size:11.5px;color:var(--ink-2);border:1px solid var(--rule);border-radius:999px;padding:1px 9px}
.mini{display:grid;grid-template-columns:1fr 1fr;margin-top:18px;border-top:1px solid var(--rule)}
.mini>div{padding:11px 0 10px;border-bottom:1px solid var(--rule)}
.mini>div:nth-child(odd){border-right:1px solid var(--rule);padding-right:12px}.mini>div:nth-child(even){padding-left:16px}
.mini .k{font-size:11px;color:var(--ink-3)}
.mini .v{font:500 20px/1.3 var(--serif);font-variant-numeric:tabular-nums}.mini .v small{font-size:12px;color:var(--ink-2);margin-left:1px}
.tl{list-style:none;margin:0;padding:0;position:relative}
.tl::before{content:"";position:absolute;left:44px;top:6px;bottom:6px;width:1px;background:var(--rule)}
.tl li{display:grid;grid-template-columns:38px 1fr;gap:18px;padding:5px 0;position:relative;font-size:12.5px;line-height:1.6}
.tl li::before{content:"";position:absolute;left:41px;top:12px;width:7px;height:7px;border-radius:50%;background:var(--card);box-shadow:inset 0 0 0 1.5px var(--c)}
.tl li.fix::before{background:var(--shu);box-shadow:none}
.tl time{color:var(--ink-3);font-variant-numeric:tabular-nums;font-size:11.5px;padding-top:1px}
.tl p{margin:0;overflow-wrap:anywhere}
.more{font-size:12px;color:var(--ink-3);margin:6px 0 0 56px}
.trow{display:grid;grid-template-columns:110px 1fr 28px;gap:10px;align-items:center;font-size:12px;padding:3px 0}
.trow .nm{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:var(--mono);font-size:11.5px}
.trow .track{height:6px;border-radius:3px;background:var(--paper-2)}
.trow .track span{display:block;height:100%;border-radius:3px;background:var(--ink-3)}
.trow .n{text-align:right;color:var(--ink-3);font-variant-numeric:tabular-nums}
.files{list-style:none;padding:0;margin:0;font:11.5px/1.7 var(--mono);color:var(--ink-2)}
.files li{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;direction:rtl;text-align:left}
.files li span{direction:ltr;unicode-bidi:plaintext}

/* AI usage */
.mrow{display:grid;grid-template-columns:1fr auto 40px;gap:10px;align-items:center;padding:6px 0;font-size:12.5px;border-bottom:1px dashed var(--rule)}
.mrow:last-child{border-bottom:0}
.mrow .nm{font-family:var(--mono);font-size:11.5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;display:flex;align-items:center;gap:8px}
.mrow .nm i{width:9px;height:9px;border-radius:2px;background:var(--ink);opacity:var(--o);flex:none}
.mrow .tm{color:var(--ink-2);font-variant-numeric:tabular-nums;text-align:right}
.mrow .tm small{color:var(--ink-3);margin-left:6px}
.mrow .pc{text-align:right;color:var(--ink-3);font-variant-numeric:tabular-nums}
.mstack{display:flex;gap:2px;height:10px;border-radius:5px;overflow:hidden;margin-bottom:8px}
.mstack span{background:var(--ink);opacity:var(--o);min-width:2px}
.chips{display:flex;flex-wrap:wrap;gap:6px}
.chips span{font-size:11.5px;color:var(--ink-2);border:1px solid var(--rule);border-radius:999px;padding:1px 9px;font-variant-numeric:tabular-nums}
.chips span b{font-weight:600;color:var(--ink);margin-left:4px}
.chips span.mono{font-family:var(--mono);font-size:11px}
.sub{padding:9px 0 10px;border-bottom:1px solid var(--rule)}
.sub:last-child{border-bottom:0}
.sub .hd{display:flex;align-items:center;gap:8px;font-size:12.5px}
.sub .ty{font:600 11px var(--mono);color:var(--ink);background:var(--paper-2);border-radius:5px;padding:1px 7px;flex:none}
.sub .ds{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--ink-2)}
.sub .bg{font-size:10.5px;color:var(--shu-ink);flex:none}
.sub .lane{position:relative;height:6px;border-radius:3px;background:var(--paper-2);margin:8px 0 6px}
.sub .lane span{position:absolute;top:0;bottom:0;border-radius:3px;background:var(--c);min-width:3px}
.sub .ft{display:flex;gap:12px;font-size:11px;color:var(--ink-3);font-variant-numeric:tabular-nums;flex-wrap:wrap}
.note{font-size:11px;color:var(--ink-3);margin-top:10px;line-height:1.7}
/* toast & dialog */
.toast{position:fixed;left:50%;bottom:28px;transform:translate(-50%,12px);background:var(--ink);color:var(--paper);padding:8px 14px;border-radius:999px;font-size:12.5px;opacity:0;transition:all .25s var(--ease);z-index:30;pointer-events:none}
.toast.on{opacity:1;transform:translate(-50%,0)}
dialog{border:0;border-radius:16px;padding:26px 28px;background:var(--card);color:var(--ink);box-shadow:0 30px 80px -20px rgba(0,0,0,.45);width:min(420px,calc(100vw - 32px))}
dialog::backdrop{background:rgba(20,18,15,.35);backdrop-filter:blur(3px)}
dialog h2{margin-top:0}
.keys{display:grid;grid-template-columns:auto 1fr;gap:9px 16px;font-size:12.5px;margin-top:14px;align-items:center}
.keys kbd{justify-self:start}

/* ── responsive ─────────────────────────────────────────── */
@media (max-width:1180px){main{grid-template-columns:minmax(0,1fr) 340px}.pane{padding:22px 20px 36px}.search input{width:150px}.search input:focus{width:180px}}
@media (max-width:1280px){.tag{display:none}}
@media (max-width:1100px){.tools .seg:not(.zoom){display:none}.search kbd{display:none}}
@media (max-width:820px){
  body{overflow:auto;height:auto}
  header{grid-template-columns:1fr auto;gap:10px;padding:12px 16px}
  .weeknav{order:3;grid-column:1/-1;justify-content:space-between}
  .tools{margin-left:auto}.tools .seg,.tools .zoom{display:none}
  .search input,.search input:focus{width:130px}
  .legend{padding:8px 16px}
  main{display:block}
  #cal{overflow:visible;height:auto}
  .daytabs{display:grid;grid-template-columns:repeat(7,1fr);gap:4px;padding:8px 12px;border-bottom:1px solid var(--rule);position:sticky;top:0;background:var(--paper);z-index:6}
  .daytabs button{padding:6px 0 7px;border-radius:9px;display:flex;flex-direction:column;align-items:center;gap:1px;color:var(--ink-2)}
  .daytabs b{font:500 17px/1.1 var(--serif)}.daytabs small{font-size:10px;color:var(--ink-3)}
  .daytabs button[aria-pressed="true"]{background:var(--ink);color:var(--paper)}.daytabs button[aria-pressed="true"] small{color:inherit;opacity:.7}
  .daytabs button.has::after{content:"";width:4px;height:4px;border-radius:50%;background:var(--shu);margin-top:2px}
  .heads{display:none}
  .grid{grid-template-columns:48px minmax(0,1fr);max-height:62vh;overflow:auto}
  .grid .day.mh{display:none}
  .empty{margin-left:48px;top:30%}
  aside{border-left:0;border-top:1px solid var(--rule)}
}
@media (prefers-reduced-motion:reduce){*,*::before,*::after{animation:none!important;transition:none!important;scroll-behavior:auto!important}}
@media (forced-colors:active){.blk{border:1px solid CanvasText}}
</style></head>
<body>
<header>
  <div class="brand"><div class="seal" aria-hidden="true">記</div><div><div class="word">kiroku</div><div class="tag">AIと過ごした時間の記録</div></div></div>
  <nav class="weeknav" aria-label="週の移動">
    <button class="iconbtn" id="prev" aria-label="前の週"><svg class="i" viewBox="0 0 24 24"><path d="M15 6l-6 6 6 6"/></svg></button>
    <div class="range" aria-live="polite"><span class="y" id="ry"></span><span class="d" id="rd"></span></div>
    <button class="iconbtn" id="next" aria-label="次の週"><svg class="i" viewBox="0 0 24 24"><path d="M9 6l6 6-6 6"/></svg></button>
    <button class="pill" id="today">今週</button>
  </nav>
  <div class="tools">
    <div class="seg" role="group" aria-label="色分け" id="colorBy">
      <button data-v="project">プロジェクト</button><button data-v="branch">ブランチ</button><button data-v="source">ツール</button>
    </div>
    <div class="seg zoom" role="group" aria-label="ズーム">
      <button id="zout" aria-label="縮小">−</button><button id="zin" aria-label="拡大">＋</button>
    </div>
    <label class="search"><span class="sr">検索</span>
      <svg class="i" viewBox="0 0 24 24"><circle cx="11" cy="11" r="6.5"/><path d="M20 20l-4.2-4.2"/></svg>
      <input id="q" type="search" placeholder="タイトル・依頼文で探す" autocomplete="off"><kbd>/</kbd></label>
    <button class="iconbtn" id="theme" aria-label="テーマを切り替え"></button>
    <button class="iconbtn" id="help" aria-label="ショートカット"><svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M9.6 9.3a2.5 2.5 0 014.8.9c0 1.7-2.4 2.2-2.4 3.6M12 17h.01"/></svg></button>
  </div>
</header>
<div class="legend" id="legend" aria-label="凡例"></div>
<main>
  <section id="cal" aria-label="週カレンダー">
    <div class="daytabs" id="daytabs" role="group" aria-label="曜日"></div>
    <div class="heads" id="heads"></div>
    <div class="grid" id="grid"></div>
  </section>
  <aside id="panel" aria-live="polite"></aside>
</main>
<div class="tip" id="tip" role="tooltip"></div>
<div class="toast" id="toast" role="status"></div>
<dialog id="keys">
  <div class="eyebrow">Shortcuts</div><h2>キーボード操作</h2>
  <div class="keys">
    <kbd>←</kbd><span>前の週</span><kbd>→</kbd><span>次の週</span><kbd>T</kbd><span>今週に戻る</span>
    <kbd>/</kbd><span>検索</span><kbd>+</kbd><span>拡大（<kbd>−</kbd> で縮小）</span>
    <kbd>Esc</kbd><span>選択を外して週のまとめに戻る</span><kbd>?</kbd><span>この一覧</span>
  </div>
  <form method="dialog" style="margin-top:22px;text-align:right"><button class="pill">閉じる</button></form>
</dialog>
<script>
const DATA = __DATA__;
const WEEKS = __WEEKS__;
const GENERATED = __GEN__;
const DOW = ["日","月","火","水","木","金","土"], SLOTS = 8;
const $ = s => document.querySelector(s);
const store = { get(k,d){ try { const v = localStorage.getItem("kiroku:"+k); return v == null ? d : JSON.parse(v); } catch(e){ return d; } },
                set(k,v){ try { localStorage.setItem("kiroku:"+k, JSON.stringify(v)); } catch(e){} } };
const st = { hh: store.get("hh", 44), colorBy: store.get("colorBy", "project"), theme: store.get("theme", "auto"),
             week: mondayOf(new Date()), hidden: new Set(), sel: null, q: "", mday: (new Date().getDay()+6)%7, animate: true };

/* ── helpers ── */
function mondayOf(d){ d = new Date(d); d.setHours(0,0,0,0); d.setDate(d.getDate()-((d.getDay()+6)%7)); return d; }
function addDays(d,n){ d = new Date(d); d.setDate(d.getDate()+n); return d; }
function key(d){ return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}-${String(d.getDate()).padStart(2,"0")}`; }
function esc(s){ return String(s ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c])); }
function hm(t){ return new Date(t*1000).toLocaleTimeString("ja-JP",{hour:"2-digit",minute:"2-digit"}); }
function md(t){ const d = new Date(t*1000); return `${d.getMonth()+1}/${d.getDate()}(${DOW[d.getDay()]})`; }
function dur(m, html){ m = Math.round(m); const h = Math.floor(m/60), r = m%60;
  if (!html) return h ? `${h}時間${r ? String(r).padStart(2,"0")+"分" : ""}` : `${r}分`;
  return h ? `${h}<small>時間</small>${r ? String(r).padStart(2,"0")+"<small>分</small>" : ""}` : `${r}<small>分</small>`; }
function tok(n){ n = n || 0; return n >= 1e9 ? (n/1e9).toFixed(1)+"B" : n >= 1e6 ? (n/1e6).toFixed(1)+"M" : n >= 1e3 ? Math.round(n/1e3)+"K" : String(n); }
function usd(v){ return v == null ? "—" : v > 0 && v < 0.01 ? "<$0.01" : "$" + (v >= 100 ? Math.round(v).toLocaleString() : v.toFixed(2)); }
function shade(i){ return [1,.72,.5,.34,.22,.14][Math.min(i,5)]; }
function secs(v){ return v == null ? "—" : v < 60 ? `${v}秒` : `${Math.floor(v/60)}分${String(v%60).padStart(2,"0")}秒`; }
function isoWeek(d){ d = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())); const n = d.getUTCDay() || 7; d.setUTCDate(d.getUTCDate()+4-n);
  const y0 = new Date(Date.UTC(d.getUTCFullYear(),0,1)); return Math.ceil(((d-y0)/864e5+1)/7); }
const keyOf = s => st.colorBy === "project" ? s.project : st.colorBy === "source" ? s.source : `${s.project} · ${s.branch || "—"}`;
let slot = {};
function assignColors(){ // 全期間の多い順に固定。週を変えても、非表示にしても色は変わらない
  const n = {}; DATA.forEach(s => n[keyOf(s)] = (n[keyOf(s)]||0) + 1);
  slot = {}; Object.keys(n).sort((a,b)=>n[b]-n[a]).forEach((k,i) => slot[k] = i < SLOTS ? `var(--c${i})` : "var(--other)");
}
const colorOf = k => slot[k] || "var(--other)";
function matches(s){
  if (st.hidden.has(keyOf(s))) return false;
  if (!st.q) return true;
  return (s.title+" "+s.project+" "+(s.branch||"")+" "+s.source+" "+s.prompts.map(p=>p.text).join(" ")).toLowerCase().includes(st.q);
}
function toast(msg){ const t = $("#toast"); t.textContent = msg; t.classList.add("on"); clearTimeout(toast.h); toast.h = setTimeout(()=>t.classList.remove("on"), 1600); }
async function copy(text){ try { await navigator.clipboard.writeText(text); toast("コピーしました"); } catch(e){ toast("コピーできませんでした"); } }
const ICON = { auto:'<svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 4a8 8 0 000 16z" fill="currentColor"/></svg>',
  light:'<svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M4.6 4.6L6 6M18 18l1.4 1.4M2.5 12h2M19.5 12h2M4.6 19.4L6 18M18 6l1.4-1.4"/></svg>',
  dark:'<svg class="i" viewBox="0 0 24 24"><path d="M20 14.5A8 8 0 019.5 4a8 8 0 1010.5 10.5z"/></svg>' };
function applyTheme(){ const r = document.documentElement; st.theme === "auto" ? r.removeAttribute("data-theme") : r.setAttribute("data-theme", st.theme);
  $("#theme").innerHTML = ICON[st.theme]; $("#theme").setAttribute("aria-label", `テーマ: ${{auto:"自動",light:"ライト",dark:"ダーク"}[st.theme]}`); }

/* ── render ── */
function render(){
  document.documentElement.style.setProperty("--hh", st.hh+"px");
  assignColors();
  const ws = st.week.getTime()/1000, we = addDays(st.week,7).getTime()/1000;
  const todayKey = key(new Date()), end = addDays(st.week,6);
  $("#ry").textContent = `${st.week.getFullYear()} · WEEK ${isoWeek(st.week)}`;
  $("#rd").textContent = `${st.week.getMonth()+1}月${st.week.getDate()}日 — ${end.getMonth()+1}月${end.getDate()}日`;
  document.querySelectorAll("#colorBy button").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.colorBy));

  const inWeek = DATA.filter(s => s.end >= ws && s.start < we);
  const shown = inWeek.filter(matches);
  // legend（今週あるものを多い順に）
  const cnt = {}; inWeek.forEach(s => cnt[keyOf(s)] = (cnt[keyOf(s)]||0)+1);
  $("#legend").innerHTML = `<span class="lab">${{project:"プロジェクト",branch:"ブランチ",source:"ツール"}[st.colorBy]}</span>` +
    Object.keys(cnt).sort((a,b)=>cnt[b]-cnt[a]).map(k => `<button class="chip" style="--c:${colorOf(k)}" data-k="${esc(k)}" aria-pressed="${!st.hidden.has(k)}"><span class="dot"></span>${esc(k)}<span class="n">${cnt[k]}</span></button>`).join("") +
    (inWeek.length ? "" : '<span class="muted" style="font-size:12px">この週の記録はありません</span>') + `<span class="count">${shown.length} / ${inWeek.length} セッション</span>`;
  document.querySelectorAll(".chip").forEach(c => c.onclick = () => { const k = c.dataset.k; st.hidden.has(k) ? st.hidden.delete(k) : st.hidden.add(k); render(); });

  // day heads + tabs
  const dayHas = Array.from({length:7}, () => false);
  shown.forEach(s => s.segs.forEach(([a,b]) => { for (let d=0; d<7; d++){ const ds = addDays(st.week,d).getTime()/1000; if (Math.min(b, ds+86400) > Math.max(a, ds)) dayHas[d] = true; } }));
  let h = '<div></div>', tabs = "";
  for (let i=0;i<7;i++){ const d = addDays(st.week,i), t = key(d) === todayKey;
    h += `<div class="head${t?" today":""}"><span class="dn">${d.getDate()}</span><span class="dw">${DOW[d.getDay()]}</span>${i===0||d.getDate()===1?`<span class="dm">${d.getMonth()+1}月</span>`:""}</div>`;
    tabs += `<button aria-pressed="${i===st.mday}" data-i="${i}" class="${dayHas[i]?"has":""}"><small>${DOW[d.getDay()]}</small><b>${d.getDate()}</b></button>`; }
  $("#heads").innerHTML = h; $("#daytabs").innerHTML = tabs;
  document.querySelectorAll("#daytabs button").forEach(b => b.onclick = () => { st.mday = +b.dataset.i; st.animate = true; render(); });

  // grid
  const H = 24*st.hh;
  let g = `<div class="hours" style="height:${H}px">` + Array.from({length:23},(_,i)=>`<span style="top:${(i+1)*st.hh}px">${String(i+1).padStart(2,"0")}:00</span>`).join("") + "</div>";
  for (let i=0;i<7;i++){ const d = addDays(st.week,i);
    g += `<div class="day${d.getDay()%6===0?" we":""}${key(d)===todayKey?" today":""}${i!==st.mday?" mh":""}" style="height:${H}px" data-i="${i}">
      <div class="nightband" style="top:0;height:${6*st.hh}px"></div><div class="nightband" style="top:${22*st.hh}px;height:${2*st.hh}px"></div>
      ${st.hh >= 56 ? Array.from({length:24},(_,k)=>`<div class="half" style="top:${(k+.5)*st.hh}px"></div>`).join("") : ""}</div>`; }
  const grid = $("#grid"); grid.innerHTML = g; grid.classList.toggle("focus", !!st.sel);
  const days = [...grid.querySelectorAll(".day")];

  const perDay = Array.from({length:7}, () => []);
  shown.forEach(s => s.segs.forEach(([a,b,n]) => { for (let d=0; d<7; d++){
    const ds = addDays(st.week,d).getTime()/1000, de = addDays(st.week,d+1).getTime()/1000, x = Math.max(a,ds), y = Math.min(b,de);
    if (y > x) perDay[d].push({s, a:x, b:y, n, ds}); } }));
  perDay.forEach((blocks, d) => {
    blocks.sort((p,q) => p.a-q.a || q.b-p.b);
    // 重なるものだけ横に並べる（クラスタごとにレーン数を決める）
    let cluster = [], cEnd = -1;
    const flush = () => { const lanes = []; cluster.forEach(bk => { let i = lanes.findIndex(e => e <= bk.a); if (i<0){ i = lanes.length; lanes.push(0); } lanes[i] = bk.b; bk.lane = i; }); cluster.forEach(bk => bk.L = lanes.length); cluster = []; };
    blocks.forEach(bk => { if (bk.a >= cEnd) { flush(); cEnd = bk.b; } else cEnd = Math.max(cEnd, bk.b); cluster.push(bk); }); flush();
    blocks.forEach((bk, j) => {
      const top = (bk.a-bk.ds)/3600*st.hh, height = Math.max(4, (bk.b-bk.a)/3600*st.hh - 1);
      const dens = Math.min(1, bk.n / Math.max(1,(bk.b-bk.a)/60) / 2.5);
      const el = document.createElement("button");
      el.className = "blk" + (height < 10 ? " thin" : "") + (st.sel === bk.s.id ? " sel" : "");
      el.style.cssText = `top:${top}px;height:${height}px;left:calc(${bk.lane/bk.L*100}% + 3px);width:calc(${100/bk.L}% - 6px);--c:${colorOf(keyOf(bk.s))};--fill:${Math.round(14+30*dens)}%;${st.animate?`--delay:${d*28+Math.min(j,12)*8}ms`:"animation:none"}`;
      el.setAttribute("aria-label", `${bk.s.title}、${bk.s.project}、${md(bk.a)} ${hm(bk.a)}から${hm(bk.b)}`);
      if (height >= 18) el.innerHTML = `<span class="t">${esc(bk.s.title)}</span>` + (height >= 34 ? `<span class="m">${hm(bk.a)}–${hm(bk.b)} · ${esc(bk.s.project)}</span>` : "");
      el.onclick = e => { e.stopPropagation(); select(bk.s.id); };
      el.onmouseenter = e => tipOn(e, bk); el.onmousemove = tipMove; el.onmouseleave = tipOff;
      days[d].appendChild(el);
    });
  });
  const oldEmpty = $("#cal .empty"); if (oldEmpty) oldEmpty.remove();
  if (!shown.length) $("#heads").insertAdjacentHTML("afterend", `<div class="empty"><div><div class="k">空</div><p>${inWeek.length ? "条件に合うセッションはありません" : "この週の記録はありません"}</p></div></div>`);
  const nowS = Date.now()/1000;
  if (nowS >= ws && nowS < we){ const di = Math.floor((nowS-ws)/86400), ds = addDays(st.week,di).getTime()/1000, y = (nowS-ds)/3600*st.hh;
    days[di].insertAdjacentHTML("beforeend", `<div class="now" style="top:${y}px"></div><div class="nowlab" style="top:${y}px">${hm(nowS)}</div>`); }
  st.animate = false;
  st.sel ? detail(DATA.find(s => s.id === st.sel)) : summary();
}

/* ── tooltip ── */
function tipOn(e, bk){ const t = $("#tip"), s = bk.s;
  t.innerHTML = `<b>${esc(s.title)}</b><div class="r" style="--c:${colorOf(keyOf(s))}"><i></i>${esc(s.project)}${s.branch?` · ${esc(s.branch)}`:""}</div><div class="r">${md(bk.a)} ${hm(bk.a)}–${hm(bk.b)}（${dur((bk.b-bk.a)/60)}）</div><div class="r">${esc(s.source)} · 依頼 ${s.nPrompts} 件${s.cost ? ` · ${usd(s.cost)}` : ""}${s.credits ? ` · ${s.credits} クレジット` : ""}${s.subagents.length ? ` · サブエージェント ${s.subagents.length}` : ""}</div>`;
  t.classList.add("on"); tipMove(e); }
function tipMove(e){ const t = $("#tip"), w = t.offsetWidth, h = t.offsetHeight;
  let x = e.clientX + 14, y = e.clientY + 16; if (x + w > innerWidth - 12) x = e.clientX - w - 14; if (y + h > innerHeight - 12) y = e.clientY - h - 14;
  t.style.left = x+"px"; t.style.top = y+"px"; }
function tipOff(){ $("#tip").classList.remove("on"); }

/* ── panel: week summary ── */
function summary(){
  const w = WEEKS[key(st.week)], pw = WEEKS[key(addDays(st.week,-7))], P = $("#panel");
  const head = `<div class="eyebrow">Week ${isoWeek(st.week)} · Reflection</div><h2>今週のふりかえり</h2>`;
  if (!w){ P.innerHTML = `<div class="pane">${head}<p class="none" style="margin-top:14px">この週の記録はありません。<br>← → で週を移動できます。</p>${foot()}</div>`; return; }
  const d = pw ? w.active - pw.active : null;
  const longest = Math.max(0, ...w.focus.map(b=>b.min));
  const maxDay = Math.max(1, ...w.days.map(x=>x.active)), total = w.projects.reduce((t,[,v])=>t+v,0) || 1;
  const stat = (k, v, s) => `<div class="stat"><div class="k">${k}</div><div class="v">${v}</div>${s?`<div class="s">${s}</div>`:""}</div>`;
  const today = key(new Date());
  P.innerHTML = `<div class="pane">${head}
    <div class="hero"><div class="big num">${dur(w.active,true).replace(/(\d+)/g,'<span class="num">$1</span>').replace(/<small>(.*?)<\/small>/g,'<span class="u">$1</span>')}</div>
      <div class="cap">AIと一緒に手を動かしていた時間${d==null?"":`<span class="delta">先週より ${d>=0?"+":"−"}${dur(Math.abs(d))}</span>`}</div></div>
    <div class="stats">
      ${stat("集中ブロック（60分以上）", `${w.focus.length}<small>回</small>`, longest ? `最長 ${dur(longest)}` : "まとまった時間はなし")}
      ${stat("1日の切り替え", `${w.switchesAvg}<small>回</small>`, `最大 ${w.switchesMax} 回`)}
      ${stat("並列で動かした時間", dur(w.parallel,true), `最大 ${w.maxConc} 本同時`)}
      ${stat("待たせ時間（中央値）", secs(w.waitMedian).replace(/(分|秒)/g,"<small>$1</small>"), `90%点 ${secs(w.waitP90)}`)}
      ${stat("深夜（22〜6時）", dur(w.night,true), "")}
      ${stat("週末", dur(w.weekend,true), "")}
      ${stat("AIの延べ稼働", dur(w.ai,true), "並列ぶんも合計")}
      ${stat("セッション / 依頼", `${w.sessions}<small>/</small>${w.prompts}`, "")}
    </div>
    ${aiUsage(w, pw)}
    <h3>日ごとのリズム</h3>
    <div class="rhythm">${w.days.map((x,i)=>{ const dd = addDays(st.week,i), hgt = x.active/maxDay*84;
      return `<div class="col${key(dd)===today?" today":""}" title="${md(dd.getTime()/1000)} 作業 ${dur(x.active)}・深夜 ${dur(x.night)}・依頼 ${x.prompts}・切り替え ${x.switches}">
        <span class="v">${x.active ? (x.active>=60 ? (x.active/60).toFixed(1)+"h" : x.active+"m") : ""}</span>
        <div class="bar" style="height:${x.active?Math.max(2,hgt):0}px"><div class="nt" style="height:${x.active?x.night/x.active*100:0}%"></div></div>
        <span class="l">${DOW[dd.getDay()]}</span></div>`; }).join("")}</div>
    <div class="keyrow"><span><i></i>作業</span><span><i class="nt"></i>うち深夜</span></div>
    <h3>プロジェクトの配分</h3>
    <div class="stack" role="img" aria-label="プロジェクト別の配分">${w.projects.map(([k,v])=>`<span style="flex:${v};--c:${st.colorBy==="project"?colorOf(k):"var(--ink-3)"}" title="${esc(k)} ${dur(v)}"></span>`).join("")}</div>
    ${w.projects.slice(0,8).map(([k,v])=>`<div class="prow" style="--c:${st.colorBy==="project"?colorOf(k):"var(--ink-3)"}"><i></i><span class="nm">${esc(k)}</span><span class="tm">${dur(v)}</span><span class="pc">${Math.round(v*100/total)}%</span></div>`).join("")}
    <h3>集中ブロック</h3>
    ${w.focus.length ? w.focus.sort((a,b)=>b.min-a.min).slice(0,5).map(b=>`<div class="focusrow" style="--c:${st.colorBy==="project"?colorOf(b.project):"var(--ink-3)"}"><span class="when">${md(b.t)} ${hm(b.t)}</span><span class="track"><span style="width:${b.min/longest*100}%"></span></span><span class="len">${dur(b.min)}</span></div>`).join("") : '<p class="none">60分以上続いた作業はありませんでした。</p>'}
    <h3>こじれたかもしれない</h3>
    ${w.friction.length ? w.friction.map(f=>`<button class="card" data-id="${esc(f.id)}"><span class="ti">${esc(f.title)}</span><span class="me">${md(f.start)} · ${esc(f.project)} ${f.why.map(x=>`<span class="tagx">${esc(x)}</span>`).join("")}</span></button>`).join("") : '<p class="none">言い直しや中断が目立つセッションはありませんでした。</p>'}
    ${foot(`python3 kiroku.py --weekly ${key(st.week)}`)}</div>`;
  P.querySelectorAll(".card").forEach(c => c.onclick = () => select(c.dataset.id));
  bindCopy(P);
}
function aiUsage(w, pw){
  const u = w.usage; if (!u || (!u.tokens && !u.credits)) return "";
  const stat = (k, v, s) => `<div class="stat"><div class="k">${k}</div><div class="v">${v}</div>${s?`<div class="s">${s}</div>`:""}</div>`;
  const dc = pw && pw.usage ? u.cost - pw.usage.cost : null;
  const totalC = u.models.reduce((t,r)=>t+r[1],0) || 1, totalT = u.models.reduce((t,r)=>t+r[2],0) || 1, byCost = totalC > 0.0001;
  return `<h3>AIの使い方</h3>
    <div class="stats" style="margin-top:0">
      ${u.tokens ? stat("目安コスト（API換算）", usd(u.cost).replace("$","<small>$</small>"), dc==null ? "" : `先週より ${dc>=0?"+":"−"}${usd(Math.abs(dc))}`) : ""}
      ${u.tokens ? stat("トークン", `${tok(u.tokens)}`, `うち出力 ${tok(u.out)}`) : ""}
      ${u.tokens ? stat("キャッシュから読んだ割合", u.cacheHit==null ? "—" : `${Math.round(u.cacheHit*100)}<small>%</small>`, "入力のうち") : ""}
      ${stat("サブエージェント", `${u.subagents}<small>回</small>`, u.subagents ? `延べ ${dur(u.subMin)}` : "使っていません")}
      ${u.credits ? stat("Kiro クレジット", `${u.credits}`, "履歴に残った実績") : ""}
    </div>
    ${u.models.length ? `<div style="margin-top:16px" class="k muted">モデル別${byCost ? "（目安コスト）" : "（トークン）"}</div>
      <div class="mstack" style="margin-top:8px">${u.models.map((r,i)=>`<span style="flex:${byCost?r[1]:r[2]};--o:${shade(i)}" title="${esc(r[0])}"></span>`).join("")}</div>
      ${u.models.slice(0,6).map((r,i)=>`<div class="mrow"><span class="nm"><i style="--o:${shade(i)}"></i>${esc(r[0])}</span><span class="tm">${usd(r[1])}<small>${tok(r[2])}</small></span><span class="pc">${Math.round((byCost?r[1]/totalC:r[2]/totalT)*100)}%</span></div>`).join("")}` : ""}
    ${u.subTypes.length ? `<div style="margin-top:14px" class="chips">${u.subTypes.map(([k,v])=>`<span class="mono">${esc(k)}<b>${v}</b></span>`).join("")}</div>` : ""}
    ${u.heavy.length ? `<div style="margin-top:16px" class="k muted">重かったセッション</div><div style="margin-top:8px">${u.heavy.map(h=>`<button class="card" data-id="${esc(h.id)}"><span class="ti">${esc(h.title)}</span><span class="me">${md(h.start)} · ${esc(h.project)} · ${usd(h.cost)}${h.subagents?` · サブエージェント ${h.subagents}`:""}</span></button>`).join("")}</div>` : ""}
    ${u.tokens ? `<p class="note">目安コストは、履歴のトークン数に API の公開料金をかけた換算です。サブスクリプションの請求額とは別物です。料金表は <code>--prices</code> で変えられます。</p>` : ""}`;
}
function foot(cmd){
  return `<div class="foot">待たせ時間は、AI が返してから次の依頼を出すまで（30分以内）。切り替えは、続けて出した依頼のプロジェクトが変わった回数です。どれも履歴からの目安で、<b>自分のふりかえり用</b>。人と比べたり評価に使ったりするための数字ではありません。
    ${cmd ? `<div class="code"><code>${esc(cmd)}</code><button class="copy" data-copy="${esc(cmd)}">コピー</button></div>` : ""}
    <div style="margin-top:12px">生成 ${new Date(GENERATED*1000).toLocaleString("ja-JP")} · <button class="muted" id="openhelp" style="text-decoration:underline dotted">ショートカット</button></div></div>`;
}
function bindCopy(root){ root.querySelectorAll("[data-copy]").forEach(b => b.onclick = () => copy(b.dataset.copy)); const h = root.querySelector("#openhelp"); if (h) h.onclick = () => $("#keys").showModal(); }

/* ── panel: session detail ── */
function detail(s){
  if (!s){ st.sel = null; return summary(); }
  const active = s.segs.reduce((t,[a,b])=>t+(b-a),0)/60, waits = s.waits.map(x=>x[1]).sort((a,b)=>a-b);
  const med = waits.length ? waits[Math.floor(waits.length/2)] : null, maxT = Math.max(1, ...s.tools.map(t=>t[1]));
  const sameDay = new Date(s.start*1000).toDateString() === new Date(s.end*1000).toDateString();
  const allTok = [s.usage, ...s.subagents.map(a=>a.usage)].reduce((t,u)=>t + (u ? u.in+u.out+u.cw+u.cw1h+u.cr : 0), 0);
  const P = $("#panel");
  P.innerHTML = `<div class="pane" style="--c:${colorOf(keyOf(s))}">
    <button class="back" id="back"><svg class="i" viewBox="0 0 24 24"><path d="M15 6l-6 6 6 6"/></svg>週のふりかえりへ</button>
    <div class="eyebrow"><span class="dot"></span>${esc(s.source)}</div>
    <h2>${esc(s.title)}</h2>
    <div class="muted" style="font-variant-numeric:tabular-nums">${md(s.start)} ${hm(s.start)} 〜 ${sameDay ? "" : md(s.end)+" "}${hm(s.end)}</div>
    <div class="meta"><span>${esc(s.project)}</span>${s.branch?`<span>${esc(s.branch)}</span>`:""}</div>
    <div class="mini">
      <div><div class="k">実働</div><div class="v">${dur(active,true)}</div></div>
      <div><div class="k">依頼</div><div class="v">${s.nPrompts}<small>件</small></div></div>
      <div><div class="k">待たせ（中央値）</div><div class="v">${med==null?"—":secs(med).replace(/(分|秒)/g,"<small>$1</small>")}</div></div>
      <div><div class="k">言い直し・中断</div><div class="v">${s.corrections + s.interrupts}<small>回</small></div></div>
      ${s.source === "Claude Code" ? `<div><div class="k">目安コスト</div><div class="v">${usd(s.cost).replace("$","<small>$</small>")}</div></div>
      <div><div class="k">トークン</div><div class="v">${tok(allTok)}</div></div>` : s.credits ? `<div><div class="k">Kiro クレジット</div><div class="v">${s.credits}</div></div><div><div class="k">1依頼あたり</div><div class="v">${s.nPrompts ? (s.credits/s.nPrompts).toFixed(2) : "—"}<small>クレジット</small></div></div>` : ""}
    </div>
    ${s.models.length ? `<h3>使ったモデル</h3><div class="chips">${s.models.map(([m,n])=>`<span class="mono">${esc(m)}<b>${n}</b></span>`).join("")}</div>` : ""}
    <h3>依頼の流れ</h3>
    ${s.prompts.length ? `<ol class="tl">${s.prompts.slice(0,30).map(p=>`<li class="${/違う|ちがう|そうじゃな|やり直|戻して|元に戻|取り消|じゃなくて|wrong|revert|undo/i.test(p.text)?"fix":""}"><time>${p.t?hm(p.t):""}</time><p>${esc(p.text.length>220?p.text.slice(0,220)+"…":p.text)}</p></li>`).join("")}</ol>${s.prompts.length>30?`<p class="more">ほか ${s.nPrompts-30} 件</p>`:""}` : '<p class="none">依頼の記録はありません。</p>'}
    ${s.subagents.length ? `<h3>サブエージェント · ${s.subagents.length}</h3>${s.subagents.map(a=>{
        const span = Math.max(1, s.end - s.start), l = a.start ? Math.max(0,(a.start - s.start)/span*100) : 0, w = a.start && a.end ? Math.max(.8,(a.end - a.start)/span*100) : .8;
        const t = a.usage, tt = t.in + t.out + t.cw + t.cw1h + t.cr;
        return `<div class="sub"><div class="hd"><span class="ty">${esc(a.type)}</span><span class="ds">${esc(a.desc || "（説明なし）")}</span>${a.bg?'<span class="bg">バックグラウンド</span>':""}</div>
          <div class="lane"><span style="left:${l}%;width:${Math.min(w,100-l)}%"></span></div>
          <div class="ft">${a.start?`<span>${hm(a.start)}${a.end?"–"+hm(a.end):""}</span>`:""}${a.start&&a.end?`<span>${dur((a.end-a.start)/60)}</span>`:""}${tt?`<span>${tok(tt)} トークン</span>`:t.reportedTokens?`<span>${tok(t.reportedTokens)} トークン（報告値）</span>`:""}${t.cost?`<span>${usd(t.cost)}</span>`:""}${a.model?`<span>${esc(a.model)}</span>`:""}${a.tools?`<span>ツール ${a.tools} 回</span>`:""}</div></div>`; }).join("")}` : ""}
    <h3>使ったツール</h3>
    ${s.tools.length ? s.tools.map(([k,v])=>`<div class="trow"><span class="nm">${esc(k)}</span><span class="track"><span style="width:${v/maxT*100}%"></span></span><span class="n">${v}</span></div>`).join("") : '<p class="none">記録なし</p>'}
    <h3>変更したファイル · ${s.nFiles}</h3>
    ${s.files.length ? `<ul class="files">${s.files.map(f=>`<li title="${esc(f)}"><span>${esc(f)}</span></li>`).join("")}</ul>` : '<p class="none">なし</p>'}
    ${s.resume ? `<h3>続きから再開</h3><div class="code"><code>${esc(s.resume)}</code><button class="copy" data-copy="${esc(s.resume)}">コピー</button></div>` : ""}
  </div>`;
  $("#back").onclick = () => select(null);
  bindCopy(P);
}
function select(id){ st.sel = id; render(); if (innerWidth <= 820 && id) $("#panel").scrollIntoView({behavior:"smooth"}); else $("#panel").scrollTop = 0; }

/* ── events ── */
function go(n){ st.week = n == null ? mondayOf(new Date()) : addDays(st.week, 7*n); st.sel = null; st.animate = true;
  if (n == null) st.mday = (new Date().getDay()+6)%7; render(); scrollToWork(); }
$("#prev").onclick = () => go(-1); $("#next").onclick = () => go(1); $("#today").onclick = () => go(null);
$("#zin").onclick = () => zoom(12); $("#zout").onclick = () => zoom(-12);
function zoom(dv){ st.hh = Math.max(24, Math.min(120, st.hh+dv)); store.set("hh", st.hh); render(); }
document.querySelectorAll("#colorBy button").forEach(b => b.onclick = () => { st.colorBy = b.dataset.v; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); });
$("#q").oninput = e => { st.q = e.target.value.trim().toLowerCase(); render(); };
$("#theme").onclick = () => { st.theme = {auto:"light",light:"dark",dark:"auto"}[st.theme]; store.set("theme", st.theme); applyTheme(); };
$("#help").onclick = () => $("#keys").showModal();
$("#grid").onclick = e => { if (e.target.classList.contains("day") && st.sel) select(null); };
document.addEventListener("keydown", e => {
  if (e.target.tagName === "INPUT"){ if (e.key === "Escape") e.target.blur(); return; }
  if (e.metaKey || e.ctrlKey || e.altKey || $("#keys").open) return;
  const k = e.key;
  if (k === "ArrowLeft") go(-1); else if (k === "ArrowRight") go(1); else if (k === "t" || k === "T") go(null);
  else if (k === "/"){ e.preventDefault(); $("#q").focus(); } else if (k === "+" || k === "=") zoom(12); else if (k === "-") zoom(-12);
  else if (k === "Escape" && st.sel) select(null); else if (k === "?") $("#keys").showModal();
});
function scrollToWork(){ // その週でいちばん早く始まった時刻の少し前へ
  const ws = st.week.getTime()/1000, we = ws + 7*86400; let first = 24;
  DATA.forEach(s => s.segs.forEach(([a,b]) => { if (b > ws && a < we){ const d = new Date(Math.max(a,ws)*1000); first = Math.min(first, d.getHours()); } }));
  const y = Math.max(0, (first === 24 ? 8 : first) - 1) * st.hh, el = innerWidth <= 820 ? $("#grid") : $("#cal");
  el.style.scrollBehavior = "auto"; el.scrollTop = y; el.style.scrollBehavior = "";
}
// 今週に記録がなければ、いちばん新しい記録の週から開く
if (DATA.length && !WEEKS[key(st.week)]) { st.week = mondayOf(new Date(DATA[DATA.length-1].end*1000)); st.mday = (new Date(DATA[DATA.length-1].end*1000).getDay()+6)%7; }
const mq = matchMedia("(max-width:820px)"), ph = () => $("#q").placeholder = mq.matches ? "検索" : "タイトル・依頼文で探す";
mq.addEventListener("change", () => { ph(); render(); }); ph();
applyTheme(); render(); scrollToWork();
</script></body></html>
"""


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=os.path.expanduser(os.environ.get("CLAUDE_CONFIG_DIR", "~/.claude")) + "/projects")
    ap.add_argument("-o", "--out", default="kiroku.html")
    ap.add_argument("--gap", type=int, default=15, help="何分あいたら帯を分けるか（既定 15）")
    ap.add_argument("--sources", default="claude,kiro", help="読むもの（claude,kiro のカンマ区切り）")
    ap.add_argument("--no-open", action="store_true")
    ap.add_argument("--prices", metavar="JSON", help="料金表の上書き（README 参照）")
    ap.add_argument("--weekly", nargs="?", const="latest", metavar="YYYY-MM-DD",
                    help="その日を含む週のふりかえりを Markdown で書き出す（日付なしなら最新の週）")
    a = ap.parse_args()
    a.sources = {x.strip().lower() for x in a.sources.split(",")}
    if a.prices:
        for k, v in json.loads(Path(a.prices).read_text(encoding="utf-8")).items():
            if isinstance(v, dict):
                v = [v.get(n, 0) for n in ("input", "output", "cache_write", "cache_write_1h", "cache_read")]
            PRICES[k.lower()] = tuple(float(x) for x in v)
    data = build(a)
    if not data:
        sys.exit("履歴が 1 件も見つからなかったよ。--root や KIRO_HOME を確認してね")
    weeks = all_weeks(data)
    if a.weekly:
        key = max(weeks) if a.weekly == "latest" else monday_of(
            datetime.strptime(a.weekly, "%Y-%m-%d").timestamp()).strftime("%Y-%m-%d")
        if key not in weeks:
            sys.exit(f"{key} の週には履歴がないよ")
        prev = weeks.get((datetime.strptime(key, "%Y-%m-%d") - timedelta(days=7)).strftime("%Y-%m-%d"))
        out = Path(f"kiroku-week-{key}.md")
        out.write_text(weekly_markdown(weeks[key], prev), encoding="utf-8")
        print(f"週のふりかえり → {out}")
        return
    dump = lambda v: json.dumps(v, ensure_ascii=False).replace("</", "<\\/")
    public = [{k: v for k, v in d.items() if not k.startswith("_")} for d in data]
    html = (HTML.replace("__DATA__", dump(public)).replace("__WEEKS__", dump(weeks))
            .replace("__GEN__", str(datetime.now(timezone.utc).timestamp())))
    Path(a.out).write_text(html, encoding="utf-8")
    print(f"{len(data)} セッション → {a.out}")
    if not a.no_open:
        webbrowser.open(Path(a.out).resolve().as_uri())


if __name__ == "__main__":
    main()
