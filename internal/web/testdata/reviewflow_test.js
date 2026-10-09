// session.js の reviewFlow を、hm と FIXRE と一緒に取り出して Node で動かす（script_test.go の TestReviewFlow）。TZ=UTC で動かす
const eq = (got, want) => { if (got !== want){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
const T = 1767225600; // 2026-01-01 00:00 UTC
const prompts = [
  {t: T + 60, text: "Add a login form"},
  {t: T + 600, text: "No, that's wrong.\n  Use the existing  component"},
  {text: "no time"},
  {t: T + 1200, kind: "command", text: "/undo"},
  {t: T + 1800, text: "x".repeat(301)},
];
const ev = [
  {t: T, l: "Subagent", d: "Explore"},                 // 最初のプロンプトより前
  {t: T + 300, l: "Interrupted"},
  {t: T + 900, l: "AI committed", d: "Add login\nform " + "y".repeat(130)},
  {t: T + 1500, l: "Created a pull request"},
  {t: T + 2400, l: "Hit a usage limit"},
];
// 出来事がプロンプトの間に時刻の順で入り、言い直しらしいプロンプトに印が付く（コマンドには付けない）
let L = reviewFlow(prompts, ev, 40, 300);
eq(L[0], "- 00:00 [Subagent] Explore");
eq(L[1], "- 00:01 Prompt: Add a login form");
eq(L[2], "- 00:05 [Interrupted]");
eq(L[3], "- 00:10 Prompt (looks like a correction): No, that's wrong. Use the existing component");
eq(L[4], "- --:-- Prompt: no time");
eq(L[5], "- 00:15 [AI committed] Add login form " + "y".repeat(105) + "…");
eq(L[6], "- 00:20 Prompt: /undo");
eq(L[7], "- 00:25 [Created a pull request]");
eq(L[8], "- 00:30 Prompt: " + "x".repeat(300) + "…");
eq(L[9], "- 00:40 [Hit a usage limit]");
eq(L.length, 10);
// 最初の n 個だけ。そのあとのプロンプトより後の出来事は入れない
L = reviewFlow(prompts, ev, 2, 300);
eq(L.join("\n"), "- 00:00 [Subagent] Explore\n- 00:01 Prompt: Add a login form\n- 00:05 [Interrupted]\n- 00:10 Prompt (looks like a correction): No, that's wrong. Use the existing component\n- 00:15 [AI committed] Add login form " + "y".repeat(105) + "…");
eq(reviewFlow([], [], 40, 300).length, 0);
// 履歴ファイルを読んでもらうときは、プロンプトを目印になる頭だけに
eq(reviewFlow([{t: T, text: "z".repeat(100)}], [], 40, 80)[0], "- 00:00 Prompt: " + "z".repeat(80) + "…");
