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
  const r = {k, cur, v, part: fr < 1, n: xs.length, xs, fmt};
  if (xs.length < INS_MIN) return r;
  const q1 = insQuant(xs, .25), med = insQuant(xs, .5), q3 = insQuant(xs, .75), iqr = q3 - q1;
  const pos = v > q3 ? "high" : v < q1 ? "low" : "typical", far = v > q3 + 1.5 * iqr || v < q1 - 1.5 * iqr;
  const better = pos === "typical" ? null : (pos === "low") === low; // 範囲の外で、よい側か
  return {...r, q1, med, q3, pos, far, better, rank: Math.round(xs.filter(x => x < v).length * 100 / xs.length)};
}
const insName = k => (H()[k] || {}).n || k;
const insUnit = () => st.mode === "month" ? t("months") : t("weeks");
// insPosText は、いつもの範囲との比べ（文）。比べられないときは、そう言う
function insPosText(b){
  if (!b || b.q1 == null) return t("Not enough history to compare yet (needs {n} past {unit})", {n: INS_MIN, unit: insUnit()});
  const fm = b.fmt || (x => fmtM(b.k, x)), range = rangeT(fm(b.q1), fm(b.q3)), v = {range, n: b.n, unit: insUnit()};
  if (b.q1 === b.q3){ const u = {v: fm(b.med), n: b.n, unit: insUnit()}; // いつもほぼ同じ値（0 回など）：範囲ではなく、その値と比べる
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
    <div class="insgrid"><div class="insleft"><div class="inscol"><div class="inshead"><h4>${esc(t("Trend and your usual range"))}</h4><div class="segc insk" role="group" aria-label="${esc(t("Metric"))}">${tabs}</div></div>${insChart(k)}</div>
    ${insScatter()}</div>
    <div class="inscol"><h4>${esc(t("Advice"))}</h4>${insAdvice(F)}</div></div>${insHeat()}</section>`;
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
  const bad = xs.filter(p => p.o === 0 && p.c >= thr), badC = bad.reduce((t, p) => t + p.c, 0), med = a => a.length ? insQuant(a, .5) : null;
  const mO = med(xs.filter(p => p.o > 0).map(p => p.c)), m0 = med(xs.filter(p => p.o === 0).map(p => p.c));
  const xt = [0, maxC / 4, maxC].map(c => `<g class="itk"><line x1="${x(c).toFixed(1)}" x2="${x(c).toFixed(1)}" y1="${T}" y2="${Hh - B}"/><text class="ixl" x="${x(c).toFixed(1)}" y="${Hh - 10}">${esc(usd(c))}</text></g>`).join("");
  const yt = [0, Math.round(maxO / 2), maxO].map(o => `<g class="itk"><line x1="${L}" x2="${W - R}" y1="${y(o).toFixed(1)}" y2="${y(o).toFixed(1)}"/><text x="${L - 8}" y="${(y(o) + 4).toFixed(1)}">${o}</text></g>`).join("");
  const zone = `<rect class="izone" x="${x(thr).toFixed(1)}" y="${(y(0) - 14).toFixed(1)}" width="${(W - R - x(thr)).toFixed(1)}" height="14"/>`;
  const pts = xs.map((p, i) => { const hit = p.o === 0 && p.c >= thr, jy = p.o === 0 ? -((i * 37) % 9) - 3 : 0; // 成果 0 の点は重なりやすいので、縦に少しずらす
    return `<circle class="ipt${hit ? " bad" : ""}" cx="${x(p.c).toFixed(1)}" cy="${(y(p.o) + jy).toFixed(1)}" r="${narrow ? 4 : 5}" tabindex="0" role="button" data-sid="${esc(p.s.id)}" aria-label="${esc(`${p.s.title || p.s.id}: ${usd(p.c)}, ${t("{n} outputs", {n: p.o})}`)}"${tipAttr(p.s.title || "", `${usd(p.c)} · ${t("{n} outputs", {n: p.o})}`, p.s.project)}/>`; }).join("");
  const note = bad.length ? t("{n} of the sessions in the top quarter of cost ({thr} or more) left no commit or PR: {cost} in all", {n: bad.length, thr: usd(thr), cost: usd(badC)}) : t("Every session in the top quarter of cost ({thr} or more) left a commit or PR", {thr: usd(thr)});
  const meds = mO != null && m0 != null ? t("Median cost: {a} for sessions with outputs, {b} for those without", {a: usd(mO), b: usd(m0)}) : "";
  return `<div class="inscol"><h4>${esc(t("Cost and outputs per session"))}</h4><figure class="insfig"><svg viewBox="0 0 ${W} ${Hh}" role="group" aria-label="${esc(t("Cost and outputs per session"))}">${xt}${yt}${zone}${pts}
    <text class="iax" x="${W - R}" y="${Hh - B - 4}">${esc(t("Estimated cost →"))}</text><text class="iax st" x="${L + 4}" y="${T + 10}">${esc(t("↑ Commits and PRs"))}</text></svg>
    <figcaption><span class="lg izone">${esc(t("Costly with no output"))}</span></figcaption><p class="insnow${bad.length ? " bad" : " good"}">${esc(note)}</p>${meds ? `<p class="note">${esc(meds)}</p>` : ""}</figure></div>`;
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
const insOff = g => { let off = 0, all = 0; g.forEach((r, d) => r.forEach((v, h) => { all += v; if (d >= 5 || h < 9 || h >= 18) off += v; })); return all ? Math.round(off * 100 / all) : null; };
function insHeat(){
  const {ws, we} = period(), g = insMinutes(ws, we), max = Math.max(...g.flat()); if (!(max > 0)) return "";
  // 作業がいちばん集まった 3 時間（曜日ごと）
  let best = {v: -1}; g.forEach((r, d) => { for (let h = 0; h + 3 <= 24; h++){ const v = r[h] + r[h + 1] + r[h + 2]; if (v > best.v) best = {v, d, h}; } });
  const off = insOff(g), b = off == null ? null : insRange("offhours", off, p => p.S ? insOff(insMinutes(p.ws, p.we)) : null, true, false, v => `${Math.round(v)}%`);
  const days = [1, 2, 3, 4, 5, 6, 0];
  const cells = g.map((r, d) => `<span class="ihd">${esc(dow(days[d]))}</span>` + r.map((v, h) => `<span class="ihc${d >= 5 || h < 9 || h >= 18 ? " off" : ""}" style="--a:${v ? (0.12 + 0.88 * v / max).toFixed(2) : 0}"${tipAttr(`${dow(days[d])} ${h}:00–${h + 1}:00`, v ? dur(v) : t("None"))}></span>`).join("")).join("");
  const hrs = `<span></span>` + Array.from({length: 24}, (_, h) => `<span class="ihh">${h % 3 ? "" : h}</span>`).join("");
  const peak = t("Most of your work was on {day} {from}:00–{to}:00", {day: dow(days[best.d]), from: best.h, to: best.h + 3});
  return `<div class="inscol insheat"><h4>${esc(t("When you worked"))}</h4><div class="ihgrid" role="img" aria-label="${esc(`${t("When you worked")}: ${peak}`)}">${hrs}${cells}</div>
    <p class="insnow">${esc(peak)}${st.mode === "month" ? esc(t(" (weekdays summed over the month)")) : ""}</p>
    ${off != null ? `<p class="insnow${insTone(b)}">${esc(t("Outside weekday 9:00–18:00: {pct}%", {pct: off}))} — ${esc(insPosText(b))}</p>` : ""}</div>`;
}
