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
	Name    string   `json:"name"` // 画面とセッションの source に出る名前（Source.Name()。Kiro Crew のように、ほかの Source が読むものもある）
	Family  string   `json:"-"`    // --sources で選ぶ名前（Source.Family()）
	Slot    int      `json:"slot"` // 画面の色（--c<Slot>）。警告の色（AgentWarnSlots）は使わない
	Mark    string   `json:"mark"` // バッジに出す頭文字
	Env     []string `json:"-"`    // 読む場所を変える、エージェント自身の環境変数（kiroku autostart が書き残す）
	Archive string   `json:"-"`    // kiroku archive がコピーを置く、保存場所の下のフォルダ（空ならコピーしない）
	Note    string   `json:"-"`    // --sources の説明で、Family に添える一言
}

// AgentWarnSlots は、警告に見えるのでエージェントに使わない色（オレンジ・黄色）。
var AgentWarnSlots = []int{1, 4}

// Agents は対応しているエージェント。並びは --sources の既定値と説明、環境変数の順。
// Amazon Q は Kiro CLI の前身なので同じ色にする。
var Agents = []Agent{
	{Name: "Claude Code", Family: "claude", Slot: 0, Mark: "CC", Env: []string{"CLAUDE_CONFIG_DIR"}, Archive: "claude"},
	{Name: "Kiro IDE", Family: "kiro", Slot: 6, Mark: "KI", Env: []string{"KIRO_HOME"}},
	{Name: "Kiro IDE (legacy)", Family: "kiro", Slot: 6, Mark: "KI"},
	{Name: "Kiro CLI", Family: "kiro", Slot: 5, Mark: "KC", Env: []string{"KIRO_HOME"}},
	{Name: "Kiro CLI (SQLite)", Family: "kiro", Slot: 5, Mark: "KC"},
	{Name: "Kiro Crew", Family: "kiro", Slot: 3, Mark: "KW", Env: []string{"KIROCREW_HOME"}, Archive: "crew", Note: "includes Kiro Crew"},
	{Name: "Amazon Q", Family: "amazonq", Slot: 5, Mark: "Q"},
	{Name: "Codex", Family: "codex", Slot: 2, Mark: "CX", Env: []string{"CODEX_HOME"}},
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

// FamiliesHelp は --sources の説明に出す一覧（例: "claude, kiro (includes Kiro Crew), amazonq, codex"）。
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
