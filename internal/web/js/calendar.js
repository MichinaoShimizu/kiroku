// 描画の入口 render、上の帯（要点）、週と月のカレンダー、ツールチップ
/* ── render ── */
const HOURS = [28, 36, 44, 56, 72, 96]; // 1 時間の高さ（ズーム）
function period(){ // 今見ている期間 [ws, we) と、その集計
  if (st.mode === "month"){ const a = st.month, b = new Date(a.getFullYear(), a.getMonth()+1, 1); return {ws:a.getTime()/1000, we:b.getTime()/1000, S:MONTHS[mkey(a)], P:MONTHS[mkey(new Date(a.getFullYear(), a.getMonth()-1, 1))]}; }
  return {ws:st.week.getTime()/1000, we:addDays(st.week,7).getTime()/1000, S:WEEKS[key(st.week)], P:WEEKS[key(addDays(st.week,-7))]};
}
function render(){
  assignColors();
  const {ws, we} = period(), M = st.mode === "month";
  const todayKey = key(today0()), end = addDays(st.week,6);
  const rel = relName(); // 今週・先週（今月・先月）なら、日付の上にそう書く（開いたときに、どの期間を見ているかわかるように）
  if (M){ $("#ry").textContent = `${st.month.getFullYear()} · ${rel || "Month"}`; $("#rd").innerHTML = `${MONTH[st.month.getMonth()]} <span>${st.month.getFullYear()}</span>`; }
  else { $("#ry").textContent = `${st.week.getFullYear()} · Week ${isoWeek(st.week)}${rel ? ` · ${rel}` : ""}`; $("#rd").innerHTML = `${dMD(st.week)}<span> – </span>${dMD(end)}`; }
  $("#today").textContent = META.scope ? `Back to included ${META.scope.mode === "month" ? "month" : "week"}` : M ? "This month" : "This week";
  [["#prev", -1], ["#next", 1]].forEach(([id, n]) => { const b = $(id), off = !canGo(n); // 先の週・ファイルの外へは押せない
    if (off && document.activeElement === b) $(n > 0 ? "#prev" : "#next").focus(); // 押せなくなるボタンにいたフォーカスは、隣のボタンへ
    b.disabled = off; });
  $("#yrbtn").textContent = "Year in review"; $("#yrbtn").hidden = !YEAR_ON || !DATA.length;
  $("#prev").setAttribute("aria-label", M ? "Previous month" : "Previous week"); $("#next").setAttribute("aria-label", M ? "Next month" : "Next week");
  document.querySelectorAll("#mode button").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.mode));

  const inRange = DATA.filter(s => s.end >= ws && s.start < we);
  const shown = inRange.filter(matches);
  kpis();
  // legend（この期間にあるものを、作業時間の多い順に。数はサマリーの「① By project」の配分と同じ作業時間）
  const cnt = {}; inRange.forEach(s => cnt[keyOf(s)] = (cnt[keyOf(s)]||0)+1);
  const mins = legendMinutes();
  // 件数は横にスクロールする並び（.lrow）の外に置く。スマホでは並びの上に出し、チップが多くても画面の外へ押し出されないように
  $("#legend").innerHTML = `<label class="cbsel"><span class="cbt">Color by</span><select id="cb2">${Object.entries(CB()).map(([v,l]) => `<option value="${v}"${st.colorBy === v ? " selected" : ""}>${l}</option>`).join("")}</select></label><div class="lrow"><span class="lab">Active time<small> · click to hide</small></span>` +
    Object.keys(cnt).sort((a,b)=>(mins[b]||0)-(mins[a]||0) || cnt[b]-cnt[a]).map(k => `<button class="chip" style="--c:${colorOf(k)}" data-k="${esc(k)}" aria-pressed="${!st.hidden.has(k)}" title="${esc(`${(k)}: ${mins[k] ? dur(mins[k]) + " · " : ""}${plural(cnt[k], "session")} (click to show or hide)`)}"><span class="dot"></span>${st.colorBy === "source" ? agMark(k) : ""}${esc((k))}<span class="n">${mins[k] ? dur(mins[k]) : "—"}<span class="sr"> active</span></span></button>`).join("") +
    `</div>${M ? "" : `<span class="zlab" aria-hidden="true">Zoom</span><div class="segc zoom" role="group" aria-label="Zoom"><button id="zout" aria-label="Zoom out">−</button><button id="zin" aria-label="Zoom in">+</button></div>`}<span class="count">${shown.length} / ${inRange.length} sessions</span>`;
  if (!M){ $("#zin").onclick = () => zoom(1); $("#zout").onclick = () => zoom(-1); }
  // 検索・凡例で絞り込んでいるあいだ、絞り込めない合計（作業時間・トークンなど）は薄くして、そう断る
  const fn = $("#fnote"); fn.hidden = !filtering();
  fn.textContent = `${st.q ? "The calendar shows only sessions that match the search, and their commits." : "Hidden items are left out of the calendar."} Grey figures, at the top and ${M ? "in each day and week" : "under each date"}, are totals for all sessions; sessions and commits there count only what is shown.`;
  $("#legend").hidden = !inRange.length; // 記録のない期間は、色分け・ズーム・件数を出さない（押しても何も変わらない）
  const cb2 = $("#cb2"); if (cb2) cb2.onchange = () => { st.colorBy = cb2.value; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); };
  document.querySelectorAll(".chip").forEach(c => c.onclick = () => { const k = c.dataset.k; st.hidden.has(k) ? st.hidden.delete(k) : st.hidden.add(k); render(); });

  if (!M && st.zAuto) st.z = fitZoom(); // ズームを選んでいなければ、この週の作業の時間帯が枠に収まる高さに
  M ? monthGrid(shown, ws, we, todayKey) : timeline(shown, inRange, ws, we, todayKey);
  edgeFade($("#tl")); // 月表示や空の週では消す
  $("#skipcal").hidden = !!st.q; // 検索の最中は、結果がカレンダーの上にあり、下のサマリーは空
  lastFlags = []; // 見直す候補は表示中の期間のものだけ（検索中や記録のない期間に、前の期間の候補を詳細に出さない）
  try { st.q ? searchPanel() : summary(); } catch(e){ // サマリーで失敗しても、カレンダーと詳細は使えるようにする
    console.error(e); $("#worth").hidden = true; $("#review").innerHTML = `<div class="panel">${noneH(`Couldn't show the summary for this period (${esc(e.message)}). Please let us know in an issue.`)}</div>`; }
  const gc = st.sel && st.sel.startsWith("git:") && (META.git || []).find(x => "git:" + x.hash === st.sel);
  const pu = st.sel && st.sel.startsWith("push:") && findPush(st.sel.slice(5)), pr = st.sel && st.sel.startsWith("pr:") && findPR(st.sel.slice(3));
  const s = !gc && !pu && !pr && st.sel && DATA.find(x => x.id === st.sel), open = !!(s || gc || pu || pr);
  if (st.sel && !open) st.sel = null;
  if (s) detail(s); else if (gc) commitDetail(gc); else if (pu) pushDetail(pu); else if (pr) prDetail(pr.s, pr.r);
  if (open){ $("#dkind").textContent = s ? "Session details" : gc ? "Commit details" : pu ? "Push details" : "Pull request details"; // 読み上げで、何の詳細かと題名がわかるように
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
function relName(){ const t = today0();
  if (st.mode === "month"){ const m = monthOf(t), d = (m.getFullYear() - st.month.getFullYear()) * 12 + m.getMonth() - st.month.getMonth();
    return d === 0 ? "This month" : d === 1 ? "Last month" : ""; }
  const d = Math.round((mondayOf(t) - st.week) / 864e5);
  return d === 0 ? "This week" : d >= 6 && d <= 8 ? "Last week" : ""; } // 夏時間をまたぐ週は 7 日ちょうどにならないので、日数を丸めて
// legendMinutes は、凡例の各くくりの作業時間（分）。同時に動いたセッションの時間は分けて数える（サマリーの配分と同じ）
function legendMinutes(){ const {S: w} = period(), m = {}; if (!w) return m;
  if (st.colorBy === "project") (w.projects || []).forEach(([k, v]) => m[k] = v);
  else ((w.shares || {})[st.colorBy] || []).forEach(x => m[x.key] = x.minutes);
  return m; }
// shownName は、表示中の期間の呼び名。今週・今月なら "This week" / "This month"、ほかは "Week of Sep 28" / "September 2026"
function shownName(){ const M = st.mode === "month", t = today0();
  if (M) return st.month.getTime() === monthOf(t).getTime() ? "This month" : dMY(st.month);
  return st.week.getTime() === mondayOf(t).getTime() ? "This week" : `Week of ${dMD(st.week)}`; }
/* 期間の要点（上の帯） */
function kpis(){
  const {S:w} = period(), K = $("#kpis"), M = st.mode === "month";
  K.classList.toggle("unf", filtering()); // 絞り込みの最中も、ここは全セッションの合計（薄くして、カレンダーの上に断り書き）
  if (!w){ K.innerHTML = `<div class="kpi"><div class="k">${META.scope ? "Not in this file" : shownName()}</div><div class="v">No records</div></div>`; return; }
  // 押すと内訳のダイアログ（metric.js）。id のない数は押せない
  const kpi = (k, v, t, cls, id) => { const tg = id ? "button" : "div";
    return `<${tg} class="kpi${cls ? " " + cls : ""}"${id ? ` type="button" data-metric="${id}" aria-haspopup="dialog"` : ""}${t || id ? ` title="${esc(t ? t + (id ? ". Click for the breakdown" : "") : "Click for the breakdown")}"` : ""}><div class="k">${k}</div><div class="v">${v}</div></${tg}>`; };
  const u = w.usage || {}, days = w.days.filter(d => d.active).length, pj = projection(w);
  const n0 = soFar(), nd = n0 == null ? w.days.length : Math.min(n0, w.days.length); // 途中の週・月は、まだ来ていない日を分母に入れない（今日までの日数）
  const pace = pj ? `Estimate: if the pace of the first ${pj.days} days continues` : "";
  // トークン・目安コスト・月末の見込み・クレジットは、いちばん大事な数なので先頭にまとめて目立たせる
  K.innerHTML = (u.tokens ? kpi("Tokens", tok(u.tokens), "", "key", "tokens") + kpi("Estimated cost", usdH(costOf(u)), costOf(u) == null ? NOPRICE : "", "key", "cost") : "") +
    (pj && pj.cost != null ? kpi("Month-end cost", `<small>≈</small>${usdH(pj.cost)}`, pace, "key", "projection") : "") +
    (u.credits ? kpi("Kiro credits", `${crN(u.credits)}<small>cr</small>`, "", "key", "credits") : "") +
    (pj && pj.credits != null ? kpi("Month-end credits", `<small>≈</small>${crN(pj.credits)}<small>cr</small>`, pace, "key", "projectionCr") : "") +
    kpi("Active time", dur(w.active, true), "", "", "active") + kpi("Active days", `${days}<small> of ${nd}${n0 == null ? "" : " so far"}</small>`, n0 == null ? "" : `${days} of the ${plural(nd, "day")} so far (${M ? "this month" : "this week"} is still in progress)`, "", "days") +
    kpi("Sessions / prompts", `${w.sessions}<small>/</small>${w.prompts}`, "", "", "sessions") +
    (() => { const {ws, we} = period(), n = limitHits(ws, we).length; return n ? kpi("Usage limit hits", `<span class="wv">${n}</span>`, "", "", "limits") : ""; })() +
    (w.git && w.git.commits ? kpi("Git commits", `${w.git.commits}<small> · ${w.git.ai} by AI</small>`, "", "", "commits") :
     w.outputs && w.outputs.commits ? kpi("AI commits", `${w.outputs.commits}`) : "");
  K.querySelectorAll("[data-metric]").forEach(b => b.onclick = () => openMetric(b.dataset.metric, b));
}

/* カレンダーの各日（月表示では各週も）に並べる：作業時間・トークン・クレジット・セッション・Git のコミット。
   トークンとクレジットは、その期間にあるほう（両方あれば両方）を、どの日にも同じ見出しで出す（日によって見出しが変わると、横に並べて比べられない）。
   un は絞り込み（検索・凡例で隠す）の最中に、絞り込まれていない合計を薄くする（1: 作業時間・トークン・クレジット、2: 全部） */
const useKinds = xs => ({tokens: xs.some(d => d && d.tokens), credits: xs.some(d => d && d.credits)});
function calRows(act, u, nS, nC, kinds, un){
  const k = un ? "k u" : "k", o = un > 1 ? "u" : "";
  return [["Active", act ? dur(act) : "—", k], kinds.tokens && ["Tokens", u && u.tokens ? tok(u.tokens) : "—", k], kinds.credits && ["Credits", u && u.credits ? cr(u.credits) : "—", k],
    ["Sessions", nS, o], ["Commits", nC, o]].filter(Boolean);
}
const calDl = rows => `<dl class="cm">${rows.map(([k, v, c]) => `<div${c ? ` class="${c}"` : ""}><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>`; // k は強く出す値
// スマホでは作業時間と使用量だけ。トークンとクレジットは別の行に出す（両方あれば 2 行）。
// ひとつの枠にまとめると、Claude Code と Kiro を両方使う人には、片方しか見えないため。
function calShort(act, u, un){
  const q = un ? " u" : "";
  const use = [u && u.tokens ? tokS(u.tokens) : "", u && u.credits ? cr(u.credits) : ""].filter(Boolean);
  return `<span class="acs${q}">${act >= 60 ? (act/60).toFixed(1)+"h" : act+"m"}</span>${use.map(v => `<span class="uss${q}">${v}</span>`).join("")}`;
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
    h += `<button class="cell wkc" data-w="${key(wk)}"${tipAttr(`Week ${isoWeek(wk)}`, W ? [`Active ${dur(wa)}`, `Sessions ${W.sessions}`, ...useLines(W.usage), `Git commits ${wc}`] : "No records")}><span class="dn">W${isoWeek(wk)}</span>${W && wa ? calDl(calRows(wa, W.usage, W.sessions, wc, kinds, un && 2)) + calShort(wa, W.usage, un) : ""}</button>`;
    for (let c = 0; c < 7; c++){
      const d = addDays(wk, c), ds = d.getTime()/1000, de = addDays(d,1).getTime()/1000, inM = d.getMonth() === st.month.getMonth();
      if (!inM){ h += `<div class="cell out"><span class="dn">${d.getDate()}</span></div>`; continue; }
      const x = S ? S.days[d.getDate()-1] : null, act = x ? x.active : 0;
      const by = {}; shown.forEach(s => s.segs.forEach(([a,b]) => { const o = Math.min(b,de) - Math.max(a,ds); if (o > 0) by[keyOf(s)] = (by[keyOf(s)]||0) + o; }));
      const pj = Object.entries(by).sort((a,b)=>b[1]-a[1]), nS = shown.filter(s => inP(s, ds, de)).length;
      const nC = G ? G.filter(c => c.t >= ds && c.t < de).length : x && x.commits || 0;
      h += `<button class="cell${d.getDay()%6===0?" we":""}${wkc(d.getDay())}${key(d)===todayKey?" today":""}" data-w="${key(mondayOf(d))}" style="--heat:${(act/max).toFixed(3)}"${tipAttr(md(ds), act ? [`Active ${dur(act)}`, `Sessions ${nS}`, ...useLines(x), `Git commits ${nC}`] : "No records")}>
        <span class="dn">${d.getDate()}</span>${act ? calDl(calRows(act, x, nS, nC, kinds, un)) + calShort(act, x, un) : ""}
        ${pj.length ? `<span class="pj">${pj.map(([k,v])=>`<span style="flex:${v};--c:${colorOf(k)}"></span>`).join("")}</span>` : ""}</button>`;
    }
  }
  h += `</div><div class="mlegend"><span class="nw">Less</span> ${[0,.25,.5,.75,1].map(v=>`<i style="--h:${v}"></i>`).join("")} ${matchMedia("(max-width:820px)").matches ? "More (active time) · Each day and week shows active time, and tokens or credits (both when the month has both). A week counts all 7 days, including those in the next or previous month · Tap a date or week to open it" : "More (active time) · Each day, and each week on the left, shows active time, tokens or credits (both when the month has both), sessions and Git commits. A week counts all 7 days, including those in the next or previous month · Click a date or week to open it"}</div>`;
  T.innerHTML = h;
  T.querySelectorAll("[data-w]").forEach(b => b.onclick = () => { const [y,m,dd] = b.dataset.w.split("-").map(Number); st.week = new Date(y, m-1, dd); setMode("week"); });
}

/* 週のカレンダー：曜日が列、時刻が縦。重なるセッションだけ横に並べる */
function timeline(shown, inWeek, ws, we, todayKey){
  const T = $("#tl"), hh = HOURS[st.z] || 44, H = 24*hh;
  if (!inWeek.length || !shown.length){
    // 記録のない週は、ここで 1 度だけ「ない」と言い、記録のある最後の週へ行けるようにする（月は大文字のまま。toLowerCase で "jul 6" にしない）
    const last = !inWeek.length && !META.scope && DATA.length ? mondayOf(new Date(Math.max(...DATA.map(s => s.end))*1000)) : null, jump = last && last.getTime() !== st.week.getTime();
    T.innerHTML = `<div class="empty"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="15" rx="3"/><path d="M3 10h18M8 3v4M16 3v4"/></svg><p>${inWeek.length ? "No sessions match the current filters" : META.scope ? "This file has no records for this week." : `No records ${shownName() === "This week" ? "this week" : `in the week of ${dMD(st.week)}`}.`}</p>${jump ? `<p><button class="pill" id="tolast">Go to the latest week with records (${dMD(last)})</button></p>` : ""}</div>`;
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
  for (let d=0; d<7; d++){
    const day = addDays(st.week,d), ds = day.getTime()/1000, de = addDays(st.week,d+1).getTime()/1000, isToday = key(day) === todayKey;
    const act = w && w.days[d] ? w.days[d].active : 0;
    const dW = w && w.days[d], nS = shown.filter(s => inP(s, ds, de)).length;
    const dayGit = G.filter(c => c.t >= ds && c.t < de).sort((a,b) => a.t - b.t);
    heads += `<div class="head${isToday?" today":""}${wkc(day.getDay())}"><div class="dd"><b>${dMD(day)}</b><i>${dow(day.getDay())}</i></div>${act || dayGit.length ? `<div${tipAttr(md(ds), [`Active ${dur(act)}`, `Sessions ${nS}`, ...useLines(dW), `Git commits ${dayGit.length}`])}>${calDl(calRows(act, dW, nS, dayGit.length, kinds, un))}</div>` : `<small>—</small>`}</div>`;
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
      html += `<button class="run${h < 20 ? " thin" : ""}${bk.cas ? " cas" : ""}${st.sel === bk.s.id ? " sel" : ""}" data-r="${id}" data-sid="${esc(bk.s.id)}" style="top:${y}px;height:${h}px;${pos};--c:${colorOf(keyOf(bk.s))};--fill:${Math.round(16+30*dens)}%;--ln:${Math.max(1, Math.floor((h - 8) / 14))};--tl:${Math.max(1, Math.min(3, Math.floor((h - (h >= 38 ? 24 : 8)) / 14.3)))};${st.animate?`--delay:${d*30+Math.min(j,14)*10}ms`:"animation:none"}" aria-label="${esc(`${bk.s.title}, ${bk.s.project}, ${md(bk.a)} ${hm(bk.a)} to ${hm(bk.b)}`)}">${h >= 10 ? `<span class="t"><span>${esc(bk.s.title)}</span></span><span class="st">${hm(bk.a)}</span>` + (h >= 38 ? `<span class="m">${hm(bk.a)}–${hm(bk.b)} · ${esc(bk.s.project)}</span>` : "") : ""}</button>`;
      if (h < 10 && free(bk)) html += `<span class="rlab" aria-hidden="true" style="top:${y + h + 1}px;${pos}">${esc(bk.s.title)}</span>`;
    });
    // 右端の溝に、コミット・push・PR を時刻の順に置く（近すぎるものは少し下へずらす）
    const showP = p => (!st.hidden.size || st.colorBy !== "project" || !st.hidden.has(p.project)) && (!st.q || shown.some(s => s.project === p.project && p.t >= s.start && p.t <= s.end + 600)); // 検索中は、一致したセッションの間の push だけ（詳細のプロンプトの流れと同じ範囲）
    const marks = [...dayGit.map(c => ({t: c.t, c})),
      ...(META.push || []).filter(p => p.t >= ds && p.t < de && showP(p)).map(p => ({t: p.t, p})),
      ...shown.flatMap(s => (s.prAt || []).filter(x => x.t >= ds && x.t < de).map(x => ({t: x.t, r: x, s})))].sort((a, b) => a.t - b.t);
    let gy = -99;
    marks.forEach(({t, c, p, r, s}) => {
      const y = Math.max(clockH(day, t)*hh, gy + 17); gy = y;
      if (p){ html += `<button class="gm push${st.sel === "push:" + pushKey(p) ? " sel" : ""}" data-push="${esc(pushKey(p))}" style="top:${y}px" title="${esc(`${hm(t)} Pushed to ${p.ref}${p.commits ? ` (${plural(p.commits, "commit")})` : ""} · ${p.project}\nFrom this computer (git reflog)`)}" aria-label="${esc(`Push ${hm(t)} to ${p.ref}`)}">${ico("push")}</button>`; return; }
      if (r){ html += `<button class="gm prm${st.sel === "pr:" + prKey(s, r) ? " sel" : ""}" data-pr="${esc(prKey(s, r))}" style="top:${y}px" title="${esc(`${hm(t)} Created a pull request · ${s.project}${r.url ? "\n" + r.url : ""}\nRecorded when an agent created it`)}" aria-label="${esc(`Pull request ${hm(t)}`)}">${ico("pr")}</button>`; return; }
      html += `<button class="gc${c.ai ? " ai" : ""}${st.sel === "git:"+c.hash ? " sel" : ""}" data-c="${esc(c.hash)}" style="top:${y}px" title="${esc(`${hm(c.t)} ${c.project}${c.branch ? " · "+c.branch : ""} · ${c.hash}\n${c.subject}\n${plural(c.nFiles, "file")} +${c.added} −${c.removed}${c.ai ? " · run by AI" : ""}`)}" aria-label="${esc(`Commit ${hm(c.t)} ${c.subject}`)}">${GIT_ICON}<span>${esc(c.hash.slice(0,7))}</span></button>`; });
    limitHits(ds, de).filter(h => matches(h.s)).forEach(h => { html += `<button class="lim" data-id="${esc(h.s.id)}" style="top:${clockH(day, h.t)*hh}px" title="${esc(`${hm(h.t)} Hit the usage limit (${h.s.title})`)}" aria-label="${esc(`${hm(h.t)} usage limit`)}">${ico("limit")}Limit</button>`; });
    if (isToday && nowS >= ds && nowS < de) html += `<div class="nowline" style="top:${clockH(day, nowS)*hh}px"></div>`;
    cols += `<div class="day${day.getDay()%6===0?" we":""}${wkc(day.getDay())}${isToday?" today":""}" style="height:${H}px;--g:${marks.length ? 18 : 0}px">${html}</div>`;
  }
  T.innerHTML = `<div class="calscroll" style="--hh:${hh}px"><div class="calin"><div class="heads">${heads}</div><div class="cgrid"><div class="hours" style="height:${H}px">${hours}</div>${cols}</div></div></div>`;
  const sc = T.querySelector(".calscroll");
  sc.style.scrollPaddingTop = T.querySelector(".heads").offsetHeight + "px"; sc.style.scrollPaddingLeft = "56px"; // Tab で移ったブロックが、固定の日付・時刻の下に隠れないように
  if (top != null) sc.scrollTop = top;
  fitRuns(T);
  sc.addEventListener("scroll", () => edgeFade(T), {passive: true}); edgeFade(T);
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
function tipOn(e, bk){ const t = $("#tip"), s = bk.s;
  t.innerHTML = `<b>${esc(s.title)}</b><div class="r" style="--c:${colorOf(keyOf(s))}"><i></i>${esc(s.project)}${s.branch?` · ${esc(s.branch)}`:""}</div><div class="r">${md(bk.a)} ${hm(bk.a)}–${hm(bk.b)}${` (${dur((bk.b-bk.a)/60)})`}</div><div class="r">${esc((s.source))} · ${plural(s.nPrompts, "prompt")}${s.cost ? ` · ${usd(s.cost)}` : ""}${s.credits ? ` · ${crN(s.credits)} credits` : ""}${s.subagents.length ? ` · ${plural(s.subagents.length, "subagent")}` : ""}</div>`;

  t.classList.add("on"); tipMove(e); }
function tipMove(e){ const t = $("#tip"), w = t.offsetWidth, h = t.offsetHeight;
  let x = e.clientX + 14, y = e.clientY + 16; if (x + w > innerWidth - 12) x = e.clientX - w - 14; if (y + h > innerHeight - 12) y = e.clientY - h - 14;
  t.style.left = x+"px"; t.style.top = y+"px"; }
function tipOff(){ $("#tip").classList.remove("on"); }
function tipAt(el){ const r = el.getBoundingClientRect(); return {clientX: r.left + Math.min(r.width/2, 24), clientY: r.top + Math.min(r.height, 28)}; } // フォーカスしたブロックの左上あたりを、マウスの位置の代わりに
// グラフの棒・帯・点は data-tip に書いた文を、マウスを載せる（タッチでは触れる）とすぐ出す。1 行目は見出し、続く行は「名前 値」
const useLines = d => d ? [d.tokens ? `Tokens ${tok(d.tokens)}` : "", d.cost >= 0.005 ? `Estimated cost ${usd(d.cost)}` : "", d.credits ? `Credits ${cr(d.credits)}` : ""].filter(Boolean) : [];
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

