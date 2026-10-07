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
  const was = document.activeElement, inCal = !!was?.closest?.("#tl");
  st.sel = null; st.back = []; st.backTop = []; st.animate = true; $("#toast").classList.remove("on"); render(); scrollToWork();
  if (inCal) keepCalFocus(n);
  else if (was && was !== document.body && (!was.isConnected || document.activeElement === document.body)) focusPeriod(); } // 描き直しで消えたら、期間の見出しへ
function focusPeriod(){ const l = $(".wk .lbl"); if (!l) return; l.tabIndex = -1; l.focus({preventScroll: true}); }
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
function zoom(dv){ st.z = Math.max(0, Math.min(HOURS.length-1, st.z+dv)); st.zAuto = false; store.set("zh", st.z); render(); scrollToWork(); }
$("#q").oninput = e => { st.q = e.target.value.trim().toLowerCase(); st.srN = st.scN = 0; render(); }; // 言葉が変わったら、結果はまた先頭の数件から
// 検索欄で Enter：最初の結果へフォーカスを移す（結果はカレンダーの上）
$("#q").addEventListener("keydown", e => { if (e.key !== "Enter" || e.isComposing || !st.q) return;
  const f = $("#sres .srow") || $("#srh"); if (f){ e.preventDefault(); f.focus(); } });
$("#theme").onclick = () => { st.theme = st.theme === "dark" ? "light" : "dark"; store.set("theme", st.theme); applyTheme(); };
$("#help").onclick = () => $("#keys").showModal();
$("#yrbtn").onclick = openYear;
addEventListener("resize", () => { if ($("#yr").open) drawPlate(); });
$("#scrim").onclick = () => select(null); $("#close").onclick = () => select(null); $("#back").onclick = () => backStep();
function backStep(){ const prev = st.back.pop(), top = (st.backTop || []).pop(); st.sel = prev || null; render(); $("#panel").scrollTop = top || 0; }
/* ブラウザの「戻る」：詳細を開いたら履歴を 1 つ積み、戻るで閉じる（詳細の中で移っていたら、1 つ前の詳細へ）。
   Esc や × で閉じたときは、積んだ分を history.back() で消して、履歴をそろえる。URL（#… も）は変えない */
const hist = {pushed: false, skip: false};
try { if (history.state && history.state.kiroku === "drawer") history.replaceState(null, ""); } catch(e){} // 開いたまま再読み込みしたときの残り
function histOpen(){ if (hist.pushed) return; try { history.pushState({kiroku: "drawer"}, ""); hist.pushed = true; } catch(e){} }
function histClose(){ if (!hist.pushed) return; hist.pushed = false; hist.skip = true; history.back(); }
addEventListener("popstate", () => {
  if (hist.skip){ hist.skip = false; return; } // 自分で消した分
  if (!hist.pushed) return;
  hist.pushed = false;
  if (!st.sel) return;
  if (st.back.length){ backStep(); histOpen(); } else select(null); });
// ←→ を自分で使う部品（入力欄・選択・切り替えのボタン群など）にいるときは、週を動かさない
const ARROW_OWN = "input, select, textarea, [contenteditable], .segc, [role=group], [role=radiogroup], [role=tablist], [role=listbox], [role=slider], [role=menu]";
document.addEventListener("keydown", e => {
  if (e.target.tagName === "INPUT"){ if (e.key === "Escape") e.target.blur(); return; }
  if (e.metaKey || e.ctrlKey || e.altKey || $("#keys").open || $("#yr").open) return;
  const k = e.key;
  if (k === "Escape" && closeHint()) return; // 開いた説明（?）があれば、それだけ閉じる
  if (k === "ArrowLeft" || k === "ArrowRight"){ // 詳細を開いているあいだも動かさない（閉じて別の週へ飛ぶと、どこにいるかわからなくなる）
    if (st.sel || e.target.closest?.(ARROW_OWN)) return;
    go(k === "ArrowLeft" ? -1 : 1); }
  else if (k === "t" || k === "T") go(null);
  else if (k === "/"){ e.preventDefault(); $("#q").focus(); } else if (k === "+" || k === "=") zoom(1); else if (k === "-") zoom(-1);
  else if (k === "Escape" && st.sel) select(null); else if (k === "?") $("#keys").showModal();
  else if (k === "w" || k === "W") setMode("week"); else if (k === "m" || k === "M") setMode("month");
  else if ((k === "y" || k === "Y") && YEAR_ON && DATA.length) openYear();
});
// workHours は、表示中の週の作業の時間帯 {first, last}（時。記録がなければ null）。
// 朝 6 時より前の開始が 2 割に満たなければ、早い時刻の数本に引っぱられないよう、6 時以降でいちばん早い開始時刻から。
// 夜型で 2 割を超えるなら、早いほうから 2 割の開始時刻から。終わりは、いちばん遅い終わり（その日のうち）
function workHours(){
  const ws = st.week.getTime()/1000, we = addDays(st.week, 7).getTime()/1000, hs = []; let last = 0;
  DATA.forEach(s => s.segs.forEach(([a,b]) => { if (b > ws && a < we){ const d = new Date(Math.max(a,ws)*1000); hs.push(d.getHours());
    last = Math.max(last, Math.min(24, clockH(new Date(d.getFullYear(), d.getMonth(), d.getDate()), Math.min(b, we)))); } }));
  if (!hs.length) return null;
  hs.sort((a,b) => a-b);
  const day = hs.filter(h => h >= 6), first = day.length >= hs.length*0.8 ? day[0] : hs[Math.floor(hs.length*0.2)];
  return {first, last: Math.max(first + 1, Math.ceil(last))}; }
// fitZoom は、作業の時間帯がカレンダーの枠（.calscroll の高さから日付の見出しを引いたもの）に収まる、いちばん大きな 1 時間の高さ。
// 既定（44px）より大きくはせず、小さくしすぎると名前が読めないので 36px より小さくもしない
function fitZoom(){ const w = workHours(); if (!w) return 2;
  const box = Math.min(innerHeight * (matchMedia("(max-width:820px)").matches ? .70 : .74), 880) - 140, span = w.last - w.first + 0.5;
  return HOURS[2] * span <= box ? 2 : 1; }
function scrollToWork(){ // その週の作業が始まるころの少し前へ
  const sc = $("#tl .calscroll"); if (!sc) return;
  const ws = st.week.getTime()/1000, we = addDays(st.week, 7).getTime()/1000, w = workHours(), first = w ? w.first : 8;
  sc.scrollTop = Math.max(0, first * (HOURS[st.z] || 44) - 14); // その時刻の線がちょうど日付の下に見えるように
  if (sc.scrollWidth > sc.clientWidth + 4){ // 横にスクロールする狭い画面では、今週は今日までで最後に作業日を、過ぎた週は月曜から見せる
    const w = WEEKS[key(st.week)], now = nowMs()/1000, heads = sc.querySelectorAll(".head");
    let i = -1; if (w && now < we) w.days.forEach((d, j) => { if (d.active && addDays(st.week, j).getTime()/1000 <= now) i = j; });
    const h = heads[i]; sc.scrollLeft = h ? Math.max(0, h.offsetLeft - 56 - h.offsetWidth) : 0; } // その日と前の日が収まるように
}
