// Package core は、どのエージェントの履歴からも同じ形のセッションを組み立てるための部品。
package core

import (
	"bufio"
	"encoding/json"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Obj は JSON のオブジェクトを読むための小さな道具。
type Obj = map[string]any

func Map(v any) Obj {
	m, _ := v.(map[string]any)
	return m
}

func List(v any) []any {
	l, _ := v.([]any)
	return l
}

func Str(v any) string {
	s, _ := v.(string)
	return s
}

// Num は数値だけを返す（bool や文字列は 0, false）。
func Num(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

func NumOr0(v any) float64 {
	f, _ := Num(v)
	return f
}

// Get はネストしたキーをたどる。途中で見つからなければ nil。
func Get(v any, keys ...string) any {
	for _, k := range keys {
		m := Map(v)
		if m == nil {
			return nil
		}
		v = m[k]
	}
	return v
}

// ParseTS は ISO 文字列 / UNIX 秒 / UNIX ミリ秒 をすべて UNIX 秒にする。読めなければ ok=false。
func ParseTS(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case int64: // SQLite の INTEGER 列
		return ParseTS(float64(x))
	case int:
		return ParseTS(float64(x))
	case []byte:
		return ParseTS(string(x))
	case float64:
		if x > 1e12 {
			return x / 1000, true
		}
		return x, true
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		if isInt(s) {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return 0, false
			}
			return ParseTS(float64(n))
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z07:00"} {
			if t, err := time.Parse(layout, strings.Replace(s, " ", "T", 1)); err == nil {
				return float64(t.UnixNano()) / 1e9, true
			}
		}
		for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, strings.Replace(s, " ", "T", 1), time.Local); err == nil {
				return float64(t.UnixNano()) / 1e9, true
			}
		}
	}
	return 0, false
}

func isInt(s string) bool {
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TextOf は content（文字列 or ブロック配列）から人が書いた文字だけ取り出す。
func TextOf(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, b := range c {
			m := Map(b)
			if m == nil {
				continue
			}
			if Str(m["type"]) == "text" {
				if t := Str(m["text"]); t != "" {
					parts = append(parts, t)
				}
			} else if Str(m["kind"]) == "text" { // Kiro CLI
				if t, ok := m["data"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// ReadJSONL は 1 行ずつ JSON を読む。壊れた行は飛ばす。名前が .zst で終わるファイルは zstd で圧縮されたものとして読む。
func ReadJSONL(path string, fn func(Obj)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if strings.HasSuffix(path, ".zst") {
		d, err := zstd.NewReader(f)
		if err != nil {
			return err
		}
		defer d.Close()
		return ReadJSONLFrom(d, fn)
	}
	return ReadJSONLFrom(f, fn)
}

// ReadJSONLFrom は ReadJSONL の io.Reader 版（圧縮されたファイルなど）。
func ReadJSONLFrom(src io.Reader, fn func(Obj)) error {
	r := bufio.NewReaderSize(src, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			var v any
			if json.Unmarshal(line, &v) == nil {
				if m := Map(v); m != nil {
					fn(m)
				}
			}
		}
		if err != nil {
			return nil
		}
	}
}

// ReadJSON は JSON ファイルを読む。読めなければ nil。
func ReadJSON(path string) any {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	return v
}

// Round は Python の round(x, n) と同じく、偶数への丸めを使う。
func Round(x float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.RoundToEven(x*p) / p
}

// Runes は先頭 n 文字（コードポイント）までに切る。
func Runes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// limitText は、エージェントが返した利用上限（使用量の上限・レート制限）のエラー文。
// 例: "Claude AI usage limit reached|1750000000"、"5-hour limit reached ∙ resets 3pm"、"API Error: 429 …rate_limit_error…"
var limitText = regexp.MustCompile(`(?i)usage limit|limit reached|limit will reset|hit your (usage )?limit|reached your .{0,20}limit|rate[_ ]limit|\b429\b`)

// IsLimitError は、文が利用上限のエラーかどうか。
func IsLimitError(text string) bool { return limitText.MatchString(text) }
