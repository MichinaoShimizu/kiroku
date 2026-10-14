// 見直す候補の判定と指標の推移（スパークライン）
/* 期間 [ws, we) に利用上限に当たった時刻と、そのセッション */
function limitHits(ws, we){ const H = []; DATA.forEach(s => (s.limits || []).forEach((t, i) => { if (t >= ws && t < we) H.push({t, s, r: limitReset(s, i)}); })); return H.sort((a,b) => a.t - b.t); }
/* 期間 [ws, we) のコンパクション（会話を要約して文脈を空けた）の時刻と、そのセッション */
function compactionsOf(ws, we){ const C = []; DATA.forEach(s => (s.compactions || []).forEach(t => { if (t >= ws && t < we) C.push({t, s}); })); return C.sort((a,b) => a.t - b.t); }
/* i 番目のコンパクションが自動か手動か（わからなければ ""）。履歴の値はそのまま出さず、決まった言葉にする */
function compactKind(s, i){ return ({auto: t("automatic"), manual: t("manual")})[(s.compactKinds || [])[i]] || ""; }
/* i 番目の利用上限のエラー文にあった解除の時刻（"3:45pm" など、書いてあるとおりの文。日付や時間帯がないこともある）。なければ "" */
function limitReset(s, i){ const r = String((s.limitResets || [])[i] || ""); // 履歴の文（"9pm" など）。日本語では読める時刻は 24 時間に
  if (!isJA()) return r;
  return r.replace(/\b(\d{1,2})(?::(\d{2}))?\s*([ap])\.?m\.?\b/gi, (m, h, mm, ap) => { h = +h % 12 + (ap.toLowerCase() === "p" ? 12 : 0); return `${String(h).padStart(2, "0")}:${mm || "00"}`; }); }
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
    if (best < 0){ best = C.length; C.push({g, n: 0, ids: new Set(), by: new Map(), last: -1}); g.forEach(x => { if (!idx.has(x)) idx.set(x, []); idx.get(x).push(best); }); }
    const c = C[best]; c.n++; c.ids.add(s.id); c.by.set(s.id, p.text); if (p.t >= c.last){ c.last = p.t; c.id = s.id; c.text = p.text; } // 見せるのは、いちばん新しい書き方。by はセッションごとに、束に入ったプロンプト（ダイアログでどれのことかを見せる）
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
  wait: {f: w => w.waitP90 != null ? Math.round(w.waitP90 / 60) : null, low: true, u: t(" min")},
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
  const lbl = p => { const [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? p.label : t("week of {date}", {date: dMD(new Date(yy, mm - 1, dd))}); }; // 期間の名前（画面の言語で）
  const pts = []; for (let i = 7; i >= 0; i--){ const p = periodBack(i); pts.push({v: metricAt(k, p), l: lbl(p)}); }
  const vs = pts.filter(p => p.v != null).map(p => p.v); if (vs.length < 2) return "";
  // 使い始めて間もないなど、はじめの期間に記録がないときは、その期間を描かずに記録のある最初の期間を左端にする（線が箱の途中から始まって短く見えないように）
  const W = 168, H = 34, f0 = pts.findIndex(p => p.v != null), step = (W - 8) / (7 - f0); // 値が 2 つ以上あるので f0 は 6 以下
  const lo = Math.min(...vs), hi = Math.max(...vs), rng = lo === hi ? fmtM(k, lo) : `${fmtM(k, lo)}–${fmtM(k, hi)}`, x = i => 4 + (i - f0) * step, y = v => hi === lo ? H / 2 : H - 4 - (v - lo) / (hi - lo) * (H - 8);
  const line = pts.map((p, i) => p.v == null ? null : `${x(i).toFixed(1)},${y(p.v).toFixed(1)}`).filter(Boolean).join(" ");
  const list = pts.map(p => `${p.l} ${fmtM(k, p.v)}`).join(LSEP()), txt = st.mode === "month" ? t("8-month trend: {list}", {list}) : t("8-week trend: {list}", {list});
  const hits = pts.map((p, i) => i < f0 ? "" : `<rect class="hit" x="${(x(i) - step / 2).toFixed(1)}" y="0" width="${step.toFixed(1)}" height="${H}"${tipAttr(p.l, fmtM(k, p.v) === "—" ? t("No records") : fmtM(k, p.v))}/>`).join(""); // 点ごとに、その期間の値を出す
  // 両端（記録のある最初の期間と、表示中の期間）には、触れなくても読めるよう期間と値を添える
  const sl = i => { const p = periodBack(7 - i), [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? MON[mm - 1] : dMD(new Date(yy, mm - 1, dd)); };
  return `<span class="spark"><span class="sr">${esc(txt)}</span><span class="spk" aria-hidden="true"><svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"><polyline points="${line}"/>${pts.map((p, i) => p.v == null ? "" : `<circle class="pt" cx="${x(i)}" cy="${y(p.v)}" r="1.6"/>`).join("")}${pts[7].v != null ? `<circle cx="${x(7)}" cy="${y(pts[7].v)}" r="2.6"/>` : ""}${hits}</svg><span class="ends"><span>${sl(f0)} ${fmtM(k, pts[f0].v)}</span><span>${sl(7)} <b>${fmtM(k, pts[7].v)}</b></span></span></span><small aria-hidden="true">${esc(plain ? (st.mode === "month" ? t("8-month range {range}", {range: rng}) : t("8-week range {range}", {range: rng})) : `${GOTO[k] ? `${t(HELP[k].n)} · ` : ""}${st.mode === "month" ? (MET[k].low ? t("8-month range {range} · higher is worth a look", {range: rng}) : t("8-month range {range} · lower is worth a look", {range: rng})) : (MET[k].low ? t("8-week range {range} · higher is worth a look", {range: rng}) : t("8-week range {range} · lower is worth a look", {range: rng}))}`)}</small></span>`;
} // 向き（高いほど・低いほど見直す）だけを書く。印が付いた基準（決まった値）は、ダイアログの見出しのすぐ下に出す
/* 見直す候補：指標が決まった基準を超えたものを拾う（AI は使わない。判定ではなく、確かめる候補） */
// 見直す候補の文は 2 つ持つ。en は英語のまま（週報のプロンプトに入れる。訳さない）、L は画面に出す今の言語の文。
// 鍵は英語の文で、{name} を埋める。英語の画面では L と en は同じになる
const fillEn = (s, v) => s.replace(/\{(\w+)\}/g, (m, k) => Object.prototype.hasOwnProperty.call(v, k) ? String(v[k]) : m);
const pluralEn = (n, one) => `${n} ${n === 1 ? one : one + "s"}`;
function findList(w, pw, unit){
  const F = [], u = w.usage || {}, o = w.outputs || {}, {ws, we} = period();
  const M = unit === "月", V = vsPrev(pw, unit);
  // en と L で違う値：期間の言い方と、数と名詞（plural は日本語の画面で日本語になる）
  const en = {period: uThis(unit), last: uLast(unit), prev: V.n == null ? uLast(unit) : V.range};
  const loc = {period: M ? t("this month") : t("this week"), last: M ? t("last month") : t("last week")}; loc.prev = V.n == null ? loc.last : V.range;
  const tx = (s, v = {}, l = {}) => ({en: fillEn(s, {...en, ...v}), l: t(s, {...loc, ...v, ...l})});
  const ses = n => [{sessions: pluralEn(n, "session")}, {sessions: plural(n, "session")}];
  const add = (k, score, see, why, rule, ids) => F.push({k, score, see: see.en, why: why.en, rule: rule.en, ids: ids || [],
    L: {see: see.l, why: why.l, rule: rule.l}});
  const LH = limitHits(ws, we);
  if (LH.length){
    const hl = (rs, hs) => hs.slice(-3).map(h => h.r ? fillEn(rs, {time: `${md(h.t)} ${hm(h.t)}`, r: h.r}) : `${md(h.t)} ${hm(h.t)}`).join(LSEP()) + (hs.length > 3 ? LSEP() + "…" : "");
    // 英語の文（週報のプロンプトにも入る）は、日付と解除時刻も英語のまま
    const v = [{n: LH.length, list: inEnglish(() => hl("{time} (resets {r})", limitHits(ws, we)))}, {list: hl(t("{time} (resets {r})"), LH)}];
    add("limits", 40, LH.length === 1 ? tx("Hit the usage limit once ({list})", ...v) : tx("Hit the usage limit {n} times ({list})", ...v), tx("Hitting a limit stops your work until it resets. The cause is often in how you worked just before"), tx("1+ hit"), [...new Set(LH.map(h => h.s.id))].reverse());
  }
  if (w.fixRate != null && w.prompts >= 10 && w.fixRate >= 20)
    add("fix", w.fixRate, tx("{pct}% of prompts had corrections or interruptions", {pct: w.fixRate}), tx("When a first prompt misses, redoing it costs time and tokens"), tx("20% or more, with 10+ prompts"), w.friction.map(f => f.id));
  else if (w.friction.length)
    add("friction", 18, tx("{sessions} with possible friction", ...ses(w.friction.length)), tx("Rework is concentrated in a few sessions. Opening them usually hints at the cause"), tx("A session with 1+ correction or interruption, or 15+ prompts"), w.friction.map(f => f.id));
  const LC = longCtxOf(ws, we);
  if (LC.length){
    const [a, b] = ses(LC.length);
    add("longctx", 34, tx("{sessions} where the conversation grew long and the input read per response rose to {x}× the first part (peak {peak} tokens)", {...a, x: Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0]))), peak: tok(Math.max(...LC.map(s => s.ctx[2])))}, b), tx("The longer a conversation, the more earlier context each response rereads, so similar prompts get heavier"), tx("Later input 4×+ the first part, peak at half the model's context window or more (100K+ tokens when the window is unknown), $0.5+"), LC.map(s => s.id));
  }
  const LT = lightOf(ws, we);
  if (u.cost >= 2 && LT.c >= Math.max(1, u.cost * 0.1))
    add("modelfit", 24, tx("Short sessions with no edits spent {cost} ({pct}% of estimated cost) on Opus-class models", {cost: usd(LT.c), pct: Math.round(LT.c*100/u.cost)}), tx("For research and questions, a lighter model is often enough"), tx("Sessions with ≤3 prompts, no edits and $0.3+ total 10%+ of all cost ($2+) and $1+"), LT.xs.map(s => s.id));
  const {xs: idle, c: idleC} = idleOf(ws, we);
  if (u.cost >= 2 && idleC >= u.cost * 0.4)
    add("heavy", 30 + Math.round(idleC*50/u.cost), tx("{pct}% of estimated cost ({cost}) went to sessions with no commit or pull request", {pct: Math.round(idleC*100/u.cost), cost: usd(idleC)}), tx("Fine for research or discussion. If they stopped partway, it's worth finding out why"), tx("Sessions of $1+ total 40%+ of all cost ($2+)"), idle.map(s => s.id));
  const pc = V.of("cost", pw && pw.usage && pw.usage.cost); // 途中の期間は、前の期間の同じ日までと比べる
  if (pc != null && pc >= 1 && u.cost >= pc * 1.5)
    add("cost", 25, tx("Estimated cost was {x}× that of {prev} ({from} → {to})", {x: (u.cost/pc).toFixed(1), from: usd(pc), to: usd(u.cost)}), tx("What it means depends on which projects and sessions the increase came from"), tx("1.5× {last} or more"), null);
  if (pw && pw.costPerCommit != null && w.costPerCommit != null && o.commits >= 3 && w.costPerCommit >= pw.costPerCommit * 1.5)
    add("costPerCommit", 22, tx("Estimated cost per commit rose from {from} to {to}", {from: usd(pw.costPerCommit), to: usd(w.costPerCommit)}), tx("Each commit is costing more than before"), tx("1.5× {last} or more, with 3+ commits"), null);
  if (u.cacheHit != null && u.tokens >= 1e6 && u.cacheHit < 0.5)
    add("cache", 20, tx("Only {pct}% of input was read from cache", {pct: Math.round(u.cacheHit*100)}), tx("You may be re-sending the same context every time"), tx("Under 50%, with 1M+ tokens"), null);
  if ((w.outBase ?? w.sessions) >= 5 && (o.commits || o.prs || (w.git && w.git.commits)) && w.outSessions / (w.outBase ?? w.sessions) < 0.25)
    add("outSessions", 16, tx("{n} of {total} sessions reached a commit or pull request", {n: w.outSessions, total: w.outBase ?? w.sessions}), tx("Many sessions may have stopped partway"), tx("Under 25%, with 5+ sessions"), null);
  const BG = bigOf(ws, we);
  if (BG.n >= 3){
    const a = {prompts: pluralEn(BG.n, "prompt")}, b = {prompts: plural(BG.n, "prompt")};
    add("bigPrompts", 11, tx("{prompts} of {min}+ characters (longest {max})", {...a, min: commas(BIG_PROMPT), max: commas(BG.max)}, b), tx("Pasting long logs or documents makes every later response re-read a heavier input, and buries the instructions that matter"), tx("3+ prompts of {min}+ characters", {min: commas(BIG_PROMPT)}), BG.ids);
  }
  const RP = repeatsOf(ws, we);
  if (RP.length)
    add("repeats", 9, tx("You wrote a similar prompt {n} times across {s} sessions {period} (\"{text}\")", {n: RP[0].n, s: RP[0].ids.size, text: snipOf(RP[0].text, 40)}), tx("A prompt you type every time can be written once as a command or in CLAUDE.md"), tx("{min}+ characters, in {ses}+ sessions", {min: REPEAT_MIN, ses: REPEAT_SES}), [...new Set([RP[0].id, ...RP[0].ids])]);
  if (w.switchesAvg >= 5)
    add("switches", 14, tx("You switched projects {n} times a day on average", {n: w.switchesAvg}), tx("Each context switch tends to add ramp-up time and rework"), tx("5+ per day on average"), null);
  if (!w.focus.length && w.active >= 240)
    add("focus", 12, tx("You worked {dur}, but never for 60 minutes straight", {dur: dur(w.active)}), tx("Fragmented time makes it harder to hand large tasks to AI"), tx("No focus blocks, with 4+ hours of work"), null);
  if (w.waitP90 != null && w.waitCount >= 10 && w.waitP90 >= 900)
    add("wait", 10, tx("At the long end, {secs} passed between an AI reply and your next prompt", {secs: secs(w.waitP90)}), tx("AI may have sat idle until you noticed its reply"), tx("90th percentile of 15+ min"), null);
  return F.sort((a,b) => b.score - a.score);
}
const GOTO = {longctx: "heavy", modelfit: "models"}; // 専用の指標がない候補は、関係する指標に印を付ける
function flagSum(F){ // 期間の要点の下（#worth）に、基準を超えた指標の名前だけを優先度の高い順に並べる（押すとサマリーのその指標へ）
  const ids = [...new Set(F.map(f => GOTO[f.k] || f.k))];
  return `<span class="lbl">${ico("flag", "fdot")}${esc(t("Worth a look"))}${hb("findings")}</span>${ids.length
    ? ids.map(id => `<button class="flink" data-goto="${id}">${esc(H()[id].n)}</button>`).join("")
    : `<span class="muted">${esc(t("No metric crossed a threshold"))}</span>`}`;
}
function sesFlags(s){ // 詳細の頭に、このセッションが関係する見直す候補（表示中の期間のもの）の名前だけを出す。押すとサマリーと同じダイアログ
  const ids = [...new Set(lastFlags.filter(f => f.ids.includes(s.id)).map(f => GOTO[f.k] || f.k))];
  return ids.length ? `<div class="sflags"><span class="lbl" title="${esc(t("This session is one of the sessions behind these flags"))}">${ico("flag", "fdot")}${esc(t("Worth a look"))}</span>${ids.map(id => `<button class="flink" data-goto="${id}" aria-haspopup="dialog">${esc(H()[id].n)}</button>`).join("")}</div>` : "";
}
// line は、カードに添える 1 行（id → 文字）。繰り返しのプロンプトでは、そのセッションで書いたもの
const flagSes = (ids, line) => ids.map(id => { const s = DATA.find(x => x.id === id), l = line && line.get(id); return s ? sesCardH(s, sesMeta(s) + (l ? `<span class="fq">${esc(snipOf(l, 90))}</span>` : "")) : ""; }).join("");
/* 見直す候補の 1 つを、画面の真ん中のダイアログで開く（サマリーへ移らずに、何が見えて、なぜ大事で、次に何をするかを読める）。
   同じ指標に印の付く候補（GOTO）はまとめて出す。セッションを押すと、ダイアログを閉じて詳細を開く */
let lastFlags = []; // 表示中の期間の候補（summary が入れる）
function openWorth(id, from){
  const D = $("#wk"), fs = lastFlags.filter(f => (GOTO[f.k] || f.k) === id), h = H()[id]; if (!fs.length || !h) return;
  const {ws, we} = period(), rp = id === "repeats" ? (repeatsOf(ws, we)[0] || {}).by : null;
  const body = fs.map(f => `<div class="fl"><p class="see">${ico("flag", "fdot")}${esc(f.L.see)}</p><p class="rule">${esc(t("Threshold: {rule}", {rule: f.L.rule}))}</p>${f.k === "repeats" ? "" : `<p class="why">${esc(f.L.why)}</p>`}${spark(f.k)}${f.ids.length ? `<div class="fss">${flagSes(f.ids.slice(0, 6), f.k === "repeats" ? rp : null)}${f.ids.length > 6 ? `<p class="more">${esc(f.ids.length - 6 === 1 ? t("1 more session") : t("{n} more sessions", {n: f.ids.length - 6}))}</p>` : ""}</div>` : ""}</div>`).join(""); // 繰り返しのプロンプトの「なぜ」は「What to try」と同じことなので出さない
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" id="wkclose" aria-label="${esc(t("Close"))}"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form><div class="eyebrow">${ico("flag", "fdot")}${esc(t("Worth a look"))}</div><h2 id="wkh">${esc(h.n)}</h2>${body}
    <dl class="wkhelp"><dt>${esc(t("What it is"))}</dt><dd>${esc(h.d)}</dd><dt>${esc(t("Doesn't tell you"))}</dt><dd>${esc(h.x)}</dd><dt>${esc(t("What to try"))}</dt><dd>${esc(h.a)}</dd></dl>`;
  D.querySelectorAll(".fss [data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
  D.onclose = () => { if (from && from.isConnected && (!st.sel || from.closest("#panel")) && !document.activeElement?.closest?.("#review")) from.focus(); }; // 閉じたら押したリンクへ（サマリーや別のセッションへ移ったときは除く）
  D.showModal(); D.scrollTop = 0;
  const hd = $("#wkh"); hd.tabIndex = -1; hd.focus(); } // 最初のボタンではなく見出しへ（読み上げで何のダイアログかわかり、セッションを押した表示にもならない）
function placeFlags(R, F){ // 基準を超えた指標の名前の横に、小さな印だけを付ける（押すと「Worth a look」と同じダイアログ。中身はダイアログにだけ出し、サマリーで繰り返さない）
  [...new Set(F.map(f => GOTO[f.k] || f.k))].forEach(id => {
    const hb = R.querySelector(`.panel .hb[data-help="${id}"]`); if (!hb) return;
    hb.insertAdjacentHTML("beforebegin", `<button class="fmark" data-goto="${id}" aria-haspopup="dialog" aria-label="${esc(t("Worth a look: {name}", {name: H()[id].n}))}" title="${esc(t("Crossed a threshold. Click to see why."))}">${ico("flag", "fdot")}</button>`);
    const dt = hb.closest("details"); // 閉じた折りたたみの中の印は、見出しにも出す（開かずに、中に印があるとわかるように）
    if (dt){ const sm = dt.querySelector("summary"); if (!sm.querySelector(".fdot")) sm.insertAdjacentHTML("beforeend", `<span title="${esc(t("A metric crossed a threshold"))}">${ico("flag", "fdot")}</span>`); }
  });
}
