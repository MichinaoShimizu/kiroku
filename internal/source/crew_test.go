package source

import (
	"path/filepath"
	"strings"
	"testing"
)

// testdata/crew は Kiro Crew の作り（session_map.json・sessions/*.jsonl・subagents/*/state.json）に合わせた合成データ。
func TestKiroCrewTagsKiroCLISessions(t *testing.T) {
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(td, "crew")}
	bs := load(t, k)
	if len(bs) != 3 {
		t.Fatalf("会話の数 = %d, want 3（Crew の分を足さない）", len(bs))
	}
	crew := 0
	for _, b := range bs {
		if b.Source == "Kiro Crew" {
			crew++
		}
	}
	if crew != 3 || k.Detail() != "うち Kiro Crew から 3 件" {
		t.Fatalf("Crew の目印 = %d, detail = %q", crew, k.Detail())
	}
	if b := bs[1]; b.Title != "夜間のデプロイ見張り" {
		t.Errorf("Crew のタイトル = %q", b.Title)
	}
	if b := bs[0]; !strings.HasPrefix(b.Title, "サブエージェント reviewer: PR #12") {
		t.Errorf("サブエージェントのタイトル = %q", b.Title)
	}
	if b := bs[2]; b.Title != "CLIでデプロイ確認" { // 古い形（文字列だけ）の対応表。タイトルは kiro-cli のまま
		t.Errorf("古い対応表のタイトル = %q", b.Title)
	}
	if bs[0].Credits == nil || len(bs[0].Credits) == 0 {
		t.Error("クレジットは kiro-cli の履歴のものを使う")
	}
}

func TestNoCrew(t *testing.T) {
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(t.TempDir(), "none")}
	for _, b := range load(t, k) {
		if b.Source != "Kiro CLI" {
			t.Fatal("Crew がなければ目印はつけない")
		}
	}
	if k.Detail() != "" {
		t.Fatal(k.Detail())
	}
}
