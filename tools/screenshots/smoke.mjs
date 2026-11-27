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
  await ctx.addInitScript(() => { try { for (const k of ["kiroku:insBrk"]) if (localStorage.getItem(k) == null) localStorage.setItem(k, "true"); } catch (e) {} }); // 「内訳」と「その他のグラフ」を開いた状態で確かめる（たたむことは別に確かめる）
  const p = await ctx.newPage();
  const errors = [];
  p.on("pageerror", e => errors.push(e.message));
  p.on("console", m => { if (m.type() === "error") errors.push(m.text()); });
  return { ctx, p, errors };
}

// legendHint は、凡例のチップの並び（横にスクロールする）の「+N ›」を確かめる（英語と日本語の流れから、Project と Agent の色分けで呼ぶ）。
// 数は名前の見えていないチップの数。札は並びに重ねず右に場所を取り、見えているチップの時間を隠さない。押すと次の隠れたチップを左端にそろえて送る。
// Tab で来たチップ（一部だけ見えていたものも）は並びの中に全部見え、最後のチップなら並びの終わりまで送る
async function legendHint(p, env, check, by) {
  const pause = () => p.waitForTimeout(300), tag = `${by === "source" ? "Agent" : "Project"}: `;
  await p.evaluate(v => { const s = document.querySelector("#cb2"); s.value = v; s.dispatchEvent(new Event("change")); }, by); await pause();
  const hint = () => p.evaluate(() => { const r = document.querySelector("#legend .lrow"), m = document.querySelector("#legend .lmore"), rb = r.getBoundingClientRect(), mb = m.getBoundingClientRect(), shown = m.checkVisibility();
    const chips = [...r.querySelectorAll(".chip")], vis = rb.right - 8, hidden = chips.filter(c => c.querySelector(".cn").getBoundingClientRect().right > vis);
    // 札が、並びの中で見えている部分（rb の内側）のチップの時間に重なっていないか
    const over = !shown ? [] : chips.filter(c => { const n = c.querySelector(".n").getBoundingClientRect(), l = Math.max(n.left, rb.left), rr = Math.min(n.right, rb.right); return rr > l && mb.left < rr && mb.right > l; }).map(c => c.dataset.k);
    return {over: r.scrollWidth > r.clientWidth + 2, more: r.scrollLeft + r.clientWidth < r.scrollWidth - 2, shown, text: m.textContent, cut: hidden.length, first: hidden.map(c => c.dataset.k)[0], left: r.scrollLeft, covers: over,
      inside: !shown || mb.right <= innerWidth, page: document.documentElement.scrollWidth <= innerWidth + 1}; });
  await p.evaluate(() => { document.querySelector("#legend .lrow").scrollLeft = 0; }); await pause();
  const h0 = await hint();
  check(`${tag}凡例のチップがはみ出すときだけ、並びの右に名前の見えていないチップの数（+N ›）を出し、見えているチップの時間に重ならない`, h0.shown === h0.more && (!h0.shown || (h0.text === `+${Math.max(1, h0.cut)} ›` && h0.inside && h0.page && !h0.covers.length)), JSON.stringify(h0));
  if (!env.isMobile) return;
  check(`${tag}スマホ（ダミーデータ）では、凡例のチップが横にはみ出し、数を出す`, h0.over && h0.shown, JSON.stringify(h0));
  if (h0.shown){
    // 押すと、次の隠れたチップ（名前の見えていなかった最初のもの）を並びの左端にそろえて送る。チップの途中で止めない
    await p.locator("#legend .lmore").click(); await p.waitForTimeout(700);
    const al = await p.evaluate(k => { const r = document.querySelector("#legend .lrow"), c = [...r.querySelectorAll(".chip")].find(x => x.dataset.k === k), rb = r.getBoundingClientRect(), cb = c.getBoundingClientRect();
      return {left: r.scrollLeft, end: r.scrollLeft + r.clientWidth >= r.scrollWidth - 2, dx: Math.round(cb.left - rb.left)}; }, h0.first);
    check(`${tag}数を押すと、次の隠れたチップが並びの左端にそろう（並びの終わりに着いたときは、そのチップが見える）`, al.left > 0 && (Math.abs(al.dx) <= 1 || (al.end && al.dx >= 0)), JSON.stringify(al));
    const h1a = await hint();
    check(`${tag}送ったあとも、数は名前の見えていないチップの数で、札は時間に重ならない`, !h1a.shown || (h1a.text === `+${Math.max(1, h1a.cut)} ›` && !h1a.covers.length), JSON.stringify(h1a));
    await p.evaluate(() => { const r = document.querySelector("#legend .lrow"); r.scrollLeft = r.scrollWidth; }); await pause();
    const h1 = await hint();
    check(`${tag}右端までスクロールすると、数が消える`, !h1.shown && !h1.more, JSON.stringify(h1));
    await p.evaluate(() => { document.querySelector("#legend .lrow").scrollLeft = 0; }); await pause();
    // キーボード：Tab で進むたびに、そのチップ（一部だけ見えていたものも）が並びの中に全部見え、ぼかしと数の札に隠れない。最後のチップでは並びの終わりまで
    const n = await p.locator("#legend .chip").count(), bad = [];
    await p.locator("#legend .chip").first().focus(); await pause();
    for (let i = 0; i < n; i++) {
      if (i) { await p.keyboard.press("Tab"); await pause(); }
      const k = await p.evaluate(() => { const c = document.activeElement, r = document.querySelector("#legend .lrow"), m = document.querySelector("#legend .lmore");
        const cb = c.getBoundingClientRect(), rb = r.getBoundingClientRect(), mb = m.checkVisibility() ? m.getBoundingClientRect() : null, last = c === [...r.querySelectorAll(".chip")].pop();
        return {k: c.dataset.k, last, inRow: cb.left >= rb.left - 1 && cb.right <= rb.right + 1 - (last ? 0 : 16), covered: !!mb && mb.left < cb.right && mb.right > cb.left, end: r.scrollLeft + r.clientWidth >= r.scrollWidth - 2}; });
      if (!k.k || !k.inRow || k.covered || (k.last && !k.end)) bad.push(k);
    }
    check(`${tag}Tab で進むと、どのチップも並びの中に全部見え、数の札に隠れない。最後のチップでは並びの終わりまで送る`, bad.length === 0, JSON.stringify(bad));
    await p.evaluate(() => { document.activeElement.blur(); document.querySelector("#legend .lrow").scrollLeft = 0; }); await pause();
  }
  // チップが 1 つだけで並びに収まるときは、数を出さない（320 では見出しとチップ 1 つでもはみ出すので、見出しも外す。描き直して元に戻す）
  await p.evaluate(() => { document.querySelectorAll("#legend .chip").forEach((c, i) => { if (i) c.remove(); }); document.querySelector("#legend .lab").remove(); lrowHint(); });
  const h2 = await hint();
  check(`${tag}チップが並びに収まるときは、数を出さない`, !h2.over && !h2.shown, JSON.stringify(h2));
  await p.evaluate(() => { const s = document.querySelector("#cb2"); s.value = "project"; s.dispatchEvent(new Event("change")); }); await pause();
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
  // 表示を切りかえ、切りかわったこと（body の data-view と押したボタン）を待つ。そのあと 1 フレーム描かせる
  const view = async v => { await p.locator(`#view button[data-v="${v}"]`).click();
    await p.waitForFunction(v => document.body.dataset.view === v && document.querySelector(`#view button[data-v="${v}"]`).getAttribute("aria-pressed") === "true", v);
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)))); };
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
    check("ロゴの下に kiroku が何かを 1 行で出し、検索欄に重ならない（とても狭い画面では出さない）", await p.evaluate(w => { const g = document.querySelector(".tagline"), r = g.getBoundingClientRect();
      return w < 375 ? !g.checkVisibility() : g.checkVisibility() && r.height < 20 && r.right <= document.querySelector(".search").getBoundingClientRect().left; }, env.viewport.width));
    // カレンダーが主役：① から下のパネルは閉じていて、上の帯は使ったものだけ
    check("上の帯は使ったものだけ（残したもの：コミット・変えた行・PR は出さない）", await p.locator('#kpis .kpi.out, #kpis [data-metric="commits"], #kpis [data-metric="lines"], #kpis [data-metric="aiCommits"], #kpis [data-metric="prs"]').count() === 0);
    if (env.viewport.width > 820 && env.viewport.width < 1440) check("上の数字の帯は 1 行（折り返さない）", await p.evaluate(() => new Set([...document.querySelectorAll("#kpis .kpi")].map(k => Math.round(k.getBoundingClientRect().top))).size === 1));
    check("上の帯には前の期間との増減（%）を出さず、枠の色もそろう（増減は押したときの内訳に）", await p.evaluate(() => !document.querySelector("#kpis .kpi .dl") && !document.querySelector("#kpis .kpi.key")));
    { const b = p.locator('#kpis [data-metric="cost"], #kpis [data-metric="active"]').first(); await b.click(); await pause();
      check("帯の数を押すと、内訳の大きな数に前の期間との増減がある", /[+−±]\d*%/.test(await p.locator("#md").innerText()) || !(await p.evaluate(() => kpiDelta("cost") || kpiDelta("active"))));
      await p.keyboard.press("Escape"); await pause(); }
    if (env === envs[0]) check("履歴が前の期間の途中から始まるときは比べない", await p.evaluate(() => { const ok = vsPrev(period().P, "週").partial === false;
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
    if (env === envs[0]) check("記録の始まった週（途中から）は、「?」と見直す候補の推移の左端に (partial) と出し、読み上げでもわけを言う（ツールチップだけにしない）", await p.evaluate(() => {
      const f = document.createRange().createContextualFragment(spark("cost", true)), e = f.querySelector(".ends span"), sr = f.querySelector(".sr");
      return !!e && / \(partial\)$/.test(e.textContent) && /\(partial week: your history starts in it, so it is left out\)/.test(sr.textContent); }));
    check("週の見出しは「Week x of y」（その年の週の数は 52 か 53）", /Week \d{1,2} of 5[23]\b/i.test(await p.locator("#ry").innerText()), await p.locator("#ry").innerText());
    if (env === envs[0]) check("週の数：2026 年は 53 週、2025 年は 52 週（12 月 28 日の週）", await p.evaluate(() => isoWeek(new Date(2026, 11, 28)) === 53 && isoWeek(new Date(2025, 11, 28)) === 52));
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
    check("カレンダーの表示に、サマリーの部品が出ない", await p.evaluate(() => st.view !== "cal" || [...document.querySelectorAll("#review .insight")].every(e => !e.offsetParent)));
    check("スクロールはページだけ：カレンダーの中ではスクロールしない", await p.evaluate(() => { const c = document.querySelector("#tl .calscroll"); return !c || c.scrollHeight <= c.clientHeight + 1; }));
    { // どの幅でも、曜日の見出しがページの見出しの下に貼り付く（狭い画面は横のスクロールにも合わせる）
      await p.evaluate(() => window.scrollTo(0, document.querySelector("#tl").getBoundingClientRect().top + scrollY + 400)); await pause();
      check("スクロールしても曜日の見出しが見えている", await p.evaluate(() => { const h = document.querySelector("#tl .heads").getBoundingClientRect(), top = document.querySelector("header").getBoundingClientRect().bottom; return Math.abs(h.top - top) <= 2; }));
      await p.evaluate(() => window.scrollTo(0, 0)); await pause();
    }
    const at = () => p.evaluate(() => ["#rd", "#view"].map(q => { const r = document.querySelector(q).getBoundingClientRect(); return [Math.round(r.x), Math.round(r.y + scrollY)]; }));
    const pos0 = await at();
    await view("sum");
    check("カレンダーからサマリーを開くと、すぐサンキーの名前の後ろに札がある（隠れたまま描いた図でも、開いたときに敷く）", await p.evaluate(() => { const ts = [...document.querySelectorAll("#review .isk text.skt")].filter(x => x.checkVisibility());
      return innerWidth <= 600 || !document.querySelector("#review .isk") || (ts.length > 0 && ts.every(x => x.previousElementSibling && x.previousElementSibling.matches("rect.skbg"))); }));
    check("内訳は初めはたたんであり、見出しの横に中のカードの名前が 1 行ある。押すと開き、開いたかは覚える。たたむ箱は 1 段だけで、消したカード（推移・使い方・ばらつき）はない", await p.evaluate(async () => {
      const wait = () => new Promise(r => setTimeout(r, 50));
      store.set("insBrk", false); summary(); const B = document.querySelector("#review details.insbrk"), sm = B && B.querySelector(":scope > summary"); if (!sm) return false;
      const closed = !B.open && !B.querySelector(".insrow").checkVisibility() && /Output, Where it went/.test(sm.textContent) && !/other charts/.test(sm.textContent) && !document.querySelector("#review details.insmore");
      sm.click(); await wait(); const saved = store.get("insBrk", false) === true; summary();
      const B2 = document.querySelector("#review details.insbrk"), top = [...B2.querySelectorAll(":scope > .insg > .insrow h4")].map(h => h.textContent);
      return closed && saved && B2.open && top.some(x => /When you worked/.test(x)) && top.some(x => /Where it went/.test(x)) && top.some(x => /Most edited files/.test(x)) && !top.some(x => /^(Trend|Usage|Spread per session)/.test(x)); }));
    check("投入から産出の流れの右（狭い画面では下）に、推定費用と産出の推移のカード（2 本の線）があり、内訳の中にはない", await p.evaluate(() => {
      const f = document.querySelector("#review .isflow"), cs = f ? [...f.children] : [], sk = cs.find(c => c.querySelector(".isk")), cu = cs.find(c => c.querySelector(".icum"));
      if (!sk || !cu || document.querySelector("#review .insbrk .icum") || cu.querySelectorAll("path.icl").length !== 2) return false;
      const a = sk.getBoundingClientRect(), b = cu.getBoundingClientRect();
      return innerWidth >= 1300 ? b.left >= a.right - 1 && Math.abs(a.top - b.top) < 2 && b.width <= innerWidth / 2 + 1 : b.top >= a.bottom - 1; }));
    check("サマリーの頭に見直す候補の行はない", await p.evaluate(() => !document.querySelector("#review .iworth") && !/Worth a look/.test(document.querySelector("#review .insight").textContent.slice(0, 400))));
    check("見直す候補（Worth a look）の指標のカードが、たたんだ内訳の中にあれば、その候補を開くと内訳が開く（開いたことも覚える）", await p.evaluate(async () => {
      store.set("insBrk", false); summary(); const B = document.querySelector("#review details.insbrk");
      const id = [...B.querySelectorAll(".fmark")].map(x => x.dataset.goto)[0]; if (!id || B.open) return false;
      insOpenWorth(id); await new Promise(r => setTimeout(r, 50));
      const ok = document.querySelector("#wk").open && B.open && store.get("insBrk", false) === true; document.querySelector("#wk").close(); return ok; }));
    check("サマリーに切り替えると、カレンダーは隠れる", await p.locator("#tl").isHidden() && await p.locator("#review").isVisible());
    const pos1 = await at();
    check("切り替えても、期間の見出しと切り替えのボタンは動かない（上の帯の高さでずれない）", JSON.stringify(pos0) === JSON.stringify(pos1), `${JSON.stringify(pos0)} → ${JSON.stringify(pos1)}`);
    check("サマリーでは上の帯を見せず、押せない", !await p.evaluate(() => { const k = document.querySelector("#kpis"); const r = k.getBoundingClientRect(); return getComputedStyle(k).visibility === "visible" && getComputedStyle(k).display !== "none" && r.height > 0; }));
    check("サマリーでは、色分けと稼働時間の行（カレンダーの凡例）を出さない", await p.locator("#legend").isHidden() && await p.locator("#fnote").isHidden());
    await p.keyboard.press("v"); await pause();
    check("V でカレンダーに戻る", await p.locator("#tl").isVisible());
    check("カレンダーに戻ると、色分けと稼働時間の行がまた出る", await p.locator("#legend").isVisible());
  });

  if (env === envs[0]) await step("既定のままで消える履歴は、Data sources で残し方とバックアップ先を見せる", async () => { // keepRow の中身は幅によらない
    const r = await p.evaluate(() => { const save = META.archive; META.archive = {on: false, dir: "/Users/me/kiroku/<b>archive</b>"};
      const d = document.createElement("div"); d.innerHTML = keepRow({name: "Claude Code", n: 1, retention: {days: 30, setting: "cleanupPeriodDays", file: "~/.claude/settings.json"}}); META.archive = save;
      return {t: d.textContent, tags: d.querySelectorAll("b").length}; });
    check("フォルダが文字のまま出る", r.t.includes("/Users/me/kiroku/<b>archive</b>") && r.tags === 0, r.t);
    check("設定と kiroku serve --archive on の 2 つの残し方があり、元のファイルは変えず外にも送らないと書いてある", /cleanupPeriodDays": 3650/.test(r.t) && /kiroku serve --archive on/.test(r.t) && /own files are not changed/.test(r.t) && /nothing is sent anywhere/.test(r.t), r.t);
    check("要約の上に知らせの枠を出さない", await p.evaluate(() => !document.querySelector("#review .keep")));
  });

  if (env === envs[0]) await step("テーマ", async () => {
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
    for (const by of ["project", "source"]) await legendHint(p, env, check, by);
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
    if (env === envs[0]) check("クレジットは 10 未満なら小数 1 桁（1.33 を 1 と丸めすぎない）", await p.evaluate(() => crN(1.333) === "1.3" && crN(20) === "20" && crN(0.5) === "0.50"));
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
    // 広い画面では、流れ（左の列）が操作・数値・情報（右の列）より先：Tab が「閉じる → 右 → 左 → 右」と行き来しない
    check("Tab の順が、見た目の列の順（左の流れ → 右の列）", await p.evaluate(() => { const c = document.querySelector("#panel .scols"); if (!c) return true;
      const f = c.querySelector(":scope > .sflow"), wide = matchMedia("(min-width:1100px)").matches; return !f || (wide ? c.firstElementChild === f : !f.nextElementSibling || f.nextElementSibling.matches(".sinfo")); }));
    check("閉じるボタンが種類の行に並び、ボタンだけの空いた帯がない", await p.evaluate(() => { const c = document.querySelector("#close").getBoundingClientRect(), e = document.querySelector("#panel .eyebrow").getBoundingClientRect();
      return Math.abs((c.top + c.bottom) / 2 - (e.top + e.bottom) / 2) < 12 && c.left > e.left; }));
    const rep = p.locator("#panel .tl li.rp").first();
    check("依頼のあとに AI の応答が出ている", await rep.count() > 0);
    if (await rep.count() > 0){
      await rep.scrollIntoViewIfNeeded();
      check("応答の中身が読める", (await rep.locator(".reptext").innerText()).trim().length > 0);
      check("だれの発言か分かる", await rep.locator(".rpav svg").count() === 1);
      // LINE のトーク画面のように：青灰色の地、あなたは右の緑・AI は左の白、時刻は吹き出しの下の端の横（あなたは左、AI は右）
      const lay = await p.evaluate(() => { const tl = document.querySelector("#panel .tl"), pr = tl.querySelector("li.pr:not(.ev)"), rp = tl.querySelector("li.rp:not(.quiet)");
        const side = li => { const b = li.querySelector(":scope>p").getBoundingClientRect(), t = li.querySelector(".pfoot time").getBoundingClientRect(); return {b, t}; };
        const a = side(pr), r = side(rp), box = tl.getBoundingClientRect();
        return {bg: getComputedStyle(tl).backgroundColor, me: getComputedStyle(pr.querySelector(":scope>p")).backgroundColor, ai: getComputedStyle(rp.querySelector(":scope>p")).backgroundColor,
          meRight: box.right - a.b.right < 40, meTime: a.t.right <= a.b.left + 1 && Math.abs(a.t.bottom - a.b.bottom) < 8,
          aiTime: r.t.left >= r.b.right - 1 && Math.abs(r.t.bottom - r.b.bottom) < 8, aiAv: rp.querySelector(".av").getBoundingClientRect().right <= r.b.left }; });
      check("流れは画面と同じ暗い地に、あなたは右の紫、AI は左の暗い灰の吹き出し", lay.bg === "rgb(29, 27, 32)" && lay.me === "rgb(208, 188, 255)" && lay.ai === "rgb(54, 52, 59)" && lay.meRight && lay.aiAv, JSON.stringify(lay));
      check("時刻は吹き出しの下の端の横（あなたは左、AI は右）", lay.meTime && lay.aiTime, JSON.stringify(lay));
      // 依頼と応答の間に起きたこと（コミットなど）は応答の上に出るので、それを飛ばした手前が依頼
      check("応答は依頼のすぐ後に並ぶ", await p.evaluate(() => { const li = [...document.querySelectorAll("#panel .tl li")], i = li.findIndex(x => x.classList.contains("rp"));
        let j = i - 1; while (j >= 0 && li[j].classList.contains("ev")) j--;
        return i > 0 && j >= 0 && li[j].classList.contains("pr"); }));
    }
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
        return {label: b.getAttribute("aria-label"), title: b.title, text: b.textContent.trim(), got: window.__copied || "", first: String(s.prompts[0].text || "").slice(0, 30), n: s.prompts.length}; });
      // 読み上げの名前は見えている文字から始まる（aria-label で別の名前にしない。説明は title に）
      check("会話全体を Markdown でコピーできる", !c.none && !c.label && c.text === "Copy as Markdown" && /Copy the conversation as Markdown/.test(c.title) && c.got.startsWith("# ") && (c.got.match(/^## You/gm) || []).length === c.n && c.got.includes(c.first), JSON.stringify({...c, got: c.got.slice(0, 80)}));
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
    const card = p.locator("#review .iwses[data-sid]").first();
    check("サマリーにセッションを開くボタン（いちばん変えたファイルの編集の回数）がある", await card.count() > 0);
    await card.scrollIntoViewIfNeeded();
    const id = await card.getAttribute("data-sid");
    await card.click(); await pause();
    const y = await p.evaluate("scrollY"); // 開いた時点の位置（クリックの前に Playwright がスクロールし直すことがあるので、開いてから測る。開いている間はページが動かない）
    await p.keyboard.press("Escape"); await pause();
    const after = await p.evaluate("scrollY");
    check("閉じてもサマリーの位置のまま", Math.abs(after - y) <= 2, `${y} → ${after}`);
    check("閉じると押したボタンにフォーカスが戻る", await p.evaluate(id => { const a = document.activeElement; return a.dataset.sid === id && !!a.closest("#review"); }, id));
    await view("cal");
  });

  await step("サマリーの数字から内訳が開く", async () => {
    await view("sum");
    { // サマリーの頭：要点の文・推移といつもの範囲のグラフ・改善のアドバイス（見直す候補の上位 3 つまで）
      const r = await p.evaluate(() => ({heads: document.querySelectorAll("#review .insight .itile").length, cum: document.querySelectorAll("#review .insfig svg polyline.icum").length, donut: document.querySelectorAll("#review .iwr").length, hist: document.querySelectorAll("#review .insfig .ihb").length, svg: !!document.querySelector("#review .insfig svg[aria-label]"), bars: document.querySelectorAll("#review .insfig .ibar").length, flags: (lastFlags || []).length, tabs: document.querySelectorAll("#review [data-ins]").length}));
      check("上の帯の数字（推定費用・稼働時間）はサマリーのカードにもあり、押すと同じ内訳が開く", await p.evaluate(() => ["cost", "active"].every(m => document.querySelector(`#review .itile [data-metric="${m}"]`))));
      check("トークンの種類別のカード（種類ごとの積み上げ棒）がある", await p.evaluate(() => !!document.querySelector("#review .itbar") && document.querySelectorAll("#review .itbar > span").length >= 2));
      check("見直す候補の並びは、いつもより悪い側に外れたもの → 比べられないもの → 範囲の中（同じ段は関わる費用の大きい順）", await p.evaluate(() => {
        const F = lastFlags, side = f => insSide(f), ok = F.every((f, i) => i === 0 || side(F[i - 1]) < side(f) || side(F[i - 1]) === side(f) && ((F[i - 1].stake && F[i - 1].stake.c) ?? -1) >= ((f.stake && f.stake.c) ?? -1));
        return ok; }));
      check("見直す候補はどれも、サマリーの指標の横の印から開ける", await p.evaluate(() => [...new Set(lastFlags.map(f => GOTO[f.k] || f.k))].every(id => document.querySelector(`#review .fmark[data-goto="${id}"]`))));
      // 表示中の期間に出ていない候補も、出たときに印の置き場所があるか（モデルの選び方は「?」のない図に置く。置き場所がないと印が消える）。
      // 写しの上で候補をすべて置いてみて、どれにも見えている印が付くかを見る
      check("どの種類の見直す候補にも、見えている印が付く", await p.evaluate(() => { const R = document.querySelector("#review").cloneNode(true); R.querySelectorAll(".fmark").forEach(x => x.remove());
        document.body.append(R); try { const ks = ["limits", "fix", "friction", "longctx", "modelfit", "heavy", "repeats"]; placeFlags(R, ks.map(k => ({k, ids: []})));
          return ks.every(k => [...R.querySelectorAll(`.fmark[data-goto="${GOTO[k] || k}"]`)].some(m => !m.closest("[hidden]"))); } finally { R.remove(); } }));
      check("エージェント別の表に、エージェントごとの行がある（費用とクレジットを混ぜない）", await p.evaluate(() => document.querySelectorAll("#review .iagt tbody tr").length >= 2));
      check("サマリーにエージェントの切り替えはない（いつもすべてのエージェント）", await p.evaluate(() => !document.querySelector("#review [data-ag]")));
      { // 投入の量（推定費用・Kiro credits・トークン）は、どのエージェントの分もまとめ、棒はエージェントの色で積み上げる。色の凡例に名前を添える
        const stack = ch => p.evaluate(ch => { const t = document.querySelector(`#review .ik-cost .itile[data-ch="${ch}"]`); if (!t) return null;
          const segs = [...t.querySelectorAll("rect.sbar.sg")], names = [...t.querySelectorAll(".tagk .lg")].map(e => e.textContent);
          return {segs: segs.length, colors: new Set(segs.map(r => r.style.getPropertyValue("--c"))).size, names, tip: [...t.querySelectorAll("rect.hit")].some(h => names.some(n => (h.dataset.tip || "").includes(n + ":")))}; }, ch);
        const ok = r => !!r && r.segs >= 2 && r.colors >= 2 && r.names.length >= 2 && r.tip;
        const cr = await stack("credits");
        check("Kiro credits の棒は、Kiro のエージェントごとの色で積み上げ、凡例と棒のツールチップにエージェントの名前がある", ok(cr) && cr.names.every(n => /^Kiro /.test(n)), JSON.stringify(cr));
        for (const ch of ["cost", "tokens"]){ const r = await stack(ch);
          check(`${ch} の棒は、エージェントごとの色で積み上げる（Claude Code と Codex を 1 つの量に）`, ok(r) && r.names.includes("Claude Code") && r.names.includes("Codex"), JSON.stringify(r)); }
      }
      check("ヒートマップの各行に曜日の合計がある（触れなくても読める）", await p.evaluate(() => document.querySelectorAll("#review .ihgrid > .iht:not(.ihh)").length === 7));
      check("3 列の並びの最後の段に空きを作らない（余りは横に広げる）", await p.evaluate(() => [...document.querySelectorAll("#review .istats")].every(g => { const k = [...g.children].filter(c => c.offsetParent !== null), /* 単価の切り替えで隠した数字は数えない */ cols = getComputedStyle(g).gridTemplateColumns.split(" ").length; if (k.length < 2) return true; const r = k.map(c => c.getBoundingClientRect()), last = r[r.length - 1], row = r.filter(b => Math.abs(b.top - last.top) < 2); return Math.abs(row[row.length - 1].right - g.getBoundingClientRect().right) < 2 || row.length === cols; })));
      check("カードの中の数の枠は最大 3 列", await p.evaluate(() => [...document.querySelectorAll("#review .istats")].every(g => getComputedStyle(g).gridTemplateColumns.split(" ").length <= 3)));
      check("サマリーの頭は投入・産出・投入単価・産出単価の 4 枚のパネルで、どれもグラフ 1 つ（選んだ選択肢のタイルだけが見える）。広い画面では 2×2（左の列が投入の側、右の列が産出の側）、狭ければ投入・産出・投入単価・産出単価の順に積み、どの数字も画面の中に見える", await p.evaluate(() => { const G = [...document.querySelectorAll("#review .insh .ikg")], g = document.querySelector("#review .insh").getBoundingClientRect();
        const k = [...document.querySelectorAll("#review .insh .ikrow > .itile")].filter(c => c.offsetParent !== null), inside = k.every(c => { const r = c.getBoundingClientRect(); return r.left >= g.left - 1 && r.right <= g.right + 1; });
        const one = G.every(c => [...c.querySelectorAll(".ikrow > [data-ch]")].filter(x => x.checkVisibility()).length === 1 && c.querySelectorAll(".ikrow > .itile .tspk svg").length >= 1);
        const r = c => document.querySelector(`#review .insh .ik-${c}`).getBoundingClientRect();
        const lay = innerWidth >= 900 ? Math.abs(r("out").top - r("cost").top) < 2 && Math.abs(r("unit").top - r("uin").top) < 2 && r("uin").top >= r("cost").bottom - 1 && r("out").left >= r("cost").right - 1
            && Math.abs(r("uin").left - r("cost").left) < 2 && Math.abs(r("unit").left - r("out").left) < 2
          : ["out", "uin", "unit"].every((c, i, a) => r(c).top >= r(i ? a[i - 1] : "cost").bottom - 1 && Math.abs(r(c).left - r("cost").left) < 2 && Math.abs(r(c).width - r("cost").width) < 2);
        return G.length === 4 && k.length === 4 && one && inside && lay; }));
      { const panelNames = () => p.evaluate(() => ({panels: [...document.querySelectorAll("#review .insh .ikg .ikgh")].map(h => [...h.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join("").trim()),
      out: !!document.querySelector("#review .iout") && [...document.querySelector("#review .iout").querySelectorAll(":scope > h4")].map(h => [...h.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join("").trim()).join("")}));
        const n = await panelNames();
        check("パネルの名前は Input・Output・Unit cost per input・Unit cost per output、内訳のカードは「Output」", JSON.stringify(n.panels) === JSON.stringify(["Input", "Output", "Unit cost per input", "Unit cost per output"]) && n.out === "Output", JSON.stringify(n)); }
      check("パネルは投入の側（投入・投入単価）と産出の側（産出・産出単価）に分かれ、広い画面では列の間の 2 色の線の上に IN・OUT、狭い画面では各パネルの上に IN・OUT がある", await p.evaluate(() => {
        const side = {cost: "in", uin: "in", out: "out", unit: "out"}, vis = e => !!e && e.checkVisibility(), G = Object.keys(side).map(c => document.querySelector(`#review .insh .ik-${c}`));
        const cls = G.every((g, i) => g.classList.contains("side-" + Object.values(side)[i]));
        if (innerWidth >= 900){ const d = document.querySelector("#review .insdiv"), dr = d.getBoundingClientRect(), I = document.querySelector("#review .insio.in"), O = document.querySelector("#review .insio.out"), c = G[0].getBoundingClientRect(), o = G[2].getBoundingClientRect(), u = G[3].getBoundingClientRect();
          return cls && vis(d) && dr.left >= c.right - 1 && dr.right <= o.left + 1 && dr.bottom >= u.bottom - 1 && vis(I) && vis(O) && I.textContent === "IN" && O.textContent === "OUT" && I.getBoundingClientRect().bottom <= c.top + 1 && I.getBoundingClientRect().right <= dr.left + 1 && O.getBoundingClientRect().left >= dr.right - 1
            && getComputedStyle(I).color !== getComputedStyle(O).color && G.every(g => !vis(g.querySelector(".ikio"))); }
        return cls && !vis(document.querySelector("#review .insdiv")) && G.every((g, i) => { const l = g.querySelector(".ikio"); return vis(l) && l.textContent === (Object.values(side)[i] === "in" ? "IN" : "OUT") && l.getBoundingClientRect().bottom <= g.querySelector(".ikgh").getBoundingClientRect().top + 1; }); }));
      const chartTops = () => p.evaluate(() => { const top = c => { const t = [...document.querySelectorAll(`#review .insh .ik-${c} .ikrow > .itile`)].find(x => x.checkVisibility()), s = t && t.querySelector(".tspk svg"); return s ? Math.round(s.getBoundingClientRect().top) : NaN; };
        return {cost: top("cost"), out: top("out"), uin: top("uin"), unit: top("unit")}; });
      const aligned = t => Math.abs(t.cost - t.out) <= 4 && Math.abs(t.uin - t.unit) <= 4;
      if (env.viewport.width >= 900){ const t = await chartTops(); check("横に並んだ 2 枚のパネルのグラフは、同じ高さから始まる（4px 以内）", aligned(t), JSON.stringify(t)); }
      check("各パネルの凡例はグラフの下にある", await p.evaluate(() => [...document.querySelectorAll("#review .insh .ikg")].every(g => { const t = [...g.querySelectorAll(".ikrow > .itile")].find(x => x.checkVisibility()), s = t && t.querySelector(".tspk svg"), l = g.querySelector(":scope > .tlg");
        return !s || (!!l && l.getBoundingClientRect().top >= s.getBoundingClientRect().bottom - 1); })));
      // 凡例の項目は、どれもそのパネルで見せているグラフが描いた印の説明（描いていない推定・予測などを並べない）
      const lgCheck = async when => { const r = await p.evaluate(() => [...document.querySelectorAll("#review .insh .ikg")].map(g => { const t = [...g.querySelectorAll(".ikrow > .itile")].find(x => x.checkVisibility()), svg = t && t.querySelector(".tspk svg");
          const items = [...g.querySelectorAll(":scope > .tlg .lg")].filter(x => x.checkVisibility());
          return {p: [...g.classList].find(c => c.startsWith("ik-")), items: items.map(x => x.textContent), bad: items.filter(x => !x.dataset.lg || !svg || !svg.querySelector(x.dataset.lg)).map(x => x.textContent)}; }));
        check(`${when}：パネルの凡例の項目は、どれもそのパネルのグラフに描いた印の説明`, r.length === 4 && r.every(x => x.items.length > 0 && x.bad.length === 0), JSON.stringify(r)); return r; };
      await lgCheck("開いた期間");
      // x 軸の期間の名前は、どのパネルのグラフでも横書き（rotate しない）。名前どうしも、x 軸の下の段の利用上限の印（三角・×n）とも重ならない
      const xlCheck = async when => { const r = await p.evaluate(() => [...document.querySelectorAll("#review .insh .ikrow > .itile .tspk svg")].filter(s => s.checkVisibility()).map(s => {
          const hit = (a, b) => a.left < b.right - 0.5 && b.left < a.right - 0.5 && a.top < b.bottom - 0.5 && b.top < a.bottom - 0.5;
          const xs = [...s.querySelectorAll("text.sxl")], vis = xs.filter(x => x.checkVisibility()).map(x => x.getBoundingClientRect()), lim = [...s.querySelectorAll("path.slim, text.slimn")].map(x => x.getBoundingClientRect());
          return {ch: s.closest("[data-ch]").dataset.ch, rot: xs.filter(x => /rotate/.test(x.getAttribute("transform") || "") || x.classList.contains("rot")).length, n: vis.length, lim: lim.length,
            overL: vis.filter((a, i) => vis.some((b, j) => j !== i && hit(a, b))).length, overM: vis.filter(a => lim.some(b => hit(a, b))).length}; }));
        check(`${when}：パネルのグラフの x 軸の名前は横書きで、名前どうしも利用上限の印とも重ならない`, r.length >= 4 && r.every(x => x.rot === 0 && x.n >= 2 && x.overL === 0 && x.overM === 0), JSON.stringify(r.filter(x => x.rot || x.overL || x.overM || x.n < 2))); };
      await xlCheck("開いた期間");
      check("投入・産出のパネルのグラフは期間ごとの棒（表示中の期間の棒は 1 本で、点と線はない）、単価のパネルのグラフは点と線（棒はない）。切り替えで隠れたタイルも", await p.evaluate(() => ["cost", "out", "uin", "unit"].every(c => { const ss = [...document.querySelectorAll(`#review .insh .ik-${c} .ikrow > .itile .tspk svg`)];
        return ss.length > 0 && ss.every(s => { const bars = s.querySelectorAll("rect.sbar").length, cur = new Set([...s.querySelectorAll("rect.sbar.cur")].map(r => r.getAttribute("x"))).size, /* 積み上げ（Kiro credits）は区分ごとの rect なので、棒の位置で数える */ dots = s.querySelectorAll("polyline.trl, circle.now, circle.pt").length;
          return c === "cost" || c === "out" ? bars >= 2 && cur === 1 && dots === 0 : bars === 0 && s.querySelectorAll("circle.now").length === 1 && !!s.querySelector("polyline.trl"); }); })));
      if (env.viewport.width >= 900){ // 頭の段は subgrid で横の 2 枚と同じ高さ。片方の頭にだけ注の行があると、もう片方の頭の下が空く
        const heads = when => p.evaluate(() => { const slack = c => { const h = document.querySelector(`#review .insh .ik-${c} > .ikhd`); if (!h) return null; const last = [...h.children].filter(x => x.checkVisibility()).pop();
          return Math.round(h.getBoundingClientRect().bottom - last.getBoundingClientRect().bottom); }; return {uin: slack("uin"), unit: slack("unit"), cost: slack("cost"), out: slack("out")}; })
          .then(hg => check(`${when}：横に並んだパネルの頭は、どちらも中身の高さ（片方の頭の注のために、もう片方の頭の下が空かない）`, (hg.uin == null || hg.unit == null || Math.abs(hg.uin - hg.unit) <= 4) && (hg.cost == null || hg.out == null || Math.abs(hg.cost - hg.out) <= 4), JSON.stringify(hg)));
        await heads("すべて"); }
      { // 途中の期間：期間の終わりの推定は、今までの値と同じ行に並ぶ（狭い画面では折り返してよいが、パネルの外に出ない）
        await p.evaluate(() => document.activeElement && document.activeElement.blur()); const lab0 = await p.locator("#rd").innerText(); await p.keyboard.press("t"); await pause();
        const r = await p.evaluate(() => [...document.querySelectorAll("#review .insh .ikrow > .itile")].filter(x => x.checkVisibility() && x.querySelector(".ivbig")).map(x => { const v = x.querySelector(".ivnow > .itv, .ivnow > b").getBoundingClientRect(), e = x.querySelector(".ivbig b").getBoundingClientRect(), t = x.getBoundingClientRect();
          return {ch: x.dataset.ch, same: e.top < v.bottom && e.bottom > v.top && Math.abs(e.bottom - v.bottom) < 8, inside: e.right <= t.right + 1 && v.right <= t.right + 1, below: e.top >= v.bottom - 2}; }));
        check("途中の期間の推定は、今までの値と同じ行に並ぶ（1100px より狭い画面では次の行に折り返してよい）。どれもパネルの中", r.length >= 2 && r.every(x => x.inside && (env.viewport.width >= 1100 ? x.same : x.same || x.below)), JSON.stringify(r));
        if (env.viewport.width >= 900){ const t = await chartTops(); check("途中の期間でも、横に並んだ 2 枚のパネルのグラフは同じ高さから始まる（4px 以内）", aligned(t), JSON.stringify(t)); }
        await xlCheck("途中の期間");
        check("途中の期間：投入のグラフは、今までの値の棒と、期間の終わりの推定までの破線の棒（凡例にも）", await p.evaluate(() => { const g = document.querySelector("#review .insh .ik-cost"), t = [...g.querySelectorAll(".ikrow > .itile")].find(x => x.checkVisibility()), s = t && t.querySelector(".tspk svg");
          const cur = s && [...s.querySelectorAll("rect.sbar.cur")].sort((p, q) => +p.getAttribute("y") - +q.getAttribute("y"))[0] /* 積み上げでは、いちばん上の区分 */, ext = s && s.querySelector("rect.sbar.ext"), lg = [...g.querySelectorAll(":scope > .tlg .lg")].filter(x => x.checkVisibility()).map(x => x.dataset.lg);
          return !!cur && !!ext && Math.abs(+ext.getAttribute("y") + +ext.getAttribute("height") - +cur.getAttribute("y")) < 0.2 && lg.includes("rect.sbar.ext") && lg.some(x => /^rect\.sbar\.(sg\.)?cur/.test(x)); }));
        { const r = await lgCheck("途中の期間"); const u = r.filter(x => x.p === "ik-uin" || x.p === "ik-unit");
          check("途中の期間でも、単価のパネルの凡例に推定・予測は出ない（単価のグラフには描かない）", u.length === 2 && u.every(x => !x.items.some(i => /estimate|Projection|予測/.test(i))), JSON.stringify(u)); }
        for (let i = 0; i < 30 && await p.locator("#rd").innerText() !== lab0; i++){ await p.keyboard.press("ArrowLeft"); await pause(); } check("今週から元の週に戻る", await p.locator("#rd").innerText() === lab0);
      }
      { // パネルの切り替え：押すと別の選択肢のタイル（とグラフ）になり、開き直しても覚えている
        const sws = await p.evaluate(() => [...document.querySelectorAll("#review .insh .iksw")].map(g => ({sw: g.dataset.sw, vs: [...g.querySelectorAll("[data-v]")].map(b => b.dataset.v), on: [...g.querySelectorAll('[data-v]:not([hidden])')].map(b => b.dataset.v), cur: g.querySelector('[aria-pressed="true"]').dataset.v})));
        const want = {cost: ["cost", "credits", "tokens", "active"], out: ["commit", "push", "pr", "merge", "review", "approve"], unitN: ["cost", "credits", "tokens"], unitD: ["commit", "push", "pr", "merge", "review"], uinN: ["cost", "credits", "tokens"], uinD: ["Hour", "Day", "Ses", "Ask"]};
        check("どのパネルにも切り替えがあり（単価は量と割る数の 2 つ）、選択肢はいつも同じ並びで、数字のあるものだけ", Object.keys(want).every(k => { const x = sws.find(z => z.sw === k); return x && x.on.length > 0 && x.vs.join() === want[k].filter(v => x.vs.includes(v)).join() && x.on.includes(x.cur); }), JSON.stringify(sws));
        const shown = sw => p.evaluate(sw => { const g = document.querySelector(`#review .iksw[data-sw="${sw}"]`), sec = g.closest(".ikg"), v = [...sec.querySelectorAll(".ikrow > [data-ch]")].filter(x => x.checkVisibility());
          return {n: v.length, ch: v[0] && v[0].dataset.ch, svg: v[0] ? (v[0].querySelector(".tspk svg") || {}).innerHTML || "" : "", label: v[0] ? (v[0].querySelector(".il") || v[0]).textContent : ""}; }, sw);
        const picked = {};
        for (const {sw} of sws){
          const {on, cur} = await p.evaluate(sw => { const g = document.querySelector(`#review .insh .iksw[data-sw="${sw}"]`); return {on: [...g.querySelectorAll('[data-v]:not([hidden])')].map(b => b.dataset.v), cur: g.querySelector('[aria-pressed="true"]').dataset.v}; }, sw); // 量を変えると、選べる割る数が変わる
          const before = await shown(sw), to = on.find(v => v !== cur), b = p.locator(`#review .iksw[data-sw="${sw}"] [data-v="${to}"]`);
          await b.click(); await pause(); const after = await shown(sw);
          check(`切り替え ${sw}: ${to} を押すと、そのタイルだけが見え、グラフが変わり、ボタンが選ばれてフォーカスが残る`, after.n === 1 && after.ch.split(":")[/D$/.test(sw) ? 1 : 0] === to && after.svg !== before.svg && after.label !== before.label
            && await b.getAttribute("aria-pressed") === "true" && await b.evaluate(e => document.activeElement === e && e.closest('[role="group"]') !== null), `${JSON.stringify(before).slice(0, 80)} → ${JSON.stringify(after).slice(0, 80)}`);
          picked[sw] = {to, ch: after.ch};
        }
        // 選択は覚えておき（localStorage）、描き直してもそのまま。file:// を開き直すと、Chromium が localStorage を空で始めることがあるので、開き直しではなくサマリーを描き直して見る
        const kept = await p.evaluate(() => ({costKind: store.get("costKind"), outKind: store.get("outKind"), unitKind: store.get("unitKind"), unitDen: store.get("unitDen"), ucKind: store.get("ucKind"), ucDen: store.get("ucDen")}));
        check("切り替えの選択は localStorage に覚えている（投入単価は、前の単価のカードの鍵を引き継ぐ）", kept.costKind === picked.cost.to && kept.outKind === picked.out.to && kept.unitKind === picked.unitN.to && kept.unitDen === picked.unitD.to && kept.ucKind === picked.uinN.to && kept.ucDen === picked.uinD.to, JSON.stringify(kept));
        await view("cal"); await view("sum"); await p.evaluate(() => summary()); await pause();
        for (const sw of Object.keys(picked)){ const x = await shown(sw), pr = await p.locator(`#review .iksw[data-sw="${sw}"] [aria-pressed="true"]`).getAttribute("data-v");
          check(`切り替え ${sw} の選択は、描き直しても覚えている`, pr === picked[sw].to && x.ch.split(":")[/D$/.test(sw) ? 1 : 0] === picked[sw].to, `${pr} ${x.ch}`); }
        for (const {sw, cur} of sws){ await p.locator(`#review .iksw[data-sw="${sw}"] [data-v="${cur}"]`).click(); await pause(); } // 元に戻す（ほかの確かめは最初の選択肢で）
      }
      check("式はふだん隠し、指標名の横の「ƒx」で、その数字の式だけを開く（もう一度押すと閉じる）", await p.evaluate(async () => {
        const vis = el => [...el.querySelectorAll(".ifx.fxo")].some(e => e.offsetParent !== null), I = document.querySelector("#review .insight");
        const b = I.querySelector(".itile .fxb"), tile = b && b.closest(".itile"); if (!tile || vis(I)) return false;
        b.click(); await new Promise(r => setTimeout(r, 100)); const on = vis(tile) && b.getAttribute("aria-pressed") === "true" && [...I.querySelectorAll(".itile")].filter(x => x !== tile).every(x => !vis(x));
        b.click(); await new Promise(r => setTimeout(r, 100)); return on && !vis(I); }));
      check("サマリーの頭に、数字 5〜32 個（上の帯の数字を含む。単価は Git コミット・AI の編集行・稼働時間・セッション・プロンプトあたりの 5 段。コスト・産出（Git はコミット・行・ファイル・リポジトリ・日・push・PR、AI の編集はファイルと行）・単価（Git コミットあたりと AI が編集した 100 行あたりを分ける）に分ける）・どこに使ったかの横棒がある", r.heads >= 5 && r.heads <= 32 && r.svg && r.donut > 0, JSON.stringify(r));
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
          return ts.length && Math.abs(Math.max(...ts.map(c => c.getBoundingClientRect().right)) - rr.right) >= 4; }).map(r => (r.closest(".ikg") || r).className + " " + JSON.stringify([...r.children].map(c => [c.className.slice(0, 20), (c.querySelector(".il") || {}).textContent, Math.round(c.getBoundingClientRect().left), Math.round(c.getBoundingClientRect().right), !!c.querySelector(".tspk")])) + " " + Math.round(r.getBoundingClientRect().right));
        // カードもタイルも単価の段も、広い画面ではページの半分より広くない（グラフのない数字の行は除く）
        const half = innerWidth < 1100 || [...document.querySelectorAll("#review .insrow > *, #review .insh > .ikg, #review .ikrow > .itile:has(.tspk)")].filter(e => e.checkVisibility()).every(e => e.getBoundingClientRect().width <= innerWidth / 2 + 1);
        const tilesOk = half && !badTiles.length
;
        const three = innerWidth < 1300 || [...document.querySelectorAll("#review .insrow")].some(r => r.children.length >= 3 && Math.abs(r.children[0].getBoundingClientRect().top - r.children[2].getBoundingClientRect().top) < 1);
        return {say, charts, rows: rowsOk, tiles: tilesOk, half, badTiles, grid: three}; });
      check("どのグラフにも、データから計算した読み取りの 1 文があり、広い画面ではカードが 3 列に並び、カードは重ならず中の下が空かない。カードとグラフのタイルはページの半分より広くなく、奇数のタイルも行の幅を埋める", lay.say === lay.charts && lay.rows && lay.tiles && lay.grid, JSON.stringify(lay));
      check("投入と産出の棒の上には、% ではなくその期間の実際の量がある", await p.evaluate(() => { const l = [...document.querySelectorAll("#review .ik-cost .tspk:not(.ww) .spc, #review .ik-out .tspk:not(.ww) .spc")]; return l.length > 0 && l.every(e => /^\$?\d+(\.\d)?[kMh]?$|^\d+m$/.test(e.textContent)); }));
      check("1 本だけ飛び抜けた棒は上を切り、切れ目の印と実際の量を付ける（ほかの棒が潰れない）。戻せば切らない", await p.evaluate(() => {
        const ks = Object.keys(WEEKS).sort(), w = WEEKS[ks[ks.length - 5]], old = w.usage.cost; w.usage.cost = 240; summary();
        const g = document.querySelector("#review .ik-cost .tspk:not(.ww)"), cut = !!g && g.querySelectorAll("path.sbrk").length === 1 && [...g.querySelectorAll(".spc.over")].some(e => e.textContent === "$240") && !!document.querySelector('#review .ik-cost .tlg [data-lg="path.sbrk"]:not([hidden])');
        w.usage.cost = old; summary(); const g2 = document.querySelector("#review .ik-cost .tspk:not(.ww)");
        return cut && !!g2 && g2.querySelectorAll("path.sbrk").length === 0; }));
      const h = await p.evaluate(() => ({rows: document.querySelectorAll("#review .ihgrid .ihd").length, cells: document.querySelectorAll("#review .ihgrid .ihc").length}));
      check("曜日×時間のヒートマップ（7×24）がある", h.rows === 7 && h.cells === 168, JSON.stringify(h));
      check("推移・累積・散布図・長さ別・箱ひげ・ガントのカードはない（上のパネルとカレンダーと重なるので消した）", await p.evaluate(() => !document.querySelector("#review .trendk, #review .insksel, #review polyline.icum, #review .ipt, #review .ihist, #review .ibxf, #review .igt, #review [data-ins], #review [data-cum], #review [data-scat], #review [data-bxk]")));
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

  if (env === envs[0]) await step("月末の推定の幅は 3σ だけ", async () => { // 内訳の表の中身は幅によらない
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
    check("振り返りは、プロンプトをコピーするボタンだけ（AI を動かすボタンはない）", await p.locator("#panel #srun.ai-copy").count() === 1 && (env !== envs[0] || await p.locator("#panel .ai-action").count() === 0));
    { const tx = await p.locator("#panel .drun").innerText(); check("振り返りのボタンの下は、エージェントに貼るプロンプトをコピーすると書くだけ（何も送らないとはあえて書かない）", /(Copies a review prompt|振り返りのプロンプトをコピー)/.test(tx) && !/(sends nothing|何も送りません)/.test(tx), tx); }
    check("AI が動いた時間は、あなたの吹き出しの中に入れない", await p.evaluate(() => !document.querySelector("#panel .tl li.pr:not(.ev) .pmeta")));
    // 左（1 列の画面では間）はプロンプトの流れだけ。再開のコマンドは操作（.sact）に 1 つだけ見え、2 列なら右の列、1 列なら流れより上
    const rs = await p.evaluate(() => { const v = [...document.querySelectorAll("#panel .dresume")].filter(e => e.offsetParent !== null);
      const act = document.querySelector("#panel .sact"), flow = document.querySelector("#panel .sflow");
      if (!act || !flow) return {n: v.length, ok: false};
      const a = act.getBoundingClientRect(), f = flow.getBoundingClientRect(), wide = innerWidth >= 1100;
      return {n: v.length, inAct: !v.length || !!v[0].closest(".sact"), wide, place: wide ? a.left > f.right - 1 : a.top < f.top,
        flowOnly: !flow.querySelector(".mini, .files, .trow, .dresume, #srun"),
        // HTML の順（Tab の順）が見た目の順と同じ：1 列では操作 → 数値 → 流れ → 情報（モデル・ツールは流れより下）。
        // 2 列では左の流れ → 右の列（操作 → 数値 → 情報）。「閉じる → 右 → 左 → 右」と行き来しない
        order: (wide ? [".sflow", ".sact", ".smet", ".sinfo"] : [".sact", ".smet", ".sflow", ".sinfo"]).map(c => document.querySelector("#panel .scols > " + c)).filter((e, i) => e || i !== (wide ? 1 : 0)).every((e, i, a) => e && (!i || (a[i-1].compareDocumentPosition(e) & Node.DOCUMENT_POSITION_FOLLOWING))),
        toolsBelow: wide || [...document.querySelectorAll("#panel .trow")].every(e => e.getBoundingClientRect().top > f.bottom - 1)}; });
    check("詳細の HTML の順は、操作・数値・流れ・情報（Tab の順が見た目と合う）", rs.order === true, JSON.stringify(rs));
    check("1 列では、使ったツールは流れより下", rs.toolsBelow === true, JSON.stringify(rs));
    check("詳細の流れの列には、プロンプトの流れだけがある", rs.flowOnly === true, JSON.stringify(rs));
    check("再開のコマンドと AI 分析は操作にあり、2 列なら右の列、1 列なら流れより上", rs.n <= 1 && rs.inAct && rs.place, JSON.stringify(rs));
    if (env === envs[0]){ // プロンプトの文は幅によらない
    const rv = await p.evaluate(() => { const s = DATA.find(x => x.id === st.sel) || DATA[0]; return sessionPrompt(s, 1, null); });
    check("振り返りは専門的な助言と証拠を備える", /professional AI prompt advisor/.test(rv) && /^- Title: /m.test(rv) && /^### Professional prompt assessment$/m.test(rv) && /^### Prioritized advice$/m.test(rv) && /^### Reusable prompt playbook$/m.test(rv) && /^\| \| This session \|$/m.test(rv) && /^\| Prompts \| \d+ \|$/m.test(rv), rv.slice(0, 200));
    check("振り返りの事実は、コードブロックの中にある（履歴ファイルを読めるなら、セッション ID とファイルと、履歴にない出来事だけ。プロンプトは書き写さない）", /# History data\n`{3,}text\n# Facts/.test(rv) && (/- Session ID: .+\n- History file: /.test(rv) ? /## Events kiroku adds/.test(rv) && !/## Prompt flow/.test(rv) : /## Prompt flow/.test(rv)), rv.slice(-400)); }
    await p.keyboard.press("Escape"); await pause();
  });

  if (env === envs[0]) await step("詳細と矢印キー・ブラウザの戻る", async () => { // キーと履歴の扱いは幅によらない
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
    // コミット・push・PR の数は上の「産出」のパネルに（下の成果のカードでは繰り返さない）。Git の行・やり直し、AI の編集の定着は成果のカードに
    for (const [name, id] of [["コミット", "gitCommits"], ["push", "pushes"], ["PR", "prs"]])
      check(`上の産出のパネルに${name}が出て、下の成果のカードでは繰り返さない`, await p.locator(`#review .insh .ik-out .hb[data-help="${id}"]`).count() === 1 && await oc.locator(`.hb[data-help="${id}"]`).count() === 0);
    const rwk = await p.evaluate(() => { const {ws, we} = period(); return reworkOf(ws, we).n > 0; }); // やり直しは 3 日たった AI のコミットがある期間だけ
    for (const id of ["lines", ...(rwk ? ["rework"] : []), "kept", "aiLines", "aiFiles"])
      check(`パネルから外した ${id} は成果のカードにある`, await oc.locator(`.hb[data-help="${id}"]`).count() === 1 && await p.locator(`#review .insh .hb[data-help="${id}"]`).count() === 0);
    { // 時間・日・セッション・プロンプトあたりは投入単価のパネルに（単価の表のカードはない）。HELP のある指標は、どれもサマリーのどこかに「?」がある
      const miss = await p.evaluate(() => [...["costPerHour", "costPerDay", "costPerSes", "costPerAsk", "tokensPerHour", "tokensPerDay", "tokensPerSes", "tokensPerAsk", "creditsPerHour", "creditsPerSes", "creditsPerAsk"].map(k => ["uin", k])].filter(([g, k]) => !document.querySelector(`#review .insh .ik-${g} .hb[data-help="${k}"]`)).map(x => x.join(":")));
      check("投入単価のパネルに時間・日・セッション・プロンプトあたりの単価がある", miss.length === 0, miss.join(","));
      const none = await p.evaluate(rwk => ["cost", "credits", "tokens", "active", "gitCommits", "lines", "prs", "pushes", "aiLines", "aiFiles", ...(rwk ? ["rework"] : []), "kept", "outSessions", "costPerCommit", "tokensPerCommit", "perPush", "perPr", "prompts", "cache", "subagents", "fix", "limits", "focus", "switches", "wait", "bigPrompts", "repeats", "friction", "compactions"].filter(k => !document.querySelector(`#review .hb[data-help="${k}"]`)), rwk);
      check("HELP のある指標は、サマリーのどこかに「?」がある", none.length === 0, none.join(","));
    }
    check("成果のカードに成果まで届いたセッションが出る", await oc.locator('.hb[data-help="outSessions"]').count() === 1);
    check("コミットあたりの費用は、産出ではなく産出単価のパネルにある", await p.evaluate(() => !!document.querySelector('#review .insh .ik-unit [data-metric="costPerCommit"]') && !document.querySelector('#review .ik-out [data-metric="costPerCommit"], #review .iout .hb[data-help="costPerCommit"]')));
    check("割った指標には式（÷）が添えてある", await oc.evaluate(e => [...e.querySelectorAll(".mrow")].filter(s => s.querySelector('.hb[data-help="outSessions"], .hb[data-help="costPerCommit"]')).every(s => /÷/.test(s.textContent))));
    check("カードが箱からはみ出さない", await oc.evaluate(e => [...e.querySelectorAll(".mrow")].every(s => s.getBoundingClientRect().right <= e.getBoundingClientRect().right + 1)));
    { // 数字の大きさの段：4 つのパネルの値がいちばん大きく、内訳のカードの値（Output の行・数の枠）はそれより小さい
      const fs = await p.evaluate(() => { const px = sel => [...document.querySelectorAll(sel)].filter(e => e.checkVisibility()).map(e => parseFloat(getComputedStyle(e).fontSize));
        return {panel: px("#review .insh .ikg .ivrow .itv, #review .insh .ikg .ivrow > b, #review .insh .ikg .ivnow > b"), card: px("#review .insrow .mrow .mv, #review .insrow .stat .v")}; });
      check("パネルの値は、内訳のカードの値より大きい（1.5 倍以上）", fs.panel.length >= 4 && fs.card.length > 0 && Math.min(...fs.panel) >= 1.5 * Math.max(...fs.card), JSON.stringify(fs));
    }
    { // いつもの範囲から大きく外れた量（かなり上・下）だけが、中立の色（.odd）と値の横の矢印（↑・↓。言葉は読み上げ用の隠れた文字と title だけ）を持つ。単価は評価の文の色があるので付けない。
      // 期間をさかのぼって、外れた数字のある期間を少なくとも 1 つ見る
      const odd = () => p.evaluate(() => { const far = /far (above|below)|かなり(上|下)/, bad = [];
        let n = 0;
        // 矢印の札：見える字は ↑ か ↓ だけ（言葉は出さない）。言葉は .sr と title にあり、色は値と同じ琥珀
        const word = /Far (above|below) usual|いつもよりかなり(高|低)い/;
        const arrow = (mk, v) => { if (!mk) return false; const vis = [...mk.childNodes].filter(x => !(x.nodeType === 1 && x.classList.contains("sr"))).map(x => x.textContent).join("").trim();
          return /^[↑↓]$/.test(vis) && word.test((mk.querySelector(".sr") || {}).textContent || "") && word.test(mk.title) && getComputedStyle(mk).color === getComputedStyle(v).color; };
        document.querySelectorAll("#review .insh .ikg .itile").forEach(e => { const v = e.querySelector(".ivrow .itv, .ivrow > b, .ivnow > b"), c = e.querySelector(".ichip .cp"); if (!v) return;
          const want = !e.closest(".ik-unit, .ik-uin") && !!c && far.test(c.textContent), has = v.classList.contains("odd"), mk = e.querySelector(".ivrow .iodd");
          if (has) n++; if (want !== has || want !== !!mk || (mk && !arrow(mk, v))) bad.push(`${e.dataset.ch}:${want}/${has}/${!!mk}`); });
        document.querySelectorAll("#review .insrow .mrow").forEach(e => { const m = e.querySelector(".imk"); if (!m) return;
          const want = far.test(m.querySelector(".sr").textContent) && !m.matches(".good, .bad"), v = e.querySelector(".mv .odd"), has = !!v, mk = e.querySelector(".mv .iodd");
          const words = m.classList.contains("odd") && m.innerText.replace(m.querySelector(".sr").innerText, "").trim() !== ""; // 外れた行の印に、見える言葉を出さない
          if (has) n++; if (want !== has || want !== m.classList.contains("odd") || want !== !!mk || (mk && !arrow(mk, v)) || words) bad.push(`${e.querySelector(".mk").textContent}:${want}/${has}/${!!mk}/${words}`); });
        if ([...document.querySelectorAll("#review .iodd")].some(e => e.innerText.replace((e.querySelector(".sr") || {}).innerText || "", "").trim().length > 1)) bad.push("words next to an arrow");
        document.querySelectorAll("#review .ik-unit .odd, #review .ik-unit .iodd, #review .ik-uin .odd, #review .ik-uin .iodd").forEach(e => bad.push("unit cost marked: " + e.textContent));
        // 大きく外れたタイル（.far・.odd）も、ふつうのタイルと同じ余白と箱：指標の名前は、横に並んだパネルの名前と同じ高さ、パネルの左端から同じ位置
        const vt = c => [...document.querySelectorAll(`#review .insh .ik-${c} .ikrow > .itile`)].find(x => x.checkVisibility()), farT = [], mis = [];
        const nm = c => { const x = vt(c), g = document.querySelector(`#review .insh .ik-${c}`); if (!x) return null; const r = x.querySelector(".il").getBoundingClientRect(), cs = getComputedStyle(x);
          if (x.matches(".far") || x.querySelector(".ivrow .odd")) farT.push(c); return {top: r.top, left: r.left - g.getBoundingClientRect().left, pad: cs.padding, bg: cs.backgroundColor}; };
        const T = Object.fromEntries(["cost", "out", "uin", "unit"].map(c => [c, nm(c)])), ref = Object.values(T).find(Boolean);
        for (const [a, b] of [["cost", "out"], ["uin", "unit"]]) if (T[a] && T[b] && innerWidth >= 900 && Math.abs(T[a].top - T[b].top) > 1) mis.push(`${a}/${b} top ${Math.round(T[a].top)}/${Math.round(T[b].top)}`);
        for (const [c, x] of Object.entries(T)) if (x && (Math.abs(x.left - ref.left) > 1 || x.pad !== ref.pad || x.bg !== ref.bg)) mis.push(`${c} ${JSON.stringify(x)} vs ${JSON.stringify(ref)}`);
        return {n, bad, far: farT, mis}; });
      await p.evaluate(() => document.activeElement && document.activeElement.blur());
      let seen = 0, back = 0, farSeen = 0; const bads = [], mis = [];
      for (; back < 8; back++){ const r = await odd(); seen += r.n; bads.push(...r.bad); farSeen += r.far.length; mis.push(...r.mis.map(m => `-${back}: ${m}`)); if (seen && back && farSeen) break; await p.keyboard.press("ArrowLeft"); await pause(); }
      check("大きく外れた量だけに中立の色と値の横の矢印（言葉は読み上げと title だけ）が付き、いつもの範囲の数字と単価には付かない", seen > 0 && bads.length === 0, JSON.stringify({seen, back, bads}));
      check("大きく外れた値のタイルも、ふつうのタイルと同じ余白と箱（地の色なし）で、指標の名前は横のパネルの名前とそろう", farSeen > 0 && mis.length === 0, JSON.stringify({farSeen, back, mis}));
      for (let i = 0; i < back; i++){ await p.keyboard.press("ArrowRight"); await pause(); }
    }
    { // アウトプットは数の枠ではなく、Git と AI の編集の 2 つの列の 1 行ずつの一覧（名前・値・いつもの範囲の印・「ƒx」「?」）。長い説明は「ƒx」で開くまで隠す
      const o = await oc.evaluate(e => { const cols = [...e.querySelectorAll(".iosec")].map(c => [...c.querySelectorAll(".mrow")]), rows = cols.flat();
        const r = id => { const h = e.querySelector(`.mrow .hb[data-help="${id}"]`); return h && h.closest(".mrow"); }, gl = r("lines");
        const side = cols.length < 2 || e.getBoundingClientRect().width < 500 || Math.abs(cols[0][0].getBoundingClientRect().top - cols[1][0].getBoundingClientRect().top) < 40;
        return {n: cols.map(c => c.length), stats: e.querySelectorAll(".iosec .stat").length, named: rows.every(x => x.querySelector(".mk > span") && x.querySelector(".mv")),
          fxb: rows.filter(x => x.querySelector(".mx")).every(x => x.querySelector(".fxb")), hidden: rows.every(x => !x.querySelector(".mx") || !x.querySelector(".mx").checkVisibility()),
          mark: !!(gl && gl.querySelector(".mm .imk")), vs: !!(gl && /vs|比/.test(gl.querySelector(".mm").textContent)), side}; });
      check("アウトプットは Git と AI の編集の 2 列の 1 行ずつの一覧で、数の枠を使わない", o.n.length === 2 && o.n[0] >= 5 && o.n[1] >= 2 && o.stats === 0 && o.named && o.side, JSON.stringify(o));
      check("アウトプットの行は、式と説明を「ƒx」で開くまで隠し、Git の行にはいつもの範囲の印と前の週との比べがある", o.fxb && o.hidden && o.mark && o.vs, JSON.stringify(o));
      const row = oc.locator('.mrow:has(.hb[data-help="rework"]), .mrow:has(.hb[data-help="kept"])').first(); await row.locator(".fxb").click(); await pause();
      check("行の「ƒx」を押すと、その行の式と注が開く（やり直し・定着の注は消えずにここにある）", await row.evaluate(r => r.querySelector(".mx").checkVisibility() && /÷/.test(r.querySelector(".mx").textContent) && r.querySelectorAll(".fxn").length >= 1));
      await row.locator(".fxb").click(); await pause();
    }
    { // AI の編集：ファイルの数・行と、いちばん変えたファイル（期間に始まったセッションの編集から。押すと、いちばん変えたセッション）
      const ed = await p.evaluate(() => { const {ws, we} = period(), xs = DATA.filter(s => s.start >= ws && s.start < we && s.nFiles > 0);
        const c = document.querySelector('#review .inscol:has(.hb[data-help="aiLines"])'); if (!c) return {none: true, n: xs.length};
        const add = xs.reduce((a, s) => a + (s.editAdded || 0), 0), del = xs.reduce((a, s) => a + (s.editRemoved || 0), 0);
        return {n: xs.length, files: document.querySelectorAll("#review .iwf").length, lines: c.querySelector('.hb[data-help="aiLines"]').closest(".mrow").innerText, want: commas(add + del), first: (document.querySelector("#review .iwf") || {}).textContent}; }); // いちばん変えたファイルは「その他のグラフ」の自分のカード
      check("AI の編集のカードに、変えたファイル・行といちばん変えたファイルが出る", ed.none ? ed.n === 0 : ed.files > 0 && ed.lines.includes(ed.want), JSON.stringify(ed));
      if (!ed.none){ check("いちばん変えたファイルの名前は、セッションではなくファイルへのリンク（serve では、コミットのリンクがなければ文字だけ）", await p.evaluate(() => [...document.querySelectorAll("#review .iwf")].every(e => !e.matches("button") && !e.hasAttribute("data-sid"))));
        await p.locator("#review .iwses").first().click(); await pause();
        check("いちばん変えたファイルの編集の回数を押すと、そのファイルを変えたセッションが開く", await drawerOpen() && await p.evaluate(f => { const s = DATA.find(x => x.id === st.sel); return !!s && s.files.some(x => x.endsWith(f.replace("↗", "").trim().split(" ").pop())); }, ed.first)); // 名前の後ろのリンクの印（↗）は除く
        await p.keyboard.press("Escape"); await pause(); }
    }
    await view("cal");
  });

  await step("単価のカード・単価の組・エージェントで絞ったとき・サンキーの名前・月の目盛り", async () => {
    await view("sum");
    for (const id of ["unit", "uin"]){ // 単価のパネル（産出単価・投入単価）：数字のない組（Kiro のクレジット ÷ コミットなど）は選べない。空のパネルや「記録がない」の 1 行を出さない
      const u = await p.evaluate(id => { const g = document.querySelector(`#review .insh .ik-${id}`); if (!g) return null; const pairs = g.dataset.pairs.split(" ");
        return {pairs, none: g.querySelectorAll(".iknone").length, items: [...g.querySelectorAll(".ikrow > [data-ch]")].map(x => x.dataset.ch), ns: [...g.querySelectorAll('.iksw[data-dim="n"] [data-v]')].map(b => b.dataset.v)}; }, id);
      check(`${id} の単価のパネルは、数字のある組だけ（空の選択肢がない）`, !!u && u.none === 0 && u.items.length === u.pairs.length, JSON.stringify(u));
      if (u) for (const n of u.ns){ await p.locator(`#review .ik-${id} .iksw[data-dim="n"] [data-v="${n}"]`).click(); await pause();
        const r = await p.evaluate(id => { const g = document.querySelector(`#review .ik-${id}`), on = [...g.querySelectorAll('.iksw[data-dim="d"] [data-v]:not([hidden])')].map(b => b.dataset.v), off = [...g.querySelectorAll('.iksw[data-dim="d"] [data-v][hidden]')].map(b => b.dataset.v), v = [...g.querySelectorAll(".ikrow > [data-ch]")].filter(x => x.checkVisibility());
          return {n: g.dataset.n, on, off, ch: v.map(x => x.dataset.ch), pairs: g.dataset.pairs.split(" "), note: [...g.querySelectorAll(".ikrow > .itile:not([hidden]) .iplain")].map(x => x.textContent).join(" ")}; }, id);
        check(`${id} の単価を ${n} にすると、押せる割る数はその量で数字のあるものだけで、見える数字は 1 つ`, r.on.every(d => r.pairs.includes(`${n}:${d}`)) && r.ch.length === 1 && r.ch[0].startsWith(n + ":"), JSON.stringify(r));
        check(`${id} の単価を ${n} にしたとき、その量で数字のない割る数は出さない`, r.off.every(d => !r.pairs.includes(`${n}:${d}`)) && r.on.length > 0, JSON.stringify(r));
        if (id === "unit" && n === "credits") check("クレジットでもコミットで割れる（期間の合計 ÷ Git のコミット。名前でそう言う）", r.on.includes("commit") && r.pairs.includes("credits:commit"), JSON.stringify(r));
        if (r.ch[0] === `${n}:pr`) check(`${n} ÷ PR の注は「費用」と決めつけない`, !/^Cost can/.test(r.note), r.note); }
      if (u) await p.locator(`#review .ik-${id} .iksw[data-dim="n"] [data-v="${u.ns[0]}"]`).click(); await pause();
    }
    { const gn = await p.evaluate(() => { const x = document.querySelector('#review .ik-unit [data-ch="credits:commit"] .il'); return x ? x.textContent : null; });
      if (gn != null) check("クレジット ÷ コミットは、Git のコミットで割ると名前で言う", /per Git commit|Git コミットあたり/.test(gn), gn); }
    { // サンキーの名前と量は、色の帯に重ねない。狭い画面（600px 以下）では、図の文字の代わりに下の一覧
      const sk = await p.evaluate(() => { const f = document.querySelector("#review .isk"); if (!f) return null;
        const ts = [...f.querySelectorAll("text.skt")].filter(x => x.checkVisibility()).map(x => x.getBoundingClientRect()), bs = [...f.querySelectorAll("path.skl")].map(x => x.getBoundingClientRect());
        const hit = (a, b) => a.left < b.right - 1 && b.left < a.right - 1 && a.top < b.bottom - 1 && b.top < a.bottom - 1;
        const li = [...f.querySelectorAll(".sklist li")].filter(x => x.checkVisibility()).length, nodes = f.querySelectorAll("rect.skn:not(.o)").length;
        return {narrow: innerWidth <= 600, texts: ts.length, over: ts.filter(t => bs.some(b => hit(t, b))).length, overT: ts.filter((t, i) => ts.some((u, j) => j !== i && hit(t, u))).length, li, nodes}; });
      if (sk) check(sk.narrow ? "狭い画面のサンキーは、図の文字を消し、名前と量を下の一覧で見せる" : "サンキーの名前と量は、帯にも互いにも重ならない", sk.narrow ? sk.texts === 0 && sk.li === sk.nodes : sk.texts === sk.nodes && sk.over === 0 && sk.overT === 0 && sk.li === 0, JSON.stringify(sk));
    }
    check("内訳のカードは、隣のカードの高さに伸ばさない（中身の高さのまま）", await p.evaluate(() => [...document.querySelectorAll("#review .insrow.two > .inscol")].every(c => { const last = [...c.children].filter(x => x.checkVisibility()).pop(); if (!last) return true;
      return c.getBoundingClientRect().bottom - last.getBoundingClientRect().bottom <= parseFloat(getComputedStyle(c).paddingBottom) + 24; })));
    { // 月で見ると、点が少ないパネルのグラフでも、どの点にも月の名前がある
      await p.keyboard.press("m"); await pause();
      const lab = await p.evaluate(() => [...document.querySelectorAll("#review .insh .ikrow > .itile:not([hidden]) .tspk svg")].map(s => { const xs = [...s.querySelectorAll("text.sxl")]; return {n: xs.length, shown: xs.filter(x => x.checkVisibility() && !x.classList.contains("nodat")).length, nodat: xs.filter(x => x.classList.contains("nodat")).length}; }));
      check("点が 6 つ以下のパネルのグラフは、どの点にも期間の名前を出す", lab.every(x => x.n > 6 || x.shown + x.nodat === x.n), JSON.stringify(lab));
      await p.keyboard.press("w"); await pause();
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
    { // 近い時刻に 5 つ以上かたまった印は、2 つと「+N」にたたみ、押すとその日の印を重ならないように並べ直す
      const g = await p.evaluate(() => { const ws = st.week.getTime() / 1000, base = (META.git || []).find(c => c.t >= ws && c.t < ws + 7 * 86400); if (!base) return null;
        const keep = META.git.slice(); for (let i = 1; i <= 6; i++) META.git.push({...base, hash: base.hash.slice(0, -3) + "zz" + i, t: base.t + i * 30}); META.git.sort((a, b) => a.t - b.t); render();
        const day = () => [...document.querySelectorAll("#tl .day")].find(d => d.querySelector(".gmore")), vis = d => [...d.querySelectorAll(".gc, .gm")].filter(x => x.checkVisibility());
        const d0 = day(), m = d0 && d0.querySelector(".gmore"), shut = d0 ? vis(d0).length : 0, txt = m ? m.textContent : ""; if (m) m.click();
        const d1 = day(), ys = d1 ? [...d1.querySelectorAll(".gc, .gm, .gmore")].map(x => x.getBoundingClientRect().top).sort((a, b) => a - b) : [], open = d1 ? vis(d1).length : 0;
        const r = {txt, shut, open, exp: d1 ? d1.querySelector(".gmore").getAttribute("aria-expanded") : "", gap: Math.min(99, ...ys.slice(1).map((y, i) => y - ys[i]))};
        META.git = keep; st.gopen.clear(); render(); return r; });
      if (g) check("かたまった印は「+N」にたたみ、開くと全部を重ならずに並べる", /^\+\d+$/.test(g.txt) && g.open === g.shut + +g.txt.slice(1) && g.exp === "true" && g.gap >= 16, JSON.stringify(g));
    }
  });

  await step("見直す候補", async () => {
    await view("sum");
    check("日の見出しの 1 行は、名前つきのただの文字（HTML を含まない）", await p.evaluate(() => [...document.querySelectorAll("#tl .head .cline > span[aria-label]")].every(e => !/[<>]/.test(e.getAttribute("aria-label")) && e.querySelectorAll("*:not(svg):not(svg *):not(small)").length === 0)));
    check("上限の印は、そのセッションの帯の左端にある（隣の帯に重ねない）", await p.evaluate(() => [...document.querySelectorAll("#tl .lim")].every(m => { const r = m.getBoundingClientRect(), bs = [...m.closest(".day").querySelectorAll(`.run[data-sid="${CSS.escape(m.dataset.id)}"]`)].map(b => b.getBoundingClientRect());
      return !bs.length || bs.some(b => Math.abs(b.left - r.left) <= 4); })));
    check("夜遅い印も日の枠の中にある", await p.evaluate(() => [...document.querySelectorAll("#tl .day")].every(d => { const db = d.getBoundingClientRect(); return [...d.querySelectorAll(".gc, .gm")].every(m => m.getBoundingClientRect().bottom <= db.bottom + 1); })));
    check("今の時刻の札に近い時刻も消さない（畳んだときは 07:00〜23:00 がそろう）", await p.evaluate(() => !document.querySelector("#tl .hours span.now") || [...document.querySelectorAll("#tl .hours span:not(.now)")].map(e => e.textContent).join(",") === Array.from({length: 17}, (_, i) => String(i + 7).padStart(2, "0") + ":00").join(",")));
    check("平日の勤務時間（9:00〜18:00）の外は地の色が違う", await p.evaluate(() => { const d = [...document.querySelectorAll("#tl .day")].find(e => !e.classList.contains("we")); return !!d && (getComputedStyle(d).backgroundImage.match(/linear-gradient/g) || []).length === 3; })); // 畳んだ帯・1 時間の線・勤務時間の外
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
    } else check("サマリーに見直す候補の印（.fmark）がある（デモのデータで、下の確かめのため）", false);
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

  // 日報のプロンプトは画面の幅によらないので、最初の幅でだけ確かめる
  if (env === envs[0]) await step("報告を書いてもらうプロンプト", async () => {
    // 日報はプロンプトをコピーするだけ（kiroku は AI を動かさない。デモでも同じ）
    check("日報はプロンプトのコピーで、AI を動かすボタンはない", await p.locator(".head .dayrun.ai-copy").count() > 0 && await p.locator(".ai-action, dialog#ai-review-dialog").count() === 0);
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
    check("検索の最中は結果だけ（期間の行・凡例・カレンダー・サマリーは隠す）", await p.evaluate(() => document.querySelector("#sres").checkVisibility() && ["#tl", "#review", "#legend", ".week"].every(q => !document.querySelector(q).checkVisibility())));
    check("一致したセッションがある", await p.locator(".srow[data-s]").count() > 0, word);
    check("検索結果は枠の中でスクロールする（どの一覧も長くなりすぎない）", await p.evaluate(() => [...document.querySelectorAll("#sres .panel")].every(e => e.getBoundingClientRect().height <= Math.min(innerHeight * .52, 520) + 1)));
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
    // 以前の版が保存した表示（journey・project）が残っていても、カレンダーで開く（幅によらないので、最初の幅で 1 つだけ）
    if (env === envs[0]) for (const v of ["journey"]) {
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

  await step("どこに使ったか・増減・サンキーの切り替え・月の濃淡・クレジットの単位", async () => {
    await p.goto(url); await loaded(p);
    check("クレジットは cr ではなく言葉の単位で書く", !/\d\s*cr\b/.test(await p.locator("#kpis").innerText()));
    await view("sum");
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
    // ドーナツ・サンキー
    check("どこに使ったかに、割合のドーナツがある（項目の数だけ弧）", await p.evaluate(() => { const d = document.querySelector("#review .iwdn svg.idn"); return !!d && d.querySelectorAll("path").length === document.querySelectorAll("#review .iwdn .iwr").length; }));
    for (const [attr, key, sel, name] of [["skk", "insSk", ".isk", "投入から産出の流れ"]]){
      const ks2 = await p.locator(`#review [data-${attr}]`).evaluateAll(xs => xs.map(x => x.dataset[Object.keys(x.dataset)[0]]));
      for (const k of ks2){ await p.locator(`#review [data-${attr}="${k}"]`).click(); await pause();
        check(`${name}を ${k} にすると、その量の図と 1 文になる`, await p.evaluate(([key, k, sel]) => st[key] === k && !!document.querySelector(`#review ${sel} svg`) && !!document.querySelector(`#review ${sel}`).closest(".inscol").querySelector(".isay"), [key, k, sel])); } }
    check("投入から産出の流れ（サンキー）は、4 枚のパネルのすぐ下（内訳の上）の段の左に 1 枚（段は幅いっぱい。右に費用と産出の推移がなければ、サンキーが幅いっぱい）で、内訳の段には繰り返さない", await p.evaluate(() => { const h = document.querySelector("#review .insh"), f = document.querySelector("#review .isflow"), k = document.querySelector("#review .isk") && document.querySelector("#review .isk").closest(".iskw"); if (!f || !k) return false;
      const hr = h.getBoundingClientRect(), fr = f.getBoundingClientRect(), kr = k.getBoundingClientRect(), between = [...document.querySelectorAll("#review .insg, #review .insrow")].every(x => x.getBoundingClientRect().top >= fr.bottom - 1);
      const side = f.classList.contains("two") && innerWidth >= 1300;
      return fr.top >= hr.bottom - 1 && Math.abs(fr.left - hr.left) < 2 && Math.abs(fr.right - hr.right) < 2 && Math.abs(kr.left - hr.left) < 2 && (side || Math.abs(kr.right - hr.right) < 2) && between && document.querySelectorAll("#review .isk").length === 1 && !k.closest(".insrow"); }));
    { // 投入から産出の流れ：エージェント → プロジェクト → AI がしたこと の 3 列。右の列は、そのセッションで AI が実行して成功したこと（いちばん先のもの）だけで、
      // その順（PR 作成・push・コミット・ファイル変更・GitHub の操作・ツールのみ・チャットのみ・記録なし）に並ぶ。1 文は、そのどれも全体に対する割合で言い（足して 100）、一覧の割合は列ごとに足して 100
      const sk = await p.evaluate(() => { const c = document.querySelector("#review .isk"); if (!c) return null; const say = c.closest(".inscol, .insg, .ikg, section, div").querySelector(".isay");
        const out = [...c.querySelectorAll(".sklist .skcol:last-child .skn2")].map(x => x.textContent);
        return {out, txt: say ? say.textContent : "", list: [...c.querySelectorAll(".sklist .skcol")].map(g => [...g.querySelectorAll("li b")].reduce((a, b) => a + parseInt(b.textContent), 0))}; });
      const ORD = [["PR opened", "PR 作成"], ["Pushed", "プッシュ"], ["Committed", "コミット"], ["Changed files", "ファイル変更"], ["GitHub comments and actions", "GitHub のコメント・操作"], ["Tools only", "ツールのみ"], ["Chat only", "チャットのみ"], ["Not recorded", "記録なし"]];
      const idx = sk ? sk.out.map(x => ORD.findIndex(o => o.includes(x))) : [];
      const pcs = sk ? [...sk.txt.matchAll(/(\d+)%/g)].map(m => +m[1]) : [];
      const onlyUnk = sk && idx.every(i => i === ORD.length - 1);
      check("サンキーの右の列は「AI がしたこと」だけで、決まった順に並び、1 文はその全部を割合（足して 100）で言う", !sk || (idx.every(i => i >= 0) && idx.every((v, i) => i === 0 || idx[i - 1] < v)
        && (onlyUnk ? pcs.length === 0 : pcs.length === idx.length && pcs.reduce((a, v) => a + v, 0) === 100)), JSON.stringify({sk, pcs}));
      check("サンキーの一覧の割合は、列ごとに足して 100", !sk || sk.list.every(v => v === 100), JSON.stringify(sk && sk.list));
    }
    { // サンキーの名前は、どれもカードの地の色の札の上（色の帯に直接乗らない）
      const lb = await p.evaluate(() => { const ts = [...document.querySelectorAll("#review .isk text.skt")].filter(x => x.checkVisibility());
        const bad = ts.filter(x => { const r = x.previousElementSibling; if (!r || !r.matches("rect.skbg")) return true; const a = x.getBoundingClientRect(), b = r.getBoundingClientRect(), f = getComputedStyle(r).fill;
          return !(b.left <= a.left + 1 && b.right >= a.right - 1 && b.top <= a.top + 1 && b.bottom >= a.bottom - 1) || f === "none" || /rgba\(.*, 0\)/.test(f) || +getComputedStyle(r).fillOpacity < 1; }).map(x => x.textContent);
        return {n: ts.length, bad, narrow: innerWidth <= 600}; });
      check("サンキーの名前と量は、どれも不透明な札の上にある（狭い画面では図の名前を出さず、下の一覧）", lb.narrow ? lb.n === 0 : lb.n > 0 && lb.bad.length === 0, JSON.stringify(lb));
    }
    { // サンキーの色：エージェントから出る帯はエージェントの色、プロジェクト（data-p）は左右 2 本の節の間を、そのプロジェクトの色の帯が埋め、プロジェクトから出る帯もプロジェクトの色
      const sc = await p.evaluate(() => { const f = document.querySelector("#review .isk svg"); if (!f || !f.checkVisibility()) return null; /* 狭い画面では図を出さない（一覧だけ） */ const fill = e => e.getAttribute("fill"), r = e => e.getBoundingClientRect(), near = (x, y) => Math.abs(x - y) < 1.5;
        const sk = [...f.querySelectorAll("rect.skp[data-p]")], L = [...f.querySelectorAll("rect.skn:not(.o)")], O = [...f.querySelectorAll("rect.skn.o[data-p]")], cvar = /^var\(--(c[0-7]|other)\)$/;
        const band = sk.filter(e => { const b = r(e), l = L.find(x => near(r(x).right, b.left) && near(r(x).top, b.top)), o = O.find(x => near(r(x).left, b.right) && near(r(x).top, b.top));
          return !!l && !!o && near(r(l).height, b.height) && near(r(o).height, b.height) && b.width > 40 && fill(e) === fill(l) && fill(e) === fill(o) && cvar.test(fill(e)) && +getComputedStyle(e).opacity >= +getComputedStyle(f.querySelector("path.skl")).opacity; }).length;
        const ps = [...f.querySelectorAll("path.skl")], ag = ps.filter(e => e.dataset.l === "0"), po = ps.filter(e => e.dataset.l === "1"), pc = new Map(sk.map(e => [e.dataset.p, fill(e)]));
        return {projects: O.length, skp: sk.length, band, agent: ag.length, agentOk: ag.filter(e => fill(e) === agColor(e.dataset.a)).length, proj: po.length, projOk: po.filter(e => pc.has(e.dataset.a) && fill(e) === pc.get(e.dataset.a)).length}; });
      if (sc){
        check("サンキーのプロジェクトは、どれも左右の節の間がそのプロジェクトの色の帯で埋まっている", sc.projects >= 2 && sc.skp === sc.projects && sc.band === sc.projects, JSON.stringify(sc));
        check("サンキーのエージェントから出る帯は、エージェントの色", sc.agent >= 2 && sc.agentOk === sc.agent, JSON.stringify(sc));
        check("サンキーのプロジェクトから出る帯は、プロジェクトの色", sc.proj >= 2 && sc.projOk === sc.proj, JSON.stringify(sc));
      }
    }
    check("サンキーは、エージェント・プロジェクト・AI がしたこと の 3 列", await p.evaluate(() => document.querySelectorAll("#review .isk .skh").length === 3 && document.querySelectorAll("#review .isk .skl").length >= 2));
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
    // 途中の週の時間外の割合は、過去の週の同じ曜日・時刻までと比べる（土日を含む週まるごととは比べない）
    check("途中の週の時間外の割合は、過去の週の同じところまでと比べる", await p.evaluate(() => { if (soFar() == null) return true;
      const {ws} = period(), cut = nowMs() / 1000 - ws, b = insOffRange(50), hs = historyStart(), want = [];
      for (let i = 1; i <= INS_N; i++){ const q = periodBack(i); if (q.ws < hs || !q.S) continue; const x = insOff(insMinutes(q.ws, Math.min(q.we, q.ws + cut))); if (x != null) want.push(x); }
      const whole = []; for (let i = 1; i <= INS_N; i++){ const q = periodBack(i); if (q.ws < hs || !q.S) continue; const x = insOff(insMinutes(q.ws, q.we)); if (x != null) whole.push(x); }
      return JSON.stringify(b.xs) === JSON.stringify(want) && JSON.stringify(want) !== JSON.stringify(whole); }));
    // 途中の週の推移の読み上げは、推定を値として読まず、これまでの値と推定だと言う
    check("途中の週の推移の読み上げは「so far」と推定を言う", await p.evaluate(() => { const x = document.querySelector('#review .itile[data-ch="cost"] .tspk .sr'); return !x || /so far, estimated .+ by the end/.test(x.textContent); }));
    const cb = p.locator('#review .iksw[data-sw="cost"] [data-v="credits"]');
    if (await cb.count()){ // 今週にクレジットの記録がなければ、投入のパネルにクレジットの選択肢は出ない（数字のない選択肢は出さない）
      await cb.click(); await pause(); // コストのパネルでクレジットを選ぶ
      const tile = await p.evaluate(() => { const b = document.querySelector('#review .itile [data-metric="credits"]'); return b ? b.closest(".itile").innerText : ""; });
      await p.locator('#review .iksw[data-sw="cost"] [data-v]').first().click(); await pause();
      check("サマリーのクレジットのカードにも、今月末の推定がある（式を出しているときは式で）", /Month-end estimate \(this month\)|Month-end credits = /.test(tile) && tile.includes(mp.v), tile);
    } else check("今週にクレジットの記録がなければ、投入のパネルにクレジットの選択肢を出さない", await p.evaluate(() => !((WEEKS[key(st.week)] || {}).usage || {}).credits));
    await view("cal");
  });

  // カレンダーの色と数の単位（週・月）。ブロックの塗りは色分けの色を混ぜたもので、色分けを変えると変わる。土日は中立の同じ地、月の濃さはエージェントの色ではない。
  // 週の日の見出し・月のマス・上の帯の数には、どれも英語の単位（5 commits・44 credits・$2.22・3h）があり、はみ出さない。今日の印は控えめ
  await step("カレンダーの色と単位", async () => {
    await open();
    const rgb = s => (s.match(/[\d.]+/g) || []).slice(0, 3).map(Number);
    const fills = () => p.evaluate(() => [...document.querySelectorAll("#tl .run[data-sid]")].map(e => getComputedStyle(e).backgroundColor));
    const sur = await p.evaluate(() => { const d = document.createElement("div"); d.style.background = "var(--surface)"; document.body.append(d); const c = getComputedStyle(d).backgroundColor; d.remove(); return c; });
    const setBy = v => p.evaluate(v => { const s = document.querySelector("#cb2"); s.value = v; s.dispatchEvent(new Event("change")); }, v);
    await setBy("source"); await pause();
    const fa = await fills();
    const tinted = f => { const [r, g, b] = rgb(f), [R, G, B] = rgb(sur); return Math.abs(r - R) + Math.abs(g - G) + Math.abs(b - B) >= 18; };
    check("週のブロックの塗りは、色分けの色を地に混ぜたもの（地の色のままではない）", fa.length > 0 && fa.every(tinted), fa.filter(f => !tinted(f)).slice(0, 3).join(" / "));
    await setBy("project"); await pause();
    const fp = await fills();
    check("Color by を変えると、ブロックの塗りも変わる", fp.length === fa.length && fp.some((f, i) => f !== fa[i]), `${fa.slice(0, 3)} → ${fp.slice(0, 3)}`);
    await setBy("source"); await pause();
    const reddish = c => { const [r, g, b] = rgb(c); return r - Math.max(g, b) > 12; };
    const we = await p.evaluate(() => [...document.querySelectorAll("#tl .head.we, #tl .day.we")].map(e => [e.className, getComputedStyle(e).backgroundColor, getComputedStyle(e.querySelector("b") || e).color]));
    check("土日は中立の同じ地（赤い地や字はない。土曜と日曜で同じ）", we.length >= 4 && we.every(([, bg, c]) => !reddish(bg) && !reddish(c)) && new Set(we.filter(([k]) => /head/.test(k)).map(([, bg, c]) => bg + c)).size === 1, JSON.stringify(we.slice(0, 4)));
    const unitOK = s => !/\d/.test(s.replace(/\$[\d.,]+[KMB]?|\b\d+[hm]\b|[\d.,]+[KMB]? (commits?|credits?|tokens)\b/g, ""));
    const audit = () => p.evaluate(() => { const bad = [], txt = [];
      for (const box of document.querySelectorAll("#tl .head, .mgrid .cell:not(.out), #kpis .kpi")) { const r = box.getBoundingClientRect(); if (!r.width) continue;
        for (const e of box.querySelectorAll(".cline > span, .acs, .acm, .act, .v")) { if (!e.getClientRects().length) continue;
          const q = e.getBoundingClientRect(); txt.push(e.innerText.replace(/\s+/g, " ").trim());
          if (q.left < r.left - 1 || q.right > r.right + 1 || e.scrollWidth > e.clientWidth + 1) bad.push(txt.at(-1)); } }
      const lines = [...document.querySelectorAll("#tl .head .cline")].flatMap(l => l.innerText.split("\n").map(x => x.trim()).filter(Boolean));
      return {bad, txt, lines}; });
    const wide = [1440, 1000, 390].includes(env.viewport.width);
    for (const m of ["week", "month"]) for (const k of ["cost", "credits", "tokens", "active"]) {
      await p.evaluate(([m, k]) => { st.shade = k; setMode(m); render(); }, [m, k]); await p.waitForTimeout(100);
      const a = await audit(), noUnit = a.txt.filter(s => !unitOK(s));
      check(`${m} · ${k}: 日の見出し・月のマス・上の帯の数には、どれも単位がある`, a.txt.length > 0 && noUnit.length === 0, noUnit.slice(0, 4).join(" / "));
      if (wide) check(`${m} · ${k}: 日の見出し・月のマス・上の帯の数は、枠からはみ出さない`, a.bad.length === 0, a.bad.slice(0, 4).join(" / "));
      if (m === "week") check(`${m} · ${k}: 日の見出しの行は「·」で始まったり終わったりしない`, a.lines.every(l => !/^·|·$/.test(l)), a.lines.filter(l => /^·|·$/.test(l)).slice(0, 3).join(" / "));
    }
    await p.evaluate(() => { st.shade = "cost"; setMode("month"); render(); }); await pause();
    check("月の濃さは凡例と同じ段（0・¼・½・¾・1）だけ", await p.evaluate(() => [...document.querySelectorAll(".mgrid .cell[style*='--heat']")].every(c => ["0", "0.25", "0.5", "0.75", "1"].includes(c.style.getPropertyValue("--heat").trim()))));
    const heat = await p.evaluate(() => [...document.querySelectorAll(".mgrid .cell:not(.wkc):not(.out)")].filter(c => +c.style.getPropertyValue("--heat") > .5).map(c => getComputedStyle(c).backgroundColor));
    const agents = await p.evaluate(() => AGENTS.map(a => { const d = document.createElement("div"); d.style.background = `var(--a${a.slot})`; document.body.append(d); const c = getComputedStyle(d).backgroundColor; d.remove(); return c; }));
    const sat = c => { const [r, g, b] = rgb(c); return Math.max(r, g, b) - Math.min(r, g, b); };
    check("月の濃さはエージェントの色相ではなく中立の灰青（鮮やかさが低い）", heat.length > 0 && heat.every(c => sat(c) <= 24 && !agents.includes(c)), heat.slice(0, 3).join(" / "));
    const td = await p.evaluate(() => { const c = document.querySelector(".mgrid .cell.today"); if (!c) return null; const s = getComputedStyle(c), d = getComputedStyle(c.querySelector(".dn"));
      return {shadow: s.boxShadow, outline: s.outlineStyle, chip: d.backgroundColor}; });
    check("月の今日のマスは枠で囲まず、日付も塗らない", !td || (td.shadow === "none" && td.outline === "none" && /rgba\(0, 0, 0, 0\)|transparent/.test(td.chip)), JSON.stringify(td));
    await p.evaluate(() => { setMode("week"); render(); }); await pause();
  });

  await step("記録のない週・月も空のカレンダー", async () => {
    const w0 = await p.evaluate(() => key(st.week));
    for (const mode of ["week", "month"]){
      await p.evaluate(m => { setMode(m); const far = addDays(mondayOf(new Date(Math.min(...DATA.map(s => s.start)) * 1000)), -7 * 20); if (m === "week") st.week = far; else st.month = monthOf(far); render(); }, mode); await pause();
      const r = await p.evaluate(() => ({note: (document.querySelector("#tl .empty") || {}).textContent || "", grid: document.querySelectorAll("#tl .day, #tl .mgrid .cell").length, go: !!document.querySelector("#tolast"), runs: document.querySelectorAll("#tl .run").length}));
      check(`記録のない${mode === "week" ? "週" : "月"}も、枠（曜日・時刻・マス）を描き、上に「ない」の 1 行と記録のある最後の期間へのボタン`, r.grid >= 7 && r.runs === 0 && /No records/.test(r.note) && r.go, JSON.stringify(r));
      await p.click("#tolast"); await pause();
      check(`ボタンで記録のある${mode === "week" ? "週" : "月"}へ行き、案内は消える`, await p.evaluate(m => !document.querySelector("#tl .empty") && (m === "month" || document.querySelectorAll("#tl .run").length > 0), mode));
    }
    await p.evaluate(w => { setMode("week"); const [y, m, d] = w.split("-").map(Number); st.week = new Date(y, m - 1, d); render(); }, w0); await pause();
  });

  await step("名前の最後の語と「ƒx」「?」", async () => {
    await view("sum");
    // 名前が折り返すとき、「?」や「ƒx」だけが次の行に落ちない（最後の語といっしょに。1440 の「…reached a commit or PR」で「?」だけが落ちた）
    const bad = await p.evaluate(() => { const out = []; document.querySelectorAll("#review .mrow .mk, #review .stat .k").forEach(k => { const lab = k.querySelector(".kt > span[id]");
      const bs = [...k.querySelectorAll("button")].filter(b => b.checkVisibility()); if (!lab || !bs.length || !lab.checkVisibility()) return;
      const tn = [...lab.childNodes].filter(n => n.nodeType === 3).pop(); if (!tn || !tn.length) return; const r = document.createRange(); r.setStart(tn, tn.length - 1); r.setEnd(tn, tn.length);
      const lr = r.getBoundingClientRect(), mid = x => (x.top + x.bottom) / 2; if (lr.height && bs.some(b => Math.abs(mid(b.getBoundingClientRect()) - mid(lr)) > 8)) out.push(k.textContent); }); return out; });
    check("名前の最後の語と「ƒx」「?」が同じ行にある", bad.length === 0, bad.slice(0, 3).join(" / "));
    check("費用と産出の推移の、線の端の 2 つの合計が重ならない", await p.evaluate(() => { const [a, b] = [...document.querySelectorAll("#review .icum .icnl")].map(e => e.getBoundingClientRect());
      return !a || !b || !a.height || a.bottom <= b.top + 0.5; }));
    check("値のボタンの名前は、分けた名前の 2 つを合わせたもの", await p.evaluate(() => { const v = document.querySelector('#review .mrow .mv[aria-labelledby]'); if (!v) return true;
      const ids = v.getAttribute("aria-labelledby").split(" "); return ids.length === 3 && ids.every(i => document.getElementById(i)) && /\S\s\S|\S$/.test(document.getElementById(ids[0]).textContent + document.getElementById(ids[1]).textContent); }));
    await view("cal");
  });
  if (env === envs[0]) await step("前の週並みに収める 1 日の目安", async () => {
    await p.evaluate(() => go(null)); await pause();
    const b = p.locator('#kpis [data-metric="pace"]');
    if (!(await b.count())) { console.log("  skip 今週に 1 日の目安がない"); return; }
    await b.click(); await p.waitForFunction(() => document.querySelector("#md").open);
    const r = await p.evaluate(() => { const md = document.querySelector("#md"), fut = !!md.querySelector(".mtrend line.fut"), key = [...md.querySelectorAll(".mbkey span")].map(x => x.className);
      const cr = [...md.querySelectorAll("h3")].find(h => h.textContent === "Kiro credits"), calc = cr && cr.nextElementSibling, last = calc && calc.matches(".mcalc") ? calc.lastElementChild.querySelector("b").textContent : null;
      return {fut, key, last, out: /Output \(commits\): /.test(md.innerText) && !/Left behind/.test(md.innerText)}; });
    // 破線は 2 本（1 日の目安と、日曜まで延ばしたトレンド）。凡例も 2 つに分ける。クレジットの式の答えには単位を
    check("延ばしたトレンド線があれば、凡例に「Trend, projected」を 1 日の目安とは別に出す", !r.fut || (r.key.includes("pace") && r.key.includes("trf")), JSON.stringify(r.key));
    check("Kiro credits の 1 日の目安の式の答えに単位がある", r.last == null || /credits?$/.test(r.last), String(r.last));
    check("残したコミットの行は「Output (commits):」（日本語の 産出（コミット） と同じ）", r.out);
    await p.keyboard.press("Escape"); await pause();
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
    if (env === envs[0]){ // 幅によらない（TestLoadingLangMatchesView も見る）ので、最初の幅でだけ
      await p.evaluate(() => localStorage.setItem("kiroku:lang", JSON.stringify("en"))); await p.reload(); await loaded(p);
      check("以前選んだ English が残っていても、日本語で開く", await p.evaluate(() => document.documentElement.lang === "ja" && JA));
      await p.evaluate(() => localStorage.removeItem("kiroku:lang")); }
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

  await step("凡例の数", async () => { // 先週のまま（日本語は見出しの「稼働時間 · 押すと非表示」が長く、チップがはみ出しやすい）
    for (const by of ["project", "source"]) await legendHint(p, env, check, by);
  });

  await step("内訳", async () => {
    await p.evaluate(() => document.querySelector('#review [data-metric="cost"]').click());
    await isOpen("#md");
    check("内訳の見出しが日本語", (await p.locator("#mdh").innerText()) !== "Estimated cost", await p.locator("#mdh").innerText());
    await noEnglish("内訳のダイアログ");
    await p.keyboard.press("Escape"); await isClosed("#md");
  });

  await step("量の名前は 1 つ", async () => {
    await p.locator('#view button[data-v="sum"]').click(); await p.waitForTimeout(300);
    const txt = await p.evaluate(() => ["#kpis", "#legend", "#tl", "#review"].map(q => (document.querySelector(q) || {}).innerText || "").join("\n"));
    check("日本語の画面では、クレジットは Kiro と同じ「credits」で書く（カタカナと混ぜない）", !/クレジット/.test(txt), (txt.match(/.{0,20}クレジット.{0,10}/) || [""])[0]);
    check("日本語のサマリーでは、費用をコストと呼ばない", !/コスト/.test(await p.locator("#review .insight").innerText()));
    { const panelNames = () => p.evaluate(() => ({panels: [...document.querySelectorAll("#review .insh .ikg .ikgh")].map(h => [...h.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join("").trim()),
      out: !!document.querySelector("#review .iout") && [...document.querySelector("#review .iout").querySelectorAll(":scope > h4")].map(h => [...h.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join("").trim()).join("")}));
      const n = await panelNames();
      check("日本語のパネルの名前は 投入・産出・投入単価・産出単価、内訳のカードは「産出」（「出力」は出力トークンと取り違える）", JSON.stringify(n.panels) === JSON.stringify(["投入", "産出", "投入単価", "産出単価"]) && n.out === "産出", JSON.stringify(n)); }
    await p.locator('#view button[data-v="cal"]').click(); await p.waitForTimeout(300);
  });

  await step("前の期間からの増減の軸の名前", async () => {
    // 日本語の名前は英語のほぼ 2 倍の幅。文字の数で切ると隣の名前と重なった（「先週の同じ日数まで」と「data-pipe…」）。途中の今週で、同じ日数までと比べる形を見る
    await p.evaluate(() => go(null)); await p.locator('#view button[data-v="sum"]').click(); await p.waitForTimeout(300);
    const r = await p.evaluate(() => { const xs = [...document.querySelectorAll("#review .iwtf text.ixl")].map(e => { const b = e.getBoundingClientRect(); return {t: [...e.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join(""), l: b.left, r: b.right}; });
      return {xs, bad: xs.slice(1).filter((x, i) => x.l < xs[i].r - 0.5).map(x => x.t)}; });
    check("前の期間からの増減の軸の名前が、隣の名前と重ならない", r.xs.length >= 3 && r.bad.length === 0, JSON.stringify(r.bad.length ? r.bad : r.xs.map(x => x.t)));
    await p.locator('#view button[data-v="cal"]').click(); await p.waitForTimeout(300);
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
