// 埋め込んだデータ（Go が JSON を入れる）、デモや期間だけのファイルの時刻ずらし、保存（localStorage）、画面の状態 st
let DATA = __DATA__;
let WEEKS = __WEEKS__;
let MONTHS = __MONTHS__;
let META = __META__;
let GENERATED = __GEN__;
// applyData は kiroku serve から届いた新しいデータに入れかえる（上の名前を書きかえるので、ここに置く）
// kiroku serve の data.json には依頼文と応答の文が入っていないので（画面を早く出すため）、描いたあとで texts.json を読んで入れる
// 前のデータで文がそろっていた会話のうち、依頼と応答の時刻と長さが変わっていないものは、前の文を使う（自動更新のたびに全部を読み直さない）。
// 足りない会話があるときだけ texts.json を読む
function applyData(j){ const prev = new Map(DATA.map(s => [s.id, s])), next = j.sessions || [];
  let missing = false; if (LIVE) next.forEach(s => { if (!carryTexts(prev.get(s.id), s)) missing = true; });
  DATA = next; WEEKS = j.weeks || {}; MONTHS = j.months || {}; META = j.meta; GENERATED = j.generated; localizeTitles(DATA); if (st.sel && !DATA.some(s => s.id === st.sel)) st.sel = null;
  if (LIVE){ TEXTS = !missing; if (missing) setTimeout(loadTexts, 0); } }
// HAS_TEXT は、文の入っているセッション
const HAS_TEXT = new WeakSet();
// carryTexts は、前のデータの同じ会話 o の文を s に写す。依頼の数・時刻・切った長さ、応答の有無・時刻・長さのどれかが違えば写さない（false）
function carryTexts(o, s){
  if (!o || !HAS_TEXT.has(o) || !Array.isArray(s.prompts) || o.prompts.length !== s.prompts.length) return false;
  const same = (a, b) => a.t === b.t && (a.len || 0) === (b.len || 0);
  for (let k = 0; k < s.prompts.length; k++){ const a = o.prompts[k], b = s.prompts[k];
    if (!same(a, b) || !a.reply !== !b.reply || (a.reply && !same(a.reply, b.reply))) return false; }
  s.prompts.forEach((b, k) => { const a = o.prompts[k]; b.text = a.text; if (b.reply) b.reply.text = a.reply.text; });
  HAS_TEXT.add(s); return true;
}
// TEXTS は、依頼文と応答の文がそろっているか（HTML のファイルは埋め込んであるので、はじめから true）
let TEXTS = true;
// loadTexts は、文のないセッションの文を texts.json で読んで DATA に入れ、描き直す。少なければ（自動更新で変わった会話）その会話の分だけ頼む。
// 読んでいる間にデータが入れかわったら（自動更新）捨てる（入れかえた側が読み直す）。読めなければ少しあとで読み直す
async function loadTexts(){
  const want = DATA, ids = [...new Set(DATA.filter(s => !HAS_TEXT.has(s)).map(s => s.id))];
  const q = ids.length && ids.length <= 50 ? "?" + ids.map(id => "id=" + encodeURIComponent(id)).join("&") : "";
  try { const r = await kfetch("texts.json" + q); if (!r.ok) throw new Error(String(r.status)); const j = await r.json();
    if (DATA !== want) return;
    if (!applyTexts(j)) throw new Error("bad"); render();
  } catch(e){ if (DATA === want) setTimeout(() => { if (DATA === want && !TEXTS) loadTexts(); }, 3000); }
}
// applyTexts は texts.json の文を、文のないセッションの依頼と応答に入れる。ID と依頼の数が合うものだけに入れ、文字列でないものは入れない
function applyTexts(j){
  const L = j && j.sessions; if (!Array.isArray(L)) return false;
  const by = new Map(); L.forEach(x => { if (Array.isArray(x) && typeof x[0] === "string" && Array.isArray(x[1])){ if (!by.has(x[0])) by.set(x[0], []); by.get(x[0]).push(x[1]); } });
  DATA.forEach(s => { if (HAS_TEXT.has(s)) return;
    const ps = (by.get(s.id) || []).find(ps => ps.length === s.prompts.length); if (!ps) return;
    ps.forEach((pr, k) => { const p = s.prompts[k]; if (!Array.isArray(pr)) return;
      if (typeof pr[0] === "string") p.text = pr[0];
      if (p.reply && typeof pr[1] === "string") p.reply.text = pr[1]; });
    HAS_TEXT.add(s); });
  localizeTitles(DATA);
  DATA = DATA.slice(); SQ = new WeakMap(); // 文から作る覚え（検索・繰り返したプロンプト）は、DATA と SQ ごと作り直す
  TEXTS = true; return true;
}
/* デモ（GitHub Pages）と、tools/html の --week / --month で書き出した期間だけのファイルは、作った人の時間帯の時計で見せる。
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
const AGENTS = __AGENTS__; // エージェントの名前・色（--a<slot>）・頭文字（core.Agents）
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
/* kiroku serve の鍵。?code= の中継のページが localStorage（このオリジン＝このポートだけ）にしまったものを、
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
/* 画面の言語。?lang=、tools/html / serve の --lang（$KIROKU_LANG）、ブラウザの言語の順。どれでもなければ英語。
   以前の版の切り替えボタンで選んだ言語（localStorage の lang）は使わない（ボタンがないので、残ると戻せない） */
const LANGS = ["en", "ja"];
const LANG = (() => { let q = null; try { q = new URLSearchParams(location.search).get("lang"); } catch(e){}
  const nav = () => { try { return (navigator.languages && navigator.languages[0] || navigator.language || "").toLowerCase().startsWith("ja") ? "ja" : "en"; } catch(e){ return "en"; } };
  const v = [q, __LANG__].find(x => LANGS.includes(x)) || nav(); return LANGS.includes(v) ? v : "en"; })();
const JA = LANG === "ja";
/* AI に渡すプロンプトは英語のまま。inEnglish の中では、t・plural・日付の小物が英語を返す */
let EN_ONLY = 0;
const isJA = () => JA && !EN_ONLY;
function inEnglish(f){ EN_ONLY++; try { return f(); } finally { EN_ONLY--; } }
const LOC = () => isJA() ? "ja-JP" : "en-US";
const DOW_EN = ["Sun","Mon","Tue","Wed","Thu","Fri","Sat"], DOW_JA = ["日","月","火","水","木","金","土"];
const DOW = new Proxy(DOW_EN, {get: (a, k) => (isJA() ? DOW_JA : DOW_EN)[k]}); // DOW[i] がその時の言語の曜日
const dow = i => DOW[i];
const wkc = i => i % 6 === 0 ? " we" : ""; // 曜日（getDay）から、土日のクラス（土曜も日曜も同じ中立の地）
const plural = (n, one, many) => isJA() ? `${n} ${t(one)}` : `${n} ${n === 1 ? one : many || one + "s"}`; // 日本語は数と名詞（単数の形を辞書で引く）
// hh: 週のカレンダーの 1 時間の高さ（render が作業の時間帯に合わせて決める。ズームはない。以前の版が保存した "zh" は読まない）
// 開くたびに今週のカレンダーから始める。表示（カレンダー・サマリー）と週・月は覚えない（以前の版が保存した "view"・"mode" も読まない）
const st = { hh: 44, colorBy: store.get("colorBy", "project") === "source" ? "source" : "project", /* 既定はプロジェクト（エージェントで分けると、ほとんどの帯が同じ 1 色になりがち） */
             week: mondayOf(today0()), month: monthOf(today0()), mode: "week", view: "cal",
             hidden: new Set(), gopen: new Set(), sel: null, back: [], q: "", animate: true, insWf: store.get("insWf", "cost"), insSk: store.get("insSk", "cost"), insSkMan: store.get("insSkMan", true), insWk: store.get("insWk", null), shade: store.get("mShade", null), insW: store.get("insW", "project"), insCo: store.get("insCo", null), early: store.get("calEarly", null) === true };

