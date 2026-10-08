package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

// 大きな履歴での速さとサイズを測る。履歴は年単位で増えるので、1 年分で読み込み・集計・画面づくりが
// どれだけかかり、HTML と /data.json がどれだけの大きさになるかを見る。
//
//	go test ./internal/cli -run '^$' -bench Large -benchtime 3x
//	KIROKU_BENCH_DAYS=730 KIROKU_BENCH_PER_DAY=12 go test ./internal/cli -run '^$' -bench Large   # 大きさを変える

// benchSize は合成する履歴の日数と、1 日あたりのセッション数。
func benchSize() (days, perDay int) {
	days, perDay = 365, 8
	if v, err := strconv.Atoi(os.Getenv("KIROKU_BENCH_DAYS")); err == nil && v > 0 {
		days = v
	}
	if v, err := strconv.Atoi(os.Getenv("KIROKU_BENCH_PER_DAY")); err == nil && v > 0 {
		perDay = v
	}
	return days, perDay
}

// writeClaudeHistory は、days 日 × perDay セッションの Claude Code の履歴を root の下に書く。
// 1 セッションは 4〜30 の依頼で、依頼ごとに応答（使用量つき）・ツールの実行・結果・最後の返事がある。
func writeClaudeHistory(tb testing.TB, root string, days, perDay int) int {
	tb.Helper()
	r := rand.New(rand.NewSource(1))
	projects := []string{"web-app", "data-pipeline", "mobile", "docs", "infra", "sdk"}
	models := []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"}
	ts := func(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	start := time.Date(2025, 10, 6, 9, 0, 0, 0, time.UTC)
	n := 0
	for d := 0; d < days; d++ {
		for k := 0; k < perDay; k++ {
			p := projects[r.Intn(len(projects))]
			cwd := "/Users/me/" + p
			dir := filepath.Join(root, "-Users-me-"+p)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				tb.Fatal(err)
			}
			t := start.AddDate(0, 0, d).Add(time.Duration(k*70+r.Intn(40)) * time.Minute)
			model := models[r.Intn(len(models))]
			var b strings.Builder
			turns := 4 + r.Intn(27)
			for i := 0; i < turns; i++ {
				prompt := fmt.Sprintf("Fix the failing test in %s/pkg%d and explain why it broke (%d)", p, r.Intn(50), i)
				if r.Intn(20) == 0 { // 貼り付けたログのような長い依頼も混ぜる
					prompt += strings.Repeat(" stack frame at handler.go:42", 200)
				}
				fmt.Fprintf(&b, `{"type":"user","timestamp":%q,"cwd":%q,"gitBranch":"main","message":{"role":"user","content":%s}}`+"\n", ts(t), cwd, q(prompt))
				t = t.Add(time.Duration(20+r.Intn(60)) * time.Second)
				id := fmt.Sprintf("m%d-%d-%d", d, k, i)
				fmt.Fprintf(&b, `{"type":"assistant","timestamp":%q,"message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d},"content":[{"type":"tool_use","id":"t%s","name":"Edit","input":{"file_path":"%s/pkg/file%d.go","old_string":"a","new_string":"b\nc"}}]}}`+"\n",
					ts(t), id, model, 200+r.Intn(3000), 100+r.Intn(2000), 20000+r.Intn(80000), r.Intn(5000), id, cwd, r.Intn(200))
				t = t.Add(time.Duration(5+r.Intn(30)) * time.Second)
				fmt.Fprintf(&b, `{"type":"user","timestamp":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t%s","content":"ok"}]}}`+"\n", ts(t), id)
				t = t.Add(time.Duration(5+r.Intn(60)) * time.Second)
				reply := "Done. I changed the handler so the retry stops after three attempts, and the test now covers the timeout case."
				fmt.Fprintf(&b, `{"type":"assistant","timestamp":%q,"message":{"id":"%s-r","model":%q,"usage":{"input_tokens":10,"output_tokens":%d},"content":[{"type":"text","text":%s}]}}`+"\n",
					ts(t), id, model, 50+r.Intn(400), q(reply))
				t = t.Add(time.Duration(30+r.Intn(300)) * time.Second)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("s-%04d-%02d.jsonl", d, k)), []byte(b.String()), 0o600); err != nil {
				tb.Fatal(err)
			}
			n++
		}
	}
	return n
}

// BenchmarkLarge は kiroku serve の読み直し 1 回ぶんを、段階ごとに測る。
//   - read: 履歴を読む（キャッシュなし。kiroku html と serve の最初の読み込み）
//   - aggregate: すべての週と月を集計する
//   - render: HTML と /data.json を作る
//
// render は、HTML と JSON の大きさ（html-MB・json-MB）と、gzip したときの大きさ（json-gz-MB）も出す。
func BenchmarkLarge(b *testing.B) {
	days, perDay := benchSize()
	root := b.TempDir()
	n := writeClaudeHistory(b, root, days, perDay)
	b.Logf("%d sessions over %d days", n, days)
	logw = io.Discard
	srcs := []source.Source{&source.Claude{Root: root}}
	want := map[string]bool{"claude": true}
	data, _ := collect(srcs, want, 15)
	if len(data) != n {
		b.Fatalf("read %d sessions, want %d", len(data), n)
	}
	weeks, months := report.AllWeeks(data), report.AllMonths(data)
	meta := map[string]any{}

	b.Run("read", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collect(srcs, want, 15)
		}
	})
	b.Run("aggregate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			report.AllWeeks(data)
			report.AllMonths(data)
		}
	})
	b.Run("render", func(b *testing.B) {
		var html string
		var js []byte
		for i := 0; i < b.N; i++ {
			parts, err := web.Marshal(data, weeks, months, meta, 0) // kiroku serve の読み直し（live.refresh）と同じ
			if err != nil {
				b.Fatal(err)
			}
			if html, err = parts.HTML(true); err != nil {
				b.Fatal(err)
			}
			js = parts.JSON()
		}
		b.StopTimer()
		var gz bytes.Buffer
		w := gzip.NewWriter(&gz)
		w.Write(js)
		w.Close()
		b.ReportMetric(float64(len(html))/1e6, "html-MB")
		b.ReportMetric(float64(len(js))/1e6, "json-MB")
		b.ReportMetric(float64(gz.Len())/1e6, "json-gz-MB")
	})
}

// 合成した履歴を読めているか（ベンチマークが別のものを測らないように）。
func TestBenchHistoryReadable(t *testing.T) {
	root := t.TempDir()
	n := writeClaudeHistory(t, root, 3, 2)
	logw = io.Discard
	data, _ := collect([]source.Source{&source.Claude{Root: root}}, map[string]bool{"claude": true}, 15)
	if len(data) != n {
		t.Fatalf("read %d sessions, want %d", len(data), n)
	}
	var prompts int
	var tokens float64
	for _, d := range data {
		prompts += d.NPrompts
		tokens += d.Usage.Total()
		if d.Project == "" || len(d.Segs) == 0 {
			t.Errorf("session %s: project %q, segs %d", d.ID, d.Project, len(d.Segs))
		}
	}
	if prompts < 4*n || tokens == 0 {
		t.Errorf("prompts %d, tokens %v", prompts, tokens)
	}
	if w := report.AllWeeks(data); len(w) == 0 {
		t.Error("no weeks")
	}
}
