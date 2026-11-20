package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 公式のページと同じ形の合成データ（数字とモデル名は作りもの）。
const anthropicPage = `---
title: Pricing
---

Intro text.

## Model pricing

| Model                                                        | Base input tokens     | 5m cache writes | 1h cache writes | Cache hits and refreshes | Output tokens          |
| :----------------------------------------------------------- | :-------------------- | :-------------- | :-------------- | :----------------------- | :--------------------- |
| Claude Opus 9.5                                              | $4 / MTok             | $5 / MTok       | $8 / MTok       | $0.20 / MTok<sup>2</sup> | $20 / MTok             |
| Claude Opus 9 ([retired, except on X](https://example.com/)) | $15 / MTok            | $18.75 / MTok   | $30 / MTok      | $1.50 / MTok             | $75 / MTok             |
| Claude Sonnet 9                                              | $2 / MTok<sup>3</sup> | $2.50 / MTok    | $4 / MTok       | $0.20 / MTok             | $10 / MTok<sup>3</sup> |
| Claude Haiku 9.7 (for prompts up to 100,000 tokens)          | $0.10 / MTok          | $0.125 / MTok   | $0.20 / MTok    | $0.01 / MTok             | $0.50 / MTok           |
| Claude Haiku 9.7 (for prompts over 100,000 tokens)           | $0.50 / MTok          | $0.625 / MTok   | $1 / MTok       | $0.05 / MTok             | $2.50 / MTok           |
| Claude Haiku 9.5                                             | $1 / MTok             | $1.25 / MTok    | $2 / MTok       | $0.10 / MTok             | $5 / MTok              |
| Claude Haiku 3.5 ([limited availability](https://x/))        | $0.80 / MTok          | $1 / MTok       | $1.60 / MTok    | $0.08 / MTok             | $4 / MTok              |

## Cloud platform pricing

| Model | Base input tokens | 5m cache writes | 1h cache writes | Cache hits and refreshes | Output tokens |
| --- | --- | --- | --- | --- | --- |
| Claude Opus 9.5 | $9 / MTok | $9 / MTok | $9 / MTok | $9 / MTok | $9 / MTok |
`

const openaiPage = `# Pricing

Flagship models

Standard

### Standard pricing data

| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write | Long context input | Long context output | Long context cached input | Long context 30m write | Long context 12h write |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9.2-sol | $2.00 | $10.00 | $0.10 | $2.50 | $5.00 | $4.00 | $15.00 | $0.20 | $5.00 | $10.00 |
| gpt-9-sol | $2.00 | $10.00 | $0.20 | $2.50 | $5.00 | $4.00 | $15.00 | $0.40 | $5.00 | $10.00 |
| gpt-8.5 (<272K context length) | $5.00 | $30.00 | $0.50 | - | - | $10.00 | $45.00 | $1.00 | - | - |
| gpt-8.5-pro (<272K context length) | $30.00 | $180.00 | - | - | - | $60.00 | $270.00 | - | - | - |

| Model | Input | Output | Cached input |
| --- | --- | --- | --- |
| gpt-8-mini | $0.25 | $2.00 | $0.025 |
| gpt-4.1 | $2.00 | $8.00 | $0.50 |
| o3 | $2.00 | $8.00 | $0.50 |

Batch

### Batch pricing data

| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write | Long context input | Long context output | Long context cached input | Long context 30m write | Long context 12h write |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9-sol | $1.00 | $5.00 | $0.10 | $1.25 | $2.50 | $2.00 | $7.50 | $0.20 | $2.50 | $5.00 |

Fast

### Fast pricing data

| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write | Long context input | Long context output | Long context cached input | Long context 30m write | Long context 12h write |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9-sol | $4.00 | $20.00 | $0.40 | $5.00 | $10.00 | $8.00 | $30.00 | $0.80 | $10.00 | $20.00 |
| gpt-8.5 (<272K context length) | $12.50 | $75.00 | $1.25 | - | - | - | - | - | - | - |

| Model | Input | Output | Cached input |
| --- | --- | --- | --- |
| gpt-8-mini | $0.45 | $3.60 | $0.045 |
| gpt-4.1 | $3.50 | $14.00 | $0.875 |

Ultrafast

### Ultrafast pricing data

| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write | Long context input | Long context output | Long context cached input | Long context 30m write | Long context 12h write |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9-sol | $12.00 | $60.00 | $0.60 | $15.00 | $30.00 | $24.00 | $90.00 | $1.20 | $30.00 | $60.00 |

Short context: ≤272K input tokens. Long context: >272K input tokens.

For GPT-9.2 Sol, Ultrafast costs 3 times the Standard rate through some date, and 12 times after.

### Ultrafast promotion pricing data

| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write | Long context input | Long context output | Long context cached input | Long context 30m write | Long context 12h write |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9.2-sol | $6.00 | $30.00 | $0.30 | $7.50 | $15.00 | $12.00 | $45.00 | $0.60 | $15.00 | $30.00 |

### Ultrafast list pricing data

| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write | Long context input | Long context output | Long context cached input | Long context 30m write | Long context 12h write |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9.2-sol | $24.00 | $120.00 | $1.20 | $30.00 | $60.00 | $48.00 | $180.00 | $2.40 | $60.00 | $120.00 |

Multimodal models

Specialized models

Standard

### Grouped Pricing Table data

| Category | Model | Input | Output | Cached input | 30m write | 12h write |
| --- | --- | --- | --- | --- | --- | --- |
| <a id="cyber-models"></a>Cyber — Trusted Access: [X](https://example.com/) | gpt-9-cyber (short context) | $20.00 | $100.00 | $2.00 | $25.00 | $50.00 |
| Cyber — Trusted Access: [X](https://example.com/) | gpt-9-cyber (long context) | $40.00 | $150.00 | $4.00 | - | - |
| Cyber — Trusted Access: [X](https://example.com/) | gpt-8.5-cyber-preview (short context) | $12.50 | $75.00 | $1.25 | - | - |
| Cyber — Trusted Access: [X](https://example.com/) | gpt-8.5-cyber-preview (long context) | - | - | - | - | - |
| <a id="other-models"></a>Life Sciences | gpt-9-bio (short context) | $10.00 | $50.00 | $1.00 | $12.50 | $25.00 |
| ChatGPT | chat-latest | $5.00 | $30.00 | $0.50 | - | - |
| Codex | gpt-8.3-codex | $1.75 | $14.00 | $0.175 | - | - |
| Moderation | omni-moderation-latest | Free | - | - | - | - |

Fast

### Grouped Pricing Table data

| Category | Model | Input | Output | Cached input |
| --- | --- | --- | --- | --- |
| Codex | gpt-8.3-codex | $3.50 | $28.00 | $0.35 |

Tools

### Grouped Pricing Table data

| Tool | Details | Pricing |
| --- | --- | --- |
| Web search | Web search | $10.00 / 1k calls |
`

func TestParseAnthropic(t *testing.T) {
	got, err := parseAnthropic(anthropicPage)
	if err != nil {
		t.Fatal(err)
	}
	want := "Long over: a request whose prompt is more than this many tokens is billed at the Long rates as a whole (\"-\": one price for every length).\n\n" +
		"| Model ID | Input | Cache write 5m | Cache write 1h | Cache read | Output | Long over | Long input | Long cache write 5m | Long cache write 1h | Long cache read | Long output |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|---|\n" +
		"| claude-opus-9-5 | 4 | 5 | 8 | 0.2 | 20 | - | - | - | - | - | - |\n" +
		"| claude-opus-9 | 15 | 18.75 | 30 | 1.5 | 75 | - | - | - | - | - | - |\n" +
		"| claude-sonnet-9 | 2 | 2.5 | 4 | 0.2 | 10 | - | - | - | - | - | - |\n" +
		"| claude-haiku-9-7 | 0.1 | 0.125 | 0.2 | 0.01 | 0.5 | 100000 | 0.5 | 0.625 | 1 | 0.05 | 2.5 |\n" +
		"| claude-haiku-9-5 | 1 | 1.25 | 2 | 0.1 | 5 | - | - | - | - | - | - |\n" +
		"| claude-3-5-haiku | 0.8 | 1 | 1.6 | 0.08 | 4 | - | - | - | - | - | - |\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	// "over" の行が先でも同じ
	up := "| Claude Haiku 9.7 (for prompts up to 100,000 tokens)          | $0.10 / MTok          | $0.125 / MTok   | $0.20 / MTok    | $0.01 / MTok             | $0.50 / MTok           |\n"
	swapped := strings.Replace(strings.Replace(anthropicPage, up, "", 1), "| Claude Haiku 9.5 ", strings.TrimSuffix(up, "\n")+"\n| Claude Haiku 9.5 ", 1)
	if got, err := parseAnthropic(swapped); err != nil || got != want {
		t.Errorf("swapped: err=%v got\n%s", err, got)
	}
}

func TestParseOpenAI(t *testing.T) {
	got, err := parseOpenAI(openaiPage)
	if err != nil {
		t.Fatal(err)
	}
	want := "Long context: more than 272K input tokens in one request (the whole request is billed at the long-context rates).\n\n" +
		"| Model ID | Input | Cached input | Cache write | Output | Long input | Long cached input | Long cache write | Long output |\n|---|---|---|---|---|---|---|---|---|\n" +
		// 長いコンテキストの料金がある表、続く表（Cached input と Output の並びを入れ替え、書き込みは "-"）。書き込みは 30 分の料金
		"| gpt-9.2-sol | 2 | 0.1 | 2.5 | 10 | 4 | 0.2 | 5 | 15 |\n" +
		"| gpt-9-sol | 2 | 0.2 | 2.5 | 10 | 4 | 0.4 | 5 | 15 |\n" +
		"| gpt-8.5 | 5 | 0.5 | - | 30 | 10 | 1 | - | 45 |\n" +
		"| gpt-8.5-pro | 30 | - | - | 180 | 60 | - | - | 270 |\n" +
		"| gpt-8-mini | 0.25 | 0.025 | - | 2 | - | - | - | - |\n" +
		// Specialized models の Cyber（short と long の 2 行を 1 行に）と Codex。Life Sciences は読まない
		"| gpt-9-cyber | 20 | 2 | 25 | 100 | 40 | 4 | - | 150 |\n" +
		"| gpt-8.5-cyber-preview | 12.5 | 1.25 | - | 75 | - | - | - | - |\n" +
		"| gpt-8.3-codex | 1.75 | 0.175 | - | 14 | - | - | - | - |\n" +
		// サービスティア（Flex の表はないので書かない）。Specialized models の Fast の表の Codex は Fast の節に足す
		"\n### Fast\n\nThe Fast service tier (Codex `service_tier`) instead of the standard rates above.\n\n" +
		"| Model ID | Input | Cached input | Cache write | Output | Long input | Long cached input | Long cache write | Long output |\n|---|---|---|---|---|---|---|---|---|\n" +
		"| gpt-9-sol | 4 | 0.4 | 5 | 20 | 8 | 0.8 | 10 | 30 |\n" +
		"| gpt-8.5 | 12.5 | 1.25 | - | 75 | - | - | - | - |\n" +
		"| gpt-8-mini | 0.45 | 0.045 | - | 3.6 | - | - | - | - |\n" +
		"| gpt-8.3-codex | 3.5 | 0.35 | - | 28 | - | - | - | - |\n" +
		// Ultrafast の list の表は足し、promotion の表（期間限定）は使わない
		"\n### Ultrafast\n\nThe Ultrafast service tier (Codex `service_tier`) instead of the standard rates above.\n\n" +
		"| Model ID | Input | Cached input | Cache write | Output | Long input | Long cached input | Long cache write | Long output |\n|---|---|---|---|---|---|---|---|---|\n" +
		"| gpt-9-sol | 12 | 0.6 | 15 | 60 | 24 | 1.2 | 30 | 90 |\n" +
		"| gpt-9.2-sol | 24 | 1.2 | 30 | 120 | 48 | 2.4 | 60 | 180 |\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// 形が違えば、推測せずに失敗する。
func TestParseRejectsUnexpectedShape(t *testing.T) {
	bad := map[string]struct {
		parse func(string) (string, error)
		page  string
	}{
		"anthropic column renamed":     {parseAnthropic, strings.Replace(anthropicPage, "| Output tokens          |", "| Output                 |", 1)},
		"anthropic no heading":         {parseAnthropic, strings.Replace(anthropicPage, "## Model pricing", "## Prices", 1)},
		"anthropic odd price":          {parseAnthropic, strings.Replace(anthropicPage, "$18.75 / MTok", "$18.75 per MTok", 1)},
		"anthropic odd name":           {parseAnthropic, strings.Replace(anthropicPage, "Claude Sonnet 9 ", "Sonnet Nine     ", 1)},
		"anthropic script in name":     {parseAnthropic, strings.Replace(anthropicPage, "Claude Sonnet 9 ", "<script>x</script>", 1)},
		"anthropic too few rows":       {parseAnthropic, strings.Join(strings.Split(anthropicPage, "\n")[:12], "\n")},
		"anthropic one tier only":      {parseAnthropic, strings.Replace(anthropicPage, "Claude Haiku 9.7 (for prompts over 100,000 tokens) ", "Claude Haiku 9.8 (for prompts over 100,000 tokens) ", 1)},
		"anthropic tiers differ":       {parseAnthropic, strings.Replace(anthropicPage, "(for prompts over 100,000 tokens) ", "(for prompts over 200,000 tokens) ", 1)},
		"anthropic same tier twice":    {parseAnthropic, strings.Replace(anthropicPage, "(for prompts over 100,000 tokens) ", "(for prompts up to 100,000 tokens)", 1)},
		"anthropic odd tier":           {parseAnthropic, strings.Replace(anthropicPage, "(for prompts over 100,000 tokens) ", "(for prompts over 100K tokens)      ", 1)},
		"anthropic tier and plain":     {parseAnthropic, strings.Replace(anthropicPage, "Claude Haiku 9.7 (for prompts over 100,000 tokens) ", "Claude Haiku 9.7                                    ", 1)},
		"openai column renamed":        {parseOpenAI, strings.Replace(openaiPage, "Short context output |", "Output |", 1)},
		"openai odd price":             {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol | $2.00 |", "| gpt-9-sol | 2 USD |", 1)},
		"openai odd model":             {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol |", "| gpt-9 sol |", 1)},
		"openai no threshold":          {parseOpenAI, strings.Replace(openaiPage, "Long context: >272K", "Long: >272K", 1)},
		"openai no codex":              {parseOpenAI, strings.Replace(openaiPage, "| Codex | gpt-8.3-codex | $1.75", "| Other | gpt-8.3-codex | $1.75", 1)},
		"openai no specialized":        {parseOpenAI, strings.Replace(openaiPage, "Specialized models", "Other models", 1)},
		"openai missing output":        {parseOpenAI, strings.Replace(openaiPage, "| gpt-8-mini | $0.25 | $2.00 | $0.025 |", "| gpt-8-mini | $0.25 | - | $0.025 |", 1)},
		"openai odd 12h write":         {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol | $2.00 | $10.00 | $0.20 | $2.50 | $5.00 |", "| gpt-9-sol | $2.00 | $10.00 | $0.20 | $2.50 | 5 USD |", 1)},
		"openai narrow renamed":        {parseOpenAI, strings.Replace(openaiPage, "| Model | Input | Output | Cached input |", "| Model | Input | Cached input | Output |", 1)},
		"openai narrow odd price":      {parseOpenAI, strings.Replace(openaiPage, "| gpt-8-mini | $0.25 |", "| gpt-8-mini | 0.25 |", 1)},
		"openai cyber no long":         {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-cyber (long context) |", "| gpt-9-other (long context) |", 1)},
		"openai cyber odd name":        {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-cyber (long context) |", "| gpt-9-cyber (longer context) |", 1)},
		"openai cyber short twice":     {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-cyber (long context) |", "| gpt-9-cyber (short context) |", 1)},
		"openai cyber odd 12h":         {parseOpenAI, strings.Replace(openaiPage, "| $25.00 | $50.00 |", "| $25.00 | 50 |", 1)},
		"openai special renamed":       {parseOpenAI, strings.Replace(openaiPage, "| Cached input | 30m write | 12h write |", "| Cached input | Cache write | 12h write |", 1)},
		"openai promotion only":        {parseOpenAI, strings.Replace(openaiPage, "### Ultrafast list pricing data", "### Ultrafast other pricing data", 1)},
		"openai list not promotion":    {parseOpenAI, strings.Replace(openaiPage, "| gpt-9.2-sol | $6.00 |", "| gpt-9.1-sol | $6.00 |", 1)},
		"openai list duplicates":       {parseOpenAI, strings.Replace(strings.Replace(openaiPage, "| gpt-9.2-sol | $24.00 |", "| gpt-9-sol | $24.00 |", 1), "| gpt-9.2-sol | $6.00 |", "| gpt-9-sol | $6.00 |", 1)},
		"openai list no table":         {parseOpenAI, strings.Replace(openaiPage, "### Ultrafast list pricing data\n\n|", "### Ultrafast list pricing data\n\nText.\n\n|", 1)},
		"openai duplicate standard":    {parseOpenAI, openaiPage + "\n### Standard pricing data\n"},
		"openai duplicate tier":        {parseOpenAI, openaiPage + "\n### Fast pricing data\n"},
		"openai tier no table":         {parseOpenAI, strings.Replace(openaiPage, "### Ultrafast pricing data\n\n|", "### Ultrafast pricing data\n\nText.\n\n|", 1)},
		"openai tier odd price":        {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol | $12.00 |", "| gpt-9-sol | 12 USD |", 1)},
		"openai tier odd model":        {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol | $12.00 |", "| gpt-9 sol | $12.00 |", 1)},
		"openai tier no models":        {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol | $12.00 |", "| o3 | $12.00 |", 1)},
		"openai tier duplicate row":    {parseOpenAI, strings.Replace(openaiPage, "| gpt-8.5 (<272K context length) | $12.50 |", "| gpt-9-sol | $12.50 |", 1)},
		"openai special fast twice":    {parseOpenAI, strings.Replace(openaiPage, "| Codex | gpt-8.3-codex | $3.50 | $28.00 | $0.35 |", "| Codex | gpt-8.3-codex | $3.50 | $28.00 | $0.35 |\n| Codex | gpt-8.3-codex | $3.50 | $28.00 | $0.35 |", 1)},
		"openai special fast no table": {parseOpenAI, strings.Replace(openaiPage, "| Category | Model | Input | Output | Cached input |\n", "| Category | Model | Input | Cached input | Output |\n", 1)},
		"openai special fast odd":      {parseOpenAI, strings.Replace(openaiPage, "| Codex | gpt-8.3-codex | $3.50 |", "| Codex | gpt-8.3-codex | 3.50 |", 1)},
	}
	for name, c := range bad {
		if out, err := c.parse(c.page); err == nil {
			t.Errorf("%s: no error, got\n%s", name, out)
		}
	}
}

// 数字が変わったときだけ書き直す（日付も）。変わらなければ日付も含めてそのまま（CI の差分が出ない）。
func TestRunWritesOnlyChanges(t *testing.T) {
	dir := t.TempDir()
	pages := map[string]string{anthropicURL: anthropicPage, openaiURL: openaiPage}
	get := func(u string) ([]byte, error) {
		p, ok := pages[u]
		if !ok {
			return nil, errors.New("unknown URL")
		}
		return []byte(p), nil
	}
	day1 := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	if err := run(dir, day1, get); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	a1 := read("anthropic-pricing.md")
	if !strings.Contains(a1, "- Fetched: 2026-10-07") || !strings.Contains(a1, "- Source: "+anthropicURL) || !strings.HasPrefix(a1, "<!-- Generated") {
		t.Errorf("header:\n%s", a1)
	}
	if err := run(dir, day1.AddDate(0, 0, 7), get); err != nil {
		t.Fatal(err)
	}
	if read("anthropic-pricing.md") != a1 {
		t.Error("同じ数字なのに書き直した")
	}
	pages[openaiURL] = strings.Replace(openaiPage, "| gpt-9-sol | $2.00 |", "| gpt-9-sol | $3.00 |", 1)
	if err := run(dir, day1.AddDate(0, 0, 14), get); err != nil {
		t.Fatal(err)
	}
	if o := read("openai-pricing.md"); !strings.Contains(o, "- Fetched: 2026-10-21") || !strings.Contains(o, "| gpt-9-sol | 3 |") {
		t.Errorf("変わった数字を書いていない:\n%s", o)
	}
	if read("anthropic-pricing.md") != a1 {
		t.Error("変わっていない方まで書き直した")
	}
	// 片方が読めなければ、どちらも書かない
	pages[anthropicURL] = "nothing"
	pages[openaiURL] = openaiPage
	before := read("openai-pricing.md")
	if err := run(dir, day1.AddDate(0, 0, 21), get); err == nil {
		t.Error("読めないページで失敗しない")
	}
	if read("openai-pricing.md") != before {
		t.Error("失敗したのに書いた")
	}
}
