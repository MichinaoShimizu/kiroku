package core

import (
	"regexp"
	"strings"
)

// Prices は USD / 100 万トークン: 入力, 出力, キャッシュ書き込み(5分), キャッシュ書き込み(1時間), キャッシュ読み込み。
// 出典: https://platform.claude.com/docs/en/about-claude/pricing （2026-10 時点）。--prices で上書きできる。
// モデル ID の先頭一致で引く（長いキーが優先）。サブスクリプションの請求額とは別物。
// PricesAsOf は収録した料金の時点。画面にも出す。料金表を更新したら合わせて変える。
const PricesAsOf = "2026-10"

var Prices = map[string][5]float64{
	"claude-fable-5-1":  {10, 50, 12.5, 20, 0.25},
	"claude-mythos-5-1": {10, 50, 12.5, 20, 0.25},
	"claude-fable-5":    {10, 50, 12.5, 20, 1.0},
	"claude-mythos-5":   {10, 50, 12.5, 20, 1.0},
	"claude-opus-5-5":   {4, 20, 5, 8, 0.20},
	"claude-opus-5":     {5, 25, 6.25, 10, 0.50},
	"claude-opus-4-8":   {5, 25, 6.25, 10, 0.50},
	"claude-opus-4-7":   {5, 25, 6.25, 10, 0.50},
	"claude-opus-4-6":   {5, 25, 6.25, 10, 0.50},
	"claude-opus-4-5":   {5, 25, 6.25, 10, 0.50},
	"claude-opus-4-1":   {15, 75, 18.75, 30, 1.50},
	"claude-opus-4":     {15, 75, 18.75, 30, 1.50},
	"claude-sonnet-5-5": {2, 10, 2.5, 4, 0.20},
	"claude-sonnet-5":   {2, 10, 2.5, 4, 0.20},
	"claude-sonnet-4-6": {3, 15, 3.75, 6, 0.30},
	"claude-sonnet-4-5": {3, 15, 3.75, 6, 0.30},
	"claude-sonnet-4":   {3, 15, 3.75, 6, 0.30},
	"claude-haiku-4-5":  {1, 5, 1.25, 2, 0.10},
	"claude-3-5-haiku":  {0.8, 4, 1, 1.6, 0.08},
}

// Tokens は 1 つの応答のトークン。
type Tokens struct {
	In   float64 `json:"in"`
	Out  float64 `json:"out"`
	CW   float64 `json:"cw"`
	CW1h float64 `json:"cw1h"`
	CR   float64 `json:"cr"`
}

func (t Tokens) Total() float64 { return t.In + t.Out + t.CW + t.CW1h + t.CR }

func (t Tokens) vals() [5]float64 { return [5]float64{t.In, t.Out, t.CW, t.CW1h, t.CR} }

var dateSuffix = regexp.MustCompile(`-\d{8}$`)

// ModelName は claude-haiku-4-5-20251001 → claude-haiku-4-5（日付の版を落としてまとめる）。
func ModelName(m string) string { return dateSuffix.ReplaceAllString(m, "") }

func PriceOf(model string) ([5]float64, bool) {
	m := strings.ReplaceAll(strings.ToLower(model), "anthropic.", "")
	best := ""
	for k := range Prices {
		if strings.HasPrefix(m, k) && len(k) > len(best) {
			best = k
		}
	}
	if best == "" {
		return [5]float64{}, false
	}
	return Prices[best], true
}

// CostOf は API 換算の目安コスト。料金表にないモデルは ok=false。
func CostOf(model string, u Tokens) (float64, bool) {
	p, ok := PriceOf(model)
	if !ok {
		return 0, false
	}
	v := u.vals()
	c := 0.0
	for i := range v {
		c += v[i] * p[i]
	}
	return c / 1e6, true
}

// ReadUsage は message.usage → Tokens。キャッシュ書き込みの内訳（cache_creation の 5 分・1 時間）があれば分ける。
// 古い Claude Code は cache_creation_input_tokens を 0 にしたまま内訳だけを書くことがあるので、
// 書き込みの合計は cache_creation_input_tokens と内訳の和の大きいほうにする。
func ReadUsage(raw any) Tokens {
	m := Map(raw)
	cw := NumOr0(m["cache_creation_input_tokens"])
	cc := Map(m["cache_creation"])
	cw1h := NumOr0(cc["ephemeral_1h_input_tokens"])
	if v, ok := cc["ephemeral_5m_input_tokens"]; ok && v != nil {
		cw = max(cw, NumOr0(v)+cw1h)
	}
	cw = max(cw, cw1h)
	return Tokens{In: NumOr0(m["input_tokens"]), Out: NumOr0(m["output_tokens"]), CW: cw - cw1h, CW1h: cw1h, CR: NumOr0(m["cache_read_input_tokens"])}
}

// Event は 1 つの応答の使用量。Cost が nil なら料金表にないモデル。
type Event struct {
	T     *float64
	Model string
	U     Tokens
	Cost  *float64
	Mult  float64 // 料金表の値に掛ける倍率（fast モード・US 内だけの推論）。0 は 1 と同じ
}

// rateMult は、応答の usage に記録された料金の倍率。
// fast モード（usage.speed が "fast"）は通常の 2 倍、US 内だけの推論（usage.inference_geo が "us"）は 1.1 倍で、重ねて掛かる。
// 出典: https://platform.claude.com/docs/en/about-claude/pricing （Fast mode pricing・Data residency pricing）
func rateMult(raw any) float64 {
	m, k := Map(raw), 1.0
	if strings.EqualFold(Str(m["speed"]), "fast") {
		k *= 2
	}
	if strings.EqualFold(Str(m["inference_geo"]), "us") {
		k *= 1.1
	}
	return k
}

// Usage は、1 つの応答が複数行に分かれて記録されるので、メッセージ ID ごとに項目別の最大値をとってから足す。
type Usage struct {
	order []string
	byMsg map[string]*Event
}

func NewUsage() *Usage { return &Usage{byMsg: map[string]*Event{}} }

func (u *Usage) Add(mid string, t *float64, model string, raw any) {
	tok, model := ReadUsage(raw), ModelName(model)
	if mid == "" {
		mid = "_" + itoa(len(u.byMsg))
	}
	cur, ok := u.byMsg[mid]
	if !ok {
		u.byMsg[mid] = &Event{T: t, Model: model, U: tok, Mult: rateMult(raw)}
		u.order = append(u.order, mid)
		return
	}
	cur.Mult = max(cur.Mult, rateMult(raw))
	if cur.T == nil || *cur.T == 0 {
		cur.T = t
	}
	if cur.Model == "" {
		cur.Model = model
	}
	cur.U = Tokens{max(cur.U.In, tok.In), max(cur.U.Out, tok.Out), max(cur.U.CW, tok.CW), max(cur.U.CW1h, tok.CW1h), max(cur.U.CR, tok.CR)}
}

func (u *Usage) Len() int { return len(u.byMsg) }

// Events は [(t, model, usage, cost)]。
func (u *Usage) Events() []Event {
	var out []Event
	for _, id := range u.order {
		e := *u.byMsg[id]
		if e.Model == "<synthetic>" {
			continue
		}
		if c, ok := CostOf(e.Model, e.U); ok {
			if e.Mult > 0 {
				c *= e.Mult
			}
			e.Cost = &c
		}
		out = append(out, e)
	}
	return out
}

// UsageTotal は使用量の合計（HTML と JSON に出す形）。
type UsageTotal struct {
	Tokens
	Cost           float64 `json:"cost"`
	Unpriced       float64 `json:"unpriced"`
	ReportedTokens float64 `json:"reportedTokens,omitempty"`
}

func SumUsage(evs []Event) UsageTotal {
	var t UsageTotal
	cost := 0.0
	for _, e := range evs {
		t.In += e.U.In
		t.Out += e.U.Out
		t.CW += e.U.CW
		t.CW1h += e.U.CW1h
		t.CR += e.U.CR
		if e.Cost == nil {
			t.Unpriced += e.U.Total()
		} else {
			cost += *e.Cost
		}
	}
	t.Cost = Round(cost, 4)
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
