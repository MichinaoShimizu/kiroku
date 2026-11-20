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
	// Standard・Flex・Fast・Ultrafast の表。長いコンテキストの料金があるモデルの表（wide）のすぐ後に、ないモデルの表（narrow）が続く。
	// キャッシュの書き込みは 30 分（30m write）と 12 時間（12h write）の 2 つ。Codex は TTL を履歴に書かない（どちらで書いたかわからない）ので、
	// 30 分の料金を kiroku のキャッシュ書き込みにする（2026-11 より前の 1 つだけの "cache writes" と同じ値）。12 時間は形だけ確かめて使わない。
	openaiWideHeader = []string{"Model", "Short context input", "Short context output", "Short context cached input", "Short context 30m write", "Short context 12h write",
		"Long context input", "Long context output", "Long context cached input", "Long context 30m write", "Long context 12h write"}
	openaiNarrowHeader = []string{"Model", "Input", "Output", "Cached input"}
	// Specialized models の Standard の表（Cyber・Codex など）と Fast の表
	openaiSpecialHeader     = []string{"Category", "Model", "Input", "Output", "Cached input", "30m write", "12h write"}
	openaiSpecialFastHeader = []string{"Category", "Model", "Input", "Output", "Cached input"}
	// Codex が使うモデル（gpt-5 以降）の ID
	openaiID = `gpt-(?:[5-9]|[1-9]\d)(?:\.\d+)?(?:-[a-z0-9]+)*`
	// 名前の後ろの "(<272K context length)" は除く
	openaiModel = regexp.MustCompile(`^(` + openaiID + `)(?: \(<\d+K context length\))?$`)
	// Specialized models の Cyber の行（短いコンテキストと長いコンテキストが別の行: "gpt-6-cyber (short context)"）
	openaiCyberModel = regexp.MustCompile(`^(` + openaiID + `) \((short|long) context\)$`)
	// Category の前の飛び先（<a id="cyber-models"></a>）
	openaiAnchor = regexp.MustCompile(`^<a id="[a-z0-9-]+"></a>`)
	openaiPrice  = regexp.MustCompile(`^(?:\$(\d+(?:\.\d+)?)|-)$`)
	openaiLong   = regexp.MustCompile(`Short context: ≤(\d+)K input tokens\. Long context: >(\d+)K input tokens\.`)
)

// openaiTiers は、Standard のほかに読むサービスティアの表（見出し → 控えの節の名前）。Codex の service_tier で選ぶ料金で、
// 節の名前は internal/core の OpenAITiers の Name と同じ。どれもなければその節を書かない（料金表に残っていれば TestPricesMatchUpstream が知らせる）。
// 「### <名前> list pricing data」の表（あれば）も同じ節に足す。期間限定の「### <名前> promotion pricing data」は日付で料金が変わるので使わず、
// list の表と同じモデルがそろっていることだけ確かめる（promotion だけのモデルを黙って落とさないように）。
var openaiTiers = []struct{ heading, name string }{
	{"### Flex pricing data", "Flex"},
	{"### Fast pricing data", "Fast"},
	{"### Ultrafast pricing data", "Ultrafast"},
}

const openaiTableHead = "| Model ID | Input | Cached input | Cache write | Output | Long input | Long cached input | Long cache write | Long output |\n|---|---|---|---|---|---|---|---|---|\n"

// parseOpenAI は「### Standard pricing data」の表（gpt-5 以降）と、Specialized models の Standard の表の
// Cyber と Codex の行を読む。サービスティア（Flex・Fast・Ultrafast）の表と、Specialized models の Fast の表の Codex の行は、
// それぞれ「### 名前」の節に書く。
func parseOpenAI(page string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(page, "\r\n", "\n"), "\n")
	at := lineIndex(lines, 0, "### Standard pricing data")
	if at < 0 {
		return "", errors.New(`no "### Standard pricing data" heading`)
	}
	if lineIndex(lines, at+1, "### Standard pricing data") >= 0 {
		return "", errors.New(`"### Standard pricing data" appears twice`)
	}
	std, err := priceTables(lines, at)
	if err != nil {
		return "", fmt.Errorf("standard: %w", err)
	}
	m := openaiLong.FindStringSubmatch(strings.Join(lines, "\n"))
	if m == nil || m[1] != m[2] {
		return "", errors.New("no short / long context threshold")
	}
	threshold := m[1]

	var b strings.Builder
	fmt.Fprintf(&b, "Long context: more than %sK input tokens in one request (the whole request is billed at the long-context rates).\n\n", threshold)
	b.WriteString(openaiTableHead)
	seen := map[string][]string{} // 読んだモデルと料金
	n, err := openaiRows(&b, std, seen)
	if err != nil {
		return "", err
	}
	if n < 3 {
		return "", fmt.Errorf("only %d gpt-5 or later models in the standard price table", n)
	}

	// Specialized models の Standard（Fast の前）の表
	sp := lineIndex(lines, at, "Specialized models")
	if sp < 0 {
		return "", errors.New(`no "Specialized models" section`)
	}
	st := lineIndex(lines, sp, "Standard")
	tl := tableLine(lines, sp, openaiSpecialHeader)
	if st < 0 || tl >= len(lines) || st > tl || firstTableLine(lines, st) != tl || lineIndex(lines[:tl], sp, "Fast") >= 0 {
		return "", errors.New("no standard table with the expected columns under Specialized models")
	}
	special := tablesAfter(lines, tl, openaiSpecialHeader)
	if len(special) == 0 {
		return "", errors.New("the standard table under Specialized models has no separator row")
	}
	// Cyber の行: 1 つのモデルが short context と long context の 2 行になっている。1 行にまとめる
	type cyberRow struct {
		id          string
		short, long []string // 入力, キャッシュ済み, 書き込み, 出力
	}
	var cyber []*cyberRow
	byID := map[string]*cyberRow{}
	var codex [][]string
	for _, r := range special[0].rows {
		if len(r) != len(openaiSpecialHeader) {
			return "", fmt.Errorf("row %q has %d cells", r, len(r))
		}
		cat := openaiAnchor.ReplaceAllString(r[0], "")
		switch {
		case cat == "Codex":
			mm := openaiModel.FindStringSubmatch(r[1])
			if mm == nil {
				return "", fmt.Errorf("unexpected Codex model name %q", r[1])
			}
			if err := checkPrices(mm[1], r[6:7]); err != nil {
				return "", err
			}
			codex = append(codex, []string{mm[1], r[2], r[4], r[5], r[3], "-", "-", "-", "-"})
		case cat == "Cyber" || strings.HasPrefix(cat, "Cyber "):
			mm := openaiCyberModel.FindStringSubmatch(r[1])
			if mm == nil {
				return "", fmt.Errorf("unexpected Cyber model name %q", r[1])
			}
			id := mm[1]
			if err := checkPrices(id, r[6:7]); err != nil {
				return "", err
			}
			c := byID[id]
			if c == nil {
				c = &cyberRow{id: id}
				byID[id] = c
				cyber = append(cyber, c)
			}
			half := &c.short
			if mm[2] == "long" {
				half = &c.long
			}
			if *half != nil {
				return "", fmt.Errorf("cyber model %s has two %s context rows", id, mm[2])
			}
			*half = []string{r[2], r[4], r[5], r[3]}
		}
	}
	var cy [][]string
	for _, c := range cyber {
		if c.short == nil || c.long == nil {
			return "", fmt.Errorf("cyber model %s needs both a short and a long context row", c.id)
		}
		cy = append(cy, append(append([]string{c.id}, c.short...), c.long...))
	}
	if _, err := openaiRows(&b, cy, seen); err != nil {
		return "", err
	}
	if len(codex) == 0 {
		return "", errors.New("no Codex models under Specialized models")
	}
	if _, err := openaiRows(&b, codex, seen); err != nil {
		return "", err
	}

	// サービスティアの表（見出しのすぐ後に Standard と同じ列の表）
	tiers := map[string]*strings.Builder{}
	tierSeen := map[string]map[string][]string{}
	for _, tr := range openaiTiers {
		hd := lineIndex(lines, at, tr.heading)
		if hd < 0 {
			continue
		}
		if lineIndex(lines, hd+1, tr.heading) >= 0 {
			return "", fmt.Errorf("%q appears twice", tr.heading)
		}
		rows, err := priceTables(lines, hd)
		if err != nil {
			return "", fmt.Errorf("%s: %w", tr.name, err)
		}
		tb := &strings.Builder{}
		tierSeen[tr.name] = map[string][]string{}
		n, err := openaiRows(tb, rows, tierSeen[tr.name])
		if err != nil {
			return "", fmt.Errorf("%s: %w", tr.name, err)
		}
		if n == 0 {
			return "", fmt.Errorf("no gpt-5 or later models in the %s price table", tr.name)
		}
		tiers[tr.name] = tb

		// list（日付によらない料金）を足し、promotion（期間限定）は list と同じモデルがそろっているかだけ見る
		list, promo := map[string][]string{}, map[string][]string{}
		for _, x := range []struct {
			kind string
			seen map[string][]string
		}{{"list", list}, {"promotion", promo}} {
			h := strings.TrimSuffix(tr.heading, " pricing data") + " " + x.kind + " pricing data"
			xh := lineIndex(lines, at, h)
			if xh < 0 {
				continue
			}
			if lineIndex(lines, xh+1, h) >= 0 {
				return "", fmt.Errorf("%q appears twice", h)
			}
			rows, err := priceTables(lines, xh)
			if err != nil {
				return "", fmt.Errorf("%s %s: %w", tr.name, x.kind, err)
			}
			xb := &strings.Builder{}
			n, err := openaiRows(xb, rows, x.seen)
			if err != nil {
				return "", fmt.Errorf("%s %s: %w", tr.name, x.kind, err)
			}
			if n == 0 {
				return "", fmt.Errorf("no gpt-5 or later models in the %s %s price table", tr.name, x.kind)
			}
			if x.kind == "list" {
				for id, v := range list {
					if tierSeen[tr.name][id] != nil {
						return "", fmt.Errorf("model %s appears twice in %s", id, tr.name)
					}
					tierSeen[tr.name][id] = v
				}
				tb.WriteString(xb.String())
			}
		}
		for id := range promo {
			if list[id] == nil {
				return "", fmt.Errorf("%s: %s has promotion prices but no list prices", tr.name, id)
			}
		}
		for id := range list {
			if len(promo) > 0 && promo[id] == nil {
				return "", fmt.Errorf("%s: %s has list prices but no promotion prices", tr.name, id)
			}
		}
	}

	// Specialized models の Fast の表（Standard の表の後）の Codex の行は、Fast の節に足す
	if fa := lineIndex(lines, tl+1, "Fast"); fa >= 0 {
		ftl := tableLine(lines, fa, openaiSpecialFastHeader)
		if ftl >= len(lines) || firstTableLine(lines, fa) != ftl {
			return "", errors.New("no table with the expected columns under Fast in Specialized models")
		}
		ft := tablesAfter(lines, ftl, openaiSpecialFastHeader)
		if len(ft) == 0 {
			return "", errors.New("the Fast table under Specialized models has no separator row")
		}
		var rows [][]string
		for _, r := range ft[0].rows {
			if len(r) != len(openaiSpecialFastHeader) {
				return "", fmt.Errorf("row %q has %d cells", r, len(r))
			}
			if openaiAnchor.ReplaceAllString(r[0], "") != "Codex" {
				continue
			}
			mm := openaiModel.FindStringSubmatch(r[1])
			if mm == nil {
				return "", fmt.Errorf("unexpected Codex model name %q", r[1])
			}
			rows = append(rows, []string{mm[1], r[2], r[4], "-", r[3], "-", "-", "-", "-"})
		}
		if len(rows) > 0 {
			if tierSeen["Fast"] == nil {
				tierSeen["Fast"], tiers["Fast"] = map[string][]string{}, &strings.Builder{}
			}
			if _, err := openaiRows(tiers["Fast"], rows, tierSeen["Fast"]); err != nil {
				return "", fmt.Errorf("specialized Fast: %w", err)
			}
		}
	}
	for _, tr := range openaiTiers {
		if tb := tiers[tr.name]; tb != nil {
			fmt.Fprintf(&b, "\n### %s\n\nThe %s service tier (Codex `service_tier`) instead of the standard rates above.\n\n%s%s", tr.name, tr.name, openaiTableHead, tb.String())
		}
	}
	return b.String(), nil
}

// firstTableLine は from から後で最初の表の行の行番号（なければ len(lines)）。
func firstTableLine(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if splitRow(lines[i]) != nil {
			return i
		}
	}
	return len(lines)
}

// tableAt は、lines[i] から始まる、見出しの行が header の表（なければ ok=false）と、表の次の行の行番号。
func tableAt(lines []string, i int, header []string) (t mdTable, next int, ok bool) {
	if i+1 >= len(lines) {
		return t, i, false
	}
	h := splitRow(lines[i])
	if h == nil || !equal(h, header) {
		return t, i, false
	}
	if sep := splitRow(lines[i+1]); len(sep) != len(h) || !allMatch(sep, sepCell) {
		return t, i, false
	}
	t.header = h
	j := i + 2
	for ; j < len(lines); j++ {
		r := splitRow(lines[j])
		if r == nil {
			break
		}
		t.rows = append(t.rows, r)
	}
	return t, j, true
}

// priceTables は、見出し（lines[hd]）の 1 行空けたすぐ後の wide の表と、空行を挟んで続く narrow の表（あれば）の行を、
// 控えの並び（モデル, 入力, キャッシュ済み, 書き込み, 出力, 長い入力, 長いキャッシュ済み, 長い書き込み, 長い出力）にして返す。
// wide の表の後に別の形の表が続いていれば失敗する（読まずに落とさないように）。
func priceTables(lines []string, hd int) ([][]string, error) {
	if hd+1 >= len(lines) || strings.TrimSpace(lines[hd+1]) != "" {
		return nil, errors.New("no price table with the expected columns right after its heading")
	}
	wide, next, ok := tableAt(lines, hd+2, openaiWideHeader)
	if !ok {
		return nil, errors.New("no price table with the expected columns right after its heading")
	}
	var out [][]string
	for _, r := range wide.rows {
		if len(r) != len(openaiWideHeader) {
			return nil, fmt.Errorf("row %q has %d cells", r, len(r))
		}
		// 12 時間の書き込み（r[5]・r[10]）は使わないが、形は確かめる
		if err := checkPrices(r[0], []string{r[5], r[10]}); err != nil {
			return nil, err
		}
		out = append(out, []string{r[0], r[1], r[3], r[4], r[2], r[6], r[8], r[9], r[7]})
	}
	for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
		next++
	}
	if next >= len(lines) || splitRow(lines[next]) == nil {
		return out, nil
	}
	narrow, _, ok := tableAt(lines, next, openaiNarrowHeader)
	if !ok {
		return nil, errors.New("a table with unexpected columns follows the price table")
	}
	for _, r := range narrow.rows {
		if len(r) != len(openaiNarrowHeader) {
			return nil, fmt.Errorf("row %q has %d cells", r, len(r))
		}
		out = append(out, []string{r[0], r[1], r[3], "-", r[2], "-", "-", "-", "-"})
	}
	return out, nil
}

// checkPrices は、使わない料金の欄も決まった形（"$1.25" か "-"）か確かめる。
func checkPrices(id string, cells []string) error {
	for _, c := range cells {
		if !openaiPrice.MatchString(c) {
			return fmt.Errorf("%s: unexpected price %q", id, c)
		}
	}
	return nil
}

// openaiRows は、控えの並びの行（モデル, 料金 8 つ。gpt-5 以降）を b に書き、書いた数を返す。
func openaiRows(b *strings.Builder, rows [][]string, seen map[string][]string) (int, error) {
	n := 0
	for _, r := range rows {
		if len(r) != 9 {
			return 0, fmt.Errorf("row %q has %d cells", r, len(r))
		}
		if !strings.HasPrefix(r[0], "gpt-") {
			continue
		}
		mm := openaiModel.FindStringSubmatch(r[0])
		if mm == nil {
			if strings.HasPrefix(r[0], "gpt-4") || strings.HasPrefix(r[0], "gpt-3") {
				continue // Codex が使わない古いモデル
			}
			return 0, fmt.Errorf("unexpected model name %q", r[0])
		}
		id := mm[1]
		v, err := openaiPrices(id, r[1:])
		if err != nil {
			return 0, err
		}
		if v[0] == "-" || v[3] == "-" {
			return 0, fmt.Errorf("%s: no input or output price", id)
		}
		if (v[4] == "-") != (v[7] == "-") {
			return 0, fmt.Errorf("%s: long-context input and output prices must both be given or both missing", id)
		}
		if seen[id] != nil {
			return 0, fmt.Errorf("model %s appears twice", id)
		}
		seen[id] = v
		n++
		fmt.Fprintf(b, "| %s | %s |\n", id, strings.Join(v, " | "))
	}
	return n, nil
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
