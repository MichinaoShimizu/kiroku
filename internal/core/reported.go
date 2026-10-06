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
//
// 使った分があるのに記録が 0 のモデルは、合わせずに kiroku の見積もりをそのまま残す。
// サブスクリプションで使っているとき、Claude Code はトークンを使っても costUSD に 0 を書く（API の請求がないため）。
// そのまま合わせると、何十万トークン使っても目安コストが $0.00 になってしまう。
//
// used は、記録した使用料を実際に使ったかどうか（画面でコストの出どころを言い分けるために返す）。
func applyReported(rs []ReportedCost, groups ...[]Event) (extra []Event, used bool) {
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
				if e, ok := reportedEvent(r.To, model, rm); ok {
					extra = append(extra, e)
					used = used || rm.Cost > 0
				}
				continue
			}
			if rm.Cost <= 0 { // 記録が 0 のモデル（サブスクリプションなど）は見積もりのまま
				continue
			}
			used = true
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
	return extra, used
}

// reportedEvent は、履歴に応答が 1 つもないモデルの分を 1 件の応答にする。
// 記録が 0（サブスクリプション）なら、料金表で見積もる。どちらもなければ足さない。
func reportedEvent(t float64, model string, rm ReportedModel) (Event, bool) {
	if rm.Cost <= 0 && rm.U.Total() <= 0 {
		return Event{}, false
	}
	e := Event{T: &t, Model: model, U: rm.U}
	if rm.Cost > 0 {
		c := rm.Cost
		e.Cost = &c
	} else if c, ok := CostOf(model, rm.U); ok {
		e.Cost = &c
	}
	return e, true
}

// NumOr0Ptr は nil なら 0。
func NumOr0Ptr(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
