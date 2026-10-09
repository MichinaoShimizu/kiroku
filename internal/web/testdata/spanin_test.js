// panels.js の spanIn を、日付の小物と一緒に取り出して Node で動かす（script_test.go の TestSpanIn）。TZ=UTC で動かす
const eq = (got, want) => { if (got !== want){ console.error(JSON.stringify(got) + " != " + JSON.stringify(want)); process.exitCode = 1; } };
const at = (d, h, m) => Date.UTC(2026, 9, d, h, m) / 1000; // 2026-10-d h:m UTC
const ws = at(5, 0, 0), we = at(12, 0, 0); // Mon, Oct 5 – Sun, Oct 11
// 前の週の分は出さず、同じ日の区間は最初から最後までにまとめる
eq(spanIn({segs: [[at(1, 15, 0), at(1, 15, 20)], [at(8, 9, 0), at(8, 9, 15)], [at(8, 13, 0), at(8, 14, 30)], [at(9, 10, 0), at(9, 10, 5)]]}, ws, we), "Thu, Oct 8 09:00–14:30, Fri, Oct 9 10:00–10:05");
// 期間の始まりをまたぐ区間は、期間の始まりから
eq(spanIn({segs: [[at(4, 23, 0), at(5, 1, 0)]]}, ws, we), "Mon, Oct 5 00:00–01:00");
// 期間の外だけなら空
eq(spanIn({segs: [[at(1, 9, 0), at(1, 10, 0)]]}, ws, we), "");
