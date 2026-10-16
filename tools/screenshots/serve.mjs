// kiroku serve の鍵（ADR 0031）を、本物のブラウザで確かめる（CI の e2e）。
//   go build -o /tmp/kiroku . && python3 -I tools/screenshots/gen.py /tmp/gen && node tools/screenshots/serve.mjs /tmp/kiroku /tmp/gen
// ダミーの履歴（gen.py）だけを読む kiroku serve を、使い捨ての HOME と設定の場所で空いているポートに立て、
//   - /?key= で開くと履歴が出て、アドレスから鍵が消え、鍵の Cookie はない
//   - 鍵なしの / を開き直しても出る（localStorage の鍵）。履歴ファイルの「Open」は鍵を付けて読んで保存する
//   - 鍵のないブラウザ（新しいコンテキスト）は、案内だけで履歴を見ない。data.json も 403
//   - 画面から同じコンピューターのほかのポートのサーバーへ移っても、鍵はどの問い合わせにも載らない（Cookie にも、ほかのところにも）
//   - 「Analyze with AI」（ADR 0034）: フラグなしで出て、押すと送るプロンプトをそのまま見せる（Run は押さない）。エージェントのコマンドがなければ押せず理由を添え、
//     --no-ai では「(off)」だけで /ai/run は 404。エージェントは偽の codex（何もしないスクリプト）を PATH の先頭に置いて決める
// を見る。Playwright が必要（smoke.mjs と同じ）。
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { createServer } from "node:http";
import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

const [bin, gen] = process.argv.slice(2);
if (!bin || !gen) { console.error("使い方: node serve.mjs <kiroku> <gen.py の出力先>"); process.exit(2); }
let failed = 0;
const check = (what, ok, detail = "") => { if (ok) console.log(`  ok   ${what}`); else { failed++; console.log(`  FAIL ${what}${detail ? `: ${detail}` : ""}`); } };

const work = await mkdtemp(path.join(tmpdir(), "kiroku-serve-e2e-"));
const home = path.join(gen, "home");
// 偽のエージェント（codex）。起動しても何もせずに終わる。画面が「使えるエージェントがある」ことを決まって見せるためだけに置き、Run は押さない
const fakeBin = path.join(work, "bin"), emptyBin = path.join(work, "empty");
await mkdir(fakeBin, { recursive: true }); await mkdir(emptyBin, { recursive: true });
await writeFile(path.join(fakeBin, "codex"), "#!/bin/sh\nexit 1\n"); await chmod(path.join(fakeBin, "codex"), 0o700);
const env = { ...process.env, PATH: `${fakeBin}${path.delimiter}${process.env.PATH || ""}`, HOME: path.join(work, "home"), KIROKU_CONFIG_DIR: path.join(work, "config"), KIROKU_LANG: "", KIROKU_NO_AI: "", TZ: "Asia/Tokyo" };
const serveArgs = ["serve", "127.0.0.1:0", "--no-open", "--sources", "claude", "--claude-root", path.join(home, ".claude", "projects"), "--archive-dir", path.join(work, "archive")];
const srv = spawn(path.resolve(bin), serveArgs, { env, stdio: ["ignore", "pipe", "pipe"] });
let log = "";
srv.stdout.on("data", d => { log += d; }); srv.stderr.on("data", d => { log += d; });
const extra = []; // あとで立てる serve（エージェントなし・--no-ai）
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
  const base = `http://localhost:${port}`;
  // 読み終わるのを待つ（以前の版の Cookie も付けるのは、修正の前の版に対してこのテストが失敗することを確かめられるように）
  await until("読み込み", async () => (await fetch(`${base}/stamp`, { headers: { "X-Kiroku-Key": key, Cookie: `kiroku_key_${port}=${key}` } }).catch(() => ({ ok: false }))).ok);
  check("鍵なしの data.json は 403", (await fetch(`${base}/data.json`)).status === 403);
  check("起動の 1 行で、AI の実行がオンで --no-ai で切れ、提供元に送りうると言う", /"Analyze with AI" is on: .*may send the prompt and the history it reads to its provider; turn it off with --no-ai/.test(log), log);
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

  // 「Analyze with AI」（ADR 0034）: フラグなしで出る。押すと送るプロンプトをそのまま見せ、提供元の一文はダイアログに出さない。Run は押さない
  await p.goto(`${base}/`);
  await p.waitForSelector("#rptbar #rptrun", { timeout: 15000 });
  check("フラグなしの serve で、週報の「Analyze with AI」が押せる", await p.locator("#rptbar #rptrun").isEnabled());
  check("バッジが LIVE · AI on", (await p.locator("#live").innerText()).includes("AI on"));
  await p.locator("#rptbar #rptrun").click();
  await p.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  const dlg = await p.evaluate(() => { const d = document.querySelector("#ai-review-dialog"); return { text: d.innerText, prompt: d.querySelector("textarea.ai-prompt").value, readOnly: d.querySelector("textarea.ai-prompt").readOnly, agents: [...d.querySelectorAll("select option")].map(o => o.value) }; });
  check("ダイアログは送るプロンプトをそのまま（読み取り専用で）見せる", dlg.readOnly && /weekly report/.test(dlg.prompt) && /Check the exact prompt below/.test(dlg.text));
  check("ダイアログに提供元の一文はない（ドキュメントと起動の 1 行にある）", !/provider/.test(dlg.text));
  check("エージェントはこのコンピューターにあるものだけ", dlg.agents.includes("codex"), JSON.stringify(dlg.agents));
  await p.locator("#ai-review-dialog button", { hasText: "Close" }).click();
  check("Close で閉じ、何も動かさない", await p.locator("#ai-review-dialog").count() === 0 && !/\$ codex/.test(log));
  // 日報の「Analyze with AI」: 週のカレンダーの、記録のある日付の横。押すとその日の日報のプロンプト（前日と並べた数字の表）
  const dayBtn = p.locator(".head .dayrun").first();
  check("記録のある日の見出しに、日報のボタンがある", await dayBtn.count() === 1 && await dayBtn.isEnabled());
  const sc = p.locator(".calscroll"); await sc.evaluate(e => { e.scrollTop = 300; }); await p.waitForTimeout(150);
  const before = await sc.evaluate(e => e.scrollTop); await dayBtn.focus(); await p.waitForTimeout(150);
  check("日報のボタンにフォーカスしても、カレンダーの位置が変わらない", await sc.evaluate(e => e.scrollTop) === before);
  await dayBtn.click();
  await p.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  const dp = await p.evaluate(() => document.querySelector("#ai-review-dialog textarea.ai-prompt").value);
  check("日報のプロンプトは、その日の形式と前日との数字の表", /## Daily report: /.test(dp) && /^\| \| This day \| Previous day \|$/m.test(dp) && /^\| Active time \| /m.test(dp), dp.slice(0, 300));
  await p.locator("#ai-review-dialog button", { hasText: "Close" }).click();
  check("日報も Close で閉じ、何も動かさない", await p.locator("#ai-review-dialog").count() === 0 && !/\$ codex/.test(log));
  // 月報はない（ADR 0035）。月の表示では報告の帯もボタンも出さない
  await p.click("#mode button[data-v=month]");
  await p.waitForFunction(() => document.querySelector("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
  check("月の表示には、報告の「Analyze with AI」がない", await p.locator("#rptbar").isHidden() && await p.locator("#rptrun").count() === 0);
  await p.click("#mode button[data-v=week]");
  await p.waitForSelector("#rptbar #rptrun", { timeout: 5000 });

  // エージェントのコマンドが 1 つもない serve と、--no-ai の serve を、同じ鍵（同じ設定の場所）で立てる
  const startExtra = async (args, env2) => { let lg = ""; const s = spawn(path.resolve(bin), args, { env: { ...env, ...env2 }, stdio: ["ignore", "pipe", "pipe"] });
    extra.push(s); s.stdout.on("data", d => { lg += d; }); s.stderr.on("data", d => { lg += d; });
    const pt = await until("待ち受け（追加）", () => (lg.match(/serving http:\/\/localhost:(\d+)\//) || [])[1]);
    await until("読み込み（追加）", async () => (await fetch(`http://localhost:${pt}/stamp`, { headers: { "X-Kiroku-Key": key } }).catch(() => ({ ok: false }))).ok);
    return { url: `http://localhost:${pt}`, log: () => lg }; };
  const none = await startExtra(serveArgs, { PATH: emptyBin });
  const pn = await ctx.newPage();
  await pn.goto(`${none.url}/?key=${key}`);
  await pn.waitForSelector("#rptbar .ai-none", { timeout: 15000 });
  check("エージェントがないと、日報のボタンは出さない（理由は週報の帯で言う）", await pn.locator(".head .dayrun").count() === 0);
  check("エージェントがないと、ボタンは押せず、理由が添えてある", await pn.locator("#rptbar .ai-none button").isDisabled() && /No AI agent found/.test(await pn.locator("#rptbar .ai-why").innerText()) && /claude, codex or kiro-cli/.test(await pn.locator("#rptbar .ai-why").getAttribute("title")));
  await pn.close();
  const off = await startExtra([...serveArgs, "--no-ai", "--ai"], {});
  check("--no-ai の起動の 1 行はオフと言い、--ai はもう要らないと知らせる", /"Analyze with AI" is off \(--no-ai\)/.test(off.log()) && /--ai is no longer needed/.test(off.log()), off.log());
  const po = await ctx.newPage();
  await po.goto(`${off.url}/?key=${key}`);
  await po.waitForSelector("#rptbar .ai-off", { timeout: 15000 });
  check("--no-ai では「Analyze with AI (off)」だけ", await po.locator("#rptrun, .ai-action").count() === 0);
  const st = await po.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("--no-ai では、鍵があっても /ai/run は 404", st === 404, String(st));
  await po.close();

  // 返事の確認と「直してもらう」: 偽の codex が、節の足りない週報を返す。kiroku が足りないところを見せ、「直してもらう」は
  // 直してもらうプロンプトを見せるだけで送らない（ADR 0034）。Run で送り、直してもらうのは 2 回まで
  const fixBin = path.join(work, "fixbin"), runs = path.join(work, "runs.txt");
  await mkdir(fixBin, { recursive: true });
  await writeFile(path.join(fixBin, "codex"), `#!/bin/sh\ncat > "${path.join(work, "last-prompt.txt")}"\necho run >> "${runs}"\nprintf '## Weekly report\\n\\n### Summary\\nDone.\\n'\n`);
  await chmod(path.join(fixBin, "codex"), 0o700);
  const fx = await startExtra(serveArgs, { PATH: `${fixBin}${path.delimiter}${process.env.PATH || ""}` });
  const nRuns = async () => (await readFile(runs, "utf8").catch(() => "")).split("\n").filter(Boolean).length;
  const pf = await ctx.newPage();
  await pf.goto(`${fx.url}/?key=${key}`);
  await pf.waitForSelector("#rptbar #rptrun", { timeout: 15000 });
  await pf.locator("#rptbar #rptrun").click();
  await pf.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  const runBtn = pf.locator("#ai-review-dialog .ai-action"), fixBtn = pf.locator("#ai-review-dialog .ai-check button", { hasText: "Ask to fix" });
  await pf.locator("#ai-review-dialog select").selectOption("codex"); // このコンピューターに本物のエージェントがあっても、偽の codex を使う
  await runBtn.click();
  await pf.waitForSelector("#ai-review-dialog .ai-check:not([hidden])", { timeout: 20000 });
  check("節の足りない返事には、kiroku の確認が足りないところを見せる", /4 of the 5 sections are missing/.test(await pf.locator("#ai-review-dialog .ai-check").innerText()));
  await fixBtn.click();
  const fixPrompt = await pf.evaluate(() => document.querySelector("#ai-review-dialog textarea.ai-prompt").value);
  check("「直してもらう」は、元のプロンプト・前の返事・直すことを入れたプロンプトを見せる", /weekly report/.test(fixPrompt) && /\n# Your previous answer\n/.test(fixPrompt) && /\n# What to fix\n- The answer has 1 of the 5 "###" sections/.test(fixPrompt), fixPrompt.slice(-400));
  check("「直してもらう」だけでは送らない（Run を押すまで）", await nRuns() === 1 && await runBtn.innerText() === "Run" && await runBtn.isEnabled());
  const dlgText = async () => pf.evaluate(() => document.querySelector("#ai-review-dialog").innerText);
  check("送らないことを、ダイアログで言う", /Nothing is sent until you press Run/.test(await dlgText()));
  await pf.locator("#ai-review-dialog .ai-check button", { hasText: "Use the original prompt" }).click();
  check("元のプロンプトに戻せる", !/# What to fix/.test(await pf.evaluate(() => document.querySelector("#ai-review-dialog textarea.ai-prompt").value)) && await fixBtn.count() === 1 && await nRuns() === 1);
  // 「直してもらう」で Enter を押したままにしても、続けて Run は押されない（フォーカスは Run に移さない）
  await fixBtn.focus(); for (let i = 0; i < 4; i++) await pf.keyboard.down("Enter"); await pf.keyboard.up("Enter"); await pf.waitForTimeout(500);
  check("「直してもらう」で Enter を押したままでも送らない", await nRuns() === 1 && await runBtn.innerText() === "Run");
  await runBtn.click();
  await until("2 回目の実行", async () => await nRuns() === 2);
  check("Run で、見せたプロンプトをそのまま送る", (await readFile(path.join(work, "last-prompt.txt"), "utf8")).trim() === fixPrompt.trim());
  await pf.waitForSelector("#ai-review-dialog .ai-check:not([hidden])", { timeout: 20000 });
  check("確認の結果を状態の 1 行でも言い、「もう一度実行」は元のプロンプトに戻る", /kiroku's check found 2 problems\./.test(await pf.locator("#ai-review-dialog [role=status]").innerText()) && !/# What to fix/.test(await pf.evaluate(() => document.querySelector("#ai-review-dialog textarea.ai-prompt").value)));
  check("確認で問題があれば、フォーカスは「直してもらう」へ", await fixBtn.evaluate(e => e === document.activeElement));
  await fixBtn.click(); await runBtn.click();
  await until("3 回目の実行", async () => await nRuns() === 3);
  await pf.waitForSelector("#ai-review-dialog .ai-check:not([hidden])", { timeout: 20000 });
  check("直してもらうのは 2 回まで", await fixBtn.count() === 0 && /Still not fixed after 2 tries/.test(await pf.locator("#ai-review-dialog .ai-check").innerText()));
  await pf.close();

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
