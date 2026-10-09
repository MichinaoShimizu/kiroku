// 画面の大事な流れが動くかを、ダミーデータの HTML をブラウザで操作して確かめる（CI の e2e）。
//   sh tools/screenshots/run.sh --html /tmp/kiroku.html && node tools/screenshots/smoke.mjs /tmp/kiroku.html
// Playwright が必要（このディレクトリで npm ci --ignore-scripts と npx playwright install chromium。版は package.json で固定）。
// 見るのは「動くか」だけで、見た目や言葉の良し悪しは docs/usability.md のテストで確かめる。
import { chromium } from "playwright";
import path from "node:path";
import { stat } from "node:fs/promises";

const html = process.argv[2];
if (!html) { console.error("使い方: node smoke.mjs <kiroku.html>"); process.exit(2); }
const url = "file://" + path.resolve(html);

const envs = [
  { name: "1440", viewport: { width: 1440, height: 900 } },
  { name: "1000", viewport: { width: 1000, height: 800 } },
  { name: "390 スマホ", viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true },
  { name: "320 スマホ", viewport: { width: 320, height: 680 }, isMobile: true, hasTouch: true }, // いちばん狭い画面ではみ出しやすい
];

const b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {}); // CHROMIUM: Playwright の Chromium を取ってこられない環境向け
// 画面の幅ごとの流れは互いに関係がないので、並べて動かす（待ち時間がほとんどなので、CI が速くなる）。
// 出力は幅ごとにためておき、終わってから順に出す
// clickableRun は、カレンダーのセッションのブロックのうち、真ん中がほかのブロックに隠れていない最初のもの（Playwright は真ん中を押す）。
// ダミーデータは今日を基準に作るので、日によっては最初のブロックに、並行したセッションのブロックが重なる
async function clickableRun(p) {
  const i = await p.evaluate(() => [...document.querySelectorAll(".run[data-sid]")].findIndex(e => {
    e.scrollIntoView({ block: "center" });
    const b = e.getBoundingClientRect();
    return e.contains(document.elementFromPoint(b.left + b.width / 2, b.top + b.height / 2));
  }));
  return p.locator(".run[data-sid]").nth(Math.max(i, 0));
}

async function run(env) {
  const out = [`# ${env.name}`];
  let failed = 0;
  const csp = []; // 流れ全体を通して、CSP に止められたもの（1 件でもあれば失敗）
  function check(what, ok, detail = "") {
    if (ok) out.push(`  ok   ${what}`);
    else { failed++; out.push(`  FAIL ${what}${detail ? `: ${detail}` : ""}`); }
  }
  const ctx = await b.newContext({ ...env, locale: "en-US", timezoneId: "Asia/Tokyo", acceptDownloads: true });
  ctx.setDefaultTimeout(5000);
  // CSP の違反はページの中で拾って知らせる（関数を渡す仕組みは CSP の外なので止められない）
  await ctx.exposeFunction("kirokuCSPViolation", v => csp.push(v));
  await ctx.addInitScript(() => addEventListener("securitypolicyviolation", e => window.kirokuCSPViolation(`${e.effectiveDirective} ${e.blockedURI || "inline"} ${e.sourceFile || ""}:${e.lineNumber || ""}`)));
  const p = await ctx.newPage();
  const errors = [];
  p.on("pageerror", e => errors.push(e.message));
  p.on("console", m => { if (m.type() === "error") errors.push(m.text()); });
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
    check("凡例の件数（20 / 20 sessions）が画面の中に見える", await p.evaluate(() => { const r = document.querySelector("#legend .count").getBoundingClientRect(); return r.width > 0 && r.left >= 0 && r.right <= innerWidth + 1; }));
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
    check("期間の日付は「Sep 28 – Oct 4」の形", /^[A-Z][a-z]{2} \d{1,2} – [A-Z][a-z]{2} \d{1,2}$/.test(await label()), await label());
    await p.keyboard.press("t"); await pause();
    check("t で今週へ戻る", await label() === before, await label());
    check("今週では「次の週」を押せない", await p.locator("#next").isDisabled());
    await p.keyboard.press("ArrowRight"); await pause();
    check("今週から → を押しても先の週へ行かない", await label() === before, await label());
    const prevX = () => p.evaluate(() => Math.round(document.querySelector("#prev").getBoundingClientRect().x));
    const x0 = await prevX();
    await p.keyboard.press("ArrowLeft"); await pause(); // 以降は、記録のそろった先週で試す
    check("前の週へ移っても ‹ の位置が変わらない", await prevX() === x0, `${x0} → ${await prevX()}`);
    check("前の週では「次の週」を押せる", await p.locator("#next").isEnabled());
    check("先週なら、期間の見出しにそう書いてある", /last week$/i.test(await p.locator("#ry").innerText()), await p.locator("#ry").innerText());
    check("凡例の数は作業時間", /\d+(h|m)/.test(await p.locator("#legend .chip .n").first().innerText()), await p.locator("#legend .chip .n").first().innerText());
    const cut = await p.evaluate(() => { const hd = document.querySelector("#tl .heads").getBoundingClientRect().bottom;
      return [...document.querySelectorAll("#tl .run")].filter(r => { const b = r.getBoundingClientRect(); return b.top < hd - 1 && b.bottom > hd + 1; }).length; });
    check("開いた位置で、日付の見出しに半分隠れたブロックがない", cut === 0, `${cut} 件`);
    check("過ぎた週は月曜から見える", await p.evaluate(() => document.querySelector("#tl .calscroll").scrollLeft) === 0);
    if (env.isMobile){ // スマホでは、右にまだ日があるあいだだけ右端をぼかす
      check("右にまだ日があると、右端がぼける", await p.evaluate(() => document.querySelector("#tl").classList.contains("morer")));
      await p.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); sc.scrollLeft = sc.scrollWidth; }); await pause();
      check("右端までスクロールすると、ぼかしが消える", await p.evaluate(() => !document.querySelector("#tl").classList.contains("morer")));
      await p.evaluate(() => { document.querySelector("#tl .calscroll").scrollLeft = 0; }); await pause();
    }
  });

  await step("セッションの詳細", async () => {
    const run = await clickableRun(p);
    check("カレンダーにセッションがある", await run.count() > 0);
    await run.scrollIntoViewIfNeeded();
    await run.click(); await pause();
    check("押すと詳細が開く", await drawerOpen());
    const box = await p.locator("#drawer").boundingBox();
    check("詳細が画面に収まる", box && box.x >= -1 && box.x + box.width <= env.viewport.width + 1, JSON.stringify(box));
    check("閉じるボタンが種類の行に並び、ボタンだけの空いた帯がない", await p.evaluate(() => { const c = document.querySelector("#close").getBoundingClientRect(), e = document.querySelector("#panel .eyebrow").getBoundingClientRect();
      return Math.abs((c.top + c.bottom) / 2 - (e.top + e.bottom) / 2) < 12 && c.left > e.left; }));
    const rep = p.locator("#panel .tl li.rp").first();
    check("依頼のあとに AI の応答が出ている", await rep.count() > 0);
    if (await rep.count() > 0){
      await rep.scrollIntoViewIfNeeded();
      check("応答の中身が読める", (await rep.locator(".reptext").innerText()).trim().length > 0);
      check("だれの発言か分かる", (await rep.locator(".rpw").innerText()).trim().length > 0);
      check("応答は依頼のすぐ後に並ぶ", await p.evaluate(() => { const li = [...document.querySelectorAll("#panel .tl li")], i = li.findIndex(x => x.classList.contains("rp"));
        return i > 0 && li[i - 1].classList.contains("pr"); }));
    }
    await p.locator('#flowBy button[data-v="user"]').click(); await pause();
    check("「ユーザープロンプトだけ」で、出来事が隠れる", await p.evaluate(() => [...document.querySelectorAll("#drawer .tl li.ev")].every(li => !li.offsetParent)));
    check("「ユーザープロンプトだけ」では、応答も隠れる", await p.evaluate(() => [...document.querySelectorAll("#drawer .tl li.rp")].every(x => !x.offsetParent)));
    check("「ユーザープロンプトだけ」では、出ていないものの説明を凡例に残さない", await p.evaluate(() => [...document.querySelectorAll("#drawer .tlkey .kev")].every(x => !x.offsetParent)));
    await p.locator('#flowBy button[data-v="all"]').click(); await pause();
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

  await step("ヘッダーから指標ガイドが開く", async () => {
    await p.click("#guide"); await pause();
    check("指標ガイドのダイアログが開き、見出しにフォーカスがある", await p.evaluate(() => document.querySelector("#mg").open && document.activeElement.id === "mgh"));
    check("かけたもの・4 つのレバー・残ったもの・比べ方がある", await p.evaluate(() => { const d = document.querySelector("#mg"); return d.querySelectorAll(".gflow .gbox").length === 3 && d.querySelectorAll(".glev li").length === 4 && d.querySelectorAll(".glever").length === 4 && !!d.querySelector(".gcmp"); }));
    check("名前は HELP から取る", await p.evaluate(() => [...document.querySelectorAll("#mg .gin li")].map(li => li.textContent).includes(HELP.active.n)));
    await p.keyboard.press("Escape"); await pause();
    check("閉じるとボタンにフォーカスが戻る", await p.evaluate(() => !document.querySelector("#mg").open && document.activeElement.id === "guide"));
    await p.keyboard.press("g"); await pause();
    check("G でも開く", await p.evaluate(() => document.querySelector("#mg").open));
    await p.keyboard.press("ArrowRight"); await pause();
    check("開いているあいだは ← → で期間が動かない", await p.evaluate(() => document.querySelector("#mg").open));
    await p.keyboard.press("Escape"); await pause();
    await p.locator("#review .stat.mstat:has([data-metric=cost]) .hb").click(); await pause();
    await p.keyboard.press("g"); await pause();
    check("開いた「?」の説明はガイドを開くと閉じる", await p.evaluate(() => document.querySelector("#mg").open && document.querySelector("#hpop").hidden));
    await p.keyboard.press("Escape"); await pause();
  });

  await step("サマリーの数字から内訳が開く", async () => {
    const v = p.locator('#review .stat [data-metric="cost"]');
    check("サマリーの目安コストを押せる", await v.count() === 1);
    await v.scrollIntoViewIfNeeded();
    const s = await p.locator("#review .stat.mstat:has([data-metric=cost]) .s").boundingBox(); // 値の下の一言を押しても開く（枠全体が押せる）
    await p.mouse.click(s.x + 3, s.y + 3); await pause();
    check("押すと内訳のダイアログが開く", await p.evaluate(() => document.querySelector("#md").open && document.querySelector("#mdh").textContent === "Estimated cost"));
    const wk = await p.evaluate(() => document.querySelector("#rd").textContent);
    await p.keyboard.press("ArrowLeft"); await p.keyboard.press("g"); await pause();
    check("内訳を開いているあいだは ← で期間が動かず、G でガイドも重ならない", await p.evaluate(wk => document.querySelector("#rd").textContent === wk && !document.querySelector("#mg").open && document.querySelector("#md").open, wk));
    check("指標の定義は大きな数字のすぐ下にあり、言えないことは最後にある", await p.evaluate(() => { const d = document.querySelector("#md"), def = d.querySelector(".mbig + .mdef"); return !!def && def.textContent.length > 0 && d.getAttribute("aria-describedby") === def.id && d.lastElementChild.matches(".mnot") && /Doesn't tell you/.test(d.lastElementChild.textContent) && def.textContent + ". " + d.querySelector(".mnot p").textContent === HELP.cost.d; })); // 定義の 1 文目が上、残りが最後
    await p.keyboard.press("Escape"); await pause();
    check("閉じると押した数字にフォーカスが戻る", await p.evaluate(() => document.activeElement.dataset.metric === "cost" && !!document.activeElement.closest("#review")));
    const hb = p.locator("#review .stat.mstat:has([data-metric=cost]) .hb");
    await hb.click(); await pause();
    check("枠の中の ? は説明を開き、内訳は開かない", await p.evaluate(() => !document.querySelector("#hpop").hidden && !document.querySelector("#md").open));
    await p.keyboard.press("Escape"); await pause();
  });

  await step("サマリーのどの数字も内訳が開く", async () => {
    const ids = await p.evaluate(() => [...document.querySelectorAll("#review [data-metric]")].map(b => b.dataset.metric));
    for (const id of ["lines", "files", "pushes", "focus", "parallel", "wait", "ai", "cache", "costPerAsk"]) check(`サマリーの ${id} を押せる`, ids.includes(id));
    for (const id of ids) {
      await p.evaluate(id => document.querySelector(`#review [data-metric="${id}"]`).click(), id); await pause();
      check(`${id} の内訳が開き、中身がある`, await p.evaluate(() => { const d = document.querySelector("#md"); return d.open && d.querySelectorAll("h3").length > 0; }));
      check(`${id} の内訳の定義は 1 つだけ`, await p.evaluate(() => document.querySelectorAll("#mdd").length <= 1));
      await p.keyboard.press("Escape"); await pause();
    }
    await p.evaluate(() => document.querySelector('#review [data-metric="commits"]').click()); await pause();
    const row = p.locator("#md [data-git]").first();
    check("Git commits の内訳にコミットの一覧がある", await row.count() === 1);
    await row.click(); await pause();
    check("一覧のコミットを押すと、ダイアログを閉じてコミットの詳細が開く", await p.evaluate(() => !document.querySelector("#md").open && st.sel && st.sel.startsWith("git:")) && await drawerOpen());
    await p.keyboard.press("Escape"); await pause();
  });

  await step("セッションを AI と振り返る", async () => {
    const run = await clickableRun(p);
    await run.scrollIntoViewIfNeeded(); await run.click(); await pause();
    const btn = p.locator("#panel .flowbar #sreview");
    check("振り返りのプロンプトのボタンが、プロンプトの流れの見出しに並ぶ", await btn.count() === 1);
    await btn.click(); await pause();
    check("押すと、プロンプトが入っていると知らせる", /Copied a prompt.*your prompts|Couldn't copy/.test(await p.locator("#toast").innerText()), await p.locator("#toast").innerText());
    await p.keyboard.press("Escape"); await pause();
  });

  await step("詳細と矢印キー・ブラウザの戻る", async () => {
    const label = () => p.locator("#rd").innerText(), before = await label();
    const run = await clickableRun(p);
    await run.scrollIntoViewIfNeeded(); await run.click(); await pause();
    check("詳細が開く", await drawerOpen());
    await p.keyboard.press("ArrowLeft"); await pause();
    check("詳細を開いているあいだは ← で週が変わらない", await drawerOpen() && await label() === before, await label());
    await p.goBack(); await pause();
    check("ブラウザの戻るで詳細が閉じ、ページに残る", !await drawerOpen() && await p.evaluate(() => typeof DATA === "object"), p.url());
    await (await clickableRun(p)).click(); await pause(); // 戻ると位置が変わり、さっきのブロックが隠れることがある
    await p.keyboard.press("Escape"); await pause();
    check("Esc で閉じると、積んだ履歴も消える", !await drawerOpen() && await p.evaluate(() => history.state === null));
    await p.locator("#mode button").first().focus(); await p.keyboard.press("ArrowRight"); await pause();
    check("切り替えのボタンにいるときは → で週が変わらない", await label() === before, await label());
  });

  await step("コストと残ったもの", async () => {
    const oc = p.locator("#review .panel.oc"); await oc.scrollIntoViewIfNeeded(); await pause();
    check("使ったものと残ったものが左右に並ぶ", await oc.locator(".ocside.spent").count() === 1 && await oc.locator(".ocside.left").count() === 1);
    // git から数える指標（ダミーデータにはコミットも push もある）
    for (const [name, id] of [["コミット", "gitCommits"], ["行", "lines"], ["ファイル", "files"], ["push", "pushes"]])
      check(`残ったものに${name}が出る`, await oc.locator(`.ocside.left .hb[data-help="${id}"]`).count() === 1);
    check("割った指標は残ったものではなく Compared の行にある", await oc.locator('.ocside.left .hb[data-help="costPerCommit"]').count() === 0
      && await oc.locator('.occmp .hb[data-help="costPerCommit"]').count() === 1 && await oc.locator('.occmp .hb[data-help="outSessions"]').count() === 1);
    check("カードが箱からはみ出さない", await oc.evaluate(e => [...e.querySelectorAll(".stat")].every(s => s.getBoundingClientRect().right <= e.getBoundingClientRect().right + 1)));
  });

  await step("説明（?）を Esc で閉じる", async () => {
    const hb = p.locator('#review .hb[data-help="active"]').first();
    await hb.scrollIntoViewIfNeeded(); await hb.click(); await pause();
    const card = await hb.evaluate(b => b.closest(".stat").getBoundingClientRect().height);
    check("? で説明が吹き出しで開く", await hb.getAttribute("aria-expanded") === "true" && await p.locator("#hpop").isVisible());
    check("説明を開いてもカードは広がらない", await hb.evaluate(b => b.closest(".stat").getBoundingClientRect().height) === card);
    const pop = await p.locator("#hpop").boundingBox();
    check("吹き出しが画面の横に収まる", pop && pop.x >= 0 && pop.x + pop.width <= env.viewport.width + 1, JSON.stringify(pop));
    check("印のない指標でも、説明に 8 週の推移が出る", await p.locator("#hpop .htrend svg").first().isVisible());
    await p.keyboard.press("Escape"); await pause();
    check("Esc で説明が閉じ、? にフォーカスが戻る", await hb.getAttribute("aria-expanded") === "false" && !(await p.locator("#hpop").isVisible()) && await hb.evaluate(b => b === document.activeElement));
    await hb.click(); await pause(); await p.mouse.click(4, env.viewport.height - 4); await pause();
    check("吹き出しの外を押すと閉じる", !(await p.locator("#hpop").isVisible()));
  });

  await step("見直す候補", async () => {
    const n = await p.locator(".flagsum .flink").count();
    check("見直す候補はカレンダーより上にある", await p.evaluate(() => document.querySelector("#worth").getBoundingClientRect().bottom <= document.querySelector("#tl").getBoundingClientRect().top));
    check("繰り返したプロンプトの欄がある", await p.locator('#review .hb[data-help="repeats"]').count() > 0);
    check("見直す候補の数だけ、サマリーの指標名の横に印が付く", n > 0 && await p.locator("#review .fmark").count() >= n, `候補 ${n}`);
    check("サマリーには理由や推移を繰り返さない（ダイアログにだけ出す）", await p.locator("#review .fl").count() === 0);
    const link = p.locator(".flagsum .flink").first(), id = await link.getAttribute("data-goto");
    await link.scrollIntoViewIfNeeded(); await pause(); const y0 = await p.evaluate("scrollY");
    await link.click(); await pause();
    check("候補を押すと、真ん中のダイアログで開く", await p.locator("#wk").evaluate(d => d.open) && (await p.locator("#wkh").innerText()).trim().length > 0, id);
    check("ダイアログには、見えたこと・基準・次にやってみることがある", await p.locator("#wk .fl .see").count() > 0 && await p.locator("#wk .fl .rule").count() > 0 && /What to try/.test(await p.locator("#wk .wkhelp").innerText()));
    const box = await p.locator("#wk").boundingBox();
    check("ダイアログが画面に収まる", box && box.x >= 0 && box.x + box.width <= env.viewport.width + 1, JSON.stringify(box));
    check("開いてもページは動かない", await p.evaluate("scrollY") === y0);
    await p.keyboard.press("Escape"); await pause();
    check("Esc で閉じ、押した候補にフォーカスが戻る", !(await p.locator("#wk").evaluate(d => d.open)) && await link.evaluate(b => b === document.activeElement));
    check("閉じるボタンは右上の × だけ（ほかのダイアログや詳細のパネルとそろえる）", await p.locator("#wk button.pill").count() === 0 && await p.locator("#wkclose").getAttribute("aria-label") === "Close");
    await link.click(); await pause();
    await p.mouse.click(4, env.viewport.height - 4); await pause();
    check("枠の外を押すと閉じる", !(await p.locator("#wk").evaluate(d => d.open)));
    const mk = p.locator(`#review .fmark[data-goto="${id}"]`).first();
    await mk.scrollIntoViewIfNeeded(); await mk.click(); await pause();
    check("サマリーの印を押すと、同じダイアログが開く", await p.locator("#wk").evaluate(d => d.open) && await p.locator("#wk .fl .see").count() > 0, id);
    await p.locator("#wkclose").click(); await pause();
    check("Close で閉じる", !(await p.locator("#wk").evaluate(d => d.open)));
    const ses = p.locator(".flagsum .flink").first(); await ses.scrollIntoViewIfNeeded(); await ses.click(); await pause();
    const fs = p.locator("#wk .fss .card").first();
    if (await fs.count()){
      // エージェントの頭文字は、題名が長くても縮まず 1 行（.fl の overflow-wrap:anywhere で「KC」が K と C の 2 行に折れていた）
      const marks = await p.locator("#wk .fss .card .agm").evaluateAll(es => es.map(e => { const r = document.createRange(); r.selectNodeContents(e);
        return {t: e.textContent, lines: new Set([...r.getClientRects()].map(x => Math.round(x.top))).size, fits: e.clientWidth >= r.getBoundingClientRect().width}; }));
      const bad = marks.filter(m => m.lines !== 1 || !m.fits);
      check("ダイアログのカードのエージェントの頭文字は 1 行に収まる", marks.length > 0 && !bad.length, JSON.stringify(bad[0] || {}));
      await fs.click(); await pause();
      check("ダイアログのセッションを押すと、ダイアログを閉じて詳細が開く", !(await p.locator("#wk").evaluate(d => d.open)) && await drawerOpen());
      check("詳細の頭に、このセッションが関係する見直す候補が出る", await p.locator("#panel .sflags .flink").count() > 0);
      await p.keyboard.press("Escape"); await pause(); }
    await open();
  });

  await step("ページの下", async () => {
    const gh = p.locator('#review .foot a[href="https://github.com/MichinaoShimizu/kiroku"]');
    check("フッターにリポジトリへのリンクがある（参照元を渡さない）", await gh.count() === 1 && /noreferrer/.test(await gh.getAttribute("rel") || "") && await gh.getAttribute("target") === "_blank");
    const ds = p.locator("#review details.dsrc");
    check("Data sources は、気をつけることがなければたたんである", await ds.count() === 1 && !(await ds.evaluate(d => d.open)));
    await ds.locator("summary").click(); await pause();
    check("開くと、読んだ履歴が見える", await ds.locator(".mlist li").first().isVisible());
  });

  await step("週報の下書き", async () => {
    const tog = p.locator("#rpttog");
    await tog.scrollIntoViewIfNeeded();
    await tog.click(); await pause();
    check("週報の下書きが開く", await p.locator("#rptbox").isVisible());
    const text = await p.locator("#rptpre").innerText();
    check("週報の下書きに文面がある", text.trim().length > 20, JSON.stringify(text.slice(0, 40)));
    check("週報の下書きの期間は「Sep 28 – Oct 4, 2026」の形", /^## Work for [A-Z][a-z]{2} \d{1,2} – ([A-Z][a-z]{2} \d{1,2}, )?\d{4}|^## Work for [A-Z][a-z]{2} \d{1,2}, \d{4} – /.test(text), JSON.stringify(text.split("\n")[0]));
    check("週報の下書きに HTML のコメントがない（貼るとそのまま見える）", !text.includes("<!--") && text.trim().endsWith("_Drafted with kiroku_"), JSON.stringify(text.trim().split("\n").pop()));
    await p.locator("#rptcopy").click(); await pause();
    check("コピーのボタンを押すと知らせが出る", /Copied|Couldn't copy/.test(await p.locator("#toast").innerText()));
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
    const heading = await p.locator("#sres h2").first().innerText();
    check("検索結果が出る", heading === "Search results", heading);
    check("検索結果はカレンダーより上にある", await p.evaluate(() => document.querySelector("#sres").getBoundingClientRect().bottom <= document.querySelector("#tl").getBoundingClientRect().top));
    check("一致したセッションがある", await p.locator(".srow[data-s]").count() > 0, word);
    check("検索結果は枠の中でスクロールし、カレンダーを押し出さない", await p.evaluate(() => [...document.querySelectorAll("#sres .panel")].every(e => e.getBoundingClientRect().height <= Math.min(innerHeight * .52, 520) + 1)));
    const more = p.locator("#srmS");
    if (await more.count()){ const before = await p.locator(".srow[data-s]").count();
      await more.click(); await pause();
      check("「もっと見る」で結果が増え、足した最初の結果へ移る", await p.locator(".srow[data-s]").count() > before && await p.evaluate(n => document.activeElement === document.querySelectorAll("#sres .srow[data-s]")[n], before)); }
    await p.locator("#q").focus(); await p.keyboard.press("Enter"); await pause();
    check("検索欄で Enter を押すと、最初の結果へ移る", await p.evaluate(() => !!document.activeElement.closest("#sres .srow")));
    await p.locator(".srow[data-s]").first().click(); await pause();
    check("検索結果から詳細が開く", await drawerOpen());
    await p.keyboard.press("Escape"); await pause();
    check("閉じると検索結果に戻る", await p.evaluate(() => !!document.activeElement.closest("#sres .srow[data-s]")));
    await p.locator("#sclear").click(); await pause();
    check("検索をやめるとサマリーに戻る", await p.locator("#q").inputValue() === "" && await p.locator("#rpttog").count() > 0 && await p.locator("#sres").isHidden());
  });

  await step("表示の切り替え", async () => {
    await p.keyboard.press("m"); await pause();
    check("m で月表示になる", await p.locator("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
    check("月表示では、ズームを出さない", await p.locator("#zin").count() === 0);
    await p.keyboard.press("w"); await pause();
    await p.keyboard.press("?"); await pause();
    check("? でショートカットの一覧が開く", await p.locator("#keys").evaluate(d => d.open));
    await p.locator("#keys form button").click(); await pause();
    check("一覧の Close（form method=dialog）で閉じる", !(await p.locator("#keys").evaluate(d => d.open)));
    await p.keyboard.press("?"); await pause();
    await p.mouse.click(4, env.viewport.height - 4); await pause();
    check("一覧は枠の外を押しても閉じる", !(await p.locator("#keys").evaluate(d => d.open)));
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
    const [dl] = await Promise.all([p.waitForEvent("download"), p.locator("#yrsave").click()]);
    check("シェア用の画像を PNG で保存できる", /^kiroku-\d{4}\.png$/.test(dl.suggestedFilename()) && (await stat(await dl.path())).size > 1000, dl.suggestedFilename());
    await p.keyboard.press("Escape"); await pause();
    check("Esc で 1 年の露光を閉じる", !(await p.locator("#yr").evaluate(d => d.open)));
  });

  check("スクリプトのエラーがない", errors.length === 0, errors.join(" / "));
  check("CSP に止められたものがない", csp.length === 0, csp.join(" / "));
  if (env === envs[0]) await step("CSP が効いている", async () => { // 履歴から HTML がまぎれこんでも、スクリプトは動かない
    check("CSP の meta がある", /script-src 'sha256-/.test(await p.locator('meta[http-equiv="Content-Security-Policy"]').getAttribute("content")));
    await p.evaluate(() => { const s = document.createElement("script"); s.textContent = "window.kirokuInjected = 1"; document.body.append(s);
      document.body.insertAdjacentHTML("beforeend", '<img src="data:," onerror="window.kirokuInjected = 2">'); });
    await pause();
    check("あとから入れたスクリプトは動かない", await p.evaluate(() => window.kirokuInjected === undefined));
    check("止めたことが securitypolicyviolation で分かる", csp.length > 0, "違反が届かない");
    csp.length = 0; // ここでわざと起こした違反は数えない
  });
  await ctx.close();
  return { out, failed };
}
const results = await Promise.all(envs.map(run));
await b.close();
for (const r of results) console.log(r.out.join("\n"));
const failed = results.reduce((n, r) => n + r.failed, 0);
if (failed) { console.log(`\n${failed} 件失敗`); process.exit(1); }
console.log("\nすべて OK");
