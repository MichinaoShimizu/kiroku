// fixture.py の履歴で作った kiroku の HTML から、日報・週報・月報のプロンプトを取り出して書き出す（/report-eval のスキル用）。
//   node tools/reportbench/prompts.mjs <kiroku.html> <出力先ディレクトリ>
// 出力: day.md・week.md・month.md・session.md（セッションの振り返り）と、大きさ（文字数・バイト数・トークンの目安）の sizes.json
// Playwright は tools/screenshots のものを使う（npm ci --ignore-scripts。Chromium は CHROMIUM か /opt/pw-browsers/chromium）
import { createRequire } from "module";
import { writeFileSync, mkdirSync, existsSync } from "fs";
import path from "path";
import { fileURLToPath } from "url";
const here = path.dirname(fileURLToPath(import.meta.url));
const { chromium } = createRequire(path.join(here, "..", "screenshots", "package.json"))("playwright");
const [html, out] = process.argv.slice(2);
const exe = process.env.CHROMIUM || (existsSync("/opt/pw-browsers/chromium") ? "/opt/pw-browsers/chromium" : undefined);
const b = await chromium.launch(exe ? { executablePath: exe } : {});
const p = await (await b.newContext({ timezoneId: "Asia/Tokyo" })).newPage();
const errors = []; p.on("pageerror", e => errors.push(String(e)));
await p.goto("file://" + path.resolve(html)); await p.waitForTimeout(500);
// 日報は 8/12（水）、週報は 8/10 の週、月報は 8 月、振り返りは 8/12 の date-fns のセッション（fixture.py と truth.json に合わせる）
const r = await p.evaluate(() => {
  const res = {};
  st.mode = "week"; st.week = new Date(2026, 7, 10); render();
  res.day = reportPrompt(null, false, new Date(2026, 7, 12));
  res.week = reportPrompt(period().S, false);
  st.mode = "month"; st.month = new Date(2026, 7, 1); render();
  res.month = reportPrompt(period().S, true);
  const s = DATA.find(x => x.branch === "chore/datefns-v4"), active = s.segs.reduce((t, [a, b]) => t + (b - a), 0) / 60;
  const w = s.waits.map(x => x[1]).sort((a, b) => a - b); // 詳細パネル（detail）と同じ数え方
  res.session = sessionPrompt(s, active, w.length ? w[Math.floor(w.length / 2)] : null);
  return res;
});
await b.close();
if (errors.length) { console.error(errors.join("\n")); process.exit(1); }
mkdirSync(out, { recursive: true });
const sizes = {};
for (const [k, v] of Object.entries(r)) {
  writeFileSync(path.join(out, `${k}.md`), v);
  // トークンの目安は 4 バイトで 1 トークン（英語の文。モデルによって違うので、比べるときは同じ数え方で）
  sizes[k] = { chars: [...v].length, bytes: Buffer.byteLength(v), approxTokens: Math.round(Buffer.byteLength(v) / 4) };
}
writeFileSync(path.join(out, "sizes.json"), JSON.stringify(sizes, null, 2) + "\n");
console.log(JSON.stringify(sizes));
