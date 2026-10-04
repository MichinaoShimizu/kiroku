package core

// ReportedCost は、エージェント自身が記録した使用料（Claude Code の cost-state）。
// Claude Code はプロセスごとに、起動からの累計をモデル別に書く。料金表は Claude Code 本体と一緒に更新されるので、
// kiroku の料金表より新しい料金やモデルに追従でき、履歴に残らない呼び出し（自動要約など）も含む。
type ReportedCost struct {
	From, To float64 // 対象の期間（プロセスの起動〜最後の記録の時刻、UNIX 秒）
	Models   map[string]ReportedModel
}

// ReportedModel は 1 つのモデルの累計。
type ReportedModel struct {
	Cost float64
	U    Tokens
}

// applyReported は、期間に入る応答の目安コストを、エージェントが記録した使用料に合わせる。
// モデルごとに、kiroku の料金表での見積もりの比で配る（見積もれないモデルはトークンの比）。
// 履歴に応答が 1 つもないモデルの分は、期間の終わりの 1 件として足して返す。
func applyReported(rs []ReportedCost, groups ...[]Event) (extra []Event) {
	for _, r := range rs {
		for model, rm := range r.Models {
			type ref struct{ g, i int }
			var refs []ref
			est, tok := 0.0, 0.0
			for g, evs := range groups {
				for i, e := range evs {
					if e.Model != model || e.T == nil || *e.T < r.From || *e.T > r.To {
						continue
					}
					refs = append(refs, ref{g, i})
					tok += e.U.Total()
					if e.Cost != nil {
						est += *e.Cost
					}
				}
			}
			if len(refs) == 0 || (est <= 0 && tok <= 0) {
				if rm.Cost > 0 || rm.U.Total() > 0 {
					t, c := r.To, rm.Cost
					extra = append(extra, Event{T: &t, Model: model, U: rm.U, Cost: &c})
				}
				continue
			}
			for _, x := range refs {
				e := &groups[x.g][x.i]
				var v float64
				if est > 0 {
					v = rm.Cost * NumOr0Ptr(e.Cost) / est
				} else {
					v = rm.Cost * e.U.Total() / tok
				}
				e.Cost = &v
			}
		}
	}
	return extra
}

// NumOr0Ptr は nil なら 0。
func NumOr0Ptr(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
