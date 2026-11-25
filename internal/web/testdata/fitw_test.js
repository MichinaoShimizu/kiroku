// format.js の dispW・fitW（script_test.go の TestFitW）。グラフの軸の名前を、文字の数でなく見た目の幅で切る
const eq = (got, want) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
eq(dispW("web-app"), 7);
eq(dispW("先週の同じ日数まで"), 18); // 漢字・かなは 2 つぶん
eq(dispW("ＡＢ"), 4); // 全角の英字も
eq(fitW("web-app", 10), "web-app");
eq(fitW("data-pipeline", 10), "data-pipe…");
eq(fitW("先週の同じ日数まで", 10), "先週の同…"); // 英字 10 字の幅に、漢字 4 字と「…」
eq(fitW("先週の同じ日数まで", 18), "先週の同じ日数まで");
eq(fitW("日本語-app", 6), "日本…");
eq(dispW("𠮷"), 2); // サロゲートペアの漢字も 1 文字で 2 つぶん（途中で切らない）
eq(fitW("𠮷𠮷𠮷𠮷", 6), "𠮷𠮷…");
