// calendar.js の lastEnd を Node で動かす（script_test.go の TestLastEnd）。
// Math.max(...配列) は、要素が 20 万ほどあると引数が多すぎて RangeError になる。lastEnd は何件でも動くこと
const eq = (got, want) => { if (got !== want){ console.error(JSON.stringify(got) + " != " + JSON.stringify(want)); process.exitCode = 1; } };
eq(lastEnd([{end: 5}, {end: 9}, {end: 7}]), 9);
eq(lastEnd([{end: 3}]), 3);
const many = Array.from({length: 250000}, (_, i) => ({end: 1700000000 + (i * 7919) % 250000}));
many[123456].end = 1800000000;
eq(lastEnd(many), 1800000000);
