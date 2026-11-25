package core

import (
	"slices"
	"testing"
)

// core.Agents の中身が、画面と CLI で使える形になっている。
func TestAgents(t *testing.T) {
	seen, slot := map[string]bool{}, map[int]string{}
	for _, a := range Agents {
		if a.Name == "" || a.Family == "" || a.Mark == "" {
			t.Errorf("名前・Family・頭文字のどれかが空: %+v", a)
		}
		if seen[a.Name] {
			t.Errorf("%q が 2 回ある", a.Name)
		}
		seen[a.Name] = true
		if a.Slot < 0 || a.Slot >= AgentColors { // 画面の色は --a0 〜（style.css）
			t.Errorf("%s の色 %d は範囲外（0〜%d）", a.Name, a.Slot, AgentColors-1)
		}
		if b, ok := slot[a.Slot]; ok && b != "Kiro CLI (V2)" && b != "Kiro CLI (SQLite)" {
			t.Errorf("%s と %s が同じ色 %d", a.Name, b, a.Slot)
		}
		slot[a.Slot] = a.Name
		if _, ok := Records[a.Name]; !ok {
			t.Errorf("core.Records に %q がない", a.Name)
		}
	}
	for name := range Records {
		if !seen[name] {
			t.Errorf("core.Records の %q が core.Agents にない", name)
		}
	}
}

// 表から作るものが、表を変える前と同じ（--sources の既定値と説明、kiroku archive のフォルダ）。
func TestAgentsDerived(t *testing.T) {
	if got, want := Families(), []string{"claude", "kiro", "codex"}; !slices.Equal(got, want) {
		t.Errorf("Families() = %v, want %v", got, want)
	}
	if got, want := FamiliesHelp(), "claude, kiro (includes Kiro Crew), codex"; got != want {
		t.Errorf("FamiliesHelp() = %q, want %q", got, want)
	}
	if got, want := ArchiveDirs(), []string{"claude", "crew"}; !slices.Equal(got, want) {
		t.Errorf("ArchiveDirs() = %v, want %v", got, want)
	}
	if ArchiveDirOf("Codex") != "" || ArchiveDirOf("no such agent") != "" {
		t.Error("コピーしないエージェントのフォルダは空")
	}
}
