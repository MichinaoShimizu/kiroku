// 埋め込んだデータ（Go が JSON を入れる）、デモや期間だけのファイルの時刻ずらし、保存（localStorage）、画面の状態 st
let DATA = __DATA__;
let WEEKS = __WEEKS__;
let MONTHS = __MONTHS__;
let META = __META__;
let GENERATED = __GEN__;
// applyData は kiroku serve から届いた新しいデータに入れかえる（上の名前を書きかえるので、ここに置く）
function applyData(j){ DATA = j.sessions || []; WEEKS = j.weeks || {}; MONTHS = j.months || {}; META = j.meta; GENERATED = j.generated; if (st.sel && !DATA.some(s => s.id === st.sel)) st.sel = null; }
/* デモ（GitHub Pages）と、kiroku html --week / --month で書き出した期間だけのファイルは、作った人の時間帯の時計で見せる。
   どこから開いても「朝から夜に作業した」ように見え、日ごとの集計（作った時間帯で区切っている）とも食い違わない。
   時刻らしい数（UNIX 秒）と「今」を同じだけずらす */
const CLOCK = META && (META.demo || META.scope);
const SHIFT = CLOCK ? CLOCK.offset + new Date(META.scope ? META.scope.from*1000 : Date.now()).getTimezoneOffset() * 60 : 0;
const nowMs = () => Date.now() + SHIFT * 1000, today0 = () => new Date(nowMs());
function shiftTimes(x){ if (!SHIFT) return x;
  const walk = v => Array.isArray(v) ? v.map(walk) : v && typeof v === "object" ? Object.fromEntries(Object.entries(v).map(([k, y]) => [k, walk(y)])) : typeof v === "number" && v > 1e9 && v < 4e9 ? v + SHIFT : v;
  return walk(x); }
[DATA, WEEKS, MONTHS, META, GENERATED] = [shiftTimes(DATA), shiftTimes(WEEKS), shiftTimes(MONTHS), shiftTimes(META), shiftTimes(GENERATED)];
const LIVE = __LIVE__; // kiroku serve で開いたとき true
const PROMPT_RUNES = __PROMPT_RUNES__; // HTML に入れるプロンプトの長さ（core.PromptRunes）
const REPLY_RUNES = __REPLY_RUNES__; // HTML に入れる応答の長さ（core.ReplyRunes）
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
             week: mondayOf(today0()), month: monthOf(today0()), mode: store.get("mode", "week"), flowUser: !!store.get("flowUser", false),
             hidden: new Set(), sel: null, back: [], q: "", animate: true };

