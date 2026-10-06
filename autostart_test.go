package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// fakeAutostart は、ホーム・OS・launchctl / systemctl・待ち受けの確認を差しかえ、実行したコマンドを返す。
func fakeAutostart(t *testing.T, goos string, running bool) (home string, ran *[]string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	oldOS, oldExe, oldHome, oldRun, oldProbe, oldRetry := autostartGOOS, autostartExe, autostartHome, runCmd, probe, autostartRetry
	t.Cleanup(func() {
		autostartGOOS, autostartExe, autostartHome, runCmd, probe, autostartRetry = oldOS, oldExe, oldHome, oldRun, oldProbe, oldRetry
	})
	autostartHome = func() (string, error) { return home, nil }
	autostartRetry = 0
	autostartGOOS = goos
	autostartExe = func() (string, error) { return filepath.Join(home, "bin", "kiroku"), nil }
	var cmds []string
	runCmd = func(name string, args ...string) error {
		cmds = append(cmds, name+" "+strings.Join(args, " "))
		return nil
	}
	probe = func(string) bool { return running }
	return home, &cmds
}

// macOS は launchd の plist、Linux は systemd --user のユニット。引数と環境変数は 1 語のまま、記号もそのまま入る。
func TestPlanAutostart(t *testing.T) {
	env := map[string]string{"PATH": "/opt/homebrew/bin:/usr/bin", "CLAUDE_CONFIG_DIR": "/Users/me/my claude"}
	p, err := planAutostart("darwin", "/Users/me", "/Users/me/bin/kiroku & co", []string{":8485"}, env)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<string>/Users/me/bin/kiroku &amp; co</string>", "<string>serve</string>", "<string>--no-open</string>", "<string>:8485</string>",
		"<key>CLAUDE_CONFIG_DIR</key><string>/Users/me/my claude</string>", "<key>RunAtLoad</key><true/>", xmlText(filepath.Join("/Users/me", "Library", "Logs", "kiroku.log"))} {
		if !strings.Contains(p.content, want) {
			t.Errorf("plist に %q がない:\n%s", want, p.content)
		}
	}
	if p.path != filepath.Join("/Users/me", "Library", "LaunchAgents", autostartLabel+".plist") {
		t.Errorf("plist の場所: %s", p.path)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	p, err = planAutostart("linux", "/home/me", "/home/me/my bin/kiroku", nil, map[string]string{"PATH": "/usr/bin", "CODEX_HOME": "/home/me/100%$x"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`ExecStart="/home/me/my bin/kiroku" "serve" "--no-open"` + "\n", `Environment="CODEX_HOME=/home/me/100%%$$x"`, "WantedBy=default.target"} {
		if !strings.Contains(p.content, want) {
			t.Errorf("ユニットに %q がない:\n%s", want, p.content)
		}
	}
	if p.path != filepath.Join("/home/me", ".config", "systemd", "user", "kiroku.service") {
		t.Errorf("ユニットの場所: %s", p.path)
	}

	if _, err := planAutostart("windows", `C:\Users\me`, `C:\bin\kiroku.exe`, nil, nil); err == nil || !strings.Contains(err.Error(), "shell:startup") {
		t.Errorf("Windows は Startup フォルダを案内する: %v", err)
	}
}

// kiroku autostart on で設定を書いて登録し、off で外して消す。登録に失敗したら設定を残さない。
func TestAutostartCommand(t *testing.T) {
	home, ran := fakeAutostart(t, "linux", false)
	unit := filepath.Join(home, ".config", "systemd", "user", "kiroku.service")
	if err := dispatch([]string{"autostart", "on", ":8485"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(unit)
	if err != nil || !strings.Contains(string(b), `"serve" "--no-open" ":8485"`) {
		t.Fatalf("ユニットが書かれていない: %v\n%s", err, b)
	}
	if got := strings.Join(*ran, "\n"); !strings.Contains(got, "systemctl --user enable kiroku.service") || !strings.Contains(got, "systemctl --user restart kiroku.service") {
		t.Errorf("登録していない:\n%s", got)
	}
	if s := autostartStatus(); !s.on || s.running || s.url != "http://localhost:8485/" {
		t.Errorf("状態: %+v", s)
	}
	*ran = nil
	if err := dispatch([]string{"autostart", "off"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unit); !os.IsNotExist(err) {
		t.Error("off でユニットが消えていない")
	}
	if got := strings.Join(*ran, "\n"); !strings.Contains(got, "systemctl --user disable --now kiroku.service") {
		t.Errorf("登録を外していない:\n%s", got)
	}
	if s := autostartStatus(); s.on {
		t.Errorf("off のあとも on: %+v", s)
	}

	*ran = nil
	runCmd = func(name string, args ...string) error {
		*ran = append(*ran, name+" "+strings.Join(args, " "))
		if strings.Contains(strings.Join(args, " "), "restart") {
			return os.ErrPermission
		}
		return nil
	}
	if err := dispatch([]string{"autostart", "on"}); err == nil {
		t.Error("登録に失敗したのにエラーにならない")
	}
	if got := strings.Join(*ran, "\n"); strings.Count(got, "restart kiroku.service") != 4 || !strings.HasSuffix(got, "systemctl --user disable --now kiroku.service") {
		t.Errorf("何回か試してから、途中までの登録を外していない:\n%s", got)
	}
	if _, err := os.Stat(unit); !os.IsNotExist(err) {
		t.Error("登録に失敗したのにユニットが残っている")
	}
	if err := dispatch([]string{"autostart", "of"}); err == nil {
		t.Error("打ちまちがいがエラーにならない")
	}
}

// kiroku doctor：消える設定のままなら「!」で設定の場所と kiroku archive on を案内し、archive がオンなら安心と出す。
func TestDoctorReport(t *testing.T) {
	fakeAutostart(t, "darwin", false)
	old := lookGit
	t.Cleanup(func() { lookGit = old })
	lookGit = func() bool { return false }
	dir := t.TempDir()
	rep := []source.Report{
		{Name: "Claude Code", N: 12, Oldest: 1.78e9, Where: "/x/projects", Keep: &source.Retention{Days: 30, Setting: "cleanupPeriodDays", Docs: "https://docs", File: "/x/settings.json"}},
		{Name: "Codex", Where: "/x/.codex"},
	}
	var b bytes.Buffer
	doctorReport(&b, rep, 12, dir)
	out := b.String()
	for _, want := range []string{"✓ Claude Code", "12 sessions, since", "· Codex", "none",
		`! Claude Code deletes history older than 30 days`, `add "cleanupPeriodDays": 3650 to /x/settings.json`, `Run "kiroku archive on"`,
		"git: not found", `autostart: off`, "kiroku archive on    keep copies", "kiroku serve"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	if err := dispatch([]string{"archive", "on", "--archive-dir", dir, "--root", t.TempDir(), "--sources", "claude"}); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	doctorReport(&b, rep, 12, dir)
	if out := b.String(); !strings.Contains(out, "but kiroku archive keeps a copy") || strings.Contains(out, "! Claude Code") {
		t.Errorf("archive がオンなのに消えると出ている:\n%s", out)
	}
	b.Reset()
	doctorReport(&b, []source.Report{{Name: "Codex", Where: "/x"}}, 0, dir)
	if out := b.String(); !strings.Contains(out, "No history found") {
		t.Errorf("履歴がないときの案内がない:\n%s", out)
	}
}

// --week / --month は this・last・日付で選べ、週はその日を含む月曜からの 7 日になる。
func TestParsePeriod(t *testing.T) {
	setup(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.Local) // 水曜
	for _, c := range []struct{ week, month, key, file string }{
		{"this", "", "2026-10-05", "kiroku-2026-10-05.html"},
		{"last", "", "2026-09-28", "kiroku-2026-09-28.html"},
		{"2026-09-03", "", "2026-08-31", "kiroku-2026-08-31.html"},
		{"", "this", "2026-10", "kiroku-2026-10.html"},
		{"", "last", "2026-09", "kiroku-2026-09.html"},
		{"", "2026-01", "2026-01", "kiroku-2026-01.html"},
	} {
		p, err := parsePeriod(c.week, c.month, now)
		if err != nil {
			t.Fatal(err)
		}
		if p.key != c.key || p.file() != c.file {
			t.Errorf("%q %q → %s %s", c.week, c.month, p.key, p.file())
		}
	}
	if p, _ := parsePeriod("", "", now); p != nil {
		t.Error("何も選ばなければ nil")
	}
	for _, c := range [][2]string{{"10/5", ""}, {"", "2026-13"}, {"this", "this"}} {
		if _, err := parsePeriod(c[0], c[1], now); err == nil {
			t.Errorf("%v がエラーにならない", c)
		}
	}
}

// 期間だけの集計には、期間に重なるセッションと、期間のうちのコミット・push しか入らない。件数も期間の分になる。
func TestScoped(t *testing.T) {
	setup(t)
	at := func(m time.Month, d, h int) float64 {
		return float64(time.Date(2026, m, d, h, 0, 0, 0, time.Local).Unix())
	}
	sess := func(id string, a, b float64) *core.Session {
		return &core.Session{ID: id, Project: "app", Start: a, End: b, Segs: [][3]float64{{a, b, 10}}}
	}
	data := []*core.Session{
		sess("before", at(9, 21, 10), at(9, 21, 11)),
		sess("cross", at(10, 4, 23), at(10, 5, 1)), // 日曜の夜から月曜にまたがる
		sess("in", at(10, 7, 10), at(10, 7, 12)),
		sess("after", at(10, 13, 10), at(10, 13, 11)),
	}
	commits := []gitlog.Commit{{Hash: "a", T: at(9, 21, 10), Project: "app"}, {Hash: "b", T: at(10, 7, 11), Project: "app"}}
	pushes := []gitlog.Push{{T: at(10, 7, 11), Project: "app"}, {T: at(10, 13, 11), Project: "app"}}
	rep := []source.Report{{Name: "Claude Code", N: 4, Oldest: at(9, 21, 10), IDs: []string{"before", "cross", "in", "after"},
		Keep: &source.Retention{Days: 30, Setting: "cleanupPeriodDays"}}, {Name: "Codex", Where: "/x/.codex"}}
	snap := snapshot{data: data, rep: rep, meta: map[string]any{"git": commits, "push": pushes, "report": rep, "archive": map[string]any{"dir": "/Users/me/kept"}}}
	p, _ := parsePeriod("2026-10-07", "", time.Now())
	got := scoped(snap, p)
	var ids []string
	for _, d := range got.data {
		ids = append(ids, d.ID)
	}
	if strings.Join(ids, ",") != "cross,in" {
		t.Errorf("セッション: %v", ids)
	}
	if cs := got.meta["git"].([]gitlog.Commit); len(cs) != 1 || cs[0].Hash != "b" {
		t.Errorf("コミット: %+v", cs)
	}
	if ps := got.meta["push"].([]gitlog.Push); len(ps) != 1 {
		t.Errorf("push: %+v", ps)
	}
	for k := range got.weeks {
		if k != "2026-10-05" && k != "2026-09-28" {
			t.Errorf("期間の外の週: %s", k)
		}
	}
	if got.weeks["2026-10-05"] == nil {
		t.Error("期間の週の集計がない")
	}
	if rs := got.meta["report"].([]source.Report); len(rs) != 1 || rs[0].N != 2 || rs[0].Oldest != 0 || rs[0].Keep != nil {
		t.Errorf("Data sources（使っていないエージェントは出さない）: %+v", rs)
	}
	if _, ok := got.meta["archive"]; ok {
		t.Error("kiroku archive の保存場所が残っている")
	}
	if snap.rep[0].N != 4 || len(snap.data) != 4 {
		t.Error("元の集計を書きかえている")
	}
	sc := got.meta["scope"].(map[string]any)
	if sc["mode"] != "week" || sc["key"] != "2026-10-05" || sc["offset"] != 9*3600 || sc["zone"] != "JST" {
		t.Errorf("scope: %v", sc)
	}
}

// 期間に履歴がなければ、履歴のある範囲を添えてエラーにする（ファイルは書かない）。
func TestHTMLEmptyPeriod(t *testing.T) {
	setup(t)
	h := filepath.Join("testdata", "home")
	out := filepath.Join(t.TempDir(), "w.html")
	err := dispatch([]string{"html", "--no-open", "--root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--sources", "claude,kiro", "--week", "2026-08-03", "-o", out})
	if err == nil || !strings.Contains(err.Error(), "no history in the week of 2026-08-03 (your history covers ") {
		t.Errorf("エラー: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("履歴がないのにファイルを書いた")
	}
}
