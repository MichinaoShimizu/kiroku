/* pacing（panels.js）の確かめ。前の期間の合計 − ここまで を、明日から期間の終わりまでの日数で割ること。
   period・nowMs は画面の状態に頼るので、ここで差し替える（script_test.go の TestPacing） */
const eq = (got, want, msg) => { const g = JSON.stringify(got), w = JSON.stringify(want); if (g !== w){ console.error(`${msg}: ${g} ではなく ${w} のはず`); process.exitCode = 1; } };
const WS = new Date(2026, 9, 12).getTime()/1000, WE = new Date(2026, 9, 19).getTime()/1000; // 月曜から 7 日
let NOW = new Date(2026, 9, 14, 15).getTime(); // 水曜の午後：残りは木〜日の 4 日
const period = () => ({ws: WS, we: WE}), nowMs = () => NOW;
const W = (cost, tokens, credits, unpriced) => ({usage: {cost, tokens, credits, unpriced: unpriced || 0}});
eq(pacing(W(6, 1, 0), W(14, 1, 0)), {left: 4, cost: {ref: 14, so: 6, rest: 8, per: 2}, credits: null}, "先週 $14、ここまで $6 なら、残り 4 日は 1 日 $2");
eq(pacing(W(20, 1, 0), W(14, 1, 0)).cost, {ref: 14, so: 20, rest: -6, per: 0}, "超えていれば 1 日は 0、超えた分は rest に");
eq(pacing(W(0, 0, 10), W(0, 0, 50)), {left: 4, cost: null, credits: {ref: 50, so: 10, rest: 40, per: 10}}, "クレジットだけでも、費用とは別に出す");
eq(pacing(W(0, 1, 0, 999), W(14, 1, 0)), null, "今週が料金表にないモデルだけなら、ここまでがわからないので出さない");
eq(pacing(W(6, 1, 0), W(0, 1, 0, 999)), null, "先週の費用が見積もれなければ出さない");
eq(pacing(W(6, 1, 0), undefined), null, "先週の記録がなければ出さない");
eq(pacing(undefined, W(14, 1, 0)), null, "今週の記録がなければ出さない");
eq(pacing(W(0, 0, 0), W(14, 1, 0)).cost.per, 3.5, "今週トークンがまだなければ、ここまでは 0");
NOW = new Date(2026, 9, 18, 9).getTime(); eq(pacing(W(6, 1, 0), W(14, 1, 0)), null, "最後の日は残りの日がない");
NOW = new Date(2026, 9, 20, 9).getTime(); eq(pacing(W(6, 1, 0), W(14, 1, 0)), null, "終わった期間には出さない");
NOW = new Date(2026, 9, 12, 0, 30).getTime(); eq(pacing(W(1, 1, 0), W(14, 1, 0)).left, 6, "最初の日は、残りが 6 日");
NOW = new Date(2026, 9, 14, 15).getTime();
eq(pacing(W(14, 1, 0), W(14, 1, 0)).cost, {ref: 14, so: 14, rest: 0, per: 0}, "ちょうど同じ額なら rest は 0（超過ではない）");
// usdS（狭いマス）：値の大小と表示の大小が食い違わないこと
[[0.004, "<$0.1"], [0.03, "<$0.1"], [0.05, "$0.1"], [2.34, "$2.3"], [9.94, "$9.9"], [9.96, "$10"], [12.6, "$13"], [1234, "$1,234"]].forEach(([v, w]) => eq(usdS(v), w, `usdS(${v})`));
