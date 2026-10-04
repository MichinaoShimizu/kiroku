// docs/screenshot.png と docs/summary.png を撮る。gen.py と mkgit.py で作ったダミーデータの HTML を使う。
//   node capture.mjs <kiroku.html> <docs のディレクトリ>
// Playwright が必要（npm i -g playwright など）。英語表示（en-US）・ダークテーマ・時刻は Asia/Tokyo・1440x900。
import { chromium } from "playwright";
import path from "node:path";

const [html, docs] = process.argv.slice(2);
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, timezoneId: "Asia/Tokyo", locale: "en-US", colorScheme: "dark" });
const p = await ctx.newPage();
await p.goto("file://" + path.resolve(html));
await p.waitForTimeout(600);
await p.keyboard.press("ArrowLeft"); // 先週（記録がそろっている週）
await p.waitForTimeout(500);
await p.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); if (sc) sc.scrollTop = 8 * 44; });
await p.waitForTimeout(200);
await p.screenshot({ path: path.join(docs, "screenshot.png") });
await p.evaluate(() => { const e = document.querySelector("#review"); window.scrollTo(0, e.getBoundingClientRect().top + window.scrollY - 20); });
await p.waitForTimeout(400);
await p.screenshot({ path: path.join(docs, "summary.png") });
await b.close();
