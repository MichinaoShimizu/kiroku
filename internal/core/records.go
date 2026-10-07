package core

import "slices"

// Records は、エージェント（Session.Source）ごとに、履歴から読めるもの。
// 画面は、ここにないものを「0」ではなく「記録されていない」と出す（記録できないものを、なかったと読ませないため）。
// 何がなぜ読めないかは docs/sources.md の「What history records」、利用者向けの表は docs/guide.md の「What each agent records」
// （その表のうちここで決まる行は、TestGuideRecordsTable がここと照らし合わせる）。
//
//   - interrupts: 人が AI を途中で止めたこと
//   - limits: 利用上限に当たったこと
//   - compactions: 会話のコンパクション
//   - files: AI が編集したファイル
//   - subagents: サブエージェント
//   - outputs: AI が実行したコミット・PR 作成・変更した行
//
// Kiro Crew の会話は kiro-cli の履歴にあるものとないもの（Crew の使用量の記録と会話ログだけのもの）があり、
// ないものは編集したファイルを記録しないので、files に入れない（あるものは、編集があればそのまま見せる）。
var Records = map[string][]string{
	"Claude Code":       {"interrupts", "limits", "compactions", "files", "subagents", "outputs"},
	"Codex":             {"interrupts", "limits", "compactions", "files", "subagents"},
	"Kiro IDE":          {"files"},
	"Kiro IDE (legacy)": {},
	"Kiro CLI":          {"files"},
	"Kiro Crew":         {},
	"Kiro CLI (SQLite)": {"interrupts", "compactions", "files"},
	"Amazon Q":          {"interrupts", "compactions", "files"},
}

// RecordsOf は、source の履歴が what（Records の説明の名前）を記録するか。
func RecordsOf(source, what string) bool { return slices.Contains(Records[source], what) }
