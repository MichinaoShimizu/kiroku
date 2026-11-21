// サマリーの頭：要点の文・推移のグラフ（自分のいつもの範囲つき）・改善のアドバイス。AI は使わず、決まった計算と規則だけで出す
/* いつもの範囲：表示中の期間の前の INS_N 期間（記録が始まる前の期間と、記録が途中から始まる期間は除く）の分布。
   中央値と四分位（真ん中の 50%）で見る。平均と標準偏差は、外れた週 1 つで大きく動くので使わない */
const INS_N = 12, INS_MIN = 4; // 比べる期間の数と、比べるのに要る最低の数（それより少なければ「まだ比べられない」と言う）
const INS_ADD = new Set(["cost", "credits", "active", "prompts", "tokens", "gitCommits", "limits", "longctx", "friction", "bigPrompts", "focus"]); // 期間の長さに比例して増える指標（途中の期間は、このペースで期間を終えたときの値で比べる）
const INS_KEYS = ["cost", "credits", "tokens", "active", "costPerCommit", "creditsPerCommit", "tokensPerCommit", "costPerEdit", "creditsPerEdit", "tokensPerEdit", "cache", "fix", "wait"]; // グラフで切り替えて見られる指標
const INS_SHORT = () => ({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), active: t("Time"), costPerCommit: t("Cost per Git commit"), creditsPerCommit: t("Credits per Git commit"), tokensPerCommit: t("Tokens per Git commit"), costPerEdit: t("Cost per 100 AI lines"), creditsPerEdit: t("Credits per 100 AI lines"), tokensPerEdit: t("Tokens per 100 AI lines"), cache: t("Cache"), fix: t("Corrections"), wait: t("Wait")}); // 切り替えの短い名前（正式な名前は title と、グラフの下の文）
// insNice は、目盛りの上端を切りのよい数（1・2・2.5・5・10 の 10 の累乗倍）に
function insNice(v){ if (!(v > 0)) return 1; const e = Math.pow(10, Math.floor(Math.log10(v))); return [1, 2, 2.5, 5, 10].map(m => m * e).find(x => x >= v); }
function insQuant(xs, q){ const s = [...xs].sort((a, b) => a - b), i = (s.length - 1) * q, lo = Math.floor(i), hi = Math.ceil(i); return s[lo] + (s[hi] - s[lo]) * (i - lo); }
// insFrac は、表示中の期間のうち過ぎた割合（終わった期間は 1）。日ごとではなく時刻で（月曜の朝に 1 日分と数えて大きく割らないように、最低 1 日分）
function insFrac(){ const {ws, we} = period(), n = soFar(); return n == null ? 1 : n / Math.round((we - ws) / 86400); } // 今日を含めた日数 ÷ 期間の日数（月末の推定 projection と同じ数え方）
// insBase は、指標 k のいつもの範囲と、表示中の期間の位置。v は比べる値（途中の期間の、比例する指標はペース換算）
function insBase(k){
  const cur = metricAt(k, periodBack(0)); if (cur == null) return null;
  return insRange(k, cur, p => metricAt(k, p), MET[k].low, INS_ADD.has(k), v => fmtM(k, k === "wait" ? Math.round(v) : v)); // low が決まっていない指標（時間・プロンプト・トークン・コミット）は、よし悪しの色を付けない
}
// insRange は、値の関数 f（期間 → 値）で、表示中の期間の値 cur をいつもの範囲と比べる。MET にない値（勤務時間外の割合など）にも使う
function insRange(k, cur, f, low, add, fmt){
  const fr = insFrac(), v = add && fr < 1 ? cur / fr : cur, hs = historyStart(), xs = [];
  for (let i = 1; i <= INS_N; i++){ const p = periodBack(i); if (p.ws < hs) continue; const x = f(p); if (x != null) xs.push(x); }
  const r = {k, cur, v, part: fr < 1 && add, n: xs.length, xs, fmt};
  if (xs.length < INS_MIN) return r;
  const q1 = insQuant(xs, .25), med = insQuant(xs, .5), q3 = insQuant(xs, .75), iqr = q3 - q1;
  const pos = v > q3 ? "high" : v < q1 ? "low" : "typical", far = v > q3 + 1.5 * iqr || v < q1 - 1.5 * iqr;
  const better = pos === "typical" || low == null ? null : (pos === "low") === low; // 範囲の外で、よい側か（向きの決まっていない指標は null）
  return {...r, q1, med, q3, pos, far, better, rank: Math.round(xs.filter(x => x < v).length * 100 / xs.length)};
}
const insName = k => (H()[k] || {}).n || k;
const insNoun = () => st.mode === "month" ? (JA ? "月" : "months") : (JA ? "週" : "weeks"); // 数を伴わない「過去の週・月」（日本語は「か月」にしない）
const insUnit = n => n === 1 && !JA ? (st.mode === "month" ? "month" : "week") : st.mode === "month" ? t("months") : t("weeks"); // 日本語は数によらず「か月」「週」
const say = (s, then) => (s ? `<p class="isay">${esc(s)}</p>` : "") + (then ? `<p class="ithen">→ ${esc(then)}</p>` : ""); // グラフの読み取り（データから計算した 1 文。見出しのすぐ下）
// 式の表示：値・式は <code class="ifx"> に入れる（文ではなく、計算だとわかるように）
// 式は「式を表示」のときだけ見せる（.fxo。ふだんは隠す）。plain があれば、ふだんはその平易な 1 行を出す（.fxp）
const fx = (s, cls = "", plain = "") => `<code class="ifx fxo${cls}">${esc(s)}</code>` + (plain ? `<span class="fxp${cls}">${esc(plain)}</span>` : "");
const pln = s => `<span class="iplain">${esc(s)}</span>`; // 式ではない短い文（いつも見せる）
// 四分位は記号でなく言葉で：下位25%点・上位25%点・幅（統計に慣れていなくても読めるように）
const LO = () => t("25th pct"), HI = () => t("75th pct"), SP = () => t("spread");
const insDef = n => n == null ? t("{lo}, {hi} = the values a quarter of the way from the bottom and from the top when your past {unit} with records (up to {n}) are sorted; half of them fall between. {sp} = {hi} − {lo}", {n: INS_N, unit: insNoun(), lo: LO(), hi: HI(), sp: SP()})
  : t("{lo}, {hi} = the values a quarter of the way from the bottom and from the top of your last {n} {unit}; half of them fall between. {sp} = {hi} − {lo}", {n, unit: insUnit(n), lo: LO(), hi: HI(), sp: SP()});
// insExpr は、比べた結果を不等式で（x は表示中の期間の値。途中の期間の、期間に比例する指標はペース）
function insExpr(b){
  if (!b || b.q1 == null) return t("To compare: needs {min} past {unit} (have {n})", {min: INS_MIN, unit: insUnit(INS_MIN), n: b ? b.n : 0});
  const fm = b.fmt || (x => fmtM(b.k, x)), x = (b.part ? t("pace") + " " : b.k === "wait" ? "P90 " : "") + fm(b.v), iqr = b.q3 - b.q1; // 待ち時間は 90 パーセンタイルで比べる
  if (b.pos === "typical") return `${LO()} ${fm(b.q1)} ≤ ${x} ≤ ${HI()} ${fm(b.q3)}`;
  if (b.pos === "high") return b.far ? `${x} > ${HI()} + 1.5×${SP()} = ${fm(b.q3 + 1.5 * iqr)}` : `${x} > ${HI()} = ${fm(b.q3)}`;
  return b.far ? `${x} < ${LO()} − 1.5×${SP()} = ${fm(b.q1 - 1.5 * iqr)}` : `${x} < ${LO()} = ${fm(b.q1)}`;
}
const insPosText = b => `${insExpr(b)} (${insDef(b && b.n)})`; // 読み上げと title 用
const insTone = b => !b || b.q1 == null || b.better == null ? "" : (b.better ? " good" : " bad") + (b.far ? " far" : ""); // 良い側・悪い側に外れたか（大きく外れたら far）。タイルの背景の色に使う
// insPlain は、いつもの範囲との比べを言葉で（式を表示しないときのチップ）
function insPlain(b){
  if (!b || b.q1 == null) return insExpr(b);
  const fm = b.fmt || (x => fmtM(b.k, x)), x = (b.part ? t("pace") + " " : b.k === "wait" ? "P90 " : "") + fm(b.v), r = {x, lo: fm(b.q1), hi: fm(b.q3)};
  return b.pos === "typical" ? t("{x}: within your usual {lo}–{hi}", r) : b.pos === "high" ? (b.far ? t("{x}: far above your usual {lo}–{hi}", r) : t("{x}: above your usual {lo}–{hi}", r))
    : b.far ? t("{x}: far below your usual {lo}–{hi}", r) : t("{x}: below your usual {lo}–{hi}", r);
}
const insChip = b => `<code class="ichip ifx${insTone(b)}" title="${esc(insDef(b && b.n))}"><span class="cx">${esc(insExpr(b))}</span><span class="cp">${esc(insPlain(b))}</span></code>`;
/* 要点の数字（最大 3 列）：上の帯と同じ数をサマリーでも、式といつもの範囲との不等式つきで。並びは判断に使う順：
   使った額（推定費用・クレジット、なければトークン）→ 予算に対する位置（前の期間並みに収める 1 日の目安／月末の推定／前の期間との差）→ 稼働時間 → 効率（コミットあたり費用）→ まず見直すもの */
function insHeadlines(F){
  const {S: w, P: pw, ws, we} = period(), T = {cost: [], outGit: [], outEdit: [], unitGit: [], unitEdit: [], unitAsk: []}, u = (w && w.usage) || {}, M = st.mode === "month";
  let G = "cost"; // コスト（時間・クレジット・金額・トークン）・出力（Git と AI の編集）・単価（コスト ÷ 出力。割る数の元ごと）に分けて並べる
  const tile = (label, b, v, f, m) => T[G].push(`<div class="itile${insTone(b)}"><span class="il">${esc(label)}</span>${m ? `<button type="button" class="itv" data-metric="${m}" aria-haspopup="dialog" aria-label="${esc(`${label} ${v}`)}">${esc(v)}</button>` : `<b>${esc(v)}</b>`}${[].concat(f || []).map(x => typeof x === "object" ? (x.n != null ? pln(x.n) : fx(x.f, "", x.p)) : x === t("Not an estimate") ? pln(x) : fx(x)).join("")}${b === false ? "" : insChip(b)}</div>`); // 値を押すと、上の帯と同じ内訳
  const days = Math.round((we - ws) / 86400), el = Math.round(insFrac() * days), V = vsPrev(pw, M ? "月" : "週");
  // 上の帯の増減（%）と同じ比べを式で：途中の期間は前の期間の同じ日まで、終わった期間は前の期間まるごと
  const vs = (a, b, f) => { if (b == null || !b) return ""; const pc = `${a >= b ? "+" : "−"}${Math.round(Math.abs(a - b) * 100 / b)}% ${V.n == null ? (M ? t("vs last month") : t("vs last week")) : t("vs {range}", {range: V.range})}`;
    return {f: `(${f(a)} − ${f(b)}) ÷ ${f(b)} = ${pc}`, p: pc}; };
  const pace = (cur, v, f) => ({f: `${t("pace")} = ${f(cur)} × ${days}${t("d")} ÷ ${el}${t("d")} = ${f(v)}`, p: t("At this pace: {v}", {v: f(v)})});
  const pu = pw && pw.usage;
  const bc = insBase("cost"); if (bc && (u.tokens || bc.cur)) tile(t("Estimated cost"), bc, fmtM("cost", bc.cur), [vs(bc.cur, V.of("cost", pu && pu.tokens ? costOf(pu) : null), usd), bc.part ? pace(bc.cur, bc.v, usd) : ""].filter(Boolean), "cost");
  if (u.credits){ const bk = insRange("credits", u.credits, p => p.S && p.S.usage ? p.S.usage.credits || null : null, true, true, cr);
    const mp = monthProj(), r1 = cr1; // 週の表示でも、クレジットの今月末の推定を添える（クレジットは月ごとの枠。カードは増やさない）
    const me = mp && mp.credits != null ? {f: `${t("Month-end credits")} = ${t("this month")} ${r1(mp.soCredits)} ÷ ${mp.days}${t("d")} × ${mp.nd}${t("d")} = ${r1(mp.credits)}`, p: t("Month-end (this month): ≈ {v}", {v: cr(mp.credits)})} : "";
    tile(t("Kiro credits"), bk, crN(u.credits), [vs(u.credits, V.of("credits", pu && pu.credits), r1)].concat(bk.part ? pace(u.credits, bk.v, r1) : t("Not an estimate"), me).filter(Boolean), "credits"); }
  else if (u.tokens) { const bt = insBase("tokens"); tile(t("Tokens"), bt, tok(u.tokens), `${t("output")} ${tok(u.out)} ÷ ${tok(u.tokens)} = ${Math.round(u.out * 100 / u.tokens)}%`, "tokens"); }
  // 予算に対する位置：途中の月は月末の推定、途中の週は前の週並みに収める 1 日の目安、終わった期間は前の期間との差
  const pj = projection(w), pc = pacing(w, pw), fmc = x => x === "credits" ? (cr1) : usd; // 式の中のクレジットは小数 1 桁（整数に丸めると、引き算の結果と合わなくなる）
  const pjf = (p, k, so) => `${fmc(k)(so)} ÷ ${p.days}${t("d")} × ${p.nd}${t("d")} = ${fmc(k)(p[k])}`; // 月末の推定の式（今日までの実績 ÷ 日数 × 月の日数）
  if (pj && (pj.cost != null || pj.credits != null)){ const ks = ["cost", "credits"].filter(k => pj[k] != null); // 推定費用とクレジットは別の枠なので、足さずに「 · 」で並べる（上の帯と同じ。内訳から、もう片方の内訳へ移れる）
    tile(t(ks.length > 1 ? "Month-end (estimate)" : ks[0] === "cost" ? "Month-end cost" : "Month-end credits"), false, ks.map(k => `≈ ${k === "credits" && ks.length === 1 ? crN(pj[k]) : fmc(k)(pj[k])}`).join(" · "), /* 「月末のクレジット」だけなら単位は見出しにある */ ks.map(k => pjf(pj, k, k === "cost" ? costOf(u) : u.credits)), ks[0] === "cost" ? "projection" : "projectionCr"); }
  else if (pc){ // 費用とクレジットのそれぞれについて：残りの日 1 日あたりの目安か、すでに超えた額（式で）
    const xs = ["cost", "credits"].filter(k => pc[k]).map(k => ({k, x: pc[k], f: fmc(k)})), ok = xs.find(o => o.x.rest >= 0), fs = xs.map(({x, f}) => x.rest >= 0 ? `(${f(x.ref)} − ${f(x.so)}) ÷ ${pc.left}${t("d")} = ${f(x.per)}/${t("d")}` : `${f(x.so)} − ${f(x.ref)} = +${f(-x.rest)} ${t("over")}`);
    if (ok) tile(M ? t("A day to match last month") : t("A day to match last week"), false, `${ok.f(ok.x.per)}/${t("d")}`, fs, ok.k === "cost" ? "pace" : "paceCr");
    else tile(M ? t("Over last month's total") : t("Over last week's total"), false, `+${xs[0].f(-xs[0].x.rest)}`, fs, xs[0].k === "cost" ? "pace" : "paceCr"); }
  else if (u.tokens && costOf(u) != null){ const a = costOf(u), dd = w.days.filter(d => d.active).length; // 終わった期間：使った日 1 日あたり（前の期間との差は推定費用のカードの式にある）
    if (dd) tile(t("Cost per active day"), false, `${usd(a / dd)}/${t("d")}`, `${usd(a)} ÷ ${dd}${t("d")} = ${usd(a / dd)}`); }
  const ba = insBase("active"); if (ba) tile(t("Active time"), ba, fmtM("active", ba.cur), [vs(ba.cur, V.of("active", pw && pw.active), v => dur(v))].concat(ba.part ? pace(ba.cur, ba.v, v => dur(v)) : "").filter(Boolean), "active");
  // 出力は、元の違う 2 つを混ぜずに分ける：Git（リポジトリから読んだコミット）と、AI の編集（エージェントの履歴の編集ツール。コミットしたかによらない）
  G = "outGit";
  const g = w && w.git && w.git.commits ? w.git : null, o = (w && w.outputs) || {};
  if (g) tile(t("Git commits"), insBase("gitCommits"), `${g.commits}`, {f: `AI ${g.ai} ÷ ${g.commits} = ${Math.round(g.ai * 100 / g.commits)}%`, p: t("{pc}% by AI", {pc: Math.round(g.ai * 100 / g.commits)})}, "commits");
  else if (o.commits) tile(t("AI commits"), false, `${o.commits}`, {n: t("Commits AI ran, from history")}, "aiCommits");
  // Git で数えられるほかの出力：変えた行・ファイル・リポジトリ・コミットのあった日・push（どれもリポジトリから。手でのコミットも入る）
  if (g){ const cs = gitIn(ws, we), lines = g.added + g.removed, n = commas;
    if (lines > 0) tile(t("Lines in Git commits"), false, n(lines), {f: `+${n(g.added)} −${n(g.removed)}; ${n(lines)} ÷ ${g.commits} = ${n(Math.round(lines / g.commits))} ${t("per commit")}`, p: `+${n(g.added)} −${n(g.removed)}`}, "lines");
    const fs = new Set(); let more = 0; cs.forEach(c => { (c.files || []).forEach(f => fs.add(`${c.repo}\u0000${f.path}`)); more += Math.max(0, (c.nFiles || 0) - (c.files || []).length); });
    if (fs.size + more) tile(t("Files in Git commits"), false, n(fs.size + more), {n: more ? t("includes about {n} from big commits that keep only their first files", {n: n(more)}) : t("Each file once")}); // 式ではない補足は、ふつうの文字で
    const rp = new Set(cs.map(c => c.repo || c.project)), dd = new Set(cs.map(c => key(new Date(c.t * 1000))));
    if (rp.size) tile(t("Repositories with commits"), false, `${rp.size}`, {n: [...new Set(cs.map(c => c.project))].slice(0, 3).join(LSEP()) + (rp.size > 3 ? "…" : "")});
    if (dd.size) tile(t("Days with commits"), false, `${dd.size}`, {f: `${dd.size} ÷ ${Math.round((we - ws) / 86400)}${t("d")}`, p: t("of {n}", {n: plural(Math.round((we - ws) / 86400), "day")})});
    const ps = (META.push || []).filter(x => x.t >= ws && x.t < we), sent = ps.reduce((a, x) => a + (x.commits || 0), 0);
    if (ps.length) tile(t("Pushes"), false, `${ps.length}`, sent ? {n: t("{n} commits sent", {n: n(sent)})} : ""); }
  if (o.prs) tile(t("Pull requests"), false, `${o.prs}`, {n: t("Created by AI")}, "prs");
  G = "outEdit";
  { const xs = DATA.filter(s => s.start >= ws && s.start < we && s.nFiles > 0), a = xs.reduce((x, s) => x + (s.editAdded || 0), 0), d = xs.reduce((x, s) => x + (s.editRemoved || 0), 0), E = insEditsBody();
    if (E && E.nf) tile(t("Files edited by AI"), false, commas(E.nf), {n: t("{n} sessions", {n: xs.length})});
    if (a + d > 0) tile(t("Lines edited by AI"), false, commas(a + d), {f: `+${commas(a)} −${commas(d)}`, p: `+${commas(a)} −${commas(d)}`}); }
  // 単価も、割る数の元ごとに分ける：Git のコミットあたり・AI が編集した 100 行あたり・プロンプトあたり
  G = "unitGit";
  const bp = insBase("costPerCommit"), oc = w && w.outCost, nc = w && w.outputs && w.outputs.commits;
  if (bp) tile(t("Estimated cost per Git commit"), bp, fmtM("costPerCommit", bp.cur), oc && nc ? `Claude Code ${usd(oc)} ÷ ${nc} ${t("AI commits")} = ${usd(bp.cur)}` : "", "costPerCommit");
  const bcc = insBase("creditsPerCommit"), gx = g ? g.commits - (g.ai || 0) : 0;
  if (bcc) tile(t("Kiro credits per Git commit"), bcc, `${Math.round(bcc.cur * 10) / 10}`, {f: `${cr1(u.credits)} ÷ ${gx} ${t("commits")} = ${fmtM("creditsPerCommit", bcc.cur)}`, p: t("÷ Git commits not run by Claude Code")});
  const btc = insBase("tokensPerCommit");
  if (btc) tile(t("Tokens per Git commit"), btc, fmtM("tokensPerCommit", btc.cur), nc ? `Claude Code ${t("tokens")} ÷ ${nc} ${t("AI commits")} = ${fmtM("tokensPerCommit", btc.cur)}` : "");
  G = "unitEdit";
  for (const [k, label, fu] of [["costPerEdit", t("Estimated cost per 100 AI-edited lines"), usd], ["creditsPerEdit", t("Kiro credits per 100 AI-edited lines"), cr1]]){ // トークンは推移だけ（数字の段を増やしすぎない）
    const b = insBase(k); if (!b) continue;
    const e = editsBasis(ws, we, k.replace("PerEdit", ""));
    tile(label, b, k === "creditsPerEdit" ? crN(b.cur) : fmtM(k, b.cur), {f: `${fu(e.used)} ÷ ${t("{n} AI-edited lines", {n: commas(e.lines)})} × 100 = ${fmtM(k, b.cur)}`, p: k === "creditsPerEdit" ? t("lines of the {n} sessions with Kiro credits", {n: e.n}) : t("lines of the {n} sessions with an estimated cost", {n: e.n})}); }
  G = "unitAsk";
  if (w && w.costPerAsk != null) tile(t("Estimated cost per prompt"), false, usd(w.costPerAsk), t("cost ÷ {n} prompts with token data", {n: commas(w.costPrompts)}), "costPerAsk");
  { const cs = DATA.filter(s => s.start >= ws && s.start < we && s.credits > 0), c = cs.reduce((a, s) => a + s.credits, 0), n = cs.reduce((a, s) => a + s.nPrompts, 0); // クレジットを記録したセッションの、プロンプトあたり
    if (c && n) tile(t("Kiro credits per prompt"), false, `${Math.round(c / n * 100) / 100}`, `${cr1(c)} ÷ ${n} ${t("prompts")} = ${Math.round(c / n * 100) / 100}`); }
  if (u.tokens && w && w.costPrompts) tile(t("Tokens per prompt"), false, tok(u.tokens / w.costPrompts), `${tok(u.tokens)} ÷ ${commas(w.costPrompts)} ${t("prompts")} = ${tok(u.tokens / w.costPrompts)}`);
  // カードは 4 列の格子に、中の数字の多い段の数だけ列を使って並べる（数字が少ないカードを横いっぱいに広げず、空いた横に次のカードを入れる）。
  // 最後のカードは、行の残りの列まで広げる（右に空きを残さない）
  // 小見出しのある段：出どころ（Git と AI の編集）が違う数字を、同じ枠の中でも段と印で分ける
  const sub = ([k, icon, label, note, empty]) => !T[k].length && empty ? `<div class="iksub ${k}" role="group" aria-label="${esc(label)}"><h6 class="iksh">${ico(icon)}${esc(label)}${note ? ` <small>${esc(note)}</small>` : ""}</h6><p class="none iknone">${esc(empty)}</p></div>` : T[k].length ? `<div class="iksub ${k}" role="group" aria-label="${esc(label)}"><h6 class="iksh">${ico(icon)}${esc(label)}${note ? ` <small>${esc(note)}</small>` : ""}</h6><div class="ikrow">${T[k].join("")}</div></div>` : "";
  const cards = [
    {label: t("Cost"), rows: null, k: "cost"},
    {label: t("Output"), rows: [["outGit", "commit", t("Git"), t("from your repositories"), T.outEdit.length ? t("No Git commits for this selection") : ""], ["outEdit", "edit", t("AI edits"), t("from the agents' history, committed or not")]]},
    {label: t("Unit cost"), rows: [["unitGit", "commit", t("Per Git commit"), ""], ["unitEdit", "edit", t("Per 100 AI-edited lines"), ""], ["unitAsk", "user", t("Per prompt"), ""]]},
  ].map(c => ({...c, n: c.rows ? Math.max(...c.rows.map(r => T[r[0]].length)) : T[c.k].length})).filter(c => c.n > 0);
  let col = 0; cards.forEach(c => { c.span = Math.min(4, c.n); if (col + c.span > 4) col = 0; c.col = col; col += c.span; });
  if (cards.length) { const z = cards[cards.length - 1]; z.span = 4 - z.col; }
  const card = c => `<section class="ikg sp${c.span}" aria-label="${esc(c.label)}"><h5 class="ikgh">${esc(c.label)}</h5>${c.rows ? c.rows.map(sub).join("") : `<div class="ikrow">${T[c.k].join("")}</div>`}</section>`;
  return `<div class="insh">${cards.map(card).join("")}</div><p class="ifxs insdef">${fx(insDef())}</p>`;
}
/* 推移のグラフ：前の INS_N 期間と表示中の期間の棒。いつもの範囲（四分位）を帯、中央値を点線で重ねる。
   途中の期間は、今の値の棒と、このペースで終えたときの値の枠。範囲から大きく外れた期間には印 */
// 目盛りの数は短く（クレジットは数だけ。単位は見出しの文にある。左の余白に収める）
const tickM = (k, v) => k === "credits" || k === "creditsPerCommit" || k === "creditsPerEdit" ? crN(v) : fmtM(k, v);
function insChart(k){
  const all = []; for (let i = INS_N; i >= 0; i--){ const p = periodBack(i); all.push({p, v: p.ws < historyStart() && i ? null : metricAt(k, p)}); }
  const f0 = Math.min(Math.max(0, all.findIndex(x => x.v != null)), all.length - 6), ps = all.slice(f0); // 記録の始まる前の期間は描かない（6 期間は残す）
  const b = insBase(k), cur = ps[ps.length - 1], proj = b && b.part ? b.v : null;
  const vals = ps.map(x => x.v).filter(v => v != null); if (!vals.length) return `<p class="note">${esc(t("No records"))}</p>`;
  // 狭い画面は小さな座標で描く（縮めて文字が読めなくならないように）
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 180 : 200, L = narrow ? 52 : 64, R = 8, T = 12, B = 26, top = insNice(Math.max(...vals, proj || 0, b && b.q3 || 0) * 1.05);
  const bw = (W - L - R) / ps.length, x = i => L + i * bw, y = v => T + (Hh - T - B) * (1 - v / top);
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 8}" y="${(y(v) + 4).toFixed(1)}">${esc(tickM(k, v))}</text></g>`).join("");
  const band = b && b.q1 != null ? `<rect class="iband" x="${L}" y="${y(b.q3).toFixed(1)}" width="${W - L - R}" height="${Math.max(1, y(b.q1) - y(b.q3)).toFixed(1)}"/><line class="imed" x1="${L}" x2="${W - R}" y1="${y(b.med).toFixed(1)}" y2="${y(b.med).toFixed(1)}"/>` : "";
  const lab = p => { const [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? (JA ? `${mm}月` : MON[mm - 1]) : dMD(new Date(yy, mm - 1, dd)); };
  const out = v => b && b.q1 != null && (v > b.q3 + 1.5 * (b.q3 - b.q1) || v < b.q1 - 1.5 * (b.q3 - b.q1));
  const n = ps.length, every = Math.ceil(n / (narrow ? 4 : 7)); // 期間の名前は間引く（右端の表示中の期間は必ず）
  const bars = ps.map(({p, v}, i) => { const last = i === n - 1, cx = x(i) + bw / 2;
    return `<g>${v == null ? "" : `<rect class="ibar${last ? " cur" : ""}" x="${(x(i) + bw * .18).toFixed(1)}" y="${y(v).toFixed(1)}" width="${(bw * .64).toFixed(1)}" height="${Math.max(0, Hh - B - y(v)).toFixed(1)}" rx="3"/>`}`
      + (last && proj != null ? `<rect class="iproj" x="${(x(i) + bw * .18).toFixed(1)}" y="${y(proj).toFixed(1)}" width="${(bw * .64).toFixed(1)}" height="${Math.max(0, Hh - B - y(proj)).toFixed(1)}" rx="3"/>` : "")
      + (v != null && !last && out(v) ? `<circle class="iout" cx="${cx.toFixed(1)}" cy="${(y(v) - 7).toFixed(1)}" r="3"/>` : "")
      + ((n - 1 - i) % every === 0 ? `<text class="ixl${last ? " cur" : ""}" x="${cx.toFixed(1)}" y="${Hh - 8}">${esc(lab(p))}</text>` : "")
      + `<rect class="ihit" x="${x(i).toFixed(1)}" y="0" width="${bw.toFixed(1)}" height="${Hh}"${tipAttr(p.label, v == null ? t("No records") : fmtM(k, v), last && proj != null ? t("At this pace: {v}", {v: fmtM(k, proj)}) : "")}/></g>`; }).join("");
  const list = ps.filter(x => x.v != null).map(x => `${x.p.label} ${fmtM(k, x.v)}`).join(LSEP());
  let sy = b ? t("{x}; {need}", {x: (b.part ? t("pace") + " " : "") + fmtM(k, b.v), need: insExpr(b)}) : ""; if (b && b.q1 != null){ const all = [...b.xs, b.v], r = 1 + b.xs.filter(z => z > b.v).length, d = b.v - b.med, pc = b.med ? Math.round(d * 100 / b.med) : null;
    sy = t("{x}: {r} highest of {n} {unit}; {d} from the median {m}{pc}", {x: (b.part ? t("At this pace") + " " : "") + fmtM(k, b.v), r: JA ? r : `${r}${["th", "st", "nd", "rd"][r % 100 > 10 && r % 100 < 14 ? 0 : r % 10 < 4 ? r % 10 : 0]}`, n: all.length, unit: insUnit(all.length), d: (d >= 0 ? "+" : "−") + fmtM(k, Math.abs(d)), m: fmtM(k, b.med), pc: pc == null ? "" : ` (${pc >= 0 ? "+" : ""}${pc}%)`}); }
  const th = !b || b.q1 == null ? "" : b.pos === "typical" ? t("Within the middle half of your past {unit}", {unit: insNoun()}) : b.better === false ? t("Outside the middle half, on the worse side; the warning marks show where it came from") : b.better ? t("Outside the middle half, on the better side; keep what you changed") : b.pos === "high" ? t("Above the 75th pct of your past {unit}", {unit: insNoun()}) : t("Below the 25th pct of your past {unit}", {unit: insNoun()});
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(t("{metric} over the last {n} {unit}", {metric: insName(k), n, unit: insUnit(n)}))}">${ticks}${band}${bars}</svg>
    <figcaption><span class="lg iband">${esc(t("Middle half of the last {n} {unit}", {n: b ? b.n : 0, unit: insUnit(b ? b.n : 0)}))}</span><span class="lg imed">${fx(t("median"), "", t("median"))}</span>${proj != null ? `<span class="lg iproj">${fx(`${t("pace")} = ${t("so far")} × ${t("period")} ÷ ${t("elapsed")}`, "", t("At this pace"))}</span>` : ""}<span class="lg iout">${fx(`< ${LO()} − 1.5×${SP()} ${t("or")} > ${HI()} + 1.5×${SP()}`, "", t("Far above or below the other periods"))}</span></figcaption>
    <p class="sr">${esc(list)} — ${esc(insPosText(b))}</p></figure>`;
}
/* 見直す候補の対象の費用（そのセッションの推定費用の合計）。費用で表せない候補（利用上限・繰り返し・長すぎるプロンプト）は null */
function insStake(f){
  const {ws, we} = period(), sum = xs => xs.reduce((a, s) => a + (s.cost || 0), 0), by = ids => DATA.filter(s => ids.includes(s.id));
  if (f.k === "modelfit"){ const L = lightOf(ws, we); return {c: L.c, n: L.xs.length}; }
  if (f.k === "heavy"){ const I = idleOf(ws, we); return {c: I.c, n: I.xs.length}; }
  if (f.k === "longctx"){ const L = longCtxOf(ws, we); return {c: sum(L), n: L.length}; }
  if ((f.k === "fix" || f.k === "friction") && f.ids.length){ const xs = by(f.ids); return {c: sum(xs), n: xs.length}; }
  return null;
}
// insRank は、見直す候補を対象の費用の大きい順に（費用で表せない候補は、そのあとに元の順で）
// 並べ方（見直す候補の一覧・「まず見直す」・アドバイスで同じ）：いつもの範囲より悪い側に外れたもの → 範囲で比べられないもの → 範囲の中のもの。同じ段では、関わる費用の大きい順
const insSide = f => { const b = MET[f.k] ? insBase(f.k) : null; return !b || b.q1 == null ? 1 : b.better === false ? 0 : 2; };
const insRank = F => F.map((f, i) => ({f, i, s: f.stake !== undefined ? f.stake : insStake(f), w: insSide(f)})).sort((a, b) => a.w - b.w || ((b.s && b.s.c) ?? -1) - ((a.s && a.s.c) ?? -1) || a.i - b.i).map(x => Object.assign({}, x.f, {stake: x.s}));
function insightH(F){
  F = insRank(F);
  const noCost = !((period().S || {}).usage || {}).tokens && !(((period().S || {}).usage || {}).cost > 0); // クレジットだけを記録するエージェントに絞ったときは、推定費用の推移を出さない
  const avail = INS_KEYS.filter(x => insBase(x) && !(noCost && (x === "cost" || x === "costPerCommit" || x === "costPerEdit"))), k = avail.includes(st.insK) ? st.insK : avail[0] || "active";
  const tabs = avail.map(x => `<button type="button" data-ins="${x}" aria-pressed="${x === k}" title="${esc(insName(x))}">${esc(INS_SHORT()[x])}</button>`).join("");
  const card = (title, body, cls = "") => body ? `<div class="inscol${cls}"><h4>${esc(title)}</h4>${body}</div>` : "";
  // グラフのカードは 2 列に並べる。エージェント別の表は幅いっぱい。2 枚の組にならずに 1 枚だけ残ったカードも幅いっぱいにする（空きを作らない）
  const sel = `<select class="insksel" aria-label="${esc(t("Metric"))}">${avail.map(x => `<option value="${x}"${x === k ? " selected" : ""}>${esc(INS_SHORT()[x])}</option>`).join("")}</select>`; // 狭い画面では、切り替えの代わりに選択リスト（横に隠さない）
  // カードは「コスト」（時間・クレジット・金額・トークン）・「出力」（コミット・変えた行）・「使い方」の 3 つに分けて並べる
  const cards = [{sec: t("Cost")},
    {h: `<div class="inscol"><div class="inshead"><h4>${esc(t("Trend"))}</h4><div class="segc insk trendk" role="group" aria-label="${esc(t("Metric"))}">${tabs}</div>${sel}</div>${insChart(k)}</div>`},
    {h: (b => b ? `<div class="inscol">${b}</div>` : "")(insCum())},
    {h: (b => b ? `<div class="inscol">${b}</div>` : "")(insWhere())}, {h: card(t("Tokens by type"), insTokens())},
    {h: card((HISTK(DATA.filter(s => inP(s, period().ws, period().we))) || {}).title || "", insHist())},
    {h: card(t("By agent"), insAgents()), full: true},
    {sec: t("Output")}, {h: card(t("Outputs"), insOutputs(), " iout"), full: true},
    {sec: t("Unit cost")}, {h: (b => b ? `<div class="inscol">${b}</div>` : "")(insScatter())},
    {sec: t("How you used AI")}, {h: card(t("How you used AI"), insUsage())}, {h: card(t("When you worked"), insHeat())}, {h: card(t("How you worked"), insWork())},
  ].filter(c => c.h || c.sec);
  let run = []; const out = [], flush = () => { if (run.length % 2) run[run.length - 1].full = true; out.push(...run); run = []; };
  cards.forEach(c => { if (c.sec){ flush(); out.push(c); } else if (c.full){ flush(); out.push(c); } else run.push(c); }); flush();
  const body = out.filter((c, i) => !c.sec || (out[i + 1] && !out[i + 1].sec)) // 中身のない見出しは出さない
    .map(c => c.sec ? `<h3 class="inssec">${esc(c.sec)}</h3>` : c.full ? c.h.replace(/^<div class="inscol/, '<div class="inscol full') : c.h).join("");
  return `<section class="insight" aria-labelledby="ins-h"><h3 id="ins-h" class="sr">${esc(t("Overview"))}</h3>${insHeadlines(F)}
    <div class="insg">${body}</div></section>`;
}
// fitTrend は、トレンドの指標の切り替えが 1 行に収まらないとき（指標が多い・カードが狭い・言語で幅が違う）、選択リストに替える。
// 折り返した切り替えは角の丸みが崩れて読みにくいので、実際に並べて測る（見えていないとき・狭い画面で隠しているときは測らない）
function fitTrend(){ const g = $("#review .insk.trendk"); if (!g) return; const h = g.closest(".inshead"); h.classList.remove("foldk");
  const bs = g.querySelectorAll("button"); if (!g.offsetParent || bs.length < 2) return;
  if (bs[bs.length - 1].offsetTop !== bs[0].offsetTop || g.getBoundingClientRect().right > h.getBoundingClientRect().right + 1) h.classList.add("foldk"); }
addEventListener("resize", () => fitTrend());
function bindInsight(R){
  requestAnimationFrame(fitTrend);
  R.querySelectorAll("[data-ins]").forEach(b => b.onclick = () => { st.insK = b.dataset.ins; store.set("insK", st.insK); summary(); const nb = $("#review").querySelector(`[data-ins="${CSS.escape(b.dataset.ins)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-wk]").forEach(b => b.onclick = () => { st.insWk = b.dataset.wk; store.set("insWk", st.insWk); summary(); const nb = $("#review").querySelector(`[data-wk="${CSS.escape(b.dataset.wk)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-scat]").forEach(b => b.onclick = () => { st.insS = b.dataset.scat; store.set("insS", st.insS); summary(); const nb = $("#review").querySelector(`[data-scat="${CSS.escape(b.dataset.scat)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-cum]").forEach(b => b.onclick = () => { st.insC = b.dataset.cum; store.set("insC", st.insC); summary(); const nb = $("#review").querySelector(`[data-cum="${CSS.escape(b.dataset.cum)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll(".insksel").forEach(sl => sl.onchange = () => { st.insK = sl.value; store.set("insK", st.insK); summary(); const ns = $("#review .insksel"); if (ns) ns.focus(); });
  R.querySelectorAll("[data-where]").forEach(b => b.onclick = () => { st.insW = b.dataset.where; store.set("insW", st.insW); summary(); const nb = $("#review").querySelector(`[data-where="${CSS.escape(b.dataset.where)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll(".iwses[data-sid]").forEach(b => b.onclick = () => select(b.dataset.sid)); // いちばん変えたファイル：編集の回数から、そのファイルをいちばん変えたセッションを開く
  R.querySelectorAll(".insfig [data-sid]:not(.iwses)").forEach(c => { c.onclick = () => select(c.dataset.sid); c.onkeydown = e => { if (e.key === "Enter" || e.key === " "){ e.preventDefault(); select(c.dataset.sid); } }; }); // 散布図の点は、そのセッションを開く
}

/* 費用と成果の散布図：表示中の期間に始まったセッションごとに、横は推定費用（平方根の目盛り）、縦は成果（そのセッションの間のコミットと作った PR）。
   費用が上位 25%（かつ $0.5 以上）なのに成果のないセッションを「見直す価値のある領域」として色を変え、件数と費用を文で言う */
// 散布図の横軸に使える量（セッションごとの値・書き方・目立たせる下限・名前）。推定費用のないエージェントでも、クレジットやトークンで描ける
const SCAT = () => ({cost: {v: s => s.cost, f: usd, floor: 0.5, n: t("cost"), title: t("Cost vs outputs per session"), ax: t("Estimated cost (√ scale) →"), s: t("Cost")},
  credits: {v: s => s.credits, f: cr, floor: 1, n: t("credits"), title: t("Credits vs outputs per session"), ax: t("Kiro credits (√ scale) →"), s: t("Credits")},
  tokens: {v: s => sesTok(s), f: tok, floor: 1e5, n: t("tokens"), title: t("Tokens vs outputs per session"), ax: t("Tokens (√ scale) →"), s: t("Tokens")}});
function insScatter(){
  const {ws, we} = period(), ss = DATA.filter(s => s.start >= ws && s.start < we), K = SCAT(), avail = Object.keys(K).filter(k => ss.filter(s => K[k].v(s) > 0).length >= 3); if (!avail.length) return "";
  const k = avail.includes(st.insS) ? st.insS : avail[0], M = K[k], fv = M.f;
  const xs = ss.filter(s => M.v(s) > 0).map(s => ({s, c: M.v(s), o: commitsOf(s).length + (s.prs || []).length}));
  const tabs = avail.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${avail.map(x => `<button type="button" data-scat="${x}" aria-pressed="${x === k}">${esc(K[x].s)}</button>`).join("")}</div>` : "";
  const head = `<div class="inshead"><h4>${esc(M.title)}</h4>${tabs}</div>`;
  const cs = xs.map(x => x.c), thr = Math.max(M.floor, insQuant(cs, .75)), maxC = insNice(Math.max(...cs)), maxO = Math.max(4, ...xs.map(x => x.o));
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 214 : 234, L = narrow ? 40 : 48, R = 12, T = 12, B = 44; // 下に目盛りと軸の名前（点に重ならないように、目盛りの下）
  const x = c => L + (W - L - R) * Math.sqrt(c / maxC), y = o => T + (Hh - T - B) * (1 - o / maxO);
  const bad = xs.filter(p => p.o === 0 && p.c >= thr), badC = bad.reduce((t, p) => t + p.c, 0);
  const xt = [0, maxC / 4, maxC].map(c => `<g class="itk"><line x1="${x(c).toFixed(1)}" x2="${x(c).toFixed(1)}" y1="${T}" y2="${Hh - B}"/><text class="ixl" x="${x(c).toFixed(1)}" y="${Hh - B + 16}">${esc(k === "credits" ? crN(c) : fv(c))}</text></g>`).join("");
  const yt = [0, Math.round(maxO / 2), maxO].map(o => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(o).toFixed(1)}" y2="${y(o).toFixed(1)}"/><text x="${L - 8}" y="${(y(o) + 4).toFixed(1)}">${o}</text></g>`).join("");
  const zx = Math.min(W - R, x(Math.min(thr, maxC))); // しきい値が軸の外（どのセッションも $0.50 未満）なら、帯は描かない
  const zone = zx >= W - R ? "" : `<rect class="izone" x="${zx.toFixed(1)}" y="${(y(0) - 14).toFixed(1)}" width="${(W - R - zx).toFixed(1)}" height="14"/>`;
  const pts = xs.map((p, i) => { const hit = p.o === 0 && p.c >= thr, jy = p.o === 0 ? -((i * 37) % 9) - 3 : 0; // 成果 0 の点は重なりやすいので、縦に少しずらす
    return `<circle class="ipt${hit ? " bad" : ""}" cx="${x(p.c).toFixed(1)}" cy="${(y(p.o) + jy).toFixed(1)}" r="${narrow ? 4 : 5}" tabindex="0" role="button" data-sid="${esc(p.s.id)}" aria-label="${esc(`${p.s.title || p.s.id}: ${fv(p.c)}, ${plural(p.o, "output")}`)}"${tipAttr(p.s.title || "", `${fv(p.c)} · ${plural(p.o, "output")}`, p.s.project)}/>`; }).join("");
  const note = `${t("outputs")} = 0 ∧ ${M.n} ≥ ${fv(thr)} : ${bad.length} · ${fv(badC)}`;
  const tq = xs.filter(p => p.c >= thr).length, all$ = cs.reduce((a, c) => a + c, 0);
  const sy = k === "cost" ? t("{m} of {sessions} costing {thr} or more had no output: {usd} = {pc}% of session cost", {m: bad.length, sessions: plural(tq, "session"), thr: usd(thr), usd: usd(badC), pc: all$ ? Math.round(badC * 100 / all$) : 0})
    : t("{m} of {sessions} using {thr} or more had no output: {v} = {pc}% of the sessions' {metric}", {m: bad.length, sessions: plural(tq, "session"), thr: fv(thr), v: fv(badC), pc: all$ ? Math.round(badC * 100 / all$) : 0, metric: M.n});
  const th = bad.length ? t("Open the red points: research is fine; work that stopped partway is the cost to cut") : "";
  return `${head}${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="group" aria-label="${esc(M.title)}">${xt}${yt}${zone}${pts}
    <text class="iax" x="${W - R}" y="${Hh - 1}">${esc(M.ax)}</text><text class="iax st" x="${L + 4}" y="${T + 10}">${esc(t("↑ Commits and PRs"))}</text></svg><figcaption><span class="lg izone">${fx(note, "", k === "cost" ? t("Costly sessions with no output") : t("Heavy sessions with no output"))}</span></figcaption><p class="ifxs">${fx(`${t("outputs")} = ${t("commits")} + PR`)}${fx(`${fv(thr)} = max(${fv(M.floor)}, P75(${M.n}))`)}</p></figure>`;
}
/* 曜日×時間帯のヒートマップ：表示中の期間に AI と作業していた時間（重なりは 1 回）。月の表示では曜日ごとに足す。
   平日 9〜18 時の外の割合を、いつもの範囲と比べる */
function insMinutes(ws, we){ // [ws, we) の作業時間を、曜日（月曜 0）× 時の分にする。セッションの重なりは 1 回だけ数える
  const iv = []; DATA.forEach(s => s.segs.forEach(([a, b]) => { a = Math.max(a, ws); b = Math.min(b, we); if (b > a) iv.push([a, b]); }));
  iv.sort((p, q) => p[0] - q[0]); const m = []; iv.forEach(([a, b]) => { const l = m[m.length - 1]; if (l && a <= l[1]) l[1] = Math.max(l[1], b); else m.push([a, b]); });
  const g = Array.from({length: 7}, () => new Array(24).fill(0));
  m.forEach(([a, b]) => { for (let t0 = a; t0 < b;){ const d = new Date(t0 * 1000), h = new Date(d.getFullYear(), d.getMonth(), d.getDate(), d.getHours() + 1).getTime() / 1000, e = Math.min(b, h);
    g[(d.getDay() + 6) % 7][d.getHours()] += (e - t0) / 60; t0 = e; } });
  return g;
}
const insOffM = g => { let off = 0, all = 0; g.forEach((r, d) => r.forEach((v, h) => { all += v; if (d >= 5 || h < 9 || h >= 18) off += v; })); return {off, all}; };
const insOff = g => { const {off, all} = insOffM(g); return all ? Math.round(off * 100 / all) : null; };
function insHeat(){
  const {ws, we} = period(), g = insMinutes(ws, we), max = Math.max(...g.flat()); if (!(max > 0)) return "";
  // 作業がいちばん集まった 3 時間（曜日ごと）
  let best = {v: -1}; g.forEach((r, d) => { for (let h = 0; h + 3 <= 24; h++){ const v = r[h] + r[h + 1] + r[h + 2]; if (v > best.v) best = {v, d, h}; } });
  const off = insOff(g), b = off == null ? null : insRange("offhours", off, p => p.S ? insOff(insMinutes(p.ws, p.we)) : null, true, false, v => `${Math.round(v)}%`);
  const days = [1, 2, 3, 4, 5, 6, 0];
  const cells = g.map((r, d) => `<span class="ihd">${esc(dow(days[d]))}</span>` + r.map((v, h) => `<span class="ihc${d >= 5 || h < 9 || h >= 18 ? " off" : ""}" style="--a:${v ? (0.12 + 0.88 * v / max).toFixed(2) : 0}"${tipAttr(`${dow(days[d])} ${h}:00–${h + 1}:00`, v ? dur(v) : t("None"))}></span>`).join("") + `<span class="iht">${(z => z ? esc(dur(z)) : "—")(r.reduce((a, v) => a + v, 0))}</span>`).join(""); // 行の右端に曜日の合計（触れなくても値が読めるように）
  const hrs = `<span></span>` + Array.from({length: 24}, (_, h) => `<span class="ihh">${h % 3 ? "" : h}</span>`).join("") + `<span class="ihh iht">${esc(t("Total"))}</span>`;
  const peak = `${dow(days[best.d])} ${best.h}–${best.h + 3}${JA ? "時" : ":00"}`;
  const tot = insOffM(g).all, sy = t("Busiest {p}: {h} = {pc}% of the {all} in this chart; off-hours {off}%", {p: peak, h: dur(best.v), pc: tot ? Math.round(best.v * 100 / tot) : 0, all: dur(tot), off: off == null ? "—" : off});
  const th = b && b.q1 != null && b.pos === "high" ? t("Off-hours share {off}% is above the 75th pct = {q3}% of your past {unit}", {off, q3: Math.round(b.q3), unit: insNoun()}) : "";
  return `${say(sy, th)}<div class="ihgrid" role="img" aria-label="${esc(t("Busiest: {p}", {p: peak}))}">${hrs}${cells}</div>
    <p class="ifxs">${fx(`${t("peak 3h")} = ${peak} (${dur(best.v)})`)}${off != null ? fx(`${t("off-hours")} = ${dur(insOffM(g).off)} ÷ ${dur(insOffM(g).all)} ${t("(session time in the period)")} = ${off}%`) : ""}${off != null ? insChip(b) : ""}</p>
    <p class="note">${esc(t("Off-hours: Saturday, Sunday, and weekdays before 9:00 or from 18:00. The chart counts time while sessions were running; Active time also counts the short pauses inside a stretch of work, so it can be a little longer"))}</p>`;
}

/* 累積の折れ線：表示中の期間の日ごとの推定費用を積み上げた線と、前の期間の線。どの時点で前より多いか少ないかを見る（費用の記録がなければ稼働時間） */
// 積み上がりの指標（日ごとの値・書き方・期間の合計）。合計は帯と同じ値に合わせる（日ごとの丸めを足すと、帯とずれる）
const CUM = () => ({cost: {f: d => d.cost, fm: usd, tot: u => costOf(u), s: t("Cost")}, credits: {f: d => d.credits, fm: cr, tot: u => u.credits || null, s: t("Credits")},
  tokens: {f: d => d.tokens, fm: tok, tot: u => u.tokens || null, s: t("Tokens")}, active: {f: d => d.active, fm: v => dur(v), tot: null, s: t("Time")}});
function insCum(){
  const {S: w, P: pw} = period(); if (!w) return "";
  const C = CUM(), avail = Object.keys(C).filter(k => w.days.some(d => C[k].f(d) > 0)); if (!avail.length) return "";
  const k = avail.includes(st.insC) ? st.insC : avail[0], useCost = k === "cost" || k === "credits", f = d => C[k].f(d) || 0, fm = C[k].fm; // 予算（前の期間並み）の目安は、費用とクレジットだけ
  const tabs = avail.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${avail.map(x => `<button type="button" data-cum="${x}" aria-pressed="${x === k}">${esc(C[x].s)}</button>`).join("")}</div>` : "";
  const head = `<div class="inshead"><h4>${esc(st.mode === "month" ? t("Cumulative, vs last month") : t("Cumulative, vs last week"))}</h4>${tabs}</div>`;
  const cum = ds => { let a = 0; return ds.map(d => a += f(d)); };
  const fr = insFrac(), n = Math.max(w.days.length, pw ? pw.days.length : 0), upto = fr < 1 ? Math.max(1, Math.ceil(fr * w.days.length)) : w.days.length;
  const cur = cum(w.days).slice(0, upto), prev = pw ? cum(pw.days) : []; // 前の期間は全部（月の日数が違っても、合計は前の月まるごと）
  const top = insNice(Math.max(...cur, ...prev, 0) * 1.05);
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 180 : 170, L = narrow ? 52 : 64, R = 10, T = 10, B = 24;
  const x = i => L + (W - L - R) * (n === 1 ? 0 : i / (n - 1)), y = v => T + (Hh - T - B) * (1 - v / top), line = a => a.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 6}" y="${(y(v) + 4).toFixed(1)}">${esc(k === "credits" ? crN(v) : fm(v))}</text></g>`).join("");
  const a = new Date(period().ws * 1000), xl = [0, n - 1].map(i => `<text class="ixl ${i ? "e" : "s"}" x="${x(i).toFixed(1)}" y="${Hh - 6}">${esc(dMD(addDays(a, i)))}</text>`).join(""); // 両端の日付は、内側に寄せる（はみ出さないように）
  if (C[k].tot){ const a = C[k].tot(w.usage || {}), b = pw ? C[k].tot(pw.usage || {}) : null; // 合計は帯と同じ値（日ごとの丸めを足すと、帯と 1 セントずれる）
    if (a != null && cur.length) cur[cur.length - 1] = a; if (b != null && prev.length) prev[prev.length - 1] = b; }
  const last = cur.length - 1, ptot = prev.length ? prev[prev.length - 1] : null, pref = fr >= 1 ? ptot : prev.length > last ? prev[last] : null, diff = pref == null ? null : cur[last] - pref; // 終わった期間は前の期間の合計と、途中の期間は同じ日までと比べる
  const sy = diff == null ? "" : (fr >= 1 ? t("Total {diff} ({pc}) vs the previous period's total", {diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: pref ? `${diff >= 0 ? "+" : ""}${Math.round(diff * 100 / pref)}%` : "—"})
      : t("Day {d} of {n}: {diff} ({pc}) vs the previous period", {d: last + 1, n: w.days.length, diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: pref ? `${diff >= 0 ? "+" : ""}${Math.round(diff * 100 / pref)}%` : "—"}))
    + (fr < 1 && ptot != null ? t("; at this pace {end} at the end vs {prev}", {end: fm(cur[last] / fr), prev: fm(ptot)}) : "");
  const left = n - (last + 1), rem = ptot == null ? null : ptot - cur[last];
  const th = fr >= 1 || rem == null || !useCost ? "" : rem <= 0 ? t("Already {x} over the previous period's total {prev}", {x: fm(-rem), prev: fm(ptot)}) : left === 1 ? t("To stay within the previous period's {prev}: at most {rem} tomorrow", {prev: fm(ptot), rem: fm(rem)}) : left > 0 ? t("To stay within the previous period's {prev}: at most {rem} in the {d} left = {per}/day", {prev: fm(ptot), rem: fm(rem), d: plural(left, "day"), per: fm(rem / left)}) : "";
  return `${head}${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(`${fm(cur[last])}${prev.length ? ` / ${fm(prev[Math.min(last, prev.length - 1)])}` : ""}`)}">${ticks}${xl}
    ${prev.length ? `<polyline class="iprev" points="${line(prev)}"/>` : ""}<polyline class="icum" points="${line(cur)}"/><circle class="icumd" cx="${x(last).toFixed(1)}" cy="${y(cur[last]).toFixed(1)}" r="4"/></svg>
    <figcaption><span class="lg icum">${esc(soFar() != null ? (st.mode === "month" ? t("This month") : t("This week")) : (st.mode === "month" ? t("Month shown") : t("Week shown")))} <b>${esc(fm(cur[last]))}</b></span>${prev.length ? `<span class="lg iprev">${esc(st.mode === "month" ? t("Previous month") : t("Previous week"))}</span>` : ""}</figcaption>${diff != null ? `<p class="ifxs">${fx(`${fm(cur[last])} − ${fm(pref)} = ${(diff >= 0 ? "+" : "−") + fm(Math.abs(diff))}`, diff > 0 ? " bad" : " good")}<span class="note">${esc(fr >= 1 ? t("vs the previous period's total") : t("vs the same day of the previous period ({d})", {d: dMD(addDays(new Date(periodBack(1).ws * 1000), last))}))}</span></p>` : ""}</figure>`;
}
/* セッションの長さ別の費用：表示中の期間に動いたセッションを、期間の中の作業時間で 6 つに分け、区分ごとの推定費用の合計を棒に（件数は棒の下）。
   費用の記録がなければ件数で。どの長さのセッションに費用が集まっているかを読む */
// 長さ別の図に使う量：推定費用 → クレジット → トークンの順に、期間のセッションに記録のあるもの
const HISTK = ss => ss.some(s => s.cost > 0) ? {k: "cost", v: s => s.cost, f: usd, n: t("cost"), title: t("Cost by session length")}
  : ss.some(s => s.credits > 0) ? {k: "credits", v: s => s.credits, f: cr, n: t("credits"), title: t("Credits by session length")}
  : ss.some(s => sesTok(s) > 0) ? {k: "tokens", v: s => sesTok(s), f: tok, n: t("tokens"), title: t("Tokens by session length")} : null;
function insHist(){
  // 長さごとに使った量（推定費用、なければクレジット、それもなければトークン）。件数だけでは、長いセッションが重いかを読めないので、量のないときは出さない
  const {ws, we} = period(), all = DATA.filter(s => inP(s, ws, we)), K = HISTK(all); if (!K) return "";
  const ss = all.map(s => ({m: sesMin(s), c: K.v(s) || 0})).filter(x => x.m > 0); if (ss.length < 3) return "";
  const edges = [0, 5, 15, 30, 60, 120, Infinity], labs = ["<5m", "5–15m", "15–30m", "30–60m", "1–2h", "2h+"], useC = K.k === "cost";
  const bins = labs.map((_, i) => ss.filter(x => x.m >= edges[i] && x.m < edges[i + 1])), val = b => b.reduce((a, x) => a + x.c, 0);
  const vs = bins.map(val), tot = vs.reduce((a, v) => a + v, 0), mi = vs.indexOf(Math.max(...vs)), med = insQuant(ss.map(x => x.m), .5);
  const max = Math.max(...vs) || 1, W = innerWidth < 600 ? 360 : 560, Hh = 180, B = 20, T = 16, bw = W / labs.length, fv = K.f;
  const bars = vs.map((v, i) => { const h = (Hh - B - T) * v / max, xx = i * bw;
    return `<rect class="ihb${i === mi ? " med" : ""}" x="${(xx + bw * .14).toFixed(1)}" y="${(Hh - B - h).toFixed(1)}" width="${(bw * .72).toFixed(1)}" height="${h.toFixed(1)}" rx="3"${tipAttr(labs[i], plural(bins[i].length, "session"), fv(v))}/>`
      + (v ? `<text class="ihn" x="${(xx + bw / 2).toFixed(1)}" y="${(Hh - B - h - 4).toFixed(1)}">${esc(fv(v))}</text>` : "") + `<text class="ixl" x="${(xx + bw / 2).toFixed(1)}" y="${Hh - 4}">${labs[i]}</text>`; }).join(""); // 件数は棒のツールチップに（上の 1 文にも）
  const sy = t("Sessions of {bin}: {pc}% of {metric} ({usd}) in {sessions}", {bin: labs[mi], pc: Math.round(vs[mi] * 100 / tot), metric: K.n, usd: fv(vs[mi]), sessions: plural(bins[mi].length, "session")});
  const LC = useC ? longCtxOf(ws, we) : [];
  const th = mi >= 4 && LC.length ? t("{n} of the long sessions grew their input per response to up to {x}× the start: split them at a stopping point", {n: LC.length, x: Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0])))}) : "";
  return `${say(sy, th)}<figure class="insfig ihist"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(labs.map((l, i) => `${l} ${fv(vs[i])} (${plural(bins[i].length, "session")})`).join(", "))}">${bars}</svg>
    <p class="ifxs">${fx(`${t("median")}(${t("active time of {n} sessions", {n: ss.length})}) = ${dur(med)}`)}${fx(`% = ${K.n} ${t("of the bin")} ÷ ${fv(tot)}`)}</p></figure>`;
}

/* どこに使ったか：プロジェクト・エージェント・モデルごとの費用（なければクレジット・トークン・時間）を、大きい順の横棒で。上位 7 つとその他 */
function insWhere(){
  const {S: w} = period(); if (!w) return "";
  const u = w.usage || {}, by = st.insW || "project";
  const src = by === "model" ? (u.models || []).map(r => ({name: r[0], cost: r[1], tokens: r[2], c: "var(--acc)"}))
    : by === "source" ? ((w.shares || {}).source || []).map(x => ({name: x.key, cost: x.cost, credits: x.credits, tokens: x.tokens, minutes: x.minutes, c: agColor(x.key)}))
    : (w.projectStats || []).map(p => ({name: p.project, cost: p.cost, credits: p.credits, tokens: p.tokens, minutes: p.minutes, c: projColor(p.project)}));
  // 量は切り替えられる（推定費用・クレジット・トークン・時間のうち記録のあるもの）。モデル別は費用とトークンだけ
  const ks = ["cost", "credits", "tokens", "minutes"].filter(f => src.some(x => x[f] > 0)); if (!ks.length) return "";
  const k = ks.includes(st.insWk) ? st.insWk : ks[0];
  const fv = {cost: usd, credits: cr, tokens: tok, minutes: v => dur(v)}[k], name = {cost: t("Estimated cost"), credits: t("Kiro credits"), tokens: t("Tokens"), minutes: t("Active time")}[k];
  const xs = src.filter(x => x[k] > 0).sort((a, b) => b[k] - a[k]), top = xs.slice(0, 7), rest = xs.slice(7), tot = xs.reduce((a, x) => a + x[k], 0);
  if (rest.length) top.push({name: t("Other ({n})", {n: rest.length}), [k]: rest.reduce((a, x) => a + x[k], 0), c: "var(--other)"});
  let acc = 0, k80 = 0; for (const x of xs){ acc += x[k]; k80++; if (acc >= tot * .8) break; }
  const sy = t("{name}: {pc}% of {metric} ({v})", {name: xs[0].name, pc: Math.round(xs[0][k] * 100 / tot), metric: name, v: fv(xs[0][k])}) + (k80 < xs.length ? t("; 80% is in {k} of {n}", {k: k80, n: xs.length}) : "");
  const tabs = [["project", t("Project")], ["source", t("Agent")], ["model", t("Model")]].map(([v, l]) => `<button type="button" data-where="${v}" aria-pressed="${v === by}">${esc(l)}</button>`).join("");
  const rows = top.map(x => `<div class="iwr"><span class="iwn" title="${esc(x.name)}"><i style="background:${x.c}"></i>${esc(x.name)}</span><span class="iwb"><span style="width:${(x[k] * 100 / top[0][k]).toFixed(1)}%;background:${x.c}"></span></span><span class="iwv">${esc(fv(x[k]))}</span><b>${Math.round(x[k] * 100 / tot)}%</b></div>`).join("");
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-wk="${x}" aria-pressed="${x === k}">${esc({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), minutes: t("Time")}[x])}</button>`).join("")}</div>` : "";
  return `<div class="inshead"><h4>${esc(t("Where it went"))}</h4><div class="segc insk" role="group" aria-label="${esc(t("Group by"))}">${tabs}</div>${mtabs}</div>${say(sy)}
    <figure class="insfig"><div class="iwrows" role="table" aria-label="${esc(name)}">${rows}</div><p class="ifxs">${fx(`% = ${name} ÷ ${fv(tot)}`)}</p></figure>`;
}
/* 数の枠（statH）に、いつもの範囲との不等式を添える。k は MET の指標名（なければ添えない） */
const insStat = (label, v, sub, help, metric, k) => statH(esc(label), v, [sub, k && MET[k] ? insChip(insBase(k)) : ""].filter(Boolean).join(" "), help, metric);
/* AI が変えたファイル：表示中の期間に始まったセッションで、編集のツールが変えたファイルの数・行（編集の前後を比べた目安）と、
   いちばん変えたファイル。git ではなく履歴のツール呼び出しから数えるので、コミットしなかった変更や、あとで戻した変更も入る。
   セッションごとに載るファイルは多く変えた 30 まで（それより多いセッションは、残りをファイルの数にだけ足す） */
function insEditsBody(){
  const {ws, we} = period(), xs = DATA.filter(s => s.start >= ws && s.start < we && s.nFiles > 0); if (!xs.length) return "";
  const by = new Map(); let add = 0, del = 0, edits = 0, extra = 0;
  for (const s of xs){ add += s.editAdded || 0; del += s.editRemoved || 0; extra += Math.max(0, s.nFiles - s.files.length);
    s.files.forEach((f, i) => { const e = (s.fileEdits && s.fileEdits[i]) || [0, 0, 0], abs = f.startsWith("/") || /^[A-Za-z]:[\\/]/.test(f), k = abs ? f : `${s.projectPath}\u0000${f}`;
      const x = by.get(k) || {f, rel: abs && s.projectPath && f.startsWith(s.projectPath + "/") ? f.slice(s.projectPath.length + 1) : f, project: s.project, n: 0, a: 0, d: 0, sid: s.id, top: -1};
      x.n += e[0]; x.a += e[1]; x.d += e[2]; edits += e[0]; if (e[1] + e[2] > x.top){ x.top = e[1] + e[2]; x.sid = s.id; } by.set(k, x); }); }
  const nf = by.size + extra, fs = [...by.values()].sort((a, b) => (b.a + b.d) - (a.a + a.d) || b.n - a.n || (a.rel < b.rel ? -1 : 1)), top = fs.slice(0, 8), n = commas;
  const w0 = Math.max(1, top[0].a + top[0].d), multi = new Set(top.map(x => x.project)).size > 1;
  const S = [insStat(t("Files edited by AI"), n(nf), esc(plural(xs.length, "session")), "aiFiles"),
    insStat(t("Lines edited by AI"), n(add + del), fx(`+${n(add)} −${n(del)}; ${n(edits)} ${t("edits")}`, "", `+${n(add)} −${n(del)}`), "aiLines")].join("");
  // ファイル名はファイルそのものを開く（セッションの間のコミットのリンク、なければこの PC のファイル。セッション詳細の一覧と同じ）。
  // そのファイルをいちばん変えたセッションは、右端の編集の回数から開く
  const href = x => { const ss = DATA.find(s => s.id === x.sid); return ss ? fileLink(ss, x.f) || localHref(x.f, ss.projectPath) : ""; };
  const rows = top.map(x => `<div class="iwr"><span class="iwn iwf" title="${esc(x.f)}">${multi ? `<small>${esc(x.project)}</small> ` : ""}${fileA(href(x), esc(x.rel))}</span><span class="iwb"><span style="width:${((x.a + x.d) * 100 / w0).toFixed(1)}%;background:var(--acc)"></span></span><span class="iwv">+${n(x.a)} −${n(x.d)}</span><button type="button" class="iwses" data-sid="${esc(x.sid)}" title="${esc(t("Open the session that changed it most"))}">${esc(t("{n}×", {n: x.n}))}</button></div>`).join("");
  return {stats: S, files: `<figure class="insfig"><figcaption class="iwcap">${esc(t("Most edited files"))}</figcaption><div class="iwrows" role="table" aria-label="${esc(t("Most edited files"))}">${rows}</div><p class="ifxs">${fx(t("+ − = lines added and removed, compared before and after each edit; × = edits"), "", t("Lines: before and after each edit, an estimate"))}</p></figure>`, nf, add, del};
}
/* 成果：コミット（うち AI）・変えた行・PR・成果まで届いたセッション・コミットあたりの費用。割った指標は式で */
function insOutputs(){
  const {S: w} = period(); if (!w) return "";
  const o = w.outputs || {commits: 0}, g = w.git && w.git.commits ? w.git : null, base = w.outBase ?? w.sessions, n = commas, E = insEditsBody() || null;
  if (!o.commits && !g && !o.prs && !E) return "";
  const lines = g ? g.added + g.removed : 0, sp = base ? Math.round(w.outSessions * 100 / base) : null;
  const S = [
    g ? insStat(t("Git commits"), `${g.commits}`, fx(`AI ${g.ai} ÷ ${g.commits} = ${Math.round(g.ai * 100 / g.commits)}%`), "gitCommits", "commits", "gitCommits") : o.commits ? insStat(t("AI commits"), `${o.commits}`, esc(t("Commits AI ran, from history")), "commits", "aiCommits") : "", // AI のコミットが 0 なら出さない（記録しないエージェントで「0」と読めてしまう）
    g ? insStat(t("Lines in Git commits"), n(lines), fx(`+${n(g.added)} −${n(g.removed)}; ${n(lines)} ÷ ${g.commits} = ${n(Math.round(lines / g.commits))} ${t("per commit")}`), "lines", "lines") : "",
    o.prs ? insStat(t("Pull requests"), `${o.prs}`, esc(t("Created by AI")), "prs", "prs") : "",
    base ? insStat(t("Sessions that reached a commit or PR"), `${sp}<small>%</small>`, fx(`${w.outSessions} ÷ ${base} ${t("Claude Code sessions")} = ${sp}%`), "outSessions", "outSessions", "outSessions") : "",
  ].join("");
  // 読み取りの 1 文も、Git と AI の編集で列ごとに分ける（1 つの文につなぐと、2 つを足した数に読めてしまう）
  const syG = g ? t("{c} commits, {ai}% by AI; {sp}% of Claude Code sessions reached a commit or PR", {c: g.commits, ai: Math.round(g.ai * 100 / g.commits), sp: sp ?? "—"}) : o.commits ? t("{c} AI commits; {sp}% of Claude Code sessions reached a commit or PR", {c: o.commits, sp: sp ?? "—"}) : "";
  const syE = E ? t("AI edited {n} files (+{a} −{d} lines)", {n: n(E.nf), a: n(E.add), d: n(E.del)}) : "";
  // Git（リポジトリ）と AI の編集（履歴）は元が違うので、列と小見出しで分け、足したり割ったりして混ぜない
  const col = (cls, icon, h, note, body) => `<div class="iosec ${cls}" role="group" aria-label="${esc(h)}"><h6 class="iksh">${ico(icon)}${esc(h)} <small>${esc(note)}</small></h6>${body}</div>`;
  return `<div class="iout2">${col("outGit", "commit", t("Git"), t("from your repositories"), S ? `${say(syG)}<div class="stats istats">${S}</div>` : noneH(esc(t("No Git commits for this selection"))))}${E ? col("outEdit", "edit", t("AI edits"), t("from the agents' history, committed or not"), `${say(syE)}<div class="stats istats">${E.stats}</div>${E.files}`) : ""}</div>`;
}
/* AI の使い方：トークン・キャッシュ・クレジット・プロンプト・プロンプトあたりの費用・サブエージェント・月末の推定 */
function insUsage(){
  const {S: w, ws, we} = period(), u = w && w.usage; if (!u || (!u.tokens && !u.credits)) return "";
  const pj = projection(w), N = !u.subagents && unrecorded(ws, we, "subagents");
  const S = [
    u.tokens ? insStat(t("Tokens"), tok(u.tokens), fx(`${t("output")} ${tok(u.out)} = ${Math.round(u.out * 100 / u.tokens)}%`), "tokens", "tokens", "tokens") : "",
    u.tokens ? insStat(t("Read from cache"), u.cacheHit == null ? "—" : `${Math.round(u.cacheHit * 100)}<small>%</small>`, fx(t("cache reads ÷ input")), "cache", u.cacheHit == null ? "" : "cache", "cache") : "",
    u.credits ? insStat(t("Kiro credits"), crN(u.credits), esc(t("Not an estimate")), "credits", "credits") : "",
    insStat(t("Prompts"), commas(w.prompts), "", "prompts", "sessions", "prompts"),
    w.costPerAsk != null ? insStat(t("Estimated cost per prompt"), usdH(w.costPerAsk), fx(t("cost ÷ {n} prompts with token data", {n: commas(w.costPrompts)})), "costPerAsk", "costPerAsk") : "",
    ...perSession(ws, we),
    N ? norecStat(esc(t("Subagents")), N, "subagents") : insStat(t("Subagents"), `${u.subagents}`, u.subagents ? esc(t("Total run time {d}", {d: dur(u.subMin)})) + (u.subTypes.length ? " · " + esc(u.subTypes.slice(0, 3).map(([k, v]) => `${k} ${v}`).join(", ")) : "") : esc(t("Not used")), "subagents", u.subagents ? "subagents" : ""),
    pj && pj.cost != null ? insStat(t("Month-end cost (estimate)"), "≈ " + usdH(pj.cost), fx(`${t("so far")} ÷ ${pj.days}${t("d")} × ${w.days.length}${t("d")}`), "projection", "projection") : "",
    pj && pj.credits != null ? insStat(t("Month-end credits (estimate)"), `≈ ${crN(pj.credits)}`, fx(`${t("so far")} ÷ ${pj.days}${t("d")} × ${w.days.length}${t("d")}`), "projectionCr", "projectionCr") : "",
  ].join("");
  const sy = u.tokens ? t("{t} tokens; {c}% of input read from cache", {t: tok(u.tokens), c: u.cacheHit == null ? "—" : Math.round(u.cacheHit * 100)}) : t("{c} Kiro credits", {c: crN(u.credits)});
  return `${say(sy)}<div class="stats istats">${S}</div>`;
}
/* セッションあたり：期間に始まったセッションのうち、その使用量（推定費用・クレジット・トークン）を記録したものの平均。
   エージェントごとに記録するものが違うので、それぞれ記録したセッションだけで割る */
function perSession(ws, we){
  const xs = DATA.filter(s => s.start >= ws && s.start < we), out = [];
  const avg = f => { const ys = xs.filter(s => f(s) > 0); return ys.length ? {n: ys.length, sum: ys.reduce((a, s) => a + f(s), 0)} : null; };
  const c = avg(s => sesTok(s) ? s.cost || 0 : 0), k = avg(s => s.credits || 0), tk = avg(sesTok);
  if (c) out.push(insStat(t("Estimated cost per session"), usdH(c.sum / c.n), fx(`${usd(c.sum)} ÷ ${c.n} ${t("sessions")} = ${usd(c.sum / c.n)}`), "costPerSes"));
  if (k) out.push(insStat(t("Kiro credits per session"), crN(k.sum / k.n), fx(`${cr1(k.sum)} ÷ ${k.n} ${t("sessions")} = ${cr1(k.sum / k.n)}`), "creditsPerSes"));
  if (tk) out.push(insStat(t("Tokens per session"), tok(tk.sum / tk.n), fx(`${tok(tk.sum)} ÷ ${tk.n} ${t("sessions")} = ${tok(tk.sum / tk.n)}`), "tokensPerSes"));
  return out;
}
/* 作業の進め方：言い直し・利用上限・コンパクション・長すぎるプロンプト・集中ブロック・切り替え・待ち時間 */
function insWork(){
  const {S: w, ws, we} = period(); if (!w) return "";
  const longest = Math.max(0, ...w.focus.map(b => b.min)), H = limitHits(ws, we), NL = unrecorded(ws, we, "limits"), C = compactionsOf(ws, we), NC = unrecorded(ws, we, "compactions"), B = bigOf(ws, we);
  const S = [
    insStat(t("Prompts with corrections or interruptions"), w.fixRate == null ? "—" : `${w.fixRate}<small>%</small>`, fx(`${t("corrected")} ÷ ${commas(w.prompts)} ${t("prompts")}`), "fix", w.fixRate ? "fix" : "", "fix"),
    NL ? norecStat(esc(t("Usage limit hits")), NL, "limits") : insStat(t("Usage limit hits"), `${H.length}`, H.length ? esc(H.slice(-3).map(h => h.r ? t("{time} (resets {r})", {time: `${md(h.t)} ${hm(h.t)}`, r: h.r}) : `${md(h.t)} ${hm(h.t)}`).join(TSEP())) : "", "limits", H.length ? "limits" : "", "limits"),
    NC ? norecStat(esc(t("Compactions")), NC, "compactions") : insStat(t("Compactions"), `${C.length}`, C.length ? esc(t("In {sessions}", {sessions: plural(new Set(C.map(c => c.s.id)).size, "session")})) : "", "compactions", C.length ? "compactions" : ""),
    insStat(t("Oversized prompts"), `${B.n}`, fx(`≥ ${commas(BIG_PROMPT)} ${t("characters")}${B.n ? `; ${t("max")} ${commas(B.max)}` : ""}`), "bigPrompts", B.n ? "bigPrompts" : "", "bigPrompts"),
    insStat(t("Focus blocks (60+ min)"), `${w.focus.length}`, longest ? esc(t("Longest {dur}", {dur: dur(longest)})) : "", "focus", w.focus.length ? "focus" : "", "focus"),
    insStat(t("Project switches per day"), `${w.switchesAvg}`, esc(t("Max {n}", {n: w.switchesMax})), "switches", "switches", "switches"),
    insStat(t("Wait time (median)"), secsH(w.waitMedian), fx(`P90 ${secs(w.waitP90)}; n = ${w.waitCount}`), "wait", w.waitCount ? "wait" : "", "wait"),
    // 見直す候補になる指標は、どれもここか上の数字に「?」を持ち、その横の印から開けるようにする
    insStat(t("Sessions with possible friction"), `${w.frictionN ?? (w.friction || []).length}`, "", "friction", "", "friction"),
    (rp => insStat(t("Repeated prompts"), `${rp.length}`, rp.length ? esc(snipOf(rp[0].text || "", 40)) : "", "repeats"))(repeatsOf(ws, we)),
  ].join("");
  const sy = t("Corrections {f}% of {p} prompts; {l} usage limit hits; {c} compactions", {f: w.fixRate ?? "—", p: commas(w.prompts), l: H.length, c: C.length});
  return `${say(sy)}<div class="stats istats">${S}</div>`;
}

/* トークンの種類別：入力・出力・キャッシュへの書き込み・キャッシュからの読み込み。積み上げの横棒 1 本と、値と割合。
   キャッシュから読む割合（= キャッシュ読み ÷ 入力の全体）を、過去の期間と比べる */
/* エージェント別：エージェントごとに単位が違う（推定費用・Kiro クレジット）ので、合計に混ぜずに並べる。
   時間・トークン・費用・クレジットは週の集計（shares.source。同時に動いた分は按分）、セッションとプロンプトは表示中の期間に動いたものを数える */
function insAgents(){
  const {S: w, ws, we} = period(), src = w && w.shares && w.shares.source || []; if (src.length < 2) return "";
  const tot = w.active || src.reduce((a, x) => a + (x.minutes || 0), 0); if (!tot) return ""; // 割合の分母は、帯とカードと同じ稼働時間
  const cnt = k => { let n = 0, p = 0; DATA.forEach(s => { if (s.source === k && inP(s, ws, we)){ n++; p += s.prompts.filter(q => !q.t || q.t >= ws && q.t < we).length; } }); return [n, p]; };
  const xs = src.map(x => ({...x, c: agColor(x.key), n: cnt(x.key)})).sort((a, b) => (b.minutes || 0) - (a.minutes || 0)), max = xs[0].minutes || 1;
  const v = (x, f) => x > 0 ? esc(f(x)) : "—";
  const head = [t("Agent"), t("Active time"), t("Sessions"), t("Prompts"), t("Estimated cost"), t("Kiro credits"), t("Tokens")].map((h, i) => `<th scope="col"${i ? ' class="num"' : ""}>${esc(h)}</th>`).join("");
  const L = [t("Active time"), t("Sessions"), t("Prompts"), t("Estimated cost"), t("Kiro credits"), t("Tokens")].map(esc);
  const rows = xs.map(x => `<tr><th scope="row"><span class="iwn"><i style="background:${x.c}"></i>${esc(x.key)}</span></th><td class="num" data-l="${L[0]}"><span class="iagb"><span style="width:${((x.minutes || 0) * 100 / max).toFixed(1)}%;background:${x.c}"></span></span>${v(x.minutes, dur)} <b>${Math.round((x.minutes || 0) * 100 / tot)}%</b></td><td class="num" data-l="${L[1]}">${commas(x.n[0])}</td><td class="num" data-l="${L[2]}">${commas(x.n[1])}</td><td class="num" data-l="${L[3]}">${v(x.cost, usd)}</td><td class="num" data-l="${L[4]}">${v(x.credits, cr)}</td><td class="num" data-l="${L[5]}">${v(x.tokens, tok)}</td></tr>`).join("");
  const sy = t("{name}: {pc}% of {metric} ({v})", {name: xs[0].key, pc: Math.round(xs[0].minutes * 100 / tot), metric: t("Active time"), v: dur(xs[0].minutes)});
  return `${say(sy)}<figure class="insfig"><div class="iagw"><table class="iagt">
    <thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div><p class="ifxs">${fx(`% = ${t("Active time")} ÷ ${dur(tot)}`)}${fx(t("Time that agents ran at once is split between them"))}</p></figure>`;
}
function insTokens(){
  const {S: w} = period(), u = w && w.usage; if (!u || !u.tokens || u.cr == null && u.in == null) return "";
  const rows = [["in", t("Input"), u.in || 0, 0], ["out", t("Output"), u.out || 0, 1], ["cw", t("Cache write"), u.cw || 0, 2], ["cr", t("Cache read"), u.cr || 0, 6]].filter(r => r[2] > 0);
  const tot = rows.reduce((a, r) => a + r[2], 0); if (!tot) return "";
  const pc = v => Math.round(v * 100 / tot), inAll = (u.in || 0) + (u.cw || 0) + (u.cr || 0), b = insBase("cache");
  const bar = `<div class="itbar" role="img" aria-label="${esc(rows.map(r => `${r[1]} ${pc(r[2])}%`).join(", "))}">${rows.map(r => `<span style="flex:${r[2]};background:var(--c${r[3]})"${tipAttr(r[1], `${tok(r[2])} · ${pc(r[2])}%`)}></span>`).join("")}</div>`;
  const list = rows.map(r => `<div class="iwr"><span class="iwn"><i style="background:var(--c${r[3]})"></i>${esc(r[1])}</span><span class="iwb"><span style="width:${(r[2] * 100 / Math.max(...rows.map(x => x[2]))).toFixed(1)}%;background:var(--c${r[3]})"></span></span><span class="iwv">${esc(tok(r[2]))}</span><b>${pc(r[2])}%</b></div>`).join("");
  const top = rows.reduce((a, r) => r[2] > a[2] ? r : a);
  const sy = t("{name}: {pc}% of tokens ({v})", {name: top[1], pc: pc(top[2]), v: tok(top[2])});
  const th = b && b.better === false ? (H().cache || {}).a || "" : "";
  return `${say(sy, th)}<figure class="insfig">${bar}<div class="iwrows">${list}</div>
    <p class="ifxs">${fx(`% = ${t("tokens of the type")} ÷ ${tok(tot)}`)}${inAll ? fx(`${t("read from cache")} = ${tok(u.cr || 0)} ÷ ${tok(inAll)} = ${Math.round((u.cr || 0) * 100 / inAll)}%`) : ""}${b ? insChip(b) : ""}</p></figure>`;
}
