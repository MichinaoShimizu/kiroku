// upstream は、kiroku が履歴を読むエージェントの最新の版を公開の情報から取ってきて、docs/upstream/agents.md に書く。
// 週 1 回のワークフロー（.github/workflows/upstream.yml）が控えと比べ、変わっていたら issue を立てる
// （新しい版で履歴の形が変わっていないかを /source-audit で調べるきっかけ）。kiroku 本体は使わない。
//
// 取ってくるものはネットワークからの信用できない文字なので、版の形（数字と点、決まった長さ）だけを受け取り、
// それ以外は書かずに失敗する（形が変わったら、ワークフローの失敗として知らせる）。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const out = "docs/upstream/agents.md"

var (
	semver    = regexp.MustCompile(`^[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,6}$`)
	kiroEntry = regexp.MustCompile(`href="/changelog/(cli|ide)/([0-9]{1,3})-([0-9]{1,3})(?:-([0-9]{1,3}))?/?[#"]`) // 2-31 と、パッチの 2-31-1
)

type agent struct {
	name, source string
	fetch        func(*http.Client) (string, error)
}

func main() {
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		if r.URL.Scheme != "https" {
			return errors.New("redirect to a non-https address")
		}
		return nil
	}}
	var kiroPage []byte // Kiro CLI と Kiro IDE は同じページから読む
	kiro := func(kind string) func(*http.Client) (string, error) {
		return func(c *http.Client) (string, error) {
			if kiroPage == nil {
				b, err := get(c, "https://kiro.dev/changelog/")
				if err != nil {
					return "", err
				}
				kiroPage = b
			}
			return kiroLatest(kiroPage, kind)
		}
	}
	agents := []agent{
		{"Claude Code", "npm @anthropic-ai/claude-code", npmLatest("@anthropic-ai/claude-code")},
		{"Codex", "npm @openai/codex", npmLatest("@openai/codex")},
		{"Kiro CLI", "kiro.dev/changelog (CLI entries)", kiro("cli")},
		{"Kiro IDE", "kiro.dev/changelog (IDE entries)", kiro("ide")},
	}
	var b strings.Builder
	b.WriteString("# Latest versions of the agents kiroku reads\n\n")
	b.WriteString("Written by `go run ./tools/upstream`; the weekly Upstream workflow opens an issue when this file changes.\n")
	b.WriteString("A new version is a reason to check that kiroku still reads its history (`/source-audit`), not proof that anything broke.\n\n")
	b.WriteString("| Agent | Latest | Source |\n|---|---|---|\n")
	for _, a := range agents {
		v, err := a.fetch(client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", a.name, err)
			os.Exit(1)
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", a.name, v, a.source)
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(b.String())
}

// get は https の URL を読む（2 MiB まで）。
func get(c *http.Client, url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}

// npmLatest は npm のパッケージの latest の版。
func npmLatest(pkg string) func(*http.Client) (string, error) {
	return func(c *http.Client) (string, error) {
		b, err := get(c, "https://registry.npmjs.org/"+pkg+"/latest")
		if err != nil {
			return "", err
		}
		return npmVersion(b)
	}
}

func npmVersion(b []byte) (string, error) {
	var m struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	if !semver.MatchString(m.Version) {
		return "", fmt.Errorf("unexpected version %q", m.Version)
	}
	return m.Version, nil
}

// kiroLatest は kiro.dev/changelog の記事へのリンク（/changelog/cli/2-31/ など）から、いちばん新しい版（2.31）。
func kiroLatest(page []byte, kind string) (string, error) {
	best := [3]int{-1, -1, -1} // パッチのない 2-31 は -1 にして、2-31-0 より前に並べる
	for _, m := range kiroEntry.FindAllSubmatch(page, -1) {
		if string(m[1]) != kind {
			continue
		}
		v := [3]int{-1, -1, -1}
		for i := range v {
			if len(m[i+2]) > 0 {
				v[i], _ = strconv.Atoi(string(m[i+2]))
			}
		}
		if slices.Compare(v[:], best[:]) > 0 {
			best = v
		}
	}
	if best[0] < 0 {
		return "", fmt.Errorf("no %s entries on the changelog page (its format may have changed)", kind)
	}
	if best[2] < 0 {
		return fmt.Sprintf("%d.%d", best[0], best[1]), nil
	}
	return fmt.Sprintf("%d.%d.%d", best[0], best[1], best[2]), nil
}
