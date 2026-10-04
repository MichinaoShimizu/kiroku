package report

import (
	"math"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// span は、セッションの記録（区間・依頼・使用量・成果など）が散らばっている時刻の幅。
// 期間の集計は、この幅が期間と重なるセッションだけを見れば足りる。
type span struct{ lo, hi float64 }

func spanOf(d *core.Session) span {
	s := span{d.Start, d.End}
	add := func(t float64) {
		if t != 0 {
			s.lo, s.hi = math.Min(s.lo, t), math.Max(s.hi, t)
		}
	}
	addP := func(t *float64) {
		if t != nil {
			add(*t)
		}
	}
	for _, sg := range d.Segs {
		add(sg[0])
		add(sg[1])
	}
	for _, p := range d.Prompts {
		addP(p.T)
	}
	for _, w := range d.Waits {
		add(w[0])
	}
	for _, t := range d.Fix {
		add(t)
	}
	for _, e := range d.UEv {
		addP(e.T)
	}
	for _, c := range d.CEv {
		addP(c.T)
	}
	for _, o := range d.OEv {
		addP(o.T)
	}
	for _, m := range d.Meas {
		addP(m.T)
	}
	for _, a := range d.Subagents {
		addP(a.Start)
		addP(a.End)
	}
	return s
}

// periods は、期間ごとに関わるセッションを選ぶ。週や月を全部集計するとき、毎回すべてのセッションを
// なめると、履歴が長いほど（期間の数 × セッションの数で）重くなるため。
type periods struct {
	data  []*core.Session
	spans []span
}

func newPeriods(data []*core.Session) *periods {
	p := &periods{data: data, spans: make([]span, len(data))}
	for i, d := range data {
		p.spans[i] = spanOf(d)
	}
	return p
}

// within は [ws, we) に記録のあるセッション。並びは元のまま（集計の「出てきた順」を変えない）。
func (p *periods) within(ws, we float64) []*core.Session {
	var out []*core.Session
	for i, s := range p.spans {
		if s.hi >= ws && s.lo < we {
			out = append(out, p.data[i])
		}
	}
	return out
}
