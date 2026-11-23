package core

import (
	"slices"
	"strings"
)

// Agent は、エージェントごとの決まりごと。エージェントを足したりやめたりするときは、まずここを直す。
// ここから作るもの: --sources の既定値と説明（internal/cli）、kiroku autostart が書き残す環境変数、
// kiroku archive のフォルダ、画面の色と頭文字（web が __AGENTS__ として埋めこむ）。
// TestAgentsCoverEverySource が、source.All の Name・Family と食い違っていないかを確かめる。
type Agent struct {
	Name    string   `json:"name"`            // 画面とセッションの source に出る名前（Source.Name()。Kiro Crew のように、ほかの Source が読むものもある）
	Family  string   `json:"-"`               // --sources で選ぶ名前（Source.Family()）
	Slot    int      `json:"slot"`            // 画面の色（style.css の --a<Slot>。0 から AgentColors-1）
	Mark    string   `json:"mark"`            // バッジに出す頭文字
	Env     []string `json:"-"`               // 読む場所を変える、エージェント自身の環境変数（kiroku autostart が書き残す）
	Archive string   `json:"-"`               // kiroku archive がコピーを置く、保存場所の下のフォルダ（空ならコピーしない）
	Note    string   `json:"-"`               // --sources の説明で、Family に添える一言
	About   string   `json:"about,omitempty"` // 画面の「Data sources」で名前に添える説明（名前だけでは何の履歴かわかりにくいとき）
}

// AgentColors は、エージェントの色（style.css の --a0〜）の数。区分の 8 色とは別の組で、良し悪しの緑・赤や琥珀は入れない。
// 違うエージェントには違う色をあてる（Kiro CLI (V2) と Kiro CLI (SQLite) は同じものの新旧なので同じ色）
const AgentColors = 5

// Agents は対応しているエージェント。並びは --sources の既定値と説明、環境変数の順。
var Agents = []Agent{
	{Name: "Claude Code", Family: "claude", Slot: 0, Mark: "CC", Env: []string{"CLAUDE_CONFIG_DIR"}, Archive: "claude"},
	{Name: "Kiro IDE / CLI (V3)", Family: "kiro", Slot: 1, Mark: "KI", Env: []string{"KIRO_HOME"},
		About: "Kiro IDE 1.x and Kiro CLI V3, which share one history that doesn't record which of them ran a session"},
	{Name: "Kiro CLI (V2)", Family: "kiro", Slot: 2, Mark: "KC", Env: []string{"KIRO_HOME"}, About: "Kiro CLI V2 (kiro-cli --v2, and the default before Kiro CLI 3.0); V3 sessions are under Kiro IDE / CLI (V3)"},
	{Name: "Kiro CLI (SQLite)", Family: "kiro", Slot: 2, Mark: "KC", About: "older Kiro CLI versions"},
	{Name: "Kiro Crew", Family: "kiro", Slot: 3, Mark: "KW", Env: []string{"KIROCREW_HOME"}, Archive: "crew", Note: "includes Kiro Crew"},
	{Name: "Codex", Family: "codex", Slot: 4, Mark: "CX", Env: []string{"CODEX_HOME"}},
}

// AgentOf は名前 name のエージェント。ないものは ok=false。
func AgentOf(name string) (Agent, bool) {
	for _, a := range Agents {
		if a.Name == name {
			return a, true
		}
	}
	return Agent{}, false
}

// ArchiveDirOf は、名前 name のエージェントのコピーを置くフォルダ（保存場所の下。なければ空）。
func ArchiveDirOf(name string) string {
	a, _ := AgentOf(name)
	return a.Archive
}

// Families は --sources で選べる名前（Agents の順、重なりなし）。
func Families() []string {
	var fs []string
	for _, a := range Agents {
		if !slices.Contains(fs, a.Family) {
			fs = append(fs, a.Family)
		}
	}
	return fs
}

// FamiliesHelp は --sources の説明に出す一覧（例: "claude, kiro (includes Kiro Crew), codex"）。
func FamiliesHelp() string {
	var parts []string
	for _, f := range Families() {
		s := f
		for _, a := range Agents {
			if a.Family == f && a.Note != "" {
				s += " (" + a.Note + ")"
			}
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// AgentEnv は、エージェントの読む場所を変える環境変数（Agents の順、重なりなし）。
func AgentEnv() []string {
	var es []string
	for _, a := range Agents {
		for _, e := range a.Env {
			if !slices.Contains(es, e) {
				es = append(es, e)
			}
		}
	}
	return es
}

// ArchiveDirs は、kiroku archive がコピーを置くフォルダ（Agents の順）。
func ArchiveDirs() []string {
	var ds []string
	for _, a := range Agents {
		if a.Archive != "" && !slices.Contains(ds, a.Archive) {
			ds = append(ds, a.Archive)
		}
	}
	return ds
}
