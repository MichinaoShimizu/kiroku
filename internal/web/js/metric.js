// 上の帯とサマリーの数字を押したときの内訳（日ごと・エージェントごと・プロジェクトごと・モデルごと・セッション）
/* ── 数字の内訳 ──
   上の帯とサマリーの数字（トークン・目安コスト・月末の見込み・クレジット・作業時間など）を押すと、画面の真ん中のダイアログで内訳を出す。
   日ごと・エージェントごと・プロジェクトごと・モデルごとは期間の集計（Go）をそのまま使うので、合計と一致する。
   セッションの一覧だけは、期間にかかったセッションの、セッション全体の値（期間の外の分も入る） */
const BRK = {
  tokens: {n: "Tokens", h: "tokens", v: x => x.tokens, f: v => tok(v), ses: s => sesTok(s), model: 2},
  cost: {n: "Estimated cost", h: "cost", v: x => x.cost, f: v => usd(v), ses: s => s.cost, model: 1},
  projection: {n: "Month-end cost (estimate)", h: "projection", v: x => x.cost, f: v => usd(v), proj: "cost"},
  credits: {n: "Kiro credits", h: "credits", v: x => x.credits, f: v => `${crN(v)} cr`, ses: s => s.credits},
  projectionCr: {n: "Month-end credits (estimate)", h: "projectionCr", v: x => x.credits, f: v => `${crN(v)} cr`, proj: "credits"},
  active: {n: "Active time", h: "active", v: x => x.minutes ?? x.active, f: v => dur(v), ses: s => sesMin(s)},
  days: {n: "Active days", h: "active", v: x => x.active, f: v => v ? dur(v) : "—"},
  sessions: {n: "Sessions / prompts", h: "prompts", v: x => x.prompts, f: v => `${Math.round(v)}`, ses: s => s.nPrompts},
  limits: {n: "Usage limit hits", h: "limits"},
  compactions: {n: "Compactions", h: "compactions"},
  commits: {n: "Git commits", h: "gitCommits", v: x => x.commits, f: v => `${Math.round(v)}`},
  // ここから下は、期間の集計（Go）に日ごと・エージェントごとの値がない数。MB の関数で、履歴やコミットから組み立てる
  aiCommits: {n: "AI commits", h: "commits"},
  lines: {n: "Lines changed", h: "lines"},
  files: {n: "Files changed", h: "files"},
  pushes: {n: "Pushes", h: "pushes"},
  prs: {n: "Pull requests", h: "prs"},
  costPerCommit: {n: "Estimated cost per commit", h: "costPerCommit"},
  outSessions: {n: "Sessions that reached a commit or PR", h: "outSessions"},
  focus: {n: "Focus blocks (60+ min)", h: "focus"},
  fix: {n: "Prompts with corrections or interruptions", h: "fix"},
  bigPrompts: {n: "Oversized prompts", h: "bigPrompts"},
  switches: {n: "Project switches per day", h: "switches"},
  parallel: {n: "Parallel time", h: "parallel"},
  wait: {n: "Wait time (median)", h: "wait"},
  ai: {n: "Total AI run time", h: "ai"},
  cache: {n: "Read from cache", h: "cache"},
  subagents: {n: "Subagents", h: "subagents"},
  costPerAsk: {n: "Estimated cost per prompt", h: "costPerAsk"},
};
const sesTok = s => [s.usage, ...(s.subagents || []).map(a => a.usage)].reduce((t,u) => t + (u ? u.in+u.out+u.cw+u.cw1h+u.cr : 0), 0);
function sesMin(s){ const {ws, we} = period(); return s.segs.reduce((t, [a, b]) => t + Math.max(0, Math.min(b, we) - Math.max(a, ws))/60, 0); } // 期間の中で作業していた分
// 名前ごとの件数。履歴の名前（プロジェクト）を {} のキーにすると、"__proto__" や "constructor" が数えられないので Map で数える
const countBy = (list, f) => [...list.reduce((m, x) => m.set(f(x), (m.get(f(x)) || 0) + 1), new Map())];
const sumBy = (list, f, v) => [...list.reduce((m, x) => m.set(f(x), (m.get(f(x)) || 0) + v(x)), new Map())]; // 名前ごとの合計（countBy と同じく Map で）
const pcOf = (v, t) => t > 0 ? Math.round(v*100/t) : 0;
// 日ごとの柱。見込み（proj）があれば、まだ来ていない日に 1 日あたりの平均を薄く積む
function mDays(w, m, proj){
  const {ws} = period(), a = new Date(ws*1000), M = st.mode === "month", vals = w.days.map(d => m.v(d) || 0);
  const now = nowMs()/1000, cur = dayNo(a, now), avg = proj ? proj.avg : 0;
  const max = Math.max(...vals, avg, 1e-9);
  const cols = w.days.map((d, i) => { const dt = addDays(a, i), fut = proj && i > cur, v = fut ? avg : vals[i];
    const tip = `${DOW[dt.getDay()]}, ${dMD(dt)}: ${fut ? `≈ ${m.f(avg)} at the current pace` : m.f(vals[i])}`;
    return `<span class="mcol${fut ? " fut" : ""}${i === cur ? " now" : ""}" title="${esc(tip)}"><span class="b" style="height:${v ? Math.max(2, v/max*100) : 0}%"></span><small>${M ? dt.getDate() : DOW[dt.getDay()]}</small></span>`; }).join("");
  return `<h3>By day</h3><div class="mdays${M ? " month" : ""}"${proj ? ` style="--avg:${avg/max*100}%"` : ""}>${cols}</div>`;
}
// 横の棒の一覧（エージェント・プロジェクト・モデル）。total が false なら割合の欄を空ける（割合や中央値のように、足しても意味のない値）。
// keep は並べ替えない（待ち時間の幅のように、順に意味がある）
function mBars(title, rows, m, total, keep){
  rows = rows.filter(r => r.v > 0); if (!keep) rows.sort((a,b) => b.v - a.v); if (!rows.length) return "";
  const pcs = total !== false; total = pcs ? total || rows.reduce((t,r) => t + r.v, 0) : 0; const max = Math.max(...rows.map(r => r.v));
  return `<h3>${title}</h3><div class="mbars">${rows.slice(0, 8).map(r => `<div class="mbar" style="--c:${r.c}"><span class="nm" title="${esc(r.name)}">${r.mark || ""}${esc(r.name)}</span><span class="tr"><span style="width:${r.v/max*100}%"></span></span><span class="vv">${esc(m.f(r.v))}</span><span class="pc">${pcs ? `${pcOf(r.v, total)}%` : ""}</span></div>`).join("")}${rows.length > 8 ? `<p class="note">${rows.length - 8} more not shown</p>` : ""}</div>`;
}
// 名前と値の組（sumBy・countBy）を、エージェントかプロジェクトの棒にする
const agRows = pairs => pairs.map(([k, v]) => ({name: k, c: agColor(k), mark: agMark(k), v}));
const pjRows = pairs => pairs.map(([k, v]) => ({name: k, c: projColor(k), v}));
// 一覧（HTML の行）を n 件まで。残りは件数だけ
const mList = (title, rows, n = 10) => rows.length ? `<h3>${title}</h3><div class="msess">${rows.slice(0, n).join("")}</div>${rows.length > n ? `<p class="note">${rows.length - n} more not shown</p>` : ""}` : "";
// 時刻つきの値 [t, v] を期間の日ごとに足す（mDays に渡す形）
function perDay(w, items){ const {ws} = period(), a = new Date(ws*1000), d = w.days.map(() => ({v: 0}));
  items.forEach(([t, v]) => { const i = dayNo(a, t); if (d[i]) d[i].v += v; }); return {days: d}; }
const WHOLE = () => `<p class="note">Session figures cover the whole session, including any part outside this ${st.mode === "month" ? "month" : "week"}.</p>`;
function mSessions(title, list, f){
  if (!list.length) return "";
  return `<h3>${title}</h3><div class="msess">${list.map(([s, v]) => sesCardH(s, sesMeta(s), f(v))).join("")}</div>`;
}
const projColor = p => st.colorBy === "project" ? colorOf(p) : "var(--acc)";
/* 期間の集計（Go）に内訳のない数の内訳。どれも {big, body} を返す（出せないときは null）。
   日ごとの値は、Go と同じ数え方（1 分刻み・記録された時刻の日）で履歴やコミットから数え直すので、合計と一致する。
   セッションの値（WHOLE を添えるもの）だけは、セッション全体の値 */
const cnt = {f: v => `${v}`}, durM = {f: v => dur(v)};
// minuteGrid は、Go の Summarize と同じ 1 分刻みで、分ごとに動いていたセッション（ai は区間の分の合計）
function minuteGrid(ws, we){ const n = Math.floor((we - ws)/60), M = new Array(n), own = new Map();
  DATA.forEach(s => { if (s.end < ws || s.start >= we) return;
    s.segs.forEach(([a, b]) => { const m0 = Math.max(0, Math.floor((a - ws)/60)), m1 = Math.min(n, Math.ceil((b - ws)/60));
      if (m1 > m0) own.set(s, (own.get(s) || 0) + m1 - m0);
      for (let m = m0; m < m1; m++) (M[m] ||= []).push(s); }); });
  return {M, own}; }
const pushRow = p => `<button class="gc-row" data-push="${esc(pushKey(p))}"><time>${md(p.t)} ${hm(p.t)}</time><span><i class="gtag">${ico("push")}${esc(p.ref)}</i>${p.commits ? ` ${plural(p.commits, "commit")}` : ""} · ${esc(p.project)}</span><b></b></button>`;
const prRow = ({s, r}) => `<button class="gc-row" data-pr="${esc(prKey(s, r))}"><time>${md(r.t)} ${hm(r.t)}</time><span><i class="gtag">${ico("pr")}${esc(r.url ? prName(r.url) : "Pull request")}</i> ${esc(s.title)}</span><b></b></button>`;
const gitIn = (ws, we) => (META.git || []).filter(c => c.t >= ws && c.t < we);
const calcH = parts => `<div class="mcalc">${parts.map(p => typeof p === "string" ? `<span>${p}</span>` : `<div><small>${p[0]}</small><b>${p[1]}</b></div>`).join("")}</div>`; // [名前, 値（HTML）] と記号（÷ =）
const MB = {
  commits(w, {ws, we}){ // Git commits：日ごと・プロジェクトごと・AI のエージェントごと・だれが・一覧
    const cs = gitIn(ws, we).sort((a,b) => b.t - a.t), g = w.git || {}; if (!cs.length) return null;
    const ag = cs.filter(c => c.ai).map(c => (DATA.find(s => s.id === c.session) || {}).source || "Unknown agent");
    return {big: `${g.commits || cs.length}`, body: mDays(w, BRK.commits) + mBars("By project", pjRows(countBy(cs, c => c.project)), cnt) +
      mBars("Who made them", [{name: "By AI", c: "var(--acc)", v: cs.filter(c => c.ai).length}, {name: "Not by AI", c: "var(--other)", v: cs.filter(c => !c.ai).length}], cnt) +
      mBars("Run by AI, by agent", agRows(countBy(ag, x => x)), cnt) +
      mList("Commits", cs.map(gitRow), 15)}; },
  aiCommits(w, {inRange}){ // git を読めなかったときの AI のコミット（履歴の outputs）
    const xs = inRange.filter(s => s.outputs && s.outputs.commits > 0);
    return {big: `${(w.outputs || {}).commits || 0}`, body: mBars("By agent", agRows(sumBy(xs, s => s.source, s => s.outputs.commits)), cnt) + mBars("By project", pjRows(sumBy(xs, s => s.project, s => s.outputs.commits)), cnt) +
      mSessions("Sessions with the most commits", xs.map(s => [s, s.outputs.commits]).sort((a,b) => b[1] - a[1]).slice(0, 8), v => plural(v, "commit")) + (xs.length ? WHOLE() : "")}; },
  lines(w, {ws, we}){ const cs = gitIn(ws, we), L = c => c.added + c.removed, g = w.git || {}; if (!cs.length) return null;
    const m = {f: v => commas(v)};
    return {big: commas(g.added + g.removed), body: `<p class="note">+${commas(g.added)} added · −${commas(g.removed)} removed</p>` + mDays(perDay(w, cs.map(c => [c.t, L(c)])), {v: d => d.v, f: v => commas(v)}) +
      mBars("By project", pjRows(sumBy(cs, c => c.project, L)), m) +
      mList("Largest commits", cs.filter(c => L(c) > 0).sort((a,b) => L(b) - L(a)).map(gitRow), 8)}; },
  files(w, {ws, we}){ const cs = gitIn(ws, we), by = new Map(), pj = new Map();
    cs.forEach(c => (c.files || []).forEach(f => { const k = `${c.repo}\u0000${f.path}`, x = by.get(k) || {name: f.path, project: c.project, v: 0}; x.v++; by.set(k, x); }));
    by.forEach(x => pj.set(x.project, (pj.get(x.project) || 0) + 1)); if (!by.size) return null;
    const dup = new Set(countBy([...by.values()], x => x.name).filter(([, v]) => v > 1).map(([k]) => k)); // 同じパスがほかのプロジェクトにもあれば、プロジェクト名を添える
    const trunc = cs.some(c => (c.files || []).length < c.nFiles);
    return {big: commas(by.size), body: mBars("By project (different files)", pjRows([...pj]), cnt) +
      mBars("Changed most often", [...by.values()].filter(x => x.v > 1).map(x => ({...x, name: dup.has(x.name) ? `${x.project}: ${x.name}` : x.name, c: projColor(x.project)})), {f: v => plural(v, "commit")}, false) +
      (trunc ? `<p class="note">Some commits changed more than 40 files; kiroku keeps the first 40 of each, so this is at least the number shown.</p>` : "")}; },
  pushes(w, {ws, we}){ const ps = (META.push || []).filter(p => p.t >= ws && p.t < we).sort((a,b) => b.t - a.t); if (!ps.length) return null;
    return {big: `${ps.length}`, body: mDays(perDay(w, ps.map(p => [p.t, 1])), {v: d => d.v, f: v => `${v}`}) + mBars("By project", pjRows(countBy(ps, p => p.project)), cnt) +
      mBars("By branch", countBy(ps, p => p.ref).map(([k, v]) => ({name: k, c: "var(--acc)", v})), cnt) + mList("Pushes", ps.map(pushRow), 15)}; },
  prs(w, {ws, we}){ const xs = DATA.flatMap(s => (s.prAt || []).filter(r => r.t >= ws && r.t < we).map(r => ({s, r}))).sort((a,b) => b.r.t - a.r.t);
    return {big: `${(w.outputs || {}).prs || xs.length}`, body: mBars("By agent", agRows(countBy(xs, x => x.s.source)), cnt) + mBars("By project", pjRows(countBy(xs, x => x.s.project)), cnt) + mList("Pull requests", xs.map(prRow), 15)}; },
  costPerCommit(w, {inRange}){ const o = w.outputs || {}; if (w.costPerCommit == null || !o.commits) return null;
    const xs = inRange.filter(s => s.outputs && s.outputs.commits > 0 && s.cost > 0).map(s => [s, s.cost / s.outputs.commits]).sort((a,b) => b[1] - a[1]);
    return {big: usdH(w.costPerCommit), body: calcH([["Claude Code's estimated cost", `≈ ${esc(usd(w.costPerCommit * o.commits))}`], "÷", ["AI commits", `${o.commits}`], "=", ["Per commit", esc(usd(w.costPerCommit))]]) +
      mSessions("Sessions with the highest cost per commit", xs.slice(0, 8), usd) + (xs.length ? WHOLE() : "")}; },
  outSessions(w, {inRange}){ const base = w.outBase ?? w.sessions; if (!base) return null;
    const xs = inRange.filter(s => s.outputs && (s.outputs.commits || s.outputs.prs)).sort((a,b) => b.start - a.start);
    const what = s => [s.outputs.commits ? plural(s.outputs.commits, "commit") : "", s.outputs.prs ? plural(s.outputs.prs, "PR") : ""].filter(Boolean).join(" · ");
    return {big: `${Math.round(w.outSessions*100/base)}<small>%</small>`, body: calcH([["Reached a commit or PR", `${w.outSessions}`], "÷", ["Claude Code sessions", `${base}`]]) +
      mBars("By project", pjRows(countBy(xs, s => s.project)), cnt) + mList("Sessions that reached one", xs.map(s => sesCardH(s, sesMeta(s), what(s))), 10)}; },
  focus(w){ if (!w.focus.length) return null; const {ws, we} = period(), ses = DATA.filter(s => inP(s, ws, we));
    const main = b => { const a = b.t, e = b.t + b.min*60; let best = null, bv = 0; // その時間にいちばん長く動いていた、同じプロジェクトのセッション
      ses.forEach(s => { if (s.project !== b.project) return; const v = s.segs.reduce((t, [x, y]) => t + Math.max(0, Math.min(y, e) - Math.max(x, a)), 0); if (v > bv){ bv = v; best = s; } }); return best; };
    const rows = w.focus.slice().sort((a,b) => b.min - a.min).map(b => { const s = main(b), when = `${md(b.t)} ${hm(b.t)}–${hm(b.t + b.min*60)} · ${esc(b.project)}`;
      return s ? sesCardH(s, when, dur(b.min)) : ""; }).filter(Boolean);
    return {big: `${w.focus.length}`, body: mDays(perDay(w, w.focus.map(b => [b.t, b.min])), {v: d => d.v, f: v => v ? dur(v) : "—"}) +
      mBars("By project", pjRows(sumBy(w.focus, b => b.project, b => b.min)), durM) + mList("Blocks, longest first", rows, 10) +
      (rows.length ? `<p class="note">Each block opens the session that ran longest in it. A block can include other sessions too.</p>` : "")}; },
  fix(w, {inRange}){ const n = s => (s.corrections || 0) + (s.interrupts || 0), xs = inRange.filter(s => n(s) > 0);
    const why = s => [s.corrections ? plural(s.corrections, "correction") : "", s.interrupts ? plural(s.interrupts, "interruption") : ""].filter(Boolean).join(" · ");
    return {big: w.fixRate == null ? "—" : `${w.fixRate}<small>%</small>`, body: `<p class="note">Of ${plural(w.prompts, "prompt")} in this ${st.mode === "month" ? "month" : "week"}.</p>` +
      mBars("By agent", agRows(sumBy(xs, s => s.source, n)), cnt) + mBars("By project", pjRows(sumBy(xs, s => s.project, n)), cnt) +
      mList("Sessions with the most", xs.sort((a,b) => n(b) - n(a)).map(s => sesCardH(s, `${sesMeta(s)} · ${plural(s.nPrompts, "prompt")}`, why(s))), 8) + (xs.length ? WHOLE() : "")}; },
  bigPrompts(w, {ws, we}){ const xs = DATA.flatMap(s => s.prompts.filter(p => p.t >= ws && p.t < we && plen(p) >= BIG_PROMPT).map(p => ({s, p})));
    if (!xs.length) return null;
    return {big: `${xs.length}`, body: mBars("By project", pjRows(countBy(xs, x => x.s.project)), cnt) +
      mList("Longest first", xs.sort((a,b) => plen(b.p) - plen(a.p)).map(({s, p}) => sesCardH(s, `${md(p.t)} ${hm(p.t)} · ${esc(s.project)} · ${esc(snipOf(p.text, 60))}`, `${commas(plen(p))} chars`)), 10)}; },
  switches(w){ const act = w.days.filter(d => d.active), sum = act.reduce((t, d) => t + d.switches, 0);
    return {big: `${w.switchesAvg}`, body: calcH([["Switches", `${sum}`], "÷", ["Days with activity", `${act.length}`], "=", ["Per day", `${w.switchesAvg}`]]) +
      mDays(w, {v: d => d.switches, f: v => plural(v, "switch", "switches")})}; },
  parallel(w, {ws, we}){ const {M} = minuteGrid(ws, we), a = new Date(ws*1000), byDay = w.days.map(() => ({v: 0})), per = new Map(); let mx = 0, at = 0;
    M.forEach((xs, m) => { const ids = new Set(xs.map(s => s.id)); if (ids.size > mx){ mx = ids.size; at = ws + m*60; }
      if (ids.size < 2) return; const d = byDay[dayNo(a, ws + m*60)]; if (d) d.v++; new Set(xs).forEach(s => per.set(s, (per.get(s) || 0) + 1)); });
    return {big: dur(w.parallel, true), body: `<p class="note">Up to ${mx} at once, first on ${md(at)} at ${hm(at)}.</p>` + mDays({days: byDay}, {v: d => d.v, f: v => v ? dur(v) : "—"}) +
      mSessions("Sessions that ran alongside others most", [...per].sort((x,y) => y[1] - x[1]).slice(0, 8), v => dur(v))}; },
  wait(w, {ws, we}){ const xs = DATA.flatMap(s => (s.waits || []).filter(x => x[0] >= ws && x[0] < we).map(x => ({s, v: x[1]}))); if (!xs.length) return null;
    const B = [[30, "Under 30s"], [60, "30s – 1 min"], [120, "1 – 2 min"], [300, "2 – 5 min"], [600, "5 – 10 min"], [Infinity, "10 min or more"]];
    const med = list => { const v = list.map(x => x.v).sort((a,b) => a - b); return v[Math.min(v.length - 1, Math.floor(v.length * 0.5))]; };
    const by = (f, mk) => { const g = new Map(); xs.forEach(x => { const k = f(x); if (!g.has(k)) g.set(k, []); g.get(k).push(x); }); return [...g].filter(([, l]) => l.length >= 3).map(([k, l]) => mk(k, med(l))); };
    const sm = {f: v => secs(Math.round(v))};
    return {big: secsH(w.waitMedian), body: `<p class="note">${plural(xs.length, "reply", "replies")} answered · 90th percentile ${secs(w.waitP90)}</p>` +
      mBars("How long you took to reply", B.map(([hi, name], i) => ({name, c: "var(--acc)", v: xs.filter(x => x.v < hi && x.v >= (i ? B[i-1][0] : 0)).length})), cnt, 0, true) +
      mBars("Median by agent", by(x => x.s.source, (k, v) => ({name: k, c: agColor(k), mark: agMark(k), v})), sm, false) +
      mBars("Median by project", by(x => x.s.project, (k, v) => ({name: k, c: projColor(k), v})), sm, false) +
      `<p class="note">Agents and projects with fewer than 3 replies are left out of the medians.</p>`}; },
  ai(w, {ws, we}){ const {M, own} = minuteGrid(ws, we), a = new Date(ws*1000), byDay = w.days.map(() => ({v: 0}));
    M.forEach((xs, m) => { const d = byDay[dayNo(a, ws + m*60)]; if (d) d.v += xs.length; });
    const ss = [...own];
    return {big: dur(w.ai, true), body: `<p class="note">${dur(w.ai)} of run time in ${dur(w.active)} of active time: about ${(w.ai / Math.max(1, w.active)).toFixed(1)} sessions running at a time.</p>` +
      mDays({days: byDay}, {v: d => d.v, f: v => v ? dur(v) : "—"}) + mBars("By agent", agRows(sumBy(ss, ([s]) => s.source, ([, v]) => v)), durM) +
      mBars("By project", pjRows(sumBy(ss, ([s]) => s.project, ([, v]) => v)), durM) + mSessions("Longest sessions", ss.sort((x,y) => y[1] - x[1]).slice(0, 5), v => dur(v))}; },
  cache(w, {inRange}){ const u = w.usage || {}; if (u.cacheHit == null) return null;
    const use = s => [s.usage, ...(s.subagents || []).map(a => a.usage)].reduce((t, x) => x ? {cr: t.cr + x.cr, all: t.all + x.cr + x.cw + x.cw1h + x.in} : t, {cr: 0, all: 0});
    const xs = inRange.map(s => [s, use(s)]).filter(([, x]) => x.all > 0), pc = {f: v => `${Math.round(v)}%`};
    const by = f => { const g = new Map(); xs.forEach(([s, x]) => { const k = f(s), y = g.get(k) || {cr: 0, all: 0}; g.set(k, {cr: y.cr + x.cr, all: y.all + x.all}); }); return [...g].map(([k, y]) => [k, y.cr*100/y.all]); };
    return {big: `${Math.round(u.cacheHit*100)}<small>%</small>`, body: mBars("By agent", agRows(by(s => s.source)), pc, false) + mBars("By project", pjRows(by(s => s.project)), pc, false) +
      mSessions("Sessions with the most input", xs.sort((p,q) => q[1].all - p[1].all).slice(0, 5), x => `${Math.round(x.cr*100/x.all)}% of ${tok(x.all)}`) + (xs.length ? WHOLE() : "")}; },
  subagents(w, {ws, we, inRange}){ const u = w.usage || {}; if (!u.subagents) return null;
    const n = s => (s.subagents || []).filter(a => a.start >= ws && a.start < we).length, xs = inRange.filter(s => n(s) > 0);
    return {big: `${u.subagents}`, body: `<p class="note">Total run time ${dur(u.subMin)}</p>` + mBars("By type", (u.subTypes || []).map(([k, v]) => ({name: k, c: "var(--acc)", v})), cnt) +
      mBars("By agent", agRows(sumBy(xs, s => s.source, n)), cnt) + mBars("By project", pjRows(sumBy(xs, s => s.project, n)), cnt) +
      mSessions("Sessions with the most", xs.map(s => [s, n(s)]).sort((a,b) => b[1] - a[1]).slice(0, 5), v => plural(v, "subagent"))}; },
  costPerAsk(w, {ws, we, inRange}){ const u = w.usage || {}; if (w.costPerAsk == null) return null;
    const tk = inRange.filter(s => sesTok(s) > 0), np = s => s.prompts.filter(p => p.t >= ws && p.t < we).length; // トークンを記録するセッションの、期間の中のプロンプト（Go と同じ分母）
    const rate = (costs, f) => { const P = new Map(sumBy(tk, f, np)); return costs.filter(([k, c]) => c > 0 && P.get(k) > 0).map(([k, c]) => [k, c / P.get(k)]); };
    const top = tk.filter(s => s.nPrompts > 0 && s.cost > 0).map(s => [s, s.cost / s.nPrompts]).sort((a,b) => b[1] - a[1]).slice(0, 5), um = {f: v => usd(v)};
    return {big: usdH(w.costPerAsk), body: calcH([["Estimated cost", esc(usd(u.cost))], "÷", ["Prompts (sessions that record tokens)", `${w.costPrompts}`], "=", ["Per prompt", esc(usd(w.costPerAsk))]]) +
      mBars("By agent", agRows(rate(((w.shares || {}).source || []).map(x => [x.key, x.cost]), s => s.source)), um, false) +
      mBars("By project", pjRows(rate((w.projectStats || []).map(p => [p.project, p.cost]), s => s.project)), um, false) +
      mSessions("Sessions with the highest cost per prompt", top, usd) + (top.length ? WHOLE() : "")}; },
};
function openMetric(id, from){
  const {S:w, ws, we} = period(), m = BRK[id]; if (!w || !m) return;
  const D = $("#md"), u = w.usage || {}, h = H()[m.h], inRange = DATA.filter(s => inP(s, ws, we));
  const agents = ((w.shares || {}).source || []).map(x => ({name: x.key, c: agColor(x.key), mark: agMark(x.key), v: m.v ? m.v(x) || 0 : 0}));
  const projs = (w.projectStats || []).map(p => ({name: p.project, c: projColor(p.project), v: m.v ? m.v(p) || 0 : 0}));
  let big = "", body = "";
  if (MB[id]){ const r = MB[id](w, {ws, we, inRange}); if (!r) return; ({big, body} = r); }
  else if (m.proj){ // 月末の見込み：今日までの実績 ÷ 日数 × 月の日数
    const pj = projection(w), so = m.proj === "cost" ? costOf(u) : u.credits; if (!pj || so == null) return;
    const val = m.proj === "cost" ? pj.cost : pj.credits, nd = w.days.length, avg = so / pj.days;
    big = `≈ ${m.f(val)}`;
    body = `<div class="mcalc"><div><small>So far</small><b>${esc(m.f(so))}</b></div><span>÷</span><div><small>Days so far</small><b>${pj.days}</b></div><span>×</span><div><small>Days in the month</small><b>${nd}</b></div><span>=</span><div><small>Month-end</small><b>≈ ${esc(m.f(val))}</b></div></div>
      <p class="note">About ${esc(m.f(avg))} a day. The faded bars are the days still to come at that pace.</p>` + mDays(w, m, {avg}) + mBars("So far, by agent", agents, m) + mBars("So far, by project", projs, m);
  } else if (id === "limits" || id === "compactions"){ // 起きた時刻の一覧：エージェントごと・プロジェクトごとと、いつどのセッションで
    const L = id === "limits", H2 = L ? limitHits(ws, we) : compactionsOf(ws, we); big = `${H2.length}`;
    const by = (key, mk) => countBy(H2, x => x.s[key]).map(([k, v]) => mk(k, v));
    const cm = {f: v => `${v}`};
    body = mBars("By agent", by("source", (k, v) => ({name: k, c: agColor(k), mark: agMark(k), v})), cm) +
      mBars("By project", by("project", (k, v) => ({name: k, c: projColor(k), v})), cm) +
      `<h3>When</h3><div class="msess">${H2.map(x => sesCardH(x.s, `${md(x.t)} ${hm(x.t)} · ${esc(x.s.project)}${x.r ? ` · resets ${esc(x.r)}` : ""}`)).join("")}</div>`;
  } else {
    const tot = id === "tokens" ? u.tokens : id === "cost" ? costOf(u) : id === "credits" ? u.credits : id === "active" || id === "days" ? w.active : w.prompts;
    big = id === "days" ? `${w.days.filter(d => d.active).length}<small> of ${w.days.length}</small>` : id === "sessions" ? `${w.sessions}<small>/</small>${w.prompts}` : id === "cost" ? usdH(tot) : esc(m.f(tot || 0));
    body = mDays(w, m);
    if (id === "sessions"){ // セッション数はエージェント別の集計にないので、期間にかかったセッションから数える
      const by = new Map(); inRange.forEach(s => by.set(s.source, (by.get(s.source) || 0) + (s.nPrompts || 0)));
      body += mBars("Prompts by agent", [...by].map(([k, v]) => ({name: k, c: agColor(k), mark: agMark(k), v})), m) + mBars("Prompts by project", projs, m);
    } else if (id !== "days"){
      body += mBars("By agent", agents, m) + mBars("By project", projs, m);
      if (m.model) body += mBars("By model", (u.models || []).map(r => ({name: r[0], c: "var(--acc)", v: r[m.model] || 0})), m);
    }
    if (m.ses){ const top = inRange.map(s => [s, m.ses(s) || 0]).filter(x => x[1] > 0).sort((a,b) => b[1] - a[1]).slice(0, 5);
      body += mSessions(id === "active" ? "Longest sessions" : id === "sessions" ? "Sessions with the most prompts" : "Top sessions", top, m.f);
      if (top.length && id !== "active") body += `<p class="note">Session figures cover the whole session, including any part outside this ${st.mode === "month" ? "month" : "week"}.</p>`; }
  }
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" aria-label="Close"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form>
    <div class="eyebrow">Breakdown · ${st.mode === "month" ? `${MONTH[st.month.getMonth()]} ${st.month.getFullYear()}` : `${dMD(st.week)} – ${dMD(addDays(st.week, 6))}`}</div><h2 id="mdh">${esc(m.n)}</h2><div class="mbig">${big}</div>${h ? `<p class="mdef" id="mdd">${esc(h.d)}</p>` : ""}${body}
    ${h ? `<p class="note mnot"><b>Doesn't tell you:</b> ${esc(h.x)}</p>` : ""}`; // 定義は数字のすぐ下に（読む前に何を数えたかがわかる）、言えないことは読み終えたあとに
  if (h) D.setAttribute("aria-describedby", "mdd"); else D.removeAttribute("aria-describedby");
  D.querySelectorAll("[data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
  ["git", "push", "pr"].forEach(k => D.querySelectorAll(`[data-${k}]`).forEach(b => b.onclick = () => { D.close(); select(`${k}:${b.dataset[k]}`); })); // コミット・push・PR の一覧
  D.onclose = () => { if (from && from.isConnected && !st.sel) from.focus(); };
  D.showModal(); D.scrollTop = 0;
  const hd = $("#mdh"); hd.tabIndex = -1; hd.focus();
}
