// 画面の大事な流れが動くかを、ダミーデータの HTML をブラウザで操作して確かめる（CI の e2e）。
//   sh tools/screenshots/run.sh --html /tmp/kiroku.html && node tools/screenshots/smoke.mjs /tmp/kiroku.html
// Playwright が必要（このディレクトリで npm i playwright と npx playwright install chromium）。
// 見るのは「動くか」だけで、見た目や言葉の良し悪しは docs/usability.md のテストで確かめる。
import { chromium } from "playwright";
import path from "node:path";

const html = process.argv[2];
if (!html) { console.error("使い方: node smoke.mjs <kiroku.html>"); process.exit(2); }
const url = "file://" + path.resolve(html);

const envs = [
  { name: "ja 1440", locale: "ja-JP", viewport: { width: 1440, height: 900 } },
  { name: "en 1440", locale: "en-US", viewport: { width: 1440, height: 900 } },
  { name: "ja 1000", locale: "ja-JP", viewport: { width: 1000, height: 800 } },
  { name: "ja 390 スマホ", locale: "ja-JP", viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true },
  { name: "en 320 スマホ", locale: "en-US", viewport: { width: 320, height: 680 }, isMobile: true, hasTouch: true }, // 英語は文言が長く、いちばん狭い画面ではみ出しやすい
];

let failed = 0;
function check(what, ok, detail = "") {
  if (ok) console.log(`  ok   ${what}`);
  else { failed++; console.log(`  FAIL ${what}${detail ? `: ${detail}` : ""}`); }
}

const b = await chromium.launch();
for (const env of envs) {
  console.log(`# ${env.name}`);
  const ctx = await b.newContext({ ...env, timezoneId: "Asia/Tokyo" });
  ctx.setDefaultTimeout(5000);
  const p = await ctx.newPage();
  const errors = [];
  p.on("pageerror", e => errors.push(e.message));
  p.on("console", m => { if (m.type() === "error") errors.push(m.text()); });
  const en = env.locale.startsWith("en");
  const pause = () => p.waitForTimeout(300);
  const drawerOpen = async () => await p.locator("#drawer").getAttribute("aria-hidden") === "false";
  // 1 つの流れが途中で止まっても、失敗として数えて次の流れへ進む（開き直して、記録のそろった先週から始める）
  const open = async () => { await p.goto(url); await p.waitForTimeout(500); await p.keyboard.press("ArrowLeft"); await pause(); };
  async function step(name, fn) {
    const before = failed;
    try { await fn(); } catch (e) { check(name, false, e.message.split("\n")[0]); }
    if (failed > before) await open().catch(() => {});
  }

  await p.goto(url);
  await p.waitForTimeout(500);

  await step("読み込み", async () => {
    check("セッションが読み込まれている", await p.evaluate("DATA.length") > 0);
    const [sw, iw] = await p.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
    check("横にはみ出さない", sw <= iw + 1, `${sw} > ${iw}`);
    check("ロゴが検索欄に隠れない", await p.evaluate(() => document.querySelector(".brand .word").getBoundingClientRect().right <= document.querySelector(".search").getBoundingClientRect().left));
  });

  await step("テーマ", async () => {
    const theme = () => p.evaluate(() => document.documentElement.dataset.theme);
    check("既定はダーク", await theme() === "dark", await theme());
    await p.locator("#theme").click(); await pause();
    check("押すとライトになる", await theme() === "light", await theme());
    await p.locator("#theme").click(); await pause();
    check("もう一度押すとダークに戻る", await theme() === "dark", await theme());
  });

  await step("週の移動", async () => {
    const label = () => p.locator("#rd").innerText();
    await p.keyboard.press("t"); await pause(); // 今週に記録がないとき（日本時間の月曜の朝など）は、開いた週が今週ではないので、先に今週へ移っておく
    const before = await label();
    await p.keyboard.press("ArrowLeft"); await pause();
    check("← で前の週へ移る", await label() !== before, `${before} のまま`);
    await p.keyboard.press("t"); await pause();
    check("t で今週へ戻る", await label() === before, await label());
    await p.keyboard.press("ArrowLeft"); await pause(); // 以降は、記録のそろった先週で試す
    const cut = await p.evaluate(() => { const hd = document.querySelector("#tl .heads").getBoundingClientRect().bottom;
      return [...document.querySelectorAll("#tl .run")].filter(r => { const b = r.getBoundingClientRect(); return b.top < hd - 1 && b.bottom > hd + 1; }).length; });
    check("開いた位置で、日付の見出しに半分隠れたブロックがない", cut === 0, `${cut} 件`);
    check("過ぎた週は月曜から見える", await p.evaluate(() => document.querySelector("#tl .calscroll").scrollLeft) === 0);
  });

  await step("セッションの詳細", async () => {
    const run = p.locator(".run[data-sid]").first();
    check("カレンダーにセッションがある", await run.count() > 0);
    await run.scrollIntoViewIfNeeded();
    await run.click(); await pause();
    check("押すと詳細が開く", await drawerOpen());
    const box = await p.locator("#drawer").boundingBox();
    check("詳細が画面に収まる", box && box.x >= -1 && box.x + box.width <= env.viewport.width + 1, JSON.stringify(box));
    await p.keyboard.press("Escape"); await pause();
    check("Esc で詳細が閉じる", !await drawerOpen());
    await run.focus(); await p.keyboard.press("ArrowLeft"); await pause();
    check("ブロックにいたまま週を移っても、フォーカスがカレンダーに残る", await p.evaluate(() => !!document.activeElement.closest("#tl")), await p.evaluate(() => document.activeElement.tagName));
    await p.keyboard.press("ArrowRight"); await pause();
  });

  await step("詳細を閉じると読んでいた場所へ戻る", async () => {
    const card = p.locator("#review .card[data-id]").first();
    check("サマリーにセッションのカードがある", await card.count() > 0);
    await card.scrollIntoViewIfNeeded();
    const id = await card.getAttribute("data-id");
    await card.click(); await pause();
    const y = await p.evaluate("scrollY"); // 開いた時点の位置（クリックの前に Playwright がスクロールし直すことがあるので、開いてから測る。開いている間はページが動かない）
    await p.keyboard.press("Escape"); await pause();
    const after = await p.evaluate("scrollY");
    check("閉じてもサマリーの位置のまま", Math.abs(after - y) <= 2, `${y} → ${after}`);
    check("閉じるとカードにフォーカスが戻る", await p.evaluate(id => { const a = document.activeElement; return a.dataset.id === id && !!a.closest("#review"); }, id));
  });

  await step("見直す候補", async () => {
    const n = await p.locator(".flagsum .flink").count();
    check("繰り返したプロンプトの欄がある", await p.locator('#review .hb[data-help="repeats"]').count() > 0);
    check("見直す候補の数だけ、指標に印が付く", n > 0 && await p.locator(".fl").count() >= n, `候補 ${n}`);
    const id = await p.locator(".flagsum .flink").first().getAttribute("data-goto");
    await p.locator(".flagsum .flink").first().click(); await pause();
    check("候補を押すと、その指標の説明が開く", await p.locator(`.panel .hb[data-help="${id}"]`).first().getAttribute("aria-expanded") === "true", id);
    await open();
  });

  await step("週報の下書き", async () => {
    const tog = p.locator("#rpttog");
    await tog.scrollIntoViewIfNeeded();
    await tog.click(); await pause();
    check("週報の下書きが開く", await p.locator("#rptbox").isVisible());
    const text = await p.locator("#rptpre").innerText();
    check("週報の下書きに文面がある", text.trim().length > 20, JSON.stringify(text.slice(0, 40)));
  });

  await step("検索", async () => {
    const word = await p.evaluate(() => {
      const t = DATA.flatMap(s => s.prompts.map(x => x.text)).find(x => x.trim().length >= 4);
      return t ? t.trim().slice(0, 4) : "";
    });
    check("検索に使うプロンプトがある", word !== "");
    await p.keyboard.press("/");
    check("/ で検索欄に移る", await p.evaluate(() => document.activeElement && document.activeElement.id === "q"));
    await p.locator("#q").fill(word); await pause();
    const heading = await p.locator("#review h2").first().innerText();
    check("検索結果が出る", heading === (en ? "Search results" : "検索結果"), heading);
    check("一致したセッションがある", await p.locator(".srow[data-s]").count() > 0, word);
    await p.locator(".srow[data-s]").first().click(); await pause();
    check("検索結果から詳細が開く", await drawerOpen());
    await p.keyboard.press("Escape"); await pause();
    check("閉じると検索結果に戻る", await p.evaluate(() => !!document.activeElement.closest("#review .srow[data-s]")));
    await p.locator("#sclear").click(); await pause();
    check("検索をやめるとサマリーに戻る", await p.locator("#q").inputValue() === "" && await p.locator("#rpttog").count() > 0);
  });

  await step("表示の切り替え", async () => {
    await p.keyboard.press("m"); await pause();
    check("m で月表示になる", await p.locator("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
    await p.keyboard.press("w"); await pause();
    await p.keyboard.press("?"); await pause();
    check("? でショートカットの一覧が開く", await p.locator("#keys").evaluate(d => d.open));
    await p.keyboard.press("Escape");
  });

  await step("1 年の露光", async () => {
    const on = await p.evaluate(() => YEAR_ON);
    if (!on){ // 一旦隠している間は、ボタンも Y キーも出ないことだけ確かめる
      check("1 年の露光のボタンが出ない", await p.locator("#yrbtn").isHidden());
      await p.keyboard.press("y"); await pause();
      check("y で 1 年の露光が開かない", !(await p.locator("#yr").evaluate(d => d.open)));
      return;
    }
    await p.keyboard.press("y"); await pause();
    check("y で 1 年の露光が開く", await p.locator("#yr").evaluate(d => d.open));
    check("シェア用の画像に光が描かれている", await p.locator("#yrcard").evaluate(c => {
      const d = c.getContext("2d").getImageData(0, 430, c.width, c.height - 430).data; let lit = 0;
      for (let i = 0; i < d.length; i += 4) if (d[i] + d[i+1] + d[i+2] > 240) lit++;
      return lit > 200; }));
    check("光の名前が出る", (await p.locator(".yrtn").innerText()).trim().length > 0);
    check("腕前のメーターが 4 つ出る", await p.locator(".yrmeter > div").count() === 4);
    const [sw, iw] = await p.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
    check("1 年の露光が横にはみ出さない", sw <= iw + 1, `${sw} > ${iw}`);
    await p.keyboard.press("Escape"); await pause();
    check("Esc で 1 年の露光を閉じる", !(await p.locator("#yr").evaluate(d => d.open)));
  });

  check("スクリプトのエラーがない", errors.length === 0, errors.join(" / "));
  await ctx.close();
}
await b.close();
if (failed) { console.log(`\n${failed} 件失敗`); process.exit(1); }
console.log("\nすべて OK");
