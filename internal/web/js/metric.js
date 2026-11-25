// 上の帯とサマリーの数字を押したときの内訳（日ごと・エージェントごと・プロジェクトごと・モデルごと・セッション）
/* ── 数字の内訳 ──
   上の帯とサマリーの数字（トークン・目安コスト・月末の推定・クレジット・作業時間など）を押すと、画面の真ん中のダイアログで内訳を出す。
   日ごと・エージェントごと・プロジェクトごと・モデルごとは期間の集計（Go）をそのまま使うので、合計と一致する。
   セッションの一覧だけは、期間にかかったセッションの、セッション全体の値（期間の外の分も入る） */
const BRK = {
  tokens: {n: "Tokens", h: "tokens", v: x => x.tokens, f: v => tok(v), ses: s => sesTok(s), model: 2},
  cost: {n: "Estimated cost", h: "cost", v: x => x.cost, f: v => usd(v), ses: s => s.cost, model: 1},
  projection: {n: "Month-end cost (estimate)", h: "projection", v: x => x.cost, f: v => usd(v), proj: "cost"},
  credits: {n: "Kiro credits", h: "credits", v: x => x.credits, f: cr, ses: s => s.credits},
  projectionCr: {n: "Month-end credits (estimate)", h: "projectionCr", v: x => x.credits, f: cr, proj: "credits"},
  pace: {n: "A day to match the last period", h: "pace", v: x => x.cost, f: v => usd(v)},
  paceCr: {n: "Credits a day to match the last period", h: "paceCr", v: x => x.credits, f: cr},
  active: {n: "Active time", h: "active", v: x => x.minutes ?? x.active, f: v => dur(v), ses: s => sesMin(s)},
  sessions: {n: "Sessions / prompts", h: "prompts", v: x => x.prompts, f: v => `${Math.round(v)}`, ses: s => s.nPrompts},
  limits: {n: "Usage limit hits", h: "limits"},
  compactions: {n: "Compactions", h: "compactions"},
  commits: {n: "Git commits", h: "gitCommits", v: x => x.commits, f: v => `${Math.round(v)}`},
  // ここから下は、期間の集計（Go）に日ごと・エージェントごとの値がない数。MB の関数で、履歴やコミットから組み立てる
  aiCommits: {n: "AI commits", h: "commits"},
  lines: {n: "Lines in Git commits", h: "lines"},
  prs: {n: "Pull requests", h: "prs"},
  costPerCommit: {n: "Claude Code cost per AI commit", h: "costPerCommit"},
  outSessions: {n: "Claude Code sessions that reached a commit or PR", h: "outSessions"},
  focus: {n: "Focus blocks (60+ min)", h: "focus"},
  fix: {n: "Prompts with corrections or interruptions", h: "fix"},
  bigPrompts: {n: "Oversized prompts", h: "bigPrompts"},
  switches: {n: "Project switches per day", h: "switches"},
  wait: {n: "Wait time (median)", h: "wait"},
  cache: {n: "Read from cache", h: "cache"},
  subagents: {n: "Subagents", h: "subagents"},
  costPerAsk: {n: "Estimated cost per prompt", h: "costPerAsk"},
};
// 内訳の大きな数字は、見出しの名前に「クレジット」とあるので単位を付けない（「Kiro クレジット 44」）
const bigF = m => m.f === cr ? crN : m.f;
const sesTok = s => [s.usage, ...(s.subagents || []).map(a => a.usage)].reduce((t,u) => t + (u ? u.in+u.out+u.cw+u.cw1h+u.cr : 0), 0);
function sesMin(s){ const {ws, we} = period(); return s.segs.reduce((t, [a, b]) => t + Math.max(0, Math.min(b, we) - Math.max(a, ws))/60, 0); } // 期間の中で作業していた分
// 名前ごとの件数。履歴の名前（プロジェクト）を {} のキーにすると、"__proto__" や "constructor" が数えられないので Map で数える
const countBy = (list, f) => [...list.reduce((m, x) => m.set(f(x), (m.get(f(x)) || 0) + 1), new Map())];
const sumBy = (list, f, v) => [...list.reduce((m, x) => m.set(f(x), (m.get(f(x)) || 0) + v(x)), new Map())]; // 名前ごとの合計（countBy と同じく Map で）
const pcOf = (v, t) => t > 0 ? Math.round(v*100/t) : 0;
// 日ごとの柱。見込み（proj）があれば、まだ来ていない日に 1 日あたりの平均を薄く積む
function mDays(w, m, proj){
  const {ws} = period(), a = new Date(ws*1000), M = st.mode === "month", vals = w.days.map(d => m.v(d) || 0);
  const now = nowMs()/1000, cur = dayNo(a, now), avg = proj ? proj.avg : 0, lines = (proj && proj.lines) || [];
  const tr = proj ? trend(vals.slice(0, cur)) : null, trAt = i => tr ? Math.max(0, tr.a + tr.b * i) : 0; // 終わった日（今日は途中なので入れない）の傾き
  const max = Math.max(...vals, avg, ...lines.map(l => l.hi), trAt(0), trAt(w.days.length - 1), 1e-9);
  const cols = w.days.map((d, i) => { const dt = addDays(a, i), fut = proj && i > cur, v = fut ? avg : vals[i];
    const tip = `${md(dt.getTime()/1000)}: ${fut ? t(proj.tip || "≈ {v} at the current pace", {v: m.f(avg)}) : m.f(vals[i])}`;
    const lab = !M && !fut && v > 0; // 週は 7 本なので、棒の上に値を書く（月は細すぎるので、ポインターを合わせたときだけ）
    return `<span class="mcol${fut ? " fut" : ""}${i === cur ? " now" : ""}" title="${esc(tip)}">${lab ? `<span class="bv">${esc(m.f(v))}</span>` : ""}<span class="b" style="height:${v ? `calc((${M ? "100%" : "100% - 16px"}) * ${Math.max(0.02, v/max).toFixed(4)})` : 0}"></span><small>${M ? dt.getDate() : DOW[dt.getDay()]}</small></span>`; }).join("");
  // 月末の推定の幅（3σ だけ）を、これからの日の 1 日あたりの量にして、点線で重ねる
  const from = ((cur + 1) / w.days.length * 100).toFixed(2);
  const band = lines.map(l => [l.hi, l.lo].filter(v => v > 0).map(v => `<i class="mbl k${l.k}" style="left:${from}%;--y:${(v/max*100).toFixed(2)}%" title="${esc(`${l.k}σ: ${rangeT(m.f(l.lo), m.f(l.hi))} ${t("a day")}`)}"></i>`).join("")).join("");
  // トレンド線：終わった日に引いた直線（最小二乗法）を、月の最後の日まで延ばす。これからの日の部分は破線
  const n = w.days.length, X = i => ((i + 0.5) / n * 100).toFixed(2), Y = i => (100 - trAt(i) / max * 100).toFixed(2);
  const trSvg = tr ? `<svg class="mtrend" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true"><line x1="${X(0)}" y1="${Y(0)}" x2="${X(cur)}" y2="${Y(cur)}"/><line class="fut" x1="${X(cur)}" y1="${Y(cur)}" x2="${X(n - 1)}" y2="${Y(n - 1)}"/></svg>` : "";
  const key = lines.length || tr ? `<p class="note mbkey">${proj ? `<span class="pace">${esc(t(proj.label || "Current pace"))}</span>` : ""}${tr ? `<span class="tr">${esc(t("Trend"))}</span>` : ""}${lines.map(l => `<span class="k${l.k}">${l.k}σ</span>`).join("")} ${esc(lines.length ? t("σ lines: the range per day for the days still to come (the table above is for the whole month)") : "")}</p>` : "";
  return `<h3>${esc(t("By day"))}</h3><div class="mdays${M ? " month" : " wk"}"${proj ? ` style="--avg:${avg/max*100}%"` : ""}>${cols}${band}${trSvg}</div>${key}`;
}
// 横の棒の一覧（エージェント・プロジェクト・モデル）。total が false なら割合の欄を空ける（割合や中央値のように、足しても意味のない値）。
// keep は並べ替えない（待ち時間の幅のように、順に意味がある）
function mBars(title, rows, m, total, keep){
  rows = rows.filter(r => r.v > 0); if (!keep) rows.sort((a,b) => b.v - a.v); if (!rows.length) return "";
  const pcs = total !== false; total = pcs ? total || rows.reduce((t,r) => t + r.v, 0) : 0; const max = Math.max(...rows.map(r => r.v));
  return `<h3>${esc(t(title))}</h3><div class="mbars">${rows.slice(0, 8).map(r => `<div class="mbar" style="--c:${r.c}"><span class="nm" title="${esc(r.name)}">${r.mark || ""}${esc(r.name)}</span><span class="tr"><span style="width:${r.v/max*100}%"></span></span><span class="vv">${esc(m.f(r.v))}</span><span class="pc">${pcs ? `${pcOf(r.v, total)}%` : ""}</span></div>`).join("")}${rows.length > 8 ? `<p class="note">${esc(t("{n} more not shown", {n: rows.length - 8}))}</p>` : ""}</div>`;
}
// 名前と値の組（sumBy・countBy）を、エージェントかプロジェクトの棒にする
const agRows = pairs => pairs.map(([k, v]) => ({name: k, c: agColor(k), mark: agMark(k), v}));
const pjRows = pairs => pairs.map(([k, v]) => ({name: k, c: projColor(k), v}));
// 一覧（HTML の行）を n 件まで。残りは件数だけ
const mList = (title, rows, n = 10) => rows.length ? `<h3>${esc(t(title))}</h3><div class="msess">${rows.slice(0, n).join("")}</div>${rows.length > n ? `<p class="note">${esc(t("{n} more not shown", {n: rows.length - n}))}</p>` : ""}` : "";
// 時刻つきの値 [t, v] を期間の日ごとに足す（mDays に渡す形）
function perDay(w, items){ const {ws} = period(), a = new Date(ws*1000), d = w.days.map(() => ({v: 0}));
  items.forEach(([t, v]) => { const i = dayNo(a, t); if (d[i]) d[i].v += v; }); return {days: d}; }
const WHOLE = () => `<p class="note">${esc(st.mode === "month" ? t("Session figures cover the whole session, including any part outside this month.") : t("Session figures cover the whole session, including any part outside this week."))}</p>`;
function mSessions(title, list, f){
  if (!list.length) return "";
  return `<h3>${esc(t(title))}</h3><div class="msess">${list.map(([s, v]) => sesCardH(s, sesMeta(s), f(v))).join("")}</div>`;
}
const projColor = p => st.colorBy === "project" ? colorOf(p) : "var(--acc)";
/* 期間の集計（Go）に内訳のない数の内訳。どれも {big, body} を返す（出せないときは null）。
   日ごとの値は、Go と同じ数え方（1 分刻み・記録された時刻の日）で履歴やコミットから数え直すので、合計と一致する。
   セッションの値（WHOLE を添えるもの）だけは、セッション全体の値 */
const cnt = {f: v => `${v}`}, durM = {f: v => dur(v)};
const prRow = ({s, r}) => `<button class="gc-row" data-pr="${esc(prKey(s, r))}"><time>${md(r.t)} ${hm(r.t)}</time><span><i class="gtag">${ico("pr")}${esc(r.url ? prName(r.url) : t("Pull request"))}</i> ${esc(s.title)}</span><b></b></button>`;
const gitIn = (ws, we) => (META.git || []).filter(c => c.t >= ws && c.t < we);
const calcH = parts => `<div class="mcalc">${parts.map(p => typeof p === "string" ? `<span>${p}</span>` : `<div><small>${esc(t(p[0]))}</small><b>${p[1]}</b></div>`).join("")}</div>`; // [名前, 値（HTML）] と記号（÷ =）
// 前の期間並みに収める 1 日の目安（pacing）：前の期間の合計 − ここまで = 残り、÷ 残りの日。
// 費用だけでなく、前の期間とここまでに残したもの（コミット）も並べる。費用が同じでも、残したものが減っていれば気づけるように
function paceBody(w, k, m, brief){
  const pc = pacing(w, period().P), x = pc && pc[k]; if (!x) return null; const M = st.mode === "month", on = x.rest >= 0, C = k === "credits";
  // 見出しは上の帯と同じ言葉（超えたあとは「超過」。超えた分を 1 日の目安と読まれないように）
  const title = on ? (C ? t(M ? "Credits a day to match last month" : "Credits a day to match last week") : t(M ? "A day to match last month" : "A day to match last week"))
    : (C ? t(M ? "Over last month's credits" : "Over last week's credits") : t(M ? "Over last month's total" : "Over last week's total"));
  const big = on ? esc(bigF(m)(x.per)) : `+${esc(bigF(m)(-x.rest))}`, last = M ? "Last month" : "Last week";
  const calc = on ? calcH([[last, esc(m.f(x.ref))], "−", ["So far", esc(m.f(x.so))], "=", ["Left", esc(m.f(x.rest))], "÷", ["Days left", `${pc.left}`], "=", ["A day", esc(bigF(m)(x.per))]])
    : calcH([["So far", esc(m.f(x.so))], "−", [last, esc(m.f(x.ref))], "=", ["Over", esc(bigF(m)(-x.rest))]]);
  const note = on ? t("If you keep to about {v} a day, this period ends near the last one. What you use for the rest of today comes out of the days ahead. The faded bars are the days still to come at that amount.", {v: m.f(x.per)}) : t("This period has already passed the last one's total by {v}.", {v: m.f(-x.rest)});
  // 残したもの：途中の期間は、前の期間の同じ日までとも比べる（途中の値を前の期間まるごととだけ並べない）
  const pw = period().P, n = soFar(), gc = g => g && g.git ? g.git.commits : null, a = gc(pw), b = gc(w);
  const same = a == null || n == null ? null : pw.days.slice(0, Math.min(n, pw.days.length)).reduce((s, d) => s + (d.commits || 0), 0);
  const out = a == null || b == null ? "" : `<p class="note">${esc(t(M ? "Left behind: {a} commits last month ({s} by the same day), {b} so far this month." : "Left behind: {a} commits last week ({s} by the same day), {b} so far this week.", {a, s: same ?? "—", b}))}</p>`;
  if (brief) return {title, body: calc + `<p class="note">${esc(note)}</p>`};
  return {title, big, body: calc + `<p class="note">${esc(note)}</p>` + out + mDays(w, m, x.rest > 0 ? {avg: x.per, tip: "≈ {v} to match the last period", label: "Pace to match the last period"} : null)};
}
const MB = {
  pace(w){ const r = paceBody(w, "cost", BRK.pace), c = r && paceBody(w, "credits", BRK.paceCr, true); // 帯は 1 つの枠なので、クレジットもここに
    if (r && c) r.body += `<h3>${esc(t("Kiro credits"))}</h3>` + c.body; return r; },
  paceCr(w){ return paceBody(w, "credits", BRK.paceCr); },
  commits(w, {ws, we}){ // Git commits：日ごと・プロジェクトごと・AI のエージェントごと・だれが・一覧
    const cs = gitIn(ws, we).sort((a,b) => b.t - a.t), g = w.git || {}; if (!cs.length) return null;
    const ag = cs.filter(c => c.ai).map(c => (DATA.find(s => s.id === c.session) || {}).source || t("Unknown agent"));
    return {big: `${g.commits || cs.length}`, body: mDays(w, BRK.commits) + mBars("By project", pjRows(countBy(cs, c => c.project)), cnt) +
      mBars("By AI or by hand", [{name: t("By AI"), c: "var(--acc)", v: cs.filter(c => c.ai).length}, {name: t("By hand (or another tool)"), c: "var(--other)", v: cs.filter(c => !c.ai).length}], cnt) +
      mBars("AI commits by agent", agRows(countBy(ag, x => x)), cnt) +
      mList("Commits", cs.map(gitRow), 15)}; },
  aiCommits(w, {inRange}){ // git を読めなかったときの AI のコミット（履歴の outputs）
    const xs = inRange.filter(s => s.outputs && s.outputs.commits > 0);
    return {big: `${(w.outputs || {}).commits || 0}`, body: mBars("By agent", agRows(sumBy(xs, s => s.source, s => s.outputs.commits)), cnt) + mBars("By project", pjRows(sumBy(xs, s => s.project, s => s.outputs.commits)), cnt) +
      mSessions("Sessions with the most commits", xs.map(s => [s, s.outputs.commits]).sort((a,b) => b[1] - a[1]).slice(0, 8), v => plural(v, "commit")) + (xs.length ? WHOLE() : "")}; },
  lines(w, {ws, we}){ const cs = gitIn(ws, we), L = c => c.added + c.removed, g = w.git || {}; if (!cs.length) return null;
    const m = {f: v => commas(v)};
    return {big: commas(g.added + g.removed), body: `<p class="note">${esc(t("+{a} added · −{r} removed", {a: commas(g.added), r: commas(g.removed)})).replace(`+${commas(g.added)}`, `<span class="la">+${commas(g.added)}</span>`).replace(`−${commas(g.removed)}`, `<span class="lr">−${commas(g.removed)}</span>`)}</p>` + mDays(perDay(w, cs.map(c => [c.t, L(c)])), {v: d => d.v, f: v => commas(v)}) +
      mBars("By project", pjRows(sumBy(cs, c => c.project, L)), m) +
      mList("Largest commits", cs.filter(c => L(c) > 0).sort((a,b) => L(b) - L(a)).map(gitRow), 8)}; },
  prs(w, {ws, we}){ const xs = DATA.flatMap(s => (s.prAt || []).filter(r => r.t >= ws && r.t < we).map(r => ({s, r}))).sort((a,b) => b.r.t - a.r.t);
    return {big: `${(w.outputs || {}).prs || xs.length}`, body: mBars("By agent", agRows(countBy(xs, x => x.s.source)), cnt) + mBars("By project", pjRows(countBy(xs, x => x.s.project)), cnt) + mList("Pull requests", xs.map(prRow), 15)}; },
  costPerCommit(w, {inRange}){ const o = w.outputs || {}; if (w.costPerCommit == null || !o.commits) return null;
    const xs = inRange.filter(s => s.outputs && s.outputs.commits > 0 && s.cost > 0).map(s => [s, s.cost / s.outputs.commits]).sort((a,b) => b[1] - a[1]);
    return {big: usdH(w.costPerCommit), body: calcH([["Claude Code's estimated cost", `${esc(usd(w.outCost ?? w.costPerCommit * o.commits))}`], "÷", ["AI commits", `${o.commits}`], "=", ["Per commit", esc(usd(w.costPerCommit))]]) +
      mSessions("Sessions with the highest cost per commit", xs.slice(0, 8), usd) + (xs.length ? WHOLE() : "")}; },
  outSessions(w, {inRange}){ const base = w.outBase ?? w.sessions; if (!base) return null;
    const xs = inRange.filter(s => s.outputs && (s.outputs.commits || s.outputs.prs)).sort((a,b) => b.start - a.start);
    const what = s => [s.outputs.commits ? plural(s.outputs.commits, "commit") : "", s.outputs.prs ? plural(s.outputs.prs, "PR") : ""].filter(Boolean).join(" · ");
    return {big: `${Math.round(w.outSessions*100/base)}<small>%</small>`, body: calcH([["Reached a commit or PR", `${w.outSessions}`], "÷", ["Claude Code sessions", `${base}`]]) +
      mBars("By project", pjRows(countBy(xs, s => s.project)), cnt) + mList("Claude Code sessions that reached a commit or PR", xs.map(s => sesCardH(s, sesMeta(s), what(s))), 10)}; },
  focus(w){ if (!w.focus.length) return null; const {ws, we} = period(), ses = DATA.filter(s => inP(s, ws, we));
    const main = b => { const a = b.t, e = b.t + b.min*60; let best = null, bv = 0; // その時間にいちばん長く動いていた、同じプロジェクトのセッション
      ses.forEach(s => { if (s.project !== b.project) return; const v = s.segs.reduce((t, [x, y]) => t + Math.max(0, Math.min(y, e) - Math.max(x, a)), 0); if (v > bv){ bv = v; best = s; } }); return best; };
    const rows = w.focus.slice().sort((a,b) => b.min - a.min).map(b => { const s = main(b), when = `${md(b.t)} ${hm(b.t)}–${hm(b.t + b.min*60)} · ${esc(b.project)}`;
      return s ? sesCardH(s, when, dur(b.min)) : ""; }).filter(Boolean);
    return {big: `${w.focus.length}`, body: mDays(perDay(w, w.focus.map(b => [b.t, b.min])), {v: d => d.v, f: v => v ? dur(v) : "—"}) +
      mBars("By project", pjRows(sumBy(w.focus, b => b.project, b => b.min)), durM) + mList("Blocks, longest first", rows, 10) +
      (rows.length ? `<p class="note">${esc(t("Each block opens the session that ran longest in it. A block can include other sessions too."))}</p>` : "")}; },
  fix(w, {inRange}){ const n = s => (s.corrections || 0) + (s.interrupts || 0), xs = inRange.filter(s => n(s) > 0);
    const why = s => [s.corrections ? plural(s.corrections, "correction") : "", s.interrupts ? plural(s.interrupts, "interruption") : ""].filter(Boolean).join(" · ");
    return {big: w.fixRate == null ? "—" : `${w.fixRate}<small>%</small>`, body: `<p class="note">${esc(t(w.prompts === 1 ? (st.mode === "month" ? "Of {n} prompt in this month." : "Of {n} prompt in this week.") : (st.mode === "month" ? "Of {n} prompts in this month." : "Of {n} prompts in this week."), {n: w.prompts}))}</p>` +
      mBars("By agent", agRows(sumBy(xs, s => s.source, n)), cnt) + mBars("By project", pjRows(sumBy(xs, s => s.project, n)), cnt) +
      mList("Sessions with the most corrections or interruptions", xs.sort((a,b) => n(b) - n(a)).map(s => sesCardH(s, `${sesMeta(s)} · ${plural(s.nPrompts, "prompt")}`, why(s))), 8) + (xs.length ? WHOLE() : "")}; },
  bigPrompts(w, {ws, we}){ const xs = DATA.flatMap(s => s.prompts.filter(p => p.t >= ws && p.t < we && plen(p) >= BIG_PROMPT).map(p => ({s, p})));
    if (!xs.length) return null;
    return {big: `${xs.length}`, body: mBars("By project", pjRows(countBy(xs, x => x.s.project)), cnt) +
      mList("Longest prompts", xs.sort((a,b) => plen(b.p) - plen(a.p)).map(({s, p}) => sesCardH(s, `${md(p.t)} ${hm(p.t)} · ${esc(s.project)} · ${esc(snipOf(p.text, 60))}`, t("{n} chars", {n: commas(plen(p))}))), 10)}; },
  switches(w){ const act = w.days.filter(d => d.active), sum = act.reduce((t, d) => t + d.switches, 0);
    return {big: `${w.switchesAvg}`, body: calcH([["Switches", `${sum}`], "÷", ["Days with activity", `${act.length}`], "=", ["Per day", `${w.switchesAvg}`]]) +
      mDays(w, {v: d => d.switches, f: v => plural(v, "switch", "switches")})}; },
  wait(w, {ws, we}){ const xs = DATA.flatMap(s => (s.waits || []).filter(x => x[0] >= ws && x[0] < we).map(x => ({s, v: x[1]}))); if (!xs.length) return null;
    const B = [[30, "Under 30s"], [60, "30s – 1 min"], [120, "1 – 2 min"], [300, "2 – 5 min"], [600, "5 – 10 min"], [Infinity, "10 min or more"]];
    const med = list => { const v = list.map(x => x.v).sort((a,b) => a - b); return v[Math.min(v.length - 1, Math.floor(v.length * 0.5))]; };
    const by = (f, mk) => { const g = new Map(); xs.forEach(x => { const k = f(x); if (!g.has(k)) g.set(k, []); g.get(k).push(x); }); return [...g].filter(([, l]) => l.length >= 3).map(([k, l]) => mk(k, med(l))); };
    const sm = {f: v => secs(Math.round(v))};
    return {big: secsH(w.waitMedian), body: `<p class="note">${esc(t(xs.length === 1 ? "{n} reply answered · 90th percentile {p}" : "{n} replies answered · 90th percentile {p}", {n: xs.length, p: secs(w.waitP90)}))}</p>` +
      mBars("How long you took to reply", B.map(([hi, name], i) => ({name: t(name), c: "var(--acc)", v: xs.filter(x => x.v < hi && x.v >= (i ? B[i-1][0] : 0)).length})), cnt, 0, true) +
      mBars("Median by agent", by(x => x.s.source, (k, v) => ({name: k, c: agColor(k), mark: agMark(k), v})), sm, false) +
      mBars("Median by project", by(x => x.s.project, (k, v) => ({name: k, c: projColor(k), v})), sm, false) +
      `<p class="note">${esc(t("Agents and projects with fewer than 3 replies are left out of the medians."))}</p>`}; },
  cache(w, {inRange}){ const u = w.usage || {}; if (u.cacheHit == null) return null;
    const use = s => [s.usage, ...(s.subagents || []).map(a => a.usage)].reduce((t, x) => x ? {cr: t.cr + x.cr, all: t.all + x.cr + x.cw + x.cw1h + x.in} : t, {cr: 0, all: 0});
    const xs = inRange.map(s => [s, use(s)]).filter(([, x]) => x.all > 0), pc = {f: v => `${Math.round(v)}%`};
    const by = f => { const g = new Map(); xs.forEach(([s, x]) => { const k = f(s), y = g.get(k) || {cr: 0, all: 0}; g.set(k, {cr: y.cr + x.cr, all: y.all + x.all}); }); return [...g].map(([k, y]) => [k, y.cr*100/y.all]); };
    return {big: `${Math.round(u.cacheHit*100)}<small>%</small>`, body: mBars("By agent", agRows(by(s => s.source)), pc, false) + mBars("By project", pjRows(by(s => s.project)), pc, false) +
      mSessions("Sessions with the most input", xs.sort((p,q) => q[1].all - p[1].all).slice(0, 5), x => t("{pc}% of {tok} tokens from cache", {pc: Math.round(x.cr*100/x.all), tok: tok(x.all)})) + (xs.length ? WHOLE() : "")}; },
  subagents(w, {ws, we, inRange}){ const u = w.usage || {}; if (!u.subagents) return null;
    const n = s => (s.subagents || []).filter(a => a.start >= ws && a.start < we).length, xs = inRange.filter(s => n(s) > 0);
    return {big: `${u.subagents}`, body: `<p class="note">${esc(t("Total run time {d}", {d: dur(u.subMin)}))}</p>` + mBars("By type", (u.subTypes || []).map(([k, v]) => ({name: k, c: "var(--acc)", v})), cnt) +
      mBars("By agent", agRows(sumBy(xs, s => s.source, n)), cnt) + mBars("By project", pjRows(sumBy(xs, s => s.project, n)), cnt) +
      mSessions("Sessions with the most subagents", xs.map(s => [s, n(s)]).sort((a,b) => b[1] - a[1]).slice(0, 5), v => plural(v, "subagent"))}; },
  costPerAsk(w, {ws, we, inRange}){ const u = w.usage || {}; if (w.costPerAsk == null) return null;
    const tk = inRange.filter(s => sesTok(s) > 0), np = s => s.prompts.filter(p => p.t >= ws && p.t < we).length; // トークンを記録するセッションの、期間の中のプロンプト（Go と同じ分母）
    const rate = (costs, f) => { const P = new Map(sumBy(tk, f, np)); return costs.filter(([k, c]) => c > 0 && P.get(k) > 0).map(([k, c]) => [k, c / P.get(k)]); };
    const top = tk.filter(s => s.nPrompts > 0 && s.cost > 0).map(s => [s, s.cost / s.nPrompts]).sort((a,b) => b[1] - a[1]).slice(0, 5), um = {f: v => usd(v)};
    return {big: usdH(w.costPerAsk), body: calcH([["Estimated cost", esc(usd(u.cost))], "÷", ["Prompts (sessions that record tokens)", `${w.costPrompts}`], "=", ["Per prompt", esc(usd(w.costPerAsk))]]) +
      mBars("By agent", agRows(rate(((w.shares || {}).source || []).map(x => [x.key, x.cost]), s => s.source)), um, false) +
      mBars("By project", pjRows(rate((w.projectStats || []).map(p => [p.project, p.cost]), s => s.project)), um, false) +
      mSessions("Sessions with the highest cost per prompt", top, usd) + (top.length ? WHOLE() : "")}; },
};
/* トレンド線のまま進んだときの月末（終わった日の傾きを、残りの日に延ばして足す。マイナスにはしない） */
function trendNote(w, pj, so, m){
  const tr = trend(w.days.slice(0, pj.days - 1).map(d => (d && m.v(d)) || 0)); if (!tr) return "";
  let rest = 0; for (let i = pj.days; i < w.days.length; i++) rest += Math.max(0, tr.a + tr.b * i);
  const td = Math.max(0, tr.a + tr.b * (pj.days - 1)); // 今日は途中なので、今日の分も傾きから（使った分より少なければ使った分）
  const v = so + rest + Math.max(0, td - ((w.days[pj.days - 1] && m.v(w.days[pj.days - 1])) || 0));
  return `<p class="note">${esc(t("If the trend line continues: ≈ {v} (the slope of the finished days, {d} a day per day)", {v: m.f(v), d: (tr.b >= 0 ? "+" : "−") + m.f(Math.abs(tr.b))}))}</p>`; }
/* 範囲の書き方（日本語は「〜」、英語は「 – 」。幅の表・点線の説明・帯のツールチップでそろえる） */
const rangeT = (lo, hi) => isJA() ? `${lo}〜${hi}` : `${lo} – ${hi}`;
/* 日ごとの値の直線の当てはめ（最小二乗法）。i 日目 ≈ a + b·i。2 日未満なら null */
function trend(ys){ const n = ys.length; if (n < 2) return null;
  const mx = (n - 1) / 2, my = ys.reduce((s, y) => s + y, 0) / n;
  let sxy = 0, sxx = 0; ys.forEach((y, i) => { sxy += (i - mx) * (y - my); sxx += (i - mx) ** 2; });
  const b = sxy / sxx; return {a: my - b * mx, b}; }
/* 月末の推定の幅：3σ（おおよそ 99.7%）だけ（1σ・2σ まで並べると読み取りにくい）。費用の推定には、トークン数の幅も添える */
function bandTable(pj, m){
  const rows = (b, f) => b.filter(x => x.k === 3).map(x => `<tr><th>${x.k}σ <small>${["≈68%", "≈95%", "≈99.7%"][x.k - 1]}</small></th><td>${esc(rangeT(f(x.lo), f(x.hi)))}</td></tr>`).join("");
  const b = pj.band[m.proj]; if (!b) return "";
  const tk = m.proj === "cost" && pj.band.tokens ? `<tr><th colspan="2">${esc(t("Tokens"))} ≈ ${esc(tok(pj.tokens))}</th></tr>${rows(pj.band.tokens, tok)}` : "";
  return `<h3>${esc(t("Range"))}</h3><table class="mband"><tbody>${rows(b, m.f)}${tk}</tbody></table>
    <p class="note">${esc(t("How far the month-end could land from the estimate, from how much the finished days varied. Days are not independent or normally distributed, so take it as a rough guide; the low end never goes below what is already used."))}</p>`; }
function openMetric(id, from){
  // monthCr：週の表示から、今月のクレジットの月末の推定を開く（内訳は月の日ごとの柱なので、そのあいだだけ今月の月表示として作る）
  if (id === "monthCr"){ const sv = [st.mode, st.month]; st.mode = "month"; st.month = monthOf(new Date(nowMs())); try { openMetric("projectionCr", from); } finally { [st.mode, st.month] = sv; } return; }
  const {S:w, ws, we} = period(), m = BRK[id]; if (!w || !m) return;
  const D = $("#md"), u = w.usage || {}, h = H()[m.h], inRange = DATA.filter(s => inP(s, ws, we));
  const agents = ((w.shares || {}).source || []).map(x => ({name: x.key, c: agColor(x.key), mark: agMark(x.key), v: m.v ? m.v(x) || 0 : 0}));
  const projs = (w.projectStats || []).map(p => ({name: p.project, c: projColor(p.project), v: m.v ? m.v(p) || 0 : 0}));
  let big = "", body = "", title = t(m.n); // 内訳によっては、状態に合わせた見出しにする（MB が title を返す）
  if (MB[id]){ const r = MB[id](w, {ws, we, inRange}); if (!r) return; ({big, body} = r); if (r.title) title = r.title; }
  else if (m.proj){ // 月末の推定：今日までの実績 ÷ 日数 × 月の日数
    const pj = projection(w), so = m.proj === "cost" ? costOf(u) : u.credits; if (!pj || so == null) return;
    const val = m.proj === "cost" ? pj.cost : pj.credits, nd = w.days.length, avg = so / pj.days;
    big = `≈ ${bigF(m)(val)}`;
    body = `<div class="mcalc"><div><small>${esc(t("So far"))}</small><b>${esc(m.f(so))}</b></div><span>÷</span><div><small>${esc(t("Days so far"))}</small><b>${pj.days}</b></div><span>×</span><div><small>${esc(t("Days in the month"))}</small><b>${nd}</b></div><span>=</span><div><small>${esc(t("Month-end"))}</small><b>≈ ${esc(m.f(val))}</b></div></div>
      <p class="note">${esc(t("About {v} a day. The faded bars are the days still to come at that pace.", {v: m.f(avg)}))}</p>` + trendNote(w, pj, so, m) + bandTable(pj, m) + mDays(w, m, {avg, lines: (pj.band[m.proj] || []).filter(x => x.k === 3).map(x => { const R = w.days.length - pj.days; return {k: x.k, hi: (x.hi - so) / R, lo: (x.lo - so) / R}; })}) + mBars("So far, by agent", agents, m) + mBars("So far, by project", projs, m);
  } else if (id === "limits" || id === "compactions"){ // 起きた時刻の一覧：エージェントごと・プロジェクトごとと、いつどのセッションで
    const L = id === "limits", H2 = L ? limitHits(ws, we) : compactionsOf(ws, we); big = `${H2.length}`;
    const by = (key, mk) => countBy(H2, x => x.s[key]).map(([k, v]) => mk(k, v));
    const cm = {f: v => `${v}`};
    body = mBars("By agent", by("source", (k, v) => ({name: k, c: agColor(k), mark: agMark(k), v})), cm) +
      mBars("By project", by("project", (k, v) => ({name: k, c: projColor(k), v})), cm) +
      `<h3>${esc(t("When"))}</h3><div class="msess">${H2.map(x => sesCardH(x.s, `${md(x.t)} ${hm(x.t)} · ${esc(x.s.project)}${x.r ? ` · ${esc(t("resets {r}", {r: x.r}))}` : ""}`)).join("")}</div>`;
  } else {
    const tot = id === "tokens" ? u.tokens : id === "cost" ? costOf(u) : id === "credits" ? u.credits : id === "active" ? w.active : w.prompts;
    // 稼働日は帯から外したので、作業時間の内訳の数字に添える（途中の期間は今日までの日数で）
    const n0 = soFar(), nd = n0 == null ? w.days.length : Math.min(n0, w.days.length), ad = w.days.filter(d => d.active).length;
    big = id === "active" ? `${dur(tot || 0, true)}<small> · ${esc(t("Active days"))} ${ad} ${esc(n0 == null ? t("of {n}", {n: nd}) : t("of {n} so far", {n: nd}))}</small>` : id === "sessions" ? `${w.sessions}<small>/</small>${w.prompts}` : id === "cost" ? usdH(tot) : esc(bigF(m)(tot || 0));
    { const dt = kpiDelta(id); if (dt) big += `<small class="mdl"> · ${esc(dt)}</small>`; } // 前の期間との増減（帯には出さない）
    body = mDays(w, m);
    if (id === "sessions"){ // セッション数はエージェント別の集計にないので、期間にかかったセッションから数える
      const by = new Map(); inRange.forEach(s => by.set(s.source, (by.get(s.source) || 0) + (s.nPrompts || 0)));
      body += mBars("Prompts by agent", [...by].map(([k, v]) => ({name: k, c: agColor(k), mark: agMark(k), v})), m) + mBars("Prompts by project", projs, m);
    } else {
      body += mBars("By agent", agents, m) + mBars("By project", projs, m);
      if (m.model) body += mBars("By model", (u.models || []).map(r => ({name: r[0], c: "var(--acc)", v: r[m.model] || 0})), m);
    }
    if (m.ses){ const top = inRange.map(s => [s, m.ses(s) || 0]).filter(x => x[1] > 0).sort((a,b) => b[1] - a[1]).slice(0, 5);
      body += mSessions(id === "active" ? "Longest sessions" : id === "sessions" ? "Sessions with the most prompts" : id === "tokens" ? "Sessions with the most tokens" : id === "cost" ? "Heaviest sessions" : "Sessions with the most credits", top, m.f);
      if (top.length && id !== "active") body += WHOLE(); }
  }
  // 帯の使用量と月末の推定は、推定費用とクレジットを 1 つの枠に並べるので、費用の内訳からクレジットの内訳へ移れるように
  const go = (id === "cost" || id === "tokens") && u.credits ? "credits" : id === "projection" && projection(w)?.credits != null ? "projectionCr" : "";
  if (go) body += `<p><button class="pill" type="button" data-go="${go}">${esc(t(go === "credits" ? "Kiro credits breakdown" : "Month-end credits breakdown"))}</button></p>`;
  // 英語は「. 」と大文字の前（「.」は残す）で、日本語（HELP の訳）は最初の「。」の後ろで切る
  const [d1, dr] = !h ? ["", ""] : JA && h.d.includes("。") ? (i => [h.d.slice(0, i + 1), h.d.slice(i + 1)])(h.d.indexOf("。")) : (i => i < 0 ? [h.d, ""] : [h.d.slice(0, i + 1), h.d.slice(i + 2)])(h.d.search(/\. (?=[A-Z])/));
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" aria-label="${esc(t("Close"))}"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form>
    <div class="eyebrow">${esc(t("Breakdown"))} · ${st.mode === "month" ? dMY(st.month) : dSpan(st.week, addDays(st.week, 6))}</div><h2 id="mdh">${esc(title)}</h2><div class="mbig">${big}</div>${h ? `<p class="mdef" id="mdd">${esc(d1)}</p>` : ""}${body}
    ${h ? `<div class="mnot">${dr && id !== "cost" ? `<p>${esc(dr)}</p>` : ""}<p><b>${esc(t("Doesn't tell you"))}:</b> ${esc(h.x)}</p></div>` : ""}`; // 定義の 1 文目は数字のすぐ下に（読む前に何を数えたかがわかる）、細則と言えないことは読み終えたあとに。推定費用の料金の細則は長いので「?」にだけ書く
  if (h) D.setAttribute("aria-describedby", "mdd"); else D.removeAttribute("aria-describedby");
  D.querySelectorAll("[data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
  ["git", "push", "pr"].forEach(k => D.querySelectorAll(`[data-${k}]`).forEach(b => b.onclick = () => { D.close(); select(`${k}:${b.dataset[k]}`); })); // コミット・push・PR の一覧
  D.querySelectorAll("[data-go]").forEach(b => b.onclick = () => { D.onclose = null; D.close(); openMetric(b.dataset.go, from); });
  D.onclose = () => { if (from && from.isConnected && !st.sel) from.focus(); };
  D.showModal(); D.scrollTop = 0;
  const hd = $("#mdh"); hd.tabIndex = -1; hd.focus();
}
