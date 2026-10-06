// ボタンとキーの操作、期間の移動、ズーム
/* ── events ── */
// navEnds は、‹ › で行ける端（表示中の週か月の初日）。先は今週・今月まで（デモはずらした時計で）。
// 期間だけのファイルは、入っている期間の中だけ（月だけのファイルの週表示は、その月にかかる週）
function navEnds(){ const M = st.mode === "month";
  if (META.scope){ const a = scopeStart(), z = META.scope.mode === "week" ? addDays(a, 6) : new Date(a.getFullYear(), a.getMonth()+1, 0);
    return M ? {lo: monthOf(a), hi: monthOf(z)} : {lo: mondayOf(a), hi: mondayOf(z)}; }
  return {lo: null, hi: M ? monthOf(today0()) : mondayOf(today0())}; }
function canGo(n){ const {lo, hi} = navEnds(), cur = st.mode === "month" ? st.month : st.week;
  return n > 0 ? cur < hi : !lo || cur > lo; }
function go(n){
  if (n != null && !canGo(n)) return; // 先の週や、ファイルに入っていない期間へは行かない
  const home = META.scope ? scopeStart() : today0(); // 期間だけのファイルでは、その期間へ戻る
  if (st.mode === "month") st.month = n == null ? monthOf(home) : new Date(st.month.getFullYear(), st.month.getMonth()+n, 1);
  else st.week = n == null ? mondayOf(home) : addDays(st.week, 7*n);
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
function setMode(m){ if (m === "month" && META.scope && META.scope.mode === "week") return; // 週だけのファイルの月表示は、1 か月ぶんに見えて紛らわしい
  if (m === "month" && st.mode !== "month") st.month = monthOf(addDays(st.week, 3));
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
  const ws = st.week.getTime()/1000, we = addDays(st.week, 7).getTime()/1000, hs = [];
  DATA.forEach(s => s.segs.forEach(([a,b]) => { if (b > ws && a < we) hs.push(new Date(Math.max(a,ws)*1000).getHours()); }));
  hs.sort((a,b) => a-b);
  // 朝 6 時より前の開始が 2 割に満たなければ、早い時刻の数本に引っぱられないよう、6 時以降でいちばん早い開始時刻へ。
  // 夜型で 2 割を超えるなら、今までどおり早いほうから 2 割の開始時刻へ
  const day = hs.filter(h => h >= 6), first = !hs.length ? 8 : day.length >= hs.length*0.8 ? day[0] : hs[Math.floor(hs.length*0.2)];
  sc.scrollTop = Math.max(0, first * (HOURS[st.z] || 44) - 14); // その時刻の線がちょうど日付の下に見えるように
  if (sc.scrollWidth > sc.clientWidth + 4){ // 横にスクロールする狭い画面では、今週は今日までで最後に作業日を、過ぎた週は月曜から見せる
    const w = WEEKS[key(st.week)], now = nowMs()/1000, heads = sc.querySelectorAll(".head");
    let i = -1; if (w && now < we) w.days.forEach((d, j) => { if (d.active && addDays(st.week, j).getTime()/1000 <= now) i = j; });
    const h = heads[i]; sc.scrollLeft = h ? Math.max(0, h.offsetLeft - 56 - h.offsetWidth) : 0; } // その日と前の日が収まるように
}
