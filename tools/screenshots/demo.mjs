// README の先頭に置く動き（docs/demo.gif）の元になる動画を撮る。gen.py と mkgit.py で作ったダミーデータの HTML を使う。
//   node demo.mjs <kiroku.html> <動画を置くディレクトリ>
// 流れ: 先週のカレンダー → セッションを開いて依頼の流れを読む → トークンの内訳 → 検索 → 月の表示。
// 動画（webm）を 1 本だけ <動画を置くディレクトリ> に書き、そのパスと、頭から切り落とす秒数を出力する。GIF への変換は run.sh が ffmpeg で行う。
// ヘッドレスの録画にはマウスの矢印が写らないので、要素の style だけで作った丸を矢印の代わりに動かす（画面の CSP が <style> の追加を止めるため）。
import { chromium } from "playwright";
import path from "node:path";

const [html, out] = process.argv.slice(2);
const W = 1280, H = 800;
const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
const ctx = await b.newContext({ viewport: { width: W, height: H }, deviceScaleFactor: 1, timezoneId: "Asia/Tokyo", locale: "en-US", recordVideo: { dir: out, size: { width: W, height: H } } });
const p = await ctx.newPage();
const tStart = Date.now(); // 録画の始まり（ほぼ）
await p.goto("file://" + path.resolve(html));
await p.waitForTimeout(600);
await p.keyboard.press("ArrowLeft"); // 先週（記録がそろっている週）
await p.waitForTimeout(400);
await p.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); if (sc) sc.scrollTop = 8 * 44; });
await p.evaluate(() => {
  const c = document.createElement("div");
  c.id = "democursor";
  Object.assign(c.style, { position: "fixed", left: "0", top: "0", width: "22px", height: "22px", margin: "-11px 0 0 -11px", borderRadius: "50%",
    background: "rgba(232,93,4,.35)", border: "2px solid rgba(232,93,4,.9)", pointerEvents: "none", zIndex: "2147483647", transition: "transform .12s" });
  document.body.appendChild(c);
  addEventListener("mousemove", e => { c.style.left = e.clientX + "px"; c.style.top = e.clientY + "px"; }, true);
  addEventListener("mousedown", () => { c.style.transform = "scale(.7)"; }, true);
  addEventListener("mouseup", () => { c.style.transform = ""; }, true);
});

let at = { x: W / 2, y: H / 2 };
async function glide(x, y, ms = 600) { // 人の手のように、なめらかに動かす
  const steps = Math.max(8, Math.round(ms / 16));
  await p.mouse.move(x, y, { steps });
  at = { x, y };
}
async function clickOn(loc, ms) {
  await loc.scrollIntoViewIfNeeded();
  const r = await loc.boundingBox();
  await glide(r.x + r.width / 2, r.y + Math.min(r.height / 2, 14), ms);
  await p.waitForTimeout(150);
  await p.mouse.down(); await p.mouse.up();
}
await p.mouse.move(at.x, at.y);

// セッションの詳細：出来事（コマンド・コミット・PR など）の種類がいちばん多いセッションを選ぶ（capture.mjs の session.png と同じ選び方）
const runs = p.locator(".run");
let best = 0, most = -1;
for (let i = 0, n = await runs.count(); i < n; i++) {
  if (!(await runs.nth(i).isVisible())) continue;
  const box = await runs.nth(i).boundingBox();
  if (!box || box.y < 260 || box.y > H - 60) continue; // 画面に見えているものだけ
  await runs.nth(i).click({ force: true });
  await p.waitForTimeout(120);
  const kinds = await p.evaluate(() => new Set([...document.querySelectorAll("#panel .kev")].map(e => e.className)).size);
  if (kinds > most) { most = kinds; best = i; }
  await p.keyboard.press("Escape");
  await p.waitForTimeout(120);
}
await glide(W / 2, H / 2, 10);
const t0 = Date.now(); // ここより前（選ぶ間のちらつき）は、GIF にするときに切り落とす
await p.waitForTimeout(1600);
await clickOn(runs.nth(best), 900);
await p.evaluate(() => document.querySelectorAll("#panel .dresume").forEach(e => { e.style.display = "none"; })); // 再開のコマンドの一時フォルダのパスを写さない
await p.waitForTimeout(1400);
for (let i = 0; i < 6; i++) { // 依頼の流れをゆっくり読む
  await p.evaluate(() => { const d = document.querySelector("#drawer .db") || document.querySelector("#panel"); d.scrollBy({ top: 150, behavior: "smooth" }); });
  await p.waitForTimeout(450);
}
await p.waitForTimeout(900);
await p.keyboard.press("Escape");
await p.waitForTimeout(500);

// どの数字も開ける：トークンの内訳
await clickOn(p.locator('#kpis .kpi[data-metric="tokens"]'), 800);
await p.waitForTimeout(2600);
await p.keyboard.press("Escape");
await p.waitForTimeout(500);

// すべての期間を検索
await clickOn(p.locator("#q"), 800);
await p.keyboard.type("test", { delay: 140 });
await p.waitForTimeout(2400);
await p.keyboard.press("Escape");
await p.evaluate(() => { const q = document.querySelector("#q"); q.value = ""; q.dispatchEvent(new Event("input", { bubbles: true })); q.blur(); });
await p.waitForTimeout(400);

// 月の表示
await clickOn(p.locator('#mode button[data-v="month"]'), 800);
await p.waitForTimeout(2400);

const video = p.video();
await ctx.close();
console.log(`${await video.path()} ${((t0 - tStart) / 1000).toFixed(2)}`); // run.sh が読む: 動画のパスと、切り落とす秒数
await b.close();
