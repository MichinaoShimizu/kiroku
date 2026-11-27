// ボタンとキーの操作、期間の移動
/* ── events ── */
// navEnds は、‹ › で行ける端（表示中の週か月の初日）。先は今週・今月まで（デモはずらした時計で）。
// 期間だけのファイルは、入っている期間の中だけ（月だけのファイルの週表示は、その月にかかる週）
function navEnds(){ const M = st.mode === "month";
  if (META.scope){ const a = scopeStart(), z = META.scope.mode === "week" ? addDays(a, 6) : new Date(a.getFullYear(), a.getMonth()+1, 0);
    return M ? {lo: monthOf(a), hi: monthOf(z)} : {lo: mondayOf(a), hi: mondayOf(z)}; }
  const hs = historyStart(), first = isFinite(hs) ? new Date(hs * 1000) : null; // 記録の始まりより前へは行かない（空の週が続くだけなので）
  return {lo: first ? (M ? monthOf(first) : mondayOf(first)) : null, hi: M ? monthOf(today0()) : mondayOf(today0())}; }
function canGo(n){ const {lo, hi} = navEnds(), cur = st.mode === "month" ? st.month : st.week;
  return n > 0 ? cur < hi : !lo || cur > lo; }
function go(n){
  if (n != null && !canGo(n)) return; // 先の週や、ファイルに入っていない期間へは行かない
  const home = META.scope ? scopeStart() : today0(); // 期間だけのファイルでは、その期間へ戻る
  if (n == null){ st.month = monthOf(home); st.week = mondayOf(home); } // 「今週・今月」は、週と月の両方を戻す（今月を開いてから週にしても今週になるように）
  else if (st.mode === "month") st.month = new Date(st.month.getFullYear(), st.month.getMonth()+n, 1);
  else st.week = addDays(st.week, 7*n);
  const was = document.activeElement, inCal = !!was?.closest?.("#tl");
  st.sel = null; st.back = []; st.backTop = []; st.animate = true; $("#toast").classList.remove("on"); render(); scrollToWork();
  if (inCal) keepCalFocus(n);
  else if (was && was !== document.body && (!was.isConnected || document.activeElement === document.body)) focusPeriod(); } // 描き直しで消えたら、期間の見出しへ
function focusPeriod(){ const l = $(".wk .lbl"); if (!l) return; l.tabIndex = -1; l.focus({preventScroll: true}); }
function keepCalFocus(n){ // カレンダーの中にいたまま期間を移ったら、描き直しで消えたフォーカスを、新しい期間の最初のブロック（なければ押したボタン）へ
  const sc = $("#tl .calscroll"), top = sc ? sc.scrollTop : 0;
  const box = sc && sc.getBoundingClientRect(), head = $("#tl .head"), y0 = box ? box.top + (head ? head.offsetHeight : 0) : -Infinity;
  const cands = [...$("#tl").querySelectorAll(".run, button.cell")], seen = cands.find(el => el.getBoundingClientRect().top >= y0); // 見出しに隠れていないもの
  const el = seen || cands[0] || $(n == null ? "#today" : n < 0 ? "#prev" : "#next");
  if (el) el.focus({preventScroll: true});
  if (sc) sc.scrollTop = top; }
function setMode(m){ if (m === "month" && META.scope && META.scope.mode === "week") return; // 週だけのファイルの月表示は、1 か月ぶんに見えて紛らわしい
  if (m === "month" && st.mode !== "month") st.month = monthOf(addDays(st.week, 3));
  if (m === "week" && st.mode === "month" && monthOf(st.week).getTime() !== st.month.getTime()) st.week = mondayOf(st.month);
  st.mode = m; st.sel = null; st.animate = true; render(); scrollToWork(); }
document.querySelectorAll("#mode button[data-v]").forEach(b => b.onclick = e => { setMode(b.dataset.v); if (e.detail) b.blur(); }); // マウスで押したら、フォーカスを外す（←→ で期間を動かせるように）
/* 表示：カレンダーかサマリー。広い画面ではページ全体を画面に固定し、選んだほうだけが中でスクロールする（ページとカレンダーの二重のスクロールをなくす） */
function showView(){ document.body.dataset.view = st.view;
  document.querySelectorAll("#view button[data-v]").forEach(b => b.setAttribute("aria-pressed", b.dataset.v === st.view)); }
function setView(v, focus){ if (v === st.view) return; st.view = v; showView();
  const m = $("main"); if (m) m.scrollTop = 0; if (v === "cal") scrollToWork(); else if (v === "prs") prsShow(); else skBack($("#review")); // サンキーの名前の札は、見えているときにしか測れない（隠れたまま描いたサマリーを開いたときに敷く）
  if (focus && v === "sum"){ const r = $("#review"); r.tabIndex = -1; r.focus({preventScroll: true}); } }
document.querySelectorAll("#view button[data-v]").forEach(b => b.onclick = e => { setView(b.dataset.v); if (e.detail) b.blur(); });
$("#skipcal").onclick = e => { e.preventDefault(); setView("sum", true); };
$("#prev").onclick = () => go(-1); $("#next").onclick = () => go(1); $("#today").onclick = () => go(null);
$("#q").oninput = e => { st.q = e.target.value.trim().toLowerCase(); st.srN = st.scN = 0; st.srP = st.srA = ""; render(); }; // 言葉が変わったら、結果はまた先頭の数件から
// 検索欄で Enter か ↓：最初の結果へフォーカスを移す（結果はカレンダーの上）。結果の中では ↑↓ で隣へ、最初の結果で ↑ なら検索欄へ
$("#q").addEventListener("keydown", e => { if ((e.key !== "Enter" && e.key !== "ArrowDown") || e.isComposing || !st.q) return;
  const f = $("#sres .srow") || $("#srh"); if (f){ e.preventDefault(); f.focus(); } });
// 「さらに表示」のボタンも順に入れる（↓ で飛ばすと、まだ結果があると気づかない）。結果がなければ見出しから ↑ で検索欄へ
$("#sres").addEventListener("keydown", e => { if ((e.key !== "ArrowDown" && e.key !== "ArrowUp") || !e.target.matches(".srow, .srmore, #srh")) return;
  const rows = [...$("#sres").querySelectorAll(".srow, .srmore")], i = rows.indexOf(e.target), to = i < 0 ? null : rows[i + (e.key === "ArrowDown" ? 1 : -1)];
  e.preventDefault(); // 最後の結果で ↓ を押しても、ページはスクロールしない
  if (to) to.focus(); else if (e.key === "ArrowUp" && i <= 0) $("#q").focus(); });
// ロゴは最初の画面へ。kiroku serve では "/"、HTML ファイルやデモでは、同じファイルを開き直す（"/" ではディスクやサイトの一番上へ行ってしまう）
if (!LIVE) $("#home").href = location.href.split(/[?#]/)[0];

// ダイアログ（ショートカット・Worth a look など）は、枠の外を押すと閉じる（詳細のパネルの #scrim と同じ）。
// 押し始めも外だったときだけ（枠の中で文字を選んで、外で離したときには閉じない）
document.querySelectorAll("dialog").forEach(d => { let down = false;
  const out = e => { const r = d.getBoundingClientRect(); return e.target === d && (e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom); };
  d.addEventListener("pointerdown", e => { down = out(e); });
  d.addEventListener("click", e => { if (down && out(e)) d.close(); down = false; }); });
$("#scrim").onclick = () => select(null); $("#close").onclick = () => select(null); $("#back").onclick = () => backStep();
function backStep(){ const prev = st.back.pop(), top = (st.backTop || []).pop(); st.sel = prev || null; render(); $("#panel").scrollTop = top || 0; dtopSync(); }
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
// ←→ を自分で使う部品（入力欄・選択・切り替えのボタン群など）と検索結果（↑↓ で動く）にいるときは、週を動かさない
const ARROW_OWN = "input, select, textarea, [contenteditable], #sres, .segc, [role=group], [role=radiogroup], [role=tablist], [role=listbox], [role=slider], [role=menu]";
document.addEventListener("keydown", e => {
  if (e.target.tagName === "INPUT"){ if (e.key === "Escape") e.target.blur(); return; }
  // ダイアログ（AI 分析も）のあいだは、うしろの画面を動かさない（Escape はダイアログだけを閉じる）
  if (e.metaKey || e.ctrlKey || e.altKey || $("#keys").open || $("#wk").open || $("#md").open) return;
  const k = e.key;
  if (k === "Escape" && closeHint()) return; // 開いた説明（?）があれば、それだけ閉じる
  const ses = st.sel && DATA.find(x => x.id === st.sel); // セッションの詳細を開いているとき
  if (k === "ArrowLeft" || k === "ArrowRight"){ // 詳細を開いているあいだは前後のセッションへ（週は動かさない。閉じて別の週へ飛ぶと、どこにいるかわからなくなる）
    if (e.target.closest?.(ARROW_OWN)) return;
    if (st.sel){ if (ses){ e.preventDefault(); const n = sesNeighbors(ses.id); goSession(k === "ArrowLeft" ? n.prev : n.next); } return; }
    go(k === "ArrowLeft" ? -1 : 1); }
  else if (st.sel && ["ArrowUp", "ArrowDown", "PageUp", "PageDown", "Home", "End", " "].includes(k) && !e.target.closest?.(ARROW_OWN) && !(k === " " && e.target.closest?.("button, a, summary"))){ // スペースは、ボタンの上では押す動きのまま // 詳細の中をキーでスクロール
    const P = $("#panel"), step = k === "ArrowUp" || k === "ArrowDown" ? 64 : P.clientHeight * 0.85;
    e.preventDefault(); if (k === "Home") P.scrollTop = 0; else if (k === "End") P.scrollTop = P.scrollHeight; else P.scrollBy({top: (k === "ArrowUp" || k === "PageUp" ? -1 : 1) * step}); }
  else if ((k === "f" || k === "F") && ses) toggleFlowFull();
  else if (k === "Escape" && $("#drawer").classList.contains("flowfull")) toggleFlowFull(false);
  else if (k === "t" || k === "T") go(null);
  else if (k === "/"){ e.preventDefault(); $("#q").focus(); }
  else if (k === "Escape" && st.sel) select(null); else if (k === "?") $("#keys").showModal();
  else if (k === "w" || k === "W") setMode("week"); else if (k === "m" || k === "M") setMode("month");
  else if ((k === "v" || k === "V") && !st.sel) setView(st.view === "cal" ? "sum" : "cal"); // プルリクエストの表からは、カレンダーへ // 詳細を開いているあいだは、うしろの表示を変えない
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
// fitHour は、作業の時間帯がカレンダーの枠（.calscroll の高さから日付の見出しを引いたもの）に収まる、いちばん大きな 1 時間の高さ。
// 既定（44px）より大きくはせず、小さくしすぎると名前が読めないので 36px より小さくもしない
function fitHour(){ const w = workHours(); if (!w) return 44;
  const box = Math.min(innerHeight * (matchMedia("(max-width:820px)").matches ? .70 : .74), 880) - 140, span = w.last - w.first + 0.5;
  return 44 * span <= box ? 44 : 36; }
function scrollToWork(){ // その週の作業が始まるころの少し前へ
  const sc = $("#tl .calscroll"); if (!sc) return;
  const ws = st.week.getTime()/1000, we = addDays(st.week, 7).getTime()/1000, w = workHours(), first = w ? w.first : 8;
  sc.scrollTop = Math.max(0, calY(first) - 14); // その時刻の線がちょうど日付の下に見えるように
  if (sc.scrollWidth > sc.clientWidth + 4){ // 横にスクロールする狭い画面では、今週は今日までで最後に作業日を、過ぎた週は月曜から見せる
    const w = WEEKS[key(st.week)], now = nowMs()/1000, heads = document.querySelectorAll("#tl .head");
    let i = -1; if (w && now < we) w.days.forEach((d, j) => { if (d.active && addDays(st.week, j).getTime()/1000 <= now) i = j; });
    const h = heads[i]; sc.scrollLeft = h ? Math.max(0, h.offsetLeft - 56 - h.offsetWidth) : 0; } // その日と前の日が収まるように
}
// openAtWork は、開いたときだけ、今週の作業が始まる時刻がページの下のほうに隠れていれば、その時刻が日付の見出しのすぐ下に来るまでページを送る
// （朝の時間帯は縮めて描くが、それでも作業の始まりが下のほうになる画面がある）。上の数字は、上へスクロールすれば見える。
// デモ（README のスクリーンショット）と、セッションを開いた状態で戻ってきたときは送らない
function openAtWork(){
  if ((META && META.demo) || st.sel || st.view !== "cal" || st.mode !== "week" || soFar() == null) return;
  const w = workHours(), sc = $("#tl .calscroll"), hd = $("#tl .tlhead"); if (!w || !sc || !hd) return;
  const lineY = sc.getBoundingClientRect().top + scrollY + calY(Math.max(0, w.first - 0.5));
  if (lineY - scrollY < innerHeight * 0.6) return; // もう最初の画面の上のほうに見えている
  const hdr = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--hdr")) || 64;
  scrollTo(0, Math.max(0, lineY - hdr - hd.offsetHeight - 8));
}
// --hdr は、ページの上の見出し（header）の高さ。カレンダーの曜日の見出しが、その下に貼り付く
function setHdr(){ const h = document.querySelector("header"); if (h) document.documentElement.style.setProperty("--hdr", h.offsetHeight + "px"); }
setHdr(); addEventListener("resize", setHdr);

// 「見直す候補」の一覧は、枠の外を押すと閉じる

document.addEventListener("click", e => { const b = e.target.closest && e.target.closest("[data-keys]"); if (b) $("#keys").showModal(); }); // ページの下の「キーボードの操作」（? と同じ一覧）
