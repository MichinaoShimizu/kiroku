// Package source は、エージェントごとの履歴を読んで core.Builder に変えるアダプター。
// 新しいエージェントを足すときは、Source を実装して All に加える。
package source

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Source は 1 種類の履歴。
type Source interface {
	Name() string   // 画面に出す名前（例: "Claude Code"）
	Family() string // --sources で選ぶ名前（例: "claude"）
	Where() string  // 読む場所（計測の状態に出す）
	Load(emit func(*core.Builder)) error
}

// Report は計測の状態に出す、読み込みの結果。
type Report struct {
	Name  string  `json:"name"`
	N     int     `json:"n"`
	Dup   int     `json:"dup,omitempty"` // ほかの場所と同じ会話だったので数えなかった数
	Where string  `json:"where"`
	Error *string `json:"error"`
}

// Options は読み込みの設定。
type Options struct {
	ClaudeRoot   string
	KiroHome     string
	KiroStorages []string // nil なら OS ごとの場所を探す
	KiroCLIDB    string   // 空なら OS ごとの場所
	AmazonQDB    string
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
		&Claude{Root: o.ClaudeRoot},
		&KiroIDE{Home: o.KiroHome},
		&KiroCLI{Home: o.KiroHome},
		&QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: q(o.KiroCLIDB, filepath.Join(DataDir("kiro-cli"), "data.sqlite3")), Command: "kiro-cli chat --resume"},
		&KiroIDELegacy{Storages: storages(o.KiroStorages)},
		&QStore{Label: "Amazon Q", Fam: "amazonq", DB: q(o.AmazonQDB, filepath.Join(DataDir("amazon-q"), "data.sqlite3")), Command: "q chat --resume"},
	}
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
