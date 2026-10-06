/* costOf（format.js）の確かめ。目安コストを出せないときに $0.00 と出さないこと。
   format.js は画面の DOM を触るので、costOf の 1 行だけを取り出して動かす（script_test.go） */
const eq = (got, want, msg) => { if (got !== want){ console.error(`${msg}: ${got} ではなく ${want} のはず`); process.exitCode = 1; } };
eq(costOf({cost: 0.5, unpriced: 0}), 0.5, "見積もれた分はそのまま");
eq(costOf({cost: 0.5, unpriced: 12345}), 0.5, "一部が料金表になくても、見積もれた分は出す");
eq(costOf({cost: 0, unpriced: 12345}), null, "料金表にないモデルのトークンだけなら出せない");
eq(costOf({cost: 0, unpriced: 0}), 0, "ほんとうに 0 なら 0");
eq(costOf(null), null, "使用量の記録がない");
