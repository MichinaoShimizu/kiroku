package report

import (
	"sort"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Share は期間の中で 1 つのくくり（ブランチ・エージェント）に使った量。
// 画面の「配分」の帯で、作業時間・トークン・目安コスト・クレジットの割合を見比べるのに使う。
type Share struct {
	Key     string  `json:"key"`
	Minutes float64 `json:"minutes"` // 作業していた時間（同時に動いていた別のくくりとは按分）
	Tokens  float64 `json:"tokens"`
	Cost    float64 `json:"cost"`
	Credits float64 `json:"credits"`
}

// ShareKeys は画面の色分けと同じくくり方。プロジェクトは ProjectStats を使う。
var ShareKeys = map[string]func(*core.Session) string{
	"branch": func(s *core.Session) string {
		b := "—"
		if s.Branch != nil && *s.Branch != "" {
			b = *s.Branch
		}
		return s.Project + " · " + b
	},
	"source": func(s *core.Session) string { return s.Source },
}

// shares はくくりごとの使った量を、作業時間の多い順に返す。mins は Summarize の分ごとの動いていたセッション。
func shares(data []*core.Session, ws, we float64, mins [][]sp, key func(*core.Session) string) []Share {
	by := map[string]*Share{}
	get := func(k string) *Share {
		if by[k] == nil {
			by[k] = &Share{Key: k}
		}
		return by[k]
	}
	keyOf := map[string]string{}
	for _, d := range data {
		if d.End < ws || d.Start >= we {
			continue
		}
		k := key(d)
		keyOf[d.ID] = k
		s := get(k)
		in := func(t *float64) bool {
			v := d.Start
			if t != nil && *t != 0 {
				v = *t
			}
			return ws <= v && v < we
		}
		for _, e := range d.UEv {
			if in(e.T) {
				s.Tokens += e.U.Total()
				if e.Cost != nil {
					s.Cost += *e.Cost
				}
			}
		}
		for _, c := range d.CEv {
			if in(c.T) {
				s.Credits += c.V
			}
		}
	}
	for _, m := range mins {
		seen := map[string]bool{}
		var ks []string
		for _, x := range m {
			if k, ok := keyOf[x.sid]; ok && !seen[k] {
				seen[k] = true
				ks = append(ks, k)
			}
		}
		for _, k := range ks {
			get(k).Minutes += 1 / float64(len(ks))
		}
	}
	out := make([]Share, 0, len(by))
	for _, s := range by {
		s.Minutes, s.Tokens, s.Cost, s.Credits = core.Round(s.Minutes, 0), core.Round(s.Tokens, 0), core.Round(s.Cost, 4), core.Round(s.Credits, 4)
		out = append(out, *s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Minutes != out[j].Minutes {
			return out[i].Minutes > out[j].Minutes
		}
		return out[i].Key < out[j].Key
	})
	return out
}
