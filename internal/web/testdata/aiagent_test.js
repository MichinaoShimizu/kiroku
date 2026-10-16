/* aiAgentOf・aiAgentIn（ai.js）の確かめ。DATA・st・keyOf は画面の状態なので、ここで差し替える（script_test.go の TestAIDefaultAgent） */
const eq = (got, want, msg) => { if (got !== want){ console.error(`${msg}: ${JSON.stringify(got)} ではなく ${JSON.stringify(want)} のはず`); process.exitCode = 1; } };
eq(aiAgentOf("Claude Code"), "claude", "Claude Code は claude");
eq(aiAgentOf("Codex"), "codex", "Codex は codex");
eq(aiAgentOf("Kiro IDE / CLI"), "kiro", "Kiro は kiro-cli");
eq(aiAgentOf("Kiro CLI (Classic)"), "kiro", "Kiro CLI (Classic) も kiro-cli");
eq(aiAgentOf("Amazon Q"), "", "動かすコマンドのないエージェントは空");
const st = {hidden: new Set()}, keyOf = s => s.project;
let DATA = [
  {source: "Claude Code", project: "a", segs: [[0, 100]]},
  {source: "Codex", project: "b", segs: [[100, 160], [200, 260]]},
  {source: "Amazon Q", project: "c", segs: [[0, 1000]]},
];
eq(aiAgentIn(0, 300), "codex", "期間の中でいちばん長いのは Codex（120 秒）。Amazon Q は数えない");
eq(aiAgentIn(0, 150), "claude", "期間の外は数えない（Claude 100 秒、Codex 50 秒）");
st.hidden.add("b");
eq(aiAgentIn(0, 300), "claude", "凡例で隠したセッションは数えない");
eq(aiAgentIn(500, 600), "", "期間に動いたエージェントがなければ空");
