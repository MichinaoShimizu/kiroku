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

// いちばん動いていた月：ほかの月の平均より 3 割以上多いときだけ
eq(texts(highlights({late: 0, weekend: 0}, [{start: "2025-01-01", active: 600}, {start: "2025-02-01", active: 780}], new Set(), {}, day0)), ["Busiest month: Feb · 13h"], "3 割多ければ出す");
eq(texts(highlights({late: 0, weekend: 0}, [{start: "2025-01-01", active: 600}, {start: "2025-02-01", active: 779}], new Set(), {}, day0)), [], "3 割に届かなければ出さない");
eq(texts(highlights({late: 0, weekend: 0}, [{start: "2025-01-01", active: 600}], new Set(), {}, day0)), [], "1 か月だけなら出さない");

// いちばん長い休み：7 日以上だけ。同じ月の中なら「Jan 3–9」
eq(texts(highlights({late: 0, weekend: 0}, [], days(2, 10), {}, day0)), ["Longest break: Jan 3–9 · 7 days"], "7 日の休みは出す");
eq(texts(highlights({late: 0, weekend: 0}, [], days(2, 9), {}, day0)), [], "6 日の休みは出さない（週末だけの人の平日など）");
eq(texts(highlights({late: 0, weekend: 0}, [], days(2, 10, 30), {}, day0)), ["Longest break: Jan 11–29 · 19 days"], "長いほうの休みを選ぶ");
eq(highlights({late: 0, weekend: 0}, [], days(2, 10, 30), {}, day0)[0].c0, 10, "休みの始まりの列（Jan 11 は 10 列目）");

// 途中から使い始めたエージェント：最初の記録から 2 週間より後
const t0 = new Date(2025, 0, 1, 10)/1000;
eq(texts(highlights({late: 0, weekend: 0}, [], new Set(), {"Claude Code": t0, "Codex": t0 + 15*86400}, day0)), ["First Codex: Jan 16"], "15 日後に始めたエージェントは出す");
eq(texts(highlights({late: 0, weekend: 0}, [], new Set(), {"Claude Code": t0, "Codex": t0 + 13*86400}, day0)), [], "2 週間以内なら出さない");

// 深夜・週末：10% 以上だけ
eq(texts(highlights({late: 10, weekend: 9.9}, [], new Set(), {}, day0)), ["Late nights (22:00–5:00): 10% of active time"], "深夜 10% は出し、週末 9.9% は出さない");

// 前半と後半
const ses = (dayN, h, mins, prompts) => { const a = new Date(2025, 0, 1 + dayN, h)/1000; return {start: a, end: a + mins*60, nPrompts: prompts, segs: [[a, a + mins*60, 10]]}; };
const run = (n, from, mins, prompts, h = 10) => Array.from({length: n}, (_, i) => ses(from + i, h, mins, prompts));
eq(halves(run(10, 0, 30, 5)), [], "28 日に満たなければ比べない");
eq(halves([...run(4, 0, 30, 5), ...run(10, 40, 60, 5)]), [], "片方が 5 セッションに満たなければ比べない");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 40, 50, 5)])), ["sessions 25% longer"], "25% 長くなれば出す");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 40, 47, 5)])), [], "18% なら出さない");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 40, 40, 7)])), ["prompts per session 5.0 → 7.0"], "依頼の数が 4 割増えれば出す");
eq(texts(halves([...run(10, 0, 40, 5), ...run(10, 40, 40, 5, 23)])), ["late nights 0% → 100%"], "深夜の割合が 5 ポイント以上動けば出す");
const par = [...run(10, 0, 60, 5), ...run(10, 40, 60, 5), ...run(10, 40, 60, 5).map(s => ({...s, start: s.start + 1800, end: s.end + 1800, segs: [[s.segs[0][0] + 1800, s.segs[0][1] + 1800, 10]]}))];
const pt = halves(par).find(h => h.t.startsWith("parallel"));
eq(pt && pt.t, "parallel 0% → 33%", "並列は、重なった時間 ÷ どれかが動いていた時間");
eq(!!(pt && pt.q), true, "変化には画面だけの問いかけを付ける");
