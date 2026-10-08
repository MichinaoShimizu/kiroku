// 印とリンク、詳細パネルのコミット・push・PR
/* git のコミットの印（コミットのノード: 線の上の丸） */
const GIT_ICON = `<svg class="gi" viewBox="0 0 16 16" aria-hidden="true"><path d="M1 8h4.2M10.8 8H15"/><circle cx="8" cy="8" r="2.8"/></svg>`;
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
    ${dHead(`GIT · ${c.ai ? "Run by AI" : "By hand (or another tool)"}`, c.subject, `${md(c.t)} ${hm(c.t)}`, [`<span>${esc(c.project)}</span>`, c.branch && `<span>${esc(c.branch)}</span>`, ext(c.url, `<span class="mono">${esc(c.hash.slice(0,7))}</span>`)])}
    <div class="dcols"><div class="dcol">
    <div class="mini">
      <div><div class="k">Files changed</div><div class="v">${c.nFiles}</div></div>
      <div><div class="k">Added</div><div class="v"><small>+</small>${c.added}<small> lines</small></div></div>
      <div><div class="k">Removed</div><div class="v"><small>−</small>${c.removed}<small> lines</small></div></div>
    </div>
    ${c.body ? `<h3>Message body</h3><p class="gbody">${esc(c.body)}</p>` : ""}
    ${ses ? sesCard(ses, c.session ? "Session that made this commit" : "Session running at this time") : ""}
    ${pushedIn(c).map(p => `<h3>Pushed</h3><button class="gc-row" data-push="${esc(pushKey(p))}"><time>${md(p.t)} ${hm(p.t)}</time><span><i class="gtag">${ico("push")}${esc(p.ref)}</i>${p.commits ? ` ${plural(p.commits, "commit")}` : ""}</span><b></b></button>`).join("")}
    ${c.url ? `<p class="dact">${ext(c.url, "Open this commit on the remote ↗", "pill")}</p>` : ""}
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
const gitRow = c => `<button class="gc-row" data-git="${esc(c.hash)}"><time>${md(c.t)} ${hm(c.t)}</time><span><i class="gtag${c.ai ? " ai" : ""}">${GIT_ICON}${esc(c.hash.slice(0,7))}</i> ${esc(c.subject)}</span><b>+${c.added} −${c.removed}</b></button>`;
function pushDetail(p){
  const P = $("#panel"), byHash = new Map((META.git || []).map(c => [c.hash, c]));
  const hs = p.hashes || [], known = hs.map(h => byHash.get(h)).filter(Boolean), unknown = hs.filter(h => !byHash.has(h));
  const ses = DATA.filter(x => x.project === p.project && p.t >= x.start - 60 && p.t <= x.end + 600);
  const cmd = p.prev ? `git -C ${p.repo} log --oneline ${p.prev.slice(0,12)}..${p.hash.slice(0,12)}` : `git -C ${p.repo} show ${p.hash.slice(0,12)}`;
  P.innerHTML = `<div style="--c:${st.colorBy === "project" ? colorOf(p.project) : "var(--ink-3)"}">
    ${dHead("GIT · Push", `Pushed to ${p.ref}`, `${md(p.t)} ${hm(p.t)}`, [`<span>${esc(p.project)}</span>`, `<span class="mono">${esc(p.ref)}</span>`, ext(p.url, `<span class="mono">${esc(p.hash.slice(0,7))}</span>`)])}
    <div class="dcols"><div class="dcol">
    <div class="mini">
      <div><div class="k">Commits sent</div><div class="v">${p.prev ? p.commits : "—"}</div></div>
      <div><div class="k">Latest commit</div><div class="v mono">${esc(p.hash.slice(0,7))}</div></div>
    </div>
    <p class="note">${p.prev ? "Read from this computer's git reflog: what moved this remote-tracking branch with a push." : "The first push kiroku can see for this branch, so how many commits it sent is unknown."}</p>
    ${ses.length ? ses.map((s, i) => sesCard(s, i ? "" : (ses.length > 1 ? "Sessions running at this time" : "Session running at this time"))).join("") : ""}
    ${p.url ? `<p class="dact">${ext(p.url, "Open the latest commit on the remote ↗", "pill")}</p>` : ""}
    </div><div class="dcol">
    <h3>${`Commits sent · ${p.prev ? p.commits : "?"}`}</h3>
    ${known.length ? known.map(gitRow).join("") : ""}
    ${unknown.length ? `<p class="note">${plural(unknown.length, "commit")} not in kiroku's view (made before the history kiroku read, or not by you): ${unknown.slice(0, 8).map(h => `<span class="mono">${esc(h.slice(0,7))}</span>`).join(", ")}${unknown.length > 8 ? " …" : ""}</p>` : ""}
    ${p.commits > hs.length ? `<p class="note">${`Showing the latest ${hs.length} of ${p.commits}.`}</p>` : ""}
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
    ${dHead(ico("pr") + "Pull request", m ? `${m[1]} #${m[2]}` : r.url ? prName(r.url) : "Pull request", `Created ${md(r.t)} ${hm(r.t)}`, [`<span>${esc(s.project)}</span>`, s.branch && `<span>${esc(s.branch)}</span>`])}
    <div class="dcols"><div class="dcol">
    ${r.url ? `<p class="dact">${ext(r.url, "Open this pull request ↗", "pill")}</p>` : ""}
    <p class="note">Recorded when an agent created it in this session (gh pr create or GitHub tools). kiroku never asks GitHub, so its title, status and reviews are not shown here.</p>
    ${sesCard(s, "Session that created it")}
    </div><div class="dcol">
    ${ps.length ? `<h3>${`Pushes before it · ${ps.length}`}</h3>${ps.map(p => `<button class="gc-row" data-push="${esc(pushKey(p))}"><time>${md(p.t)} ${hm(p.t)}</time><span><i class="gtag">${ico("push")}${esc(p.ref)}</i>${p.commits ? ` ${plural(p.commits, "commit")}` : ""}</span><b></b></button>`).join("")}` : ""}
    <h3>${`Commits in the session before it · ${cs.length}`}</h3>
    ${cs.length ? cs.map(gitRow).join("") : `<p class="none">None</p>`}
  </div></div></div>`;
  P.querySelectorAll(".card").forEach(b => b.onclick = () => select(b.dataset.id));
  bindGitEvents(P);
}
