/* records と unrecorded（state.js）の確かめ。記録できないものを「0」と読ませないこと。
   state.js はほかの値に頼るので、その 4 行だけを取り出して、RECORDS と DATA を差し替えて動かす（script_test.go） */
const eq = (got, want, msg) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(`${msg}: ${JSON.stringify(got)} ではなく ${JSON.stringify(want)} のはず`); process.exitCode = 1; } };
const ses = (source, start) => ({source, start, end: start + 60});
eq(records("Claude Code", "limits"), true, "Claude Code は上限を記録する");
eq(records("Kiro IDE / CLI", "limits"), false, "Kiro IDE は上限を記録しない");
eq(records("Unknown agent", "files"), false, "表にないエージェントは記録しないとみなす");
DATA = [ses("Kiro IDE / CLI", 100), ses("Kiro CLI (Classic)", 200), ses("Kiro IDE / CLI", 300)];
eq(unrecorded(0, 1000, "limits"), ["Kiro IDE / CLI", "Kiro CLI (Classic)"], "どれも記録しないなら、そのエージェントの名前（重ねない）");
eq(unrecorded(0, 1000, "files"), null, "どれかが記録するなら null（0 を出す）");
eq(unrecorded(5000, 6000, "limits"), null, "期間にセッションがなければ null");
DATA.push(ses("Claude Code", 400));
eq(unrecorded(0, 1000, "limits"), null, "記録するエージェントが混ざれば null");
eq(unrecorded(0, 150, "limits"), ["Kiro IDE / CLI"], "期間の外のセッションは見ない");
eq(notRec(["Kiro IDE / CLI", "Kiro CLI (Classic)"]), "Not recorded in Kiro IDE / CLI, Kiro CLI (Classic) history", "言い方");
