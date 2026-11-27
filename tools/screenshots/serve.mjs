// kiroku serve の鍵を、本物のブラウザで確かめる（CI の e2e）。
//   go build -o /tmp/kiroku . && python3 -I tools/screenshots/gen.py /tmp/gen && node tools/screenshots/serve.mjs /tmp/kiroku /tmp/gen
// ダミーの履歴（gen.py）だけを読む kiroku serve を、使い捨ての HOME と設定の場所で空いているポートに立て、
//   - 動いている kiroku serve をもう一度打つと、使い捨てのコード（?code=）つきの中継のページを開く。そのコードで開くと履歴が出て、どの URL にも鍵が載らない（ブラウザの履歴に鍵が残らない）。
//     鍵は中継のページの POST の答えでだけ受け取り、コードは 2 回目には使えない
//   - 以前の版の /?key= は、鍵が合っていても 403 で、履歴を出さず、鍵をしまわない（1.0 から）
//   - 鍵なしの / を開き直しても出る（localStorage の鍵）。履歴ファイルの「Open」は鍵を付けて読んで保存する
//   - 鍵のないブラウザ（新しいコンテキスト）は、案内だけで履歴を見ない。data.json も 403
//   - 画面から同じコンピューターのほかのポートのサーバーへ移っても、鍵はどの問い合わせにも載らない（Cookie にも、ほかのところにも）
//   - kiroku は AI のエージェントを動かさない（4.0 で「Analyze with AI」を外した）: 日報と振り返りはプロンプトのコピーだけで、/ai/run はなく（404）、
//     --ai を付けても止まらずに外したことを 1 行で言い、AI を動かす道は出てこない
// を見る。Playwright が必要（smoke.mjs と同じ）。
import { chromium } from "playwright";
import { execFileSync, spawn } from "node:child_process";
import { createServer } from "node:http";
import { chmod, cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

const [bin, gen] = process.argv.slice(2);
if (!bin || !gen) { console.error("使い方: node serve.mjs <kiroku> <gen.py の出力先>"); process.exit(2); }
let failed = 0;
const check = (what, ok, detail = "") => { if (ok) console.log(`  ok   ${what}`); else { failed++; console.log(`  FAIL ${what}${detail ? `: ${detail}` : ""}`); } };

const work = await mkdtemp(path.join(tmpdir(), "kiroku-serve-e2e-"));
const home = path.join(gen, "home");
const env = { ...process.env, HOME: path.join(work, "home"), KIROKU_CONFIG_DIR: path.join(work, "config"), KIROKU_LANG: "", KIROKU_AI: "", TZ: "Asia/Tokyo" };
const serveArgs = ["serve", "127.0.0.1:0", "--no-open", "--sources", "claude", "--claude-root", path.join(home, ".claude", "projects"), "--archive-dir", path.join(work, "archive")];
const srv = spawn(path.resolve(bin), serveArgs, { env, stdio: ["ignore", "pipe", "pipe"] });
let log = "";
srv.stdout.on("data", d => { log += d; }); srv.stderr.on("data", d => { log += d; });
const extra = []; // あとで立てる serve（--ai を付けたもの）
const stop = async () => { srv.kill(); extra.forEach(s => s.kill()); await rm(work, { recursive: true, force: true }); };

// ほかのユーザーのものかもしれない、同じコンピューターのほかのポートのサーバー。届いた問い合わせをすべて残す
const seen = [];
const other = createServer((req, res) => { seen.push({ url: req.url, headers: req.headers }); res.setHeader("Content-Type", "text/html"); res.end("<!doctype html><title>other</title><p>other server</p>"); });
await new Promise(r => other.listen(0, "127.0.0.1", r));
const otherPort = other.address().port;

let b;
try {
  const until = async (what, f, ms = 30000) => { const end = Date.now() + ms; for (;;) { const v = await f(); if (v) return v; if (Date.now() > end) throw new Error(`${what} にならない: ${log}`); await new Promise(r => setTimeout(r, 100)); } };
  const port = await until("待ち受け", () => (log.match(/serving http:\/\/localhost:(\d+)\//) || [])[1]);
  const key = (await until("鍵", () => readFile(path.join(env.KIROKU_CONFIG_DIR, "serve-key"), "utf8").catch(() => ""))).trim();
  // 別のポートの serve は別のオリジンなので、その localStorage に鍵をしまってから開く（kiroku serve --open の中継のページと同じこと）
  const openWithKey = async (page, url) => {
    await page.goto(`${url}/`);
    await page.evaluate(k => localStorage.setItem("kiroku:key", k), key);
    await page.goto(`${url}/`);
  };
  const base = `http://localhost:${port}`;
  // 読み終わるのを待つ（以前の版の Cookie も付けるのは、修正の前の版に対してこのテストが失敗することを確かめられるように）
  await until("読み込み", async () => (await fetch(`${base}/stamp`, { headers: { "X-Kiroku-Key": key, Cookie: `kiroku_key_${port}=${key}` } }).catch(() => ({ ok: false }))).ok);
  check("鍵なしの data.json は 403", (await fetch(`${base}/data.json`)).status === 403);
  check("起動の行に「Analyze with AI」はない", !/Analyze with AI/.test(log), log);
  check("鍵の Cookie（以前の版）だけの data.json は 403", (await fetch(`${base}/data.json`, { headers: { Cookie: `kiroku_key_${port}=${key}` } })).status === 403);

  b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
  const ctx = await b.newContext({ acceptDownloads: true });
  const p = await ctx.newPage();
  const errors = [];
  p.on("pageerror", e => errors.push(e.message));
  p.on("console", m => { if (m.type() === "error" && /Content Security Policy|Refused/.test(m.text())) errors.push(m.text()); });

  // 動いている kiroku serve をもう一度打つ（5.0 で消した kiroku open の代わり）：止まらずに、使い捨てのコードの中継のページ（設定の場所の open.html）を
  // ブラウザで開く。ここではブラウザを開くコマンド（xdg-open・open）を何もしないものに差しかえ、中継のページからコードの URL を取り出す。
  // 開いた URL（ブラウザの履歴に残るもの）のどれにも鍵が載らない
  const pc = await (await b.newContext()).newPage();
  const visited = [];
  pc.on("request", r => visited.push(r.url()));
  const fakeBin = path.join(work, "fakebin"); await mkdir(fakeBin, { recursive: true });
  for (const name of ["xdg-open", "open"]) { await writeFile(path.join(fakeBin, name), "#!/bin/sh\nexit 0\n"); await chmod(path.join(fakeBin, name), 0o755); }
  const again = execFileSync(path.resolve(bin), ["serve", `127.0.0.1:${port}`], { env: { ...env, PATH: `${fakeBin}${path.delimiter}${process.env.PATH}` }, encoding: "utf8" });
  const hopFile = await readFile(path.join(env.KIROKU_CONFIG_DIR, "open.html"), "utf8");
  const once = (hopFile.match(/http:\/\/localhost:\d+\/\?code=[0-9a-f]{64}/) || [""])[0];
  check("動いている kiroku serve をもう一度打つと、止まらずにそれを開く（使い捨てのコードの URL で、鍵は出さない）", /already running at/.test(again) && !!once && !hopFile.includes(key) && !again.includes(key), `${again} / ${once}`);
  await pc.goto(once);
  await pc.waitForURL(u => !u.search.includes("code="), { timeout: 10000 });
  await pc.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("?code= で開くと履歴が出て、鍵は localStorage にある", await pc.evaluate(() => localStorage.getItem("kiroku:key")) === key);
  check("開いた URL のどれにも鍵が載らない", !visited.some(u => u.includes(key)), JSON.stringify(visited));
  // 使い終わったコードでは鍵を受け取れず、そう伝える（アドレスからコードも消す）
  const pa = await (await b.newContext()).newPage();
  await pa.goto(once);
  await pa.waitForSelector("#msg.show", { timeout: 10000 });
  check("使い終わったコードでは鍵を受け取れず、そう伝える", /already used or has expired/.test(await pa.locator("#msg").innerText()) &&
    await pa.evaluate(() => localStorage.getItem("kiroku:key")) === null && !pa.url().includes("code="), pa.url());
  // 中継のページの HTML（GET の答え）に鍵は入らない（リンクのプレビューなどが受け取らないように）
  const hop = await (await fetch(once)).text();
  check("?code= の GET の答えに鍵が入らない", !hop.includes(key));

  // 以前の版の ?key= は受け付けない（鍵が合っていても）。新しいコンテキストで開き、履歴も鍵の保存もないことを見る
  const pk = await (await b.newContext()).newPage();
  const rk = await pk.goto(`${base}/?key=${key}`);
  check("?key= は 403", rk && rk.status() === 403, String(rk && rk.status()));
  check("?key= では履歴を出さず、鍵もしまわない", await pk.locator(".run[data-sid]").count() === 0 &&
    await pk.evaluate(() => localStorage.getItem("kiroku:key")) === null);
  const cookies = await ctx.cookies();
  check("鍵の Cookie がない", !cookies.some(c => c.value.includes(key) || c.name.startsWith("kiroku_key_")), JSON.stringify(cookies));

  await openWithKey(p, base); // kiroku serve --open の中継のページがしまうのと同じく、localStorage に鍵を置いてから鍵なしの / を開く
  await p.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("鍵なしの / を開き直しても履歴が出る", await p.locator(".run[data-sid]").count() > 0);
  const title = await p.evaluate(() => DATA.length ? DATA[DATA.length - 1].title : "");
  check("履歴の題がある（下の確かめのため）", !!title);

  // 画面から、同じコンピューターのほかのポートのサーバーへ（リンクで移る・アドレスを打つ・Referer・favicon）
  await p.evaluate(u => { const a = document.createElement("a"); a.href = u; document.body.append(a); a.click(); }, `http://localhost:${otherPort}/from-link`);
  await p.waitForURL(u => u.port === String(otherPort), { timeout: 10000 });
  await p.goto(`http://localhost:${otherPort}/typed`);
  await p.goto(`http://127.0.0.1:${otherPort}/typed-ip`);
  // favicon を取りに来るのを待つ（来たらすぐ先へ。ブラウザが取りに来なくても、長くて 2 秒で先へ）
  for (const end = Date.now() + 2000; Date.now() < end && !["localhost", "127.0.0.1"].every(h => seen.some(r => r.url === "/favicon.ico" && (r.headers.host || "").startsWith(h + ":")));) await p.waitForTimeout(50);
  check("ほかのポートのサーバーに届いた（確かめになっている）", seen.length >= 3, String(seen.length));
  const leaks = seen.filter(r => JSON.stringify(r).includes(key));
  check("ほかのポートのサーバーへの問い合わせに鍵が載らない", leaks.length === 0, JSON.stringify(leaks));
  check("ほかのポートのサーバーへ kiroku の Cookie を送らない", !seen.some(r => /kiroku/.test(r.headers.cookie || "")), JSON.stringify(seen.map(r => r.headers.cookie)));

  // 履歴ファイルの「Open」は、鍵を付けて読み、Blob から保存する
  await p.goto(`${base}/`);
  await p.waitForSelector(".run[data-sid]", { timeout: 15000 });
  await p.evaluate(() => { const r = [...document.querySelectorAll(".run[data-sid]")].find(e => DATA.some(s => s.id === e.dataset.sid && s.file)); r.scrollIntoView({ block: "center" }); r.click(); });
  check("セッションの詳細の履歴ファイル・モデル・ツールは畳まずに出ている", await p.evaluate(() => !document.querySelector("#panel details.sdet") && !!document.querySelector("#panel .sdet a[data-hist]")));
  const link = p.locator("#panel a[data-hist]");
  await link.waitFor({ timeout: 5000 });
  const [dl] = await Promise.all([p.waitForEvent("download", { timeout: 10000 }), link.click()]);
  const saved = await readFile(await dl.path(), "utf8");
  check("履歴ファイルを保存できる", /\.jsonl$/.test(dl.suggestedFilename()) && saved.includes('"sessionId"'), dl.suggestedFilename());
  await p.keyboard.press("Escape");
  check("CSP に止められたもの・ページのエラーがない", errors.length === 0, JSON.stringify(errors));

  // 日報と振り返りは、プロンプトをコピーするだけ（kiroku は AI を動かさない）。AI を動かすボタン・ダイアログ・/ai/run はない
  await p.goto(`${base}/`);
  await p.waitForSelector(".head .dayrun", { timeout: 15000 });
  check("AI を動かすボタンはなく、日報はプロンプトのコピー", await p.locator(".ai-action").count() === 0 && await p.locator(".head .dayrun.ai-copy").count() > 0);
  check("バッジ（点だけ）の名前に AI はない", !((await p.locator("#live").getAttribute("aria-label")) || "").includes("AI"));
  const dayBtn = p.locator(".head .dayrun").first();
  await p.evaluate(() => scrollTo(0, 400)); await p.waitForTimeout(150);
  const before = await p.evaluate(() => scrollY); await dayBtn.focus(); await p.waitForTimeout(150);
  check("日報のボタンにフォーカスしても、ページの位置が変わらない", await p.evaluate(() => scrollY) === before);
  const grab = async (pg, sel) => pg.evaluate(async q => { window.__copied = null; navigator.clipboard.writeText = t => { window.__copied = t; return Promise.resolve(); };
    document.querySelector(q).click(); await new Promise(z => setTimeout(z, 300));
    return {got: window.__copied || "", toast: document.querySelector("#toast").textContent, dialog: !!document.querySelector("dialog[open]")}; }, sel);
  { const r = await grab(p, ".head .dayrun");
    check("日報のボタンは、その日の形式と前日との数字の表のプロンプトをコピーするだけ", /## Daily report: /.test(r.got) && /^\| \| This day \| Previous day \|$/m.test(r.got) && !r.dialog && /^Copied the prompt\. kiroku didn't run or send anything\. Paste it into your AI agent\.$/.test(r.toast) && !/--ai/.test(r.toast), JSON.stringify({...r, got: r.got.slice(0, 200)})); }
  await p.evaluate(() => select(DATA.find(s => s.prompts.length).id)); await p.waitForSelector("#panel #srun.ai-copy", { timeout: 5000 });
  { const r = await grab(p, "#panel #srun");
    check("振り返りはプロンプトをコピーするだけ（ダイアログも実行もない）", /History data|history file/i.test(r.got) && !r.dialog && !/--ai/.test(r.toast), JSON.stringify({...r, got: r.got.slice(0, 80)})); }
  await p.keyboard.press("Escape");
  const sz = await p.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("鍵があっても /ai/run は 404", sz === 404, String(sz));
  await p.click("#mode button[data-v=month]");
  await p.waitForFunction(() => document.querySelector("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
  check("月の表示には、報告のボタンがない", await p.locator(".dayrun").count() === 0);
  await p.click("#mode button[data-v=week]");
  await p.waitForSelector(".head .dayrun", { timeout: 5000 });

  // --ai（4.0 で外した）を付けても、止まらずに外したことを 1 行で言い、AI を動かす道は出てこない
  const startExtra = async (args, env2) => { let lg = ""; const s = spawn(path.resolve(bin), args, { env: { ...env, ...env2 }, stdio: ["ignore", "pipe", "pipe"] });
    extra.push(s); s.stdout.on("data", d => { lg += d; }); s.stderr.on("data", d => { lg += d; });
    const pt = await until("待ち受け（追加）", () => (lg.match(/serving http:\/\/localhost:(\d+)\//) || [])[1]);
    await until("読み込み（追加）", async () => (await fetch(`http://localhost:${pt}/stamp`, { headers: { "X-Kiroku-Key": key } }).catch(() => ({ ok: false }))).ok);
    return { url: `http://localhost:${pt}`, log: () => lg }; };
  // PRトリアージ（--prs）：GitHub に問い合わせる（gh を動かす）のは、ページの中で PRトリアージへのリンクを押したときと「再取得」だけ。
  // URL（/#/prs）で開いても（ほかのサイトのリンクから移ってきたときと同じ）、戻る・進むでも問い合わせない。gh は呼ばれた回数を残す偽物
  {
    const fakeGh = path.join(work, "fakegh"), ghLog = path.join(work, "gh-calls"); await mkdir(fakeGh, { recursive: true });
    await writeFile(path.join(fakeGh, "gh"), `#!/bin/sh
echo x >> "${ghLog}"
printf '%s' '{"data":{"viewer":{"login":"me-dev"},"author":{"nodes":[]},"review":{"nodes":[]},"mention":{"nodes":[]}}}'
`); await chmod(path.join(fakeGh, "gh"), 0o755);
    const calls = async () => (await readFile(ghLog, "utf8").catch(() => "")).split("\n").filter(Boolean).length;
    const ps = await startExtra([...serveArgs, "--prs"], { PATH: `${fakeGh}${path.delimiter}${process.env.PATH}` });
    const pp = await ctx.newPage(), asked = []; pp.on("request", r => { if (new URL(r.url()).pathname === "/prs") asked.push(r.method()); });
    await pp.goto(`${ps.url}/`); await pp.evaluate(k => localStorage.setItem("kiroku:key", k), key);
    await pp.goto(`${ps.url}/#/prs`); await pp.reload(); await pp.waitForSelector("#prsv .prpanel", { timeout: 15000 }); await pp.waitForTimeout(800);
    check("--prs: URL（/#/prs）で開いても GitHub に問い合わせない（ほかのサイトから移らせても gh は動かない）", asked.length === 0 && await calls() === 0 && /Refresh/.test(await pp.locator("#prsv").innerText()), `${asked} ${await calls()}`);
    await pp.locator("#home").click(); await pp.waitForFunction(() => document.body.dataset.page === "sessions");
    await pp.goBack(); await pp.waitForFunction(() => document.body.dataset.page === "prs"); await pp.waitForTimeout(500);
    check("--prs: 戻るで PRトリアージに戻っても問い合わせない", asked.length === 0 && await calls() === 0);
    await pp.goForward(); await pp.waitForFunction(() => document.body.dataset.page === "sessions");
    await pp.evaluate(() => { location.hash = "#/prs"; }); await pp.waitForFunction(() => document.body.dataset.page === "prs"); await pp.waitForTimeout(500);
    check("--prs: スクリプトで # を変えても問い合わせない", asked.length === 0);
    await pp.locator("#home").click(); await pp.waitForFunction(() => document.body.dataset.page === "sessions");
    await pp.locator('#gnav a[data-page="prs"]').click(); await pp.waitForSelector("#prsv .prsec, #prsv .none", { timeout: 15000 });
    check("--prs: メニューの PRトリアージを押すと問い合わせる", asked.join() === "GET" && await calls() === 1, `${asked} ${await calls()}`);
    await pp.close();
  }
  const old = await startExtra([...serveArgs, "--ai"], { KIROKU_AI: "1" });
  check("--ai を付けても起動し、外したことと「Copy prompt」を言う", /--ai \(\$KIROKU_AI\) was removed in 4\.0/.test(old.log()) && /Copy prompt/.test(old.log()), old.log());
  const po = await ctx.newPage();
  await openWithKey(po, old.url);
  await po.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("--ai を付けても、AI を動かすボタンはない", await po.locator(".ai-action").count() === 0 && await po.locator(".head .dayrun.ai-copy").count() > 0);
  const so = await po.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("--ai を付けても、/ai/run は 404", so === 404, String(so));
  await po.close();

  // kiroku serve が 500 や読めないデータを返したときは、「つながっていない」ではなく、そう言って端末を見るよう案内する。
  // 読み直しに失敗して前の集計のまま答えているとき（/stamp の X-Kiroku-Reload: failed）は、バッジがそれを言う
  for (const [what, fulfill, want] of [["500", { status: 500, body: "boom" }, /answered with an error \(500\)/], ["読めないデータ", { status: 200, contentType: "application/json", body: "{" }, /could not be read/]]) {
    const pe = await ctx.newPage();
    await pe.route("**/data.json", r => r.fulfill(fulfill));
    await pe.goto(`${base}/`);
    await pe.waitForSelector("#locked", { timeout: 10000 });
    const lt = await pe.locator("#locked").innerText();
    check(`data.json が${what}のとき、つながっていないとは言わず、そう案内する`, want.test(lt) && /kiroku serve could not send your history/.test(lt) && !/Not connected/.test(lt), lt);
    await pe.close();
  }
  {
    const pr = await ctx.newPage();
    let mode = "ok";
    await pr.route("**/stamp", async r => {
      if (mode === "ok") return r.continue();
      const res = await r.fetch();
      await r.fulfill(mode === "500" ? { status: 500, body: "boom" } : { response: res, headers: { ...res.headers(), "x-kiroku-reload": "failed" } });
    });
    await pr.goto(`${base}/`);
    await pr.waitForSelector(".run[data-sid]", { timeout: 15000 });
    mode = "failed";
    await pr.waitForFunction(() => /Could not read the latest history/.test(document.querySelector("#live").title), null, { timeout: 15000 });
    check("読み直しに失敗しているとき、バッジがそう言う", await pr.evaluate(() => document.querySelector("#live").classList.contains("off")));
    mode = "500";
    await pr.waitForFunction(() => /answered with an error \(500\)/.test(document.querySelector("#live").title), null, { timeout: 15000 }).then(() => true, () => false)
      .then(ok => check("/stamp が 500 のとき、バッジはつながっていないとは言わない", ok));
    mode = "ok";
    await pr.waitForFunction(() => /last checked/.test(document.querySelector("#live").title), null, { timeout: 15000 }).then(() => true, () => false)
      .then(ok => check("直れば、バッジは最終確認の時刻に戻る", ok));
    await pr.close();
  }

  // 「Data sources」の「Keep a copy in this folder」をキーボードで押すと、コピーを残し始め、パネルは開いたまま、
  // フォーカスは描き直したその行（「kiroku keeps a copy」）に移る（ボタンが消えても、ページの頭へ飛ばない）
  // gen.py の履歴は cleanupPeriodDays を 3650 にしてあるので、設定のファイルのない写しを読む serve を別に立てる（既定の 30 日で消える履歴）
  // 知らせ（「Keeping a copy in …」）は 6 秒出るので、フォーカスを移した行と「kiroku's copy: on」の行に重ならない（下に重なるなら上に出す）。広い画面とスマホで
  for (const vp of [{ name: "1440", viewport: { width: 1440, height: 900 } }, { name: "320", viewport: { width: 320, height: 680 }, isMobile: true, hasTouch: true }]) {
    const keepHome = path.join(work, `keephome${vp.name}`, ".claude");
    await cp(path.join(home, ".claude", "projects"), path.join(keepHome, "projects"), { recursive: true });
    const kp = await startExtra(["serve", "127.0.0.1:0", "--no-open", "--sources", "claude", "--claude-root", path.join(keepHome, "projects"), "--archive-dir", path.join(work, `keeparchive${vp.name}`)], {});
    const kctx = await b.newContext({ viewport: vp.viewport, isMobile: !!vp.isMobile, hasTouch: !!vp.hasTouch });
    const pd = await kctx.newPage();
    await openWithKey(pd, kp.url);
    await pd.waitForSelector(".run[data-sid]", { timeout: 15000 });
    await pd.locator("#review details.dsrc > summary").click();
    const btn = pd.locator("#review details.dsrc .keeparch").first();
    check(`${vp.name}: 既定のままで消える履歴の行に「Keep a copy in this folder」がある`, await btn.count() > 0);
    const ag = await btn.evaluate(e => e.closest("li").dataset.dsrc);
    await btn.evaluate(e => e.scrollIntoView({ block: "end" })); // 下へスクロールしてきて、画面の下の端にあるボタンを押す（知らせの出る場所）
    await btn.focus({ preventScroll: true }); await pd.keyboard.press("Enter");
    await pd.waitForFunction(() => /Keeping a copy in/.test(document.querySelector("#toast").textContent), null, { timeout: 15000 });
    await pd.waitForTimeout(400); // 知らせが出きるまで
    const r = await pd.evaluate(() => { const D = document.querySelector("#review details.dsrc"), a = document.activeElement, ar = D && D.querySelector("li.arch"), tb = document.querySelector("#toast").getBoundingClientRect();
      const hit = e => { if (!e) return false; const x = e.getBoundingClientRect(); return x.bottom > tb.top && x.top < tb.bottom && x.right > tb.left && x.left < tb.right; };
      return {open: !!D && D.open, ag: a && a.dataset ? a.dataset.dsrc : null, inD: !!D && D.contains(a), text: a ? a.textContent : "", buttons: document.querySelectorAll("#review .keeparch").length,
        arch: ar ? ar.textContent : "", on: document.querySelector("#toast").classList.contains("on"), coversFocus: hit(a), coversArch: hit(ar), toast: [Math.round(tb.top), Math.round(tb.bottom)]}; });
    check(`${vp.name}: コピーを残すと、Data sources は開いたまま、その行は残していると言い、フォーカスはその行にある`, r.open && r.inD && r.ag === ag && /kiroku keeps a copy/.test(r.text) && r.buttons === 0 && /kiroku's copy: on/.test(r.arch), JSON.stringify(r));
    check(`${vp.name}: 「Keeping a copy in …」の知らせは、フォーカスした行にも「kiroku's copy: on」の行にも重ならない`, r.on && !r.coversFocus && !r.coversArch, JSON.stringify(r));
    await kctx.close();
  }

  // 鍵のないブラウザ（同じコンピューターのほかのユーザーなど）は、案内だけを見る
  const fresh = await b.newContext();
  const q = await fresh.newPage();
  const qErrors = [];
  q.on("pageerror", e => qErrors.push(e.message));
  await q.goto(`${base}/`);
  await q.waitForSelector("#locked", { timeout: 10000 });
  const text = await q.evaluate(() => document.body.innerText);
  check("鍵のないブラウザには案内が出る", text.includes("Open kiroku with its key") && text.includes("kiroku serve") && !text.includes("--open") && !text.includes("--print"));
  check("鍵のないブラウザには履歴が出ない", !text.includes(title) && await q.locator(".run[data-sid]").count() === 0 && await q.evaluate(() => DATA.length) === 0);
  const html = await (await q.request.get(`${base}/`)).text();
  check("鍵なしの / の HTML に履歴の題が入っていない", !html.includes(title));
  check("鍵のないブラウザで data.json は 403", (await q.request.get(`${base}/data.json`)).status() === 403);
  check("鍵のないブラウザでページのエラーがない", qErrors.length === 0, JSON.stringify(qErrors));
  // 違う鍵の ?key= は 403 で、localStorage にしまわない
  const bad = await q.goto(`${base}/?key=${"0".repeat(64)}`);
  check("違う鍵の ?key= は 403", bad.status() === 403 && await q.evaluate(() => localStorage.getItem("kiroku:key")) === null);
} catch (e) {
  failed++; console.log(`  FAIL ${e.message}`);
} finally {
  if (b) await b.close();
  other.close();
  await stop();
}
console.log(failed ? `\n${failed} 件 NG` : "\nすべて OK");
process.exit(failed ? 1 : 0);
