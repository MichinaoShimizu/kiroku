// サマリーの頭：要点の文・推移のグラフ（自分のいつもの範囲つき）・改善のアドバイス。AI は使わず、決まった計算と規則だけで出す
/* いつもの範囲：表示中の期間の前の INS_N 期間（記録が始まる前の期間と、記録が途中から始まる期間は除く）の分布。
   中央値と四分位（真ん中の 50%）で見る。平均と標準偏差は、外れた週 1 つで大きく動くので使わない */
const INS_N = 12, INS_MIN = 4; // 比べる期間の数と、比べるのに要る最低の数（それより少なければ「まだ比べられない」と言う）
const INS_ADD = new Set(["cost", "credits", "active", "prompts", "tokens", "gitCommits", "limits", "longctx", "friction", "bigPrompts", "focus"]); // 期間の長さに比例して増える指標（途中の期間は、このペースで期間を終えたときの値で比べる）
const INS_KEYS = ["cost", "credits", "tokens", "active", "costPerCommit", "cache", "fix", "wait"]; // グラフで切り替えて見られる指標
const INS_SHORT = () => ({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), active: t("Time"), costPerCommit: t("Cost per commit"), cache: t("Cache"), fix: t("Corrections"), wait: t("Wait")}); // 切り替えの短い名前（正式な名前は title と、グラフの下の文）
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
const insTone = b => !b || b.q1 == null || b.better == null ? "" : b.better ? " good" : " bad";
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
  const {S: w, P: pw, ws, we} = period(), T = [], u = (w && w.usage) || {}, M = st.mode === "month";
  const tile = (label, b, v, f, m) => T.push(`<div class="itile${insTone(b)}"><span class="il">${esc(label)}</span>${m ? `<button type="button" class="itv" data-metric="${m}" aria-haspopup="dialog" aria-label="${esc(`${label} ${v}`)}">${esc(v)}</button>` : `<b>${esc(v)}</b>`}${[].concat(f || []).map(x => typeof x === "object" ? fx(x.f, "", x.p) : x === t("Not an estimate") ? pln(x) : fx(x)).join("")}${b === false ? "" : insChip(b)}</div>`); // 値を押すと、上の帯と同じ内訳
  const days = Math.round((we - ws) / 86400), el = Math.round(insFrac() * days), V = vsPrev(pw, M ? "月" : "週");
  // 上の帯の増減（%）と同じ比べを式で：途中の期間は前の期間の同じ日まで、終わった期間は前の期間まるごと
  const vs = (a, b, f) => { if (b == null || !b) return ""; const pc = `${a >= b ? "+" : "−"}${Math.round(Math.abs(a - b) * 100 / b)}% ${V.n == null ? (M ? t("vs last month") : t("vs last week")) : t("vs {range}", {range: V.range})}`;
    return {f: `(${f(a)} − ${f(b)}) ÷ ${f(b)} = ${pc}`, p: pc}; };
  const pace = (cur, v, f) => ({f: `${t("pace")} = ${f(cur)} × ${days}${t("d")} ÷ ${el}${t("d")} = ${f(v)}`, p: t("At this pace: {v}", {v: f(v)})});
  const pu = pw && pw.usage;
  const bc = insBase("cost"); if (bc && (u.tokens || bc.cur)) tile(t("Estimated cost"), bc, fmtM("cost", bc.cur), [vs(bc.cur, V.of("cost", pu && pu.tokens ? costOf(pu) : null), usd), bc.part ? pace(bc.cur, bc.v, usd) : ""].filter(Boolean), "cost");
  if (u.credits){ const bk = insRange("credits", u.credits, p => p.S && p.S.usage ? p.S.usage.credits || null : null, true, true, v => `${crN(v)} cr`);
    const mp = monthProj(), r1 = v => `${Math.round(v * 10) / 10} cr`; // 週の表示でも、クレジットの今月末の推定を添える（クレジットは月ごとの枠。カードは増やさない）
    const me = mp && mp.credits != null ? {f: `${t("Month-end credits")} = ${t("this month")} ${r1(mp.soCredits)} ÷ ${mp.days}${t("d")} × ${mp.nd}${t("d")} = ${r1(mp.credits)}`, p: t("Month-end (this month): ≈ {v}", {v: `${crN(mp.credits)} cr`})} : "";
    tile(t("Kiro credits"), bk, `${crN(u.credits)} cr`, [vs(u.credits, V.of("credits", pu && pu.credits), r1)].concat(bk.part ? pace(u.credits, bk.v, r1) : t("Not an estimate"), me).filter(Boolean), "credits"); }
  else if (u.tokens) { const bt = insBase("tokens"); tile(t("Tokens"), bt, tok(u.tokens), `${t("output")} ${tok(u.out)} ÷ ${tok(u.tokens)} = ${Math.round(u.out * 100 / u.tokens)}%`, "tokens"); }
  // 予算に対する位置：途中の月は月末の推定、途中の週は前の週並みに収める 1 日の目安、終わった期間は前の期間との差
  const pj = projection(w), pc = pacing(w, pw), fmc = x => x === "credits" ? (v => `${Math.round(v * 10) / 10} cr`) : usd; // 式の中のクレジットは小数 1 桁（整数に丸めると、引き算の結果と合わなくなる）
  const pjf = (p, k, so) => `${fmc(k)(so)} ÷ ${p.days}${t("d")} × ${p.nd}${t("d")} = ${fmc(k)(p[k])}`; // 月末の推定の式（今日までの実績 ÷ 日数 × 月の日数）
  if (pj && (pj.cost != null || pj.credits != null)){ const ks = ["cost", "credits"].filter(k => pj[k] != null); // 推定費用とクレジットは別の枠なので、足さずに「 · 」で並べる（上の帯と同じ。内訳から、もう片方の内訳へ移れる）
    tile(t(ks.length > 1 ? "Month-end (estimate)" : ks[0] === "cost" ? "Month-end cost" : "Month-end credits"), false, ks.map(k => `≈ ${fmc(k)(pj[k])}`).join(" · "), ks.map(k => pjf(pj, k, k === "cost" ? costOf(u) : u.credits)), ks[0] === "cost" ? "projection" : "projectionCr"); }
  else if (pc){ // 費用とクレジットのそれぞれについて：残りの日 1 日あたりの目安か、すでに超えた額（式で）
    const xs = ["cost", "credits"].filter(k => pc[k]).map(k => ({k, x: pc[k], f: fmc(k)})), ok = xs.find(o => o.x.rest >= 0), fs = xs.map(({x, f}) => x.rest >= 0 ? `(${f(x.ref)} − ${f(x.so)}) ÷ ${pc.left}${t("d")} = ${f(x.per)}/${t("d")}` : `${f(x.so)} − ${f(x.ref)} = +${f(-x.rest)} ${t("over")}`);
    if (ok) tile(M ? t("A day to match last month") : t("A day to match last week"), false, `${ok.f(ok.x.per)}/${t("d")}`, fs, ok.k === "cost" ? "pace" : "paceCr");
    else tile(M ? t("Over last month's total") : t("Over last week's total"), false, `+${xs[0].f(-xs[0].x.rest)}`, fs, xs[0].k === "cost" ? "pace" : "paceCr"); }
  else if (u.tokens && costOf(u) != null){ const a = costOf(u), dd = w.days.filter(d => d.active).length; // 終わった期間：使った日 1 日あたり（前の期間との差は推定費用のカードの式にある）
    if (dd) tile(t("Cost per active day"), false, `${usd(a / dd)}/${t("d")}`, `${usd(a)} ÷ ${dd}${t("d")} = ${usd(a / dd)}`); }
  const ba = insBase("active"); if (ba) tile(t("Active time"), ba, fmtM("active", ba.cur), [vs(ba.cur, V.of("active", pw && pw.active), v => dur(v))].concat(ba.part ? pace(ba.cur, ba.v, v => dur(v)) : "").filter(Boolean), "active");
  const bp = insBase("costPerCommit"), oc = w && w.outCost, nc = w && w.outputs && w.outputs.commits;
  if (bp) tile(t("Cost per commit"), bp, fmtM("costPerCommit", bp.cur), oc && nc ? `Claude Code ${usd(oc)} ÷ ${nc} ${t("AI commits")} = ${usd(bp.cur)}` : "", "costPerCommit");
  const f0 = F[0], g0 = f0 && (GOTO[f0.k] || f0.k), b0 = f0 && MET[f0.k] ? insBase(f0.k) : null, nIds = new Set(F.map(f => GOTO[f.k] || f.k)).size;
  const n0 = f0 && F.filter(f => (GOTO[f.k] || f.k) === g0).length === 1 && H()[f0.k] ? H()[f0.k].n : g0 && insName(g0); // 開くダイアログの見出しと同じ名前
  T.push(f0 ? `<div class="itile${b0 && b0.better === false ? " bad" : ""}"><span class="il">${esc(t("Change first"))}${nIds > 1 ? ` · ${esc(t("{n} worth a look", {n: nIds}))}` : ""}</span><button type="button" class="itv sm insgo" data-goto="${esc(g0)}" aria-haspopup="dialog">${esc(n0)}</button><span class="isee">${esc(f0.L.see)}</span>${fx(t("Threshold: {rule}", {rule: f0.L.rule}), "", t("Threshold: {rule}", {rule: f0.L.rule}))}${b0 ? insChip(b0) : ""}</div>`
    : `<div class="itile good"><span class="il">${esc(t("Change first"))}</span><b class="sm">${esc(t("Nothing"))}</b>${pln(t("No metric crossed its threshold"))}</div>`);
  return `<div class="insh">${T.join("")}</div><p class="ifxs insdef">${fx(insDef())}</p>`;
}
/* 推移のグラフ：前の INS_N 期間と表示中の期間の棒。いつもの範囲（四分位）を帯、中央値を点線で重ねる。
   途中の期間は、今の値の棒と、このペースで終えたときの値の枠。範囲から大きく外れた期間には印 */
function insChart(k){
  const all = []; for (let i = INS_N; i >= 0; i--){ const p = periodBack(i); all.push({p, v: p.ws < historyStart() && i ? null : metricAt(k, p)}); }
  const f0 = Math.min(Math.max(0, all.findIndex(x => x.v != null)), all.length - 6), ps = all.slice(f0); // 記録の始まる前の期間は描かない（6 期間は残す）
  const b = insBase(k), cur = ps[ps.length - 1], proj = b && b.part ? b.v : null;
  const vals = ps.map(x => x.v).filter(v => v != null); if (!vals.length) return `<p class="note">${esc(t("No records"))}</p>`;
  // 狭い画面は小さな座標で描く（縮めて文字が読めなくならないように）
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 180 : 200, L = narrow ? 52 : 64, R = 8, T = 12, B = 26, top = insNice(Math.max(...vals, proj || 0, b && b.q3 || 0) * 1.05);
  const bw = (W - L - R) / ps.length, x = i => L + i * bw, y = v => T + (Hh - T - B) * (1 - v / top);
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 8}" y="${(y(v) + 4).toFixed(1)}">${esc(fmtM(k, v))}</text></g>`).join("");
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
  const th = !b || b.q1 == null ? "" : b.pos === "typical" ? t("Within the middle half of your past {unit}", {unit: insNoun()}) : b.better === false ? t("Outside the middle half, on the worse side; \"Change first\" and the warning marks show where it came from") : b.better ? t("Outside the middle half, on the better side; keep what you changed") : b.pos === "high" ? t("Above the 75th pct of your past {unit}", {unit: insNoun()}) : t("Below the 25th pct of your past {unit}", {unit: insNoun()});
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
  const avail = INS_KEYS.filter(x => insBase(x) && !(noCost && (x === "cost" || x === "costPerCommit"))), k = avail.includes(st.insK) ? st.insK : avail[0] || "active";
  const tabs = avail.map(x => `<button type="button" data-ins="${x}" aria-pressed="${x === k}" title="${esc(insName(x))}">${esc(INS_SHORT()[x])}</button>`).join("");
  const useCostW = ((period().S || {}).days || []).some(d => d.cost > 0); // 費用の記録がなければ、時間・件数の図になるので見出しもそう言う
  const card = (title, body, cls = "") => body ? `<div class="inscol${cls}"><h4>${esc(title)}</h4>${body}</div>` : "";
  // グラフのカードは 2 列に並べる。エージェント別の表は幅いっぱい。2 枚の組にならずに 1 枚だけ残ったカードも幅いっぱいにする（空きを作らない）
  const sel = `<select class="insksel" aria-label="${esc(t("Metric"))}">${avail.map(x => `<option value="${x}"${x === k ? " selected" : ""}>${esc(INS_SHORT()[x])}</option>`).join("")}</select>`; // 狭い画面では、切り替えの代わりに選択リスト（横に隠さない）
  const cards = [
    {h: `<div class="inscol"><div class="inshead"><h4>${esc(t("Trend"))}</h4><div class="segc insk trendk" role="group" aria-label="${esc(t("Metric"))}">${tabs}</div>${sel}</div>${insChart(k)}</div>`},
    {h: card(useCostW ? (st.mode === "month" ? t("Cumulative cost vs the previous month") : t("Cumulative cost vs the previous week")) : (st.mode === "month" ? t("Cumulative active time vs the previous month") : t("Cumulative active time vs the previous week")), insCum())},
    {h: card(t("By agent"), insAgents()), full: true},
    {h: (b => b ? `<div class="inscol">${b}</div>` : "")(insWhere())}, {h: card(t("Tokens by type"), insTokens())},
    {h: card(useCostW ? t("Cost by session length") : t("Sessions by length"), insHist())}, {h: card(t("Cost vs outputs per session"), insScatter())},
    {h: card(t("Outputs (Git)"), insOutputs())}, {h: card(t("How you used AI"), insUsage())},
    {h: card(t("When you worked"), insHeat())}, {h: card(t("How you worked"), insWork())},
  ].filter(c => c.h);
  let run = []; const out = [], flush = () => { if (run.length % 2) run[run.length - 1].full = true; out.push(...run); run = []; };
  cards.forEach(c => { if (c.full){ flush(); out.push(c); } else run.push(c); }); flush();
  return `<section class="insight" aria-labelledby="ins-h"><h3 id="ins-h" class="sr">${esc(t("Overview"))}</h3>${insHeadlines(F)}
    <div class="insg">${out.map(c => c.full ? c.h.replace(/^<div class="inscol/, '<div class="inscol full') : c.h).join("")}</div></section>`;
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
  R.querySelectorAll(".insksel").forEach(sl => sl.onchange = () => { st.insK = sl.value; store.set("insK", st.insK); summary(); const ns = $("#review .insksel"); if (ns) ns.focus(); });
  R.querySelectorAll(".insgo").forEach(b => b.onclick = () => insOpenWorth(b.dataset.goto, b));
  R.querySelectorAll("[data-where]").forEach(b => b.onclick = () => { st.insW = b.dataset.where; store.set("insW", st.insW); summary(); const nb = $("#review").querySelector(`[data-where="${CSS.escape(b.dataset.where)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll(".insfig [data-sid]").forEach(c => { c.onclick = () => select(c.dataset.sid); c.onkeydown = e => { if (e.key === "Enter" || e.key === " "){ e.preventDefault(); select(c.dataset.sid); } }; }); // 散布図の点は、そのセッションを開く
}

/* 費用と成果の散布図：表示中の期間に始まったセッションごとに、横は推定費用（平方根の目盛り）、縦は成果（そのセッションの間のコミットと作った PR）。
   費用が上位 25%（かつ $0.5 以上）なのに成果のないセッションを「見直す価値のある領域」として色を変え、件数と費用を文で言う */
function insScatter(){
  const {ws, we} = period(), xs = DATA.filter(s => s.start >= ws && s.start < we && s.cost > 0).map(s => ({s, c: s.cost, o: commitsOf(s).length + (s.prs || []).length}));
  if (xs.length < 3) return "";
  const cs = xs.map(x => x.c), thr = Math.max(0.5, insQuant(cs, .75)), maxC = insNice(Math.max(...cs)), maxO = Math.max(4, ...xs.map(x => x.o));
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 214 : 234, L = narrow ? 40 : 48, R = 12, T = 12, B = 44; // 下に目盛りと軸の名前（点に重ならないように、目盛りの下）
  const x = c => L + (W - L - R) * Math.sqrt(c / maxC), y = o => T + (Hh - T - B) * (1 - o / maxO);
  const bad = xs.filter(p => p.o === 0 && p.c >= thr), badC = bad.reduce((t, p) => t + p.c, 0);
  const xt = [0, maxC / 4, maxC].map(c => `<g class="itk"><line x1="${x(c).toFixed(1)}" x2="${x(c).toFixed(1)}" y1="${T}" y2="${Hh - B}"/><text class="ixl" x="${x(c).toFixed(1)}" y="${Hh - B + 16}">${esc(usd(c))}</text></g>`).join("");
  const yt = [0, Math.round(maxO / 2), maxO].map(o => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(o).toFixed(1)}" y2="${y(o).toFixed(1)}"/><text x="${L - 8}" y="${(y(o) + 4).toFixed(1)}">${o}</text></g>`).join("");
  const zx = Math.min(W - R, x(Math.min(thr, maxC))); // しきい値が軸の外（どのセッションも $0.50 未満）なら、帯は描かない
  const zone = zx >= W - R ? "" : `<rect class="izone" x="${zx.toFixed(1)}" y="${(y(0) - 14).toFixed(1)}" width="${(W - R - zx).toFixed(1)}" height="14"/>`;
  const pts = xs.map((p, i) => { const hit = p.o === 0 && p.c >= thr, jy = p.o === 0 ? -((i * 37) % 9) - 3 : 0; // 成果 0 の点は重なりやすいので、縦に少しずらす
    return `<circle class="ipt${hit ? " bad" : ""}" cx="${x(p.c).toFixed(1)}" cy="${(y(p.o) + jy).toFixed(1)}" r="${narrow ? 4 : 5}" tabindex="0" role="button" data-sid="${esc(p.s.id)}" aria-label="${esc(`${p.s.title || p.s.id}: ${usd(p.c)}, ${plural(p.o, "output")}`)}"${tipAttr(p.s.title || "", `${usd(p.c)} · ${plural(p.o, "output")}`, p.s.project)}/>`; }).join("");
  const note = `${t("outputs")} = 0 ∧ ${t("cost")} ≥ ${usd(thr)} : ${bad.length} · ${usd(badC)}`;
  const tq = xs.filter(p => p.c >= thr).length, all$ = cs.reduce((a, c) => a + c, 0);
  const sy = t("{m} of {sessions} costing {thr} or more had no output: {usd} = {pc}% of session cost", {m: bad.length, sessions: plural(tq, "session"), thr: usd(thr), usd: usd(badC), pc: all$ ? Math.round(badC * 100 / all$) : 0});
  const th = bad.length ? t("Open the red points: research is fine; work that stopped partway is the cost to cut") : "";
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="group" aria-label="${esc(t("Cost and outputs per session"))}">${xt}${yt}${zone}${pts}
    <text class="iax" x="${W - R}" y="${Hh - 1}">${esc(t("Estimated cost (√ scale) →"))}</text><text class="iax st" x="${L + 4}" y="${T + 10}">${esc(t("↑ Commits and PRs"))}</text></svg><figcaption><span class="lg izone">${fx(note, "", t("Costly sessions with no output"))}</span></figcaption><p class="ifxs">${fx(`${t("outputs")} = ${t("commits")} + PR`)}${fx(`${usd(thr)} = max($0.50, P75(${t("cost")}))`)}</p></figure>`;
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
function insCum(){
  const {S: w, P: pw} = period(); if (!w) return "";
  const useCost = w.days.some(d => d.cost > 0), f = d => (useCost ? d.cost : d.active) || 0, fm = useCost ? usd : v => dur(v);
  const cum = ds => { let a = 0; return ds.map(d => a += f(d)); };
  const fr = insFrac(), n = Math.max(w.days.length, pw ? pw.days.length : 0), upto = fr < 1 ? Math.max(1, Math.ceil(fr * w.days.length)) : w.days.length;
  const cur = cum(w.days).slice(0, upto), prev = pw ? cum(pw.days) : []; // 前の期間は全部（月の日数が違っても、合計は前の月まるごと）
  const top = insNice(Math.max(...cur, ...prev, 0) * 1.05);
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 180 : 170, L = narrow ? 52 : 64, R = 10, T = 10, B = 24;
  const x = i => L + (W - L - R) * (n === 1 ? 0 : i / (n - 1)), y = v => T + (Hh - T - B) * (1 - v / top), line = a => a.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 6}" y="${(y(v) + 4).toFixed(1)}">${esc(fm(v))}</text></g>`).join("");
  const a = new Date(period().ws * 1000), xl = [0, n - 1].map(i => `<text class="ixl ${i ? "e" : "s"}" x="${x(i).toFixed(1)}" y="${Hh - 6}">${esc(dMD(addDays(a, i)))}</text>`).join(""); // 両端の日付は、内側に寄せる（はみ出さないように）
  if (useCost){ const a = costOf(w.usage || {}), b = pw ? costOf(pw.usage || {}) : null; // 合計は帯と同じ値（日ごとの丸めを足すと、帯と 1 セントずれる）
    if (a != null && cur.length) cur[cur.length - 1] = a; if (b != null && prev.length) prev[prev.length - 1] = b; }
  const last = cur.length - 1, ptot = prev.length ? prev[prev.length - 1] : null, pref = fr >= 1 ? ptot : prev.length > last ? prev[last] : null, diff = pref == null ? null : cur[last] - pref; // 終わった期間は前の期間の合計と、途中の期間は同じ日までと比べる
  const sy = diff == null ? "" : (fr >= 1 ? t("Total {diff} ({pc}) vs the previous period's total", {diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: pref ? `${diff >= 0 ? "+" : ""}${Math.round(diff * 100 / pref)}%` : "—"})
      : t("Day {d} of {n}: {diff} ({pc}) vs the previous period", {d: last + 1, n: w.days.length, diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: pref ? `${diff >= 0 ? "+" : ""}${Math.round(diff * 100 / pref)}%` : "—"}))
    + (fr < 1 && ptot != null ? t("; at this pace {end} at the end vs {prev}", {end: fm(cur[last] / fr), prev: fm(ptot)}) : "");
  const left = n - (last + 1), rem = ptot == null ? null : ptot - cur[last];
  const th = fr >= 1 || rem == null || !useCost ? "" : rem <= 0 ? t("Already {x} over the previous period's total {prev}", {x: fm(-rem), prev: fm(ptot)}) : left === 1 ? t("To stay within the previous period's {prev}: at most {rem} tomorrow", {prev: fm(ptot), rem: fm(rem)}) : left > 0 ? t("To stay within the previous period's {prev}: at most {rem} in the {d} left = {per}/day", {prev: fm(ptot), rem: fm(rem), d: plural(left, "day"), per: fm(rem / left)}) : "";
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(`${fm(cur[last])}${prev.length ? ` / ${fm(prev[Math.min(last, prev.length - 1)])}` : ""}`)}">${ticks}${xl}
    ${prev.length ? `<polyline class="iprev" points="${line(prev)}"/>` : ""}<polyline class="icum" points="${line(cur)}"/><circle class="icumd" cx="${x(last).toFixed(1)}" cy="${y(cur[last]).toFixed(1)}" r="4"/></svg>
    <figcaption><span class="lg icum">${esc(soFar() != null ? (st.mode === "month" ? t("This month") : t("This week")) : (st.mode === "month" ? t("Month shown") : t("Week shown")))} <b>${esc(fm(cur[last]))}</b></span>${prev.length ? `<span class="lg iprev">${esc(st.mode === "month" ? t("Previous month") : t("Previous week"))}</span>` : ""}</figcaption>${diff != null ? `<p class="ifxs">${fx(`${fm(cur[last])} − ${fm(pref)} = ${(diff >= 0 ? "+" : "−") + fm(Math.abs(diff))}`, diff > 0 ? " bad" : " good")}<span class="note">${esc(fr >= 1 ? t("vs the previous period's total") : t("vs the same day of the previous period ({d})", {d: dMD(addDays(new Date(periodBack(1).ws * 1000), last))}))}</span></p>` : ""}</figure>`;
}
/* セッションの長さ別の費用：表示中の期間に動いたセッションを、期間の中の作業時間で 6 つに分け、区分ごとの推定費用の合計を棒に（件数は棒の下）。
   費用の記録がなければ件数で。どの長さのセッションに費用が集まっているかを読む */
function insHist(){
  const {ws, we} = period(), ss = DATA.filter(s => inP(s, ws, we)).map(s => ({m: sesMin(s), c: s.cost || 0})).filter(x => x.m > 0); if (ss.length < 3) return "";
  const edges = [0, 5, 15, 30, 60, 120, Infinity], labs = ["<5m", "5–15m", "15–30m", "30–60m", "1–2h", "2h+"], useC = ss.some(x => x.c > 0);
  const bins = labs.map((_, i) => ss.filter(x => x.m >= edges[i] && x.m < edges[i + 1])), val = b => useC ? b.reduce((a, x) => a + x.c, 0) : b.length;
  const vs = bins.map(val), tot = vs.reduce((a, v) => a + v, 0), mi = vs.indexOf(Math.max(...vs)), med = insQuant(ss.map(x => x.m), .5);
  const max = Math.max(...vs) || 1, W = innerWidth < 600 ? 360 : 560, Hh = 180, B = 34, T = 16, bw = W / labs.length, fv = v => useC ? usd(v) : String(v);
  const bars = vs.map((v, i) => { const h = (Hh - B - T) * v / max, xx = i * bw;
    return `<rect class="ihb${i === mi ? " med" : ""}" x="${(xx + bw * .14).toFixed(1)}" y="${(Hh - B - h).toFixed(1)}" width="${(bw * .72).toFixed(1)}" height="${h.toFixed(1)}" rx="3"${tipAttr(labs[i], plural(bins[i].length, "session"), useC ? usd(v) : "")}/>`
      + (v ? `<text class="ihn" x="${(xx + bw / 2).toFixed(1)}" y="${(Hh - B - h - 4).toFixed(1)}">${esc(fv(v))}</text>` : "") + `<text class="ixl" x="${(xx + bw / 2).toFixed(1)}" y="${Hh - 18}">${labs[i]}</text><text class="ixl sub" x="${(xx + bw / 2).toFixed(1)}" y="${Hh - 4}">n=${bins[i].length}</text>`; }).join("");
  const sy = useC ? t("Sessions of {bin}: {pc}% of cost ({usd}) in {sessions}", {bin: labs[mi], pc: Math.round(vs[mi] * 100 / tot), usd: usd(vs[mi]), sessions: plural(bins[mi].length, "session")}) : t("Most sessions ran {bin} ({n})", {bin: labs[mi], n: vs[mi]});
  const LC = useC ? longCtxOf(ws, we) : [];
  const th = mi >= 4 && LC.length ? t("{n} of the long sessions grew their input per response to up to {x}× the start: split them at a stopping point", {n: LC.length, x: Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0])))}) : "";
  return `${say(sy, th)}<figure class="insfig ihist"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(labs.map((l, i) => `${l} ${fv(vs[i])} n=${bins[i].length}`).join(", "))}">${bars}</svg>
    <p class="ifxs">${fx(`${t("median")}(${t("active time of {n} sessions", {n: ss.length})}) = ${dur(med)}`)}${useC ? fx(`% = ${t("cost of the bin")} ÷ ${usd(tot)}`) : ""}</p></figure>`;
}

/* どこに使ったか：プロジェクト・エージェント・モデルごとの費用（なければクレジット・トークン・時間）を、大きい順の横棒で。上位 7 つとその他 */
function insWhere(){
  const {S: w} = period(); if (!w) return "";
  const u = w.usage || {}, by = st.insW || "project";
  const src = by === "model" ? (u.models || []).map(r => ({name: r[0], cost: r[1], tokens: r[2], c: "var(--acc)"}))
    : by === "source" ? ((w.shares || {}).source || []).map(x => ({name: x.key, cost: x.cost, credits: x.credits, tokens: x.tokens, minutes: x.minutes, c: agColor(x.key)}))
    : (w.projectStats || []).map(p => ({name: p.project, cost: p.cost, credits: p.credits, tokens: p.tokens, minutes: p.minutes, c: projColor(p.project)}));
  const k = ["cost", "credits", "tokens", "minutes"].find(f => src.some(x => x[f] > 0)); if (!k) return "";
  const fv = {cost: usd, credits: v => `${crN(v)} cr`, tokens: tok, minutes: v => dur(v)}[k], name = {cost: t("Estimated cost"), credits: t("Kiro credits"), tokens: t("Tokens"), minutes: t("Active time")}[k];
  const xs = src.filter(x => x[k] > 0).sort((a, b) => b[k] - a[k]), top = xs.slice(0, 7), rest = xs.slice(7), tot = xs.reduce((a, x) => a + x[k], 0);
  if (rest.length) top.push({name: t("Other ({n})", {n: rest.length}), [k]: rest.reduce((a, x) => a + x[k], 0), c: "var(--other)"});
  let acc = 0, k80 = 0; for (const x of xs){ acc += x[k]; k80++; if (acc >= tot * .8) break; }
  const sy = t("{name}: {pc}% of {metric} ({v})", {name: xs[0].name, pc: Math.round(xs[0][k] * 100 / tot), metric: name, v: fv(xs[0][k])}) + (k80 < xs.length ? t("; 80% is in {k} of {n}", {k: k80, n: xs.length}) : "");
  const tabs = [["project", t("Project")], ["source", t("Agent")], ["model", t("Model")]].map(([v, l]) => `<button type="button" data-where="${v}" aria-pressed="${v === by}">${esc(l)}</button>`).join("");
  const rows = top.map(x => `<div class="iwr"><span class="iwn" title="${esc(x.name)}"><i style="background:${x.c}"></i>${esc(x.name)}</span><span class="iwb"><span style="width:${(x[k] * 100 / top[0][k]).toFixed(1)}%;background:${x.c}"></span></span><span class="iwv">${esc(fv(x[k]))}</span><b>${Math.round(x[k] * 100 / tot)}%</b></div>`).join("");
  return `<div class="inshead"><h4>${esc(t("Where it went"))}</h4><div class="segc insk" role="group" aria-label="${esc(t("Group by"))}">${tabs}</div></div>${say(sy)}
    <figure class="insfig"><div class="iwrows" role="table" aria-label="${esc(name)}">${rows}</div><p class="ifxs">${fx(`% = ${name} ÷ ${fv(tot)}`)}</p></figure>`;
}
/* 数の枠（statH）に、いつもの範囲との不等式を添える。k は MET の指標名（なければ添えない） */
const insStat = (label, v, sub, help, metric, k) => statH(esc(label), v, [sub, k && MET[k] ? insChip(insBase(k)) : ""].filter(Boolean).join(" "), help, metric);
/* 成果：コミット（うち AI）・変えた行・PR・成果まで届いたセッション・コミットあたりの費用。割った指標は式で */
function insOutputs(){
  const {S: w} = period(); if (!w) return "";
  const o = w.outputs || {commits: 0}, g = w.git && w.git.commits ? w.git : null, base = w.outBase ?? w.sessions, n = commas;
  if (!o.commits && !g && !o.prs) return "";
  const lines = g ? g.added + g.removed : 0, sp = base ? Math.round(w.outSessions * 100 / base) : null;
  const S = [
    g ? insStat(t("Git commits"), `${g.commits}`, fx(`AI ${g.ai} ÷ ${g.commits} = ${Math.round(g.ai * 100 / g.commits)}%`), "gitCommits", "commits", "gitCommits") : insStat(t("AI commits"), `${o.commits}`, "", "commits", o.commits ? "aiCommits" : ""),
    g ? insStat(t("Lines changed"), n(lines), fx(`+${n(g.added)} −${n(g.removed)}; ${n(lines)} ÷ ${g.commits} = ${n(Math.round(lines / g.commits))} ${t("per commit")}`), "lines", "lines") : "",
    o.prs ? insStat(t("Pull requests"), `${o.prs}`, esc(t("Created by AI")), "prs", "prs") : "",
    base ? insStat(t("Sessions that reached a commit or PR"), `${sp}<small>%</small>`, fx(`${w.outSessions} ÷ ${base} ${t("Claude Code sessions")} = ${sp}%`), "outSessions", "outSessions", "outSessions") : "",
    w.costPerCommit != null ? insStat(t("Estimated cost per commit"), usdH(w.costPerCommit), fx(`Claude Code ${w.outCost != null ? usd(w.outCost) : ""} ÷ ${o.commits} ${t("AI commits")} = ${usd(w.costPerCommit)}`), "costPerCommit", "costPerCommit", "costPerCommit") : "",
  ].join("");
  const sy = g ? t("{c} commits, {ai}% by AI; {sp}% of Claude Code sessions reached a commit or PR", {c: g.commits, ai: Math.round(g.ai * 100 / g.commits), sp: sp ?? "—"}) : t("{c} AI commits; {sp}% of Claude Code sessions reached a commit or PR", {c: o.commits, sp: sp ?? "—"});
  return `${say(sy)}<div class="stats istats">${S}</div>`;
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
    N ? norecStat(esc(t("Subagents")), N, "subagents") : insStat(t("Subagents"), `${u.subagents}`, u.subagents ? esc(t("Total run time {d}", {d: dur(u.subMin)})) + (u.subTypes.length ? " · " + esc(u.subTypes.slice(0, 3).map(([k, v]) => `${k} ${v}`).join(", ")) : "") : esc(t("Not used")), "subagents", u.subagents ? "subagents" : ""),
    pj && pj.cost != null ? insStat(t("Month-end cost (estimate)"), "≈ " + usdH(pj.cost), fx(`${t("so far")} ÷ ${pj.days}${t("d")} × ${w.days.length}${t("d")}`), "projection", "projection") : "",
    pj && pj.credits != null ? insStat(t("Month-end credits (estimate)"), `≈ ${crN(pj.credits)}<small> cr</small>`, fx(`${t("so far")} ÷ ${pj.days}${t("d")} × ${w.days.length}${t("d")}`), "projectionCr", "projectionCr") : "",
  ].join("");
  const sy = u.tokens ? t("{t} tokens; {c}% of input read from cache", {t: tok(u.tokens), c: u.cacheHit == null ? "—" : Math.round(u.cacheHit * 100)}) : t("{c} Kiro credits", {c: crN(u.credits)});
  return `${say(sy)}<div class="stats istats">${S}</div>`;
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
  const rows = xs.map(x => `<tr><th scope="row"><span class="iwn"><i style="background:${x.c}"></i>${esc(x.key)}</span></th><td class="num" data-l="${L[0]}"><span class="iagb"><span style="width:${((x.minutes || 0) * 100 / max).toFixed(1)}%;background:${x.c}"></span></span>${v(x.minutes, dur)} <b>${Math.round((x.minutes || 0) * 100 / tot)}%</b></td><td class="num" data-l="${L[1]}">${commas(x.n[0])}</td><td class="num" data-l="${L[2]}">${commas(x.n[1])}</td><td class="num" data-l="${L[3]}">${v(x.cost, usd)}</td><td class="num" data-l="${L[4]}">${v(x.credits, c => `${crN(c)} cr`)}</td><td class="num" data-l="${L[5]}">${v(x.tokens, tok)}</td></tr>`).join("");
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
