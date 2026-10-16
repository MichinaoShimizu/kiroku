// docs/screenshot.png・docs/summary.png・docs/worth.png・docs/session.png・docs/og.png を撮る。gen.py と mkgit.py で作ったダミーデータの HTML を使う。
//   node capture.mjs <kiroku.html> <docs のディレクトリ>
// Playwright が必要（このディレクトリで npm ci --ignore-scripts と npx playwright install chromium。版は package.json で固定）。英語表示（en-US）・ライトテーマ・プロジェクトごとの色分け（画面の既定ではない）・時刻は Asia/Tokyo・1440x900。
import { chromium } from "playwright";
import path from "node:path";

const [html, docs] = process.argv.slice(2);
// KIROKU_FAKE_NOW があれば、ブラウザの時計もその時刻から進める（gen.py・mkgit.py と同じ「今」。smoke.mjs と同じ）。
// 同じ時刻で撮り直すと同じ画像になるので、tools/screenshots/keep-changed.sh で本当に変わった画像だけを残せる
const fakeNow = process.env.KIROKU_FAKE_NOW ? new Date(process.env.KIROKU_FAKE_NOW) : null;
if (fakeNow && isNaN(fakeNow)) { console.error(`KIROKU_FAKE_NOW を日時として読めません: ${process.env.KIROKU_FAKE_NOW}`); process.exit(2); }
// README と guide の画像は、ライトテーマ・プロジェクトごとの色分けで撮る（画面の既定はダーク・エージェントごと。説明の文と合わせる）。
// 撮るためのどのコンテキストも clock を通るので、ここで決める
const clock = async ctx => { if (fakeNow) await ctx.clock.install({ time: fakeNow });
  await ctx.addInitScript(() => { try { localStorage.setItem("kiroku:theme", '"light"'); localStorage.setItem("kiroku:colorBy", '"project"'); } catch (e) {} });
  return ctx; };
const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
const ctx = await clock(await b.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, timezoneId: "Asia/Tokyo", locale: "en-US" }));
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
// 見直す候補（Worth a look）：期間の要点の下の「Possible friction」を押して開くダイアログ
await p.evaluate(() => window.scrollTo(0, 0));
await p.locator("#worth .flink", { hasText: "Possible friction" }).first().click();
await p.waitForTimeout(600);
await p.evaluate(() => { const d = document.querySelector("#wk"); d.style.maxHeight = "none"; }); // 中身がまるごと写るように
await p.locator("#wk").screenshot({ path: path.join(docs, "worth.png") });
await p.keyboard.press("Escape");
await p.waitForTimeout(300);
// セッションの詳細：依頼の流れに出来事（コマンド・コミット・PR など）がいちばん多い種類そろうセッションを開き、左の列（数字と依頼の流れ）だけを 2 倍で撮る。
// 右の列にはダミーデータの一時フォルダのパスが出るので入れない
const sc = await clock(await b.newContext({ viewport: { width: 1440, height: 1700 }, deviceScaleFactor: 2, timezoneId: "Asia/Tokyo", locale: "en-US" }));
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
const og = await clock(await b.newContext({ viewport: { width: 1440, height: 754 }, deviceScaleFactor: 2, timezoneId: "Asia/Tokyo", locale: "en-US" }));
const q = await og.newPage();
await q.goto("file://" + path.resolve(html));
await q.waitForTimeout(600);
await q.keyboard.press("ArrowLeft");
await q.waitForTimeout(500);
await q.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); if (sc) sc.scrollTop = 8 * 44; });
await q.waitForTimeout(200);
await q.screenshot({ path: path.join(docs, "og.png") });
await b.close();
