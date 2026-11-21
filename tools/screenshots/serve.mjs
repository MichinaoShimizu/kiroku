// kiroku serve の鍵を、本物のブラウザで確かめる（CI の e2e）。
//   go build -o /tmp/kiroku . && python3 -I tools/screenshots/gen.py /tmp/gen && node tools/screenshots/serve.mjs /tmp/kiroku /tmp/gen
// ダミーの履歴（gen.py）だけを読む kiroku serve を、使い捨ての HOME と設定の場所で空いているポートに立て、
//   - kiroku open --print の使い捨てのコード（?code=）で開くと履歴が出て、どの URL にも鍵が載らない（ブラウザの履歴に鍵が残らない）。
//     鍵は中継のページの POST の答えでだけ受け取り、コードは 2 回目には使えない
//   - 以前の版の /?key= は、鍵が合っていても 403 で、履歴を出さず、鍵をしまわない（1.0 から）
//   - 鍵なしの / を開き直しても出る（localStorage の鍵）。履歴ファイルの「Open」は鍵を付けて読んで保存する
//   - 鍵のないブラウザ（新しいコンテキスト）は、案内だけで履歴を見ない。data.json も 403
//   - 画面から同じコンピューターのほかのポートのサーバーへ移っても、鍵はどの問い合わせにも載らない（Cookie にも、ほかのところにも）
//   - 「Analyze with AI」: 既定はオフ（--ai を付けたときだけ）。--ai で出て、押すと送るプロンプトをそのまま見せる（Run は押さない）。エージェントのコマンドがなければ押せず理由を添え、
//     --no-ai では「(off)」だけで /ai/run は 404。エージェントは偽の codex（何もしないスクリプト）を PATH の先頭に置いて決める
//   - 偽の codex での実行: 返事の確認と「直してもらう」、届いた分から見える返事、エージェントの失敗、Cancel で止めるとエージェントとその子が残らない
// を見る。Playwright が必要（smoke.mjs と同じ）。
import { chromium } from "playwright";
import { execFileSync, spawn } from "node:child_process";
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
const serveArgs = ["serve", "127.0.0.1:0", "--no-open", "--sources", "claude", "--claude-root", path.join(home, ".claude", "projects"), "--archive-dir", path.join(work, "archive"), "--ai"]; // 「Analyze with AI」は --ai を付けたときだけ（既定はオフ）
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
  check("起動の 1 行で、--ai で AI の実行がオンになっていて、提供元に送りうると言う", /"Analyze with AI" is on \(--ai or KIROKU_AI\): .*may send the prompt and the history it reads to its provider; start without them to turn it off/.test(log), log);
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

  // 「Analyze with AI」: フラグなしで出る。押すと送るプロンプトをそのまま見せ、提供元の一文はダイアログに出さない。Run は押さない
  await p.goto(`${base}/`);
  await p.waitForSelector(".head .dayrun", { timeout: 15000 });
  check("週報の「Analyze with AI」はない（日報とセッションの振り返りだけ）", await p.locator("#rptbar, #rptrun").count() === 0);
  const dayBtn = p.locator(".head .dayrun").first();
  check("フラグなしの serve で、記録のある日の見出しに日報の「Analyze with AI」が押せる", await dayBtn.count() === 1 && await dayBtn.isEnabled());
  check("バッジ（点だけ）の名前が LIVE · AI on", ((await p.locator("#live").getAttribute("aria-label")) || "").includes("AI on") && await p.locator("#live").evaluate(e => e.classList.contains("ai")));
  await p.locator("#live").click(); await p.waitForTimeout(200);
  check("バッジを押すと、説明（--ai で入れたこと）が出る", /--ai/.test(await p.locator("#toast").innerText()));
  await dayBtn.click();
  await p.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  const dlg = await p.evaluate(() => { const d = document.querySelector("#ai-review-dialog"); return { text: d.innerText, prompt: d.querySelector("textarea.ai-prompt").value, readOnly: d.querySelector("textarea.ai-prompt").readOnly, agents: [...d.querySelectorAll("select option")].map(o => o.value) }; });
  check("ダイアログは送るプロンプトをそのまま（読み取り専用で）見せる", dlg.readOnly && /daily report/.test(dlg.prompt) && /Check the exact prompt below/.test(dlg.text));
  check("ダイアログに提供元の一文はない（ドキュメントと起動の 1 行にある）", !/provider/.test(dlg.text));
  check("エージェントはこのコンピューターにあるものだけ", dlg.agents.includes("codex"), JSON.stringify(dlg.agents));
  await p.locator("#ai-review-dialog button", { hasText: "Close" }).click();
  check("Close で閉じ、何も動かさない", await p.locator("#ai-review-dialog").count() === 0 && !/\$ codex/.test(log));
  // 日報のプロンプトは、その日の形式と前日と並べた数字の表
  await p.evaluate(() => scrollTo(0, 400)); await p.waitForTimeout(150);
  const before = await p.evaluate(() => scrollY); await dayBtn.focus(); await p.waitForTimeout(150);
  check("日報のボタンにフォーカスしても、ページの位置が変わらない", await p.evaluate(() => scrollY) === before);
  await dayBtn.click();
  await p.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  const dp = await p.evaluate(() => document.querySelector("#ai-review-dialog textarea.ai-prompt").value);
  check("日報のプロンプトは、その日の形式と前日との数字の表", /## Daily report: /.test(dp) && /^\| \| This day \| Previous day \|$/m.test(dp) && /^\| Active time \| /m.test(dp), dp.slice(0, 300));
  await p.locator("#ai-review-dialog button", { hasText: "Close" }).click();
  check("日報も Close で閉じ、何も動かさない", await p.locator("#ai-review-dialog").count() === 0 && !/\$ codex/.test(log));
  // 月報はない。月の表示では報告の帯もボタンも出さない
  await p.click("#mode button[data-v=month]");
  await p.waitForFunction(() => document.querySelector("#mode button[data-v=month]").getAttribute("aria-pressed") === "true");
  check("月の表示には、報告の「Analyze with AI」がない", await p.locator(".dayrun, #rptrun").count() === 0);
  await p.click("#mode button[data-v=week]");
  await p.waitForSelector(".head .dayrun", { timeout: 5000 });

  // エージェントのコマンドが 1 つもない serve と、--no-ai の serve を、同じ鍵（同じ設定の場所）で立てる
  const startExtra = async (args, env2) => { let lg = ""; const s = spawn(path.resolve(bin), args, { env: { ...env, ...env2 }, stdio: ["ignore", "pipe", "pipe"] });
    extra.push(s); s.stdout.on("data", d => { lg += d; }); s.stderr.on("data", d => { lg += d; });
    const pt = await until("待ち受け（追加）", () => (lg.match(/serving http:\/\/localhost:(\d+)\//) || [])[1]);
    await until("読み込み（追加）", async () => (await fetch(`http://localhost:${pt}/stamp`, { headers: { "X-Kiroku-Key": key } }).catch(() => ({ ok: false }))).ok);
    return { url: `http://localhost:${pt}`, log: () => lg }; };
  const none = await startExtra(serveArgs, { PATH: emptyBin });
  const pn = await ctx.newPage();
  await openWithKey(pn, none.url);
  await pn.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("エージェントがないと、日報はプロンプトのコピーになる", await pn.locator(".head .dayrun.ai-copy").count() > 0 && await pn.locator(".ai-action").count() === 0);
  await pn.evaluate(() => select(DATA.find(s => s.prompts.length).id)); await pn.waitForSelector("#panel #srun.ai-copy", { timeout: 5000 });
  { const r = await pn.evaluate(async () => { window.__copied = null; navigator.clipboard.writeText = t => { window.__copied = t; return Promise.resolve(); };
      document.querySelector("#panel #srun").click(); await new Promise(z => setTimeout(z, 300));
      return {got: window.__copied || "", toast: document.querySelector("#toast").textContent, dialog: !!document.getElementById("ai-review-dialog")}; });
    check("エージェントがないと、振り返りはプロンプトをコピーするだけ（ダイアログも実行もない）", /History data|history file/i.test(r.got) && !r.dialog && /--ai/.test(r.toast), JSON.stringify({...r, got: r.got.slice(0, 60)})); }
  await pn.close();
  // 既定（--ai なし）はオフ: 起動の 1 行がそう言い、/ai/run はなく、日報のボタンも出ない
  const dflt = await startExtra(serveArgs.filter(a => a !== "--ai"), {});
  check("--ai を付けなければ、起動の 1 行はオフと言い、--ai でオンにできると言う", /"Analyze with AI" is off: .*start with --ai/.test(dflt.log()), dflt.log());
  const pz = await ctx.newPage();
  await openWithKey(pz, dflt.url);
  await pz.waitForSelector(".run[data-sid]", { timeout: 15000 });
  check("--ai を付けなければ、AI を動かすボタンはなく、日報はプロンプトのコピーになる", await pz.locator(".ai-action").count() === 0 && await pz.locator(".head .dayrun.ai-copy").count() > 0 && !((await pz.locator("#live").getAttribute("aria-label")) || "").includes("AI on"));
  const sz = await pz.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("--ai を付けなければ、鍵があっても /ai/run は 404", sz === 404, String(sz));
  await pz.close();
  const off = await startExtra([...serveArgs, "--no-ai"], {});
  check("--ai と --no-ai の両方なら、--no-ai が勝ってオフ", /"Analyze with AI" is off: /.test(off.log()), off.log());
  const po = await ctx.newPage();
  await openWithKey(po, off.url);
  await po.waitForSelector(".run[data-sid]", { timeout: 15000 });
  await po.evaluate(() => select(DATA.find(s => s.prompts.length).id)); await po.waitForSelector("#panel #srun.ai-copy", { timeout: 5000 });
  check("--no-ai では、振り返りと日報はプロンプトのコピーだけ", await po.locator(".ai-action").count() === 0 && await po.locator(".head .dayrun.ai-copy").count() > 0);
  const st = await po.evaluate(async k => (await fetch("ai/run", { method: "POST", headers: { "X-Kiroku-Key": k, "X-Kiroku": "1", "Content-Type": "application/json" }, body: JSON.stringify({ agent: "codex", prompt: "x", consent: true }) })).status, key);
  check("--no-ai では、鍵があっても /ai/run は 404", st === 404, String(st));
  await po.close();

  // 偽の codex は、本物と同じく --help（kiroku が使えるオプションを確かめる）にはすぐ答え、実行には数えない
  // 返事の確認と「直してもらう」: 偽の codex が、節の足りない週報を返す。kiroku が足りないところを見せ、「直してもらう」は
  // 直してもらうプロンプトを見せるだけで送らない。Run で送り、直してもらうのは 2 回まで
  const fixBin = path.join(work, "fixbin"), runs = path.join(work, "runs.txt");
  await mkdir(fixBin, { recursive: true });
  await writeFile(path.join(fixBin, "codex"), `#!/bin/sh\ncase " $* " in *" --help "*) exit 0;; esac\ncat > "${path.join(work, "last-prompt.txt")}"\necho run >> "${runs}"\nprintf '## Daily report\\n\\n### Summary\\nDone.\\n'\n`);
  await chmod(path.join(fixBin, "codex"), 0o700);
  const fx = await startExtra(serveArgs, { PATH: `${fixBin}${path.delimiter}${process.env.PATH || ""}` });
  const nRuns = async () => (await readFile(runs, "utf8").catch(() => "")).split("\n").filter(Boolean).length;
  const pf = await ctx.newPage();
  await openWithKey(pf, fx.url);
  await pf.waitForSelector(".head .dayrun", { timeout: 15000 });
  await pf.locator(".head .dayrun").first().click();
  await pf.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  const runBtn = pf.locator("#ai-review-dialog .ai-action"), fixBtn = pf.locator("#ai-review-dialog .ai-check button", { hasText: "Ask to fix" });
  await pf.locator("#ai-review-dialog select").selectOption("codex"); // このコンピューターに本物のエージェントがあっても、偽の codex を使う
  await runBtn.click();
  await pf.waitForSelector("#ai-review-dialog .ai-check:not([hidden])", { timeout: 20000 });
  check("節の足りない返事には、kiroku の確認が足りないところを見せる", /4 of the 5 sections are missing/.test(await pf.locator("#ai-review-dialog .ai-check").innerText()));
  await fixBtn.click();
  const fixPrompt = await pf.evaluate(() => document.querySelector("#ai-review-dialog textarea.ai-prompt").value);
  check("「直してもらう」は、元のプロンプト・前の返事・直すことを入れたプロンプトを見せる", /daily report/.test(fixPrompt) && /\n# Your previous answer\n/.test(fixPrompt) && /\n# What to fix\n- The answer has 1 of the 5 "###" sections/.test(fixPrompt), fixPrompt.slice(-400));
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

  // 実行の途中・失敗・中止: 偽の codex は mode のファイルで振る舞いを変える
  //   stream: 返事の 1 行目を書き、go のファイルができるまで待ってから残りを書く（届いた分から見えるか）
  //   fail: エラー出力に 1 行書いて 3 で終わる（理由が経過に出て、状態の 1 行が失敗を言うか）
  //   hang: 子のプロセスを起こして待ち続ける（Cancel で止まり、エージェントもその子も残らないか）
  const sBin = path.join(work, "sbin"), modeF = path.join(work, "mode"), goF = path.join(work, "go"), pidF = path.join(work, "pids");
  await mkdir(sBin, { recursive: true });
  await writeFile(path.join(sBin, "codex"), `#!/bin/sh
case " $* " in *" --help "*) exit 0;; esac
cat >/dev/null
case "$(cat "${modeF}")" in
stream) printf '## Daily report\\n\\n'; while [ ! -e "${goF}" ]; do sleep 0.1; done; printf '### Summary\\nAll done.\\n' ;;
fail) echo "boom: the agent could not start" >&2; exit 3 ;;
hang) echo started; sleep 60 & echo "$$ $!" > "${pidF}"; wait ;;
esac
`);
  await chmod(path.join(sBin, "codex"), 0o700);
  const sx = await startExtra(serveArgs, { PATH: `${sBin}${path.delimiter}${process.env.PATH || ""}` });
  const ps = await ctx.newPage();
  await openWithKey(ps, sx.url);
  await ps.waitForSelector(".head .dayrun", { timeout: 15000 });
  await ps.locator(".head .dayrun").first().click();
  await ps.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  await ps.locator("#ai-review-dialog select").selectOption("codex");
  const sRun = ps.locator("#ai-review-dialog .ai-action"), sStatus = ps.locator("#ai-review-dialog [role=status]"), sOut = ps.locator("#ai-review-dialog .ai-out");
  const sCancel = ps.locator("#ai-review-dialog button", { hasText: /^(Cancel|Close)$/ });
  await writeFile(modeF, "stream");
  await sRun.click();
  await until("返事の 1 行目", async () => /Daily report/.test(await sOut.innerText().catch(() => "")));
  check("返事は届いた分から見え、終わるまでは実行中と言う", /Running Codex/.test(await sStatus.innerText()) && await sRun.isDisabled() && await sCancel.innerText() === "Cancel");
  await writeFile(goF, "");
  await until("実行の終わり", async () => /Completed with Codex/.test(await sStatus.innerText()));
  check("終わると、返事の全部と「もう一度実行」", /All done\./.test(await sOut.innerText()) && await sRun.innerText() === "Run again" && await sRun.isEnabled());
  await writeFile(modeF, "fail");
  await sRun.click();
  await until("失敗", async () => /stopped with an error/.test(await sStatus.innerText()));
  check("エージェントが失敗すると、理由は経過に出て、状態の 1 行は失敗を言う", /boom: the agent could not start/.test(await ps.locator("#ai-review-dialog .ai-log").innerText()) && /Codex stopped with an error\. Its output is shown above\./.test(await sStatus.innerText()));
  check("失敗のあとも、もう一度実行できる", await sRun.isEnabled() && await sRun.innerText() === "Run again");
  await writeFile(modeF, "hang");
  await sRun.click();
  const pids = (await until("エージェントの PID", () => readFile(pidF, "utf8").catch(() => ""))).trim().split(/\s+/).map(Number);
  await sCancel.click();
  await until("中止", async () => /^Canceled$/.test((await sStatus.innerText()).trim()));
  check("Cancel で止まり、ダイアログは開いたまま、もう一度実行できる", await ps.locator("#ai-review-dialog").count() === 1 && await sRun.isEnabled() && await sCancel.innerText() === "Close");
  // ゾンビ（親が待つ前の終わったプロセス）は残っていないと数える
  const alive = async pid => { try { process.kill(pid, 0); } catch { return false; } const st = await readFile(`/proc/${pid}/stat`, "utf8").catch(() => null); return st === null ? process.platform !== "linux" : !/^\d+ \(.*\) Z/.test(st); };
  let left = pids;
  await until("エージェントとその子が終わる", async () => { left = []; for (const pid of pids) if (await alive(pid)) left.push(pid); return left.length === 0; }, 10000).catch(() => {});
  check("Cancel のあと、エージェントもその子も残らない", pids.length === 2 && left.length === 0, `pids ${pids}, left ${left}`);
  await ps.close();

  // エージェントの既定: claude と codex の両方があるとき、セッションの振り返りはそのセッションのエージェント、
  // 日報はその日にいちばん長く動いていたエージェントを選んでおく（一覧の先頭の Claude Code ではなく）
  const dBin = path.join(work, "dbin");
  await mkdir(dBin, { recursive: true });
  for (const n of ["claude", "codex"]){ await writeFile(path.join(dBin, n), "#!/bin/sh\ncat >/dev/null\n"); await chmod(path.join(dBin, n), 0o700); }
  const dx = await startExtra([...serveArgs.slice(0, 4), "claude,codex", ...serveArgs.slice(5), "--codex-home", path.join(home, ".codex")], { PATH: `${dBin}${path.delimiter}${process.env.PATH || ""}` });
  const pd = await ctx.newPage();
  await openWithKey(pd, dx.url);
  await pd.waitForSelector(".run[data-sid]", { timeout: 15000 });
  const cx = await pd.evaluate(() => { const s = DATA.find(x => x.source === "Codex"); if (!s) return null; st.mode = "week"; st.week = mondayOf(new Date(s.start * 1000)); select(s.id); return s.id; });
  check("ダミーの履歴に Codex のセッションがある（下の確かめのため）", !!cx);
  await pd.waitForSelector("#srun", { timeout: 5000 });
  await pd.locator("#srun").click();
  await pd.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  check("Codex のセッションの振り返りは、Codex を選んでおく", await pd.locator("#ai-review-dialog select").inputValue() === "codex");
  // ダイアログのあいだのキーは、うしろの画面を動かさない: ← で週が変わらず、Escape はダイアログだけを閉じる（詳細は開いたまま）
  const wk0 = await pd.evaluate(() => st.week.getTime());
  await pd.locator("#ai-review-dialog button", { hasText: "Close" }).focus();
  await pd.keyboard.press("ArrowLeft"); await pd.keyboard.press("m");
  await pd.keyboard.press("Escape");
  check("AI 分析のダイアログのキーは、うしろの画面を動かさない", !(await pd.evaluate(() => document.getElementById("ai-review-dialog")?.open)) &&
    await pd.evaluate(() => [st.sel, st.mode, st.week.getTime()]).then(([sel, mode, w]) => sel === cx && mode === "week" && w === wk0));
  await pd.keyboard.press("Escape");
  await pd.waitForSelector(".head .dayrun", { timeout: 5000 });
  const want = await pd.evaluate(() => { const dd = addDays(st.week, +document.querySelector(".head .dayrun").dataset.d); return aiAgentIn(dd.getTime() / 1000, addDays(dd, 1).getTime() / 1000); });
  await pd.locator(".head .dayrun").first().click();
  await pd.waitForSelector("#ai-review-dialog", { timeout: 5000 });
  check("日報は、その日にいちばん長く動いていたエージェントを選んでおく", ["claude", "codex"].includes(want) && await pd.locator("#ai-review-dialog select").inputValue() === want, want);
  await pd.locator("#ai-review-dialog button", { hasText: "Close" }).click();
  check("既定を選んでも、何も動かさない", !/\$ (claude|codex)/.test(dx.log()));
  await pd.close();

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
