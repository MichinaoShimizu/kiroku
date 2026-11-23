// 描画の入口 render、上の帯（要点）、週と月のカレンダー、ツールチップ
/* ── render ── */
function period(){ // 今見ている期間 [ws, we) と、その集計
  if (st.mode === "month"){ const a = st.month, b = new Date(a.getFullYear(), a.getMonth()+1, 1); return {ws:a.getTime()/1000, we:b.getTime()/1000, S:MONTHS[mkey(a)], P:MONTHS[mkey(new Date(a.getFullYear(), a.getMonth()-1, 1))]}; }
  return {ws:st.week.getTime()/1000, we:addDays(st.week,7).getTime()/1000, S:WEEKS[key(st.week)], P:WEEKS[key(addDays(st.week,-7))]};
}
function render(){
  assignColors();
  const {ws, we} = period(), M = st.mode === "month";
  const todayKey = key(today0()), end = addDays(st.week,6);
  const rel = relName(); // 今週・先週（今月・先月）なら、日付の上にそう書く（開いたときに、どの期間を見ているかわかるように）
  if (M){ $("#ry").textContent = `${st.month.getFullYear()} · ${rel || t("Month")}`; $("#rd").innerHTML = isJA() ? `${st.month.getFullYear()}年<span>${st.month.getMonth()+1}月</span>` : `${MONTH[st.month.getMonth()]} <span>${st.month.getFullYear()}</span>`; }
  else { const iy = addDays(st.week, 3).getFullYear(); // ISO の週の年（その週の木曜の年）。週の数は、その年の 12 月 28 日の週（いつも最後の週）
    $("#ry").textContent = `${iy} · ${t("Week {n} of {total}", {n: isoWeek(st.week), total: isoWeek(new Date(iy, 11, 28))})}${rel ? ` · ${rel}` : ""}`; $("#rd").innerHTML = `${dMD(st.week)}<span> – </span>${dMD(end)}`; }
  $("#today").textContent = META.scope ? (META.scope.mode === "month" ? t("Back to this file's month") : t("Back to this file's week")) : M ? t("This month") : t("This week");
  [["#prev", -1], ["#next", 1]].forEach(([id, n]) => { const b = $(id), off = !canGo(n); // 先の週・ファイルの外へは押せない
    if (off && document.activeElement === b) $(n > 0 ? "#prev" : "#next").focus(); // 押せなくなるボタンにいたフォーカスは、隣のボタンへ
    b.disabled = off; });
  $("#prev").setAttribute("aria-label", M ? t("Previous month") : t("Previous week")); $("#next").setAttribute("aria-label", M ? t("Next month") : t("Next week"));
  document.querySelectorAll("#mode button[data-v]").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.mode));
  showView(); // カレンダーかサマリーか（画面いっぱいに 1 つだけ）

  const inRange = DATA.filter(s => s.end >= ws && s.start < we);
  const shown = inRange.filter(matches);
  kpis();
  // legend（この期間にあるものを、作業時間の多い順に。数はサマリーの「① By project」の配分と同じ作業時間）
  const cnt = {}; inRange.forEach(s => cnt[keyOf(s)] = (cnt[keyOf(s)]||0)+1);
  const mins = legendMinutes();
  const noHit = !!st.q && !DATA.some(matches); // 検索がどこにも当たらない（カレンダーは空になり、上の検索結果だけが「ない」と言う）
  // 件数は横にスクロールする並び（.lrow）の外に置く。スマホでは並びの上に出し、チップが多くても画面の外へ押し出されないように
  const CP = calPick((M ? (MONTHS[mkey(st.month)] || {}).days : (WEEKS[key(st.week)] || {}).days) || []), SH = SHADE();
  $("#legend").innerHTML = `<label class="cbsel"><span class="cbt">${esc(t("Color by"))}</span><select id="cb2">${Object.entries(CB()).map(([v,l]) => `<option value="${v}"${st.colorBy === v ? " selected" : ""}>${esc(l)}</option>`).join("")}</select></label>${CP.avail.length > 1 ? `<label class="cbsel"><span class="cbt">${esc(t("Figures"))}</span><select id="calk">${CP.avail.map(k => `<option value="${k}"${k === CP.k ? " selected" : ""}>${esc(SH[k].s)}</option>`).join("")}</select></label>` : ""}<div class="lrow"><span class="lab">${esc(t("Active time"))}<small> · ${esc(t("press to hide"))}</small></span>` +
    Object.keys(cnt).sort((a,b)=>(mins[b]||0)-(mins[a]||0) || cnt[b]-cnt[a]).map(k => `<button class="chip" style="--c:${colorOf(k)}" data-k="${esc(k)}" aria-pressed="${!st.hidden.has(k)}" title="${esc(t("{name}: {stats} (click to show or hide)", {name: k, stats: `${mins[k] ? dur(mins[k]) + " · " : ""}${plural(cnt[k], "session")}`}))}"><span class="dot"></span>${st.colorBy === "source" ? agMark(k) : ""}<span class="cn">${esc((k))}</span><span class="n">${mins[k] ? dur(mins[k]) : "—"}<span class="sr"> ${esc(t("active"))}</span></span></button>`).join("") +
    `</div>${filtering() ? `<span class="count">${esc(t(M ? "{n} of the month's {total} sessions" : "{n} of the week's {total} sessions", {n: shown.length, total: inRange.length}))}</span>` : ""}`; // 件数は絞り込んでいるときだけ（ふだんは全件なので言うことがない）
  // 検索・凡例で絞り込んでいるあいだ、絞り込めない合計（作業時間・トークンなど）は薄くして、そう断る
  const fn = $("#fnote"); fn.hidden = !filtering() || noHit;
  fn.textContent = `${st.q ? t("This view shows only sessions matching the search and their linked records.") : t("Hidden items are left out of this view.")} ${M ? t("Gray figures, at the top and in each day and week, are totals for all sessions; sessions and commits there count only what is shown.") : t("Gray figures, at the top and under each date, are totals for all sessions; sessions and commits there count only what is shown.")}`;
  $("#legend").hidden = !inRange.length; // 記録のない期間は、色分け・件数を出さない（押しても何も変わらない）
  const ck = $("#calk"); if (ck) ck.onchange = () => { st.shade = ck.value; store.set("mShade", st.shade); render(); const n = $("#calk"); if (n) n.focus(); }; // 日の数（と月の濃さ）に使う量
  const cb2 = $("#cb2"); if (cb2) cb2.onchange = () => { st.colorBy = cb2.value; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); };
  document.querySelectorAll(".chip").forEach(c => c.onclick = () => { const k = c.dataset.k; st.hidden.has(k) ? st.hidden.delete(k) : st.hidden.add(k); render(); });

  if (!M) st.hh = fitHour(); // 1 時間の高さは、この週の作業の時間帯が枠に収まるように（44px か 36px）
  M ? monthGrid(shown, ws, we, todayKey) : timeline(shown, inRange, ws, we, todayKey);
  edgeFade($("#tl")); // 月表示や空の週では消す
  $("#skipcal").hidden = !!st.q; // 検索の最中は、結果がカレンダーの上にあり、下のサマリーは空
  navSync(); // 詳細の上の帯の前後のセッション
  lastFlags = []; // 見直す候補は表示中の期間のものだけ（検索中や記録のない期間に、前の期間の候補を詳細に出さない）
  try { st.q ? searchPanel() : summary(); } catch(e){ // サマリーで失敗しても、カレンダーと詳細は使えるようにする
    console.error(e); $("#review").innerHTML = `<div class="panel">${noneH(esc(t("Couldn't show the summary for this period ({error}). Please let us know in an issue.", {error: e.message})))}</div>`; }
  const gc = st.sel && st.sel.startsWith("git:") && (META.git || []).find(x => "git:" + x.hash === st.sel);
  const pu = st.sel && st.sel.startsWith("push:") && findPush(st.sel.slice(5)), pr = st.sel && st.sel.startsWith("pr:") && findPR(st.sel.slice(3));
  const s = !gc && !pu && !pr && st.sel && DATA.find(x => x.id === st.sel), open = !!(s || gc || pu || pr);
  if (st.sel && !open) st.sel = null;
  if (s) detail(s); else if (gc) commitDetail(gc); else if (pu) pushDetail(pu); else if (pr) prDetail(pr.s, pr.r);
  if (open){ $("#dkind").textContent = s ? t("Session details") : gc ? t("Commit details") : pu ? t("Push details") : t("Pull request details"); // 読み上げで、何の詳細かと題名がわかるように
    const h2 = $("#panel h2"); if (h2) h2.id = "dtitle"; }
  dtopSync();
  document.body.classList.toggle("open", open); document.body.classList.toggle("lock", open);
  $("#drawer").setAttribute("aria-hidden", String(!open));
  document.querySelector("header").inert = document.querySelector("main").inert = open; // 背後に Tab で入らない
  $("#back").hidden = !st.back.length;
  if (open && (!focusDrawer.was || focusDrawer.sel !== st.sel)) $(st.back.length ? "#back" : "#close").focus();
  if (!open && focusDrawer.was){ const last = focusDrawer.sel; // ←→ でほかのセッションへ移っていたら、最後に読んでいたセッションの帯へ戻る（最初に開いた帯ではなく）
    const el = last && last !== focusDrawer.first && document.querySelector(`.run[data-sid="${CSS.escape(last)}"]`);
    if (el){ el.scrollIntoView({block: "nearest", inline: "nearest"}); el.focus({preventScroll: true}); el.classList.remove("flash"); void el.offsetWidth; el.classList.add("flash"); }
    else focusOpener(focusDrawer.first, focusDrawer.from); }
  if (open && !focusDrawer.was){ focusDrawer.first = st.sel; focusDrawer.from = focusDrawer.next; histOpen(); }
  if (!open && focusDrawer.was) histClose();
  focusDrawer.next = null;
  focusDrawer.was = open; focusDrawer.sel = st.sel;
  $("#tl").classList.toggle("focus", open);
  st.animate = false;
}

// relName は、表示中の期間が今週・先週（今月・先月）ならその呼び名、ほかは ""
function relName(){ const t0 = today0();
  if (st.mode === "month"){ const m = monthOf(t0), d = (m.getFullYear() - st.month.getFullYear()) * 12 + m.getMonth() - st.month.getMonth();
    return d === 0 ? t("This month") : d === 1 ? t("Last month") : ""; }
  const d = Math.round((mondayOf(t0) - st.week) / 864e5);
  return d === 0 ? t("This week") : d >= 6 && d <= 8 ? t("Last week") : ""; } // 夏時間をまたぐ週は 7 日ちょうどにならないので、日数を丸めて
// legendMinutes は、凡例の各くくりの作業時間（分）。同時に動いたセッションの時間は分けて数える（サマリーの配分と同じ）
function legendMinutes(){ const {S: w} = period(), m = {}; if (!w) return m;
  if (st.colorBy === "project") (w.projects || []).forEach(([k, v]) => m[k] = v);
  else ((w.shares || {})[st.colorBy] || []).forEach(x => m[x.key] = x.minutes);
  return m; }
// shownName は、表示中の期間の呼び名。今週・今月なら "This week" / "This month"、ほかは "Week of Sep 28" / "September 2026"
function shownName(){ const M = st.mode === "month", t0 = today0();
  if (M) return st.month.getTime() === monthOf(t0).getTime() ? t("This month") : dMY(st.month);
  return st.week.getTime() === mondayOf(t0).getTime() ? t("This week") : t("Week of {date}", {date: dMD(st.week)}); }
/* 期間の要点（上の帯） */
// kpiDelta は、帯の数（費用・クレジット・トークン・稼働時間）の前の期間との増減の文（なければ ""）。帯には出さず、押したときの内訳の大きな数に添える。
// 途中の期間は、前の期間の同じ日までと比べる（vsPrev）。良し悪しの色は付けない
function kpiDelta(id){
  const {S: w, P: pw} = period(); if (!w) return "";
  const M = st.mode === "month", V = vsPrev(pw, M ? "月" : "週"), vsT = V.n == null ? t(M ? "vs last month" : "vs last week") : t("vs {range}", {range: V.range});
  const u = w.usage || {}, pu = pw && pw.usage;
  const pair = {cost: [u.cost, V.of("cost", pu && pu.cost)], credits: [u.credits, V.of("credits", pu && pu.credits)], tokens: [u.tokens, V.of("tokens", pu && pu.tokens)], active: [w.active, V.of("active", pw && pw.active)]}[id];
  if (!pair) return ""; const [a, b] = pair; if (b == null || !(b > 0) || a == null) return "";
  return `${a === b ? "±0" : (a > b ? "+" : "−") + Math.round(Math.abs(a - b) / b * 100)}% ${vsT}`;
}
function kpis(){
  const {S:w, P:pw} = period(), K = $("#kpis"), M = st.mode === "month";
  K.classList.toggle("unf", filtering()); // 絞り込みの最中も、ここは全セッションの合計（薄くして、カレンダーの上に断り書き）
  if (!w){ K.innerHTML = `<div class="kpi"><div class="k">${esc(META.scope ? t("Not in this file") : shownName())}</div><div class="v">${esc(t("No records"))}</div></div>`; return; }
  // 押すと内訳のダイアログ（metric.js）。id のない数は押せない
  const kpi = (k, v, tt, cls, id) => { const tg = id ? "button" : "div";
    return `<${tg} class="kpi${cls ? " " + cls : ""}"${id ? ` type="button" data-metric="${id}" aria-haspopup="dialog"` : ""}${tt || id ? ` title="${esc(tt ? (id ? t("{text}. Click for the breakdown.", {text: tt}) : tt) : t("Click for the breakdown"))}"` : ""}><div class="k">${esc(k)}</div><div class="v">${v}</div></${tg}>`; };
  const u = w.usage || {}, days = w.days.filter(d => d.active).length, pj = projection(w);
  const n0 = soFar(), nd = n0 == null ? w.days.length : Math.min(n0, w.days.length); // 途中の週・月は、まだ来ていない日を分母に入れない（今日までの日数）
  // 前の期間並みに収めるなら、残りの日 1 日あたりいくらか（pacing）。費用とクレジットは 1 つの枠に並べる（枠を増やすと帯がカレンダーと主役を争うため）。
  // 超えていれば、その分を「超過」と書く。良し悪しは言わない。ちょうど同じ額なら、超過 +0 ではなく 1 日 0
  const pc = pacing(w, pw), paceK = () => {
    const xs = [[pc.cost, (v, txt) => txt ? usd(v) : usdH(v)], [pc.credits, (v, txt) => txt ? cr(v) : crH(v)]].filter(([x]) => x);
    const all = xs.every(([x]) => x.rest < 0);
    if (all) return kpi(t(M ? "Over last month's total" : "Over last week's total"), xs.map(([x, f]) => `<small>+</small>${f(-x.rest)}<small> ${esc(t("over"))}</small>`).join(`<small> · </small>`),
      t(M ? "Last month's total was {v}" : "Last week's total was {v}", {v: xs.map(([x, f]) => f(x.ref, true)).join(" · ")}), "", pc.cost ? "pace" : "paceCr");
    // 片方（多くはクレジット）だけ超えたときは、1 日の目安の数に超過を混ぜない（「$3.20 · +5 cr 超過」は 1 日の額に読める）。
    // 数は超えていない方の目安だけにし、超えた方は短い添え書き（「クレジットは超過」）と、ポップアップの文で伝える
    const ok = xs.filter(([x]) => x.rest >= 0), ov = xs.filter(([x]) => x.rest < 0);
    const v = ok.map(([x, f]) => f(x.per)).join(`<small> · </small>`) + (ov.length ? `<small class="ovr"> · ${esc(t("{v} over", {v: ov[0][1](-ov[0][0].rest, true)}))}</small>` : "");
    const tt = t(M ? "To stay at last month's total of {v}, for the {n} left" : "To stay at last week's total of {v}, for the {n} left", {v: ok.map(([x, f]) => f(x.ref, true)).join(" · "), n: plural(pc.left, "day")}) +
      ov.map(([x, f]) => " · " + t(M ? "{v} over last month's total of {r}" : "{v} over last week's total of {r}", {v: f(-x.rest, true), r: f(x.ref, true)})).join("");
    return kpi(t(M ? "A day to match last month" : "A day to match last week"), v, tt, "", ok[0][0] === pc.cost ? "pace" : "paceCr"); };
  const pace = pj ? t("If the pace of the first {n} days continues", {n: pj.days}) + bandNote(pj.cost != null ? pj.band.cost : pj.band.credits, pj.cost != null ? usd : cr) : "";
  // 帯は日々の判断に使う数だけ：左に使ったもの（推定費用とクレジット・前の期間並みの 1 日の目安・月末の推定（月）・稼働時間）、残したもの（コミット・PR など）は出さない（カレンダーの各日とサマリーの ② に）。
  // 稼働日・セッション / プロンプト・利用上限への到達は振り返りの数なので、サマリーと「見直す候補」に任せる（帯が多いとカレンダーと主役を争う）。
  // 推定費用とクレジットは別の枠の使用量なので足さずに、1 つの枠に「 · 」で並べる。トークンは推定費用を出せないときだけ
  const c = u.tokens ? costOf(u) : null, two = (a, b) => [a, b].filter(Boolean).join(`<small> · </small>`);
  const useV = two(u.tokens ? (c == null ? tok(u.tokens) : usdH(c)) : "", u.credits ? crN(u.credits) : ""); // 見出しに「クレジット」とあるので、数に単位は付けない
  const useK = u.tokens && u.credits ? t(c == null ? "Tokens · credits" : "Estimated cost · credits") : u.tokens ? t(c == null ? "Tokens" : "Estimated cost") : t("Kiro credits");
  const pjV = pj ? two(pj.cost != null ? `<small>≈</small>${usdH(pj.cost)}` : "", pj.credits != null ? `<small>≈</small>${pj.cost != null ? crH(pj.credits) : crN(pj.credits)}` : "") : ""; // 「月末のクレジット」だけなら単位は見出しにある
  K.innerHTML = (useV ? kpi(useK, useV, u.tokens && c == null ? NOPRICE : "", "", u.tokens ? (c == null ? "tokens" : "cost") : "credits") : "") + // 増減（%）は帯に出さず、押したときの内訳に（kpiDelta）。枠の色もそろえる
    (pc && !pj ? paceK() : "") + // 月の途中は月末の推定を出すので、前の月並みの目安は出さない（内訳とサマリーにある）
    ((mp => mp && mp.credits != null ? kpi(t("Month-end credits"), `<small>≈</small>${crN(mp.credits)}`, t("This month so far: {v}", {v: cr(mp.soCredits)}) + " · " + t("If the pace of the first {n} days continues", {n: mp.days}), "", "monthCr") : "")(monthProj())) + // 今週を見ているときも、クレジットの今月末の推定（クレジットは月ごとの枠）
    (pjV ? kpi(t(pj.cost != null && pj.credits != null ? "Month-end (estimate)" : pj.cost != null ? "Month-end cost" : "Month-end credits"), pjV, pace, "", pj.cost != null ? "projection" : "projectionCr") : "") +
    kpi(t("Active time"), dur(w.active, true), n0 == null ? "" : M ? t("{days} of the {n} so far (this month is still in progress)", {days, n: plural(nd, "day")}) : t("{days} of the {n} so far (this week is still in progress)", {days, n: plural(nd, "day")}), "", "active");
  K.classList.toggle("dense", K.children.length > 7); // 数が多い（月表示など）ときは、一回り小さくして 1 行に
  K.querySelectorAll("[data-metric]").forEach(b => b.onclick = () => openMetric(b.dataset.metric, b));
  kWrap();
}
// kWrap は、上の帯が期間の行の右に収まらず、下の行に回ったかを印にする（サマリーでは、回ったときだけ帯の場所を残さない）
function kWrap(){ const K = $("#kpis"), W = $("#kpis") && $("#kpis").parentElement, N = W && W.querySelector(".wk"); if (!N) return;
  W.classList.remove("kwrap"); if (K.offsetHeight && K.offsetTop >= N.offsetTop + N.offsetHeight - 4) W.classList.add("kwrap"); }
addEventListener("resize", () => kWrap());

/* カレンダーの各日（月表示では各週も）に並べるのは 2 つだけ：使用量（推定費用。見積もれないならトークン、トークンがなければクレジット）と Git のコミット。
   費用と残したものを上下に並べ、費用が増えても残したものが増えていない日を、カレンダーを見るだけで拾えるようにする。
   作業時間は週では帯の長さ・月ではマスの濃さ、セッションは帯の数で見えるので、数は日のツールチップに（トークン・クレジットも）。
   見出しは期間の中でそろえる（日によって見出しが変わると、横に並べて比べられない）。
   un は絞り込み（検索・凡例で隠す）の最中に、絞り込まれていない合計を薄くする（1: 作業時間・トークン・クレジット、2: 全部） */
/* カレンダーに書く量（週の日の見出し・月のマスの数と濃さ）。凡例の「量」で選ぶ。選べるのは表示中の期間の日に記録のある量で、
   既定は推定費用、なければトークン、なければクレジット、どれもなければ稼働時間 */
function calPick(days){ const SH = SHADE(), avail = Object.keys(SH).filter(k => days.some(d => d && SH[k].f(d) > 0)), kn = useKinds(days);
  const def = kn.cost ? "cost" : kn.tokens ? "tokens" : kn.credits ? "credits" : "active";
  return {avail, k: avail.includes(st.shade) ? st.shade : avail.includes(def) ? def : avail[0] || "active"}; }
// calVal は、量 k の値を書く（short はマスの狭い所：桁を減らし、クレジットは単位を省く。何の量かは凡例の「量」にある）
const calVal = (k, act, u, short) => k === "active" ? (act ? dur(act) : "") : !u ? "" : k === "cost" ? (u.cost > 0 ? (short ? usdS(u.cost) : usd(costOf(u))) : "")
  : k === "tokens" ? (u.tokens ? (short ? tokS(u.tokens) : tok(u.tokens)) : "") : (u.credits ? (short ? crN(u.credits) : cr(u.credits)) : "");
const useKinds = xs => ({tokens: xs.some(d => d && d.tokens), cost: xs.some(d => d && d.cost > 0), credits: xs.some(d => d && d.credits)});
// 時間とコスト系の両方を書く。コスト系は「量」で選んだもの（時間を選んだときは、推定費用・トークン・クレジットのうち記録のあるもの）
const useKind = kinds => kinds.k && kinds.k !== "active" ? kinds.k : kinds.cost ? "cost" : kinds.tokens ? "tokens" : kinds.credits ? "credits" : "";
function calRows(act, u, nS, nC, kinds, un){
  const k = un ? "k u" : "k", o = un > 1 ? "u" : "", uk = useKind(kinds);
  // [名前, 値（ただの文字）, class, 種類]。HTML に入れるときは、呼ぶ側で esc する
  // 作業のない日（コミットだけの日）は、時間とコストの「—」を並べず、コミットだけ
  // 出力（コミット）を先頭に。使ったもの（費用・時間）はその後ろ
  return [[t("Commits"), String(nC), o, "commit"], act && uk && [SHADE()[uk].s, calVal(uk, act, u) || "—", k, "use"], act && [t("Active time"), dur(act), k, "time"]].filter(Boolean);
}
// 週の日の見出しは 1 行：使用量（推定費用など）と、コミットの数（印つき）。何の数かは、見出しのツールチップと読み上げの名前で言う
const calLine = rows => `<div class="cline">${rows.filter(([, v, , ty]) => !(ty === "commit" && v === "0")).map(([k, v, c, ty]) => `<span class="${ty === "use" ? "cu" : ty === "time" ? "ct" : "cc"}${c && c.includes("u") ? " u" : ""}" aria-label="${esc(`${k}: ${v}`)}">${ty === "commit" ? ico("commit") : ""}${esc(v)}</span>`).join(`<span class="sep" aria-hidden="true">·</span>`)}</div>`;
// スマホでは作業時間と使用量だけ。トークンとクレジットは別の行に出す（両方あれば 2 行）。
// ひとつの枠にまとめると、Claude Code と Kiro を両方使う人には、片方しか見えないため。
function calShort(act, u, un, kinds){ // 月のマス：コスト系（太く）と稼働時間（小さく）の 2 行
  const q = un ? " u" : "", uk = useKind(kinds), v = uk ? calVal(uk, act, u, true) : "";
  // 時間は 2 通り書き、マスが狭い（スマホの月）ときは短い方（「5h」）だけを見せる
  const sh = !act ? "—" : act < 60 ? `${Math.round(act)}m` : `${Math.round(act / 60)}h`;
  return `${uk ? `<span class="acs${q}">${esc(v || "—")}</span>` : ""}<span class="act${q}"><span class="tlong">${esc(act ? dur(act) : "—")}</span><span class="tshort" aria-hidden="true">${esc(sh)}</span></span>`;
}
const filtering = () => !!st.q || st.hidden.size > 0;
/* カレンダーに出すコミット：凡例で隠したプロジェクトのものは出さない。検索中は、一致したセッションの間のコミットと、それ自体が一致したコミットだけ */
const gitHit = c => [c.hash, c.subject, c.body || "", ...(c.files || []).map(f => f.path)].join("\n").toLowerCase().includes(st.q);
function gitShown(shown){
  const linked = st.q ? new Set(shown.flatMap(s => commitsOf(s).map(c => c.hash))) : null;
  return (META.git || []).filter(c => (!st.hidden.size || st.colorBy !== "project" || !st.hidden.has(c.project)) && (!linked || linked.has(c.hash) || gitHit(c)));
}
/* 月のカレンダー：日ごとの量を濃さで。濃さの量は選べる（推定費用・クレジット・トークン・稼働時間）。
   既定は、マスに書く数と同じ量（推定費用、なければトークン、なければクレジット）。どれもなければ稼働時間 */
const SHADE = () => ({cost: {f: d => d.cost > 0 ? d.cost : 0, s: t("Est. cost")}, credits: {f: d => d.credits || 0, s: t("Credits")}, tokens: {f: d => d.tokens || 0, s: t("Tokens")}, active: {f: d => d.active || 0, s: t("Active time")}});
// emptyCal は、記録のない週・月の 1 枚（「ない」の一言と、記録のある最後の期間へ行くボタン）。文は呼ぶ側の t() で、ここでエスケープする
function emptyCal(T, msg, btn, onClick){
  T.innerHTML = `<div class="empty"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="15" rx="3"/><path d="M3 10h18M8 3v4M16 3v4"/></svg><p>${esc(msg)}</p>${btn ? `<p><button class="pill" id="tolast">${esc(btn)}</button></p>` : ""}</div>`;
  if (btn) $("#tolast").onclick = onClick; }
function monthGrid(shown, ms, me, todayKey){
  const T = $("#tl"), S = MONTHS[mkey(st.month)], first = mondayOf(st.month);
  const un = filtering() ? 1 : 0, G = un ? gitShown(shown) : null;
  if (!un && !META.scope && !(S && S.days.some(d => d && d.active))){ // 記録のない月は、空のマスと濃さの目盛りではなく、週と同じく「ない」と言い、記録のある最後の月へ行けるようにする
    const last = DATA.length ? monthOf(new Date(Math.max(...DATA.map(s => s.end)) * 1000)) : null, jump = last && last.getTime() !== st.month.getTime();
    emptyCal(T, t("No records in {month}.", {month: dMY(st.month)}), jump ? t("Go to the latest month with records ({month})", {month: dMY(last)}) : "", () => { st.month = last; st.sel = null; st.animate = true; render(); });
    return; }
  const wks = []; for (let r = 0; r < 6 && addDays(first, 7*r).getTime()/1000 < me; r++) wks.push(addDays(first, 7*r));
  const kinds = useKinds([...(S ? S.days : []), ...wks.map(wk => (WEEKS[key(wk)] || {}).usage)]);
  const SH = SHADE(), days = S ? S.days : [], sk = calPick(days).k; kinds.k = sk;
  const max = Math.max(1e-9, ...days.map(d => d ? SH[sk].f(d) : 0));
  // 日ごとの、色分け（Color by）ごとの作業時間（秒）。セッションを 1 度だけたどり、日ごとに振り分ける（日ごとに全セッションを見直さない）
  const byDay = new Map(); shown.forEach(s => { const k = keyOf(s); s.segs.forEach(([p, q]) => { if (q <= ms || p >= me) return;
    for (let d = new Date(Math.max(p, ms) * 1000); d.getTime() / 1000 < Math.min(q, me); d = addDays(new Date(d.getFullYear(), d.getMonth(), d.getDate()), 1)){
      const d0 = new Date(d.getFullYear(), d.getMonth(), d.getDate()), a = Math.max(p, d0.getTime() / 1000), b = Math.min(q, addDays(d0, 1).getTime() / 1000), dk = key(d0);
      if (b > a){ if (!byDay.has(dk)) byDay.set(dk, new Map()); const m = byDay.get(dk); m.set(k, (m.get(k) || 0) + b - a); } } }); });
  let h = `<div class="mgrid"><div class="mh"></div>${[1,2,3,4,5,6,0].map(i=>`<div class="mh${wkc(i)}">${dow(i)}</div>`).join("")}`;
  for (let r = 0; r < 6; r++){
    const wk = addDays(first, 7*r); if (wk.getTime()/1000 >= me) break;
    // 週の合計は、この月の日の分だけ（月の外の日を足すと、マスの数と合わない）。その週まるごとの数はツールチップに
    const W = WEEKS[key(wk)], mi = [0,1,2,3,4,5,6].map(c => addDays(wk, c)).filter(d => d.getMonth() === st.month.getMonth()).map(d => S ? S.days[d.getDate()-1] : null).filter(Boolean);
    const part = {active: mi.reduce((a, x) => a + (x.active || 0), 0), cost: mi.reduce((a, x) => a + (x.cost || 0), 0), tokens: mi.reduce((a, x) => a + (x.tokens || 0), 0), credits: mi.reduce((a, x) => a + (x.credits || 0), 0), commits: mi.reduce((a, x) => a + (x.commits || 0), 0)};
    const whole = W && W.active && mi.length < 7 ? [`${t("Whole week, with days in other months")}: ${dur(W.active)}${W.usage && W.usage.cost > 0 ? ` · ${usd(costOf(W.usage))}` : ""}${W.usage && W.usage.credits ? ` · ${t("Credits")} ${crN(W.usage.credits)}` : ""}`] : [];
    h += `<button class="cell wkc" data-w="${key(wk)}"${tipAttr(t("Week {n}", {n: isoWeek(wk)}), part.active ? [`${t("Active")} ${dur(part.active)}`, ...useLines(part), `${t("Git commits")} ${part.commits}`, ...whole] : whole.length ? [t("No records in this month's days"), ...whole] : t("No records"))}><span class="dn">${isJA() ? `${isoWeek(wk)}週` : `W${isoWeek(wk)}`}</span>${part.active ? calShort(part.active, part, un, kinds) : ""}</button>`;
    for (let c = 0; c < 7; c++){
      const d = addDays(wk, c), ds = d.getTime()/1000, de = addDays(d,1).getTime()/1000, inM = d.getMonth() === st.month.getMonth();
      if (!inM){ h += `<div class="cell out"><span class="dn">${d.getDate()}</span></div>`; continue; }
      const x = S ? S.days[d.getDate()-1] : null, act = x ? x.active : 0;
      const nS = shown.filter(s => inP(s, ds, de)).length;
      const nC = G ? G.filter(c => c.t >= ds && c.t < de).length : x && x.commits || 0;
      // 色分け（Color by）ごとの、その日の作業時間の細い帯。凡例で隠したもの・検索に合わないものは入らない
      const by = byDay.get(key(d)) || new Map();
      const tot = [...by.values()].reduce((a, v) => a + v, 0), mbar = tot ? `<span class="mbar" aria-hidden="true">${[...by].sort((a, b) => b[1] - a[1]).map(([k, v]) => `<i style="flex:${v};background:${colorOf(k)}"></i>`).join("")}</span>` : "";
      h += `<button class="cell${d.getDay()%6===0?" we":""}${wkc(d.getDay())}${key(d)===todayKey?" today":""}${d.getTime() > nowMs() ? " fut" : ""}${un && act && !nS ? " dim" : ""}" data-w="${key(mondayOf(d))}" data-d="${key(d)}" style="--heat:${(x ? SH[sk].f(x) / max : 0).toFixed(3)}"${tipAttr(md(ds), act ? [`${t("Active")} ${dur(act)}`, `${t("Sessions")} ${nS}`, ...useLines(x), `${t("Git commits")} ${nC}`] : t("No records"))}>
        <span class="dn">${d.getDate()}</span>${x.commits ? `<span class="acm" aria-label="${esc(`${t("Git commits")}: ${x.commits}`)}">${ico("commit")}${x.commits}</span>` : ""}${act ? calShort(act, x, un, kinds) : ""}${mbar}
        </button>`;
    }
  }
  h += `</div><div class="mlegend"><span class="mshade">${esc(t("Shade: {k}", {k: SH[sk].s}))}</span> <span class="nw">${esc(t("Less"))}</span> ${[0,.25,.5,.75,1].map(v=>`<i style="--h:${v}"></i>`).join("")} ${esc(t("More"))} · ${esc(matchMedia("(max-width:820px)").matches ? t("Week totals count this month's days. Tap a date or week to open it.") : t("Week totals count this month's days. Click a date or week to open it."))}</div>`;
  T.innerHTML = h;
  T.querySelectorAll("[data-w]").forEach(b => b.onclick = () => { const [y,m,dd] = b.dataset.w.split("-").map(Number); st.week = new Date(y, m-1, dd); tipOff(); setMode("week"); if (b.dataset.d) focusDay(b.dataset.d); });
}
// focusDay は、週のカレンダーで日 dk（YYYY-MM-DD）を見せる：横に流れる狭い画面ではその日の列までスクロールし、列を少し光らせ、
// その日の最初のセッション（なければ日の見出し）にフォーカスする（月のマスから来たとき、押した日を探し直さなくてよいように）
function focusDay(dk){
  const [y, m, d] = dk.split("-").map(Number), i = (new Date(y, m-1, d).getDay() + 6) % 7, col = $("#tl").querySelectorAll(".day")[i], head = $("#tl").querySelectorAll(".head")[i];
  if (!col){ const e = $("#tolast") || $("#tl .empty"); if (e){ if (!e.matches("button")) e.tabIndex = -1; e.focus({preventScroll: true}); } return; } // 記録のない週：押したマスは描き直しで消えるので、空の案内へ
  const sc = $("#tl .calscroll");
  if (sc && sc.scrollWidth > sc.clientWidth) sc.scrollLeft = Math.max(0, col.offsetLeft - (sc.querySelector(".hours") || {offsetWidth: 0}).offsetWidth);
  [col, head].forEach(e => { if (!e) return; e.classList.remove("flash"); void e.offsetWidth; e.classList.add("flash"); });
  const r = col.querySelector(".run"); (r || head).setAttribute?.("tabindex", r ? r.getAttribute("tabindex") || "0" : "-1"); (r || head).focus({preventScroll: true}); }

// 週のカレンダーの 0 時から CAL_D0 時は、ふだん畳んで（1 時間を 1/4 の高さで）描く。最初の画面に朝からの作業の時間帯が入り、
// 夜中の記録も正しい位置に小さく見える。左上の「0–7時」を押すと広げる・畳む（選んだものを覚える）。
// 選んでいなければ、その週の 0〜7 時にセッションがあるときは広げておく（実際の作業を潰さない）
const CAL_D0 = 7;
let earlyMemo = {}; // weekHasEarly は描画のたびに何百回も呼ばれる（calY）ので、週と DATA の組ごとに覚える
function weekHasEarly(){ const ws = st.week.getTime()/1000, we = addDays(st.week, 7).getTime()/1000;
  if (earlyMemo.ws === ws && earlyMemo.data === DATA) return earlyMemo.v;
  earlyMemo = {ws, data: DATA, v: weekHasEarlyScan(ws, we)}; return earlyMemo.v; }
function weekHasEarlyScan(ws, we){
  return DATA.some(s => s.segs.some(([a, b]) => { if (b <= ws || a >= we) return false;
    for (let d = 0; d < 7; d++){ const ds = addDays(st.week, d).getTime()/1000, x = new Date(ds*1000); x.setHours(CAL_D0); if (a < x.getTime()/1000 && b > ds) return true; } return false; })); }
const calFold = () => st.early == null ? !weekHasEarly() : !st.early; // 畳んでいるか
// calY は、時刻（時。0〜24）の、列の上からの位置（px）
function calY(h){ const hh = st.hh || 44; return !calFold() ? h * hh : h < CAL_D0 ? h * hh / 4 : CAL_D0 * hh / 4 + (h - CAL_D0) * hh; }

/* 週のカレンダー：曜日が列、時刻が縦（0 時から CAL_D0 時はふだん畳む）。重なるセッションだけ横に並べる */
function timeline(shown, inWeek, ws, we, todayKey){
  const T = $("#tl"), hh = st.hh || 44, Y = calY, H = Y(24), fold = calFold(), E0 = fold ? Y(CAL_D0) : 0;
  if (inWeek.length && !shown.length && st.q && !DATA.some(matches)){ T.innerHTML = ""; return; } // 検索がどこにも当たらないときは、上の検索結果の「ない」だけにする
  if (!inWeek.length || !shown.length){
    // 記録のない週は、ここで 1 度だけ「ない」と言い、記録のある最後の週へ行けるようにする（月は大文字のまま。toLowerCase で "jul 6" にしない）
    const last = !inWeek.length && !META.scope && DATA.length ? mondayOf(new Date(Math.max(...DATA.map(s => s.end))*1000)) : null, jump = last && last.getTime() !== st.week.getTime();
    emptyCal(T, inWeek.length ? t("No sessions match the current filters.") : META.scope ? t("This file has no records for this week.") : st.week.getTime() === mondayOf(today0()).getTime() ? t("No records this week.") : t("No records in the week of {date}.", {date: dMD(st.week)}),
      jump ? t("Go to the latest week with records ({date})", {date: dMD(last)}) : "", () => { st.week = last; st.sel = null; st.animate = true; render(); scrollToWork(); });
    return;
  }
  const keep = T.querySelector(".calscroll"), top = keep ? keep.scrollTop : null;
  const nowS = nowMs()/1000, w = WEEKS[key(st.week)];
  let heads = '<div></div>', hours = "", cols = "";
  const nowIn = nowS >= ws && nowS < we, nowCol = nowIn ? [0,1,2,3,4,5,6].find(d => nowS < addDays(st.week, d + 1).getTime()/1000) : -1, nowH = nowIn ? clockH(today0(), nowS) : -9;
  // 畳んだ時間帯には時刻を出さない（そこは「0–7」のボタン）。今の時刻の札（高さ約 20px）に重なる時刻は、消さずに札の上か下へずらして薄く出す（12:00 が見えなくならないように）
  for (let h=1; h<24; h++){ if (fold && h < CAL_D0) continue; const d = Y(h) - Y(nowH), near = nowIn && Math.abs(d) < 20;
    hours += `<span${near ? ' class="near"' : ""} style="top:${near ? Y(nowH) + (d >= 0 ? 20 : -20) : Y(h)}px">${String(h).padStart(2,"0")}:00</span>`; }
  hours += `<button type="button" class="calfold" id="calfold" style="height:${fold ? Math.max(18, Y(CAL_D0) - 14) : 18}px" aria-expanded="${!fold}" title="${esc(fold ? t("Show 0:00–7:00 at full height") : t("Fold 0:00–7:00"))}" aria-label="${esc(fold ? t("Show 0:00–7:00 at full height") : t("Fold 0:00–7:00"))}">${esc(t("0–7h"))}<svg viewBox="0 0 24 24" aria-hidden="true"><path d="${fold ? "M7 10l5 5 5-5" : "M7 14l5-5 5 5"}"/></svg></button>`;
  if (nowIn) hours += `<span class="now" style="top:${Y(nowH)}px">${hm(nowS)}</span>`;
  const runs = [], un = filtering() ? 1 : 0, G = gitShown(shown), kinds = useKinds(w ? w.days : []); kinds.k = calPick(w ? w.days : []).k;
  for (let d=0; d<7; d++){
    const day = addDays(st.week,d), ds = day.getTime()/1000, de = addDays(st.week,d+1).getTime()/1000, isToday = key(day) === todayKey;
    const act = w && w.days[d] ? w.days[d].active : 0;
    const dW = w && w.days[d], nS = shown.filter(s => inP(s, ds, de)).length;
    const dayGit = G.filter(c => c.t >= ds && c.t < de).sort((a,b) => a.t - b.t);
    heads += `<div class="head${isToday?" today":""}${wkc(day.getDay())}"><div class="dd"><b>${dMD(day)}</b><i>${dow(day.getDay())}</i>${isToday ? `<span class="tdy">${esc(t("Today"))}</span>` : ""}${(act || dayGit.length) ? `<button class="pill dayrun ai-copy" data-d="${d}" type="button" aria-label="${esc(t("Copy the daily report prompt for {date}", {date: md(ds)}))}" title="${esc(t("Copy the daily report prompt for {date}", {date: md(ds)}))}">${ico("copy")}<span>${esc(t("Copy prompt"))}</span></button>` : ""}</div>${act || dayGit.length ? `<div${tipAttr(md(ds), [`${t("Active")} ${dur(act)}`, `${t("Sessions")} ${nS}`, ...useLines(dW), `${t("Git commits")} ${dayGit.length}`])}>${calLine(calRows(act, dW, nS, dayGit.length, kinds, un))}</div>` : `<small>—</small>`}</div>`; // まだ来ていない日に「目安」は置かない（同じ額が上の帯の「前の週並みの 1 日の目安」にある）
    const blocks = [];
    shown.forEach(s => s.segs.forEach(([a,b,n]) => { const x = Math.max(a,ds), y = Math.min(b,de); if (y > x) blocks.push({s, a:x, b:y, n}); }));
    blocks.sort((p,q) => p.a-q.a || q.b-p.b);
    let html = "";
    blocks.forEach(bk => { bk.y = Y(clockH(day, bk.a)); bk.h = Math.max(4, Y(clockH(day, bk.b)) - bk.y - 2); });
    // 重なる帯の置き方（l・w は溝を除いた幅に対する左端と幅の割合）。ほぼ同時に始まった帯（CASCADE px 以内。名前の 1 行目が見える高さ）はかたまりにして横に並べ、
    // それより後に始まった帯は、前の帯の上に右へずらして重ねる。前の帯は名前のある上の部分が隠れないので、幅いっぱいのまま読める
    // （重なれば最後まで 1/L の幅にすると、並行して動かすことの多い人ほど名前が数文字しか読めない）。見た目で重なるか（短い帯の下の名前も入れる）で決める
    const CASCADE = 20, bot = o => o.y + o.h + (o.h < 10 ? 14 : 0), over = (o, bk) => o.y < bot(bk) && bot(o) > bk.y;
    const placed = [];
    for (let i = 0; i < blocks.length;){
      const grp = [blocks[i]]; let j = i + 1; // かたまり：最初の帯から CASCADE px 以内に始まり、かたまりのどれかと重なる帯
      while (j < blocks.length && blocks[j].y - blocks[i].y < CASCADE && grp.some(o => over(o, blocks[j]))) grp.push(blocks[j++]);
      const lanes = []; grp.forEach(bk => { let k = lanes.findIndex(e => e <= bk.y); if (k < 0){ k = lanes.length; lanes.push(0); } lanes[k] = bot(bk); bk.lane = k; });
      const base = Math.min(.6, Math.max(0, ...placed.filter(o => grp.some(bk => over(o, bk))).map(o => o.l + o.w * .35))); // 重なる前の帯の 35% 右から（何段も重なっても 4 割の幅は残す）
      grp.forEach(bk => { bk.w = (1 - base) / lanes.length; bk.l = base + bk.lane * bk.w; bk.cas = base > 0; placed.push(bk); });
      i = j;
    }
    // 名前の入らない短い帯は、すぐ下があいていれば、そこに名前を出す（ほかの帯と重なるなら出さない。押せば詳細は開ける）
    const free = bk => !blocks.some(o => o !== bk && o.y < bk.y + bk.h + 14 && o.y + o.h > bk.y + bk.h && o.l < bk.l + bk.w && o.l + o.w > bk.l);
    // 後から右に重なった帯があれば、前の帯の名前と時刻は、その帯の上端より上に収める（下の行が重なった帯の左にちぎれて見えないように）
    const room = bk => Math.min(bk.h, ...blocks.filter(o => o.cas && o.y > bk.y && o.y < bk.y + bk.h && o.l < bk.l + bk.w - 1e-6 && o.l + o.w > bk.l + 1e-6).map(o => o.y - bk.y - 2));
    blocks.forEach((bk, j) => {
      const {y, h} = bk, th = room(bk), mins = (bk.b-bk.a)/60;
      const dens = Math.min(1, bk.n / Math.max(1, mins) / 2.5), id = runs.length;
      const pos = `left:calc((100% - var(--g)) * ${bk.l.toFixed(4)} + 3px);width:calc((100% - var(--g)) * ${bk.w.toFixed(4)} - 6px)`;
      runs.push(bk);
      // 20px 以上は名前（高さに入るだけ 3 行まで折り返す。38px からは下に時刻も）、10px からは小さい字で名前を帯の中に 1 行（帯の外へはみ出して見えないように）、それより短いものは帯の下に名前。名前の入らない細い帯は始まりの時刻（style.css の .st）
      html += `<button class="run${h < 20 ? " thin" : ""}${bk.cas ? " cas" : ""}${st.sel === bk.s.id ? " sel" : ""}" data-r="${id}" data-sid="${esc(bk.s.id)}" style="top:${y}px;height:${h}px;${pos};--c:${colorOf(keyOf(bk.s))};--fill:${Math.round(16+30*dens)}%;--ln:${Math.max(1, Math.floor((th - 8) / 14))};--tl:${Math.max(1, Math.min(3, Math.floor((th - (th >= 38 ? 24 : 8)) / 14.3)))};${st.animate?`--delay:${d*15+Math.min(j,14)*5}ms`:"animation:none"}" aria-label="${esc(t("{title}, {project}, {date} {start} to {end}", {title: bk.s.title, project: bk.s.project, date: md(bk.a), start: hm(bk.a), end: bk.b >= de ? "24:00" : hm(bk.b)}))}">${h >= 10 ? `<span class="t"><span>${esc(bk.s.title)}</span></span><span class="st">${hm(bk.a)}</span>` + (th >= 38 ? `<span class="m">${hm(bk.a)}–${bk.b >= de ? "24:00" : hm(bk.b)} · ${esc(bk.s.project)}</span>` : "") : ""}</button>`;
      if (h < 10 && free(bk)) html += `<span class="rlab" aria-hidden="true" style="top:${y + h + 1}px;${pos}">${esc(bk.s.title)}</span>`;
    });
    // 右端の溝に、コミット・push・PR を時刻の順に置く（近すぎるものは少し下へずらす）
    const showP = p => (!st.hidden.size || st.colorBy !== "project" || !st.hidden.has(p.project)) && (!st.q || shown.some(s => s.project === p.project && p.t >= s.start && p.t <= s.end + 600)); // 検索中は、一致したセッションの間の push だけ（詳細のプロンプトの流れと同じ範囲）
    const marks = [...dayGit.map(c => ({t: c.t, c})),
      ...(META.push || []).filter(p => p.t >= ds && p.t < de && showP(p)).map(p => ({t: p.t, p})),
      ...shown.flatMap(s => (s.prAt || []).filter(x => x.t >= ds && x.t < de).map(x => ({t: x.t, r: x, s})))].map(m => ({...m, y0: Y(clockH(day, m.t))})).sort((a, b) => a.t - b.t);
    // 印の縦の位置：重ならないよう下へ積み、日の終わり（24:00）を越える分は上へ戻す（夜遅い印が枠の外に出て見えなくならないように）
    const ys = []; { let g = -99; marks.forEach(m => { g = Math.max(m.y0, g + 17); ys.push(g); });
      for (let i = ys.length - 1; i >= 0; i--) ys[i] = Math.min(ys[i], i === ys.length - 1 ? H - 16 : ys[i+1] - 17); }
    marks.forEach(({t: at, c, p, r, s}, mi) => { // 読み上げの名前には日付も入れる（セッションと同じく、どの日のものかわかるように）
      const y = ys[mi];
      if (p){ html += `<button class="gm push${st.sel === "push:" + pushKey(p) ? " sel" : ""}" data-push="${esc(pushKey(p))}" style="top:${y}px" title="${esc((p.commits ? t("{time} Pushed to {ref} ({commits}) · {project}", {time: hm(at), ref: p.ref, commits: plural(p.commits, "commit"), project: p.project}) : t("{time} Pushed to {ref} · {project}", {time: hm(at), ref: p.ref, project: p.project})) + "\n" + t("From this computer (git reflog)"))}" aria-label="${esc(t("Push {time} to {ref}", {time: md(at) + " " + hm(at), ref: p.ref}))}">${ico("push")}</button>`; return; }
      if (r){ html += `<button class="gm prm${st.sel === "pr:" + prKey(s, r) ? " sel" : ""}" data-pr="${esc(prKey(s, r))}" style="top:${y}px" title="${esc(t("{time} Created a pull request · {project}", {time: hm(at), project: s.project}) + (r.url ? "\n" + r.url : "") + "\n" + t("Recorded when an agent created it"))}" aria-label="${esc(t("Pull request {time}", {time: md(at) + " " + hm(at)}))}">${ico("pr")}</button>`; return; }
      html += `<button class="gc${c.ai ? " ai" : ""}${st.sel === "git:"+c.hash ? " sel" : ""}" data-c="${esc(c.hash)}" style="top:${y}px" title="${esc(`${hm(c.t)} ${c.project}${c.branch ? " · "+c.branch : ""} · ${c.hash}\n${c.subject}\n${plural(c.nFiles, "file")} +${c.added} −${c.removed}${c.ai ? " · " + t("run by AI") : ""}`)}" aria-label="${esc(t("Commit {time} {subject}", {time: md(c.t) + " " + hm(c.t), subject: c.subject}))}">${GIT_ICON}<span>${esc(c.hash.slice(0,7))}</span></button>`; });
    limitHits(ds, de).filter(h => matches(h.s)).forEach(h => { // 上限の印は、そのセッションの帯の左端に置く（隣の帯の上に出して、ほかの作業の印に見えないように）
      const bk = blocks.find(b => b.s === h.s && b.a <= h.t && h.t <= b.b) || blocks.find(b => b.s === h.s), left = bk ? `left:calc((100% - var(--g)) * ${bk.l.toFixed(4)} + 3px);` : "";
      html += `<button class="lim" data-id="${esc(h.s.id)}" data-t="${h.t}" style="${left}top:${Y(clockH(day, h.t))}px" title="${esc(t("{time} Hit the usage limit ({title})", {time: hm(h.t), title: h.s.title}))}" aria-label="${esc(t("{time} Hit the usage limit ({title})", {time: md(h.t) + " " + hm(h.t), title: h.s.title}))}">${ico("limit")}<span class="sr">${esc(t("Limit"))}</span></button>`; }); // 印だけ（狭い帯の名前を隠さないように。名前は読み上げと title）
    if (d === nowCol) html += `<div class="nowline" style="top:${Y(nowH)}px"></div>`;
    cols += `<div class="day${day.getDay()%6===0?" we":""}${wkc(day.getDay())}${isToday?" today":""}" style="height:${H}px;--g:${marks.length ? 18 : 0}px;--e0:${E0}px;--ch:${fold ? CAL_D0 : 0}">${html}</div>`;
  }
  // 曜日の見出しは、横にスクロールする本体（.calscroll）の外に置き、ページのスクロールで上に貼り付ける（.tlhead）。
  // 横のスクロールは本体に合わせて動かす（狭い画面でも、どの列が何日かわかるように）
  T.innerHTML = `<div class="tlhead"><div class="calin"><div class="heads">${heads}</div></div></div><div class="calscroll" style="--hh:${hh}px"><div class="calin"><div class="cgrid"><div class="hours" style="height:${H}px">${hours}</div>${cols}</div></div></div>`;
  const sc = T.querySelector(".calscroll"), th = T.querySelector(".tlhead");
  $("#calfold").onclick = () => { st.early = calFold(); store.set("calEarly", st.early); render(); const b = $("#calfold"); if (b) b.focus({preventScroll: true}); };
  sc.addEventListener("scroll", () => { th.scrollLeft = sc.scrollLeft; }, {passive: true});
  sc.style.scrollPaddingTop = T.querySelector(".heads").offsetHeight + "px"; sc.style.scrollPaddingLeft = "56px"; // Tab で移ったブロックが、固定の日付・時刻の下に隠れないように
  if (top != null) sc.scrollTop = top;
  fitRuns(T);
  sc.addEventListener("scroll", () => edgeFade(T), {passive: true}); edgeFade(T);
  // 上に固定した日付の見出しのボタン（日報の AI 分析）にフォーカスが移ると、ブラウザがそれを見せようとして 0 時まで巻き戻す。
  // フォーカスは見出しの中だけなので、巻き戻す前（まだ scroll が届いていない値）に戻す
  let seenTop = sc.scrollTop;
  sc.addEventListener("scroll", () => { seenTop = sc.scrollTop; }, {passive: true});
  sc.addEventListener("focusin", e => { if (e.target.closest(".heads") && sc.scrollTop !== seenTop) sc.scrollTop = seenTop; });
  // 日報のプロンプトのコピー（kiroku は AI を動かさない）
  T.querySelectorAll(".dayrun").forEach(b => b.onclick = e => { e.stopPropagation(); copyPrompt(b, reportPrompt(addDays(st.week, +b.dataset.d))); });
  T.querySelectorAll(".lim").forEach(el => el.onclick = e => { e.stopPropagation(); select(el.dataset.id); showFlowEvent("warn", +el.dataset.t); });
  bindGitEvents(T);
  T.querySelectorAll(".gc").forEach(el => el.onclick = e => { e.stopPropagation(); select("git:" + el.dataset.c); });
  T.querySelectorAll(".run").forEach(el => { const bk = runs[+el.dataset.r];
    el.onclick = e => { e.stopPropagation(); select(bk.s.id); };
    el.onmouseenter = e => tipOn(e, bk); el.onmousemove = tipMove; el.onmouseleave = tipOff;
    el.onfocus = () => { if (el.matches(":focus-visible")) tipOn(tipAt(el), bk); }; el.onblur = tipOff; }); // キーボードで移ったときも出す
}

// 細い帯で、名前の最初の語がまるごと入らず 4 文字（と …）も入らなければ、名前の代わりに始まりの時刻（.nt）、それも入らなければ色だけ（.nn）。
// 幅は描いたあとでないとわからないので、描くたびと、カレンダーの幅が変わるたびに測る
const fitCv = document.createElement("canvas").getContext("2d");
function fitRuns(T){
  const tw = (el, x) => { const c = getComputedStyle(el); fitCv.font = `${c.fontWeight} ${c.fontSize} ${c.fontFamily}`; return fitCv.measureText(x).width; }; // font の一括指定は tabular-nums があると空になるので、組み立てる
  T.querySelectorAll(".run").forEach(el => { const t = el.querySelector(".t"), s = el.querySelector(".st"); if (!t || !s) return;
    el.classList.remove("nt", "nn", "bw");
    const w = el.getBoundingClientRect().width - (el.classList.contains("thin") ? 12 : 14); if (w >= 64) return; // 広い帯は今までどおり。幅は端数まで（clientWidth は丸める）
    const name = t.textContent.trim(), first = name.split(/\s+/)[0];
    if (tw(t, first) <= w && (first.length >= 3 || tw(t, name.slice(0, 4)) <= w) || tw(t, name.slice(0, 4) + "…") <= w) return;
    // 背の高い帯なら、時刻に替えずに名前を単語の途中でも折り返して書く（幅の狭い画面でも、どのセッションかわかるように）
    if (el.getBoundingClientRect().height >= 40 && tw(t, name.slice(0, 3)) <= w){ el.classList.add("bw"); return; }
    el.classList.add(tw(s, "00:00") <= w + 12 ? "nt" : "nn"); }); // 時刻は数字の幅がそろう（tabular-nums）ので 0 で測る。.st は左右の余白に 6px ずつはみ出せる
}
if (window.ResizeObserver) new ResizeObserver(() => { const T = $("#tl"); if (T.querySelector(".run")) fitRuns(T); edgeFade(T); }).observe($("#tl"));
// edgeFade は、週のカレンダーが右へまだスクロールできるあいだだけ、右端をぼかす（.morer）。スクロールのたびと、描くたびに見直す
function edgeFade(T){ const sc = T.querySelector(".calscroll"); T.classList.toggle("morer", !!sc && sc.scrollLeft + sc.clientWidth < sc.scrollWidth - 2); }

/* ── tooltip ── */
function tipOn(e, bk){ const tp = $("#tip"), s = bk.s;
  tp.innerHTML = `<b>${esc(s.title)}</b><div class="r" style="--c:${colorOf(keyOf(s))}"><i></i>${esc(s.project)}${s.branch?` · ${esc(s.branch)}`:""}</div><div class="r">${md(bk.a)} ${hm(bk.a)}–${hm(bk.b)}${` (${dur((bk.b-bk.a)/60)})`}</div><div class="r">${esc((s.source))} · ${plural(s.nPrompts, "prompt")}${s.cost ? ` · ${usd(s.cost)}` : ""}${s.credits ? ` · ${cr(s.credits)}` : ""}${s.subagents.length ? ` · ${plural(s.subagents.length, "subagent")}` : ""}</div>`;

  tp.classList.add("on"); tipMove(e); }
function tipMove(e){ const t = $("#tip"), w = t.offsetWidth, h = t.offsetHeight;
  let x = e.clientX + 14, y = e.clientY + 16; if (x + w > innerWidth - 12) x = e.clientX - w - 14; if (y + h > innerHeight - 12) y = e.clientY - h - 14;
  t.style.left = x+"px"; t.style.top = y+"px"; }
function tipOff(){ $("#tip").classList.remove("on"); }
function tipAt(el){ const r = el.getBoundingClientRect(); return {clientX: r.left + Math.min(r.width/2, 24), clientY: r.top + Math.min(r.height, 28)}; } // フォーカスしたブロックの左上あたりを、マウスの位置の代わりに
// グラフの棒・帯・点は data-tip に書いた文を、マウスを載せる（タッチでは触れる）とすぐ出す。1 行目は見出し、続く行は「名前 値」
const useLines = d => d ? [d.tokens ? `${t("Tokens")} ${tok(d.tokens)}` : "", d.cost >= 0.005 ? `${t("Estimated cost")} ${usd(d.cost)}` : "", d.credits ? `${t("Credits")} ${crN(d.credits)}` : ""].filter(Boolean) : [];
const tipAttr = (...lines) => ` data-tip="${esc(lines.flat().filter(Boolean).join("\n"))}"`;
function tipText(e, text){ const t = $("#tip"), [h, ...r] = (text.includes("\n") ? text : text.replace(/・| · /g, "\n")).split("\n"); // 1 行で書いたもの（月表示の日など）は「・」で行に分ける
  t.innerHTML = `<b>${esc(h)}</b>${r.map(x => `<div class="r">${esc(x)}</div>`).join("")}`; t.classList.add("on"); tipMove(e); }
document.addEventListener("pointerover", e => { const el = e.target.closest && e.target.closest("[data-tip]"); if (el) tipText(e, el.dataset.tip); });
document.addEventListener("pointermove", e => { const el = e.target.closest && e.target.closest("[data-tip]"); if (!el) return; // スクロールで消えたあとも、動かせばまた出す
  if ($("#tip").classList.contains("on")) tipMove(e); else tipText(e, el.dataset.tip); });
document.addEventListener("pointerout", e => { const el = e.target.closest && e.target.closest("[data-tip]"); if (el && !(e.relatedTarget && el.contains(e.relatedTarget))) tipOff(); });
addEventListener("scroll", () => { if (st.sel) return;
  const a = document.activeElement, r = a && a.classList && a.classList.contains("run") && a.matches(":focus-visible") && $("#tip").classList.contains("on") && a.getBoundingClientRect();
  if (r && r.bottom > 0 && r.top < innerHeight) tipMove(tipAt(a)); else tipOff(); }, {passive: true, capture: true}); // キーボードで選んだブロックは、Tab で移ったときのスクロールで消さずに付いていく


