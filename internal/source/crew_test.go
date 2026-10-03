package source

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// testdata/crew は Kiro Crew の作り（session_map.json・sessions/*.jsonl・subagents/*/state.json）に合わせた合成データ。
func TestKiroCrewTagsKiroCLISessions(t *testing.T) {
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(td, "crew")}
	bs := load(t, k)
	if len(bs) != 6 {
		t.Fatalf("会話の数 = %d, want 6（kiro-cli の 3 件 + Crew の記録だけにある 3 件）", len(bs))
	}
	crew := 0
	for _, b := range bs {
		if b.Source == "Kiro Crew" {
			crew++
		}
	}
	if crew != 6 || k.Detail() != "うち Kiro Crew から 3 件・Crew の使用量の記録でクレジットを補った会話 1 件・Crew の使用量の記録だけにある 3 件（2.00 クレジット）" {
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
	if n := nativeOf(bs[0].Finish(15)); n["Crew から動かした会話"].V != 1 || n["うちサブエージェント"].V != 1 || n["ターン"].V == 0 {
		t.Errorf("Crew の参考指標 = %+v", n)
	}
	if bs[0].Credits == nil || len(bs[0].Credits) == 0 {
		t.Error("クレジットは kiro-cli の履歴のものを使う")
	}
}

// Crew の使用量の記録（usage/tokens）: 同じ会話は多いほうを使い、kiro-cli にない記録は Crew のセッションにする。
func TestKiroCrewUsage(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo") // 裏方の処理は、このマシンの時刻の日ごとにまとめる
	if err != nil {
		t.Skip(err)
	}
	defer func(l *time.Location) { time.Local = l }(time.Local)
	time.Local = tokyo
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(td, "crew")}
	by := map[string]*core.Builder{}
	for _, b := range load(t, k) {
		by[b.ID] = b
	}
	credits := func(id string) float64 { return math.Round(sumCredits(by[id].Credits)*100) / 100 }
	// dashboard の会話: kiro-cli の記録 3.69 より Crew の記録 4.5 が多いので、Crew のほうを使う
	if v := credits("1cb4ad2f-90ba-4c5f-970b-5767003804b9"); v != 4.5 {
		t.Errorf("Crew の記録で補った会話のクレジット = %v, want 4.5", v)
	}
	// legacy-key の会話: Crew の記録 0.01 のほうが少ないので、kiro-cli の記録のまま
	if v := credits("1e13c3c1-d7ae-41c2-a324-e6a440665d9a"); v <= 0.01 {
		t.Errorf("kiro-cli の記録が多いときは置きかえない: %v", v)
	}
	// kiro-cli にない記録: 裏方の処理は日ごと、チャットは会話ごと。壊れた行（credits が負）と tokens 以外の行は数えない
	for id, want := range map[string]float64{"crew:_bg:2026-09-29": 0.25, "crew:_bg:2026-09-30": 0.5, "crew:chat-9-1790000000": 1.25} {
		b := by[id]
		if b == nil {
			t.Errorf("%s がない", id)
			continue
		}
		if v := credits(id); v != want {
			t.Errorf("%s のクレジット = %v, want %v", id, v, want)
		}
		if b.Source != "Kiro Crew" || b.Key != "kiro-crew:"+id[len("crew:"):] {
			t.Errorf("%s: source=%q key=%q", id, b.Source, b.Key)
		}
	}
	if s := by["crew:chat-9-1790000000"].Finish(15); s == nil || s.Models[0][0] != "deepseek-3.2" {
		t.Errorf("Crew のセッションのモデル = %+v", s)
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
