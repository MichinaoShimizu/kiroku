// review.js の lightOf と pricey（Opus 系・Fable 系・Mythos 系）（script_test.go の TestLightOf）。DATA と inP は置き換える
const eq = (got, want) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
const inP = (s, ws, we) => s.start >= ws && s.start < we;
const S = (id, models, o = {}) => ({id, models, start: 100, nPrompts: 2, nFiles: 0, cost: 1, ...o});
var DATA = [
  S("opus", [["claude-opus-5-5", 5]]),
  S("fable", [["claude-fable-5-2", 5]], {cost: 2}),
  S("fable-bedrock", [["us.anthropic.claude-fable-5-1-v1:0", 3]], {cost: 0.5}),
  S("mythos", [["claude-mythos-5-2", 5]], {cost: 0.4}),
  S("sonnet", [["claude-sonnet-5-5", 5]]),
  S("mostly-sonnet", [["claude-fable-5-2", 1], ["claude-sonnet-5-5", 4]]), // 主なモデル（応答のいちばん多いもの）で決める
  S("edited", [["claude-fable-5-2", 5]], {nFiles: 1}),
  S("long", [["claude-fable-5-2", 5]], {nPrompts: 4}),
  S("cheap", [["claude-fable-5-2", 5]], {cost: 0.29}),
  S("outside", [["claude-fable-5-2", 5]], {start: 300}),
  S("no-model", []),
];
const L = lightOf(0, 200);
eq(L.xs.map(s => s.id), ["fable", "opus", "fable-bedrock", "mythos"]); // 費用の高い順
eq(L.c, 3.9);
eq(pricey(S("x", [["Claude-Opus-4-1", 1]])), true);
eq(pricey(S("x", [["claude-haiku-4-5", 1]])), false);
eq(pricey(S("x", [["gpt-6-sol", 1]])), false);
