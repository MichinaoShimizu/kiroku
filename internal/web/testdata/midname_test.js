// i18n.js の midName（script_test.go の TestMidName。前に let ja = false と isJA = () => ja を置く）。文の途中に入れる指標の名前は、英語では頭を小文字に（固有名と略語はそのまま）。日本語は変えない
const eq = (got, want) => { if (got !== want){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
eq(midName("Estimated cost"), "estimated cost");
eq(midName("Active time"), "active time");
eq(midName("Tokens"), "tokens");
eq(midName("Kiro credits"), "Kiro credits");
eq(midName("Git commits"), "Git commits");
eq(midName("AI-edited lines"), "AI-edited lines");
eq(midName("PR"), "PR");
eq(midName("Claude Code sessions"), "Claude Code sessions");
ja = true;
eq(midName("Estimated cost"), "Estimated cost");
eq(midName("推定費用"), "推定費用");
