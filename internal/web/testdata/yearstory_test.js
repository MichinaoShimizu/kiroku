/* 1 年の露光の「見どころ」（highlights）と「前半と後半」（halves）の確かめ（year.js）。
   year.js は画面の DOM を触るので、2 つの関数だけを取り出し、使う関数をここで用意して動かす（script_test.go）。
   境界の値で、出る・出ないが決まりどおりかを見る */
const MON = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
const dMD = d => `${MON[d.getMonth()]} ${d.getDate()}`;
const addDays = (d, n) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + n);
const plural = (n, w) => `${n} ${w}${n === 1 ? "" : "s"}`;
const key = d => `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}-${String(d.getDate()).padStart(2,"0")}`;
const eq = (got, want, msg) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(`${msg}: ${JSON.stringify(got)}（期待 ${JSON.stringify(want)}）`); process.exitCode = 1; } };
const day0 = new Date(2025, 0, 1);
const texts = H => H.map(h => h.t);
const days = (...ds) => new Set(ds.map(d => key(new Date(2025, 0, d))));

// いちばん動いていた月：記録が 8 週以上・3 か月以上あり、ほかの月の平均より 3 割以上多いときだけ
const m3 = (a, b, c) => [{start: "2025-01-01", active: a}, {start: "2025-02-01", active: b}, {start: "2025-03-01", active: c}];
eq(texts(highlights({late: 0, weekend: 0}, m3(600, 780, 600), new Set(), {}, day0)), ["Busiest month: Feb · 13h"], "3 割多ければ出す");
eq(texts(highlights({late: 0, weekend: 0}, m3(600, 779, 600), new Set(), {}, day0)), [], "3 割に届かなければ出さない");
eq(texts(highlights({late: 0, weekend: 0}, m3(600, 780, 600).slice(0, 2), new Set(), {}, day0)), [], "2 か月だけなら出さない");
eq(texts(highlights({late: 0, weekend: 0, short: true}, m3(600, 780, 600), new Set(), {}, day0)), [], "記録が 8 週に満たなければ出さない");
eq(texts(highlights({late: 0, weekend: 0}, m3(100, 590, 100), new Set(), {}, day0)), [], "10 時間に満たない月は、目立っても出さない");

// いちばん長い休み：7 日以上だけ。同じ月の中なら「Jan 3–9」。画像の文には日付を書かない
eq(texts(highlights({late: 0, weekend: 0}, [], days(2, 10), {}, day0)), ["Longest stretch without AI: 7 days (Jan 3–9)"], "7 日の休みは出す");
eq(texts(highlights({late: 0, weekend: 0}, [], days(2, 9), {}, day0)), [], "6 日の休みは出さない（週末だけの人の平日など）");
eq(texts(highlights({late: 0, weekend: 0}, [], days(2, 10, 30), {}, day0)), ["Longest stretch without AI: 19 days (Jan 11–29)"], "長いほうの休みを選ぶ");
eq(highlights({late: 0, weekend: 0}, [], days(2, 10, 30), {}, day0)[0].c0, 10, "休みの始まりの列（Jan 11 は 10 列目）");

// いちばん長い連続：動いた日が 14 日以上。土日の空きは途切れにしない（2025-01-04・05、11・12、18・19 は土日）
const span = (a, b) => days(...Array.from({length: b - a + 1}, (_, i) => a + i));
const weekdays = (a, b) => days(...Array.from({length: b - a + 1}, (_, i) => a + i).filter(d => ![0, 6].includes(new Date(2025, 0, d).getDay())));
eq(texts(highlights({late: 0, weekend: 0}, [], span(1, 14), {}, day0)), ["Most consecutive active days: 14 (a weekend off doesn't break a run)"], "14 日続けば出す");
eq(texts(highlights({late: 0, weekend: 0}, [], span(1, 13), {}, day0)), [], "13 日なら出さない");
eq(texts(highlights({late: 0, weekend: 0}, [], weekdays(1, 20), {}, day0)), ["Most consecutive active days: 14 (a weekend off doesn't break a run)"], "平日だけ 14 日続けば出す（土日の空きで途切れない）");
eq(texts(highlights({late: 0, weekend: 0}, [], new Set([...weekdays(1, 20)].filter(k => k !== key(new Date(2025, 0, 8)))), {}, day0)), [], "平日を 1 日休めば途切れる");
eq(texts(highlights({late: 0, weekend: 0}, [], new Set([...span(1, 14), ...span(22, 23)]), {}, day0)), ["Most consecutive active days: 14 (a weekend off doesn't break a run)", "Longest stretch without AI: 7 days (Jan 15–21)"], "連続と休みは両方出す");

// 途中から使い始めたエージェント：最初の記録から 2 週間より後
const t0 = new Date(2025, 0, 1, 10)/1000;
eq(texts(highlights({late: 0, weekend: 0}, [], new Set(), {"Claude Code": t0, "Codex": t0 + 15*86400}, day0)), ["First Codex: Jan 16"], "15 日後に始めたエージェントは出す");
eq(texts(highlights({late: 0, weekend: 0}, [], new Set(), {"Claude Code": t0, "Codex": t0 + 13*86400}, day0)), [], "2 週間以内なら出さない");

// 深夜・週末：10% 以上だけ
eq(texts(highlights({late: 10, weekend: 9.9}, [], new Set(), {}, day0)), ["Late nights (22:00–5:00): 10% of active time"], "深夜 10% は出し、週末 9.9% は出さない");

// 前半と後半
const ses = (dayN, h, mins, prompts) => { const a = new Date(2025, 0, 1 + dayN, h)/1000; return {start: a, end: a + mins*60, nPrompts: prompts, segs: [[a, a + mins*60, 10]]}; };
const run = (n, from, mins, prompts, h = 10) => Array.from({length: n}, (_, i) => ses(from + i, h, mins, prompts));
eq(halves(run(10, 0, 30, 5)), [], "8 週に満たなければ比べない");
eq(halves([...run(10, 0, 40, 5), ...run(10, 45, 50, 5)]), [], "54 日でも比べない（8 週に満たない）");
eq(halves([...run(4, 0, 30, 5), ...run(10, 60, 60, 5)]), [], "片方が 5 セッションに満たなければ比べない");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 60, 50, 5)])), ["sessions 25% longer"], "25% 長くなれば出す");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 60, 47, 5)])), [], "18% なら出さない");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 60, 40, 7)])), ["prompts per session 5.0 → 7.0"], "依頼の数が 4 割増えれば出す");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 60, 40, 5, 23)])), ["late nights (22:00–5:00) 0% → 100%"], "深夜の割合が 5 ポイント以上動けば出す");
const par = [...run(10, 0, 60, 5), ...run(10, 60, 60, 5), ...run(10, 60, 60, 5).map(s => ({...s, start: s.start + 1800, end: s.end + 1800, segs: [[s.segs[0][0] + 1800, s.segs[0][1] + 1800, 10]]}))];
const pt = halves(par).find(h => h.k === "par");
eq(pt && pt.t, "2+ sessions at once 0% → 33%", "並列は、重なった時間 ÷ どれかが動いていた時間");
eq(!!(pt && pt.q), true, "変化には画面だけの問いかけを付ける");
const three = [...run(10, 0, 60, 5), ...[0, 1, 2].flatMap(k => run(10, 60, 60, 5).map(s => ({...s, start: s.start + k*600, end: s.end + k*600, segs: [[s.segs[0][0] + k*600, s.segs[0][1] + k*600, 10]]})))];
const p3 = halves(three).find(h => h.k === "par");
eq(p3 && p3.t, "2+ sessions at once 0% → 75%", "3 つ同時でも 100% を超えない（0・10・20 分に始まる 60 分：2 つ以上が動いていた 60 分 ÷ 80 分）");

// 画像の文：休みは日付を書かず、選んだときだけ載せる。連続は画像に載せない。深夜・週末は選んだときだけ
const hb = highlights({late: 30, weekend: 20}, [], new Set([...span(1, 14), ...span(22, 23)]), {}, day0);
eq(hb.find(h => h.k === "break").img, "Longest stretch without AI: 7 days", "画像の休みには日付を書かない");
eq(!!hb.find(h => h.k === "streak").img, false, "連続は画像に載せない");
eq(cardMarks({hi: hb}, {breaks: false}).map(h => h.k), [], "休みは選ばなければ画像に載せない");
eq(cardMarks({hi: hb}, {breaks: true}).map(h => h.k), ["break"], "選べば載せる");
eq(sideLines({hi: hb, halves: []}, {late: false}), [], "深夜・週末は選ばなければ画像に載せない");
eq(sideLines({hi: hb, halves: []}, {late: true}), ["Late nights (22:00–5:00): 30% of active time", "Weekends: 20% of active time"], "選べば載せる");
eq(sideLines({hi: [], halves: [{k: "late", t: "late nights (22:00–5:00) 0% → 100%"}, {k: "par", t: "2+ sessions at once 0% → 33%"}]}, {late: false}), ["2nd half vs 1st: 2+ sessions at once 0% → 33%"], "深夜の変化も、選ばなければ載せない");

// 光の列：朝 6 時から翌朝 6 時。大晦日の夜（1/1 の 0〜6 時）は前の年の最後の列に写り、どの年からも消えない
const sec = (...a) => new Date(...a)/1000;
const nye = plateCols(sec(2024, 11, 31, 23), sec(2025, 0, 1, 2), 2024);
eq(nye.map(c => [c[0], +c[1].toFixed(4), +c[2].toFixed(4)]), [[365, +(17/24).toFixed(4), +(20/24).toFixed(4)]], "大晦日 23 時〜元日 2 時は、2024 年の最後の列（366 日目）に 1 本で写る");
eq(plateCols(sec(2024, 11, 31, 23), sec(2025, 0, 1, 2), 2025), [], "同じ区間は 2025 年には写らない（二重に描かない）");
eq(plateCols(sec(2025, 0, 1, 3), sec(2025, 0, 1, 4), 2024).map(c => c[0]), [365], "元日 3 時の作業も、前の年の大晦日の列に写る");
eq(plateCols(sec(2025, 2, 1, 5), sec(2025, 2, 1, 7), 2025).map(c => c[0]), [58, 59], "朝 6 時をまたぐ作業は、前の日と当日の 2 列に分かれる");
eq(plateCols(sec(2025, 11, 31, 23), sec(2026, 0, 1, 7), 2025).map(c => c[0]), [364], "年の最後の列は、翌年の元日 6 時まで");

// 途中から使い始めたエージェント：すべての記録の最初（t0）と比べる。印の列は光と同じく朝 6 時で区切る
eq(texts(highlights({late: 0, weekend: 0}, [], new Set(), {"Codex": new Date(2025, 2, 1, 10)/1000}, day0, new Date(2024, 5, 1)/1000)), ["First Codex: Mar 1"], "前の年から記録があっても、この年に初めて使ったエージェントは出す");
eq(highlights({late: 0, weekend: 0}, [], new Set(), {"Codex": new Date(2025, 2, 1, 2)/1000}, day0, new Date(2024, 5, 1)/1000)[0].c0, 58, "深夜 2 時に始めたなら、印は前の日の列（光と同じ）");
eq(texts(highlights({late: 0, weekend: 0}, [], new Set(), {}, day0, new Date(2024, 5, 1)/1000)), [], "前の年から使っていたエージェント（firstOf に入らない）は出さない");

// 並列：続けて動かしただけ（重ならない）のセッションは数えない。同じセッションの区間の重なりも数えない
const chain = (from, n) => Array.from({length: n}, (_, d) => Array.from({length: 6}, (_, k) => { const a = new Date(2025, 0, 1 + from + d, 10)/1000 + k*602; return {start: a, end: a + 600, nPrompts: 5, segs: [[a, a + 600, 10]]}; })).flat();
eq(halves([...chain(0, 10), ...chain(60, 10)]).filter(h => h.k === "par"), [], "2 秒あけて続けたセッションは、並列にならない");
const touch = (from, n) => Array.from({length: n}, (_, d) => [0, 1].map(k => { const a = new Date(2025, 0, 1 + from + d, 10)/1000 + k*600; return {start: a, end: a + 600, nPrompts: 5, segs: [[a, a + 600, 10]]}; })).flat();
eq(halves([...touch(0, 10), ...touch(60, 10)]).filter(h => h.k === "par"), [], "ぴったりつながるだけのセッションも、並列にならない");
eq(mergeSegs([[0, 600, 1], [300, 900, 1], [1000, 1100, 1]]), [[0, 900], [1000, 1100]], "同じセッションの区間は重なりなくつなげる");

// 依頼の記録がほとんどない履歴では、依頼の数を比べない
eq(halves([...run(10, 0, 40, 0), ...run(10, 60, 40, 0)]), [], "0.0 → 0.0 は出さない");

// 深夜・週末を選んだなら、前半と後半の深夜の変化を先に（2 行で切られて落ちないように）
const three3 = [{k: "len", t: "sessions 30% longer"}, {k: "par", t: "2+ sessions at once 0% → 33%"}, {k: "late", t: "late nights (22:00–5:00) 2% → 11%"}];
eq(sideLines({hi: [], halves: three3}, {late: true}), ["2nd half vs 1st: late nights (22:00–5:00) 2% → 11%", "2nd half vs 1st: sessions 30% longer"], "選んだ深夜の変化は落ちない");
eq(sideLines({hi: [], halves: three3}, {late: false}), ["2nd half vs 1st: sessions 30% longer", "2nd half vs 1st: 2+ sessions at once 0% → 33%"], "選ばなければ深夜の変化は出さない");

// 写す範囲：最初の記録の日から。4 週に広げるのは記録の後ろへだけで、今日や年の終わりを越えない（記録の前の日を「使わなかった日」に見せない）
const xr = (o) => ({y: 2026, nd: 365, partial: null, lines: [1], short: true, ...o});
eq(yrRange(xr({partial: new Date(2026, 9, 10), c0: 277, c1: 281})), {c0: 277, last: 283}, "使い始めたばかり（10/5〜）なら、10/5 から今日まで（4 週に満たなくてよい）");
eq(yrRange(xr({c0: 350, c1: 360})), {c0: 350, last: 365}, "過去の年の終わりに始めたなら、年の終わりまで");
eq(yrRange(xr({c0: 10, c1: 20})), {c0: 10, last: 38}, "後ろに余裕があれば 4 週に広げる");
eq(yrRange(xr({c0: 10, c1: 200, short: false})), {c0: 10, last: 365}, "8 週以上なら、最初の記録から年の終わりまで");
eq(yrRange(xr({lines: [], c0: 365, c1: -1})), {c0: 337, last: 365}, "光のない年でも範囲は壊れない");

// 見出し：記録の長さに合わせる
eq(yrTitle({short: true, span: 1, firstYear: true}), "Your first day with AI.", "1 日");
eq(yrTitle({short: true, span: 5, firstYear: true}), "Your first 5 days with AI.", "2 週に満たなければ日で");
eq(yrTitle({short: true, span: 21, firstYear: false}), "3 weeks with AI.", "前の年に記録があれば「first」を付けない");
eq(yrTitle({short: true, span: 0, firstYear: true}), "Your first days with AI.", "光がなければ数を言わない");
eq(yrTitle({short: false, partial: new Date()}), "This year with AI, so far.", "今年");
eq(yrTitle({short: false, partial: null}), "A year with AI.", "過ぎた年");
