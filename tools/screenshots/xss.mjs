// 悪意のある文字列を仕込んだ履歴（hostile.py）の HTML を開いて操作し、スクリプトが動かないこと・要素が差しこまれないことを確かめる（CI の e2e）。
//   python3 tools/screenshots/hostile.py /tmp/h && TZ=UTC go run . html --no-open --sources claude --claude-root /tmp/h/fx/projects --week 2026-10-05 -o /tmp/xss.html
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
await p.goto("file://" + path.resolve(html));
await p.waitForTimeout(300);

let failed = 0;
const check = async step => {
  const r = await p.evaluate(() => ({
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
// 押すとほかの画面に移るものは、押すたびに確かめて Esc で戻す
const clickAll = async (sel, step, after) => {
  const n = await p.locator(sel).count();
  for (let i = 0; i < n; i++) {
    try { await p.locator(sel).nth(i).click({ force: true }); await p.waitForTimeout(100); await check(`${step} ${i + 1}`); if (after) await after(); }
    catch (e) { console.log(`  skip ${step} ${i + 1}: ${e.message.split("\n")[0]}`); }
    await p.keyboard.press("Escape"); await p.waitForTimeout(50);
  }
};

await check("load");
// 見直す候補の中身（繰り返したプロンプトの文など）は、ダイアログにだけ出る。どれも開いて確かめる
if (!(await p.locator('.flagsum .flink[data-goto="repeats"]').count())) { failed++; console.log("  FAIL the repeated-prompt finding is not shown (the test would not cover it)"); }
await clickAll(".flagsum .flink", "worth a look", async () => { await p.keyboard.press("Escape"); await p.waitForTimeout(50); });
const runs = p.locator(".run");
for (let i = 0; i < await runs.count(); i++) await runs.nth(i).hover({ force: true });
await check("hover sessions");
await clickAll(".run", "session", async () => {
  const g = "#panel [data-git], #panel [data-pr], #panel [data-push]";
  for (let j = 0; j < await p.locator(g).count(); j++) {
    try { await p.locator(g).nth(j).click({ force: true }); await p.waitForTimeout(100); await check("session → git"); await p.locator("#back").click().catch(() => {}); } catch (e) {}
  }
});
await clickAll(".gc", "commit");
await clickAll(".gm", "pull request");
await clickAll("#review .card", "summary card");
// 内訳のダイアログ（上の帯とサマリーの数字）。プロジェクト名やセッション名が棒やカードに入るので、どれも開いて確かめる
await clickAll("[data-metric]", "breakdown", async () => { // 内訳の中のコミット・push・PR の行は、詳細を開く
  const g = "#md [data-git], #md [data-pr], #md [data-push]";
  if (await p.locator(g).count()) { await p.locator(g).first().click({ force: true }); await p.waitForTimeout(100); await check("breakdown → git"); }
});
for (const v of ["branch", "source", "project"]) { await p.selectOption("#cb2", v); await check(`color by ${v}`); }
for (const q of ["<img", "alert", "__META__", "javascript", '"><svg']) {
  await p.fill("#q", q); await p.waitForTimeout(200); await check(`search ${q}`);
  await clickAll("#sres .srow", `search result ${q}`);
}
await p.fill("#q", "");
try { await p.click("#mode button[data-v=month]"); await check("month"); } catch (e) { console.log("  skip month (not in this file)"); }

// AI に渡すプロンプト（セッションの振り返り・改善案・週報）で、履歴の改行から kiroku の行に見える偽の行を作れない
{
  const forged = await p.evaluate(() => {
    const texts = DATA.map(s => sessionPrompt(s, 1, 1));
    const {S, P} = period(); if (S){ texts.push(askPrompt(S, P, st.mode === "month")); if (typeof reportPrompt === "function") texts.push(reportPrompt(S, st.mode === "month")); }
    return texts.flatMap(t => t.split(/[\n\r\v\f\u0085\u2028\u2029]/).filter(l => /^\s*(- (History file|Project|Agent): FORGED_|## FORGED_|- FORGED_)|^- 11:01 \[Interrupted\]$/.test(l)));
  });
  if (forged.length) { failed++; console.log(`  FAIL forged lines in AI prompts: ${JSON.stringify(forged)}`); } else console.log("  ok   no forged lines in AI prompts");
}

// 空白のない長い語（繰り返したプロンプトの URL など）で、スマートフォンの幅でも横にはみ出さない
await p.setViewportSize({ width: 390, height: 844 }); await p.waitForTimeout(200);
const [sw, iw] = await p.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
if (sw > iw) { failed++; console.log(`  FAIL overflows sideways at 390px: ${sw} > ${iw}`); } else console.log("  ok   no sideways overflow at 390px");

if (dialogs.length) { failed++; console.log(`  FAIL dialogs: ${JSON.stringify(dialogs)}`); }
if (errors.length) { failed++; console.log(`  FAIL page errors: ${JSON.stringify(errors)}`); }
if (external.length) { failed++; console.log(`  FAIL external requests: ${JSON.stringify(external)}`); }
await b.close();
console.log(failed ? `\n${failed} 件 NG` : "\nすべて OK");
process.exit(failed ? 1 : 0);
