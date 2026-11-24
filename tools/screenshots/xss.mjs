// 悪意のある文字列を仕込んだ履歴（hostile.py）の HTML を開いて操作し、スクリプトが動かないこと・要素が差しこまれないことを確かめる（CI の e2e）。
//   python3 tools/screenshots/hostile.py /tmp/h && TZ=UTC go run ./tools/html --no-open --sources claude --claude-root /tmp/h/fx/projects --week 2026-10-05 -o /tmp/xss.html
//   node tools/screenshots/xss.mjs /tmp/xss.html
import { chromium } from "playwright";
import path from "node:path";

const html = process.argv[2];
if (!html) { console.error("使い方: node xss.mjs <xss.html>"); process.exit(2); }

const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {}); // CHROMIUM: Playwright の Chromium を取ってこられない環境向け
const ctx = await b.newContext({ viewport: { width: 1500, height: 1000 }, locale: "en-US", timezoneId: "UTC" });
ctx.setDefaultTimeout(3000);
const p = await ctx.newPage();
const dialogs = [], errors = [], external = [];
p.on("dialog", async d => { dialogs.push(d.message()); await d.dismiss(); });
p.on("pageerror", e => errors.push(e.message));
p.on("request", r => { if (!/^(file|data|blob):/.test(r.url())) external.push(r.url()); });
await p.addInitScript(() => { window.__hits = []; window.alert = x => { window.__hits.push(String(x)); }; });
// 画面が読む localStorage の鍵（kiroku:…）を覚えておく（あとで、どの鍵にも壊れた値を入れて開き直す）
await p.addInitScript(() => { window.__lsKeys = new Set(); const gi = Storage.prototype.getItem;
  Storage.prototype.getItem = function (k) { if (typeof k === "string" && k.startsWith("kiroku:")) window.__lsKeys.add(k); return gi.call(this, k); }; });
// 同じ働きのものを 1 度だけ押すための鍵（描き直すと要素が替わるので、要素そのものでなく中身で見分ける）。どの切り替えの、どの選択肢の中のものかも入れる
await p.addInitScript(() => { window.__keyOf = el => { const chs = []; for (let x = el.closest("[data-ch]"); x; x = x.parentElement && x.parentElement.closest("[data-ch]")) chs.push(x.dataset.ch);
  return [el.tagName, el.getAttribute("class") || "", el.getAttribute("aria-label") || "", ...[...el.attributes].filter(a => a.name.startsWith("data-")).map(a => `${a.name}=${a.value}`),
    (el.textContent || "").trim().slice(0, 80), ...chs, (el.closest(".ikg, .inscol") || {}).className || ""].join("|"); }; });
let failed = 0;
await p.goto("file://" + path.resolve(html));
await p.waitForTimeout(300);
// 保存した HTML には、AI を動かす操作がない（kiroku は AI を動かさない）。あるのはプロンプトのコピーだけ
{ const n = await p.locator(".ai-action, #rptrun, #ai-review-dialog").count();
  if (n) { failed++; console.log(`  FAIL saved HTML shows AI run controls: ${n}`); } else console.log("  ok   saved HTML shows no AI run controls"); }

const check = async (step, pg = p) => {
  const r = await pg.evaluate(() => ({
    hits: window.__hits.splice(0),
    injected: [...document.querySelectorAll('img[src="x"], svg[onload], [onerror], [onload], [onclick], [onmouseover]')].map(e => e.outerHTML.slice(0, 120)),
    scripts: document.querySelectorAll("script").length,
    badHref: [...document.querySelectorAll("a[href]")].map(a => a.getAttribute("href")).filter(h => !/^(https?:|file:|history\?|#)/.test(h) || /^file:\/\/\/[\/\\]/.test(h)) // file:////host（UNC）も不可
      // ブラウザーが解いたあとの URL でも確かめる（/a/..//host のような . と .. で、開くときに //host になるもの）
      .concat([...document.querySelectorAll('a[href^="file:"]')].map(a => a.href).filter(h => !/^file:\/\/\/(?![\/\\])/.test(h))), // new URL の pathname は先頭の // を 1 つにまとめるので、href の文字列で見る
  }));
  if (r.hits.length || r.injected.length || r.scripts !== 1 || r.badHref.length) { failed++; console.log(`  FAIL ${step}: ${JSON.stringify(r)}`); }
  else console.log(`  ok   ${step}`);
};
// 押す前に画面の中へスクロールし、見えるかを確かめる。見えない（隠れた・たたまれた）ものは押せないので、確かめられなかったことになる
const reach = async loc => { if (!(await loc.isVisible())) return false; await loc.scrollIntoViewIfNeeded(); return loc.isVisible(); };
// 押すとほかの画面に移るものは、押すたびに確かめて Esc で戻す。
// 要素がないのはかまわない（この履歴にはない）。あるのに押せないものは、確かめられないまま緑にならないよう NG にする
const clickAll = async (sel, step, after) => {
  const n = await p.locator(sel).count();
  if (!n) { console.log(`  none ${step} (no such element in this file)`); return; }
  for (let i = 0; i < n; i++) {
    const loc = p.locator(sel).nth(i);
    try {
      if (!(await reach(loc))) { failed++; console.log(`  FAIL ${step} ${i + 1}: exists but is not visible, so it was not checked`); }
      else { await loc.click({ force: true }); await p.waitForTimeout(100); await check(`${step} ${i + 1}`); if (after) await after(); }
    } catch (e) { failed++; console.log(`  FAIL ${step} ${i + 1}: could not be checked: ${e.message.split("\n")[0]}`); }
    await p.keyboard.press("Escape"); await p.waitForTimeout(50);
  }
};

await check("load");
// push の記録（リモート追跡ブランチの reflog）が、悪意のある ref の名前と、__proto__・constructor"><img …> のプロジェクト名で読めている（確かめる対象に入っている）
{ const r = await p.evaluate(() => { const ps = META.push || []; return {n: ps.length, projects: [...new Set(ps.map(x => x.project))], refs: [...new Set(ps.map(x => x.ref))]}; });
  const ok = r.n >= 6 && r.projects.includes("__proto__") && r.projects.some(x => x.startsWith('constructor"><img')) && r.refs.some(x => x.includes("<svg/onload=alert(40)>"));
  if (!ok) { failed++; console.log(`  FAIL hostile push records are not in the page (the test would not cover them): ${JSON.stringify(r)}`); } else console.log(`  ok   hostile push records are in the page (${r.n})`); }
// 見直す候補の中身（繰り返したプロンプトの文など）は、ダイアログにだけ出る。どれも開いて確かめる
const ids = await p.evaluate(() => [...new Set((lastFlags || []).map(f => GOTO[f.k] || f.k))]); // 期間の行の一覧は出さないので、候補のダイアログを 1 つずつ開く
if (!ids.includes("repeats")) { failed++; console.log("  FAIL the repeated-prompt finding is not shown (the test would not cover it)"); }
for (const id of ids) { await p.evaluate(id => openWorth(id), id); await p.waitForTimeout(100); await check(`worth a look ${id}`); await p.keyboard.press("Escape"); await p.waitForTimeout(50); }
const runs = p.locator(".run");
for (let i = 0; i < await runs.count(); i++) await runs.nth(i).hover({ force: true });
await check("hover sessions");
await clickAll(".run", "session", async () => {
  const g = "#panel [data-git], #panel [data-pr], #panel [data-push]";
  for (let j = 0; j < await p.locator(g).count(); j++) {
    try { await p.locator(g).nth(j).scrollIntoViewIfNeeded(); await p.locator(g).nth(j).click({ force: true }); await p.waitForTimeout(100); await check("session → git"); }
    catch (e) { failed++; console.log(`  FAIL session → git ${j + 1}: could not be checked: ${e.message.split("\n")[0]}`); }
    await p.locator("#back").click().catch(() => {}); // 詳細から戻る（戻るボタンのない詳細もある）
  }
});
await clickAll(".gc", "commit");
await clickAll(".gm", "pull request");
// サマリー（概要のカード）。履歴の名前が文字・title・aria-label・ツールチップ・data-* に入るので、どの切り替えも押して確かめる。
// パネルとカードの多くは切り替えで選んだものだけを見せ、「内訳」と、その中の「その他のグラフ」はたたんである。見えないものは押せないので、
// 切り替えの選択肢を 1 つずつ選び（量 × 割る数も）、「内訳」と「その他のグラフ」を開いて、そのたびに見えるようになったものを押して確かめる。
// どの状態でも見えなかったもの（あるのに確かめられなかったもの）は、最後に NG にする。
// style には決まった値（色の変数・数）だけが入る（履歴の文字は入らない）
const SUM_TARGETS = [ // [名前, 押すもの（#review の中）, 押し方]
  ["formula", ".fxb", "click"],                       // 式（ƒx）。開いたままにして、式の中の印やボタンも見えるようにする
  ["help", ".hb[data-help]", "click"],                // 「?」の説明
  ["advice", ".fmark, .iwlink", "click"],             // 見直す候補（Worth a look）のダイアログ
  ["breakdown", "[data-metric]", "click"],            // 内訳のダイアログ
  ["point", "[data-sid]", "click"],                   // いちばん変えたファイルのセッション
  ["tooltip", "[data-tip]", "hover"],                 // ツールチップ
];
// 押すと描き直す切り替え（カードの指標など）
const SUM_SWITCHES = ["skk", "wfk", "wk", "where"];
const summaryCheck = async label => {
  const sw = p.locator("#view button[data-v=sum]"); if (!(await sw.count())) { failed++; console.log(`  FAIL summary ${label}: no view switch`); return; }
  const seen = new Set(), hidden = new Map(), count = {}, badStyles = new Set();
  const onSummary = async () => {
    if ((await sw.getAttribute("aria-pressed")) !== "true") { await sw.click(); await p.waitForTimeout(200); }
    // たたんだ「内訳」と、その中の「その他のグラフ」を開く（外から順に。たたんだ箱の中の箱は見えないので押せない）
    for (const sel of ["#review details.insbrk:not([open]) > summary"]) {
      const more = p.locator(sel);
      if (await more.count()) { await more.first().scrollIntoViewIfNeeded(); await more.first().click(); await p.waitForTimeout(100); }
    }
  };
  const close = async () => { await p.keyboard.press("Escape"); await p.waitForTimeout(50); await p.keyboard.press("Escape"); await p.waitForTimeout(50); await onSummary(); };
  // 今の状態で見えるものを押す（まだ押していないものだけ）。式を開くと中にボタンが増えるので、増えなくなるまで繰り返す
  // 隠れている（hidden・display:none・たたんだ中）ものは押せないので、あとの状態で見えるまで残しておく。
  // 大きさが 0 のもの（記録のない棒のツールチップなど）は、マウスを載せたときと同じ pointerover を送って出す
  const sweep = async state => {
    for (let grew = true; grew;) { // 式を開くと中に押すものが増えるので、増えなくなるまで
      grew = false;
      for (const [name, sel, how] of SUM_TARGETS) for (;;) {
        // 押すと描き直されることがある（セッションを開いて閉じたときなど）ので、1 つ押すたびに数え直す
        const all = p.locator(`#review ${sel}`);
        const info = await all.evaluateAll(els => els.map(el => ({key: window.__keyOf(el), shown: el.checkVisibility({checkVisibilityCSS: true, visibilityProperty: true})})));
        for (const {key, shown} of info) if (!shown && !seen.has(key) && !hidden.has(key)) hidden.set(key, `${name} (${state})`);
        const i = info.findIndex(x => x.shown && !seen.has(x.key)); if (i < 0) break;
        const loc = all.nth(i);
        seen.add(info[i].key); grew = true;
        const step = `summary ${label} ${name} ${count[name] = (count[name] || 0) + 1}`;
        try {
          await loc.scrollIntoViewIfNeeded();
          if (how === "hover") {
            if (await loc.isVisible()) await loc.hover({ force: true }); else await loc.dispatchEvent("pointerover", { bubbles: true });
            await p.waitForTimeout(30); await check(step); continue;
          }
          await loc.click({ force: true }); await p.waitForTimeout(100); await check(step);
        } catch (e) { failed++; console.log(`  FAIL ${step}: could not be checked: ${e.message.split("\n")[0]}`); }
        if (name !== "formula") await close();
      }
    }
    for (const v of await p.evaluate(() => [...document.querySelectorAll("#review .insight [style]")].map(e => e.getAttribute("style")).filter(v => !/^(--a:[\d.]+|--rs: [\d]+;|(fill|background):var\(--[\w-]+\)|--c:var\(--(a\d+|other)\)|(width:[\d.]+%|flex:[\d.]+);background:var\(--[\w-]+\))$/.test(v)))) badStyles.add(v);
  };
  // パネルとカードの切り替え（描き直さずに選んだものだけを見せる）。量を選ぶたびに、その量で選べる割る数をどれも選ぶ
  const panels = async state => {
    for (const g of await p.evaluate(() => [...document.querySelectorAll('#review .iksw[data-dim="n"]')].map(x => x.dataset.sw))) {
      for (const v of await p.evaluate(g => [...document.querySelectorAll(`#review .iksw[data-sw="${g}"] [data-v]`)].map(x => x.dataset.v), g)) {
        const b = p.locator(`#review .iksw[data-sw="${g}"] [data-v="${v}"]`);
        if (!(await reach(b))) { failed++; console.log(`  FAIL summary ${label} switch ${g}=${v}: exists but is not visible`); continue; }
        await b.click(); await p.waitForTimeout(80); await check(`summary ${label} switch ${g}=${v}`);
        const dg = p.locator(`#review .iksw[data-sw="${g}"]`).locator('xpath=ancestor::*[contains(concat(" ", normalize-space(@class), " "), " ikg ")][1]').locator('.iksw[data-dim="d"]');
        const ds = (await dg.count()) ? await dg.locator("[data-v]").evaluateAll(xs => xs.map(x => x.dataset.v)) : [];
        if (!ds.length) { await sweep(`${state}, ${g}=${v}`); continue; }
        for (const d of ds) {
          const db = dg.locator(`[data-v="${d}"]`);
          if (await db.evaluate(e => e.hidden)) continue; // その量で数字のない割る数は隠してある（押せるものだけを確かめる）
          if (!(await reach(db))) { failed++; console.log(`  FAIL summary ${label} switch ${g}=${v} ÷ ${d}: exists but is not visible`); continue; }
          await db.click(); await p.waitForTimeout(80); await check(`summary ${label} switch ${g}=${v} ÷ ${d}`);
          await sweep(`${state}, ${g}=${v} ÷ ${d}`);
        }
      }
    }
  };
  // 描き直す切り替えを 1 つ選ぶ（選べなければ NG）
  const pick = async (attr, v) => {
    const b = p.locator(`#review [data-${attr}="${v}"]`).first();
    if (!(await reach(b))) { failed++; console.log(`  FAIL summary ${label} switch ${attr}=${v}: exists but is not visible`); return false; }
    await b.click(); await p.waitForTimeout(200); await onSummary(); await check(`summary ${label} switch ${attr}=${v}`); return true;
  };
  const vals = attr => p.evaluate(a => [...new Set([...document.querySelectorAll(`#review [data-${a}]`)].map(x => x.getAttribute(`data-${a}`)))], attr);

  await onSummary(); await check(`summary ${label}`);
  // 出力のパネルに Pushes の選択肢がある（push の数・グラフ・ツールチップを、切り替えを押して確かめる）
  if (!(await p.locator('#review .iksw[data-sw="out"] [data-v="push"]').count())) { failed++; console.log(`  FAIL summary ${label}: Output has no Pushes switch (push records are not checked)`); }
  await sweep("default");
  await panels("default");
  for (const attr of SUM_SWITCHES) for (const v of await vals(attr)) { if (await pick(attr, v)) await sweep(`${attr}=${v}`); }
  for (const [name] of SUM_TARGETS) if (!count[name]) console.log(`  none summary ${label} ${name} (no such element in this file)`);
  for (const [key, where] of hidden) if (!seen.has(key)) { failed++; console.log(`  FAIL summary ${label} ${where}: exists but was never visible in any switch state, so it was not checked: ${key.slice(0, 160)}`); }
  if (badStyles.size) { failed++; console.log(`  FAIL summary ${label} style values: ${JSON.stringify([...badStyles].slice(0, 5))}`); } else console.log(`  ok   summary ${label} style values`);
  console.log(`  ---- summary ${label}: checked ${seen.size} targets (${Object.entries(count).map(([k, n]) => `${k} ${n}`).join(", ")})`);
  await p.click("#view button[data-v=cal]"); await p.waitForTimeout(150);
};
await summaryCheck("week");
await clickAll("#review .card", "summary card");
// 内訳のダイアログ（上の帯とサマリーの数字）。プロジェクト名やセッション名が棒やカードに入るので、どれも開いて確かめる
await clickAll("#kpis [data-metric]", "breakdown", async () => { // 帯の数（サマリーの数は summaryCheck で押す） // 内訳の中のコミット・push・PR の行は、詳細を開く
  const g = "#md [data-git], #md [data-pr], #md [data-push]";
  if (await p.locator(g).count()) { await p.locator(g).first().click({ force: true }); await p.waitForTimeout(100); await check("breakdown → git"); }
});
for (const v of ["source", "project"]) { await p.selectOption("#cb2", v); await check(`color by ${v}`); }
for (const q of ["<img", "alert", "__META__", "javascript", '"><svg']) {
  await p.fill("#q", q); await p.waitForTimeout(200); await check(`search ${q}`);
  await clickAll("#sres .srow", `search result ${q}`);
}
await p.fill("#q", "");
// 月の表示は、月のある HTML でだけ確かめる（hostile.py は 2026-10-05 の 1 週だけで、--week で書き出すので月の切り替えがない）
if (!(await p.evaluate(() => META.scope && META.scope.mode === "week"))) { await p.click("#mode button[data-v=month]"); await p.waitForTimeout(200); await check("month"); await summaryCheck("month"); }
else console.log("  none month (this file holds a single week: no month switch)");

// AI に渡すプロンプト（セッションの振り返り・日報）で、履歴の改行から kiroku の行に見える偽の行を作れない（日報は、週の表示の週の初日。hostile.py のセッションはその日にある）
await p.evaluate(() => { if (st.mode !== "week"){ st.mode = "week"; render(); } }); // ボタンが出ない HTML もあるので、表示を直接切り替える
{
  const forged = await p.evaluate(() => {
    const texts = DATA.map(s => sessionPrompt(s, 1, 1));
    texts.push(reportPrompt(st.week));
    return texts.flatMap(t => t.split(/[\n\r\v\f\u0085\u2028\u2029]/).filter(l => /^\s*(- (History file|Project|Agent|Title): FORGED_|## FORGED_|- FORGED_)|^- 11:01 \[Interrupted\]$/.test(l)));
  });
  if (forged.length) { failed++; console.log(`  FAIL forged lines in AI prompts: ${JSON.stringify(forged)}`); } else console.log("  ok   no forged lines in AI prompts");
}

// 繰り返したプロンプト（ただの文字）は、日報のプロンプトに書いたとおりに入る（HTML として読んで、<…> や &lt; を消したり戻したりしない）
{
  const line = await p.evaluate(() => reportPrompt(st.week).split("\n").find(l => /^  - ".*" \(\d+ times in \d+ sessions\)$/.test(l)) || "");
  if (!line) { failed++; console.log("  FAIL no repeated prompt in the daily report prompt"); }
  else if (!line.includes('"fix: <img src=x onerror=alert(20)>')) { failed++; console.log(`  FAIL repeated prompt text changed in the daily report prompt: ${JSON.stringify(line)}`); }
  else console.log("  ok   repeated prompt text kept as written in the daily report prompt");
}

// 空白のない長い語（繰り返したプロンプトの URL など）で、スマートフォンの幅でも横にはみ出さない
await p.setViewportSize({ width: 390, height: 844 }); await p.waitForTimeout(200);
const [sw, iw] = await p.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
if (sw > iw) { failed++; console.log(`  FAIL overflows sideways at 390px: ${sw} > ${iw}`); } else console.log("  ok   no sideways overflow at 390px");

// localStorage を書き換えられていても（同じ PC のほかのページ・拡張機能・手で）、開いた画面にスクリプトや要素が入らず、
// Object.prototype が汚れず、ページのエラーにならない。画面が読むどの kiroku: の鍵にも、壊れた値を 1 つずつ入れて開き直す
{
  const keys = [...new Set([...(await p.evaluate(() => [...window.__lsKeys])),
    ...(await p.evaluate(() => Object.keys(localStorage).filter(k => k.startsWith("kiroku:"))))])].sort();
  const VALUES = [["__proto__", JSON.stringify("__proto__")], ["constructor", JSON.stringify("constructor")], ["raw __proto__", "__proto__"],
    ["html", JSON.stringify('"><img src=x onerror=alert(50)><svg onload=alert(51)>')], ["raw html", '<img src=x onerror=alert(52)>'],
    ["proto object", '{"__proto__":{"polluted":1}}'], ["constructor object", '{"constructor":{"prototype":{"polluted":1}}}'], ["array", '["__proto__",{"polluted":1}]'],
    ["invalid JSON", "{"], ["unknown switch value", JSON.stringify("no-such-choice")], ["number", "1e309"], ["long", JSON.stringify("A".repeat(200 * 1024))]];
  if (keys.length < 10) { failed++; console.log(`  FAIL localStorage tamper: found only ${keys.length} kiroku: keys: ${JSON.stringify(keys)}`); }
  for (const [name, raw] of VALUES) {
    const pg = await ctx.newPage(), errs = [], dl = [];
    pg.on("pageerror", e => errs.push(e.message)); pg.on("dialog", async d => { dl.push(d.message()); await d.dismiss(); });
    await pg.addInitScript(() => { window.__hits = []; window.alert = x => { window.__hits.push(String(x)); }; });
    await pg.addInitScript(([keys, raw]) => { let n = 0; for (const k of keys) { try { localStorage.setItem(k, raw); n++; } catch (e) {} } window.__tampered = n; }, [keys, raw]);
    try {
      await pg.goto("file://" + path.resolve(html)); await pg.waitForTimeout(300);
      const sw = pg.locator("#view button[data-v=sum]");
      if ((await sw.getAttribute("aria-pressed")) !== "true") { await sw.click(); await pg.waitForTimeout(300); }
      await check(`localStorage ${name}`, pg);
      const r = await pg.evaluate(() => ({polluted: ({}).polluted, objPolluted: Object.prototype.polluted, tampered: window.__tampered, summary: !!document.querySelector("#review .insh .ikg")}));
      if (r.polluted !== undefined || r.objPolluted !== undefined || !r.tampered || !r.summary || errs.length || dl.length) { failed++; console.log(`  FAIL localStorage ${name}: ${JSON.stringify({...r, errors: errs.slice(0, 3), dialogs: dl})}`); }
      else console.log(`  ok   localStorage ${name}: no pollution, no page error, summary shown (${r.tampered} keys)`);
    } catch (e) { failed++; console.log(`  FAIL localStorage ${name}: could not be checked: ${e.message.split("\n")[0]}`); }
    await pg.evaluate(() => localStorage.clear()).catch(() => {});
    await pg.close();
  }
}

if (dialogs.length) { failed++; console.log(`  FAIL dialogs: ${JSON.stringify(dialogs)}`); }
if (errors.length) { failed++; console.log(`  FAIL page errors: ${JSON.stringify(errors)}`); }
if (external.length) { failed++; console.log(`  FAIL external requests: ${JSON.stringify(external)}`); }
await b.close();
console.log(failed ? `\n${failed} 件 NG` : "\nすべて OK");
process.exit(failed ? 1 : 0);
