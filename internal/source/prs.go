package source

import "github.com/MichinaoShimizu/kiroku/internal/core"

// maxPRCalls は、結果を待っている PR の呼び出しを 1 つの会話で覚えておく数の上限（壊れた・細工した履歴でメモリを使い切らない）。
const maxPRCalls = 10000

// prCalls は、PR の操作（作成・マージ・レビュー。core.PRActions）をするツールの呼び出しを ID ごとに覚え、
// 結果が成功だったときだけセッションのアウトプットにする。Claude Code の outputs（claude.go）と同じ考えで、Codex と Kiro に使う。
// 結果が履歴にない呼び出しは数えない（成功したかわからないため）。
// 同じ PR の URL は 1 回だけ数える（細工した履歴で同じ call_id を使い直すなど）。
type prCalls struct {
	calls map[string]prCall
	urls  map[string]bool
}

func newPRCalls() *prCalls { return &prCalls{calls: map[string]prCall{}, urls: map[string]bool{}} }

type prCall struct {
	t    *float64
	acts []core.PRAction
}

func (p *prCalls) call(id, name string, input any, t *float64) {
	if id == "" || len(p.calls) >= maxPRCalls {
		return
	}
	if acts := core.PRActions(name, input); len(acts) > 0 {
		p.calls[id] = prCall{t, acts}
	}
}

// waiting は、id が結果を待っている PR の呼び出しか。結果の文を読む（JSON を解く）前に確かめ、
// PR の操作でない結果まで読まないようにする。
func (p *prCalls) waiting(id string) bool {
	_, ok := p.calls[id]
	return ok
}

// result は呼び出し id の結果を受け取る。ok が false（失敗）なら数えない。作った PR の URL は結果の文から探す。
// 時刻は結果の時刻、なければ呼び出しの時刻にする。
func (p *prCalls) result(s *core.Builder, id string, ok bool, text string, t *float64) {
	c, found := p.calls[id]
	if !found {
		return
	}
	delete(p.calls, id)
	if !ok {
		return
	}
	if t == nil {
		t = c.t
	}
	for _, a := range c.acts {
		o := core.Output{T: t, Kind: a.Kind, V: 1, Ref: a.Ref}
		if a.Kind == "pr" {
			if o.URL = core.PRURL(text); o.URL != "" {
				if p.urls[o.URL] || len(p.urls) >= maxPRCalls {
					continue // 同じ PR をもう数えた
				}
				p.urls[o.URL] = true
			}
		}
		s.Outputs = append(s.Outputs, o)
	}
}
