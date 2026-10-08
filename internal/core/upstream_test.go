package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// upstreamTable は docs/upstream/<name> の表（Model ID → 列の値。"-" は料金なし）と、取ってきた日（YYYY-MM-DD）。
func upstreamTable(t *testing.T, name string, cols int) (map[string][]string, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "upstream", name))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	m := regexp.MustCompile(`(?m)^- Fetched: (\d{4}-\d{2}-\d{2})\b`).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("%s: 取ってきた日がない", name)
	}
	rows := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| Model ID ") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		if len(cells) != cols+1 {
			t.Fatalf("%s: 列の数が違う: %q", name, line)
		}
		if _, dup := rows[cells[0]]; dup {
			t.Fatalf("%s: %s が 2 回ある", name, cells[0])
		}
		rows[cells[0]] = cells[1:]
	}
	if len(rows) == 0 {
		t.Fatalf("%s: 表がない", name)
	}
	return rows, m[1]
}

// num は表の値（"-" は or の値）。
func num(t *testing.T, s string, or float64) float64 {
	t.Helper()
	if s == "-" {
		return or
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("数字でない: %q", s)
	}
	return v
}

// 料金表（Prices・LongPrices・OpenAIPrices・OpenAILongPrices）が、公式のページから tools/prices が作った控え（docs/upstream）と同じか。
// 控えだけ新しくしてコードを直し忘れると、ここで落ちる。
func TestPricesMatchUpstream(t *testing.T) {
	var fetched []string

	anth, d := upstreamTable(t, "anthropic-pricing.md", 11)
	fetched = append(fetched, d)
	for id, c := range anth {
		// 控えの並び: 入力, 5 分, 1 時間, 読み込み, 出力, 長いときの境目, 長いときの 入力, 5 分, 1 時間, 読み込み, 出力
		// → Prices の並び: 入力, 出力, 5 分, 1 時間, 読み込み
		want := [5]float64{num(t, c[0], -1), num(t, c[4], -1), num(t, c[1], -1), num(t, c[2], -1), num(t, c[3], -1)}
		if got, ok := Prices[id]; !ok {
			t.Errorf("Prices に %s がない（公式の料金 %v）", id, want)
		} else if got != want {
			t.Errorf("Prices[%q] = %v, 公式は %v", id, got, want)
		}
		got, ok := LongPrices[id]
		if c[5] == "-" {
			if ok {
				t.Errorf("LongPrices の %s は公式にプロンプトの長さによる料金がない", id)
			}
			continue
		}
		l := LongPrice{Over: num(t, c[5], -1), Rates: [5]float64{num(t, c[6], -1), num(t, c[10], -1), num(t, c[7], -1), num(t, c[8], -1), num(t, c[9], -1)}}
		if !ok {
			t.Errorf("LongPrices に %s がない（公式の料金 %v）", id, l)
		} else if got != l {
			t.Errorf("LongPrices[%q] = %v, 公式は %v", id, got, l)
		}
	}
	for id := range Prices {
		if _, ok := anth[id]; !ok {
			t.Errorf("Prices の %s は公式の表にない（消えたモデルなら料金表からも消す）", id)
		}
	}
	for id := range LongPrices {
		if _, ok := anth[id]; !ok {
			t.Errorf("LongPrices の %s は公式の表にない", id)
		}
	}

	oai, d := upstreamTable(t, "openai-pricing.md", 8)
	fetched = append(fetched, d)
	for id, c := range oai {
		// 控えの並び: 入力, キャッシュ済み, 書き込み, 出力, 長い入力, 長いキャッシュ済み, 長い書き込み, 長い出力。
		// キャッシュ済み・書き込みの料金がない（"-"）ものは入力の料金にする
		in := num(t, c[0], -1)
		cw := num(t, c[2], in)
		want := [5]float64{in, num(t, c[3], -1), cw, cw, num(t, c[1], in)}
		if got, ok := OpenAIPrices[id]; !ok {
			t.Errorf("OpenAIPrices に %s がない（公式の料金 %v）", id, want)
		} else if got != want {
			t.Errorf("OpenAIPrices[%q] = %v, 公式は %v", id, got, want)
		}
		got, ok := OpenAILongPrices[id]
		if c[4] == "-" {
			if ok {
				t.Errorf("OpenAILongPrices の %s は公式に長いコンテキストの料金がない", id)
			}
			continue
		}
		lin := num(t, c[4], -1)
		lcw := num(t, c[6], lin)
		want = [5]float64{lin, num(t, c[7], -1), lcw, lcw, num(t, c[5], lin)}
		if !ok {
			t.Errorf("OpenAILongPrices に %s がない（公式の料金 %v）", id, want)
		} else if got != want {
			t.Errorf("OpenAILongPrices[%q] = %v, 公式は %v", id, got, want)
		}
	}
	for id := range OpenAIPrices {
		if _, ok := oai[id]; !ok {
			t.Errorf("OpenAIPrices の %s は公式の表にない", id)
		}
	}
	for id := range OpenAILongPrices {
		if _, ok := oai[id]; !ok {
			t.Errorf("OpenAILongPrices の %s は公式の表にない", id)
		}
	}
	b, _ := os.ReadFile(filepath.Join("..", "..", "docs", "upstream", "openai-pricing.md"))
	if m := regexp.MustCompile(`more than (\d+)K input tokens`).FindStringSubmatch(string(b)); m == nil || m[1]+"000" != strconv.Itoa(OpenAILongContext) {
		t.Errorf("OpenAILongContext = %d, 控えは %v", OpenAILongContext, m)
	}

	// 画面に出す時点（PricesAsOf）は、控えの数字が変わった月より前にしない
	for _, d := range fetched {
		if PricesAsOf < d[:7] {
			t.Errorf("PricesAsOf = %s, 控えは %s に変わっている", PricesAsOf, d)
		}
	}
}
