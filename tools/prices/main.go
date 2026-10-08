// prices は、Anthropic と OpenAI の公式の料金ページ（Markdown 版）を取ってきて、kiroku が使う数字だけを
// docs/upstream/*.md に書き出す（開発と CI の .github/workflows/prices.yml で使う。kiroku 本体は料金を取りに行かない）。
//
//	go run ./tools/prices                # docs/upstream を書き直す（数字が変わったファイルだけ）
//	go run ./tools/prices -dir <folder>  # 別のフォルダに書く
//
// 取ってきたページは信頼できないデータとして扱い、決まった表の決まった形だけを読む。形が違えば何も書かずに失敗する。
// internal/core の料金表がこの控えと合っているかは、TestPricesMatchUpstream（internal/core）が調べる。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	anthropicURL = "https://platform.claude.com/docs/en/about-claude/pricing.md"
	openaiURL    = "https://developers.openai.com/api/docs/pricing.md"
	maxBody      = 4 << 20 // 料金ページ 1 つの上限（いまは 50 KB ほど）
)

func main() {
	dir := flag.String("dir", filepath.Join("docs", "upstream"), "folder to write the snapshots to")
	flag.Parse()
	if err := run(*dir, time.Now().UTC(), fetch); err != nil {
		fmt.Fprintln(os.Stderr, "prices:", err)
		os.Exit(1)
	}
}

// run は 2 つのページを取ってきて読み、両方読めたときだけ書く（片方だけ新しくならないように）。
func run(dir string, now time.Time, get func(string) ([]byte, error)) error {
	date := now.Format("2006-01-02")
	var files []snapshot
	for _, v := range vendors {
		body, err := get(v.url)
		if err != nil {
			return fmt.Errorf("%s: %w", v.url, err)
		}
		table, err := v.parse(string(body))
		if err != nil {
			return fmt.Errorf("%s: %w", v.url, err)
		}
		files = append(files, snapshot{name: v.file, head: v.head, url: v.url, table: table})
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		changed, err := f.write(dir, date)
		if err != nil {
			return err
		}
		state := "unchanged"
		if changed {
			state = "updated"
		}
		fmt.Printf("%s: %s\n", filepath.Join(dir, f.name), state)
	}
	return nil
}

type vendor struct {
	file, head, url string
	parse           func(string) (string, error) // ページ → 控えの表（Markdown）
}

var vendors = []vendor{
	{"anthropic-pricing.md", "Anthropic API prices", anthropicURL, parseAnthropic},
	{"openai-pricing.md", "OpenAI API prices", openaiURL, parseOpenAI},
}

// fetch は https の URL を取ってくる。Markdown か文字列だけを受け付け、大きさに上限を設ける。
func fetch(url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, errors.New("only https URLs")
	}
	c := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if r.URL.Scheme != "https" || len(via) >= 5 {
			return errors.New("redirect refused")
		}
		return nil
	}}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	if mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil || mt != "text/markdown" && mt != "text/plain" {
		return nil, fmt.Errorf("unexpected Content-Type %q", resp.Header.Get("Content-Type"))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBody {
		return nil, errors.New("page too large")
	}
	if !utf8.Valid(b) {
		return nil, errors.New("page is not UTF-8")
	}
	return b, nil
}

// ── 表を読む ──

// mdTable は Markdown の表（見出しの行と本文の行）。
type mdTable struct {
	header []string
	rows   [][]string
}

// splitRow は "| a | b |" → ["a", "b"]。表の行でなければ nil。
func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	if len(line) < 2 || line[0] != '|' || line[len(line)-1] != '|' {
		return nil
	}
	cells := strings.Split(line[1:len(line)-1], "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

var sepCell = regexp.MustCompile(`^:?-{3,}:?$`)

// tablesAfter は、lines[from:] にある表を、見出しの行が header と同じものだけ、出てくる順に返す。
func tablesAfter(lines []string, from int, header []string) []mdTable {
	var out []mdTable
	for i := from; i+1 < len(lines); i++ {
		h := splitRow(lines[i])
		if h == nil || !equal(h, header) {
			continue
		}
		sep := splitRow(lines[i+1])
		if len(sep) != len(h) || !allMatch(sep, sepCell) {
			continue
		}
		t := mdTable{header: h}
		j := i + 2
		for ; j < len(lines); j++ {
			r := splitRow(lines[j])
			if r == nil {
				break
			}
			t.rows = append(t.rows, r)
		}
		out = append(out, t)
		i = j
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func allMatch(cells []string, re *regexp.Regexp) bool {
	for _, c := range cells {
		if !re.MatchString(c) {
			return false
		}
	}
	return true
}

// lineIndex は、前後の空白を除いて text と同じ最初の行（なければ -1）。
func lineIndex(lines []string, from int, text string) int {
	for i := from; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == text {
			return i
		}
	}
	return -1
}

// ── Anthropic ──

var (
	anthropicHeader = []string{"Model", "Base input tokens", "5m cache writes", "1h cache writes", "Cache hits and refreshes", "Output tokens"}
	// モデル名の後ろの注（"([retired, …](https://…))" や "([limited availability](…))"）
	anthropicNote  = regexp.MustCompile(`\s*\(\[[^\]]*\]\([^)]*\)\)\s*$`)
	anthropicName  = regexp.MustCompile(`^Claude ([A-Z][a-z]+) (\d+)(?:\.(\d+))?$`)
	anthropicPrice = regexp.MustCompile(`^\$(\d+(?:\.\d+)?) / MTok(?:<sup>\d+</sup>)?$`)
	// プロンプトの長さで料金が変わるモデルの名前の後ろ（"Claude Haiku 5.5 (for prompts up to 100,000 tokens)" と "… over 100,000 tokens)"）
	anthropicTier = regexp.MustCompile(`^(.*\S) \(for prompts (up to|over) (\d{1,3}(?:,\d{3})+|\d+) tokens\)$`)
)

// anthropicRow は Model pricing の表の 1 行を読んだもの。
type anthropicRow struct {
	id   string
	v    [5]string // 入力, 5 分, 1 時間, 読み込み, 出力
	long *anthropicRow
	over string // long の境目（トークン）
}

// anthropicID は "Claude Opus 4.1" → claude-opus-4-1、"Claude Haiku 3.5" → claude-3-5-haiku（3 までの名前の付け方）。
func anthropicID(name string) (string, error) {
	m := anthropicName.FindStringSubmatch(anthropicNote.ReplaceAllString(name, ""))
	if m == nil {
		return "", fmt.Errorf("unexpected model name %q", name)
	}
	family, ver := strings.ToLower(m[1]), m[2]
	if m[3] != "" {
		ver += "-" + m[3]
	}
	if major, _ := strconv.Atoi(m[2]); major < 4 {
		return "claude-" + ver + "-" + family, nil
	}
	return "claude-" + family + "-" + ver, nil
}

// parseAnthropic は「## Model pricing」の最初の表だけを読む。
func parseAnthropic(page string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(page, "\r\n", "\n"), "\n")
	at := lineIndex(lines, 0, "## Model pricing")
	if at < 0 {
		return "", errors.New(`no "## Model pricing" heading`)
	}
	ts := tablesAfter(lines, at, anthropicHeader)
	if len(ts) == 0 {
		return "", errors.New("no model price table with the expected columns")
	}
	if next := lineIndex(lines, at+1, "## Cloud platform pricing"); next >= 0 && next < tableLine(lines, at, anthropicHeader) {
		return "", errors.New("the model price table is not in the Model pricing section")
	}
	t := ts[0]
	if len(t.rows) < 5 {
		return "", fmt.Errorf("only %d models in the price table", len(t.rows))
	}
	// プロンプトの長さで料金が変わるモデルは "up to N" と "over N" の 2 行になっている。1 行にまとめ、over の料金を Long の列に入れる
	var rows []*anthropicRow
	byID := map[string]*anthropicRow{}
	tiers := map[string]string{} // id → "up to" / "over" の片方だけが出てきたもの
	for _, r := range t.rows {
		if len(r) != len(anthropicHeader) {
			return "", fmt.Errorf("row %q has %d cells", r, len(r))
		}
		name, tier, over := r[0], "", ""
		if m := anthropicTier.FindStringSubmatch(name); m != nil {
			name, tier, over = m[1], m[2], strings.ReplaceAll(m[3], ",", "")
			if n, err := strconv.Atoi(over); err != nil || n < 1000 || n > 10_000_000 {
				return "", fmt.Errorf("unexpected prompt length %q", m[3])
			}
		}
		id, err := anthropicID(name)
		if err != nil {
			return "", err
		}
		row := &anthropicRow{id: id}
		for i := range row.v {
			m := anthropicPrice.FindStringSubmatch(r[i+1])
			if m == nil {
				return "", fmt.Errorf("%s: unexpected price %q", id, r[i+1])
			}
			if row.v[i], err = number(m[1]); err != nil {
				return "", fmt.Errorf("%s: %w", id, err)
			}
		}
		prev := byID[id]
		switch {
		case prev == nil:
			row.over = over
			if tier != "" {
				tiers[id] = tier
			}
			byID[id] = row
			rows = append(rows, row)
			continue
		case tier == "" || tiers[id] == "" || tiers[id] == tier || prev.over != over:
			return "", fmt.Errorf("model %s appears twice", id)
		}
		delete(tiers, id)
		if tier == "over" {
			prev.long = row
		} else { // over の行が先にあった。並びは最初に出てきた位置のまま
			long := *prev
			*prev = *row
			prev.long, prev.over = &long, over
		}
	}
	for id, tier := range tiers {
		return "", fmt.Errorf("%s: prices for prompts %s %s tokens, but not for the other lengths", id, tier, byID[id].over)
	}

	var b strings.Builder
	b.WriteString("Long over: a request whose prompt is more than this many tokens is billed at the Long rates as a whole (\"-\": one price for every length).\n\n")
	b.WriteString("| Model ID | Input | Cache write 5m | Cache write 1h | Cache read | Output | Long over | Long input | Long cache write 5m | Long cache write 1h | Long cache read | Long output |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		long := []string{"-", "-", "-", "-", "-", "-"}
		if r.long != nil {
			long = append([]string{r.over}, r.long.v[:]...)
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.id, strings.Join(r.v[:], " | "), strings.Join(long, " | "))
	}
	return b.String(), nil
}

// tableLine は from から後で、見出しが header の最初の表の行番号。
func tableLine(lines []string, from int, header []string) int {
	for i := from; i < len(lines); i++ {
		if h := splitRow(lines[i]); h != nil && equal(h, header) {
			return i
		}
	}
	return len(lines)
}

// ── OpenAI ──

var (
	openaiHeader = []string{"Model", "Short context input", "Short context cached input", "Short context cache writes", "Short context output",
		"Long context input", "Long context cached input", "Long context cache writes", "Long context output"}
	openaiSpecialHeader = []string{"Category", "Model", "Input", "Cached input", "Output"}
	// Codex が使うモデル（gpt-5 以降）。名前の後ろの "(<272K context length)" は除く
	openaiModel = regexp.MustCompile(`^(gpt-(?:[5-9]|[1-9]\d)(?:\.\d+)?(?:-[a-z0-9]+)*)(?: \(<\d+K context length\))?$`)
	openaiPrice = regexp.MustCompile(`^(?:\$(\d+(?:\.\d+)?)|-)$`)
	openaiLong  = regexp.MustCompile(`Short context: ≤(\d+)K input tokens\. Long context: >(\d+)K input tokens\.`)
)

// parseOpenAI は「### Standard pricing data」の表（gpt-5 以降）と、Specialized models の Standard の表の Codex の行を読む。
func parseOpenAI(page string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(page, "\r\n", "\n"), "\n")
	at := lineIndex(lines, 0, "### Standard pricing data")
	if at < 0 {
		return "", errors.New(`no "### Standard pricing data" heading`)
	}
	if lineIndex(lines, at+1, "### Standard pricing data") >= 0 {
		return "", errors.New(`"### Standard pricing data" appears twice`)
	}
	ts := tablesAfter(lines, at, openaiHeader)
	if len(ts) == 0 || tableLine(lines, at, openaiHeader) != at+2 {
		return "", errors.New("no standard price table with the expected columns right after its heading")
	}
	m := openaiLong.FindStringSubmatch(strings.Join(lines, "\n"))
	if m == nil || m[1] != m[2] {
		return "", errors.New("no short / long context threshold")
	}
	threshold := m[1]

	var b strings.Builder
	fmt.Fprintf(&b, "Long context: more than %sK input tokens in one request (the whole request is billed at the long-context rates).\n\n", threshold)
	b.WriteString("| Model ID | Input | Cached input | Cache write | Output | Long input | Long cached input | Long cache write | Long output |\n|---|---|---|---|---|---|---|---|---|\n")
	seen := map[string]bool{}
	n := 0
	for _, r := range ts[0].rows {
		if len(r) != len(openaiHeader) {
			return "", fmt.Errorf("row %q has %d cells", r, len(r))
		}
		if !strings.HasPrefix(r[0], "gpt-") {
			continue
		}
		mm := openaiModel.FindStringSubmatch(r[0])
		if mm == nil {
			if strings.HasPrefix(r[0], "gpt-4") || strings.HasPrefix(r[0], "gpt-3") {
				continue // Codex が使わない古いモデル
			}
			return "", fmt.Errorf("unexpected model name %q", r[0])
		}
		id := mm[1]
		v, err := openaiPrices(id, r[1:])
		if err != nil {
			return "", err
		}
		if v[0] == "-" || v[3] == "-" {
			return "", fmt.Errorf("%s: no input or output price", id)
		}
		if (v[4] == "-") != (v[7] == "-") {
			return "", fmt.Errorf("%s: long-context input and output prices must both be given or both missing", id)
		}
		if seen[id] {
			return "", fmt.Errorf("model %s appears twice", id)
		}
		seen[id] = true
		n++
		fmt.Fprintf(&b, "| %s | %s |\n", id, strings.Join(v, " | "))
	}
	if n < 3 {
		return "", fmt.Errorf("only %d gpt-5 or later models in the standard price table", n)
	}

	// Specialized models の Standard（Fast の前）の表の Codex の行
	sp := lineIndex(lines, at, "Specialized models")
	if sp < 0 {
		return "", errors.New(`no "Specialized models" section`)
	}
	st := lineIndex(lines, sp, "Standard")
	tl := tableLine(lines, sp, openaiSpecialHeader)
	if st < 0 || tl >= len(lines) || st > tl || lineIndex(lines[:tl], sp, "Fast") >= 0 {
		return "", errors.New("no standard table under Specialized models")
	}
	special := tablesAfter(lines, tl, openaiSpecialHeader)
	if len(special) == 0 {
		return "", errors.New("the standard table under Specialized models has no separator row")
	}
	codex := 0
	for _, r := range special[0].rows {
		if len(r) != len(openaiSpecialHeader) {
			return "", fmt.Errorf("row %q has %d cells", r, len(r))
		}
		if r[0] != "Codex" {
			continue
		}
		mm := openaiModel.FindStringSubmatch(r[1])
		if mm == nil {
			return "", fmt.Errorf("unexpected Codex model name %q", r[1])
		}
		id := mm[1]
		v, err := openaiPrices(id, []string{r[2], r[3], "-", r[4]})
		if err != nil {
			return "", err
		}
		if v[0] == "-" || v[3] == "-" {
			return "", fmt.Errorf("%s: no input or output price", id)
		}
		if seen[id] {
			return "", fmt.Errorf("model %s appears twice", id)
		}
		seen[id] = true
		codex++
		fmt.Fprintf(&b, "| %s | %s | - | - | - | - |\n", id, strings.Join(v, " | "))
	}
	if codex == 0 {
		return "", errors.New("no Codex models under Specialized models")
	}
	return b.String(), nil
}

// openaiPrices は "$1.25" や "-" の並び → 数字か "-" の並び。
func openaiPrices(id string, cells []string) ([]string, error) {
	out := make([]string, len(cells))
	for i, c := range cells {
		m := openaiPrice.FindStringSubmatch(c)
		if m == nil {
			return nil, fmt.Errorf("%s: unexpected price %q", id, c)
		}
		if m[1] == "" {
			out[i] = "-"
			continue
		}
		v, err := number(m[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		out[i] = v
	}
	return out, nil
}

// number は "12.50" → "12.5"（0 以上の有限の数だけ）。
func number(s string) (string, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || f > 10000 {
		return "", fmt.Errorf("unexpected number %q", s)
	}
	return strconv.FormatFloat(f, 'f', -1, 64), nil
}

// ── 書き出す ──

type snapshot struct {
	name, head, url, table string
}

const generated = "<!-- Generated by `go run ./tools/prices`. Do not edit by hand. internal/core/usage.go must match it (TestPricesMatchUpstream). -->"

func (s snapshot) render(date string) string {
	return generated + "\n\n# " + s.head + "\n\n" +
		"- Source: " + s.url + "\n" +
		"- Fetched: " + date + " (the last time these numbers changed)\n\n" +
		"USD per million tokens, standard rates. Only the numbers kiroku uses.\n\n" + s.table
}

// write は、表が変わったときだけ（日付も新しくして）書く。変わっていなければ日付も含めてそのまま。
func (s snapshot) write(dir, date string) (bool, error) {
	path := filepath.Join(dir, s.name)
	if old, err := os.ReadFile(path); err == nil {
		if _, after, ok := strings.Cut(strings.ReplaceAll(string(old), "\r\n", "\n"), "Only the numbers kiroku uses.\n\n"); ok && after == s.table {
			return false, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, os.WriteFile(path, []byte(s.render(date)), 0o644)
}
