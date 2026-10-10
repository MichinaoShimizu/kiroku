package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 画面のスクリプトに構文の誤りがないこと（文字列の中に ${} を書いてしまうなど）。
// Node があるときだけ node --check で確かめる（CI のランナーには入っている）。
func TestScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	html, err := Render([]any{}, map[string]any{}, map[string]any{}, map[string]any{"report": []any{}, "git": []any{}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(html, -1)
	if len(m) == 0 {
		t.Fatal("script が見つからない")
	}
	f := filepath.Join(t.TempDir(), "page.js")
	if err := os.WriteFile(f, []byte(m[len(m)-1][1]), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, "--check", f).CombinedOutput(); err != nil {
		t.Errorf("画面のスクリプトに構文の誤り:\n%s", out)
	}
}

// 書き出す Markdown に入れた履歴の文字が、リンク・強調・HTML・メンションとして働かないこと。
// markdown.js はほかのファイルに頼らないので、それだけを Node で動かして確かめる。
func TestMarkdownEscape(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/markdown.js")
	if err != nil {
		t.Fatal(err)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "markdown_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "md.js")
	if err := os.WriteFile(f, append(append(src, '\n'), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("Markdown の打ち消しが違う:\n%s", out)
	}
}

// 計測の状態の保存期間の一言（keepRow）。0 日（now）を「わからない期間のあとで消える」や「0 日残す」と出さないこと。
// panels.js はほかのファイルに頼るので、keepRow の関数だけを取り出して Node で動かす。
func TestKeepRow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	// Windows のチェックアウトでは CRLF になり、$ が行末に当たらないので LF にそろえる
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	fn := regexp.MustCompile(`(?ms)^function keepRow\(r\)\{.*?\); \}$`).FindString(text)
	if fn == "" {
		t.Fatal("panels.js に keepRow が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "keeprow_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "keep.js")
	if err := os.WriteFile(f, append([]byte(fn+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("keepRow が違う:\n%s", out)
	}
}

// 内訳の件数（countBy）が、"__proto__" や "constructor" のような名前のプロジェクトも数えること。
// metric.js は画面の DOM を触るので、countBy の 1 行だけを取り出して Node で動かす。
func TestCountBy(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/metric.js")
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^const countBy = .*$`).FindString(string(src))
	if line == "" {
		t.Fatal("metric.js に countBy が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "countby_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "countby.js")
	if err := os.WriteFile(f, append([]byte(line+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("countBy が違う:\n%s", out)
	}
}

// 目安コストを出せないとき（料金表にないモデルのトークンだけ）に $0.00 と出さないこと。
// format.js は画面の DOM を触るので、costOf の 1 行だけを取り出して Node で動かす。
func TestCostOf(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/format.js")
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^const costOf = .*$`).FindString(string(src))
	if line == "" {
		t.Fatal("format.js に costOf が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "costof_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "cost.js")
	if err := os.WriteFile(f, append([]byte(line+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("costOf が違う:\n%s", out)
	}
}

// 履歴が記録しないもの（core.Records）を「0」ではなく「記録されていない」と出すための records・unrecorded。
// state.js はほかの値に頼るので、その行だけを取り出し、RECORDS には core.Records を入れて Node で動かす。
func TestUnrecorded(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/state.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	code := regexp.MustCompile(`(?ms)^const RECORDS = .*?^const notRec = .*?$`).FindString(text)
	if code == "" {
		t.Fatal("state.js に RECORDS から notRec までが見つからない")
	}
	recs, err := json.Marshal(core.Records)
	if err != nil {
		t.Fatal(err)
	}
	code = "let DATA = [];\n" + strings.Replace(code, "__RECORDS__", string(recs), 1)
	cases, err := os.ReadFile(filepath.Join("testdata", "records_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "records.js")
	if err := os.WriteFile(f, append([]byte(code+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("records・unrecorded が違う:\n%s", out)
	}
}

// 振り返りのプロンプトの流れ（reviewFlow）。出来事がプロンプトの間に時刻の順で入り、最初の n 個のプロンプトのあとの出来事は入れないこと。
// session.js はほかのファイルに頼るので、reviewFlow と、それが使う hm・FIXRE だけを取り出して Node で動かす。
func TestReviewFlow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	var code []string
	for _, x := range []struct{ file, re string }{
		{"js/format.js", `(?m)^function hm\(t\)\{.*$`},
		{"js/markdown.js", `(?ms)^function oneLine\(s\)\{.*?\}$`},
		{"js/session.js", `(?m)^const FIXRE = .*$`},
		{"js/session.js", `(?ms)^function reviewFlow\(.*?^\}$`},
	} {
		src, err := jsFiles.ReadFile(x.file)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(x.re).FindString(strings.ReplaceAll(string(src), "\r\n", "\n"))
		if m == "" {
			t.Fatalf("%s に %s が見つからない", x.file, x.re)
		}
		code = append(code, m)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "reviewflow_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "reviewflow.js")
	if err := os.WriteFile(f, append([]byte(strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, f)
	cmd.Env = append(os.Environ(), "TZ=UTC")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("reviewFlow が違う:\n%s", out)
	}
}

// 週報のプロンプトで、セッションが期間の中で動いていた時間（spanIn）。週をまたぐセッションは期間の中だけを、日ごとにまとめて出すこと。
// panels.js はほかのファイルに頼るので、spanIn と、それが使う日付の小物だけを取り出して Node で動かす。
func TestSpanIn(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	var code []string
	for _, x := range []struct{ file, re string }{
		{"js/state.js", `(?m)^const DOW = .*$`},
		{"js/format.js", `(?m)^const MON = .*$`},
		{"js/format.js", `(?m)^const dMD = .*$`},
		{"js/format.js", `(?m)^function md\(t\)\{.*$`},
		{"js/format.js", `(?m)^function hm\(t\)\{.*$`},
		{"js/panels.js", `(?ms)^function spanIn\(.*?^\}$`},
	} {
		src, err := jsFiles.ReadFile(x.file)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(x.re).FindString(strings.ReplaceAll(string(src), "\r\n", "\n"))
		if m == "" {
			t.Fatalf("%s に %s が見つからない", x.file, x.re)
		}
		code = append(code, m)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "spanin_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "spanin.js")
	if err := os.WriteFile(f, append([]byte(strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, f)
	cmd.Env = append(os.Environ(), "TZ=UTC")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("spanIn が違う:\n%s", out)
	}
}

// 1 年の露光の「見どころ」と「前半と後半」が、決めた境界で出る・出ないこと。シェア画像には、選んだものだけが載ること。
// year.js は画面の DOM を触るので、highlights・halves・mergeSegs・plateCols・sideLines・cardMarks・yrRange・yrTitle だけを取り出して Node で動かす。
func TestYearStory(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/year.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	var fns []string
	for _, name := range []string{"highlights", "halves", "mergeSegs", "plateCols", "sideLines", "cardMarks", "yrRange", "yrTitle", "yrSpan"} {
		fn := regexp.MustCompile(`(?ms)^function ` + name + `\(.*?\n\}\n`).FindString(text)
		if fn == "" {
			t.Fatalf("year.js に %s が見つからない", name)
		}
		fns = append(fns, fn)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "yearstory_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "year.js")
	if err := os.WriteFile(f, append([]byte(strings.Join(fns, "\n")), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("見どころ・前半と後半が違う:\n%s", out)
	}
}
