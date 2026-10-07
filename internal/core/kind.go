package core

import (
	"regexp"
	"strings"
)

// 人が打ったプロンプトの種類（Prompt.Kind）。"" はふつうに書いたもの。
const (
	KindCommand = "command" // スラッシュコマンド（/review など）
	KindShell   = "shell"   // ! で打ったシェルのコマンド
)

// Note は、人ではなくエージェントや仕組みが会話に入れたもの（通知・要約・hook の出力など）。
// プロンプトには数えず、プロンプトの流れに別の色で出す。
type Note struct {
	T    *float64 `json:"t"`
	Kind string   `json:"kind"` // reminder・notice・hook・output・compact・agent・meta・other
	Text string   `json:"text"`
}

// NoteRunes は Note の文を、HTML にどこまで入れるか。maxNotes は 1 セッションで残す数の上限。
const (
	NoteRunes = 160
	maxNotes  = 300
)

var (
	reminderRe = regexp.MustCompile(`(?s)<system-reminder>(.*?)</system-reminder>`)
	cmdNameRe  = regexp.MustCompile(`(?s)<command-name>\s*(.*?)\s*</command-name>`)
	cmdArgsRe  = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
	bashInRe   = regexp.MustCompile(`(?s)<bash-input>(.*?)</bash-input>`)
	leadTagRe  = regexp.MustCompile(`^<([a-z][a-z0-9_-]*)[\s>]`)
	anyTagRe   = regexp.MustCompile(`</?[a-z][a-z0-9_-]*[^>]*>`)
	tagKinds   = map[string]string{
		"task-notification": "notice", "user-prompt-submit-hook": "hook",
		"local-command-stdout": "output", "local-command-stderr": "output", "bash-stdout": "output", "bash-stderr": "output",
		"command-message": "meta",
	}
)

type part struct{ kind, text string }

// splitUser は user の発言を、人が打ったプロンプト（なければ nil）と、会話に入れられたもの（notes）に分ける。
// プロンプトに埋め込まれた <system-reminder> は外して notes に回す。
func splitUser(text string) (p *part, notes []part) {
	t := strings.TrimSpace(text)
	if t == "" || strings.HasPrefix(t, "[Request interrupted") { // 中断は別に数える
		return nil, nil
	}
	for _, m := range reminderRe.FindAllStringSubmatch(t, -1) {
		notes = append(notes, part{"reminder", m[1]})
	}
	if t = strings.TrimSpace(reminderRe.ReplaceAllString(t, "")); t == "" {
		return nil, notes
	}
	if m := cmdNameRe.FindStringSubmatch(t); m != nil { // スラッシュコマンドは、人が打ったプロンプトとして「/名前 引数」で残す
		name := m[1]
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		if a := cmdArgsRe.FindStringSubmatch(t); a != nil {
			name += " " + strings.TrimSpace(a[1])
		}
		return &part{KindCommand, strings.TrimSpace(name)}, notes
	}
	if m := bashInRe.FindStringSubmatch(t); m != nil {
		return &part{KindShell, "! " + strings.TrimSpace(m[1])}, notes
	}
	if strings.HasPrefix(t, "Caveat:") {
		return nil, append(notes, part{"meta", t})
	}
	if m := leadTagRe.FindStringSubmatch(t); m != nil {
		k := tagKinds[m[1]]
		if k == "" {
			k = "other"
		}
		return nil, append(notes, part{k, t})
	}
	return &part{"", t}, notes
}

// noteText は Note に入れる文を、タグを外して 1 行にし、短くする。
func noteText(t string) string {
	return Runes(strings.Join(strings.Fields(anyTagRe.ReplaceAllString(t, " ")), " "), NoteRunes)
}

// Inject は、エージェントや仕組みが会話に入れたものを残す（プロンプトには数えない）。
func (s *Builder) Inject(ts *float64, kind, text string) {
	if text = noteText(text); text == "" || len(s.Notes) >= maxNotes {
		return
	}
	s.Notes = append(s.Notes, Note{T: ts, Kind: kind, Text: text})
}

// InjectAll は、まとめて入れられたもの（isMeta の発言など）を残す。中に人の文に見える部分があっても kind として扱う。
func (s *Builder) InjectAll(ts *float64, kind, text string) {
	p, notes := splitUser(text)
	for _, n := range notes {
		s.Inject(ts, n.kind, n.text)
	}
	if p != nil {
		s.Inject(ts, kind, p.text)
	}
}
