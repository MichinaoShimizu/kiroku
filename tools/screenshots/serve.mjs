// kiroku serve の鍵を、本物のブラウザで確かめる（CI の e2e）。
//   go build -o /tmp/kiroku . && python3 -I tools/screenshots/gen.py /tmp/gen && node tools/screenshots/serve.mjs /tmp/kiroku /tmp/gen
// ダミーの履歴（gen.py）だけを読む kiroku serve を、使い捨ての HOME と設定の場所で空いているポートに立て、
//   - kiroku open --print の使い捨てのコード（?code=）で開くと履歴が出て、どの URL にも鍵が載らない（ブラウザの履歴に鍵が残らない）。
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
import { mkdtemp, readFile, rm } from "node:fs/promises";
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
  // 別のポートの serve は別のオリジンなので、その localStorage に鍵をしまってから開く（kiroku open の中継のページと同じこと）
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

  // kiroku open --print の使い捨てのコード。開いた URL（ブラウザの履歴に残るもの）のどれにも鍵が載らない
  const pc = await (await b.newContext()).newPage();
  const visited = [];
  pc.on("request", r => visited.push(r.url()));
  const once = execFileSync(path.resolve(bin), ["open", "--print", `127.0.0.1:${port}`], { env, encoding: "utf8" }).split("\n")[0].trim();
  check("kiroku open --print は使い捨てのコードの URL で、鍵を出さない", /^http:\/\/localhost:\d+\/\?code=[0-9a-f]{64}$/.test(once) && !once.includes(key), once);
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

  await openWithKey(p, base); // kiroku open の中継のページがしまうのと同じく、localStorage に鍵を置いてから鍵なしの / を開く
  await p.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("鍵なしの / を開き直しても履歴が出る", await p.locator(".run[data-sid]").count() > 0);
  const title = await p.evaluate(() => DATA.length ? DATA[DATA.length - 1].title : "");
  check("履歴の題がある（下の確かめのため）", !!title);

  // 画面から、同じコンピューターのほかのポートのサーバーへ（リンクで移る・アドレスを打つ・Referer・favicon）
  await p.evaluate(u => { const a = document.createElement("a"); a.href = u; document.body.append(a); a.click(); }, `http://localhost:${otherPort}/from-link`);
  await p.waitForURL(u => u.port === String(otherPort), { timeout: 10000 });
  await p.goto(`http://localhost:${otherPort}/typed`);
  await p.goto(`http://127.0.0.1:${otherPort}/typed-ip`);
  await p.waitForTimeout(500); // favicon を取りに来るのを待つ
  check("ほかのポートのサーバーに届いた（確かめになっている）", seen.length >= 3, String(seen.length));
  const leaks = seen.filter(r => JSON.stringify(r).includes(key));
  check("ほかのポートのサーバーへの問い合わせに鍵が載らない", leaks.length === 0, JSON.stringify(leaks));
  check("ほかのポートのサーバーへ kiroku の Cookie を送らない", !seen.some(r => /kiroku/.test(r.headers.cookie || "")), JSON.stringify(seen.map(r => r.headers.cookie)));

  // 履歴ファイルの「Open」は、鍵を付けて読み、Blob から保存する
  await p.goto(`${base}/`);
  await p.waitForSelector(".run[data-sid]", { timeout: 15000 });
  await p.evaluate(() => { const r = [...document.querySelectorAll(".run[data-sid]")].find(e => DATA.some(s => s.id === e.dataset.sid && s.file)); r.scrollIntoView({ block: "center" }); r.click(); });
  check("セッションの詳細の「詳しい情報」は、いつもは閉じている", await p.evaluate(() => { const d = document.querySelector("#panel details.sdet"); return !!d && !d.open; }));
  await p.evaluate(() => { const d = document.querySelector("#panel details.sdet"); if (d) d.open = true; }); // 履歴ファイルは「詳しい情報」の中
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
  check("週報のボタンはない（日報とセッションの振り返りだけ）", await p.locator("#rptbar, #rptrun").count() === 0);
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
    check("日報のボタンは、その日の形式と前日との数字の表のプロンプトをコピーするだけ", /## Daily report: /.test(r.got) && /^\| \| This day \| Previous day \|$/m.test(r.got) && !r.dialog && /kiroku ran nothing and sent nothing; paste it into your AI agent/.test(r.toast) && !/--ai/.test(r.toast), JSON.stringify({...r, got: r.got.slice(0, 200)})); }
  await p.evaluate(() => select(DATA.find(s => s.prompts.length).id)); await p.waitForSelector("#panel #srun.ai-copy", { timeout: 5000 });
  { const r = await grab(p, "#panel #srun");
    check("振り返りはプロンプトをコピーするだけ（ダイアログも実行もない）", /History data|history file/i.test(r.got) && !r.dialog && !/--ai/.test(r.toast), JSON.stringify({...r, got: r.got.slice(0, 80)})); }
  await p.keyboard.press("Escape");
  const sz = await p.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("鍵があっても /ai/run は 404", sz === 404, String(sz));
  await p.click("#mode button[data-v=month]");
  await p.waitForFunction(() => document.querySelector("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
  check("月の表示には、報告のボタンがない", await p.locator(".dayrun, #rptrun").count() === 0);
  await p.click("#mode button[data-v=week]");
  await p.waitForSelector(".head .dayrun", { timeout: 5000 });

  // --ai（4.0 で外した）を付けても、止まらずに外したことを 1 行で言い、AI を動かす道は出てこない
  const startExtra = async (args, env2) => { let lg = ""; const s = spawn(path.resolve(bin), args, { env: { ...env, ...env2 }, stdio: ["ignore", "pipe", "pipe"] });
    extra.push(s); s.stdout.on("data", d => { lg += d; }); s.stderr.on("data", d => { lg += d; });
    const pt = await until("待ち受け（追加）", () => (lg.match(/serving http:\/\/localhost:(\d+)\//) || [])[1]);
    await until("読み込み（追加）", async () => (await fetch(`http://localhost:${pt}/stamp`, { headers: { "X-Kiroku-Key": key } }).catch(() => ({ ok: false }))).ok);
    return { url: `http://localhost:${pt}`, log: () => lg }; };
  const old = await startExtra([...serveArgs, "--ai"], { KIROKU_AI: "1" });
  check("--ai を付けても起動し、外したことと「Copy prompt」を言う", /--ai \(\$KIROKU_AI\) was removed in 4\.0/.test(old.log()) && /Copy prompt/.test(old.log()), old.log());
  const po = await ctx.newPage();
  await openWithKey(po, old.url);
  await po.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("--ai を付けても、AI を動かすボタンはない", await po.locator(".ai-action").count() === 0 && await po.locator(".head .dayrun.ai-copy").count() > 0);
  const so = await po.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("--ai を付けても、/ai/run は 404", so === 404, String(so));
  await po.close();

  // 鍵のないブラウザ（同じコンピューターのほかのユーザーなど）は、案内だけを見る
  const fresh = await b.newContext();
  const q = await fresh.newPage();
  const qErrors = [];
  q.on("pageerror", e => qErrors.push(e.message));
  await q.goto(`${base}/`);
  await q.waitForSelector("#locked", { timeout: 10000 });
  const text = await q.evaluate(() => document.body.innerText);
  check("鍵のないブラウザには案内が出る", text.includes("Open kiroku with its key") && text.includes("kiroku open"));
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
