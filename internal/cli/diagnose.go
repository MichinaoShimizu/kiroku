package cli

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// 履歴の形の手がかり（ADR 0032）。kiroku は何も送らない。kiroku doctor --report が書いた文を、利用者が読んで、
// 自分で GitHub の issue に貼る。中身は種類の名前と数と版だけで、パス・プロジェクト名・ID・発言は入れない。

// formatShown は、Format check と --report に出すエージェントか（セッションがあるか、形の手がかりがあるもの）。
func formatShown(r source.Report) bool {
	return r.N > 0 || r.BadLines > 0 || len(r.Unknown) > 0 || r.AgentVersion != ""
}

// unchecked は、形を調べていないエージェント（FormatChecker でなく、手がかりも何もない）か。
func unchecked(r source.Report) bool {
	return !r.Checked && r.BadLines == 0 && len(r.Unknown) == 0 && r.AgentVersion == ""
}

// kindList は知らない種類を「名前 数」で、数の多い順（同じなら名前の順）に並べる。名前はもう一度 core.SafeKind で確かめる。
func kindList(unknown map[string]int) string {
	all := map[string]int{}
	for k, n := range unknown {
		if n > 0 {
			all[core.SafeKind(k)] += n
		}
	}
	names := make([]string, 0, len(all))
	for k := range all {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if all[names[i]] != all[names[j]] {
			return all[names[i]] > all[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, len(names))
	for i, k := range names {
		parts[i] = fmt.Sprintf("%s %d", k, all[k])
	}
	return strings.Join(parts, ", ")
}

// formatCheck は kiroku doctor の「Format check」。エージェントごとに、履歴にあったいちばん新しい版、読めなかった行、知らない記録の種類。
func formatCheck(w io.Writer, rep []source.Report) {
	s := styleFor(w)
	fmt.Fprintf(w, "\n%s\n", s.bold("Format check"))
	shown, odd := 0, false
	for _, r := range rep {
		if !formatShown(r) {
			continue
		}
		shown++
		if unchecked(r) {
			fmt.Fprintf(w, "  %s %-18s %s\n", s.dim("·"), r.Name, s.dim("format not checked for this agent yet"))
			continue
		}
		var what []string
		if v := core.SafeVersion(r.AgentVersion); v != "" {
			what = append(what, "newest version seen "+v)
		}
		mark := s.ok("✓")
		if r.BadLines > 0 {
			mark, odd = s.warn("!"), true
			what = append(what, plural(r.BadLines, "unreadable line"))
		}
		switch {
		case len(r.Unknown) > 0:
			mark, odd = s.warn("!"), true
			what = append(what, "unknown kinds: "+kindList(r.Unknown))
		case r.Checked:
			what = append(what, "no unknown kinds")
		}
		fmt.Fprintf(w, "  %s %-18s %s\n", mark, r.Name, strings.Join(what, "; "))
	}
	if shown == 0 {
		fmt.Fprintf(w, "  %s %s\n", s.dim("·"), s.dim("no history to check"))
	}
	if odd {
		fmt.Fprintf(w, "      %s\n", s.dim("unknown kinds can mean the agent changed its history format; please report them with \"kiroku doctor --report\" (review it before you share it)"))
	}
}

// diagnostics は kiroku doctor --report の文（GitHub の issue に貼るためのもの）。kiroku の版、OS、エージェントごとの
// セッションの数・いちばん新しい版・読めなかった行・知らない種類と数だけを書く。パス・プロジェクト名・ID・中身・ホームのフォルダは書かない。
// 画面の「Copy diagnostics」（internal/web/js）も同じ形の文を作る。
func diagnostics(w io.Writer, rep []source.Report) {
	fmt.Fprintln(w, "kiroku diagnostics (kinds and counts only)")
	fmt.Fprintf(w, "kiroku: %s\n", kirokuVersion())
	fmt.Fprintf(w, "os: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	n := 0
	for _, r := range rep {
		if !formatShown(r) {
			continue
		}
		n++
		if unchecked(r) {
			fmt.Fprintf(w, "%s: sessions %d; format not checked\n", r.Name, r.N)
			continue
		}
		v := core.SafeVersion(r.AgentVersion)
		if v == "" {
			v = "unknown"
		}
		kinds := "none"
		if len(r.Unknown) > 0 {
			kinds = kindList(r.Unknown)
		}
		fmt.Fprintf(w, "%s: sessions %d; version %s; unreadable lines %d; unknown kinds: %s\n", r.Name, r.N, v, r.BadLines, kinds)
	}
	if n == 0 {
		fmt.Fprintln(w, "no history found")
	}
}

// kirokuVersion は、動いている kiroku の版（v0.31.0 の形。手元でビルドしたものは dev）。
func kirokuVersion() string {
	if version == "dev" {
		return version
	}
	return "v" + strings.TrimPrefix(version, "v")
}
