/* keepRow（panels.js）の確かめ。0 日の保存期間（Kiro Crew の session.archive_retention_days: 0。now）を、
   「わからない期間のあとで消える」（days の 0）や「0 日残す」と出さないこと。
   panels.js はほかのファイルに頼るので、keepRow だけを取り出し、使う関数をここで用意して動かす（script_test.go） */
let META = {archive: {on: false}};
const LIVE = false;
const esc = s => String(s).replace(/[&<>"']/g, c => ({"&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"}[c]));
const ext = (u, t) => `<a href="${esc(u)}">${t}</a>`;
const plural = (n, w) => `${n} ${w}${n === 1 ? "" : "s"}`;
const dMDY = d => d.toISOString().slice(0, 10);
const archOn = () => !!(META.archive && META.archive.on);
const has = (got, want, msg) => { if (!got.includes(want)){ console.error(`${msg}: ${got} に ${want} がない`); process.exitCode = 1; } };
const not = (got, bad, msg) => { if (got.includes(bad)){ console.error(`${msg}: ${got} に ${bad} がある`); process.exitCode = 1; } };
const zero = {days: 0, now: true, set: true, who: "Kiro Crew", setting: "session.archive_retention_days", docs: "https://docs"};
let row = keepRow({name: "Kiro CLI", retention: zero});
has(row, "Older records are deleted at the next cleanup (<code>session.archive_retention_days</code> is 0)", "0 日は次の片付けで消える");
has(row, `class="kw"`, "0 日は注意として出す");
not(row, "after a period", "0 日を「わからない期間」と出さない");
not(row, "Kept for 0 days", "0 日を「0 日残す」と出さない");
META.archive.on = true;
row = keepRow({name: "Kiro CLI", retention: zero});
has(row, "but kiroku keeps a copy", "kiroku archive がオンなら、コピーがあると出す");
not(row, `class="kw"`, "コピーがあれば注意にしない");
META.archive.on = false;
has(keepRow({name: "Kiro CLI", retention: {days: 90, set: true, setting: "s"}}), "Kept for 90 days", "設定した日数");
has(keepRow({name: "X", retention: {days: 0, set: false, setting: "s", docs: "d"}}), "Older records are deleted after a period", "日数がわからないとき");
has(keepRow({name: "X", retention: {days: 30, set: false, setting: "<s>", docs: "d"}}), "<code>&lt;s&gt;</code>", "設定の名前は打ち消す");

// 既定のままで消える履歴には、2 つの残し方と、バックアップ先のフォルダ（文字のまま）を添える
META.archive = {on: false, dir: "/Users/me/<b>archive</b>"};
row = keepRow({name: "Claude Code", retention: {days: 30, setting: "cleanupPeriodDays", file: "~/.claude/settings.json"}});
has(row, "<code>&quot;cleanupPeriodDays&quot;: 3650</code> in <code>~/.claude/settings.json</code>", "設定とファイルを出す");
has(row, "back them up to <code>/Users/me/&lt;b&gt;archive&lt;/b&gt;</code>", "バックアップ先を文字のまま出す");
has(row, "run <code>kiroku archive on</code> to let kiroku back them up", "HTML ではコマンドを出す");
not(row, "<b>", "フォルダの名前を HTML にしない");
