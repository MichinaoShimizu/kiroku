// docs/screenshot.png・docs/summary.png・docs/worth.png・docs/session.png・docs/og.png・docs/year.png・docs/year-params.png を撮る。gen.py と mkgit.py で作ったダミーデータの HTML を使う。
//   node capture.mjs <kiroku.html> <docs のディレクトリ>
// Playwright が必要（このディレクトリで npm ci --ignore-scripts と npx playwright install chromium。版は package.json で固定）。英語表示（en-US）・ライトテーマ（既定）・時刻は Asia/Tokyo・1440x900。
import { chromium } from "playwright";
import path from "node:path";
import fs from "node:fs/promises";

const [html, docs] = process.argv.slice(2);
const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, timezoneId: "Asia/Tokyo", locale: "en-US" });
const p = await ctx.newPage();
await p.goto("file://" + path.resolve(html));
await p.waitForTimeout(600);
await p.keyboard.press("ArrowLeft"); // 先週（記録がそろっている週）
await p.waitForTimeout(500);
await p.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); if (sc) sc.scrollTop = 8 * 44; });
await p.waitForTimeout(200);
await p.screenshot({ path: path.join(docs, "screenshot.png") });
await p.evaluate(() => { const e = document.querySelector("#review"); window.scrollTo(0, e.getBoundingClientRect().top + window.scrollY - document.querySelector("header").offsetHeight - 16); }); // 固定の見出しの下から
await p.waitForTimeout(400);
await p.screenshot({ path: path.join(docs, "summary.png") });
// 見直す候補（Worth a look）：期間の要点の下の「Sessions with possible friction」を押して開くダイアログ
await p.evaluate(() => window.scrollTo(0, 0));
await p.locator("#worth .flink", { hasText: "Sessions with possible friction" }).first().click();
await p.waitForTimeout(600);
await p.evaluate(() => { const d = document.querySelector("#wk"); d.style.maxHeight = "none"; }); // 中身がまるごと写るように
await p.locator("#wk").screenshot({ path: path.join(docs, "worth.png") });
await p.keyboard.press("Escape");
await p.waitForTimeout(300);
if (await p.evaluate(() => YEAR_ON)) { // 1 年の露光（一旦隠している間は、year.png を撮り直さない）。シェア用の画像（1600x900）をそのまま保存する
  await p.evaluate(() => openYear());
  await p.waitForTimeout(600);
  const png = await p.evaluate(() => document.querySelector("#yrcard").toDataURL("image/png").split(",")[1]);
  await fs.writeFile(path.join(docs, "year.png"), Buffer.from(png, "base64"));
  // 読み方の図（year-params.png）：year.png の光の部分に、読み方と Your light の決まり方を添える（year-params.html）
  const yp = await (await b.newContext({ viewport: { width: 1600, height: 900 }, deviceScaleFactor: 1 })).newPage();
  await yp.goto("file://" + path.resolve(path.dirname(new URL(import.meta.url).pathname), "year-params.html") + "?img=" + encodeURIComponent("file://" + path.resolve(docs, "year.png")));
  await yp.waitForTimeout(300);
  await yp.screenshot({ path: path.join(docs, "year-params.png"), fullPage: true });
}
// セッションの詳細：依頼の流れに出来事（コマンド・コミット・PR など）がいちばん多い種類そろうセッションを開き、左の列（数字と依頼の流れ）だけを 2 倍で撮る。
// 右の列にはダミーデータの一時フォルダのパスが出るので入れない
const sc = await b.newContext({ viewport: { width: 1440, height: 1700 }, deviceScaleFactor: 2, timezoneId: "Asia/Tokyo", locale: "en-US" });
const s = await sc.newPage();
await s.goto("file://" + path.resolve(html));
await s.waitForTimeout(600);
await s.keyboard.press("ArrowLeft");
await s.waitForTimeout(500);
let best = 0, most = -1;
for (let i = 0, n = await s.locator(".run").count(); i < n; i++) {
  await s.locator(".run").nth(i).click({ force: true });
  await s.waitForTimeout(200);
  const kinds = await s.evaluate(() => new Set([...document.querySelectorAll("#panel .kev")].map(e => e.className)).size);
  if (kinds > most) { most = kinds; best = i; }
  await s.keyboard.press("Escape");
  await s.waitForTimeout(150);
}
await s.locator(".run").nth(best).click({ force: true });
await s.waitForTimeout(600);
// 再開のコマンド（いちばん上）にも一時フォルダのパスが出るので、写さない
// 画面の CSP が <style> の追加を止めるので、要素の style で隠す
await s.evaluate(() => document.querySelectorAll("#panel .dresume").forEach(e => { e.style.display = "none"; }));
const box = await s.evaluate(() => {
  const pn = document.querySelector("#panel"), d = document.querySelector("#drawer").getBoundingClientRect();
  const top = pn.querySelector(".eyebrow").getBoundingClientRect(), col = pn.querySelector(".dcols > *").getBoundingClientRect();
  const x = Math.max(d.left + 2, col.left - 24), y = Math.max(d.top + 2, top.top - 22);
  return { x, y, width: col.right + 24 - x, height: Math.min(980, d.bottom - 2 - y) };
});
await s.screenshot({ path: path.join(docs, "session.png"), clip: box });
// SNS のリンクカード用（og.png）。ふつうの画面幅（1440）で、カードの比率（1.91:1）に合わせた高さ 754 を 2 倍の 2880x1508 で撮る。デモページの og:image に使う
const og = await b.newContext({ viewport: { width: 1440, height: 754 }, deviceScaleFactor: 2, timezoneId: "Asia/Tokyo", locale: "en-US" });
const q = await og.newPage();
await q.goto("file://" + path.resolve(html));
await q.waitForTimeout(600);
await q.keyboard.press("ArrowLeft");
await q.waitForTimeout(500);
await q.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); if (sc) sc.scrollTop = 8 * 44; });
await q.waitForTimeout(200);
await q.screenshot({ path: path.join(docs, "og.png") });
await b.close();
