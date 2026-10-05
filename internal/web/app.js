let DATA = __DATA__;
let WEEKS = __WEEKS__;
let MONTHS = __MONTHS__;
let META = __META__;
let GENERATED = __GEN__;
/* デモ（GitHub Pages）では、ダミーデータを作った時間帯の時計で見せる。どこから開いても「朝から夜に作業した」ように見え、
   日ごとの集計（作った時間帯で区切っている）とも食い違わない。時刻らしい数（UNIX 秒）と「今」を同じだけずらす */
const SHIFT = META && META.demo ? META.demo.offset + new Date().getTimezoneOffset() * 60 : 0;
const nowMs = () => Date.now() + SHIFT * 1000, today0 = () => new Date(nowMs());
function shiftTimes(x){ if (!SHIFT) return x;
  const walk = v => Array.isArray(v) ? v.map(walk) : v && typeof v === "object" ? Object.fromEntries(Object.entries(v).map(([k, y]) => [k, walk(y)])) : typeof v === "number" && v > 1e9 && v < 4e9 ? v + SHIFT : v;
  return walk(x); }
[DATA, WEEKS, MONTHS, META, GENERATED] = [shiftTimes(DATA), shiftTimes(WEEKS), shiftTimes(MONTHS), shiftTimes(META), shiftTimes(GENERATED)];
const LIVE = __LIVE__; // kiroku serve で開いたとき true
const PROMPT_RUNES = __PROMPT_RUNES__; // HTML に入れるプロンプトの長さ（core.PromptRunes）
const YEAR_ON = false; // 1 年の露光は一旦隠す（ボタンと Y キーを出さない）。戻すときは true にする
const SLOTS = 8;
const $ = s => document.querySelector(s);
const store = { get(k,d){ try { const v = localStorage.getItem("kiroku:"+k); return v == null ? d : JSON.parse(v); } catch(e){ return d; } },
                set(k,v){ try { localStorage.setItem("kiroku:"+k, JSON.stringify(v)); } catch(e){} } };
const LOC = () => "en-US";
const DOW = ["Sun","Mon","Tue","Wed","Thu","Fri","Sat"];
const dow = i => DOW[i];
const wkc = i => i === 6 ? " sat" : i === 0 ? " sun" : ""; // 曜日（getDay）から、土日の色のクラス
const plural = (n, one, many) => `${n} ${n === 1 ? one : many || one + "s"}`;
const st = { z: store.get("zh", 2), colorBy: store.get("colorBy", "project"), theme: store.get("theme", "dark") === "light" ? "light" : "dark", // 既定はダーク（以前の「自動」もダークにする）
             week: mondayOf(today0()), month: monthOf(today0()), mode: store.get("mode", "week"), use: store.get("use", "tokens"), flowUser: !!store.get("flowUser", false),
             hidden: new Set(), sel: null, back: [], q: "", animate: true };

/* ── helpers ── */
function mondayOf(d){ d = new Date(d); d.setHours(0,0,0,0); d.setDate(d.getDate()-((d.getDay()+6)%7)); return d; }
function monthOf(d){ d = new Date(d); return new Date(d.getFullYear(), d.getMonth(), 1); }
function mkey(d){ return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}`; }
function addDays(d,n){ d = new Date(d); d.setDate(d.getDate()+n); return d; }
function key(d){ return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}-${String(d.getDate()).padStart(2,"0")}`; }
function esc(s){ return String(s ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c])); }
function hm(t){ return new Date(t*1000).toLocaleTimeString("en-US",{hour:"2-digit",minute:"2-digit",hourCycle:"h23"}); }
function md(t){ const d = new Date(t*1000); return `${DOW[d.getDay()]} ${d.getMonth()+1}/${d.getDate()}`; }
function dur(m, html){ m = Math.round(m); const h = Math.floor(m/60), r = m%60;
  const u = x => html ? `<small>${x}</small>` : x; return h ? `${h}${u("h")}${r ? ` ${r}${u("m")}` : ""}` : `${r}${u("m")}`; }
function tok(n){ n = n || 0; return n >= 1e9 ? (n/1e9).toFixed(1)+"B" : n >= 1e6 ? (n/1e6).toFixed(1)+"M" : n >= 1e3 ? Math.round(n/1e3)+"K" : String(n); }
function usd(v){ return v == null ? "—" : v > 0 && v < 0.01 ? "<$0.01" : "$" + (v >= 100 ? Math.round(v).toLocaleString() : v.toFixed(2)); }
const crN = v => v >= 1 || v <= 0 ? Math.round(v).toLocaleString(LOC()) : v.toFixed(2); // クレジットは整数で（1 未満だけ小数 2 桁）
function cr(v){ return `${crN(v)} cr`; }
const tokS = v => v >= 1e7 ? Math.round(v/1e6)+"M" : v >= 1e6 ? (v/1e6).toFixed(1)+"M" : v >= 1e4 ? Math.round(v/1e3)+"K" : tok(v); // 狭いマス用に、桁を減らしたトークン
function shade(i){ return [1,.72,.5,.34,.22,.14][Math.min(i,5)]; }
function secs(v){ return v == null ? "—" : (v < 60 ? `${v}s` : `${Math.floor(v/60)}m ${v%60}s`); }
function secsH(v){ return secs(v).replace(/(?<=\d)([ms])\b/g, "<small>$1</small>"); } // 単位を小さく
function isoWeek(d){ d = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())); const n = d.getUTCDay() || 7; d.setUTCDate(d.getUTCDate()+4-n);
  const y0 = new Date(Date.UTC(d.getUTCFullYear(),0,1)); return Math.ceil(((d-y0)/864e5+1)/7); }
const keyOf = s => st.colorBy === "project" ? s.project : st.colorBy === "source" ? s.source : `${s.project} · ${s.branch || "—"}`;
let slot = {};
function assignColors(){ // 全期間の多い順に固定。週を変えても、非表示にしても色は変わらない
  const n = {}; DATA.forEach(s => n[keyOf(s)] = (n[keyOf(s)]||0) + 1);
  slot = {}; Object.keys(n).sort((a,b)=>n[b]-n[a]).forEach((k,i) => slot[k] = i < SLOTS ? `var(--c${i})` : "var(--other)");
}
const colorOf = k => slot[k] || "var(--other)";
function matches(s){
  if (st.hidden.has(keyOf(s))) return false;
  return !st.q || searchText(s).includes(st.q);
}
/* 検索の対象：タイトル・プロンプト・プロジェクト・ブランチ・ツール・変更したファイル・PR・そのセッションの間のコミット（件名・ハッシュ・ファイル） */
const SQ = new WeakMap(); // DATA が入れ替わる（自動更新）と作り直される
function searchText(s){
  let t = SQ.get(s);
  if (t == null){ t = [s.title, s.project, s.branch || "", s.source, ...s.prompts.map(p => p.text), ...s.files, ...(s.prs || []),
      ...commitsOf(s).flatMap(c => [c.hash, c.subject, ...(c.files || []).map(f => f.path)])].join("\n").toLowerCase(); SQ.set(s, t); }
  return t;
}
function toast(msg, ms){ const t = $("#toast"); t.textContent = msg; t.classList.add("on"); clearTimeout(toast.h); toast.h = setTimeout(()=>t.classList.remove("on"), ms || 1600); }
async function copy(text, msg, ms){ try { await navigator.clipboard.writeText(text); toast(msg || "Copied", ms); } catch(e){ toast("Couldn't copy"); } }
const ICON = { light:'<svg class="i" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M4.6 4.6L6 6M18 18l1.4 1.4M2.5 12h2M19.5 12h2M4.6 19.4L6 18M18 6l1.4-1.4"/></svg>',
  dark:'<svg class="i" viewBox="0 0 24 24"><path d="M20 14.5A8 8 0 019.5 4a8 8 0 1010.5 10.5z"/></svg>' };
function applyTheme(){ document.documentElement.setAttribute("data-theme", st.theme); // ボタンには、押すと切り替わる先のテーマを出す
  const next = st.theme === "dark" ? "light" : "dark";
  $("#theme").innerHTML = ICON[next]; $("#theme").setAttribute("aria-label", `Switch to ${next} theme`); }
const CB = () => ({project: "Project", branch: "Branch", source: "Agent"});
if (YEAR_ON) $("#keys .keys kbd:nth-of-type(8)").insertAdjacentHTML("beforebegin", `<kbd>Y</kbd><span>Year in review</span>`); // Esc の前に足す


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
  if (M){ $("#ry").textContent = `${st.month.getFullYear()} · Month`; $("#rd").innerHTML = `${st.month.getFullYear()}<span>.</span>${st.month.getMonth()+1}`; }
  else { $("#ry").textContent = `${st.week.getFullYear()} · Week ${isoWeek(st.week)}`; $("#rd").innerHTML = `${st.week.getMonth()+1}.${st.week.getDate()}<span>—</span>${end.getMonth()+1}.${end.getDate()}`; }
  $("#today").textContent = M ? "This month" : "This week";
  $("#yrbtn").textContent = "Year in review"; $("#yrbtn").hidden = !YEAR_ON || !DATA.length;
  $("#prev").setAttribute("aria-label", M ? "Previous month" : "Previous week"); $("#next").setAttribute("aria-label", M ? "Next month" : "Next week");
  document.querySelectorAll("#mode button").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.mode));
  document.querySelectorAll("#colorBy button").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.colorBy));
  document.querySelector(".zoom").style.visibility = M ? "hidden" : "";

  const inRange = DATA.filter(s => s.end >= ws && s.start < we);
  const shown = inRange.filter(matches);
  kpis();
  // legend（この期間にあるものを多い順に）
  const cnt = {}; inRange.forEach(s => cnt[keyOf(s)] = (cnt[keyOf(s)]||0)+1);
  $("#legend").innerHTML = `<label class="cbsel"><span class="sr">Color by</span><select id="cb2">${Object.entries(CB()).map(([v,l]) => `<option value="${v}"${st.colorBy === v ? " selected" : ""}>${l}</option>`).join("")}</select></label><span class="lab"><span class="ln">${CB()[st.colorBy]}</span><small> (sessions)</small></span>` +
    Object.keys(cnt).sort((a,b)=>cnt[b]-cnt[a]).map(k => `<button class="chip" style="--c:${colorOf(k)}" data-k="${esc(k)}" aria-pressed="${!st.hidden.has(k)}" title="${esc(`${(k)}: ${plural(cnt[k], "session")} (click to show or hide)`)}"><span class="dot"></span>${esc((k))}<span class="n">${cnt[k]}<span class="sr"> sessions</span></span></button>`).join("") +
    `<span class="count">${shown.length} / ${inRange.length} sessions${st.q ? ` · <button class="flink" id="tosr">All-time search results ↓</button>` : ""}</span>`;
  const cb2 = $("#cb2"); if (cb2) cb2.onchange = () => { st.colorBy = cb2.value; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); };
  const tosr = $("#tosr"); if (tosr) tosr.onclick = () => $("#review").scrollIntoView({behavior:"smooth"});
  document.querySelectorAll(".chip").forEach(c => c.onclick = () => { const k = c.dataset.k; st.hidden.has(k) ? st.hidden.delete(k) : st.hidden.add(k); render(); });

  M ? monthGrid(shown, ws, we, todayKey) : timeline(shown, inRange, ws, we, todayKey);
  try { st.q ? searchPanel() : summary(); } catch(e){ // サマリーで失敗しても、カレンダーと詳細は使えるようにする
    console.error(e); $("#review").innerHTML = `<div class="panel"><p class="none">${`Couldn't show the summary for this period (${esc(e.message)}). Please let us know in an issue.`}</p></div>`; }
  const gc = st.sel && st.sel.startsWith("git:") && (META.git || []).find(x => "git:" + x.hash === st.sel);
  const s = !gc && st.sel && DATA.find(x => x.id === st.sel), open = !!(s || gc);
  if (st.sel && !open) st.sel = null;
  if (s) detail(s); else if (gc) commitDetail(gc);
  document.body.classList.toggle("open", open); document.body.classList.toggle("lock", open);
  $("#drawer").setAttribute("aria-hidden", String(!open));
  document.querySelector("header").inert = document.querySelector("main").inert = open; // 背後に Tab で入らない
  $("#back").hidden = !st.back.length;
  if (open && (!focusDrawer.was || focusDrawer.sel !== st.sel)) $(st.back.length ? "#back" : "#close").focus();
  if (!open && focusDrawer.was) focusOpener(focusDrawer.first, focusDrawer.from);
  if (open && !focusDrawer.was){ focusDrawer.first = st.sel; focusDrawer.from = focusDrawer.next; }
  focusDrawer.next = null;
  focusDrawer.was = open; focusDrawer.sel = st.sel;
  $("#tl").classList.toggle("focus", open);
  st.animate = false;
}

/* 期間の要点（上の帯） */
function kpis(){
  const {S:w} = period(), K = $("#kpis"), M = st.mode === "month";
  if (!w){ K.innerHTML = `<div class="kpi"><div class="k">${M ? "This month" : "This week"}</div><div class="v">No records</div></div>`; return; }
  const kpi = (k, v) => `<div class="kpi"><div class="k">${k}</div><div class="v">${v}</div></div>`;
  const u = w.usage || {}, days = w.days.filter(d => d.active).length;
  K.innerHTML = kpi("Active time", dur(w.active, true)) + kpi("Active days", `${days}<small>/ ${w.days.length}</small>`) +
    kpi("Sessions / prompts", `${w.sessions}<small>/</small>${w.prompts}`) +
    (u.tokens ? kpi("Tokens", tok(u.tokens)) + kpi("Estimated cost", usd(u.cost).replace("$","<small>$</small>")) : "") +
    (u.credits ? kpi("Kiro credits", `${crN(u.credits)}<small>cr</small>`) : "") +
    (() => { const {ws, we} = period(), n = limitHits(ws, we).length; return n ? kpi("Usage limit hits", `<span style="color:var(--warn)">${n}</span>`) : ""; })() +
    (w.git && w.git.commits ? kpi("Commits (by AI)", `${w.git.commits}<small>${` (${w.git.ai})`}</small>`) :
     w.outputs && w.outputs.commits ? kpi("AI commits", `${w.outputs.commits}`) : "");
}

/* カレンダーの各日（月表示では各週も）に並べる 4 つ：作業時間・トークン（なければクレジット）・セッション・Git のコミット */
function calRows(act, u, nS, nC){
  const use = !u ? null : u.tokens ? ["Tokens", tok(u.tokens)] : u.credits ? ["Credits", cr(u.credits)] : null; // トークンがなければ（Kiro など）クレジット
  return [["Active", act ? dur(act) : "—", "k"], use && [...use, "k"], ["Sessions", nS], ["Commits", nC]].filter(Boolean);
}
const calDl = rows => `<dl class="cm">${rows.map(([k, v, c]) => `<div${c ? ` class="${c}"` : ""}><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>`; // k は強く出す値
const calShort = (act, u) => `<span class="acs">${act >= 60 ? (act/60).toFixed(1)+"h" : act+"m"}</span>${u && (u.tokens || u.credits) ? `<span class="uss">${u.tokens ? tokS(u.tokens) : cr(u.credits)}</span>` : ""}`; // スマホでは作業時間とトークン（なければクレジット）だけ
/* 月のカレンダー：日ごとの作業時間を濃さで、プロジェクトの配分を細い帯で */
function monthGrid(shown, ms, me, todayKey){
  const T = $("#tl"), S = MONTHS[mkey(st.month)], first = mondayOf(st.month);
  const max = S ? Math.max(1, ...S.days.map(d => d.active)) : 1;
  let h = `<div class="mgrid"><div class="mh"></div>${[1,2,3,4,5,6,0].map(i=>`<div class="mh${wkc(i)}">${dow(i)}</div>`).join("")}`;
  for (let r = 0; r < 6; r++){
    const wk = addDays(first, 7*r); if (wk.getTime()/1000 >= me) break;
    const W = WEEKS[key(wk)], wa = W ? W.active : 0, wc = W && W.git ? W.git.commits : 0; // 週の合計（月の外の日も含む、その週まるごと）
    h += `<button class="cell wkc" data-w="${key(wk)}"${tipAttr(`Week ${isoWeek(wk)}`, W ? [`Active ${dur(wa)}`, `Sessions ${W.sessions}`, ...useLines(W.usage), `Git commits ${wc}`] : "No records")}><span class="dn">W${isoWeek(wk)}</span>${W && wa ? calDl(calRows(wa, W.usage, W.sessions, wc)) + calShort(wa, W.usage) : ""}</button>`;
    for (let c = 0; c < 7; c++){
      const d = addDays(wk, c), ds = d.getTime()/1000, de = addDays(d,1).getTime()/1000, inM = d.getMonth() === st.month.getMonth();
      if (!inM){ h += `<div class="cell out"><span class="dn">${d.getDate()}</span></div>`; continue; }
      const x = S ? S.days[d.getDate()-1] : null, act = x ? x.active : 0;
      const by = {}; shown.forEach(s => s.segs.forEach(([a,b]) => { const o = Math.min(b,de) - Math.max(a,ds); if (o > 0) by[keyOf(s)] = (by[keyOf(s)]||0) + o; }));
      const pj = Object.entries(by).sort((a,b)=>b[1]-a[1]), nS = shown.filter(s => inP(s, ds, de)).length;
      const nC = x && x.commits || 0;
      h += `<button class="cell${d.getDay()%6===0?" we":""}${wkc(d.getDay())}${key(d)===todayKey?" today":""}" data-w="${key(mondayOf(d))}" style="--heat:${(act/max).toFixed(3)}"${tipAttr(md(ds), act ? [`Active ${dur(act)}`, `Sessions ${nS}`, ...useLines(x), `Git commits ${nC}`] : "No records")}>
        <span class="dn">${d.getDate()}</span>${act ? calDl(calRows(act, x, nS, nC)) + calShort(act, x) : ""}
        ${pj.length ? `<span class="pj">${pj.map(([k,v])=>`<span style="flex:${v};--c:${colorOf(k)}"></span>`).join("")}</span>` : ""}</button>`;
    }
  }
  h += `</div><div class="mlegend"><span style="white-space:nowrap">Less</span> ${[0,.25,.5,.75,1].map(v=>`<i style="--h:${v}"></i>`).join("")} ${matchMedia("(max-width:820px)").matches ? "More (active time) · Each day and week shows active time and tokens (or credits) · Tap a date or week to open it" : "More (active time) · Each day, and each week on the left, shows active time, tokens (or credits), sessions and Git commits · Click a date or week to open it"}</div>`;
  T.innerHTML = h;
  T.querySelectorAll("[data-w]").forEach(b => b.onclick = () => { const [y,m,dd] = b.dataset.w.split("-").map(Number); st.week = new Date(y, m-1, dd); setMode("week"); });
}

/* 週のカレンダー：曜日が列、時刻が縦。重なるセッションだけ横に並べる */
function timeline(shown, inWeek, ws, we, todayKey){
  const T = $("#tl"), hh = HOURS[st.z] || 44, H = 24*hh;
  if (!inWeek.length || !shown.length){
    T.innerHTML = `<div class="empty"><svg class="i" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="15" rx="3"/><path d="M3 10h18M8 3v4M16 3v4"/></svg><p>${inWeek.length ? "No sessions match the current filters" : "No records this week. Use ← → to move between weeks."}</p></div>`;
    return;
  }
  const keep = T.querySelector(".calscroll"), top = keep ? keep.scrollTop : null;
  const nowS = nowMs()/1000, w = WEEKS[key(st.week)];
  let heads = '<div></div>', hours = "", cols = "";
  const nowH = nowS >= ws && nowS < we ? (() => { const d0 = new Date(nowS*1000); d0.setHours(0,0,0,0); return (nowS - d0.getTime()/1000)/3600; })() : -9;
  for (let h=1; h<24; h++) if (Math.abs(h - nowH) * hh >= 20) hours += `<span style="top:${h*hh}px">${String(h).padStart(2,"0")}:00</span>`;
  if (nowS >= ws && nowS < we){ const d0 = new Date(nowS*1000); d0.setHours(0,0,0,0); hours += `<span class="now" style="top:${(nowS - d0.getTime()/1000)/3600*hh}px">${hm(nowS)}</span>`; }
  const runs = [];
  for (let d=0; d<7; d++){
    const day = addDays(st.week,d), ds = day.getTime()/1000, de = addDays(st.week,d+1).getTime()/1000, isToday = key(day) === todayKey;
    const act = w && w.days[d] ? w.days[d].active : 0;
    const dW = w && w.days[d], nS = shown.filter(s => inP(s, ds, de)).length;
    const dayGit = (META.git || []).filter(c => c.t >= ds && c.t < de && (!st.hidden.size || st.colorBy !== "project" || !st.hidden.has(c.project))).sort((a,b) => a.t - b.t);
    heads += `<div class="head${isToday?" today":""}${wkc(day.getDay())}"><div class="dd"><b>${day.getMonth()+1}/${day.getDate()}</b><i>${dow(day.getDay())}</i></div>${act || dayGit.length ? `<div${tipAttr(md(ds), [`Active ${dur(act)}`, `Sessions ${nS}`, ...useLines(dW), `Git commits ${dayGit.length}`])}>${calDl(calRows(act, dW, nS, dayGit.length))}</div>` : `<small>—</small>`}</div>`;
    const blocks = [];
    shown.forEach(s => s.segs.forEach(([a,b,n]) => { const x = Math.max(a,ds), y = Math.min(b,de); if (y > x) blocks.push({s, a:x, b:y, n}); }));
    blocks.sort((p,q) => p.a-q.a || q.b-p.b);
    // 重なるものだけ横に並べる（かたまりごとに列の数を決める）
    let cluster = [], cEnd = -1;
    const flush = () => { const lanes = []; cluster.forEach(bk => { let i = lanes.findIndex(e => e <= bk.a); if (i<0){ i = lanes.length; lanes.push(0); } lanes[i] = bk.b; bk.lane = i; }); cluster.forEach(bk => bk.L = lanes.length); cluster = []; };
    blocks.forEach(bk => { if (bk.a >= cEnd){ flush(); cEnd = bk.b; } else cEnd = Math.max(cEnd, bk.b); cluster.push(bk); }); flush();
    let html = "";
    blocks.forEach((bk, j) => {
      const y = (bk.a-ds)/3600*hh, h = Math.max(4, (bk.b-bk.a)/3600*hh - 2), mins = (bk.b-bk.a)/60;
      const dens = Math.min(1, bk.n / Math.max(1, mins) / 2.5), id = runs.length;
      runs.push(bk);
      html += `<button class="run${h < 16 ? " thin" : ""}${st.sel === bk.s.id ? " sel" : ""}" data-r="${id}" data-sid="${esc(bk.s.id)}" style="top:${y}px;height:${h}px;left:calc((100% - var(--g)) * ${(bk.lane/bk.L).toFixed(4)} + 3px);width:calc((100% - var(--g)) / ${bk.L} - 6px);--c:${colorOf(keyOf(bk.s))};--fill:${Math.round(16+30*dens)}%;${st.animate?`--delay:${d*30+Math.min(j,14)*10}ms`:"animation:none"}" aria-label="${esc(`${bk.s.title}, ${bk.s.project}, ${md(bk.a)} ${hm(bk.a)} to ${hm(bk.b)}`)}">${h >= 20 ? `<span class="t"><span>${esc(bk.s.title)}</span></span>` + (h >= 38 ? `<span class="m">${hm(bk.a)}–${hm(bk.b)} · ${esc(bk.s.project)}</span>` : "") : ""}</button>`;
    });
    let gy = -99; // コミットは右端の溝に、時刻の順に置く（近すぎるものは少し下へずらす）
    dayGit.forEach(c => {
      const y = Math.max((c.t-ds)/3600*hh, gy + 17); gy = y;
      html += `<button class="gc${c.ai ? " ai" : ""}${st.sel === "git:"+c.hash ? " sel" : ""}" data-c="${esc(c.hash)}" style="top:${y}px" title="${esc(`${hm(c.t)} ${c.project}${c.branch ? " · "+c.branch : ""} · ${c.hash}\n${c.subject}\n${plural(c.nFiles, "file")} +${c.added} −${c.removed}${c.ai ? " · run by AI" : ""}`)}" aria-label="${esc(`Commit ${hm(c.t)} ${c.subject}`)}">${GIT_ICON}<span>${esc(c.hash.slice(0,7))}</span></button>`; });
    limitHits(ds, de).filter(h => matches(h.s)).forEach(h => { html += `<button class="lim" data-id="${esc(h.s.id)}" style="top:${(h.t-ds)/3600*hh}px" title="${esc(`${hm(h.t)} Hit the usage limit (${h.s.title})`)}" aria-label="${esc(`${hm(h.t)} usage limit`)}">Limit</button>`; });
    if (isToday && nowS >= ds && nowS < de) html += `<div class="nowline" style="top:${(nowS-ds)/3600*hh}px"></div>`;
    cols += `<div class="day${day.getDay()%6===0?" we":""}${wkc(day.getDay())}${isToday?" today":""}" style="height:${H}px;--g:${dayGit.length ? 18 : 0}px">${html}</div>`;
  }
  T.innerHTML = `<div class="calscroll" style="--hh:${hh}px"><div class="calin"><div class="heads">${heads}</div><div class="cgrid"><div class="hours" style="height:${H}px">${hours}</div>${cols}</div></div></div>`;
  const sc = T.querySelector(".calscroll");
  sc.style.scrollPaddingTop = T.querySelector(".heads").offsetHeight + "px"; sc.style.scrollPaddingLeft = "56px"; // Tab で移ったブロックが、固定の日付・時刻の下に隠れないように
  if (top != null) sc.scrollTop = top;
  T.querySelectorAll(".lim").forEach(el => el.onclick = e => { e.stopPropagation(); select(el.dataset.id); });
  T.querySelectorAll(".gc").forEach(el => el.onclick = e => { e.stopPropagation(); select("git:" + el.dataset.c); });
  T.querySelectorAll(".run").forEach(el => { const bk = runs[+el.dataset.r];
    el.onclick = e => { e.stopPropagation(); select(bk.s.id); };
    el.onmouseenter = e => tipOn(e, bk); el.onmousemove = tipMove; el.onmouseleave = tipOff; });
}

/* ── tooltip ── */
function tipOn(e, bk){ const t = $("#tip"), s = bk.s;
  t.innerHTML = `<b>${esc(s.title)}</b><div class="r" style="--c:${colorOf(keyOf(s))}"><i></i>${esc(s.project)}${s.branch?` · ${esc(s.branch)}`:""}</div><div class="r">${md(bk.a)} ${hm(bk.a)}–${hm(bk.b)}${` (${dur((bk.b-bk.a)/60)})`}</div><div class="r">${esc((s.source))} · ${plural(s.nPrompts, "prompt")}${s.cost ? ` · ${usd(s.cost)}` : ""}${s.credits ? ` · ${crN(s.credits)} credits` : ""}${s.subagents.length ? ` · ${plural(s.subagents.length, "subagent")}` : ""}</div>`;

  t.classList.add("on"); tipMove(e); }
function tipMove(e){ const t = $("#tip"), w = t.offsetWidth, h = t.offsetHeight;
  let x = e.clientX + 14, y = e.clientY + 16; if (x + w > innerWidth - 12) x = e.clientX - w - 14; if (y + h > innerHeight - 12) y = e.clientY - h - 14;
  t.style.left = x+"px"; t.style.top = y+"px"; }
function tipOff(){ $("#tip").classList.remove("on"); }
// グラフの棒・帯・点は data-tip に書いた文を、マウスを載せる（タッチでは触れる）とすぐ出す。1 行目は見出し、続く行は「名前 値」
const useLines = d => d ? [d.tokens ? `Tokens ${tok(d.tokens)}` : "", d.cost >= 0.005 ? `Estimated cost ${usd(d.cost)}` : "", d.credits ? `Credits ${cr(d.credits)}` : ""].filter(Boolean) : [];
const tipAttr = (...lines) => ` data-tip="${esc(lines.flat().filter(Boolean).join("\n"))}"`;
function tipText(e, text){ const t = $("#tip"), [h, ...r] = (text.includes("\n") ? text : text.replace(/・| · /g, "\n")).split("\n"); // 1 行で書いたもの（月表示の日など）は「・」で行に分ける
  t.innerHTML = `<b>${esc(h)}</b>${r.map(x => `<div class="r">${esc(x)}</div>`).join("")}`; t.classList.add("on"); tipMove(e); }
document.addEventListener("pointerover", e => { const el = e.target.closest && e.target.closest("[data-tip]"); if (el) tipText(e, el.dataset.tip); });
document.addEventListener("pointermove", e => { const el = e.target.closest && e.target.closest("[data-tip]"); if (!el) return; // スクロールで消えたあとも、動かせばまた出す
  if ($("#tip").classList.contains("on")) tipMove(e); else tipText(e, el.dataset.tip); });
document.addEventListener("pointerout", e => { const el = e.target.closest && e.target.closest("[data-tip]"); if (el && !(e.relatedTarget && el.contains(e.relatedTarget))) tipOff(); });
addEventListener("scroll", () => { if (!st.sel) tipOff(); }, {passive: true, capture: true});

/* ── 週次・月次サマリー：① プロジェクト別 ② コストとアウトプット ③ 時間 ④ AI ⑤ かたち ⑥ 改善案を聞く。基準を超えた指標には、その場に印と推移を付ける ── */
function summary(){
  const {S:w, P:pw} = period(), R = $("#review"), M = st.mode === "month", unit = M ? "月" : "週";
  const head = `<div class="rvhead"><h2>${M ? "Monthly summary" : "Weekly summary"}</h2><p>Rough figures for reflecting on how you work. They are not for comparing people or for evaluations.</p>${w ? `<button class="pill rpt" id="rpttog" aria-expanded="${!!st.rpt}" aria-controls="rptbox">${`${M ? "Monthly" : "Weekly"} report draft`}</button><button class="pill rpt" id="pxtog" aria-expanded="${!!st.px}" aria-controls="pxbox">Export prompts</button>` : ""}</div>${w ? `<section class="panel rptbox" id="pxbox"${st.px ? "" : " hidden"} aria-label="Export prompts">
    <div class="askbar"><button class="pill" id="pxcopy">Copy</button><button class="pill" id="pxclose">Close</button>
      <span class="muted" style="font-size:var(--fs-xs)">${`Markdown with only the user prompts ${uThis(unit)} (commands included), by session in time order. Automatic notifications and summaries are left out.`}</span></div>
    <pre class="askpre" id="pxpre">${esc(periodPrompts())}</pre></section>` : ""}${w ? `<section class="panel rptbox" id="rptbox"${st.rpt ? "" : " hidden"} aria-label="${`${M ? "Monthly" : "Weekly"} report draft`}">
    <div class="askbar"><button class="pill" id="rptcopy">Copy</button><button class="pill" id="rptclose">Close</button>
      <span class="muted" style="font-size:var(--fs-xs)">Markdown with what you did, commits and pull requests for each project. Session names are the start of your prompts, so edit them before pasting.</span></div>
    <pre class="askpre" id="rptpre">${esc(reportText(w, M))}</pre></section>` : ""}`;
  if (!w){ R.innerHTML = `${head}<div class="rvgrid"><div class="panel"><p class="none">${`No records ${uThis(unit)}.`}</p>${foot()}</div></div>`; bindCopy(R); return; }
  const longest = Math.max(0, ...w.focus.map(b=>b.min));
  const total = w.projects.reduce((t,[,v])=>t+v,0) || 1;
  const stat = (k, v, s, h) => `<div class="stat"><div class="k">${k}${hb(h)}</div><div class="v">${v}</div>${s?`<div class="s">${s}</div>`:""}${hint(h)}</div>`;
  const ph = (n, t, s, h) => `<div class="ph"><span class="no">${n}</span><b>${t}${hb(h)}</b><span>${s}</span></div>${hint(h)}`;
  const pct = v => v == null ? "Unknown" : `${v}<small>%</small>`;
  const times = n => `${n}`;
  const F = findList(w, pw, unit);
  R.innerHTML = `${head}${keepNotice()}${flagSum(F)}<div class="rvgrid">
  ${projectPanel(w, ph, unit)}
  ${outcomePanel(w, pw, unit, ph, stat)}
  <section class="panel">${ph(3, "How you spent time", "When and how long sessions ran")}
    <div class="stats">
      ${stat("Focus blocks (60+ min)", times(w.focus.length), longest ? `Longest ${dur(longest)}` : "", "focus")}
      ${stat("Weekend", dur(w.weekend,true), "", "weekend")}
      ${stat("Total AI run time", dur(w.ai,true), "Includes parallel runs", "ai")}
      ${(() => { const {ws, we} = period(), H = limitHits(ws, we); return stat("Usage limit hits", times(H.length), H.length ? H.slice(-4).map(h => `${md(h.t)} ${hm(h.t)}`).join(", ") + (H.length > 4 ? ", …" : "") : "From Claude Code history", "limits"); })()}
    </div>
    <details class="moreS" id="moreS"${st.moreS ? " open" : ""}><summary>More metrics (includes estimates)</summary><div class="stats">
      ${stat("Prompts with corrections or interruptions", pct(w.fixRate), `n=${w.prompts}`, "fix")}
      ${(() => { const {ws, we} = period(), B = bigOf(ws, we); return stat("Oversized prompts", `${B.n}`, `${BIG_PROMPT.toLocaleString()}+ characters${B.n ? ` · longest ${B.max.toLocaleString()}` : ""}`, "bigPrompts"); })()}
      ${stat("Project switches per day", times(w.switchesAvg), `Max ${w.switchesMax}`, "switches")}
      ${stat("Parallel time", dur(w.parallel,true), `Up to ${w.maxConc} at once`, "parallel")}
      ${stat("Wait time (median)", secsH(w.waitMedian), `n=${w.waitCount} · 90th percentile ${secs(w.waitP90)}`, "wait")}
    </div></details>
    </section>
  <section class="panel">${ph(4, "How you used AI", "Cache, models and heavy sessions")}
    ${aiUsage(w, pw, unit) || `<p class="none">No token or credit records.</p>`}
    ${nativeSection(w)}</section>
  <section class="panel shape">${ph(5, M ? "Shape of the month" : "Shape of the week", "Focus and friction")}
    ${w.focus.length ? `<h3>Focus blocks${hb("focusList")}</h3>${hint("focusList")}` : ""}
    ${w.focus.length ? [...w.focus].sort((a,b)=>b.min-a.min).slice(0,5).map(b=>`<div class="focusrow" style="--c:${st.colorBy==="project"?colorOf(b.project):"var(--ink-3)"}"><span class="when">${md(b.t)} ${hm(b.t)}</span><span class="track2"><span style="width:${b.min/longest*100}%"></span></span><span class="len">${dur(b.min)}</span></div>`).join("") : ""}
    ${w.friction.length ? `<h3>Possible friction${hb("friction")}</h3>${hint("friction")}` : ""}
    ${w.friction.length ? w.friction.map(f=>`<button class="card" data-id="${esc(f.id)}"><span class="ti">${esc(f.title)}</span><span class="me">${md(f.start)} · ${esc(f.project)} ${whyOf(f).map(x=>`<span class="tagx">${esc(x)}</span>`).join("")}</span></button>`).join("") : ""}
    ${repeatsOf(period().ws, period().we).length ? `<h3>Repeated prompts${hb("repeats")}</h3>${hint("repeats")}` : ""}
    ${(() => { const {ws, we} = period(), RP = repeatsOf(ws, we); return RP.length ? RP.slice(0, 3).map(c => `<button class="card" data-id="${esc(c.id)}"><span class="ti">${esc(snipOf(c.text, 90))}</span><span class="me">${`${c.n} times in ${c.ids.size} sessions · last on ${md(c.last)}`}</span></button>`).join("") : ""; })()}
    ${w.focus.length || w.friction.length || repeatsOf(period().ws, period().we).length ? "" : `<p class="none">No work of 60+ minutes, no sessions with possible friction and no repeated prompts.</p>`}</section>
  <section class="panel ask">${ph(6, "Ask AI for suggestions", "A prompt that asks for suggestions based on this data")}
    <div class="askbar"><button class="pill" id="askcopy">Copy prompt</button>
      <span class="muted" style="font-size:var(--fs-xs)">Paste it into the AI agent you use. kiroku never calls an AI. It includes session names (parts of your prompts) and project names, so review it before sending.</span></div>
    <pre class="askpre" id="askpre">${esc(askPrompt(w, pw, M))}</pre></section>
  <section class="panel metap">${measure(w)}${foot()}</section></div>`;
  placeFlags(R, F);
  R.querySelectorAll(".card,.pses,.fses").forEach(c => c.onclick = () => select(c.dataset.id));
  R.querySelectorAll("[data-goto]").forEach(b => b.onclick = () => { // 見直す候補から、印の付いた指標へ移動して説明を開く
    const t = R.querySelector(`.panel .hb[data-help="${b.dataset.goto}"]`) || R.querySelector(`.panel.${b.dataset.goto}`); if (!t) return;
    const dt = t.closest("details"); if (dt && !dt.open){ dt.open = true; st.moreS = true; }
    t.scrollIntoView({behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "center"});
    if (t.classList.contains("hb") && t.getAttribute("aria-expanded") !== "true") t.click(); });
  const more = R.querySelector("#pmore"); if (more) more.onclick = () => { st.allProj = !st.allProj; summary(); };
  const ko = R.querySelector("#keepoff"); if (ko) ko.onclick = () => { store.set("keepNoticeOff", true); summary(); };
  const ka = R.querySelector("#keeparch"); if (ka) ka.onclick = () => { ka.disabled = true; keepArchive(); };
  const ms = R.querySelector("#moreS"); if (ms) ms.ontoggle = () => { if (ms.dataset.auto){ delete ms.dataset.auto; return; } st.moreS = ms.open; }; // 印のために自動で開いたときは、次の期間に持ち越さない
  const rt = R.querySelector("#rpttog"), rb = R.querySelector("#rptbox"), rpt = open => { st.rpt = open; rb.hidden = !open; rt.setAttribute("aria-expanded", open); if (open) rb.scrollIntoView({behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest"}); else rt.focus(); };
  if (rt && rb){ rt.onclick = () => rpt(!st.rpt); R.querySelector("#rptclose").onclick = () => rpt(false); }
  const xt = R.querySelector("#pxtog"), xb = R.querySelector("#pxbox"), px = open => { st.px = open; xb.hidden = !open; xt.setAttribute("aria-expanded", open); if (open) xb.scrollIntoView({behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest"}); else xt.focus(); };
  if (xt && xb){ xt.onclick = () => px(!st.px); R.querySelector("#pxclose").onclick = () => px(false); R.querySelector("#pxcopy").onclick = () => copy($("#pxpre").textContent); }
  const rc = R.querySelector("#rptcopy"); if (rc) rc.onclick = () => copy($("#rptpre").textContent, "Copied. Session names are the start of your prompts, so edit them before pasting", 4500);

  const ac = R.querySelector("#askcopy"); if (ac) ac.onclick = () => copy($("#askpre").textContent);
  R.querySelectorAll("#useBy button").forEach(b => b.onclick = () => { st.use = b.dataset.v; store.set("use", st.use); summary(); });
  bindCopy(R); bindHelp(R);
}
/* 期間の言い方（unit は "週" か "月"） */
const uThis = unit => unit === "月" ? "this month" : "this week";
const uLast = unit => unit === "月" ? "last month" : "last week";
const uNext = unit => unit === "月" ? "next month" : "next week";
const whyOf = f => f.whyEn && f.whyEn.length === f.why.length ? f.whyEn : f.why; // こじれた理由（Go の英語の文）
/* 配分：作業時間・トークン・目安コスト・クレジットのそれぞれで、何割をどのくくりに使ったか。くくりは色分け（プロジェクト・ブランチ・エージェント）に合わせる。
   指標を縦に並べ、「時間の割にコストが多い」のようなずれを見比べられるようにする（上位 5 つ＋その他） */
function shareBlock(w, ps){
  const all = st.colorBy === "project" ? ps.map(p => ({key: p.project, minutes: p.minutes, tokens: p.tokens, cost: p.cost, credits: p.credits})) : ((w.shares || {})[st.colorBy] || []);
  const head = `<h3 class="shh">${`Share by ${CB()[st.colorBy].toLowerCase()}`}</h3><p class="shn">Follows the Project / Branch / Agent switch at the top</p>`;
  if (!all.length) return "";
  if (all.length < 2) return head + `<p class="none">${`Only "${esc(all[0].key)}" this period.`}</p>`;
  const N = 5, top = all.slice(0, N), rest = all.slice(N);
  const rows = top.map(p => ({name: p.key, c: colorOf(p.key), v: p}));
  if (rest.length) rows.push({name: `Other (${rest.length})`, c: "var(--other)", other: true,
    v: rest.reduce((a, p) => ({minutes: a.minutes + p.minutes, tokens: a.tokens + p.tokens, cost: a.cost + p.cost, credits: a.credits + p.credits}), {minutes: 0, tokens: 0, cost: 0, credits: 0})});
  const ms = [["minutes", "Active time", dur], ["tokens", "Tokens", tok], ["cost", "Est. cost", usd], ["credits", "Credits", cr]]
    .map(([k, n, f]) => ({k, n, f, t: rows.reduce((t, r) => t + (r.v[k] || 0), 0)})).filter(m => m.t > 0);
  if (!ms.length) return "";
  const pc = (r, m) => Math.round((r.v[m.k] || 0) * 100 / m.t);
  const bars = ms.map(m => `<span class="k">${m.n}</span><div class="stack" role="img" aria-label="${esc(`${m.n}: ${rows.map(r => `${r.name} ${pc(r, m)}%`).join(", ")}`)}">${rows.filter(r => r.v[m.k] > 0).map(r => `<span style="flex:${r.v[m.k]};--c:${r.c}" ${tipAttr(r.name, `${m.n} ${m.f(r.v[m.k])}${` (${pc(r, m)}%)`}`)}></span>`).join("")}</div>`).join("");
  const tab = `<table class="shtab"><thead><tr><th scope="col">${CB()[st.colorBy]}</th>${ms.map(m => `<th scope="col">${m.n}</th>`).join("")}</tr></thead><tbody>${rows.map(r => `<tr style="--c:${r.c}"><td title="${esc(r.name)}"><i aria-hidden="true"></i>${esc(r.name)}</td>${ms.map(m => `<td>${r.v[m.k] ? pc(r, m) + "%" : "—"}</td>`).join("")}</tr>`).join("")}</tbody></table>`;
  return `${head}<div class="sharebox"><div class="share">${bars}</div>${tab}</div>`;
}
/* プロジェクト別：何に何時間、どれだけのトークン・コスト・クレジットを、どのモデル中心に使い、何が重かったか */
function projectPanel(w, ph, unit){
  const ps = w.projectStats || []; if (!ps.length) return "";
  const total = w.projects.reduce((t,[,v])=>t+v,0) || 1, LIMIT = 6, shown = st.allProj ? ps : ps.slice(0, LIMIT);
  const use = x => [x.cost >= 0.005 ? usd(x.cost) : "", x.credits ? cr(x.credits) : "", !x.cost && x.tokens ? tok(x.tokens)+" tok" : ""].filter(Boolean).join(" · ");
  const cards = shown.map(p => {
    const c = st.colorBy === "project" ? colorOf(p.project) : "var(--ink-3)", share = Math.round(p.minutes*100/total);
    const mt = p.models.reduce((t,m)=>t+m.tokens,0) || 1; // トークンのないモデル（Kiro など）は回数で出す
    return `<article class="pcard" style="--c:${c}">
      <div class="hd"><i></i><b title="${esc(p.project)}">${esc(p.project)}</b><span>${p.minutes ? dur(p.minutes) : "—"}${p.minutes ? ` · ${share}%` : ""}</span></div>
      <div class="bar2"><span style="width:${share}%"></span></div>
      <div class="kv"><div><div class="k">Sessions / prompts</div><div class="v">${p.sessions}/${p.prompts}</div></div>
        <div><div class="k">Tokens</div><div class="v">${p.tokens ? tok(p.tokens) : "—"}</div></div>
        <div><div class="k">Est. cost</div><div class="v">${p.cost >= 0.005 ? usd(p.cost) : "—"}</div></div>
        <div><div class="k">Credits</div><div class="v">${p.credits ? cr(p.credits) : "—"}</div></div></div>
      ${p.git && p.git.commits ? `<div><div class="lab">Outputs</div><div class="pout">Commits ${p.git.commits} (AI ${p.git.ai}) · <span>+${p.git.added} −${p.git.removed} lines</span></div></div>` : p.outputs && p.outputs.commits ? `<div><div class="lab">Outputs</div><div class="pout">AI commits ${p.outputs.commits}</div></div>` : ""}
      ${p.models.length ? `<div><div class="lab">Main models</div><div class="mods">${p.models.map(m=>`<span title="${esc((m.model))}">${esc((m.model))}<b>${m.tokens ? Math.round(m.tokens*100/mt)+"%" : plural(m.turns, "turn")}</b></span>`).join("")}</div></div>` : ""}
      <div><div class="lab">Top sessions</div>${p.top.map(t=>`<button class="pses" data-id="${esc(t.id)}"><span class="t">${esc(t.title)}</span><span class="m">${dur(t.minutes)}${use(t) ? " · "+use(t) : ""}</span></button>`).join("")}</div>
    </article>`; }).join("");
  return `<section class="panel projs">${ph(1, "By project", `Where your time and AI went ${uThis(unit)}`, "projects")}
    ${shareBlock(w, ps)}
    <div class="pgrid">${cards}</div>
    ${ps.length > LIMIT ? `<button class="pill pmore" id="pmore">${st.allProj ? "Show top projects only" : `Show ${plural(ps.length - LIMIT, "more project")}`}</button>` : ""}</section>`;

}
/* 指標の読み方：定義・言えること・言えないこと・打てる手。画面の「?」と、改善案プロンプトの前提に使う */
const HELP = {
  findings: {n: "Worth a look", d: "Metrics that crossed a fixed threshold. Each is marked (●) where it appears, with what was observed, an 8-period trend, the related sessions and the threshold; their names are listed here in priority order", c: "Which metric is worth reviewing first", x: "Whether something is good or bad. Thresholds are generic and may not fit how you work", a: "Pick one, try it next period, and check the change with the same metric"},
  projects: {n: "By project", d: "Time, usage, main models and top sessions per project. Time when several projects ran at once is split between them", c: "How you divided time and AI across projects", x: "How important or successful a project was", a: "If the split differs from what you intended, revisit how you work or your priorities"},
  active: {n: "Active time", d: "Time when any session was running (overlaps count once)", c: "Total time you worked together with AI", x: "Whether you were focused. Includes time you left a session idle", a: "Compare with the previous period to see how much more or less you rely on AI"},
  ai: {n: "Total AI run time", d: "Session run time added up, including sessions running in parallel", c: "How much work you gave to AI. The gap from active time shows how much ran in parallel", x: "Time saved or productivity", a: "If it is close to active time, you could do other work while waiting"},
  focus: {n: "Focus blocks", d: "Work that continued for 60 minutes or more. Gaps within a session up to the session gap (15 minutes by default) and gaps of up to 5 minutes between sessions are treated as continuous", c: "Whether you had long stretches of uninterrupted work", x: "The quality of the work in that time", a: "If your time is fragmented, group the work you hand to AI into blocks of time"},
  focusList: {n: "Focus blocks (list)", d: "The 5 longest", c: "When and in which project you worked for long stretches", x: "Outcomes", a: "Learn when you focus best and schedule heavy work then"},
  switches: {n: "Project switches per day", d: "How often the project changed between consecutive prompts (daily average and maximum)", c: "How much you moved between projects", x: "Whether switching is bad (you may just be using wait time well)", a: "If high, next period limit each day to 2–3 projects"},
  parallel: {n: "Parallel time", d: "Time when 2 or more sessions ran at once, and the most at once", c: "Whether you kept several sessions going in parallel", x: "What the parallel work achieved", a: "If low, give AI another task while it works"},
  wait: {n: "Wait time", d: "Median and 90th percentile of the time from an AI reply to your next prompt (up to 30 minutes)", c: "How quickly you responded to AI replies", x: "Shorter is not always better (you may be moving on without checking)", a: "If long, use notifications or batch your reviews"},
  weekend: {n: "Weekend", d: "Work time on Saturdays and Sundays", c: "How much you worked on weekends", x: "Whether you are overworking", a: "If not intended, pick a day off for next period"},
  fix: {n: "Prompts with corrections or interruptions", d: "Share estimated from the opening words of each prompt (such as 'No, that's wrong' or 'undo that') and interruptions. The first prompt of a conversation and words inside pasted code or logs are not counted", c: "Roughly how often a first prompt did not get your intent across", x: "Estimates can be wrong, and they don't show the cause", a: "If high, add background, constraints and done criteria to your prompts"},
  limits: {n: "Usage limit hits", d: "Count and times of usage limit errors (usage caps and rate limits) in Claude Code history. Hits within 1 minute count once", c: "When and during which work you hit a limit and had to stop", x: "How much headroom is left. Usage from other agents, the browser or the app", a: "Spread heavy work over time, use lighter models, and start new sessions for long conversations"},
  longctx: {n: "Long conversations", d: "Sessions where the input read per response (new input plus cache reads and writes) in the last quarter of the conversation was at least 4 times that of the first quarter, peaking at 100K tokens or more (estimated cost $0.5 or more; only agents that record tokens)", c: "Sessions where each response got heavier as the conversation went on", x: "Whether continuing the conversation was the right call (some work needs the earlier context)", a: "At a good stopping point, write down the key points and continue in a new session"},
  modelfit: {n: "Expensive models for light work", d: "Total for sessions that mainly used Opus-class models, had 3 or fewer prompts, edited no files and cost $0.3 or more", c: "How much went to short tasks on an expensive model", x: "Whether that model was needed (some research or design questions are hard)", a: "Try a lighter model first for research and questions, and switch if it falls short"},
  friction: {n: "Sessions with possible friction", d: "Up to 3 sessions started in the period with at least one correction or interruption, or 15 or more prompts, most first", c: "Sessions where rework piled up", x: "Why it went wrong", a: "Next period, split big requests into one step per prompt"},
  bigPrompts: {n: "Oversized prompts", d: "Number of prompts of 4,000+ characters in the period, and the length of the longest", c: "Whether you hand over large inputs at once, such as pasting whole logs or documents", x: "Whether that length was needed (some long prompts, like design explanations, are fine)", a: "Next period, put long logs or documents in a file and write only its path and the part to look at"},
  repeats: {n: "Repeated prompts", d: "Prompts of 12+ characters in the period, grouped by how similar their text is; up to 3 written in 3 or more sessions, most first", c: "Routine requests you type every time", x: "Whether those requests worked well", a: "Next period, write the most repeated one once as a custom command or in CLAUDE.md"},
  daily: {n: "Daily trend", d: "A bar chart per day that switches between tokens, credits, estimated cost, active time, sessions and prompts", c: "Which days you worked the most and used AI the most", x: "Whether that day's usage was appropriate", a: "Open the sessions on outlier days to see why they were heavy"},
  cost: {n: "Estimated cost (API pricing)", d: "Usage priced at public API rates. Uses the cost Claude Code records for itself when available (this also covers price changes, new models and calls not in the history, such as auto-compaction); otherwise tokens in the history times the kiroku price table", c: "A rough way to compare how heavy usage was, in money", x: "What you are actually billed (subscriptions differ)", a: "Open the sessions behind the increase, and next period keep that kind of work in shorter conversations"},
  projection: {n: "Month-end projection (estimate)", d: "In a month in progress, the estimated cost (and credits) from the 1st through today, divided by the days so far and multiplied by the days in the month. Not shown for the first 7 days or on the last day", c: "Roughly where this month is heading at the current pace", x: "Your actual bill, or how you will work from now on (it is off if the pace changes)", a: "If it is too high, look at the heavy sessions and models"},
  tokens: {n: "Tokens", d: "Input, output, cache reads and cache writes combined", c: "How much you consumed", x: "Whether more or less is good", a: "Look for skew by project and by day"},
  cache: {n: "Share of input read from cache", d: "The share of input tokens read from cache", c: "Whether the same context was reused", x: "Why it is low (it may just be many short sessions)", a: "If low, next period write long background once in a project file (such as CLAUDE.md) instead of pasting it every time"},
  models: {n: "By model", d: "Estimated cost and tokens per model", c: "Which models your usage leaned toward", x: "Whether that model was needed", a: "Next period, try lighter models for routine work (formatting, renames, adding tests)"},
  subagents: {n: "Subagents", d: "Number of Task / Agent calls, their types and total run time", c: "Whether you delegated research and similar work", x: "How much delegating helped", a: "Check that you use them to save the main conversation's context"},
  credits: {n: "Kiro credits", d: "Total credits recorded in Kiro history", c: "Credits actually consumed", x: "Differences from your account page (period boundaries or use on other machines)", a: "Track your pace against your limit"},
  costPerAsk: {n: "Estimated cost per prompt", d: "Estimated cost ÷ number of prompts", c: "How heavy a typical prompt was", x: "Differences in prompt size", a: "Watch the trend to see how the size of your prompts changes"},
  heavy: {n: "Heaviest sessions", d: "The 3 sessions with the highest estimated cost", c: "Sessions that drove usage up", x: "Whether the result was worth it", a: "Next period, restart long conversations in a new session, and carry work to a commit in small steps"},
  outputs: {n: "Outputs", d: "Commits, pull requests and file edits that AI ran with tools and that succeeded (Claude Code only for now)", c: "Whether the cost turned into work that left a trace", x: "Value, quality or productivity. Commits you made by hand are not included", a: "Put them next to cost and look for usage that produced nothing"},
  gitCommits: {n: "Git commits", d: "Your own commits (user.email) in the repositories agents worked in, including ones made by hand. Each mark on the right edge of a day in the calendar is one commit (filled = run by AI; touch it to see the short hash)", c: "How much of your time with AI became recorded changes", x: "The value of the changes. Work outside the repositories or commits by others", a: "On days with much time or cost but few commits, check where the time went"},
  commits: {n: "Commits", d: "Number of git commits that AI ran successfully", c: "Roughly how often work reached a checkpoint", x: "The value or size of the changes. Commit size varies by person and task", a: "In periods with few commits for the cost, check where the time went"},
  outSessions: {n: "Sessions that reached a commit", d: "Number and share of sessions that made a commit or pull request in the period, counting only Claude Code sessions, the only agent whose outputs are recorded", c: "The share of sessions that left something behind", x: "The value of sessions not meant to commit, such as research or discussion", a: "If low, next period state at the start of each session what done looks like (when to commit)"},
  costPerCommit: {n: "Estimated cost per commit", d: "Estimated cost of Claude Code sessions ÷ number of commits (only Claude Code records outputs, so other agents' cost is left out)", c: "Roughly how heavy it was to reach a checkpoint", x: "Differences in commit size. Commits made by hand are not included", a: "Next period, keep each prompt to one change and commit often"},
  native: {n: "Agent-specific metrics", d: "Numbers each agent records in its history", c: "Trends within the same agent", x: "Comparisons between agents (definitions differ)", a: "Only look at changes over time for the same agent"},
};
const H = () => HELP;
const openHelp = new Set(); // 再描画（週の移動・自動更新）しても開いた説明は開いたまま
function hb(id){ return H()[id] ? `<button class="hb" data-help="${id}" aria-label="${`How to read ${H()[id].n}`}" aria-expanded="${openHelp.has(id)}">?</button>` : ""; }
function hint(id){ const h = H()[id]; if (!h) return "";
  return `<div class="hint" data-hint="${id}"${openHelp.has(id) ? "" : " hidden"}><p>${esc(h.d)}</p><dl><dt>Tells you</dt><dd>${esc(h.c)}</dd><dt>Doesn't tell you</dt><dd>${esc(h.x)}</dd><dt>What to try</dt><dd>${esc(h.a)}</dd></dl></div>`; }

function bindHelp(root){ root.querySelectorAll(".hb").forEach(b => b.onclick = e => { e.stopPropagation();
  let t = null; // 同じ指標の説明が画面に 2 か所あるので、押したボタンにいちばん近いものを開く
  for (let a = b.parentElement; a && !t; a = a === root ? null : a.parentElement) t = a.querySelector(`[data-hint="${b.dataset.help}"]`);
  if (!t) return; t.hidden = !t.hidden; t.hidden ? openHelp.delete(b.dataset.help) : openHelp.add(b.dataset.help); b.setAttribute("aria-expanded", String(!t.hidden)); }); }
/* AI に改善案を聞くためのプロンプト（kiroku 自身は AI を呼ばない。利用者が自分のエージェントに貼る） */
function askPrompt(w, pw, M){
  const unit = M ? "月" : "週", u = w.usage || {}, L = [];
  const start = M ? st.month : st.week, last = M ? new Date(st.month.getFullYear(), st.month.getMonth()+1, 0) : addDays(st.week, 6);
  const ymd = d => `${d.getFullYear()}/${d.getMonth()+1}/${d.getDate()}`;
  const cut = t => { t = String(t || "").replace(/\s+/g, " ").trim(); return t.length > 60 ? t.slice(0, 60) + "…" : t; };
  const use = x => [x.tokens ? `tokens ${tok(x.tokens)}` : "", x.cost >= 0.005 ? `estimated cost ${usd(x.cost)}` : "", x.credits ? `credits ${cr(x.credits)}` : ""].filter(Boolean).join(", ");
  const V = vsPrev(pw, unit); // 途中の期間は、前の期間の同じ日までと比べる
  const prev = (v, p, f, k) => (p = V.of(k, p)) == null ? "" : V.n == null ? ` (previous ${M ? "month" : "week"}: ${f(p)})` : ` (${V.range} of the previous ${M ? "month" : "week"}: ${f(p)})`;
  const sep = ", ", wk = M ? "month" : "week";
  L.push(`You are an advisor on using AI agents effectively. Below are figures from my AI agent history for ${ymd(start)}–${ymd(last)} (one ${wk}), aggregated with kiroku. Suggest improvements that help me use AI agents more effectively within limited credits and tokens.`,
    "", "# What I'd like from you",
    "1. What the data says about how I work (strengths and concerns)",
    `2. Up to 3 high-priority improvements. For each, cite the figures behind it and a concrete action I can try ${uNext(unit)}`,
    `3. Metrics to check the effect ${uNext(unit)}`,
    "", "# Assumptions",
    "- Estimated cost is token counts priced at public API rates. It is not what I am actually billed",
    "- Metric definitions differ by agent. Don't compare agents directly",
    "- All figures are rough estimates from history. Clearly mark anything the data can't support as a guess",
    ...(V.n == null ? [] : [`- This ${wk} is still in progress (${V.n} days, ${ymd(start)}–${ymd(today0())}). Figures for the previous ${wk} cover the same days (${V.range})`]),
    "- See \"How to read the metrics\" at the end for each metric's definition and what it can and cannot tell you",
    "", "# Metrics kiroku flagged by threshold (candidates, not verdicts)",
    ...(() => { const F = findList(w, pw, unit); return F.length ? F.map(f => `- ${f.see.replace(/<[^>]+>/g, "")} (threshold: ${f.rule.charAt(0).toLowerCase() + f.rule.slice(1)})`) : ["- No metric crossed a threshold"]; })(),
    "", "# Overall",
    `- Active time: ${dur(w.active)}${prev(w.active, pw && pw.active, dur, "active")}`,
    `- Total AI run time (including parallel runs): ${dur(w.ai)}`,
    `- Sessions / prompts: ${w.sessions} / ${w.prompts}`,
    `- Focus blocks (60+ min): ${w.focus.length}${w.focus.length ? `, longest ${dur(Math.max(...w.focus.map(b => b.min)))}` : ""}`,
    `- Project switches per day: average ${w.switchesAvg}, max ${w.switchesMax}`,
    `- Parallel time: ${dur(w.parallel)} (up to ${w.maxConc} at once)`,
    `- Wait time (from an AI reply to my next prompt): median ${secs(w.waitMedian)}, 90th percentile ${secs(w.waitP90)} (n=${w.waitCount})`,
    `- Weekend: ${dur(w.weekend)}`,
    `- Prompts with corrections or interruptions: ${w.fixRate == null ? "unknown" : w.fixRate + "%"} (n=${w.prompts})`,
    (() => { const {ws, we} = period(), H = limitHits(ws, we); return `- Usage limit hits (Claude Code): ${H.length ? `${H.length} (${H.map(h => `${md(h.t)} ${hm(h.t)}`).join(", ")})` : "0"}`; })());
  if (u.tokens || u.credits){
    L.push("", "# AI usage");
    if (u.tokens) L.push(`- Tokens: ${tok(u.tokens)} (output ${tok(u.out)})`,
      `- Share of input read from cache: ${u.cacheHit == null ? "unknown" : Math.round(u.cacheHit*100) + "%"}`,
      `- Estimated cost: ${usd(u.cost)}${prev(u.cost, pw && pw.usage && pw.usage.cost, usd, "cost")}${w.costPerAsk != null ? `, ${usd(w.costPerAsk)} per prompt` : ""}`);
    if (u.credits) L.push(`- Kiro credits: ${cr(u.credits)}`);
    L.push(`- Subagents: ${u.subagents}${u.subagents ? ` (total ${dur(u.subMin)})` : ""}`);
    if (u.models.length) L.push("- By model: " + u.models.slice(0, 6).map(r => `${(r[0])} (estimated cost ${usd(r[1])}, tokens ${tok(r[2])})`).join(sep));
  }
  const o = w.outputs;
  if (w.git && w.git.commits) L.push("", "# Git commits (my own commits in the repositories agents worked in)",
    `- ${w.git.commits} (${w.git.ai} run by AI), +${w.git.added} −${w.git.removed} lines per git`);
  if (o && o.commits){
    L.push("", "# Outputs (run by AI and succeeded; Claude Code only)",
      `- Commits: ${o.commits}`,
      `- Sessions that reached a commit: ${w.outSessions} / ${w.outBase ?? w.sessions}`);
    if (w.costPerCommit != null) L.push(`- Estimated cost per commit: ${usd(w.costPerCommit)}`);
  }
  const start0 = M ? st.month : st.week, days = w.days.map((d, i) => [addDays(start0, i), d]).filter(([, d]) => d.active || d.tokens || d.credits || d.commits);
  if (days.length){
    L.push("", "# By day");
    days.forEach(([dd, d]) => L.push(`- ${md(dd.getTime()/1000)}: active ${dur(d.active)}, prompts ${d.prompts}, switches ${d.switches}${use(d) ? ", " + use(d) : ""}${d.commits ? `, commits ${d.commits}` : ""}`));
  }
  const ps = (w.projectStats || []).slice(0, 8);
  if (ps.length){
    L.push("", "# By project");
    ps.forEach(p => {
      const mt = p.models.reduce((t, m) => t + m.tokens, 0) || 1;
      L.push(`## ${p.project}`, `- Active time: ${dur(p.minutes)}, sessions / prompts: ${p.sessions} / ${p.prompts}${use(p) ? ", " + use(p) : ""}`);
      if (p.git && p.git.commits) L.push(`- Git commits: ${p.git.commits} (${p.git.ai} by AI), +${p.git.added} −${p.git.removed} lines`);
      else if (p.outputs && p.outputs.commits) L.push(`- AI commits: ${p.outputs.commits}`);
      if (p.models.length) L.push("- Main models: " + p.models.map(m => `${(m.model)} (${m.tokens ? Math.round(m.tokens*100/mt) + "%" : plural(m.turns, "turn")})`).join(sep));
      if (p.top.length) L.push("- Sessions that took the most time: " + p.top.map(t => `"${cut(t.title)}" ${dur(t.minutes)}${use(t) ? ", " + use(t) : ""}`).join(sep));
      if (p.heavy && use(p.heavy)) L.push(`- Heaviest session: "${cut(p.heavy.title)}" ${use(p.heavy)}`);
    });
    if ((w.projectStats || []).length > ps.length) L.push(`- ${plural(w.projectStats.length - ps.length, "more project")}`);
  }
  if (w.friction.length){
    L.push("", "# Sessions with possible friction");
    w.friction.forEach(f => L.push(`- "${cut(f.title)}" (${f.project}): ${whyOf(f).join(", ")}`));
  }
  if (w.native && w.native.length){
    L.push("", "# Agent-specific metrics");
    w.native.forEach(g => L.push(`- ${(g.source)} (${plural(g.sessions, "session")}): ` + g.values.map(v => `${nlabel(v)} ${nativeText(v)}`).join(sep)));
  }
  L.push("", "# How to read the metrics");
  Object.values(H()).forEach(h => L.push(`- ${h.n}: ${h.d}. Tells you: ${h.c}. Doesn't tell you: ${h.x}`));
  return L.join("\n");
}
/* 期間 [ws, we) に利用上限に当たった時刻と、そのセッション */
function limitHits(ws, we){ const H = []; DATA.forEach(s => (s.limits || []).forEach(t => { if (t >= ws && t < we) H.push({t, s}); })); return H.sort((a,b) => a.t - b.t); }
/* 見直す候補の判定に使う部品（期間 [ws, we) を渡す。推移の計算でも使う） */
const inP = (s, ws, we) => s.segs.some(([a,b]) => b > ws && a < we);
function idleOf(ws, we){ // 目安コストが大きいのにコミットも PR もないセッション（アウトプットを記録できる Claude Code のみ）
  const xs = DATA.filter(s => s.cost >= 1 && /claude/i.test(s.source) && !(s.outputs && (s.outputs.commits || s.outputs.prs)) && inP(s, ws, we)).sort((a,b) => b.cost - a.cost);
  return {xs, c: xs.reduce((t,s) => t + s.cost, 0)}; }
function longCtxOf(ws, we){ // 会話が長くなり、1 回の応答で読む入力が大きく増えたセッション
  return DATA.filter(s => inP(s, ws, we) && s.ctx && s.ctx.length === 3 && s.ctx[0] > 0 && s.ctx[1] >= s.ctx[0] * 4 && s.ctx[2] >= 1e5 && s.cost >= 0.5).sort((a,b) => b.cost - a.cost); }
const mainModel = s => ((s.models || []).slice().sort((a,b) => b[1] - a[1])[0] || [""])[0];
function lightOf(ws, we){ // 編集のない短いセッションで、高いモデル（Opus 系）を使ったもの
  const xs = DATA.filter(s => inP(s, ws, we) && /opus/i.test(mainModel(s)) && s.nPrompts <= 3 && !s.nFiles && s.cost >= 0.3).sort((a,b) => b.cost - a.cost);
  return {xs, c: xs.reduce((t,s) => t + s.cost, 0)}; }
/* 繰り返したプロンプト：期間中のプロンプトを、文字の 3 文字組の重なり（Jaccard 0.6 以上）でまとめ、3 つ以上のセッションで書いたものを拾う（AI は使わない） */
const REPEAT_MIN = 12, REPEAT_SES = 3, repeatCache = {};
function repeatsOf(ws, we){
  const ck = `${ws}:${we}:${DATA.length}`; if (repeatCache[ck]) return repeatCache[ck];
  const norm = t => String(t || "").toLowerCase().replace(/[\s　]+/g, " ").replace(/[「」『』"'`。、.,!?！？:：;；()（）\[\]]/g, "").trim();
  const C = [], idx = new Map(); // 3 文字組 → それを含む束の番号（束の代表は最初に見つけたプロンプト）
  DATA.forEach(s => s.prompts.forEach(p => {
    if (!(p.t >= ws && p.t < we)) return;
    const t = norm(p.text).slice(0, 200); if (t.length < REPEAT_MIN) return; // 「はい」「続けて」のような短い返事は、書き置いても意味がない
    const g = new Set(); for (let i = 0; i + 3 <= t.length; i++) g.add(t.slice(i, i + 3));
    const cnt = new Map(); g.forEach(x => (idx.get(x) || []).forEach(i => cnt.set(i, (cnt.get(i) || 0) + 1)));
    let best = -1, bs = 0.6; cnt.forEach((n, i) => { const j = n / (C[i].g.size + g.size - n); if (j >= bs){ bs = j; best = i; } });
    if (best < 0){ best = C.length; C.push({g, n: 0, ids: new Set(), last: -1}); g.forEach(x => { if (!idx.has(x)) idx.set(x, []); idx.get(x).push(best); }); }
    const c = C[best]; c.n++; c.ids.add(s.id); if (p.t >= c.last){ c.last = p.t; c.id = s.id; c.text = p.text; } // 見せるのは、いちばん新しい書き方
  }));
  return repeatCache[ck] = C.filter(c => c.ids.size >= REPEAT_SES).sort((a,b) => b.ids.size - a.ids.size || b.n - a.n);
}
const snipOf = (t, n) => { t = String(t || "").replace(/\s+/g, " ").trim(); return t.length > n ? t.slice(0, n) + "…" : t; };
/* 長すぎるプロンプト：期間中に、BIG_PROMPT 文字以上のプロンプト（ログや資料をそのまま貼ったものなど） */
const BIG_PROMPT = 4000, plen = p => p.len || String(p.text || "").length;
function bigOf(ws, we){ const ps = [], ids = new Set();
  DATA.forEach(s => s.prompts.forEach(p => { if (p.t >= ws && p.t < we && plen(p) >= BIG_PROMPT){ ps.push(plen(p)); ids.add(s.id); } }));
  return {n: ps.length, max: Math.max(0, ...ps), ids: [...ids]}; }
/* 指標の値（見直す候補の推移用）。low: 大きいときに印が付く向き（良し悪しの判定ではない） */
const pctOf = (a, b) => b ? Math.round(a * 100 / b) : null;
const MET = {
  limits: {f: (w, ws, we) => limitHits(ws, we).length, low: true, u: ""},
  longctx: {f: (w, ws, we) => longCtxOf(ws, we).length, low: true, u: ""},
  heavy: {f: (w, ws, we) => w.usage && w.usage.cost >= 0.01 ? pctOf(idleOf(ws, we).c, w.usage.cost) : null, low: true, u: "%"},
  cost: {f: w => w.usage ? w.usage.cost : null, low: true, fmt: v => usd(v)},
  costPerCommit: {f: w => w.costPerCommit, low: true, fmt: v => usd(v)},
  modelfit: {f: (w, ws, we) => w.usage && w.usage.cost >= 0.01 ? pctOf(lightOf(ws, we).c, w.usage.cost) : null, low: true, u: "%"},
  fix: {f: w => w.fixRate, low: true, u: "%"},
  friction: {f: w => w.friction.length, low: true, u: ""},
  bigPrompts: {f: (w, ws, we) => bigOf(ws, we).n, low: true, u: ""},
  cache: {f: w => w.usage && w.usage.cacheHit != null ? Math.round(w.usage.cacheHit * 100) : null, low: false, u: "%"},
  outSessions: {f: w => pctOf(w.outSessions, w.outBase ?? w.sessions), low: false, u: "%"},
  switches: {f: w => w.switchesAvg, low: true, u: ""},
  focus: {f: w => w.focus.length, low: false, u: ""},
  wait: {f: w => w.waitP90 != null ? Math.round(w.waitP90 / 60) : null, low: true, u: " min"},
};
const fmtM = (k, v) => v == null ? "—" : MET[k].fmt ? MET[k].fmt(v) : `${Math.round(v * 10) / 10}${MET[k].u ?? ""}`; // 本文と同じく「8 件」「5.2 回」
function periodBack(i){ // 表示中の期間から i 個前の期間
  if (st.mode === "month"){ const a = new Date(st.month.getFullYear(), st.month.getMonth() - i, 1), b = new Date(a.getFullYear(), a.getMonth() + 1, 1);
    return {S: MONTHS[mkey(a)], ws: a.getTime()/1000, we: b.getTime()/1000, label: periodLabel("month", mkey(a)), key: mkey(a)}; }
  const a = addDays(st.week, -7 * i);
  return {S: WEEKS[key(a)], ws: a.getTime()/1000, we: addDays(a, 7).getTime()/1000, label: periodLabel("week", key(a)), key: key(a)};
}
function periodLabel(mode, k){ const [y, m, d] = k.split("-").map(Number); // 期間の名前（"2026-05" か "2026-05-04"）
  return mode === "month" ? `${y}/${m}` : `week of ${m}/${d}`; }
function metricAt(k, p){ if (!MET[k] || !p.S) return null; const v = MET[k].f(p.S, p.ws, p.we); return v == null || Number.isNaN(v) ? null : v; }
function spark(k){ // 8 期間の推移（記録のない期間は飛ばす）
  if (!MET[k]) return "";
  const pts = []; for (let i = 7; i >= 0; i--){ const p = periodBack(i); pts.push({v: metricAt(k, p), l: p.label}); }
  const vs = pts.filter(p => p.v != null).map(p => p.v); if (vs.length < 2) return "";
  const lo = Math.min(...vs), hi = Math.max(...vs), rng = lo === hi ? fmtM(k, lo) : `${fmtM(k, lo)}–${fmtM(k, hi)}`, W = 112, H = 26, x = i => 4 + i * (W - 8) / 7, y = v => hi === lo ? H / 2 : H - 4 - (v - lo) / (hi - lo) * (H - 8);
  const line = pts.map((p, i) => p.v == null ? null : `${x(i).toFixed(1)},${y(p.v).toFixed(1)}`).filter(Boolean).join(" ");
  const txt = `${st.mode === "month" ? "8-month" : "8-week"} trend: ${pts.map(p => `${p.l} ${fmtM(k, p.v)}`).join(", ")}`;
  const hits = pts.map((p, i) => `<rect class="hit" x="${(x(i) - (W - 8) / 14).toFixed(1)}" y="0" width="${((W - 8) / 7).toFixed(1)}" height="${H}"${tipAttr(p.l, fmtM(k, p.v) === "—" ? "No records" : fmtM(k, p.v))}/>`).join(""); // 点ごとに、その期間の値を出す
  return `<span class="spark"><span class="sr">${esc(txt)}</span><svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" aria-hidden="true"><polyline points="${line}"/>${pts.map((p, i) => p.v == null ? "" : `<circle class="pt" cx="${x(i)}" cy="${y(p.v)}" r="1.6"/>`).join("")}${pts[7].v != null ? `<circle cx="${x(7)}" cy="${y(pts[7].v)}" r="2.6"/>` : ""}${hits}</svg><small aria-hidden="true"><b>${fmtM(k, pts[7].v)}</b>${` (${rng} over ${st.mode === "month" ? "8 months" : "8 weeks"} · flagged when ${MET[k].low ? "high" : "low"})`}</small></span>`;
}
/* 見直す候補：指標が決まった基準を超えたものを拾う（AI は使わない。判定ではなく、確かめる候補） */
function findList(w, pw, unit){
  const F = [], u = w.usage || {}, o = w.outputs || {}, {ws, we} = period();
  const add = (k, score, see, why, rule, ids) => F.push({k, score, see, why, rule, ids: ids || []});
  const LH = limitHits(ws, we);
  if (LH.length)
    add("limits", 40, `Hit the usage limit ${LH.length === 1 ? "once" : LH.length + " times"} (${LH.slice(-3).map(h => `${md(h.t)} ${hm(h.t)}`).join(", ")}${LH.length > 3 ? ", …" : ""})`, "Hitting a limit stops your work until it resets. The cause is often in how you worked just before", "1 or more", [...new Set(LH.map(h => h.s.id))].reverse());
  if (w.fixRate != null && w.prompts >= 10 && w.fixRate >= 20)
    add("fix", w.fixRate, `${w.fixRate}% of prompts had corrections or interruptions`, "When a first prompt misses, redoing it costs time and tokens", "20% or more, with 10+ prompts", w.friction.map(f => f.id));
  else if (w.friction.length)
    add("friction", 18, `${plural(w.friction.length, "session")} with possible friction`, "Rework is concentrated in a few sessions. Opening them usually hints at the cause", "Many corrections, interruptions or 15+ prompts", w.friction.map(f => f.id));
  const LC = longCtxOf(ws, we);
  if (LC.length)
    add("longctx", 34, `${plural(LC.length, "session")} where the conversation grew long and the input read per response rose to ${Math.round(Math.max(...LC.map(s => s.ctx[1] / s.ctx[0])))}× the first part (peak ${tok(Math.max(...LC.map(s => s.ctx[2])))} tokens)`, "The longer a conversation, the more earlier context each response rereads, so similar prompts get heavier", "Later input 4×+ the first part, peak 100K+ tokens, $0.5+", LC.map(s => s.id));
  const LT = lightOf(ws, we);
  if (u.cost >= 2 && LT.c >= Math.max(1, u.cost * 0.1))
    add("modelfit", 24, `Short sessions with no edits spent ${usd(LT.c)} (${Math.round(LT.c*100/u.cost)}% of estimated cost) on Opus-class models`, "For research and questions, a lighter model is often enough", "Sessions with ≤3 prompts, no edits and $0.3+ total 10%+ of all cost ($2+) and $1+", LT.xs.map(s => s.id));
  const {xs: idle, c: idleC} = idleOf(ws, we);
  if (u.cost >= 2 && idleC >= u.cost * 0.4)
    add("heavy", 30 + Math.round(idleC*50/u.cost), `${Math.round(idleC*100/u.cost)}% of estimated cost (${usd(idleC)}) went to sessions with no commit or pull request`, "Fine for research or discussion. If they stopped partway, it's worth finding out why", "Sessions of $1+ total 40%+ of all cost ($2+)", idle.map(s => s.id));
  const V = vsPrev(pw, unit), pc = V.of("cost", pw && pw.usage && pw.usage.cost), pl = V.n == null ? uLast(unit) : V.range; // 途中の期間は、前の期間の同じ日までと比べる
  if (pc != null && pc >= 1 && u.cost >= pc * 1.5)
    add("cost", 25, `Estimated cost was ${(u.cost/pc).toFixed(1)}× ${pl} (${usd(pc)} → ${usd(u.cost)})`, "What it means depends on which projects and sessions the increase came from", `1.5× ${uLast(unit)} or more`);
  if (pw && pw.costPerCommit != null && w.costPerCommit != null && o.commits >= 3 && w.costPerCommit >= pw.costPerCommit * 1.5)
    add("costPerCommit", 22, `Estimated cost per commit rose from ${usd(pw.costPerCommit)} to ${usd(w.costPerCommit)}`, "Reaching a checkpoint is getting heavier than before", `1.5× ${uLast(unit)} or more, with 3+ commits`);
  if (u.cacheHit != null && u.tokens >= 1e6 && u.cacheHit < 0.5)
    add("cache", 20, `Only ${Math.round(u.cacheHit*100)}% of input was read from cache`, "You may be re-sending the same context every time", "Under 50%, with 1M+ tokens");
  if ((w.outBase ?? w.sessions) >= 5 && (o.commits || o.prs || (w.git && w.git.commits)) && w.outSessions / (w.outBase ?? w.sessions) < 0.25)
    add("outSessions", 16, `${w.outSessions} of ${w.outBase ?? w.sessions} sessions reached a commit`, "Many sessions may have stopped partway", "Under 25%, with 5+ sessions");
  const BG = bigOf(ws, we);
  if (BG.n >= 3)
    add("bigPrompts", 11, `${plural(BG.n, "prompt")} of ${BIG_PROMPT.toLocaleString()}+ characters (longest ${BG.max.toLocaleString()})`, "Pasting long logs or documents makes every later response re-read a heavier input, and buries the instructions that matter", `3+ prompts of ${BIG_PROMPT.toLocaleString()}+ characters`, BG.ids.slice(0, 6));
  const RP = repeatsOf(ws, we);
  if (RP.length)
    add("repeats", 9, `You wrote a similar prompt ${RP[0].n} times across ${RP[0].ids.size} sessions ("${snipOf(RP[0].text, 40)}")`, "A prompt you type every time can be written once as a command or in CLAUDE.md", `${REPEAT_MIN}+ characters, in ${REPEAT_SES}+ sessions`, [...new Set([RP[0].id, ...RP[0].ids])].slice(0, 6));
  if (w.switchesAvg >= 5)
    add("switches", 14, `You switched projects ${w.switchesAvg} times a day on average`, "Each context switch tends to add ramp-up time and rework", "5+ per day on average");
  if (!w.focus.length && w.active >= 240)
    add("focus", 12, `You worked ${dur(w.active)}, but never for 60 minutes straight`, "Fragmented time makes it harder to hand large tasks to AI", "No focus blocks, with 4+ hours of work");
  if (w.waitP90 != null && w.waitCount >= 10 && w.waitP90 >= 900)
    add("wait", 10, `At the long end, ${secs(w.waitP90)} passed between an AI reply and your next prompt`, "AI may have sat idle until you noticed its reply", "90th percentile of 15+ min");
  return F.sort((a,b) => b.score - a.score);
}
const GOTO = {longctx: "heavy", modelfit: "models"}; // 専用の指標がない候補は、関係する指標に印を付ける
function flagSum(F){ // サマリーの先頭に、基準を超えた指標の名前だけを優先度の高い順に並べる（押すとその指標へ）
  const ids = [...new Set(F.map(f => GOTO[f.k] || f.k))];
  return `<div class="flagsum"><span class="lbl"><i class="fdot"></i>Worth a look${hb("findings")}</span>${ids.length
    ? ids.map(id => `<button class="flink" data-goto="${id}">${esc(H()[id].n)}</button>`).join("")
    : `<span class="muted">No metric crossed a threshold</span>`}${hint("findings")}</div>`;
}
function placeFlags(R, F){ // 基準を超えた指標の、その場に印・見えたこと・推移・該当するセッション・基準を添える
  const ses = ids => ids.map(id => { const s = DATA.find(x => x.id === id); return s ? `<button class="fses" data-id="${esc(id)}" style="--c:${colorOf(keyOf(s))}"><i></i><span>${esc(s.title)}</span><small>${md(s.start)}</small></button>` : ""; }).join("");
  F.forEach(f => {
    const t = R.querySelector(`.panel .hb[data-help="${GOTO[f.k] || f.k}"]`); if (!t) return;
    const panel = t.closest(".panel"), shown = new Set([...panel.querySelectorAll(".card[data-id]")].map(c => c.dataset.id)); // すぐ下にカードで並ぶセッションは繰り返さない
    const ids = f.k === "friction" ? [] : f.ids.filter(id => !shown.has(id)), more = ids.length - 3;
    const own = !GOTO[f.k] && t.closest(".stat"); // 自分の数字の上に出す印は、数字の言い直し（見えたこと）を省く
    const html = `<div class="fl">${own ? "" : `<p class="see"><i class="fdot"></i>${f.see}</p>`}<p class="why">${esc(f.why)}</p>${spark(f.k)}${ids.length ? `<div class="fss">${ses(ids.slice(0, 3))}${more > 0 ? `<p class="more">${`${more} more`}</p>` : ""}</div>` : ""}<p class="rule">Threshold: ${esc(f.rule)}</p></div>`;
    const dt = t.closest("details"); // 閉じた折りたたみの中の印は、見出しにも出し、自分で閉じていなければ開いておく
    if (dt){ const sm = dt.querySelector("summary"); if (!sm.querySelector(".fdot")) sm.insertAdjacentHTML("beforeend", `<i class="fdot" title="A metric crossed a threshold"></i>`); if (st.moreS !== false && !dt.open){ dt.dataset.auto = "1"; dt.open = true; } }
    const stat = t.closest(".stat");
    if (stat){ stat.classList.add("flagged"); stat.insertAdjacentHTML("beforeend", html); return; }
    const a = t.closest("h3, .ph, .cap, div"), next = a.nextElementSibling;
    (next && next.classList.contains("hint") ? next : a).insertAdjacentHTML("afterend", html);
  });
}
/* 全期間の検索結果：どこに一致したかを抜き出して並べる。押すとその週を開いて詳細を出す */
function snip(t, q){
  t = String(t || "").replace(/\s+/g, " "); const i = t.toLowerCase().indexOf(q); if (i < 0) return null;
  const a = Math.max(0, i - 36), b = Math.min(t.length, i + q.length + 64);
  return `${a ? "…" : ""}${esc(t.slice(a, i))}<mark>${esc(t.slice(i, i + q.length))}</mark>${esc(t.slice(i + q.length, b))}${b < t.length ? "…" : ""}`;
}
function hitOf(s, q){
  const tries = [["Title", [s.title]], ["Prompt", s.prompts.map(p => p.text)], ["File", s.files], ["PR", s.prs || []],
    ["Commit", commitsOf(s).flatMap(c => [`${c.hash.slice(0,7)} ${c.subject}`, ...(c.files || []).map(f => f.path)])],
    ["Project", [s.project]], ["Branch", [s.branch || ""]], ["Agent", [s.source]]];
  for (const [label, xs] of tries) for (const x of xs){ const h = snip(x, q); if (h) return {label, h}; }
  return {label: "", h: ""};
}
function searchPanel(){
  const R = $("#review"), q = st.q, LIMIT = 100;
  const ss = DATA.filter(s => !st.hidden.has(keyOf(s)) && searchText(s).includes(q)).sort((a,b) => b.start - a.start);
  const cs = (META.git || []).filter(c => [c.hash, c.subject, c.body || "", ...(c.files || []).map(f => f.path)].join("\n").toLowerCase().includes(q)).sort((a,b) => b.t - a.t);
  const row = s => { const h = hitOf(s, q);
    return `<button class="srow" data-s="${esc(s.id)}" style="--c:${colorOf(keyOf(s))}"><time>${md(s.start)}<small>${hm(s.start)}</small></time>
      <span class="b"><span class="ti"><i></i>${esc(s.title)}</span><span class="me">${esc(s.project)}${s.branch ? ` · ${esc(s.branch)}` : ""} · ${esc((s.source))}</span>
      ${h.h ? `<span class="hit"><em>${h.label}</em>${h.h}</span>` : ""}</span></button>`; };
  const crow = c => `<button class="srow" data-c="${esc(c.hash)}" style="--c:${colorOf(c.project)}"><time>${md(c.t)}<small>${hm(c.t)}</small></time>
      <span class="b"><span class="ti"><i></i>${snip(c.subject, q) || esc(c.subject)}</span><span class="me">${esc(c.project)}${c.branch ? ` · ${esc(c.branch)}` : ""} · <span class="mono">${esc(c.hash.slice(0,7))}</span> · ${plural(c.nFiles, "file")} +${c.added} −${c.removed}</span>
      ${(() => { const f = (c.files || []).find(f => f.path.toLowerCase().includes(q)); return f ? `<span class="hit"><em>File</em>${snip(f.path, q)}</span>` : ""; })()}</span></button>`;
  R.innerHTML = `<div class="rvhead"><h2>Search results</h2><p>${`Matches for "${esc(q)}" (all time). Sessions are matched on prompts, project, branch, agent, files changed by AI, pull requests and commits made during the session; commits on subject, body, hash and changed files. Click one to open its week.`}</p>
      <button class="pill" id="sclear">Clear search</button></div>
    <div class="rvgrid srgrid">
      <section class="panel"><div class="ph"><b>Sessions · ${ss.length}</b></div>
        ${ss.length ? ss.slice(0, LIMIT).map(row).join("") + (ss.length > LIMIT ? `<p class="more">${`${ss.length - LIMIT} more. Add words to narrow down.`}</p>` : "") : `<p class="none">No matching sessions.</p>`}</section>
      <section class="panel"><div class="ph"><b>Commits · ${cs.length}</b></div>
        ${cs.length ? cs.slice(0, LIMIT).map(crow).join("") + (cs.length > LIMIT ? `<p class="more">${`${cs.length - LIMIT} more`}</p>` : "") : `<p class="none">${(META.git || []).length ? "No matching commits." : "No git commits were loaded."}</p>`}</section>
    </div>`;
  const open = (t, id) => { st.mode = "week"; store.set("mode", "week"); st.week = mondayOf(new Date(t*1000)); select(id); };
  R.querySelectorAll("[data-s]").forEach(b => b.onclick = () => { const s = DATA.find(x => x.id === b.dataset.s); if (s) open(s.start, s.id); });
  R.querySelectorAll("[data-c]").forEach(b => b.onclick = () => { const c = META.git.find(x => x.hash === b.dataset.c); if (c) open(c.t, "git:" + c.hash); });
  $("#sclear").onclick = () => { $("#q").value = ""; st.q = ""; render(); };
}
/* 週報・月報の下書き：プロジェクトごとに、やったこと（セッション）・コミット・PR を Markdown で並べる（AI は使わない） */
function reportText(w, M){
  const {ws, we} = period(), unit = M ? "月" : "週", L = [];
  const start = M ? st.month : st.week, last = M ? new Date(st.month.getFullYear(), st.month.getMonth()+1, 0) : addDays(st.week, 6);
  const ymd = d => `${d.getFullYear()}/${d.getMonth()+1}/${d.getDate()}`;
  const ses = DATA.filter(s => s.segs.some(([a,b]) => b > ws && a < we) && !st.hidden.has(keyOf(s))).sort((a,b) => a.start - b.start);
  const gits = (META.git || []).filter(c => c.t >= ws && c.t < we).sort((a,b) => a.t - b.t);
  const g = w.git || {};
  L.push(`## Work for ${ymd(start)}–${ymd(last)} (${M ? "monthly" : "weekly"} report draft)`, "",
    `- Active time ${dur(w.active)} · sessions ${w.sessions} · prompts ${w.prompts}${g.commits ? ` · commits ${g.commits} (${g.ai} by AI)` : ""}${w.outputs && w.outputs.prs ? ` · pull requests ${w.outputs.prs}` : ""}`);
  const projs = [...new Set([...(w.projects || []).map(([k]) => k), ...gits.map(c => c.project)])];
  projs.forEach(pj => {
    const ps = ses.filter(s => s.project === pj), pc = gits.filter(c => c.project === pj), prs = [...new Set(ps.flatMap(s => s.prs || []))];
    if (!ps.length && !pc.length) return;
    const min = ((w.projects || []).find(([k]) => k === pj) || [0, 0])[1];
    L.push("", `### ${pj}${min ? ` (${dur(min)})` : ""}`);
    if (ps.length){ L.push("", "What I did:");
      const seen = new Map(); ps.forEach(s => { const t = s.title.replace(/\s+/g, " ").trim(); const x = seen.get(t); x ? x.n++ : seen.set(t, {s, n: 1}); });
      [...seen.values()].forEach(({s, n}) => L.push(`- ${s.title.replace(/\s+/g, " ").trim()} (${md(s.start)}${n > 1 ? ` and ${n-1} more` : ""}, ${(s.source)})`)); }
    if (pc.length){ L.push("", "Commits:"); pc.slice(-15).forEach(c => L.push(`- ${c.url ? `[${c.hash.slice(0,7)}](${c.url})` : c.hash.slice(0,7)} ${c.subject}`)); if (pc.length > 15) L.push(`- ${pc.length - 15} more`); }
    if (prs.length){ L.push("", "Pull requests:"); prs.forEach(u => L.push(`- ${u}`)); }
  });
  L.push("", `<!-- Generated by kiroku. Session names are the start of your prompts. Review and edit this ${M ? "month" : "week"}'s content before sharing -->`);

  return L.join("\n");
}
/* アウトプット：AI が実行したコミット・PR 作成・変更した行（出したものの量。価値や生産性ではない） */
function outcomePanel(w, pw, unit, ph, stat){ // 使ったもの（コスト）→ 残ったもの（コミットと、使ったものと比べた指標）を左右に並べる
  const o = w.outputs || {commits:0}, g = w.git, u = w.usage || {};
  const hasOut = o.commits || (g && g.commits);
  const po = pw && pw.outputs, V = vsPrev(pw, unit), d = (a, b) => V.diff(a, V.of(null, b)); // AI のコミットと PR は日ごとの値がないので、途中の期間は比べない
  const n = v => v.toLocaleString(LOC()), times = v => `${v}`, base = w.outBase ?? w.sessions;
  const cost = [
    stat("Active time", dur(w.active,true), V.diff(w.active, V.of("active", pw && pw.active), dur), "active"),
    u.tokens ? stat("Estimated cost", usd(u.cost).replace("$","<small>$</small>"), V.diff(u.cost, V.of("cost", pw && pw.usage && pw.usage.cost), usd), "cost") : "",
    u.tokens ? stat("Tokens", tok(u.tokens), `Output ${tok(u.out)}`, "tokens") : "",
    u.credits ? stat("Kiro credits", crN(u.credits), "As recorded in history", "credits") : "",
  ].join("");
  const out = hasOut ? [
    g && g.commits ? stat("Git commits", times(g.commits), `${g.ai} by AI · +${n(g.added)} −${n(g.removed)} lines${pw && pw.git ? ` · ${V.diff(g.commits, V.of("commits", pw.git.commits))}` : ""}`, "gitCommits") : "",
    g && g.commits ? "" : stat("AI commits", times(o.commits), d(o.commits, po && po.commits), "commits"), // Git のコミットがあれば「うち AI」に出ている
    // 使ったものと比べた指標。何と何を割ったかを添える
    w.costPerCommit != null ? stat("Estimated cost per commit", usd(w.costPerCommit).replace("$","<small>$</small>"), `Estimated cost ${usd(u.cost)} ÷ ${plural(o.commits, "AI commit")}`, "costPerCommit") : "",
    stat("Sessions that reached a commit", base ? `${Math.round(w.outSessions*100/base)}<small>%</small>` : "—", `${w.outSessions} of ${plural(base, "session")}`, "outSessions"),
  ].join("") : "";
  const side = (cls, label, sub, body) => `<div class="ocside ${cls}"><div class="ocl"><b>${label}</b><span>${sub}</span></div>${body}</div>`;
  return `<section class="panel oc">${ph(2, "Cost and outputs", `What you spent ${uThis(unit)} and what came out of it`, "outputs")}
    <div class="ocgrid">
      ${side("spent", "Cost", "Time and usage", `<div class="stats">${cost}</div>`)}
      <div class="ocarrow" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M5 12h14M13 6l6 6-6 6"/></svg></div>
      ${side("left", "Outputs", "Commits and cost per commit", out ? `<div class="stats">${out}</div>` : `<p class="none">No commits recorded.</p>`)}
    </div>
    <div class="ocdaily">${usageChart(w, unit === "月")}</div></section>`;
}
/* 日ごとの推移（トークン・クレジット・目安コスト・作業時間・セッション・プロンプトを切り替え。今までのリズムもここにまとめた） */
function usageChart(w, M){ // 日ごとの推移：トークン・クレジット・目安コスト・作業時間・セッション・プロンプトを切り替えて 1 本の棒グラフで見る
  const start = M ? st.month : st.week, today = key(today0());
  const days = w.days.map((d, i) => { const dd = addDays(start, i), ds = dd.getTime()/1000;
    return {...d, dd, sessions: DATA.filter(s => inP(s, ds, ds + 86400)).length}; });
  const hrs = a => a >= 60 ? (a/60).toFixed(1)+"h" : a+"m", n = v => String(v);
  const all = [["tokens", "Tokens", tok], ["credits", "Credits", cr], ["cost", "Estimated cost", usd],
    ["active", "Active time", hrs], ["sessions", "Sessions", n], ["prompts", "Prompts", n]];
  const opts = all.filter(([k]) => days.some(d => k === "cost" ? d.cost >= 0.005 : d[k]));
  if (!opts.length) return "";
  const [m, , fmt] = opts.find(([k]) => k === st.use) || opts[0];
  const max = Math.max(...days.map(d => d[m] || 0)) || 1, total = days.reduce((t,d) => t + (d[m]||0), 0);
  const cols = days.map(d => { const v = d[m] || 0, dd = d.dd;
    const tip = [`Active ${dur(d.active)}`, `Sessions ${d.sessions}`, `Prompts ${d.prompts}`, ...useLines(d)];
    return `<div class="c${key(dd)===today?" today":""}${wkc(dd.getDay())}"${tipAttr(md(dd.getTime()/1000), d.active || d.tokens || d.credits ? tip : "No records")}>
      ${M ? "" : `<span class="v">${v ? fmt(v) : ""}</span>`}<div class="b" style="height:${v ? Math.max(2, v/max*112) : 0}px"></div>
      <span class="l">${M ? (dd.getDate() === 1 || dd.getDate() % 5 === 0 ? dd.getDate() : "") : dow(dd.getDay())}</span></div>`; }).join("");
  return `<h3>Daily trend${hb("daily")}</h3>${hint("daily")}
    <div class="useg"><div class="segc" role="group" aria-label="Show" id="useBy">${opts.map(([k,l]) => `<button data-v="${k}" aria-pressed="${k===m}">${l}</button>`).join("")}</div>
      <span class="muted" style="font-size:var(--fs-xs)">${`Total ${fmt(total)} · max ${fmt(max)}/day`}</span></div>
    <div class="ubar" style="grid-template-columns:repeat(${days.length},minmax(0,1fr))${M ? ";gap:2px" : ""}">${cols}</div>`;
}
/* 計測の状態 */
/* 履歴を自動で消すエージェント（既定のままの Claude Code など）を知らせ、公式ドキュメントへ案内する。閉じたら出さない */
function keepNotice(){
  if (store.get("keepNoticeOff", false) || archOn()) return ""; // kiroku がコピーを残していれば、消えても見られる
  const r = (META.report || []).find(r => r.n && r.retention && r.retention.days && !r.retention.set); if (!r) return "";
  const k = r.retention, snippet = `"${k.setting}": 3650`, cmd = "kiroku archive on";
  return `<div class="keep" role="note"><b>Your older history will be deleted</b>
    <p>${`${esc((r.name))} automatically deletes conversation history older than ${k.days} days (<code>${esc(k.setting)}</code> is at its default). Deleted history cannot be shown by kiroku and cannot be recovered. To keep it, set a long period such as <code>${esc(snippet)}</code> in your settings file (<code>~/.claude/settings.json</code>).`}</p>
    <p>${`If you'd rather not change the setting, kiroku can keep a copy of the history instead. It saves a compressed copy each time you open kiroku and shows deleted conversations from it (copies stay on this computer only).${LIVE ? "" : ` To turn it on, run <code>${cmd}</code>.`}`}</p>
    <div class="ka">${ext(k.docs, "See how to set it in the official docs ↗", "pill")}<button class="pill" data-copy="${esc(snippet)}">Copy setting</button>${LIVE ? `<button class="pill" id="keeparch">Keep a copy in kiroku</button>` : `<button class="pill" data-copy="${cmd}">Copy command</button>`}<button class="pill" id="keepoff">Dismiss</button></div></div>`;
}
function archOn(){ return !!(META.archive && META.archive.on); }
function bytes(n){ return n >= 1<<30 ? (n/(1<<30)).toFixed(1)+" GB" : n >= 1<<20 ? (n/(1<<20)).toFixed(1)+" MB" : n >= 1<<10 ? (n/(1<<10)).toFixed(1)+" KB" : n+" B"; }
// keepArchive は「kiroku にコピーを残す」（kiroku serve のときだけ）。kiroku archive on と同じことをして、集計を取り込み直す。
async function keepArchive(){
  try {
    const r = await fetch("archive", {method:"POST", headers:{"X-Kiroku":"1"}, cache:"no-store"}); if (!r.ok) throw new Error(r.status);
    applyData(await r.json()); render();
    toast("Kept a copy. kiroku will save one each time you open it", 4500);
  } catch(e){ toast("Couldn't keep a copy. Run kiroku archive on instead", 4500); }
}
function applyData(j){ DATA = j.sessions || []; WEEKS = j.weeks || {}; MONTHS = j.months || {}; META = j.meta; GENERATED = j.generated; if (st.sel && !DATA.some(s => s.id === st.sel)) st.sel = null; }
function keepRow(r){ // 計測の状態に添える：どこまでさかのぼれるか、いつ消えるか
  const d = r.oldest ? new Date(r.oldest*1000) : null, o = d ? `${d.getFullYear()}/${d.getMonth()+1}/${d.getDate()}` : "";
  const k = r.retention, link = k && k.docs ? ` ${ext(k.docs, "official docs ↗")}` : "";
  return (o ? ` · oldest record ${o}` : "") +
    (r.archived ? ` · ${plural(r.archived, "deleted conversation")} shown from kiroku's copy` : "") +
    (!k ? "" : k.days && !k.set && archOn() ? `<br>${`Records older than ${k.days} days are deleted automatically, but kiroku keeps a copy`}`
      : k.days && !k.set ? `<br><span class="kw">${`Records older than ${k.days} days are deleted automatically (<code>${esc(k.setting)}</code> is at its default)`}${link}</span>`
      : k.days ? `<br>${`Kept for ${k.days} days (<code>${esc(k.setting)}</code>)`}`
      : `<br>${`Older records are deleted after a period (<code>${esc(k.setting)}</code>)`}${link}`); }
function measure(w){
  const rows = META.report.map(r => { const dt = r.detailEn || r.detail;
    return `<li class="${r.error?"warn":""}">${esc((r.name))}: ${plural(r.n, "session")}${dt?` (${esc(dt)})`:""}${r.dup?` (${plural(r.dup, "duplicate conversation")} found elsewhere not counted)`:""}${r.error?` (some files couldn't be read)`:""}${keepRow(r)}</li>`; });
  const a = META.archive; // kiroku archive
  if (a && (a.on || a.files)) rows.push(`<li>${a.on ? `kiroku's copy: on (${plural(a.files, "file")}, ${bytes(a.bytes)})`
      : `kiroku's copy: off. Copies kept so far (${plural(a.files, "file")}, ${bytes(a.bytes)}) are still shown`}<br><code>${esc(a.dir)}</code></li>`);
  if (w.usage && w.usage.unpriced) { const ms = (w.usage.unpricedModels || []).map(m => `<code>${esc((m))}</code>`).join(", ");
    rows.push(`<li class="warn">${`${tok(w.usage.unpriced)} tokens from models not in the price table are not included in the estimated cost${ms ? ` (${ms})` : ""}. Add their prices with <code>--prices</code>`}</li>`); }
  if (w.usage && w.usage.tokens) rows.push(`<li>${`Price table for estimated cost: ${META.prices && META.prices.custom ? "from <code>--prices</code>" : (META.prices ? `public rates as of ${esc(META.prices.asOf)}` : "public rates") + " (change it with <code>--prices</code>)"}`}</li>`);
  return `<h3>Data sources</h3><ul class="mlist">${rows.join("")}</ul>`;
}
// soFar は、今見ている期間が途中なら、始まりから今日までの日数（今日を含む）。終わった期間や先の期間は null。
function soFar(){ const {ws, we} = period(), now = nowMs()/1000;
  return ws <= now && now < we ? Math.min(Math.ceil((now - ws) / 86400), Math.round((we - ws) / 86400)) : null; }
// vsPrev は、前の期間との比べ方。途中の期間は、前の期間の同じ日まで（先頭から今日と同じ日数）と比べる。
// 途中の値を前の期間まるごとと比べると、いつも少なく見えるため。
//   label: 差に添える言葉、range: 比べた前の期間の日付（途中のときだけ）
//   of(f, whole): 比べる前の期間の値。途中なら日ごとの集計 f の合計（日ごとの値がない指標は null）、終わった期間なら whole
//   diff(a, b, fmt): 「<label> +差」。b が null なら空
function vsPrev(pw, unit){
  const n0 = pw ? soFar() : null, n = n0 == null ? null : Math.min(n0, pw.days.length), mdy = d => `${d.getMonth()+1}/${d.getDate()}`; // 先月が今月より短いときは、先月の最後の日まで
  const a = n == null ? null : new Date(periodBack(1).ws * 1000), range = n == null ? "" : n === 1 ? mdy(a) : `${mdy(a)}–${mdy(addDays(a, n - 1))}`;
  const label = n == null ? `vs ${uLast(unit)}` : `vs ${range}`;
  const of = (f, whole) => !pw || whole == null ? null : n == null ? whole : f ? pw.days.slice(0, n).reduce((t, d) => t + (d[f] || 0), 0) : null;
  const diff = (a, b, fmt = x => x) => b == null ? "" : `${label} <span class="nw">${a-b>=0?"+":"−"}${fmt(Math.abs(a-b))}</span>`;
  return {n, range, label, of, diff};
}
// projection は、今月の途中なら、今日までのペースが月末まで続いたときの目安コストとクレジット（推定）。
// 今日までの日数（今日を含む）で割り、月の日数を掛ける。月の初めは日数が少なく当てにならないので 7 日たつまで、
// 最後の日は実績とほとんど変わらないので出さない。
function projection(w){ const {ws, we} = period(), now = nowMs()/1000, u = w.usage;
  if (st.mode !== "month" || !u || !(ws <= now && now < we) || now - ws < 7 * 86400 || we - now < 86400) return null;
  const days = Math.ceil((now - ws) / 86400), k = Math.round((we - ws) / 86400) / days;
  return {cost: u.tokens ? u.cost * k : null, credits: u.credits ? u.credits * k : null, days}; }
function aiUsage(w, pw, unit){
  const u = w.usage; if (!u || (!u.tokens && !u.credits)) return "";
  const stat = (k, v, s, h) => `<div class="stat"><div class="k">${k}${hb(h)}</div><div class="v">${v}</div>${s?`<div class="s">${s}</div>`:""}${hint(h)}</div>`;
  const pj = projection(w);
  const totalC = u.models.reduce((t,r)=>t+r[1],0) || 1, totalT = u.models.reduce((t,r)=>t+r[2],0) || 1, byCost = totalC > 0.0001;
  return `<div class="stats" style="margin-top:4px">
      ${pj ? stat("Month-end projection (estimate)", [pj.cost != null ? "≈ " + usd(pj.cost).replace("$","<small>$</small>") : "", pj.credits != null ? (pj.cost != null ? `<small> · </small>` : "≈ ") + `${Math.round(pj.credits)}<small> credits</small>` : ""].join(""), `If the pace of the first ${pj.days} days continues`, "projection") : ""}
      ${u.tokens ? stat("Read from cache", u.cacheHit==null ? "—" : `${Math.round(u.cacheHit*100)}<small>%</small>`, "Share of input", "cache") : ""}
      ${stat("Subagents", `${u.subagents}`, u.subagents ? `Total ${dur(u.subMin)}` : "Not used", "subagents")}
      ${w.costPerAsk != null ? stat("Estimated cost per prompt", usd(w.costPerAsk).replace("$","<small>$</small>"), `n=${w.prompts}`, "costPerAsk") : ""}
    </div>
    ${u.models.length ? `<div style="margin-top:16px" class="k muted">By model${byCost ? " (estimated cost)" : " (tokens)"}${hb("models")}</div>${hint("models")}
      <div class="mstack" style="margin-top:8px">${u.models.map((r,i)=>`<span style="flex:${byCost?r[1]:r[2]};--o:${shade(i)}"${tipAttr((r[0]), `Estimated cost ${usd(r[1])}`, `Tokens ${tok(r[2])}`, `${Math.round((byCost?r[1]/totalC:r[2]/totalT)*100)}%`)}></span>`).join("")}</div>
      ${u.models.slice(0,6).map((r,i)=>`<div class="mrow"><span class="nm"><i style="--o:${shade(i)}"></i>${esc((r[0]))}</span><span class="tm">${!r[1] && (u.unpricedModels || []).includes(r[0]) ? `<span title="Not in the price table">—</span>` : usd(r[1])}<small>${tok(r[2])}</small></span><span class="pc">${Math.round((byCost?r[1]/totalC:r[2]/totalT)*100)}%</span></div>`).join("")}` : ""}
    ${u.subTypes.length ? `<div style="margin-top:14px" class="chips">${u.subTypes.map(([k,v])=>`<span class="mono">${esc(k)}<b>${v}</b></span>`).join("")}</div>` : ""}
    ${u.heavy.length ? `<div style="margin-top:16px" class="k muted">Heaviest sessions${hb("heavy")}</div>${hint("heavy")}<div style="margin-top:8px">${u.heavy.map(h=>`<button class="card" data-id="${esc(h.id)}"><span class="ti">${esc(h.title)}</span><span class="me">${md(h.start)} · ${esc(h.project)} · ${usd(h.cost)}${h.subagents?` · ${plural(h.subagents, "subagent")}`:""}</span></button>`).join("")}</div>` : ""}`;
}
function nativeText(v){
  const num = (x, d) => Number(x.toFixed(d)).toLocaleString(LOC());
  const u = {"回": "", "件": "", "トークン": " tokens", "文字": " chars"}[v.unit]; // 単位は Go の定義（日本語）を英語に読みかえる
  return v.unit === "%" ? `${num(v.v,1)}%` : v.unit === "秒" ? `${num(v.v,1)}s` : v.unit === "クレジット" ? `${crN(v.v)} credits` : `${num(v.v,0)}${u ?? " " + v.unit}`;
}
const nlabel = v => v.labelEn || v.label; // 参考指標の名前（Go の英語の名前）
function nativeRows(values){
  return values.map(v=>`<div class="nrow"><span>${esc(nlabel(v))}</span><span class="v">${nativeText(v)}</span><span class="n">n=${v.n}</span></div>`).join("");
}
function nativeSection(w){
  if (!w.native || !w.native.length) return "";
  return `<h3>Agent-specific metrics${hb("native")}</h3>${hint("native")}
    ${w.native.map(g=>`<div class="ngroup"><div class="hd"><b>${esc((g.source))}</b><span>${plural(g.sessions, "session")}</span></div>${nativeRows(g.values)}</div>`).join("")}`;
}
function foot(){
  return `<div class="foot"><div>Generated ${new Date(GENERATED*1000).toLocaleString(LOC())} · <button class="muted" id="openhelp" style="text-decoration:underline dotted">Keyboard shortcuts</button></div></div>`;
}

function bindCopy(root){ root.querySelectorAll("[data-copy]").forEach(b => b.onclick = () => copy(b.dataset.copy)); const h = root.querySelector("#openhelp"); if (h) h.onclick = () => $("#keys").showModal(); }

/* git のコミットの印（コミットのノード: 線の上の丸） */
const GIT_ICON = `<svg class="gi" viewBox="0 0 16 16" aria-hidden="true"><path d="M1 8h4.2M10.8 8H15"/><circle cx="8" cy="8" r="2.8"/></svg>`;
/* リンク：外のページ（GitHub など）は新しいタブで開く */
function ext(url, label, cls){ return url ? `<a class="xl${cls ? " "+cls : ""}" href="${esc(url)}" target="_blank" rel="noopener noreferrer">${label}</a>` : label; }
function fileHref(path){ // HTML で見るときの履歴ファイルの file:// の URL（Windows のパスにも対応）
  const p = path.replace(/\\/g, "/");
  return "file://" + (/^[A-Za-z]:/.test(p) ? "/" : "") + p.split("/").map(encodeURIComponent).join("/").replace(/%3A/g, ":");
}
/* ── drawer: git のコミット ── */
function commitsOf(s){ // セッションが作ったコミットと、同じプロジェクトでセッションの間（終わってから 10 分まで）のコミット
  return (META.git || []).filter(c => c.session === s.id || (!c.session && c.project === s.project && c.t >= s.start - 60 && c.t <= s.end + 600));
}
function fileRows(files, n){
  const num = v => v < 0 ? "bin" : v;
  return `<ul class="gfiles">${files.map(f => `<li title="${esc(f.path)}"><span>${ext(f.url, esc(f.path))}</span><b><i>+${num(f.added)}</i> −${num(f.removed)}</b></li>`).join("")}</ul>${n > files.length ? `<p class="more">${`${plural(n - files.length, "more file")}`}</p>` : ""}`;
}
function fileLink(s, abs){ // セッションの間のコミットのうち、最後にそのファイルを変えたコミットでのリンク
  const p = abs.replace(/\\/g, "/");
  let url = "";
  commitsOf(s).forEach(c => (c.files || []).forEach(f => { if (f.url && (p === f.path || p.endsWith("/" + f.path))) url = f.url; }));
  return url;
}
function sessionCommits(s){
  const cs = commitsOf(s); if (!cs.length) return "";
  return `<h3>Commits during this session · ${cs.length}</h3>${cs.map(c => `<button class="gc-row" data-git="${esc(c.hash)}"><time>${hm(c.t)}</time><span><i class="gtag${c.ai ? " ai" : ""}">${GIT_ICON}${esc(c.hash.slice(0,7))}</i> ${esc(c.subject)}</span><b>+${c.added} −${c.removed}</b></button>`).join("")}`;
}
function commitDetail(c){
  const ses = DATA.find(x => x.id === c.session) || DATA.find(x => x.project === c.project && c.t >= x.start - 60 && c.t <= x.end + 600);
  const P = $("#panel");
  P.innerHTML = `<div style="--c:${st.colorBy === "project" ? colorOf(c.project) : "var(--ink-3)"}">
    <div class="eyebrow"><span class="dot"></span>GIT · ${c.ai ? "Run by AI" : "By hand (or another tool)"}</div>
    <h2>${esc(c.subject)}</h2>
    <div class="muted" style="font-variant-numeric:tabular-nums">${md(c.t)} ${hm(c.t)}</div>
    <div class="meta"><span>${esc(c.project)}</span>${c.branch ? `<span>${esc(c.branch)}</span>` : ""}${ext(c.url, `<span class="mono">${esc(c.hash.slice(0,7))}</span>`)}</div>
    <div class="dcols"><div class="dcol">
    <div class="mini">
      <div><div class="k">Files changed</div><div class="v">${c.nFiles}</div></div>
      <div><div class="k">Added</div><div class="v"><small>+</small>${c.added}<small> lines</small></div></div>
      <div><div class="k">Removed</div><div class="v"><small>−</small>${c.removed}<small> lines</small></div></div>
    </div>
    ${c.body ? `<h3>Message body</h3><p class="gbody">${esc(c.body)}</p>` : ""}
    ${ses ? `<h3>${c.session ? "Session that made this commit" : "Session running at this time"}</h3><button class="card" data-id="${esc(ses.id)}"><span class="ti">${esc(ses.title)}</span><span class="me">${md(ses.start)} ${hm(ses.start)}–${hm(ses.end)} · ${esc((ses.source))}</span></button>` : ""}
    ${c.url ? `<p style="margin-top:14px">${ext(c.url, "Open this commit on the remote ↗", "pill")}</p>` : ""}
    </div><div class="dcol">
    <h3>Files changed · ${c.nFiles}</h3>
    ${c.files.length ? fileRows(c.files, c.nFiles) : `<p class="none">None</p>`}
    <h3>Repository</h3><div class="code"><code>${esc(`git -C ${c.repo} show ${c.hash}`)}</code><button class="copy" data-copy="${esc(`git -C ${c.repo} show ${c.hash}`)}">Copy</button></div>
  </div></div></div>`;
  P.querySelectorAll(".card").forEach(b => b.onclick = () => select(b.dataset.id));
  bindCopy(P);
}
/* ── drawer: session detail ── */
const FIXRE = /違う|ちがう|そうじゃな|やり直|戻して|元に戻|取り消|じゃなくて|\b(?:wrong|incorrect|nope|revert|undo|roll ?back|start over|not what|that's not|try again|(?:doesn't|does not|didn't|did not|still not|isn't|is not) work(?:ing)?|still (?:broken|failing|fails)|you broke)\b/i; // 言い直し・中断らしいプロンプト（プロンプトの流れで点の色を変える）
function detail(s){
  if (!s){ st.sel = null; return summary(); }
  const active = s.segs.reduce((t,[a,b])=>t+(b-a),0)/60, waits = s.waits.map(x=>x[1]).sort((a,b)=>a-b);
  const med = waits.length ? waits[Math.floor(waits.length/2)] : null, maxT = Math.max(1, ...s.tools.map(t=>t[1]));
  const sameDay = new Date(s.start*1000).toDateString() === new Date(s.end*1000).toDateString();
  const allTok = [s.usage, ...s.subagents.map(a=>a.usage)].reduce((t,u)=>t + (u ? u.in+u.out+u.cw+u.cw1h+u.cr : 0), 0);
  const P = $("#panel");
  P.innerHTML = `<div style="--c:${colorOf(keyOf(s))}">
    <div class="eyebrow"><span class="dot"></span>${esc((s.source))}</div>
    <h2>${esc(s.title)}</h2>
    <div class="muted" style="font-variant-numeric:tabular-nums">${md(s.start)} ${hm(s.start)} – ${sameDay ? "" : md(s.end)+" "}${hm(s.end)}</div>
    <div class="meta"><span>${esc(s.project)}</span>${s.branch?`<span>${esc(s.branch)}</span>`:""}</div>
    <div class="dcols"><div class="dcol">
    <div class="mini">
      <div><div class="k">Active time</div><div class="v">${dur(active,true)}</div></div>
      <div><div class="k">Prompts</div><div class="v">${s.nPrompts}</div></div>
      <div><div class="k">Wait time (median)</div><div class="v">${med==null?"—":secsH(med)}</div></div>
      <div><div class="k">Corrections / interruptions</div><div class="v">${s.corrections + s.interrupts}</div></div>
      ${s.limits && s.limits.length ? `<div><div class="k">Usage limit hits</div><div class="v" style="color:var(--warn)">${s.limits.length}</div><div class="k" style="margin-top:2px">${s.limits.map(hm).join(", ")}</div></div>` : ""}
      ${s.source === "Claude Code" ? `<div><div class="k">Estimated cost${s.costReported ? " (from Claude Code)" : ""}</div><div class="v">${usd(s.cost).replace("$","<small>$</small>")}</div></div>
      <div><div class="k">Tokens</div><div class="v">${tok(allTok)}</div></div>
      ${(() => { const cs = commitsOf(s), ai = cs.filter(c => c.ai).length, o = s.outputs || {}; // 右の「このセッションの間のコミット」と同じ数え方（手でのコミットも入れ、うち AI を添える）
        if (!cs.length) return o.commits ? `<div><div class="k">AI commits</div><div class="v">${o.commits}</div></div>` : ""; // git を読めないときは、AI が実行した回数
        return `<div><div class="k">Commits (by AI)</div><div class="v">${cs.length}<small>${` (${ai})`}</small></div></div>${o.prs ? `<div><div class="k">Pull requests created</div><div class="v">${o.prs}</div></div>` : ""}`; })()}
` : s.credits ? `<div><div class="k">Kiro credits</div><div class="v">${crN(s.credits)}</div></div><div><div class="k">Per prompt</div><div class="v">${s.nPrompts ? crN(s.credits/s.nPrompts) : "—"}<small> credits</small></div></div>` : ""}
    </div>
    <div class="sact"><button class="pill" id="sreview">Review this session with AI (copy prompt)</button>
      <span>Asks for ways to improve how you prompted and split the work, based on the prompt flow and numbers. It includes your prompts, so review it before sending.</span></div>
    <h3>Prompt flow</h3>
    ${s.prompts.length ? promptFlow(s) : `<p class="none">No prompts recorded.${s.source === "Kiro Crew" ? " Kiro Crew deletes conversation records after a while, so only the usage record remains for this conversation." : ""}</p>`}
    ${s.subagents.length ? `<h3>Subagents · ${s.subagents.length}</h3>${s.subagents.map(a=>{
        const span = Math.max(1, s.end - s.start), l = a.start ? Math.max(0,(a.start - s.start)/span*100) : 0, w = a.start && a.end ? Math.max(.8,(a.end - a.start)/span*100) : .8;
        const t = a.usage, tt = t.in + t.out + t.cw + t.cw1h + t.cr;
        return `<div class="sub"><div class="hd"><span class="ty">${esc(a.type)}</span><span class="ds">${esc(a.desc || "(no description)")}</span>${a.bg?`<span class="bg">Background</span>`:""}</div>
          <div class="lane"><span style="left:${l}%;width:${Math.min(w,100-l)}%"></span></div>
          <div class="ft">${a.start?`<span>${hm(a.start)}${a.end?"–"+hm(a.end):""}</span>`:""}${a.start&&a.end?`<span>${dur((a.end-a.start)/60)}</span>`:""}${tt?`<span>${tok(tt)} tokens</span>`:t.reportedTokens?`<span>${tok(t.reportedTokens)} tokens (reported)</span>`:""}${t.cost?`<span>${usd(t.cost)}</span>`:""}${a.model?`<span>${esc(a.model)}</span>`:""}${a.tools?`<span>${plural(a.tools, "tool call")}</span>`:""}</div></div>`; }).join("")}` : ""}
    ${s.native && s.native.length ? `<h3>${`${esc((s.source))} metrics`}</h3>${nativeRows(s.native)}<p class="note">Numbers this agent records itself. Definitions differ from other agents.</p>` : ""}
    </div><div class="dcol">
    ${sessionCommits(s)}
    ${s.prs && s.prs.length ? `<h3>Pull requests created · ${s.prs.length}</h3><ul class="files">${s.prs.map(u => `<li title="${esc(u)}"><span>${ext(u, esc(u.replace(/^https?:\/\//, "")))}</span></li>`).join("")}</ul>` : ""}
    <h3>Files changed · ${s.nFiles}</h3>
    ${s.files.length ? `<ul class="files">${s.files.map(f=>{ const u = fileLink(s, f); return `<li title="${esc(f)}"><span>${u ? ext(u, esc(f)) : esc(f)}</span></li>`; }).join("")}</ul>` : `<p class="none">None</p>`}
    ${s.files.length && commitsOf(s).some(c => c.url) ? `<p class="note">Links open each file as of the commits made during this session.</p>` : ""}
    ${s.models.length ? `<h3>Models used</h3><div class="chips">${s.models.map(([m,n])=>`<span class="mono">${esc((m))}<b>${n}</b></span>`).join("")}</div>` : ""}
    <h3>Tools used</h3>
    ${s.tools.length ? s.tools.map(([k,v])=>`<div class="trow"><span class="nm">${esc(k)}</span><span class="track2"><span style="width:${v/maxT*100}%"></span></span><span class="n">${v}</span></div>`).join("") : `<p class="none">None recorded</p>`}
    ${s.file ? `<h3>History file</h3><div class="code"><code>${esc(s.file)}</code><a class="copy" href="${esc(LIVE ? "history?id=" + encodeURIComponent(s.id) : fileHref(s.file))}" target="_blank" rel="noopener">Open</a><button class="copy" data-copy="${esc(s.file)}">Copy</button></div>` : ""}
    ${s.resume ? `<h3>Resume</h3><div class="code"><code>${esc(s.resume)}</code><button class="copy" data-copy="${esc(s.resume)}">Copy</button></div>` : ""}

  </div></div></div>`;
  P.querySelectorAll("[data-git]").forEach(b => b.onclick = () => select("git:" + b.dataset.git));
  P.querySelector("#sreview").onclick = () => copy(sessionPrompt(s, active, med));
  P.querySelectorAll(".pexp").forEach(b => b.onclick = () => { const li = b.closest("li"), open = b.getAttribute("aria-expanded") !== "true";
    li.querySelector(".pshort").hidden = open; li.querySelector(".pfull").hidden = !open; b.setAttribute("aria-expanded", open); b.textContent = open ? "Show less" : pexpLabel(li.dataset.full ? {} : s.prompts[b.dataset.i]); });
  P.querySelectorAll(".pload").forEach(b => b.onclick = async () => { // kiroku serve のときだけ、切ったプロンプトの全文をサーバーから読む
    b.disabled = true;
    try { const r = await fetch(`prompt?id=${encodeURIComponent(s.id)}&i=${b.dataset.i}`, {cache: "no-store"}); if (!r.ok) throw 0;
      const li = b.closest("li"); li.querySelector(".pfull").textContent = await r.text(); li.dataset.full = "1"; li.querySelector(".pexp").focus(); // 「閉じる」は残して、また畳めるように
    } catch { b.disabled = false; b.textContent = "Couldn't load. Try again"; } });
  P.querySelectorAll("#flowBy button").forEach(b => b.onclick = () => { st.flowUser = b.dataset.v === "user"; store.set("flowUser", st.flowUser); P.querySelector(".tl").classList.toggle("only-user", st.flowUser); P.querySelectorAll("#flowBy button").forEach(x => x.setAttribute("aria-pressed", String(x === b))); });
  const pc = P.querySelector("#pcopy"); if (pc) pc.onclick = () => copy(userPrompts(s), `Copied ${plural(s.prompts.length, "user prompt")}`);
  const pall = P.querySelector(".pall"); if (pall) pall.onclick = () => { const shown = [...P.querySelectorAll(".tl li[hidden]")]; shown.forEach(li => li.hidden = false); pall.remove();
    const first = shown.find(li => li.classList.contains("pr")); if (first) first.focus(); }; // 出した最初のプロンプトへ（フォーカスを失わないように）
  bindCopy(P);
}
/* プロンプトの流れ: プロンプトと、その間に起きたこと（コミット・PR・利用上限・中断・サブエージェント）を時刻の順に 1 本に並べる */
const FLOW_SHOW = 30, FLOW_GAP = 1800; // はじめに出すプロンプトの数 / これより長い待たせは「あいた」の区切りにする（待たせの中央値と同じ 30 分）
function span(v){ return v < 3600 ? secs(v) : dur(v / 60); } // 秒数を、1 時間からは時間と分で
function prName(url){ // GitHub の PR は「リポジトリ#番号」と短く（狭い画面でも番号が見えるように）
  const m = /github\.com\/[^/]+\/([^/]+)\/pull\/(\d+)/.exec(url || "");
  return m ? `${m[1]}#${m[2]}` : String(url || "").replace(/^https?:\/\//, "");
}
const NOTE_LABEL = () => ({reminder: "System note", notice: "Notification", hook: "Hook output", output: "Command output",
  compact: "Conversation summary", agent: "From another agent or schedule", meta: "Added by the agent", other: "Added automatically"});
const PKIND = () => ({command: "Command", shell: "Shell"}); // 人が打ったプロンプトのうち、ふつうの文でないもの
function periodPrompts(){ // 表示中の週・月に、人が打ったプロンプトだけを、セッションごとに書き出す
  const {ws, we} = period(), L = [];
  DATA.filter(s => s.prompts.some(p => p.t >= ws && p.t < we)).sort((a, b) => a.start - b.start).forEach(s => {
    const ps = s.prompts.filter(p => p.t >= ws && p.t < we);
    L.push(`## ${md(ps[0].t)} ${hm(ps[0].t)} ${s.project}${s.branch ? ` (${s.branch})` : ""} · ${String(s.title).replace(/\s+/g, " ")}`, userPrompts({prompts: ps}), "");
  });
  return L.length ? L.join("\n").trim() : "No prompts in this period."; }
function userPrompts(s){ // 人が打ったプロンプトだけを、時刻つきの Markdown の箇条書きにする（書き出し用）
  return s.prompts.map(p => `- ${p.t ? `${md(p.t)} ${hm(p.t)}` : "--:--"}${p.kind ? ` [${PKIND()[p.kind] || p.kind}]` : ""} ${String(p.text || "").replace(/\s+/g, " ").trim()}${p.len > 0 ? ` (first ${PROMPT_RUNES} characters)` : ""}`).join("\n"); }
function flowEvents(s){ // l: 何が起きたか / d: 中身（狭い画面では d だけを省略する）
  const ev = [];
  commitsOf(s).forEach(c => ev.push({t: c.t, k: c.ai ? "commit ai" : "commit", l: c.ai ? "AI committed" : "Committed by hand", d: `<button class="evd" data-git="${esc(c.hash)}"><i class="gtag${c.ai ? " ai" : ""}">${GIT_ICON}${esc(c.hash.slice(0,7))}</i> ${esc(c.subject)}</button>`}));
  (s.prAt || []).forEach(p => ev.push({t: p.t, k: "pr", l: "Created a pull request", d: p.url ? `<span class="evd">${ext(p.url, esc(prName(p.url)))}</span>` : ""}));
  (s.limits || []).forEach(t => ev.push({t, k: "warn", l: "Hit a usage limit"}));
  (s.interruptsAt || []).forEach(t => ev.push({t, k: "int", l: "Interrupted"}));
  (s.notes || []).forEach(x => { if (x.t) ev.push({t: x.t, k: `note ${x.kind}`, l: NOTE_LABEL()[x.kind] || NOTE_LABEL().other, d: `<span class="evd">${esc(x.text)}</span>`}); }); // 人が打っていないもの（通知・要約など）
  s.subagents.forEach(a => { if (a.start) ev.push({t: a.start, k: "agent", l: "Subagent", d: `<span class="evd"><span class="mono">${esc(a.type)}</span>${a.desc ? ` · ${esc(a.desc)}` : ""}</span>`}); });
  return ev.sort((a, b) => a.t - b.t);
}
const pexpLabel = p => p.len > 0 ? `Read more (${p.len.toLocaleString()} characters)` : "Show all";
function promptFlow(s){
  const ev = flowEvents(s), rows = [];
  let e = 0, n = 0, hiddenEv = 0, gap = null; // gap: 前のプロンプトのあと、長くあいたところ {from: AI が最後に動いた時刻, v: 秒}
  const hide = () => n > FLOW_SHOW ? " hidden" : "";
  const flush = until => { for (; e < ev.length && ev[e].t < until; e++){ if (hide()) hiddenEv++;
    rows.push(`<li class="ev ${ev[e].k}"${hide()}><time>${hm(ev[e].t)}</time><p><span class="evl">${ev[e].l}</span>${ev[e].d || ""}</p></li>`); } };
  s.prompts.forEach((p, i) => {
    if (p.t){ flush(p.t); // あいた間に起きたこと（手でのコミットなど）は、区切りの上に出す
      if (gap){ rows.push(`<li class="gap"${hide()}><p>${`${hm(gap.from)}–${hm(p.t)}: ${span(gap.v)} gap`}</p></li>`); gap = null; } }
    n++;
    const t = String(p.text || ""), long = t.length > 220, fix = !p.kind && FIXRE.test(t), cut = p.len > 0;
    const more = cut ? `<span class="pcut"> ${`(first ${PROMPT_RUNES} of ${p.len.toLocaleString()} characters)`}${LIVE ? ` <button class="pload" data-i="${i}">Load the full prompt</button>` : ` Open with kiroku serve to read it in full.`}</span>` : "";
    const meta = [p.work ? `AI worked ${span(p.work)}` : "", p.wait && p.wait <= FLOW_GAP ? `wait ${secs(p.wait)}` : ""].filter(Boolean).join(" · ");
    rows.push(`<li class="pr${fix ? " fix" : ""}${p.kind ? " " + p.kind : ""}" tabindex="-1"${hide()}><time>${p.t ? hm(p.t) : ""}</time><p>${fix ? `<span class="sr">Looks like a correction: </span>` : ""}${p.kind ? `<span class="pkind">${PKIND()[p.kind] || esc(p.kind)}</span>` : ""}${plen(p) >= BIG_PROMPT ? `<span class="pkind big">${`Long · ${plen(p).toLocaleString()} chars`}</span>` : ""}<span class="ptext">${long ? `<span class="pshort">${esc(t.slice(0,220))}…</span><span class="pfull" hidden>${esc(t)}${more}</span> <button class="pexp" aria-expanded="false" data-i="${i}">${pexpLabel(p)}</button>` : esc(t)}</span>${meta ? `<span class="pmeta">${meta}</span>` : ""}</p></li>`);
    if (p.t && p.wait > FLOW_GAP) gap = {from: p.t + p.work, v: p.wait};
  });
  flush(Infinity);
  const fixes = s.prompts.some(p => !p.kind && FIXRE.test(String(p.text || ""))), kinds = new Set(ev.map(x => x.k.split(" ")[0])), cmds = s.prompts.some(p => p.kind);
  const names = list => { const t = list.filter(([k]) => kinds.has(k)).map(([, x]) => x).join(", "); return t.charAt(0).toUpperCase() + t.slice(1); }; // そのセッションにあるものの名前だけ
  const warn = names([["int", "interruption"], ["warn", "usage limit"]]), other = names([["commit", "commit"], ["pr", "pull request"], ["agent", "subagent"]]);
  const key = [`<span><i class="kp"></i>User prompt</span>`,
    cmds ? `<span><i class="kp cmd"></i>Commands the user typed (/ or !)</span>` : "",
    kinds.has("note") ? `<span><i class="ke note"></i>Added automatically (notifications, summaries, hooks; not counted as prompts)</span>` : "",
    fixes ? `<span><i class="kp fix"></i>Looks like a correction (guessed from the wording)</span>` : "",
    warn ? `<span><i class="ke int"></i>${warn}</span>` : "", other ? `<span><i class="ke"></i>${other}</span>` : ""].filter(Boolean).join("");
  const rest = s.prompts.length - FLOW_SHOW, also = hiddenEv ? ` (and ${plural(hiddenEv, "other event")})` : "";
  const bar = `<div class="flowbar"><div class="segc" role="group" aria-label="Show" id="flowBy"><button data-v="all" aria-pressed="${!st.flowUser}">Everything</button><button data-v="user" aria-pressed="${!!st.flowUser}">Only user prompts</button></div><button class="pill" id="pcopy">Copy prompts</button></div>`;
  return `${bar}<div class="tlkey">${key}</div><ol class="tl${st.flowUser ? " only-user" : ""}">${rows.join("")}</ol>${rest > 0 ? `<button class="more pall">${`Show ${plural(rest, "more prompt")}${also}`}</button>` : ""}${s.prompts.some(p => p.work) ? `<p class="note">${"\"AI worked\" is the time from a prompt to the AI's last activity; \"wait\" is the time from there to your next prompt. Both are estimates from the history's timestamps."}</p>` : ""}`;
}
/* 1 つのセッションを AI と振り返るためのプロンプト（kiroku 自身は AI を呼ばない） */
function sessionPrompt(s, active, med){
  const o = s.outputs || {}, L = [
    "You are an advisor on using AI agents effectively. Below is the record of one session I had with an AI agent (aggregated with kiroku).",
    "", "# What I'd like from you",
    "1. What went well in how this session was run",
    "2. Information that would have reduced rework if it had been in the first prompt (background, constraints, done criteria, etc.)",
    "3. Better ways to split and order the prompts, and an example first prompt for similar work next time",
    "", "# Assumptions",
    "- Figures are rough estimates from history. Clearly mark anything the data can't support as a guess",
    "- Estimated cost is priced at public API rates, not what I am actually billed",
    "", "# Session",
    `- Agent: ${(s.source)}`, `- Project: ${s.project}${s.branch ? ` (branch ${s.branch})` : ""}`,
    `- Time: ${md(s.start)} ${hm(s.start)}–${hm(s.end)}, active time ${dur(active)}`,
    `- Prompts: ${s.nPrompts}, corrections: ${s.corrections}, interruptions: ${s.interrupts}${med == null ? "" : `, median wait time ${secs(med)}`}`];
  if (s.limits && s.limits.length) L.push(`- Usage limit hits: ${s.limits.length} (${s.limits.map(hm).join(", ")})`);
  if (s.cost) L.push(`- Estimated cost: ${usd(s.cost)}`);
  if (s.credits) L.push(`- Kiro credits: ${crN(s.credits)}`);
  L.push(o.commits ? `- Commits: ${o.commits}${o.prs ? `, pull requests: ${o.prs}` : ""}` : "- No commits recorded");
  if (s.tools.length) L.push(`- Most used tools: ${s.tools.slice(0,6).map(([k,v]) => `${k} ${v}`).join(", ")}`);
  if (s.subagents.length) L.push(`- Subagents: ${s.subagents.length}`);
  L.push("", "# Prompt flow (time and prompt; long ones are truncated)");
  if (s.prompts.length) s.prompts.slice(0, 40).forEach(p => { const t = String(p.text || "").replace(/\s+/g, " ").trim(); L.push(`- ${p.t ? hm(p.t) : "--:--"} ${t.length > 300 ? t.slice(0, 300) + "…" : t}`); });
  else L.push("- No prompts recorded");
  if (s.prompts.length > 40) L.push(`- ${s.nPrompts - 40} more`);
  return L.join("\n");
}
const focusDrawer = {was: false, sel: null, first: null, from: null, next: null};
const OPENER_ATTRS = ["data-sid", "data-id", "data-s", "data-c", "data-git"];
let lastClick = null; addEventListener("click", e => { lastClick = e.target; }, true); // Safari はボタンを押してもフォーカスが移らないので、押した要素も覚える
function openerOf(el){ // 詳細を開いた要素を、描き直したあとも同じものを探せる形で覚える（どの枠の、どの属性の、何番目か）
  el = el && el.closest && el.closest(OPENER_ATTRS.map(a => `[${a}]`).join(","));
  const root = el && el.closest("#tl, #review"), a = root && OPENER_ATTRS.find(a => el.hasAttribute(a));
  if (!a) return null;
  const q = `[${a}="${CSS.escape(el.getAttribute(a))}"]`;
  return {root: root.id, q, n: [...root.querySelectorAll(q)].indexOf(el), y: scrollY}; }
function focusOpener(sel, from){ // 閉じたら、詳細を開いた要素（帯・バッジ・カード・検索結果）へフォーカスと読んでいた位置を戻す
  if (from){ const r = document.getElementById(from.root), el = r && r.querySelectorAll(from.q)[from.n];
    if (el){ scrollTo(0, from.y); el.focus({preventScroll: true}); return; } }
  if (!sel) return; const v = CSS.escape(sel.replace(/^git:/, ""));
  const el = sel.startsWith("git:") ? document.querySelector(`.gc[data-c="${v}"],[data-git="${v}"],[data-c="${v}"]`)
    : document.querySelector(`.run[data-sid="${v}"],[data-id="${v}"],[data-s="${v}"]`);
  if (el) el.focus({preventScroll: false}); }
function select(id){ // 詳細の中で別の詳細へ移ったときは、戻れるように前のものを積む
  if (id && !st.sel) focusDrawer.next = openerOf(lastClick) || openerOf(document.activeElement);
  if (id && st.sel && id !== st.sel){ st.back.push(st.sel); (st.backTop ||= []).push($("#panel").scrollTop); } else if (!id){ st.back = []; st.backTop = []; } // 戻ったとき、読んでいた位置に戻す
  st.sel = id; tipOff(); render(); if (id) $("#panel").scrollTop = 0; }

/* ── 1 年の露光（ふりかえり） ──
   1 年ぶんの作業していた区間を、横に日付・縦に時刻（朝 6 時から翌朝 6 時）の光の筋として描く。同時に動いていたほど明るく写る。
   シェア用の 1 枚もこのブラウザの中で描くだけで、どこにも送らない。画像に載せるのは集計した数字と光の筋だけで、
   プロンプト・プロジェクト名・ブランチ・ファイル・目安コストは載せない */
const PLATE = ["#3a9be0","#f07a2b","#1fbf8f","#e08fbd","#f2b53a","#7cc6f0","#9c84e6","#c9b51c"], PLATE_OTHER = "#6b7180";
const FONT = { mincho: '"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Hiragino Mincho ProN","Noto Serif JP",serif',
  sans: '"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Hiragino Sans","Noto Sans JP",system-ui,sans-serif',
  mono: 'ui-monospace,"SFMono-Regular","JetBrains Mono",Menlo,Consolas,monospace' };
const yr = { y: null, x: null, L: null, G: null, opt: Object.assign({out: true, type: true, grade: true, agents: true}, store.get("yrOpt", {})) };
function yearsOf(){ return [...new Set(Object.keys(MONTHS).map(k => +k.slice(0, 4)))].sort((a, b) => a - b); }
function yearData(y){
  const ms = Object.keys(MONTHS).filter(k => k.startsWith(y + "-")).sort().map(k => MONTHS[k]);
  const sum = f => ms.reduce((a, m) => a + (f(m) || 0), 0);
  const active = sum(m => m.active), fixP = sum(m => m.fixRate == null ? 0 : m.prompts);
  const by = {}; ms.forEach(m => ((m.shares || {}).source || []).forEach(s => by[s.key] = (by[s.key] || 0) + s.minutes));
  const agents = Object.keys(by).filter(k => by[k] > 0).sort((a, b) => by[b] - by[a]).map((k, i) => ({k, min: by[k], c: PLATE[i] || PLATE_OTHER}));
  const aMin = agents.reduce((a, g) => a + g.min, 0); agents.forEach(g => g.pct = aMin ? g.min / aMin * 100 : 0);
  const color = Object.fromEntries(agents.map(g => [g.k, g.c]));
  // いちばん長く続けて作業日数
  const on = new Set(); ms.forEach(m => (m.days || []).forEach((d, i) => { if (d.active > 0){ const [Y, M, D] = m.start.split("-").map(Number); on.add(key(new Date(Y, M-1, D+i))); } }));
  let streak = 0, run = 0; for (let d = new Date(y, 0, 1); d.getFullYear() === y; d = addDays(d, 1)){ run = on.has(key(d)) ? run + 1 : 0; streak = Math.max(streak, run); }
  // 光の筋：[列（日）, 上端, 下端（列の中の 0〜1）, 色]。列は朝 6 時で区切るので、深夜の作業は前の日の列の下のほうに写る
  const y0 = new Date(y, 0, 1)/1000, y1 = new Date(y+1, 0, 1)/1000, nd = Math.round((y1 - y0)/86400), day0 = new Date(y, 0, 1);
  const lines = [], hist = new Array(24).fill(0); let segMin = 0, nses = 0, nprompts = 0;
  DATA.forEach(s => {
    const mine = s.start >= y0 && s.start < y1; if (mine){ nses++; nprompts += s.nPrompts || 0; }
    s.segs.forEach(([a, b]) => {
      if (mine) segMin += (b - a)/60;
      a = Math.max(a, y0); b = Math.min(b, y1); if (b <= a) return;
      for (let t = a; t < b; t += 300) hist[new Date(t*1000).getHours()] += Math.min(300, b - t)/60;
      while (a < b){
        const d = new Date((a - 6*3600)*1000), c0 = new Date(d.getFullYear(), d.getMonth(), d.getDate(), 6)/1000, c1 = new Date(d.getFullYear(), d.getMonth(), d.getDate()+1, 6)/1000, e = Math.min(b, c1);
        const col = Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - day0)/864e5);
        if (col >= 0 && col < nd) lines.push([col, (a - c0)/(c1 - c0), (e - c0)/(c1 - c0), color[s.source] || PLATE_OTHER]);
        a = e;
      }
    });
  });
  const ht = hist.reduce((a, v) => a + v, 0) || 1, now = today0();
  // 腕前に使う数字（gradeOf）。割合は、記録のある期間・エージェントだけで出す
  const outBase = sum(m => m.outBase), cost = sum(m => m.usage && m.usage.cost), nY = Math.max(1, nses);
  const mods = {}; ms.forEach(m => ((m.usage || {}).models || []).forEach(([k, , t]) => mods[k] = (mods[k] || 0) + (t || 0)));
  const mT = Object.values(mods).reduce((a, v) => a + v, 0);
  // 上達：記録のある月が 4 か月以上なら、前半と後半で言い直しの割合と 1 コミットあたりのコストを比べる
  const half = ms.filter(m => m.prompts), hv = xs => { const p = xs.reduce((a, m) => a + (m.fixRate == null ? 0 : m.prompts), 0), c = xs.reduce((a, m) => a + (m.outputs ? m.outputs.commits : 0), 0);
    return {fix: p ? xs.reduce((a, m) => a + (m.fixRate == null ? 0 : m.fixRate * m.prompts), 0)/p : null, cpc: c >= 3 ? xs.reduce((a, m) => a + (m.costPerCommit == null ? 0 : m.costPerCommit * m.outputs.commits), 0)/c : null}; };
  const h1 = half.length >= 4 ? hv(half.slice(0, half.length >> 1)) : null, h2 = h1 && hv(half.slice(half.length >> 1));
  const grow = !!h1 && (h1.fix != null && h2.fix != null && h1.fix - h2.fix >= 3 || h1.cpc != null && h2.cpc != null && h2.cpc <= h1.cpc * .8);
  const first = DATA.find(s => s.start >= y0 && s.start < y1), span = Math.max(1, Math.round(((now.getFullYear() === y ? +now/1000 : y1) - (first ? Math.max(y0, first.start) : y0))/86400));
  return { y, nd, active, lines, agents, streak, sessions: nses,
    morningPct: (hist[5] + hist[6] + hist[7] + hist[8])/ht*100, peak: hist.indexOf(Math.max(...hist)),
    fix: fixP ? sum(m => m.fixRate == null ? 0 : m.fixRate * m.prompts)/fixP : null,
    avgMin: nses ? segMin/nses : 0, perSes: nses ? nprompts/nses : 0,
    commits: sum(m => m.git && m.git.commits), prs: sum(m => m.outputs && m.outputs.prs),
    days: on.size, prompts: nprompts, span, fixP, outBase, grow,
    reach: outBase >= 5 ? sum(m => m.outSessions)/outBase*100 : null,
    perCommit: outBase >= 5 && sum(m => m.outputs && m.outputs.commits) ? sum(m => m.prompts)/sum(m => m.outputs.commits) : null,
    parPct: active ? sum(m => m.parallel)/active*100 : 0, subagents: sum(m => m.usage && m.usage.subagents),
    models: mT ? Object.values(mods).filter(v => v/mT >= .1).length : 0,
    longPct: longCtxOf(y0, y1).length/nY*100, idlePct: cost >= 1 ? idleOf(y0, y1).c/cost*100 : null, lightPct: cost >= 1 ? lightOf(y0, y1).c/cost*100 : null,
    limits: limitHits(y0, y1).length,
    partial: now.getFullYear() === y ? now : null };
}
/* 光の名前：使い方の傾向を写真の言葉で表す（良し悪しではない）。上から順に、最初に当てはまったもの */
const LIGHTS = [
  {k: "dawn", en: "Daybreak", m: "morning", t: x => x.morningPct >= 25,
   de: "You get things done while the morning is fresh.",
   re: "25% or more of active time is between 5:00 and 9:00"},
  {k: "multi", en: "Multiple Exposure", m: "agents", t: x => x.agents.filter(g => g.pct >= 10).length >= 3,
   de: "You pick a partner for each job and layer them.",
   re: "3 or more agents, each with 10% or more of active time"},
  {k: "long", en: "Long Exposure", m: "avg", t: x => x.avgMin >= 45 && x.perSes <= 10,
   de: "You hand over big tasks and let them run.",
   re: "45 minutes or more of active time per session, with 10 prompts or fewer on average"},
  {k: "burst", en: "Burst", m: "per", t: x => x.perSes >= 15,
   de: "You ask small and often, and move fast.",
   re: "15 or more prompts per session on average"},
  {k: "focus", en: "Refocus", m: "fix", t: x => x.fix != null && x.fix >= 15,
   de: "You keep adjusting until it is just right.",
   re: "15% or more of prompts had a correction or interruption"},
  {k: "day", en: "Daylight", m: "peak", t: () => true,
   de: "You work with AI steadily through the day.",
   re: "None of the above"}];
/* 腕前：光の名前（型）とは別に、4 つのメーターで積み重ね・仕上げる力・使いこなしの幅・回り道を 0〜1 で測る。露光の画面だけの遊びで、週次・月次サマリーには使わない */
const TIERS = [ // 点数で決める 10 段階（10% ごと）
  "Novice", "Beginner", "Apprentice", "Competent", "Skilled",
  "Expert", "Master", "Virtuoso", "Grandmaster", "Legendary"].map((en, i) => ({en, u: i === 9 ? Infinity : (i + 1)/10}));
const OVERS = [ // 露出オーバー（働き方の危うさ・回り道が多すぎる）。上から順に、最初に当てはまったものを等級の代わりに冠にする
  {k: "blown", en: "Blown-Out", t: x => x.streak >= 30 && x.active/Math.max(1, x.days) >= 120,
   re: "30+ days in a row without a break at 2+ hours a day"},
  {k: "film", en: "Out-of-Film", t: x => x.limits >= 10,
   re: "Hit a usage limit 10 or more times"},
  {k: "noise", en: "Grainy", t: (x, G) => G.m.noise != null && G.m.noise >= .6,
   re: "Noise (detours) at 60% or more"}];
function gradeOf(x){
  const c = v => Math.max(0, Math.min(1, v)), avg = a => (a = a.filter(v => v != null)).length ? a.reduce((s, v) => s + v, 0)/a.length : null;
  const top2 = a => avg(a.slice().sort((p, q) => q - p).slice(0, 2)); // 使いこなしの幅は、どれか 2 つが得意なら高くなる
  const f = Math.max(.5, Math.min(1, x.span/365)); // 年の途中や使い始めの年は、経った日数のぶんだけ満点を下げる（使い始めてすぐ満点にならないよう、半分まで）
  const eff = 1/(x.agents.reduce((s, g) => s + (g.pct/100)**2, 0) || 1); // 使い分けの度合い（1 つだけなら 1、3 つを均等なら 3）
  const m = {
    shutter: avg([c(x.days/(180*f)), c(Math.log1p(x.prompts)/Math.log1p(3000*f)), c(Math.log1p(x.active/60)/Math.log1p(800*f))]),
    focus: avg([x.reach == null ? null : c(x.reach/50), x.perCommit == null ? null : c((20 - x.perCommit)/15)]), // コミットまで 50%・5 プロンプトで 1 コミットで満点
    multi: top2([c(x.parPct/30), c(Math.log1p(x.subagents)/Math.log1p(200*f)), c((eff - 1)/2), c((x.models - 1)/2)]),
    noise: avg([x.fix == null ? null : c((x.fix - 5)/20), c(x.longPct/15), x.idlePct == null ? null : c(x.idlePct/40), x.lightPct == null ? null : c(x.lightPct/20)]) };
  const base = avg([m.shutter, m.focus, m.multi]) || 0, score = Math.max(0, base - .4*(m.noise || 0));
  let ti = TIERS.findIndex(t => score < t.u);
  if (x.grow) ti = Math.min(TIERS.length - 1, ti + 1); // 上達していれば 1 段上げる
  if (ti === TIERS.length - 1 && !([m.shutter, m.focus ?? 1, m.multi].every(v => v >= .8) && (m.noise || 0) <= .2)) ti--; // 伝説は 3 つとも高く、ノイズが少ないときだけ
  const G = {m, score, tier: TIERS[ti], grow: x.grow};
  G.over = OVERS.find(o => o.t(x, G));
  G.exp = G.over ? "over" : score < .4 ? "under" : "ok";
  return G;
}
const pips = v => v == null ? 0 : Math.max(0, Math.min(10, Math.round(v*10))); // メーターの 10 段階
function lightMetric(k, x){
  return ({ morning: ["Work between 5:00 and 9:00", `${Math.round(x.morningPct)}%`],
    agents: ["Agents with 10%+ of time", String(x.agents.filter(g => g.pct >= 10).length)],
    avg: ["Active time per session", dur(x.avgMin)],
    per: ["Prompts per session", x.perSes.toFixed(1)],
    fix: ["Prompts with corrections", x.fix == null ? "—" : `${Math.round(x.fix)}%`],
    peak: ["Busiest hour", `${x.peak}:00`] })[k];
}
function seeded(a){ return () => (a = (a * 16807) % 2147483647) / 2147483647; }
function exposure(g, X, Y, W, H, x, p, grid, c0 = 0, c1 = x.nd){ // 列 [c0, c1) だけを描く
  g.save(); g.beginPath(); g.rect(X, Y, W, H); g.clip();
  g.fillStyle = "#06070a"; g.fillRect(X, Y, W, H);
  const r = seeded(7); g.fillStyle = "#ffffff";
  for (let i = 0; i < W*H/6000; i++){ g.globalAlpha = r()*.18; g.fillRect(X + r()*W, Y + r()*H, p, p); }
  g.globalAlpha = 1;
  if (grid){ g.strokeStyle = "rgba(255,255,255,.07)"; g.lineWidth = p; [.25, .5, .75].forEach(t => { g.beginPath(); g.moveTo(X, Y + t*H); g.lineTo(X + W, Y + t*H); g.stroke(); }); }
  g.globalCompositeOperation = "lighter"; g.lineCap = "round";
  const cw = W/(c1 - c0), lw = Math.min(3.5*p, Math.max(p*.9, cw*.7)), glow = Math.min(lw*6, Math.max(lw*2.2, cw*.9));
  x.lines.forEach(([c, a, b, col]) => {
    if (c < c0 || c >= c1) return;
    const px = X + (c - c0 + .5)*cw, y0 = Y + a*H, y1 = Math.max(y0 + lw*.5, Y + b*H);
    g.strokeStyle = col;
    g.globalAlpha = .1; g.lineWidth = glow; g.beginPath(); g.moveTo(px, y0); g.lineTo(px, y1); g.stroke();
    g.globalAlpha = .75; g.lineWidth = lw*.8; g.beginPath(); g.moveTo(px, y0); g.lineTo(px, y1); g.stroke();
  });
  g.restore();
}
function drawPlate(){
  const cv = $("#yrplate"); if (!cv || !yr.x) return;
  const r = cv.getBoundingClientRect(), p = Math.min(2, devicePixelRatio || 1), W = Math.round(r.width*p), H = Math.round(r.height*p);
  if (!W || !H) return; cv.width = W; cv.height = H; exposure(cv.getContext("2d"), 0, 0, W, H, yr.x, p, true);
}
function cardMeters(g, X, Y, ink2, amber){ // シェア画像の腕前：4 つのメーターを 10 段階で
  const G = yr.G; g.save(); g.textAlign = "left"; g.textBaseline = "alphabetic";
  [["shutter", "SHUTTER"], ["focus", "FOCUS"], ["multi", "LAYERS"], ["noise", "NOISE"]].forEach(([k, n], r) => {
    const y = Y + r*38, p = pips(G.m[k]), on = k === "noise" ? "#ff7a5c" : amber;
    g.fillStyle = ink2; g.font = `500 ${(16)}px ${(FONT.mono)}`; if (("letterSpacing" in g)) g.letterSpacing = "3px"; g.fillText(n, X, y); if ("letterSpacing" in g) g.letterSpacing = "0px";
    for (let i = 0; i < 10; i++){ g.fillStyle = i < p ? on : "rgba(255,255,255,.12)"; g.shadowColor = on; g.shadowBlur = i < p ? 10 : 0;
      g.beginPath(); g.roundRect ? g.roundRect(X + 140 + i*16, y - 14, 12, 12, 4) : g.rect(X + 140 + i*16, y - 14, 12, 12); g.fill(); }
    g.shadowBlur = 0; });
  g.restore();
}
function drawCard(){
  const cv = $("#yrcard"); if (!cv || !yr.x) return;
  const x = yr.x, L = yr.L, o = yr.opt, W = 1600, H = 900, g = cv.getContext("2d");
  const ink = "#ece8df", ink2 = "#a9a69e", amber = "#ffae57", top = 430;
  const ls = v => { if ("letterSpacing" in g) g.letterSpacing = v; };
  g.clearRect(0, 0, W, H); g.fillStyle = "#06070a"; g.fillRect(0, 0, W, H);
  // 使い始めたばかりでも光が端に寄らないよう、画像は記録のある日から（短ければ 8 週ぶん）だけを写す
  const last = x.partial ? Math.round((new Date(x.partial.getFullYear(), x.partial.getMonth(), x.partial.getDate()) - new Date(x.y, 0, 1))/864e5) + 1 : x.nd;
  const first = x.lines.reduce((a, l) => Math.min(a, l[0]), last), c0 = Math.max(0, Math.min(first, last - 56));
  exposure(g, 0, top, W, H - top, x, 2, false, c0, last);
  const fade = g.createLinearGradient(0, top, 0, H);
  fade.addColorStop(0, "rgba(6,7,10,1)"); fade.addColorStop(.3, "rgba(6,7,10,0)"); fade.addColorStop(.72, "rgba(6,7,10,0)"); fade.addColorStop(1, "rgba(6,7,10,.92)");
  g.fillStyle = fade; g.fillRect(0, top, W, H - top);
  g.textAlign = "left"; g.textBaseline = "alphabetic";
  const p2 = n => String(n).padStart(2, "0"), pt = x.partial;
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; ls("4px");
  const d0 = new Date(x.y, 0, 1 + c0), d1 = new Date(x.y, 0, last);
  g.fillText(c0 || pt ? `KIROKU — ${x.y}.${p2(d0.getMonth()+1)}.${p2(d0.getDate())} → ${p2(d1.getMonth()+1)}.${p2(d1.getDate())}` : `KIROKU — ${x.y} EXPOSURE`, 80, 110); ls("0px");
  const hrs = x.active/60, hs = (hrs >= 10 ? Math.round(hrs) : Math.round(hrs*10)/10).toLocaleString(LOC());
  g.fillStyle = ink; g.font = `800 184px ${FONT.mincho}`; g.fillText(hs, 72, 300);
  const hw = g.measureText(hs).width;
  g.font = `600 54px ${FONT.mincho}`; g.fillText("hours", 72 + hw + 16, 300);
  g.fillStyle = ink2; g.font = `500 30px ${FONT.sans}`;
  g.fillText(pt ? "This year's light, with AI." : "A year of light, with AI.", 80, 360);
  const items = [[x.sessions, "sessions"]];
  if (o.out){ if (x.commits) items.push([x.commits, "commits"]); if (x.prs) items.push([x.prs, "PR" + (("s"))]); }
  let cx = 80;
  items.forEach(([n, l]) => { g.font = `500 26px ${FONT.mono}`; g.fillStyle = ink; const t = n.toLocaleString(LOC()); g.fillText(t, cx, 412); cx += g.measureText(t).width + 10;
    g.font = `500 24px ${FONT.sans}`; g.fillStyle = ink2; g.fillText(l, cx, 412); cx += g.measureText(l).width + 40; });
  if (o.type && L){
    const G = yr.G, pre = o.grade ? (G.over ? G.over.en : G.tier.en) : "", hot = "#ff7a5c", pc = o.grade && G.over ? hot : amber;
    g.shadowColor = "rgba(255,174,87,.5)"; g.shadowBlur = 36;
    {
      g.textAlign = "right"; g.shadowBlur = 0; g.fillStyle = ink2; g.font = `500 20px ${FONT.mono}`; ls("4px"); g.fillText("YOUR LIGHT", W - 80, 110); ls("0px");
      if (pre){ g.shadowColor = pc; g.shadowBlur = 24; g.fillStyle = pc; g.font = `700 34px ${FONT.mincho}`; g.fillText(pre, W - 80, 160); }
      g.shadowColor = "rgba(255,174,87,.5)"; g.shadowBlur = 36; g.fillStyle = amber; g.font = `800 64px ${FONT.mincho}`; g.fillText(L.en, W - 80, pre ? 228 : 190);
      g.shadowBlur = 0; if (o.grade) cardMeters(g, W - 80 - 300, 280, ink2, amber);
    }
    g.shadowBlur = 0; g.textAlign = "left"; g.textBaseline = "alphabetic";
  }
  if (o.agents){
    let lx = 80; g.font = `500 24px ${FONT.sans}`;
    x.agents.slice(0, 4).forEach(a => { g.fillStyle = a.c; g.shadowColor = a.c; g.shadowBlur = 14; g.beginPath(); g.arc(lx + 8, 842, 8, 0, Math.PI*2); g.fill(); g.shadowBlur = 0;
      const t = `${(a.k)} ${Math.round(a.pct)}%`; g.fillStyle = ink; g.fillText(t, lx + 26, 850); lx += 26 + g.measureText(t).width + 36; });
  }
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; g.textAlign = "right"; ls("2px"); g.fillText("kiroku", W - 80, 850); ls("0px"); g.textAlign = "left";
}
function renderYear(){
  const ys = yearsOf(), dlg = $("#yr"); if (!ys.length) return;
  if (!ys.includes(yr.y)) yr.y = ys.includes(today0().getFullYear()) ? today0().getFullYear() : ys[ys.length-1];
  const x = yr.x = yearData(yr.y), L = yr.L = LIGHTS.find(l => l.t(x)), G = yr.G = gradeOf(x), pt = x.partial;
  const pos = d => (Math.round((d - new Date(x.y, 0, 1))/864e5)/x.nd*100).toFixed(2);
  const months = Array.from({length: 12}, (_, i) => `<span style="left:${pos(new Date(x.y, i, 1))}%">${(new Date(x.y, i, 1).toLocaleString("en-US", {month: "short"}))}</span>`).join("");
  const hours = [["6", 0], ["12", 25], ["18", 50], ["0", 75], ["6", 100]].map(([h, t]) => `<span style="top:${t}%">${`${h}:00`}</span>`).join("");
  const ev = [L.m, ...["morning", "avg", "per"].filter(k => k !== L.m)].slice(0, 3).map(k => lightMetric(k, x));
  const vt = t => esc(t);
  const name = l => l.en, opt = (k, l) => `<label><input type="checkbox" data-o="${k}"${yr.opt[k] ? " checked" : ""}>${l}</label>`;
  dlg.innerHTML = `<div class="yrhd"><div><div class="eyebrow">Year in review</div><h2 id="yrh">${`${x.y} exposure`}</h2></div>
    <label><span class="sr">Year</span><select id="yrsel">${ys.map(v => `<option value="${v}"${v === x.y ? " selected" : ""}>${v}</option>`).join("")}</select></label>
    <form method="dialog"><button class="iconbtn" aria-label="Close"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form></div>
  <p class="yrlead">Your active time with AI, drawn like a year-long exposure. Across is the date, down is the time of day (6:00 to 6:00 the next morning). Each stretch of active time is a streak of light, brighter where sessions overlapped.${pt ? ` Recorded up to ${pt.toLocaleDateString("en-US", {month: "short", day: "numeric"})}.` : ""}</p>
  <div class="yrscroll"><div class="yrchart"><div class="yrax" aria-hidden="true">${hours}</div><canvas class="yrplate" id="yrplate" role="img" aria-label="${esc(`Active time in ${x.y} by date and time of day. Active time ${dur(x.active)}`)}"></canvas><div class="yrx" aria-hidden="true">${months}</div></div></div>
  <div class="yrleg">${x.agents.map(a => `<span><i style="background:${a.c}"></i>${esc((a.k))} ${Math.round(a.pct)}%</span>`).join("")}</div>
  <div class="yrhow"><div><b>Length = how long you worked</b>Each streak is one stretch of active time in a session.</div>
    <div><b>Color = agent</b>Colors that start to mix show when you began using more than one.</div>
    <div><b>Dark bands = time off</b>Holidays and days off show up dark.</div></div>

  <h3>An image to share</h3>
  <canvas class="yrcard" id="yrcard" width="1600" height="900" role="img" aria-label="${esc(`Image to share: ${dur(x.active)} with AI in ${x.y}`)}"></canvas>
  <div class="yropts">${opt("out", "Commits and PRs")}${opt("type", "Your light")}${opt("grade", "Skill")}${opt("agents", "Agent breakdown")}</div>
  <div class="yract"><button class="pill" id="yrsave">Save as PNG</button><button class="pill" id="yrcopy">Copy image</button></div>
  <p class="note">The image shows only totals such as active time and sessions, and the streaks of light. Prompts, project names, branches, files and estimated cost are never included. It is made in this browser and sent nowhere.</p>

  <h3>Your light</h3>
  <div class="yrtype"><div class="yrtn"><span class="pre${G.over ? " over" : ""}">${vt(G.over ? G.over.en : G.tier.en)}</span><span>${vt(name(L))}</span></div><div>
    <p class="yrtd">${L.de}</p>
    <div class="yrgrade"><div class="yrexp ${G.exp}">${{under: "Underexposed: room to grow", ok: "Well exposed", over: `Overexposed: ${G.over ? G.over.re : ""}`}[G.exp]}</div>${G.grow ? `<div class="yrexp ok">↗ Improving: the second half beat the first (one level up)</div>` : ""}
      ${meters(x, G)}</div>
    <dl class="yrev">${ev.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>
    <p class="note">${`Your light describes the shape of your year in photography terms. It is not a verdict. Why: ${L.re}`}${x.active < 600 ? " (based on little history, so take it lightly)" : ""}</p>
    <details class="yrtypes"><summary>All six and how they are chosen</summary><ul>${LIGHTS.map(l => `<li><b>${name(l)}</b> — ${l.re}</li>`).join("")}</ul>
      <p class="note">The first one that matches, from the top, is chosen.</p></details>
    <details class="yrtypes"><summary>Skill levels and overexposure</summary>
      <ul>${TIERS.map((t, i) => `<li><b>${t.en}</b> — ${`Score ${i*10}–${i*10 + 10}%`}</li>`).join("")}
        ${OVERS.map(o => `<li><b>${o.en}</b> — ${o.re}</li>`).join("")}</ul>
      <p class="note">${"The score is the average of shutter count, focus and layers, minus 40% of noise. Legendary needs all three at 80% or more and noise at 20% or less. If corrections drop by 3 points or cost per commit falls by 20% from the first half of the year to the second, you are \"improving\" and go up one level. Shutter count comes from active days (180), prompts (3,000) and active time (800 hours); focus from sessions that reached a commit or pull request (50%) and few prompts per commit (5); layers from your best two of parallel time (30%), subagents (200 runs), and using several agents and models; noise from corrections, long conversations, cost with no commit and expensive models for light work (full marks in brackets, scaled down to as little as half for a year in progress). These are rough proxies that can move for reasons unrelated to skill, so treat it as a game."}</p></details></details></div></div>`;
  $("#yrsel").onchange = e => { yr.y = +e.target.value; renderYear(); };
  dlg.querySelectorAll("[data-o]").forEach(c => c.onchange = () => { yr.opt[c.dataset.o] = c.checked; store.set("yrOpt", yr.opt); drawCard(); });
  const blob = () => new Promise(res => $("#yrcard").toBlob(res, "image/png"));
  $("#yrsave").onclick = async () => { const b = await blob(); if (!b) return toast("Couldn't make the image");
    const a = document.createElement("a"); a.href = URL.createObjectURL(b); a.download = `kiroku-${x.y}.png`; document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000); toast("Saved the PNG"); };
  $("#yrcopy").onclick = async () => { try { await navigator.clipboard.write([new ClipboardItem({"image/png": blob()})]); toast("Copied the image"); }
    catch(e){ toast("Couldn't copy the image. Save it as a PNG instead", 2600); } };
  drawPlate(); drawCard();
  const sc = dlg.querySelector(".yrscroll"); sc.scrollLeft = sc.scrollWidth; // 狭い画面では、新しい記録の側を見せる
}
function meters(x, G){
  const pct = v => v == null ? "—" : `${Math.round(v)}%`;
  const rows = [
    ["shutter", "Shutter count", "Practice", `${plural(x.days, "day")} · ${plural(x.prompts, "prompt")} · ${dur(x.active)}`],
    ["focus", "Focus", "Follow-through", `To commit ${pct(x.reach)} · ${x.perCommit == null ? "—" : x.perCommit.toFixed(1)} prompts per commit`],
    ["multi", "Layers", "Range", `Parallel ${pct(x.parPct)} · ${plural(x.subagents, "subagent run")} · ${plural(x.agents.length, "agent")} · ${plural(x.models, "model")}`],
    ["noise", "Noise", "Detours (fewer means a cleaner shot)", `Corrections ${pct(x.fix)} · long conversations ${pct(x.longPct)} · cost with no commit ${pct(x.idlePct)} · expensive models for light work ${pct(x.lightPct)}`]];
  return `<dl class="yrmeter">${rows.map(([k, n, sub, v]) => { const p = pips(G.m[k]);
    return `<div${k === "noise" ? ' class="neg"' : ""}><dt>${n}<small>${sub}</small></dt><dd><span class="pp" role="img" aria-label="${`${p} of 10`}">${"<i class=on></i>".repeat(p)}${"<i></i>".repeat(10 - p)}</span><span class="pv">${v}</span></dd></div>`; }).join("")}</dl>`;
}
function openYear(){ const d = $("#yr"); if (!d.open) d.showModal(); renderYear(); }

/* ── events ── */
function go(n){
  if (st.mode === "month") st.month = n == null ? monthOf(today0()) : new Date(st.month.getFullYear(), st.month.getMonth()+n, 1);
  else st.week = n == null ? mondayOf(today0()) : addDays(st.week, 7*n);
  const inCal = !!document.activeElement?.closest?.("#tl");
  st.sel = null; st.back = []; st.backTop = []; st.animate = true; $("#toast").classList.remove("on"); render(); scrollToWork();
  if (inCal) keepCalFocus(n); }
function keepCalFocus(n){ // カレンダーの中にいたまま期間を移ったら、描き直しで消えたフォーカスを、新しい期間の最初のブロック（なければ押したボタン）へ
  const sc = $("#tl .calscroll"), top = sc ? sc.scrollTop : 0;
  const box = sc && sc.getBoundingClientRect(), head = sc && sc.querySelector(".head"), y0 = box ? box.top + (head ? head.offsetHeight : 0) : -Infinity;
  const cands = [...$("#tl").querySelectorAll(".run, button.cell")], seen = cands.find(el => el.getBoundingClientRect().top >= y0); // 見出しに隠れていないもの
  const el = seen || cands[0] || $(n == null ? "#today" : n < 0 ? "#prev" : "#next");
  if (el) el.focus({preventScroll: true});
  if (sc) sc.scrollTop = top; }
function setMode(m){ if (m === "month" && st.mode !== "month") st.month = monthOf(addDays(st.week, 3));
  if (m === "week" && st.mode === "month" && monthOf(st.week).getTime() !== st.month.getTime()) st.week = mondayOf(st.month);
  st.mode = m; store.set("mode", m); st.sel = null; st.animate = true; render(); scrollToWork(); }
document.querySelectorAll("#mode button").forEach(b => b.onclick = () => setMode(b.dataset.v));
$("#prev").onclick = () => go(-1); $("#next").onclick = () => go(1); $("#today").onclick = () => go(null);
$("#zin").onclick = () => zoom(1); $("#zout").onclick = () => zoom(-1);
function zoom(dv){ st.z = Math.max(0, Math.min(HOURS.length-1, st.z+dv)); store.set("zh", st.z); render(); scrollToWork(); }
document.querySelectorAll("#colorBy button").forEach(b => b.onclick = () => { st.colorBy = b.dataset.v; st.hidden.clear(); store.set("colorBy", st.colorBy); render(); });
$("#q").oninput = e => { st.q = e.target.value.trim().toLowerCase(); render(); };
$("#theme").onclick = () => { st.theme = st.theme === "dark" ? "light" : "dark"; store.set("theme", st.theme); applyTheme(); };
$("#help").onclick = () => $("#keys").showModal();
$("#yrbtn").onclick = openYear;
addEventListener("resize", () => { if ($("#yr").open) drawPlate(); });
$("#scrim").onclick = () => select(null); $("#close").onclick = () => select(null); $("#back").onclick = () => { const prev = st.back.pop(), top = (st.backTop || []).pop(); st.sel = prev || null; render(); $("#panel").scrollTop = top || 0; };
document.addEventListener("keydown", e => {
  if (e.target.tagName === "INPUT"){ if (e.key === "Escape") e.target.blur(); return; }
  if (e.metaKey || e.ctrlKey || e.altKey || $("#keys").open || $("#yr").open) return;
  const k = e.key;
  if (k === "ArrowLeft") go(-1); else if (k === "ArrowRight") go(1); else if (k === "t" || k === "T") go(null);
  else if (k === "/"){ e.preventDefault(); $("#q").focus(); } else if (k === "+" || k === "=") zoom(1); else if (k === "-") zoom(-1);
  else if (k === "Escape" && st.sel) select(null); else if (k === "?") $("#keys").showModal();
  else if (k === "w" || k === "W") setMode("week"); else if (k === "m" || k === "M") setMode("month");
  else if ((k === "y" || k === "Y") && YEAR_ON && DATA.length) openYear();
});
function scrollToWork(){ // その週の作業が始まるころの少し前へ
  const sc = $("#tl .calscroll"); if (!sc) return;
  const ws = st.week.getTime()/1000, we = ws + 7*86400, hs = [];
  DATA.forEach(s => s.segs.forEach(([a,b]) => { if (b > ws && a < we) hs.push(new Date(Math.max(a,ws)*1000).getHours()); }));
  hs.sort((a,b) => a-b);
  // 朝 6 時より前の開始が 2 割に満たなければ、早い時刻の数本に引っぱられないよう、6 時以降でいちばん早い開始時刻へ。
  // 夜型で 2 割を超えるなら、今までどおり早いほうから 2 割の開始時刻へ
  const day = hs.filter(h => h >= 6), first = !hs.length ? 8 : day.length >= hs.length*0.8 ? day[0] : hs[Math.floor(hs.length*0.2)];
  sc.scrollTop = Math.max(0, first * (HOURS[st.z] || 44) - 14); // その時刻の線がちょうど日付の下に見えるように
  if (sc.scrollWidth > sc.clientWidth + 4){ // 横にスクロールする狭い画面では、今週は今日までで最後に作業日を、過ぎた週は月曜から見せる
    const w = WEEKS[key(st.week)], now = nowMs()/1000, heads = sc.querySelectorAll(".head");
    let i = -1; if (w && now < we) w.days.forEach((d, j) => { if (d.active && ws + j*86400 <= now) i = j; });
    const h = heads[i]; sc.scrollLeft = h ? Math.max(0, h.offsetLeft - 56 - h.offsetWidth) : 0; } // その日と前の日が収まるように
}
// 今週に記録がなければ、いちばん新しい記録の週から開く
if (DATA.length && !WEEKS[key(st.week)]) st.week = mondayOf(new Date(DATA[DATA.length-1].end*1000));
if (DATA.length && !MONTHS[mkey(st.month)]) st.month = monthOf(new Date(DATA[DATA.length-1].end*1000));
const mq = matchMedia("(max-width:1000px)"),
 ph = () => $("#q").placeholder = mq.matches ? "Search" : "Prompts, files, commits"; // 狭い画面では入力欄も狭いので短く
mq.addEventListener("change", () => { ph(); render(); }); ph();
applyTheme(); render(); scrollToWork();

/* ── live（kiroku serve） ── 新しい履歴を見つけたら、見ている週・選んだセッションはそのままで取り込む */
if (LIVE){
  const badge = $("#live"); badge.hidden = false;
  let busy = false;
  async function poll(){
    if (busy || document.hidden) return; busy = true;
    try {
      const r = await fetch("stamp", {cache:"no-store"}); if (!r.ok) throw new Error(r.status);
      if (parseFloat(await r.text()) !== GENERATED){
        const j = await (await fetch("data.json", {cache:"no-store"})).json(), before = DATA.length;
        applyData(j);
        render(); if ($("#yr").open) renderYear();
        const d = DATA.length - before;
        toast(d > 0 ? `Added ${plural(d, "new session")}` : "Updated to the latest history");
      }
      badge.classList.remove("off"); badge.title = `Updates automatically as your history grows (last checked ${new Date().toLocaleTimeString("en-US")})`;
    } catch(e){ badge.classList.add("off"); badge.title = "Not connected to kiroku serve. If you stopped it, start it again to resume"; }

    busy = false;
  }
  setInterval(poll, 3000); document.addEventListener("visibilitychange", poll);
}
