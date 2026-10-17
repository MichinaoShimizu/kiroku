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
const aiAgents = () => LIVE && META && Array.isArray(META.aiAgents) ? META.aiAgents.map(String) : []; // このコンピューターにコマンドがあるエージェント
const aiOn = () => LIVE && !!(META && META.aiRun); // kiroku serve が AI の実行を許しているか（--no-ai でなく、ループバックで待ち受けている）。保存した HTML ではいつも false
const aiRun = () => aiOn() && aiAgents().length > 0; // 使えるエージェントもあって、画面から AI を動かせる
// 公開のデモ（KIROKU_DEMO で作ったダミーデータの HTML）では、「Analyze with AI」でプロンプトを見せるだけ（実行はしない）
const aiDemo = () => !LIVE && !!(META && META.demo);
const PROMPT_RUNES = __PROMPT_RUNES__; // HTML に入れるプロンプトの長さ（core.PromptRunes）
const REPLY_RUNES = __REPLY_RUNES__; // HTML に入れる応答の長さ（core.ReplyRunes）
const AGENTS = __AGENTS__, AG_WARN = __AGENT_WARN__; // エージェントの名前・色・頭文字（core.Agents）と、エージェントに使わない警告の色
const RECORDS = __RECORDS__; // エージェントごとに、履歴から読めるもの（core.Records）。ないものは「0」ではなく「記録されていない」と出す
const records = (src, what) => (RECORDS[src] || []).includes(what);
// 期間 [ws, we) のセッションのエージェントのどれも what を記録しないなら、それらのエージェントの名前。どれかが記録するか、セッションがなければ null
function unrecorded(ws, we, what){ const srcs = [...new Set(DATA.filter(s => s.end >= ws && s.start < we).map(s => s.source))];
  return srcs.length && !srcs.some(x => records(x, what)) ? srcs : null; }
const notRec = srcs => t("Not recorded in {list} history", {list: srcs.join(LSEP())});
const SLOTS = 8;
const $ = s => document.querySelector(s);
const store = { get(k,d){ try { const v = localStorage.getItem("kiroku:"+k); return v == null ? d : JSON.parse(v); } catch(e){ return d; } },
                set(k,v){ try { localStorage.setItem("kiroku:"+k, JSON.stringify(v)); } catch(e){} } };
/* kiroku serve の鍵。?key= の中継のページが localStorage（このオリジン＝このポートだけ）にしまったものを、
   kfetch が X-Kiroku-Key ヘッダーに付ける。Cookie と違って、ほかのポートのサーバーには送られない。
   JSON ではなくそのまま入れてあるので store ではなく直接読む。送り先はこのサイトの中だけ */
const serveKey = () => { try { const k = localStorage.getItem("kiroku:key"); return k && /^[0-9a-f]{64}$/.test(k) ? k : null; } catch(e){ return null; } };
function kfetch(url, opts = {}){
  const u = new URL(url, location.href);
  if (u.origin !== location.origin) return Promise.reject(new Error("kfetch: not this site"));
  const headers = new Headers(opts.headers || {}), k = serveKey();
  if (k) headers.set("X-Kiroku-Key", k);
  return fetch(u.href, {cache: "no-store", ...opts, headers});
}
/* 画面の言語。?lang=、見る人が選んだもの、kiroku html / serve の --lang（$KIROKU_LANG）、ブラウザの言語の順。
   どれでもなければ英語。切り替えるときは ?lang= を付けて開き直すので、localStorage が使えなくても効く */
const LANGS = ["en", "ja"];
const LANG = (() => { let q = null; try { q = new URLSearchParams(location.search).get("lang"); } catch(e){}
  const nav = () => { try { return (navigator.languages && navigator.languages[0] || navigator.language || "").toLowerCase().startsWith("ja") ? "ja" : "en"; } catch(e){ return "en"; } };
  const v = [q, store.get("lang", null), __LANG__].find(x => LANGS.includes(x)) || nav(); return LANGS.includes(v) ? v : "en"; })();
const JA = LANG === "ja";
/* AI に渡すプロンプトは英語のまま。inEnglish の中では、t・plural・日付の小物が英語を返す */
let EN_ONLY = 0;
const isJA = () => JA && !EN_ONLY;
function inEnglish(f){ EN_ONLY++; try { return f(); } finally { EN_ONLY--; } }
const LOC = () => isJA() ? "ja-JP" : "en-US";
const DOW_EN = ["Sun","Mon","Tue","Wed","Thu","Fri","Sat"], DOW_JA = ["日","月","火","水","木","金","土"];
const DOW = new Proxy(DOW_EN, {get: (a, k) => (isJA() ? DOW_JA : DOW_EN)[k]}); // DOW[i] がその時の言語の曜日
const dow = i => DOW[i];
const wkc = i => i === 6 ? " sat" : i === 0 ? " sun" : ""; // 曜日（getDay）から、土日の色のクラス
const plural = (n, one, many) => isJA() ? `${n} ${t(one)}` : `${n} ${n === 1 ? one : many || one + "s"}`; // 日本語は数と名詞（単数の形を辞書で引く）
// hh: 週のカレンダーの 1 時間の高さ（render が作業の時間帯に合わせて決める。ズームはない。以前の版が保存した "zh" は読まない）
// 画面はカレンダーだけ。以前の版が保存した "view"（journey・project）と、Year in review の "yrOpt" は読まない
const st = { hh: 44, colorBy: store.get("colorBy", "source"), theme: store.get("theme", "dark") === "light" ? "light" : "dark", // 既定は、エージェントごとの色分けとダーク（ライトを選んだ人はライトのまま）
             week: mondayOf(today0()), month: monthOf(today0()), mode: store.get("mode", "week"), flowUser: !!store.get("flowUser", false),
             hidden: new Set(), sel: null, back: [], q: "", animate: true };

