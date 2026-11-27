package cli

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// kiroku serve --prs：自分に関係する、開いているプルリクエストを GitHub に問い合わせて画面に出す。
// 既定ではオフ（付けなければ、kiroku は GitHub に何も問い合わせない）。
//   - 問い合わせは GitHub CLI（gh）に任せる。kiroku はトークンを読まず、持たず、渡さない。
//   - gh に渡すのは下の決まった問い合わせ（prsQuery）だけ。履歴から来た文字は 1 文字も入れない
//     （「自分」は @me と書き、だれなのかは gh のログインが決める）。
//   - 返ってきたもの（題名・名前・URL）は、ほかの人が書ける文字。形を確かめ、長さを切り、画面ではエスケープして出す。
//   - 問い合わせるのは、画面で一覧を開いたときと「更新」を押したときだけ。裏で定期的には問い合わせない。
//   - 結果はメモリに持つだけで、ファイルには書かない。

// prsQuery は gh api graphql に渡す問い合わせ。決まった文字だけで、実行時に何も足さない。
const prsQuery = `query {
  viewer { login }
  author: search(query: "is:pr is:open archived:false author:@me sort:updated-desc", type: ISSUE, first: 50) { ...prs }
  review: search(query: "is:pr is:open archived:false review-requested:@me sort:updated-desc", type: ISSUE, first: 50) { ...prs }
  mention: search(query: "is:pr is:open archived:false mentions:@me sort:updated-desc", type: ISSUE, first: 50) { ...prs }
}
fragment prs on SearchResultItemConnection {
  issueCount
  nodes {
    ... on PullRequest {
      url
      title
      isDraft
      createdAt
      updatedAt
      reviewDecision
      mergeable
      author { login }
      commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
    }
  }
}`

// prsWhys は、一覧に出すわけ（問い合わせの名前と同じ）。この順に並べる。
var prsWhys = []string{"author", "review", "mention"}

const (
	prsTimeout   = 25 * time.Second
	prsMaxStdout = 4 << 20
	prsFresh     = 2 * time.Minute  // これより新しい結果があれば、一覧を開いても問い合わせ直さない
	prsMinGap    = 10 * time.Second // 「更新」を続けて押しても、これより短い間隔では問い合わせない
	prsTitleMax  = 300
)

// prItem は、画面に出す 1 つのプルリクエスト。どの値も、確かめた形のものだけ。
type prItem struct {
	URL      string   `json:"url"`    // https://HOST/OWNER/REPO/pull/N（形を確かめたもの）
	Repo     string   `json:"repo"`   // OWNER/REPO（URL から取る）
	Number   int      `json:"number"` // URL から取る
	Title    string   `json:"title"`
	Author   string   `json:"author,omitempty"`
	Draft    bool     `json:"draft,omitempty"`
	Created  float64  `json:"created,omitempty"` // UNIX 秒
	Updated  float64  `json:"updated,omitempty"`
	Review   string   `json:"review,omitempty"` // approved・changes・required（なければ空）
	Checks   string   `json:"checks,omitempty"` // passing・failing・pending（なければ空）
	Conflict bool     `json:"conflict,omitempty"`
	Why      []string `json:"why"` // author・review・mention
}

// prsResult は /prs の答え。Error は画面に出す短い種類だけ（くわしいわけは端末に出す）：
// nogh（gh がない）・auth（gh にログインしていない）・failed（そのほか）。
type prsResult struct {
	On      bool            `json:"on"`
	Fetched float64         `json:"fetched,omitempty"`
	Login   string          `json:"login,omitempty"`
	Items   []prItem        `json:"items"`
	More    map[string]bool `json:"more,omitempty"` // そのわけの PR が、取った数（50）より多い
	Partial bool            `json:"partial,omitempty"`
	Error   string          `json:"error,omitempty"`
}

var (
	prURLPattern   = regexp.MustCompile(`^https://([A-Za-z0-9][A-Za-z0-9.-]{0,252}(?::\d{1,5})?)/([A-Za-z0-9][A-Za-z0-9_.-]{0,99})/([A-Za-z0-9_.][A-Za-z0-9_.-]{0,99})/pull/([1-9]\d{0,8})$`)
	prLoginPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}(?:\[bot\])?$`)
)

// prsNode は gh の答えの 1 件（PullRequest でないものは、どの欄も空で来る）。
type prsNode struct {
	URL            string `json:"url"`
	Title          string `json:"title"`
	IsDraft        bool   `json:"isDraft"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	ReviewDecision string `json:"reviewDecision"`
	Mergeable      string `json:"mergeable"`
	Author         *struct {
		Login string `json:"login"`
	} `json:"author"`
	Commits *struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type prsList struct {
	IssueCount int        `json:"issueCount"`
	Nodes      []*prsNode `json:"nodes"`
}

// parsePRs は gh api graphql の答えを、画面に出す形にする。形の合わないものは捨てる。
// 同じ PR がいくつかのわけで来たら 1 つにまとめ、わけを並べる。data がなければ ok は false。
func parsePRs(out []byte) (res prsResult, ok bool) {
	var body struct {
		Data *struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
			Author  *prsList `json:"author"`
			Review  *prsList `json:"review"`
			Mention *prsList `json:"mention"`
		} `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(out, &body); err != nil || body.Data == nil {
		return res, false
	}
	res.On, res.Items, res.Partial = true, []prItem{}, len(body.Errors) > 0
	if prLoginPattern.MatchString(body.Data.Viewer.Login) {
		res.Login = body.Data.Viewer.Login
	}
	at := map[string]int{}
	for i, list := range []*prsList{body.Data.Author, body.Data.Review, body.Data.Mention} {
		why := prsWhys[i]
		if list == nil {
			res.Partial = true
			continue
		}
		if list.IssueCount > len(list.Nodes) {
			if res.More == nil {
				res.More = map[string]bool{}
			}
			res.More[why] = true
		}
		for _, n := range list.Nodes {
			if n == nil {
				continue
			}
			it, good := prFromNode(n)
			if !good {
				continue
			}
			if j, seen := at[it.URL]; seen {
				res.Items[j].Why = append(res.Items[j].Why, why)
				continue
			}
			it.Why = []string{why}
			at[it.URL] = len(res.Items)
			res.Items = append(res.Items, it)
		}
	}
	return res, true
}

// prFromNode は 1 件を確かめて prItem にする。URL が PR の形でなければ捨てる。
func prFromNode(n *prsNode) (prItem, bool) {
	m := prURLPattern.FindStringSubmatch(n.URL)
	if m == nil {
		return prItem{}, false
	}
	num := 0
	for _, c := range m[4] {
		num = num*10 + int(c-'0')
	}
	it := prItem{URL: n.URL, Repo: m[2] + "/" + m[3], Number: num, Title: prText(n.Title, prsTitleMax), Draft: n.IsDraft,
		Created: prTime(n.CreatedAt), Updated: prTime(n.UpdatedAt), Conflict: n.Mergeable == "CONFLICTING"}
	if n.Author != nil && prLoginPattern.MatchString(n.Author.Login) {
		it.Author = n.Author.Login
	}
	switch n.ReviewDecision {
	case "APPROVED":
		it.Review = "approved"
	case "CHANGES_REQUESTED":
		it.Review = "changes"
	case "REVIEW_REQUIRED":
		it.Review = "required"
	}
	if n.Commits != nil && len(n.Commits.Nodes) > 0 && n.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
		switch n.Commits.Nodes[0].Commit.StatusCheckRollup.State {
		case "SUCCESS":
			it.Checks = "passing"
		case "FAILURE", "ERROR":
			it.Checks = "failing"
		case "PENDING", "EXPECTED":
			it.Checks = "pending"
		}
	}
	return it, true
}

// prText は、GitHub から来た文字（ほかの人が書ける）から制御文字と書式文字を除き、n 文字で切る。
func prText(s string, n int) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s))
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func prTime(s string) float64 {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil || t.Year() < 2000 || t.Year() > 2200 {
		return 0
	}
	return float64(t.Unix())
}

// fetchPRs は gh に 1 回だけ問い合わせる。失敗のくわしいわけは err で返す（端末に出す用。gh の文は端末に出せる形にしてある）。
func fetchPRs(ctx context.Context) (prsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, prsTimeout)
	defer cancel()
	out, err := execGH(ctx, nil, prsMaxStdout, "api", "graphql", "-f", "query="+prsQuery)
	if res, ok := parsePRs(out); ok { // 一部が取れなかっただけなら（err があっても）、取れた分を出す
		if err != nil {
			res.Partial = true
		}
		return res, err
	}
	kind := "failed"
	var ge *ghError
	switch {
	case errors.Is(err, errGHNotFound):
		kind = "nogh"
	case errors.As(err, &ge) && (ge.code == 4 || strings.Contains(strings.ToLower(ge.detail), "gh auth login")):
		kind = "auth"
	case err == nil:
		err = errors.New("gh returned something that is not a list of pull requests")
	}
	return prsResult{On: true, Items: []prItem{}, Error: kind}, err
}

// prBoard は、最後に取った一覧（メモリの中だけ）。問い合わせは 1 本ずつ。
type prBoard struct {
	mu    sync.Mutex
	at    time.Time
	body  []byte
	fetch func(context.Context) (prsResult, error) // テストで差しかえる
	now   func() time.Time
	logf  func(format string, args ...any)
}

func newPRBoard(logf func(string, ...any)) *prBoard {
	return &prBoard{fetch: fetchPRs, now: time.Now, logf: logf}
}

// get は一覧の JSON を返す。force（「更新」）なら prsMinGap より古ければ、そうでなければ prsFresh より古ければ問い合わせる。
func (b *prBoard) get(ctx context.Context, force bool) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	age, limit := b.now().Sub(b.at), prsFresh
	if force {
		limit = prsMinGap
	}
	if b.body != nil && age < limit {
		return b.body
	}
	res, err := b.fetch(ctx)
	if err != nil && b.logf != nil {
		b.logf("could not list pull requests (--prs): %v\n", err)
	}
	b.at = b.now()
	res.Fetched = float64(b.at.UnixNano()) / 1e9
	js, mErr := json.Marshal(res)
	if mErr != nil {
		js = []byte(`{"on":true,"items":[],"error":"failed"}`)
	}
	b.body = js
	return js
}
