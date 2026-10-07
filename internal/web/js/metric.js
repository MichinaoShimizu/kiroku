// 上の帯の数字を押したときの内訳（日ごと・エージェントごと・プロジェクトごと・モデルごと・セッション）
/* ── 数字の内訳 ──
   上の帯の数字（トークン・目安コスト・月末の見込み・クレジット・作業時間など）を押すと、画面の真ん中のダイアログで内訳を出す。
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
  commits: {n: "Git commits", h: "gitCommits", v: x => x.commits, f: v => `${Math.round(v)}`},
};
const sesTok = s => [s.usage, ...(s.subagents || []).map(a => a.usage)].reduce((t,u) => t + (u ? u.in+u.out+u.cw+u.cw1h+u.cr : 0), 0);
function sesMin(s){ const {ws, we} = period(); return s.segs.reduce((t, [a, b]) => t + Math.max(0, Math.min(b, we) - Math.max(a, ws))/60, 0); } // 期間の中で作業していた分
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
// 横の棒の一覧（エージェント・プロジェクト・モデル）
function mBars(title, rows, m, total){
  rows = rows.filter(r => r.v > 0).sort((a,b) => b.v - a.v); if (!rows.length) return "";
  total = total || rows.reduce((t,r) => t + r.v, 0); const max = rows[0].v;
  return `<h3>${title}</h3><div class="mbars">${rows.slice(0, 8).map(r => `<div class="mbar" style="--c:${r.c}"><span class="nm" title="${esc(r.name)}">${r.mark || ""}${esc(r.name)}</span><span class="tr"><span style="width:${r.v/max*100}%"></span></span><span class="vv">${esc(m.f(r.v))}</span><span class="pc">${pcOf(r.v, total)}%</span></div>`).join("")}${rows.length > 8 ? `<p class="note">${plural(rows.length - 8, "more")} not shown</p>` : ""}</div>`;
}
function mSessions(title, list, f){
  if (!list.length) return "";
  return `<h3>${title}</h3><div class="msess">${list.map(([s, v]) => `<button class="card ag" data-id="${esc(s.id)}" style="--ag:${agColor(s.source)}"><span class="ti">${esc(s.title)}</span><span class="me">${md(s.start)} · ${esc(s.project)} · ${esc(s.source)}<b>${esc(f(v))}</b></span></button>`).join("")}</div>`;
}
const projColor = p => st.colorBy === "project" ? colorOf(p) : "var(--acc)";
function openMetric(id, from){
  const {S:w, ws, we} = period(), m = BRK[id]; if (!w || !m) return;
  const D = $("#md"), u = w.usage || {}, h = H()[m.h], inRange = DATA.filter(s => inP(s, ws, we));
  const agents = ((w.shares || {}).source || []).map(x => ({name: x.key, c: agColor(x.key), mark: agMark(x.key), v: m.v ? m.v(x) || 0 : 0}));
  const projs = (w.projectStats || []).map(p => ({name: p.project, c: projColor(p.project), v: m.v ? m.v(p) || 0 : 0}));
  let big = "", body = "";
  if (m.proj){ // 月末の見込み：今日までの実績 ÷ 日数 × 月の日数
    const pj = projection(w), so = m.proj === "cost" ? costOf(u) : u.credits; if (!pj || so == null) return;
    const val = m.proj === "cost" ? pj.cost : pj.credits, nd = w.days.length, avg = so / pj.days;
    big = `≈ ${m.f(val)}`;
    body = `<div class="mcalc"><div><small>So far</small><b>${esc(m.f(so))}</b></div><span>÷</span><div><small>Days so far</small><b>${pj.days}</b></div><span>×</span><div><small>Days in the month</small><b>${nd}</b></div><span>=</span><div><small>Month-end</small><b>≈ ${esc(m.f(val))}</b></div></div>
      <p class="note">About ${esc(m.f(avg))} a day. The faded bars are the days still to come at that pace.</p>` + mDays(w, m, {avg}) + mBars("So far, by agent", agents, m) + mBars("So far, by project", projs, m);
  } else if (id === "limits"){
    const H2 = limitHits(ws, we); big = `${H2.length}`;
    const by = {}; H2.forEach(x => by[x.s.source] = (by[x.s.source] || 0) + 1);
    const cm = {f: v => `${v}`};
    body = mBars("By agent", Object.entries(by).map(([k, v]) => ({name: k, c: agColor(k), mark: agMark(k), v})), cm) +
      `<h3>When</h3><div class="msess">${H2.map(x => `<button class="card ag" data-id="${esc(x.s.id)}" style="--ag:${agColor(x.s.source)}"><span class="ti">${esc(x.s.title)}</span><span class="me">${md(x.t)} ${hm(x.t)} · ${esc(x.s.project)}${x.r ? ` · resets ${esc(x.r)}` : ""}</span></button>`).join("")}</div>`;
  } else {
    const tot = id === "tokens" ? u.tokens : id === "cost" ? costOf(u) : id === "credits" ? u.credits : id === "active" || id === "days" ? w.active : id === "sessions" ? w.prompts : (w.git || {}).commits || 0;
    big = id === "days" ? `${w.days.filter(d => d.active).length}<small> of ${w.days.length}</small>` : id === "sessions" ? `${w.sessions}<small>/</small>${w.prompts}` : id === "cost" ? usdH(tot) : esc(m.f(tot || 0));
    body = mDays(w, m);
    if (id === "sessions"){ // セッション数はエージェント別の集計にないので、期間にかかったセッションから数える
      const by = {}; inRange.forEach(s => { const b = by[s.source] = by[s.source] || {s: 0, p: 0}; b.s++; b.p += s.nPrompts || 0; });
      body += mBars("Prompts by agent", Object.entries(by).map(([k, b]) => ({name: k, c: agColor(k), mark: agMark(k), v: b.p})), m) + mBars("Prompts by project", projs, m);
    } else if (id === "commits"){
      const cm = {f: v => `${v}`};
      body += mBars("By project", (w.projectStats || []).map(p => ({name: p.project, c: projColor(p.project), v: (p.git || {}).commits || 0})), cm);
      if (w.git && w.git.commits) body += mBars("Who made them", [{name: "By AI", c: "var(--acc)", v: w.git.ai}, {name: "Not by AI", c: "var(--other)", v: w.git.commits - w.git.ai}], cm);
    } else if (id !== "days"){
      body += mBars("By agent", agents, m) + mBars("By project", projs, m);
      if (m.model) body += mBars("By model", (u.models || []).map(r => ({name: r[0], c: "var(--acc)", v: r[m.model] || 0})), m);
    }
    if (m.ses){ const top = inRange.map(s => [s, m.ses(s) || 0]).filter(x => x[1] > 0).sort((a,b) => b[1] - a[1]).slice(0, 5);
      body += mSessions(id === "active" ? "Longest sessions" : id === "sessions" ? "Sessions with the most prompts" : "Top sessions", top, m.f);
      if (top.length && id !== "active") body += `<p class="note">Session figures cover the whole session, including any part outside this ${st.mode === "month" ? "month" : "week"}.</p>`; }
  }
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" aria-label="Close"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form>
    <div class="eyebrow">Breakdown · ${st.mode === "month" ? `${MONTH[st.month.getMonth()]} ${st.month.getFullYear()}` : `${dMD(st.week)} – ${dMD(addDays(st.week, 6))}`}</div><h2 id="mdh">${esc(m.n)}</h2><div class="mbig">${big}</div>${body}
    ${h ? `<dl class="wkhelp"><dt>What it is</dt><dd>${esc(h.d)}</dd><dt>Doesn't tell you</dt><dd>${esc(h.x)}</dd></dl>` : ""}`;
  D.querySelectorAll("[data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
  D.onclose = () => { if (from && from.isConnected && !st.sel) from.focus(); };
  D.showModal(); D.scrollTop = 0;
  const hd = $("#mdh"); hd.tabIndex = -1; hd.focus();
}
