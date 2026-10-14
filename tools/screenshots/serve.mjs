// kiroku serve の鍵（ADR 0031）を、本物のブラウザで確かめる（CI の e2e）。
//   go build -o /tmp/kiroku . && python3 -I tools/screenshots/gen.py /tmp/gen && node tools/screenshots/serve.mjs /tmp/kiroku /tmp/gen
// ダミーの履歴（gen.py）だけを読む kiroku serve を、使い捨ての HOME と設定の場所で空いているポートに立て、
//   - /?key= で開くと履歴が出て、アドレスから鍵が消え、鍵の Cookie はない
//   - 鍵なしの / を開き直しても出る（localStorage の鍵）。履歴ファイルの「Open」は鍵を付けて読んで保存する
//   - 鍵のないブラウザ（新しいコンテキスト）は、案内だけで履歴を見ない。data.json も 403
//   - 画面から同じコンピューターのほかのポートのサーバーへ移っても、鍵はどの問い合わせにも載らない（Cookie にも、ほかのところにも）
// を見る。Playwright が必要（smoke.mjs と同じ）。
import { chromium } from "playwright";
import { spawn } from "node:child_process";
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
const env = { ...process.env, HOME: path.join(work, "home"), KIROKU_CONFIG_DIR: path.join(work, "config"), KIROKU_LANG: "", TZ: "Asia/Tokyo" };
const srv = spawn(path.resolve(bin), ["serve", "127.0.0.1:0", "--no-open", "--sources", "claude", "--claude-root", path.join(home, ".claude", "projects"),
  "--archive-dir", path.join(work, "archive")], { env, stdio: ["ignore", "pipe", "pipe"] });
let log = "";
srv.stdout.on("data", d => { log += d; }); srv.stderr.on("data", d => { log += d; });
const stop = async () => { srv.kill(); await rm(work, { recursive: true, force: true }); };

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
  const base = `http://localhost:${port}`;
  // 読み終わるのを待つ（以前の版の Cookie も付けるのは、修正の前の版に対してこのテストが失敗することを確かめられるように）
  await until("読み込み", async () => (await fetch(`${base}/stamp`, { headers: { "X-Kiroku-Key": key, Cookie: `kiroku_key_${port}=${key}` } }).catch(() => ({ ok: false }))).ok);
  check("鍵なしの data.json は 403", (await fetch(`${base}/data.json`)).status === 403);
  check("鍵の Cookie（以前の版）だけの data.json は 403", (await fetch(`${base}/data.json`, { headers: { Cookie: `kiroku_key_${port}=${key}` } })).status === 403);

  b = await chromium.launch(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {});
  const ctx = await b.newContext({ acceptDownloads: true });
  const p = await ctx.newPage();
  const errors = [];
  p.on("pageerror", e => errors.push(e.message));
  p.on("console", m => { if (m.type() === "error" && /Content Security Policy|Refused/.test(m.text())) errors.push(m.text()); });

  await p.goto(`${base}/?key=${key}`);
  await p.waitForURL(u => !u.search.includes("key="), { timeout: 10000 });
  await p.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("?key= で開くと履歴が出る", await p.locator(".run[data-sid]").count() > 0);
  check("アドレスに鍵が残らない", !p.url().includes(key), p.url());
  check("鍵は localStorage にある", await p.evaluate(() => localStorage.getItem("kiroku:key")) === key);
  const cookies = await ctx.cookies();
  check("鍵の Cookie がない", !cookies.some(c => c.value.includes(key) || c.name.startsWith("kiroku_key_")), JSON.stringify(cookies));
  await p.goBack(); // ?key= のページは location.replace で置きかえたので、戻ると開く前（about:blank）
  check("鍵がブラウザの履歴に残らない", !p.url().includes("key="), p.url());

  await p.goto(`${base}/`);
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
  const link = p.locator("#panel a[data-hist]");
  await link.waitFor({ timeout: 5000 });
  const [dl] = await Promise.all([p.waitForEvent("download", { timeout: 10000 }), link.click()]);
  const saved = await readFile(await dl.path(), "utf8");
  check("履歴ファイルを保存できる", /\.jsonl$/.test(dl.suggestedFilename()) && saved.includes('"sessionId"'), dl.suggestedFilename());
  await p.keyboard.press("Escape");
  check("CSP に止められたもの・ページのエラーがない", errors.length === 0, JSON.stringify(errors));

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
