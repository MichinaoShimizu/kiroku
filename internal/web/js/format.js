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
function esc(s){ return String(s ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c])); }
function hm(t){ return new Date(t*1000).toLocaleTimeString("en-US",{hour:"2-digit",minute:"2-digit",hourCycle:"h23"}); }
/* 日付の書き方はここの小物にそろえる。見る人の言語の設定によらず英語の月名で（"Sep 28"・"Sep 28, 2026"・"October 2026"）。
   どれも Date の日付をそのまま読むだけなので、デモや期間だけのファイルの時計（SHIFT）はそのまま効く */
const MON = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
const MONTH = ["January","February","March","April","May","June","July","August","September","October","November","December"];
// 日本語では 9月28日・2026年9月28日・2026年10月
const dMD = d => isJA() ? `${d.getMonth()+1}月${d.getDate()}日` : `${MON[d.getMonth()]} ${d.getDate()}`; // Sep 28
const dMDY = d => isJA() ? `${d.getFullYear()}年${dMD(d)}` : `${dMD(d)}, ${d.getFullYear()}`; // Sep 28, 2026
const dMY = d => isJA() ? `${d.getFullYear()}年${d.getMonth()+1}月` : `${MONTH[d.getMonth()]} ${d.getFullYear()}`; // October 2026
// dSpan は a から b まで（"Sep 28 – Oct 4"）。y なら年も付ける（"Sep 28 – Oct 4, 2026"、年をまたぐときは両方に）
function dSpan(a, b, y){ if (a.toDateString() === b.toDateString()) return y ? dMDY(a) : dMD(a); // 同じ日なら 1 度だけ（Oct 5 – Oct 5 にしない）
  return !y ? `${dMD(a)} – ${dMD(b)}` : a.getFullYear() !== b.getFullYear() ? `${dMDY(a)} – ${dMDY(b)}` : isJA() ? `${dMDY(a)} – ${dMD(b)}` : `${dMD(a)} – ${dMDY(b)}`; }
function dStamp(t){ return `${dMDY(new Date(t*1000))}${isJA() ? " " : ", "}${hm(t)}`; } // Oct 6, 2026, 17:14（時刻はカレンダーと同じ 24 時間で）
function md(t){ const d = new Date(t*1000); return isJA() ? `${dMD(d)}(${DOW[d.getDay()]})` : `${DOW[d.getDay()]}, ${dMD(d)}`; } // Mon, Sep 28・9月28日(月)
function dur(m, html){ m = Math.round(m); const h = Math.floor(m/60), r = m%60;
  const u = x => html ? `<small>${x}</small>` : x; return h ? `${h}${u("h")}${r ? ` ${r}${u("m")}` : ""}` : `${r}${u("m")}`; }
function tok(n){ n = n || 0; return n >= 1e9 ? (n/1e9).toFixed(1)+"B" : n >= 1e6 ? (n/1e6).toFixed(1)+"M" : n >= 1e3 ? Math.round(n/1e3)+"K" : String(n); }
const commas = n => n.toLocaleString(LOC()); // 1,234（数のカンマ区切りはこれで）
// 足した行・消した行：git diff にならって、足した行は緑、消した行は赤（a と r は書式を整えた数の文字列）
const lnPM = (a, r) => `<span class="la">+${esc(String(a))}</span> <span class="lr">−${esc(String(r))}</span>`;
function usd(v){ return v == null ? "—" : v > 0 && v < 0.01 ? "<$0.01" : "$" + (v >= 100 ? commas(Math.round(v)) : v.toFixed(2)); }
const usdS = v => v >= 9.95 ? "$" + commas(Math.round(v)) : v >= 0.05 ? "$" + v.toFixed(1) : "<$0.1"; // 小数 1 桁に丸めて 10 になる値は整数に、0.1 に届かない値は "<$0.1" に // 狭いマス用に、桁を減らした推定費用
// usdH は usd を HTML に入れる形にする（$ を小さく出す。"<$0.01" の < もエスケープする）
function usdH(v){ const s = esc(usd(v)), i = s.indexOf("$"); return i < 0 ? s : s.slice(0, i) + "<small>$</small>" + s.slice(i + 1); }
// costOf は使用量の目安コスト。見積もれた分がなく、料金表にないモデルのトークンだけなら null（usd・usdH が "—" にする）。
// 値がないところに $0.00 と出すと「使っていない」と読めてしまうため（Codex のように料金表にないモデルだけを使ったとき）
const costOf = u => !u ? null : !u.cost && u.unpriced ? null : (u.cost || 0);
const NOPRICE = t("The models used are not in the price table, so the estimated cost cannot be shown. Add their prices with --prices.");
const crN = v => v >= 10 || v <= 0 ? commas(Math.round(v)) : v >= 1 ? (Math.round(v * 10) / 10).toString() : v.toFixed(2); // クレジットは整数で。10 未満は小数 1 桁（1.33 を 1 と丸めすぎない）、1 未満は小数 2 桁
// クレジットの単位（"cr" では何の数かわからないので、言葉で。日本語の画面でも英字の「credits」）
const crUnit = v => v === 1 && !isJA() ? "credit" : "credits"; // 日本語でもカタカナにせず「credits」（Kiro の画面と同じ言葉）
function cr(v){ return `${crN(v)} ${crUnit(v)}`; }
const crR = v => v >= 100 ? commas(Math.round(v)) : (Math.round(v * 10) / 10).toLocaleString(LOC()); // クレジットの率（小数 1 桁。いつもの範囲の数と同じ丸め）
const cr1 = v => `${Math.round(v * 10) / 10} ${crUnit(v)}`; // 式の中のクレジット（小数 1 桁。整数に丸めると、引き算の結果と合わなくなる）
const crH = v => `${crN(v)}<small>&nbsp;${esc(crUnit(v))}</small>`; // HTML に入れる形（単位を小さく）
const tokS = v => v >= 1e7 ? Math.round(v/1e6)+"M" : v >= 1e6 ? (v/1e6).toFixed(1)+"M" : v >= 1e4 ? Math.round(v/1e3)+"K" : tok(v); // 狭いマス用に、桁を減らしたトークン
function bytes(n){ return n >= 1<<30 ? (n/(1<<30)).toFixed(1)+" GB" : n >= 1<<20 ? (n/(1<<20)).toFixed(1)+" MB" : n >= 1<<10 ? (n/(1<<10)).toFixed(1)+" KB" : n+" B"; }
function secs(v){ return v == null ? "—" : (v < 60 ? `${v}s` : `${Math.floor(v/60)}m ${v%60}s`); }
function secsH(v){ return secs(v).replace(/(?<=\d)([ms])\b/g, "<small>$1</small>"); } // 単位を小さく
function isoWeek(d){ d = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())); const n = d.getUTCDay() || 7; d.setUTCDate(d.getUTCDate()+4-n);
  const y0 = new Date(Date.UTC(d.getUTCFullYear(),0,1)); return Math.ceil(((d-y0)/864e5+1)/7); }
const keyOf = s => st.colorBy === "project" ? s.project : s.source;
// エージェントの色は、どの画面でも同じにする（色分けの切り替えや件数の順では変わらない）。
// 色は core.Agents の Slot（警告に見えないよう、AG_WARN のオレンジと黄色は使わない）。
// 表にないエージェントは、表でも警告でも使っていない色を、多い順にあてる
const AG_SLOT = Object.fromEntries(AGENTS.map(a => [a.name, a.slot]));
let slot = Object.create(null), agSlot = Object.create(null); // 名前（プロジェクト名など、履歴から来る）を鍵にするので、__proto__ などがふつうの鍵になるように
function assignColors(){ // 全期間の多い順に固定。週を変えても、非表示にしても色は変わらない
  const count = f => { const n = Object.create(null); DATA.forEach(s => n[f(s)] = (n[f(s)]||0) + 1); return Object.keys(n).sort((a,b)=>n[b]-n[a]); };
  const free = [...Array(SLOTS).keys()].filter(i => !Object.values(AG_SLOT).includes(i) && !AG_WARN.includes(i));
  agSlot = Object.create(null); count(s => s.source).filter(k => !Object.hasOwn(AG_SLOT, k)).forEach((k,i) => { if (i < free.length) agSlot[k] = free[i]; });
  slot = Object.create(null); if (st.colorBy === "source") return;
  count(keyOf).forEach((k,i) => slot[k] = i < SLOTS ? `var(--c${i})` : "var(--other)");
}
const agIdx = name => (Object.hasOwn(AG_SLOT, name) ? AG_SLOT[name] : undefined) ?? agSlot[name]; // 色の番号（--c0〜--c7）。色がなければ undefined
const agColor = name => { const i = agIdx(name); return i == null ? "var(--other)" : `var(--c${i})`; };
/* モデル名の色（--c0〜--c7）。系統（Opus・Sonnet・Haiku・GPT など）ごとに決まった色で、知らない名前は名前から決める。
   同じ一覧の中で色が重なったら（同じ系統の別の版など）、空いている次の色にして、モデル名ごとに見分けられるようにする */
const MODEL_FAM = [[/opus/i, 6], [/sonnet/i, 0], [/haiku/i, 2], [/fable/i, 1], [/gpt|codex|^o\d/i, 5], [/gemini/i, 4], [/kiro|amazon|nova/i, 3]];
const modelSlot = m => { m = String(m || ""); const f = MODEL_FAM.find(([re]) => re.test(m)); if (f) return f[1];
  let h = 0; for (const ch of m) h = (h * 31 + ch.codePointAt(0)) >>> 0; return h % 8; };
function modelColors(names){ const used = new Set();
  return names.map(m => { let i = modelSlot(m); for (let k = 0; k < 8 && used.has(i); k++) i = (i + 1) % 8; used.add(i); return `var(--c${i})`; }); }
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
async function copy(text, msg, ms, btn){ try { await navigator.clipboard.writeText(text); toast(msg || t("Copied"), ms); copied(btn); } catch(e){ toast(t("Couldn't copy")); } }
function copied(btn){ if (!btn) return; btn.classList.add("ok"); clearTimeout(btn._ok); btn._ok = setTimeout(() => btn.classList.remove("ok"), 1600); } // copyBtn の印を ✓ に
const CB = () => ({project: t("Project"), source: t("Agent")}); // 色分けはプロジェクトとエージェントだけ


