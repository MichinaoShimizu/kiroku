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
eq(P2.map(p => p.l), ["The numbers table differs from kiroku's in: Git commits (by AI)"], "numbers");
// 表がまるごとない
eq(aiOutputProblems(prompt, good.replace(/^\|.*\n/gm, "")).map(p => p.l), ["The numbers table is missing"], "numbers missing");
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
// 落とされないこと: バッククォートの並びがとても多い返事（mdFence が引数を広げて落ちていた）
const many = "## x\n### Summary\n" + "` ".repeat(200000);
eq(aiFixPrompt(prompt, many, aiOutputProblems(prompt, many)).length > many.length, true, "many backticks");
// 強調で囲んだリンク、UUID、色、短い形がプロンプトにある長いハッシュは、作り話とみなさない
eq(aiOutputProblems(prompt, good + "\n**https://github.com/example/reports/pull/12** _https://github.com/example/reports/pull/12_ 123e4567-e89b-12d3-a456-426614174000 #1a2b3c4d fca3214b6b5317fd44d4c2d34e3076f14e482e45"), [], "not invented");
// コードブロックの中の ### は節に数えない
eq(aiOutputProblems(prompt, cutOff + "\n" + F + "\n### a\n### b\n" + F).map(p => p.l), ["2 of the 5 sections are missing"], "### in code");
// 表の値の「−」と「-」、強調、引用の中の表は同じとみなす
eq(aiOutputProblems(prompt, good.replace("| 稼働時間 | 2h 1m | 4m |", "> | 稼働時間 | **2h 1m** | 4m |")), [], "table variants");
eq(aiOutputProblems(prompt.replace("| 7 (6) | 2 |", "| +82 −0 | 2 |"), good.replace("| 7 (6) | 2 |", "| +82 -0 | 2 |")), [], "minus sign");
// 長いリンクは、画面の一覧で切る（英語の直すことは 200 字まで）。空白は入らない
const longU = "https://evil.example/" + "a".repeat(5000);
const PL = aiOutputProblems(prompt, good + "\n" + longU + " x");
eq(PL[0].l.length < 200 && PL[0].en.length < 400 && !/\s/.test(PL[0].en.split(": ")[1]), true, "long url cut");
