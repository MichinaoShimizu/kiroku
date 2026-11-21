// サマリーの頭：要点の文・推移のグラフ（自分のいつもの範囲つき）・改善のアドバイス。AI は使わず、決まった計算と規則だけで出す
/* いつもの範囲：表示中の期間の前の INS_N 期間（記録が始まる前の期間と、記録が途中から始まる期間は除く）の分布。
   中央値と四分位（真ん中の 50%）で見る。平均と標準偏差は、外れた週 1 つで大きく動くので使わない */
const INS_N = 12, INS_MIN = 4; // 比べる期間の数と、比べるのに要る最低の数（それより少なければ「まだ比べられない」と言う）
const INS_ADD = new Set(["cost", "active", "prompts", "tokens", "gitCommits"]); // 期間の長さに比例して増える指標（途中の期間は、このペースで期間を終えたときの値で比べる）
const INS_KEYS = ["cost", "active", "costPerCommit", "cache", "fix", "wait"]; // グラフで切り替えて見られる指標
const INS_SHORT = () => ({cost: t("Cost"), active: t("Time"), costPerCommit: t("Cost per commit"), cache: t("Cache"), fix: t("Corrections"), wait: t("Wait")}); // 切り替えの短い名前（正式な名前は title と、グラフの下の文）
// insNice は、目盛りの上端を切りのよい数（1・2・2.5・5・10 の 10 の累乗倍）に
function insNice(v){ if (!(v > 0)) return 1; const e = Math.pow(10, Math.floor(Math.log10(v))); return [1, 2, 2.5, 5, 10].map(m => m * e).find(x => x >= v); }
function insQuant(xs, q){ const s = [...xs].sort((a, b) => a - b), i = (s.length - 1) * q, lo = Math.floor(i), hi = Math.ceil(i); return s[lo] + (s[hi] - s[lo]) * (i - lo); }
// insFrac は、表示中の期間のうち過ぎた割合（終わった期間は 1）。日ごとではなく時刻で（月曜の朝に 1 日分と数えて大きく割らないように、最低 1 日分）
function insFrac(){ const {ws, we} = period(), now = nowMs() / 1000; return now >= we || now < ws ? 1 : Math.min(1, Math.max(86400, now - ws) / (we - ws)); }
// insBase は、指標 k のいつもの範囲と、表示中の期間の位置。v は比べる値（途中の期間の、比例する指標はペース換算）
function insBase(k){
  const cur = metricAt(k, periodBack(0)); if (cur == null) return null;
  return insRange(k, cur, p => metricAt(k, p), !!MET[k].low, INS_ADD.has(k), v => fmtM(k, v));
}
// insRange は、値の関数 f（期間 → 値）で、表示中の期間の値 cur をいつもの範囲と比べる。MET にない値（勤務時間外の割合など）にも使う
function insRange(k, cur, f, low, add, fmt){
  const fr = insFrac(), v = add && fr < 1 ? cur / fr : cur, hs = historyStart(), xs = [];
  for (let i = 1; i <= INS_N; i++){ const p = periodBack(i); if (p.ws < hs) continue; const x = f(p); if (x != null) xs.push(x); }
  const r = {k, cur, v, part: fr < 1 && add, n: xs.length, xs, fmt};
  if (xs.length < INS_MIN) return r;
  const q1 = insQuant(xs, .25), med = insQuant(xs, .5), q3 = insQuant(xs, .75), iqr = q3 - q1;
  const pos = v > q3 ? "high" : v < q1 ? "low" : "typical", far = v > q3 + 1.5 * iqr || v < q1 - 1.5 * iqr;
  const better = pos === "typical" ? null : (pos === "low") === low; // 範囲の外で、よい側か
  return {...r, q1, med, q3, pos, far, better, rank: Math.round(xs.filter(x => x < v).length * 100 / xs.length)};
}
const insName = k => (H()[k] || {}).n || k;
const insUnit = () => st.mode === "month" ? t("months") : t("weeks");
const say = (s, then) => (s ? `<p class="isay">${esc(s)}</p>` : "") + (then ? `<p class="ithen">→ ${esc(then)}</p>` : ""); // グラフの読み取り（データから計算した 1 文。見出しのすぐ下）
// 式の表示：値・式は <code class="ifx"> に入れる（文ではなく、計算だとわかるように）
const fx = (s, cls = "") => `<code class="ifx${cls}">${esc(s)}</code>`;
const insDef = n => n == null ? t("Q1, Q3 = 25th and 75th percentiles of the past {unit} with records (up to {n}); IQR = Q3 − Q1", {n: INS_N, unit: insUnit()}) : t("Q1, Q3 = 25th and 75th percentiles of the last {n} {unit}; IQR = Q3 − Q1", {n, unit: insUnit()});
// insExpr は、比べた結果を不等式で（x は表示中の期間の値。途中の期間の、期間に比例する指標はペース）
function insExpr(b){
  if (!b || b.q1 == null) return t("Q1–Q3: needs {min} past {unit} (have {n})", {min: INS_MIN, unit: insUnit(), n: b ? b.n : 0});
  const fm = b.fmt || (x => fmtM(b.k, x)), x = (b.part ? t("pace") + " " : "") + fm(b.v), iqr = b.q3 - b.q1;
  if (b.pos === "typical") return `Q1 ${fm(b.q1)} ≤ ${x} ≤ Q3 ${fm(b.q3)}`;
  if (b.pos === "high") return b.far ? `${x} > Q3 + 1.5×IQR = ${fm(b.q3 + 1.5 * iqr)}` : `${x} > Q3 = ${fm(b.q3)}`;
  return b.far ? `${x} < Q1 − 1.5×IQR = ${fm(b.q1 - 1.5 * iqr)}` : `${x} < Q1 = ${fm(b.q1)}`;
}
const insPosText = b => `${insExpr(b)} (${insDef(b && b.n)})`; // 読み上げと title 用
const insTone = b => !b || b.q1 == null || b.better == null ? "" : b.better ? " good" : " bad";
const insChip = b => `<code class="ichip ifx${insTone(b)}" title="${esc(insDef(b && b.n))}">${esc(insExpr(b))}</code>`;
/* 要点：大きな数字と、その式・いつもの範囲との不等式（3 つ）。費用・成果あたりの費用（なければ稼働時間）・最初に見直すもの */
function insHeadlines(F){
  const {S: w, ws, we} = period(), T = [], tile = (label, b, v, f) => T.push(`<div class="itile${insTone(b)}"><span class="il">${esc(label)}</span><b>${esc(v)}</b>${f ? fx(f) : ""}${insChip(b)}</div>`);
  const bc = insBase("cost"), days = (we - ws) / 86400, el = insFrac() * days;
  if (bc) tile(t("Estimated cost"), bc, fmtM("cost", bc.cur), bc.part ? `${t("pace")} = ${usd(bc.cur)} × ${days}${t("d")} ÷ ${el.toFixed(1)}${t("d")} = ${usd(bc.v)}` : "");
  const bp = insBase("costPerCommit"), ba = insBase("active"), oc = w && w.outCost, nc = w && w.outputs && w.outputs.commits;
  if (bp) tile(t("Cost per commit"), bp, fmtM("costPerCommit", bp.cur), oc && nc ? `= ${usd(oc)} ÷ ${nc} ${t("commits")}` : "");
  else if (ba) tile(t("Active time"), ba, fmtM("active", ba.cur), "");
  T.push(F.length ? `<div class="itile bad"><span class="il">${esc(t("Change first"))}</span><b class="sm">${esc(insName(GOTO[F[0].k] || F[0].k))}</b>${fx(t("Threshold: {rule}", {rule: F[0].L.rule}))}</div>`
    : `<div class="itile good"><span class="il">${esc(t("Change first"))}</span><b class="sm">${esc(t("Nothing"))}</b>${fx(t("No metric crossed its threshold"))}</div>`);
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
  const lab = p => { const [yy, mm, dd] = p.key.split("-").map(Number); return st.mode === "month" ? MON[mm - 1] : dMD(new Date(yy, mm - 1, dd)); };
  const out = v => b && b.q1 != null && (v > b.q3 + 1.5 * (b.q3 - b.q1) || v < b.q1 - 1.5 * (b.q3 - b.q1));
  const n = ps.length, every = Math.ceil(n / (narrow ? 4 : 7)); // 期間の名前は間引く（右端の表示中の期間は必ず）
  const bars = ps.map(({p, v}, i) => { const last = i === n - 1, cx = x(i) + bw / 2;
    return `<g>${v == null ? "" : `<rect class="ibar${last ? " cur" : ""}" x="${(x(i) + bw * .18).toFixed(1)}" y="${y(v).toFixed(1)}" width="${(bw * .64).toFixed(1)}" height="${Math.max(0, Hh - B - y(v)).toFixed(1)}" rx="3"/>`}`
      + (last && proj != null ? `<rect class="iproj" x="${(x(i) + bw * .18).toFixed(1)}" y="${y(proj).toFixed(1)}" width="${(bw * .64).toFixed(1)}" height="${Math.max(0, Hh - B - y(proj)).toFixed(1)}" rx="3"/>` : "")
      + (v != null && !last && out(v) ? `<circle class="iout" cx="${cx.toFixed(1)}" cy="${(y(v) - 7).toFixed(1)}" r="3"/>` : "")
      + ((n - 1 - i) % every === 0 ? `<text class="ixl${last ? " cur" : ""}" x="${cx.toFixed(1)}" y="${Hh - 8}">${esc(lab(p))}</text>` : "")
      + `<rect class="ihit" x="${x(i).toFixed(1)}" y="0" width="${bw.toFixed(1)}" height="${Hh}"${tipAttr(p.label, v == null ? t("No records") : fmtM(k, v), last && proj != null ? t("At this pace: {v}", {v: fmtM(k, proj)}) : "")}/></g>`; }).join("");
  const list = ps.filter(x => x.v != null).map(x => `${x.p.label} ${fmtM(k, x.v)}`).join(LSEP());
  let sy = ""; if (b && b.q1 != null){ const all = [...b.xs, b.v], r = 1 + b.xs.filter(z => z > b.v).length, d = b.v - b.med, pc = b.med ? Math.round(d * 100 / b.med) : null;
    sy = t("{x}: {r} of {n} {unit} from the highest; {d} from the median {m}{pc}", {x: (b.part ? t("pace") + " " : "") + fmtM(k, b.v), r, n: all.length, unit: insUnit(), d: (d >= 0 ? "+" : "−") + fmtM(k, Math.abs(d)), m: fmtM(k, b.med), pc: pc == null ? "" : ` (${pc >= 0 ? "+" : ""}${pc}%)`}); }
  const th = !b || b.q1 == null ? "" : b.better === false ? t("Worse side of Q1–Q3; the advice lists where it came from") : b.better ? t("Better side of Q1–Q3; keep what you changed") : t("Within Q1–Q3 of your past {unit}", {unit: insUnit()});
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(t("{metric} over the last {n} {unit}", {metric: insName(k), n, unit: insUnit()}))}">${ticks}${band}${bars}</svg>
    <figcaption><span class="lg iband">${fx(`Q1–Q3`)} ${esc(t("of the last {n} {unit}", {n: b ? b.n : 0, unit: insUnit()}))}</span><span class="lg imed">${fx(t("median"))}</span>${proj != null ? `<span class="lg iproj">${fx(`${t("pace")} = ${t("so far")} × ${t("period")} ÷ ${t("elapsed")}`)}</span>` : ""}<span class="lg iout">${fx(`< Q1 − 1.5×IQR ${t("or")} > Q3 + 1.5×IQR`)}</span></figcaption>
    <p class="sr">${esc(list)} — ${esc(insPosText(b))}</p></figure>`;
}
/* 改善のアドバイス：見直す候補を効き目の大きい順に 3 つまで。何が・いつもと比べて・何をするか（指標の説明の「打てる手」）と、該当するセッション。
   よくなった指標も 1 つ返す（続けることがわかるように） */
function insAdvice(F){
  const items = F.slice(0, 3).map(f => { const id = GOTO[f.k] || f.k, b = MET[f.k] ? insBase(f.k) : null, h = H()[f.k] || H()[id] || {};
    return `<li class="insa" title="${esc(f.L.why)}"><div class="insw"><b>${esc(f.L.see)}</b>${b ? insChip(b) : ""}</div>${h.a ? `<p class="todo">${ico("ai", "ti")}${esc(h.a)}</p>` : ""}
      ${f.ids.length || f.k === "repeats" ? `<button class="pill insgo" type="button" data-goto="${esc(id)}">${esc(f.ids.length ? plural(f.ids.length, "session") : t("Details"))}</button>` : ""}</li>`; });
  const good = Object.keys(MET).map(k => insBase(k)).filter(b => b && b.better).sort((a, b) => Math.abs(b.v - b.med) / ((b.q3 - b.q1) || 1) - Math.abs(a.v - a.med) / ((a.q3 - a.q1) || 1))[0];
  const keep = good ? `<p class="inskeep">${ico("check")}${esc(insName(good.k))} ${insChip(good)}</p>` : "";
  if (!items.length) return `${keep}<p class="note">${esc(t("Nothing to change"))}</p>`;
  return `<ol class="insadv">${items.join("")}</ol>${keep}`;
}
function insightH(F){
  const k = INS_KEYS.includes(st.insK) ? st.insK : "cost";
  const tabs = INS_KEYS.filter(x => insBase(x)).map(x => `<button type="button" data-ins="${x}" aria-pressed="${x === k}" title="${esc(insName(x))}">${esc(INS_SHORT()[x])}</button>`).join("");
  const card = (title, body, cls = "") => body ? `<div class="inscol${cls}"><h4>${esc(title)}</h4>${body}</div>` : "";
  return `<section class="insight" aria-labelledby="ins-h"><h3 id="ins-h" class="sr">${esc(t("Overview"))}</h3>${insHeadlines(F)}
    <div class="insgrid"><div class="insleft"><div class="inscol"><div class="inshead"><h4>${esc(t("Trend"))}</h4><div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${tabs}</div></div>${insChart(k)}</div>
    ${card(st.mode === "month" ? t("Cost so far vs last month") : t("Cost so far vs last week"), insCum())}</div>
    ${card(t("Advice"), insAdvice(F), " insadvc")}</div>
    <div class="insrow two">${card(t("Cost by model"), insDonut())}${card(t("Cost by session length"), insHist())}</div>
    <div class="insrow two">${card(t("Cost and outputs"), insScatter())}${card(t("When you worked"), insHeat())}</div></section>`;
}
function bindInsight(R){
  R.querySelectorAll("[data-ins]").forEach(b => b.onclick = () => { st.insK = b.dataset.ins; store.set("insK", st.insK); summary(); const nb = $("#review").querySelector(`[data-ins="${b.dataset.ins}"]`); if (nb) nb.focus(); });
  R.querySelectorAll(".insgo").forEach(b => b.onclick = () => openWorth(b.dataset.goto, b));
  R.querySelectorAll(".insfig [data-sid]").forEach(c => { c.onclick = () => select(c.dataset.sid); c.onkeydown = e => { if (e.key === "Enter" || e.key === " "){ e.preventDefault(); select(c.dataset.sid); } }; }); // 散布図の点は、そのセッションを開く
}

/* 費用と成果の散布図：表示中の期間に始まったセッションごとに、横は推定費用（平方根の目盛り）、縦は成果（そのセッションの間のコミットと作った PR）。
   費用が上位 25%（かつ $0.5 以上）なのに成果のないセッションを「見直す価値のある領域」として色を変え、件数と費用を文で言う */
function insScatter(){
  const {ws, we} = period(), xs = DATA.filter(s => s.start >= ws && s.start < we && s.cost > 0).map(s => ({s, c: s.cost, o: commitsOf(s).length + (s.prs || []).length}));
  if (xs.length < 3) return "";
  const cs = xs.map(x => x.c), thr = Math.max(0.5, insQuant(cs, .75)), maxC = insNice(Math.max(...cs)), maxO = Math.max(4, ...xs.map(x => x.o));
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 200 : 220, L = narrow ? 40 : 48, R = 12, T = 12, B = 30;
  const x = c => L + (W - L - R) * Math.sqrt(c / maxC), y = o => T + (Hh - T - B) * (1 - o / maxO);
  const bad = xs.filter(p => p.o === 0 && p.c >= thr), badC = bad.reduce((t, p) => t + p.c, 0);
  const xt = [0, maxC / 4, maxC].map(c => `<g class="itk"><line x1="${x(c).toFixed(1)}" x2="${x(c).toFixed(1)}" y1="${T}" y2="${Hh - B}"/><text class="ixl" x="${x(c).toFixed(1)}" y="${Hh - 10}">${esc(usd(c))}</text></g>`).join("");
  const yt = [0, Math.round(maxO / 2), maxO].map(o => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(o).toFixed(1)}" y2="${y(o).toFixed(1)}"/><text x="${L - 8}" y="${(y(o) + 4).toFixed(1)}">${o}</text></g>`).join("");
  const zone = `<rect class="izone" x="${x(thr).toFixed(1)}" y="${(y(0) - 14).toFixed(1)}" width="${(W - R - x(thr)).toFixed(1)}" height="14"/>`;
  const pts = xs.map((p, i) => { const hit = p.o === 0 && p.c >= thr, jy = p.o === 0 ? -((i * 37) % 9) - 3 : 0; // 成果 0 の点は重なりやすいので、縦に少しずらす
    return `<circle class="ipt${hit ? " bad" : ""}" cx="${x(p.c).toFixed(1)}" cy="${(y(p.o) + jy).toFixed(1)}" r="${narrow ? 4 : 5}" tabindex="0" role="button" data-sid="${esc(p.s.id)}" aria-label="${esc(`${p.s.title || p.s.id}: ${usd(p.c)}, ${t("{n} outputs", {n: p.o})}`)}"${tipAttr(p.s.title || "", `${usd(p.c)} · ${t("{n} outputs", {n: p.o})}`, p.s.project)}/>`; }).join("");
  const note = `${t("outputs")} = 0 ∧ ${t("cost")} ≥ ${usd(thr)} : ${bad.length} · ${usd(badC)}`;
  const tq = xs.filter(p => p.c >= thr).length, all$ = cs.reduce((a, c) => a + c, 0);
  const sy = t("{m} of the {k} sessions costing {thr} or more had no output: {usd} = {pc}% of session cost", {m: bad.length, k: tq, thr: usd(thr), usd: usd(badC), pc: all$ ? Math.round(badC * 100 / all$) : 0});
  const th = bad.length ? t("Open the red points: research is fine; work that stopped partway is the cost to cut") : "";
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="group" aria-label="${esc(t("Cost and outputs per session"))}">${xt}${yt}${zone}${pts}
    <text class="iax" x="${W - R}" y="${Hh - B - 4}">${esc(t("Estimated cost →"))}</text><text class="iax st" x="${L + 4}" y="${T + 10}">${esc(t("↑ Commits and PRs"))}</text></svg><figcaption><span class="lg izone">${fx(note)}</span></figcaption><p class="ifxs">${fx(`${t("outputs")} = ${t("commits")} + PRs`)}${fx(`${usd(thr)} = max($0.50, P75(${t("cost")}))`)}</p></figure>`;
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
  const cells = g.map((r, d) => `<span class="ihd">${esc(dow(days[d]))}</span>` + r.map((v, h) => `<span class="ihc${d >= 5 || h < 9 || h >= 18 ? " off" : ""}" style="--a:${v ? (0.12 + 0.88 * v / max).toFixed(2) : 0}"${tipAttr(`${dow(days[d])} ${h}:00–${h + 1}:00`, v ? dur(v) : t("None"))}></span>`).join("")).join("");
  const hrs = `<span></span>` + Array.from({length: 24}, (_, h) => `<span class="ihh">${h % 3 ? "" : h}</span>`).join("");
  const peak = `${dow(days[best.d])} ${best.h}–${best.h + 3}${JA ? "時" : ":00"}`;
  const tot = insOffM(g).all, sy = t("Busiest {p}: {h} = {pc}% of active time; off-hours {off}%", {p: peak, h: dur(best.v), pc: tot ? Math.round(best.v * 100 / tot) : 0, off: off == null ? "—" : off});
  const th = b && b.q1 != null && b.pos === "high" ? t("Off-hours share {off}% is above Q3 = {q3}% of your past {unit}", {off, q3: Math.round(b.q3), unit: insUnit()}) : "";
  return `${say(sy, th)}<div class="ihgrid" role="img" aria-label="${esc(t("Busiest: {p}", {p: peak}))}">${hrs}${cells}</div>
    <p class="ifxs">${fx(`${t("peak 3h")} = ${peak} (${dur(best.v)})`)}${off != null ? fx(`${t("off-hours")} = ${dur(insOffM(g).off)} ÷ ${dur(insOffM(g).all)} = ${off}%`) : ""}${off != null ? insChip(b) : ""}</p>
    <p class="note">${esc(t("Off-hours: Saturday, Sunday, and weekdays before 9:00 or from 18:00"))}</p>`;
}

/* 累積の折れ線：表示中の期間の日ごとの推定費用を積み上げた線と、前の期間の線。どの時点で前より多いか少ないかを見る（費用の記録がなければ稼働時間） */
function insCum(){
  const {S: w, P: pw} = period(); if (!w) return "";
  const useCost = w.days.some(d => d.cost > 0), f = d => (useCost ? d.cost : d.active) || 0, fm = useCost ? usd : v => dur(v);
  const cum = ds => { let a = 0; return ds.map(d => a += f(d)); };
  const fr = insFrac(), n = w.days.length, upto = fr < 1 ? Math.max(1, Math.ceil(fr * n)) : n;
  const cur = cum(w.days).slice(0, upto), prev = pw ? cum(pw.days).slice(0, n) : [];
  const top = insNice(Math.max(...cur, ...prev, 0) * 1.05);
  const narrow = innerWidth < 600, W = narrow ? 360 : 720, Hh = narrow ? 180 : 170, L = narrow ? 52 : 64, R = 10, T = 10, B = 24;
  const x = i => L + (W - L - R) * (n === 1 ? 0 : i / (n - 1)), y = v => T + (Hh - T - B) * (1 - v / top), line = a => a.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const ticks = [0, top / 2, top].map(v => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}"/><text x="${L - 6}" y="${(y(v) + 4).toFixed(1)}">${esc(fm(v))}</text></g>`).join("");
  const a = new Date(period().ws * 1000), xl = [0, n - 1].map(i => `<text class="ixl ${i ? "e" : "s"}" x="${x(i).toFixed(1)}" y="${Hh - 6}">${esc(dMD(addDays(a, i)))}</text>`).join(""); // 両端の日付は、内側に寄せる（はみ出さないように）
  const last = cur.length - 1, diff = prev.length > last ? cur[last] - prev[last] : null, ptot = prev.length ? prev[prev.length - 1] : null;
  const sy = diff == null ? "" : t("Day {d} of {n}: {diff} ({pc}) vs the previous period", {d: last + 1, n, diff: (diff >= 0 ? "+" : "−") + fm(Math.abs(diff)), pc: prev[last] ? `${diff >= 0 ? "+" : ""}${Math.round(diff * 100 / prev[last])}%` : "—"})
    + (fr < 1 && ptot != null ? t("; at this pace {end} at the end vs {prev}", {end: fm(cur[last] / fr), prev: fm(ptot)}) : "");
  const left = n - (last + 1), rem = ptot == null ? null : ptot - cur[last];
  const th = fr >= 1 || rem == null || !useCost ? "" : rem <= 0 ? t("Already {x} over the previous period's total {prev}", {x: fm(-rem), prev: fm(ptot)}) : left > 0 ? t("To stay within the previous period's {prev}: at most {rem} in the {d} left = {per}/day", {prev: fm(ptot), rem: fm(rem), d: plural(left, "day"), per: fm(rem / left)}) : "";
  return `${say(sy, th)}<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(`${fm(cur[last])}${prev.length ? ` / ${fm(prev[Math.min(last, prev.length - 1)])}` : ""}`)}">${ticks}${xl}
    ${prev.length ? `<polyline class="iprev" points="${line(prev)}"/>` : ""}<polyline class="icum" points="${line(cur)}"/><circle class="icumd" cx="${x(last).toFixed(1)}" cy="${y(cur[last]).toFixed(1)}" r="4"/></svg>
    <figcaption><span class="lg icum">${esc(st.mode === "month" ? t("This month") : t("This week"))} <b>${esc(fm(cur[last]))}</b></span>${prev.length ? `<span class="lg iprev">${esc(st.mode === "month" ? t("Last month") : t("Last week"))}</span>` : ""}</figcaption>${diff != null ? `<p class="ifxs">${fx(`${fm(cur[last])} − ${fm(prev[last])} = ${(diff >= 0 ? "+" : "−") + fm(Math.abs(diff))}`, diff > 0 ? " bad" : " good")}<span class="note">${esc(t("vs the same day of the previous period ({d})", {d: dMD(addDays(new Date(periodBack(1).ws * 1000), last))}))}</span></p>` : ""}</figure>`;
}
/* ドーナツ：モデル別の推定費用の割合（上位 5 つとその他）。真ん中に合計 */
function insDonut(){
  const {S: w} = period(), ms = ((w && w.usage && w.usage.models) || []).filter(r => r[1] > 0); if (!ms.length) return "";
  const top = ms.slice(0, 5).map(r => [r[0], r[1]]), rest = ms.slice(5).reduce((t, r) => t + r[1], 0); if (rest > 0) top.push([t("Other"), rest]);
  const tot = top.reduce((t, r) => t + r[1], 0), R0 = 60, r0 = 38, C = 70; let a0 = -Math.PI / 2;
  const arc = (v, i) => { const a1 = a0 + v / tot * Math.PI * 2, big = a1 - a0 > Math.PI ? 1 : 0, p = (r, a) => `${(C + r * Math.cos(a)).toFixed(2)},${(C + r * Math.sin(a)).toFixed(2)}`;
    const d = v >= tot ? `M${C},${C - R0}A${R0},${R0} 0 1 1 ${C - .01},${C - R0}L${C - .01},${C - r0}A${r0},${r0} 0 1 0 ${C},${C - r0}Z` : `M${p(R0, a0)}A${R0},${R0} 0 ${big} 1 ${p(R0, a1)}L${p(r0, a1)}A${r0},${r0} 0 ${big} 0 ${p(r0, a0)}Z`; a0 = a1; return d; };
  const col = i => `var(--c${i % 8})`;
  const segs = top.map((r, i) => `<path d="${arc(r[1], i)}" style="fill:${col(i)}"${tipAttr(r[0], `${usd(r[1])} · ${Math.round(r[1] * 100 / tot)}%`)}/>`).join("");
  let acc = 0, k80 = 0; for (const r of top){ acc += r[1]; k80++; if (acc >= tot * .8) break; }
  const sy = t("{model}: {pc}% of cost ({usd}); 80% of cost is in {k} of {n} models", {model: top[0][0], pc: Math.round(top[0][1] * 100 / tot), usd: usd(top[0][1]), k: k80, n: ms.length});
  const {ws: dws, we: dwe} = period(), LT = lightOf(dws, dwe);
  const th = LT.c >= 0.01 ? t("{usd} went to short Opus-class sessions with no edits ({n}): candidates for a lighter model", {usd: usd(LT.c), n: plural(LT.xs.length, "session")}) : "";
  return `${say(sy, th)}<figure class="insfig idonut"><svg viewBox="0 0 140 140" role="img" aria-label="${esc(top.map(r => `${r[0]} ${Math.round(r[1] * 100 / tot)}%`).join(", "))}">${segs}<text class="idc" x="70" y="75">${esc(usd(tot))}</text></svg>
    <ul class="idleg">${top.map((r, i) => `<li><i style="background:${col(i)}"></i><span title="${esc(r[0])}">${esc(r[0])}</span><span class="iv">${esc(usd(r[1]))}</span><b>${Math.round(r[1] * 100 / tot)}%</b></li>`).join("")}</ul></figure>
    <p class="ifxs">${fx(`% = ${t("model cost")} ÷ ${usd(tot)}`)}</p>`;
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
  const sy = useC ? t("Sessions of {bin}: {pc}% of cost ({usd}) in {n} sessions", {bin: labs[mi], pc: Math.round(vs[mi] * 100 / tot), usd: usd(vs[mi]), n: bins[mi].length}) : t("Most sessions ran {bin} ({n})", {bin: labs[mi], n: vs[mi]});
  const LC = useC ? longCtxOf(ws, we) : [];
  const th = mi >= 4 && LC.length ? t("{n} of the long sessions grew their input per response to up to {x}× the start: split them at a stopping point", {n: LC.length, x: Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0])))}) : "";
  return `${say(sy, th)}<figure class="insfig ihist"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(labs.map((l, i) => `${l} ${fv(vs[i])} n=${bins[i].length}`).join(", "))}">${bars}</svg>
    <p class="ifxs">${fx(`${t("median")}(${t("active time of {n} sessions", {n: ss.length})}) = ${dur(med)}`)}${useC ? fx(`% = ${t("cost of the bin")} ÷ ${usd(tot)}`) : ""}</p></figure>`;
}
