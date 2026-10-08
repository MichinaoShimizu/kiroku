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

### Standard pricing data

| Model | Short context input | Short context cached input | Short context cache writes | Short context output | Long context input | Long context cached input | Long context cache writes | Long context output |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9-sol | $2.00 | $0.20 | $2.50 | $10.00 | $4.00 | $0.40 | $5.00 | $15.00 |
| gpt-8.5 (<272K context length) | $5.00 | $0.50 | - | $30.00 | $10.00 | $1.00 | - | $45.00 |
| gpt-8.5-pro (<272K context length) | $30.00 | - | - | $180.00 | $60.00 | - | - | $270.00 |
| gpt-8-mini | $0.25 | $0.025 | - | $2.00 | - | - | - | - |
| gpt-4.1 | $2.00 | $0.50 | - | $8.00 | - | - | - | - |
| o3 | $2.00 | $0.50 | - | $8.00 | - | - | - | - |

### Batch pricing data

| Model | Short context input | Short context cached input | Short context cache writes | Short context output | Long context input | Long context cached input | Long context cache writes | Long context output |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gpt-9-sol | $1.00 | $0.10 | $1.25 | $5.00 | $2.00 | $0.20 | $2.50 | $7.50 |

Short context: ≤272K input tokens. Long context: >272K input tokens.

Specialized models

Standard

### Grouped Pricing Table data

| Category | Model | Input | Cached input | Output |
| --- | --- | --- | --- | --- |
| ChatGPT | chat-latest | $5.00 | $0.50 | $30.00 |
| Codex | gpt-8.3-codex | $1.75 | $0.175 | $14.00 |

Fast

### Grouped Pricing Table data

| Category | Model | Input | Cached input | Output |
| --- | --- | --- | --- | --- |
| Codex | gpt-8.3-codex | $3.50 | $0.35 | $28.00 |
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
		"| gpt-9-sol | 2 | 0.2 | 2.5 | 10 | 4 | 0.4 | 5 | 15 |\n" +
		"| gpt-8.5 | 5 | 0.5 | - | 30 | 10 | 1 | - | 45 |\n" +
		"| gpt-8.5-pro | 30 | - | - | 180 | 60 | - | - | 270 |\n" +
		"| gpt-8-mini | 0.25 | 0.025 | - | 2 | - | - | - | - |\n" +
		"| gpt-8.3-codex | 1.75 | 0.175 | - | 14 | - | - | - | - |\n"
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
		"anthropic column renamed":  {parseAnthropic, strings.Replace(anthropicPage, "| Output tokens          |", "| Output                 |", 1)},
		"anthropic no heading":      {parseAnthropic, strings.Replace(anthropicPage, "## Model pricing", "## Prices", 1)},
		"anthropic odd price":       {parseAnthropic, strings.Replace(anthropicPage, "$18.75 / MTok", "$18.75 per MTok", 1)},
		"anthropic odd name":        {parseAnthropic, strings.Replace(anthropicPage, "Claude Sonnet 9 ", "Sonnet Nine     ", 1)},
		"anthropic script in name":  {parseAnthropic, strings.Replace(anthropicPage, "Claude Sonnet 9 ", "<script>x</script>", 1)},
		"anthropic too few rows":    {parseAnthropic, strings.Join(strings.Split(anthropicPage, "\n")[:12], "\n")},
		"anthropic one tier only":   {parseAnthropic, strings.Replace(anthropicPage, "Claude Haiku 9.7 (for prompts over 100,000 tokens) ", "Claude Haiku 9.8 (for prompts over 100,000 tokens) ", 1)},
		"anthropic tiers differ":    {parseAnthropic, strings.Replace(anthropicPage, "(for prompts over 100,000 tokens) ", "(for prompts over 200,000 tokens) ", 1)},
		"anthropic same tier twice": {parseAnthropic, strings.Replace(anthropicPage, "(for prompts over 100,000 tokens) ", "(for prompts up to 100,000 tokens)", 1)},
		"anthropic odd tier":        {parseAnthropic, strings.Replace(anthropicPage, "(for prompts over 100,000 tokens) ", "(for prompts over 100K tokens)      ", 1)},
		"anthropic tier and plain":  {parseAnthropic, strings.Replace(anthropicPage, "Claude Haiku 9.7 (for prompts over 100,000 tokens) ", "Claude Haiku 9.7                                    ", 1)},
		"openai column renamed":     {parseOpenAI, strings.Replace(openaiPage, "Short context output |", "Output |", 1)},
		"openai odd price":          {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol | $2.00 |", "| gpt-9-sol | 2 USD |", 1)},
		"openai odd model":          {parseOpenAI, strings.Replace(openaiPage, "| gpt-9-sol |", "| gpt-9 sol |", 1)},
		"openai no threshold":       {parseOpenAI, strings.Replace(openaiPage, "Long context: >272K", "Long: >272K", 1)},
		"openai no codex":           {parseOpenAI, strings.Replace(openaiPage, "| Codex | gpt-8.3-codex | $1.75", "| Other | gpt-8.3-codex | $1.75", 1)},
		"openai no specialized":     {parseOpenAI, strings.Replace(openaiPage, "Specialized models", "Other models", 1)},
		"openai missing output":     {parseOpenAI, strings.Replace(openaiPage, "| gpt-8-mini | $0.25 | $0.025 | - | $2.00 |", "| gpt-8-mini | $0.25 | $0.025 | - | - |", 1)},
		"openai duplicate standard": {parseOpenAI, openaiPage + "\n### Standard pricing data\n"},
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
