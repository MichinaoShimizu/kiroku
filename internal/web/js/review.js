// 見直す候補の判定と指標の推移（スパークライン）
/* 期間 [ws, we) に利用上限に当たった時刻と、そのセッション */
function limitHits(ws, we){ const H = []; DATA.forEach(s => (s.limits || []).forEach((t, i) => { if (t >= ws && t < we) H.push({t, s, r: limitReset(s, i)}); })); return H.sort((a,b) => a.t - b.t); }
/* 期間 [ws, we) のコンパクション（会話を要約して文脈を空けた）の時刻と、そのセッション */
function compactionsOf(ws, we){ const C = []; DATA.forEach(s => (s.compactions || []).forEach(t => { if (t >= ws && t < we) C.push({t, s}); })); return C.sort((a,b) => a.t - b.t); }
/* i 番目のコンパクションが自動か手動か（わからなければ ""）。履歴の値はそのまま出さず、決まった言葉にする */
function compactKind(s, i){ return ({auto: t("automatic"), manual: t("manual")})[(s.compactKinds || [])[i]] || ""; }
/* i 番目の利用上限の解除の時刻。履歴に時刻（UNIX 秒、Codex の resets_at）があれば、ほかの時刻と同じく今いる時間帯で出す
   （当たった日と違う日なら日付も付ける）。なければエラー文にあった時刻（"3:45pm" など、書いてあるとおりの文。日付や時間帯がないこともある）。どちらもなければ "" */
function limitReset(s, i){ const at = +((s.limitResetAt || [])[i]) || 0, hit = (s.limits || [])[i];
  if (hit > 0 && at > hit && at <= hit + 366 * 86400) return md(at) === md(hit) ? hm(at) : `${md(at)} ${hm(at)}`; // 当たったあと 1 年までの時刻だけ（Go の LimitUntil と同じ）
  const r = String((s.limitResets || [])[i] || ""); // 履歴の文（"9pm" など）。読める時刻は、ほかの時刻と同じ 24 時間に
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
const pricey = s => /opus|fable|mythos/i.test(mainModel(s)); // 主なモデルが高いモデル（Opus 系・Fable 系・Mythos 系）か
function lightOf(ws, we){ // 編集のない短いセッションで、高いモデル（Opus 系・Fable 系・Mythos 系）を使ったもの
  const xs = DATA.filter(s => inP(s, ws, we) && pricey(s) && s.nPrompts <= 3 && !s.nFiles && s.cost >= 0.3).sort((a,b) => b.cost - a.cost);
  return {xs, c: xs.reduce((t,s) => t + s.cost, 0)}; }
/* 繰り返したプロンプト：期間中のプロンプトを、文字の 3 文字組の重なり（Jaccard 0.6 以上）でまとめ、3 つ以上のセッションで書いたものを拾う（AI は使わない） */
const REPEAT_MIN = 12, REPEAT_SES = 3, repeatCache = new WeakMap(); // DATA の配列ごと（エージェントで絞ったサマリーは別の配列なので、絞り方が違えば別に数える）
function repeatsOf(ws, we){
  if (!repeatCache.has(DATA)) repeatCache.set(DATA, {}); const memo = repeatCache.get(DATA), ck = `${ws}:${we}`; if (memo[ck]) return memo[ck];
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
  return memo[ck] = C.filter(c => c.ids.size >= REPEAT_SES).sort((a,b) => b.ids.size - a.ids.size || b.n - a.n);
}
const snipOf = (t, n) => { t = String(t || "").replace(/\s+/g, " ").trim(); return t.length > n ? t.slice(0, n) + "…" : t; };
/* 長すぎるプロンプト：期間中に、BIG_PROMPT 文字以上のプロンプト（ログや資料をそのまま貼ったものなど） */
const BIG_PROMPT = 4000, plen = p => p.len || String(p.text || "").length;
function bigOf(ws, we){ const ps = [], ids = new Set();
  DATA.forEach(s => s.prompts.forEach(p => { if (p.t >= ws && p.t < we && plen(p) >= BIG_PROMPT){ ps.push(plen(p)); ids.add(s.id); } }));
  return {n: ps.length, max: Math.max(0, ...ps), ids: [...ids]}; }
/* 時間・セッション・プロンプトあたりの元（期間 [ws, we) に始まったセッションのうち、その使用量を記録したもの）。
   分子と分母は同じセッションから。時間は各セッションの作業時間（期間の中の分。重なったセッションはそれぞれ数える） */
const minIn = (s, ws, we) => (s.segs || []).reduce((a, [x, y]) => a + Math.max(0, Math.min(y, we) - Math.max(x, ws)) / 60, 0);
// 同じ DATA・期間・量の集計は覚えておく（推移の 13 期間 × 指標の数だけ、全セッションをたどり直さない。エージェントで絞ると DATA が替わるので、DATA ごとに）
const basisCache = new WeakMap();
const memoBasis = (tag, ws, we, k, f) => { if (!basisCache.has(DATA)) basisCache.set(DATA, new Map()); const m = basisCache.get(DATA), ck = `${tag}:${ws}:${we}:${k}`;
  if (!m.has(ck)) m.set(ck, f()); return m.get(ck); };
function useBasis(ws, we, k){ return memoBasis("use", ws, we, k, () => useBasis0(ws, we, k)); }
function useBasis0(ws, we, k){
  const use = s => k === "cost" ? (sesTok(s) ? s.cost || 0 : 0) : k === "credits" ? s.credits || 0 : sesTok(s);
  const xs = DATA.filter(s => s.start >= ws && s.start < we && use(s) > 0);
  let days = 0; for (let d = ws; d < we; d += 86400) if (xs.some(s => minIn(s, d, Math.min(we, d + 86400)) > 0)) days++; // 稼働した日：その使用量を記録したセッションが動いた日の数
  return {n: xs.length, used: xs.reduce((a, s) => a + use(s), 0), min: xs.reduce((a, s) => a + minIn(s, ws, we), 0), prompts: xs.reduce((a, s) => a + (s.nPrompts || 0), 0), days};
}
const perUse = (ws, we, k, by) => { const b = useBasis(ws, we, k), d = by === "hour" ? b.min / 60 : by === "day" ? b.days : by === "ses" ? b.n : b.prompts; return d > 0 && b.used > 0 ? b.used / d : null; };
/* Git から読むコミットの扱い（どれも決まった規則。AI は使わない）
   reworkOf: 期間 [ws, we) に AI が実行したコミットのうち、3 日以内に同じリポジトリで同じファイルを変えた別のコミット（人でも AI でも）があったものの数。
     3 日たっていないコミット（今に近いもの）は、まだ決まらないので数えない。ファイルはコミットごとに先頭の一定数までしか持たないので、少なめに数えることがある
   keptOf: 期間に始まった Claude Code のセッションで AI が編集した行（足した行）のうち、そのセッションの AI のコミットに入った行の割合（セッションごとに編集した行を上限に） */
const REWORK_S = 3 * 86400;
let gitByRepoCache = null;
function gitByRepo(){ const G = META.git || [];
  if (gitByRepoCache && gitByRepoCache.g === G) return gitByRepoCache.m;
  const m = new Map(); G.forEach(c => { if (!m.has(c.repo)) m.set(c.repo, []); m.get(c.repo).push(c); }); m.forEach(xs => xs.sort((a, b) => a.t - b.t));
  gitByRepoCache = {g: G, m}; return m; }
// reworkFor は、コミット c のファイルを 3 日以内に変えた最初の別のコミット（なければ null）
function reworkFor(c){ if (!(c.files || []).length) return null; const fs = new Set(c.files.map(f => f.path));
  return (gitByRepo().get(c.repo) || []).find(d => d.hash !== c.hash && d.t > c.t && d.t <= c.t + REWORK_S && (d.files || []).some(f => fs.has(f.path))) || null; }
function reworkOf0(ws, we){
  const now = nowMs() / 1000, R = gitByRepo(), out = {n: 0, re: 0, xs: []};
  (META.git || []).forEach(c => { if (!c.ai || c.t < ws || c.t >= we || c.t + REWORK_S > now || !(c.files || []).length) return;
    const fs = new Set(c.files.map(f => f.path)), later = (R.get(c.repo) || []).find(d => d.hash !== c.hash && d.t > c.t && d.t <= c.t + REWORK_S && (d.files || []).some(f => fs.has(f.path)));
    out.n++; if (later){ out.re++; out.xs.push({c, by: later}); } });
  return out; }
function reworkOf(ws, we){ return memoBasis("rework", ws, we, "", () => reworkOf0(ws, we)); }
function keptOf0(ws, we){
  const by = new Map(); (META.git || []).forEach(c => { if (c.ai && c.session) by.set(c.session, (by.get(c.session) || 0) + (c.added || 0)); });
  const xs = DATA.filter(s => s.source === "Claude Code" && s.start >= ws && s.start < we && (s.editAdded || 0) > 0);
  const edit = xs.reduce((a, s) => a + s.editAdded, 0), kept = xs.reduce((a, s) => a + Math.min(s.editAdded, by.get(s.id) || 0), 0);
  return {n: xs.length, edit, kept}; }
function keptOf(ws, we){ return memoBasis("kept", ws, we, "", () => keptOf0(ws, we)); }
/* push の数：この PC の git の reflog から読んだ push（META.push）のうち、期間 [ws, we) のもの。
   push の記録が 1 つもないとき、いちばん古い push より前に終わった期間は null（0 と言わない。reflog は既定で 90 日しか残らないので、古い期間の 0 で推移やいつもの範囲を下げないように） */
let pushFirst = null;
const pushesIn = (ws, we) => { const P = META.push || []; if (!P.length) return null;
  if (!pushFirst || pushFirst.p !== P) pushFirst = {p: P, t: P.reduce((m, x) => Math.min(m, x.t), Infinity)};
  return we <= pushFirst.t ? null : P.filter(x => x.t >= ws && x.t < we).length; };
/* push・PR・Git のコミットあたりの単価：期間の量（推定費用・クレジット・トークン）の合計 ÷ 数。どれも AI の分だけを取り出せないので、期間の合計で割る */
const usedOf = (w, k) => { const u = w.usage; if (!u) return null; const x = k === "cost" ? u.cost : k === "credits" ? u.credits : u.tokens; return x > 0 ? x : null; };
const perOut = (w, k, n) => { const x = usedOf(w, k); return n > 0 && x ? x / n : null; };
/* 指標の値（見直す候補の推移用）。low: 大きいときに印が付く向き（良し悪しの判定ではない） */
const pctOf = (a, b) => b ? Math.round(a * 100 / b) : null;
const MET = {
  limits: {f: (w, ws, we) => limitHits(ws, we).length, low: true, u: "", int: true},
  longctx: {f: (w, ws, we) => longCtxOf(ws, we).length, low: true, u: "", int: true},
  heavy: {f: (w, ws, we) => w.usage && w.usage.cost >= 0.01 ? pctOf(idleOf(ws, we).c, w.usage.cost) : null, low: true, u: "%", cap: "Share of estimated cost with no commit or PR"},
  cost: {f: w => w.usage ? w.usage.cost : null, low: true, fmt: v => usd(v)},
  costPerCommit: {f: w => w.costPerCommit, low: true, fmt: v => usd(v)},
  // コミットあたりのトークン：推定費用と同じく、Claude Code のセッション（期間に始まったもの）のトークン ÷ AI が実行したコミット
  tokensPerCommit: {f: (w, ws, we) => { const c = w.outputs && w.outputs.commits; if (!c) return null; const n = DATA.filter(s => s.source === "Claude Code" && s.start >= ws && s.start < we).reduce((a, s) => a + sesTok(s), 0); return n ? n / c : null; }, low: true, fmt: v => tok(v)},
  // 時間・セッション・プロンプトあたり（良し悪しの向きは付けない：1 時間あたりが高いのは、集中して任せた結果のこともある）
  costPerHour: {f: (w, ws, we) => perUse(ws, we, "cost", "hour"), fmt: v => usd(v)},
  creditsPerHour: {f: (w, ws, we) => perUse(ws, we, "credits", "hour"), fmt: v => cr1(v)},
  tokensPerHour: {f: (w, ws, we) => perUse(ws, we, "tokens", "hour"), fmt: v => tok(v)},
  costPerDay: {f: (w, ws, we) => perUse(ws, we, "cost", "day"), fmt: v => usd(v)},
  creditsPerDay: {f: (w, ws, we) => perUse(ws, we, "credits", "day"), fmt: v => cr1(v)},
  tokensPerDay: {f: (w, ws, we) => perUse(ws, we, "tokens", "day"), fmt: v => tok(v)},
  costPerSes: {f: (w, ws, we) => perUse(ws, we, "cost", "ses"), fmt: v => usd(v)},
  creditsPerSes: {f: (w, ws, we) => perUse(ws, we, "credits", "ses"), fmt: v => cr1(v)},
  tokensPerSes: {f: (w, ws, we) => perUse(ws, we, "tokens", "ses"), fmt: v => tok(v)},
  costPerAsk: {f: w => w.costPerAsk ?? null, fmt: v => usd(v)},
  creditsPerAsk: {f: (w, ws, we) => perUse(ws, we, "credits", "ask"), fmt: v => cr1(v)},
  tokensPerAsk: {f: (w, ws, we) => perUse(ws, we, "tokens", "ask"), fmt: v => tok(v)},
  modelfit: {f: (w, ws, we) => w.usage && w.usage.cost >= 0.01 ? pctOf(lightOf(ws, we).c, w.usage.cost) : null, low: true, u: "%"},
  fix: {f: w => w.fixRate, low: true, u: "%"},
  friction: {f: w => w.frictionN ?? w.friction.length, low: true, u: "", int: true},
  bigPrompts: {f: (w, ws, we) => bigOf(ws, we).n, low: true, u: "", int: true},
  cache: {f: w => w.usage && w.usage.cacheHit != null ? Math.round(w.usage.cacheHit * 100) : null, low: false, u: "%"},
  outSessions: {f: w => pctOf(w.outSessions, w.outBase ?? w.sessions), low: false, u: "%"},
  switches: {f: w => w.switchesAvg, low: true, u: ""},
  focus: {f: w => w.focus.length, low: false, u: "", int: true},
  wait: {f: w => w.waitP90, low: true, fmt: v => secs(Math.round(v)), cap: "90th percentile"},
  // 見直す候補にはならないが、「?」の説明に推移を出す指標（試したことが効いたかを、印が付いていなくても確かめられるように）
  active: {f: w => w.active, fmt: v => dur(v)},
  prompts: {f: w => w.prompts, u: "", int: true},
  tokens: {f: w => w.usage && w.usage.tokens ? w.usage.tokens : null, fmt: v => tok(v)},
  credits: {f: w => w.usage && w.usage.credits ? w.usage.credits : null, low: true, fmt: cr}, // 推定費用と同じく、多いときに印（クレジットは使う枠）
  gitCommits: {f: w => w.git ? w.git.commits : null, u: "", int: true},
  gitLines: {f: w => w.git && w.git.commits ? w.git.added + w.git.removed : null, fmt: v => commas(Math.round(v)), int: true},
  prs: {f: w => w.outputs ? w.outputs.prs || 0 : null, u: "", int: true},
  merges: {f: w => w.outputs ? w.outputs.merges || 0 : null, u: "", int: true}, // AI がマージした PR
  reviews: {f: w => w.outputs ? w.outputs.reviews || 0 : null, u: "", int: true}, // AI が出したレビュー（承認も入る）
  approvals: {f: w => w.outputs ? w.outputs.approvals || 0 : null, u: "", int: true}, // そのうち承認
  pushes: {f: (w, ws, we) => pushesIn(ws, we), u: "", int: true},
  // push・PR あたり（良し悪しの向きは付けない）：期間の合計 ÷ push・AI が作った PR の数
  costPerPush: {f: (w, ws, we) => perOut(w, "cost", pushesIn(ws, we)), fmt: v => usd(v)},
  creditsPerPush: {f: (w, ws, we) => perOut(w, "credits", pushesIn(ws, we)), fmt: v => cr1(v)},
  tokensPerPush: {f: (w, ws, we) => perOut(w, "tokens", pushesIn(ws, we)), fmt: v => tok(v)},
  costPerPr: {f: w => perOut(w, "cost", w.outputs && w.outputs.prs), fmt: v => usd(v)},
  creditsPerPr: {f: w => perOut(w, "credits", w.outputs && w.outputs.prs), fmt: v => cr1(v)},
  tokensPerPr: {f: w => perOut(w, "tokens", w.outputs && w.outputs.prs), fmt: v => tok(v)},
  // マージ・レビューあたり（PR あたりと同じく、期間の合計 ÷ AI がしたマージ・出したレビューの数）
  costPerMerge: {f: w => perOut(w, "cost", w.outputs && w.outputs.merges), fmt: v => usd(v)},
  creditsPerMerge: {f: w => perOut(w, "credits", w.outputs && w.outputs.merges), fmt: v => cr1(v)},
  tokensPerMerge: {f: w => perOut(w, "tokens", w.outputs && w.outputs.merges), fmt: v => tok(v)},
  costPerReview: {f: w => perOut(w, "cost", w.outputs && w.outputs.reviews), fmt: v => usd(v)},
  creditsPerReview: {f: w => perOut(w, "credits", w.outputs && w.outputs.reviews), fmt: v => cr1(v)},
  tokensPerReview: {f: w => perOut(w, "tokens", w.outputs && w.outputs.reviews), fmt: v => tok(v)},
  // Git のコミットあたり（良し悪しの向きは付けない）：期間の合計 ÷ 期間の Git のコミット（出力のパネルと同じ数）。クレジットと、AI のコミットがない期間の費用・トークンに使う
  costPerGitCommit: {f: w => perOut(w, "cost", w.git && w.git.commits), fmt: v => usd(v)},
  creditsPerGitCommit: {f: w => perOut(w, "credits", w.git && w.git.commits), fmt: v => cr1(v)},
  tokensPerGitCommit: {f: w => perOut(w, "tokens", w.git && w.git.commits), fmt: v => tok(v)},
  rework: {f: (w, ws, we) => { const r = reworkOf(ws, we); return r.n ? Math.round(r.re * 100 / r.n) : null; }, low: true, u: "%"},
  kept: {f: (w, ws, we) => { const k = keptOf(ws, we); return k.edit ? Math.round(k.kept * 100 / k.edit) : null; }, low: false, u: "%"},
  aiLines: {f: (w, ws, we) => memoBasis("lines", ws, we, "", () => DATA.reduce((x, s) => s.start >= ws && s.start < we && s.nFiles > 0 ? x + (s.editAdded || 0) + (s.editRemoved || 0) : x, 0)), fmt: v => commas(Math.round(v)), int: true},
};
const fmtM = (k, v) => v == null ? "—" : MET[k].fmt ? MET[k].fmt(v) : `${Math.round(v * 10) / 10}${MET[k].u ?? ""}`; // 本文と同じく「8 件」「5.2 回」
function periodBack(i){ // 表示中の期間から i 個前の期間
  if (st.mode === "month"){ const a = new Date(st.month.getFullYear(), st.month.getMonth() - i, 1), b = new Date(a.getFullYear(), a.getMonth() + 1, 1);
    return {S: MONTHS[mkey(a)], ws: a.getTime()/1000, we: b.getTime()/1000, label: periodLabel("month", mkey(a)), key: mkey(a)}; }
  const a = addDays(st.week, -7 * i);
  return {S: WEEKS[key(a)], ws: a.getTime()/1000, we: addDays(a, 7).getTime()/1000, label: periodLabel("week", key(a)), key: key(a)};
}
function periodLabel(mode, k){ const [y, m, d] = k.split("-").map(Number); // 期間の名前（"2026-05" か "2026-05-04"）
  return mode === "month" ? dMY(new Date(y, m-1, 1)) : t("week of {date}", {date: dMD(new Date(y, m-1, d))}); } // 画面の言語で
function metricAt(k, p){ if (!MET[k] || !p.S) return null; const v = MET[k].f(p.S, p.ws, p.we); return v == null || Number.isNaN(v) ? null : v; }
// 推移は、見出しのグラフと同じ期間（過去 INS_N と表示中の期間）
const SPN = 12;
function spark(k, plain){ // 8 期間の推移（記録のない期間は飛ばす。記録のある最初の期間から、幅いっぱいに描く）。plain は「?」の説明に添えるもの（見直す向きは書かない）
  if (!MET[k]) return "";
  // 記録の始まった週・月（途中から始まった期間）は、サマリーのグラフと同じく値を出さない（数日分だけの小さな値を、ほかの期間と並べない）
  const hs = historyStart(), pts = []; for (let i = SPN; i >= 0; i--){ const p = periodBack(i), mid = p.ws < hs && p.we > hs; pts.push({v: p.ws < hs ? null : metricAt(k, p), l: p.label, mid}); }
  // 表示中の期間がまだ途中なら、そう書いて点を白抜きにする（数える指標は、途中の期間だけ小さく見えるので）
  const part = Date.now() / 1000 < periodBack(0).we; if (part) pts[SPN].l = t("{period} (so far)", {period: pts[SPN].l});
  const vs = pts.filter(p => p.v != null).map(p => p.v); if (vs.length < 2) return "";
  // 使い始めて間もないなど、はじめの期間に記録がないときは、その期間を描かずに記録のある最初の期間を左端にする（線が箱の途中から始まって短く見えないように）
  const W = 168, H = 34, f0 = pts.findIndex(p => p.v != null || p.mid), step = (W - 8) / (SPN - f0); // 値が 2 つ以上あるので f0 は SPN − 1 以下。記録の始まった期間は「—」で左端に（サマリーのグラフと同じ）
  const lo = Math.min(...vs), hi = Math.max(...vs), rng = lo === hi ? fmtM(k, lo) : `${fmtM(k, lo)}${isJA() ? "〜" : "–"}${fmtM(k, hi)}`, x = i => 4 + (i - f0) * step, y = v => hi === lo ? H / 2 : H - 4 - (v - lo) / (hi - lo) * (H - 8);
  const line = pts.map((p, i) => p.v == null ? null : `${x(i).toFixed(1)},${y(p.v).toFixed(1)}`).filter(Boolean).join(" ");
  const list = pts.map(p => `${p.l} ${fmtM(k, p.v)}${p.mid ? paren(insMidS()) : ""}`).join(LSEP()), txt = st.mode === "month" ? t("{n}-month trend: {list}", {n: SPN + 1, list}) : t("{n}-week trend: {list}", {n: SPN + 1, list});
  const hits = pts.map((p, i) => i < f0 ? "" : `<rect class="hit" x="${(x(i) - step / 2).toFixed(1)}" y="0" width="${step.toFixed(1)}" height="${H}"${tipAttr(p.l, p.mid ? insMidT() : fmtM(k, p.v) === "—" ? t("No records") : fmtM(k, p.v))}/>`).join(""); // 点ごとに、その期間の値を出す
  // 両端（記録のある最初の期間と、表示中の期間）には、触れなくても読めるよう期間と値を添える。記録の始まった期間（値を出さない）は「Jul 20 (partial)」と、わけを目に見える形で
  const sl = i => { const p = periodBack(SPN - i), [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? (isJA() ? `${mm}月` : MON[mm - 1]) : dMD(new Date(yy, mm - 1, dd)); }; // 日本語の画面では「10月」（insight.js の月の目盛りと同じ）
  return `<span class="spark"><span class="sr">${esc(txt)}</span><span class="spk" aria-hidden="true"><svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"><polyline points="${line}"/>${pts.map((p, i) => p.v == null ? "" : `<circle class="pt" cx="${x(i)}" cy="${y(p.v)}" r="1.6"/>`).join("")}${pts[SPN].v != null ? `<circle${part ? ' class="part"' : ""} cx="${x(SPN)}" cy="${y(pts[SPN].v)}" r="2.6"/>` : ""}${hits}</svg><span class="ends"><span>${sl(f0)}${pts[f0].mid ? esc(insMidL()) : ` ${fmtM(k, pts[f0].v)}`}</span><span>${sl(SPN)}${part ? esc(t(" (so far)")) : ""} <b>${fmtM(k, pts[SPN].v)}</b></span></span></span><small aria-hidden="true">${esc((MET[k].cap ? `${t(MET[k].cap)} · ` : "") + (plain ? (st.mode === "month" ? t("{n}-month range {range}", {n: SPN + 1, range: rng}) : t("{n}-week range {range}", {n: SPN + 1, range: rng})) : `${GOTO[k] ? `${t(HELP[k].n)} · ` : ""}${st.mode === "month" ? (MET[k].low ? t("{n}-month range {range} · higher is worth a look", {n: SPN + 1, range: rng}) : t("{n}-month range {range} · lower is worth a look", {n: SPN + 1, range: rng})) : (MET[k].low ? t("{n}-week range {range} · higher is worth a look", {n: SPN + 1, range: rng}) : t("{n}-week range {range} · lower is worth a look", {n: SPN + 1, range: rng}))}`))}</small></span>`;
} // 向き（高いほど・低いほど見直す）だけを書く。印が付いた基準（決まった値）は、ダイアログの見出しのすぐ下に出す
/* 見直す候補：指標が決まった基準を超えたものを拾う（AI は使わない。判定ではなく、確かめる候補） */
// 見直す候補の文（L）は、画面に出す今の言語の文。鍵は英語の文で、{name} を埋める
const fillEn = (s, v) => s.replace(/\{(\w+)\}/g, (m, k) => Object.prototype.hasOwnProperty.call(v, k) ? String(v[k]) : m);
// 見直す候補は、次に何を変えればよいかがはっきりするものだけ：利用上限・修正の多さ（またはつまずき）・長くなった会話・モデルの選び方・
// 何も残さなかったセッションの費用・繰り返したプロンプト（費用の増え方・キャッシュ・切り替えなどは数字としてサマリーにある）
function findList(w, pw, unit){
  const F = [], u = w.usage || {}, o = w.outputs || {}, {ws, we} = period();
  const M = unit === "月", V = vsPrev(pw, unit);
  // 期間の言い方と、数と名詞（plural は日本語の画面で日本語になる）
  const cur = soFar() != null; // 表示中の期間が今の週・月でなければ「今週」と言わない
  const loc = {period: cur ? (M ? t("this month") : t("this week")) : M ? t("in the month shown") : t("in the week shown"), last: M ? t("last month") : t("last week")}; loc.prev = V.n == null ? loc.last : V.range;
  const tx = (s, v = {}) => t(s, {...loc, ...v});
  const ses = n => ({sessions: plural(n, "session")});
  const add = (k, score, see, why, rule, ids) => F.push({k, score, ids: ids || [], L: {see, why, rule}});
  const LH = limitHits(ws, we);
  if (LH.length){
    const hl = (rs, hs) => hs.slice(-3).map(h => h.r ? fillEn(rs, {time: `${md(h.t)} ${hm(h.t)}`, r: h.r}) : `${md(h.t)} ${hm(h.t)}`).join(TSEP()) + (hs.length > 3 ? TSEP() + "…" : "");
    const v = {n: LH.length, list: hl(t("{time} (resets {r})"), LH)};
    add("limits", 40, LH.length === 1 ? tx("Hit the usage limit once: {list}", v) : tx("Hit the usage limit {n} times: {list}", v), tx("Hitting a limit stops your work until it resets. The cause is often in how you worked just before"), tx("1+ hit"), [...new Set(LH.map(h => h.s.id))].reverse());
  }
  if (w.fixRate != null && w.prompts >= 10 && w.fixRate >= 20)
    add("fix", w.fixRate, tx("{pct}% of prompts had corrections or interruptions", {pct: w.fixRate}), tx("When a first prompt misses, redoing it costs time and tokens"), tx("20% or more, with 10+ prompts"), w.friction.map(f => f.id));
  else if (w.friction.length)
    add("friction", 18, tx("{sessions} with possible friction", ses(w.frictionN ?? w.friction.length)), tx("Rework is concentrated in a few sessions. Opening them usually hints at the cause"), tx("A session with 1+ correction or interruption, or 15+ prompts"), w.friction.map(f => f.id));
  const LC = longCtxOf(ws, we);
  if (LC.length){
    add("longctx", 34, tx("{sessions} where the conversation grew long and the input read per response rose to {x}× the first part (peak {peak} tokens)", {...ses(LC.length), x: Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0]))), peak: tok(Math.max(...LC.map(s => s.ctx[2])))}), tx("The longer a conversation, the more earlier context each response rereads, so similar prompts get heavier"), tx("Later input 4×+ the first part, peak at half the model's context window or more (100K+ tokens when the window is unknown), $0.5+"), LC.map(s => s.id));
  }
  const LT = lightOf(ws, we);
  if (u.cost >= 2 && LT.c >= Math.max(1, u.cost * 0.1))
    add("modelfit", 24, tx("Short sessions with no edits spent {cost} ({pct}% of estimated cost) on Opus-, Fable- or Mythos-class models", {cost: usd(LT.c), pct: Math.round(LT.c*100/u.cost)}), tx("For research and questions, a lighter model is often enough"), tx("Sessions mainly on these models with 3 or fewer prompts, no file edits and $0.3 or more each; together $1 or more and 10% or more of the period's estimated cost (when it is $2 or more)"), LT.xs.map(s => s.id));
  const {xs: idle, c: idleC} = idleOf(ws, we);
  if (u.cost >= 2 && idleC >= u.cost * 0.4)
    add("heavy", 30 + Math.round(idleC*50/u.cost), tx("{pct}% of estimated cost ({cost}) went to sessions with no commit or pull request", {pct: Math.round(idleC*100/u.cost), cost: usd(idleC)}), tx("Fine for research or discussion. If they stopped partway, it's worth finding out why"), tx("Sessions of $1+ total 40%+ of all cost ($2+)"), idle.map(s => s.id));
  const RP = repeatsOf(ws, we);
  if (RP.length)
    add("repeats", 9, tx("You wrote a similar prompt {n} times across {s} sessions {period} (\"{text}\")", {n: RP[0].n, s: RP[0].ids.size, text: snipOf(RP[0].text, 40)}), tx("A prompt you type every time can be written once as a command or in the agent's project instructions (CLAUDE.md, AGENTS.md or Kiro steering)"), tx("{min}+ characters, in {ses}+ sessions", {min: REPEAT_MIN, ses: REPEAT_SES}), [...new Set([RP[0].id, ...RP[0].ids])]);
  return F.sort((a,b) => b.score - a.score);
}
const GOTO = {longctx: "heavy", modelfit: "models"}; // 専用の指標がない候補は、関係する指標に印を付ける
function sesFlags(s){ // 詳細の頭に、このセッションが関係する見直す候補（表示中の期間のもの）の名前だけを出す。押すとサマリーと同じダイアログ
  const ids = [...new Set(lastFlags.filter(f => f.ids.includes(s.id)).map(f => GOTO[f.k] || f.k))];
  return ids.length ? `<div class="sflags"><span class="lbl" title="${esc(t("This session is one of the sessions behind these flags"))}">${ico("flag", "fdot")}${esc(t("Worth a look"))}</span>${ids.map(id => `<button class="flink" data-goto="${id}" aria-haspopup="dialog">${esc(H()[id].n)}</button>`).join("")}</div>` : "";
}
// line は、カードに添える 1 行（id → 文字）。繰り返しのプロンプトでは、そのセッションで書いたもの
// 候補のセッションには、なぜ重い（長い）のかの手がかりに、プロンプトの数・推定費用・作業時間を添える
const flagSes = (ids, line) => ids.map(id => { const s = DATA.find(x => x.id === id), l = line && line.get(id); return s ? sesCardH(s, sesMeta(s, s.corrections ? esc(plural(s.corrections, "correction")) : "", s.interrupts ? esc(plural(s.interrupts, "interruption")) : "", esc(plural(s.nPrompts || 0, "prompt")), s.cost >= 0.005 ? esc(usd(s.cost)) : "", esc(dur(minIn(s, -Infinity, Infinity)))) + (l ? `<span class="fq">${esc(snipOf(l, 90))}</span>` : "")) : ""; }).join("");
/* 見直す候補の 1 つを、画面の真ん中のダイアログで開く（サマリーへ移らずに、何が見えて、なぜ大事で、次に何をするかを読める）。
   同じ指標に印の付く候補（GOTO）はまとめて出す。セッションを押すと、ダイアログを閉じて詳細を開く */
let lastFlags = []; // 表示中の期間の候補（summary が入れる）
function openWorth(id, from, F = lastFlags){
  const D = $("#wk"), fs = F.filter(f => (GOTO[f.k] || f.k) === id), h = H()[id]; if (!fs.length || !h) return;
  const ids = [...new Set(F.map(f => GOTO[f.k] || f.k))]; // 前後の候補へ（「まず見直す」から開いても、ほかの候補を順に見られるように）
  const hk = fs.length === 1 && H()[fs[0].k] ? H()[fs[0].k] : h; // 印が 1 つだけなら、その候補自身の説明（長い会話で開いたときに、重いセッションの説明を出さないように）
  const {ws, we} = period(), r0 = id === "repeats" && repeatsOf(ws, we)[0], rp = r0 ? new Map([...r0.by].filter(([, x]) => x !== r0.text)) : null; // 見出しと同じ書き方は、カードごとに繰り返さない
  const body = fs.map(f => `<div class="fl"><p class="see">${ico("flag", "fdot")}${esc(f.L.see)}</p><p class="rule">${esc(t("Threshold: {rule}", {rule: f.L.rule}))}</p>${f.k === "repeats" ? "" : `<p class="why">${esc(f.L.why)}</p>`}${spark(f.k)}${f.ids.length ? `<div class="fss">${flagSes(f.ids.slice(0, 6), f.k === "repeats" ? rp : null)}${f.ids.length > 6 ? `<div class="fss-rest" hidden>${flagSes(f.ids.slice(6), f.k === "repeats" ? rp : null)}</div><button type="button" class="more fmore">${esc(f.ids.length - 6 === 1 ? t("1 more session") : t("{n} more sessions", {n: f.ids.length - 6}))}</button>` : ""}</div>` : ""}</div>`).join(""); // 繰り返しのプロンプトの「なぜ」は「What to try」と同じことなので出さない
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" id="wkclose" aria-label="${esc(t("Close"))}"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form><div class="eyebrow">${ico("flag", "fdot")}${esc(t("Worth a look"))}${ids.length > 1 ? `<span class="wknav"><button type="button" class="iconbtn" data-nav="-1" aria-label="${esc(t("Previous"))}">‹</button><span>${esc(t("{i} of {n}", {i: ids.indexOf(id) + 1, n: ids.length}))}</span><button type="button" class="iconbtn" data-nav="1" aria-label="${esc(t("Next"))}">›</button></span>` : ""}</div><h2 id="wkh">${esc(hk.n)}</h2><p class="wktry">${ico("ai", "ti")}<b>${esc(t("What to try"))}</b> ${esc(hk.a)}</p>${body}
    <dl class="wkhelp"><dt>${esc(t("What it is"))}</dt><dd>${esc(hk.d)}</dd><dt>${esc(t("Doesn't tell you"))}</dt><dd>${esc(hk.x)}</dd></dl>`; // 次にやってみることは、見出しのすぐ下に（セッションの一覧より先に）
  const bindSes = el => el.querySelectorAll("[data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
  D.querySelectorAll(".fss").forEach(bindSes);
  // 「ほか N セッション」: 隠しておいた残りのセッションをその場に出す（フォーカスは出した最初のセッションへ）
  D.querySelectorAll(".fmore").forEach(b => b.onclick = () => { const rest = b.previousElementSibling; if (!rest) return;
    rest.hidden = false; b.remove(); const first = rest.querySelector("[data-id]"); if (first) first.focus(); });
  D.onclose = () => { if (from && from.isConnected && (!st.sel || from.closest("#panel")) && !document.activeElement?.closest?.("#review")) from.focus(); }; // 閉じたら押したリンクへ（サマリーや別のセッションへ移ったときは除く）
  D.querySelectorAll("[data-nav]").forEach(b => b.onclick = () => { const i = ids.indexOf(id), next = ids[(i + +b.dataset.nav + ids.length) % ids.length];
    (F === lastFlags ? insOpenWorth : (x, fr) => openWorth(x, fr, F))(next, from); const nb = D.querySelector(`[data-nav="${b.dataset.nav}"]`); if (nb) nb.focus(); });
  if (!D.open) D.showModal(); D.scrollTop = 0;
  const hd = $("#wkh"); hd.tabIndex = -1; hd.focus(); } // 最初のボタンではなく見出しへ（読み上げで何のダイアログかわかり、セッションを押した表示にもならない）
function placeFlags(R, F){ // 基準を超えた指標の名前の横に、小さな印だけを付ける（押すと「Worth a look」と同じダイアログ。中身はダイアログにだけ出し、サマリーで繰り返さない）
  [...new Set(F.map(f => GOTO[f.k] || f.k))].forEach(id => {
    // 「?」のない候補（コミットにつながらなかった費用など）は、関係する図の見出しの印の場所（data-flag）に置く
    // モデルの選び方（GOTO の models）は、投入のパネルのモデル別のドーナツか、内訳の「どこに使ったか」の見出しに
    const vis = h => !h.closest("[hidden]"), hs = [...R.querySelectorAll(`.insight .hb[data-help="${id}"], .panel .hb[data-help="${id}"]`)], fs = [...R.querySelectorAll(`.insight [data-flag="${id}"]`)];
    const hb = hs.find(vis) || hs[0] || fs.find(vis) || fs[0]; if (!hb) return; // パネルの切り替えで隠れた選択肢の「?」より、見えているものに
    hb.insertAdjacentHTML("beforebegin", `<button class="fmark" data-goto="${id}" aria-haspopup="dialog" aria-label="${esc(t("Worth a look: {name}", {name: H()[id].n}))}" title="${esc(t("Crossed a threshold. Click to see why."))}">${ico("flag", "fdot")}</button>`);
    // 閉じた折りたたみの中の印は、見出しにも出す（開かずに、中に印があるとわかるように）。折りたたみが重なっていれば（③ からのパネルの中の「もっと見る」）、どれの見出しにも
    for (let dt = hb.closest("details"); dt; dt = dt.parentElement.closest("details")){ const sm = dt.querySelector(":scope > summary");
      if (sm && !sm.contains(hb) && !sm.querySelector(".fdot")) sm.insertAdjacentHTML("beforeend", `<span title="${esc(t("A metric crossed a threshold"))}">${ico("flag", "fdot")}</span>`); }
  });
}
