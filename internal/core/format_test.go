package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// 履歴から読んだ種類の名前は、英数字と _.:- の 40 文字までだけを出す。パス・発言・空白・日本語・長いものは other にまとめる。
func TestSafeKind(t *testing.T) {
	for in, want := range map[string]string{
		"file-history-snapshot": "file-history-snapshot",
		"event_msg:foo.bar":     "event_msg:foo.bar",
		"":                      OtherKind,
		"/Users/alice/secret":   OtherKind,
		"fix the login bug":     OtherKind,
		"秘密":                    OtherKind,
		"<script>":              OtherKind,
		"a\nb":                  OtherKind,
		"x23456789012345678901234567890123456789012": OtherKind,
	} {
		if got := SafeKind(in); got != want {
			t.Errorf("SafeKind(%q) = %q, want %q", in, got, want)
		}
	}
}

// 版は数字とドットだけの短い形にする。うしろの -alpha.1 などは落とし、版の形でないものは使わない。
func TestSafeVersion(t *testing.T) {
	for in, want := range map[string]string{
		"2.1.300": "2.1.300", "v0.50.0": "0.50.0", "0.47.0-alpha.1": "0.47.0", "1.2.3+build": "1.2.3",
		"": "", "latest": "", "1..2": "", "1.2.3.4.5": "", "/Users/alice": "", "1234567.1": "", "2.1.300 (Claude Code)": "2.1.300",
	} {
		if got := SafeVersion(in); got != want {
			t.Errorf("SafeVersion(%q) = %q, want %q", in, got, want)
		}
	}
	if !NewerVersion("2.1.10", "2.1.9") || NewerVersion("2.1.9", "2.1.10") || !NewerVersion("1.0", "") || NewerVersion("1.0", "1.0.0") {
		t.Error("NewerVersion は数字の部分ごとに比べる")
	}
}

// 知らない種類は 1 つのエージェントで 20 まで名前を出し、残りは other にまとめる。版はいちばん新しいものを残す。
func TestFormatCaps(t *testing.T) {
	var f Format
	for i := range 30 {
		f.Kind(fmt.Sprintf("kind%02d", i))
	}
	f.Kind("/secret/path")
	if len(f.Unknown) != MaxKinds+1 || f.Unknown[OtherKind] != 11 {
		t.Errorf("Unknown = %v（20 種類と other）", f.Unknown)
	}
	f.SawVersion("2.1.9")
	f.SawVersion("2.1.10")
	f.SawVersion("2.1.2")
	f.SawVersion("not a version")
	if f.Version != "2.1.10" {
		t.Errorf("Version = %q", f.Version)
	}
	var g Format
	g.Merge(&f)
	other := Format{Unknown: map[string]int{"new-kind": 100, "kind00": 1}, Bad: 2, Version: "2.2.0"}
	g.Merge(&other)
	if len(g.Unknown) != MaxKinds+1 || g.Unknown["new-kind"] != 100 || g.Unknown["kind00"] != 2 || g.Bad != 2 || g.Version != "2.2.0" {
		t.Errorf("Merge = %+v", g)
	}
	total := 0
	for _, n := range g.Unknown {
		total += n
	}
	if total != 31+101 {
		t.Errorf("Merge で数が失われた: %d", total)
	}
}

// 読めない行を数える。空白だけの行と、改行で終わらない最後の行（書いている途中）は数えない。
func TestReadJSONLBad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "h.jsonl")
	os.WriteFile(p, []byte("{\"a\":1}\nnot json\n\n   \n[1,2]\n\"str\"\n{\"b\":2}\n{\"half\":"), 0o600)
	n, bad := 0, 0
	if err := ReadJSONLBad(p, func(Obj) { n++ }, &bad); err != nil {
		t.Fatal(err)
	}
	if n != 2 || bad != 3 {
		t.Errorf("objects = %d, bad = %d, want 2 and 3", n, bad)
	}
}
