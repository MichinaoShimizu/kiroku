package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// コンパクションは時刻の順に並べ、1 分以内に続いたものは 1 回と数える。種類は auto・manual だけを残し、
// どれもわからなければ JSON に compactKinds を出さない。コンパクションのないセッションは compactions も出さない。
func TestBuilderCompact(t *testing.T) {
	at := func(v float64) *float64 { return &v }
	b := NewBuilder("x", "s1")
	b.Tick(at(1000))
	b.Compact(at(5000), "")
	b.Compact(at(5030), "manual") // 同じ 1 回（あとの行の種類で埋める）
	b.Compact(at(2000), "auto")   // 時刻の順は前後してもよい
	b.Compact(at(9000), "<b>")
	b.Compact(nil, "auto")
	s := b.Finish(15)
	if got := s.Compactions; len(got) != 3 || got[0] != 2000 || got[1] != 5000 || got[2] != 9000 {
		t.Errorf("compactions = %v, want [2000 5000 9000]", got)
	}
	if got := strings.Join(s.CompactKinds, "|"); got != "auto|manual|" {
		t.Errorf("compactKinds = %q, want %q", got, "auto|manual|")
	}

	b = NewBuilder("x", "s2")
	b.Tick(at(1000))
	b.Compact(at(2000), "")
	raw, _ := json.Marshal(b.Finish(15))
	if !strings.Contains(string(raw), `"compactions":[2000]`) || strings.Contains(string(raw), "compactKinds") {
		t.Errorf("種類がわからなければ compactKinds を出さない: %s", raw)
	}

	b = NewBuilder("x", "s3")
	b.Tick(at(1000))
	raw, _ = json.Marshal(b.Finish(15))
	if strings.Contains(string(raw), "compact") {
		t.Errorf("コンパクションがなければ出さない: %s", raw)
	}
}
