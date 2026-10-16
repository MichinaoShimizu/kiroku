// ai.js の aiOutputProblems と aiFixPrompt を Node で動かす（script_test.go の TestAIOutputCheck）
const eq = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(what + ": " + JSON.stringify(got) + " != " + JSON.stringify(want)); process.exitCode = 1; } };
const F = "```";
const prompt = ["Act as an advisor.", "", "# Format", "Write it.", "", F + "text", "## Weekly report: Aug 10 – Aug 16, 2026", "", "### Summary", "x", "", "### Numbers", "x", "",
  "### By project", "#### <project>", "", "### Expert observations", "x", "", "### Next actions", "x", F, "", "# History data", F + "text", "# Facts", "", "## Numbers",
  "| | This week | Last week |", "|---|---|---|", "| Active time | 2h 1m | 4m |", "| Git commits (by AI) | 7 (6) | 2 |", "",
  "- [fca3214b6b53](https://github.com/example/reports/commit/fca3214b6b5317fd44d4c2d34e3076f14e482e45) Stream rows", "- https://github.com/example/reports/pull/12", F].join("\n");
const good = ["## 週報: 2026年8月10日〜16日", "", "### 要約", "CSV を出せるようにした（PR https://github.com/example/reports/pull/12）。", "", "### 数字",
  "| | 今週 | 先週 |", "|---|---|---|", "| 稼働時間 | 2h 1m | 4m |", "| Git のコミット（AI） | 7 (6) | 2 |", "", "### プロジェクトごと", "#### `reports`", "- fca3214b6b53 で行を流すようにした", "",
  "### 専門家としての所見", "- よい", "", "### 次の一手", "- テストを足す。"].join("\n");
// 見出しを訳しても、表の 1 列目を訳しても、プロンプトにあるリンクとハッシュ（短いものも）なら、問題なし
eq(aiOutputProblems(prompt, good), [], "good");
eq(aiOutputProblems(prompt, good.replace("fca3214b6b53 で", "fca3214 で")), [], "short hash in prompt");
// 途中で切れた（節が足りない）
const cutOff = good.slice(0, good.indexOf("### 専門家"));
eq(aiOutputProblems(prompt, cutOff).map(p => p.l), ["2 of the 5 sections are missing"], "cut off");
eq(/has 3 of the 5 "###" sections/.test(aiOutputProblems(prompt, cutOff)[0].en), true, "cut off en");
// 数字の表の値が違う
const P2 = aiOutputProblems(prompt, good.replace("| 7 (6) | 2 |", "| 7 | 2 |"));
eq(P2.map(p => p.l), ["The numbers table differs from kiroku's in 1 row"], "numbers");
eq(P2[0].en.includes('"Git commits (by AI)"'), true, "numbers en names the row");
// 作ったリンクとハッシュ
const P3 = aiOutputProblems(prompt, good + "\n- https://github.com/example/reports/pull/13 と 9abc1234def0 も。");
eq(P3.map(p => p.l), ["Links that are not in the prompt: https://github.com/example/reports/pull/13", "Commit hashes that are not in the prompt: 9abc1234def0"], "invented");
// 数字だけ・文字だけの語はハッシュとみなさない
eq(aiOutputProblems(prompt, good + "\n1234567 decaded"), [], "not hashes");
// 「# Format」と数字の表がないプロンプトでは、見出しと表は見ない
eq(aiOutputProblems("Just a question", "An answer"), [], "no format");
// 直してもらうプロンプト: 元のプロンプトそのまま、前の返事は（中のバッククォートより長い）囲みの中、直すことの一覧
const fx = aiFixPrompt(prompt, cutOff + "\n" + F + "\ncode\n" + F, aiOutputProblems(prompt, cutOff));
eq(fx.startsWith(prompt + "\n\n# Your previous answer\n"), true, "fix keeps the prompt");
eq(fx.includes("\n````text\n"), true, "fix fences the answer");
eq(/\n# What to fix\n- The answer has 3 of the 5 "###" sections/.test(fx), true, "fix lists the problems");
