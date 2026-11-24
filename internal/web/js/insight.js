// サマリーの頭：要点の文・推移のグラフ（自分のいつもの範囲つき）・改善のアドバイス。AI は使わず、決まった計算と規則だけで出す
/* いつもの範囲：表示中の期間の前の INS_N 期間（記録が始まる前の期間と、記録が途中から始まる期間は除く）の分布。
   中央値と四分位（真ん中の 50%）で見る。平均と標準偏差は、外れた週 1 つで大きく動くので使わない */
const INS_VOL = new Set(["cache", "offhours", "switches", "cost", "credits", "tokens", "active", "prompts", "gitCommits", "gitLines", "prs", "pushes", "aiLines"]); // 使った量・作業の量（色で良し悪しを付けない）
const INS_USE = new Set(["cost", "credits", "tokens", "active"]); // 使った量（累積の線と利用上限の印を重ねる）
const INS_N = 12, INS_MIN = 4; // 比べる期間の数と、比べるのに要る最低の数（それより少なければ「まだ比べられない」と言う）
const INS_ADD = new Set(["cost", "credits", "active", "prompts", "tokens", "gitCommits", "gitLines", "prs", "pushes", "aiLines", "limits", "longctx", "friction", "bigPrompts", "focus"]); // 期間の長さに比例して増える指標（途中の期間は、このペースで期間を終えたときの値で比べる）
const INS_SHORT = () => ({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), active: t("Active time"), gitLines: t("Git lines"), costPerCommit: t("Cost per AI commit"), tokensPerCommit: t("Tokens per AI commit"), costPerEdit: t("Cost per 100 AI lines"), creditsPerEdit: t("Credits per 100 AI lines"), tokensPerEdit: t("Tokens per 100 AI lines"), costPerHour: t("Cost per hour"), creditsPerHour: t("Credits per hour"), tokensPerHour: t("Tokens per hour"), costPerDay: t("Cost per day"), creditsPerDay: t("Credits per day"), tokensPerDay: t("Tokens per day"), costPerSes: t("Cost per session"), creditsPerSes: t("Credits per session"), tokensPerSes: t("Tokens per session"), costPerAsk: t("Cost per prompt"), creditsPerAsk: t("Credits per prompt"), tokensPerAsk: t("Tokens per prompt"), pushes: t("Pushes"), costPerPush: t("Cost per push"), creditsPerPush: t("Credits per push"), tokensPerPush: t("Tokens per push"), costPerPr: t("Cost per PR"), creditsPerPr: t("Credits per PR"), tokensPerPr: t("Tokens per PR"), cache: t("Cache"), fix: t("Corrections"), wait: t("Wait")}); // 切り替えの短い名前（正式な名前は title と、グラフの下の文）
// 量の切り替えの名前（サマリーのどのカードでも同じ）。名前の決まり：見出し・数字の名前・文では正式な名前（推定費用・Kiro クレジット）、切り替えのボタンでは短い名前（費用・クレジット）。
// 時間は 2 つを分ける：稼働時間（Active time。同時に動いていたセッションは 1 度だけ数える）と、セッションの時間（Session time。セッションごとの稼働を足す。重なりもそれぞれ数える）
const INS_STIME = () => fx(t("Session time adds up each session's own active time, so sessions that ran at once each count; Active time counts that time once"));
const INS_SW_NAME = () => ({cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), active: t("Active time"), minutes: t("Session time")});
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
const insName = k => (H()[k] || H()[{gitLines: "lines"}[k]] || {}).n || k; // Git の行は、説明（HELP）の id が lines
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
// insOdd は、良し悪しの決まらない数字（使った量・作業の量など。insTone が付かないもの）が、いつもの範囲から大きく外れたか。
// 「かなり上・下」（insPlain）や推移の赤い点と同じ決まり（上位25%点 + 1.5×幅より上か、下位25%点 − 1.5×幅より下）。上でも下でも同じ中立の色で目を留め、色だけに頼らないよう札（insOddH）も添える
const insOdd = b => !!(b && b.q1 != null && b.far && b.better == null);
const insOddT = b => b.pos === "high" ? t("Far above usual") : t("Far below usual");
// 札は言葉を出さず、値の横の小さな矢印だけ（同じ琥珀色。色だけに頼らない目印）。言葉は読み上げ用の隠れた文字と title に
const insOddH = b => insOdd(b) ? `<span class="iodd" title="${esc(insOddT(b))}"><span aria-hidden="true">${b.pos === "high" ? "↑" : "↓"}</span><span class="sr">${esc(insOddT(b))}</span></span>` : "";
const insOddV = (b, v) => insOdd(b) ? `<span class="odd">${v}</span>${insOddH(b)}` : v; // 数の枠・行の値（HTML）を琥珀色にして矢印を添える
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
/* insSpark は、要点の数字の横に置く推移のグラフ。過去 INS_N 期間と表示中の期間を、いつもの範囲（真ん中の 50%）の帯と中央値つきで描く。
   - 量のパネル（投入・出力。bar）は、期間ごとの棒（量は足し上げるものなので、棒の高さで比べる）。表示中の期間の棒は濃い色。
     途中の期間は、今までの値の棒に、このペースで期間を終えたときの値までの淡い破線の棒を重ねる
   - 単価のパネルは、点と線（率は足し上げる量ではないので、棒の面積で比べさせない）。途中の期間で期間の長さに比例する指標は、ペース換算の値を白抜きの点で描く（線は終わった期間だけ）
   - 累積：使った量（費用・時間など）は、グラフの左端の期間からの足し上げ（右の軸。表示中の期間は今までの実際の値）。
     率（コミットあたり・時間あたりなど）は足し上げられないので、左端からの期間の値の平均（同じ左の軸）
   軸は実線、目盛りは両方の軸に、y の目盛りには点線のグリッド */
// insBarS は、棒の上に置く量の短い書き方（棒の幅に収まるよう、千を k・百万を M に。費用は $、時間は h）
const insBarS = (k, v) => { const kn = x => x >= 1e6 ? `${+(x / 1e6).toFixed(x >= 1e7 ? 0 : 1)}M` : x >= 1e3 ? `${+(x / 1e3).toFixed(x >= 1e4 ? 0 : 1)}k` : `${+x.toFixed(x >= 10 || Number.isInteger(x) ? 0 : 1)}`;
  return k === "cost" ? "$" + kn(v) : k === "active" ? (v >= 60 ? `${kn(v / 60)}h` : `${Math.round(v)}m`) : kn(v); };
function insSpark(b, label, W = 480, bar = false){
  if (!b || !b.k || !MET[b.k]) return "";
  const k = b.k, hs = historyStart(), fm = b.fmt || (x => fmtM(k, x)), pts = [];
  for (let i = INS_N; i >= 1; i--){ const p = periodBack(i); pts.push({p, v: p.ws < hs ? null : metricAt(k, p)}); }
  pts.push({p: periodBack(0), v: b.v, a: b.cur, cur: true});
  if (!pts.some(x => x.v != null)) return ""; // 記録が 1 期間でもあれば描く（月で見ると、履歴の短いエージェントのグラフだけが消えて、週と月で並びが変わらないように）
  // 累積（右の軸）と利用上限の印は、使った量（費用・クレジット・トークン・時間）だけ。コミットや行の数に重ねても、いつも右上がりなだけで読み取れることがない
  const f0 = Math.min(Math.max(0, pts.findIndex(x => x.p.we > hs)), pts.length - 4), ps = pts.slice(f0), n = ps.length, /* 記録の始まる前の期間は描かない（履歴の短い人のグラフが、右端に寄って空かないように。最大 13 期間で「?」の推移と同じ） */ use = INS_USE.has(k), add = use, cnt = INS_ADD.has(k) && !use;
  { let s = 0, c = 0; ps.forEach(q => { const a = q.cur ? q.a : q.v; if (a != null){ s += a; c++; } q.c = cnt || !c ? null : add ? s : s / c; }); }
  const vals = ps.filter(x => x.v != null).map(x => x.v).concat(add ? [] : ps.filter(x => x.c != null).map(x => x.c));
  // 利用上限に当たった期間（棒のグラフでは、棒に重ねず x 軸のすぐ下の段に印を置き、期間の名前をその分だけ下げる）
  const lh = use ? ps.map(q => limitHits(q.p.ws, q.p.we)) : [], LB = bar && lh.some(h => h.length) ? 12 : 0;
  const wk = st.mode !== "month", H = 270, L = 50, R = add ? 50 : 12, T = 20, PX = !bar && n > 1 ? 16 : 0, step = bar ? (W - L - R) / n : n > 1 ? (W - L - R - 2 * PX) / (n - 1) : W - L - R;
  // 棒のグラフで 1 本だけ飛び抜けている（2 番目の 2.5 倍を超える）ときは、目盛りを 2 番目といつもの範囲に合わせ、その棒だけ上を切る
  // （ほかの棒が潰れて読めなくならないように）。切った棒には切れ目の印を付け、実際の量は棒の中に書く（insBarS）。対数の目盛りは使わない（棒の長さが量に比例しなくなる）
  const tops = !bar ? [] : ps.map(q => q.v == null ? 0 : q.cur && b.part ? Math.max(b.v, b.cur) : q.v).filter(v => v > 0).sort((p, q) => q - p);
  const cut = tops.length >= 3 && tops[0] > 2.5 * tops[1] ? Math.max(tops[1], b.q3 || 0, ...(add ? [] : vals.filter(v => v < tops[0]))) * 1.15 : null;
  const thin = wk && n > 1 && (bar ? step : (W - L - R - 32) / (n - 1)) < 32, B = 34 + LB, mx = cut != null ? cut : Math.max(...vals, b.q3 || 0);
  // 目盛りは、各点の値を読み取れるよう細かく（6〜10 本ほど）、切りのよい刻みで。数える指標は整数、時間は 1 時間単位の刻み
  const Y1 = insTicks(k, MET[k].u === "%" ? Math.min(100, mx * 1.1) : mx * 1.02), top = Y1.top, x = i => bar ? L + (i + .5) * step : L + (n === 1 ? (W - L - R) / 2 : PX + i * step), y = v => T + (H - T - B) * (1 - v / top);
  const cProj = add && b.part && ps[n - 1].c != null ? ps[n - 1].c - b.cur + b.v : null; // 途中の期間：このペースで期間を終えたときの累積
  const Y2 = add ? insTicks(k, Math.max(ps[n - 1].c, cProj || 0) * 1.02, Y1.n, true) : Y1, top2 = Y2.top, y2 = add ? (v => T + (H - T - B) * (1 - v / top2)) : y, y0 = y(0).toFixed(1);
  const tk = (yy, lab, right) => `<line class="tm" x1="${right ? W - R : L - 3}" x2="${right ? W - R + 3 : L}" y1="${yy}" y2="${yy}"/>${lab == null ? "" : `<text class="${right ? "r" : ""}" x="${right ? W - R + 5 : L - 5}" y="${(+yy + 3.5).toFixed(1)}">${esc(lab)}</text>`}`;
  // y 軸：0 から 4 等分の目盛り（軸の外に短い線、プロットの中に点線のグリッド。値は 0・半分・上端に。数える指標で 6.25 のような値を出さないように）
  const ticks = Array.from({length: Y1.n + 1}, (_, i) => { const v = Y1.step * i, yy = y(v).toFixed(1);
    return `<g class="tk">${i ? `<line class="grid" x1="${L}" x2="${W - R}" y1="${yy}" y2="${yy}"/>` : ""}${tk(yy, tickS(k, v, Y1.step))}</g>`; }).join("")
    + (add ? Array.from({length: Y2.n + 1}, (_, i) => tk(y2(Y2.step * i).toFixed(1), tickS(k, Y2.step * i, Y2.step), true)).join("") : "")
    + `<line class="axis" x1="${L}" x2="${L}" y1="${T}" y2="${y0}"/><line class="axis" x1="${L}" x2="${W - R}" y1="${y0}" y2="${y0}"/>${add ? `<line class="axis cum" x1="${W - R}" x2="${W - R}" y1="${T}" y2="${y0}"/>` : ""}`;
  // 帯は棒の後ろ、中央値の線は棒の上（棒に隠れず、どの棒が中央値より上かを読めるように。線のグラフでは今までどおり帯と一緒に後ろ）
  const band = b.q1 == null ? "" : `<rect class="band" x="${L}" y="${y(b.q3).toFixed(1)}" width="${W - L - R}" height="${Math.max(1, y(b.q1) - y(b.q3)).toFixed(1)}"/>`;
  const med = b.q1 == null ? "" : `<line class="med" x1="${L}" x2="${W - R}" y1="${y(b.med).toFixed(1)}" y2="${y(b.med).toFixed(1)}"/>`;
  const xtm = ps.map((_, i) => `<line class="tm" x1="${x(i).toFixed(1)}" x2="${x(i).toFixed(1)}" y1="${y0}" y2="${(+y0 + 3).toFixed(1)}"/>`).join("");
  // x 軸は、どの期間にも名前を付ける。週は短い日付（月/日）、月は月の名前
  const sl = (p, i) => { const [yy, mm, dd] = p.key.split("-").map(Number), yr = i === 0 || mm === 1; return wk ? `${mm}/${dd}` : JA ? (yr ? `${yy}年${mm}月` : `${mm}月`) : (yr ? `${MON[mm - 1]} ${yy}` : MON[mm - 1]); }; // 月は、最初と 1 月に年を添える（同じ月の名前が 2 度出ても見分けられるように）
  // 名前はいつも横書き（斜めにすると読みにくく、下の段の上限の印とも重なる）。間が狭いとき（週の日付が 1 期間の幅に収まらない）は、1 つおきに出す（表示中の期間と、そこから 2 つおきは残す）
  const xl = ps.map((q, i) => { const alt = (n - 1 - i) % 2 && (n > 6 || thin); return `<text class="sxl${i === n - 1 ? " cur" : alt ? ` alt${thin ? " thin" : ""}` : i === 0 && !wk && !bar ? " s" : ""}${i === 0 && q.v == null && n > 1 ? " nodat" : ""}" x="${x(i).toFixed(1)}" y="${(+y0 + 14 + LB).toFixed(1)}">${esc(sl(q.p, i))}</text>`; }).join("");
  // 累積は時間の流れなので、途中の期間の「ここまで」は右端（期間の終わり）ではなく、期間のうち過ぎた割合の位置に置く。そこから期間の終わりの予測までを破線に
  const xc = i => ps[i].cur && b.part && n > 1 ? x(n - 2) + insFrac() * (x(n - 1) - x(n - 2)) : x(i);
  const cpts = ps.map((q, i) => q.c == null ? null : `${xc(i).toFixed(1)},${y2(q.c).toFixed(1)}`).filter(Boolean);
  const cuma = cpts.length < 2 || !add ? "" : `<polygon class="cuma" points="${x(ps.findIndex(q => q.c != null)).toFixed(1)},${y0} ${cpts.join(" ")} ${xc(n - 1).toFixed(1)},${y0}"/>`, cuml = cpts.length < 2 ? "" : `<polyline class="cuml" points="${cpts.join(" ")}"/>`;
  const cprj = cProj != null && ps[n - 1].c != null ? `<line class="cprj" x1="${xc(n - 1).toFixed(1)}" y1="${y2(ps[n - 1].c).toFixed(1)}" x2="${x(n - 1).toFixed(1)}" y2="${y2(cProj).toFixed(1)}"/><circle class="cprjd" cx="${x(n - 1).toFixed(1)}" cy="${y2(cProj).toFixed(1)}" r="3"/>` : "";
  let marks;
  if (bar){ // 棒：過去の期間は淡い色、表示中の期間は濃い色。途中の期間は、今までの値の棒の上に、期間の終わりの推定までの淡い破線の棒
    const bw = Math.min(22, step * .64), yc = v => y(Math.min(v, top)), rc = (cls, i, v0, v1) => `<rect class="${cls}" x="${(x(i) - bw / 2).toFixed(1)}" y="${yc(v1).toFixed(1)}" width="${bw.toFixed(1)}" height="${Math.max(0, yc(v0) - yc(v1)).toFixed(1)}"/>`
      + (v1 > top ? `<path class="sbrk" d="M${(x(i) - bw / 2 - 2).toFixed(1)} ${(T + 12).toFixed(1)}l${(bw + 4).toFixed(1)} -5m0 5l${(-bw - 4).toFixed(1)} 5"/>` : ""); // 上を切った棒の切れ目（地の色の 2 本の斜め線）
    marks = ps.map((q, i) => q.v == null ? "" : !q.cur ? rc("sbar", i, 0, q.v) : b.part ? (b.v > b.cur ? rc("sbar ext", i, b.cur, b.v) : "") + rc("sbar cur", i, 0, b.cur) : rc("sbar cur", i, 0, q.v)).join("");
  } else {
    const line = ps.map((q, i) => q.v == null || (q.cur && b.part) ? null : `${x(i).toFixed(1)},${y(q.v).toFixed(1)}`).filter(Boolean).join(" ");
    // 途中の期間の予測：前の期間の点からペース換算の点まで（推移）と、前の期間の累積から予測の累積まで（累積）を破線で
    const pv = n > 1 ? ps[n - 2] : null, prj = !b.part || !pv ? "" : (pv.v != null ? `<line class="prj" x1="${x(n - 2).toFixed(1)}" y1="${y(pv.v).toFixed(1)}" x2="${x(n - 1).toFixed(1)}" y2="${y(b.v).toFixed(1)}"/>` : "")
      + `<circle class="sofar" cx="${xc(n - 1).toFixed(1)}" cy="${y(b.cur).toFixed(1)}" r="2.5"/><text class="sofl" x="${(xc(n - 1) - 5).toFixed(1)}" y="${(y(b.cur) + (y(b.cur) < H - B - 22 ? 15 : -7)).toFixed(1)}">${esc(t("So far"))}</text>`; // 途中の期間の今までの値（印だけだと、データのずれに見える）
    marks = `<polyline class="trl" points="${line}"/>${prj}` + ps.map((q, i) => q.v == null || q.cur ? "" : `<circle class="pt" cx="${x(i).toFixed(1)}" cy="${y(q.v).toFixed(1)}" r="2.5"/>`).join("") + `<circle class="now${b.part ? " part" : ""}" cx="${x(n - 1).toFixed(1)}" cy="${y(b.v).toFixed(1)}" r="4.5"/>`;
  }
  const cl = add ? t("Cumulative") : t("Running average");
  // 前の期間からの増減は、ツールチップに（vsL・pcS）。棒の上には、その期間の実際の量を小さく（insBarS。途中の期間は今までの値を、推定の棒の上に）
  const pcOf = i => { const a = ps[i], p = i > 0 ? ps[i - 1] : null; return !bar || !p || a.v == null || !(p.v > 0) || (a.cur && b.part) ? null : Math.round((a.v - p.v) * 100 / p.v); };
  const pcS = v => `${v > 0 ? "+" : v < 0 ? "−" : "±"}${Math.abs(v)}%`, vsL = st.mode === "month" ? t("vs last month") : t("vs last week");
  // 数字が棒の間に収まらない（9px の字で 1 字 5.5px ほど）ときだけ、1 つおきに（表示中の期間と、そこから 2 つおきは残す）
  const bv = q => q.v == null ? null : insBarS(k, q.cur && b.part ? b.cur : q.v), alt = bar && Math.max(0, ...ps.map(q => (bv(q) || "").length)) * 5.5 > step - 3;
  const pcl = !bar ? "" : ps.map((q, i) => { const hv = q.v == null ? 0 : q.cur && b.part ? Math.max(b.v, b.cur) : q.v, over = hv > top; if (q.v == null || (!over && (n - 1 - i) % 2 && alt)) return ""; const yy = y(Math.min(hv, top)), up = yy > T + 12;
    return `<text class="spc${q.cur ? " cur" : ""}${over ? " over" : ""}" x="${x(i).toFixed(1)}" y="${(over ? T + 28 : up ? yy - 4 : yy + 12).toFixed(1)}">${esc(bv(q))}</text>`; }).join(""); // 上端に近い棒は棒の中に。上を切った棒は、切れ目の下に
  const hits = ps.map((q, i) => `<rect class="hit" x="${(x(i) - step / 2).toFixed(1)}" y="0" width="${step.toFixed(1)}" height="${H - B}"${tipAttr(q.p.label, q.v == null ? t("No records") : (q.cur && b.part ? insEndLabel(fm(q.v)) : fm(q.v)), q.cur && b.part ? `${t("So far")}: ${fm(b.cur)}` : "", q.c == null ? "" : `${cl}: ${fm(q.c)}`, pcOf(i) == null ? "" : `${vsL} ${pcS(pcOf(i))}`, q.cur && cProj != null ? `${cl} (${insEndLabel(fm(cProj))})` : "")}/>`).join("");
  // 利用上限に当たった時刻：小さな赤い三角（縦線はグラフを隠すので引かない）。同じ期間の到達は 1 つにまとめ、回数を添える。
  // 棒のグラフでは x 軸のすぐ下の段に（棒の上に重ねると、棒の色に紛れる）、線のグラフでは x 軸の上に
  const lim = !use ? "" : ps.map((q, i) => ({i, q, h: lh[i]})).filter(z => z.h.length).map(({i, h}) => {
    const lx = x(i) - (bar && h.length > 1 ? 5 : 0), ty = bar ? +y0 + 11 : +y0;
    const rows = h.slice(0, 6).map(z => z.r ? t("{time} (resets {r})", {time: `${md(z.t)} ${hm(z.t)}`, r: z.r}) : `${md(z.t)} ${hm(z.t)}`).concat(h.length > 6 ? ["…"] : []);
    return `<path class="slim" d="M${lx.toFixed(1)} ${(ty - 7).toFixed(1)}l-4.5 7h9z"/>${h.length > 1 ? `<text class="slimn${bar ? " s" : ""}" x="${(bar ? lx + 5.5 : lx).toFixed(1)}" y="${(bar ? ty : ty - 10).toFixed(1)}">×${h.length}</text>` : ""}<rect class="hit slimhit" x="${(x(i) - 6 - (bar ? 4 : 0)).toFixed(1)}" y="${(ty - 10).toFixed(1)}" width="${bar ? 20 : 12}" height="12"${tipAttr(h.length === 1 ? t("Usage limit hits") : `${t("Usage limit hits")} ×${h.length}`, rows)}/>`; }).join("");
  const yt = (INS_SHORT()[k] || label || insName(k)), xt = st.mode === "month" ? t("Month") : t("Week");
  const list = ps.map(q => `${q.p.label} ${q.v == null ? "—" : fm(q.v)}${q.c == null ? "" : ` (${cl} ${fm(q.c)})`}`).join(LSEP());
  return `<span class="tspk${bar ? " tbar" : ""}"><span class="sr">${esc(t("Trend: {list}", {list}))}</span><svg aria-hidden="true"${LB ? ` class="lb"` : ""} viewBox="0 0 ${W} ${H}"><text class="syt" x="${L}" y="11">${esc(yt)}</text>${add ? `<text class="syt r" x="${W - R}" y="11">${esc(cl)}</text>` : ""}${bar ? band + ticks + xtm + cuma + marks + med + cuml + cprj : band + med + ticks + xtm + cuma + cuml + marks}${pcl}${xl}<text class="sxt" x="${(L + (W - L - R) / 2).toFixed(1)}" y="${H - 3}">${esc(xt)}</text>${hits}${lim}</svg></span>`;
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
  // 範囲の外だが 1 割に満たないときは「ほぼいつも通り」（「いつも通り」と範囲を並べると、範囲の外の値と食い違って読める）
  const head = pos === "low" ? t("Below usual") : pos === "high" ? t("Above usual") : b.pos === "low" ? t("Close to usual (slightly below)") : b.pos === "high" ? t("Close to usual (slightly above)") : t("As usual");
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
/* 要点の 3 つのパネル：コスト（使った量）・出力（残ったもの）・単価（その比）。どのパネルも推移のグラフ 1 つと、見る量の切り替え（選んだものは覚える）。
   切り替えの選択肢ごとに、上の帯と同じ数を式といつもの範囲との不等式つきのタイル（名前・値・期間の終わりの推定・前の期間との比べ・評価・グラフ）にし、選んだものだけを見せる。
   パネルにない数字（Git の行・ファイル・リポジトリ・日、AI の編集、ほかの単価、前の期間に収める 1 日の目安）は、下の「内訳」と「その他のグラフ」のカードにある */
const INS_SW = {cost: "costKind", out: "outKind", unitN: "unitKind", unitD: "unitDen", uinN: "ucKind", uinD: "ucDen"}; // 切り替えの選択を覚える鍵（単価は量と割る数の 2 つ。投入単価は、前の「単価」のカードの鍵をそのまま使い、選んでいたものを引き継ぐ）
const INS_RATE = new Set(["unit", "uin"]); // 単価のパネル（評価の文が付き、% の増減と累積の線は付けない）
const INS_SIDE = {cost: "in", uin: "in", out: "out", unit: "out"}; // 投入の側（左の列）と産出の側（右の列）
// 投入のパネルの名前。英語の「Input」は、トークンの種類の「入力」（t("Input")）と同じ語なので、辞書を分けられるよう、ここで言語ごとに決める
const INS_IN = () => JA ? "投入" : "Input";
// 産出のパネル（コミット・push・PR）の名前。同じく、英語の「Output」はトークンの種類の「出力」と同じ語なので、ここで決める
// （日本語は「投入」と対の「産出」。「出力」は LLM の出力トークン、「成果」は事業の成果と取り違えるため使わない）
const INS_OUT = () => JA ? "産出" : "Output";
// insSwH は、パネルとカードの切り替え（選んだ選択肢だけを見せる。押したときの動きは bindInsight）。dim は選択を入れる data-*（量は n、割る数は d）。
// 数字のない選択肢は出さない。割る数は量によって数字のあるものが変わるので、ぜんぶ描いて、今の量で数字のないものを hidden にする（量を切り替えたときは bindInsight が付け替える）
// 選択肢が 1 つでも出す（何を数えているか・何で割っているかがわかり、横に並んだパネルの頭の高さがそろう）
const insSwH = (sw, label, opts, cur, dim = "n", hid = () => false) => opts.length ? `<div class="segc iksw" role="group" aria-label="${esc(label)}" data-sw="${sw}" data-dim="${dim}">${opts.map(([k, l]) =>
  `<button type="button" data-v="${k}" aria-pressed="${k === cur}"${hid(k) ? " hidden" : ""}>${esc(l)}</button>`).join("")}</div>` : "";
// insPickD は、量 n で数字のある割る数のうち、覚えている want（なければ最初）
const insPickD = (pairs, n, ds, want) => { const ok = ds.filter(d => pairs.has(`${n}:${d}`)); return ok.includes(want) ? want : ok[0] || ds[0]; };
// insCmp は、前の期間との比べ（上の帯の増減と同じ）。パネルのタイルと、内訳のカードの行で同じ比べ方にする
function insCmp(){
  const {P: pw} = period(), M = st.mode === "month", V = vsPrev(pw, M ? "月" : "週");
  const lab0 = () => V.n == null ? (M ? t("vs last month") : t("vs last week")) : t("vs {range}", {range: V.range});
  // 上の帯の増減（%）と同じ比べを式で：途中の期間は前の期間の同じ日まで、終わった期間は前の期間まるごと
  const vs = (a, b, f) => { if (b == null || !b) return "";
    // 前の期間が小さいと、% が「+1076%」のように大きくなって読めないので、3 倍を超える増減は差と元の値で
    if (Math.abs(a - b) > 3 * b){ const d = `${a >= b ? "+" : "−"}${f(Math.abs(a - b))}`;
      return {f: `${f(a)} − ${f(b)} = ${d}`, p: `${lab0()} ${d} (${t("from {v}", {v: f(b)})})`}; }
    const pc = `${a >= b ? "+" : "−"}${Math.round(Math.abs(a - b) * 100 / b)}% ${lab0()}`;
    return {f: `(${f(a)} − ${f(b)}) ÷ ${f(b)} = ${pc}`, p: vsFirst(pc)}; };
  // 平易な 1 行では「先週比」を先に（「+108 先週比」の +108 が、緑と赤の行数と同じ形に見えないように）
  const vsFirst = s => { const m = /^([+−±][^ ]+) (.+)$/.exec(s); return m ? `${m[2]} ${m[1]}` : s; };
  // 数えるものの差（途中の期間は、前の期間の同じ日数まで。上の帯と同じ）
  const vsInt = (k, cur, prev) => { if (prev == null || cur == null) return ""; const fm = v => fmtM(k, v), d = cur - prev, p = `${d > 0 ? "+" : d < 0 ? "−" : "±"}${fm(Math.abs(d))} ${lab0()}`; return {f: `${fm(cur)} − ${fm(prev)} = ${p}`, p: vsFirst(p)}; };
  // 前の期間との比べ（率は %、数えるものは差）。数えるものは、途中の期間なら前の期間の同じ日数まで（上の帯と同じ）。率は前の期間まるごとと
  const vsk = (k, cur, field) => { if (MET[k].int) return vsInt(k, cur, V.of(field, metricAt(k, periodBack(1))));
    const prev = V.partial ? null : metricAt(k, periodBack(1)); if (!prev || cur == null) return "";
    const pc = `${cur >= prev ? "+" : "−"}${Math.round(Math.abs(cur - prev) * 100 / prev)}% ${M ? t("vs last month") : t("vs last week")}`; return {f: `(${fmtM(k, cur)} − ${fmtM(k, prev)}) ÷ ${fmtM(k, prev)} = ${pc}`, p: vsFirst(pc)}; };
  return {V, lab0, vs, vsInt, vsk, vsFirst};
}
function insHeadlines(F){
  const {S: w, P: pw, ws, we} = period(), u = (w && w.usage) || {}, M = st.mode === "month";
  const P = {cost: [], out: [], unit: [], uin: []}; // パネルごとの選択肢 {key, h}
  let HAB = null; // 作業のしかたの違い（単価の評価で使う。期間ごとに 1 度だけ数える）
  // エージェントで絞っていて期間に push があるときは、出力単価の各タイルの式（ƒx）の中に「push あたりは「すべて」のときだけ」と書く。パネルの頭に置くと、横の投入単価の頭が 1 行ぶん空く（subgrid で行の高さをそろえるため）
  const pushNote = insAg && pushesIn(ws, we) ? t("Per push is shown for All agents only: Git doesn't record which agent's work a push sent") : "";
  // 途中の期間で、期間の長さに比例する数字には、どれも期間の終わりの予測を添える（費用・クレジット・時間・トークン・コミット・push・PR）
  const tile = (panel, key, label, b, v, f, m, help) => { f = [].concat(f || []).filter(Boolean); if (panel === "unit" && pushNote) f.push({fn: pushNote}); const ob = INS_RATE.has(panel) ? null : b; // 単価には評価の文の色（insVerdict）があるので、大きく外れた印（insOdd）は重ねない
    if (b && b.part && !f.some(x => x && x.pc)) f.push(pace(b.cur, b.v, MET[b.k] && MET[b.k].int ? (z => fmtM(b.k, Math.round(z))) : (b.fmt || (z => fmtM(b.k, z)))));
    if (INS_RATE.has(panel) && b && b.k && MET[b.k]) f.unshift({v: insVerdict(b, F, HAB || (HAB = insHabits()))});
    const obj = x => x && typeof x === "object", big = f.filter(x => obj(x) && x.big); /* 文字列には String.prototype.big があるので、オブジェクトだけを見る */
    const hk = help || (b && b.k && H()[b.k] ? b.k : m && H()[m] ? m : "");
    const vh = m ? `<button type="button" class="itv${insOdd(ob) ? " odd" : ""}" data-metric="${m}" aria-haspopup="dialog" aria-label="${esc(`${label} ${v}`)}">${esc(v)}</button>` : `<b${insOdd(ob) ? ' class="odd"' : ""}>${esc(v)}</b>`;
    // 値の段：途中の期間は「今までの値」と「期間の終わりの推定」を同じ行に並べる（推定は少し小さく、同じ基準線。入らなければ折り返す）。
    // パネルの頭（.ithd）・グラフ・下の印（.itft）を分けておき、横に並んだパネルのグラフの上端を CSS の subgrid でそろえる
    const row = `<div class="ivrow">${big.length ? `<span class="ivnow">${vh}${insOddH(ob)}<span class="ivk">${esc(t("So far"))}</span></span>${big.map(x => `<span class="ivbig"><b>≈ ${esc(x.big.v)}</b><span class="ivk">${esc(x.big.l)}</span></span>`).join("")}` : vh + insOddH(ob)}</div>`;
    const chip = b === false || f.some(x => obj(x) && x.v != null) ? "" : insChip(b);
    P[panel].push({key, h: `<div class="itile${insTone(b)}" data-ch="${key}"><div class="ithd"><span class="il">${esc(label)}${hb(hk)}</span>${row}${f.map(x => obj(x) ? (x.big ? fx(x.f) : x.v != null ? x.v : x.n != null ? pln(x.n) : x.fn != null ? `<span class="fxo fxn">${esc(x.fn)}</span>` : fx(x.f, "", x.p, x.h)) : x === t("Kiro's own count, not an estimate") ? pln(x) : fx(x)).join("")}</div>${insSpark(b, label, 480, !INS_RATE.has(panel))}${chip ? `<div class="itft">${chip}</div>` : ""}</div>`}); }; // 値を押すと、上の帯と同じ内訳
  const days = Math.round((we - ws) / 86400), el = Math.round(insFrac() * days), {V, vs, vsInt, vsk} = insCmp();
  const pace = (cur, v, f, fv = f) => ({f: `${t("pace")} = ${f(cur)} × ${days}${t("d")} ÷ ${el}${t("d")} = ${f(v)}`, big: {l: M ? t("Month-end estimate") : t("End-of-week estimate"), v: fv(v)}, pc: true}); // fv は大きな数字の形（今までの値と同じ形に） // 期間の終わりの推定は、今までの値と同じ大きさで並べる
  // 直近の期間（表示中と過去 INS_N）のどれかに 1 以上あるか（今の期間が 0 でも、推移は見られるように選択肢に残す）
  const seen = k => Array.from({length: INS_N + 1}, (_, i) => periodBack(i)).some(p => metricAt(k, p) > 0);
  const pu = pw && pw.usage;
  // ── コスト：推定費用・Kiro のクレジット・トークン・稼働時間 ──
  const bc = insBase("cost"); if (bc && (u.tokens || bc.cur)) tile("cost", "cost", t("Estimated cost"), bc, fmtM("cost", bc.cur), [vs(bc.cur, V.of("cost", pu && pu.tokens ? costOf(pu) : null), usd), bc.part ? pace(bc.cur, bc.v, usd) : ""], "cost");
  if (u.credits){ const bk = insRange("credits", u.credits, p => p.S && p.S.usage ? p.S.usage.credits || null : null, true, true, cr);
    const mp = monthProj(), r1 = cr1; // 週の表示でも、クレジットの今月末の推定を添える（クレジットは月ごとの枠）
    const me = mp && mp.credits != null ? {f: `${t("Month-end credits")} = ${t("this month")} ${r1(mp.soCredits)} ÷ ${mp.days}${t("d")} × ${mp.nd}${t("d")} = ${r1(mp.credits)}`, big: {l: t("Month-end estimate (this month)"), v: crN(mp.credits)}} : "";
    tile("cost", "credits", t("Kiro credits"), bk, crN(u.credits), [vs(u.credits, V.of("credits", pu && pu.credits), r1)].concat(bk.part ? pace(u.credits, bk.v, r1, crN) : t("Kiro's own count, not an estimate"), me), "credits"); }
  if (u.tokens){ const bt = insBase("tokens"); if (bt) tile("cost", "tokens", t("Tokens"), bt, tok(u.tokens), [vs(u.tokens, V.of("tokens", pu && pu.tokens), tok), `${t("output")} ${tok(u.out)} ÷ ${tok(u.tokens)} = ${Math.round(u.out * 100 / u.tokens)}%`], "tokens"); }
  const ba = insBase("active"); if (ba) tile("cost", "active", t("Active time"), ba, fmtM("active", ba.cur), [vs(ba.cur, V.of("active", pw && pw.active), v => dur(v)), ba.part ? pace(ba.cur, ba.v, v => dur(v)) : ""], "active");
  // ── 出力：コミット・push・PR（どれも Git の側。AI の編集の行は下の Outputs のカード） ──
  const g = w && w.git && w.git.commits ? w.git : null, o = (w && w.outputs) || {};
  const nps = pushesIn(ws, we);
  if (g) tile("out", "commit", t("Git commits"), insBase("gitCommits"), `${g.commits}`, [{f: `AI ${g.ai} ÷ ${g.commits} = ${Math.round(g.ai * 100 / g.commits)}%`, p: t("{pc}% by AI", {pc: Math.round(g.ai * 100 / g.commits)})}, vsk("gitCommits", g.commits, "commits")], "commits");
  else if (o.commits) tile("out", "commit", t("AI commits"), false, `${o.commits}`, {n: t("Commits AI ran, from history")}, "aiCommits");
  const bpu = insBase("pushes");
  if (bpu && seen("pushes")){ const ps = (META.push || []).filter(x => x.t >= ws && x.t < we), sent = ps.reduce((a, x) => a + (x.commits || 0), 0), pb = periodBack(1);
    const prev = V.partial || !pb.S ? null : pushesIn(pb.ws, V.n == null ? pb.we : Math.min(pb.we, addDays(new Date(pb.ws * 1000), V.n).getTime() / 1000)); // 途中の期間は、前の期間の同じ日まで
    tile("out", "push", t("Pushes"), bpu, commas(ps.length), [insAg ? {n: t("Pushes count every agent: Git doesn't record which agent's work a push sent")} : "", sent ? {n: t("{c} sent, including commits made before the period", {c: plural(sent, "commit")})} : "", vsInt("pushes", ps.length, prev)], "", "pushes"); }
  const bpr = insBase("prs"); if (bpr && (o.prs || seen("prs"))) tile("out", "pr", t("Pull requests"), bpr, `${o.prs || 0}`, [{n: t("Created by AI")}, vsk("prs", o.prs || 0)], o.prs ? "prs" : "", "prs");
  // ── 単価：量（費用・クレジット・トークン）÷ 出力（コミット・push・PR）。割る範囲を名前に入れ、「分子 ÷ 分母」をいつも値の下に出す ──
  const UL = {cost: [t("Estimated cost per push"), t("Estimated cost per pull request"), usd, t("Estimated cost per Git commit")], credits: [t("Kiro credits per push"), t("Kiro credits per pull request"), cr1, t("Kiro credits per Git commit")], tokens: [t("Tokens per push"), t("Tokens per pull request"), tok, t("Tokens per Git commit")]};
  // コミットあたり：AI が実行したコミットがあれば、費用とトークンは Claude Code の分 ÷ AI のコミット（どのコミットを実行したかを記録するのは Claude Code だけで、こちらがより正確）。
  // それ以外（クレジットと、AI のコミットがない期間の費用・トークン）は、期間の合計 ÷ Git のコミット（出力のパネルと同じ数）。Git のコミットは AI の分だけを取り出せないので、そう書き添える
  const bp = insBase("costPerCommit"), oc = w && w.outCost, nc = o.commits, gc = g ? g.commits : 0;
  const perGit = kind => { const k = kind + "PerGitCommit", b = insBase(k); if (!b || !gc) return; const fu = UL[kind][2], used = usedOf(w, kind), den = plural(gc, "Git commit");
    tile("unit", `${kind}:commit`, UL[kind][3], b, kind === "credits" ? crR(b.cur) : fmtM(k, b.cur), [{f: `${t("total")} ${fu(used)} ÷ ${den} = ${fmtM(k, b.cur)}`, p: `${fu(used)} ÷ ${den}`}, {n: t("Git commits can't be split into AI's share, so this divides the period's whole total")}, vsk(k, b.cur)], "", "perGitCommit"); };
  if (bp && oc && nc) tile("unit", "cost:commit", t("Claude Code cost per AI commit"), bp, fmtM("costPerCommit", bp.cur), [{f: `${usd(oc)} ÷ ${nc} ${t("AI commits")} = ${usd(bp.cur)}`, p: `${usd(oc)} ÷ ${nc} ${t("AI commits")}`}, vsk("costPerCommit", bp.cur)], "costPerCommit");
  else perGit("cost");
  // トークンも金額と同じく（Claude Code のセッションのトークン ÷ AI のコミット）
  { const bt = insBase("tokensPerCommit"), ot = DATA.filter(s => s.source === "Claude Code" && s.start >= ws && s.start < we).reduce((a, s) => a + sesTok(s), 0);
    if (bt && ot && nc) tile("unit", "tokens:commit", t("Claude Code tokens per AI commit"), bt, fmtM("tokensPerCommit", bt.cur), [{f: `${tok(ot)} ÷ ${nc} ${t("AI commits")} = ${tok(bt.cur)}`, p: `${tok(ot)} ÷ ${nc} ${t("AI commits")}`}, vsk("tokensPerCommit", bt.cur)]);
    else perGit("tokens"); }
  perGit("credits");
  // push・PR は AI の分だけを取り出せないので、期間の量の合計で割る（そう書き添える）
  // エージェントで絞っているときは、push あたりを出さない（push はどのエージェントの分かわからないので、1 つのエージェントの量 ÷ 全員の push になってしまう）。PR は作ったエージェントの分だけを数える
  for (const [d, n, sfx, help, note] of [["push", insAg ? 0 : nps, "PerPush", "perPush", t("Pushes can't be split into AI's share, so this divides the period's whole total")], ["pr", o.prs, "PerPr", "perPr", t("Pull requests can't be split into AI's share, so this divides the period's whole total")]])
    for (const kind of ["cost", "credits", "tokens"]){
      const k = kind + sfx, b = insBase(k); if (!b || !n) continue; const fu = UL[kind][2], used = usedOf(w, kind), den = d === "push" ? plural(n, "push", "pushes") : plural(n, "PR");
      tile("unit", `${kind}:${d}`, UL[kind][d === "push" ? 0 : 1], b, kind === "credits" ? crR(b.cur) : fmtM(k, b.cur), [{f: `${t("total")} ${fu(used)} ÷ ${den} = ${fmtM(k, b.cur)}`, p: `${fu(used)} ÷ ${den}`}, {n: note}, vsk(k, b.cur)], "", help); }
  // AI が編集した 100 行あたり（出力単価）と、稼働 1 時間・稼働 1 日・セッション・プロンプトあたり（投入単価）。どれも、割る量を記録したセッションだけで割る
  { const FU = {cost: usd, credits: cr1, tokens: tok};
    for (const by of ["Edit", "Hour", "Day", "Ses", "Ask"]) for (const kind of ["cost", "credits", "tokens"]){
      const k = kind + "Per" + by, b = insBase(k); if (!b) continue;
      const e = by === "Edit" ? editsBasis(ws, we, kind) : useBasis(ws, we, kind), fu = FU[kind], v = fmtM(k, b.cur);
      const den = by === "Edit" ? t("{n} AI-edited lines", {n: commas(e.lines)}) : by === "Hour" ? dur(e.min) : by === "Day" ? t("{n} active days", {n: e.days}) : by === "Ses" ? t("{n} sessions", {n: e.n}) : plural(e.prompts, "prompt");
      const f = by === "Edit" ? `${fu(e.used)} ÷ ${den} × 100 = ${v}` : `${fu(e.used)} ÷ ${den} = ${v}`, plain = by === "Edit" ? `${fu(e.used)} ÷ ${den} × 100` : by === "Hour" ? `${fu(e.used)} ÷ ${t("{d} of session time", {d: dur(e.min)})}` : `${fu(e.used)} ÷ ${den}`;
      tile(by === "Edit" ? "unit" : "uin", `${kind}:${by === "Edit" ? "edit" : by}`, insName(k), b, kind === "credits" ? crR(b.cur) : v, [{f, p: plain}, by === "Edit" ? "" : {fn: t("Each divides only by the sessions that record what it divides")}, vsk(k, b.cur)], k === "costPerAsk" ? "costPerAsk" : "", k); } } // プロンプトあたりの費用は、押すと内訳
  // 切り替え：前に選んだもの（記録がなければ、記録のある最初のもの）を見せる。記録のない選択肢は出さない
  const NAME = {cost: t("Cost"), credits: t("Credits"), tokens: t("Tokens"), active: t("Active time"), commit: t("Commits"), push: t("Pushes"), pr: t("PRs"), edit: t("100 AI lines"), Hour: t("Active hour"), Day: t("Active day"), Ses: t("Session"), Ask: t("Prompt")};
  const pick = (sw, ks) => ks.includes(store.get(INS_SW[sw], "")) ? store.get(INS_SW[sw], "") : ks[0];
  const swH = (sw, label, ks, cur, dim = "n", hid = () => false) => insSwH(sw, label, ks.map(k => [k, NAME[k]]), cur, dim, hid);
  const now = soFar() != null ? (M ? t("This month") : t("This week")) : (M ? t("Month shown") : t("Week shown"));
  const limAny = limitHits(periodBack(INS_N).ws, periodBack(0).we).length > 0; // グラフの範囲に利用上限への到達があれば、凡例に赤い三角を足す
  // 推移のグラフの凡例は、パネルごとに 1 つ。グラフの下に置く（グラフの上に行を足して、隣のパネルのグラフより下がらないように）
  // 項目ごとに、グラフのどの印の説明か（data-lg）。見せているタイルのグラフにその印がなければ隠す（insLg。単価に推定・予測は描かない、など）
  const L1 = (cls, sel, text) => `<span class="lg ${cls}" data-lg="${sel}">${esc(text)}</span>`;
  // 量のパネル（投入・出力）は棒、単価のパネルは点と線（insSpark）。凡例の印もそれに合わせる
  const lgd = id => { const bar = !INS_RATE.has(id), est = M ? t("Month-end estimate") : t("End-of-week estimate");
    return `<p class="tlg" aria-hidden="true">${bar ? L1("bnow", "rect.sbar.cur", now) : L1("now", "circle.now", now)}${insFrac() < 1 ? (bar ? L1("bnow part", "rect.sbar.ext", est) + L1("prj cumj", "line.cprj", t("Projection")) : L1("now part", "circle.now.part", est) + L1("prj", "line.prj, line.cprj", t("Projection"))) : ""}${L1("band", "rect.band", t("Middle half of your past {unit}", {unit: insNoun()}))}${bar ? L1("brk", "path.sbrk", t("Cut bar: beyond the scale, with its amount on it")) : ""}${L1("med", "line.med", t("median"))}${id === "out" ? "" : L1("cum", "polyline.cuml", INS_RATE.has(id) ? t("Running average") : t("Cumulative (right axis)"))}${limAny && id === "cost" ? L1("lgl", "path.slim", t("Usage limit hits")) : ""}</p>`; };
  // パネル：頭（見出し・切り替え・注）、タイル（値の段・グラフ・印）、凡例。投入（cost・uin）は左の列、出力（out・unit）は右の列（INS_SIDE）
  const card = (id, label, sws, data, cur, items, note = "") => `<section class="ikg ik-${id} side-${INS_SIDE[id]}"${data} aria-label="${esc(label)}"><div class="ikhd"><span class="ikio" aria-hidden="true">${INS_SIDE[id] === "in" ? "IN" : "OUT"}</span><h5 class="ikgh">${esc(label)}${sws}</h5>${note}</div><div class="ikrow">${items.map(x => x.key === cur ? x.h : x.h.replace(`data-ch="${x.key}"`, `data-ch="${x.key}" hidden`)).join("")}</div>${lgd(id)}</section>`;
  // 選択肢はいつも同じ並び。数字のないものは出さない（覚えている選択が使えなければ、最初の使えるものに）
  const ORDER = {cost: ["cost", "credits", "tokens", "active"], out: ["commit", "push", "pr"]}, MS = ["cost", "credits", "tokens"];
  const one = (id, label) => { const ks = P[id].map(x => x.key); if (!ks.length) return ""; const cur = pick(id, ks);
    return card(id, label, swH(id, id === "out" ? INS_OUT() : t("Measure"), ORDER[id].filter(k => ks.includes(k)), cur), ` data-n="${cur}"`, cur, P[id]); };
  // 単価のパネルは、量と割る数の 2 つの切り替え。数字のある組だけを選べる（Kiro は PR を記録しないので、クレジットを選ぶと PR は出ない、など）。
  // 割る数は、どれかの量で数字のあるものだけを描き、今の量で数字のないものは隠す（量を切り替えたら bindInsight が付け替える）
  const two = (id, label, dOrder, note) => { if (!P[id].length) return "";
    const pairs = new Set(P[id].map(x => x.key)), ns = MS.filter(k => dOrder.some(d => pairs.has(`${k}:${d}`))), ds = dOrder.filter(d => ns.some(k => pairs.has(`${k}:${d}`)));
    const n = pick(id + "N", ns), d = insPickD(pairs, n, ds, store.get(INS_SW[id + "D"], ""));
    const br = `<span class="ikbr" aria-hidden="true"></span>`; // 広い画面の 2×2 では、量と割る数をそれぞれ 1 行に（選択肢の数で折り返す位置が変わり、横のパネルと頭の高さがずれないように）
    return card(id, label, br + swH(id + "N", t("Measure"), ns, n) + br + swH(id + "D", t("Divided by"), ds, d, "d", dd => !pairs.has(`${n}:${dd}`)), ` data-n="${n}" data-d="${d}" data-pairs="${[...pairs].join(" ")}"`, `${n}:${d}`, P[id], note); };
  const unit = two("unit", t("Unit cost per output"), ["commit", "push", "pr", "edit"]);
  // 「割る量を記録したセッションだけで割る」の注は、各タイルの式（ƒx）の中に。頭に置くと、横の出力単価の頭が 1 行ぶん空く（subgrid で行の高さをそろえるため）。push あたりの注（pushNote）も同じ
  const uin = two("uin", t("Unit cost per input"), ["Hour", "Day", "Ses", "Ask"]);
  const nb = insBase("active"), few = nb && nb.q1 == null ? `<p class="note insfew">${esc(t("To compare with your usual range, kiroku needs {min} past {unit} with records (have {n}).", {min: INS_MIN, unit: insUnit(INS_MIN), n: nb.n}))}</p>` : ""; // 比べられないことは、数字ごとでなく 1 度だけ言う
  return `${few}<div class="insh"><span class="insio in" aria-hidden="true">IN</span><span class="insdiv" aria-hidden="true"></span><span class="insio out" aria-hidden="true">OUT</span>${one("cost", INS_IN())}${one("out", INS_OUT())}${uin}${unit}</div><p class="ifxs insdef">${fx(insDef())}</p>`;
}
const tickM = (k, v) => /^credits/.test(k) ? crN(v) : fmtM(k, v);
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
  const card = (title, body, cls = "") => body ? `<div class="inscol${cls}"><h4>${esc(title)}</h4>${body}</div>` : "";
  const col = b => b ? `<div class="inscol">${b}</div>` : "";
  // 内訳：上のサンキーとパネルにない数字と図だけ（出力の内訳・どこに使ったか・時間帯・前の期間からの増減・トークンの種類・エージェント別・よく変えたファイル・作業の進め方）。
  // 先頭は背の高い出力のカード。広い画面では 1 行に 3 枚、狭くなると 2 枚・1 枚（.insrow。各カードは自分の高さのまま詰める）
  const cards = [[INS_OUT(), card(INS_OUT(), insOutputs(), " iout wide")], [t("Where it went"), col(insWhere())], [t("When you worked"), card(t("When you worked"), insHeat())],
    [t("What changed from the last period"), col(insWater())], [t("Tokens by type"), card(t("Tokens by type"), insTokens())],
    [t("By agent"), card(t("By agent"), insAgents(), " agw")], [t("Most edited files"), card(t("Most edited files"), (insEditsBody() || {}).files)],
    [t("How you worked"), card(t("How you worked"), insWork(), " iwork")]].filter(c => c[1]);
  // 内訳は、たたんだ箱で始める（ページを短く。開いたかは見る人ごとに覚える）。見出しの横に、中にあるカードの名前を 1 行で
  const body = cards.length ? `<details class="insbrk"${store.get("insBrk", false) ? " open" : ""}><summary class="inssec"><span>${esc(t("Breakdown"))}</span> <small>${esc(cards.map(c => c[0]).join(LSEP()))}</small></summary><div class="insg"><div class="insrow">${cards.map(c => c[1]).join("")}</div></div></details>` : "";
  return `<section class="insight" aria-labelledby="ins-h"><h3 id="ins-h" class="sr">${esc(t("Overview"))}</h3>${(b => b ? `<div class="inscol iskw">${b}</div>` : "")(insSankey())}${insHeadlines(F)}
    <div class="insg">${body}</div></section>`;
}
// skBack は、サンキーの名前（エージェント・プロジェクト・成果）の後ろに、カードの地の色の丸い札を敷く（色の帯の上に字が直接乗らないように）。
// 字の幅は書体と大きさ（狭いカードでは大きくする）で変わるので、描いたあとに測って置き、画面の幅が変わったら置き直す
function skBack(R){ (R || document).querySelectorAll(".isk svg").forEach(svg => { svg.querySelectorAll("rect.skbg").forEach(x => x.remove());
  svg.querySelectorAll("text.skt").forEach(tx => { if (!tx.checkVisibility()) return; const b = tx.getBBox(); if (!b.width) return;
    const r = document.createElementNS("http://www.w3.org/2000/svg", "rect"); r.setAttribute("class", "skbg");
    r.setAttribute("x", (b.x - 4).toFixed(1)); r.setAttribute("y", (b.y - 1).toFixed(1)); r.setAttribute("width", (b.width + 8).toFixed(1)); r.setAttribute("height", (b.height + 2).toFixed(1)); r.setAttribute("rx", "4");
    tx.before(r); }); }); }
addEventListener("resize", () => { clearTimeout(skBack.h); skBack.h = setTimeout(() => skBack($("#review")), 150); });
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
// insLg は、パネルの凡例を、見せているタイルのグラフが描いた印の項目だけにする（描いていない推定・予測・上限の印などを並べない）
function insLg(sec){ const tl = [...sec.querySelectorAll(".ikrow > .itile")].find(x => !x.hidden), svg = tl && tl.querySelector(".tspk svg");
  sec.querySelectorAll(".tlg [data-lg]").forEach(x => { x.hidden = !svg || !svg.querySelector(x.dataset.lg); }); }
function bindInsight(R){
  // パネルの切り替え：描き直さずに、選んだ選択肢のタイルだけを見せる（フォーカスは押したボタンに残る）。選んだものは覚える
  // 量と割る数の 2 つがあるときは、量を変えたら、その量で数字のない割る数を隠す（今の割る数がなければ、覚えている割る数か最初のものに）
  R.querySelectorAll(".iksw [data-v]").forEach(b => b.onclick = () => { const g = b.closest(".iksw"), sec = b.closest(".ikg"), sw = g.dataset.sw, dim = g.dataset.dim || "n";
    sec.dataset[dim] = b.dataset.v; store.set(INS_SW[sw], b.dataset.v);
    g.querySelectorAll("[data-v]").forEach(x => x.setAttribute("aria-pressed", String(x === b)));
    const dg = sec.querySelector('.iksw[data-dim="d"]');
    if (dim === "n" && sec.dataset.pairs && dg){ const pairs = new Set(sec.dataset.pairs.split(" ")), bs = [...dg.querySelectorAll("[data-v]")];
      bs.forEach(x => { x.hidden = !pairs.has(`${sec.dataset.n}:${x.dataset.v}`); });
      const d = insPickD(pairs, sec.dataset.n, bs.map(x => x.dataset.v), pairs.has(`${sec.dataset.n}:${sec.dataset.d}`) ? sec.dataset.d : store.get(INS_SW[dg.dataset.sw], ""));
      sec.dataset.d = d; bs.forEach(x => x.setAttribute("aria-pressed", String(x.dataset.v === d))); }
    const key = sec.dataset.d ? `${sec.dataset.n}:${sec.dataset.d}` : sec.dataset.n;
    sec.querySelectorAll("[data-ch]").forEach(x => { x.hidden = x.dataset.ch !== key; }); insLg(sec); });
  R.querySelectorAll(".insh .ikg").forEach(insLg);
  R.querySelectorAll(".iwlink").forEach(b => b.onclick = () => insOpenWorth(b.dataset.goto, b)); // 見直す候補の 1 行
  R.querySelectorAll("details.insbrk").forEach(d => d.ontoggle = () => store.set("insBrk", d.open)); // 「内訳」を開いたかを覚える
  // 式は、指標名の横の「ƒx」から、その数字（タイル・枠・グラフのカード）ごとに開く。ふだんは平易な 1 行だけを見せる
  R.querySelectorAll(".insight .itile, .insight .stat, .insight .mrow, .insight .inscol").forEach(el => {
    const own = [...el.querySelectorAll(".fxo, .ichip .cx")].some(f => f.closest(".itile, .stat, .mrow, .inscol") === el); if (!own) return;
    const lab = el.matches(".itile") ? el.querySelector(".il") : el.matches(".stat") ? el.querySelector(".k > span") : el.matches(".mrow") ? el.querySelector(".mk > span") : (el.querySelector(":scope > .inshead > .skil") || el.querySelector(":scope > h4, :scope > .inshead > h4, :scope h4")); if (!lab) return;
    const lc = lab.cloneNode(true); lc.querySelectorAll(".hb, .fxb, .fmark").forEach(x => x.remove()); // 読み上げの名前に「?」や印の文字を混ぜない
    const name = (lc.textContent || "").trim(), b = document.createElement("button");
    b.type = "button"; b.className = "fxb"; b.textContent = "ƒx"; b.setAttribute("aria-pressed", "false");
    b.title = t("Show the formula"); b.setAttribute("aria-label", t("Show the formula: {name}", {name}));
    b.onclick = e => { e.stopPropagation(); const on = el.classList.toggle("fxon"); b.setAttribute("aria-pressed", String(on)); b.title = on ? t("Hide the formula") : t("Show the formula"); };
    lab.append(b); }); // 名前の要素の中に置く（カードは縦の flex なので、外に置くと横いっぱいの帯になる）
  requestAnimationFrame(() => skBack(R));
  insPack(R);
  for (const [attr, key] of [["skk", "insSk"]]) R.querySelectorAll(`[data-${attr}]`).forEach(b => b.onclick = () => { st[key] = b.dataset[attr]; store.set(key, st[key]); summary(); const nb = $("#review").querySelector(`[data-${attr}="${CSS.escape(b.dataset[attr])}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-wfk]").forEach(b => b.onclick = () => { st.insWf = b.dataset.wfk; store.set("insWf", st.insWf); summary(); const nb = $("#review").querySelector(`[data-wfk="${CSS.escape(b.dataset.wfk)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-wk]").forEach(b => b.onclick = () => { st.insWk = b.dataset.wk; store.set("insWk", st.insWk); summary(); const nb = $("#review").querySelector(`[data-wk="${CSS.escape(b.dataset.wk)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll("[data-where]").forEach(b => b.onclick = () => { st.insW = b.dataset.where; store.set("insW", st.insW); summary(); const nb = $("#review").querySelector(`[data-where="${CSS.escape(b.dataset.where)}"]`); if (nb) nb.focus(); });
  R.querySelectorAll(".iwses[data-sid]").forEach(b => b.onclick = () => select(b.dataset.sid)); // いちばん変えたファイル：編集の回数から、そのファイルをいちばん変えたセッションを開く
  R.querySelectorAll(".insfig [data-sid]:not(.iwses)").forEach(c => { c.onclick = () => select(c.dataset.sid); c.onkeydown = e => { if (e.key === "Enter" || e.key === " "){ e.preventDefault(); select(c.dataset.sid); } }; }); // 散布図の点は、そのセッションを開く
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

/* どこに使ったか：プロジェクト・エージェント・モデルごとの費用（なければクレジット・トークン・時間）を、大きい順の横棒で。上位 7 つとその他 */
function insWhere(){
  const {S: w} = period(); if (!w) return "";
  const u = w.usage || {}, by = st.insW || "project";
  const src = by === "model" ? (u.models || []).map(r => ({name: r[0], cost: r[1], tokens: r[2], credits: r[4] || 0, c: "var(--acc)"}))
    : by === "source" ? ((w.shares || {}).source || []).map(x => ({name: x.key, cost: x.cost, credits: x.credits, tokens: x.tokens, minutes: x.minutes, c: agColor(x.key)}))
    : (w.projectStats || []).map(p => ({name: p.project, cost: p.cost, credits: p.credits, tokens: p.tokens, minutes: p.minutes, c: projColor(p.project)}));
  // 量は切り替えられる（推定費用・クレジット・トークン・時間のうち記録のあるもの）。モデル別は費用・トークンと、どのモデルで使ったかを記録したクレジット（Kiro）
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
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-wk="${x}" aria-pressed="${x === k}">${esc(INS_SW_NAME()[x === "minutes" ? "active" : x])}</button>`).join("")}</div>` : "";
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
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-wfk="${x}" aria-pressed="${x === k}">${esc(INS_SW_NAME()[x])}</button>`).join("")}</div>` : "";
  return `<div class="inshead"><h4>${esc(t("What changed from the last period"))}</h4>${mtabs}</div>${say(sy)}<figure class="insfig iwtf"><svg viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(t("What changed from the last period"))}">${ticks}${body}</svg><p class="ifxs">${fx(t("Each bar between the totals is one project's change; projects are grouped by where their sessions started"), "", t("By project · light: up · dark: down"))}${k === "minutes" ? INS_STIME() : ""}</p></figure>`;
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
// insPct は、量の並び xs の割合（整数の %）。最大剰余法で丸め、0 でないものの合計がちょうど 100 になるように（四捨五入だと 99 や 101 になる）
function insPct(xs){ const tot = xs.reduce((a, v) => a + Math.max(0, v), 0); if (!(tot > 0)) return xs.map(() => 0);
  const r = xs.map(v => Math.max(0, v) * 100 / tot), f = r.map(Math.floor); let left = 100 - f.reduce((a, v) => a + v, 0);
  r.map((v, i) => [v - f[i], i]).sort((a, b) => b[0] - a[0] || a[1] - b[1]).forEach(([, i]) => { if (left > 0){ f[i]++; left--; } }); return f; }
/* insSankey は、量（費用・クレジット・トークン・時間）がエージェント → プロジェクト → 出力（コミットか PR に届いたか）へどう流れたか。
   プロジェクトは多い順に 6 つまで、残りは「その他」。帯の太さは量に比例する */
function insSankey(){
  const {ws, we} = period(), ss = DATA.filter(s => s.start >= ws && s.start < we);
  const val = {cost: s => sesTok(s) ? s.cost || 0 : 0, credits: s => s.credits || 0, tokens: s => sesTok(s), minutes: s => minIn(s, s.start, s.end)};
  const ks = ["cost", "credits", "tokens", "minutes"].filter(k => ss.some(s => val[k](s) > 0)); if (!ks.length) return "";
  const k = ks.includes(st.insSk) ? st.insSk : ks[0], fv = {cost: usd, credits: cr, tokens: tok, minutes: v => dur(v)}[k];
  const xs = ss.map(s => ({s, v: val[k](s)})).filter(x => x.v > 0); if (xs.length < 2) return "";
  const pt = new Map(); xs.forEach(x => pt.set(x.s.project, (pt.get(x.s.project) || 0) + x.v));
  const topP = [...pt].sort((a, b) => b[1] - a[1]).slice(0, 6).map(x => x[0]), OTH = t("Other"), pj = p => topP.includes(p) ? p : OTH;
  // コミットか PR がなくても、ファイルを作ったり変えたりしたセッションは「ファイルの変更だけ」（Git には出ていないが、手が動いた分）。
  // どちらもなく、出力を記録しないエージェント（Claude Code 以外）のセッションは「記録なし」に（コミットがなかったとは言えない）
  const YES = t("Commit or PR"), FIL = t("Files changed only"), NO = t("None"), UNK = t("Not recorded");
  const out = s => commitsOf(s).length || (s.prs || []).length ? YES : (s.nFiles || 0) > 0 ? FIL : records(s.source, "outputs") ? NO : UNK;
  const cols = [[], [], []], vals = [new Map(), new Map(), new Map()], links = new Map();
  const add = (m, key, v) => m.set(key, (m.get(key) || 0) + v);
  xs.forEach(({s, v}) => { const a = s.source, b = pj(s.project), c = out(s); add(vals[0], a, v); add(vals[1], b, v); add(vals[2], c, v); add(links, `0\u0000${a}\u0000${b}`, v); add(links, `1\u0000${b}\u0000${c}`, v); });
  const ord = {[YES]: 0, [FIL]: 1, [NO]: 2, [UNK]: 3}; vals.forEach((m, i) => cols[i] = [...m].sort((a, b) => (i === 2 ? ord[a[0]] - ord[b[0]] : b[1] - a[1])));
  const shr = cols.map(c => { const ps = insPct(c.map(x => x[1])); return new Map(c.map(([key], j) => [key, ps[j]])); }); // 列ごとの割合（足して 100）
  // 名前と量は、帯に重ねない：エージェントは左の外、成果は右の外、プロジェクトは節の右に帯を通さない余白（LW）を空けて、そこに置く。
  // プロジェクトから成果への帯は、余白の右の細い節から出る。狭い画面（600px 以下）では図の文字を消し、同じ名前と量を図の下の一覧で見せる
  // サマリーの先頭の、幅いっぱいのカード（サマリーで唯一の全幅。期間の全体がどこへ流れたかを先に見せ、いつもとの比べは下のパネルで）。見出しは投入・出力のパネルと同じ段（.ikgh）で、その下に一段小さく図の名前（サンキーダイアグラム）と量の切り替え。列の間を広くとり、名前も長めに出す
  const tot = xs.reduce((a, x) => a + x.v, 0), H = 300, T = 14, B = 10, nw = 10, gap = 10, LW = 200, X = [170, 440, 440 + nw + LW, 860], W = X[3] + nw + 170;
  const pos = cols.map(c => { const g = (c.length - 1) * gap, sc = (H - T - B - g) / tot; let y = T; const m = new Map(); c.forEach(([key, v]) => { m.set(key, {y, h: Math.max(2, v * sc), o: 0, oi: 0}); y += v * sc + gap; }); return {m, sc}; });
  // 色：エージェントからの帯はエージェントの色、プロジェクトの節と、そこから出る帯はプロジェクトの色、成果はコミットの有無の色
  const color = (i, key) => i === 0 ? agColor(key) : i === 1 ? (key === OTH ? "var(--other)" : `var(--c${(topP.indexOf(key) + 3) % 8})`) : key === YES ? "var(--go)" : key === FIL ? "var(--go-2)" : key === NO ? "var(--ink-3)" : "var(--other)";
  const XO = [X[0] + nw, X[2] + nw]; // 帯の出る x（プロジェクトは余白の右の節から）
  const paths = [...links].sort((a, b) => b[1] - a[1]).map(([key, v]) => { const [ci, a, b] = key.split("\u0000"), i = +ci, A = pos[i].m.get(a), Bn = pos[i + 1].m.get(b), h = v * pos[i].sc;
    const y0 = A.y + A.o, y1 = Bn.y + Bn.oi; A.o += h; Bn.oi += h; const x0 = XO[i], x1 = i ? X[3] : X[1], xm = (x0 + x1) / 2;
    return `<path class="skl" data-l="${i}" data-a="${esc(a)}" data-b="${esc(b)}" d="M${x0} ${y0.toFixed(1)} C${xm} ${y0.toFixed(1)} ${xm} ${y1.toFixed(1)} ${x1} ${y1.toFixed(1)} L${x1} ${(y1 + h).toFixed(1)} C${xm} ${(y1 + h).toFixed(1)} ${xm} ${(y0 + h).toFixed(1)} ${x0} ${(y0 + h).toFixed(1)} Z" fill="${color(i, a)}"${tipAttr(`${a} → ${b}`, `${fv(v)} · ${Math.round(v * 100 / tot)}%`)}/>`; }).join("");
  // プロジェクトの列：左右 2 本の節の間（名前を置く余白）を、そのプロジェクトの色の帯で埋める（プロジェクトの部分が 1 本の色の帯に見えるように）。名前は帯の上の札に
  const pband = cols[1].map(([key, v]) => { const n = pos[1].m.get(key);
    return `<rect class="skp" data-p="${esc(key)}" x="${X[1] + nw}" y="${n.y.toFixed(1)}" width="${X[2] - X[1] - nw}" height="${n.h.toFixed(1)}" fill="${color(1, key)}"${tipAttr(key, `${fv(v)} · ${shr[1].get(key)}%`)}/>`; }).join("");
  const short = x => x.length > 22 ? x.slice(0, 21) + "…" : x, NX = [X[0], X[1], X[3]];
  // 名前の縦の位置：節の真ん中。小さな節が続いて名前が重なるときは、下へずらす（13 の間をあける）
  const ly = c => { let last = -Infinity; return c.map(n => { const y = Math.max(n.y + n.h / 2 + 4, last + 13); last = y; return y; }); };
  const nodes = cols.map((c, i) => { const ns = c.map(([key]) => pos[i].m.get(key)), ys = ly(ns), lx = i === 0 ? X[0] - 6 : i === 1 ? X[1] + nw + 6 : X[3] + nw + 6;
    return c.map(([key, v], j) => { const n = ns[j], r = (x, cls = "") => `<rect class="skn${cls}" x="${x}" y="${n.y.toFixed(1)}" width="${nw}" height="${n.h.toFixed(1)}" fill="${color(i, key)}"${tipAttr(key, `${fv(v)} · ${shr[i].get(key)}%`)}/>`;
      const mid = n.y + n.h / 2, ex = i === 0 ? X[0] : NX[i] + nw, lead = Math.abs(ys[j] - 4 - mid) > 2 ? `<path class="sklead" d="M${ex} ${mid.toFixed(1)} L${(i === 0 ? lx + 2 : lx - 2)} ${(ys[j] - 4).toFixed(1)}"/>` : ""; // ずらした名前は、節まで短い線でつなぐ
      return r(NX[i]) + (i === 1 ? r(X[2], " o") : "") + lead + `<text class="skt${i ? " s" : ""}" x="${lx}" y="${ys[j].toFixed(1)}">${esc(short(key))} <tspan class="skv">${esc(fv(v))}</tspan></text>`; }).join(""); }).join("");
  // 1 文で、全体がどこへ行ったかを言い切る（コミットか PR あり・なし・記録しないエージェントの分。0 のものは言わない）。割合は全体に対して、足して 100 になるように丸める
  const yes = vals[2].get(YES) || 0, fil = vals[2].get(FIL) || 0, no = vals[2].get(NO) || 0, unk = vals[2].get(UNK) || 0, mname = {cost: t("estimated cost"), credits: t("Kiro credits"), tokens: t("tokens"), minutes: t("session time")}[k];
  const pcs = insPct([yes, fil, no, unk]), parts = [];
  if (yes > 0) parts.push(t("{v} ({pc}%) went to sessions with a commit or PR", {v: fv(yes), pc: pcs[0]}));
  if (fil > 0) parts.push(t("{v} ({pc}%) to sessions that changed files without one", {v: fv(fil), pc: pcs[1]}));
  if (no > 0) parts.push(fil > 0 ? t("{v} ({pc}%) to sessions that changed no files", {v: fv(no), pc: pcs[2]}) : yes > 0 ? t("{v} ({pc}%) to sessions without", {v: fv(no), pc: pcs[2]}) : t("{v} ({pc}%) went to sessions without a commit or PR", {v: fv(no), pc: pcs[2]}));
  if (unk > 0) parts.push(t("{v} ({pc}%) is from agents that don't record commits", {v: fv(unk), pc: pcs[3]}));
  const sy = yes + fil + no <= 0 ? t("These agents don't record their commits, so it is not known which sessions led to one")
    : t("Of {tot} {metric}, {parts}", {tot: fv(tot), metric: mname, parts: JA ? parts.join("、") : parts.length < 3 ? parts.join(" and ") : `${parts.slice(0, -1).join(", ")}, and ${parts[parts.length - 1]}`});
  const mtabs = ks.length > 1 ? `<div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${ks.map(x => `<button type="button" data-skk="${x}" aria-pressed="${x === k}">${esc(INS_SW_NAME()[x])}</button>`).join("")}</div>` : "";
  const heads = [t("Agent"), t("Project"), INS_OUT()], hx = [X[0] + nw / 2, X[1] + nw / 2 + (nw + LW) / 2, X[3] + nw / 2];
  // 狭い画面の一覧：列ごとに、色の印・名前・量・割合（図の文字の代わり。読み上げもこちらで）
  const list = `<div class="sklist">${cols.map((c, i) => `<div class="skcol" role="group" aria-label="${esc(heads[i])}"><h6 class="iksh">${esc(heads[i])}</h6><ul>${c.map(([key, v]) => `<li><i style="background:${color(i, key)}"></i><span class="skn2">${esc(key)}</span><span class="iwv">${esc(fv(v))}</span><b>${shr[i].get(key)}%</b></li>`).join("")}</ul></div>`).join("")}</div>`;
  return `<h4 class="ikgh skgh">${esc(t("From input to output"))}</h4><div class="inshead"><h5 class="skil">${esc(t("Sankey diagram"))}<span data-flag="heavy"></span></h5>${mtabs}</div>${say(sy)}<figure class="insfig isk"><svg viewBox="-24 -18 ${W + 24} ${H + 18}" role="img" aria-label="${esc(t("From input to output"))}">${heads.map((h, i) => `<text class="skh" x="${hx[i]}" y="-2">${esc(h)}</text>`).join("")}${paths}${pband}${nodes}</svg>${list}<p class="ifxs">${fx(t("Band width = the amount. A session has a commit or PR when it made one, or when a commit was made in its project from its start to 10 minutes after its end. A session of an agent that doesn't record its commits, with no commit found that way, is \"Not recorded\""), "", t("Band width is the amount"))}${k === "minutes" ? INS_STIME() : ""}</p></figure>`;
}
/* 数の枠（statH）に、いつもの範囲との不等式を添える。k は MET の指標名（なければ添えない） */
// 大きく外れた量は、値を中立の色にして札を添える（insOdd）。verd は評価の文（insVerdict）の色が付く数字で、印を重ねない
const insStat = (label, v, sub, help, metric, k, verd = false) => { const b = k && MET[k] ? insBase(k) : null, ob = verd ? null : b;
  return statH(esc(label), insOddV(ob, v), [sub, insChip(b)].filter(Boolean).join(" "), help, metric); };
/* insRow は、内訳のカードの 1 行（rowH）。short は値の横にいつも見せる短い添え書き（HTML）、fxs は「ƒx」で開く式（文字）と注（{n: 文字}）、
   k はいつもの範囲と比べる MET の指標（印を出し、範囲の不等式は「ƒx」に）、vs は前の期間との比べ（insCmp の {f, p}。ふだんは平易な形） */
const insMark = b => !b || b.q1 == null ? "" : `<span class="imk${insTone(b)}${b.pos === "typical" ? "" : " off"}${insOdd(b) ? " odd" : ""}">${insOdd(b) ? "" : `<span aria-hidden="true">${esc(b.pos === "high" ? t("Above usual") : b.pos === "low" ? t("Below usual") : t("As usual"))}</span>`}<span class="sr">${esc(insPlain(b))}</span></span>`;
const insRow = (label, v, {short = "", fxs = [], k = "", help = "", metric = "", vs = ""} = {}) => { const b = k && MET[k] ? insBase(k) : null;
  const mark = [short ? `<span class="mrs">${short}</span>` : "", insMark(b), vs ? fx(vs.f, "", vs.p) : ""].filter(Boolean).join("");
  const more = fxs.filter(Boolean).map(x => typeof x === "string" ? fx(x) : `<span class="fxo fxn">${esc(x.n)}</span>`).join("") + (b && b.q1 != null ? `<code class="ifx fxo">${esc(insExpr(b))}</code>` : "");
  return rowH(esc(label), insOddV(b, v), mark, more, help, metric); }; // 大きく外れた量は、値も中立の色に（印の札が色だけに頼らない目印）
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
  // 書いた行の定着：Claude Code のセッションで AI が足した行のうち、そのセッションの AI のコミットに入った割合
  const kp = keptOf(ws, we), bk = insBase("kept");
  const {vsk} = insCmp();
  const S = [insRow(t("Files edited by AI"), n(nf), {fxs: [{n: plural(xs.length, "session")}], help: "aiFiles"}),
    insRow(t("Lines edited by AI"), n(add + del), {short: lnPM(n(add), n(del)), fxs: [`+${n(add)} −${n(del)}; ${n(edits)} ${t("edits")}`], k: "aiLines", help: "aiLines"}),
    kp.edit && bk ? insRow(t("AI-added lines that reached a commit"), `${bk.cur}<small>%</small>`, {fxs: [`${n(kp.kept)} ÷ ${n(kp.edit)} = ${bk.cur}%`, {n: t("{kept} of {edit} lines · {n} Claude Code sessions", {kept: n(kp.kept), edit: n(kp.edit), n: kp.n})}, {n: t("{pc}% was rewritten or dropped", {pc: 100 - bk.cur})}], k: "kept", help: "kept", vs: vsk("kept", bk.cur)}) : ""].join("");
  // ファイル名はファイルそのものを開く（セッションの間のコミットのリンク、なければこの PC のファイル。セッション詳細の一覧と同じ）。
  // そのファイルをいちばん変えたセッションは、右端の編集の回数から開く
  const href = x => { const ss = DATA.find(s => s.id === x.sid); return ss ? fileLink(ss, x.f) || localHref(x.f, ss.projectPath) : ""; };
  // 同じファイルを Git のコミットで何回変えたか（AI と手）。「よく変えたファイル」を 1 つの表にまとめる
  const gc = new Map(); gitIn(ws, we).forEach(c => (c.files || []).forEach(f => { const k = `${c.repo}/${f.path}`, z = gc.get(k) || {n: 0, ai: 0}; z.n++; if (c.ai) z.ai++; gc.set(k, z); }));
  const gcOf = x => gc.get(x.f) || null;
  const rows = top.map(x => `<div class="iwr"><span class="iwn iwf" title="${esc(x.f)}">${multi ? `<small>${esc(x.project)}</small> ` : ""}${fileA(href(x), esc(x.rel))}</span><span class="iwb"><span style="width:${((x.a + x.d) * 100 / w0).toFixed(1)}%;background:var(--acc)"></span></span><span class="iwv">+${n(x.a)} −${n(x.d)}${(z => z ? ` · ${esc(t("{c}, {ai} by AI", {c: plural(z.n, "commit"), ai: z.ai}))}` : "")(gcOf(x))}</span><button type="button" class="iwses" data-sid="${esc(x.sid)}" title="${esc(t("Open the session that changed it most"))}">${esc(t("{n}×", {n: x.n}))}</button></div>`).join("");
  return {stats: S, files: `${say(t("{file} changed the most: {lines} lines in {n} edits", {file: top[0].rel, lines: n(top[0].a + top[0].d), n: n(top[0].n)}))}<figure class="insfig"><div class="iwrows" role="table" aria-label="${esc(t("Most edited files"))}"><div class="iwr iwh" aria-hidden="true"><span>${esc(t("File"))}</span><span></span><span class="iwv">${esc(t("Lines · Git commits"))}</span><span>${esc(t("Edits"))}</span></div>${rows}</div><p class="ifxs">${fx(t("+ − = lines added and removed, compared before and after each edit; × = edits"), "", t("Lines: before and after each edit, an estimate"))}</p></figure>`, nf, add, del};
}
/* 出力：コミット（うち AI）・変えた行・PR・出力まで届いたセッション・コミットあたりの費用。割った指標は式で */
function insOutputs(){
  const w0 = period(), {S: w} = w0; if (!w) return "";
  const o = w.outputs || {commits: 0}, g = w.git && w.git.commits ? w.git : null, base = w.outBase ?? w.sessions, n = commas, E = insEditsBody() || null;
  if (!o.commits && !g && !o.prs && !E) return "";
  const lines = g ? g.added + g.removed : 0, sp = base ? Math.round(w.outSessions * 100 / base) : null;
  // コミット・push・PR の数は、上の「出力」のパネルにある（ここでは繰り返さない）。ここには、パネルにない Git の数だけ
  const r = reworkOf(w0.ws, w0.we), br = insBase("rework"), {vsk, vsInt, V} = insCmp(), pb = periodBack(1);
  // Git の行の前の期間との比べ：途中の期間は、前の期間の同じ日まで（日ごとの行の数はないので、コミットから数える。上の帯と同じ比べ方）
  const pl = !g || V.partial || !pb.S || (insAg && V.n != null) ? null : V.n == null ? metricAt("gitLines", pb) : gitIn(pb.ws, Math.min(pb.we, addDays(new Date(pb.ws * 1000), V.n).getTime() / 1000)).reduce((a, c) => a + (c.added || 0) + (c.removed || 0), 0);
  // 1 行ずつの一覧（名前・値・いつもの範囲の印・前の期間との比べ）。長い説明と式は、その行の「ƒx」と「?」に
  const S = [
    g && lines ? insRow(t("Lines in Git commits"), n(lines), {short: lnPM(n(g.added), n(g.removed)), fxs: [`+${n(g.added)} −${n(g.removed)}; ${n(lines)} ÷ ${g.commits} = ${n(Math.round(lines / g.commits))} ${t("per commit")}`], k: "gitLines", help: "lines", metric: "lines", vs: vsInt("gitLines", lines, pl)}) : "",
    ...gitMore(),
    // AI のコミットのやり直し：3 日以内に同じファイルがまた変えられた割合（3 日たったコミットだけ）
    r.n && br ? insRow(t("AI commits changed again within 3 days"), `${br.cur}<small>%</small>`, {fxs: [`${r.re} ÷ ${plural(r.n, "AI commit")} = ${br.cur}%`, {n: t("Lower is usually better: fewer files needed another pass")}], k: "rework", help: "rework", vs: vsk("rework", br.cur)}) : "",
    base ? insRow(t("Sessions that reached a commit or PR"), `${sp}<small>%</small>`, {fxs: [`${w.outSessions} ÷ ${base} ${t("Claude Code sessions")} = ${sp}%`], k: "outSessions", help: "outSessions", metric: "outSessions"}) : "",
  ].join("");
  // 読み取りの 1 文も、Git と AI の編集で列ごとに分ける（1 つの文につなぐと、2 つを足した数に読めてしまう）
  const syG = g ? t("{c} commits, {ai}% by AI; {sp}% of Claude Code sessions reached a commit or PR", {c: g.commits, ai: Math.round(g.ai * 100 / g.commits), sp: sp ?? "—"}) : o.commits ? t("{c} AI commits; {sp}% of Claude Code sessions reached a commit or PR", {c: o.commits, sp: sp ?? "—"}) : "";
  const syE = E ? t("AI edited {n} files (+{a} −{d} lines)", {n: n(E.nf), a: n(E.add), d: n(E.del)}) : "";
  // Git（リポジトリ）と AI の編集（履歴）は元が違うので、列と小見出しで分け、足したり割ったりして混ぜない
  const col = (cls, icon, h, note, body) => `<div class="iosec ${cls}" role="group" aria-label="${esc(h)}"><h6 class="iksh">${ico(icon)}${esc(h)} <small>${esc(note)}</small></h6>${body}</div>`;
  return `<div class="iout2">${col("outGit", "commit", t("Git"), t("from your repositories"), S ? `${say(syG)}<div class="mrows">${S}</div>` : noneH(esc(t("No Git commits for this selection"))))}${E ? col("outEdit", "edit", t("AI edits"), t("from the agents' history, committed or not"), `${say(syE)}<div class="mrows">${E.stats}</div>`) : ""}${S ? `<div class="iokinds">${insKinds()}</div>` : ""}</div>`; // 作業の種類は 2 列の下に幅いっぱいで（Git の列にだけ置くと、左の列が長くなって右の列の下が空く）
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
/* Git のほかの出力（ファイル・リポジトリ・コミットのあった日・コミットから push まで）。どれもリポジトリから。途中の期間は、今日までの日数で割る */
function gitMore(){
  const {S: w, ws, we} = period(), g = w && w.git && w.git.commits ? w.git : null; if (!g) return [];
  const cs = gitIn(ws, we), n = commas, out = [], fs = new Set(); let more = 0;
  cs.forEach(c => { (c.files || []).forEach(f => fs.add(`${c.repo}\u0000${f.path}`)); more += Math.max(0, (c.nFiles || 0) - (c.files || []).length); });
  if (fs.size + more) out.push(insRow(t("Files in Git commits"), n(fs.size + more), {fxs: [{n: more ? t("includes about {n} from big commits that keep only their first files", {n: n(more)}) : t("Each file once")}]}));
  const rp = [...new Set(cs.map(c => c.project))], dd = new Set(cs.map(c => key(new Date(c.t * 1000)))), sf = soFar(), nd = sf == null ? Math.round((we - ws) / 86400) : sf;
  if (rp.length) out.push(insRow(t("Repositories with commits"), `${rp.length}`, {fxs: [{n: rp.join(LSEP())}]}));
  if (dd.size) out.push(insRow(t("Days with commits"), `${dd.size}`, {short: esc(sf == null ? t("of {n}", {n: plural(nd, "day")}) : t("of {n} so far", {n: plural(nd, "day")})), fxs: [`${dd.size} ÷ ${nd}${t("d")}`]}));
  const ps = (META.push || []).filter(x => x.t >= ws && x.t < we); // push の数は上の「出力」のパネルに。ここでは、コミットから push までの時間だけ
  // 時間の目安（中央値）：セッションを始めてから、そのセッションの最初の AI のコミットまで。コミットしてから、それを送った push まで
  const first = new Map(); (META.git || []).forEach(c => { if (c.ai && c.session && (!first.has(c.session) || c.t < first.get(c.session))) first.set(c.session, c.t); });
  const tf = DATA.filter(s => s.start >= ws && s.start < we && first.has(s.id)).map(s => (first.get(s.id) - s.start) / 60).filter(m => m >= 0);
  if (tf.length) out.push(insRow(t("From session start to its first commit"), esc(dur(Math.round(insQuant(tf, .5)))), {fxs: [{n: t("median of {n}", {n: plural(tf.length, "session")})}]}));
  const byHash = new Map((META.git || []).map(c => [c.hash, c])), tp = [];
  ps.forEach(p => (p.hashes || []).forEach(h => { const c = byHash.get(h); if (c && p.t >= c.t) tp.push((p.t - c.t) / 60); }));
  if (tp.length) out.push(insRow(t("From commit to push"), esc(dur(Math.round(insQuant(tp, .5)))), {fxs: [{n: t("median of {n}", {n: plural(tp.length, "commit")})}]}));
  return out;
}
/* insUseStats は、作業の進め方のカードに足す AI の使い方の数字：キャッシュからの読み込み・プロンプト・サブエージェント・前の期間に収める 1 日の目安。
   トークンと月末の推定は上の投入のパネルにあるので、ここには出さない */
function insUseStats(w, ws, we){
  const u = w.usage || {}, N = !u.subagents && unrecorded(ws, we, "subagents");
  return [
    u.tokens ? insStat(t("Read from cache"), u.cacheHit == null ? "—" : `${Math.round(u.cacheHit * 100)}<small>%</small>`, fx(t("cache reads ÷ input"), "", t("Share of input tokens")), "cache", u.cacheHit == null ? "" : "cache", "cache") : "",
    insStat(t("Prompts"), commas(w.prompts), "", "prompts", "sessions", "prompts"),
    N ? norecStat(esc(t("Subagents")), N, "subagents") : insStat(t("Subagents"), `${u.subagents || 0}`, u.subagents ? esc(t("Total run time {d}", {d: dur(u.subMin)})) + (u.subTypes.length ? " · " + esc(u.subTypes.slice(0, 3).map(([k, v]) => `${k} ${v}`).join(", ")) : "") : esc(t("Not used")), "subagents", u.subagents ? "subagents" : ""),
    ...insPace(w, projection(w)),
  ];
}
/* 前の期間の合計に収める 1 日の目安（途中の週、月末の推定がまだない途中の月）。費用とクレジットのそれぞれに：残りの日 1 日あたりの目安か、すでに超えた額（式で） */
function insPace(w, pj){
  const pc = pj && (pj.cost != null || pj.credits != null) ? null : pacing(w, period().P); if (!pc) return []; // 月末の推定があれば、そちら（上）で足りる
  const M = st.mode === "month";
  return ["cost", "credits"].filter(k => pc[k]).map(k => { const x = pc[k], f = k === "credits" ? cr1 : usd, on = x.rest >= 0, C = k === "credits";
    const lab = on ? (C ? (M ? t("Credits a day to match last month") : t("Credits a day to match last week")) : (M ? t("Daily spend to stay at last month's total") : t("Daily spend to stay at last week's total")))
      : (C ? (M ? t("Over last month's credits") : t("Over last week's credits")) : (M ? t("Over last month's total") : t("Over last week's total")));
    return insStat(lab, esc(on ? `${f(x.per)}/${t("d")}` : `+${f(-x.rest)}`), fx(on ? `(${f(x.ref)} − ${f(x.so)}) ÷ ${pc.left}${t("d")} = ${f(x.per)}/${t("d")}` : `${f(x.so)} − ${f(x.ref)} = +${f(-x.rest)} ${t("over")}`), C ? "paceCr" : "pace", C ? "paceCr" : "pace"); });
}
/* 単価のカード：上の「単価」のパネルにない割り方（AI が編集した 100 行・稼働 1 時間・稼働 1 日・セッション・プロンプトあたり）。
   パネルと同じく、量（推定費用・クレジット・トークン）と割る数の 2 つの切り替えで 1 つの数字を見せる（値・式・いつもの範囲・評価・「?」）。選んだものは覚える。
   どれも、その量を記録したセッションだけで割る（記録のないエージェントで薄めない）。推移は「その他のグラフ」の Trend で見られる */
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
    // 数は、3 つ以上のセッションで書いた似たプロンプトの組の数。添えるのはいちばん多い組と、その回数（ダイアログの数と同じ）
    (rp => insStat(t("Repeated prompts"), `${rp.length}`, rp.length ? esc(t("{n} kinds; most: “{p}” {c}× in {s}", {n: rp.length, p: snipOf(rp[0].text || "", 40), c: rp[0].n, s: plural(rp[0].ids.size, "session")})) : "", "repeats"))(repeatsOf(ws, we)),
    ...insUseStats(w, ws, we),
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
