// 画面の大事な流れが動くかを、ダミーデータの HTML をブラウザで操作して確かめる（CI の e2e）。
//   sh tools/screenshots/run.sh --html /tmp/kiroku.html && node tools/screenshots/smoke.mjs /tmp/kiroku.html
// Playwright が必要（このディレクトリで npm ci --ignore-scripts と npx playwright install chromium。版は package.json で固定）。
// 見るのは「動くか」だけで、見た目や言葉の良し悪しは docs/usability.md のテストで確かめる。
import { chromium } from "playwright";
import path from "node:path";

const html = process.argv[2];
if (!html) { console.error("使い方: node smoke.mjs <kiroku.html>"); process.exit(2); }
const url = "file://" + path.resolve(html);
// KIROKU_FAKE_NOW（例: 2026-03-01T08:00:00+09:00）があれば、ブラウザの時計をその時刻から進める。ダミーデータ（gen.py・mkgit.py）も
// 同じ時刻を「今」として作るので、e2e を走らせた日付や時刻で結果が変わらない（tools/check.sh と CI は決まった時刻を渡す）
const fakeNow = process.env.KIROKU_FAKE_NOW ? new Date(process.env.KIROKU_FAKE_NOW) : null;
if (fakeNow && isNaN(fakeNow)) { console.error(`KIROKU_FAKE_NOW を日時として読めません: ${process.env.KIROKU_FAKE_NOW}`); process.exit(2); }

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
// ダミーデータは「今」（KIROKU_FAKE_NOW）を基準に作るので、日によっては最初のブロックに、並行したセッションのブロックが重なる
async function clickableRun(p) {
  const i = await p.evaluate(() => [...document.querySelectorAll(".run[data-sid]")].findIndex(e => {
    e.scrollIntoView({ block: "center" });
    const b = e.getBoundingClientRect();
    return e.contains(document.elementFromPoint(b.left + b.width / 2, b.top + b.height / 2));
  }));
  return p.locator(".run[data-sid]").nth(Math.max(i, 0));
}

// loaded は、開いたページが期間の見出しとサマリーを描き終えるまで待つ（決まった時間だけ待つ代わりに）
const loaded = p => p.waitForFunction(() => document.querySelector("#rd") && document.querySelector("#rd").textContent.trim() !== "" && document.querySelector("#review .stat"));

// openContext は、画面の幅 env・言語 locale のブラウザを開く。スクリプトのエラーは errors に、CSP に止められたものは csp にためる
async function openContext(env, locale, csp) {
  const ctx = await b.newContext({ ...env, locale, timezoneId: "Asia/Tokyo", acceptDownloads: true, colorScheme: "dark" }); // 画面はいつもダーク（「テーマ」の手順で、OS がライトでも変わらないことを見る）
  ctx.setDefaultTimeout(5000);
  if (fakeNow) await ctx.clock.install({ time: fakeNow }); // 時計はこの時刻から普通に進む（タイマーも動く）
  // CSP の違反はページの中で拾って知らせる（関数を渡す仕組みは CSP の外なので止められない）
  await ctx.exposeFunction("kirokuCSPViolation", v => csp.push(v));
  await ctx.addInitScript(() => addEventListener("securitypolicyviolation", e => window.kirokuCSPViolation(`${e.effectiveDirective} ${e.blockedURI || "inline"} ${e.sourceFile || ""}:${e.lineNumber || ""}`)));
  await ctx.addInitScript(() => { try { if (localStorage.getItem("kiroku:insMore") == null) localStorage.setItem("kiroku:insMore", "true"); } catch (e) {} }); // 「その他のグラフ」を開いた状態で確かめる（たたむことは別に確かめる）
  const p = await ctx.newPage();
  const errors = [];
  p.on("pageerror", e => errors.push(e.message));
  p.on("console", m => { if (m.type() === "error") errors.push(m.text()); });
  return { ctx, p, errors };
}

async function run(env) {
  const out = [`# ${env.name}`];
  let failed = 0;
  const csp = []; // 流れ全体を通して、CSP に止められたもの（1 件でもあれば失敗）
  function check(what, ok, detail = "") {
    if (ok) out.push(`  ok   ${what}`);
    else { failed++; out.push(`  FAIL ${what}${detail ? `: ${detail}` : ""}`); }
  }
  const { ctx, p, errors } = await openContext(env, "en-US", csp);
  const pause = () => p.waitForTimeout(300);
  const drawerOpen = async () => await p.locator("#drawer").getAttribute("aria-hidden") === "false";
  // 1 つの流れが途中で止まっても、失敗として数えて次の流れへ進む（開き直して、記録のそろった先週から始める）
  const open = async () => { await p.goto(url); await loaded(p); await p.keyboard.press("ArrowLeft"); await pause(); };
  // 表示の切り替え（カレンダー / サマリー）。サマリーを触る確認は、サマリーに切り替えてから行い、終わったらカレンダーへ戻す
  const view = async v => { await p.locator(`#view button[data-v="${v}"]`).click(); await p.waitForTimeout(150); };
  async function step(name, fn) {
    const before = failed;
    try { await fn(); } catch (e) { check(name, false, e.message.split("\n")[0]); }
    if (failed > before) await open().catch(() => {});
  }

  await p.goto(url);
  await loaded(p);

  await step("読み込み", async () => {
    check("セッションが読み込まれている", await p.evaluate("DATA.length") > 0);
    if (fakeNow) check("ブラウザの時計は KIROKU_FAKE_NOW から進んでいる", await p.evaluate(t => Date.now() >= t && Date.now() - t < 10 * 60e3, fakeNow.getTime()));
    const [sw, iw] = await p.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
    check("横にはみ出さない", sw <= iw + 1, `${sw} > ${iw}`);
    check("絞り込んでいないときは、凡例に件数（20 / 20 sessions）を出さない", await p.locator("#legend .count").count() === 0);
    check("ロゴはこのファイルを開き直すリンク（/ ではディスクの一番上へ行く）", await p.evaluate(() => document.querySelector("#home").getAttribute("href") === location.href.split(/[?#]/)[0]));
    check("ロゴが検索欄に隠れない", await p.evaluate(() => document.querySelector(".brand .word").getBoundingClientRect().right <= document.querySelector(".search").getBoundingClientRect().left));
    // カレンダーが主役：① から下のパネルは閉じていて、上の帯は使ったものだけ
    check("サマリーは概要のカードだけ（折りたたむパネルはない）", await p.evaluate(() => document.querySelectorAll("#review details.panel.fold").length === 0 && document.querySelectorAll("#review .insight .inscol").length >= 8));
    check("上の帯は使ったものだけ（残したもの：コミット・変えた行・PR は出さない）", await p.locator('#kpis .kpi.out, #kpis [data-metric="commits"], #kpis [data-metric="lines"], #kpis [data-metric="aiCommits"], #kpis [data-metric="prs"]').count() === 0);
    if (env.viewport.width > 820) check("上の数字の帯は 1 行（折り返さない）", await p.evaluate(() => new Set([...document.querySelectorAll("#kpis .kpi")].map(k => Math.round(k.getBoundingClientRect().top))).size === 1));
    check("上の帯には前の期間との増減（%）を出さず、枠の色もそろう（増減は押したときの内訳に）", await p.evaluate(() => !document.querySelector("#kpis .kpi .dl") && !document.querySelector("#kpis .kpi.key")));
    { const b = p.locator('#kpis [data-metric="cost"], #kpis [data-metric="active"]').first(); await b.click(); await pause();
      check("帯の数を押すと、内訳の大きな数に前の期間との増減がある", /[+−±]\d*%/.test(await p.locator("#md").innerText()) || !(await p.evaluate(() => kpiDelta("cost") || kpiDelta("active"))));
      await p.keyboard.press("Escape"); await pause(); }
    check("履歴が前の期間の途中から始まるときは比べない", await p.evaluate(() => { const ok = vsPrev(period().P, "週").partial === false;
      historyStartCache = {d: DATA, n: DATA.length, t: Infinity}; const V = vsPrev(period().P, "週"), r = V.partial === true && V.of("cost", 5) === null && V.diff(3, V.of("cost", 5)) === "";
      historyStartCache = null; return ok && r; }));
    if (env.viewport.width <= 820){ // スマホの検索欄は虫めがねだけ。押すと広がり、画面からはみ出さない
      const box = () => p.evaluate(() => { const b = document.querySelector("#q").getBoundingClientRect(); return {l: b.left, r: b.right, w: b.width}; });
      const b0 = await box();
      await p.locator("#q").focus(); await pause(); const b1 = await box();
      check("スマホの検索欄は、閉じているあいだは虫めがねだけ", b0.w <= 40, JSON.stringify(b0));
      check("押すと広がり、画面に収まる", b1.w > 150 && b1.l >= 0 && b1.r <= env.viewport.width, JSON.stringify(b1));
      await p.locator("#q").blur(); await pause();
    }
    check("週の見出しは「Week x of y」（その年の週の数は 52 か 53）", /Week \d{1,2} of 5[23]\b/i.test(await p.locator("#ry").innerText()), await p.locator("#ry").innerText());
    check("週の数：2026 年は 53 週、2025 年は 52 週（12 月 28 日の週）", await p.evaluate(() => isoWeek(new Date(2026, 11, 28)) === 53 && isoWeek(new Date(2025, 11, 28)) === 52));
    check("既定はカレンダー：サマリーは隠れ、カレンダーが見える", await p.locator("#review .rvhead").isHidden() && await p.locator("#review .insight").isHidden() && await p.locator("#tl").isVisible());
    check("カレンダーの表示でも、データソースとページの下（版）は見える", await p.locator("#review .dsrc").isVisible() && await p.locator("#review .foot").isVisible());
    check("カレンダーとデータソースのあいだが空いている", await p.evaluate(() => document.querySelector("#review .metap").getBoundingClientRect().top - document.querySelector("#tl").getBoundingClientRect().bottom >= 16));
    check("期間の行に「期間」「表示」の見出しを並べない", await p.evaluate(() => [...document.querySelectorAll(".control-label")].every(e => e.getBoundingClientRect().width <= 1)));
    { const lim = p.locator("#tl .lim").first(); // 上限の印を押すと、そのセッションの流れのその出来事まで移って強調する
      if (await lim.count()){ await lim.scrollIntoViewIfNeeded(); await lim.click(); await pause();
        check("上限の印を押すと、流れの上限の出来事が強調される", await p.evaluate(() => !!document.querySelector("#panel .tl li.ev.warn.flash")));
        await p.keyboard.press("Escape"); await pause(); }
      else console.log("  skip この週に上限の印がない"); }
    check("0〜7 時を選んでいなければ、その週に 0〜7 時のセッションがあるときだけ広げる", await p.evaluate(async () => { const was = st.early; st.early = null; render(); await new Promise(r => setTimeout(r, 100));
      const ok = document.querySelector("#calfold").getAttribute("aria-expanded") === String(weekHasEarly()); st.early = was; render(); return ok; }));
    { // 0〜7 時は正しい位置に畳む（1/4 の高さ）。ボタンで広げ、もう一度で畳む。セッションの色は消えない
      const pos = () => p.evaluate(() => { const sp = [...document.querySelectorAll("#tl .hours span:not(.now)")], at = l => { const x = sp.find(e => e.textContent === l); return x ? parseFloat(x.style.top) : NaN; }; return {hh: parseFloat(getComputedStyle(document.querySelector("#tl .calscroll")).getPropertyValue("--hh")), h1: at("01:00"), h7: at("07:00"), h8: at("08:00"), exp: document.querySelector("#calfold").getAttribute("aria-expanded")}; });
      const a = await pos(), r = await p.evaluate(() => { const r = document.querySelector("#tl .run"), c = r && getComputedStyle(r).getPropertyValue("--c").trim(); return !!c && !/^\d/.test(c); });
      check("週のカレンダーは 0〜7 時を正しい位置で畳む（07:00 は 7/4 時間ぶん下、01:00 は出さない）", a.exp === "false" && Math.abs(a.h7 - 1.75 * a.hh) < 1 && Math.abs(a.h8 - 2.75 * a.hh) < 1 && Number.isNaN(a.h1) && r, JSON.stringify(a));
      await p.click("#calfold"); await pause(); const b = await pos();
      check("「0–7」で広げると 0 時から 1 時間ずつ", b.exp === "true" && Math.abs(b.h1 - b.hh) < 1 && Math.abs(b.h7 - 7 * b.hh) < 1, JSON.stringify(b));
      await p.click("#calfold"); await pause();
      check("もう一度押すと畳む", (await pos()).exp === "false");
    }
    check("カレンダーの表示に、サマリーの部品（エージェントの切り替えなど）が出ない", await p.evaluate(() => st.view !== "cal" || [...document.querySelectorAll("#review .insagbar, #review .insight")].every(e => !e.offsetParent)));
    check("スクロールはページだけ：カレンダーの中ではスクロールしない", await p.evaluate(() => { const c = document.querySelector("#tl .calscroll"); return !c || c.scrollHeight <= c.clientHeight + 1; }));
    { // どの幅でも、曜日の見出しがページの見出しの下に貼り付く（狭い画面は横のスクロールにも合わせる）
      await p.evaluate(() => window.scrollTo(0, document.querySelector("#tl").getBoundingClientRect().top + scrollY + 400)); await pause();
      check("スクロールしても曜日の見出しが見えている", await p.evaluate(() => { const h = document.querySelector("#tl .heads").getBoundingClientRect(), top = document.querySelector("header").getBoundingClientRect().bottom; return Math.abs(h.top - top) <= 2; }));
      await p.evaluate(() => window.scrollTo(0, 0)); await pause();
    }
    const at = () => p.evaluate(() => ["#rd", "#view"].map(q => { const r = document.querySelector(q).getBoundingClientRect(); return [Math.round(r.x), Math.round(r.y + scrollY)]; }));
    const pos0 = await at();
    await view("sum");
    check("よく見るカードが上に、その他のグラフはたたんだ箱にあり、初めはたたんである。開いたかは覚える", await p.evaluate(async () => { const top = [...document.querySelectorAll("#review .insg > .insrow h4")].map(h => h.textContent);
      store.set("insMore", false); summary(); const d0 = document.querySelector("#review details.insmore"); const closed = d0 && !d0.open && !d0.querySelector(".insrow").checkVisibility();
      d0.querySelector("summary").click(); await new Promise(r => setTimeout(r, 50)); const saved = store.get("insMore", false) === true; summary();
      return closed && saved && document.querySelector("#review details.insmore").open && top.some(x => /When you worked/.test(x)) && top.some(x => /Where it went/.test(x)) && !top.some(x => /^Trend/.test(x)); }));
    check("サマリーに切り替えると、カレンダーは隠れる", await p.locator("#tl").isHidden() && await p.locator("#review").isVisible());
    const pos1 = await at();
    check("切り替えても、期間の見出しと切り替えのボタンは動かない（上の帯の高さでずれない）", JSON.stringify(pos0) === JSON.stringify(pos1), `${JSON.stringify(pos0)} → ${JSON.stringify(pos1)}`);
    check("サマリーでは上の帯を見せず、押せない", !await p.evaluate(() => { const k = document.querySelector("#kpis"); const r = k.getBoundingClientRect(); return getComputedStyle(k).visibility === "visible" && getComputedStyle(k).display !== "none" && r.height > 0; }));
    check("サマリーでは、色分けと稼働時間の行（カレンダーの凡例）を出さない", await p.locator("#legend").isHidden() && await p.locator("#fnote").isHidden());
    await p.keyboard.press("v"); await pause();
    check("V でカレンダーに戻る", await p.locator("#tl").isVisible());
    check("カレンダーに戻ると、色分けと稼働時間の行がまた出る", await p.locator("#legend").isVisible());
  });

  await step("既定のままで消える履歴は、Data sources で残し方とバックアップ先を見せる", async () => {
    const r = await p.evaluate(() => { const save = META.archive; META.archive = {on: false, dir: "/Users/me/kiroku/<b>archive</b>"};
      const d = document.createElement("div"); d.innerHTML = keepRow({name: "Claude Code", n: 1, retention: {days: 30, setting: "cleanupPeriodDays", file: "~/.claude/settings.json"}}); META.archive = save;
      return {t: d.textContent, tags: d.querySelectorAll("b").length}; });
    check("フォルダが文字のまま出る", r.t.includes("/Users/me/kiroku/<b>archive</b>") && r.tags === 0, r.t);
    check("設定と kiroku archive on の 2 つの残し方があり、元のファイルは変えず外にも送らないと書いてある", /cleanupPeriodDays": 3650/.test(r.t) && /kiroku archive on/.test(r.t) && /own files are not changed/.test(r.t) && /nothing is sent anywhere/.test(r.t), r.t);
    check("要約の上に知らせの枠を出さない", await p.evaluate(() => !document.querySelector("#review .keep")));
  });

  await step("テーマ", async () => {
    check("言語とテーマの切り替えボタンはない", await p.locator("#lang").count() === 0 && await p.locator("#theme").count() === 0);
    await p.emulateMedia({ colorScheme: "light" }); await pause();
    check("OS がライトでも、画面はいつもダーク", await p.evaluate(() => getComputedStyle(document.documentElement).colorScheme === "dark" && getComputedStyle(document.body).backgroundColor === "rgb(20, 18, 24)"));
    await p.emulateMedia({ colorScheme: "dark" }); await pause();
  });

  await step("週の移動", async () => {
    await p.evaluate(() => document.activeElement && document.activeElement.blur()); // 表示の切り替え（←→ を自分で使う）から離れる
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
    await p.evaluate(() => scrollTo(0, 0)); await pause(); // ページを下にスクロールしたあとは、貼り付いた見出しの下をブロックが通るのが正しい。開いた位置（ページの先頭）で確かめる
    const cut = await p.evaluate(() => { const hd = document.querySelector("#tl .heads").getBoundingClientRect().bottom;
      return [...document.querySelectorAll("#tl .run")].filter(r => { const b = r.getBoundingClientRect(); return b.top < hd - 1 && b.bottom > hd + 1; }).length; });
    check("開いた位置で、日付の見出しに半分隠れたブロックがない", cut === 0, `${cut} 件`);
    check("過ぎた週は月曜から見える", await p.evaluate(() => document.querySelector("#tl .calscroll").scrollLeft) === 0);
    if (env.isMobile){ // スマホでは、右にまだ日があるあいだだけ右端をぼかす
      check("右にまだ日があると、右端がぼける", await p.evaluate(() => document.querySelector("#tl").classList.contains("morer")));
      check("右端のぼかしは、ヘッダー（検索のボタン）にかからない", await p.evaluate(() => { const h = document.querySelector("header"), f = getComputedStyle(document.querySelector("#tl"), "::after");
        return +getComputedStyle(h).zIndex > +f.zIndex; })); // 開いた位置まで下へ送られていると、カレンダーはヘッダーの下を通る
      await p.evaluate(() => { const sc = document.querySelector("#tl .calscroll"); sc.scrollLeft = sc.scrollWidth; }); await pause();
      check("右端までスクロールすると、ぼかしが消える", await p.evaluate(() => !document.querySelector("#tl").classList.contains("morer")));
      await p.evaluate(() => { document.querySelector("#tl .calscroll").scrollLeft = 0; }); await pause();
    }
  });

  await step("前の週のサマリーは「今週」と言わない", async () => {
    await p.keyboard.press("ArrowLeft"); await pause();
    await view("sum"); const subs = await p.evaluate(() => document.querySelector("#review .insight").innerText); await view("cal");
    check("前の週を見ているとき、サマリーに「this week」がない", !/this week/i.test(subs) && /Week shown/.test(subs), subs.slice(0, 200));
    check("クレジットは 10 未満なら小数 1 桁（1.33 を 1 と丸めすぎない）", await p.evaluate(() => crN(1.333) === "1.3" && crN(20) === "20" && crN(0.5) === "0.50"));
    await p.keyboard.press("t"); await pause();
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
      check("だれの発言か分かる", await rep.locator(".rpav svg").count() === 1);
      // 依頼と応答の間に起きたこと（コミットなど）は応答の上に出るので、それを飛ばした手前が依頼
      check("応答は依頼のすぐ後に並ぶ", await p.evaluate(() => { const li = [...document.querySelectorAll("#panel .tl li")], i = li.findIndex(x => x.classList.contains("rp"));
        let j = i - 1; while (j >= 0 && li[j].classList.contains("ev")) j--;
        return i > 0 && j >= 0 && li[j].classList.contains("pr"); }));
    }
    check("流れには、出し方の切り替えと凡例を置かない", await p.locator("#drawer #flowBy, #drawer .tlkeyd, #drawer .tlkey").count() === 0);
    { // プロンプトをコピーできる（ボタンには名前がある。写すのはそのプロンプトの文）
      const r = await p.evaluate(async () => { window.__copied = null; navigator.clipboard.writeText = t => { window.__copied = t; return Promise.resolve(); };
        const b = document.querySelector("#drawer .tl li.pr .pcopy"); if (!b) return {none: true};
        const s = DATA.find(x => x.id === st.sel), p = s.prompts[+b.dataset.i]; b.click(); await new Promise(r => setTimeout(r, 300));
        return {label: b.getAttribute("aria-label"), got: window.__copied, want: String(p.text || ""), cut: p.len > 0}; });
      check("プロンプトにコピーのボタンがあり、そのプロンプトを写す", !r.none && /Copy this prompt/.test(r.label) && (r.cut ? r.want.startsWith(r.got.slice(0, 20)) : r.got === r.want), JSON.stringify({...r, got: (r.got || "").slice(0, 40), want: (r.want || "").slice(0, 40)}));
      const q = await p.evaluate(async () => { window.__copied = null;
        const b = document.querySelector("#drawer .tl li.rp .rcopy"); if (!b) return {none: true};
        const s = DATA.find(x => x.id === st.sel), r = s.prompts[+b.dataset.i].reply; b.click(); await new Promise(z => setTimeout(z, 300));
        return {label: b.getAttribute("aria-label"), got: window.__copied, want: String(r.text || ""), cut: r.len > 0}; });
      check("AI の応答にもコピーのボタンがあり、その応答を写す", q.none || (/Copy this reply/.test(q.label) && (q.cut ? q.want.startsWith(q.got.slice(0, 20)) : q.got === q.want)), JSON.stringify({...q, got: (q.got || "").slice(0, 40), want: (q.want || "").slice(0, 40)}));
      const c = await p.evaluate(async () => { window.__copied = null; // 会話全体を Markdown で写す（見出しは発言者と時刻、本文はプロンプトの順）
        const b = document.querySelector("#drawer .flowhd .fcopy"); if (!b) return {none: true};
        const s = DATA.find(x => x.id === st.sel); b.click(); await new Promise(z => setTimeout(z, 600));
        return {label: b.getAttribute("aria-label"), got: window.__copied || "", first: String(s.prompts[0].text || "").slice(0, 30), n: s.prompts.length}; });
      check("会話全体を Markdown でコピーできる", !c.none && /Copy the conversation as Markdown/.test(c.label) && c.got.startsWith("# ") && (c.got.match(/^## You/gm) || []).length === c.n && c.got.includes(c.first), JSON.stringify({...c, got: c.got.slice(0, 80)}));
    }
    { // 前後のセッションへ（キーと、見出しの帯の ‹ ›）・キーで流れをスクロール・流れの全画面
      const id0 = await p.evaluate(() => st.sel);
      const nb = await p.evaluate(() => { const v = b => ({dis: b.disabled, label: b.getAttribute("aria-label") || ""}); return {shown: !$("#snav").hidden, prev: v($("#sprev")), next: v($("#snext"))}; });
      check("見出しの帯に前後のセッションへのボタンがあり、名前がある", nb.shown && /session/i.test(nb.prev.label) && /session/i.test(nb.next.label), JSON.stringify(nb));
      const mv = nb.next.dis ? {key: "ArrowLeft", back: "ArrowRight", btn: "#snext"} : {key: "ArrowRight", back: "ArrowLeft", btn: "#sprev"};
      await p.evaluate(() => { $("#panel").scrollTop = $("#panel").scrollHeight; document.activeElement.blur(); }); // 下まで読んだところから移る
      await p.keyboard.press(mv.key); await pause();
      const id1 = await p.evaluate(() => st.sel);
      const ft = await p.evaluate(() => { const P = $("#panel"), f = $("#panel .sflow").getBoundingClientRect(), r = P.getBoundingClientRect(); return {d: Math.round(f.top - r.top), h: P.clientHeight}; });
      check("移った先では、プロンプトの流れが頭から見えている", ft.d >= 0 && ft.d <= ft.h / 3, JSON.stringify(ft));
      check("← → で隣のセッションに移り、詳細は開いたまま", id1 && id1 !== id0 && await drawerOpen(), `${id0} → ${id1}`);
      await p.keyboard.press(mv.back); await pause();
      check("反対のキーで元のセッションに戻る", await p.evaluate(() => st.sel) === id0);
      const btn = mv.key === "ArrowRight" ? "#snext" : "#sprev";
      await p.locator(btn).click(); await pause();
      check("帯のボタンを押しても隣のセッションに移る", await p.evaluate(() => st.sel) === id1);
      await p.keyboard.press(mv.back); await pause();
      await p.evaluate(() => { $("#panel").scrollTop = 0; document.activeElement.blur(); });
      await p.keyboard.press("ArrowDown"); await p.keyboard.press("PageDown"); await pause();
      const y = await p.evaluate(() => [$("#panel").scrollTop, $("#panel").scrollHeight - $("#panel").clientHeight]);
      check("↓ と PageDown で詳細がスクロールする", y[1] <= 0 || y[0] > 0, JSON.stringify(y));
      await p.keyboard.press("f"); await pause();
      const full = await p.evaluate(() => { const b = $("#drawer").getBoundingClientRect(), f = $("#panel .sflow"), btn = $("#panel .ffull");
        return {on: $("#drawer").classList.contains("flowfull"), w: b.width, vw: innerWidth, flow: !!f && f.offsetParent !== null, pressed: btn && btn.getAttribute("aria-pressed")}; });
      check("F で流れが全画面になる", full.on && full.w >= full.vw - 1 && full.flow && full.pressed === "true", JSON.stringify(full));
      await p.keyboard.press("Escape"); await pause();
      check("Esc はまず全画面だけを解き、詳細は開いたまま", !await p.evaluate(() => $("#drawer").classList.contains("flowfull")) && await drawerOpen());
      await p.locator("#panel .ffull").click(); await pause();
      check("流れの見出しのボタンでも全画面になる", await p.evaluate(() => $("#drawer").classList.contains("flowfull")));
      await p.locator("#panel .ffull").click(); await pause();
      check("もう一度押すと元に戻る", !await p.evaluate(() => $("#drawer").classList.contains("flowfull")));
    }
    await p.keyboard.press("Escape"); await pause();
    check("Esc で詳細が閉じる", !await drawerOpen());
    await run.focus(); await p.keyboard.press("ArrowLeft"); await pause();
    check("ブロックにいたまま週を移っても、フォーカスがカレンダーに残る", await p.evaluate(() => !!document.activeElement.closest("#tl")), await p.evaluate(() => document.activeElement.tagName));
    await p.keyboard.press("ArrowRight"); await pause();
  });

  await step("詳細を閉じると読んでいた場所へ戻る", async () => {
    await view("sum");
    const card = p.locator("#review .ipt[data-sid]").first();
    check("サマリーにセッションを開く点（散布図）がある", await card.count() > 0);
    await card.scrollIntoViewIfNeeded();
    const id = await card.getAttribute("data-sid");
    await card.click(); await pause();
    const y = await p.evaluate("scrollY"); // 開いた時点の位置（クリックの前に Playwright がスクロールし直すことがあるので、開いてから測る。開いている間はページが動かない）
    await p.keyboard.press("Escape"); await pause();
    const after = await p.evaluate("scrollY");
    check("閉じてもサマリーの位置のまま", Math.abs(after - y) <= 2, `${y} → ${after}`);
    check("閉じると押した点にフォーカスが戻る", await p.evaluate(id => { const a = document.activeElement; return a.dataset.sid === id && !!a.closest("#review"); }, id));
    await view("cal");
  });

  await step("ガイドとズームはない", async () => {
    check("ヘッダーに指標ガイドのボタンがなく、ダイアログもない", await p.locator("#guide, #mg").count() === 0);
    const wk = await p.evaluate(() => document.querySelector("#rd").textContent);
    await p.keyboard.press("g"); await p.keyboard.press("+"); await p.keyboard.press("-"); await pause();
    check("G・+・− を押しても何も開かず、期間も変わらない", await p.evaluate(wk => document.querySelector("#rd").textContent === wk && ![...document.querySelectorAll("dialog")].some(d => d.open), wk));
    check("週のカレンダーにズームの操作がない", await p.locator("#legend .zoom, #zin, #zout").count() === 0);
    check("ショートカットの一覧に G・ズームがない", !/Metrics guide|Zoom/.test(await p.locator("#keys").textContent()));
  });

  await step("サマリーの数字から内訳が開く", async () => {
    await view("sum");
    { // サマリーの頭：要点の文・推移といつもの範囲のグラフ・改善のアドバイス（見直す候補の上位 3 つまで）
      const r = await p.evaluate(() => ({heads: document.querySelectorAll("#review .insight .itile").length, cum: document.querySelectorAll("#review .insfig svg polyline.icum").length, donut: document.querySelectorAll("#review .iwr").length, hist: document.querySelectorAll("#review .insfig .ihb").length, svg: !!document.querySelector("#review .insfig svg[aria-label]"), bars: document.querySelectorAll("#review .insfig .ibar").length,
        adv: document.querySelectorAll("#review .insadvc, #review .insadv").length, flags: (lastFlags || []).length, tabs: document.querySelectorAll("#review [data-ins]").length}));
      check("上の帯の数字（推定費用・稼働時間）はサマリーのカードにもあり、押すと同じ内訳が開く", await p.evaluate(() => ["cost", "active"].every(m => document.querySelector(`#review .itile [data-metric="${m}"]`))));
      check("トークンの種類別のカード（種類ごとの積み上げ棒）がある", await p.evaluate(() => !!document.querySelector("#review .itbar") && document.querySelectorAll("#review .itbar > span").length >= 2));
      check("見直す候補の並びは、いつもより悪い側に外れたもの → 比べられないもの → 範囲の中（同じ段は関わる費用の大きい順）。「まず見直す」の行と期間の行の ⚠ は出さない", await p.evaluate(() => {
        const F = lastFlags, side = f => insSide(f), ok = F.every((f, i) => i === 0 || side(F[i - 1]) < side(f) || side(F[i - 1]) === side(f) && ((F[i - 1].stake && F[i - 1].stake.c) ?? -1) >= ((f.stake && f.stake.c) ?? -1));
        return ok && !document.querySelector("#review .icall") && !document.querySelector("#worth"); }));
      check("見直す候補はどれも、サマリーの指標の横の印から開ける（「まず見直す」の行はない）", await p.evaluate(() => [...new Set(lastFlags.map(f => GOTO[f.k] || f.k))].every(id => document.querySelector(`#review .fmark[data-goto="${id}"]`))));
      check("エージェント別の表に、エージェントごとの行がある（費用とクレジットを混ぜない）", await p.evaluate(() => document.querySelectorAll("#review .iagt tbody tr").length >= 2));
      { // エージェントで絞る：サマリーの数字がそのエージェントの分になり、上の帯（すべて）は変わらない。戻すと元の数字
        const ags = await p.evaluate(() => [...document.querySelectorAll("#review [data-ag]")].map(b => b.dataset.ag));
        check("サマリーにエージェントの切り替えがある（すべて + 2 つ以上）", ags.length >= 3 && ags[0] === "");
        if (ags.length >= 3){
          const val = () => p.evaluate(() => (document.querySelector('#review .itile [data-metric="active"]') || {}).textContent || "");
          const band = () => p.evaluate(() => document.querySelector(".kpis").textContent);
          const all = await val(), b0 = await band();
          await p.click(`#review [data-ag="${ags[2]}"]`); await pause();
          const one = await val(), b1 = await band();
          check("絞ると稼働時間が変わり、エージェント別の表は消え、上の帯は変わらない", one !== all && b1 === b0 && await p.evaluate(() => !document.querySelector("#review .iagt") && !!document.querySelector('#review [data-ag][aria-pressed="true"]:not([data-ag=""])')));
          check("絞ったまま、カレンダーのセッションは減らない", await p.evaluate(() => DATA.length > 0 && new Set(DATA.map(s => s.source)).size >= 2));
          await p.click('#review [data-ag=""]'); await pause();
          check("「すべて」に戻すと元の数字", await val() === all);
          if (ags.length >= 4){ // A から B に切り替えた結果と、B を直接開いた結果が同じ（前のエージェントの候補が残らない）
            const advice = () => p.evaluate(() => [...document.querySelectorAll("#review .fmark")].map(e => e.dataset.goto).join("|"));
            await p.click(`#review [data-ag="${ags[1]}"]`); await pause(); await p.click(`#review [data-ag="${ags[3]}"]`); await pause(); const viaA = await advice();
            await p.click('#review [data-ag=""]'); await pause(); await p.evaluate(() => { DATA = DATA.slice(); }); await p.click(`#review [data-ag="${ags[3]}"]`); await pause(); const direct = await advice();
            check("エージェントを切り替えても、前のエージェントのアドバイスが残らない", viaA === direct, `${viaA} / ${direct}`);
            await p.click('#review [data-ag=""]'); await pause(); }
        }
      }
      check("ヒートマップの各行に曜日の合計がある（触れなくても読める）", await p.evaluate(() => document.querySelectorAll("#review .ihgrid > .iht:not(.ihh)").length === 7));
      check("3 列の並びの最後の段に空きを作らない（余りは横に広げる）", await p.evaluate(() => [...document.querySelectorAll("#review .istats, #review .ikrow")].every(g => { const k = [...g.children].filter(c => c.offsetParent !== null), /* 単価の切り替えで隠した数字は数えない */ cols = getComputedStyle(g).gridTemplateColumns.split(" ").length; if (k.length < 2) return true; const r = k.map(c => c.getBoundingClientRect()), last = r[r.length - 1], row = r.filter(b => Math.abs(b.top - last.top) < 2); return Math.abs(row[row.length - 1].right - g.getBoundingClientRect().right) < 2 || row.length === cols; })));
      check("カードの中の数の枠は最大 3 列", await p.evaluate(() => [...document.querySelectorAll("#review .istats")].every(g => getComputedStyle(g).gridTemplateColumns.split(" ").length <= 3)));
      check("サマリーの頭はコスト・出力・単価の 3 枚のパネルで、どれもグラフ 1 つ（選んだ選択肢のタイルだけが見える）。広い画面では横に 3 枚、狭ければ上から順に積み、どの数字も画面の中に見える", await p.evaluate(() => { const G = [...document.querySelectorAll("#review .insh .ikg")], g = document.querySelector("#review .insh").getBoundingClientRect();
        const k = [...document.querySelectorAll("#review .insh .ikrow > .itile")].filter(c => c.offsetParent !== null), inside = k.every(c => { const r = c.getBoundingClientRect(); return r.left >= g.left - 1 && r.right <= g.right + 1; });
        const one = G.every(c => [...c.querySelectorAll(".ikrow > [data-ch]")].filter(x => x.checkVisibility()).length === 1 && c.querySelectorAll(".ikrow > .itile .tspk svg").length >= 1);
        const r = c => document.querySelector(`#review .insh .ik-${c}`).getBoundingClientRect();
        const lay = innerWidth >= 1100 ? Math.abs(r("out").top - r("cost").top) < 2 && Math.abs(r("unit").top - r("cost").top) < 2 && r("out").left >= r("cost").right - 1 && r("unit").left >= r("out").right - 1
          : r("out").top >= r("cost").bottom - 1 && r("unit").top >= r("out").bottom - 1 && ["out", "unit"].every(c => Math.abs(r(c).left - r("cost").left) < 2 && Math.abs(r(c).width - r("cost").width) < 2);
        return G.length === 3 && k.length === 3 && one && inside && lay && !document.querySelector("#review .icall"); }));
      { // パネルの切り替え：押すと別の選択肢のタイル（とグラフ）になり、開き直しても覚えている
        const sws = await p.evaluate(() => [...document.querySelectorAll("#review .iksw")].map(g => ({sw: g.dataset.sw, vs: [...g.querySelectorAll("[data-v]")].map(b => b.dataset.v), cur: g.querySelector('[aria-pressed="true"]').dataset.v})));
        const want = {cost: ["cost", "credits", "tokens", "active"], out: ["commit", "push", "pr"], unitN: ["cost", "credits", "tokens"], unitD: ["commit", "push", "pr"]};
        check("どのパネルにも切り替えがあり（単価は量と割る数の 2 つ）、記録のある選択肢が並ぶ", Object.keys(want).every(k => { const x = sws.find(z => z.sw === k); return x && want[k].every(v => x.vs.includes(v)); }), JSON.stringify(sws));
        const shown = sw => p.evaluate(sw => { const g = document.querySelector(`#review .iksw[data-sw="${sw}"]`), sec = g.closest(".ikg"), v = [...sec.querySelectorAll(".ikrow > [data-ch]")].filter(x => x.checkVisibility());
          return {n: v.length, ch: v[0] && v[0].dataset.ch, svg: v[0] ? (v[0].querySelector(".tspk svg") || {}).innerHTML || "" : "", label: v[0] ? (v[0].querySelector(".il") || v[0]).textContent : ""}; }, sw);
        const picked = {};
        for (const {sw, vs, cur} of sws){
          const before = await shown(sw), to = vs.find(v => v !== cur), b = p.locator(`#review .iksw[data-sw="${sw}"] [data-v="${to}"]`);
          await b.click(); await pause(); const after = await shown(sw);
          check(`切り替え ${sw}: ${to} を押すと、そのタイルだけが見え、グラフが変わり、ボタンが選ばれてフォーカスが残る`, after.n === 1 && after.ch.split(":")[sw === "unitD" ? 1 : 0] === to && after.svg !== before.svg && after.label !== before.label
            && await b.getAttribute("aria-pressed") === "true" && await b.evaluate(e => document.activeElement === e && e.closest('[role="group"]') !== null), `${JSON.stringify(before).slice(0, 80)} → ${JSON.stringify(after).slice(0, 80)}`);
          picked[sw] = {to, ch: after.ch};
        }
        // 選択は覚えておき（localStorage）、描き直してもそのまま。file:// を開き直すと、Chromium が localStorage を空で始めることがあるので、開き直しではなくサマリーを描き直して見る
        const kept = await p.evaluate(() => ({costKind: store.get("costKind"), outKind: store.get("outKind"), unitKind: store.get("unitKind"), unitDen: store.get("unitDen")}));
        check("切り替えの選択は localStorage に覚えている", kept.costKind === picked.cost.to && kept.outKind === picked.out.to && kept.unitKind === picked.unitN.to && kept.unitDen === picked.unitD.to, JSON.stringify(kept));
        await view("cal"); await view("sum"); await p.evaluate(() => summary()); await pause();
        for (const sw of Object.keys(picked)){ const x = await shown(sw), pr = await p.locator(`#review .iksw[data-sw="${sw}"] [aria-pressed="true"]`).getAttribute("data-v");
          check(`切り替え ${sw} の選択は、描き直しても覚えている`, pr === picked[sw].to && x.ch.split(":")[sw === "unitD" ? 1 : 0] === picked[sw].to, `${pr} ${x.ch}`); }
        for (const {sw, cur} of sws){ await p.locator(`#review .iksw[data-sw="${sw}"] [data-v="${cur}"]`).click(); await pause(); } // 元に戻す（ほかの確かめは最初の選択肢で）
      }
      check("式はふだん隠し、指標名の横の「ƒx」で、その数字の式だけを開く（もう一度押すと閉じる）", await p.evaluate(async () => {
        const vis = el => [...el.querySelectorAll(".ifx.fxo")].some(e => e.offsetParent !== null), I = document.querySelector("#review .insight");
        const b = I.querySelector(".itile .fxb"), tile = b && b.closest(".itile"); if (!tile || vis(I) || document.querySelector("#fxbtn")) return false;
        b.click(); await new Promise(r => setTimeout(r, 100)); const on = vis(tile) && b.getAttribute("aria-pressed") === "true" && [...I.querySelectorAll(".itile")].filter(x => x !== tile).every(x => !vis(x));
        b.click(); await new Promise(r => setTimeout(r, 100)); return on && !vis(I); }));
      check("サマリーの頭に、数字 5〜32 個（上の帯の数字を含む。単価は Git コミット・AI の編集行・稼働時間・セッション・プロンプトあたりの 5 段。コスト・出力（Git はコミット・行・ファイル・リポジトリ・日・push・PR、AI の編集はファイルと行）・単価（Git コミットあたりと AI が編集した 100 行あたりを分ける）に分ける）・推移・累積の線・どこに使ったかの横棒・長さ別の費用があり、改善のアドバイスの枠はない", r.heads >= 5 && r.heads <= 32 && r.svg && r.bars > 0 && r.cum === 1 && r.donut > 0 && r.hist === 6 && r.adv === 0, JSON.stringify(r));
      const fx = await p.evaluate(() => ({chips: [...document.querySelectorAll("#review .insight .ichip.ifx")].map(e => e.textContent), def: (document.querySelector("#review .insdef") || {}).textContent || "",
        vague: /Usual|Higher|Lower|usual range|いつもの範囲|いつもどおり|高め|低め|\bQ[13]\b|IQR/.test(document.querySelector("#review .insight").innerText)}));
      check("いつもとの比べは不等式（下位・上位25%点の値）で出し、定義を 1 行で示す。曖昧な言葉と Q1・Q3 の記号は使わない", fx.chips.length > 0 && fx.chips.every(c => /pct|%点|≤|needs|必要/.test(c)) && /(spread|幅) = /.test(fx.def) && !fx.vague, JSON.stringify(fx).slice(0, 300));
      const lay = await p.evaluate(() => { const cs = [...document.querySelectorAll("#review .insight .inscol")]; const say = cs.filter(c => c.querySelector(".insfig, .ihgrid") && c.querySelector(".isay")).length, charts = cs.filter(c => c.querySelector(".insfig, .ihgrid")).length;
        // グラフのカードは見出しごとの石積み（.insrow）。広い画面では 3 列。カードは重ならず、どのカードも中の下が空かない（中身の高さのまま置く）
        const inner = c => { const cb = c.getBoundingClientRect(); let bot = cb.top; c.querySelectorAll("*").forEach(e => { const r = e.getBoundingClientRect(); if (r.height && getComputedStyle(e).display !== "none") bot = Math.max(bot, r.bottom); }); return cb.bottom - bot; };
        const rowsOk = [...document.querySelectorAll("#review .insrow")].every(r => { const bs = [...r.children].map(c => c.getBoundingClientRect());
          const apart = bs.every((a, i) => bs.every((b, j) => i >= j || a.right <= b.left + 1 || b.right <= a.left + 1 || a.bottom <= b.top + 1 || b.bottom <= a.top + 1));
          return apart && [...r.children].every(c => inner(c) <= 40); });
        // 上の数字のグラフ：1 つだけ残ったタイル（段）も、行の幅いっぱいに広がる（右が空かない）
        const badTiles = [...document.querySelectorAll("#review .ikrow")].filter(r => r.offsetParent).filter(r => { const rr = r.getBoundingClientRect(), ts = [...r.children].filter(c => c.offsetParent && c.querySelector(".tspk"));
          return ts.length && Math.abs(Math.max(...ts.map(c => c.getBoundingClientRect().right)) - rr.right) >= 4; }).map(r => (r.closest(".iksub, .ikg") || r).className + " " + JSON.stringify([...r.children].map(c => [c.className.slice(0, 20), (c.querySelector(".il") || {}).textContent, Math.round(c.getBoundingClientRect().left), Math.round(c.getBoundingClientRect().right), !!c.querySelector(".tspk")])) + " " + Math.round(r.getBoundingClientRect().right));
        // カードもタイルも単価の段も、広い画面ではページの半分より広くない（グラフのない数字の行は除く）
        const half = innerWidth < 1100 || [...document.querySelectorAll("#review .insrow > *, #review .insh > .ikg, #review .ikrow > .itile:has(.tspk)")].filter(e => e.checkVisibility()).every(e => e.getBoundingClientRect().width <= innerWidth / 2 + 1);
        const tilesOk = half && !badTiles.length
;
        const three = innerWidth < 1300 || [...document.querySelectorAll("#review .insrow")].some(r => r.children.length >= 3 && Math.abs(r.children[0].getBoundingClientRect().top - r.children[2].getBoundingClientRect().top) < 1);
        return {say, charts, rows: rowsOk, tiles: tilesOk, half, badTiles, grid: three}; });
      check("どのグラフにも、データから計算した読み取りの 1 文があり、広い画面ではカードが 3 列に並び、カードは重ならず中の下が空かない。カードとグラフのタイルはページの半分より広くなく、奇数のタイルも行の幅を埋める", lay.say === lay.charts && lay.rows && lay.tiles && lay.grid, JSON.stringify(lay));
      check("コストのグラフの各点に、前の期間からの増減（%）がある", await p.evaluate(() => { const l = [...document.querySelectorAll("#review .ik-cost .tspk:not(.ww) .spc")]; return l.length > 0 && l.every(e => /^[+−±]\d+%$/.test(e.textContent)); }));
      const tk = await p.evaluate(() => { const g = document.querySelector("#review .insk.trendk"), bs = [...g.querySelectorAll("button")], sel = document.querySelector("#review .insksel");
        const ks = (getComputedStyle(g).display === "none" ? [...sel.options].map(o => o.value) : bs.map(b => b.dataset.ins));
        return {tabs: getComputedStyle(g).display !== "none", oneRow: new Set(bs.map(b => Math.round(b.getBoundingClientRect().top))).size === 1, ks}; });
      check("トレンドでクレジットとトークンも選べる", tk.ks.includes("credits") && tk.ks.includes("tokens"), JSON.stringify(tk.ks));
      check("トレンドの指標の切り替えは 1 行に収まるか、収まらなければ選択リストになる（折り返さない）", !tk.tabs || tk.oneRow, JSON.stringify(tk));
      if (tk.tabs){
        const t0 = await p.locator('#review [data-ins][aria-pressed="true"]').getAttribute("data-ins");
        await p.locator('#review [data-ins]:not([aria-pressed="true"])').first().click(); await pause();
        check("グラフの指標を切り替えられる（押したものが選ばれ、フォーカスが残る）", await p.evaluate(t0 => { const b = document.querySelector('#review [data-ins][aria-pressed="true"]'); return !!b && b.dataset.ins !== t0 && document.activeElement === b; }, t0));
        await p.locator(`#review [data-ins="${t0}"]`).click(); await pause();
      } else { // 狭い画面（と切り替えが 1 行に収まらない幅）では選択リスト。どの指標も選べる（横に隠れない）
        const opts = await p.evaluate(() => [...document.querySelectorAll("#review .insksel option")].map(o => o.value));
        let all = opts.length > 1; for (const v of opts){ await p.selectOption("#review .insksel", v); await pause(); all = all && await p.evaluate(v => st.insK === v && document.querySelector("#review .insksel").value === v, v); }
        check("狭い画面では、グラフの指標を選択リストで選べ、どれも選べる", all && await p.evaluate(() => getComputedStyle(document.querySelector("#review .insk.trendk")).display === "none"), JSON.stringify(opts));
        await p.selectOption("#review .insksel", opts[0]); await pause(); }
      const h = await p.evaluate(() => ({rows: document.querySelectorAll("#review .ihgrid .ihd").length, cells: document.querySelectorAll("#review .ihgrid .ihc").length, pts: document.querySelectorAll("#review .insfig .ipt[data-sid]").length, ptab: document.querySelectorAll("#review .ptab tbody tr").length, pcard: document.querySelectorAll("#review .pcard").length}));
      check("曜日×時間のヒートマップ（7×24）と費用と成果の散布図があり、プロジェクトのカードの列はない", h.rows === 7 && h.cells === 168 && h.pts > 0 && h.pcard === 0, JSON.stringify(h));
      const sid = await p.locator("#review .insfig .ipt[data-sid]").first().getAttribute("data-sid");
      await p.locator("#review .insfig .ipt[data-sid]").first().focus(); await p.keyboard.press("Enter"); await pause();
      check("散布図の点から、そのセッションが開く（キーボードでも）", await p.evaluate(sid => st.sel === sid, sid));
      await p.keyboard.press("Escape"); await pause();
    }
    const v = p.locator('#review .itile [data-metric="cost"]');
    check("サマリーの推定費用（上の 1 枚目）を押せる", await v.count() === 1);
    await v.scrollIntoViewIfNeeded(); await v.click(); await pause();
    check("押すと内訳のダイアログが開く", await p.evaluate(() => document.querySelector("#md").open && document.querySelector("#mdh").textContent === "Estimated cost"));
    const wk = await p.evaluate(() => document.querySelector("#rd").textContent);
    await p.keyboard.press("ArrowLeft"); await pause();
    check("内訳を開いているあいだは ← で期間が動かない", await p.evaluate(wk => document.querySelector("#rd").textContent === wk && document.querySelector("#md").open, wk));
    check("指標の定義は大きな数字のすぐ下にあり、言えないことは最後にある", await p.evaluate(() => { const d = document.querySelector("#md"), def = d.querySelector(".mbig + .mdef"); return !!def && def.textContent.length > 0; }));
    await p.keyboard.press("Escape"); await pause();
    check("閉じると押した数字にフォーカスが戻る", await p.evaluate(() => document.activeElement.dataset.metric === "cost" && !!document.activeElement.closest("#review")));
    const hb = p.locator("#review .stat.mstat:has([data-metric=focus]) .hb");
    await hb.scrollIntoViewIfNeeded(); await hb.click(); await pause();
    check("枠の中の ? は説明を開き、内訳は開かない", await p.evaluate(() => !document.querySelector("#hpop").hidden && !document.querySelector("#md").open));
    await p.keyboard.press("Escape"); await pause();
    await view("cal");
  });

  await step("月末の推定の幅は 3σ だけ", async () => {
    await p.keyboard.press("m"); await pause();
    const has = await p.locator('#kpis [data-metric="projection"], #kpis [data-metric="projectionCr"]').count();
    if (has){
      await p.locator('#kpis [data-metric="projection"], #kpis [data-metric="projectionCr"]').first().click(); await pause();
      const r = await p.evaluate(() => ({rows: [...document.querySelectorAll("#md .mband tbody tr th")].map(e => e.textContent), lines: [...document.querySelectorAll("#md .mbl")].map(e => e.className)}));
      check("幅の表と点線は 3σ だけ", r.rows.every(x => !/^[12]σ/.test(x)) && r.lines.every(c => /\bk3\b/.test(c)), JSON.stringify(r));
      await p.keyboard.press("Escape"); await pause();
    }
    await p.keyboard.press("w"); await pause();
  });

  await step("サマリーのどの数字も内訳が開く", async () => {
    await view("sum");
    const ids = await p.evaluate(() => [...document.querySelectorAll("#review [data-metric]")].map(b => b.dataset.metric));
    for (const id of ["lines", "focus", "wait", "cache", "costPerAsk"]) check(`サマリーの ${id} を押せる`, ids.includes(id));
    for (const id of ["files", "pushes", "parallel", "ai"]) check(`サマリーに ${id} は出さない（数を絞る）`, !ids.includes(id));
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
    await view("cal");
  });

  await step("セッションの振り返りのプロンプト", async () => {
    const run = await clickableRun(p);
    await run.scrollIntoViewIfNeeded(); await run.click(); await pause();
    check("振り返りは、プロンプトをコピーするボタンだけ（AI を動かすボタンはない）", await p.locator("#panel #srun.ai-copy").count() === 1 && await p.locator("#panel .ai-action").count() === 0);
    { const tx = await p.locator("#panel .drun").innerText(); check("振り返りのボタンの下は、エージェントに貼るプロンプトをコピーすると書くだけ（何も送らないとはあえて書かない）", /(Copies a review prompt|振り返りのプロンプトをコピー)/.test(tx) && !/(sends nothing|何も送りません)/.test(tx), tx); }
    check("AI が動いた時間は、あなたの吹き出しの中に入れない", await p.evaluate(() => !document.querySelector("#panel .tl li.pr:not(.ev) .pmeta")));
    // 左（1 列の画面では間）はプロンプトの流れだけ。再開のコマンドは操作（.sact）に 1 つだけ見え、2 列なら右の列、1 列なら流れより上
    const rs = await p.evaluate(() => { const v = [...document.querySelectorAll("#panel .dresume")].filter(e => e.offsetParent !== null);
      const act = document.querySelector("#panel .sact"), flow = document.querySelector("#panel .sflow");
      if (!act || !flow) return {n: v.length, ok: false};
      const a = act.getBoundingClientRect(), f = flow.getBoundingClientRect(), wide = innerWidth >= 1100;
      return {n: v.length, inAct: !v.length || !!v[0].closest(".sact"), wide, place: wide ? a.left > f.right - 1 : a.top < f.top,
        flowOnly: !flow.querySelector(".mini, .files, .trow, .dresume, #srun"),
        // HTML の順（Tab の順）が見た目の順と同じ：操作 → 数値 → 流れ → 情報。1 列ではモデル・ツールは流れより下
        order: [".sact", ".smet", ".sflow", ".sinfo"].map(c => document.querySelector("#panel .scols > " + c)).filter((e, i) => e || i).every((e, i, a) => e && (!i || (a[i-1].compareDocumentPosition(e) & Node.DOCUMENT_POSITION_FOLLOWING))),
        toolsBelow: wide || [...document.querySelectorAll("#panel .trow")].every(e => e.getBoundingClientRect().top > f.bottom - 1)}; });
    check("詳細の HTML の順は、操作・数値・流れ・情報（Tab の順が見た目と合う）", rs.order === true, JSON.stringify(rs));
    check("1 列では、使ったツールは流れより下", rs.toolsBelow === true, JSON.stringify(rs));
    check("詳細の流れの列には、プロンプトの流れだけがある", rs.flowOnly === true, JSON.stringify(rs));
    check("再開のコマンドと AI 分析は操作にあり、2 列なら右の列、1 列なら流れより上", rs.n <= 1 && rs.inAct && rs.place, JSON.stringify(rs));
    const rv = await p.evaluate(() => { const s = DATA.find(x => x.id === st.sel) || DATA[0]; return sessionPrompt(s, 1, null); });
    check("振り返りは専門的な助言と証拠を備える", /professional AI prompt advisor/.test(rv) && /^- Title: /m.test(rv) && /^### Professional prompt assessment$/m.test(rv) && /^### Prioritized advice$/m.test(rv) && /^### Reusable prompt playbook$/m.test(rv) && /^\| \| This session \|$/m.test(rv) && /^\| Prompts \| \d+ \|$/m.test(rv), rv.slice(0, 200));
    check("振り返りの事実は、コードブロックの中にある（履歴ファイルを読めるなら、セッション ID とファイルと、履歴にない出来事だけ。プロンプトは書き写さない）", /# History data\n`{3,}text\n# Facts/.test(rv) && (/- Session ID: .+\n- History file: /.test(rv) ? /## Events kiroku adds/.test(rv) && !/## Prompt flow/.test(rv) : /## Prompt flow/.test(rv)), rv.slice(-400));
    await p.keyboard.press("Escape"); await pause();
  });

  await step("詳細と矢印キー・ブラウザの戻る", async () => {
    const label = () => p.locator("#rd").innerText(), before = await label();
    const run = await clickableRun(p);
    await run.scrollIntoViewIfNeeded(); await run.click(); await pause();
    check("詳細が開く", await drawerOpen());
    const id0 = await p.evaluate(() => st.sel), hasPrev = await p.evaluate(() => !$("#sprev").disabled);
    await p.keyboard.press("ArrowLeft"); await pause();
    check("詳細を開いているあいだの ← は週ではなく前のセッションへ移る（週はそのセッションに合わせる）", await drawerOpen() && await p.evaluate(([id0, hasPrev]) => {
      const s = DATA.find(x => x.id === st.sel); return (hasPrev ? st.sel !== id0 : st.sel === id0) && +mondayOf(new Date(s.start * 1000)) === +st.week; }, [id0, hasPrev]), await label());
    if (hasPrev){ await p.keyboard.press("ArrowRight"); await pause(); }
    check("→ で元のセッションと週に戻る", await p.evaluate(() => st.sel) === id0 && await label() === before, await label());
    await p.goBack(); await pause();
    check("ブラウザの戻るで詳細が閉じ、ページに残る", !await drawerOpen() && await p.evaluate(() => typeof DATA === "object"), p.url());
    await (await clickableRun(p)).click(); await pause(); // 戻ると位置が変わり、さっきのブロックが隠れることがある
    await p.keyboard.press("Escape"); await pause();
    check("Esc で閉じると、積んだ履歴も消える", !await drawerOpen() && await p.evaluate(() => history.state === null));
    await p.locator("#mode button").first().focus(); await p.keyboard.press("ArrowRight"); await pause();
    check("切り替えのボタンにいるときは → で週が変わらない", await label() === before, await label());
  });

  await step("コストと残ったもの", async () => {
    await view("sum");
    const oc = p.locator("#review .inscol:has(.hb[data-help=\"outSessions\"])"); await oc.scrollIntoViewIfNeeded(); await pause();
    // コミット・push・PR の数は上の「出力」のパネルに（下の成果のカードでは繰り返さない）。Git の行・やり直し、AI の編集の定着は成果のカードに
    for (const [name, id] of [["コミット", "gitCommits"], ["push", "pushes"], ["PR", "prs"]])
      check(`上の出力のパネルに${name}が出て、下の成果のカードでは繰り返さない`, await p.locator(`#review .insh .ik-out .hb[data-help="${id}"]`).count() === 1 && await oc.locator(`.hb[data-help="${id}"]`).count() === 0);
    const rwk = await p.evaluate(() => { const {ws, we} = period(); return reworkOf(ws, we).n > 0; }); // やり直しは 3 日たった AI のコミットがある期間だけ
    for (const id of ["lines", ...(rwk ? ["rework"] : []), "kept", "aiLines", "aiFiles"])
      check(`パネルから外した ${id} は成果のカードにある`, await oc.locator(`.hb[data-help="${id}"]`).count() === 1 && await p.locator(`#review .insh .hb[data-help="${id}"]`).count() === 0);
    { // パネルから外した単価は、単価の表のカードに。HELP のある指標は、どれもサマリーのどこかに「?」がある
      const miss = await p.evaluate(() => ["costPerEdit", "costPerHour", "costPerDay", "costPerSes", "costPerAsk", "tokensPerEdit", "tokensPerHour", "tokensPerDay", "tokensPerSes", "tokensPerAsk", "creditsPerHour", "creditsPerSes", "creditsPerAsk"].filter(k => !document.querySelector(`#review .iunits .hb[data-help="${k}"]`)));
      check("単価の表のカードに、100 行・時間・日・セッション・プロンプトあたりの単価がある", miss.length === 0, miss.join(","));
      const none = await p.evaluate(rwk => ["cost", "credits", "tokens", "active", "gitCommits", "lines", "prs", "pushes", "aiLines", "aiFiles", ...(rwk ? ["rework"] : []), "kept", "outSessions", "costPerCommit", "tokensPerCommit", "perPush", "perPr", "prompts", "cache", "subagents", "fix", "limits", "focus", "switches", "wait", "bigPrompts", "repeats", "friction", "compactions"].filter(k => !document.querySelector(`#review .hb[data-help="${k}"]`)), rwk);
      check("HELP のある指標は、サマリーのどこかに「?」がある", none.length === 0, none.join(","));
    }
    check("成果のカードに成果まで届いたセッションが出る", await oc.locator('.hb[data-help="outSessions"]').count() === 1);
    check("コミットあたりの費用は、出力ではなく単価の段にある", await p.evaluate(() => { const G = [...document.querySelectorAll("#review .insh .ikg")]; return G.length === 3 && !!G[2].querySelector('[data-metric="costPerCommit"]') && !document.querySelector('#review .iout .hb[data-help="costPerCommit"]'); }));
    check("割った指標には式（÷）が添えてある", await oc.evaluate(e => [...e.querySelectorAll(".stat")].filter(s => s.querySelector('.hb[data-help="outSessions"], .hb[data-help="costPerCommit"]')).every(s => /÷/.test(s.textContent))));
    check("カードが箱からはみ出さない", await oc.evaluate(e => [...e.querySelectorAll(".stat")].every(s => s.getBoundingClientRect().right <= e.getBoundingClientRect().right + 1)));
    { // AI の編集：ファイルの数・行と、いちばん変えたファイル（期間に始まったセッションの編集から。押すと、いちばん変えたセッション）
      const ed = await p.evaluate(() => { const {ws, we} = period(), xs = DATA.filter(s => s.start >= ws && s.start < we && s.nFiles > 0);
        const c = document.querySelector('#review .inscol:has(.hb[data-help="aiLines"])'); if (!c) return {none: true, n: xs.length};
        const add = xs.reduce((a, s) => a + (s.editAdded || 0), 0), del = xs.reduce((a, s) => a + (s.editRemoved || 0), 0);
        return {n: xs.length, files: document.querySelectorAll("#review .iwf").length, lines: c.querySelector('.hb[data-help="aiLines"]').closest(".stat").innerText, want: commas(add + del), first: (document.querySelector("#review .iwf") || {}).textContent}; }); // いちばん変えたファイルは「その他のグラフ」の自分のカード
      check("AI の編集のカードに、変えたファイル・行といちばん変えたファイルが出る", ed.none ? ed.n === 0 : ed.files > 0 && ed.lines.includes(ed.want), JSON.stringify(ed));
      if (!ed.none){ check("いちばん変えたファイルの名前は、セッションではなくファイルへのリンク（serve では、コミットのリンクがなければ文字だけ）", await p.evaluate(() => [...document.querySelectorAll("#review .iwf")].every(e => !e.matches("button") && !e.hasAttribute("data-sid"))));
        await p.locator("#review .iwses").first().click(); await pause();
        check("いちばん変えたファイルの編集の回数を押すと、そのファイルを変えたセッションが開く", await drawerOpen() && await p.evaluate(f => { const s = DATA.find(x => x.id === st.sel); return !!s && s.files.some(x => x.endsWith(f.replace("↗", "").trim().split(" ").pop())); }, ed.first)); // 名前の後ろのリンクの印（↗）は除く
        await p.keyboard.press("Escape"); await pause(); }
    }
    await view("cal");
  });

  await step("説明（?）を Esc で閉じる", async () => {
    await view("sum");
    const hb = p.locator('#review .stat .hb[data-help="focus"]').first();
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
    await view("cal");
  });

  await step("見直す候補", async () => {
    await view("sum");
    check("日の見出しの 1 行は、名前つきのただの文字（HTML を含まない）", await p.evaluate(() => [...document.querySelectorAll("#tl .head .cline > span[aria-label]")].every(e => !/[<>]/.test(e.getAttribute("aria-label")) && e.querySelectorAll("*:not(svg):not(svg *)").length === 0)));
    check("上限の印は、そのセッションの帯の左端にある（隣の帯に重ねない）", await p.evaluate(() => [...document.querySelectorAll("#tl .lim")].every(m => { const r = m.getBoundingClientRect(), bs = [...m.closest(".day").querySelectorAll(`.run[data-sid="${CSS.escape(m.dataset.id)}"]`)].map(b => b.getBoundingClientRect());
      return !bs.length || bs.some(b => Math.abs(b.left - r.left) <= 4); })));
    check("夜遅い印も日の枠の中にある", await p.evaluate(() => [...document.querySelectorAll("#tl .day")].every(d => { const db = d.getBoundingClientRect(); return [...d.querySelectorAll(".gc, .gm")].every(m => m.getBoundingClientRect().bottom <= db.bottom + 1); })));
    check("今の時刻の札に近い時刻も消さない（畳んだときは 07:00〜23:00 がそろう）", await p.evaluate(() => !document.querySelector("#tl .hours span.now") || [...document.querySelectorAll("#tl .hours span:not(.now)")].map(e => e.textContent).join(",") === Array.from({length: 17}, (_, i) => String(i + 7).padStart(2, "0") + ":00").join(",")));
    check("平日の勤務時間（9:00〜18:00）の外は地の色が違う", await p.evaluate(() => { const d = [...document.querySelectorAll("#tl .day")].find(e => !e.classList.contains("we")); return !!d && (getComputedStyle(d).backgroundImage.match(/linear-gradient/g) || []).length === 3; })); // 畳んだ帯・1 時間の線・勤務時間の外
    check("期間の行の ⚠（見直す候補の一覧）は出さない", await p.evaluate(() => !document.querySelector("#worth") && !document.querySelector("#worth summary")));
    if (env.viewport.width >= 1440) check("上の帯（期間の操作と指標）は 1 行に収まる（週・月とも）", await p.evaluate(async () => { // 死守する: 指標が次の行に落ちると、カレンダーが下がる
      setView("cal"); await new Promise(r => setTimeout(r, 200)); // 帯はカレンダーの表示で見る（サマリーの表示では出さない）
      const one = () => { const k = document.querySelector(".kpis").getBoundingClientRect(), n = document.querySelector("#today").getBoundingClientRect(); return k.top < n.bottom && k.bottom > n.top; };
      const w = st.week, a = one(); setMode("month"); await new Promise(r => setTimeout(r, 300)); const b = one(); setMode("week"); st.week = w; render(); await new Promise(r => setTimeout(r, 300));
      setView("sum"); await new Promise(r => setTimeout(r, 200));
      return a && b; }));
    const go = p.locator("#review .fmark").first(); // 見直す候補は、サマリーの指標の横の印から開く
    if (await go.count()){
      const id = await go.getAttribute("data-goto");
      await go.scrollIntoViewIfNeeded(); await pause(); const y0 = await p.evaluate("scrollY");
      await go.click(); await pause();
      check("見直す候補の印を押すと、真ん中のダイアログで開く", await p.locator("#wk").evaluate(d => d.open) && (await p.locator("#wkh").innerText()).trim().length > 0, id);
      check("ダイアログには、見えたこと・基準・次にやってみることがある", await p.locator("#wk .fl .see").count() > 0 && await p.locator("#wk .fl .rule").count() > 0);
      const box = await p.locator("#wk").boundingBox();
      check("ダイアログが画面に収まる", box && box.x >= 0 && box.x + box.width <= env.viewport.width + 1, JSON.stringify(box));
      check("開いてもページは動かない", await p.evaluate("scrollY") === y0);
      await p.keyboard.press("Escape"); await pause();
      check("Esc で閉じ、押したボタンにフォーカスが戻る", !(await p.locator("#wk").evaluate(d => d.open)) && await go.evaluate(e => e === document.activeElement));
      await go.click(); await pause();
      await p.mouse.click(4, env.viewport.height - 4); await pause();
      check("枠の外を押すと閉じる", !(await p.locator("#wk").evaluate(d => d.open)));
      const mk = p.locator("#review .fmark").first(); // サマリーの印（指標の枠のある候補だけに付く）
      if (await mk.count()){ await mk.scrollIntoViewIfNeeded(); await mk.click(); await pause();
        check("サマリーの印を押すと、同じダイアログが開く", await p.locator("#wk").evaluate(d => d.open) && await p.locator("#wk .fl .see").count() > 0);
        await p.locator("#wkclose").click(); await pause();
        check("Close で閉じる", !(await p.locator("#wk").evaluate(d => d.open))); } else console.log("  skip サマリーに印の付いた指標がない");
      await go.click(); await pause();
      const fs = p.locator("#wk .fss .card").first();
      if (await fs.count()){
        // エージェントの頭文字は、題名が長くても縮まず 1 行。見えている印だけ（畳んだ一覧の中のものは測れない）
        const marks = await p.locator("#wk .fss .card .agm").evaluateAll(es => es.filter(e => e.getClientRects().length).map(e => { const r = document.createRange(); r.selectNodeContents(e);
          return {t: e.textContent, lines: new Set([...r.getClientRects()].map(x => Math.round(x.top))).size, fits: e.clientWidth >= r.getBoundingClientRect().width}; }));
        const bad = marks.filter(m => m.lines !== 1 || !m.fits);
        check("ダイアログのカードのエージェントの頭文字は 1 行に収まる", !bad.length, JSON.stringify(bad[0] || {}));
        await fs.click(); await pause();
        check("ダイアログのセッションを押すと、ダイアログを閉じて詳細が開く", !(await p.locator("#wk").evaluate(d => d.open)) && await drawerOpen());
        check("詳細の頭に、このセッションが関係する見直す候補が出る", await p.locator("#panel .sflags .flink").count() > 0);
        await p.keyboard.press("Escape"); await pause(); }
      else { console.log("  skip ダイアログにセッションのカードがない"); await p.keyboard.press("Escape"); await pause(); }
    } else check("「まず見直す」に候補がある（デモのデータで、下の確かめのため）", false);
    await open();
    await view("cal");
  });

  await step("ページの下", async () => {
    await view("sum");
    const gh = p.locator('#review .foot a[href="https://github.com/MichinaoShimizu/kiroku"]');
    check("フッターにリポジトリへのリンクがある（参照元を渡さない）", await gh.count() === 1 && /noreferrer/.test(await gh.getAttribute("rel") || "") && await gh.getAttribute("target") === "_blank");
    const ds = p.locator("#review details.dsrc");
    check("Data sources は、いつもたたんである（気をつけることは見出しの ⚠）", await ds.count() === 1 && !(await ds.evaluate(d => d.open)));
    await ds.locator(":scope > summary").click(); await pause();
    check("開くと、読んだ履歴が見える", await ds.locator(".mlist li").first().isVisible());
    const nf = ds.locator("details.dnf");
    if (await nf.count()) check("セッションのないエージェントは「Not found: N」の下にたたむ", /^Not found: \d+$/.test((await nf.locator("summary").innerText()).trim()) && !(await nf.evaluate(d => d.open)));
    await view("cal");
  });

  // 問題の報告と診断のコピーは画面に置かない（kiroku doctor --report が同じことをする）
  await step("ページの下", async () => {
    await view("sum");
    check("フッターに「Report a problem」「Copy diagnostics」がない", await p.locator(".reportlink, .diagcopy, #review .foot .report").count() === 0 && !/Report a problem|Copy diagnostics/.test(await p.locator("#review .foot").innerText()));
    check("フッターには版と GitHub のリンクがある", /kiroku/.test(await p.locator("#review .foot").innerText()) && await p.locator("#review .foot a", { hasText: "GitHub" }).count() === 1);
    await view("cal");
  });

  await step("報告を書いてもらうプロンプト", async () => {
    check("週報のプロンプトをコピーするボタンはない", await p.locator("#rptcopy").count() === 0);
    // 日報はプロンプトをコピーするだけ（kiroku は AI を動かさない。デモでも同じ）。週報のボタンはない
    check("週報のボタンはなく、日報はプロンプトのコピー", await p.locator("#rptbar, #rptrun").count() === 0 && await p.locator(".head .dayrun.ai-copy").count() > 0 && await p.locator(".ai-action, dialog#ai-review-dialog").count() === 0);
    { const r = await p.evaluate(async () => { window.__copied = null; navigator.clipboard.writeText = t => { window.__copied = t; return Promise.resolve(); };
        document.querySelector(".head .dayrun").click(); await new Promise(z => setTimeout(z, 300)); return {got: window.__copied || "", dialog: !!document.querySelector("dialog[open]")}; });
      check("日報のボタンは、その日のプロンプトをコピーするだけ", /## Daily report: /.test(r.got) && !r.dialog, r.got.slice(0, 120)); }
    // 試す日は、表示中の週で、コミットのある日のうちセッションのいちばん多い日（ダミーデータの「今」は KIROKU_FAKE_NOW で決まる）
    const dayLabel = await p.evaluate(() => { let best = null, score = -1;
      for (let i = 0; i < 7; i++){ const d = addDays(st.week, i), ws = d.getTime() / 1000, we = addDays(d, 1).getTime() / 1000;
        const n = DATA.filter(s => s.segs.some(([a, b]) => b > ws && a < we)).length, c = (META.git || []).some(c => c.t >= ws && c.t < we && c.project);
        const sc = n ? n + (c ? 1000 : 0) : -1; if (sc > score){ score = sc; best = d; } }
      window.__rptDay = best; return best ? best.toDateString() : ""; });
    check("日報を試す、セッションのある日がある", !!dayLabel);
    const text = await p.evaluate(() => reportPrompt(window.__rptDay));
    check("事実の見出しは「Work on Wed, Oct 14, 2026」の形", /^## Work on [A-Z][a-z]{2}, [A-Z][a-z]{2} \d{1,2}, \d{4}$/m.test(text) && /^## Daily report: /m.test(text), JSON.stringify(text.slice(0, 80)) + " " + dayLabel);
    check("履歴ファイルのパスと、読む範囲の決まりがある", /History file: \//.test(text) && /Read only the files listed/.test(text));
    check("決まった形式と、kiroku が埋めた数字の表がある", /# Format/.test(text) && /### By project/.test(text) && /^\| Active time \| \S/m.test(text) && /^\| Git commits \(by AI\) \| \d+ \(\d+\) \|/m.test(text), JSON.stringify(text.match(/^\| Active time.*$/m)));
    // 長い日本語のプロンプトが多い日でも、エージェントに貼れる大きさに収まる（要約を短くし、そう書く）
    const big = await p.evaluate(() => { const was = DATA, day = window.__rptDay, ws = day.getTime() / 1000, we = addDays(day, 1).getTime() / 1000;
      try { const inDay = DATA.filter(s => s.segs.some(([a, b]) => b > ws && a < we));
        const many = []; for (let i = 0; i < 120; i++) inDay.forEach(s => many.push({...s, id: s.id + "-x" + i,
          prompts: s.prompts.map(q => ({...q, text: "日本語の長いプロンプト。".repeat(40), reply: {t: q.t, text: "日本語の長い応答。".repeat(40)}}))}));
        DATA = was.concat(many); const t = reportPrompt(day); return {size: utf8Len(t), short: /Shortened to keep this prompt small/.test(t), budget: PROMPT_BUDGET}; }
      finally { DATA = was; } });
    check("長い日でも、報告のプロンプトは送れる大きさに収まり、短くしたと書く", big.size <= big.budget && big.short, JSON.stringify(big));
    check("デモでは、記録のある日付の見出しに日報のボタンが出る", await p.locator(".head .dayrun").count() > 0);
    check("セッションごとに、プロンプトと応答の要約がある", /^  - [A-Z][a-z]{2}, [A-Z][a-z]{2} \d{1,2} \d\d:\d\d Prompt: /m.test(text));
    check("事実と履歴は、コードブロックの中にある", /# History data\n`{3,}text\n# Facts/.test(text));
    check("日報には日の決まりだけがあり、ほかのコマンドを禁じる", /tomorrow's next step/.test(text) && !/next-week priorities|sustained trends/.test(text) && /don't run other commands, change or write files, or send anything to another service/.test(text));
    // セッション全体の目安コストは、期間の数（Numbers）と合わないので、手がかりの行に入れない
    const sig = text.slice(text.indexOf("## Signals for advice"), text.indexOf("# Sessions"));
    check("モデルと短いセッションの手がかりに、セッション全体のコストを入れない", !/^- (Main model of each session|Short sessions).*\$\d/m.test(sig), sig.split("\n").filter(l => /Main model|Short sessions/.test(l)).join(" / "));
    // 凡例で隠したプロジェクトは、セッションだけでなく、コミット・プロジェクトの節・繰り返しの手がかりからも外す
    const hid = await p.evaluate(() => { const day = window.__rptDay, ws = day.getTime() / 1000, we = addDays(day, 1).getTime() / 1000;
      const c = (META.git || []).find(c => c.t >= ws && c.t < we && c.project); if (!c) return null;
      const by = st.colorBy, was = new Set(st.hidden); st.colorBy = "project"; st.hidden = new Set([c.project]);
      try { return {project: c.project, hash: c.hash.slice(0, 12), others: (META.git || []).filter(x => x.t >= ws && x.t < we && x.project === c.project).map(x => x.hash.slice(0, 12)),
        text: reportPrompt(day)}; } finally { st.colorBy = by; st.hidden = was; } });
    if (hid){ const facts = hid.text.slice(hid.text.indexOf("# History data"));
      check("隠したプロジェクトのコミットと節を報告に入れない", hid.others.every(h => !facts.includes(h)) && !facts.includes("### `" + hid.project + "`") && !facts.includes(`- ${hid.project} · `), hid.project);
      check("隠したものがあると、Numbers の表はそれも数えると伝える", /left out of this report on purpose/.test(hid.text) && !/left out of this report on purpose/.test(text)); }
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
    check("検索をやめると元の表示に戻る", await p.locator("#q").inputValue() === "" && await p.locator("#review .stat").count() > 0 && await p.locator("#tl").isVisible() && await p.locator("#sres").isHidden());
  });

  await step("表示の切り替え", async () => {
    await p.keyboard.press("m"); await pause();
    check("m で月表示になる", await p.locator("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
    await p.keyboard.press("w"); await pause();
    check("表示の切り替え（Journey・Project）はない。カレンダーだけ", await p.locator("#journeybtn, #projectbtn, #calendarbtn").count() === 0 && await p.locator("#tl").getAttribute("aria-label") === "Calendar");
    // 以前の版が保存した表示（journey・project）が残っていても、カレンダーで開く
    for (const v of ["journey", "project"]) {
      await p.evaluate(v => localStorage.setItem("kiroku:view", JSON.stringify(v)), v);
      await open();
      check(`保存された表示が ${v} でも、カレンダーで開く`, await p.locator("#tl .calscroll, #tl .empty").count() === 1 && await p.locator("#tl").getAttribute("aria-label") === "Calendar");
    }
    await p.evaluate(() => localStorage.removeItem("kiroku:view"));
    await p.keyboard.press("?"); await pause();
    check("? でショートカットの一覧が開く", await p.locator("#keys").evaluate(d => d.open));
    await p.locator("#keys form button").click(); await pause();
    check("一覧の Close（form method=dialog）で閉じる", !(await p.locator("#keys").evaluate(d => d.open)));
    await p.keyboard.press("?"); await pause();
    await p.mouse.click(4, env.viewport.height - 4); await pause();
    check("一覧は枠の外を押しても閉じる", !(await p.locator("#keys").evaluate(d => d.open)));
  });

  await step("積み上がりの切り替え・月の濃淡・クレジットの単位", async () => {
    await p.goto(url); await loaded(p);
    check("クレジットは cr ではなく言葉の単位で書く", !/\d\s*cr\b/.test(await p.locator("#kpis").innerText()));
    await view("sum");
    const ks = await p.locator("#review [data-cum]").evaluateAll(xs => xs.map(x => x.dataset.cum));
    check("積み上がりは、記録のある量を切り替えられる", ks.length === 0 || ks.includes("active"), JSON.stringify(ks));
    for (const k of ks){ await p.locator(`#review [data-cum="${k}"]`).click(); await pause();
      check(`積み上がりを ${k} にすると、その量の線と 1 文になる`, await p.evaluate(k => st.insC === k && document.querySelector(`#review [data-cum="${k}"]`).getAttribute("aria-pressed") === "true" && !!document.querySelector("#review polyline.icum"), k)); }
    check("見出しは「前週比」の言い方（vs last week）", /Cumulative, vs last (week|month)/.test(await p.locator("#review .inscol:has(polyline.icum) h4").innerText()));
    const sk = await p.locator("#review [data-scat]").evaluateAll(xs => xs.map(x => x.dataset.scat));
    for (const k of sk){ await p.locator(`#review [data-scat="${k}"]`).click(); await pause();
      check(`散布図の横軸を ${k} にすると、その量で点を描く`, await p.evaluate(k => st.insS === k && document.querySelector(`#review [data-scat="${k}"]`).getAttribute("aria-pressed") === "true" && document.querySelectorAll("#review .ipt[data-sid]").length >= 3, k)); }
    const wk = await p.locator("#review [data-wk]").evaluateAll(xs => xs.map(x => x.dataset.wk));
    check("どこに使ったかは、記録のある量（費用・クレジット・トークン・時間）を切り替えられる", wk.length >= 2, JSON.stringify(wk));
    for (const k of wk){ await p.locator(`#review [data-wk="${k}"]`).click(); await pause();
      check(`どこに使ったかを ${k} にすると、その量の横棒になる`, await p.evaluate(k => st.insWk === k && document.querySelector(`#review [data-wk="${k}"]`).getAttribute("aria-pressed") === "true" && document.querySelectorAll("#review .iwrows .iwr").length > 0, k)); }
    // 前の期間からの増減（ウォーターフォール）：前の合計 → プロジェクトごとの増減 → 今の合計。最後の棒の値は、前の合計に増減を足したもの
    const wf = await p.locator("#review [data-wfk]").evaluateAll(xs => xs.map(x => x.dataset.wfk));
    check("前の期間からの増減は、記録のある量を切り替えられる", wf.length >= 2, JSON.stringify(wf));
    for (const k of wf){ await p.locator(`#review [data-wfk="${k}"]`).click(); await pause();
      check(`前の期間からの増減を ${k} にすると、合計 2 本とプロジェクトの棒になり、1 文がある`, await p.evaluate(k => { const f = document.querySelector("#review .iwtf"), c = f && f.closest(".inscol");
        return st.insWf === k && document.querySelector(`#review [data-wfk="${k}"]`).getAttribute("aria-pressed") === "true" && f.querySelectorAll("rect.wft").length === 2 && f.querySelectorAll("rect.wfu, rect.wfd").length >= 1 && !!c.querySelector(".isay"); }, k)); }
    // ドーナツ・サンキー・箱ひげ・ガント
    check("どこに使ったかに、割合のドーナツがある（項目の数だけ弧）", await p.evaluate(() => { const d = document.querySelector("#review .iwdn svg.idn"); return !!d && d.querySelectorAll("path").length === document.querySelectorAll("#review .iwdn .iwr").length; }));
    for (const [attr, key, sel, name] of [["skk", "insSk", ".isk", "どこへ流れたか"], ["bxk", "insBk", ".ibxf", "1 セッションあたりのばらつき"]]){
      const ks2 = await p.locator(`#review [data-${attr}]`).evaluateAll(xs => xs.map(x => x.dataset[Object.keys(x.dataset)[0]]));
      for (const k of ks2){ await p.locator(`#review [data-${attr}="${k}"]`).click(); await pause();
        check(`${name}を ${k} にすると、その量の図と 1 文になる`, await p.evaluate(([key, k, sel]) => st[key] === k && !!document.querySelector(`#review ${sel} svg`) && !!document.querySelector(`#review ${sel}`).closest(".inscol").querySelector(".isay"), [key, k, sel])); } }
    check("「グラフ」の先頭のカードは、どこへ流れたか（サンキー）", await p.evaluate(() => { const r = document.querySelector("#review .insrow.two"), c = r && r.firstElementChild; return !!c && !!c.querySelector(".isk") && [...r.children].every(x => x === c || x.getBoundingClientRect().top >= c.getBoundingClientRect().top - 1); }));
    check("サンキーは、エージェント・プロジェクト・成果の 3 列", await p.evaluate(() => document.querySelectorAll("#review .isk .skh").length === 3 && document.querySelectorAll("#review .isk .skl").length >= 2));
    { const g = p.locator("#review .igt .igr").first(); if (await g.count()){ const sid = await g.getAttribute("data-sid"); await g.click(); await pause();
      check("いちばん作業した日のガントの行を押すと、そのセッションが開く", await p.evaluate(sid => st.sel === sid, sid)); await p.keyboard.press("Escape"); await pause(); } }
    check("長さ別の図は 1 セッションあたりの平均なので、区間ごとのセッションの数を軸の下に出す（1 件だけの平均を見分けられるように）", await p.evaluate(() => { const g = document.querySelector("#review .ihist svg"); return !g || g.querySelectorAll(".ixn").length === g.querySelectorAll(".ihb").length; }));
    await view("cal");
    await p.keyboard.press("m"); await pause();
    const opts = await p.locator("#calk option").evaluateAll(xs => xs.map(x => x.value));
    if (opts.length > 1){ const heat = () => p.evaluate(() => [...document.querySelectorAll(".mgrid .cell[style*='--heat']")].map(c => c.style.getPropertyValue("--heat")).join(","));
      const h0 = await heat(); await p.selectOption("#calk", opts[opts.length - 1]); await pause();
      check("月の濃淡の量を凡例の「数字」で選べ、塗り方が変わり、選んだものが残る", await heat() !== h0 && await p.evaluate(v => st.shade === v && document.querySelector("#calk").value === v, opts[opts.length - 1]), JSON.stringify(opts));
      await p.keyboard.press("w"); await pause();
      const lab = await p.evaluate(() => [...document.querySelectorAll("#tl .head:not(.empty) .cline")].slice(0, 1).map(l => [...l.querySelectorAll(".cu, .ct")].map(e => e.getAttribute("aria-label")).join(" | ")).join(""));
      check("週カレンダーの日の数字も、選んだ量で書く（稼働時間とコスト系の両方を並べる）", await p.evaluate(v => document.querySelector("#calk") && document.querySelector("#calk").value === v, opts[opts.length - 1]) && lab.includes(await p.evaluate(v => SHADE()[v].s, opts[opts.length - 1])) && lab.includes(await p.evaluate(() => t("Active time"))), lab);
      await p.selectOption("#calk", opts[0]); await pause(); await p.keyboard.press("m"); await pause(); }
    else console.log("  skip 月の濃淡に選べる量が 1 つしかない");
    await p.keyboard.press("w"); await pause();
  });

  await step("今週の表示でも、クレジットの今月末の推定がわかる", async () => {
    await p.goto(url); await loaded(p); await p.keyboard.press("t"); await pause(); // 今週へ（今週に記録がなくても、今月の記録から推定する）
    const mp = await p.evaluate(() => { const m = monthProj(); return m && m.credits != null ? {v: crN(m.credits), so: crN(m.soCredits)} : null; });
    if (!mp){ console.log("  skip 今月のクレジットの記録がない（月の最初の 7 日間か、今週を見ていない）"); return; }
    const band = p.locator('#kpis [data-metric="monthCr"]');
    check("上の帯に、今月末の推定クレジットがある", await band.count() === 1 && (await band.innerText()).includes(mp.v), mp.v);
    await band.click(); await pause();
    const dlg = await p.evaluate(() => ({open: $("#md").open, txt: $("#md").innerText}));
    check("押すと、今月の日ごとの内訳（月末の推定）が開き、週の表示はそのまま", dlg.open && dlg.txt.includes(mp.v) && dlg.txt.includes(mp.so) && await p.evaluate(() => st.mode === "week"), dlg.txt.slice(0, 120));
    await p.keyboard.press("Escape"); await pause();
    await view("sum");
    await p.locator('#review .iksw[data-sw="cost"] [data-v="credits"]').click(); await pause(); // コストのパネルでクレジットを選ぶ
    const tile = await p.evaluate(() => { const b = document.querySelector('#review .itile [data-metric="credits"]'); return b ? b.closest(".itile").innerText : ""; });
    await p.locator('#review .iksw[data-sw="cost"] [data-v="cost"]').click(); await pause();
    check("サマリーのクレジットのカードにも、今月末の推定がある（式を出しているときは式で）", /Month-end estimate \(this month\)|Month-end credits = /.test(tile) && tile.includes(mp.v), tile);
    await view("cal");
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
// 日本語の表示。日本語の文言は英語より折り返し方が違うので、同じ 4 つの幅で、主な画面がスクリプトのエラーなしに開き、
// 横にはみ出さず、訳のある画面の文言が英語のまま残っていないかを見る。操作の細かな確かめは英語の流れ（run）でしているので、ここでは開いて見るだけ
async function runJa(env) {
  const out = [`# ${env.name} 日本語`];
  let failed = 0;
  const csp = [];
  function check(what, ok, detail = "") {
    if (ok) out.push(`  ok   ${what}`);
    else { failed++; out.push(`  FAIL ${what}${detail ? `: ${detail}` : ""}`); }
  }
  const { ctx, p, errors } = await openContext(env, "ja-JP", csp);
  async function step(name, fn) {
    try { await fn(); } catch (e) { check(name, false, e.message.split("\n")[0]); await p.keyboard.press("Escape").catch(() => {}); }
  }
  const overflow = () => p.evaluate(() => [document.documentElement.scrollWidth, innerWidth]);
  const isOpen = sel => p.waitForFunction(sel => document.querySelector(sel).open, sel);
  const isClosed = sel => p.waitForFunction(sel => !document.querySelector(sel).open, sel);
  // 辞書（JA_TEXT）に訳のある英語の文が、そのまま文字・aria-label・title・placeholder に出ているもの。
  // 履歴から来た文（ダミーデータのプロンプトや題名など）と、わざと別の言語で書いたもの（lang の違う要素。「EN」のボタンなど）は除く
  const untranslated = () => p.evaluate(() => {
    const hist = new Set();
    for (const s of DATA) for (const v of [s.title, s.project, s.branch, s.source, ...(s.prompts || []).map(x => x.text)]) if (typeof v === "string") hist.add(v.trim());
    const bad = new Set(), el = n => n.nodeType === 3 ? n.parentElement : n, ja = new Set(Object.values(JA_TEXT)); // ほかの文の訳と同じ文字（「PRs」の訳の「PR」など）は、訳したもの
    const look = (n, v) => { v = (v || "").trim();
      if (v && Object.prototype.hasOwnProperty.call(JA_TEXT, v) && JA_TEXT[v] !== v && !ja.has(v) && !hist.has(v) && (el(n).closest("[lang]") || {}).lang === "ja") bad.add(v); };
    const w = document.createTreeWalker(document.body, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
    for (let n = w.nextNode(); n; n = w.nextNode()) {
      if (n.nodeType === 3) look(n, n.nodeValue);
      else for (const a of ["aria-label", "title", "placeholder"]) look(n, n.getAttribute(a));
    }
    return [...bad];
  });
  const noEnglish = async where => { const bad = await untranslated(); check(`${where}に、訳のある英語の文言が残っていない`, bad.length === 0, bad.slice(0, 5).join(" / ")); };

  await p.goto(url);
  await loaded(p);

  await step("読み込み", async () => {
    check("ブラウザの言語が日本語なら、日本語で開く", await p.evaluate(() => document.documentElement.lang === "ja" && JA));
    // 以前の版の切り替えボタンで English を選んだまま（localStorage に残っている）でも、ブラウザの言語に従う（ボタンがないので戻せないため）
    await p.evaluate(() => localStorage.setItem("kiroku:lang", JSON.stringify("en"))); await p.reload(); await loaded(p);
    check("以前選んだ English が残っていても、日本語で開く", await p.evaluate(() => document.documentElement.lang === "ja" && JA));
    await p.evaluate(() => localStorage.removeItem("kiroku:lang"));
    check("期間の見出しは日本語の日付", /\d+月\d+日/.test(await p.locator("#rd").innerText()), await p.locator("#rd").innerText());
    const [sw, iw] = await overflow();
    check("横にはみ出さない", sw <= iw + 1, `${sw} > ${iw}`);
    await noEnglish("最初の画面");
  });

  await step("セッションの詳細", async () => {
    const before = await p.locator("#rd").innerText();
    await p.keyboard.press("ArrowLeft"); // 記録のそろった先週で開く
    await p.waitForFunction(b => document.querySelector("#rd").textContent !== b, before);
    await (await clickableRun(p)).click();
    await p.waitForFunction(() => document.querySelector("#drawer").getAttribute("aria-hidden") === "false");
    const box = await p.locator("#drawer").boundingBox();
    check("詳細が開き、画面に収まる", box && box.x >= -1 && box.x + box.width <= env.viewport.width + 1, JSON.stringify(box));
    await noEnglish("セッションの詳細");
    await p.keyboard.press("Escape");
    await p.waitForFunction(() => document.querySelector("#drawer").getAttribute("aria-hidden") !== "false");
  });

  await step("内訳", async () => {
    await p.evaluate(() => document.querySelector('#review [data-metric="cost"]').click());
    await isOpen("#md");
    check("内訳の見出しが日本語", (await p.locator("#mdh").innerText()) !== "Estimated cost", await p.locator("#mdh").innerText());
    await noEnglish("内訳のダイアログ");
    await p.keyboard.press("Escape"); await isClosed("#md");
  });

  await step("月表示", async () => {
    await p.keyboard.press("m");
    await p.waitForFunction(() => document.querySelector("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
    const [sw, iw] = await overflow();
    check("月表示でも横にはみ出さない", sw <= iw + 1, `${sw} > ${iw}`);
    await noEnglish("月表示");
    await p.keyboard.press("w");
    await p.waitForFunction(() => document.querySelector("#mode button[data-v=week]").getAttribute("aria-pressed") === "true");
  });

  check("スクリプトのエラーがない", errors.length === 0, errors.join(" / "));
  check("CSP に止められたものがない", csp.length === 0, csp.join(" / "));
  await ctx.close();
  return { out, failed };
}

const results = await Promise.all([...envs.map(run), ...envs.map(runJa)]);
await b.close();
for (const r of results) console.log(r.out.join("\n"));
const failed = results.reduce((n, r) => n + r.failed, 0);
if (failed) { console.log(`\n${failed} 件失敗`); process.exit(1); }
console.log("\nすべて OK");
