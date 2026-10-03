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
import sys
import webbrowser
from datetime import datetime, timezone
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


class Session:
    EDIT_TOOLS = {"Edit", "Write", "MultiEdit", "NotebookEdit",  # Claude Code
                  "fsWrite", "fsAppend", "strReplace", "fs_write", "write", "editCode"}  # Kiro

    def __init__(self, source, sid):
        self.source, self.id = source, sid
        self.project = self.branch = self.title = self.resume = None
        self.prompts, self.times, self.tools, self.files = [], [], {}, set()

    def tick(self, ts):
        if ts is not None:
            self.times.append(ts)

    def prompt(self, ts, text):
        if text and not is_noise(text):
            self.prompts.append({"t": ts, "text": text.strip()[:400]})

    def tool(self, name, args=None):
        name = name or "?"
        self.tools[name] = self.tools.get(name, 0) + 1
        args = args if isinstance(args, dict) else {}
        fp = args.get("file_path") or args.get("path") or args.get("targetFile")
        if isinstance(fp, str) and name in self.EDIT_TOOLS:
            self.files.add(fp)


# ── Claude Code ─────────────────────────────────────────────────────

def load_claude(root):
    for path in sorted(Path(root).glob("*/*.jsonl")):
        s = Session("Claude Code", path.stem)
        summaries = []
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
            s.tick(ts)
            s.project = s.project or e.get("cwd")
            s.branch = s.branch or e.get("gitBranch")
            msg = e.get("message") or {}
            if typ == "user" and not e.get("isMeta") and not e.get("isSidechain"):
                s.prompt(ts, text_of(msg.get("content")))
            elif typ == "assistant" and isinstance(msg.get("content"), list):
                for b in msg["content"]:
                    if isinstance(b, dict) and b.get("type") == "tool_use":
                        s.tool(b.get("name"), b.get("input"))
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
        s.tick(parse_ts(meta.get("createdAt")))
        msgs = meta_path.parent / "messages.jsonl"
        if msgs.exists():
            for e in read_jsonl(msgs):
                ts = parse_ts(e.get("timestamp"))
                s.tick(ts)
                p = e.get("payload") or {}
                if p.get("type") == "user":
                    s.prompt(ts, text_of(p.get("content")))
                elif p.get("type") == "tool_call":
                    s.tool(p.get("toolName"), p.get("args"))
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
        for t in turns:
            s.tick(parse_ts((t or {}).get("end_timestamp")))
        log = meta_path.with_suffix(".jsonl")
        if log.exists():
            for e in read_jsonl(log):
                data = e.get("data") or {}
                ts = parse_ts(e.get("timestamp") or (data.get("meta") or {}).get("timestamp"))
                s.tick(ts)
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
        })
    out.sort(key=lambda x: x["start"])
    return out


HTML = r"""<!doctype html>
<html lang="ja"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>kiroku</title>
<style>
:root{--bg:#fafafa;--panel:#fff;--fg:#1f2328;--mute:#6b7280;--line:#e5e7eb;--today:#eef4ff;--now:#e11d48}
@media (prefers-color-scheme:dark){:root{--bg:#111318;--panel:#1a1d24;--fg:#e6e8eb;--mute:#9aa1ab;--line:#2a2f38;--today:#1b2433}}
*{box-sizing:border-box}html,body{margin:0;height:100%}
body{background:var(--bg);color:var(--fg);font:13px/1.5 system-ui,-apple-system,"Hiragino Sans","Noto Sans JP",sans-serif;display:flex;flex-direction:column}
header{display:flex;gap:8px;align-items:center;padding:10px 14px;border-bottom:1px solid var(--line);background:var(--panel);flex-wrap:wrap}
button,select,input{font:inherit;color:inherit;background:var(--panel);border:1px solid var(--line);border-radius:6px;padding:4px 10px;cursor:pointer}
input{cursor:text;min-width:180px}
#range{font-weight:600;font-size:15px;margin:0 6px}.mute{color:var(--mute)}
#legend{display:flex;gap:10px;flex-wrap:wrap;padding:6px 14px;border-bottom:1px solid var(--line)}
#legend span{display:inline-flex;align-items:center;gap:4px;cursor:pointer;user-select:none}#legend span.off{opacity:.35}
#legend i{width:10px;height:10px;border-radius:2px;display:inline-block}
main{flex:1;display:flex;min-height:0}
#cal{flex:1;overflow:auto;position:relative}
#heads{display:grid;grid-template-columns:48px repeat(7,1fr);position:sticky;top:0;background:var(--panel);z-index:3;border-bottom:1px solid var(--line)}
#heads div{padding:6px;text-align:center;color:var(--mute)}#heads .today{color:#2563eb;font-weight:700}
#grid{display:grid;grid-template-columns:48px repeat(7,1fr);position:relative}
.hours div{height:var(--hh);border-top:1px solid var(--line);font-size:11px;color:var(--mute);text-align:right;padding-right:4px;transform:translateY(-8px);border:0}
.day{position:relative;border-left:1px solid var(--line);background-image:linear-gradient(var(--line) 1px,transparent 1px);background-size:100% var(--hh)}
.day.today{background-color:var(--today)}
.blk{position:absolute;border-radius:3px;border:1px solid rgba(0,0,0,.25);overflow:hidden;font-size:11px;padding:0 3px;cursor:pointer;color:#111;white-space:nowrap;text-overflow:ellipsis}
.blk:hover,.blk.sel{outline:2px solid var(--fg);z-index:2}
.now{position:absolute;left:0;right:0;height:2px;background:var(--now);z-index:2}
aside{width:420px;max-width:45vw;border-left:1px solid var(--line);background:var(--panel);overflow:auto;padding:14px}
aside h2{font-size:15px;margin:0 0 6px;overflow-wrap:anywhere}aside h3{font-size:12px;color:var(--mute);margin:14px 0 4px;text-transform:uppercase;letter-spacing:.04em}
aside ol,aside ul{padding-left:18px;margin:0}aside li{margin:3px 0;overflow-wrap:anywhere}
.chip{display:inline-block;border:1px solid var(--line);border-radius:10px;padding:0 8px;margin:2px 4px 2px 0;font-size:12px}
code{font-size:12px;word-break:break-all}
@media (max-width:800px){main{flex-direction:column}aside{width:auto;max-width:none;border-left:0;border-top:1px solid var(--line);max-height:45vh}}
</style></head><body>
<header>
  <b style="font-size:15px;margin-right:6px">kiroku</b>
  <button id="today">今週</button><button id="prev">◀</button><button id="next">▶</button>
  <span id="range"></span><span class="mute" id="count"></span>
  <span style="flex:1"></span>
  <label class="mute">色分け <select id="colorBy"><option value="project">プロジェクト</option><option value="branch">ブランチ</option><option value="source">ツール</option></select></label>
  <button id="zout">−</button><button id="zin">＋</button>
  <input id="q" placeholder="検索（タイトル・依頼文）">
</header>
<div id="legend"></div>
<main><div id="cal"><div id="heads"></div><div id="grid"></div></div>
<aside id="detail"><p class="mute">帯をクリックすると、そのセッションの中身がここに出ます。</p><p class="mute" id="gen"></p></aside></main>
<script>
const DATA = __DATA__;
const GENERATED = __GEN__;
const PALETTE = ["#a78bfa","#f0b35a","#7dd3a8","#7aa7f0","#f28b8b","#c49a6c","#5fc4d0","#e58fd6","#b5c26b","#9ca3af","#f6a2b5","#86b0a0"];
const DOW = ["日","月","火","水","木","金","土"];
const $ = s => document.querySelector(s);
let hh = 40, weekStart = mondayOf(new Date()), colorBy = "project", hidden = new Set(), selected = null, q = "";
function mondayOf(d){ d = new Date(d); d.setHours(0,0,0,0); d.setDate(d.getDate() - ((d.getDay()+6)%7)); return d; }
function fmt(t,o){ return new Date(t*1000).toLocaleString("ja-JP",o); }
function hm(t){ return fmt(t,{hour:"2-digit",minute:"2-digit"}); }
function dur(s){ s = Math.round(s/60); return s < 60 ? s+"分" : Math.floor(s/60)+"時間"+(s%60 ? (s%60)+"分" : ""); }
function esc(s){ return String(s ?? "").replace(/[&<>"]/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c])); }
const keyOf = s => colorBy === "project" ? s.project : colorBy === "source" ? s.source : (s.project + " @ " + (s.branch || "-"));
let colors = {};
function assignColors(){
  const counts = {}; DATA.forEach(s => counts[keyOf(s)] = (counts[keyOf(s)]||0) + 1);
  colors = {}; Object.keys(counts).sort((a,b)=>counts[b]-counts[a]).forEach((k,i)=>colors[k]=PALETTE[i%PALETTE.length]);
  return counts;
}
function matches(s){
  if (hidden.has(keyOf(s))) return false;
  if (!q) return true;
  const hay = (s.title + " " + s.source + " " + s.project + " " + (s.branch||"") + " " + s.prompts.map(p=>p.text).join(" ")).toLowerCase();
  return hay.includes(q);
}
function render(){
  document.documentElement.style.setProperty("--hh", hh+"px");
  const ws = weekStart.getTime()/1000, we = ws + 7*86400, todayIdx = Math.floor((mondayOf(new Date()).getTime()/1000 === ws) ? ((new Date().getDay()+6)%7) : -1);
  const end = new Date((we-1)*1000);
  $("#range").textContent = `${weekStart.getFullYear()}年${weekStart.getMonth()+1}/${weekStart.getDate()}〜${end.getMonth()+1}/${end.getDate()}`;
  const counts = assignColors();
  // legend
  const weekKeys = {}; DATA.filter(s=>s.end>=ws && s.start<we).forEach(s=>weekKeys[keyOf(s)]=(weekKeys[keyOf(s)]||0)+1);
  $("#legend").innerHTML = Object.keys(weekKeys).sort((a,b)=>weekKeys[b]-weekKeys[a]).map(k =>
    `<span data-k="${esc(k)}" class="${hidden.has(k)?"off":""}"><i style="background:${colors[k]}"></i>${esc(k)} <b class="mute">${weekKeys[k]}</b></span>`).join("") || '<span class="mute">この週のセッションはありません</span>';
  document.querySelectorAll("#legend span[data-k]").forEach(el => el.onclick = () => { const k = el.dataset.k; hidden.has(k) ? hidden.delete(k) : hidden.add(k); render(); });
  // heads
  let h = "<div></div>";
  for (let i=0;i<7;i++){ const d = new Date(weekStart); d.setDate(d.getDate()+i);
    h += `<div class="${i===todayIdx?"today":""}">${DOW[d.getDay()]} ${d.getMonth()+1}/${d.getDate()}</div>`; }
  $("#heads").innerHTML = h;
  // grid
  let g = '<div class="hours">' + Array.from({length:24},(_,i)=>`<div>${i?i+":00":""}</div>`).join("") + "</div>";
  for (let i=0;i<7;i++) g += `<div class="day ${i===todayIdx?"today":""}" style="height:${24*hh}px" data-i="${i}"></div>`;
  $("#grid").innerHTML = g;
  const days = [...document.querySelectorAll(".day")];
  // collect blocks per day, split at midnight
  const perDay = Array.from({length:7},()=>[]);
  let n = 0;
  DATA.forEach(s => {
    if (s.end < ws || s.start >= we || !matches(s)) return; n++;
    s.segs.forEach(([a,b,cnt]) => {
      for (let d=0; d<7; d++){
        const ds = ws + d*86400, de = ds + 86400;
        // DST を気にするなら Date で日境界を作るべきだが、日本時間なら問題なし
        const x = Math.max(a, ds), y = Math.min(b, de);
        if (y > x) perDay[d].push({s, a:x, b:y, cnt});
      }
    });
  });
  $("#count").textContent = `${n} セッション / 全 ${DATA.length}`;
  perDay.forEach((blocks, d) => {
    const ds = ws + d*86400;
    blocks.sort((p,r)=>p.a-r.a);
    const lanes = [];  // 重なりはレーンに分けて横に並べる
    blocks.forEach(bk => { let i = lanes.findIndex(e => e <= bk.a); if (i<0){ i = lanes.length; lanes.push(0);} lanes[i] = bk.b + 120; bk.lane = i; });
    const L = Math.max(1, lanes.length);
    blocks.forEach(bk => {
      const top = (bk.a-ds)/3600*hh, height = Math.max(3, (bk.b-bk.a)/3600*hh);
      const el = document.createElement("div");
      el.className = "blk" + (selected===bk.s.id ? " sel" : "");
      const dens = Math.min(1, bk.cnt / Math.max(1,(bk.b-bk.a)/60) / 3); // 1 分あたりの発言量で濃さ
      el.style.cssText = `top:${top}px;height:${height}px;left:calc(${bk.lane/L*100}% + 1px);width:calc(${100/L}% - 2px);background:${colors[keyOf(bk.s)]};opacity:${0.45+0.55*dens}`;
      el.title = `${bk.s.title}\n[${bk.s.source}] ${bk.s.project}${bk.s.branch?" @ "+bk.s.branch:""}\n${hm(bk.a)}–${hm(bk.b)}`;
      if (height > 14) el.textContent = bk.s.title;
      el.onclick = () => { selected = bk.s.id; showDetail(bk.s); render(); };
      days[d].appendChild(el);
    });
  });
  if (todayIdx >= 0){ const now = Date.now()/1000, ds = ws + todayIdx*86400; const ln = document.createElement("div"); ln.className="now"; ln.style.top = ((now-ds)/3600*hh)+"px"; days[todayIdx].appendChild(ln); }
}
function showDetail(s){
  const active = s.segs.reduce((t,[a,b])=>t+(b-a),0);
  $("#detail").innerHTML = `
    <h2>${esc(s.title)}</h2>
    <div class="mute">${fmt(s.start,{month:"numeric",day:"numeric",weekday:"short",hour:"2-digit",minute:"2-digit"})} 〜 ${fmt(s.end,{month:"numeric",day:"numeric",hour:"2-digit",minute:"2-digit"})}</div>
    <div style="margin-top:6px"><span class="chip" style="background:${colors[keyOf(s)]};color:#111">${esc(s.project)}</span><span class="chip">${esc(s.source)}</span>${s.branch?`<span class="chip">${esc(s.branch)}</span>`:""}
      <span class="chip">実働 ${dur(active)}</span><span class="chip">依頼 ${s.nPrompts} 件</span><span class="chip">イベント ${s.events}</span></div>
    <h3>依頼の流れ</h3><ol>${s.prompts.map(p=>`<li><span class="mute">${hm(p.t)}</span> ${esc(p.text.length>200?p.text.slice(0,200)+"…":p.text)}</li>`).join("") || '<li class="mute">なし</li>'}</ol>
    <h3>使ったツール</h3><div>${s.tools.map(([k,v])=>`<span class="chip">${esc(k)} ${v}</span>`).join("") || '<span class="mute">なし</span>'}</div>
    <h3>変更したファイル ${s.nFiles} 件</h3><ul>${s.files.map(f=>`<li><code>${esc(f)}</code></li>`).join("")}</ul>
    ${s.resume?`<h3>再開</h3><code>${esc(s.resume)}</code>`:""}`;
}
$("#prev").onclick = () => { weekStart.setDate(weekStart.getDate()-7); render(); };
$("#next").onclick = () => { weekStart.setDate(weekStart.getDate()+7); render(); };
$("#today").onclick = () => { weekStart = mondayOf(new Date()); render(); };
$("#zin").onclick = () => { hh = Math.min(160, hh+10); render(); };
$("#zout").onclick = () => { hh = Math.max(16, hh-10); render(); };
$("#colorBy").onchange = e => { colorBy = e.target.value; hidden.clear(); render(); };
$("#q").oninput = e => { q = e.target.value.trim().toLowerCase(); render(); };
document.addEventListener("keydown", e => { if (e.target.tagName==="INPUT") return; if (e.key==="ArrowLeft") $("#prev").click(); if (e.key==="ArrowRight") $("#next").click(); if (e.key==="t") $("#today").click(); });
$("#gen").textContent = "生成: " + new Date(GENERATED*1000).toLocaleString("ja-JP");
// 最新セッションのある週から開く
if (DATA.length && !DATA.some(s => s.end >= weekStart.getTime()/1000)) weekStart = mondayOf(new Date(DATA[DATA.length-1].end*1000));
render();
setTimeout(()=>{ $("#cal").scrollTop = 8*hh; }, 0);
</script></body></html>
"""


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=os.path.expanduser(os.environ.get("CLAUDE_CONFIG_DIR", "~/.claude")) + "/projects")
    ap.add_argument("-o", "--out", default="kiroku.html")
    ap.add_argument("--gap", type=int, default=15, help="何分あいたら帯を分けるか（既定 15）")
    ap.add_argument("--sources", default="claude,kiro", help="読むもの（claude,kiro のカンマ区切り）")
    ap.add_argument("--no-open", action="store_true")
    a = ap.parse_args()
    a.sources = {x.strip().lower() for x in a.sources.split(",")}
    data = build(a)
    if not data:
        sys.exit("履歴が 1 件も見つからなかったよ。--root や KIRO_HOME を確認してね")
    payload = json.dumps(data, ensure_ascii=False).replace("</", "<\\/")
    html = HTML.replace("__DATA__", payload).replace("__GEN__", str(datetime.now(timezone.utc).timestamp()))
    Path(a.out).write_text(html, encoding="utf-8")
    print(f"{len(data)} セッション → {a.out}")
    if not a.no_open:
        webbrowser.open(Path(a.out).resolve().as_uri())


if __name__ == "__main__":
    main()
