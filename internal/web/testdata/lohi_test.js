// insight.js の insLoHi（script_test.go の TestLoHi）。MET と fmtM は review.js と同じ形の小さな代わり
const eq = (got, want) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
const MET = {pushes: {u: "", int: true}, cache: {u: "%"}, cost: {fmt: v => "$" + v.toFixed(2)}};
const fmtM = (k, v) => v == null ? "—" : MET[k].fmt ? MET[k].fmt(v) : `${Math.round(v * 10) / 10}${MET[k].u ?? ""}`;
const B = (k, q1, q3, v, pos) => ({k, q1, q3, v, pos});
const fm = k => x => fmtM(k, x);
// 数えるもの：ふだんは整数。丸めると端が値をまたぐときだけ小数で
eq(insLoHi(fm("pushes"), B("pushes", 9.2, 11.4, 11.7, "high")), {lo: "9", hi: "11"});
eq(insLoHi(fm("pushes"), B("pushes", 9.2, 11.6, 11.7, "high")), {lo: "9.2", hi: "11.6"});
eq(insLoHi(fm("pushes"), B("pushes", 9.2, 11.66, 11.68, "high")), {lo: "9.20", hi: "11.66"});
eq(insLoHi(fm("pushes"), B("pushes", 3.6, 8, 3.2, "low")), {lo: "4", hi: "8"});
eq(insLoHi(fm("pushes"), B("pushes", 3.4, 8, 3.2, "low")), {lo: "3.4", hi: "8.0"}); // 「3.2：いつもの 3–8 より下」としない
eq(insLoHi(fm("pushes"), B("pushes", 3.6, 8, 3.5, "low")), {lo: "4", hi: "8"});
// 小数の桁をそろえる
eq(insLoHi(fm("cache"), B("cache", 50.8, 64, 60, "typical")), {lo: "50.8%", hi: "64.0%"});
eq(insLoHi(fm("cache"), B("cache", 50, 64, 60, "typical")), {lo: "50%", hi: "64%"});
// 書式のある指標はその書式のまま
eq(insLoHi(fm("cost"), B("cost", 1, 2.5, 3, "high")), {lo: "$1.00", hi: "$2.50"});
