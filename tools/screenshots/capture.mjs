// docs/screenshot.png・docs/summary.png・docs/sum-io.png・docs/sum-unit.png・docs/sum-sankey.png・docs/worth.png・docs/session.png・docs/og.png を撮る。gen.py と mkgit.py で作ったダミーデータの HTML を使う。
//   node capture.mjs <kiroku.html> <docs のディレクトリ>
// Playwright が必要（このディレクトリで npm ci --ignore-scripts と npx playwright install chromium。版は package.json で固定）。英語表示（en-US）・プロジェクトごとの色分け（画面の既定と同じ。前に選んだ色分けが残っていても、こちらに決める）・時刻は Asia/Tokyo・1440x900。
import { chromium } from "playwright";
import path from "node:path";

const [html, docs] = process.argv.slice(2);
// KIROKU_FAKE_NOW があれば、ブラウザの時計もその時刻から進める（gen.py・mkgit.py と同じ「今」。smoke.mjs と同じ）。
// 同じ時刻で撮り直すと同じ画像になるので、tools/screenshots/keep-changed.sh で本当に変わった画像だけを残せる
const fakeNow = process.env.KIROKU_FAKE_NOW ? new Date(process.env.KIROKU_FAKE_NOW) : null;
if (fakeNow && isNaN(fakeNow)) { console.error(`KIROKU_FAKE_NOW を日時として読めません: ${process.env.KIROKU_FAKE_NOW}`); process.exit(2); }
// README と guide の画像は、プロジェクトごとの色分けで撮る（画面の既定。説明の文と合わせる）。画面はいつもダーク。
// 撮るためのどのコンテキストも clock を通るので、ここで決める
const clock = async ctx => { if (fakeNow) await ctx.clock.install({ time: fakeNow });
  await ctx.addInitScript(() => { try { localStorage.setItem("kiroku:colorBy", '"project"'); } catch (e) {} });
  return ctx; };
const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
const ctx = await clock(await b.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, timezoneId: "Asia/Tokyo", locale: "en-US", colorScheme: "dark" }));
const p = await ctx.newPage();
await p.goto("file://" + path.resolve(html));
await p.waitForTimeout(600);
await p.keyboard.press("ArrowLeft"); // 先週（記録がそろっている週）
await p.waitForTimeout(500);
await p.evaluate(() => window.scrollTo(0, 0)); // 開いたときの作業の時刻への送り（今週だけ）をやめ、期間と上の数字から写す
await p.waitForTimeout(200);
await p.screenshot({ path: path.join(docs, "screenshot.png") });
// サマリーは「表示: サマリー」に切り替えて、サマリーの見出しが上に来るまで中をスクロールして撮る（広い画面ではページでなく main がスクロールする）
await p.locator('#view button[data-v="sum"]').click();
await p.waitForTimeout(300);
await p.evaluate(() => { const e = document.querySelector("#review"), m = document.querySelector("main"); m.scrollTop += e.getBoundingClientRect().top - document.querySelector("header").offsetHeight - 16; });
await p.waitForTimeout(400);
await p.screenshot({ path: path.join(docs, "summary.png") });
// README のサマリーの主要なグラフ：投入と産出（1 段目）、投入単価と産出単価（2 段目）、投入から産出の流れ（サンキー）。
// パネルの組は、2 枚を囲む範囲を少し余白をつけて切り抜く（2×2 のまま 1 枚にすると縦に長く、README で字が読めない）
const rect = sel => p.evaluate(s => { const r = document.querySelector(s).getBoundingClientRect(); return { l: r.left, t: r.top, r: r.right, b: r.bottom }; }, sel);
// shotPair は、パネル a と b を囲む範囲を撮る。top は上の余白（1 段目は IN・OUT の札まで入れる）
const shotPair = async (a, b, file, top = 28) => { const x = await rect(a), y = await rect(b), l = Math.min(x.l, y.l), t = Math.min(x.t, y.t);
  await p.screenshot({ path: path.join(docs, file), clip: { x: l - 12, y: t - top, width: Math.max(x.r, y.r) - l + 24, height: Math.max(x.b, y.b) - t + top + 12 } }); };
// 縦に長い窓にして、パネル 2 段とサンキーがスクロールせずに入るようにする（main の中のスクロールでは、切り抜きが窓の外に出る）
await p.setViewportSize({ width: 1440, height: 3200 });
await p.evaluate(() => { const e = document.querySelector("#review"), m = document.querySelector("main"); m.scrollTop += e.getBoundingClientRect().top - document.querySelector("header").offsetHeight - 16; });
await p.waitForTimeout(500);
await shotPair("#review .ik-cost", "#review .ik-out", "sum-io.png");
await shotPair("#review .ik-uin", "#review .ik-unit", "sum-unit.png", 8);
// サンキーの枠（「From input to output」）。同じ段の「Cost and output over time」も .iskw なので、最初の 1 つ
await p.locator("#review .iskw").first().screenshot({ path: path.join(docs, "sum-sankey.png") });
await p.setViewportSize({ width: 1440, height: 900 });
await p.locator('#view button[data-v="cal"]').click(); // 以降の画像はカレンダーで
await p.waitForTimeout(300);
// 見直す候補（Worth a look）：「Possible friction」のダイアログ
await p.evaluate(() => window.scrollTo(0, 0));
await p.evaluate(() => openWorth("friction")); // 期間の行の一覧はないので、ダイアログを直接開く
await p.waitForTimeout(600);
await p.evaluate(() => { const d = document.querySelector("#wk"); d.style.maxHeight = "none"; }); // 中身がまるごと写るように
await p.locator("#wk").screenshot({ path: path.join(docs, "worth.png") });
await p.keyboard.press("Escape");
await p.waitForTimeout(300);
// セッションの詳細：依頼の流れに出来事（コマンド・コミット・PR など）がいちばん多い種類そろうセッションを開き、左の列（数字と依頼の流れ）だけを 2 倍で撮る。
// 右の列にはダミーデータの一時フォルダのパスが出るので入れない
const sc = await clock(await b.newContext({ viewport: { width: 1440, height: 1700 }, deviceScaleFactor: 2, timezoneId: "Asia/Tokyo", locale: "en-US", colorScheme: "dark" }));
const s = await sc.newPage();
await s.goto("file://" + path.resolve(html));
await s.waitForTimeout(600);
await s.keyboard.press("ArrowLeft");
await s.waitForTimeout(500);
let best = 0, most = -1;
for (let i = 0, n = await s.locator(".run").count(); i < n; i++) {
  await s.locator(".run").nth(i).click({ force: true });
  await s.waitForTimeout(200);
  const kinds = await s.evaluate(() => new Set([...document.querySelectorAll("#panel .tl li.ev")].map(e => e.classList[1])).size); // 出来事の種類（commit・pr・warn など）
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
  const top = pn.querySelector(".eyebrow").getBoundingClientRect(), col = pn.querySelector(".scols > .sflow").getBoundingClientRect(); // 左の列（プロンプトの流れ）だけ。右の列には一時フォルダのパスが出る
  const x = Math.max(d.left + 2, col.left - 24), y = Math.max(d.top + 2, top.top - 22);
  return { x, y, width: col.right + 24 - x, height: Math.min(980, d.bottom - 2 - y) };
});
await s.screenshot({ path: path.join(docs, "session.png"), clip: box });
// SNS のリンクカード用（og.png）。ふつうの画面幅（1440）で、カードの比率（1.91:1）に合わせた高さ 754 を 2 倍の 2880x1508 で撮る。デモページの og:image に使う
const og = await clock(await b.newContext({ viewport: { width: 1440, height: 754 }, deviceScaleFactor: 2, timezoneId: "Asia/Tokyo", locale: "en-US", colorScheme: "dark" }));
const q = await og.newPage();
await q.goto("file://" + path.resolve(html));
await q.waitForTimeout(600);
await q.keyboard.press("ArrowLeft");
await q.waitForTimeout(500);
await q.evaluate(() => window.scrollTo(0, 0));
await q.waitForTimeout(200);
await q.screenshot({ path: path.join(docs, "og.png") });
await b.close();
