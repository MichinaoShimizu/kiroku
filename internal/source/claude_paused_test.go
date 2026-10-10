package source

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// 設定のファイルがあるのに読めない・壊れているとき、Claude Code は保存期間を決められないので片付けを止める（settings_unknowable）。
// 組織の設定に正しい値があれば、その値で消す。managed-settings.json が壊れていても、managed-settings.d/ に値があればそれを使う。
func TestClaudeRetentionPausedOnUnreadableSettings(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "projects")
	managed := filepath.Join(home, "managed")
	drop := filepath.Join(managed, "managed-settings.d")
	os.MkdirAll(root, 0o755)
	os.MkdirAll(drop, 0o755)
	old := claudeManagedDir
	claudeManagedDir = managed
	t.Cleanup(func() { claudeManagedDir = old })
	c := &Claude{Root: root}
	user, file := filepath.Join(home, "settings.json"), filepath.Join(managed, "managed-settings.json")
	for _, bad := range []string{`{"cleanupPeriodDays": `, `[1, 2]`, `"text"`, ``} {
		os.WriteFile(user, []byte(bad), 0o644)
		if r := c.Retention(); r != nil {
			t.Errorf("壊れた利用者の設定 %q = %+v, want nil（片付けを止める）", bad, r)
		}
	}
	os.WriteFile(file, []byte(`{"cleanupPeriodDays": 14}`), 0o644)
	if r := c.Retention(); r == nil || r.Days != 14 || r.File != file {
		t.Errorf("利用者の設定が壊れていても組織の設定が勝つ = %+v, want 14 日", r)
	}
	os.WriteFile(user, []byte(`{"cleanupPeriodDays": 90}`), 0o644)
	os.WriteFile(file, []byte(`{"cleanupPeriodDays": `), 0o644)
	if r := c.Retention(); r != nil {
		t.Errorf("壊れた managed-settings.json = %+v, want nil（片付けを止める）", r)
	}
	os.WriteFile(filepath.Join(drop, "10-retention.json"), []byte(`{"cleanupPeriodDays": 7}`), 0o644)
	if r := c.Retention(); r == nil || r.Days != 7 {
		t.Errorf("managed-settings.d/ に値があれば止めない = %+v, want 7 日", r)
	}
	os.WriteFile(filepath.Join(drop, "10-retention.json"), []byte(`{"model": "opus"}`), 0o644)
	os.Remove(file)
	os.WriteFile(filepath.Join(drop, "20-broken.json"), []byte(`{`), 0o644)
	if r := c.Retention(); r != nil {
		t.Errorf("壊れた managed-settings.d/ のファイル = %+v, want nil（片付けを止める）", r)
	}
	os.Remove(filepath.Join(drop, "20-broken.json"))
	if r := c.Retention(); r == nil || r.Days != 90 {
		t.Errorf("ないファイルは読めないファイルではない = %+v, want 利用者の 90 日", r)
	}
	if os.Getuid() != 0 { // root は権限がなくても読める
		os.Chmod(user, 0)
		if r := c.Retention(); r != nil {
			t.Errorf("読めない利用者の設定 = %+v, want nil（片付けを止める）", r)
		}
	}
}

// Claude Code 2.1.284〜2.1.295 は Sonnet 5.5 のキャッシュの読み込みを $0.20/Mtok（公式は $0.10）で数えた。
// その版の cost-state の Sonnet 5.5 の額は使わず、料金表で見積もる。ほかのモデルと、版がわからない記録はそのまま使う。
func TestClaudeCostStateSonnet55CacheReads(t *testing.T) {
	start := `"startTime": 1790730000000` // 2026-09-30T00:20:00Z
	for _, tc := range []struct {
		version string
		want    float64
	}{
		// 料金表: 入力 1,000 × $2 + 読み込み 1,000,000 × $0.10 + 出力 1,000 × $10 = $0.112。opus の記録 0.05 はそのまま
		{`"version":"2.1.290",`, 0.162},
		{`"version":"2.1.296",`, 0.262},
		{``, 0.262},
	} {
		root := t.TempDir()
		dir := filepath.Join(root, "-Users-me-app")
		lines := []string{
			`{"type":"user",` + tc.version + `"timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直して"}}`,
			`{"type":"assistant",` + tc.version + `"timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","usage":{"input_tokens":1000,"cache_read_input_tokens":1000000,"output_tokens":1000},"content":[{"type":"text","text":"ok"}]}}`,
			`{"type":"assistant",` + tc.version + `"timestamp":"2026-09-30T01:02:00Z","message":{"id":"m2","model":"claude-opus-5-5","usage":{"input_tokens":1000,"output_tokens":1000},"content":[{"type":"text","text":"ok"}]}}`,
			`{"type":"cost-state",` + start + `,"modelUsage":{"claude-sonnet-5-5":{"costUSD":0.212,"inputTokens":1000,"cacheReadInputTokens":1000000,"outputTokens":1000},"claude-opus-5-5":{"costUSD":0.05}}}`,
		}
		writeLines(t, filepath.Join(dir, "s1.jsonl"), lines)
		s := load(t, &Claude{Root: root})[0].Finish(15)
		if math.Abs(s.Cost-tc.want) > 1e-9 {
			t.Errorf("%s 目安コスト = %v, want %v", tc.version, s.Cost, tc.want)
		}
	}
}

func TestOverpricedCacheReads(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"2.1.295", true}, {"2.1.284", true}, {"1.0.0", true}, {"2.0.999", true},
		{"2.1.296", false}, {"2.2.0", false}, {"3.0.0", false}, {"", false}, {"x.y.z", false}, {"2.1", false},
	} {
		if got := overpricedCacheReads("claude-sonnet-5-5-20260101", tc.version); got != tc.want {
			t.Errorf("overpricedCacheReads(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
	if overpricedCacheReads("claude-opus-5-5", "2.1.290") {
		t.Error("Opus 5.5 の額は直さない")
	}
}
