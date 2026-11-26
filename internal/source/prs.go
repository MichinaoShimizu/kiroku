package source

import "github.com/MichinaoShimizu/kiroku/internal/core"

// maxPRCalls は、結果を待っている PR の呼び出しを 1 つの会話で覚えておく数の上限（壊れた・細工した履歴でメモリを使い切らない）。
const maxPRCalls = 10000

// prCalls は、PR を作るツールの呼び出し（core.CreatesPR）を ID ごとに覚え、結果が成功だったときだけ
// セッションのアウトプットにする。Claude Code の outputs（claude.go）と同じ考えで、Codex と Kiro に使う。
// 結果が履歴にない呼び出しは数えない（成功したかわからないため）。
type prCalls map[string]*float64

func (p prCalls) call(id, name string, input any, t *float64) {
	if id == "" || len(p) >= maxPRCalls || !core.CreatesPR(name, input) {
		return
	}
	p[id] = t
}

// result は呼び出し id の結果を受け取る。ok が false（失敗）なら数えない。PR の URL は結果の文から探す。
// 時刻は結果の時刻、なければ呼び出しの時刻にする。
func (p prCalls) result(s *core.Builder, id string, ok bool, text string, t *float64) {
	at, found := p[id]
	if !found {
		return
	}
	delete(p, id)
	if !ok {
		return
	}
	if t == nil {
		t = at
	}
	s.Outputs = append(s.Outputs, core.Output{T: t, Kind: "pr", V: 1, URL: core.PRURL(text)})
}
