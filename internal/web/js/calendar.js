// 描画の入口 render、上の帯（要点）、週と月のカレンダー、ツールチップ
/* ── render ── */
const HOURS = [28, 36, 44, 56, 72, 96]; // 1 時間の高さ（ズーム）
function period(){ // 今見ている期間 [ws, we) と、その集計
  if (st.mode === "month"){ const a = st.month, b = new Date(a.getFullYear(), a.getMonth()+1, 1); return {ws:a.getTime()/1000, we:b.getTime()/1000, S:MONTHS[mkey(a)], P:MONTHS[mkey(new Date(a.getFullYear(), a.getMonth()-1, 1))]}; }
  return {ws:st.week.getTime()/1000, we:addDays(st.week,7).getTime()/1000, S:WEEKS[key(st.week)], P:WEEKS[key(addDays(st.week,-7))]};
}
// プロジェクトのないセッションのまとまりの名前。まとめて照らし合わせる名前は英語のまま、出すときだけ訳す
const projName = (name, items) => name === "Unknown project" && items.every(s => !s.project) ? t(name) : name;
function render(){
  assignColors();
  const {ws, we} = period(), M = st.mode === "month";
  const todayKey = key(today0()), end = addDays(st.week,6);
  const rel = relName(); // 今週・先週（今月・先月）なら、日付の上にそう書く（開いたときに、どの期間を見ているかわかるように）
  if (M){ $("#ry").textContent = `${st.month.getFullYear()} · ${rel || t("Month")}`; $("#rd").innerHTML = `${MONTH[st.month.getMonth()]} <span>${st.month.getFullYear()}</span>`; }
  else { $("#ry").textContent = `${st.week.getFullYear()} · ${t("Week {n}", {n: isoWeek(st.week)})}${rel ? ` · ${rel}` : ""}`; $("#rd").innerHTML = `${dMD(st.week)}<span> – </span>${dMD(end)}`; }
  $("#today").textContent = META.scope ? (META.scope.mode === "month" ? t("Back to included month") : t("Back to included week")) : M ? t("This month") : t("This week");
  [["#prev", -1], ["#next", 1]].forEach(([id, n]) => { const b = $(id), off = !canGo(n); // 先の週・ファイルの外へは押せない
    if (off && document.activeElement === b) $(n > 0 ? "#prev" : "#next").focus(); // 押せなくなるボタンにいたフォーカスは、隣のボタンへ
    b.disabled = off; });
  $("#yrbtn").hidden = !YEAR_ON || !DATA.length || !!(META && META.scope); // 週・月だけを書き出したファイルには 1 年の記録がない（最初の記録や「初めて」を取り違える）
  $("#prev").setAttribute("aria-label", M ? t("Previous month") : t("Previous week")); $("#next").setAttribute("aria-label", M ? t("Next month") : t("Next week"));
  document.querySelectorAll("#mode button[data-v]").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.mode));
  $("#calendarbtn").setAttribute("aria-pressed", String(st.view === "calendar"));
  $("#journeybtn").setAttribute("aria-pressed", String(st.view === "journey"));
  $("#projectbtn").setAttribute("aria-pressed", String(st.view === "project"));
  const viewLabel = {calendar:t("Calendar"), journey:t("Journey"), project:t("Project activity")}[st.view];
  $("#tl").setAttribute("aria-label", viewLabel);
  $("#skipcal").textContent = {calendar:t("Skip calendar"), journey:t("Skip journey"), project:t("Skip project activity")}[st.view];

  const inRange = DATA.filter(s => s.end >= ws && s.start < we);
  const shown = inRange.filter(matches);
  kpis();
  // legend（この期間にあるものを、作業時間の多い順に。数はサマリーの「① By project」の配分と同じ作業時間）
  const cnt = {}; inRange.forEach(s => cnt[keyOf(s)] = (cnt[keyOf(s)]||0)+1);
  const mins = legendMinutes();
  // 件数は横にスクロールする並び（.lrow）の外に置く。スマホでは並びの上に出し、チップが多くても画面の外へ押し出されないように
  $("#legend").innerHTML = `<label class="cbsel"><span class="cbt">${esc(t("Color by"))}</span><select id="cb2">${Object.entries(CB()).map(([v,l]) => `<option value="${v}"${st.colorBy === v ? " selected" : ""}>${esc(l)}</option>`).join("")}</select></label><div class="lrow"><span class="lab">${esc(t("Active time"))}<small> · ${esc(t("click to hide"))}</small></span>` +
    Object.keys(cnt).sort((a,b)=>(mins[b]||0)-(mins[a]||0) || cnt[b]-cnt[a]).map(k => `<button class="chip" style="--c:${colorOf(k)}" data-k="${esc(k)}" aria-pressed="${!st.hidden.has(k)}" title="${esc(t("{name}: {stats} (click to show or hide)", {name: k, stats: `${mins[k] ? dur(mins[k]) + " · " : ""}${plural(cnt[k], "session")}`}))}"><span class="dot"></span>${st.colorBy === "source" ? agMark(k) : ""}<span class="cn">${esc((k))}</span><span class="n">${mins[k] ? dur(mins[k]) : "—"}<span class="sr"> ${esc(t("active"))}</span></span></button>`).join("") +
    `</div>${M ? "" : `<span class="zlab" aria-hidden="true">${esc(t("Zoom"))}</span><div class="segc zoom" role="group" aria-label="${esc(t("Zoom"))}"><button id="zout" aria-label="${esc(t("Zoom out"))}">−</button><button id="zin" aria-label="${esc(t("Zoom in"))}">+</button></div>`}<span class="count">${esc(t("{n} / {total} sessions", {n: shown.length, total: inRange.length}))}</span>`;
  if (!M){ $("#zin").onclick = () => zoom(1); $("#zout").onclick = () => zoom(-1); }
  // 検索・凡例で絞り込んでいるあいだ、絞り込めない合計（作業時間・トークンなど）は薄くして、そう断る
  const fn = $("#fnote"); fn.hidden = !filtering();
  fn.textContent = `${st.q ? t("This view shows only sessions matching the search and their linked records.") : t("Hidden items are left out of this view.")} ${M ? t("Grey figures, at the top and in each day and week, are totals for all sessions; sessions and commits there count only what is shown.") : t("Grey figures, at the top and under each date, are totals for all sessions; sessions and commits there count only what is shown.")}`;
  $("#legend").hidden = !inRange.length; // 記録のない期間は、色分け・ズーム・件数を出さない（押しても何も変わらない）
  const cb2 = $("#cb2"); if (cb2) cb2.onchange = () => { st.colorBy = cb2.value; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); };
  document.querySelectorAll(".chip").forEach(c => c.onclick = () => { const k = c.dataset.k; st.hidden.has(k) ? st.hidden.delete(k) : st.hidden.add(k); render(); });

  if (!M && st.zAuto) st.z = fitZoom(); // ズームを選んでいなければ、この週の作業の時間帯が枠に収まる高さに
  if (st.view === "project") projectGrid(shown, ws, we); else if (st.journey) journeyGrid(shown, ws, we); else M ? monthGrid(shown, ws, we, todayKey) : timeline(shown, inRange, ws, we, todayKey);
  edgeFade($("#tl")); // 月表示や空の週では消す
  $("#skipcal").hidden = !!st.q; // 検索の最中は、結果がカレンダーの上にあり、下のサマリーは空
  lastFlags = []; // 見直す候補は表示中の期間のものだけ（検索中や記録のない期間に、前の期間の候補を詳細に出さない）
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
  const {S:w} = period(), K = $("#kpis"), M = st.mode === "month";
  K.classList.toggle("unf", filtering()); // 絞り込みの最中も、ここは全セッションの合計（薄くして、カレンダーの上に断り書き）
  if (!w){ K.innerHTML = `<div class="kpi"><div class="k">${esc(META.scope ? t("Not in this file") : shownName())}</div><div class="v">${esc(t("No records"))}</div></div>`; return; }
  // 押すと内訳のダイアログ（metric.js）。id のない数は押せない
  const kpi = (k, v, tt, cls, id) => { const tg = id ? "button" : "div";
    return `<${tg} class="kpi${cls ? " " + cls : ""}"${id ? ` type="button" data-metric="${id}" aria-haspopup="dialog"` : ""}${tt || id ? ` title="${esc(tt ? (id ? t("{text}. Click for the breakdown", {text: tt}) : tt) : t("Click for the breakdown"))}"` : ""}><div class="k">${esc(k)}</div><div class="v">${v}</div></${tg}>`; };
  const u = w.usage || {}, days = w.days.filter(d => d.active).length, pj = projection(w);
  const n0 = soFar(), nd = n0 == null ? w.days.length : Math.min(n0, w.days.length); // 途中の週・月は、まだ来ていない日を分母に入れない（今日までの日数）
  const pace = pj ? t("Estimate: if the pace of the first {n} days continues", {n: pj.days}) : "";
  // トークン・目安コスト・月末の見込み・クレジットは、いちばん大事な数なので先頭にまとめて目立たせる
  K.innerHTML = (u.tokens ? kpi(t("Tokens"), tok(u.tokens), "", "key", "tokens") + kpi(t("Estimated cost"), usdH(costOf(u)), costOf(u) == null ? NOPRICE : "", "key", "cost") : "") +
    (pj && pj.cost != null ? kpi(t("Month-end cost"), `<small>≈</small>${usdH(pj.cost)}`, pace, "key", "projection") : "") +
    (u.credits ? kpi(t("Kiro credits"), `${crN(u.credits)}<small>cr</small>`, "", "key", "credits") : "") +
    (pj && pj.credits != null ? kpi(t("Month-end credits"), `<small>≈</small>${crN(pj.credits)}<small>cr</small>`, pace, "key", "projectionCr") : "") +
    kpi(t("Active time"), dur(w.active, true), "", "", "active") + kpi(t("Active days"), `${days}<small> ${esc(n0 == null ? t("of {n}", {n: nd}) : t("of {n} so far", {n: nd}))}</small>`, n0 == null ? "" : M ? t("{days} of the {n} so far (this month is still in progress)", {days, n: plural(nd, "day")}) : t("{days} of the {n} so far (this week is still in progress)", {days, n: plural(nd, "day")}), "", "days") +
    kpi(t("Sessions / prompts"), `${w.sessions}<small>/</small>${w.prompts}`, "", "", "sessions") +
    (() => { const {ws, we} = period(), n = limitHits(ws, we).length; return n ? kpi(t("Usage limit hits"), `<span class="wv">${n}</span>`, "", "", "limits") : ""; })() +
    (w.git && w.git.commits ? kpi(t("Git commits"), `${w.git.commits}<small> · ${esc(t("{n} by AI", {n: w.git.ai}))}</small>`, "", "", "commits") :
     w.outputs && w.outputs.commits ? kpi(t("AI commits"), `${w.outputs.commits}`) : "");
  K.querySelectorAll("[data-metric]").forEach(b => b.onclick = () => openMetric(b.dataset.metric, b));
}

/* カレンダーの各日（月表示では各週も）に並べる：作業時間・トークン・クレジット・セッション・Git のコミット。
   トークンとクレジットは、その期間にあるほう（両方あれば両方）を、どの日にも同じ見出しで出す（日によって見出しが変わると、横に並べて比べられない）。
   un は絞り込み（検索・凡例で隠す）の最中に、絞り込まれていない合計を薄くする（1: 作業時間・トークン・クレジット、2: 全部） */
const useKinds = xs => ({tokens: xs.some(d => d && d.tokens), credits: xs.some(d => d && d.credits)});
function calRows(act, u, nS, nC, kinds, un){
  const k = un ? "k u" : "k", o = un > 1 ? "u" : "";
  return [[t("Active"), act ? dur(act) : "—", k], kinds.tokens && [t("Tokens"), u && u.tokens ? tok(u.tokens) : "—", k], kinds.credits && [t("Credits"), u && u.credits ? cr(u.credits) : "—", k],
    [t("Sessions"), nS, o], [t("Commits"), nC, o]].filter(Boolean);
}
const calDl = rows => `<dl class="cm">${rows.map(([k, v, c]) => `<div${c ? ` class="${c}"` : ""}><dt>${esc(k)}</dt><dd>${v}</dd></div>`).join("")}</dl>`; // k は強く出す値
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
    h += `<button class="cell wkc" data-w="${key(wk)}"${tipAttr(t("Week {n}", {n: isoWeek(wk)}), W ? [`${t("Active")} ${dur(wa)}`, `${t("Sessions")} ${W.sessions}`, ...useLines(W.usage), `${t("Git commits")} ${wc}`] : t("No records"))}><span class="dn">W${isoWeek(wk)}</span>${W && wa ? calDl(calRows(wa, W.usage, W.sessions, wc, kinds, un && 2)) + calShort(wa, W.usage, un) : ""}</button>`;
    for (let c = 0; c < 7; c++){
      const d = addDays(wk, c), ds = d.getTime()/1000, de = addDays(d,1).getTime()/1000, inM = d.getMonth() === st.month.getMonth();
      if (!inM){ h += `<div class="cell out"><span class="dn">${d.getDate()}</span></div>`; continue; }
      const x = S ? S.days[d.getDate()-1] : null, act = x ? x.active : 0;
      const by = {}; shown.forEach(s => s.segs.forEach(([a,b]) => { const o = Math.min(b,de) - Math.max(a,ds); if (o > 0) by[keyOf(s)] = (by[keyOf(s)]||0) + o; }));
      const pj = Object.entries(by).sort((a,b)=>b[1]-a[1]), nS = shown.filter(s => inP(s, ds, de)).length;
      const nC = G ? G.filter(c => c.t >= ds && c.t < de).length : x && x.commits || 0;
      h += `<button class="cell${d.getDay()%6===0?" we":""}${wkc(d.getDay())}${key(d)===todayKey?" today":""}" data-w="${key(mondayOf(d))}" style="--heat:${(act/max).toFixed(3)}"${tipAttr(md(ds), act ? [`${t("Active")} ${dur(act)}`, `${t("Sessions")} ${nS}`, ...useLines(x), `${t("Git commits")} ${nC}`] : t("No records"))}>
        <span class="dn">${d.getDate()}</span>${act ? calDl(calRows(act, x, nS, nC, kinds, un)) + calShort(act, x, un) : ""}
        ${pj.length ? `<span class="pj">${pj.map(([k,v])=>`<span style="flex:${v};--c:${colorOf(k)}"></span>`).join("")}</span>` : ""}</button>`;
    }
  }
  h += `</div><div class="mlegend"><span class="nw">${esc(t("Less"))}</span> ${[0,.25,.5,.75,1].map(v=>`<i style="--h:${v}"></i>`).join("")} ${esc(matchMedia("(max-width:820px)").matches ? t("More (active time) · Each day and week shows active time, and tokens or credits (both when the month has both). A week counts all 7 days, including those in the next or previous month · Tap a date or week to open it") : t("More (active time) · Each day, and each week on the left, shows active time, tokens or credits (both when the month has both), sessions and Git commits. A week counts all 7 days, including those in the next or previous month · Click a date or week to open it"))}</div>`;
  T.innerHTML = h;
  T.querySelectorAll("[data-w]").forEach(b => b.onclick = () => { const [y,m,dd] = b.dataset.w.split("-").map(Number); st.week = new Date(y, m-1, dd); setMode("week"); });
}

/* 週のカレンダー：曜日が列、時刻が縦。重なるセッションだけ横に並べる */
function timeline(shown, inWeek, ws, we, todayKey){
  const T = $("#tl"), hh = HOURS[st.z] || 44, H = 24*hh;
  if (!inWeek.length || !shown.length){
    // 記録のない週は、ここで 1 度だけ「ない」と言い、記録のある最後の週へ行けるようにする（月は大文字のまま。toLowerCase で "jul 6" にしない）
    const last = !inWeek.length && !META.scope && DATA.length ? mondayOf(new Date(Math.max(...DATA.map(s => s.end))*1000)) : null, jump = last && last.getTime() !== st.week.getTime();
    T.innerHTML = `<div class="empty"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="15" rx="3"/><path d="M3 10h18M8 3v4M16 3v4"/></svg><p>${esc(inWeek.length ? t("No sessions match the current filters") : META.scope ? t("This file has no records for this week.") : st.week.getTime() === mondayOf(today0()).getTime() ? t("No records this week.") : t("No records in the week of {date}.", {date: dMD(st.week)}))}</p>${jump ? `<p><button class="pill" id="tolast">${esc(t("Go to the latest week with records ({date})", {date: dMD(last)}))}</button></p>` : ""}</div>`;
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
    heads += `<div class="head${isToday?" today":""}${wkc(day.getDay())}"><div class="dd"><b>${dMD(day)}</b><i>${dow(day.getDay())}</i>${act || dayGit.length ? copyBtn(`<span class="dl">${esc(t("Report"))}</span>`, `data-d="${d}" aria-label="${esc(t("Copy daily report prompt for {date}", {date: md(ds)}))}" title="${esc(t("A prompt that asks an AI agent on this computer to write your daily report for {date}", {date: md(ds)}))}"`, "pill fill dayrpt") : ""}${act || dayGit.length ? (aiRun() ? `<button class="pill dayrun ai-action" data-d="${d}" type="button" aria-label="${esc(t("Run with AI: daily report for {date}", {date: md(ds)}))}">${esc(t("Run with AI"))}</button>` : `<button class="pill ai-off dayrun-demo ai-demo-info" type="button" aria-label="${esc(t("Run with AI is off: {why}", {why: AI_OFF_WHY()}))}" title="${esc(AI_OFF_WHY())}">${esc(t("AI off"))}</button>`) : ""}</div>${act || dayGit.length ? `<div${tipAttr(md(ds), [`${t("Active")} ${dur(act)}`, `${t("Sessions")} ${nS}`, ...useLines(dW), `${t("Git commits")} ${dayGit.length}`])}>${calDl(calRows(act, dW, nS, dayGit.length, kinds, un))}</div>` : `<small>—</small>`}</div>`;
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
    marks.forEach(({t: at, c, p, r, s}) => {
      const y = Math.max(clockH(day, at)*hh, gy + 17); gy = y;
      if (p){ html += `<button class="gm push${st.sel === "push:" + pushKey(p) ? " sel" : ""}" data-push="${esc(pushKey(p))}" style="top:${y}px" title="${esc((p.commits ? t("{time} Pushed to {ref} ({commits}) · {project}", {time: hm(at), ref: p.ref, commits: plural(p.commits, "commit"), project: p.project}) : t("{time} Pushed to {ref} · {project}", {time: hm(at), ref: p.ref, project: p.project})) + "\n" + t("From this computer (git reflog)"))}" aria-label="${esc(t("Push {time} to {ref}", {time: hm(at), ref: p.ref}))}">${ico("push")}</button>`; return; }
      if (r){ html += `<button class="gm prm${st.sel === "pr:" + prKey(s, r) ? " sel" : ""}" data-pr="${esc(prKey(s, r))}" style="top:${y}px" title="${esc(t("{time} Created a pull request · {project}", {time: hm(at), project: s.project}) + (r.url ? "\n" + r.url : "") + "\n" + t("Recorded when an agent created it"))}" aria-label="${esc(t("Pull request {time}", {time: hm(at)}))}">${ico("pr")}</button>`; return; }
      html += `<button class="gc${c.ai ? " ai" : ""}${st.sel === "git:"+c.hash ? " sel" : ""}" data-c="${esc(c.hash)}" style="top:${y}px" title="${esc(`${hm(c.t)} ${c.project}${c.branch ? " · "+c.branch : ""} · ${c.hash}\n${c.subject}\n${plural(c.nFiles, "file")} +${c.added} −${c.removed}${c.ai ? " · " + t("run by AI") : ""}`)}" aria-label="${esc(t("Commit {time} {subject}", {time: hm(c.t), subject: c.subject}))}">${GIT_ICON}<span>${esc(c.hash.slice(0,7))}</span></button>`; });
    limitHits(ds, de).filter(h => matches(h.s)).forEach(h => { html += `<button class="lim" data-id="${esc(h.s.id)}" style="top:${clockH(day, h.t)*hh}px" title="${esc(t("{time} Hit the usage limit ({title})", {time: hm(h.t), title: h.s.title}))}" aria-label="${esc(t("{time} usage limit", {time: hm(h.t)}))}">${ico("limit")}${esc(t("Limit"))}</button>`; });
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
  T.querySelectorAll(".dayrpt").forEach(b => b.onclick = e => { e.stopPropagation(); // 日報のプロンプト（週報と同じ作り。その日の分だけ）
    copy(reportPrompt(null, false, addDays(st.week, +b.dataset.d)), t("Copied. Paste it into an AI agent on this computer. It includes your prompts and history file paths, and the agent reads those files, so check it before sending"), 8000, b); });
  T.querySelectorAll(".ai-demo-info").forEach(b => b.onclick = e => { e.stopPropagation(); showLocalAIInfo(); });
  T.querySelectorAll(".dayrun").forEach(b => b.onclick = e => { e.stopPropagation(); runAIReview(b, reportPrompt(null, false, addDays(st.week, +b.dataset.d)), t("Daily report")); });
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
  tp.innerHTML = `<b>${esc(s.title)}</b><div class="r" style="--c:${colorOf(keyOf(s))}"><i></i>${esc(s.project)}${s.branch?` · ${esc(s.branch)}`:""}</div><div class="r">${md(bk.a)} ${hm(bk.a)}–${hm(bk.b)}${` (${dur((bk.b-bk.a)/60)})`}</div><div class="r">${esc((s.source))} · ${plural(s.nPrompts, "prompt")}${s.cost ? ` · ${usd(s.cost)}` : ""}${s.credits ? ` · ${esc(t("{n} credits", {n: crN(s.credits)}))}` : ""}${s.subagents.length ? ` · ${plural(s.subagents.length, "subagent")}` : ""}</div>`;

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


// Journey: project lanes across the selected week or month. No inferred relationships.
function journeyGrid(sessions, ws, we){
  const root = $("#tl"), start = new Date(ws*1000), days = Math.round((Date.UTC(new Date(we*1000).getFullYear(),new Date(we*1000).getMonth(),new Date(we*1000).getDate())-Date.UTC(start.getFullYear(),start.getMonth(),start.getDate()))/864e5);
  const groups = new Map();
  sessions.forEach(s => { const name = s.project || "Unknown project";
    if (!groups.has(name)) groups.set(name, []);
    groups.get(name).push(s);
  });
  if (!groups.size){ root.innerHTML = '<div class="journey-empty">'+esc(t("No matching project activity in this period."))+'</div>'; return; }
  const ordered = [...groups].sort((a,b) => b[1].length-a[1].length || a[0].localeCompare(b[0]));
  const shownCommits = gitShown(sessions);
  const dayBounds = Array.from({length:days},(_,i)=>addDays(start,i).getTime()/1000);
  dayBounds.push(addDays(start,days).getTime()/1000);
  root.innerHTML = '<div class="journey"><div class="journey-legend" aria-label="'+esc(t("Journey marker legend"))+'"><span><i class="journey-legend-session"></i> '+esc(t("Session"))+'</span><span><i class="journey-legend-commit">◆</i> '+esc(t("Git commit"))+'</span><span><i class="journey-legend-pr">PR</i> '+esc(t("Pull request"))+'</span><small>'+esc(t("Recorded events only; not proof of progress or completion."))+'</small></div><div class="journey-axis"><span>'+esc(t("Project"))+'</span><div class="journey-days">' +
    Array.from({length:days},(_,i)=>'<span>'+esc(dMD(addDays(new Date(ws*1000),i)))+'</span>').join("") +
    '</div></div>' + ordered.map(([name,items]) => {
      const byDay = Array.from({length:days},()=>[]);
      items.forEach(s => { for(let i=0;i<days;i++){ const a=dayBounds[i], b=dayBounds[i+1];
        if(s.start<b && s.end>a) byDay[i].push(s);
      }});
      // Observed Git commits are milestones, not inferred completion.
      const commitsByDay = Array.from({length:days},()=>[]);
      shownCommits.filter(c => c.project === name && c.t >= ws && c.t < we).forEach(c => {
        for(let i=0;i<days;i++){
          const a=addDays(start,i).getTime()/1000, b=addDays(start,i+1).getTime()/1000;
          if(c.t>=a && c.t<b){ commitsByDay[i].push(c); break; }
        }
      });
      const prsByDay = Array.from({length:days},()=>[]);
      items.forEach(session => (session.prAt || []).forEach(pr => {
        if(typeof pr.t !== "number" || pr.t < ws || pr.t >= we) return;
        for(let i=0;i<days;i++){
          const a=addDays(start,i).getTime()/1000, b=addDays(start,i+1).getTime()/1000;
          if(pr.t>=a && pr.t<b){ prsByDay[i].push({session,pr}); break; }
        }
      }));
      const uniqueDays = byDay.filter(day => day.length).length;
      const orderedItems = [...items].sort((a,b)=>a.start-b.start);
      const first = orderedItems[0];
      const activity = plural(uniqueDays, "active day") + " · " + plural(items.length, "session");
      const lastEnd = items.reduce((latest,item)=>Math.max(latest,item.end),ws);
      const span = first ? dMD(new Date(Math.max(ws, first.start)*1000)) + " → " + dMD(new Date(Math.min(we-1,lastEnd)*1000)) : "";
      return '<div class="journey-row"><div class="journey-name" title="'+esc(projName(name, items))+'">'+esc(projName(name, items))+'<small title="'+esc(t("Observed sessions only; not a measure of progress"))+'">'+esc(activity)+'</small><small>'+esc(span)+'</small></div><div class="journey-days">' +
        byDay.map((day,i) => '<div class="journey-cell">'+day.slice(0,4).map(s =>
          '<button class="journey-dot" style="--c:'+colorOf(keyOf(s))+'" data-session="'+esc(s.id)+'" title="'+esc((s.title||s.project||t("Session"))+' · '+s.source)+'" aria-label="'+esc((s.title||t("Session"))+' · '+s.source)+'"></button>'
        ).join("")+(day.length>4?'<span class="journey-more">+'+(day.length-4)+'</span>':"")+commitsByDay[i].slice(0,3).map(c => '<button class="journey-dot journey-commit" data-git="'+esc(c.hash)+'" title="'+esc(t("Commit · {subject}", {subject: c.subject}))+'" aria-label="'+esc(t("Commit · {subject}", {subject: c.subject}))+'" style="--c:var(--ink-3)">◆</button>').join("")+(commitsByDay[i].length>3?'<span class="journey-more">'+esc(t("+{n} commits", {n: commitsByDay[i].length-3}))+'</span>':"")+prsByDay[i].slice(0,2).map(({session,pr}) => '<button class="journey-dot journey-pr" data-pr="'+esc(prKey(session,pr))+'" title="'+esc(t("Recorded pull request")+(pr.url?" · "+pr.url:""))+'" aria-label="'+esc(t("Recorded pull request"))+'" style="--c:var(--ink-3)">PR</button>').join("")+(prsByDay[i].length>2?'<span class="journey-more">'+esc(t("+{n} PRs", {n: prsByDay[i].length-2}))+'</span>':"")+'</div>').join("")+'</div></div>';
    }).join("")+'</div>';
  root.querySelectorAll("[data-session]").forEach(b => b.onclick=()=>select(b.dataset.session));
  root.querySelectorAll("[data-git]").forEach(b => b.onclick=()=>select("git:"+b.dataset.git));
  root.querySelectorAll("[data-pr]").forEach(b => b.onclick=()=>select("pr:"+b.dataset.pr));
}

/* Project is an inventory of observed work in the selected period.
   Unlike Journey it does not infer a sequence or a project status. */
function projectGrid(sessions, ws, we){
  const root = $("#tl"), groups = new Map(), commitsByProject = new Map();
  // Index commits once rather than scanning all commits for every project card.
  gitShown(sessions).forEach(c => {
    if (c.t < ws || c.t >= we) return;
    if (!commitsByProject.has(c.project)) commitsByProject.set(c.project, []);
    commitsByProject.get(c.project).push(c);
  });
  sessions.forEach(s => {
    const name = s.project || "Unknown project";
    if (!groups.has(name)) groups.set(name, []);
    groups.get(name).push(s);
  });
  if (!groups.size){ root.innerHTML = '<div class="journey-empty">'+esc(t("No matching projects in this period."))+'</div>'; return; }
  const rows = [...groups].sort((a,b)=>b[1].length-a[1].length || a[0].localeCompare(b[0]));
  root.innerHTML = '<div class="project-view"><p class="project-note">'+esc(t("Observed activity in the selected period. Commits and recorded PR creation are not measures of completion or business impact."))+'</p>' +
    rows.map(([name,items])=>{
      const commits = commitsByProject.get(name) || [];
      const prs = items.flatMap(s=>(s.prAt || []).filter(pr=>typeof pr.t === "number" && pr.t >= ws && pr.t < we).map(pr=>({s,pr})));
      const first = Math.max(ws,items.reduce((v,s)=>Math.min(v,s.start),Infinity));
      const last = Math.min(we-1,items.reduce((v,s)=>Math.max(v,s.end),ws));
      const eventList = (label, more, entries, limit, makeButton) => {
        if (!entries.length) return "";
        const ordered = entries;
        const visible = ordered.slice(0,limit).map(makeButton).join("");
        const rest = ordered.length > limit ? '<details class="project-more"><summary>'+esc(more(ordered.length-limit))+'</summary>'+ordered.slice(limit).map(makeButton).join("")+'</details>' : "";
        return '<strong>'+esc(label)+'</strong>'+visible+rest;
      };
      const byAgent = new Map();
      items.forEach(s=>byAgent.set(s.source || t("Unknown agent"),(byAgent.get(s.source || t("Unknown agent")) || 0)+1));
      const agents = [...byAgent].sort((a,b)=>b[1]-a[1] || a[0].localeCompare(b[0]));
      const agentSummary = '<div class="project-agents" aria-label="'+esc(t("Sessions by agent"))+'">'+agents.map(([source,count])=>
        '<span class="project-agent">'+esc(source)+' <strong>'+count+'</strong></span>').join("")+'</div>';
      const sessionsList = items.slice().sort((a,b)=>b.start-a.start);
      const commitsList = commits.slice().sort((a,b)=>b.t-a.t);
      const prsList = prs.slice().sort((a,b)=>b.pr.t-a.pr.t);
      return '<section class="project-card"><h3>'+esc(projName(name, items))+'</h3><p class="project-meta">'+plural(items.length,"session")+' · '+plural(commits.length,"Git commit")+' · '+plural(prs.length,"recorded PR")+' · '+esc(dMD(new Date(first*1000)))+' → '+esc(dMD(new Date(last*1000)))+'</p>'+agentSummary+
        '<div class="project-events">'+
        eventList(t("Sessions"),n=>t("Show {n} more sessions",{n}),sessionsList,5,s=>'<button data-session="'+esc(s.id)+'">'+esc(s.title||s.project||t("Session"))+' · '+esc(s.source)+'</button>')+
        eventList(t("Git commits"),n=>t("Show {n} more git commits",{n}),commitsList,3,c=>'<button data-git="'+esc(c.hash)+'">'+esc(c.subject||c.hash)+'</button>')+
        eventList(t("Recorded PRs"),n=>t("Show {n} more recorded prs",{n}),prsList,3,({s,pr})=>'<button data-pr="'+esc(prKey(s,pr))+'">'+esc(pr.url||t("Pull request"))+'</button>')+
        '</div></section>';
    }).join("")+'</div>';
  root.querySelectorAll("[data-session]").forEach(b=>b.onclick=()=>select(b.dataset.session));
  root.querySelectorAll("[data-git]").forEach(b=>b.onclick=()=>select("git:"+b.dataset.git));
  root.querySelectorAll("[data-pr]").forEach(b=>b.onclick=()=>select("pr:"+b.dataset.pr));
}
