package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ワークフローの書き方の見本（ci.yml・release.yml と同じ形）。
const ciFixture = `jobs:
  test:
    steps:
      - uses: actions/setup-node@0000000000000000000000000000000000000000 # v6
        with:
          node-version: 22
      - name: staticcheck
        run: GOTOOLCHAIN=$(go env GOVERSION) go run honnef.co/go/tools/cmd/staticcheck@2026.2.2 ./...
      - name: govulncheck
        run: GOTOOLCHAIN=$(go env GOVERSION) go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
      - uses: anchore/sbom-action/download-syft@66cbf4bc1f1c0d2edc94016e65bc221b6bb0ad6c # v0.24.3
        with:
          syft-version: v1.54.1
      - uses: goreleaser/goreleaser-action@a4f02ab5355a589de60d917a659ce617ed19b3de # v7.3.0
        with:
          distribution: goreleaser
          version: v2.18.3 # pinned
          args: release --snapshot
      - uses: other/action@0000000000000000000000000000000000000000
        with:
          version: v9.9.9
`

const releaseFixture = `jobs:
  release:
    steps:
      - uses: anchore/sbom-action/download-syft@66cbf4bc1f1c0d2edc94016e65bc221b6bb0ad6c # v0.24.3
        with:
          syft-version: "v1.55.0"
      - uses: goreleaser/goreleaser-action@a4f02ab5355a589de60d917a659ce617ed19b3de # v7.3.0
        with:
          version: v2.18.3
`

func toolNamed(t *testing.T, name string) tool {
	t.Helper()
	for _, x := range tools() {
		if x.name == name {
			return x
		}
	}
	t.Fatalf("no tool %s", name)
	return tool{}
}

func TestPinnedVersions(t *testing.T) {
	texts := []string{ciFixture, releaseFixture}
	for name, want := range map[string]string{
		"staticcheck": "2026.2.2",
		"govulncheck": "1.9.0",
		"syft":        "1.54.1,1.55.0", // 2 つのワークフローで違えば両方
		"GoReleaser":  "2.18.3",        // ほかのステップの version: は読まない
	} {
		got, err := pinnedVersions(toolNamed(t, name), texts)
		if err != nil || strings.Join(got, ",") != want {
			t.Errorf("%s = %v, %v; want %s", name, got, err, want)
		}
	}
	// ワークフローの文字も、版の形でなければ受け取らない。見つからなければ失敗する
	for name, text := range map[string]string{
		"staticcheck": "go run honnef.co/go/tools/cmd/staticcheck@latest ./...",
		"govulncheck": "go run golang.org/x/vuln/cmd/govulncheck@v1.9.0|x ./...",
		"GoReleaser":  "      - uses: goreleaser/goreleaser-action@abc\n        with:\n          version: '~> v2'\n",
		"syft":        "nothing here",
	} {
		if got, err := pinnedVersions(toolNamed(t, name), []string{text}); err == nil {
			t.Errorf("%s from %q = %v, want an error", name, text, got)
		}
	}
}

func TestLatestToolVersions(t *testing.T) {
	if v, err := proxyVersion([]byte(`{"Version":"v1.10.0","Time":"2026-11-17T20:25:28Z"}`)); err != nil || v != "1.10.0" {
		t.Errorf("proxy = %q, %v", v, err)
	}
	if v, err := githubTag([]byte(`{"tag_name":"2026.2.2"}`), yearly); err != nil || v != "2026.2.2" {
		t.Errorf("staticcheck = %q, %v", v, err)
	}
	if v, err := githubTag([]byte(`{"tag_name":"2026.1"}`), yearly); err != nil || v != "2026.1" {
		t.Errorf("staticcheck without a patch = %q, %v", v, err)
	}
	if v, err := githubTag([]byte(`{"tag_name":"v2.19.0"}`), semver); err != nil || v != "2.19.0" {
		t.Errorf("goreleaser = %q, %v", v, err)
	}
	// ネットワークからの文字は、版の形でなければ書かない
	for _, bad := range []string{`{"Version":"v1.10.0-pre.1"}`, `{"Version":"v1.0 | [x](https://e.x)"}`, `{"Version":""}`, `not json`} {
		if _, err := proxyVersion([]byte(bad)); err == nil {
			t.Errorf("proxy %s: want an error", bad)
		}
	}
	for _, bad := range []string{`{"tag_name":"v0.2.0"}`, `{"tag_name":"2026.2.2-rc1"}`, `{"tag_name":"1999.1"}`} {
		if _, err := githubTag([]byte(bad), yearly); err == nil {
			t.Errorf("staticcheck %s: want an error", bad)
		}
	}
}

func TestToolsTable(t *testing.T) {
	ts := []tool{toolNamed(t, "staticcheck"), toolNamed(t, "syft")}
	got := toolsTable(ts, [][]string{{"2026.2.2"}, {"1.54.1", "1.55.0"}}, []string{"2026.2.3", "1.55.0"})
	for _, want := range []string{
		"| Tool | Pinned | Latest | Source |",
		"| staticcheck | 2026.2.2 | 2026.2.3 | GitHub dominikh/go-tools (latest release) |",
		"| syft | 1.54.1, 1.55.0 | 1.55.0 | GitHub anchore/syft (latest release) |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table lacks %q:\n%s", want, got)
		}
	}
}

// 今のワークフローから、どの道具の版も読める。docs/upstream/agents.md に道具の表があれば、その Pinned はワークフローと同じ
// （版を手で上げたら、表の Pinned も直す。直さないと、週 1 回の実行が版の変化として issue を立てる）。
func TestPinnedInRepo(t *testing.T) {
	var texts []string
	for _, w := range workflows {
		b, err := os.ReadFile(filepath.Join("..", "..", w))
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, string(b))
	}
	doc, err := os.ReadFile(filepath.Join("..", "..", out))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range tools() {
		p, err := pinnedVersions(x, texts)
		if err != nil {
			t.Error(err)
			continue
		}
		prefix := "| " + x.name + " | "
		for _, line := range strings.Split(string(doc), "\n") {
			if strings.HasPrefix(line, prefix) && !strings.HasPrefix(line, prefix+strings.Join(p, ", ")+" | ") {
				t.Errorf("%s: %s says %q, the workflows pin %s", x.name, out, line, strings.Join(p, ", "))
			}
		}
	}
}
