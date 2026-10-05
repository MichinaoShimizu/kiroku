// docs/screenshot.png・docs/summary.png・docs/year.png を撮る。gen.py と mkgit.py で作ったダミーデータの HTML を使う。
//   node capture.mjs <kiroku.html> <docs のディレクトリ>
// Playwright が必要（このディレクトリで npm i playwright と npx playwright install chromium）。英語表示（en-US）・ダークテーマ（既定）・時刻は Asia/Tokyo・1440x900。
import { chromium } from "playwright";
import path from "node:path";
import fs from "node:fs/promises";

const [html, docs] = process.argv.slice(2);
const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, timezoneId: "Asia/Tokyo", locale: "en-US", colorScheme: "dark" });
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
if (await p.evaluate(() => YEAR_ON)) { // 1 年の露光（一旦隠している間は、year.png を撮り直さない）。シェア用の画像（1600x900）をそのまま保存する
  await p.evaluate(() => openYear());
  await p.waitForTimeout(600);
  const png = await p.evaluate(() => document.querySelector("#yrcard").toDataURL("image/png").split(",")[1]);
  await fs.writeFile(path.join(docs, "year.png"), Buffer.from(png, "base64"));
}
await b.close();
