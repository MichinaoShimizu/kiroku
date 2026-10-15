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
  else { $("#ry").textContent = `${st.week.getFullYear()} · ${t("Week {n}", {n: isoWeek(st.week)})}${rel ? ` · ${rel}` : ""}`; $("#rd").innerHTML = `${dMD(st.week)}<span> – </span>${dMD(end)}`; }
  $("#today").textContent = META.scope ? (META.scope.mode === "month" ? t("Back to this file's month") : t("Back to this file's week")) : M ? t("This month") : t("This week");
  [["#prev", -1], ["#next", 1]].forEach(([id, n]) => { const b = $(id), off = !canGo(n); // 先の週・ファイルの外へは押せない
    if (off && document.activeElement === b) $(n > 0 ? "#prev" : "#next").focus(); // 押せなくなるボタンにいたフォーカスは、隣のボタンへ
    b.disabled = off; });
  $("#prev").setAttribute("aria-label", M ? t("Previous month") : t("Previous week")); $("#next").setAttribute("aria-label", M ? t("Next month") : t("Next week"));
  document.querySelectorAll("#mode button[data-v]").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.mode));

  const inRange = DATA.filter(s => s.end >= ws && s.start < we);
  const shown = inRange.filter(matches);
  kpis();
  // legend（この期間にあるものを、作業時間の多い順に。数はサマリーの「① By project」の配分と同じ作業時間）
  const cnt = {}; inRange.forEach(s => cnt[keyOf(s)] = (cnt[keyOf(s)]||0)+1);
  const mins = legendMinutes();
  const noHit = !!st.q && !DATA.some(matches); // 検索がどこにも当たらない（カレンダーは空になり、上の検索結果だけが「ない」と言う）
  // 件数は横にスクロールする並び（.lrow）の外に置く。スマホでは並びの上に出し、チップが多くても画面の外へ押し出されないように
  $("#legend").innerHTML = `<label class="cbsel"><span class="cbt">${esc(t("Color by"))}</span><select id="cb2">${Object.entries(CB()).map(([v,l]) => `<option value="${v}"${st.colorBy === v ? " selected" : ""}>${esc(l)}</option>`).join("")}</select></label><div class="lrow"><span class="lab">${esc(t("Active time"))}<small> · ${esc(t("click to hide"))}</small></span>` +
    Object.keys(cnt).sort((a,b)=>(mins[b]||0)-(mins[a]||0) || cnt[b]-cnt[a]).map(k => `<button class="chip" style="--c:${colorOf(k)}" data-k="${esc(k)}" aria-pressed="${!st.hidden.has(k)}" title="${esc(t("{name}: {stats} (click to show or hide)", {name: k, stats: `${mins[k] ? dur(mins[k]) + " · " : ""}${plural(cnt[k], "session")}`}))}"><span class="dot"></span>${st.colorBy === "source" ? agMark(k) : ""}<span class="cn">${esc((k))}</span><span class="n">${mins[k] ? dur(mins[k]) : "—"}<span class="sr"> ${esc(t("active"))}</span></span></button>`).join("") +
    `</div><span class="count">${esc(t("{n} / {total} sessions", {n: shown.length, total: inRange.length}))}</span>`;
  // 検索・凡例で絞り込んでいるあいだ、絞り込めない合計（作業時間・トークンなど）は薄くして、そう断る
  const fn = $("#fnote"); fn.hidden = !filtering() || noHit;
  fn.textContent = `${st.q ? t("This view shows only sessions matching the search and their linked records.") : t("Hidden items are left out of this view.")} ${M ? t("Gray figures, at the top and in each day and week, are totals for all sessions; sessions and commits there count only what is shown.") : t("Gray figures, at the top and under each date, are totals for all sessions; sessions and commits there count only what is shown.")}`;
  $("#legend").hidden = !inRange.length; // 記録のない期間は、色分け・件数を出さない（押しても何も変わらない）
  const cb2 = $("#cb2"); if (cb2) cb2.onchange = () => { st.colorBy = cb2.value; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); };
  document.querySelectorAll(".chip").forEach(c => c.onclick = () => { const k = c.dataset.k; st.hidden.has(k) ? st.hidden.delete(k) : st.hidden.add(k); render(); });

  if (!M) st.hh = fitHour(); // 1 時間の高さは、この週の作業の時間帯が枠に収まるように（44px か 36px）
  M ? monthGrid(shown, ws, we, todayKey) : timeline(shown, inRange, ws, we, todayKey);
  edgeFade($("#tl")); // 月表示や空の週では消す
  $("#skipcal").hidden = !!st.q; // 検索の最中は、結果がカレンダーの上にあり、下のサマリーは空
  lastFlags = []; // 見直す候補は表示中の期間のものだけ（検索中や記録のない期間に、前の期間の候補を詳細に出さない）
  $("#rptbar").hidden = true; // 検索の最中は出さない（summary が期間の記録があれば出す）
  try { st.q ? searchPanel() : summary(); } catch(e){ // サマリーで失敗しても、カレンダーと詳細は使えるようにする
    console.error(e); $("#worth").hidden = true; $("#review").innerHTML = `<div class="panel">${noneH(esc(t("Couldn't show the summary for this period ({error}). Please let us know in an issue.", {error: e.message})))}</div>`; }
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
  if (!open && focusDrawer.was) focusOpener(focusDrawer.first, focusDrawer.from);
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
    const xs = [[pc.cost, (v, txt) => txt ? usd(v) : usdH(v)], [pc.credits, (v, txt) => txt ? cr(v) : `${crN(v)}<small> cr</small>`]].filter(([x]) => x);
    const all = xs.every(([x]) => x.rest < 0);
    const v = xs.map(([x, f]) => x.rest >= 0 ? f(x.per) : `<small>+</small>${f(-x.rest)}<small> ${esc(t("over"))}</small>`).join(`<small> · </small>`);
    const ref = xs.map(([x, f]) => f(x.ref, true)).join(" · ");
    return kpi(all ? t(M ? "Past last month's total" : "Past last week's total") : t(M ? "A day to match last month" : "A day to match last week"), v,
      all ? t(M ? "Last month's total was {v}" : "Last week's total was {v}", {v: ref}) : t(M ? "To stay at last month's total of {v}, for the {n} left" : "To stay at last week's total of {v}, for the {n} left", {v: ref, n: plural(pc.left, "day")}), "key", pc.cost ? "pace" : "paceCr"); };
  const pace = pj ? t("If the pace of the first {n} days continues", {n: pj.days}) : "";
  // 帯は日々の判断に使う 5 つ前後だけ：使用量（推定費用とクレジット）・前の期間並みの 1 日の目安・月末の見込み（月）・稼働時間・残したもの（コミット）。
  // 稼働日・セッション / プロンプト・利用上限への到達は振り返りの数なので、サマリーと「見直す候補」に任せる（帯が多いとカレンダーと主役を争う）。
  // 推定費用とクレジットは別の枠の使用量なので足さずに、1 つの枠に「 · 」で並べる。トークンは推定費用を出せないときだけ
  const c = u.tokens ? costOf(u) : null, two = (a, b) => [a, b].filter(Boolean).join(`<small> · </small>`);
  const useV = two(u.tokens ? (c == null ? tok(u.tokens) : usdH(c)) : "", u.credits ? `${crN(u.credits)}<small> cr</small>` : "");
  const useK = u.tokens && u.credits ? t(c == null ? "Tokens · credits" : "Estimated cost · credits") : u.tokens ? t(c == null ? "Tokens" : "Estimated cost") : t("Kiro credits");
  const pjV = pj ? two(pj.cost != null ? `<small>≈</small>${usdH(pj.cost)}` : "", pj.credits != null ? `<small>≈</small>${crN(pj.credits)}<small> cr</small>` : "") : "";
  K.innerHTML = (useV ? kpi(useK, useV, u.tokens && c == null ? NOPRICE : "", "key", u.tokens ? (c == null ? "tokens" : "cost") : "credits") : "") +
    (pc ? paceK() : "") +
    (pjV ? kpi(t(pj.cost != null && pj.credits != null ? "Month-end (estimate)" : pj.cost != null ? "Month-end cost" : "Month-end credits"), pjV, pace, "key", pj.cost != null ? "projection" : "projectionCr") : "") +
    kpi(t("Active time"), dur(w.active, true), n0 == null ? "" : M ? t("{days} of the {n} so far (this month is still in progress)", {days, n: plural(nd, "day")}) : t("{days} of the {n} so far (this week is still in progress)", {days, n: plural(nd, "day")}), "", "active") +
    (w.git && w.git.commits ? kpi(t("Git commits"), `${w.git.commits}<small> · ${esc(t("{n} by AI", {n: w.git.ai}))}</small>`, "", "", "commits") :
     w.outputs && w.outputs.commits ? kpi(t("AI commits"), `${w.outputs.commits}`) : "");
  K.querySelectorAll("[data-metric]").forEach(b => b.onclick = () => openMetric(b.dataset.metric, b));
}

/* カレンダーの各日（月表示では各週も）に並べるのは 2 つだけ：使用量（推定費用。見積もれないならトークン、トークンがなければクレジット）と Git のコミット。
   費用と残したものを上下に並べ、費用が増えても残したものが増えていない日を、カレンダーを見るだけで拾えるようにする。
   作業時間は週では帯の長さ・月ではマスの濃さ、セッションは帯の数で見えるので、数は日のツールチップに（トークン・クレジットも）。
   見出しは期間の中でそろえる（日によって見出しが変わると、横に並べて比べられない）。
   un は絞り込み（検索・凡例で隠す）の最中に、絞り込まれていない合計を薄くする（1: 作業時間・トークン・クレジット、2: 全部） */
const useKinds = xs => ({tokens: xs.some(d => d && d.tokens), cost: xs.some(d => d && d.cost > 0), credits: xs.some(d => d && d.credits)});
function calRows(act, u, nS, nC, kinds, un){
  const k = un ? "k u" : "k", o = un > 1 ? "u" : "";
  const use = kinds.cost ? [t("Est. cost"), u && u.cost > 0 ? esc(usd(costOf(u))) : "—", k] : kinds.tokens ? [t("Tokens"), u && u.tokens ? tok(u.tokens) : "—", k] : kinds.credits && [t("Credits"), u && u.credits ? cr(u.credits) : "—", k];
  return [use, [t("Commits"), nC, o]].filter(Boolean);
}
const calDl = rows => `<dl class="cm">${rows.map(([k, v, c]) => `<div${c ? ` class="${c}"` : ""}><dt>${esc(k)}</dt><dd>${v}</dd></div>`).join("")}</dl>`; // k は強く出す値
// スマホでは作業時間と使用量だけ。トークンとクレジットは別の行に出す（両方あれば 2 行）。
// ひとつの枠にまとめると、Claude Code と Kiro を両方使う人には、片方しか見えないため。
function calShort(act, u, un, kinds){
  const q = un ? " u" : "";
  const v = !u ? "" : kinds.cost ? u.cost > 0 && usdS(u.cost) : kinds.tokens ? u.tokens && tokS(u.tokens) : u.credits && cr(u.credits);
  return `<span class="acs${q}">${esc(v || "—")}</span>`;
}
// allowH は、まだ来ていない日に置く「目安」：前の期間と同じ推定費用（クレジット）に収めるなら、残りの日 1 日あたりいくらか（pacing）。
// 前の期間をもう超えていれば出さない（0 と書くと「使うな」と読めるため。超えたことは上の帯が言う）。月のマスには出さない（同じ額が並ぶだけなので、帯の 1 か所で足りる）
function allowH(pc){
  const xs = [pc.cost && pc.cost.rest > 0 && usd(pc.cost.per), pc.credits && pc.credits.rest > 0 && cr(pc.credits.per)].filter(Boolean); if (!xs.length) return `<small>—</small>`;
  const tip = t(st.mode === "month" ? "To stay at last month's total, about this much a day for the {n} left" : "To stay at last week's total, about this much a day for the {n} left", {n: plural(pc.left, "day")});
  return `<div class="allow"${tipAttr(t("To match"), tip, ...xs)}><span>${esc(t("To match"))}</span>${xs.map(x => `<b>${esc(x)}</b>`).join("")}</div>`;
}
const filtering = () => !!st.q || st.hidden.size > 0;
/* カレンダーに出すコミット：凡例で隠したプロジェクトのものは出さない。検索中は、一致したセッションの間のコミットと、それ自体が一致したコミットだけ */
const gitHit = c => [c.hash, c.subject, c.body || "", ...(c.files || []).map(f => f.path)].join("\n").toLowerCase().includes(st.q);
function gitShown(shown){
  const linked = st.q ? new Set(shown.flatMap(s => commitsOf(s).map(c => c.hash))) : null;
  return (META.git || []).filter(c => (!st.hidden.size || st.colorBy !== "project" || !st.hidden.has(c.project)) && (!linked || linked.has(c.hash) || gitHit(c)));
}
/* 月のカレンダー：日ごとの作業時間を濃さで、プロジェクトの配分を細い帯で */
function monthGrid(shown, ms, me, todayKey){
  const T = $("#tl"), S = MONTHS[mkey(st.month)], first = mondayOf(st.month);
  const max = S ? Math.max(1, ...S.days.map(d => d.active)) : 1, un = filtering() ? 1 : 0, G = un ? gitShown(shown) : null;
  const wks = []; for (let r = 0; r < 6 && addDays(first, 7*r).getTime()/1000 < me; r++) wks.push(addDays(first, 7*r));
  const kinds = useKinds([...(S ? S.days : []), ...wks.map(wk => (WEEKS[key(wk)] || {}).usage)]);
  let h = `<div class="mgrid"><div class="mh"></div>${[1,2,3,4,5,6,0].map(i=>`<div class="mh${wkc(i)}">${dow(i)}</div>`).join("")}`;
  for (let r = 0; r < 6; r++){
    const wk = addDays(first, 7*r); if (wk.getTime()/1000 >= me) break;
    const W = WEEKS[key(wk)], wa = W ? W.active : 0, wc = W && W.git ? W.git.commits : 0; // 週の合計（月の外の日も含む、その週まるごと）
    h += `<button class="cell wkc" data-w="${key(wk)}"${tipAttr(t("Week {n}", {n: isoWeek(wk)}), W ? [`${t("Active")} ${dur(wa)}`, `${t("Sessions")} ${W.sessions}`, ...useLines(W.usage), `${t("Git commits")} ${wc}`] : t("No records"))}><span class="dn">${isJA() ? `${isoWeek(wk)}週` : `W${isoWeek(wk)}`}</span>${W && wa ? calDl(calRows(wa, W.usage, W.sessions, wc, kinds, un && 2)) + calShort(wa, W.usage, un, kinds) : ""}</button>`;
    for (let c = 0; c < 7; c++){
      const d = addDays(wk, c), ds = d.getTime()/1000, de = addDays(d,1).getTime()/1000, inM = d.getMonth() === st.month.getMonth();
      if (!inM){ h += `<div class="cell out"><span class="dn">${d.getDate()}</span></div>`; continue; }
      const x = S ? S.days[d.getDate()-1] : null, act = x ? x.active : 0;
      const by = {}; shown.forEach(s => s.segs.forEach(([a,b]) => { const o = Math.min(b,de) - Math.max(a,ds); if (o > 0) by[keyOf(s)] = (by[keyOf(s)]||0) + o; }));
      const pj = Object.entries(by).sort((a,b)=>b[1]-a[1]), nS = shown.filter(s => inP(s, ds, de)).length;
      const nC = G ? G.filter(c => c.t >= ds && c.t < de).length : x && x.commits || 0;
      h += `<button class="cell${d.getDay()%6===0?" we":""}${wkc(d.getDay())}${key(d)===todayKey?" today":""}" data-w="${key(mondayOf(d))}" style="--heat:${(act/max).toFixed(3)}"${tipAttr(md(ds), act ? [`${t("Active")} ${dur(act)}`, `${t("Sessions")} ${nS}`, ...useLines(x), `${t("Git commits")} ${nC}`] : t("No records"))}>
        <span class="dn">${d.getDate()}</span>${act ? calDl(calRows(act, x, nS, nC, kinds, un)) + calShort(act, x, un, kinds) : ""}
        ${pj.length ? `<span class="pj">${pj.map(([k,v])=>`<span style="flex:${v};--c:${colorOf(k)}"></span>`).join("")}</span>` : ""}</button>`;
    }
  }
  h += `</div><div class="mlegend"><span class="nw">${esc(t("Less"))}</span> ${[0,.25,.5,.75,1].map(v=>`<i style="--h:${v}"></i>`).join("")} ${esc(t("More"))} · ${esc(matchMedia("(max-width:820px)").matches ? t("Shade = active time. Week totals include all 7 days. Tap a date or week to open it.") : t("Shade = active time. Week totals include all 7 days. Click a date or week to open it."))}</div>`;
  T.innerHTML = h;
  T.querySelectorAll("[data-w]").forEach(b => b.onclick = () => { const [y,m,dd] = b.dataset.w.split("-").map(Number); st.week = new Date(y, m-1, dd); setMode("week"); });
}

/* 週のカレンダー：曜日が列、時刻が縦。重なるセッションだけ横に並べる */
function timeline(shown, inWeek, ws, we, todayKey){
  const T = $("#tl"), hh = st.hh || 44, H = 24*hh;
  if (inWeek.length && !shown.length && st.q && !DATA.some(matches)){ T.innerHTML = ""; return; } // 検索がどこにも当たらないときは、上の検索結果の「ない」だけにする
  if (!inWeek.length || !shown.length){
    // 記録のない週は、ここで 1 度だけ「ない」と言い、記録のある最後の週へ行けるようにする（月は大文字のまま。toLowerCase で "jul 6" にしない）
    const last = !inWeek.length && !META.scope && DATA.length ? mondayOf(new Date(Math.max(...DATA.map(s => s.end))*1000)) : null, jump = last && last.getTime() !== st.week.getTime();
    T.innerHTML = `<div class="empty"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="15" rx="3"/><path d="M3 10h18M8 3v4M16 3v4"/></svg><p>${esc(inWeek.length ? t("No sessions match the current filters.") : META.scope ? t("This file has no records for this week.") : st.week.getTime() === mondayOf(today0()).getTime() ? t("No records this week.") : t("No records in the week of {date}.", {date: dMD(st.week)}))}</p>${jump ? `<p><button class="pill" id="tolast">${esc(t("Go to the latest week with records ({date})", {date: dMD(last)}))}</button></p>` : ""}</div>`;
    if (jump) $("#tolast").onclick = () => { st.week = last; st.sel = null; st.animate = true; render(); scrollToWork(); };
    return;
  }
  const keep = T.querySelector(".calscroll"), top = keep ? keep.scrollTop : null;
  const nowS = nowMs()/1000, w = WEEKS[key(st.week)];
  let heads = '<div></div>', hours = "", cols = "";
  const nowH = nowS >= ws && nowS < we ? clockH(new Date(nowS*1000), nowS) : -9;
  for (let h=1; h<24; h++) if (Math.abs(h - nowH) * hh >= 20) hours += `<span style="top:${h*hh}px">${String(h).padStart(2,"0")}:00</span>`;
  if (nowS >= ws && nowS < we) hours += `<span class="now" style="top:${nowH*hh}px">${hm(nowS)}</span>`;
  const runs = [], un = filtering() ? 1 : 0, G = gitShown(shown), kinds = useKinds(w ? w.days : []);
  const pc = pacing(w, period().P), tmr = addDays(today0(), 1).getTime()/1000; // まだ来ていない日の見出しには、前の週並みに収める 1 日の目安
  for (let d=0; d<7; d++){
    const day = addDays(st.week,d), ds = day.getTime()/1000, de = addDays(st.week,d+1).getTime()/1000, isToday = key(day) === todayKey;
    const act = w && w.days[d] ? w.days[d].active : 0;
    const dW = w && w.days[d], nS = shown.filter(s => inP(s, ds, de)).length;
    const dayGit = G.filter(c => c.t >= ds && c.t < de).sort((a,b) => a.t - b.t);
    heads += `<div class="head${isToday?" today":""}${wkc(day.getDay())}"><div class="dd"><b>${dMD(day)}</b><i>${dow(day.getDay())}</i>${(act || dayGit.length) && aiRun() ? `<button class="pill dayrun ai-action" data-d="${d}" type="button" aria-label="${esc(t("Analyze with AI: daily report for {date}", {date: md(ds)}))}" title="${esc(t("Analyze with AI: daily report for {date}", {date: md(ds)}))}">${esc(t("Daily report"))}</button>` : ""}</div>${act || dayGit.length ? `<div${tipAttr(md(ds), [`${t("Active")} ${dur(act)}`, `${t("Sessions")} ${nS}`, ...useLines(dW), `${t("Git commits")} ${dayGit.length}`])}>${calDl(calRows(act, dW, nS, dayGit.length, kinds, un))}</div>` : pc && ds >= tmr ? allowH(pc) : `<small>—</small>`}</div>`;
    const blocks = [];
    shown.forEach(s => s.segs.forEach(([a,b,n]) => { const x = Math.max(a,ds), y = Math.min(b,de); if (y > x) blocks.push({s, a:x, b:y, n}); }));
    blocks.sort((p,q) => p.a-q.a || q.b-p.b);
    let html = "";
    blocks.forEach(bk => { bk.y = clockH(day, bk.a)*hh; bk.h = Math.max(4, (clockH(day, bk.b) - clockH(day, bk.a))*hh - 2); });
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
    blocks.forEach((bk, j) => {
      const {y, h} = bk, mins = (bk.b-bk.a)/60;
      const dens = Math.min(1, bk.n / Math.max(1, mins) / 2.5), id = runs.length;
      const pos = `left:calc((100% - var(--g)) * ${bk.l.toFixed(4)} + 3px);width:calc((100% - var(--g)) * ${bk.w.toFixed(4)} - 6px)`;
      runs.push(bk);
      // 20px 以上は名前（高さに入るだけ 3 行まで折り返す。38px からは下に時刻も）、10px からは小さい字で名前を帯の中に 1 行（帯の外へはみ出して見えないように）、それより短いものは帯の下に名前。名前の入らない細い帯は始まりの時刻（style.css の .st）
      html += `<button class="run${h < 20 ? " thin" : ""}${bk.cas ? " cas" : ""}${st.sel === bk.s.id ? " sel" : ""}" data-r="${id}" data-sid="${esc(bk.s.id)}" style="top:${y}px;height:${h}px;${pos};--c:${colorOf(keyOf(bk.s))};--fill:${Math.round(16+30*dens)}%;--ln:${Math.max(1, Math.floor((h - 8) / 14))};--tl:${Math.max(1, Math.min(3, Math.floor((h - (h >= 38 ? 24 : 8)) / 14.3)))};${st.animate?`--delay:${d*30+Math.min(j,14)*10}ms`:"animation:none"}" aria-label="${esc(t("{title}, {project}, {date} {start} to {end}", {title: bk.s.title, project: bk.s.project, date: md(bk.a), start: hm(bk.a), end: hm(bk.b)}))}">${h >= 10 ? `<span class="t"><span>${esc(bk.s.title)}</span></span><span class="st">${hm(bk.a)}</span>` + (h >= 38 ? `<span class="m">${hm(bk.a)}–${hm(bk.b)} · ${esc(bk.s.project)}</span>` : "") : ""}</button>`;
      if (h < 10 && free(bk)) html += `<span class="rlab" aria-hidden="true" style="top:${y + h + 1}px;${pos}">${esc(bk.s.title)}</span>`;
    });
    // 右端の溝に、コミット・push・PR を時刻の順に置く（近すぎるものは少し下へずらす）
    const showP = p => (!st.hidden.size || st.colorBy !== "project" || !st.hidden.has(p.project)) && (!st.q || shown.some(s => s.project === p.project && p.t >= s.start && p.t <= s.end + 600)); // 検索中は、一致したセッションの間の push だけ（詳細のプロンプトの流れと同じ範囲）
    const marks = [...dayGit.map(c => ({t: c.t, c})),
      ...(META.push || []).filter(p => p.t >= ds && p.t < de && showP(p)).map(p => ({t: p.t, p})),
      ...shown.flatMap(s => (s.prAt || []).filter(x => x.t >= ds && x.t < de).map(x => ({t: x.t, r: x, s})))].sort((a, b) => a.t - b.t);
    let gy = -99;
    marks.forEach(({t: at, c, p, r, s}) => { // 読み上げの名前には日付も入れる（セッションと同じく、どの日のものかわかるように）
      const y = Math.max(clockH(day, at)*hh, gy + 17); gy = y;
      if (p){ html += `<button class="gm push${st.sel === "push:" + pushKey(p) ? " sel" : ""}" data-push="${esc(pushKey(p))}" style="top:${y}px" title="${esc((p.commits ? t("{time} Pushed to {ref} ({commits}) · {project}", {time: hm(at), ref: p.ref, commits: plural(p.commits, "commit"), project: p.project}) : t("{time} Pushed to {ref} · {project}", {time: hm(at), ref: p.ref, project: p.project})) + "\n" + t("From this computer (git reflog)"))}" aria-label="${esc(t("Push {time} to {ref}", {time: md(at) + " " + hm(at), ref: p.ref}))}">${ico("push")}</button>`; return; }
      if (r){ html += `<button class="gm prm${st.sel === "pr:" + prKey(s, r) ? " sel" : ""}" data-pr="${esc(prKey(s, r))}" style="top:${y}px" title="${esc(t("{time} Created a pull request · {project}", {time: hm(at), project: s.project}) + (r.url ? "\n" + r.url : "") + "\n" + t("Recorded when an agent created it"))}" aria-label="${esc(t("Pull request {time}", {time: md(at) + " " + hm(at)}))}">${ico("pr")}</button>`; return; }
      html += `<button class="gc${c.ai ? " ai" : ""}${st.sel === "git:"+c.hash ? " sel" : ""}" data-c="${esc(c.hash)}" style="top:${y}px" title="${esc(`${hm(c.t)} ${c.project}${c.branch ? " · "+c.branch : ""} · ${c.hash}\n${c.subject}\n${plural(c.nFiles, "file")} +${c.added} −${c.removed}${c.ai ? " · " + t("run by AI") : ""}`)}" aria-label="${esc(t("Commit {time} {subject}", {time: md(c.t) + " " + hm(c.t), subject: c.subject}))}">${GIT_ICON}<span>${esc(c.hash.slice(0,7))}</span></button>`; });
    limitHits(ds, de).filter(h => matches(h.s)).forEach(h => { html += `<button class="lim" data-id="${esc(h.s.id)}" style="top:${clockH(day, h.t)*hh}px" title="${esc(t("{time} Hit the usage limit ({title})", {time: hm(h.t), title: h.s.title}))}" aria-label="${esc(t("{time} usage limit", {time: md(h.t) + " " + hm(h.t)}))}">${ico("limit")}${esc(t("Limit"))}</button>`; });
    if (isToday && nowS >= ds && nowS < de) html += `<div class="nowline" style="top:${clockH(day, nowS)*hh}px"></div>`;
    cols += `<div class="day${day.getDay()%6===0?" we":""}${wkc(day.getDay())}${isToday?" today":""}" style="height:${H}px;--g:${marks.length ? 18 : 0}px">${html}</div>`;
  }
  T.innerHTML = `<div class="calscroll" style="--hh:${hh}px"><div class="calin"><div class="heads">${heads}</div><div class="cgrid"><div class="hours" style="height:${H}px">${hours}</div>${cols}</div></div></div>`;
  const sc = T.querySelector(".calscroll");
  sc.style.scrollPaddingTop = T.querySelector(".heads").offsetHeight + "px"; sc.style.scrollPaddingLeft = "56px"; // Tab で移ったブロックが、固定の日付・時刻の下に隠れないように
  if (top != null) sc.scrollTop = top;
  fitRuns(T);
  sc.addEventListener("scroll", () => edgeFade(T), {passive: true}); edgeFade(T);
  // 上に固定した日付の見出しのボタン（日報の AI 分析）にフォーカスが移ると、ブラウザがそれを見せようとして 0 時まで巻き戻す。
  // フォーカスは見出しの中だけなので、巻き戻す前（まだ scroll が届いていない値）に戻す
  let seenTop = sc.scrollTop;
  sc.addEventListener("scroll", () => { seenTop = sc.scrollTop; }, {passive: true});
  sc.addEventListener("focusin", e => { if (e.target.closest(".heads") && sc.scrollTop !== seenTop) sc.scrollTop = seenTop; });
  // 日報の AI 分析（kiroku serve で、使えるエージェントがあるときだけ。週報と同じ作りで、その日の分だけ）
  T.querySelectorAll(".dayrun").forEach(b => b.onclick = e => { e.stopPropagation(); const dd = addDays(st.week, +b.dataset.d);
    runAIReview(b, reportPrompt(null, false, dd), t("Daily report for {date}", {date: md(dd.getTime() / 1000)})); });
  T.querySelectorAll(".lim").forEach(el => el.onclick = e => { e.stopPropagation(); select(el.dataset.id); });
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
    el.classList.remove("nt", "nn");
    const w = el.getBoundingClientRect().width - (el.classList.contains("thin") ? 12 : 14); if (w >= 64) return; // 広い帯は今までどおり。幅は端数まで（clientWidth は丸める）
    const name = t.textContent.trim(), first = name.split(/\s+/)[0];
    if (tw(t, first) <= w && (first.length >= 3 || tw(t, name.slice(0, 4)) <= w) || tw(t, name.slice(0, 4) + "…") <= w) return;
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
const useLines = d => d ? [d.tokens ? `${t("Tokens")} ${tok(d.tokens)}` : "", d.cost >= 0.005 ? `${t("Estimated cost")} ${usd(d.cost)}` : "", d.credits ? `${t("Credits")} ${cr(d.credits)}` : ""].filter(Boolean) : [];
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
