// Package source は、エージェントごとの履歴を読んで core.Builder に変えるアダプター。
// 新しいエージェントを足すときは、Source を実装して All に加える。
package source

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Source は 1 種類の履歴。
type Source interface {
	Name() string   // 画面に出す名前（例: "Claude Code"）
	Family() string // --sources で選ぶ名前（例: "claude"）
	Where() string  // 読む場所（計測の状態に出す）
	Load(emit func(*core.Builder)) error
}

// Detailer は計測の状態に一言添えたい Source が実装する。DetailEn は英語表示のときの一言。
type Detailer interface {
	Detail() string
	DetailEn() string
}

// Report は計測の状態に出す、読み込みの結果。
type Report struct {
	Name     string     `json:"name"`
	N        int        `json:"n"`
	Dup      int        `json:"dup,omitempty"`      // ほかの場所と同じ会話だったので数えなかった数
	Archived int        `json:"archived,omitempty"` // 元の履歴が消えていて、kiroku archive のコピーから読んだ数
	Detail   string     `json:"detail,omitempty"`
	DetailEn string     `json:"detailEn,omitempty"` // 英語表示のときの Detail
	Where    string     `json:"where"`
	Error    *string    `json:"error"`
	Oldest   float64    `json:"oldest,omitempty"`    // いちばん古い記録の時刻（UNIX 秒）。これより前は見られない
	Keep     *Retention `json:"retention,omitempty"` // エージェントが履歴を自動で消す設定
	IDs      []string   `json:"-"`                   // 読んだセッションの ID（kiroku html --week などで、期間の分だけ数え直すため）
}

// Retention は、エージェントが古い履歴を自動で消す設定。画面で、過去の分が見られなくなることを知らせ、公式ドキュメントへ案内する。
type Retention struct {
	Days    int    `json:"days"`    // 何日より古いものを消すか（0 はわからない）
	Set     bool   `json:"set"`     // 利用者が設定しているか（false なら既定値のまま）
	Setting string `json:"setting"` // 設定の名前
	Docs    string `json:"docs"`    // 公式ドキュメント
	File    string `json:"-"`       // 設定を書くファイル（kiroku doctor で案内する。わからなければ空）
}

// Retainer は、履歴を自動で消すエージェントの Source が実装する。
type Retainer interface {
	Retention() *Retention
}

// Keeper は、古い履歴を自動で消すエージェントの Source が実装する。kiroku archive がオンなら、
// Src の下の .jsonl を Dst の下に圧縮して残す（internal/archive.Sync）。Source は、元が消えたらコピーを読む。
type Keeper interface {
	Keep() []Kept
}

// Kept は、残す元の場所と、コピーを置く場所。
type Kept struct{ Src, Dst string }

// Options は読み込みの設定。
type Options struct {
	ClaudeRoot   string
	KiroHome     string
	KiroStorages []string // nil なら OS ごとの場所を探す
	CrewHome     string   // 空なら KIROCREW_HOME か <KiroHome>/crew
	KiroCLIDB    string   // 空なら OS ごとの場所
	AmazonQDB    string
	CodexHome    string // 空なら CODEX_HOME か ~/.codex
	Archive      string // kiroku archive の保存場所（空ならコピーを読まない）
}

func q(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// All は対応しているすべての履歴。並びは画面の「計測の状態」の順。
func All(o Options) []Source {
	return []Source{
		&Claude{Root: o.ClaudeRoot, Archive: sub(o.Archive, "claude")},
		&KiroIDE{Home: o.KiroHome},
		&KiroCLI{Home: o.KiroHome, CrewHome: q(o.CrewHome, DefaultCrewHome(o.KiroHome)), CrewArchive: sub(o.Archive, "crew")},
		&QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: q(o.KiroCLIDB, filepath.Join(DataDir("kiro-cli"), "data.sqlite3")), Command: "kiro-cli chat --resume"},
		&KiroIDELegacy{Storages: storages(o.KiroStorages)},
		&QStore{Label: "Amazon Q", Fam: "amazonq", DB: q(o.AmazonQDB, filepath.Join(DataDir("amazon-q"), "data.sqlite3")), Command: "q chat --resume"},
		&Codex{Home: q(o.CodexHome, DefaultCodexHome())},
	}
}

// sub は保存場所 dir の下のフォルダ（dir が空なら空）。
func sub(dir, name string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name)
}

func storages(s []string) []string {
	if s != nil {
		return s
	}
	return KiroGlobalStorage()
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

// DefaultClaudeRoot は CLAUDE_CONFIG_DIR か ~/.claude の projects。
func DefaultClaudeRoot() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "projects")
	}
	return filepath.Join(home(), ".claude", "projects")
}

// DefaultKiroHome は KIRO_HOME か ~/.kiro。
func DefaultKiroHome() string {
	if d := os.Getenv("KIRO_HOME"); d != "" {
		return d
	}
	return filepath.Join(home(), ".kiro")
}

// KiroGlobalStorage は Kiro IDE（v1.0 より前）の保存場所。あるものだけ返す。
func KiroGlobalStorage() []string {
	var bases []string
	switch runtime.GOOS {
	case "darwin":
		bases = []string{filepath.Join(home(), "Library", "Application Support", "Kiro")}
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home(), "AppData", "Roaming")
		}
		bases = []string{filepath.Join(appdata, "Kiro")}
	default:
		bases = []string{filepath.Join(home(), ".config", "Kiro"), filepath.Join(home(), ".kiro-server", "data")}
	}
	var out []string
	for _, b := range bases {
		p := filepath.Join(b, "User", "globalStorage", "kiro.kiroagent")
		if isDir(p) {
			out = append(out, p)
		}
	}
	return out
}

// safeName は、履歴の中身から読んだ名前（セッション ID など）を、ファイル名の 1 つの部分として使ってよいか。
// パスの区切り（/ と \）、..、ドライブ名などの :、制御文字、Windows の予約名（NUL など）を含むものは使わない。
func safeName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return filepath.IsLocal(name) && filepath.IsLocal(name+".json")
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func glob(pattern string) []string {
	m, _ := filepath.Glob(pattern)
	sort.Strings(m)
	return m
}

func ts(v any) *float64 {
	if t, ok := core.ParseTS(v); ok {
		return &t
	}
	return nil
}
