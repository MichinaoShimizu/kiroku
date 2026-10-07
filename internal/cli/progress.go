package cli

import (
	"encoding/json"
	"sync"
	"time"
)

// loadProgress は kiroku serve の最初の読み込みの進み具合（読み込み中の画面が /progress で見る）。
// 見せるのはエージェントの名前・セッションの数・かかった時間と、いまの段階だけ（履歴の中身は入れない）。
type loadProgress struct {
	mu     sync.Mutex
	start  time.Time
	stage  string // history（履歴を読んでいる）→ git → view（集計して画面を作っている）
	agents []agentProgress
}

type agentProgress struct {
	Name string `json:"name"`
	N    int    `json:"n"`
	Ms   int64  `json:"ms"`
	Done bool   `json:"done"`
}

// tracker は、いま進み具合を記録している先（なければ nil）。live.start が最初の読み込みのあいだだけ置く。
// エージェントは並べて読むので、メソッドは同時に呼んでよい。nil なら何もしない。
var tracker *loadProgress

func newProgress(names []string) *loadProgress {
	p := &loadProgress{start: time.Now(), stage: "history"}
	for _, n := range names {
		p.agents = append(p.agents, agentProgress{Name: n})
	}
	return p
}

func (p *loadProgress) agentDone(name string, n int, took time.Duration) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.agents {
		if p.agents[i].Name == name && !p.agents[i].Done {
			p.agents[i] = agentProgress{Name: name, N: n, Ms: took.Milliseconds(), Done: true}
			return
		}
	}
}

func (p *loadProgress) setStage(s string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.stage = s
	p.mu.Unlock()
}

// json は /progress の本文。ready は最初の読み込みが終わったか、failed は失敗したか。
func (p *loadProgress) json(ready, failed bool) []byte {
	out := map[string]any{"ready": ready, "failed": failed, "stage": "", "agents": []agentProgress{}, "ms": 0}
	if p != nil {
		p.mu.Lock()
		out["stage"], out["agents"], out["ms"] = p.stage, append([]agentProgress(nil), p.agents...), time.Since(p.start).Milliseconds()
		p.mu.Unlock()
	}
	b, _ := json.Marshal(out)
	return b
}
