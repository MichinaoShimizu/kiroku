package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/klauspost/compress/zstd"
)

// testdata/snapshot.json は、testdata/ のすべての合成データ（Claude Code・Kiro IDE・Kiro CLI・Kiro Crew・
// SQLite の Kiro CLI と Amazon Q・Kiro IDE（旧）・Codex）を読んだ Go 版の出力。
// golden.json（Python 版との比較）が見ない履歴と項目も含めて、集計の数字が意図せず変わっていないかを確かめる。
// 集計を変えて数字が変わるときは、なぜ変わるのかを PR に書いてから、次で作り直す。
//
//	go test -run TestSnapshot -update ./internal/cli
var update = flag.Bool("update", false, "testdata/snapshot.json を作り直す")

const snapshotPath = "testdata/snapshot.json"

func TestSnapshot(t *testing.T) {
	got := loadAllSources(t)
	if *update {
		b, err := json.MarshalIndent(got, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(snapshotPath, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s を作り直しました", snapshotPath)
		return
	}
	b, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("%v（go test -run TestSnapshot -update ./internal/cli で作る）", err)
	}
	var want any
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	var diffs []string
	diffJSON(want, got, "", &diffs)
	for i, d := range diffs {
		if i == 30 {
			t.Errorf("…ほか %d 件", len(diffs)-30)
			break
		}
		t.Error(d)
	}
	if len(diffs) > 0 {
		t.Log("意図した変更なら go test -run TestSnapshot -update ./internal/cli で作り直す")
	}
}

// loadAllSources は、すべての合成データを読んだ集計（セッション・週・月・計測の状態）を JSON の形で返す。
func loadAllSources(t *testing.T) any {
	t.Helper()
	setup(t) // 時間帯を Asia/Tokyo に、古い Kiro IDE のファイルの更新時刻を戻す
	h := filepath.Join("testdata", "home")
	codex := codexHome(t)
	all := source.All(source.Options{
		ClaudeRoot:   filepath.Join(h, ".claude", "projects"),
		KiroHome:     filepath.Join(h, ".kiro"),
		KiroStorages: []string{filepath.Join(h, ".config", "Kiro", "User", "globalStorage", "kiro.kiroagent")},
		CrewHome:     filepath.Join("testdata", "crew"),
		KiroCLIDB:    filepath.Join("testdata", "sqlite", "kiro-cli.sqlite3"),
		AmazonQDB:    filepath.Join("testdata", "sqlite", "amazon-q.sqlite3"),
		CodexHome:    codex,
	})
	sessions, rep := collect(all, map[string]bool{"claude": true, "kiro": true, "amazonq": true, "codex": true}, 15)
	for _, r := range rep {
		if r.N == 0 {
			t.Errorf("%s の合成データが読めていない", r.Name)
		}
	}
	got := roundTrip(t, map[string]any{"sessions": sessions, "weeks": report.AllWeeks(sessions), "months": report.AllMonths(sessions), "sources": rep})
	return portable(got, codex, "")
}

// portable は、OS や一時ディレクトリで変わる部分をそろえる（パスの区切りを / に、Codex の一時ディレクトリを $CODEX に）。
func portable(v any, codex, key string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = portable(e, codex, k)
		}
	case []any:
		for i, e := range x {
			x[i] = portable(e, codex, key)
		}
	case string:
		if key == "file" || key == "where" {
			x = strings.ReplaceAll(x, codex, "$CODEX")
			return strings.ReplaceAll(x, `\`, "/")
		}
	}
	return v
}

// codexHome は testdata/codex を一時ディレクトリに写し、新しい版の圧縮ファイル（.jsonl.zst）を作る（internal/source の codex_test.go と同じ）。
func codexHome(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(filepath.Join("testdata", "codex"))); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(filepath.Join(dst, "thr-new.jsonl.src"))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	dir := filepath.Join(dst, "sessions", "2026", "09", "30")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(filepath.Join(dir, "rollout-2026-09-30T14-00-00-thr-new.jsonl.zst"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	enc, _ := zstd.NewWriter(out)
	if _, err := io.Copy(enc, in); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return dst
}

// diffJSON は JSON の形の 2 つの値を比べる。数値は、OS や CPU による計算の端数の違いを許す。
func diffJSON(want, got any, path string, diffs *[]string) {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: want object, got %T", path, got))
			return
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			wv, wok := w[k]
			gv, gok := g[k]
			switch {
			case !wok:
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: 増えた %s", path, k, short(gv)))
			case !gok:
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: なくなった（want %s）", path, k, short(wv)))
			default:
				diffJSON(wv, gv, path+"."+k, diffs)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			*diffs = append(*diffs, fmt.Sprintf("%s: want %s, got %s", path, short(want), short(got)))
			return
		}
		for i := range w {
			diffJSON(w[i], g[i], fmt.Sprintf("%s[%d]", path, i), diffs)
		}
	case float64:
		g, ok := got.(float64)
		if !ok || math.Abs(w-g) > 1e-9*math.Max(1, math.Abs(w)) {
			*diffs = append(*diffs, fmt.Sprintf("%s: want %v, got %v", path, want, got))
		}
	default:
		if fmt.Sprint(want) != fmt.Sprint(got) || (want == nil) != (got == nil) {
			*diffs = append(*diffs, fmt.Sprintf("%s: want %s, got %s", path, short(want), short(got)))
		}
	}
}

func short(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 80 {
		return string(b[:80]) + "…"
	}
	return string(b)
}

// AllWeeks・AllMonths は期間に関わるセッションだけを選んで集計する（report/span.go）。
// すべてのセッションを渡して集計したときと同じになるかを、すべての合成データで確かめる。
// セッションに時刻を持つ項目を足して span.go に入れ忘れると、ここで違いが出る。
func TestPeriodsMatchFullScan(t *testing.T) {
	setup(t)
	h := filepath.Join("testdata", "home")
	all := source.All(source.Options{
		ClaudeRoot:   filepath.Join(h, ".claude", "projects"),
		KiroHome:     filepath.Join(h, ".kiro"),
		KiroStorages: []string{filepath.Join(h, ".config", "Kiro", "User", "globalStorage", "kiro.kiroagent")},
		CrewHome:     filepath.Join("testdata", "crew"),
		KiroCLIDB:    filepath.Join("testdata", "sqlite", "kiro-cli.sqlite3"),
		AmazonQDB:    filepath.Join("testdata", "sqlite", "amazon-q.sqlite3"),
		CodexHome:    codexHome(t),
	})
	data, _ := collect(all, map[string]bool{"claude": true, "kiro": true, "amazonq": true, "codex": true}, 15)
	for k, w := range report.AllWeeks(data) {
		ws, _ := time.ParseInLocation("2006-01-02", k, time.Local)
		if full, _ := json.Marshal(report.Stats(data, ws)); string(full) != mustJSON(t, w) {
			t.Errorf("週 %s: 期間に関わるセッションだけで集計すると、結果が変わる", k)
		}
	}
	for k, m := range report.AllMonths(data) {
		ms, _ := time.ParseInLocation("2006-01", k, time.Local)
		if full, _ := json.Marshal(report.Summarize(data, ms, ms.AddDate(0, 1, 0))); string(full) != mustJSON(t, m) {
			t.Errorf("月 %s: 期間に関わるセッションだけで集計すると、結果が変わる", k)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
