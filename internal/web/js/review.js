// AI に改善案を聞くプロンプト、見直す候補の判定と指標の推移（スパークライン）
/* AI に改善案を聞くためのプロンプト（kiroku 自身は AI を呼ばない。利用者が自分のエージェントに貼る） */
function askPrompt(w, pw, M){
  const unit = M ? "月" : "週", u = w.usage || {}, L = [];
  const start = M ? st.month : st.week, last = M ? new Date(st.month.getFullYear(), st.month.getMonth()+1, 0) : addDays(st.week, 6);
  const cut = t => { t = String(t || "").replace(/\s+/g, " ").trim(); return t.length > 60 ? t.slice(0, 60) + "…" : t; };
  const use = x => [x.tokens ? `tokens ${tok(x.tokens)}` : "", x.cost >= 0.005 ? `estimated cost ${usd(x.cost)}` : "", x.credits ? `credits ${cr(x.credits)}` : ""].filter(Boolean).join(", ");
  const V = vsPrev(pw, unit); // 途中の期間は、前の期間の同じ日までと比べる
  const prev = (v, p, f, k) => (p = V.of(k, p)) == null ? "" : V.n == null ? ` (previous ${M ? "month" : "week"}: ${f(p)})` : ` (${V.range} of the previous ${M ? "month" : "week"}: ${f(p)})`;
  const sep = ", ", wk = M ? "month" : "week";
  L.push(`You are an advisor on using AI agents effectively. Below are figures from my AI agent history for ${dPeriod(M, start, last)} (one ${wk}), aggregated with kiroku. Suggest improvements that help me use AI agents more effectively within limited credits and tokens.`,
    "", "# What I'd like from you",
    "1. What the data says about how I work (strengths and concerns)",
    `2. Up to 3 high-priority improvements. For each, cite the figures behind it and a concrete action I can try ${uNext(unit)}`,
    `3. Metrics to check the effect ${uNext(unit)}`,
    "", "# Assumptions",
    "- Estimated cost is token counts priced at public API rates. It is not what I am actually billed",
    "- Metric definitions differ by agent. Don't compare agents directly",
    "- All figures are rough estimates from history. Clearly mark anything the data can't support as a guess",
    ...(V.n == null ? [] : [`- This ${wk} is still in progress (${V.n} days, ${dSpan(start, today0(), true)}). Figures for the previous ${wk} cover the same days (${V.range})`]),
    "- See \"How to read the metrics\" at the end for each metric's definition and what it can and cannot tell you",
    AI_DATA_NOTE);
  const di = L.length; // ここから下は履歴から作ったデータ（プロジェクト名や題が入る）。最後にコードブロックで囲む
  L.push("# Metrics kiroku flagged by threshold (candidates, not verdicts)",
    ...(() => { const F = findList(w, pw, unit); return F.length ? F.map(f => `- ${plainText(f.see)} (threshold: ${f.rule.charAt(0).toLowerCase() + f.rule.slice(1)})`) : ["- No metric crossed a threshold"]; })(),
    "", "# Overall",
    `- Active time: ${dur(w.active)}${prev(w.active, pw && pw.active, dur, "active")}`,
    `- Total AI run time (including parallel runs): ${dur(w.ai)}`,
    `- Sessions / prompts: ${w.sessions} / ${w.prompts}`,
    `- Focus blocks (60+ min): ${w.focus.length}${w.focus.length ? `, longest ${dur(Math.max(...w.focus.map(b => b.min)))}` : ""}`,
    `- Project switches per day: average ${w.switchesAvg}, max ${w.switchesMax}`,
    `- Parallel time: ${dur(w.parallel)} (up to ${w.maxConc} at once)`,
    `- Wait time (from an AI reply to my next prompt): median ${secs(w.waitMedian)}, 90th percentile ${secs(w.waitP90)} (n=${w.waitCount})`,
    `- Prompts with corrections or interruptions: ${w.fixRate == null ? "unknown" : w.fixRate + "%"} (n=${w.prompts})`,
    (() => { const {ws, we} = period(), H = limitHits(ws, we), N = unrecorded(ws, we, "limits"); return `- Usage limit hits (Claude Code and Codex): ${N ? notRec(N).toLowerCase() : H.length ? `${H.length} (${H.map(h => `${md(h.t)} ${hm(h.t)}`).join(", ")})` : "0"}`; })(),
    (() => { const {ws, we} = period(), C = compactionsOf(ws, we), N = unrecorded(ws, we, "compactions"); return `- Compactions (conversations summarized to free context; Claude Code, Codex and the SQLite history of Amazon Q / Kiro CLI): ${N ? notRec(N).toLowerCase() : C.length}${C.length ? ` in ${plural(new Set(C.map(c => c.s.id)).size, "session")}` : ""}`; })());
  if (u.tokens || u.credits){
    L.push("", "# AI usage");
    if (u.tokens) L.push(`- Tokens: ${tok(u.tokens)} (output ${tok(u.out)})`,
      `- Share of input read from cache: ${u.cacheHit == null ? "unknown" : Math.round(u.cacheHit*100) + "%"}`,
      `- Estimated cost: ${costOf(u) == null ? "unknown (the models used are not in the price table)" : `${usd(u.cost)}${prev(u.cost, pw && pw.usage && pw.usage.cost, usd, "cost")}${w.costPerAsk != null ? `, ${usd(w.costPerAsk)} per prompt` : ""}`}`);
    if (u.credits) L.push(`- Kiro credits: ${cr(u.credits)}`);
    L.push(`- Subagents: ${u.subagents}${u.subagents ? ` (total ${dur(u.subMin)})` : ""}`);
    if (u.models.length) L.push("- By model: " + u.models.slice(0, 6).map(r => `${(r[0])} (estimated cost ${usd(r[1])}, tokens ${tok(r[2])})`).join(sep));
  }
  const o = w.outputs, G = periodGit();
  if (w.git && w.git.commits){
    L.push("", "# Left behind in git (my own commits in the repositories agents worked in)",
      `- Commits: ${w.git.commits} (${w.git.ai} run by AI), +${w.git.added} −${w.git.removed} lines`,
      `- Files changed: ${G.files}${G.trunc ? " or more" : ""} in ${plural(G.projects, "project")}`);
    if (G.pushes) L.push(`- Pushes from this computer: ${G.pushes} to ${plural(G.refs, "branch", "branches")}`);
  }
  if (o && o.commits){
    L.push("", "# Outputs (run by AI and succeeded; Claude Code only)",
      `- Commits: ${o.commits}`,
      `- Sessions that reached a commit or pull request: ${w.outSessions} / ${w.outBase ?? w.sessions}`);
    if (w.costPerCommit != null) L.push(`- Estimated cost per commit: ${usd(w.costPerCommit)}`);
  }
  const start0 = M ? st.month : st.week, days = w.days.map((d, i) => [addDays(start0, i), d]).filter(([, d]) => d.active || d.tokens || d.credits || d.commits);
  if (days.length){
    L.push("", "# By day");
    days.forEach(([dd, d]) => L.push(`- ${md(dd.getTime()/1000)}: active ${dur(d.active)}, prompts ${d.prompts}, switches ${d.switches}${use(d) ? ", " + use(d) : ""}${d.commits ? `, commits ${d.commits}` : ""}`));
  }
  const ps = (w.projectStats || []).slice(0, 8);
  if (ps.length){
    L.push("", "# By project");
    ps.forEach(p => {
      const mt = p.models.reduce((t, m) => t + m.tokens, 0) || 1;
      L.push(`## ${p.project}`, `- Active time: ${dur(p.minutes)}, sessions / prompts: ${p.sessions} / ${p.prompts}${use(p) ? ", " + use(p) : ""}`);
      if (p.git && p.git.commits) L.push(`- Git commits: ${p.git.commits} (${p.git.ai} by AI), +${p.git.added} −${p.git.removed} lines`);
      else if (p.outputs && p.outputs.commits) L.push(`- AI commits: ${p.outputs.commits}`);
      if (p.models.length) L.push("- Main models: " + p.models.map(m => `${(m.model)} (${m.tokens ? Math.round(m.tokens*100/mt) + "%" : plural(m.turns, "turn")})`).join(sep));
      if (p.top.length) L.push("- Sessions that took the most time: " + p.top.map(t => `"${cut(t.title)}" ${dur(t.minutes)}${use(t) ? ", " + use(t) : ""}`).join(sep));
      if (p.heavy && use(p.heavy)) L.push(`- Heaviest session: "${cut(p.heavy.title)}" ${use(p.heavy)}`);
    });
    if ((w.projectStats || []).length > ps.length) L.push(`- ${plural(w.projectStats.length - ps.length, "more project")}`);
  }
  if (w.friction.length){
    L.push("", "# Sessions with possible friction");
    w.friction.forEach(f => L.push(`- "${cut(f.title)}" (${f.project}): ${whyOf(f).join(", ")}`));
  }
  if (w.native && w.native.length){
    L.push("", "# Agent-specific metrics");
    w.native.forEach(g => L.push(`- ${(g.source)} (${plural(g.sessions, "session")}): ` + g.values.map(v => `${nlabel(v)} ${nativeText(v)}`).join(sep)));
  }
  L.push("", "# History data", mdFence(L.splice(di).join("\n")));
  L.push("", "# How to read the metrics");
  Object.values(H()).forEach(h => L.push(`- ${h.n}: ${h.d}. Tells you: ${h.c}. Doesn't tell you: ${h.x}`));
  return L.join("\n");
}
/* 期間 [ws, we) に利用上限に当たった時刻と、そのセッション */
function limitHits(ws, we){ const H = []; DATA.forEach(s => (s.limits || []).forEach((t, i) => { if (t >= ws && t < we) H.push({t, s, r: limitReset(s, i)}); })); return H.sort((a,b) => a.t - b.t); }
/* 期間 [ws, we) のコンパクション（会話を要約して文脈を空けた）の時刻と、そのセッション */
function compactionsOf(ws, we){ const C = []; DATA.forEach(s => (s.compactions || []).forEach(t => { if (t >= ws && t < we) C.push({t, s}); })); return C.sort((a,b) => a.t - b.t); }
/* i 番目のコンパクションが自動か手動か（わからなければ ""）。履歴の値はそのまま出さず、決まった言葉にする */
function compactKind(s, i){ return ({auto: "automatic", manual: "manual"})[(s.compactKinds || [])[i]] || ""; }
/* i 番目の利用上限のエラー文にあった解除の時刻（"3:45pm" など、書いてあるとおりの文。日付や時間帯がないこともある）。なければ "" */
function limitReset(s, i){ return String((s.limitResets || [])[i] || ""); }
/* 見直す候補の判定に使う部品（期間 [ws, we) を渡す。推移の計算でも使う） */
const inP = (s, ws, we) => s.segs.some(([a,b]) => b > ws && a < we);
function idleOf(ws, we){ // 目安コストが大きいのにコミットも PR もないセッション（アウトプットを記録できる Claude Code のみ）
  const xs = DATA.filter(s => s.cost >= 1 && /claude/i.test(s.source) && !(s.outputs && (s.outputs.commits || s.outputs.prs)) && inP(s, ws, we)).sort((a,b) => b.cost - a.cost);
  return {xs, c: xs.reduce((t,s) => t + s.cost, 0)}; }
function longCtxOf(ws, we){ // 会話が長くなり、1 回の応答で読む入力が大きく増えたセッション
  return DATA.filter(s => inP(s, ws, we) && s.ctx && s.ctx.length === 3 && s.ctx[0] > 0 && s.ctx[1] >= s.ctx[0] * 4 && s.ctx[2] >= ctxPeakMin(s) && s.cost >= 0.5).sort((a,b) => b.cost - a.cost); }
// 長い会話とみなす最大の文脈：モデルのコンテキストウィンドウがわかれば、その半分（1M のモデルで 100K は長くないため）。わからなければ 100K
const ctxPeakMin = s => s.ctxWindow > 0 ? s.ctxWindow * 0.5 : 1e5;
const mainModel = s => ((s.models || []).slice().sort((a,b) => b[1] - a[1])[0] || [""])[0];
function lightOf(ws, we){ // 編集のない短いセッションで、高いモデル（Opus 系）を使ったもの
  const xs = DATA.filter(s => inP(s, ws, we) && /opus/i.test(mainModel(s)) && s.nPrompts <= 3 && !s.nFiles && s.cost >= 0.3).sort((a,b) => b.cost - a.cost);
  return {xs, c: xs.reduce((t,s) => t + s.cost, 0)}; }
/* 繰り返したプロンプト：期間中のプロンプトを、文字の 3 文字組の重なり（Jaccard 0.6 以上）でまとめ、3 つ以上のセッションで書いたものを拾う（AI は使わない） */
const REPEAT_MIN = 12, REPEAT_SES = 3, repeatCache = {};
function repeatsOf(ws, we){
  const ck = `${ws}:${we}:${DATA.length}`; if (repeatCache[ck]) return repeatCache[ck];
  const norm = t => String(t || "").toLowerCase().replace(/[\s　]+/g, " ").replace(/[「」『』"'`。、.,!?！？:：;；()（）\[\]]/g, "").trim();
  const C = [], idx = new Map(); // 3 文字組 → それを含む束の番号（束の代表は最初に見つけたプロンプト）
  DATA.forEach(s => s.prompts.forEach(p => {
    if (!(p.t >= ws && p.t < we)) return;
    const t = norm(p.text).slice(0, 200); if (t.length < REPEAT_MIN) return; // 「はい」「続けて」のような短い返事は、書き置いても意味がない
    const g = new Set(); for (let i = 0; i + 3 <= t.length; i++) g.add(t.slice(i, i + 3));
    const cnt = new Map(); g.forEach(x => (idx.get(x) || []).forEach(i => cnt.set(i, (cnt.get(i) || 0) + 1)));
    let best = -1, bs = 0.6; cnt.forEach((n, i) => { const j = n / (C[i].g.size + g.size - n); if (j >= bs){ bs = j; best = i; } });
    if (best < 0){ best = C.length; C.push({g, n: 0, ids: new Set(), last: -1}); g.forEach(x => { if (!idx.has(x)) idx.set(x, []); idx.get(x).push(best); }); }
    const c = C[best]; c.n++; c.ids.add(s.id); if (p.t >= c.last){ c.last = p.t; c.id = s.id; c.text = p.text; } // 見せるのは、いちばん新しい書き方
  }));
  return repeatCache[ck] = C.filter(c => c.ids.size >= REPEAT_SES).sort((a,b) => b.ids.size - a.ids.size || b.n - a.n);
}
const snipOf = (t, n) => { t = String(t || "").replace(/\s+/g, " ").trim(); return t.length > n ? t.slice(0, n) + "…" : t; };
/* 長すぎるプロンプト：期間中に、BIG_PROMPT 文字以上のプロンプト（ログや資料をそのまま貼ったものなど） */
const BIG_PROMPT = 4000, plen = p => p.len || String(p.text || "").length;
function bigOf(ws, we){ const ps = [], ids = new Set();
  DATA.forEach(s => s.prompts.forEach(p => { if (p.t >= ws && p.t < we && plen(p) >= BIG_PROMPT){ ps.push(plen(p)); ids.add(s.id); } }));
  return {n: ps.length, max: Math.max(0, ...ps), ids: [...ids]}; }
/* 指標の値（見直す候補の推移用）。low: 大きいときに印が付く向き（良し悪しの判定ではない） */
const pctOf = (a, b) => b ? Math.round(a * 100 / b) : null;
const MET = {
  limits: {f: (w, ws, we) => limitHits(ws, we).length, low: true, u: ""},
  longctx: {f: (w, ws, we) => longCtxOf(ws, we).length, low: true, u: ""},
  heavy: {f: (w, ws, we) => w.usage && w.usage.cost >= 0.01 ? pctOf(idleOf(ws, we).c, w.usage.cost) : null, low: true, u: "%"},
  cost: {f: w => w.usage ? w.usage.cost : null, low: true, fmt: v => usd(v)},
  costPerCommit: {f: w => w.costPerCommit, low: true, fmt: v => usd(v)},
  modelfit: {f: (w, ws, we) => w.usage && w.usage.cost >= 0.01 ? pctOf(lightOf(ws, we).c, w.usage.cost) : null, low: true, u: "%"},
  fix: {f: w => w.fixRate, low: true, u: "%"},
  friction: {f: w => w.friction.length, low: true, u: ""},
  bigPrompts: {f: (w, ws, we) => bigOf(ws, we).n, low: true, u: ""},
  cache: {f: w => w.usage && w.usage.cacheHit != null ? Math.round(w.usage.cacheHit * 100) : null, low: false, u: "%"},
  outSessions: {f: w => pctOf(w.outSessions, w.outBase ?? w.sessions), low: false, u: "%"},
  switches: {f: w => w.switchesAvg, low: true, u: ""},
  focus: {f: w => w.focus.length, low: false, u: ""},
  wait: {f: w => w.waitP90 != null ? Math.round(w.waitP90 / 60) : null, low: true, u: " min"},
  // 見直す候補にはならないが、「?」の説明に推移を出す指標（試したことが効いたかを、印が付いていなくても確かめられるように）
  active: {f: w => w.active, fmt: v => dur(v)},
  prompts: {f: w => w.prompts, u: ""},
  tokens: {f: w => w.usage && w.usage.tokens ? w.usage.tokens : null, fmt: v => tok(v)},
  gitCommits: {f: w => w.git ? w.git.commits : null, u: ""},
};
const fmtM = (k, v) => v == null ? "—" : MET[k].fmt ? MET[k].fmt(v) : `${Math.round(v * 10) / 10}${MET[k].u ?? ""}`; // 本文と同じく「8 件」「5.2 回」
function periodBack(i){ // 表示中の期間から i 個前の期間
  if (st.mode === "month"){ const a = new Date(st.month.getFullYear(), st.month.getMonth() - i, 1), b = new Date(a.getFullYear(), a.getMonth() + 1, 1);
    return {S: MONTHS[mkey(a)], ws: a.getTime()/1000, we: b.getTime()/1000, label: periodLabel("month", mkey(a)), key: mkey(a)}; }
  const a = addDays(st.week, -7 * i);
  return {S: WEEKS[key(a)], ws: a.getTime()/1000, we: addDays(a, 7).getTime()/1000, label: periodLabel("week", key(a)), key: key(a)};
}
function periodLabel(mode, k){ const [y, m, d] = k.split("-").map(Number); // 期間の名前（"2026-05" か "2026-05-04"）
  return mode === "month" ? dMY(new Date(y, m-1, 1)) : `week of ${dMD(new Date(y, m-1, d))}`; }
function metricAt(k, p){ if (!MET[k] || !p.S) return null; const v = MET[k].f(p.S, p.ws, p.we); return v == null || Number.isNaN(v) ? null : v; }
function spark(k, plain){ // 8 期間の推移（記録のない期間は飛ばす。記録のある最初の期間から、幅いっぱいに描く）。plain は「?」の説明に添えるもの（見直す向きは書かない）
  if (!MET[k]) return "";
  const pts = []; for (let i = 7; i >= 0; i--){ const p = periodBack(i); pts.push({v: metricAt(k, p), l: p.label}); }
  const vs = pts.filter(p => p.v != null).map(p => p.v); if (vs.length < 2) return "";
  // 使い始めて間もないなど、はじめの期間に記録がないときは、その期間を描かずに記録のある最初の期間を左端にする（線が箱の途中から始まって短く見えないように）
  const W = 168, H = 34, f0 = pts.findIndex(p => p.v != null), step = (W - 8) / (7 - f0); // 値が 2 つ以上あるので f0 は 6 以下
  const lo = Math.min(...vs), hi = Math.max(...vs), rng = lo === hi ? fmtM(k, lo) : `${fmtM(k, lo)}–${fmtM(k, hi)}`, x = i => 4 + (i - f0) * step, y = v => hi === lo ? H / 2 : H - 4 - (v - lo) / (hi - lo) * (H - 8);
  const line = pts.map((p, i) => p.v == null ? null : `${x(i).toFixed(1)},${y(p.v).toFixed(1)}`).filter(Boolean).join(" ");
  const txt = `${st.mode === "month" ? "8-month" : "8-week"} trend: ${pts.map(p => `${p.l} ${fmtM(k, p.v)}`).join(", ")}`;
  const hits = pts.map((p, i) => i < f0 ? "" : `<rect class="hit" x="${(x(i) - step / 2).toFixed(1)}" y="0" width="${step.toFixed(1)}" height="${H}"${tipAttr(p.l, fmtM(k, p.v) === "—" ? "No records" : fmtM(k, p.v))}/>`).join(""); // 点ごとに、その期間の値を出す
  // 両端（記録のある最初の期間と、表示中の期間）には、触れなくても読めるよう期間と値を添える
  const sl = i => { const p = periodBack(7 - i), [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? MON[mm - 1] : dMD(new Date(yy, mm - 1, dd)); };
  return `<span class="spark"><span class="sr">${esc(txt)}</span><span class="spk" aria-hidden="true"><svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"><polyline points="${line}"/>${pts.map((p, i) => p.v == null ? "" : `<circle class="pt" cx="${x(i)}" cy="${y(p.v)}" r="1.6"/>`).join("")}${pts[7].v != null ? `<circle cx="${x(7)}" cy="${y(pts[7].v)}" r="2.6"/>` : ""}${hits}</svg><span class="ends"><span>${sl(f0)} ${fmtM(k, pts[f0].v)}</span><span>${sl(7)} <b>${fmtM(k, pts[7].v)}</b></span></span></span><small aria-hidden="true">${plain ? `${st.mode === "month" ? "8-month" : "8-week"} trend · range ${rng}` : `${GOTO[k] ? `${HELP[k].n} · ` : ""}${st.mode === "month" ? "8-month" : "8-week"} range ${rng} · ${MET[k].low ? "higher" : "lower"} is worth a look`}</small></span>`;
} // 向き（高いほど・低いほど見直す）だけを書く。印が付いた理由は、推移の下に「Flagged because …」で別に出す（基準は推移の高低ではなく、決まった値なので）
/* 見直す候補：指標が決まった基準を超えたものを拾う（AI は使わない。判定ではなく、確かめる候補） */
function findList(w, pw, unit){
  const F = [], u = w.usage || {}, o = w.outputs || {}, {ws, we} = period();
  // because は、印が付いた理由（この期間の実際の数と基準）。推移の高低で付くのではないので、推移とは別に書く
  const add = (k, score, see, why, rule, ids, because) => F.push({k, score, see, why, rule, ids: ids || [], because});
  const P = uThis(unit);
  const LH = limitHits(ws, we);
  if (LH.length)
    add("limits", 40, `Hit the usage limit ${LH.length === 1 ? "once" : LH.length + " times"} (${LH.slice(-3).map(h => `${md(h.t)} ${hm(h.t)}${h.r ? ` (resets ${h.r})` : ""}`).join(", ")}${LH.length > 3 ? ", …" : ""})`, "Hitting a limit stops your work until it resets. The cause is often in how you worked just before", "1 or more", [...new Set(LH.map(h => h.s.id))].reverse(), `you hit the usage limit ${LH.length === 1 ? "once" : LH.length + " times"} ${P}`);
  if (w.fixRate != null && w.prompts >= 10 && w.fixRate >= 20)
    add("fix", w.fixRate, `${w.fixRate}% of prompts had corrections or interruptions`, "When a first prompt misses, redoing it costs time and tokens", "20% or more, with 10+ prompts", w.friction.map(f => f.id), `${w.fixRate}% of ${w.prompts} prompts ${P} had corrections or interruptions`);
  else if (w.friction.length)
    add("friction", 18, `${plural(w.friction.length, "session")} with possible friction`, "Rework is concentrated in a few sessions. Opening them usually hints at the cause", "Many corrections, interruptions or 15+ prompts", w.friction.map(f => f.id), `${plural(w.friction.length, "session")} ${P} crossed the threshold`);
  const LC = longCtxOf(ws, we);
  if (LC.length)
    add("longctx", 34, `${plural(LC.length, "session")} where the conversation grew long and the input read per response rose to ${Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0])))}× the first part (peak ${tok(Math.max(...LC.map(s => s.ctx[2])))} tokens)`, "The longer a conversation, the more earlier context each response rereads, so similar prompts get heavier", "Later input 4×+ the first part, peak at half the model's context window or more (100K+ tokens when the window is unknown), $0.5+", LC.map(s => s.id), `${plural(LC.length, "session")} ${P} crossed the threshold`);
  const LT = lightOf(ws, we);
  if (u.cost >= 2 && LT.c >= Math.max(1, u.cost * 0.1))
    add("modelfit", 24, `Short sessions with no edits spent ${usd(LT.c)} (${Math.round(LT.c*100/u.cost)}% of estimated cost) on Opus-class models`, "For research and questions, a lighter model is often enough", "Sessions with ≤3 prompts, no edits and $0.3+ total 10%+ of all cost ($2+) and $1+", LT.xs.map(s => s.id), `such sessions took ${Math.round(LT.c*100/u.cost)}% of estimated cost ${P}`);
  const {xs: idle, c: idleC} = idleOf(ws, we);
  if (u.cost >= 2 && idleC >= u.cost * 0.4)
    add("heavy", 30 + Math.round(idleC*50/u.cost), `${Math.round(idleC*100/u.cost)}% of estimated cost (${usd(idleC)}) went to sessions with no commit or pull request`, "Fine for research or discussion. If they stopped partway, it's worth finding out why", "Sessions of $1+ total 40%+ of all cost ($2+)", idle.map(s => s.id), `such sessions took ${Math.round(idleC*100/u.cost)}% of estimated cost ${P}`);
  const V = vsPrev(pw, unit), pc = V.of("cost", pw && pw.usage && pw.usage.cost), pl = V.n == null ? uLast(unit) : V.range; // 途中の期間は、前の期間の同じ日までと比べる
  if (pc != null && pc >= 1 && u.cost >= pc * 1.5)
    add("cost", 25, `Estimated cost was ${(u.cost/pc).toFixed(1)}× ${pl} (${usd(pc)} → ${usd(u.cost)})`, "What it means depends on which projects and sessions the increase came from", `1.5× ${uLast(unit)} or more`, null, `estimated cost was ${(u.cost/pc).toFixed(1)}× ${pl}`);
  if (pw && pw.costPerCommit != null && w.costPerCommit != null && o.commits >= 3 && w.costPerCommit >= pw.costPerCommit * 1.5)
    add("costPerCommit", 22, `Estimated cost per commit rose from ${usd(pw.costPerCommit)} to ${usd(w.costPerCommit)}`, "Reaching a checkpoint is getting heavier than before", `1.5× ${uLast(unit)} or more, with 3+ commits`, null, `it was ${(w.costPerCommit/pw.costPerCommit).toFixed(1)}× ${uLast(unit)}`);
  if (u.cacheHit != null && u.tokens >= 1e6 && u.cacheHit < 0.5)
    add("cache", 20, `Only ${Math.round(u.cacheHit*100)}% of input was read from cache`, "You may be re-sending the same context every time", "Under 50%, with 1M+ tokens", null, `${Math.round(u.cacheHit*100)}% of input ${P} was read from cache`);
  if ((w.outBase ?? w.sessions) >= 5 && (o.commits || o.prs || (w.git && w.git.commits)) && w.outSessions / (w.outBase ?? w.sessions) < 0.25)
    add("outSessions", 16, `${w.outSessions} of ${w.outBase ?? w.sessions} sessions reached a commit or pull request`, "Many sessions may have stopped partway", "Under 25%, with 5+ sessions", null, `${pctOf(w.outSessions, w.outBase ?? w.sessions)}% of sessions ${P} reached a commit or pull request`);
  const BG = bigOf(ws, we);
  if (BG.n >= 3)
    add("bigPrompts", 11, `${plural(BG.n, "prompt")} of ${commas(BIG_PROMPT)}+ characters (longest ${commas(BG.max)})`, "Pasting long logs or documents makes every later response re-read a heavier input, and buries the instructions that matter", `3+ prompts of ${commas(BIG_PROMPT)}+ characters`, BG.ids, `${plural(BG.n, "prompt")} ${P} crossed the threshold`);
  const RP = repeatsOf(ws, we);
  if (RP.length)
    add("repeats", 9, `You wrote a similar prompt ${RP[0].n} times across ${RP[0].ids.size} sessions ${P} ("${snipOf(RP[0].text, 40)}")`, "A prompt you type every time can be written once as a command or in CLAUDE.md", `${REPEAT_MIN}+ characters, in ${REPEAT_SES}+ sessions`, [...new Set([RP[0].id, ...RP[0].ids])], RP.length === 1 ? `1 prompt ${P} was written in ${REPEAT_SES} or more sessions` : `${RP.length} different prompts ${P} were each written in ${REPEAT_SES} or more sessions`);
  if (w.switchesAvg >= 5)
    add("switches", 14, `You switched projects ${w.switchesAvg} times a day on average`, "Each context switch tends to add ramp-up time and rework", "5+ per day on average", null, `you averaged ${w.switchesAvg} switches a day ${P}`);
  if (!w.focus.length && w.active >= 240)
    add("focus", 12, `You worked ${dur(w.active)}, but never for 60 minutes straight`, "Fragmented time makes it harder to hand large tasks to AI", "No focus blocks, with 4+ hours of work", null, `there were no focus blocks in ${dur(w.active)} of work ${P}`);
  if (w.waitP90 != null && w.waitCount >= 10 && w.waitP90 >= 900)
    add("wait", 10, `At the long end, ${secs(w.waitP90)} passed between an AI reply and your next prompt`, "AI may have sat idle until you noticed its reply", "90th percentile of 15+ min", null, `the 90th percentile ${P} was ${secs(w.waitP90)}`);
  return F.sort((a,b) => b.score - a.score);
}
const GOTO = {longctx: "heavy", modelfit: "models"}; // 専用の指標がない候補は、関係する指標に印を付ける
function flagSum(F){ // 期間の要点の下（#worth）に、基準を超えた指標の名前だけを優先度の高い順に並べる（押すとサマリーのその指標へ）
  const ids = [...new Set(F.map(f => GOTO[f.k] || f.k))];
  return `<span class="lbl">${ico("flag", "fdot")}Worth a look${hb("findings")}</span>${ids.length
    ? ids.map(id => `<button class="flink" data-goto="${id}">${esc(H()[id].n)}</button>`).join("")
    : `<span class="muted">No metric crossed a threshold</span>`}`;
}
function sesFlags(s){ // 詳細の頭に、このセッションが関係する見直す候補（表示中の期間のもの）の名前だけを出す。押すとサマリーと同じダイアログ
  const ids = [...new Set(lastFlags.filter(f => f.ids.includes(s.id)).map(f => GOTO[f.k] || f.k))];
  return ids.length ? `<div class="flagsum sflags"><span class="lbl" title="This session is one of the sessions behind these flags">${ico("flag", "fdot")}Worth a look</span>${ids.map(id => `<button class="flink" data-goto="${id}" aria-haspopup="dialog">${esc(H()[id].n)}</button>`).join("")}</div>` : "";
}
const flagSes = ids => ids.map(id => { const s = DATA.find(x => x.id === id); return s ? sesCardH(s, sesMeta(s)) : ""; }).join("");
/* 見直す候補の 1 つを、画面の真ん中のダイアログで開く（サマリーへ移らずに、何が見えて、なぜ大事で、次に何をするかを読める）。
   同じ指標に印の付く候補（GOTO）はまとめて出す。セッションを押すと、ダイアログを閉じて詳細を開く */
let lastFlags = []; // 表示中の期間の候補（summary が入れる）
function openWorth(id, from){
  const D = $("#wk"), fs = lastFlags.filter(f => (GOTO[f.k] || f.k) === id), h = H()[id]; if (!fs.length || !h) return;
  const body = fs.map(f => `<div class="fl"><p class="see">${ico("flag", "fdot")}${esc(f.see)}</p><p class="why">${esc(f.why)}</p>${spark(f.k)}${f.because ? `<p class="because">Flagged because ${esc(f.because)}</p>` : ""}${f.ids.length ? `<div class="fss">${flagSes(f.ids.slice(0, 6))}${f.ids.length > 6 ? `<p class="more">${plural(f.ids.length - 6, "more session")}</p>` : ""}</div>` : ""}<p class="rule">Threshold: ${esc(f.rule)}</p></div>`).join("");
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" id="wkclose" aria-label="Close"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form><div class="eyebrow">${ico("flag", "fdot")}Worth a look</div><h2 id="wkh">${esc(h.n)}</h2>${body}
    <dl class="wkhelp"><dt>What it is</dt><dd>${esc(h.d)}</dd><dt>Doesn't tell you</dt><dd>${esc(h.x)}</dd><dt>What to try</dt><dd>${esc(h.a)}</dd></dl>`;
  D.querySelectorAll(".fss [data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
  D.onclose = () => { if (from && from.isConnected && (!st.sel || from.closest("#panel")) && !document.activeElement?.closest?.("#review")) from.focus(); }; // 閉じたら押したリンクへ（サマリーや別のセッションへ移ったときは除く）
  D.showModal(); D.scrollTop = 0;
  const hd = $("#wkh"); hd.tabIndex = -1; hd.focus(); } // 最初のボタンではなく見出しへ（読み上げで何のダイアログかわかり、セッションを押した表示にもならない）
function placeFlags(R, F){ // 基準を超えた指標の名前の横に、小さな印だけを付ける（押すと「Worth a look」と同じダイアログ。中身はダイアログにだけ出し、サマリーで繰り返さない）
  [...new Set(F.map(f => GOTO[f.k] || f.k))].forEach(id => {
    const t = R.querySelector(`.panel .hb[data-help="${id}"]`); if (!t) return;
    t.insertAdjacentHTML("beforebegin", `<button class="fmark" data-goto="${id}" aria-haspopup="dialog" aria-label="${esc(`Worth a look: ${H()[id].n}`)}" title="Crossed a threshold. Press to see why">${ico("flag", "fdot")}</button>`);
    const dt = t.closest("details"); // 閉じた折りたたみの中の印は、見出しにも出す（開かずに、中に印があるとわかるように）
    if (dt){ const sm = dt.querySelector("summary"); if (!sm.querySelector(".fdot")) sm.insertAdjacentHTML("beforeend", `<span title="A metric crossed a threshold">${ico("flag", "fdot")}</span>`); }
  });
}
