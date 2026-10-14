// 起動：最初に開く期間を決めて、はじめて描く。データがそろってから呼ぶ（kiroku serve では data.json が届いてから。live.js の最後）
function boot(){
// 今週に記録がなければ、いちばん新しい記録の週から開く
if (DATA.length && !WEEKS[key(st.week)]) st.week = mondayOf(new Date(DATA[DATA.length-1].end*1000));
// デモは週に 1 回（月曜）作り直すので、今週は記録が少ないことが多い。平日がそろったいちばん新しい週から開く
if (META && META.demo){ const full = Object.keys(WEEKS).filter(k => WEEKS[k].days.slice(0,5).every(d => d.active)).sort().pop();
  if (full){ const [y,m,dd] = full.split("-").map(Number); st.week = new Date(y, m-1, dd); } }
if (DATA.length && !MONTHS[mkey(st.month)]) st.month = monthOf(new Date(DATA[DATA.length-1].end*1000));
// 期間だけのファイルは、その期間から開く（週か月かも、書き出したときのもの）
if (META && META.scope){ const f = scopeStart(); st.week = mondayOf(f); st.month = monthOf(f); st.mode = META.scope.mode;
  const bar = $("#scopebar"); bar.textContent = ""; bar.insertAdjacentHTML("beforeend", scopeNote()); bar.hidden = false;
  if (META.scope.mode === "week") $("#mode").hidden = true; }
// 言語を切り替えて開き直したときは、切り替える前の期間とセッションに戻る（setLang）
try { const k = JSON.parse(sessionStorage.getItem("kiroku:keep") || "null"); sessionStorage.removeItem("kiroku:keep");
  if (k && /^\d{4}-\d{2}-\d{2}$/.test(k.week) && /^\d{4}-\d{2}$/.test(k.month)){
    const [y, m, d] = k.week.split("-").map(Number), [my, mm] = k.month.split("-").map(Number);
    st.week = mondayOf(new Date(y, m-1, d)); st.month = new Date(my, mm-1, 1);
    if (k.mode === "week" || k.mode === "month") st.mode = k.mode;
    if (typeof k.sel === "string" && DATA.some(s => s.id === k.sel)) st.sel = k.sel; } } catch(e){}
mq.addEventListener("change", () => { ph(); render(); });
render(); scrollToWork();
}
// 文言・入力欄・テーマは、データを待たずに整える（kiroku serve で data.json を読んでいるあいだも、見た目がそろうように）
applyStaticText();
const mq = matchMedia("(max-width:1000px)"),
 ph = () => $("#q").placeholder = t(mq.matches ? "Search" : "Prompts, files, commits"); // 狭い画面では入力欄も狭いので短く
ph(); applyTheme();
