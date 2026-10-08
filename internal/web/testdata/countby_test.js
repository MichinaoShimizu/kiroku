/* countBy（metric.js）の確かめ。履歴から来た名前が Object のプロパティ名と同じでも、消えたり数え違えたりしないこと（script_test.go） */
const eq = (got, want, msg) => { if (got !== want){ console.error(`${msg}: ${got} ではなく ${want} のはず`); process.exitCode = 1; } };
const names = ["web", "__proto__", "__proto__", "constructor", "toString", "hasOwnProperty", "web", "web"];
const got = new Map(countBy(names, x => x));
eq(got.size, 5, "名前の数");
eq(got.get("web"), 3, "ふつうの名前");
eq(got.get("__proto__"), 2, "__proto__ も数える");
eq(got.get("constructor"), 1, "constructor は 1（Object の関数と足さない）");
eq(got.get("toString"), 1, "toString");
eq(got.get("hasOwnProperty"), 1, "hasOwnProperty");
eq(countBy(names, x => x).reduce((t, [, v]) => t + v, 0), names.length, "合計が件数と一致する");
eq(countBy([], x => x).length, 0, "空");
