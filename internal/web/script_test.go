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

// i18nStub は、js/ から関数だけを取り出して Node で動かすテストの前に置く。画面の言語は英語（t・JA・inEnglish の代わり）
const i18nStub = "const JA = false, isJA = () => false, inEnglish = f => f();\n" +
	"function t(s, vars){ return vars ? s.replace(/\\{(\\w+)\\}/g, (m, k) => Object.prototype.hasOwnProperty.call(vars, k) ? String(vars[k]) : m) : s; }\n"

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
	// keepRow は文言を tph（訳して <code> を差し込む）で作るので、それもいっしょに取り出す
	tph := regexp.MustCompile(`(?ms)^function tph\(s, vars\)\{.*?^\}$`).FindString(text)
	if tph == "" {
		t.Fatal("panels.js に tph が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "keeprow_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "keep.js")
	if err := os.WriteFile(f, append([]byte(tph+"\n"+fn+"\n"), cases...), 0o600); err != nil {
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

// 前の期間並みに収める 1 日の目安（pacing）と、狭いマスの推定費用（usdS）。panels.js と format.js から取り出して Node で動かす。
func TestPacing(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	var code strings.Builder
	for _, x := range []struct{ file, re string }{
		{"js/format.js", `(?m)^const costOf = .*$`},
		{"js/format.js", `(?m)^function dayNo\(.*$`},
		{"js/format.js", `(?m)^const usdS = .*$`},
		{"js/panels.js", `(?ms)^function pacing\(w, pw\)\{.*? : null; \}$`},
	} {
		src, err := jsFiles.ReadFile(x.file)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(x.re).FindString(strings.ReplaceAll(string(src), "\r\n", "\n"))
		if m == "" {
			t.Fatalf("%s に %s が見つからない", x.file, x.re)
		}
		code.WriteString(m + "\n")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "pacing_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "pacing.js")
	if err := os.WriteFile(f, append([]byte("const commas = n => n.toLocaleString(\"en-US\");\n"+code.String()), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("pacing が違う:\n%s", out)
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
	if err := os.WriteFile(f, append([]byte(i18nStub+"const LSEP = () => \", \";\nconst TSEP = () => \"; \";\n"+code+"\n"), cases...), 0o600); err != nil {
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
	if err := os.WriteFile(f, append([]byte(i18nStub+strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
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
		{"js/state.js", `(?m)^const DOW_EN = .*$`},
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
	if err := os.WriteFile(f, append([]byte(i18nStub+strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, f)
	cmd.Env = append(os.Environ(), "TZ=UTC")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("spanIn が違う:\n%s", out)
	}
}

// 画面の日本語（i18n.js の JA_TEXT）。値に HTML を入れていないこと、{name} が英語の鍵とそろっていること、
// t() が辞書を引くこと。js/ の t("…") で日本語のないものは、落とさずに一覧だけ出す（訳のない文は英語で出る）
func TestI18nCatalog(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n") // Windows の取り出しでは改行が CRLF になる
	m := regexp.MustCompile(`(?ms)^const JA_TEXT = \{.*?^\};$`).FindString(text)
	fn := regexp.MustCompile(`(?ms)^function t\(s, vars\)\{.*?^\}$`).FindString(text)
	if m == "" || fn == "" {
		t.Fatal("i18n.js に JA_TEXT か t が見つからない")
	}
	keys := map[string]bool{}
	call := regexp.MustCompile(`\bt\("((?:[^"\\]|\\.)*)"`)
	for _, name := range scripts {
		body, err := jsFiles.ReadFile("js/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range call.FindAllStringSubmatch(string(body), -1) {
			var k string
			if err := json.Unmarshal([]byte(`"`+c[1]+`"`), &k); err == nil {
				keys[k] = true
			}
		}
	}
	list := make([]string, 0, len(keys))
	for k := range keys {
		list = append(list, k)
	}
	kj, _ := json.Marshal(list)
	cases, err := os.ReadFile(filepath.Join("testdata", "i18n_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "i18n.js")
	code := "const JA = true, isJA = () => true;\n" + m + "\n" + fn + "\nconst KEYS = " + string(kj) + ";\n" + string(cases)
	if err := os.WriteFile(f, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, f).CombinedOutput()
	if err != nil {
		t.Errorf("JA_TEXT が違う:\n%s", out)
	} else if len(out) > 0 {
		t.Logf("日本語のない文言（英語で出る）:\n%s", out)
	}
}

// 月末の推定の幅（spread）とトレンド線（trend）：終わった日のばらつきから 1σ・2σ・3σ。下は使った分より小さくしない。日が足りなければ出さない。
func TestProjectionSpread(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	fn := regexp.MustCompile(`(?ms)^function spread\(.*?\); \}$`).FindString(strings.ReplaceAll(string(src), "\r\n", "\n")) // Windows のチェックアウトは CRLF
	if fn == "" {
		t.Fatal("panels.js に spread が見つからない")
	}
	msrc, err := jsFiles.ReadFile("js/metric.js")
	if err != nil {
		t.Fatal(err)
	}
	tfn := regexp.MustCompile(`(?ms)^function trend\(ys\)\{.*?return \{a: my - b \* mx, b\}; \}$`).FindString(strings.ReplaceAll(string(msrc), "\r\n", "\n"))
	if tfn == "" {
		t.Fatal("metric.js に trend が見つからない")
	}
	fn += "\n" + tfn
	code := fn + `
const eq = (a, b, m) => { if (Math.abs(a - b) > 1e-9) { console.error(m, a, "!=", b); process.exit(1); } };
// 10 日たって（今日を含む）、終わった 9 日は 10 と 20 が交互。30 日の月
const xs = [10,20,10,20,10,20,10,20,10], so = 150, b = spread(xs, so, 30, 10);
const m = 130/9, sd = Math.sqrt(xs.reduce((a, x) => a + (x - m) ** 2, 0) / 8), P = 450, e = sd * Math.sqrt(20 + 400/9);
eq(b.length, 3, "three bands"); eq(b[0].hi, P + e, "1σ hi"); eq(b[2].hi, P + 3*e, "3σ hi"); eq(b[1].lo, P - 2*e, "2σ lo");
// 毎日同じなら幅はない
const z = spread([5,5,5,5,5,5,5,5], 45, 30, 9); eq(z[2].lo, 45*30/9, "flat lo"); eq(z[2].hi, 45*30/9, "flat hi");
// 下は使った分より小さくしない
const w = spread([0,0,0,0,0,0,0,100], 100, 30, 9); eq(w[2].lo, 100, "floor at so");
// 終わった日が 1 日しかない・最後の日は出さない
if (spread([3], 3, 30, 2) !== null || spread([1,2,3], 6, 4, 4) !== null) { console.error("should be null"); process.exit(1); }
// トレンド線：y = 2 + 3x にぴったり乗る点なら a=2, b=3。1 点では引かない
const tr = trend([2,5,8,11]); eq(tr.a, 2, "trend a"); eq(tr.b, 3, "trend b"); if (trend([4]) !== null) { console.error("trend of 1"); process.exit(1); }
`
	f := filepath.Join(t.TempDir(), "spread.js")
	if err := os.WriteFile(f, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("spread が違う:\n%s", out)
	}
}

// JA_TEXT（i18n.js）のどの鍵も、画面のスクリプト・テンプレート・Go のどこかで使われている（消した画面の訳を残さない）。
// 鍵は英語の文そのものなので、ほかのファイルにその文（i18n.js と同じ書き方）があるかで見る
func TestI18nNoUnusedKeys(t *testing.T) {
	src, err := jsFiles.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	var other strings.Builder
	names, _ := filepath.Glob("js/*.js")
	names = append(names, "template.html")
	goFiles, _ := filepath.Glob("../*/*.go")
	for _, f := range goFiles {
		if !strings.HasSuffix(f, "_test.go") {
			names = append(names, f)
		}
	}
	for _, f := range names {
		if filepath.Base(f) == "i18n.js" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		other.Write(b)
		other.WriteByte('\n')
	}
	all := other.String()
	var unused []string
	for _, m := range regexp.MustCompile(`(?m)^  "((?:[^"\\]|\\.)*)": `).FindAllStringSubmatch(string(src), -1) {
		raw := m[1]
		var plain string
		if err := json.Unmarshal([]byte(`"`+raw+`"`), &plain); err != nil {
			plain = raw
		}
		head := plain // Go が組み立てる題（"Kiro Crew memory consolidation (" + slot + ")" など）は、{ の前までで見る
		if i := strings.Index(plain, "{"); i > 8 {
			head = plain[:i]
		}
		if !strings.Contains(all, raw) && !strings.Contains(all, plain) && !strings.Contains(all, head) {
			unused = append(unused, plain)
		}
	}
	if len(unused) > 0 {
		t.Errorf("JA_TEXT に使われていない鍵が %d 個ある（消すか、使う場所を直す）:\n%s", len(unused), strings.Join(unused, "\n"))
	}
}
