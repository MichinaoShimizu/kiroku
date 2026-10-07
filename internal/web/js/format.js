// 日付・数の書き方などの小物、色の割り当て、検索の文字、トースト、テーマ
/* ── helpers ── */
function mondayOf(d){ d = new Date(d); return new Date(d.getFullYear(), d.getMonth(), d.getDate()-((d.getDay()+6)%7)); } // 日付から作る（0 時のない日をまたいでも 0 時にそろう）
function monthOf(d){ d = new Date(d); return new Date(d.getFullYear(), d.getMonth(), 1); }
function mkey(d){ return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}`; }
function addDays(d,n){ d = new Date(d); return new Date(d.getFullYear(), d.getMonth(), d.getDate()+n); }
// 日付 d から時刻 t（秒）の日まで、暦で何日目か。夏時間で 23 時間や 25 時間の日があっても、日付で数えるのでずれない
function dayNo(d, t){ const x = new Date(t*1000); return Math.round((Date.UTC(x.getFullYear(), x.getMonth(), x.getDate()) - Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))/864e5); }
// 時刻 t（秒）が、日 d の 0 時から時計で何時間目か（0〜24）。経った秒数でなく時計の針で数えるので、夏時間の日も時刻の線とそろう
function clockH(d, t){ const x = new Date(t*1000); return Math.min(24, Math.max(0, (x.getTime() - x.getTimezoneOffset()*6e4 - Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))/36e5)); }
function key(d){ return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}-${String(d.getDate()).padStart(2,"0")}`; }
// plainText は、画面用の HTML（中の文字はエスケープ済み）から文字だけを取り出す（コピーする Markdown 用。どこにも挿入しない）
function plainText(html){ return new DOMParser().parseFromString(String(html ?? ""), "text/html").body.textContent; }
function esc(s){ return String(s ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c])); }
function hm(t){ return new Date(t*1000).toLocaleTimeString("en-US",{hour:"2-digit",minute:"2-digit",hourCycle:"h23"}); }
/* 日付の書き方はここの小物にそろえる。見る人の言語の設定によらず英語の月名で（"Sep 28"・"Sep 28, 2026"・"October 2026"）。
   どれも Date の日付をそのまま読むだけなので、デモや期間だけのファイルの時計（SHIFT）はそのまま効く */
const MON = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
const MONTH = ["January","February","March","April","May","June","July","August","September","October","November","December"];
const dMD = d => `${MON[d.getMonth()]} ${d.getDate()}`; // Sep 28
const dMDY = d => `${dMD(d)}, ${d.getFullYear()}`; // Sep 28, 2026
const dMY = d => `${MONTH[d.getMonth()]} ${d.getFullYear()}`; // October 2026
// dSpan は a から b まで（"Sep 28 – Oct 4"）。y なら年も付ける（"Sep 28 – Oct 4, 2026"、年をまたぐときは両方に）
function dSpan(a, b, y){ return !y ? `${dMD(a)} – ${dMD(b)}` : a.getFullYear() === b.getFullYear() ? `${dMD(a)} – ${dMDY(b)}` : `${dMDY(a)} – ${dMDY(b)}`; }
// dPeriod は、文面（週報の下書き・AI へのプロンプト）に書く期間の名前。月は "October 2026"、週は "Sep 28 – Oct 4, 2026"
const dPeriod = (M, a, b) => M ? dMY(a) : dSpan(a, b, true);
function dStamp(t){ return `${dMDY(new Date(t*1000))}, ${hm(t)}`; } // Oct 6, 2026, 17:14（時刻はカレンダーと同じ 24 時間で）
function md(t){ const d = new Date(t*1000); return `${DOW[d.getDay()]}, ${dMD(d)}`; } // Mon, Sep 28
function dur(m, html){ m = Math.round(m); const h = Math.floor(m/60), r = m%60;
  const u = x => html ? `<small>${x}</small>` : x; return h ? `${h}${u("h")}${r ? ` ${r}${u("m")}` : ""}` : `${r}${u("m")}`; }
function tok(n){ n = n || 0; return n >= 1e9 ? (n/1e9).toFixed(1)+"B" : n >= 1e6 ? (n/1e6).toFixed(1)+"M" : n >= 1e3 ? Math.round(n/1e3)+"K" : String(n); }
function usd(v){ return v == null ? "—" : v > 0 && v < 0.01 ? "<$0.01" : "$" + (v >= 100 ? Math.round(v).toLocaleString(LOC()) : v.toFixed(2)); }
// usdH は usd を HTML に入れる形にする（$ を小さく出す。"<$0.01" の < もエスケープする）
function usdH(v){ const s = esc(usd(v)), i = s.indexOf("$"); return i < 0 ? s : s.slice(0, i) + "<small>$</small>" + s.slice(i + 1); }
// costOf は使用量の目安コスト。見積もれた分がなく、料金表にないモデルのトークンだけなら null（usd・usdH が "—" にする）。
// 値がないところに $0.00 と出すと「使っていない」と読めてしまうため（Codex のように料金表にないモデルだけを使ったとき）
const costOf = u => !u ? null : !u.cost && u.unpriced ? null : (u.cost || 0);
const NOPRICE = "The models used are not in the price table, so the estimated cost cannot be shown. Add their prices with --prices";
const crN = v => v >= 1 || v <= 0 ? Math.round(v).toLocaleString(LOC()) : v.toFixed(2); // クレジットは整数で（1 未満だけ小数 2 桁）
function cr(v){ return `${crN(v)} cr`; }
const tokS = v => v >= 1e7 ? Math.round(v/1e6)+"M" : v >= 1e6 ? (v/1e6).toFixed(1)+"M" : v >= 1e4 ? Math.round(v/1e3)+"K" : tok(v); // 狭いマス用に、桁を減らしたトークン
function shade(i){ return [1,.72,.5,.34,.22,.14][Math.min(i,5)]; }
function secs(v){ return v == null ? "—" : (v < 60 ? `${v}s` : `${Math.floor(v/60)}m ${v%60}s`); }
function secsH(v){ return secs(v).replace(/(?<=\d)([ms])\b/g, "<small>$1</small>"); } // 単位を小さく
function isoWeek(d){ d = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())); const n = d.getUTCDay() || 7; d.setUTCDate(d.getUTCDate()+4-n);
  const y0 = new Date(Date.UTC(d.getUTCFullYear(),0,1)); return Math.ceil(((d-y0)/864e5+1)/7); }
const keyOf = s => st.colorBy === "project" ? s.project : st.colorBy === "source" ? s.source : `${s.project} · ${s.branch || "—"}`;
// エージェントの色は、どの画面でも同じにする（色分けの切り替えや件数の順では変わらない）。
// 表にないエージェントは、表で使っていない色を、多い順にあてる
const AG_SLOT = {"Claude Code": 1, "Kiro IDE": 6, "Kiro IDE (legacy)": 6, "Kiro CLI": 3, "Kiro CLI (SQLite)": 3, "Kiro Crew": 7, "Amazon Q": 5, "Codex": 2};
let slot = {}, agSlot = {};
function assignColors(){ // 全期間の多い順に固定。週を変えても、非表示にしても色は変わらない
  const count = f => { const n = {}; DATA.forEach(s => n[f(s)] = (n[f(s)]||0) + 1); return Object.keys(n).sort((a,b)=>n[b]-n[a]); };
  const free = [...Array(SLOTS).keys()].filter(i => !Object.values(AG_SLOT).includes(i));
  agSlot = {}; count(s => s.source).filter(k => !(k in AG_SLOT)).forEach((k,i) => { if (i < free.length) agSlot[k] = free[i]; });
  slot = {}; if (st.colorBy === "source") return;
  count(keyOf).forEach((k,i) => slot[k] = i < SLOTS ? `var(--c${i})` : "var(--other)");
}
const agIdx = name => AG_SLOT[name] ?? agSlot[name]; // 色の番号（--c0〜--c7）。色がなければ undefined
const agColor = name => { const i = agIdx(name); return i == null ? "var(--other)" : `var(--c${i})`; };
const colorOf = k => st.colorBy === "source" ? agColor(k) : slot[k] || "var(--other)";
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


