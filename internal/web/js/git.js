// 印とリンク、詳細パネルのコミット・push・PR
/* git のコミットの印（コミットのノード: 線の上の丸） */
const GIT_ICON = `<svg class="gi" viewBox="0 0 16 16" aria-hidden="true"><path d="M1 8h4.2M10.8 8H15"/><circle cx="8" cy="8" r="2.8"/></svg>`;
/* 色だけに頼らず、形でも見分けられるようにする小さな印（16×16、線は currentColor）。意味は必ず文字（凡例・ツールチップ）でも出す */
const ICON_PATH = {
  commit: '<path d="M1 8h4.2M10.8 8H15"/><circle cx="8" cy="8" r="2.8"/>',
  pr: '<circle cx="4" cy="3.5" r="1.6"/><circle cx="4" cy="12.5" r="1.6"/><circle cx="12" cy="12.5" r="1.6"/><path d="M4 5.1v5.8M12 10.9V7a2.5 2.5 0 0 0-2.5-2.5H7M8.5 3 7 4.5 8.5 6"/>',
  limit: '<path d="M4 2h8M4 14h8M5 2c0 3.2 3 4.3 3 6s-3 2.8-3 6M11 2c0 3.2-3 4.3-3 6s3 2.8 3 6"/>',
  int: '<circle cx="8" cy="8" r="6"/><path d="M6.4 5.6v4.8M9.6 5.6v4.8"/>',
  agent: '<path d="M3 2.5v4a3 3 0 0 0 3 3h7M10 6.5l3 3-3 3"/>',
  push: '<path d="M8 12V2.8M4.2 6.6 8 2.8l3.8 3.8M3.5 14h9"/>',
  compact: '<path d="M8 1.5v4.3M5.8 3.8 8 6l2.2-2.2M8 14.5v-4.3M5.8 12.2 8 10l2.2 2.2M2.5 8h11"/>', /* 上下から寄せる（会話を要約して縮めた） */
  note: '<path d="M4.2 10.8V7a3.8 3.8 0 0 1 7.6 0v3.8l1.2 1.4H3zM6.6 14a1.5 1.5 0 0 0 2.8 0"/>',
  command: '<path d="M10.5 2.5 5.5 13.5"/>',
  shell: '<path d="M3 4.5 6.5 8 3 11.5M8 12h5"/>',
  flag: '<path d="M8 1.8 14.6 13.4H1.4z"/><path d="M8 6.2v3.4M8 11.4v.1"/>',
  reply: '<path d="M6.2 3.5 2.5 7.2l3.7 3.7M3 7.2h6.2A3.8 3.8 0 0 1 13 11v1.5"/>',
  caret: '<path d="M6.5 3.5 11 8l-4.5 4.5"/>', /* 開け閉めの印（開くと 90 度回る） */
};
const ico = (k, cls = "") => `<svg class="ic${cls ? " " + cls : ""}" viewBox="0 0 16 16" aria-hidden="true">${ICON_PATH[k] || ""}</svg>`;
/* エージェントの目印：ロゴは使わず、頭文字のバッジにする */
const AG_MARK = {"Claude Code": "CC", "Kiro IDE": "KI", "Kiro IDE (legacy)": "KI", "Kiro CLI": "KC", "Kiro CLI (SQLite)": "KC", "Kiro Crew": "KW", "Amazon Q": "Q", "Codex": "CX"};
const agMark = name => `<i class="agm" style="--ag:${agColor(name)}" aria-hidden="true">${esc(AG_MARK[name] || String(name).slice(0, 2).toUpperCase())}</i>`;
/* セッションのカード：セッション名で始まるカードは、どこでもこの形（エージェントの色のグラデーションとエージェントの頭文字）。
   me はタイトルの下の行（HTML。呼ぶ側でエスケープする）、v は右に出す値（文字。ここでエスケープする） */
const sesCardH = (s, me, v) => `<button class="card ag" data-id="${esc(s.id)}" style="--ag:${agColor(s.source)}"><span class="ti">${agMark(s.source)}<span>${esc(s.title)}</span></span><span class="me">${me}${v != null ? `<b>${esc(v)}</b>` : ""}</span></button>`;
const sesById = (id, title) => ({id, title, source: (DATA.find(x => x.id === id) || {}).source}); // 集計（Go）が id と名前だけを持つセッション
/* リンク：外のページ（GitHub など）は新しいタブで開く */
function ext(url, label, cls){ return url && /^https?:\/\//i.test(url) ? `<a class="xl${cls ? " "+cls : ""}" href="${esc(url)}" target="_blank" rel="noopener noreferrer">${label}</a>` : label; }
function fileHref(path){ // HTML で見るときの履歴ファイルの file:// の URL（Windows のパスにも対応）
  const p = path.replace(/\\/g, "/");
  return "file://" + (/^[A-Za-z]:/.test(p) ? "/" : "") + p.split("/").map(encodeURIComponent).join("/").replace(/%3A/g, ":");
}
/* ── drawer: git のコミット ── */
function commitsOf(s){ // セッションが作ったコミットと、同じプロジェクトでセッションの間（終わってから 10 分まで）のコミット
  return (META.git || []).filter(c => c.session === s.id || (!c.session && c.project === s.project && c.t >= s.start - 60 && c.t <= s.end + 600));
}
function fileRows(files, n){
  const num = v => v < 0 ? "bin" : v;
  return `<ul class="gfiles">${files.map(f => `<li title="${esc(f.path)}"><span>${ext(f.url, esc(f.path))}</span><b><i>+${num(f.added)}</i> −${num(f.removed)}</b></li>`).join("")}</ul>${n > files.length ? `<p class="more">${`${plural(n - files.length, "more file")}`}</p>` : ""}`;
}
function fileLink(s, abs){ // セッションの間のコミットのうち、最後にそのファイルを変えたコミットでのリンク
  const p = abs.replace(/\\/g, "/");
  let url = "";
  commitsOf(s).forEach(c => (c.files || []).forEach(f => { if (f.url && (p === f.path || p.endsWith("/" + f.path))) url = f.url; }));
  return url;
}
function sessionCommits(s){
  const cs = commitsOf(s); if (!cs.length) return "";
  return `<h3>Commits during this session · ${cs.length}</h3>${cs.map(c => `<button class="gc-row" data-git="${esc(c.hash)}"><time>${hm(c.t)}</time><span><i class="gtag${c.ai ? " ai" : ""}">${GIT_ICON}${esc(c.hash.slice(0,7))}</i> ${esc(c.subject)}</span><b>+${c.added} −${c.removed}</b></button>`).join("")}`;
}
function commitDetail(c){
  const ses = DATA.find(x => x.id === c.session) || DATA.find(x => x.project === c.project && c.t >= x.start - 60 && c.t <= x.end + 600);
  const P = $("#panel");
  P.innerHTML = `<div style="--c:${st.colorBy === "project" ? colorOf(c.project) : "var(--ink-3)"}">
    <div class="eyebrow"><span class="dot"></span>GIT · ${c.ai ? "Run by AI" : "By hand (or another tool)"}</div>
    <h2>${esc(c.subject)}</h2>
    <div class="muted" style="font-variant-numeric:tabular-nums">${md(c.t)} ${hm(c.t)}</div>
    <div class="meta"><span>${esc(c.project)}</span>${c.branch ? `<span>${esc(c.branch)}</span>` : ""}${ext(c.url, `<span class="mono">${esc(c.hash.slice(0,7))}</span>`)}</div>
    <div class="dcols"><div class="dcol">
    <div class="mini">
      <div><div class="k">Files changed</div><div class="v">${c.nFiles}</div></div>
      <div><div class="k">Added</div><div class="v"><small>+</small>${c.added}<small> lines</small></div></div>
      <div><div class="k">Removed</div><div class="v"><small>−</small>${c.removed}<small> lines</small></div></div>
    </div>
    ${c.body ? `<h3>Message body</h3><p class="gbody">${esc(c.body)}</p>` : ""}
    ${ses ? `<h3>${c.session ? "Session that made this commit" : "Session running at this time"}</h3>${sesCardH(ses, `${md(ses.start)} ${hm(ses.start)}–${hm(ses.end)} · ${esc(ses.source)}`)}` : ""}
    ${pushedIn(c).map(p => `<h3>Pushed</h3><button class="gc-row" data-push="${esc(pushKey(p))}"><time>${md(p.t)} ${hm(p.t)}</time><span><i class="gtag">${ico("push")}${esc(p.ref)}</i>${p.commits ? ` ${plural(p.commits, "commit")}` : ""}</span><b></b></button>`).join("")}
    ${c.url ? `<p style="margin-top:14px">${ext(c.url, "Open this commit on the remote ↗", "pill")}</p>` : ""}
    </div><div class="dcol">
    <h3>Files changed · ${c.nFiles}</h3>
    ${c.files.length ? fileRows(c.files, c.nFiles) : `<p class="none">None</p>`}
    <h3>Repository</h3><div class="code"><code>${esc(`git -C ${c.repo} show ${c.hash}`)}</code><button class="copy" data-copy="${esc(`git -C ${c.repo} show ${c.hash}`)}">Copy</button></div>
  </div></div></div>`;
  P.querySelectorAll(".card").forEach(b => b.onclick = () => select(b.dataset.id));
  bindGitEvents(P); bindCopy(P);
}
// pushedIn は、そのコミットを送った push（いちばん早いもの）。リモートのブランチごとに 1 つ
function pushedIn(c){ const by = new Map(); (META.push || []).filter(p => p.repo === c.repo && (p.hashes || []).includes(c.hash)).sort((a, b) => a.t - b.t).forEach(p => { if (!by.has(p.ref)) by.set(p.ref, p); }); return [...by.values()]; }
/* push と PR の詳細。カレンダーの右端の印と、プロンプトの流れから開く */
const pushKey = p => `${p.ref}@${p.hash}@${p.t}`; // 同じコミットを別のブランチや別の時刻に push することもあるので、3 つで見分ける
const prKey = (s, r) => `${s.id}#${(s.prAt || []).indexOf(r)}`;
function findPush(k){ return (META.push || []).find(p => pushKey(p) === k); }
function findPR(k){ const i = k.lastIndexOf("#"), s = DATA.find(x => x.id === k.slice(0, i)), r = s && (s.prAt || [])[+k.slice(i + 1)]; return r ? {s, r} : null; }
function bindGitEvents(root){ // コミット・push・PR を開くボタン（カレンダーの印、プロンプトの流れ、詳細の中の一覧）
  root.querySelectorAll("[data-git]").forEach(b => b.onclick = e => { e.stopPropagation(); select("git:" + b.dataset.git); });
  root.querySelectorAll("[data-push]").forEach(b => b.onclick = e => { e.stopPropagation(); select("push:" + b.dataset.push); });
  root.querySelectorAll("[data-pr]").forEach(b => b.onclick = e => { e.stopPropagation(); select("pr:" + b.dataset.pr); }); }
const sesCard = (s, label) => `${label ? `<h3>${label}</h3>` : ""}${sesCardH(s, `${md(s.start)} ${hm(s.start)}–${hm(s.end)} · ${esc(s.source)}`)}`;
const gitRow = c => `<button class="gc-row" data-git="${esc(c.hash)}"><time>${md(c.t)} ${hm(c.t)}</time><span><i class="gtag${c.ai ? " ai" : ""}">${GIT_ICON}${esc(c.hash.slice(0,7))}</i> ${esc(c.subject)}</span><b>+${c.added} −${c.removed}</b></button>`;
function pushDetail(p){
  const P = $("#panel"), byHash = new Map((META.git || []).map(c => [c.hash, c]));
  const hs = p.hashes || [], known = hs.map(h => byHash.get(h)).filter(Boolean), unknown = hs.filter(h => !byHash.has(h));
  const ses = DATA.filter(x => x.project === p.project && p.t >= x.start - 60 && p.t <= x.end + 600);
  const cmd = p.prev ? `git -C ${p.repo} log --oneline ${p.prev.slice(0,12)}..${p.hash.slice(0,12)}` : `git -C ${p.repo} show ${p.hash.slice(0,12)}`;
  P.innerHTML = `<div style="--c:${st.colorBy === "project" ? colorOf(p.project) : "var(--ink-3)"}">
    <div class="eyebrow"><span class="dot"></span>GIT · Push</div>
    <h2>${esc(`Pushed to ${p.ref}`)}</h2>
    <div class="muted" style="font-variant-numeric:tabular-nums">${md(p.t)} ${hm(p.t)}</div>
    <div class="meta"><span>${esc(p.project)}</span><span class="mono">${esc(p.ref)}</span>${ext(p.url, `<span class="mono">${esc(p.hash.slice(0,7))}</span>`)}</div>
    <div class="dcols"><div class="dcol">
    <div class="mini">
      <div><div class="k">Commits sent</div><div class="v">${p.prev ? p.commits : "—"}</div></div>
      <div><div class="k">Latest commit</div><div class="v mono" style="font-size:var(--fs-md)">${esc(p.hash.slice(0,7))}</div></div>
    </div>
    <p class="muted" style="font-size:var(--fs-xs)">${p.prev ? "Read from this computer's git reflog: what moved this remote-tracking branch with a push." : "The first push kiroku can see for this branch, so how many commits it sent is unknown."}</p>
    ${ses.length ? ses.map((s, i) => sesCard(s, i ? "" : (ses.length > 1 ? "Sessions running at this time" : "Session running at this time"))).join("") : ""}
    ${p.url ? `<p style="margin-top:14px">${ext(p.url, "Open the latest commit on the remote ↗", "pill")}</p>` : ""}
    </div><div class="dcol">
    <h3>${`Commits sent · ${p.prev ? p.commits : "?"}`}</h3>
    ${known.length ? known.map(gitRow).join("") : ""}
    ${unknown.length ? `<p class="muted" style="font-size:var(--fs-xs)">${plural(unknown.length, "commit")} not in kiroku's view (made before the history kiroku read, or not by you): ${unknown.slice(0, 8).map(h => `<span class="mono">${esc(h.slice(0,7))}</span>`).join(", ")}${unknown.length > 8 ? " …" : ""}</p>` : ""}
    ${p.commits > hs.length ? `<p class="muted" style="font-size:var(--fs-xs)">${`Showing the latest ${hs.length} of ${p.commits}.`}</p>` : ""}
    ${!hs.length ? `<p class="none">${p.prev ? "None recorded" : "Unknown"}</p>` : ""}
    <h3>Repository</h3><div class="code"><code>${esc(cmd)}</code><button class="copy" data-copy="${esc(cmd)}">Copy</button></div>
  </div></div></div>`;
  P.querySelectorAll(".card").forEach(b => b.onclick = () => select(b.dataset.id));
  bindGitEvents(P); bindCopy(P);
}
function prDetail(s, r){
  const P = $("#panel"), m = /github\.com\/([^/]+\/[^/]+)\/pull\/(\d+)/.exec(r.url || "");
  const cs = commitsOf(s).filter(c => c.t <= r.t + 60); // PR を作るまでにそのセッションでしたコミット
  const ps = (META.push || []).filter(p => p.project === s.project && p.t >= s.start - 60 && p.t <= r.t + 60);
  P.innerHTML = `<div style="--c:${colorOf(keyOf(s))}">
    <div class="eyebrow"><span class="dot"></span>${ico("pr")}Pull request</div>
    <h2>${esc(m ? `${m[1]} #${m[2]}` : r.url ? prName(r.url) : "Pull request")}</h2>
    <div class="muted" style="font-variant-numeric:tabular-nums">${`Created ${md(r.t)} ${hm(r.t)}`}</div>
    <div class="meta"><span>${esc(s.project)}</span>${s.branch ? `<span>${esc(s.branch)}</span>` : ""}</div>
    <div class="dcols"><div class="dcol">
    ${r.url ? `<p style="margin-top:4px">${ext(r.url, "Open this pull request ↗", "pill")}</p>` : ""}
    <p class="muted" style="font-size:var(--fs-xs)">Recorded when an agent created it in this session (gh pr create or GitHub tools). kiroku never asks GitHub, so its title, status and reviews are not shown here.</p>
    ${sesCard(s, "Session that created it")}
    </div><div class="dcol">
    ${ps.length ? `<h3>${`Pushes before it · ${ps.length}`}</h3>${ps.map(p => `<button class="gc-row" data-push="${esc(pushKey(p))}"><time>${md(p.t)} ${hm(p.t)}</time><span><i class="gtag">${ico("push")}${esc(p.ref)}</i>${p.commits ? ` ${plural(p.commits, "commit")}` : ""}</span><b></b></button>`).join("")}` : ""}
    <h3>${`Commits in the session before it · ${cs.length}`}</h3>
    ${cs.length ? cs.map(gitRow).join("") : `<p class="none">None</p>`}
  </div></div></div>`;
  P.querySelectorAll(".card").forEach(b => b.onclick = () => select(b.dataset.id));
  bindGitEvents(P);
}
