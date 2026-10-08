package core

import (
	"slices"
	"testing"
)

// core.Agents の中身が、画面と CLI で使える形になっている。
func TestAgents(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range Agents {
		if a.Name == "" || a.Family == "" || a.Mark == "" {
			t.Errorf("名前・Family・頭文字のどれかが空: %+v", a)
		}
		if seen[a.Name] {
			t.Errorf("%q が 2 回ある", a.Name)
		}
		seen[a.Name] = true
		if a.Slot < 0 || a.Slot >= 8 || slices.Contains(AgentWarnSlots, a.Slot) { // 画面の色は --c0 〜 --c7（state.js の SLOTS）
			t.Errorf("%s の色 %d は使えない（警告の色 %v か範囲外）", a.Name, a.Slot, AgentWarnSlots)
		}
		if _, ok := Records[a.Name]; !ok {
			t.Errorf("core.Records に %q がない", a.Name)
		}
	}
	for name := range Records {
		if !seen[name] {
			t.Errorf("core.Records の %q が core.Agents にない", name)
		}
	}
	for name := range NativeDefs {
		if !seen[name] {
			t.Errorf("core.NativeDefs の %q が core.Agents にない", name)
		}
	}
}

// 表から作るものが、表を変える前と同じ（--sources の既定値と説明、kiroku autostart の環境変数、kiroku archive のフォルダ）。
func TestAgentsDerived(t *testing.T) {
	if got, want := Families(), []string{"claude", "kiro", "amazonq", "codex"}; !slices.Equal(got, want) {
		t.Errorf("Families() = %v, want %v", got, want)
	}
	if got, want := FamiliesHelp(), "claude, kiro (includes Kiro Crew), amazonq, codex"; got != want {
		t.Errorf("FamiliesHelp() = %q, want %q", got, want)
	}
	if got, want := AgentEnv(), []string{"CLAUDE_CONFIG_DIR", "KIRO_HOME", "KIROCREW_HOME", "CODEX_HOME"}; !slices.Equal(got, want) {
		t.Errorf("AgentEnv() = %v, want %v", got, want)
	}
	if got, want := ArchiveDirs(), []string{"claude", "crew"}; !slices.Equal(got, want) {
		t.Errorf("ArchiveDirs() = %v, want %v", got, want)
	}
	if ArchiveDirOf("Codex") != "" || ArchiveDirOf("no such agent") != "" {
		t.Error("コピーしないエージェントのフォルダは空")
	}
}
