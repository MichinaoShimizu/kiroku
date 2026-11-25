// upstream は、kiroku が履歴を読むエージェントの最新の版を公開の情報から取ってきて、docs/upstream/agents.md に書く。
// 週 1 回のワークフロー（.github/workflows/upstream.yml）が控えと比べ、変わっていたら issue を立てる
// （新しい版で履歴の形が変わっていないかを /source-audit で調べるきっかけ）。kiroku 本体は使わない。
// Dependabot が追わない、ワークフローで版を固定した道具（staticcheck・govulncheck・syft・GoReleaser）も、固定した版と
// 最新の版を並べて書く（新しい版が出れば同じ issue で知らせる。上げるのは手で）。
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

// workflows は、道具の版を固定しているワークフロー（リポジトリの根からの場所）。
var workflows = []string{".github/workflows/ci.yml", ".github/workflows/release.yml"}

// tool は、ワークフローで版を固定している道具。pinned はワークフローの文から固定した版を取り出す。
type tool struct {
	name, source string
	shape        *regexp.Regexp // 版の形（先頭の v を除いたもの）。ワークフローとネットワークのどちらの文字も、これに合うものだけを受け取る
	pinned       func(text string) []string
	fetch        func(*http.Client) (string, error)
}

var (
	yearly = regexp.MustCompile(`^20[0-9]{2}\.[0-9]{1,2}(\.[0-9]{1,3})?$`) // staticcheck の 2026.2.2・2026.1
)

func tools() []tool {
	return []tool{
		{"staticcheck", "GitHub dominikh/go-tools (latest release)", yearly, goRunPin("honnef.co/go/tools/cmd/staticcheck"), githubLatestShape("dominikh/go-tools", yearly)},
		{"govulncheck", "proxy.golang.org golang.org/x/vuln (@latest)", semver, goRunPin("golang.org/x/vuln/cmd/govulncheck"), proxyLatest("golang.org/x/vuln")},
		{"syft", "GitHub anchore/syft (latest release)", semver, stepPin("anchore/sbom-action/download-syft@", "syft-version"), githubLatestShape("anchore/syft", semver)},
		{"GoReleaser", "GitHub goreleaser/goreleaser (latest release)", semver, stepPin("goreleaser/goreleaser-action@", "version"), githubLatestShape("goreleaser/goreleaser", semver)},
	}
}

// goRunPin は「go run <pkg>@<版>」の版。
func goRunPin(pkg string) func(string) []string {
	re := regexp.MustCompile(regexp.QuoteMeta(pkg) + `@([^\s"'\x60]+)`)
	return func(text string) []string {
		var out []string
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			out = append(out, m[1])
		}
		return out
	}
}

// stepPin は、uses が prefix で始まるステップの with: の下の key の値（行末のコメントは除く）。
func stepPin(prefix, key string) func(string) []string {
	keyRe := regexp.MustCompile(`^\s+` + regexp.QuoteMeta(key) + `:\s*["']?([^\s"'#]+)`)
	return func(text string) []string {
		var out []string
		in := false
		for _, line := range strings.Split(text, "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "- ") { // 次のステップ
				in = strings.Contains(trim, "uses: "+prefix)
				continue
			}
			if m := keyRe.FindStringSubmatch(line); in && m != nil {
				out = append(out, m[1])
			}
		}
		return out
	}
}

// pinnedVersions は、ワークフローの文 texts で固定した t の版（先頭の v を除き、重なりなし、出てきた順）。
// 見つからない・版の形でないものがあれば失敗する（ワークフローの書き方が変わったら知らせる）。
func pinnedVersions(t tool, texts []string) ([]string, error) {
	var out []string
	for _, text := range texts {
		for _, v := range t.pinned(text) {
			v = strings.TrimPrefix(v, "v")
			if !t.shape.MatchString(v) {
				return nil, fmt.Errorf("%s: unexpected pinned version %q", t.name, v)
			}
			if !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no pinned version found in %s", t.name, strings.Join(workflows, ", "))
	}
	return out, nil
}

// proxyLatest は Go のモジュール mod の @latest の版（proxy.golang.org）。
func proxyLatest(mod string) func(*http.Client) (string, error) {
	return func(c *http.Client) (string, error) {
		b, err := get(c, "https://proxy.golang.org/"+mod+"/@latest")
		if err != nil {
			return "", err
		}
		return proxyVersion(b)
	}
}

// proxyVersion は proxy.golang.org の @latest（{"Version":"v1.9.0",...}）の版（1.9.0）。版の形でなければ失敗する。
func proxyVersion(b []byte) (string, error) {
	var m struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	v := strings.TrimPrefix(m.Version, "v")
	if !semver.MatchString(v) {
		return "", fmt.Errorf("unexpected module version %q", m.Version)
	}
	return v, nil
}

// toolsTable は道具の表（Markdown）。pinned と latest は、版の形を確かめたものだけ。
func toolsTable(ts []tool, pinned [][]string, latest []string) string {
	var b strings.Builder
	b.WriteString("\n## Tools pinned in the workflows\n\n")
	b.WriteString("Dependabot does not track these. Pinned is read from `.github/workflows/ci.yml` and `release.yml`; when Latest is newer, bump the pin there by hand (`tools/check.sh` reads staticcheck and govulncheck from `ci.yml`) and update Pinned here.\n\n")
	b.WriteString("| Tool | Pinned | Latest | Source |\n|---|---|---|---|\n")
	for i, t := range ts {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", t.name, strings.Join(pinned[i], ", "), latest[i], t.source)
	}
	return b.String()
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
		{"Kiro Crew", "GitHub kirodotdev/kirocrew (latest release)", githubLatest("kirodotdev/kirocrew")},
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
	var texts []string
	for _, w := range workflows {
		t, err := os.ReadFile(w)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		texts = append(texts, string(t))
	}
	ts := tools()
	pinned := make([][]string, len(ts))
	latest := make([]string, len(ts))
	for i, t := range ts {
		p, err := pinnedVersions(t, texts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		v, err := t.fetch(client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", t.name, err)
			os.Exit(1)
		}
		pinned[i], latest[i] = p, v
	}
	b.WriteString(toolsTable(ts, pinned, latest))
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

// githubLatest は GitHub のリポジトリ repo の最新のリリース（下書きと pre-release を除く）の版。
func githubLatest(repo string) func(*http.Client) (string, error) {
	return githubLatestShape(repo, semver)
}

// githubLatestShape は githubLatest と同じで、版の形を shape で確かめる（staticcheck の 2026.2.2 など）。
func githubLatestShape(repo string, shape *regexp.Regexp) func(*http.Client) (string, error) {
	return func(c *http.Client) (string, error) {
		b, err := get(c, "https://api.github.com/repos/"+repo+"/releases/latest")
		if err != nil {
			return "", err
		}
		return githubTag(b, shape)
	}
}

// githubVersion はリリースの tag_name（v0.14.0）から版（0.14.0）を取る。版の形でなければ失敗する。
func githubVersion(b []byte) (string, error) { return githubTag(b, semver) }

// githubTag は githubVersion と同じで、版の形を shape で確かめる。
func githubTag(b []byte, shape *regexp.Regexp) (string, error) {
	var m struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	v := strings.TrimPrefix(m.Tag, "v")
	if !shape.MatchString(v) {
		return "", fmt.Errorf("unexpected release tag %q", m.Tag)
	}
	return v, nil
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
