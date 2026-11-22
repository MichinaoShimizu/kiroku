// サマリーの頭：要点の文・推移のグラフ（自分のいつもの範囲つき）・改善のアドバイス。AI は使わず、決まった計算と規則だけで出す
/* いつもの範囲：表示中の期間の前の INS_N 期間（記録が始まる前の期間と、記録が途中から始まる期間は除く）の分布。
   中央値と四分位（真ん中の 50%）で見る。平均と標準偏差は、外れた週 1 つで大きく動くので使わない */
const INS_VOL = new Set(["cache", "offhours", "switches", "cost", "credits", "tokens", "active", "prompts", "gitCommits", "gitLines", "prs", "aiLines"]); // 使った量・作業の量（色で良し悪しを付けない）
const INS_USE = new Set(["cost", "credits", "tokens", "active"]); // 使った量（累積の線と利用上限の印を重ねる）
const INS_N = 12, INS_MIN = 4; // 比べる期間の数と、比べるのに要る最低の数（それより少なければ「まだ比べられない」と言う）
const INS_ADD = new Set(["cost", "credits", "active", "prompts", "tokens", "gitCommits", "gitLines", "prs", "aiLines", "limits", "longctx", "friction", "bigPrompts", "focus"]); // 期間の長さに比例して増える指標（途中の期間は、このペースで期間を終えたときの値で比べる）
const INS_KEYS = ["cost", "credits", "tokens", "active", "costPerCommit", "tokensPerCommit", "costPerEdit", "creditsPerEdit", "tokensPerEdit", "costPerHour", "creditsPerHour", "tokensPerHour", "costPerDay", "creditsPerDay", "tokensPerDay", "costPerSes", "creditsPerSes", "tokensPerSes", "costPerAsk", "creditsPerAsk", "tokensPerAsk", "cache", "fix", "wait"]; // グラフで切り替えて見られる指標
const INS_SHORT = () => ({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), active: t("Time"), costPerCommit: t("Cost per AI commit"), tokensPerCommit: t("Tokens per AI commit"), costPerEdit: t("Cost per 100 AI lines"), creditsPerEdit: t("Credits per 100 AI lines"), tokensPerEdit: t("Tokens per 100 AI lines"), costPerHour: t("Cost per hour"), creditsPerHour: t("Credits per hour"), tokensPerHour: t("Tokens per hour"), costPerDay: t("Cost per day"), creditsPerDay: t("Credits per day"), tokensPerDay: t("Tokens per day"), costPerSes: t("Cost per session"), creditsPerSes: t("Credits per session"), tokensPerSes: t("Tokens per session"), costPerAsk: t("Cost per prompt"), creditsPerAsk: t("Credits per prompt"), tokensPerAsk: t("Tokens per prompt"), cache: t("Cache"), fix: t("Corrections"), wait: t("Wait")}); // 切り替えの短い名前（正式な名前は title と、グラフの下の文）
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
  const better = pos === "typical" || low == null || INS_VOL.has(k) ? null : (pos === "low") === low; // 範囲の外で、よい側か（向きの決まっていない指標と、使った量・作業の量は null。量の増減に良し悪しは付けない）
  return {...r, q1, med, q3, pos, far, better, rank: Math.round(xs.filter(x => x < v).length * 100 / xs.length)};
}
const insName = k => (H()[k] || {}).n || k;
const insNoun = () => st.mode === "month" ? (JA ? "月" : "months") : (JA ? "週" : "weeks"); // 数を伴わない「過去の週・月」（日本語は「か月」にしない）
const insUnit = n => n === 1 && !JA ? (st.mode === "month" ? "month" : "week") : st.mode === "month" ? t("months") : t("weeks"); // 日本語は数によらず「か月」「週」
const say = (s, then) => (s ? `<p class="isay">${esc(s)}</p>` : "") + (then ? `<p class="ithen">→ ${esc(then)}</p>` : ""); // グラフの読み取り（データから計算した 1 文。見出しのすぐ下）
// 式の表示：値・式は <code class="ifx"> に入れる（文ではなく、計算だとわかるように）
// 式はサマリーでは、その数字の「ƒx」を押したときだけ見せる（.fxo。.fxon の中）。plain があれば、ふだんはその平易な 1 行を出す（.fxp）。ダイアログでは、いつも式を出す
const fx = (s, cls = "", plain = "", plainH = "") => `<code class="ifx fxo${cls}">${esc(s)}</code>` + (plain || plainH ? `<span class="fxp${cls}">${plainH || esc(plain)}</span>` : ""); // plainH は組み立て済みの HTML（lnPM など）
const pln = s => `<span class="iplain">${esc(s)}</span>`; // 式ではない短い文（いつも見せる）
// 四分位は記号でなく言葉で：下位25%点・上位25%点・幅（統計に慣れていなくても読めるように）
const LO = () => t("25th pct"), HI = () => t("75th pct"), SP = () => t("spread");
const insDef = n => n == null ? t("{lo}, {hi} = the values a quarter of the way from the bottom and from the top when your past {unit} with records (up to {n}) are sorted; half of them fall between. {sp} = {hi} − {lo}", {n: INS_N, unit: insNoun(), lo: LO(), hi: HI(), sp: SP()})
  : t("{lo}, {hi} = the values a quarter of the way from the bottom and from the top of your last {n} {unit}; half of them fall between. {sp} = {hi} − {lo}", {n, unit: insUnit(n), lo: LO(), hi: HI(), sp: SP()});
// insExpr は、比べた結果を不等式で（x は表示中の期間の値。途中の期間の、期間に比例する指標はペース）
function insExpr(b){
  if (!b || b.q1 == null) return t("To compare: needs {min} past {unit} (have {n})", {min: INS_MIN, unit: insUnit(INS_MIN), n: b ? b.n : 0});
  const fm = b.fmt || (x => fmtM(b.k, x)), x = (b.part ? t("On pace for") + " " : b.k === "wait" ? "P90 " : "") + fm(b.v), iqr = b.q3 - b.q1; // 待ち時間は 90 パーセンタイルで比べる
  if (b.pos === "typical") return `${LO()} ${fm(b.q1)} ≤ ${x} ≤ ${HI()} ${fm(b.q3)}`;
  if (b.pos === "high") return b.far ? `${x} > ${HI()} + 1.5×${SP()} = ${fm(b.q3 + 1.5 * iqr)}` : `${x} > ${HI()} = ${fm(b.q3)}`;
  return b.far ? `${x} < ${LO()} − 1.5×${SP()} = ${fm(b.q1 - 1.5 * iqr)}` : `${x} < ${LO()} = ${fm(b.q1)}`;
}
const insPosText = b => `${insExpr(b)} (${insDef(b && b.n)})`; // 読み上げと title 用
const insTone = b => !b || b.q1 == null || b.better == null ? "" : (b.better ? " good" : " bad") + (b.far ? " far" : ""); // 良い側・悪い側に外れたか（大きく外れたら far）。タイルの背景の色に使う
// insLoHi は、いつもの範囲の両端。単位は後ろに 1 度だけ（「14–35 credits」）、数えるものは整数で
function insLoHi(fm, b){
  const cnt = MET[b.k] && MET[b.k].int, f = cnt ? (v => fmtM(b.k, Math.round(v))) : fm, lo = f(b.q1), hi = f(b.q3), m = /^([\d.,]+)\s+(\D+)$/.exec(lo), m2 = /^([\d.,]+)\s+(\D+)$/.exec(hi);
  return m && m2 && m[2] === m2[2] ? {lo: m[1], hi} : {lo, hi};
}
// insPlain は、いつもの範囲との比べを言葉で（式を表示しないときのチップ）
function insPlain(b){
  if (!b || b.q1 == null) return insExpr(b);
  const fm = b.fmt || (x => fmtM(b.k, x)), x = (b.part ? t("On pace for") + " " : b.k === "wait" ? t("Slowest 10%") + " " : "") + fm(b.v), r = {x, ...insLoHi(fm, b)};
  return b.pos === "typical" ? t("{x}: within your usual {lo}–{hi}", r) : b.pos === "high" ? (b.far ? t("{x}: far above your usual {lo}–{hi}", r) : t("{x}: above your usual {lo}–{hi}", r))
    : b.far ? t("{x}: far below your usual {lo}–{hi}", r) : t("{x}: below your usual {lo}–{hi}", r);
}
/* insSpark は、要点の数字の横に置く推移のグラフ（正方形）。2 本の線を重ねる：
   - 推移：過去 INS_N 期間と表示中の期間の値（点と線）。いつもの範囲（真ん中の 50%）の帯と中央値つき。
     途中の期間で期間の長さに比例する指標は、上の帯と同じくペース換算の値を白抜きの点で描く（線は終わった期間だけ）
   - 累積：期間の長さに比例する量（費用・時間・コミットなど）は、グラフの左端の期間からの足し上げ（右の軸。表示中の期間は今までの実際の値）。
     率（コミットあたり・時間あたりなど）は足し上げられないので、左端からの期間の値の平均（同じ左の軸）
   軸は実線、目盛りは両方の軸に、y の目盛りには点線のグリッド */
function insSpark(b, label, W = 480, chg = false){
  if (!b || !b.k || !MET[b.k]) return "";
  const k = b.k, hs = historyStart(), fm = b.fmt || (x => fmtM(k, x)), pts = [];
  for (let i = INS_N; i >= 1; i--){ const p = periodBack(i); pts.push({p, v: p.ws < hs ? null : metricAt(k, p)}); }
  pts.push({p: periodBack(0), v: b.v, a: b.cur, cur: true});
  if (!pts.some(x => x.v != null)) return ""; // 記録が 1 期間でもあれば描く（月で見ると、履歴の短いエージェントのグラフだけが消えて、週と月で並びが変わらないように）
  // 累積（右の軸）と利用上限の印は、使った量（費用・クレジット・トークン・時間）だけ。コミットや行の数に重ねても、いつも右上がりなだけで読み取れることがない
  const f0 = Math.min(Math.max(0, pts.findIndex(x => x.p.we > hs)), pts.length - 4), ps = pts.slice(f0), n = ps.length, /* 記録の始まる前の期間は描かない（履歴の短い人のグラフが、右端に寄って空かないように。最大 13 期間で「?」の推移と同じ） */ use = INS_USE.has(k), add = use, cnt = INS_ADD.has(k) && !use;
  { let s = 0, c = 0; ps.forEach(q => { const a = q.cur ? q.a : q.v; if (a != null){ s += a; c++; } q.c = cnt || !c ? null : add ? s : s / c; }); }
  const vals = ps.filter(x => x.v != null).map(x => x.v).concat(add ? [] : ps.filter(x => x.c != null).map(x => x.c));
  const wk = st.mode !== "month", H = 270, L = 50, R = add ? 50 : 12, T = 20, rot = wk && n > 1 && (W - L - R) / (n - 1) < 30, B = rot ? 46 : 34, mx = Math.max(...vals, b.q3 || 0);
  // 目盛りは、各点の値を読み取れるよう細かく（6〜10 本ほど）、切りのよい刻みで。数える指標は整数、時間は 1 時間単位の刻み
  const Y1 = insTicks(k, MET[k].u === "%" ? Math.min(100, mx * 1.1) : mx * 1.02), top = Y1.top, x = i => L + (n === 1 ? (W - L - R) / 2 : i * (W - L - R) / (n - 1)), y = v => T + (H - T - B) * (1 - v / top);
  const cProj = add && b.part && ps[n - 1].c != null ? ps[n - 1].c - b.cur + b.v : null; // 途中の期間：このペースで期間を終えたときの累積
  const Y2 = add ? insTicks(k, Math.max(ps[n - 1].c, cProj || 0) * 1.02, Y1.n, true) : Y1, top2 = Y2.top, y2 = add ? (v => T + (H - T - B) * (1 - v / top2)) : y, y0 = y(0).toFixed(1);
  const tk = (yy, lab, right) => `<line class="tm" x1="${right ? W - R : L - 3}" x2="${right ? W - R + 3 : L}" y1="${yy}" y2="${yy}"/>${lab == null ? "" : `<text class="${right ? "r" : ""}" x="${right ? W - R + 5 : L - 5}" y="${(+yy + 3.5).toFixed(1)}">${esc(lab)}</text>`}`;
  // y 軸：0 から 4 等分の目盛り（軸の外に短い線、プロットの中に点線のグリッド。値は 0・半分・上端に。数える指標で 6.25 のような値を出さないように）
  const ticks = Array.from({length: Y1.n + 1}, (_, i) => { const v = Y1.step * i, yy = y(v).toFixed(1);
    return `<g class="tk">${i ? `<line class="grid" x1="${L}" x2="${W - R}" y1="${yy}" y2="${yy}"/>` : ""}${tk(yy, tickS(k, v, Y1.step))}</g>`; }).join("")
    + (add ? Array.from({length: Y2.n + 1}, (_, i) => tk(y2(Y2.step * i).toFixed(1), tickS(k, Y2.step * i, Y2.step), true)).join("") : "")
    + `<line class="axis" x1="${L}" x2="${L}" y1="${T}" y2="${y0}"/><line class="axis" x1="${L}" x2="${W - R}" y1="${y0}" y2="${y0}"/>${add ? `<line class="axis cum" x1="${W - R}" x2="${W - R}" y1="${T}" y2="${y0}"/>` : ""}`;
  const band = b.q1 == null ? "" : `<rect class="band" x="${L}" y="${y(b.q3).toFixed(1)}" width="${W - L - R}" height="${Math.max(1, y(b.q1) - y(b.q3)).toFixed(1)}"/><line class="med" x1="${L}" x2="${W - R}" y1="${y(b.med).toFixed(1)}" y2="${y(b.med).toFixed(1)}"/>`;
  const xtm = ps.map((_, i) => `<line class="tm" x1="${x(i).toFixed(1)}" x2="${x(i).toFixed(1)}" y1="${y0}" y2="${(+y0 + 3).toFixed(1)}"/>`).join("");
  // x 軸は、どの期間にも名前を付ける。週は短い日付（月/日）を斜めに、月は月の名前
  const sl = (p, i) => { const [yy, mm, dd] = p.key.split("-").map(Number), yr = i === 0 || mm === 1; return wk ? `${mm}/${dd}` : JA ? (yr ? `${yy}年${mm}月` : `${mm}月`) : (yr ? `${MON[mm - 1]} ${yy}` : MON[mm - 1]); }; // 月は、最初と 1 月に年を添える（同じ月の名前が 2 度出ても見分けられるように）
  const xl = ps.map((q, i) => rot ? `<text class="sxl rot${i === n - 1 ? " cur" : (n - 1 - i) % 2 ? " alt" : i === 0 ? " s" : ""}${i === 0 && q.v == null && n > 1 ? " nodat" : ""}" transform="translate(${x(i).toFixed(1)} ${(+y0 + 8).toFixed(1)}) rotate(-45)">${esc(sl(q.p, i))}</text>` : `<text class="sxl${i === n - 1 ? " cur" : (n - 1 - i) % 2 ? " alt" : i === 0 ? " s" : ""}${i === 0 && q.v == null && n > 1 ? " nodat" : ""}" x="${x(i).toFixed(1)}" y="${(+y0 + 14).toFixed(1)}">${esc(sl(q.p, i))}</text>`).join("");
  // 累積は時間の流れなので、途中の期間の「ここまで」は右端（期間の終わり）ではなく、期間のうち過ぎた割合の位置に置く。そこから期間の終わりの予測までを破線に
  const xc = i => ps[i].cur && b.part && n > 1 ? x(n - 2) + insFrac() * (x(n - 1) - x(n - 2)) : x(i);
  const cpts = ps.map((q, i) => q.c == null ? null : `${xc(i).toFixed(1)},${y2(q.c).toFixed(1)}`).filter(Boolean);
  const cum = cpts.length < 2 ? "" : (add ? `<polygon class="cuma" points="${x(ps.findIndex(q => q.c != null)).toFixed(1)},${y0} ${cpts.join(" ")} ${xc(n - 1).toFixed(1)},${y0}"/>` : "") + `<polyline class="cuml" points="${cpts.join(" ")}"/>`;
  const line = ps.map((q, i) => q.v == null || (q.cur && b.part) ? null : `${x(i).toFixed(1)},${y(q.v).toFixed(1)}`).filter(Boolean).join(" ");
  // 途中の期間の予測：前の期間の点からペース換算の点まで（推移）と、前の期間の累積から予測の累積まで（累積）を破線で
  const pv = n > 1 ? ps[n - 2] : null, prj = !b.part || !pv ? "" : (pv.v != null ? `<line class="prj" x1="${x(n - 2).toFixed(1)}" y1="${y(pv.v).toFixed(1)}" x2="${x(n - 1).toFixed(1)}" y2="${y(b.v).toFixed(1)}"/>` : "")
    + (cProj != null && ps[n - 1].c != null ? `<line class="cprj" x1="${xc(n - 1).toFixed(1)}" y1="${y2(ps[n - 1].c).toFixed(1)}" x2="${x(n - 1).toFixed(1)}" y2="${y2(cProj).toFixed(1)}"/><circle class="cprjd" cx="${x(n - 1).toFixed(1)}" cy="${y2(cProj).toFixed(1)}" r="3"/>` : "")
    + `<circle class="sofar" cx="${xc(n - 1).toFixed(1)}" cy="${y(b.cur).toFixed(1)}" r="2.5"/><text class="sofl" x="${(xc(n - 1) - 5).toFixed(1)}" y="${(y(b.cur) + (y(b.cur) < H - B - 22 ? 15 : -7)).toFixed(1)}">${esc(t("So far"))}</text>`; // 途中の期間の今までの値（印だけだと、データのずれに見える）
  const dots = ps.map((q, i) => q.v == null || q.cur ? "" : `<circle class="pt" cx="${x(i).toFixed(1)}" cy="${y(q.v).toFixed(1)}" r="2.5"/>`).join("") + `<circle class="now${b.part ? " part" : ""}" cx="${x(n - 1).toFixed(1)}" cy="${y(b.v).toFixed(1)}" r="4.5"/>`;
  const cl = add ? t("Cumulative") : t("Running average");
  // 前の期間からの増減（コストのカード）：各点の上に小さく %。途中の期間は、見出しの数字（同じ日数での比べ）と食い違わないように付けない
  const pcOf = i => { const a = ps[i], p = i > 0 ? ps[i - 1] : null; return !chg || !p || a.v == null || !(p.v > 0) || (a.cur && b.part) ? null : Math.round((a.v - p.v) * 100 / p.v); };
  const pcS = v => `${v > 0 ? "+" : v < 0 ? "−" : "±"}${Math.abs(v)}%`, vsL = st.mode === "month" ? t("vs last month") : t("vs last week");
  const pcl = !chg ? "" : ps.map((q, i) => { const v = pcOf(i); if (v == null) return ""; const yy = y(q.v), up = yy > T + 14; const end = i === n - 1 && n > 1; return `<text class="spc${end ? " e" : ""}" x="${(end ? x(i) - 7 : x(i)).toFixed(1)}" y="${(up ? yy - 8 : yy + 15).toFixed(1)}">${esc(pcS(v))}</text>`; }).join(""); // 右端の点は、右の軸の目盛りに重ならないよう左に寄せる
  const step = n > 1 ? (W - L - R) / (n - 1) : W - L - R, hits = ps.map((q, i) => `<rect class="hit" x="${(x(i) - step / 2).toFixed(1)}" y="0" width="${step.toFixed(1)}" height="${H - B}"${tipAttr(q.p.label, q.v == null ? t("No records") : (q.cur && b.part ? insEndLabel(fm(q.v)) : fm(q.v)), q.cur && b.part ? `${t("So far")}: ${fm(b.cur)}` : "", q.c == null ? "" : `${cl}: ${fm(q.c)}`, pcOf(i) == null ? "" : `${vsL} ${pcS(pcOf(i))}`, q.cur && cProj != null ? `${cl} (${insEndLabel(fm(cProj))})` : "")}/>`).join("");
  // 利用上限に当たった時刻：x 軸の上の小さな赤い三角（縦線はグラフを隠すので引かない）。期間の点はその期間の真ん中なので、時刻を「点 ± 半期間」の位置に置く。同じ期間の到達は 1 本にまとめ、回数を添える
  const lim = !use ? "" : ps.map((q, i) => ({i, q, h: limitHits(q.p.ws, q.p.we)})).filter(z => z.h.length).map(({i, q, h}) => {
    const idx = i, lx = (L + (n === 1 ? (W - L - R) / 2 : idx * (W - L - R) / (n - 1))).toFixed(1);
    const rows = h.slice(0, 6).map(z => z.r ? t("{time} (resets {r})", {time: `${md(z.t)} ${hm(z.t)}`, r: z.r}) : `${md(z.t)} ${hm(z.t)}`).concat(h.length > 6 ? ["…"] : []);
    return `<path class="slim" d="M${lx} ${(+y0 - 7).toFixed(1)}l-4.5 7h9z"/>${h.length > 1 ? `<text class="slimn" x="${lx}" y="${(+y0 - 10).toFixed(1)}">×${h.length}</text>` : ""}<rect class="hit slimhit" x="${(+lx - 6).toFixed(1)}" y="${(+y0 - 10).toFixed(1)}" width="12" height="12"${tipAttr(h.length === 1 ? t("Usage limit hits") : `${t("Usage limit hits")} ×${h.length}`, rows)}/>`; }).join("");
  const yt = (INS_SHORT()[k] || label || insName(k)), xt = st.mode === "month" ? t("Month") : t("Week");
  const list = ps.map(q => `${q.p.label} ${q.v == null ? "—" : fm(q.v)}${q.c == null ? "" : ` (${cl} ${fm(q.c)})`}`).join(LSEP());
  return `<span class="tspk"><span class="sr">${esc(t("Trend: {list}", {list}))}</span><svg aria-hidden="true" viewBox="0 0 ${W} ${H}"><text class="syt" x="${L}" y="11">${esc(yt)}</text>${add ? `<text class="syt r" x="${W - R}" y="11">${esc(cl)}</text>` : ""}${band}${ticks}${xtm}${cum}<polyline class="trl" points="${line}"/>${prj}${dots}${pcl}${xl}<text class="sxt" x="${(L + (W - L - R) / 2).toFixed(1)}" y="${H - 3}">${esc(xt)}</text>${hits}${lim}</svg></span>`;
}
/* insVerdict は、単価の数字の「評価」と「次の一手」（AI は使わず、決まった規則で）。
   評価：いつもの範囲（過去の真ん中 50%）より低ければ良い側（コミット・行あたりは「効率が良い」、時間・セッション・プロンプトあたりは「軽い」）。
   根拠：過去の期間で最も低い・高い → 同じ向きに 3 期間以上続いている → いつもの範囲、の順に 1 つ。
   次の一手：範囲の外に出たときだけ。悪い側なら、この期間の見直す候補（長い会話・モデル・繰り返し・つまずき・利用上限）から 1 つを原因の候補として開けるように。
   なければ、使用量の大きいセッション（内訳）を開けるように */
// insHabits は、表示中の期間の作業のしかたが、いつもとどう違ったか（セッションの長さ・1 セッションのプロンプト数）。
// 過去 INS_N 期間の中央値より 25% 以上離れたものだけを、短い文にする。dir は「軽い側（短い・少ない）」なら -1、「重い側」なら 1
function insHabits(){
  const {ws, we} = period(), hs = historyStart(), f = (a, b) => { const xs = DATA.filter(s => s.start >= a && s.start < b && minIn(s, a, b) > 0);
    return xs.length ? {min: xs.reduce((x, s) => x + minIn(s, a, b), 0) / xs.length, ask: xs.reduce((x, s) => x + (s.nPrompts || 0), 0) / xs.length} : null; };
  const cur = f(ws, we); if (!cur) return [];
  const past = []; for (let i = 1; i <= INS_N; i++){ const p = periodBack(i); if (p.ws >= hs){ const v = f(p.ws, p.we); if (v) past.push(v); } }
  if (past.length < INS_MIN) return [];
  const out = [], med = key => insQuant(past.map(v => v[key]), .5);
  const m1 = med("min"), m2 = med("ask");
  if (m1 && Math.abs(cur.min - m1) / m1 >= .25) out.push({dir: Math.sign(cur.min - m1), s: cur.min < m1 ? t("Shorter sessions (average {v}, usually {u})", {v: dur(Math.round(cur.min)), u: dur(Math.round(m1))}) : t("Longer sessions (average {v}, usually {u})", {v: dur(Math.round(cur.min)), u: dur(Math.round(m1))})});
  if (m2 && Math.abs(cur.ask - m2) / m2 >= .25) out.push({dir: Math.sign(cur.ask - m2), s: cur.ask < m2 ? t("Fewer prompts per session ({v}, usually {u})", {v: Math.round(cur.ask * 10) / 10, u: Math.round(m2 * 10) / 10}) : t("More prompts per session ({v}, usually {u})", {v: Math.round(cur.ask * 10) / 10, u: Math.round(m2 * 10) / 10})});
  return out;
}
function insVerdict(b, F, hab){
  const k = b.k, use = /^cost/.test(k) ? "cost" : /^credits/.test(k) ? "credits" : "tokens", M = st.mode === "month";
  if (b.q1 == null) return `<p class="iverd"><b>${esc(t("Not enough history to judge yet"))}</b></p>`;
  // 範囲の端からの離れ方が 1 割に満たなければ「いつも通り」と言う（わずかな差で良し悪しを言わない）
  const off = b.pos === "low" ? (b.q1 - b.v) / (b.q1 || 1) : b.pos === "high" ? (b.v - b.q3) / (b.q3 || 1) : 0, pos = b.far || off >= .1 ? b.pos : "typical";
  const tone = pos === "low" ? "good" : pos === "high" ? "bad" : "";
  const head = pos === "low" ? t("Below usual") : pos === "high" ? t("Above usual") : t("As usual");
  const n = b.xs.length, fm = b.fmt || (x => fmtM(k, x));
  const ser = []; for (let i = INS_N; i >= 1; i--){ const p = periodBack(i); if (p.ws >= historyStart()) ser.push(metricAt(k, p)); } ser.push(b.v);
  let run = 0, dir = 0; for (let i = ser.length - 1; i > 0 && ser[i] != null && ser[i - 1] != null; i--){ const d = Math.sign(ser[i] - ser[i - 1]); if (!d || (dir && d !== dir)) break; dir = d; run++; }
  const rng = () => { let lo = fm(b.q1); const hi = fm(b.q3), sx = (/\s[^\d\s]+$/.exec(hi) || [""])[0]; if (sx && lo.endsWith(sx)) lo = lo.slice(0, -sx.length); return {lo, hi}; }; // 「13.1 credits〜13.8 credits」を「13.1〜13.8 credits」に
  const why = pos === "typical" ? t("Typically {lo}–{hi}", rng()) : b.v < Math.min(...b.xs) ? t("Lowest of the last {n} {unit}", {n: n + 1, unit: insUnit(n + 1)}) : b.v > Math.max(...b.xs) ? t("Highest of the last {n} {unit}", {n: n + 1, unit: insUnit(n + 1)})
    : run >= 3 ? (dir > 0 ? t("Up {n} {unit} in a row", {n: run, unit: insUnit(run)}) : t("Down {n} {unit} in a row", {n: run, unit: insUnit(run)})) : t("Typically {lo}–{hi}", rng());
  let act = "";
  const good = (hab || []).filter(h => h.dir < 0), bad = (hab || []).filter(h => h.dir > 0);
  if (pos === "low") act = good.length ? esc(`${M ? t("Different this month:") : t("Different this week:")} ${good.map(h => h.s).join(LSEP())}`) : "";
  else if (pos === "high"){
    const id = ["heavy", "longctx", "modelfit", "repeats", "friction", "fix", "limits"].map(x => (F || []).find(f => f.k === x)).filter(Boolean).map(f => GOTO[f.k] || f.k)[0];
    act = bad.length ? esc(`${t("Likely cause:")} ${bad.map(h => h.s).join(LSEP())}`) + (id ? ` · <button type="button" class="iwlink" data-goto="${esc(id)}" aria-haspopup="dialog">${esc(insName(id))}</button>` : "") : id ? `${esc(t("Likely cause:"))} <button type="button" class="iwlink" data-goto="${esc(id)}" aria-haspopup="dialog">${esc(insName(id))}</button>`
      : `<button type="button" class="ivgo" data-metric="${use}" aria-haspopup="dialog">${esc(use === "cost" ? t("See the sessions that cost the most") : use === "credits" ? t("See the sessions that used the most credits") : t("See the sessions that used the most tokens"))}</button>`;
  }
  return `<div class="iverd${tone ? " " + tone : ""}"><p><b>${esc(head)}</b> — ${esc(why)}</p>${act ? `<p class="ivact">→ ${act}</p>` : ""}</div>`;
}
// tickS は目盛りの値。数だけの指標（クレジット・件数・割合）は刻みと同じ桁数でそろえる（0.25 刻みが「1.3」「0.75」と混ざらないように）
const tickS = (k, v, step) => { const dp = Math.min(3, (String(+step.toPrecision(6)).split(".")[1] || "").length);
  if (/^cost/.test(k) && dp > 2) return "$" + v.toFixed(dp); // 0.025 刻みの費用は 3 桁で（2 桁に丸めると $0.03・$0.05・$0.08 と刻みが不ぞろいに見える）
  if (!/^credits/.test(k) && MET[k] && MET[k].fmt) return tickM(k, v); return (dp ? v.toFixed(dp) : commas(v)) + (MET[k] && MET[k].u === "%" ? "%" : ""); };
// insEndLabel は、途中の期間をこのペースで終えたときの値の言い方（月は「月末の予測」、週は「週末の予測」）
const insEndLabel = v => st.mode === "month" ? t("Month-end estimate: {v}", {v}) : t("End-of-week estimate: {v}", {v});
// insTicks は、0 から v までを切りのよい刻み（1・2・2.5・5 の 10 の累乗倍）で want 本ほどに分ける。数える指標は整数の刻み、時間（分）は 1 時間単位
// fixed なら本数を want にそろえる（右の軸の目盛りを左の軸のグリッドに合わせる）
function insTicks(k, v, want = 8, fixed = false){
  const hr = k === "active", u = hr ? 60 : 1, r = Math.max(v, 0) / u / want || 1, e = Math.pow(10, Math.floor(Math.log10(r)));
  let step = [1, 2, 2.5, 5, 10].map(m => m * e).find(x => x >= r) * u;
  if (MET[k] && MET[k].int || hr) step = Math.max(u, Math.ceil(step / u) * u);
  if (MET[k] && MET[k].int && step % 1) step = Math.ceil(step);
  if (fixed){ while (step * want < v) step = [1, 2, 2.5, 5, 10].map(m => m * Math.pow(10, Math.floor(Math.log10(step / u)))).find(x => x * u > step) * u; return {step, n: want, top: step * want}; }
  const n = Math.max(1, Math.ceil(v / step - 1e-9)); return {step, n, top: step * n};
}
const insChip = b => !b || b.q1 == null ? "" : `<code class="ichip ifx${insTone(b)}" title="${esc(insDef(b && b.n))}"><span class="cx">${esc(insExpr(b))}</span><span class="cp">${esc(insPlain(b))}</span></code>`;
/* 要点の数字（最大 3 列）：上の帯と同じ数をサマリーでも、式といつもの範囲との不等式つきで。並びは判断に使う順：
   使った額（推定費用・クレジット、なければトークン）→ 予算に対する位置（前の期間並みに収める 1 日の目安／月末の推定／前の期間との差）→ 稼働時間 → 効率（コミットあたり費用）→ まず見直すもの */
function insHeadlines(F){
  const {S: w, P: pw, ws, we} = period(), T = {cost: [], outGit: [], outEdit: [], unitGit: [], unitEdit: [], unitHour: [], unitDay: [], unitSes: [], unitAsk: []}, u = (w && w.usage) || {}, M = st.mode === "month";
  let G = "cost"; // コスト（時間・クレジット・金額・トークン）・出力（Git と AI の編集）・単価（コスト ÷ 出力。割る数の元ごと）に分けて並べる
  let HAB = null; // 作業のしかたの違い（単価の評価で使う。期間ごとに 1 度だけ数える）
  // 途中の期間で、期間の長さに比例する数字には、どれも期間の終わりの予測を添える（費用・クレジット・時間・トークン・コミット・行・PR）
  const tile = (label, b, v, f, m) => { f = [].concat(f || []); if (b && b.part && !f.some(x => x && x.pc)) f.push(pace(b.cur, b.v, MET[b.k] && MET[b.k].int ? (z => fmtM(b.k, Math.round(z))) : (b.fmt || (z => fmtM(b.k, z))))); if (/^unit/.test(G) && b && b.k && MET[b.k]) f.unshift({v: insVerdict(b, F, HAB || (HAB = insHabits()))}); tile0(label, b, v, f, m);
    // 単価は、費用・クレジット・トークンのどれで見るかを、カードの頭の切り替えで選ぶ（同じ形のグラフが 3 つずつ並ばないように）
    if (/^unit/.test(G) && b && b.k){ const uk = /^cost/.test(b.k) ? "cost" : /^credits/.test(b.k) ? "credits" : "tokens"; T[G][T[G].length - 1] = T[G][T[G].length - 1].replace('<div class="itile', `<div data-uk="${uk}" class="itile`); UK.add(uk); } };
  const UK = new Set(); // 単価で記録のある量
  const SP = Object.fromEntries(Object.keys(T).map(g => [g, []])); // 各タイルのグラフ（1 つだけ残ったタイルに、横に広い形を足すため）
  const tile0 = (label, b, v, f, m) => T[G].push(`<div class="itile${insTone(b)}"><span class="il">${esc(label)}${hb(b && b.k && H()[b.k] ? b.k : m && H()[m] ? m : "")}</span>${[].concat(f || []).some(x => x && typeof x === "object" && x.big) ? /* 文字列には String.prototype.big があるので、オブジェクトだけを見る */ `<span class="ivk">${esc(t("So far"))}</span>` : ""}${m ? `<button type="button" class="itv" data-metric="${m}" aria-haspopup="dialog" aria-label="${esc(`${label} ${v}`)}">${esc(v)}</button>` : `<b>${esc(v)}</b>`}${[].concat(f || []).filter(x => x && typeof x === "object" && x.big).map(x => `<span class="ivbig"><span class="ivk">${esc(x.big.l)}</span><b>${esc(x.big.v)}</b></span>`).join("")}${[].concat(f || []).map(x => typeof x === "object" ? (x.big ? fx(x.f) : x.v != null ? x.v : x.n != null ? pln(x.n) : fx(x.f, "", x.p, x.h)) : x === t("Kiro's own count, not an estimate") ? pln(x) : fx(x)).join("")}${(SP[G][T[G].length] = {b, label, sp: insSpark(b, label, 480, G === "cost")}).sp}${b === false || [].concat(f || []).some(x => x && x.v != null) ? "" : insChip(b)}</div>`); // 値を押すと、上の帯と同じ内訳
  const days = Math.round((we - ws) / 86400), el = Math.round(insFrac() * days), V = vsPrev(pw, M ? "月" : "週");
  // 上の帯の増減（%）と同じ比べを式で：途中の期間は前の期間の同じ日まで、終わった期間は前の期間まるごと
  const vs = (a, b, f) => { if (b == null || !b) return "";
    // 前の期間が小さいと、% が「+1076%」のように大きくなって読めないので、3 倍を超える増減は差と元の値で
    if (Math.abs(a - b) > 3 * b){ const lab = V.n == null ? (M ? t("vs last month") : t("vs last week")) : t("vs {range}", {range: V.range}), d = `${a >= b ? "+" : "−"}${f(Math.abs(a - b))}`;
      return {f: `${f(a)} − ${f(b)} = ${d}`, p: `${lab} ${d} (${t("from {v}", {v: f(b)})})`}; }
    const pc = `${a >= b ? "+" : "−"}${Math.round(Math.abs(a - b) * 100 / b)}% ${V.n == null ? (M ? t("vs last month") : t("vs last week")) : t("vs {range}", {range: V.range})}`;
    return {f: `(${f(a)} − ${f(b)}) ÷ ${f(b)} = ${pc}`, p: vsFirst(pc)}; };
  // 平易な 1 行では「先週比」を先に（「+108 先週比」の +108 が、緑と赤の行数と同じ形に見えないように）
  const vsFirst = s => { const m = /^([+−±][^ ]+) (.+)$/.exec(s); return m ? `${m[2]} ${m[1]}` : s; };
  // 前の期間との比べ（率は %、数えるものは差）。数えるものは、途中の期間なら前の期間の同じ日数まで（上の帯と同じ）。率は前の期間まるごとと
  const vsk = (k, cur, field) => { const fm = v => fmtM(k, v), prev = MET[k] && MET[k].int ? V.of(field, metricAt(k, periodBack(1))) : V.partial ? null : metricAt(k, periodBack(1));
    if (prev == null || cur == null) return ""; const lab = V.n == null || !MET[k].int ? (M ? t("vs last month") : t("vs last week")) : t("vs {range}", {range: V.range});
    if (MET[k].int){ const d = cur - prev, p = `${d > 0 ? "+" : d < 0 ? "−" : "±"}${fm(Math.abs(d))} ${lab}`; return {f: `${fm(cur)} − ${fm(prev)} = ${p}`, p: vsFirst(p)}; }
    if (!prev) return ""; const pc = `${cur >= prev ? "+" : "−"}${Math.round(Math.abs(cur - prev) * 100 / prev)}% ${lab}`; return {f: `(${fm(cur)} − ${fm(prev)}) ÷ ${fm(prev)} = ${pc}`, p: vsFirst(pc)}; };
  const pace = (cur, v, f, fv = f) => ({f: `${t("pace")} = ${f(cur)} × ${days}${t("d")} ÷ ${el}${t("d")} = ${f(v)}`, big: {l: M ? t("Month-end estimate") : t("End-of-week estimate"), v: fv(v)}, pc: true}); // fv は大きな数字の形（今までの値と同じ形に） // 期間の終わりの推定は、今までの値と同じ大きさで並べる
  const pu = pw && pw.usage;
  const bc = insBase("cost"); if (bc && (u.tokens || bc.cur)) tile(t("Estimated cost"), bc, fmtM("cost", bc.cur), [vs(bc.cur, V.of("cost", pu && pu.tokens ? costOf(pu) : null), usd), bc.part ? pace(bc.cur, bc.v, usd) : ""].filter(Boolean), "cost");
  if (u.credits){ const bk = insRange("credits", u.credits, p => p.S && p.S.usage ? p.S.usage.credits || null : null, true, true, cr);
    const mp = monthProj(), r1 = cr1; // 週の表示でも、クレジットの今月末の推定を添える（クレジットは月ごとの枠。カードは増やさない）
    const me = mp && mp.credits != null ? {f: `${t("Month-end credits")} = ${t("this month")} ${r1(mp.soCredits)} ÷ ${mp.days}${t("d")} × ${mp.nd}${t("d")} = ${r1(mp.credits)}`, big: {l: t("Month-end estimate (this month)"), v: crN(mp.credits)}} : "";
    tile(t("Kiro credits"), bk, crN(u.credits), [vs(u.credits, V.of("credits", pu && pu.credits), r1)].concat(bk.part ? pace(u.credits, bk.v, r1, crN) : t("Kiro's own count, not an estimate"), me).filter(Boolean), "credits"); }
  else if (u.tokens) { const bt = insBase("tokens"); tile(t("Tokens"), bt, tok(u.tokens), [vs(u.tokens, V.of("tokens", pu && pu.tokens), tok), `${t("output")} ${tok(u.out)} ÷ ${tok(u.tokens)} = ${Math.round(u.out * 100 / u.tokens)}%`].filter(Boolean), "tokens"); }
  // 予算に対する位置：途中の月は月末の推定、途中の週は前の週並みに収める 1 日の目安、終わった期間は前の期間との差
  const pj = projection(w), pc = pacing(w, pw), fmc = x => x === "credits" ? (cr1) : usd; // 式の中のクレジットは小数 1 桁（整数に丸めると、引き算の結果と合わなくなる）
  if (pj && (pj.cost != null || pj.credits != null)){
    } // 月末の推定は、各数字の下（「月末の予測」）と上の帯にあるので、ここに別の枠は置かない
  else if (pc){ // 費用とクレジットのそれぞれについて：残りの日 1 日あたりの目安か、すでに超えた額（式で）
    const xs = ["cost", "credits"].filter(k => pc[k]).map(k => ({k, x: pc[k], f: fmc(k)})), ok = xs.find(o => o.x.rest >= 0), fs = xs.map(({x, f}) => x.rest >= 0 ? `(${f(x.ref)} − ${f(x.so)}) ÷ ${pc.left}${t("d")} = ${f(x.per)}/${t("d")}` : `${f(x.so)} − ${f(x.ref)} = +${f(-x.rest)} ${t("over")}`);
    if (ok) tile(M ? t("Daily spend to stay at last month's total") : t("Daily spend to stay at last week's total"), false, `${ok.f(ok.x.per)}/${t("d")}`, fs, ok.k === "cost" ? "pace" : "paceCr");
    else tile(M ? t("Over last month's total") : t("Over last week's total"), false, `+${xs[0].f(-xs[0].x.rest)}`, fs, xs[0].k === "cost" ? "pace" : "paceCr"); }
  const ba = insBase("active"); if (ba) tile(t("Active time"), ba, fmtM("active", ba.cur), [vs(ba.cur, V.of("active", pw && pw.active), v => dur(v))].concat(ba.part ? pace(ba.cur, ba.v, v => dur(v)) : "").filter(Boolean), "active");
  // 出力は、元の違う 2 つを混ぜずに分ける：Git（リポジトリから読んだコミット）と、AI の編集（エージェントの履歴の編集ツール。コミットしたかによらない）
  G = "outGit";
  const g = w && w.git && w.git.commits ? w.git : null, o = (w && w.outputs) || {};
  if (g) tile(t("Git commits"), insBase("gitCommits"), `${g.commits}`, [{f: `AI ${g.ai} ÷ ${g.commits} = ${Math.round(g.ai * 100 / g.commits)}%`, p: t("{pc}% by AI", {pc: Math.round(g.ai * 100 / g.commits)})}, vsk("gitCommits", g.commits, "commits")].filter(Boolean), "commits");
  else if (o.commits) tile(t("AI commits"), false, `${o.commits}`, {n: t("Commits AI ran, from history")}, "aiCommits");
  // 見出しには主な数（コミット・行・PR）だけ。ファイル・リポジトリ・日・push は下の「出力」のカードに
  if (g && g.added + g.removed > 0){ const lines = g.added + g.removed, n = commas;
    tile(t("Lines in Git commits"), insBase("gitLines"), n(lines), [{f: `+${n(g.added)} −${n(g.removed)}; ${n(lines)} ÷ ${g.commits} = ${n(Math.round(lines / g.commits))} ${t("per commit")}`, h: lnPM(n(g.added), n(g.removed))}, vsk("gitLines", lines)].filter(Boolean), "lines"); }
  if (o.prs) tile(t("Pull requests"), insBase("prs"), `${o.prs}`, [{n: t("Created by AI")}, vsk("prs", o.prs)].filter(Boolean), "prs");
  // AI のコミットのやり直し：3 日以内に同じファイルがまた変えられた割合（3 日たったコミットだけ）
  { const r = reworkOf(ws, we), b = insBase("rework");
    if (r.n && b) tile(t("AI commits changed again within 3 days"), b, `${b.cur}%`, [{f: `${r.re} ÷ ${plural(r.n, "AI commit")} = ${b.cur}%`, p: t("{re} of {n} AI commits", {re: r.re, n: r.n})}, {n: t("Lower is usually better: fewer files needed another pass")}, vsk("rework", b.cur)].filter(Boolean)); }
  G = "outEdit";
  { const xs = DATA.filter(s => s.start >= ws && s.start < we && s.nFiles > 0), a = xs.reduce((x, s) => x + (s.editAdded || 0), 0), d = xs.reduce((x, s) => x + (s.editRemoved || 0), 0);
    if (a + d > 0) tile(t("Lines edited by AI"), insBase("aiLines"), commas(a + d), [{f: `+${commas(a)} −${commas(d)}`, h: lnPM(commas(a), commas(d))}, vsk("aiLines", a + d)].filter(Boolean)); }
  // 書いた行の定着：Claude Code のセッションで AI が足した行のうち、そのセッションの AI のコミットに入った割合
  { const k = keptOf(ws, we), b = insBase("kept");
    if (k.edit && b) tile(t("AI-added lines that reached a commit"), b, `${b.cur}%`, [{f: `${commas(k.kept)} ÷ ${commas(k.edit)} = ${b.cur}%`, p: t("{kept} of {edit} lines · {n} Claude Code sessions", {kept: commas(k.kept), edit: commas(k.edit), n: k.n})}, {n: t("{pc}% was rewritten or dropped", {pc: 100 - b.cur})}, vsk("kept", b.cur)].filter(Boolean)); }
  // 単価も、割る数の元ごとに分ける：Git のコミットあたり・AI が編集した 100 行あたり・プロンプトあたり
  // 単価は、割る範囲を名前に入れ、「分子 ÷ 分母」をいつも値の下に出す（式を開かなくても、隣の数と食い違って見えないように）。
  // 見出しには推定費用とクレジットだけ。トークンの単価・セッションあたり・プロンプトあたりは推移と「使い方」のカードに
  G = "unitGit";
  const bp = insBase("costPerCommit"), oc = w && w.outCost, nc = w && w.outputs && w.outputs.commits;
  if (bp && oc && nc) tile(t("Claude Code cost per AI commit"), bp, fmtM("costPerCommit", bp.cur), [{f: `${usd(oc)} ÷ ${nc} ${t("AI commits")} = ${usd(bp.cur)}`, p: `${usd(oc)} ÷ ${nc} ${t("AI commits")}`}, vsk("costPerCommit", bp.cur)].filter(Boolean), "costPerCommit");
  // トークンも金額と同じく（Claude Code のセッションのトークン ÷ AI のコミット）
  { const bt = insBase("tokensPerCommit"), ot = DATA.filter(s => s.source === "Claude Code" && s.start >= ws && s.start < we).reduce((a, s) => a + sesTok(s), 0);
    if (bt && ot && nc) tile(t("Claude Code tokens per AI commit"), bt, fmtM("tokensPerCommit", bt.cur), [{f: `${tok(ot)} ÷ ${nc} ${t("AI commits")} = ${tok(bt.cur)}`, p: `${tok(ot)} ÷ ${nc} ${t("AI commits")}`}, vsk("tokensPerCommit", bt.cur)].filter(Boolean)); }
  G = "unitEdit";
  for (const [k, label, fu] of [["costPerEdit", t("Estimated cost per 100 AI-edited lines"), usd], ["creditsPerEdit", t("Kiro credits per 100 AI-edited lines"), cr1], ["tokensPerEdit", t("Tokens per 100 AI-edited lines"), tok]]){
    const b = insBase(k); if (!b) continue;
    const e = editsBasis(ws, we, k.replace("PerEdit", "")), den = t("{n} AI-edited lines", {n: commas(e.lines)});
    tile(label, b, /^credits/.test(k) ? crR(b.cur) : fmtM(k, b.cur), [{f: `${fu(e.used)} ÷ ${den} × 100 = ${fmtM(k, b.cur)}`, p: `${fu(e.used)} ÷ ${den} × 100 · ${t("{n} sessions", {n: e.n})}`}, vsk(k, b.cur)].filter(Boolean)); }
  // 稼働 1 時間あたり：その使用量を記録したセッションの作業時間で割る（重なったセッションはそれぞれ数えるので、上の稼働時間より長いことがある）
  G = "unitHour";
  for (const [k, label, kind, fu] of [["costPerHour", t("Estimated cost per active hour"), "cost", usd], ["creditsPerHour", t("Kiro credits per active hour"), "credits", cr1], ["tokensPerHour", t("Tokens per active hour"), "tokens", tok]]){
    const b = insBase(k); if (!b) continue; const e = useBasis(ws, we, kind);
    tile(label, b, /^credits/.test(k) ? crR(b.cur) : fmtM(k, b.cur), [{f: `${fu(e.used)} ÷ ${dur(e.min)} = ${fmtM(k, b.cur)}`, p: `${fu(e.used)} ÷ ${t("{d} of session time", {d: dur(e.min)})}`}, vsk(k, b.cur)].filter(Boolean)); }
  // 稼働 1 日あたり：その使用量を記録したセッションの使用量 ÷ それらが動いた日の数（動かなかった日で薄めない）
  G = "unitDay";
  for (const [k, label, kind, fu] of [["costPerDay", t("Estimated cost per active day"), "cost", usd], ["creditsPerDay", t("Kiro credits per active day"), "credits", cr1], ["tokensPerDay", t("Tokens per active day"), "tokens", tok]]){
    const b = insBase(k); if (!b) continue; const e = useBasis(ws, we, kind), den = t("{n} active days", {n: e.days});
    tile(label, b, /^credits/.test(k) ? crR(b.cur) : fmtM(k, b.cur), [{f: `${fu(e.used)} ÷ ${den} = ${fmtM(k, b.cur)}`, p: `${fu(e.used)} ÷ ${den}`}, vsk(k, b.cur)].filter(Boolean)); }
  // セッションあたり・プロンプトあたり：その使用量を記録したセッションの数・プロンプトの数で割る（記録のないエージェントで薄めない）
  for (const [g, by, rows] of [["unitSes", "ses", [["costPerSes", t("Estimated cost per session"), "cost", usd], ["creditsPerSes", t("Kiro credits per session"), "credits", cr1], ["tokensPerSes", t("Tokens per session"), "tokens", tok]]], ["unitAsk", "ask", [["costPerAsk", t("Estimated cost per prompt"), "cost", usd], ["creditsPerAsk", t("Kiro credits per prompt"), "credits", cr1], ["tokensPerAsk", t("Tokens per prompt"), "tokens", tok]]]]){
    G = g;
    for (const [k, label, kind, fu] of rows){
      const b = insBase(k); if (!b) continue; const e = useBasis(ws, we, kind), den = by === "ses" ? t("{n} sessions", {n: e.n}) : plural(e.prompts, "prompt");
      tile(label, b, /^credits/.test(k) ? crR(b.cur) : fmtM(k, b.cur), [{f: `${fu(e.used)} ÷ ${den} = ${fmtM(k, b.cur)}`, p: `${fu(e.used)} ÷ ${den}`}, vsk(k, b.cur)].filter(Boolean), k === "costPerAsk" ? "costPerAsk" : undefined); } } // プロンプトあたりの費用は、押すと内訳
  // カードは折り返す横の並びに置く：広い画面では 1 行に 3 枚、狭くなると 2 枚・1 枚。余ったカードは行の残りの幅に広がる
  // 小見出しのある段：出どころ（Git と AI の編集）が違う数字を、同じ枠の中でも段と印で分ける
  const sub = ([k, icon, label, note, empty]) => !T[k].length && empty ? `<div class="iksub ${k}" role="group" aria-label="${esc(label)}"><h6 class="iksh">${ico(icon)}${esc(label)}${note ? ` <small>${esc(note)}</small>` : ""}</h6><p class="none iknone">${esc(empty)}</p></div>` : T[k].length ? `<div class="iksub ${k}"${U3.has(k) ? " data-third" : ""} role="group" aria-label="${esc(label)}"><h6 class="iksh">${ico(icon)}${esc(label)}${note ? ` <small>${esc(note)}</small>` : ""}</h6><div class="ikrow">${T[k].join("")}</div></div>` : "";
  const cards = [
    {id: "cost", label: t("Cost trend"), rows: null, k: "cost"},
    {id: "out", label: t("Outputs"), rows: [["outGit", "commit", t("Git"), t("from your repositories"), T.outEdit.length ? t("No Git commits for this selection") : ""], ["outEdit", "edit", t("AI edits"), t("from the agents' history, committed or not")]]},
    {id: "unit", label: t("Unit cost calculation"), rows: [["unitGit", "commit", t("Per AI commit"), ""], ["unitEdit", "edit", t("Per 100 AI-edited lines"), ""], ["unitHour", "clock", t("Per active hour"), ""], ["unitDay", "day", t("Per active day"), ""], ["unitSes", "agent", t("Per session"), ""], ["unitAsk", "user", t("Per prompt"), ""]]},
  ].map(c => ({...c, n: c.rows ? Math.max(...c.rows.map(r => T[r[0]].length)) : T[c.k].length})).filter(c => c.n > 0);
  // 推移のグラフの凡例は、カードごとに 1 つ（グラフごとに置くと、カードが縦に伸びる）
  const limAny = limitHits(periodBack(INS_N).ws, periodBack(0).we).length > 0; // グラフの範囲に利用上限への到達があれば、凡例に赤い縦線を足す
  const now = soFar() != null ? (M ? t("This month") : t("This week")) : (M ? t("Month shown") : t("Week shown"));
  const lgd = c => `<p class="tlg" aria-hidden="true"><span class="lg now">${esc(now)}</span>${insFrac() < 1 ? `<span class="lg now part">${esc(M ? t("Month-end estimate") : t("End-of-week estimate"))}</span><span class="lg prj">${esc(t("Projection"))}</span>` : ""}<span class="lg band">${esc(t("Middle half of your past {unit}", {unit: insNoun()}))}</span><span class="lg med">${esc(t("median"))}</span>${c.id === "out" ? "" : `<span class="lg cum">${esc(c.id === "unit" ? t("Running average") : t("Cumulative (right axis)"))}</span>`}${c.id === "cost" ? `<span class="lg pcl">${esc(M ? t("% = change from the previous month") : t("% = change from the previous week"))}</span>` : ""}${limAny && c.id === "cost" ? `<span class="lg lgl">${esc(t("Usage limit hits"))}</span>` : ""}</p>`;
  const ukAll = ["cost", "credits", "tokens"].filter(k => UK.has(k)), ukCur = ukAll.includes(store.get("unitKind", "")) ? store.get("unitKind", "") : ukAll[0];
  const ukName = k => k === "cost" ? t("Cost") : k === "credits" ? t("Credits") : t("Tokens");
  const ukSw = ukAll.length > 1 ? `<div class="segc uksw" role="group" aria-label="${esc(t("Measure"))}">${ukAll.map(k => `<button type="button" data-uk="${k}" aria-pressed="${k === ukCur}">${esc(ukName(k))}</button>`).join("")}</div>` : "";
  // 選んだ量の数字がない段には、ないことを 1 行で（Kiro はどのコミットを実行したかを記録しないので、コミットあたりのクレジットはない、など）
  ["unitGit", "unitEdit", "unitHour", "unitDay", "unitSes", "unitAsk"].forEach(g => { if (!T[g].length) return; ukAll.forEach(k => { if (!T[g].some(h => h.includes(`data-uk="${k}"`))) T[g].push(`<p class="none iknone" data-uk="${k}">${esc(k === "credits" && g === "unitGit" ? t("Kiro does not record which commits it ran") : t("Not recorded for this measure"))}</p>`); }); });
  // ページの半分より広いカードは作らない：グラフのタイルが奇数なら、最後の 3 つを 3 等分で並べる（1 つだけ半分の幅に残して、右を空けない）
  const U3 = new Set(), third = (g, ix) => ix.forEach(i => { T[g][i] = T[g][i].replace("<div ", "<div data-third "); });
  ["cost", "outGit", "outEdit"].forEach(g => { const ix = SP[g].map((z, i) => z && z.sp ? i : -1).filter(i => i >= 0); if (ix.length % 2 && ix.length >= 3) third(g, ix.slice(-3)); });
  { const us = ["unitGit", "unitEdit", "unitHour", "unitDay", "unitSes", "unitAsk"].filter(g => T[g].length); if (us.length % 2 && us.length >= 3) us.slice(-3).forEach(g => U3.add(g)); }
  const card = c => `<section class="ikg ik-${c.id}"${c.id === "unit" && ukCur ? ` data-uk="${ukCur}"` : ""} aria-label="${esc(c.label)}"><h5 class="ikgh">${esc(c.label)}${c.id === "unit" ? ukSw : ""}</h5>${lgd(c)}${c.rows ? c.rows.map(sub).join("") : `<div class="ikrow">${T[c.k].join("")}</div>`}</section>`;
  // 見直す候補：基準を超えた指標を、見出しのすぐ下に 1 行で（押すと、その候補のダイアログ）。どれも、その指標の横の印からも開ける
  const ids = [...new Set((F || []).map(f => GOTO[f.k] || f.k))]; // 説明（HELP）のない候補も落とさない（名前は insName が ID に戻す）
  const worth = `<div class="iworth" role="group" aria-label="${esc(t("Worth a look"))}"><span class="iwl">${ico("flag")}${esc(t("Worth a look"))}${ids.length ? ` · ${ids.length}` : ""}</span>${ids.length ? ids.map(id => `<button type="button" class="iwlink" data-goto="${esc(id)}" aria-haspopup="dialog">${esc(insName(id))}</button>`).join("") : `<span class="iwnone">${esc(t("Nothing crossed a threshold this period"))}</span>`}</div>`;
  const nb = insBase("active"), few = nb && nb.q1 == null ? `<p class="note insfew">${esc(t("To compare with your usual range, kiroku needs {min} past {unit} with records (have {n}).", {min: INS_MIN, unit: insUnit(INS_MIN), n: nb.n}))}</p>` : ""; // 比べられないことは、数字ごとでなく 1 度だけ言う
  return `${worth}${few}<div class="insh">${cards.map(card).join("")}</div><p class="ifxs insdef">${fx(insDef())}</p>`;
}
/* 推移のグラフ：前の INS_N 期間と表示中の期間の棒。いつもの範囲（四分位）を帯、中央値を点線で重ねる。
   途中の期間は、今の値の棒と、このペースで終えたときの値の枠。範囲から大きく外れた期間には印 */
// 目盛りの数は短く（クレジットは数だけ。単位は見出しの文にある。左の余白に収める）
const tickM = (k, v) => /^credits/.test(k) ? crN(v) : fmtM(k, v);
function insChart(k){
  const all = []; for (let i = INS_N; i >= 0; i--){ const p = periodBack(i); all.push({p, v: p.ws < historyStart() && i ? null : metricAt(k, p)}); }
  const hs0 = historyStart(), f0 = Math.min(Math.max(0, all.findIndex(x => x.p.we > hs0)), all.length - 4), ps = all.slice(f0); // どのグラフも、履歴の始まる期間から（同じ枠のグラフの横軸がそろう） // 記録の始まる前の期間は描かない
  const b = insBase(k), cur = ps[ps.length - 1], proj = b && b.part ? b.v : null;
  const vals = ps.map(x => x.v).filter(v => v != null); if (!vals.length) return `<p class="note">${esc(t("No records"))}</p>`;
  // 狭い画面は小さな座標で描く（縮めて文字が読めなくならないように）
  // カードの幅（1 行に 2〜3 枚）に合う小さな座標で描く（大きな座標を縮めると、文字と点が読めないほど小さくなる）
  const narrow = true, W = 360, Hh = 180, L = 52, R = 8, T = 12, B = 26, mx = Math.max(...vals, proj || 0, b && b.q3 || 0);
  const top = MET[k] && MET[k].u === "%" && mx <= 100 ? 100 : insNice(mx * 1.05); // 割合は 100% まで（200% の軸では線が平らに見える）
  const bw = (W - L - R) / ps.length, x = i => L + i * bw, y = v => T + (Hh - T - B) * (1 - v / top);
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 8}" y="${(y(v) + 4).toFixed(1)}">${esc(tickM(k, v))}</text></g>`).join("");
  const band = b && b.q1 != null ? `<rect class="iband" x="${L}" y="${y(b.q3).toFixed(1)}" width="${W - L - R}" height="${Math.max(1, y(b.q1) - y(b.q3)).toFixed(1)}"/><line class="imed" x1="${L}" x2="${W - R}" y1="${y(b.med).toFixed(1)}" y2="${y(b.med).toFixed(1)}"/>` : "";
  const lab = p => { const [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? (JA ? `${mm}月` : MON[mm - 1]) : dMD(new Date(yy, mm - 1, dd)); };
  const out = v => b && b.q1 != null && (v > b.q3 + 1.5 * (b.q3 - b.q1) || v < b.q1 - 1.5 * (b.q3 - b.q1));
  const n = ps.length, every = Math.ceil(n / (narrow ? 4 : 7)); // 期間の名前は間引く（右端の表示中の期間は必ず）
  // 合計の指標（期間の長さに比例して増える量）は棒、率の指標（コミットあたり・行あたり・割合・待ち時間）は線と点で描く。率は足し上げる量ではないので、棒の高さで面積を比べさせない
  const rate = !INS_ADD.has(k);
  const line = rate ? (() => { const segs = []; let cur = []; ps.forEach(({v}, i) => { if (v == null){ if (cur.length) segs.push(cur); cur = []; } else cur.push(`${(x(i) + bw / 2).toFixed(1)},${y(v).toFixed(1)}`); }); if (cur.length) segs.push(cur);
    return segs.map(sg => sg.length > 1 ? `<polyline class="iline" points="${sg.join(" ")}"/>` : "").join(""); })() : "";
  const bars = ps.map(({p, v}, i) => { const last = i === n - 1, cx = x(i) + bw / 2;
    return `<g>${v == null ? "" : rate ? `<circle class="ipt${last ? " cur" : ""}" cx="${cx.toFixed(1)}" cy="${y(v).toFixed(1)}" r="${last ? 5 : 3.5}"/>` : `<rect class="ibar${last ? " cur" : ""}" x="${(x(i) + bw * .18).toFixed(1)}" y="${y(v).toFixed(1)}" width="${(bw * .64).toFixed(1)}" height="${Math.max(0, Hh - B - y(v)).toFixed(1)}" rx="3"/>`}`
      + (last && proj != null ? `<rect class="iproj" x="${(x(i) + bw * .18).toFixed(1)}" y="${y(proj).toFixed(1)}" width="${(bw * .64).toFixed(1)}" height="${Math.max(0, Hh - B - y(proj)).toFixed(1)}" rx="3"/>` : "")
      + (v != null && !last && out(v) ? `<circle class="iout" cx="${cx.toFixed(1)}" cy="${(y(v) - 7).toFixed(1)}" r="3"/>` : "")
      + ((n - 1 - i) % every === 0 ? `<text class="ixl${last ? " cur" : ""}" x="${cx.toFixed(1)}" y="${Hh - 8}">${esc(lab(p))}</text>` : "")
      + `<rect class="ihit" x="${x(i).toFixed(1)}" y="0" width="${bw.toFixed(1)}" height="${Hh}"${tipAttr(p.label, v == null ? t("No records") : fmtM(k, v), last && proj != null ? t("At this pace: {v}", {v: fmtM(k, proj)}) : "")}/></g>`; }).join("");
  const list = ps.filter(x => x.v != null).map(x => `${x.p.label} ${fmtM(k, x.v)}`).join(LSEP());
  let sy = b ? t("{x}; {need}", {x: (b.part ? t("On pace for") + " " : "") + fmtM(k, b.v), need: insExpr(b)}) : ""; if (b && b.q1 != null){ const all = [...b.xs, b.v], r = 1 + b.xs.filter(z => z > b.v).length, d = b.v - b.med, pc = b.med ? Math.round(d * 100 / b.med) : null;
    const rk = r === 1 ? t("the highest of {n} {unit}", {n: all.length, unit: insUnit(all.length)}) : r === all.length ? t("the lowest of {n} {unit}", {n: all.length, unit: insUnit(all.length)}) : (() => { const lo = r > all.length / 2, q = lo ? all.length + 1 - r : r, ord = JA ? q : `${q}${["th", "st", "nd", "rd"][q % 100 > 10 && q % 100 < 14 ? 0 : q % 10 < 4 ? q % 10 : 0]}`; // 下半分は「下から何番目」で（「11 週で 10 番目に高い」より読みやすい）
      return lo ? t("{r} lowest of {n} {unit}", {r: ord, n: all.length, unit: insUnit(all.length)}) : t("{r} highest of {n} {unit}", {r: ord, n: all.length, unit: insUnit(all.length)}); })(); // 1 位・最下位は言葉で
    sy = t("{x}: {rank}; {d} from the median {m}{pc}", {rank: rk, x: (b.part ? t("At this pace") + " " : "") + fmtM(k, b.v), n: all.length, unit: insUnit(all.length), d: (d >= 0 ? "+" : "−") + fmtM(k, Math.abs(d)), m: fmtM(k, b.med), pc: pc == null ? "" : ` (${pc >= 0 ? "+" : "−"}${Math.abs(pc)}%)`}); }
  const th = !b || b.q1 == null ? "" : b.pos === "typical" ? t("Within the middle half of your past {unit}", {unit: insNoun()}) : b.better === false ? t("Outside the middle half, on the worse side; see Worth a look at the top") : b.better ? t("Outside the middle half, on the better side") : b.pos === "high" ? t("In your highest quarter of past {unit}", {unit: insNoun()}) : t("In your lowest quarter of past {unit}", {unit: insNoun()});
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(t("{metric} over the last {n} {unit}", {metric: insName(k), n, unit: insUnit(n)}))}">${ticks}${band}${line}${bars}</svg>
    <figcaption>${b && b.q1 != null ? `<span class="lg iband">${esc(t("Middle half of the last {n} {unit}", {n: b.n, unit: insUnit(b.n)}))}</span><span class="lg imed">${fx(t("median"), "", t("median"))}</span>` : ""}${proj != null ? `<span class="lg iproj">${fx(`${t("pace")} = ${t("so far")} × ${t("period")} ÷ ${t("elapsed")}`, "", t("At this pace"))}</span>` : ""}${b && b.q1 != null ? `<span class="lg iout">${fx(`< ${LO()} − 1.5×${SP()} ${t("or")} > ${HI()} + 1.5×${SP()}`, "", t("Far above or below the other periods"))}</span>` : ""}</figcaption>
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
  const avail = INS_KEYS.filter(x => insBase(x) && !(noCost && /^cost/.test(x))), k = avail.includes(st.insK) ? st.insK : avail[0] || "active";
  const tabs = avail.map(x => `<button type="button" data-ins="${x}" aria-pressed="${x === k}" title="${esc(insName(x))}">${esc(INS_SHORT()[x])}</button>`).join("");
  const card = (title, body, cls = "") => body ? `<div class="inscol${cls}"><h4>${esc(title)}</h4>${body}</div>` : "";
  // 選択リストは、指標の種類ごとにまとめ、見出しの数字と同じ正式な名前で並べる
  const grpOf = x => INS_VOL.has(x) ? t("Totals") : /PerCommit$/.test(x) ? t("Per AI commit") : /PerEdit$/.test(x) ? t("Per 100 AI-edited lines") : /PerHour$/.test(x) ? t("Per active hour") : /PerDay$/.test(x) ? t("Per active day") : /PerSes$/.test(x) ? t("Per session") : /PerAsk$/.test(x) ? t("Per prompt") : t("Habits");
  const groups = [...new Set(avail.map(grpOf))];
  const sel = `<select class="insksel" aria-label="${esc(t("Metric"))}">${groups.map(gn => `<optgroup label="${esc(gn)}">${avail.filter(x => grpOf(x) === gn).map(x => `<option value="${x}"${x === k ? " selected" : ""}>${esc(insName(x))}</option>`).join("")}</optgroup>`).join("")}</select>`; // 狭い画面では、切り替えの代わりに選択リスト（横に隠さない）
  // カードは「コスト」（時間・クレジット・金額・トークン）・「出力」（コミット・変えた行）・「使い方」の 3 つに分けて並べる
  // よく見るカード（流れ・ばらつき・アウトプット・作業の種類・時間帯）を上に。ほかは「その他」として、たたんだ箱の中に（開けば見られる）
  const col = b => b ? `<div class="inscol">${b}</div>` : "";
  const cards = [{sec: t("Cost details")}, {h: col(insSankey())}, {h: col(insBox())},
    {sec: t("Output and unit cost")}, {h: card(t("Outputs"), insOutputs(), " iout wide")},
    {sec: t("How you used AI")}, {h: card(t("When you worked"), insHeat())},
    {sec: t("Other charts"), more: true},
    {h: `<div class="inscol"><div class="inshead"><h4>${esc(t("Trend"))}</h4><div class="segc insk trendk" role="group" aria-label="${esc(t("Metric"))}">${tabs}</div>${sel}</div>${insChart(k)}</div>`},
    {h: col(insCum())}, {h: col(insWhere())}, {h: col(insWater())}, {h: card(t("Tokens by type"), insTokens())},
    {h: card((HISTK(DATA.filter(s => inP(s, period().ws, period().we))) || {}).title || "", insHist())},
    {h: card(t("By agent"), insAgents(), " agw")}, {h: card(t("Most edited files"), (insEditsBody() || {}).files)},
    {h: (b => b ? `<div class="inscol isccol">${b}</div>` : "")(insScatter())},
    {h: card(t("How you worked"), insWork(), " iwork")}, {h: card(t("Usage"), insUsage())}, {h: col(insGantt())},
  ].filter(c => c.h || c.sec);
  // 見出しごとに、カードを折り返す横の並び（.insrow）に入れる。広い画面では 1 行に 3 枚、狭くなると 2 枚・1 枚。余ったカードは行の残りの幅に広がる
  const secs = []; cards.forEach(c => c.sec ? secs.push({sec: c.sec, more: c.more, hs: []}) : secs.length && secs[secs.length - 1].hs.push(c.h));
  const body = secs.filter(x => x.hs.length).map(x => x.more ? `<details class="insmore"${store.get("insMore", false) ? " open" : ""}><summary class="inssec">${esc(x.sec)} <small>${esc(t("{n} cards", {n: x.hs.length}))}</small></summary><div class="insrow">${x.hs.join("")}</div></details>` : `<h3 class="inssec">${esc(x.sec)}</h3><div class="insrow">${x.hs.join("")}</div>`).join("");
  return `<section class="insight" aria-labelledby="ins-h"><h3 id="ins-h" class="sr">${esc(t("Overview"))}</h3>${insHeadlines(F)}
    <div class="insg">${body}</div></section>`;
}
// fitTrend は、トレンドの指標の切り替えが 1 行に収まらないとき（指標が多い・カードが狭い・言語で幅が違う）、選択リストに替える。
// 折り返した切り替えは角の丸みが崩れて読みにくいので、実際に並べて測る（見えていないとき・狭い画面で隠しているときは測らない）
function fitTrend(){ const g = $("#review .insk.trendk"); if (!g) return; const h = g.closest(".inshead"); h.classList.remove("foldk");
  const bs = g.querySelectorAll("button"); if (!g.offsetParent || bs.length < 2) return;
  if (bs[bs.length - 1].offsetTop !== bs[0].offsetTop || g.getBoundingClientRect().right > h.getBoundingClientRect().right + 1) h.classList.add("foldk"); }
addEventListener("resize", () => fitTrend());
// insPack は、サマリーのカードを列に詰めて並べる（石積み）。行ごとに高さをそろえると、低いカードの中が大きく空くので、
// 各カードを自分の高さのまま、空いている列のいちばん上に置く。カードの高さは中身や幅で変わるので、変わるたびに測り直す
let insRO = null;
function insPack(R){
  const rows = [...R.querySelectorAll(".insrow")], gap = 16;
  const fit = () => rows.forEach(r => { if (!r.isConnected) return;
    [...r.children].forEach(c => c.style.setProperty("--rs", String(Math.ceil(c.getBoundingClientRect().height) + gap))); });
  fit();
  if (insRO) insRO.disconnect();
  if (window.ResizeObserver){ insRO = new ResizeObserver(() => requestAnimationFrame(fit)); rows.forEach(r => { insRO.observe(r); [...r.children].forEach(c => insRO.observe(c)); }); }
}
function bindInsight(R){
  R.querySelectorAll(".uksw [data-uk]").forEach(b => b.onclick = () => { const sec = b.closest(".ik-unit"); sec.dataset.uk = b.dataset.uk; store.set("unitKind", b.dataset.uk);
    sec.querySelectorAll(".uksw [data-uk]").forEach(x => x.setAttribute("aria-pressed", String(x === b))); });
  R.querySelectorAll(".iwlink").forEach(b => b.onclick = () => insOpenWorth(b.dataset.goto, b)); // 見直す候補の 1 行
  R.querySelectorAll("details.insmore").forEach(d => d.ontoggle = () => store.set("insMore", d.open)); // 「その他のグラフ」を開いたかを覚える
  // 式は、指標名の横の「ƒx」から、その数字（タイル・枠・グラフのカード）ごとに開く。ふだんは平易な 1 行だけを見せる
  R.querySelectorAll(".insight .itile, .insight .stat, .insight .inscol").forEach(el => {
    const own = [...el.querySelectorAll(".fxo, .ichip .cx")].some(f => f.closest(".itile, .stat, .inscol") === el); if (!own) return;
    const lab = el.matches(".itile") ? el.querySelector(".il") : el.matches(".stat") ? el.querySelector(".k > span") : el.querySelector(":scope > h4, :scope > .inshead > h4, :scope h4"); if (!lab) return;
    const lc = lab.cloneNode(true); lc.querySelectorAll(".hb, .fxb, .fmark").forEach(x => x.remove()); // 読み上げの名前に「?」や印の文字を混ぜない
    const name = (lc.textContent || "").trim(), b = document.createElement("button");
    b.type = "button"; b.className = "fxb"; b.textContent = "ƒx"; b.setAttribute("aria-pressed", "false");
    b.title = t("Show the formula"); b.setAttribute("aria-label", t("Show the formula: {name}", {name}));
    b.onclick = e => { e.stopPropagation(); const on = el.classList.toggle("fxon"); b.setAttribute("aria-pressed", String(on)); b.title = on ? t("Hide the formula") : t("Show the formula"); };
    lab.append(b); }); // 名前の要素の中に置く（カードは縦の flex なので、外に置くと横いっぱいの帯になる）
  requestAnimationFrame(fitTrend);
  insPack(R);
  R.querySelectorAll("[data-ins]").forEach(b => b.onclick = () => { st.insK = b.dataset.ins; store.set("insK", st.insK); summary(); const nb = $("#review").querySelector(`[data-ins="${CSS.escape(b.dataset.ins)}"]`); if (nb) nb.focus(); });
  for (const [attr, key] of [["skk", "insSk"], ["bxk", "insBk"]]) R.querySelectorAll(`[data-${attr}]`).forEach(b => b.onclick = () => { st[key] = b.dataset[attr]; store.set(key, st[key]); summary(); const nb = $("#review").querySelector(`[data-${attr}="${CSS.escape(b.dataset[attr])}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-wfk]").forEach(b => b.onclick = () => { st.insWf = b.dataset.wfk; store.set("insWf", st.insWf); summary(); const nb = $("#review").querySelector(`[data-wfk="${CSS.escape(b.dataset.wfk)}"]`); if (nb) nb.focus(); });
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
  const narrow = innerWidth < 600, W = narrow ? 360 : innerWidth < 1200 ? 720 : 1200, Hh = narrow ? 214 : innerWidth < 1200 ? 234 : 290, /* 1 行を占める広い画面では、横に長い座標で描く（文字と点が大きくなりすぎて、縦に空かないように） */ L = narrow ? 40 : 48, R = 12, T = 12, B = 44; // 下に目盛りと軸の名前（点に重ならないように、目盛りの下）
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
  const th = b && b.q1 != null && b.pos === "high" ? t("Off-hours share {off}% is higher than in most of your past {unit} (usually up to {q3}%)", {off, q3: Math.round(b.q3), unit: insNoun()}) : "";
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
  const narrow = true, W = 360, Hh = 180, L = 52, R = 10, T = 10, B = 24; // カードの幅に合う小さな座標（推移と同じ）
  const x = i => L + (W - L - R) * (n === 1 ? 0 : i / (n - 1)), y = v => T + (Hh - T - B) * (1 - v / top), line = a => a.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 6}" y="${(y(v) + 4).toFixed(1)}">${esc(k === "credits" ? crN(v) : fm(v))}</text></g>`).join("");
  const a = new Date(period().ws * 1000), xl = [0, n - 1].map(i => `<text class="ixl ${i ? "e" : "s"}" x="${x(i).toFixed(1)}" y="${Hh - 6}">${esc(dMD(addDays(a, i)))}</text>`).join(""); // 両端の日付は、内側に寄せる（はみ出さないように）
  if (C[k].tot){ const a = C[k].tot(w.usage || {}), b = pw ? C[k].tot(pw.usage || {}) : null; // 合計は帯と同じ値（日ごとの丸めを足すと、帯と 1 セントずれる）
    if (a != null && cur.length) cur[cur.length - 1] = a; if (b != null && prev.length) prev[prev.length - 1] = b; }
  const last = cur.length - 1, ptot = prev.length ? prev[prev.length - 1] : null, pref = fr >= 1 ? ptot : prev.length > last ? prev[last] : null, diff = pref == null ? null : cur[last] - pref; // 終わった期間は前の期間の合計と、途中の期間は同じ日までと比べる
  const sy = diff == null ? "" : (fr >= 1 ? t("Total {diff} ({pc}) vs the previous period's total", {diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: pref ? `${diff >= 0 ? "+" : "−"}${Math.abs(Math.round(diff * 100 / pref))}%` : "—"})
      : t("Day {d} of {n}: {diff} ({pc}) vs the previous period", {d: last + 1, n: w.days.length, diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: pref ? `${diff >= 0 ? "+" : "−"}${Math.abs(Math.round(diff * 100 / pref))}%` : "—"}))
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
const HISTK = ss => ss.some(s => s.cost > 0) ? {k: "cost", v: s => s.cost, f: usd, n: t("cost"), title: t("Cost per session, by length")}
  : ss.some(s => s.credits > 0) ? {k: "credits", v: s => s.credits, f: cr, n: t("credits"), title: t("Credits per session, by length")}
  : ss.some(s => sesTok(s) > 0) ? {k: "tokens", v: s => sesTok(s), f: tok, n: t("tokens"), title: t("Tokens per session, by length")} : null;
function insHist(){
  // 長さごとの、1 セッションあたりの量（推定費用、なければクレジット、それもなければトークン）。合計だと、セッションの多い区間が大きく見えるだけなので平均にする。
  // 区間の合計とセッションの数は、ツールチップと読み取りの文に残す。強調するのは 2 セッション以上の区間（1 つだけでは傾向と言えない）。量のないときは出さない
  const {ws, we} = period(), all = DATA.filter(s => inP(s, ws, we)), K = HISTK(all); if (!K) return "";
  const ss = all.map(s => ({m: sesMin(s), c: K.v(s) || 0})).filter(x => x.m > 0); if (ss.length < 3) return "";
  const edges = [0, 5, 15, 30, 60, 120, Infinity], labs = ["<5m", "5–15m", "15–30m", "30–60m", "1–2h", "2h+"], useC = K.k === "cost";
  const bins = labs.map((_, i) => ss.filter(x => x.m >= edges[i] && x.m < edges[i + 1])), val = b => b.reduce((a, x) => a + x.c, 0);
  const sums = bins.map(val), tot = sums.reduce((a, v) => a + v, 0), vs = bins.map((b, i) => b.length ? sums[i] / b.length : 0), many = vs.map((v, i) => bins[i].length >= 2 ? v : -1), mi = Math.max(...many) >= 0 ? many.indexOf(Math.max(...many)) : vs.indexOf(Math.max(...vs)), med = insQuant(ss.map(x => x.m), .5);
  const max = Math.max(...vs) || 1, W = 360, Hh = 196, B = 34, T = 16, bw = W / labs.length, fv = K.f; // カードの幅に合う小さな座標。下に長さとセッションの数
  const bars = vs.map((v, i) => { const h = (Hh - B - T) * v / max, xx = i * bw;
    return `<rect class="ihb${i === mi ? " med" : ""}" x="${(xx + bw * .14).toFixed(1)}" y="${(Hh - B - h).toFixed(1)}" width="${(bw * .72).toFixed(1)}" height="${h.toFixed(1)}" rx="3"${tipAttr(labs[i], t("{v} per session", {v: fv(v)}), `${plural(bins[i].length, "session")} · ${t("total")} ${fv(sums[i])}`)}/>`
      + (v ? `<text class="ihn" x="${(xx + bw / 2).toFixed(1)}" y="${(Hh - B - h - 4).toFixed(1)}">${esc(fv(v))}</text>` : "") + `<text class="ixl" x="${(xx + bw / 2).toFixed(1)}" y="${Hh - 18}">${labs[i]}</text><text class="ixl ixn" x="${(xx + bw / 2).toFixed(1)}" y="${Hh - 4}">${esc(t("{n} sessions", {n: bins[i].length}))}</text>`; }).join(""); // 件数は棒のツールチップに（上の 1 文にも）
  const sy = t("Sessions of {bin} took the most per session: {v} on average ({sessions}, {pc}% of {metric} in total)", {bin: labs[mi], v: fv(vs[mi]), sessions: plural(bins[mi].length, "session"), pc: Math.round(sums[mi] * 100 / tot), metric: K.n});
  const LC = useC ? longCtxOf(ws, we) : [];
  const th = mi >= 4 && LC.length ? t("{n} of the long sessions grew their input per response to up to {x}× the start: split them at a stopping point", {n: LC.length, x: Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0])))}) : "";
  return `${say(sy, th)}<figure class="insfig ihist"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(labs.map((l, i) => `${l} ${t("{v} per session", {v: fv(vs[i])})} (${plural(bins[i].length, "session")})`).join(", "))}">${bars}</svg>
    <p class="ifxs">${fx(`${t("median")}(${t("active time of {n} sessions", {n: ss.length})}) = ${dur(med)}`)}${fx(`${t("per session")} = ${K.n} ${t("of the bin")} ÷ ${t("sessions of the bin")}`)}</p></figure>`;
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
  if (new Set(top.map(x => x.c)).size === 1) top.forEach((x, i) => { if (x.c !== "var(--other)") x.c = `var(--c${i % 8})`; }); // 色分けがプロジェクトでないときも、ドーナツと横棒で項目を見分けられるように
  let acc = 0, k80 = 0; for (const x of xs){ acc += x[k]; k80++; if (acc >= tot * .8) break; }
  const sy = t("{name}: {pc}% of {metric} ({v})", {name: xs[0].name, pc: Math.round(xs[0][k] * 100 / tot), metric: name, v: fv(xs[0][k])}) + (k80 < xs.length ? t("; 80% is in {k} of {n}", {k: k80, n: xs.length}) : "");
  const tabs = [["project", t("Project")], ["source", t("Agent")], ["model", t("Model")]].map(([v, l]) => `<button type="button" data-where="${v}" aria-pressed="${v === by}">${esc(l)}</button>`).join("");
  const rows = top.map(x => `<div class="iwr"><span class="iwn" title="${esc(x.name)}"><i style="background:${x.c}"></i>${esc(x.name)}</span><span class="iwb"><span style="width:${(x[k] * 100 / top[0][k]).toFixed(1)}%;background:${x.c}"></span></span><span class="iwv">${esc(fv(x[k]))}</span><b>${Math.round(x[k] * 100 / tot)}%</b></div>`).join("");
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-wk="${x}" aria-pressed="${x === k}">${esc({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), minutes: t("Time")}[x])}</button>`).join("")}</div>` : "";
  return `<div class="inshead"><h4>${esc(t("Where it went"))}</h4><div class="segc insk" role="group" aria-label="${esc(t("Group by"))}">${tabs}</div>${mtabs}</div>${say(sy)}
    <figure class="insfig iwdn">${insDonut(top.map(x => ({name: x.name, v: x[k], c: x.c})), tot, fv, name)}<div class="iwrows" role="table" aria-label="${esc(name)}">${rows}</div><p class="ifxs">${fx(`% = ${name} ÷ ${fv(tot)}`)}</p></figure>`;
}
/* insWater は、前の期間からの増減をプロジェクトごとに積み上げるウォーターフォール：前の期間の合計 → 増えた（減った）プロジェクト → 今の期間の合計。
   途中の期間は、前の期間の同じ日数まで（上の帯の前週比と同じ比べ方）。量は推定費用・クレジット・トークン・時間から選ぶ。セッションは始まった時刻で期間に数える */
function insWater(){
  const {ws, we} = period(), pb = periodBack(1), sf = soFar(), days = sf == null ? null : sf;
  const ce = days == null ? we : Math.min(we, ws + days * 86400), pe = days == null ? pb.we : Math.min(pb.we, pb.ws + days * 86400);
  if (historyStart() > pb.ws) return ""; // 前の期間の途中から記録が始まるときは比べない（上の帯と同じ）
  const val = {cost: s => sesTok(s) ? s.cost || 0 : 0, credits: s => s.credits || 0, tokens: s => sesTok(s), minutes: s => minIn(s, s.start, s.end)};
  const sum = (a, b, f) => { const m = new Map(); DATA.forEach(s => { if (s.start >= a && s.start < b){ const v = f(s); if (v) m.set(s.project, (m.get(s.project) || 0) + v); } }); return m; };
  const ks = ["cost", "credits", "tokens", "minutes"].filter(k => [...sum(ws, ce, val[k]).values(), ...sum(pb.ws, pe, val[k]).values()].some(v => v > 0)); if (!ks.length) return "";
  const k = ks.includes(st.insWf) ? st.insWf : ks[0], fv = {cost: usd, credits: cr, tokens: tok, minutes: v => dur(v)}[k];
  const cur = sum(ws, ce, val[k]), prev = sum(pb.ws, pe, val[k]), tc = [...cur.values()].reduce((a, v) => a + v, 0), tp = [...prev.values()].reduce((a, v) => a + v, 0);
  if (!tc && !tp) return "";
  const ds = [...new Set([...cur.keys(), ...prev.keys()])].map(p => ({p, d: (cur.get(p) || 0) - (prev.get(p) || 0)})).filter(x => Math.abs(x.d) > (k === "minutes" ? 0.5 : 1e-6)).sort((a, b) => Math.abs(b.d) - Math.abs(a.d));
  const top = ds.slice(0, 6), rest = ds.slice(6); if (rest.length) top.push({p: t("Other ({n})", {n: rest.length}), d: rest.reduce((a, x) => a + x.d, 0), other: true});
  const lab = days == null ? (st.mode === "month" ? t("Last month") : t("Last week")) : (st.mode === "month" ? t("Last month, same days") : t("Last week, same days")), now = st.mode === "month" ? (sf == null ? t("Month shown") : t("This month")) : (sf == null ? t("Week shown") : t("This week"));
  const bars = [{l: lab, a: 0, b: tp, tot: true}]; let run = tp; top.forEach(x => { bars.push({l: x.p, a: run, b: run + x.d, d: x.d}); run += x.d; }); bars.push({l: now, a: 0, b: tc, tot: true, cur: true});
  const W = 460, H = 240, L = 52, R = 8, T = 22, B = 46, mx = Math.max(...bars.map(x => Math.max(x.a, x.b))) * 1.08 || 1, Y = insTicks(k === "minutes" ? "active" : k, mx, 5), top_ = Y.top, y = v => T + (H - T - B) * (1 - v / top_);
  const n = bars.length, bw = (W - L - R) / n, short = x => x.length > 9 ? x.slice(0, 8) + "…" : x;
  const ticks = Array.from({length: Y.n + 1}, (_, i) => { const v = Y.step * i, yy = y(v).toFixed(1); return `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${yy}" y2="${yy}"/><text x="${L - 8}" y="${(+yy + 4).toFixed(1)}">${esc(k === "minutes" ? dur(v) : tickM(k === "credits" ? "credits" : k === "tokens" ? "tokens" : "cost", v))}</text></g>`; }).join("");
  const body = bars.map((x, i) => { const x0 = L + i * bw + bw * .18, w0 = bw * .64, y1 = y(Math.max(x.a, x.b)), h = Math.max(3, Math.abs(y(x.a) - y(x.b))), cls = x.tot ? (x.cur ? "wft cur" : "wft") : x.d >= 0 ? "wfu" : "wfd";
    const txt = x.tot ? fv(x.b) : `${x.d >= 0 ? "+" : "−"}${fv(Math.abs(x.d))}`, nxt = i < n - 1 ? `<line class="wfc" x1="${(x0 + w0).toFixed(1)}" x2="${(L + (i + 1) * bw + bw * .18).toFixed(1)}" y1="${y(x.b).toFixed(1)}" y2="${y(x.b).toFixed(1)}"/>` : "";
    return `<g><rect class="${cls}" x="${x0.toFixed(1)}" y="${y1.toFixed(1)}" width="${w0.toFixed(1)}" height="${h.toFixed(1)}" rx="2"/>${nxt}<text class="wfv" x="${(x0 + w0 / 2).toFixed(1)}" y="${(y1 - 5).toFixed(1)}">${esc(txt)}</text><text class="ixl${x.cur ? " cur" : ""}" x="${(x0 + w0 / 2).toFixed(1)}" y="${H - B + 16}">${esc(short(x.l))}</text><rect class="ihit" x="${(L + i * bw).toFixed(1)}" y="0" width="${bw.toFixed(1)}" height="${H - B}"${tipAttr(x.l, txt)}/></g>`; }).join("");
  const d = tc - tp, big = ds[0], vsl = days == null ? (st.mode === "month" ? t("vs last month") : t("vs last week")) : (st.mode === "month" ? t("vs the same days last month") : t("vs the same days last week")), sy = !tp ? "" : t("{d} {vs} ({pc}); the biggest change is {p} ({pd})", {d: `${d >= 0 ? "+" : "−"}${fv(Math.abs(d))}`, vs: vsl, pc: `${d >= 0 ? "+" : "−"}${Math.round(Math.abs(d) * 100 / tp)}%`, p: big ? big.p : "—", pd: big ? `${big.d >= 0 ? "+" : "−"}${fv(Math.abs(big.d))}` : ""});
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-wfk="${x}" aria-pressed="${x === k}">${esc({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), minutes: t("Session time")}[x])}</button>`).join("")}</div>` : "";
  return `<div class="inshead"><h4>${esc(t("What changed from the last period"))}</h4>${mtabs}</div>${say(sy)}<figure class="insfig iwtf"><svg viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(t("What changed from the last period"))}">${ticks}${body}</svg><p class="ifxs">${fx(t("Each bar between the totals is one project's change; projects are grouped by where their sessions started"), "", t("By project · light: up · dark: down"))}</p></figure>`;
}
/* insDonut は、割合のドーナツ（parts: {name, v, c}。c は色の CSS 値で、こちらで決めたものだけ）。真ん中に合計 */
function insDonut(parts, total, fv, label){
  if (!total || parts.length < 2) return "";
  const R = 64, r = 40, C = 70; let a0 = -Math.PI / 2;
  const arcs = parts.map(x => { const a1 = a0 + 2 * Math.PI * Math.min(.9999, x.v / total), big = a1 - a0 > Math.PI ? 1 : 0, p = (rr, a) => `${(C + rr * Math.cos(a)).toFixed(2)} ${(C + rr * Math.sin(a)).toFixed(2)}`;
    const d = `M${p(R, a0)} A${R} ${R} 0 ${big} 1 ${p(R, a1)} L${p(r, a1)} A${r} ${r} 0 ${big} 0 ${p(r, a0)} Z`; a0 = a1;
    return `<path d="${d}" fill="${x.c}"${tipAttr(x.name, `${fv(x.v)} · ${Math.round(x.v * 100 / total)}%`)}/>`; }).join("");
  return `<svg class="idn" viewBox="0 0 140 140" role="img" aria-label="${esc(label)}">${arcs}<text class="idnv" x="70" y="68">${esc(fv(total))}</text><text class="idnl" x="70" y="84">${esc(t("total"))}</text></svg>`;
}
/* insSankey は、量（費用・クレジット・トークン・時間）がエージェント → プロジェクト → 成果（コミットか PR に届いたか）へどう流れたか。
   プロジェクトは多い順に 6 つまで、残りは「その他」。帯の太さは量に比例する */
function insSankey(){
  const {ws, we} = period(), ss = DATA.filter(s => s.start >= ws && s.start < we);
  const val = {cost: s => sesTok(s) ? s.cost || 0 : 0, credits: s => s.credits || 0, tokens: s => sesTok(s), minutes: s => minIn(s, s.start, s.end)};
  const ks = ["cost", "credits", "tokens", "minutes"].filter(k => ss.some(s => val[k](s) > 0)); if (!ks.length) return "";
  const k = ks.includes(st.insSk) ? st.insSk : ks[0], fv = {cost: usd, credits: cr, tokens: tok, minutes: v => dur(v)}[k];
  const xs = ss.map(s => ({s, v: val[k](s)})).filter(x => x.v > 0); if (xs.length < 2) return "";
  const pt = new Map(); xs.forEach(x => pt.set(x.s.project, (pt.get(x.s.project) || 0) + x.v));
  const topP = [...pt].sort((a, b) => b[1] - a[1]).slice(0, 6).map(x => x[0]), OTH = t("Other"), pj = p => topP.includes(p) ? p : OTH;
  // 成果を記録しないエージェント（Claude Code 以外）のセッションは「記録なし」に（コミットがなかったとは言えない）
  const YES = t("Committed"), NO = t("No commit"), UNK = t("Not recorded"), out = s => commitsOf(s).length || (s.prs || []).length ? YES : records(s.source, "outputs") ? NO : UNK;
  const cols = [[], [], []], vals = [new Map(), new Map(), new Map()], links = new Map();
  const add = (m, key, v) => m.set(key, (m.get(key) || 0) + v);
  xs.forEach(({s, v}) => { const a = s.source, b = pj(s.project), c = out(s); add(vals[0], a, v); add(vals[1], b, v); add(vals[2], c, v); add(links, `0\u0000${a}\u0000${b}`, v); add(links, `1\u0000${b}\u0000${c}`, v); });
  const ord = {[YES]: 0, [NO]: 1, [UNK]: 2}; vals.forEach((m, i) => cols[i] = [...m].sort((a, b) => (i === 2 ? ord[a[0]] - ord[b[0]] : b[1] - a[1])));
  const tot = xs.reduce((a, x) => a + x.v, 0), W = 460, H = 300, T = 14, B = 10, nw = 10, gap = 10, X = [96, 222, 340];
  const pos = cols.map(c => { const g = (c.length - 1) * gap, sc = (H - T - B - g) / tot; let y = T; const m = new Map(); c.forEach(([key, v]) => { m.set(key, {y, h: Math.max(2, v * sc), o: 0, oi: 0}); y += v * sc + gap; }); return {m, sc}; });
  const color = (i, key) => i === 0 ? agColor(key) : i === 1 ? (key === OTH ? "var(--other)" : `var(--c${(topP.indexOf(key) + 3) % 8})`) : key === YES ? "var(--go)" : key === NO ? "var(--ink-3)" : "var(--other)";
  const paths = [...links].sort((a, b) => b[1] - a[1]).map(([key, v]) => { const [ci, a, b] = key.split("\u0000"), i = +ci, A = pos[i].m.get(a), Bn = pos[i + 1].m.get(b), h = v * pos[i].sc;
    const y0 = A.y + A.o, y1 = Bn.y + Bn.oi; A.o += h; Bn.oi += h; const x0 = X[i] + nw, x1 = X[i + 1], xm = (x0 + x1) / 2;
    return `<path class="skl" d="M${x0} ${y0.toFixed(1)} C${xm} ${y0.toFixed(1)} ${xm} ${y1.toFixed(1)} ${x1} ${y1.toFixed(1)} L${x1} ${(y1 + h).toFixed(1)} C${xm} ${(y1 + h).toFixed(1)} ${xm} ${(y0 + h).toFixed(1)} ${x0} ${(y0 + h).toFixed(1)} Z" fill="${color(i, a)}"${tipAttr(`${a} → ${b}`, `${fv(v)} · ${Math.round(v * 100 / tot)}%`)}/>`; }).join("");
  const short = x => x.length > 12 ? x.slice(0, 11) + "…" : x;
  const nodes = cols.map((c, i) => c.map(([key, v]) => { const n = pos[i].m.get(key), lx = i === 2 ? X[i] + nw + 6 : X[i] - 6;
    return `<rect class="skn" x="${X[i]}" y="${n.y.toFixed(1)}" width="${nw}" height="${n.h.toFixed(1)}" fill="${color(i, key)}"${tipAttr(key, `${fv(v)} · ${Math.round(v * 100 / tot)}%`)}/><text class="skt${i === 2 ? " s" : ""}" x="${lx}" y="${(n.y + n.h / 2 + 4).toFixed(1)}">${esc(short(key))} <tspan class="skv">${esc(fv(v))}</tspan></text>`; }).join("")).join("");
  const yes = vals[2].get(YES) || 0, rec = tot - (vals[2].get(UNK) || 0), mname = {cost: t("Estimated cost"), credits: t("Kiro credits"), tokens: t("Tokens"), minutes: t("Session time")}[k];
  const sy = rec <= 0 ? t("These agents don't record which commits they ran, so the outcome is not known") : t("{pc}% of {metric} from agents that record commits went to sessions that reached a commit or PR ({v} of {tot})", {pc: Math.round(yes * 100 / rec), metric: mname, v: fv(yes), tot: fv(rec)});
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-skk="${x}" aria-pressed="${x === k}">${esc({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), minutes: t("Session time")}[x])}</button>`).join("")}</div>` : "";
  return `<div class="inshead"><h4>${esc(t("Where it flowed"))}</h4>${mtabs}</div>${say(sy)}<figure class="insfig isk"><svg viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(t("Where it flowed"))}"><text class="skh" x="${X[0] + nw / 2}" y="-2">${esc(t("Agent"))}</text><text class="skh" x="${X[1] + nw / 2}" y="-2">${esc(t("Project"))}</text><text class="skh" x="${X[2] + nw / 2}" y="-2">${esc(t("Outcome"))}</text>${paths}${nodes}</svg><p class="ifxs">${fx(t("Band width = the amount; a session reached a commit or PR when it ran one (Claude Code) or one was made in its project while it ran"), "", t("Band width is the amount"))}</p></figure>`;
}
/* insBox は、1 セッションあたりの量のばらつきを期間ごとに箱ひげで（箱 = 真ん中の 50%、線 = 中央値、ひげ = 下位・上位 10%）。最大 13 期間 */
function insBox(){
  const hs = historyStart(), K = {cost: {f: s => sesTok(s) ? s.cost || 0 : null, fv: usd, n: t("Cost")}, minutes: {f: s => minIn(s, s.start, s.end) || null, fv: v => dur(v), n: t("Time")}, prompts: {f: s => s.nPrompts || null, fv: v => `${Math.round(v)}`, n: t("Prompts")}};
  const ps = []; for (let i = INS_N; i >= 0; i--){ const p = periodBack(i); if (p.we > hs) ps.push(p); } if (ps.length < 2) return "";
  const ks = Object.keys(K).filter(k => DATA.some(s => s.start >= ps[0].ws && K[k].f(s) != null)); if (!ks.length) return "";
  const k = ks.includes(st.insBk) ? st.insBk : ks[0], M = K[k];
  const bx = ps.map(p => { const v = DATA.filter(s => s.start >= p.ws && s.start < p.we).map(M.f).filter(x => x != null && x > 0); return v.length >= 3 ? {p, n: v.length, q: [.1, .25, .5, .75, .9].map(q => insQuant(v, q))} : {p, n: v.length, q: null}; });
  const has = bx.filter(x => x.q); if (has.length < 2) return "";
  const W = 460, H = 240, L = 50, R = 8, T = 14, B = 34, mx = Math.max(...has.map(x => x.q[4])) * 1.05, Y = insTicks(k === "minutes" ? "active" : k === "cost" ? "cost" : "prompts", mx, 5), y = v => T + (H - T - B) * (1 - v / Y.top), n = bx.length, bw = (W - L - R) / n;
  const ticks = Array.from({length: Y.n + 1}, (_, i) => { const v = Y.step * i, yy = y(v).toFixed(1); return `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${yy}" y2="${yy}"/><text x="${L - 8}" y="${(+yy + 4).toFixed(1)}">${esc(M.fv(v))}</text></g>`; }).join("");
  const lab = p => { const [, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? (JA ? `${mm}月` : MON[mm - 1]) : `${mm}/${dd}`; };
  const body = bx.map((x, i) => { const cx = L + i * bw + bw / 2, w = bw * .5, last = i === n - 1, lbl = `<text class="ixl${last ? " cur" : ""}" x="${cx.toFixed(1)}" y="${H - B + 16}">${esc(lab(x.p))}</text>`;
    if (!x.q) return lbl; const [p10, q1, md, q3, p90] = x.q;
    return `<g class="ibx${last ? " cur" : ""}"><line class="bwh" x1="${cx.toFixed(1)}" x2="${cx.toFixed(1)}" y1="${y(p90).toFixed(1)}" y2="${y(p10).toFixed(1)}"/><rect x="${(cx - w / 2).toFixed(1)}" y="${y(q3).toFixed(1)}" width="${w.toFixed(1)}" height="${Math.max(1, y(q1) - y(q3)).toFixed(1)}" rx="2"/><line class="bmd" x1="${(cx - w / 2).toFixed(1)}" x2="${(cx + w / 2).toFixed(1)}" y1="${y(md).toFixed(1)}" y2="${y(md).toFixed(1)}"/></g>${lbl}<rect class="ihit" x="${(L + i * bw).toFixed(1)}" y="0" width="${bw.toFixed(1)}" height="${H - B}"${tipAttr(x.p.label, [`${t("median")} ${M.fv(md)}`, `${t("middle half")} ${M.fv(q1)}–${M.fv(q3)}`, t("{n} sessions", {n: x.n})])}/>`; }).join("");
  const cur = bx[n - 1].q, past = has.filter(x => x !== bx[n - 1]).map(x => x.q[2]), um = past.length ? insQuant(past, .5) : null;
  const sy = cur && um != null ? t("Median session {v}; across past periods the median was usually {u}", {v: M.fv(cur[2]), u: M.fv(um)}) : "";
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-bxk="${x}" aria-pressed="${x === k}">${esc(K[x].n)}</button>`).join("")}</div>` : "";
  return `<div class="inshead"><h4>${esc(t("Spread per session"))}</h4>${mtabs}</div>${say(sy)}<figure class="insfig ibxf"><svg viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(t("Spread per session"))}">${ticks}${body}</svg><p class="ifxs">${fx(t("Box = the middle half of sessions, line = median, whiskers = 10th to 90th percentile; periods with fewer than 3 sessions are left empty"), "", t("Box: middle half · line: median · whiskers: 10%–90%"))}</p></figure>`;
}
/* insGantt は、表示中の期間でいちばん作業した日の、セッションの時間の帯（ガント）。同時に動いていたセッションの数がわかる。押すとそのセッション */
function insGantt(){
  const {ws, we} = period(); let best = null;
  // 日の作業時間は、重なったセッションを 1 度だけ数える（カレンダーと同じ数え方）
  const unionMin = (d, e) => { const iv = DATA.flatMap(s => (s.segs || []).map(([a, b]) => [Math.max(a, d), Math.min(b, e)]).filter(([a, b]) => b > a)).sort((p, q) => p[0] - q[0]); let m = 0, ca = null, cb = null; iv.forEach(([a, b]) => { if (cb == null || a > cb){ if (cb != null) m += cb - ca; ca = a; cb = b; } else cb = Math.max(cb, b); }); if (cb != null) m += cb - ca; return m / 60; };
  for (let d = ws; d < we; d += 86400){ const e = d + 86400, m = unionMin(d, e); if (m > 0 && (!best || m > best.m)) best = {d, e, m}; }
  if (!best) return "";
  const ss = DATA.filter(s => minIn(s, best.d, best.e) > 0).sort((a, b) => a.start - b.start).slice(0, 14); if (ss.length < 2) return "";
  const segs = ss.flatMap(s => (s.segs || []).map(([a, b]) => [Math.max(a, best.d), Math.min(b, best.e)]).filter(([a, b]) => b > a));
  const h0 = Math.floor((Math.min(...segs.map(x => x[0])) - best.d) / 3600), h1 = Math.ceil((Math.max(...segs.map(x => x[1])) - best.d) / 3600);
  const W = 460, rh = ss.length <= 6 ? 30 : ss.length <= 10 ? 24 : 20, T = 18, /* セッションが少ない日は行を太く（カードの下が空かないように） */ L = 130, R = 8, H = T + ss.length * rh + 6, x = tt => L + (W - L - R) * ((tt - best.d) / 3600 - h0) / Math.max(1, h1 - h0);
  let peak = 0; const ev = segs.flatMap(([a, b]) => [[a, 1], [b, -1]]).sort((p, q) => p[0] - q[0] || p[1] - q[1]); let c = 0; ev.forEach(([, d]) => { c += d; peak = Math.max(peak, c); });
  const hrs = Array.from({length: h1 - h0 + 1}, (_, i) => h0 + i).filter((h, i, a) => (h1 - h0 <= 12 || i % 2 === 0) || i === a.length - 1).map(h => `<g class="itk"><line x1="${x(best.d + h * 3600).toFixed(1)}" x2="${x(best.d + h * 3600).toFixed(1)}" y1="${T - 4}" y2="${H - 4}"/><text class="ixl" x="${x(best.d + h * 3600).toFixed(1)}" y="${T - 7}">${h}:00</text></g>`).join("");
  const short = v => v.length > 18 ? v.slice(0, 17) + "…" : v;
  const rows = ss.map((s, i) => { const yy = T + i * rh, col = st.colorBy === "project" ? colorOf(s.project) : agColor(s.source);
    const bars = (s.segs || []).map(([a, b]) => [Math.max(a, best.d), Math.min(b, best.e)]).filter(([a, b]) => b > a).map(([a, b]) => `<rect x="${x(a).toFixed(1)}" y="${(yy + 3).toFixed(1)}" width="${Math.max(1.5, x(b) - x(a)).toFixed(1)}" height="${rh - 6}" rx="2" fill="${col}"/>`).join("");
    return `<g class="igr" data-sid="${esc(s.id)}" tabindex="0" role="button" aria-label="${esc(s.title)}"><text class="igl" x="${L - 8}" y="${(yy + rh / 2 + 4).toFixed(1)}">${esc(short(s.title))}</text>${bars}<rect class="ihit" x="0" y="${yy}" width="${W}" height="${rh}"${tipAttr(s.title, `${hm(s.start)}–${hm(s.end)} · ${s.project} · ${s.source}`)}/></g>`; }).join("");
  const sy = t("{d}: {n} sessions, {h} of active time; up to {p} at the same time", {d: dMD(new Date(best.d * 1000)), n: ss.length, h: dur(Math.round(best.m)), p: peak});
  return `<div class="inshead"><h4>${esc(t("Busiest day"))}</h4></div>${say(sy)}<figure class="insfig igt"><svg viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(t("Busiest day"))}">${hrs}${rows}</svg><p class="ifxs">${fx(t("Each row is a session that worked that day; bars are its active stretches. Click a row to open the session"), "", t("Click a row to open the session"))}</p></figure>`;
}
/* 数の枠（statH）に、いつもの範囲との不等式を添える。k は MET の指標名（なければ添えない） */
const insStat = (label, v, sub, help, metric, k) => statH(esc(label), v, [sub, k && MET[k] ? insChip(insBase(k)) : ""].filter(Boolean).join(" "), help, metric);
/* AI が変えたファイル：表示中の期間に始まったセッションで、編集のツールが変えたファイルの数・行（編集の前後を比べた目安）と、
   いちばん変えたファイル。git ではなく履歴のツール呼び出しから数えるので、コミットしなかった変更や、あとで戻した変更も入る。
   セッションごとに載るファイルは多く変えた 30 まで（それより多いセッションは、残りをファイルの数にだけ足す） */
function insEditsBody(){
  const {ws, we} = period(), xs = DATA.filter(s => s.start >= ws && s.start < we && s.nFiles > 0); if (!xs.length || !xs.some(s => (s.editAdded || 0) + (s.editRemoved || 0) > 0)) return ""; // 行を数えられない編集（Kiro の editCode など）だけなら、0 と出さずに AI の編集の列ごと出さない
  const by = new Map(); let add = 0, del = 0, edits = 0, extra = 0;
  for (const s of xs){ add += s.editAdded || 0; del += s.editRemoved || 0; extra += Math.max(0, s.nFiles - s.files.length);
    s.files.forEach((f, i) => { const e = (s.fileEdits && s.fileEdits[i]) || [0, 0, 0], abs = f.startsWith("/") || /^[A-Za-z]:[\\/]/.test(f), k = abs ? f : `${s.projectPath}\u0000${f}`;
      const x = by.get(k) || {f, rel: abs && s.projectPath && f.startsWith(s.projectPath + "/") ? f.slice(s.projectPath.length + 1) : f, project: s.project, n: 0, a: 0, d: 0, sid: s.id, top: -1};
      x.n += e[0]; x.a += e[1]; x.d += e[2]; edits += e[0]; if (e[1] + e[2] > x.top){ x.top = e[1] + e[2]; x.sid = s.id; } by.set(k, x); }); }
  const nf = by.size + extra, fs = [...by.values()].sort((a, b) => (b.a + b.d) - (a.a + a.d) || b.n - a.n || (a.rel < b.rel ? -1 : 1)), top = fs.slice(0, 8), n = commas;
  const w0 = Math.max(1, top[0].a + top[0].d), multi = new Set(top.map(x => x.project)).size > 1;
  const S = [insStat(t("Files edited by AI"), n(nf), esc(plural(xs.length, "session")), "aiFiles"),
    insStat(t("Lines edited by AI"), n(add + del), fx(`+${n(add)} −${n(del)}; ${n(edits)} ${t("edits")}`, "", "", lnPM(n(add), n(del))), "aiLines")].join("");
  // ファイル名はファイルそのものを開く（セッションの間のコミットのリンク、なければこの PC のファイル。セッション詳細の一覧と同じ）。
  // そのファイルをいちばん変えたセッションは、右端の編集の回数から開く
  const href = x => { const ss = DATA.find(s => s.id === x.sid); return ss ? fileLink(ss, x.f) || localHref(x.f, ss.projectPath) : ""; };
  // 同じファイルを Git のコミットで何回変えたか（AI と手）。「よく変えたファイル」を 1 つの表にまとめる
  const gc = new Map(); gitIn(ws, we).forEach(c => (c.files || []).forEach(f => { const k = `${c.repo}/${f.path}`, z = gc.get(k) || {n: 0, ai: 0}; z.n++; if (c.ai) z.ai++; gc.set(k, z); }));
  const gcOf = x => gc.get(x.f) || null;
  const rows = top.map(x => `<div class="iwr"><span class="iwn iwf" title="${esc(x.f)}">${multi ? `<small>${esc(x.project)}</small> ` : ""}${fileA(href(x), esc(x.rel))}</span><span class="iwb"><span style="width:${((x.a + x.d) * 100 / w0).toFixed(1)}%;background:var(--acc)"></span></span><span class="iwv">+${n(x.a)} −${n(x.d)}${(z => z ? ` · ${esc(t("{c}, {ai} by AI", {c: plural(z.n, "commit"), ai: z.ai}))}` : "")(gcOf(x))}</span><button type="button" class="iwses" data-sid="${esc(x.sid)}" title="${esc(t("Open the session that changed it most"))}">${esc(t("{n}×", {n: x.n}))}</button></div>`).join("");
  return {stats: S, files: `${say(t("{file} changed the most: {lines} lines in {n} edits", {file: top[0].rel, lines: n(top[0].a + top[0].d), n: n(top[0].n)}))}<figure class="insfig"><div class="iwrows" role="table" aria-label="${esc(t("Most edited files"))}"><div class="iwr iwh" aria-hidden="true"><span>${esc(t("File"))}</span><span></span><span class="iwv">${esc(t("Lines · Git commits"))}</span><span>${esc(t("Edits"))}</span></div>${rows}</div><p class="ifxs">${fx(t("+ − = lines added and removed, compared before and after each edit; × = edits"), "", t("Lines: before and after each edit, an estimate"))}</p></figure>`, nf, add, del};
}
/* 成果：コミット（うち AI）・変えた行・PR・成果まで届いたセッション・コミットあたりの費用。割った指標は式で */
function insOutputs(){
  const {S: w} = period(); if (!w) return "";
  const o = w.outputs || {commits: 0}, g = w.git && w.git.commits ? w.git : null, base = w.outBase ?? w.sessions, n = commas, E = insEditsBody() || null;
  if (!o.commits && !g && !o.prs && !E) return "";
  const lines = g ? g.added + g.removed : 0, sp = base ? Math.round(w.outSessions * 100 / base) : null;
  // コミット・行・PR の数は、上の「出力」の枠にある（ここでは繰り返さない）。ここには、上にない Git の数だけ
  const S = [
    ...gitMore(),
    base ? insStat(t("Sessions that reached a commit or PR"), `${sp}<small>%</small>`, fx(`${w.outSessions} ÷ ${base} ${t("Claude Code sessions")} = ${sp}%`), "outSessions", "outSessions", "outSessions") : "",
  ].join("");
  // 読み取りの 1 文も、Git と AI の編集で列ごとに分ける（1 つの文につなぐと、2 つを足した数に読めてしまう）
  const syG = g ? t("{c} commits, {ai}% by AI; {sp}% of Claude Code sessions reached a commit or PR", {c: g.commits, ai: Math.round(g.ai * 100 / g.commits), sp: sp ?? "—"}) : o.commits ? t("{c} AI commits; {sp}% of Claude Code sessions reached a commit or PR", {c: o.commits, sp: sp ?? "—"}) : "";
  const syE = E ? t("AI edited {n} files (+{a} −{d} lines)", {n: n(E.nf), a: n(E.add), d: n(E.del)}) : "";
  // Git（リポジトリ）と AI の編集（履歴）は元が違うので、列と小見出しで分け、足したり割ったりして混ぜない
  const col = (cls, icon, h, note, body) => `<div class="iosec ${cls}" role="group" aria-label="${esc(h)}"><h6 class="iksh">${ico(icon)}${esc(h)} <small>${esc(note)}</small></h6>${body}</div>`;
  return `<div class="iout2">${col("outGit", "commit", t("Git"), t("from your repositories"), S ? `${say(syG)}<div class="stats istats">${S}</div>` : noneH(esc(t("No Git commits for this selection"))))}${E ? col("outEdit", "edit", t("AI edits"), t("from the agents' history, committed or not"), `${say(syE)}<div class="stats istats">${E.stats}</div>`) : ""}${S ? `<div class="iokinds">${insKinds()}</div>` : ""}</div>`; // 作業の種類は 2 列の下に幅いっぱいで（Git の列にだけ置くと、左の列が長くなって右の列の下が空く）
}
/* 作業の種類：コミットの件名を決まった規則で分ける（AI は使わない）。Conventional Commits の型（feat: など）があればそれを、なければ先頭の動詞を見る。
   件名は信用できない入力なので、分類にだけ使い、画面に出すときは今までどおりエスケープする */
const KIND_ORDER = ["feature", "fix", "refactor", "test", "docs", "chore", "revert", "other"];
function commitKind(subj){
  const s = String(subj || "").trim().toLowerCase(), cc = /^([a-z]+)(\([^)]*\))?!?:/.exec(s);
  if (/^revert\b/.test(s)) return "revert";
  if (cc){ const k = cc[1]; return k === "feat" || k === "feature" ? "feature" : k === "fix" || k === "bugfix" || k === "hotfix" ? "fix" : ["refactor", "perf", "style"].includes(k) ? "refactor" : k === "test" || k === "tests" ? "test" : k === "docs" || k === "doc" ? "docs" : ["chore", "build", "ci", "deps", "release"].includes(k) ? "chore" : "other"; }
  const w = s.split(/[\s:]+/)[0];
  if (/\b(readme|docs?|documentation|release notes|changelog|api reference|guide)\b/.test(s)) return "docs";
  if (/^(add|adds|added|write|create)$/.test(w) && /\b(tests?|e2e|specs?)\b/.test(s)) return "test";
  if (/^(tweak|polish|adjust)$/.test(w)) return "refactor";
  if (/^update (the )?(dep|deps|dependencies|packages|lockfile)/.test(s)) return "chore";
  if (/^(speed|optimi[sz]e|improve)$/.test(w) && /\b(up|performance|speed|faster|slow)\b/.test(s)) return "refactor";
  if (/^(add|adds|added|implement|create|introduce|support|enable|allow|new|build|write|make)$/.test(w)) return "feature";
  if (/^(fix|fixes|fixed|correct|resolve|repair|handle|prevent|avoid)$/.test(w)) return "fix";
  if (/^(refactor|clean|cleanup|rename|move|simplify|extract|tidy|reorganize|split|merge)$/.test(w) && !/^merge (branch|pull request|remote)/.test(s)) return "refactor";
  if (/^(test|tests|testing)$/.test(w)) return "test";
  if (/^(doc|docs|document|readme)$/.test(w) || /^update (the )?(readme|docs|documentation)/.test(s)) return "docs";
  if (/^(bump|upgrade|release|chore|ci)$/.test(w)) return "chore";
  return "other";
}
const kindName = k => ({feature: t("Features"), fix: t("Fixes"), refactor: t("Refactoring"), test: t("Tests"), docs: t("Docs"), chore: t("Maintenance"), revert: t("Reverts"), other: t("Other")})[k];
/* insKinds は、表示中の期間の Git のコミットを種類ごとに：件数・行・（AI のコミットなら）そのセッションの推定費用をコミットの数で割った分。取り消しの数も */
function insKinds(){
  const {ws, we} = period(), cs = gitIn(ws, we); if (!cs.length) return "";
  const per = new Map(); (META.git || []).forEach(c => { if (c.ai && c.session) per.set(c.session, (per.get(c.session) || 0) + 1); });
  const ses = new Map(DATA.map(s => [s.id, s])), K = new Map();
  cs.forEach(c => { const k = commitKind(c.subject), x = K.get(k) || {k, n: 0, a: 0, d: 0, cost: 0}; x.n++; x.a += c.added || 0; x.d += c.removed || 0;
    const s = c.ai && c.session && ses.get(c.session); if (s && sesTok(s)) x.cost += (s.cost || 0) / per.get(c.session); K.set(k, x); });
  const xs = KIND_ORDER.map(k => K.get(k)).filter(Boolean), top = Math.max(...xs.map(x => x.n)), cost = xs.some(x => x.cost >= 0.005), n = commas;
  const rows = xs.map((x, i) => `<div class="iwr"><span class="iwn"><i class="kd k${i % 8}"></i>${esc(kindName(x.k))}</span><span class="iwb"><span style="width:${(x.n * 100 / top).toFixed(1)}%;background:var(--c${i % 8})"></span></span><span class="iwv">${esc(plural(x.n, "commit"))} · ${lnPM(n(x.a), n(x.d))}${cost ? ` · ${esc(usd(x.cost))}` : ""}</span><b>${Math.round(x.n * 100 / cs.length)}%</b></div>`).join("");
  // 取り消し：「Revert "…"」の件名と、本文の「This reverts commit <hash>」から、取り消された元のコミットを探す
  const rv = cs.filter(c => commitKind(c.subject) === "revert"), byHash = new Map((META.git || []).map(c => [c.hash, c]));
  const tgt = rv.map(c => { const m = /this reverts commit ([0-9a-f]{7,40})/i.exec(c.body || ""), q = /^revert "(.+)"$/i.exec(String(c.subject || "").trim());
    return m ? byHash.get(m[1]) || [...byHash.values()].find(d => d.hash.startsWith(m[1])) : q ? (META.git || []).find(d => d.repo === c.repo && d.subject === q[1] && d.t < c.t) : null; });
  const rvAI = tgt.filter(d => d && d.ai).length;
  const rvLine = rv.length ? `<p class="note">${esc(t("{n} reverted, {ai} of them AI commits", {n: plural(rv.length, "commit"), ai: rvAI}))}</p>` : "";
  return `<figure class="insfig ikinds"><figcaption class="iwcap">${esc(t("Kinds of work, from commit subjects"))}</figcaption><div class="iwrows" role="table" aria-label="${esc(t("Kinds of work, from commit subjects"))}">${rows}</div>${rvLine}<p class="ifxs">${fx(t("Sorted by the type in the subject (feat:, fix: …) or its first verb (Add, Fix …); the cost of an AI commit is its session's estimated cost divided by that session's AI commits"), "", t("By the commit subject; cost only for AI commits"))}</p></figure>`;
}
/* Git のほかの出力（ファイル・リポジトリ・コミットのあった日・push）。どれもリポジトリから。途中の期間は、今日までの日数で割る */
function gitMore(){
  const {S: w, ws, we} = period(), g = w && w.git && w.git.commits ? w.git : null; if (!g) return [];
  const cs = gitIn(ws, we), n = commas, out = [], fs = new Set(); let more = 0;
  cs.forEach(c => { (c.files || []).forEach(f => fs.add(`${c.repo}\u0000${f.path}`)); more += Math.max(0, (c.nFiles || 0) - (c.files || []).length); });
  if (fs.size + more) out.push(insStat(t("Files in Git commits"), n(fs.size + more), esc(more ? t("includes about {n} from big commits that keep only their first files", {n: n(more)}) : t("Each file once"))));
  const rp = [...new Set(cs.map(c => c.project))], dd = new Set(cs.map(c => key(new Date(c.t * 1000)))), sf = soFar(), nd = sf == null ? Math.round((we - ws) / 86400) : sf;
  if (rp.length) out.push(insStat(t("Repositories with commits"), `${rp.length}`, esc(rp.slice(0, 3).join(LSEP()) + (rp.length > 3 ? "…" : ""))));
  if (dd.size) out.push(insStat(t("Days with commits"), `${dd.size}`, fx(`${dd.size} ÷ ${nd}${t("d")}`, "", sf == null ? t("of {n}", {n: plural(nd, "day")}) : t("of {n} so far", {n: plural(nd, "day")}))));
  const ps = (META.push || []).filter(x => x.t >= ws && x.t < we), sent = ps.reduce((a, x) => a + (x.commits || 0), 0);
  if (ps.length) out.push(insStat(t("Pushes"), `${ps.length}`, esc(sent ? t("{c} sent, including commits made before the period", {c: plural(sent, "commit")}) : "")));
  // 時間の目安（中央値）：セッションを始めてから、そのセッションの最初の AI のコミットまで。コミットしてから、それを送った push まで
  const first = new Map(); (META.git || []).forEach(c => { if (c.ai && c.session && (!first.has(c.session) || c.t < first.get(c.session))) first.set(c.session, c.t); });
  const tf = DATA.filter(s => s.start >= ws && s.start < we && first.has(s.id)).map(s => (first.get(s.id) - s.start) / 60).filter(m => m >= 0);
  if (tf.length) out.push(insStat(t("From session start to its first commit"), esc(dur(Math.round(insQuant(tf, .5)))), esc(t("median of {n}", {n: plural(tf.length, "session")}))));
  const byHash = new Map((META.git || []).map(c => [c.hash, c])), tp = [];
  ps.forEach(p => (p.hashes || []).forEach(h => { const c = byHash.get(h); if (c && p.t >= c.t) tp.push((p.t - c.t) / 60); }));
  if (tp.length) out.push(insStat(t("From commit to push"), esc(dur(Math.round(insQuant(tp, .5)))), esc(t("median of {n}", {n: plural(tp.length, "commit")}))));
  return out;
}
/* AI の使い方：トークン・キャッシュ・クレジット・プロンプト・プロンプトあたりの費用・サブエージェント・月末の推定 */
function insUsage(){
  const {S: w, ws, we} = period(), u = w && w.usage; if (!u || (!u.tokens && !u.credits)) return "";
  const pj = projection(w), N = !u.subagents && unrecorded(ws, we, "subagents");
  const S = [
    u.tokens ? insStat(t("Tokens"), tok(u.tokens), fx(`${t("output")} ${tok(u.out)} = ${Math.round(u.out * 100 / u.tokens)}%`), "tokens", "tokens", "tokens") : "",
    u.tokens ? insStat(t("Read from cache"), u.cacheHit == null ? "—" : `${Math.round(u.cacheHit * 100)}<small>%</small>`, fx(t("cache reads ÷ input"), "", t("Share of input tokens")), "cache", u.cacheHit == null ? "" : "cache", "cache") : "",
    insStat(t("Prompts"), commas(w.prompts), "", "prompts", "sessions", "prompts"),
    N ? norecStat(esc(t("Subagents")), N, "subagents") : insStat(t("Subagents"), `${u.subagents}`, u.subagents ? esc(t("Total run time {d}", {d: dur(u.subMin)})) + (u.subTypes.length ? " · " + esc(u.subTypes.slice(0, 3).map(([k, v]) => `${k} ${v}`).join(", ")) : "") : esc(t("Not used")), "subagents", u.subagents ? "subagents" : ""),
    pj && pj.cost != null ? insStat(t("Month-end cost (estimate)"), "≈ " + usdH(pj.cost), fx(`${t("so far")} ÷ ${pj.days}${t("d")} × ${w.days.length}${t("d")}`), "projection", "projection") : "",
    pj && pj.credits != null ? insStat(t("Month-end credits (estimate)"), `≈ ${crN(pj.credits)}`, fx(`${t("so far")} ÷ ${pj.days}${t("d")} × ${w.days.length}${t("d")}`), "projectionCr", "projectionCr") : "",
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
    // 見直す候補になる指標は、どれもここか上の数字に「?」を持ち、その横の印から開けるようにする
    insStat(t("Sessions with possible friction"), `${w.frictionN ?? (w.friction || []).length}`, "", "friction", "", "friction"),
    (rp => insStat(t("Repeated prompts"), `${rp.length}`, rp.length ? esc(`“${snipOf(rp[0].text || "", 40)}”`) : "", "repeats"))(repeatsOf(ws, we)),
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
  const rows = xs.map(x => `<tr><th scope="row"><span class="iwn" title="${esc(x.key)}"><i style="background:${x.c}"></i>${esc(x.key)}</span></th><td class="num" data-l="${L[0]}"><span class="iagb"><span style="width:${((x.minutes || 0) * 100 / max).toFixed(1)}%;background:${x.c}"></span></span>${v(x.minutes, dur)} <b>${Math.round((x.minutes || 0) * 100 / tot)}%</b></td><td class="num" data-l="${L[1]}">${commas(x.n[0])}</td><td class="num" data-l="${L[2]}">${commas(x.n[1])}</td><td class="num" data-l="${L[3]}">${v(x.cost, usd)}</td><td class="num" data-l="${L[4]}">${v(x.credits, cr)}</td><td class="num" data-l="${L[5]}">${v(x.tokens, tok)}</td></tr>`).join("");
  const sy = t("{name}: {pc}% of {metric} ({v})", {name: xs[0].key, pc: Math.round(xs[0].minutes * 100 / tot), metric: t("Active time"), v: dur(xs[0].minutes)});
  // 狭いカード（3 列に並べたときの 1 枚やスマホ）では、表の代わりに、エージェントごとに 2 行（名前と時間、残りの数字を 1 行に）。列は隠さない
  const list = xs.map(x => `<div class="iagi" role="listitem"><div class="iagr"><span class="iwn" title="${esc(x.key)}"><i style="background:${x.c}"></i>${esc(x.key)}</span><span class="iagb"><span style="width:${((x.minutes || 0) * 100 / max).toFixed(1)}%;background:${x.c}"></span></span><span class="iagv">${v(x.minutes, dur)} <b>${Math.round((x.minutes || 0) * 100 / tot)}%</b></span></div><p class="iagm">${[[L[1], commas(x.n[0])], [L[2], commas(x.n[1])], [L[3], v(x.cost, usd)], [L[4], v(x.credits, cr)], [L[5], v(x.tokens, tok)]].filter(z => z[1] !== "—").map(z => `${z[0]} <b>${z[1]}</b>`).join(" · ")}</p></div>`).join("");
  return `${say(sy)}<figure class="insfig"><div class="iagl" role="list" aria-label="${esc(t("By agent"))}">${list}</div><div class="iagw"><table class="iagt">
    <thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div><p class="ifxs">${fx(`% = ${t("Active time")} ÷ ${dur(tot)}`)}${fx(t("Time that agents ran at once is split between them"))}</p></figure>`;
}
function insTokens(){
  const {S: w} = period(), u = w && w.usage; if (!u || !u.tokens || u.cr == null && u.in == null) return "";
  const rows = [["in", t("Input"), u.in || 0, 0], ["out", t("Output"), u.out || 0, 1], ["cw", t("Cache write"), u.cw || 0, 2], ["cr", t("Cache read"), u.cr || 0, 6]].filter(r => r[2] > 0);
  const tot = rows.reduce((a, r) => a + r[2], 0); if (!tot) return "";
  const pc = v => Math.round(v * 100 / tot), inAll = (u.in || 0) + (u.cw || 0) + (u.cr || 0), b = insBase("cache");
  const bar = `<div class="itbar" role="img" aria-label="${esc(rows.map(r => `${r[1]} ${pc(r[2])}%`).join(", "))}">${rows.map(r => `<span style="flex:${r[2]};background:var(--c${r[3]})"${tipAttr(r[1], `${tok(r[2])} · ${pc(r[2])}%`)}></span>`).join("")}</div>`;
  const list = rows.map(r => `<div class="iwr"><span class="iwn" title="${esc(r[1])}"><i style="background:var(--c${r[3]})"></i>${esc(r[1])}</span><span class="iwb"><span style="width:${(r[2] * 100 / Math.max(...rows.map(x => x[2]))).toFixed(1)}%;background:var(--c${r[3]})"></span></span><span class="iwv">${esc(tok(r[2]))}</span><b>${pc(r[2])}%</b></div>`).join("");
  const top = rows.reduce((a, r) => r[2] > a[2] ? r : a);
  const sy = t("{name}: {pc}% of tokens ({v})", {name: top[1], pc: pc(top[2]), v: tok(top[2])});
  const th = b && b.better === false ? (H().cache || {}).a || "" : "";
  return `${say(sy, th)}<figure class="insfig">${bar}<div class="iwrows">${list}</div>
    <p class="ifxs">${fx(`% = ${t("tokens of the type")} ÷ ${tok(tot)}`)}${inAll ? fx(`${t("read from cache")} = ${tok(u.cr || 0)} ÷ ${tok(inAll)} = ${Math.round((u.cr || 0) * 100 / inAll)}%`) : ""}</p></figure>`; // いつもの範囲との比べは、入力に対する割合（「使い方」の「キャッシュから読んだ割合」）にだけ付ける
}
