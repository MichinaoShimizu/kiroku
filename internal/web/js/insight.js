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
  const fr = insFrac(), v = INS_ADD.has(k) && fr < 1 ? cur / fr : cur, hs = historyStart(), xs = [];
  for (let i = 1; i <= INS_N; i++){ const p = periodBack(i); if (p.ws < hs) continue; const x = metricAt(k, p); if (x != null) xs.push(x); }
  const r = {k, cur, v, part: fr < 1, n: xs.length, xs};
  if (xs.length < INS_MIN) return r;
  const q1 = insQuant(xs, .25), med = insQuant(xs, .5), q3 = insQuant(xs, .75), iqr = q3 - q1, low = !!MET[k].low;
  const pos = v > q3 ? "high" : v < q1 ? "low" : "typical", far = v > q3 + 1.5 * iqr || v < q1 - 1.5 * iqr;
  const better = pos === "typical" ? null : (pos === "low") === low; // 範囲の外で、よい側か
  return {...r, q1, med, q3, pos, far, better, rank: Math.round(xs.filter(x => x < v).length * 100 / xs.length)};
}
const insName = k => (H()[k] || {}).n || k;
const insUnit = () => st.mode === "month" ? t("months") : t("weeks");
// insPosText は、いつもの範囲との比べ（文）。比べられないときは、そう言う
function insPosText(b){
  if (!b || b.q1 == null) return t("Not enough history to compare yet (needs {n} past {unit})", {n: INS_MIN, unit: insUnit()});
  const range = rangeT(fmtM(b.k, b.q1), fmtM(b.k, b.q3)), v = {range, n: b.n, unit: insUnit()};
  if (b.q1 === b.q3){ const u = {v: fmtM(b.k, b.med), n: b.n, unit: insUnit()}; // いつもほぼ同じ値（0 回など）：範囲ではなく、その値と比べる
    return b.pos === "typical" ? t("Same as usual ({v} in most of the last {n} {unit})", u) : b.pos === "high" ? t("Higher than usual (usually {v} over the last {n} {unit})", u) : t("Lower than usual (usually {v} over the last {n} {unit})", u); }
  if (b.pos === "typical") return t("Within your usual range ({range}, the middle half of the last {n} {unit})", v);
  const s = b.pos === "high" ? (b.far ? t("Far above your usual range ({range}, the middle half of the last {n} {unit})", v) : t("Above your usual range ({range}, the middle half of the last {n} {unit})", v))
    : (b.far ? t("Far below your usual range ({range}, the middle half of the last {n} {unit})", v) : t("Below your usual range ({range}, the middle half of the last {n} {unit})", v));
  return s;
}
const insTone = b => !b || b.q1 == null || b.better == null ? "" : b.better ? " good" : " bad";
/* 要点の文（3 つまで）：費用・成果あたりの費用・いちばん効く見直し。数字を並べず、いつもの範囲と比べて言い切る */
function insHeadlines(F){
  const L = [], bc = insBase("cost");
  if (bc){ const now = fmtM("cost", bc.cur), pace = bc.part ? t(" (at this pace, {v} for the {period})", {v: fmtM("cost", bc.v), period: st.mode === "month" ? t("month") : t("week")}) : "";
    L.push({k: "cost", tone: insTone(bc), h: t("Estimated cost {v}{pace}", {v: now, pace}), d: insPosText(bc)}); }
  const bp = insBase("costPerCommit");
  if (bp) L.push({k: "costPerCommit", tone: insTone(bp), h: t("{v} per commit", {v: fmtM("costPerCommit", bp.cur)}), d: insPosText(bp)});
  else { const ba = insBase("active"); if (ba) L.push({k: "active", tone: insTone(ba), h: t("Active time {v}", {v: fmtM("active", ba.cur)}), d: insPosText(ba)}); }
  L.push(F.length ? {k: "", tone: " bad", h: t("Most worth changing: {what}", {what: insName(GOTO[F[0].k] || F[0].k)}), d: t("See the advice below")}
    : {k: "", tone: " good", h: t("Nothing stands out to change"), d: t("No metric crossed its threshold in the period shown")});
  return `<div class="insh">${L.map(x => `<div class="inshl${x.tone}"><b>${esc(x.h)}</b><span>${esc(x.d)}</span></div>`).join("")}</div>`;
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
  return `<figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="img" aria-label="${esc(t("{metric} over the last {n} {unit}", {metric: insName(k), n, unit: insUnit()}))}">${ticks}${band}${bars}</svg>
    <figcaption><span class="lg iband">${esc(t("Your usual range (middle half)"))}</span><span class="lg imed">${esc(t("Median"))}</span>${proj != null ? `<span class="lg iproj">${esc(t("At this pace"))}</span>` : ""}<span class="lg iout">${esc(t("Far outside the usual range"))}</span></figcaption>
    <p class="sr">${esc(list)}</p><p class="insnow${insTone(b)}">${esc(insName(k))}: ${esc(fmtM(k, cur.v))} — ${esc(insPosText(b))}</p></figure>`;
}
/* 改善のアドバイス：見直す候補を効き目の大きい順に 3 つまで。何が・いつもと比べて・何をするか（指標の説明の「打てる手」）と、該当するセッション。
   よくなった指標も 1 つ返す（続けることがわかるように） */
function insAdvice(F){
  const items = F.slice(0, 3).map(f => { const id = GOTO[f.k] || f.k, b = MET[f.k] ? insBase(f.k) : null, h = H()[f.k] || H()[id] || {};
    return `<li class="insa"><div class="insw"><b>${esc(f.L.see)}</b>${b && b.q1 != null ? `<span class="insb${insTone(b)}">${esc(insPosText(b))}</span>` : ""}</div>
      <p class="why">${esc(f.L.why)}</p>${h.a ? `<p class="todo"><span>${esc(t("Try"))}</span>${esc(h.a)}</p>` : ""}
      ${f.ids.length || f.k === "repeats" ? `<button class="pill insgo" type="button" data-goto="${esc(id)}">${esc(f.ids.length ? t("See {sessions}", {sessions: plural(f.ids.length, "session")}) : t("See the details"))}</button>` : ""}</li>`; });
  // よくなった指標：範囲の外でよい側のうち、いちばん外れているもの
  const good = Object.keys(MET).map(k => insBase(k)).filter(b => b && b.better).sort((a, b) => Math.abs(b.v - b.med) / ((b.q3 - b.q1) || 1) - Math.abs(a.v - a.med) / ((a.q3 - a.q1) || 1))[0];
  const keep = good ? `<p class="inskeep">${ico("check")}${esc(t("Keep it up: {metric} is {v}, better than your usual range ({range})", {metric: insName(good.k), v: fmtM(good.k, good.cur), range: rangeT(fmtM(good.k, good.q1), fmtM(good.k, good.q3))}))}</p>` : "";
  if (!items.length) return `${keep}<p class="note">${esc(t("No metric crossed its threshold in the period shown. Nothing to change for now."))}</p>`;
  return `<ol class="insadv">${items.join("")}</ol>${keep}`;
}
function insightH(F){
  const k = INS_KEYS.includes(st.insK) ? st.insK : "cost";
  const tabs = INS_KEYS.filter(x => insBase(x)).map(x => `<button type="button" data-ins="${x}" aria-pressed="${x === k}" title="${esc(insName(x))}">${esc(INS_SHORT()[x])}</button>`).join("");
  return `<section class="insight" aria-labelledby="ins-h"><h3 id="ins-h" class="sr">${esc(t("Overview"))}</h3>${insHeadlines(F)}
    <div class="insgrid"><div class="inscol"><div class="inshead"><h4>${esc(t("Trend and your usual range"))}</h4><div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${tabs}</div></div>${insChart(k)}</div>
    <div class="inscol"><h4>${esc(t("Advice"))}</h4>${insAdvice(F)}</div></div></section>`;
}
function bindInsight(R){
  R.querySelectorAll("[data-ins]").forEach(b => b.onclick = () => { st.insK = b.dataset.ins; store.set("insK", st.insK); summary(); const nb = $("#review").querySelector(`[data-ins="${b.dataset.ins}"]`); if (nb) nb.focus(); });
  R.querySelectorAll(".insgo").forEach(b => b.onclick = () => openWorth(b.dataset.goto, b));
}
