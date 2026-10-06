// サマリーの各パネル：検索結果、週報・月報の下書き、アウトプット、日ごとの推移、計測の状態、AI の利用
/* 全期間の検索結果：どこに一致したかを抜き出して並べる。押すと詳細を出し、カレンダーはその週へ移る */
function snip(t, q){
  t = String(t || "").replace(/\s+/g, " "); const i = t.toLowerCase().indexOf(q); if (i < 0) return null;
  const a = Math.max(0, i - 36), b = Math.min(t.length, i + q.length + 64);
  return `${a ? "…" : ""}${esc(t.slice(a, i))}<mark>${esc(t.slice(i, i + q.length))}</mark>${esc(t.slice(i + q.length, b))}${b < t.length ? "…" : ""}`;
}
function hitOf(s, q){
  const tries = [["Title", [s.title]], ["Prompt", s.prompts.map(p => p.text)], ["File", s.files], ["PR", s.prs || []],
    ["Commit", commitsOf(s).flatMap(c => [`${c.hash.slice(0,7)} ${c.subject}`, ...(c.files || []).map(f => f.path)])],
    ["Project", [s.project]], ["Branch", [s.branch || ""]], ["Agent", [s.source]]];
  for (const [label, xs] of tries) for (const x of xs){ const h = snip(x, q); if (h) return {label, h}; }
  return {label: "", h: ""};
}
const SR_FIRST = 10, SR_STEP = 50; // 検索結果は、はじめに 10 件、「もっと見る」で 50 件ずつ
function searchPanel(){
  const R = $("#sres"), q = st.q, nS = st.srN || SR_FIRST, nC = st.scN || SR_FIRST;
  $("#review").innerHTML = ""; $("#worth").hidden = true; R.hidden = false;
  const ss = DATA.filter(s => !st.hidden.has(keyOf(s)) && searchText(s).includes(q)).sort((a,b) => b.start - a.start);
  const cs = (META.git || []).filter(gitHit).sort((a,b) => b.t - a.t);
  const row = s => { const h = hitOf(s, q);
    return `<button class="srow" data-s="${esc(s.id)}" style="--c:${colorOf(keyOf(s))}"><time>${md(s.start)}<small>${hm(s.start)}</small></time>
      <span class="b"><span class="ti"><i></i>${esc(s.title)}</span><span class="me">${esc(s.project)}${s.branch ? ` · ${esc(s.branch)}` : ""} · ${esc((s.source))}</span>
      ${h.h ? `<span class="hit"><em>${h.label}</em>${h.h}</span>` : ""}</span></button>`; };
  const crow = c => `<button class="srow" data-c="${esc(c.hash)}" style="--c:${colorOf(c.project)}"><time>${md(c.t)}<small>${hm(c.t)}</small></time>
      <span class="b"><span class="ti"><i></i>${snip(c.subject, q) || esc(c.subject)}</span><span class="me">${esc(c.project)}${c.branch ? ` · ${esc(c.branch)}` : ""} · <span class="mono">${esc(c.hash.slice(0,7))}</span> · ${plural(c.nFiles, "file")} +${c.added} −${c.removed}</span>
      ${(() => { const f = (c.files || []).find(f => f.path.toLowerCase().includes(q)); return f ? `<span class="hit"><em>File</em>${snip(f.path, q)}</span>` : ""; })()}</span></button>`;
  const more = (id, rest) => rest > 0 ? `<button class="pill srmore" id="${id}">${`Show ${Math.min(rest, SR_STEP)} more of ${rest}`}</button>` : "";
  R.innerHTML = `<div class="rvhead"><h2 id="srh" tabindex="-1">Search results</h2><p>${`Matches for "${esc(q)}" ${META.scope ? "in this file" : "(all time)"}, newest first. Sessions are matched on prompts, project, branch, agent, files changed by AI, pull requests and commits made during the session; commits on subject, body, hash and changed files. Click one to see its details; the calendar below moves to its week.`}</p>
      <button class="pill" id="sclear">Clear search</button></div>
    <div class="rvgrid srgrid">
      <section class="panel"><div class="ph"><h3>Sessions · ${ss.length}</h3></div>
        ${ss.length ? ss.slice(0, nS).map(row).join("") + more("srmS", ss.length - nS) : `<p class="none">No matching sessions.</p>`}</section>
      <section class="panel"><div class="ph"><h3>Commits · ${cs.length}</h3></div>
        ${cs.length ? cs.slice(0, nC).map(crow).join("") + more("srmC", cs.length - nC) : `<p class="none">${(META.git || []).length ? "No matching commits." : "No git commits were loaded."}</p>`}</section>
    </div>`;
  // もっと見る：足した最初の結果へフォーカスを移す（押したボタンは消えるので）
  const grow = (k, n, sel) => { const b = R.querySelector(sel === "s" ? "#srmS" : "#srmC"); if (!b) return;
    b.onclick = () => { st[k] = n + SR_STEP; searchPanel(); const f = R.querySelectorAll(`.srow[data-${sel}]`)[n]; if (f) f.focus(); }; };
  grow("srN", nS, "s"); grow("scN", nC, "c");
  const open = (t, id) => { st.mode = "week"; store.set("mode", "week"); st.week = mondayOf(new Date(t*1000)); select(id); };
  R.querySelectorAll("[data-s]").forEach(b => b.onclick = () => { const s = DATA.find(x => x.id === b.dataset.s); if (s) open(s.start, s.id); });
  R.querySelectorAll("[data-c]").forEach(b => b.onclick = () => { const c = META.git.find(x => x.hash === b.dataset.c); if (c) open(c.t, "git:" + c.hash); });
  $("#sclear").onclick = () => { $("#q").value = ""; st.q = ""; st.srN = st.scN = 0; render(); };
}
/* 週報・月報の下書き：プロジェクトごとに、やったこと（セッション）・コミット・PR を Markdown で並べる（AI は使わない） */
function reportText(w, M){
  const {ws, we} = period(), L = [];
  const start = M ? st.month : st.week, last = M ? new Date(st.month.getFullYear(), st.month.getMonth()+1, 0) : addDays(st.week, 6);
  const ses = DATA.filter(s => s.segs.some(([a,b]) => b > ws && a < we) && !st.hidden.has(keyOf(s))).sort((a,b) => a.start - b.start);
  const gits = (META.git || []).filter(c => c.t >= ws && c.t < we).sort((a,b) => a.t - b.t);
  const g = w.git || {};
  L.push(`## Work for ${dPeriod(M, start, last)} (${M ? "monthly" : "weekly"} report draft)`, "",
    `- Active time ${dur(w.active)} · sessions ${w.sessions} · prompts ${w.prompts}${g.commits ? ` · commits ${g.commits} (${g.ai} by AI)` : ""}${w.outputs && w.outputs.prs ? ` · pull requests ${w.outputs.prs}` : ""}`);
  const projs = [...new Set([...(w.projects || []).map(([k]) => k), ...gits.map(c => c.project)])];
  projs.forEach(pj => {
    const ps = ses.filter(s => s.project === pj), pc = gits.filter(c => c.project === pj), prs = [...new Set(ps.flatMap(s => s.prs || []))];
    if (!ps.length && !pc.length) return;
    const min = ((w.projects || []).find(([k]) => k === pj) || [0, 0])[1];
    L.push("", `### ${mdCode(pj)}${min ? ` (${dur(min)})` : ""}`);
    if (ps.length){ L.push("", "What I did:");
      const seen = new Map(); ps.forEach(s => { const t = s.title.replace(/\s+/g, " ").trim(); const x = seen.get(t); x ? x.n++ : seen.set(t, {s, n: 1}); });
      [...seen.values()].forEach(({s, n}) => L.push(`- ${mdText(s.title)} (${md(s.start)}${n > 1 ? ` and ${n-1} more` : ""}, ${mdText(s.source)})`)); }
    if (pc.length){ L.push("", "Commits:"); pc.slice(-15).forEach(c => { const h = mdText(String(c.hash).slice(0,7)); L.push(`- ${/^https?:\/\//i.test(c.url || "") ? `[${h}](${mdURL(c.url)})` : h} ${mdText(c.subject)}`); }); if (pc.length > 15) L.push(`- ${pc.length - 15} more`); }
    if (prs.length){ L.push("", "Pull requests:"); prs.forEach(u => L.push(`- ${mdURL(u)}`)); }
  });
  L.push("", "_Drafted with kiroku_"); // HTML のコメントは Slack などに貼るとそのまま見えるので、ふつうの 1 行に（見直してから使う旨は、下書きの上の説明に書いてある）

  return L.join("\n");
}
/* 期間に git へ残ったもの：コミットから数えたファイルと、この PC からの push。
   w.git と同じ範囲（凡例で隠した分は、ほかの数字と同じで差し引かない）で数える */
function periodGit(){
  const {ws, we} = period(), cs = (META.git || []).filter(c => c.t >= ws && c.t < we);
  const files = new Set(), projects = new Set(); let trunc = false;
  cs.forEach(c => { const fs = c.files || [];
    fs.forEach(f => files.add(`${c.repo}\u0000${f.path}`));
    if (fs.length < c.nFiles) trunc = true; // 1 コミットにつき残すファイルは先頭 40 件なので、それを超えたら「少なくとも」
    projects.add(c.project); });
  const ps = (META.push || []).filter(p => p.t >= ws && p.t < we);
  return {files: files.size, trunc, projects: projects.size, pushes: ps.length, refs: new Set(ps.map(p => `${p.repo}\u0000${p.ref}`)).size};
}
/* アウトプット：使ったもの（コスト）→ git に残ったもの → その 2 つを割った指標（Compared）の順に並べる。
   残ったものは、出したものの量であって、価値や生産性ではない */
function outcomePanel(w, pw, unit, ph, stat){
  const o = w.outputs || {commits:0}, g = w.git, u = w.usage || {}, G = periodGit();
  const gc = g && g.commits ? g : null; // git のコミット（行・ファイル・push は git から数えるので、これがないときは出さない）
  const hasOut = o.commits || gc || G.pushes || o.prs, hasCommit = o.commits || gc;
  const po = pw && pw.outputs, V = vsPrev(pw, unit), d = (a, b) => V.diff(a, V.of(null, b)); // AI のコミットと PR は日ごとの値がないので、途中の期間は比べない
  const n = v => v.toLocaleString(LOC()), times = v => `${v}`, base = w.outBase ?? w.sessions;
  const cost = [
    stat("Active time", dur(w.active,true), V.diff(w.active, V.of("active", pw && pw.active), dur), "active"),
    stat("Prompts", n(w.prompts), V.diff(w.prompts, V.of("prompts", pw && pw.prompts)), "prompts"),
    u.tokens ? stat("Estimated cost", usdH(costOf(u)), costOf(u) == null ? "Models not in the price table" : V.diff(u.cost, V.of("cost", pw && pw.usage && pw.usage.cost), usd), "cost") : "",
    u.tokens ? stat("Tokens", tok(u.tokens), `Output ${tok(u.out)}`, "tokens") : "",
    u.credits ? stat("Kiro credits", crN(u.credits), "As recorded in history", "credits") : "",
  ].join("");
  const lines = gc ? gc.added + gc.removed : 0;
  const out = hasOut ? [
    gc ? stat("Git commits", times(gc.commits), `${gc.ai} by AI (${Math.round(gc.ai*100/gc.commits)}%)${pw && pw.git ? ` · ${V.diff(gc.commits, V.of("commits", pw.git.commits))}` : ""}`, "gitCommits") : "",
    gc ? "" : stat("AI commits", times(o.commits), d(o.commits, po && po.commits), "commits"), // Git のコミットがあれば「うち AI」に出ている
    gc ? stat("Lines changed", n(lines), `+${n(gc.added)} −${n(gc.removed)} · ${n(Math.round(lines/gc.commits))} per git commit`, "lines") : "",
    gc && G.files ? stat("Files changed", n(G.files), `In ${plural(G.projects, "project")}${G.trunc ? " · at least" : ""}`, "files") : "",
    G.pushes ? stat("Pushes", times(G.pushes), `To ${plural(G.refs, "branch", "branches")} · from this computer`, "pushes") : "",
    o.prs ? stat("Pull requests", times(o.prs), "Created by AI", "prs") : "",
  ].join("") : "";
  // 使ったものと比べた指標。残ったものではないので別の行にして、何と何を割ったかを添える
  const cmp = hasCommit ? [
    w.costPerCommit != null ? stat("Estimated cost per commit", usdH(w.costPerCommit), `Claude Code's estimated cost ÷ ${plural(o.commits, "AI commit")}`, "costPerCommit") : "",
    stat("Sessions that reached a commit or PR", base ? `${Math.round(w.outSessions*100/base)}<small>%</small>` : "—", `${w.outSessions} of ${plural(base, "session")}`, "outSessions"),
  ].join("") : "";
  // 「残ったもの」に添える一言は、実際に出したカードから作る（git を読めなかった週に、行やファイルがあるように書かないため）
  const names = [o.commits || gc ? "commits" : "", gc ? "lines" : "", gc && G.files ? "files" : "", G.pushes ? "pushes" : "", o.prs ? "pull requests" : ""].filter(Boolean);
  const outSub = names.length ? names.join(", ").replace(/, ([^,]+)$/, " and $1").replace(/^./, c => c.toUpperCase()) : "From git and from history";
  const side = (cls, label, sub, body) => `<div class="ocside ${cls}"><div class="ocl"><b>${label}</b><span>${sub}</span></div>${body}</div>`;
  return `<section class="panel oc">${ph(2, "Cost and outputs", `What you spent ${uThis(unit)} and what it left behind`, "outputs")}
    <div class="ocgrid">
      ${side("spent", "Cost", "Time and usage", `<div class="stats">${cost}</div>`)}
      <div class="ocarrow" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M5 12h14M13 6l6 6-6 6"/></svg></div>
      ${side("left", "Left behind", outSub, out ? `<div class="stats">${out}</div>${gc ? "" : `<p class="none">Lines, files and pushes are counted from git commits, which were not read here.</p>`}` : `<p class="none">No commits recorded.</p>`)}
    </div>
    ${cmp ? side("occmp", "Compared", "Cost ÷ what it left behind", `<div class="stats">${cmp}</div>`) : ""}</section>`;
}
/* 計測の状態 */
/* 履歴を自動で消すエージェント（既定のままの Claude Code など）を知らせ、公式ドキュメントへ案内する。閉じたら出さない */
function keepNotice(){
  if (store.get("keepNoticeOff", false) || archOn() || META.scope) return ""; // kiroku がコピーを残していれば、消えても見られる
  const r = (META.report || []).find(r => r.n && r.retention && r.retention.days && !r.retention.set); if (!r) return "";
  const k = r.retention, snippet = `"${k.setting}": 3650`, cmd = "kiroku archive on";
  return `<div class="keep" role="note"><b>Your older history will be deleted</b>
    <p>${`${esc((r.name))} automatically deletes conversation history older than ${k.days} days (<code>${esc(k.setting)}</code> is at its default). Deleted history cannot be shown by kiroku and cannot be recovered. To keep it, set a long period such as <code>${esc(snippet)}</code> in your settings file (<code>~/.claude/settings.json</code>).`}</p>
    <p>${`If you'd rather not change the setting, kiroku can keep a copy of the history instead. It saves a compressed copy each time you open kiroku and shows deleted conversations from it (copies stay on this computer only).${LIVE ? "" : ` To turn it on, run <code>${cmd}</code>.`}`}</p>
    <div class="ka">${ext(k.docs, "See how to set it in the official docs ↗", "pill")}<button class="pill" data-copy="${esc(snippet)}">Copy setting</button>${LIVE ? `<button class="pill" id="keeparch">Keep a copy in kiroku</button>` : `<button class="pill" data-copy="${cmd}">Copy command</button>`}<button class="pill" id="keepoff">Dismiss</button></div></div>`;
}
/* kiroku html --week / --month で書き出した、1 つの期間だけのファイル。渡された人がいちばん上で、何のファイルか・ほかの期間が空の理由・
   どの時計で見ているかがわかるように */
function scopeNote(){
  const sc = META.scope; if (!sc) return "";
  const h = sc.offset === 0 ? "" : `${sc.offset < 0 ? "−" : "+"}${Math.floor(Math.abs(sc.offset)/3600)}${Math.abs(sc.offset)%3600 ? ":" + String(Math.abs(sc.offset)%3600/60).padStart(2, "0") : ""}`;
  return `This file only includes ${sc.mode === "week" ? "the " : ""}${periodLabel(sc.mode, sc.key)}. Sessions that cross its edges are included whole; other ${sc.mode}s have no records here.${SHIFT ? ` Times are shown in ${esc(sc.zone || "")} (UTC${h}), where the file was written.` : ""}`; }
// scopeStart は、期間だけのファイルの期間の初日。時刻（from）ではなく名前（2026-09-28 / 2026-09）から作る。
// 週・月の集計は書き出した人の時間帯で区切っていて、ほかの時間帯で開くと from が前の日になり、前の週を開いてしまうため
function scopeStart(){ const [y, m, d] = META.scope.key.split("-").map(Number); return new Date(y, m-1, d || 1); }
function archOn(){ return !!(META.archive && META.archive.on); }
function bytes(n){ return n >= 1<<30 ? (n/(1<<30)).toFixed(1)+" GB" : n >= 1<<20 ? (n/(1<<20)).toFixed(1)+" MB" : n >= 1<<10 ? (n/(1<<10)).toFixed(1)+" KB" : n+" B"; }
// keepArchive は「kiroku にコピーを残す」（kiroku serve のときだけ）。kiroku archive on と同じことをして、集計を取り込み直す。
async function keepArchive(){
  try {
    const r = await fetch("archive", {method:"POST", headers:{"X-Kiroku":"1"}, cache:"no-store"}); if (!r.ok) throw new Error(r.status);
    applyData(await r.json()); render();
    toast("Kept a copy. kiroku will save one each time you open it", 4500);
  } catch(e){ toast("Couldn't keep a copy. Run kiroku archive on instead", 4500); }
}
function keepRow(r){ // 計測の状態に添える：どこまでさかのぼれるか、いつ消えるか
  const o = r.oldest ? dMDY(new Date(r.oldest*1000)) : "";
  const k = r.retention, link = k && k.docs ? ` ${ext(k.docs, "official docs ↗")}` : "";
  return (o ? ` · oldest record ${o}` : "") +
    (r.archived ? ` · ${plural(r.archived, "deleted conversation")} shown from kiroku's copy` : "") +
    (!k ? "" : k.days && !k.set && archOn() ? `<br>${`Records older than ${k.days} days are deleted automatically, but kiroku keeps a copy`}`
      : k.days && !k.set ? `<br><span class="kw">${`Records older than ${k.days} days are deleted automatically (<code>${esc(k.setting)}</code> is at its default)`}${link}</span>`
      : k.days ? `<br>${`Kept for ${k.days} days (<code>${esc(k.setting)}</code>)`}`
      : `<br>${`Older records are deleted after a period (<code>${esc(k.setting)}</code>)`}${link}`); }
function measure(w){
  const rows = META.report.map(r => { const dt = r.detailEn || r.detail;
    return `<li class="${r.error?"warn":""}">${esc((r.name))}: ${plural(r.n, "session")}${dt?` (${esc(dt)})`:""}${r.dup?` (${plural(r.dup, "duplicate conversation")} found elsewhere not counted)`:""}${r.error?` (some files couldn't be read)`:""}${keepRow(r)}</li>`; });
  const a = META.archive; // kiroku archive
  if (a && (a.on || a.files)) rows.push(`<li>${a.on ? `kiroku's copy: on (${plural(a.files, "file")}, ${bytes(a.bytes)})`
      : `kiroku's copy: off. Copies kept so far (${plural(a.files, "file")}, ${bytes(a.bytes)}) are still shown`}<br><code>${esc(a.dir)}</code></li>`);
  if (w.usage && w.usage.unpriced) { const ms = (w.usage.unpricedModels || []).map(m => `<code>${esc((m))}</code>`).join(", ");
    rows.push(`<li class="warn">${`${tok(w.usage.unpriced)} tokens from models not in the price table are not included in the estimated cost${ms ? ` (${ms})` : ""}. Add their prices with <code>--prices</code>`}</li>`); }
  if (w.usage && w.usage.tokens) rows.push(`<li>${`Price table for estimated cost: ${META.prices && META.prices.custom ? "from <code>--prices</code>" : (META.prices ? `public rates as of ${esc(META.prices.asOf)}` : "public rates") + " (change it with <code>--prices</code>)"}`}</li>`);
  return `<h3>Data sources</h3><ul class="mlist">${rows.join("")}</ul>`;
}
// soFar は、今見ている期間が途中なら、始まりから今日までの日数（今日を含む）。終わった期間や先の期間は null。
function soFar(){ const {ws, we} = period(), now = nowMs()/1000, a = new Date(ws*1000);
  return ws <= now && now < we ? Math.min(dayNo(a, now) + 1, dayNo(a, we)) : null; }
// vsPrev は、前の期間との比べ方。途中の期間は、前の期間の同じ日まで（先頭から今日と同じ日数）と比べる。
// 途中の値を前の期間まるごとと比べると、いつも少なく見えるため。
//   label: 差に添える言葉、range: 比べた前の期間の日付（途中のときだけ）
//   of(f, whole): 比べる前の期間の値。途中なら日ごとの集計 f の合計（日ごとの値がない指標は null）、終わった期間なら whole
//   diff(a, b, fmt): 「<label> +差」。b が null なら空
function vsPrev(pw, unit){
  const n0 = pw ? soFar() : null, n = n0 == null ? null : Math.min(n0, pw.days.length); // 先月が今月より短いときは、先月の最後の日まで
  const a = n == null ? null : new Date(periodBack(1).ws * 1000), range = n == null ? "" : n === 1 ? dMD(a) : dSpan(a, addDays(a, n - 1));
  const label = n == null ? `vs ${uLast(unit)}` : `vs ${range}`;
  const of = (f, whole) => !pw || whole == null ? null : n == null ? whole : f ? pw.days.slice(0, n).reduce((t, d) => t + (d[f] || 0), 0) : null;
  const diff = (a, b, fmt = x => x) => b == null ? "" : a === b ? `${label} <span class="nw">no change</span>` : `${label} <span class="nw">${a-b>0?"+":"−"}${fmt(Math.abs(a-b))}</span>`;
  return {n, range, label, of, diff};
}
// projection は、今月の途中なら、今日までのペースが月末まで続いたときの目安コストとクレジット（推定）。
// 今日までの日数（今日を含む）で割り、月の日数を掛ける。月の初めは日数が少なく当てにならないので 7 日たつまで、
// 最後の日は実績とほとんど変わらないので出さない。
// 目安コスト（トークン）とクレジットは別の枠に出す。Claude Code と Kiro を両方使う人には、追うべき枠が 2 つあるため。
function projection(w){ const {ws, we} = period(), now = nowMs()/1000, u = w.usage;
  const a = new Date(ws*1000), nd = dayNo(a, we), days = dayNo(a, now) + 1; // 今日を含めた日数（日付で数える）
  if (st.mode !== "month" || !u || !(ws <= now && now < we) || days <= 7 || days >= nd) return null;
  const k = nd / days, c = u.tokens ? costOf(u) : null; // 料金表にないモデルだけなら、月末の目安コストも出さない
  return {cost: c == null ? null : c * k, credits: u.credits ? u.credits * k : null, days}; }
function aiUsage(w, pw, unit){
  const u = w.usage; if (!u || (!u.tokens && !u.credits)) return "";
  const stat = (k, v, s, h) => `<div class="stat"><div class="k">${k}${hb(h)}</div><div class="v">${v}</div>${s?`<div class="s">${s}</div>`:""}${hint(h)}</div>`;
  const pj = projection(w);
  const totalC = u.models.reduce((t,r)=>t+r[1],0) || 1, totalT = u.models.reduce((t,r)=>t+r[2],0) || 1, byCost = totalC > 0.0001;
  return `<div class="stats" style="margin-top:4px">
      ${pj && pj.cost != null ? stat("Month-end cost (estimate)", "≈ " + usdH(pj.cost), `If the pace of the first ${pj.days} days continues`, "projection") : ""}
      ${pj && pj.credits != null ? stat("Month-end credits (estimate)", `≈ ${crN(pj.credits)}<small> credits</small>`, `If the pace of the first ${pj.days} days continues`, "projectionCr") : ""}
      ${u.tokens ? stat("Read from cache", u.cacheHit==null ? "—" : `${Math.round(u.cacheHit*100)}<small>%</small>`, "Share of input", "cache") : ""}
      ${stat("Subagents", `${u.subagents}`, u.subagents ? `Total run time ${dur(u.subMin)}` : "Not used", "subagents")}
      ${w.costPerAsk != null ? stat("Estimated cost per prompt", usdH(w.costPerAsk), `n=${w.prompts}`, "costPerAsk") : ""}
    </div>
    ${u.models.length ? `<div style="margin-top:16px" class="k muted">By model${byCost ? " (estimated cost)" : " (tokens)"}${hb("models")}</div>${hint("models")}
      <div class="mstack" style="margin-top:8px">${u.models.map((r,i)=>`<span style="flex:${byCost?r[1]:r[2]};--o:${shade(i)}"${tipAttr((r[0]), `Estimated cost ${usd(r[1])}`, `Tokens ${tok(r[2])}`, `${Math.round((byCost?r[1]/totalC:r[2]/totalT)*100)}%`)}></span>`).join("")}</div>
      ${u.models.slice(0,6).map((r,i)=>`<div class="mrow"><span class="nm"><i style="--o:${shade(i)}"></i>${esc((r[0]))}</span><span class="tm">${!r[1] && (u.unpricedModels || []).includes(r[0]) ? `<span title="Not in the price table">—</span>` : usd(r[1])}<small>${tok(r[2])}</small></span><span class="pc">${Math.round((byCost?r[1]/totalC:r[2]/totalT)*100)}%</span></div>`).join("")}` : ""}
    ${u.subTypes.length ? `<div style="margin-top:16px" class="k muted">Subagent types (calls)</div><div style="margin-top:8px" class="chips">${u.subTypes.map(([k,v])=>`<span class="mono">${esc(k)}<b>${v}</b></span>`).join("")}</div>` : ""}
    ${u.heavy.length ? `<div style="margin-top:16px" class="k muted">Heaviest sessions${hb("heavy")}</div>${hint("heavy")}<div style="margin-top:8px">${u.heavy.map(h=>`<button class="card" data-id="${esc(h.id)}"><span class="ti">${esc(h.title)}</span><span class="me">${md(h.start)} · ${esc(h.project)} · ${usd(h.cost)}${h.subagents?` · ${plural(h.subagents, "subagent")}`:""}</span></button>`).join("")}</div>` : ""}`;
}
function nativeText(v){
  const num = (x, d) => Number(x.toFixed(d)).toLocaleString(LOC());
  const u = {"回": "", "件": "", "トークン": " tokens", "文字": " chars"}[v.unit]; // 単位は Go の定義（日本語）を英語に読みかえる
  return v.unit === "%" ? `${num(v.v,1)}%` : v.unit === "秒" ? `${num(v.v,1)}s` : v.unit === "クレジット" ? `${crN(v.v)} credits` : `${num(v.v,0)}${u ?? " " + v.unit}`;
}
const nlabel = v => v.labelEn || v.label; // 参考指標の名前（Go の英語の名前）
function nativeRows(values){
  return values.map(v=>`<div class="nrow"><span>${esc(nlabel(v))}</span><span class="v">${esc(nativeText(v))}</span><span class="n">n=${v.n}</span></div>`).join("");
}
function nativeSection(w){
  if (!w.native || !w.native.length) return "";
  // 各エージェントが自分で記録する数。エージェントどうしでは比べられないので、たたんでおく
  return `<details class="moreS"><summary>Agent-specific metrics</summary><p class="k muted">Numbers each agent records itself${hb("native")}</p>${hint("native")}
    ${w.native.map(g=>`<div class="ngroup"><div class="hd"><b>${esc((g.source))}</b><span>${plural(g.sessions, "session")}</span></div>${nativeRows(g.values)}</div>`).join("")}</details>`;
}
function foot(){
  return `<div class="foot"><div>Generated ${dStamp(GENERATED)}</div></div>`;
}

function bindCopy(root){ root.querySelectorAll("[data-copy]").forEach(b => b.onclick = () => copy(b.dataset.copy)); }

