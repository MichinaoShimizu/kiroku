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
	html, err := render([]any{}, map[string]any{}, map[string]any{}, map[string]any{"report": []any{}, "git": []any{}}, 0, false)
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

// 日報のプロンプトで、セッションが期間の中で動いていた時間（spanIn）。期間をまたぐセッションは期間の中だけを、日ごとにまとめて出すこと。
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

// 利用上限の解除の時刻（limitReset、#570）。履歴の時刻（limitResetAt、UNIX 秒）があれば今いる時間帯の 24 時間の時刻で、
// 当たった日と違う日なら日付も付ける。なければエラー文の時刻（"9pm" などを 24 時間に）。review.js と format.js から取り出し、TZ=Asia/Tokyo で動かす。
func TestLimitReset(t *testing.T) {
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
		{"js/review.js", `(?ms)^function limitReset\(s, i\)\{.*?\); \}$`},
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
	cases, err := os.ReadFile(filepath.Join("testdata", "limitreset_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "limitreset.js")
	if err := os.WriteFile(f, append([]byte(i18nStub+strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, f)
	cmd.Env = append(os.Environ(), "TZ=Asia/Tokyo")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("limitReset が違う:\n%s", out)
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
// 鍵は英語の文そのものなので、ほかのファイルにその文がまるごと 1 つの文字列（"…"・'…'・`…`）として書いてあるか、
// template.html の要素の文字（>…<。data-t の要素を applyStaticText が訳す）として書いてあるかで見る。
// 長い文の一部に同じ言葉があるだけでは、使われているとみなさない（"Run" が "Run kiroku …" の中にあるなど）。
// i18n.js のうち JA_TEXT の外（localizeTitles が Go の組み立てた題を引く t("Kiro Crew memory consolidation ({slot})") など）も見る
func TestI18nNoUnusedKeys(t *testing.T) {
	b, err := jsFiles.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ReplaceAll(string(b), "\r\n", "\n") // Windows のチェックアウトは CRLF
	start := strings.Index(src, "const JA_TEXT = {\n")
	end := strings.Index(src, "\n};\n")
	if start < 0 || end < start {
		t.Fatal("i18n.js に JA_TEXT が見つからない")
	}
	dict := src[start:end]
	var other strings.Builder
	other.WriteString(src[:start])
	other.WriteString(src[end:])
	tmpl, err := os.ReadFile("template.html")
	if err != nil {
		t.Fatal(err)
	}
	other.Write(tmpl)
	names, _ := filepath.Glob("js/*.js")
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
		other.WriteByte('\n')
		other.Write(b)
	}
	all := other.String()
	used := func(k string) bool {
		for _, q := range []string{`"`, `'`, "`"} {
			if strings.Contains(all, q+k+q) {
				return true
			}
		}
		// テンプレートの要素の文字は、テンプレートでだけ見る（Go が書くほかのページの <p>Run <code>… を数えない）
		return regexp.MustCompile(`>\s*` + regexp.QuoteMeta(k) + `\s*<`).Match(tmpl)
	}
	keys := regexp.MustCompile(`(?m)^  "((?:[^"\\]|\\.)*)": `).FindAllStringSubmatch(dict, -1)
	if len(keys) < 100 {
		t.Fatalf("JA_TEXT の鍵が %d 個しか読めない", len(keys))
	}
	// 同じ鍵が 2 つあると、あとのほうが黙って勝つ（前の訳は使われない）
	seen := map[string]bool{}
	for _, m := range keys {
		if seen[m[1]] {
			t.Errorf("JA_TEXT に同じ鍵が 2 つある: %q", m[1])
		}
		seen[m[1]] = true
	}
	var unused []string
	for _, m := range keys {
		raw := m[1]
		var plain string
		if err := json.Unmarshal([]byte(`"`+raw+`"`), &plain); err != nil {
			plain = raw
		}
		if !used(raw) && !used(plain) {
			unused = append(unused, plain)
		}
	}
	if len(unused) > 0 {
		t.Errorf("JA_TEXT に使われていない鍵が %d 個ある（消すか、使う場所を直す）:\n%s", len(unused), strings.Join(unused, "\n"))
	}
}

// 読み込み中の画面（loading.html）は、本物の画面（state.js の LANG）と同じ言語になる。?lang=、--lang、ブラウザの言語の順で、
// 以前の版の切り替えボタンが localStorage に残した言語（kiroku:lang）は、どちらも使わない
func TestLoadingLangMatchesView(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	state, err := jsFiles.ReadFile("js/state.js")
	if err != nil {
		t.Fatal(err)
	}
	lf := func(x string) string { return strings.ReplaceAll(x, "\r\n", "\n") } // Windows のチェックアウトは CRLF
	view := regexp.MustCompile(`(?s)const LANGS = .*?\}\)\(\);\n`).FindString(lf(string(state)))
	pick := regexp.MustCompile(`(?s)const pick = \(\) => \{.*?\};\n  const JA = `).FindString(lf(loading))
	if view == "" || pick == "" {
		t.Fatal("state.js の LANG か loading.html の pick が見つからない")
	}
	pick = strings.TrimSuffix(pick, "\n  const JA = ")
	args, _ := json.Marshal([]string{view + "return LANG;", pick + "return pick();"})
	code := `const [view, pick] = ` + string(args) + `;
const viewLang = new Function("location", "navigator", "localStorage", "__LANG__", view);
const loadLang = new Function("location", "navigator", "localStorage", "document", pick);
let bad = 0;
for (const q of ["", "?lang=ja", "?lang=en", "?lang=fr"]) for (const flag of ["", "en", "ja"]) for (const nav of ["en-US", "ja-JP", "ja", ""]) for (const saved of [null, "ja", "en"]) {
  const loc = {search: q}, nv = {languages: nav ? [nav] : [], language: nav}, ls = {getItem: k => k === "kiroku:lang" && saved ? JSON.stringify(saved) : null};
  const v = viewLang(loc, nv, ls, flag), l = loadLang(loc, nv, ls, {documentElement: {dataset: {lang: flag}}});
  const want = ["en", "ja"].includes(q.slice(6)) ? q.slice(6) : flag || (nav.startsWith("ja") ? "ja" : "en");
  if (v !== want || l !== want) { bad++; console.error(JSON.stringify({q, flag, nav, saved}), "view", v, "loading", l, "want", want); }
}
if (bad) process.exit(1);
`
	f := filepath.Join(t.TempDir(), "lang.js")
	if err := os.WriteFile(f, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("読み込み中の画面と本物の画面で言語が違う:\n%s", out)
	}
}

// ページのいちばん下のリンク先（repoURL）。META.repo が https://HOST/OWNER/REPO の形のときだけ使い、
// それ以外（javascript: や、ほかの形、文字列でないもの）は公開のリポジトリにする。
func TestRepoURL(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	def := regexp.MustCompile(`(?m)^const REPO_DEFAULT = .*;$`).FindString(text)
	fn := regexp.MustCompile(`(?ms)^function repoURL\(\)\{.*?^\}$`).FindString(text)
	if def == "" || fn == "" {
		t.Fatal("panels.js に REPO_DEFAULT か repoURL が見つからない")
	}
	cases := `let META;
const up = "https://github.com/MichinaoShimizu/kiroku";
const want = (meta, w) => { META = meta; const got = repoURL(); if (got !== w) { console.error(JSON.stringify(meta), "→", got, "want", w); process.exitCode = 1; } };
want(null, up);
want({}, up);
want({repo: "https://github.com/corp/kiroku"}, "https://github.com/corp/kiroku");
want({repo: "https://corp.ghe.com/team/kiroku-dist"}, "https://corp.ghe.com/team/kiroku-dist");
for (const bad of ["javascript:alert(1)", "http://github.com/corp/kiroku", "https://github.com/corp/kiroku/", "https://github.com/corp/kiroku?x=1",
  "https://github.com/corp/kiroku\"onmouseover=x", "https://github.com/corp", "https://github.com/corp/kiroku/extra", "https://-x/corp/kiroku", "https://github.com/corp/ki roku", 42, ["https://github.com/corp/kiroku"]])
  want({repo: bad}, up);
`
	f := filepath.Join(t.TempDir(), "repo.js")
	if err := os.WriteFile(f, []byte(def+"\n"+fn+"\n"+cases), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("repoURL が違う:\n%s", out)
	}
}

// 「Expensive models for light work」（lightOf）が、主なモデルが Opus 系か Fable 系で、プロンプト 3 件以下・編集なし・$0.3 以上の
// セッションを拾うこと（#569）。review.js から mainModel・pricey・lightOf だけを取り出し、DATA と inP を置き換えて Node で動かす。
func TestLightOf(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/review.js")
	if err != nil {
		t.Fatal(err)
	}
	var code []string
	for _, re := range []string{`(?m)^const mainModel = .*$`, `(?m)^const pricey = .*$`, `(?ms)^function lightOf\(ws, we\)\{.*?; \}$`} {
		m := regexp.MustCompile(re).FindString(strings.ReplaceAll(string(src), "\r\n", "\n"))
		if m == "" {
			t.Fatalf("review.js に %s が見つからない", re)
		}
		code = append(code, m)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "lightof_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "lightof.js")
	if err := os.WriteFile(f, append([]byte(strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("lightOf が違う:\n%s", out)
	}
}

// 最後のセッションの終わり（calendar.js の lastEnd。記録のない週・月から、記録のある最後の期間へ行くボタン）。
// 25 万セッションでも動くこと（Math.max(...DATA.map(…)) は引数が多すぎて RangeError になっていた）
func TestLastEnd(t *testing.T) {
	src, err := jsFiles.ReadFile("js/calendar.js")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(src), "\r\n", "\n")
	if regexp.MustCompile(`Math\.(max|min)\(\.\.\.DATA\b`).MatchString(s) {
		t.Error("calendar.js spreads DATA into Math.max/min, which throws RangeError with ~200k sessions; use lastEnd")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	m := regexp.MustCompile(`(?m)^const lastEnd = .*$`).FindString(s)
	if m == "" {
		t.Fatal("calendar.js に lastEnd が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "lastend_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "lastend.js")
	if err := os.WriteFile(f, append([]byte(m+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("lastEnd が違う:\n%s", out)
	}
}

// グラフの軸の名前を見た目の幅で切る（format.js の dispW・fitW）。日本語の名前を文字の数で切ると、英語のほぼ 2 倍の幅になって隣と重なった
func TestFitW(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/format.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	code := regexp.MustCompile(`(?ms)^const WIDE = .*?^function fitW\(s, max\)\{.*?return o \+ "…"; \}$`).FindString(text)
	if code == "" {
		t.Fatal("format.js に WIDE から fitW までが見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "fitw_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "fitw.js")
	if err := os.WriteFile(f, append([]byte(code+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("dispW・fitW が違う:\n%s", out)
	}
}

// いつもの範囲の両端（insight.js の insLoHi）。丸めた端が値をまたがないこと（「11.7：いつもの 9–12 より上」としない）、
// 両端の小数の桁がそろうこと（「50.8%–64%」としない）。insLoHi・insEnds・insExpr を取り出し、MET と fmtM は置き換える。insBase が付ける b.fmt があっても桁がそろうこと
func TestLoHi(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/insight.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	var m string
	for _, re := range []string{`(?ms)^function insLoHi\(fm, b\)\{.*?^\}$`, `(?ms)^function insEnds\(fm, b\)\{.*?^\}$`, `(?ms)^function insExpr\(b\)\{.*?^\}$`} {
		x := regexp.MustCompile(re).FindString(text)
		if x == "" {
			t.Fatalf("insight.js に %s が見つからない", re)
		}
		m += x + "\n"
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "lohi_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "lohi.js")
	if err := os.WriteFile(f, append([]byte(i18nStub+m), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("insLoHi が違う:\n%s", out)
	}
}

// 文の途中に入れる指標の名前（i18n.js の midName）。「No stretch where Estimated cost rose…」と大文字のまま入れない
func TestMidName(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^const midName = .*$`).FindString(strings.ReplaceAll(string(src), "\r\n", "\n"))
	if m == "" {
		t.Fatal("i18n.js に midName が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "midname_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "midname.js")
	if err := os.WriteFile(f, append([]byte("let ja = false;\nconst isJA = () => ja;\n"+m+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("midName が違う:\n%s", out)
	}
}

// PR の短い名前（prName・ghPull）は、行き先が https://github.com/ のときだけ。https://evil.example/github.com/o/r/pull/1 のような
// URL を「r#1」と見せて別のホストへ飛ばさないこと。session.js から 2 つの関数だけを取り出して Node で動かす。
func TestPRName(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/session.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	gh := regexp.MustCompile(`(?m)^function ghPull\(url\)\{.*\}$`).FindString(text)
	name := regexp.MustCompile(`(?ms)^function prName\(url\)\{.*?^\}$`).FindString(text)
	if gh == "" || name == "" {
		t.Fatal("session.js に ghPull・prName が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "prname_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "prname.js")
	if err := os.WriteFile(f, append([]byte(gh+"\n"+name+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("PR の名前が違う:\n%s", out)
	}
}

// 「前回の続き」（leftOff）。コミットに入ったファイル・今日のセッションが続きをしたファイル・git を読めていないプロジェクトを出さないこと。
// pickup.js はほかのファイルに頼るので、PICK_DAYS・pickPath・leftOff だけを取り出して Node で動かす。
func TestLeftOff(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/pickup.js")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	var code []string
	for _, re := range []string{`(?m)^const PICK_DAYS = .*$`, `(?ms)^function pickPath\(.*?^\}$`, `(?ms)^function leftOff\(.*?^\}$`} {
		m := regexp.MustCompile(re).FindString(text)
		if m == "" {
			t.Fatalf("js/pickup.js に %s が見つからない", re)
		}
		code = append(code, m)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "pickup_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "pickup.js")
	if err := os.WriteFile(f, append([]byte(strings.Join(code, "\n")+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, f)
	cmd.Env = append(os.Environ(), "TZ=UTC")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("leftOff の確かめに失敗:\n%s", out)
	}
}
