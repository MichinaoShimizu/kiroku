// Package core は、どのエージェントの履歴からも同じ形のセッションを組み立てるための部品。
package core

import (
	"bufio"
	"encoding/json"
	"fmt"
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

// 履歴の時刻としてありえる範囲。外れた時刻（"timestamp": 100 や "0001-01-01T00:00:00Z" など）は読めなかったものとする。
// 1 つでも混ざると、セッションが 1970 年から始まったり（何十年分の週を回して重くなる）、
// 「いちばん古い記録」がおかしくなったりするため。
//   - 下限: 2000-01-01 UTC（AI エージェントの履歴がそれより前にあることはない）
//   - 上限: いまから 1 日先まで（時計のずれやタイムゾーンの書き間違いくらいは許す）
var (
	minTS      = float64(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
	maxTSAhead = 24 * time.Hour
	nowForTS   = time.Now // テストで差し替える
)

func plausibleTS(sec float64) bool {
	if math.IsNaN(sec) || math.IsInf(sec, 0) {
		return false
	}
	return sec >= minTS && sec <= float64(nowForTS().Add(maxTSAhead).Unix())
}

// unixSec は time.Time を UNIX 秒にする（UnixNano は 1678 年より前や 2262 年より後であふれるので使わない）。
func unixSec(t time.Time) float64 {
	return float64(t.Unix()) + float64(t.Nanosecond())/1e9
}

// ParseTS は ISO 文字列 / UNIX 秒 / UNIX ミリ秒 をすべて UNIX 秒にする。
// 読めないときと、ありえない時刻（plausibleTS）のときは ok=false。
func ParseTS(v any) (float64, bool) {
	t, ok := parseTS(v)
	if !ok || !plausibleTS(t) {
		return 0, false
	}
	return t, true
}

func parseTS(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case int64: // SQLite の INTEGER 列
		return parseTS(float64(x))
	case int:
		return parseTS(float64(x))
	case []byte:
		return parseTS(string(x))
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
			return parseTS(float64(n))
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z07:00"} {
			if t, err := time.Parse(layout, strings.Replace(s, " ", "T", 1)); err == nil {
				return unixSec(t), true
			}
		}
		for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, strings.Replace(s, " ", "T", 1), time.Local); err == nil {
				return unixSec(t), true
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
// 開けない・途中で読めなくなったときはエラーを返す（それまでに読めた行は fn に渡してある）。
// MaxLine より長い行は飛ばして続きを読み、最後にそのことをエラーで返す（読めないファイルとして知らせる）。
func ReadJSONL(path string, fn func(Obj)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if strings.HasSuffix(path, ".zst") {
		d, err := NewZstdReader(f)
		if err != nil {
			return err
		}
		defer d.Close()
		return ReadJSONLFrom(d, fn)
	}
	return ReadJSONLFrom(f, fn)
}

// MaxLine は 1 行の上限（バイト）。壊れたファイルやわざと作ったファイルで、メモリを使い切らないため。
// これより長い行は、全部をメモリに読まずに飛ばす。テストで小さくする。
var MaxLine = 64 << 20

// zstd をほどくときのメモリの上限と、窓の大きさの上限（ライブラリの既定は 64 GiB と 512 MiB）。
const (
	zstdMaxMemory = 1 << 30
	zstdMaxWindow = 1 << 28
)

// NewZstdReader は、使うメモリに上限をつけた zstd の読み手（履歴のコピーや Codex の圧縮した履歴を読む）。
func NewZstdReader(r io.Reader) (*zstd.Decoder, error) {
	return zstd.NewReader(r, zstd.WithDecoderMaxMemory(zstdMaxMemory), zstd.WithDecoderMaxWindow(zstdMaxWindow))
}

// ReadLine は改行までの 1 行を読む（改行もふくむ）。max より長い行は long を true にして、中身は持たずに読み飛ばす。
// err は bufio.Reader.ReadBytes と同じ（最後の行に改行がなければ io.EOF といっしょに返す）。
func ReadLine(r *bufio.Reader, max int) (line []byte, long bool, err error) {
	for {
		frag, err := r.ReadSlice('\n')
		if !long {
			if len(line)+len(frag) > max {
				long, line = true, nil // ここから先は捨てる
			} else {
				line = append(line, frag...) // frag は次に読むと書きかわるので写す
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return line, long, err
	}
}

func lineLimit() string {
	if MaxLine >= 1<<20 && MaxLine%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", MaxLine>>20)
	}
	return fmt.Sprintf("%d bytes", MaxLine)
}

// ReadJSONLFrom は ReadJSONL の io.Reader 版（圧縮されたファイルなど）。
func ReadJSONLFrom(src io.Reader, fn func(Obj)) error {
	r := bufio.NewReaderSize(src, 1<<20)
	skipped := 0
	for {
		line, long, err := ReadLine(r, MaxLine)
		if long {
			skipped++
		} else if len(line) > 0 {
			var v any
			if json.Unmarshal(line, &v) == nil {
				if m := Map(v); m != nil {
					fn(m)
				}
			}
		}
		if err == io.EOF {
			if skipped > 0 {
				return fmt.Errorf("skipped %d line(s) longer than %s", skipped, lineLimit())
			}
			return nil
		}
		if err != nil { // 読めた行までは fn に渡したうえで、途中で読めなくなったこと（壊れた .zst など）を返す
			return err
		}
	}
}

// ReadJSON は JSON ファイルを読む。読めなければ nil。
func ReadJSON(path string) any {
	v, _ := ReadJSONFile(path)
	return v
}

// ReadJSONFile は ReadJSON と同じだけど、読めなかったわけ（開けない・JSON として壊れている）も返す。
func ReadJSONFile(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
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
// 例: "Claude AI usage limit reached|1750000000"、"5-hour limit reached ∙ resets 3pm"、"API Error: 429 …rate_limit_error…"、
// "You've hit your session limit · resets 3:45pm"（weekly・Opus・Sonnet も同じ形）、"You've hit your org's monthly spend limit"、
// "You've hit your team's shared budget"、"API Error: Request rejected (429)"（API キーのレート制限）。
// 出典: https://code.claude.com/docs/en/errors （Usage limits）
var limitText = regexp.MustCompile(`(?i)usage limit|limit reached|limit will reset|hit your [^.\n]{0,40}?(limit|budget)|reached your .{0,20}limit|rate[_ ]limit|\b429\b`)

// notLimitText は、limitText に当たるが利用上限ではないエラー文。
// "API Error: Server is temporarily limiting requests (not your usage limit)" はサーバー側の一時的な絞り込み、
// "Context limit reached · /compact or /clear to continue" は会話が長すぎるだけで、どちらも使用量の上限ではない。
var notLimitText = regexp.MustCompile(`(?i)not your usage limit|temporarily limiting requests|context limit reached`)

// IsLimitError は、文が利用上限のエラーかどうか。
func IsLimitError(text string) bool {
	return limitText.MatchString(text) && !notLimitText.MatchString(text)
}

// resetText は、利用上限のエラー文にある解除の時刻（"resets " のあと）。
// 例: "You've hit your session limit · resets 3:45pm"、"You've hit your weekly limit · resets Mon 12:00am"、
// "5-hour limit reached ∙ resets 3pm"、"spend limit reached (daily; resets 2026-08-09 00:00 UTC)"。
// 時間帯の名前が括弧で付くこと（"resets 3pm (Asia/Tokyo)"）も考えて読む。曜日・月日・日付・時刻・時間帯の形の文字だけを拾う。
// 出典: https://code.claude.com/docs/en/errors （Usage limits・Spend limit reached）
var resetText = regexp.MustCompile(`(?i)\bresets\s+(?:at\s+)?((?:[a-z]{3,9},?\s+)?(?:\d{1,2},?\s+(?:at\s+)?)?(?:\d{4}-\d{2}-\d{2}\s+)?\d{1,2}(?::\d{2})?(?:\s?[ap]m)?(?:\s+UTC)?(?:\s+\([a-z_]+(?:/[a-z_+-]+){0,2}\))?)`)

// LimitReset は、利用上限のエラー文にある解除の時刻の文（"3:45pm"、"Mon 12:00am" など）。なければ空。
// 日付や時間帯がないことが多いので、時刻には直さず、書いてあるとおりの文を返す。
func LimitReset(text string) string {
	m := resetText.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.Join(strings.Fields(m[1]), " ")
}
