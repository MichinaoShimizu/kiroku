package core

import (
	"regexp"
	"strings"
)

// Prices は Anthropic のモデルの料金。USD / 100 万トークン: 入力, 出力, キャッシュ書き込み(5分), キャッシュ書き込み(1時間), キャッシュ読み込み。
// 出典: https://platform.claude.com/docs/en/about-claude/pricing （2026-10 時点）。--prices で上書きできる。
// 公式の数字の控えは docs/upstream/anthropic-pricing.md（tools/prices が作る）。TestPricesMatchUpstream が食い違いを見つける。
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

// OpenAIPrices は OpenAI のモデル（Codex が使う gpt-5 以降）の料金。並びは Prices と同じで、
// 入力（キャッシュなし）, 出力, キャッシュ書き込み, キャッシュ書き込み（同じ値。OpenAI に 1 時間の区別はない）, キャッシュ済み入力。
// 出典: https://developers.openai.com/api/docs/pricing の Standard（短いコンテキスト）と Specialized models の Codex（2026-10 時点）。
// 公式にキャッシュ書き込み・キャッシュ済み入力の料金がない（"-"）モデルは、その分も入力の料金にする。
// 公式の数字の控えは docs/upstream/openai-pricing.md。Fast・Batch・Flex・データ所在地の上乗せは入れていない。
var OpenAIPrices = map[string][5]float64{
	"gpt-6-astra":   {10, 50, 12.5, 12.5, 1},
	"gpt-6.1-sol":   {2, 10, 2.5, 2.5, 0.1},
	"gpt-6-luna":    {0.1, 0.5, 0.125, 0.125, 0.01},
	"gpt-6-sol":     {2, 10, 2.5, 2.5, 0.2},
	"gpt-5.6-sol":   {4, 20, 5, 5, 0.4},
	"gpt-5.6-terra": {2, 12, 2.5, 2.5, 0.2},
	"gpt-5.6-luna":  {0.2, 1.2, 0.25, 0.25, 0.02},
	"gpt-5.5":       {5, 30, 5, 5, 0.5},
	"gpt-5.5-pro":   {30, 180, 30, 30, 30},
	"gpt-5.4":       {2.5, 15, 2.5, 2.5, 0.25},
	"gpt-5.4-mini":  {0.75, 4.5, 0.75, 0.75, 0.075},
	"gpt-5.4-nano":  {0.2, 1.25, 0.2, 0.2, 0.02},
	"gpt-5.4-pro":   {30, 180, 30, 30, 30},
	"gpt-5.3-codex": {1.75, 14, 1.75, 1.75, 0.175},
	"gpt-5.2":       {1.75, 14, 1.75, 1.75, 0.175},
	"gpt-5.2-pro":   {21, 168, 21, 21, 21},
	"gpt-5.1":       {1.25, 10, 1.25, 1.25, 0.125},
	"gpt-5":         {1.25, 10, 1.25, 1.25, 0.125},
	"gpt-5-mini":    {0.25, 2, 0.25, 0.25, 0.025},
	"gpt-5-nano":    {0.05, 0.4, 0.05, 0.05, 0.005},
	"gpt-5-pro":     {15, 120, 15, 15, 15},
}

// OpenAILongPrices は、1 回の応答の入力が OpenAILongContext を超えたときの料金（その応答の全部にかかる）。並びは OpenAIPrices と同じ。
var OpenAILongPrices = map[string][5]float64{
	"gpt-6-astra":   {20, 75, 25, 25, 2},
	"gpt-6.1-sol":   {4, 15, 5, 5, 0.2},
	"gpt-6-luna":    {0.2, 0.75, 0.25, 0.25, 0.02},
	"gpt-6-sol":     {4, 15, 5, 5, 0.4},
	"gpt-5.6-sol":   {8, 30, 10, 10, 0.8},
	"gpt-5.6-terra": {4, 18, 5, 5, 0.4},
	"gpt-5.6-luna":  {0.4, 1.8, 0.5, 0.5, 0.04},
	"gpt-5.5":       {10, 45, 10, 10, 1},
	"gpt-5.5-pro":   {60, 270, 60, 60, 60},
	"gpt-5.4":       {5, 22.5, 5, 5, 0.5},
	"gpt-5.4-pro":   {60, 270, 60, 60, 60},
}

// OpenAILongContext は OpenAI の長いコンテキストの境目（入力トークン。これを超えると OpenAILongPrices）。
const OpenAILongContext = 272000

// Tokens は 1 つの応答のトークン。
type Tokens struct {
	In   float64 `json:"in"`
	Out  float64 `json:"out"`
	CW   float64 `json:"cw"`
	CW1h float64 `json:"cw1h"`
	CR   float64 `json:"cr"`
}

func (t Tokens) Total() float64 { return t.In + t.Out + t.CW + t.CW1h + t.CR }

// Input は 1 つの応答の入力（キャッシュの読み書きを含む）。
func (t Tokens) Input() float64 { return t.In + t.CW + t.CW1h + t.CR }

func (t Tokens) vals() [5]float64 { return [5]float64{t.In, t.Out, t.CW, t.CW1h, t.CR} }

var dateSuffix = regexp.MustCompile(`-\d{8}$`)

// ModelName は claude-haiku-4-5-20251001 → claude-haiku-4-5（日付の版を落としてまとめる）。
func ModelName(m string) string { return dateSuffix.ReplaceAllString(m, "") }

// Price は料金表で引いた結果。
type Price struct {
	Rates [5]float64  // 並びは Prices と同じ
	Long  *[5]float64 // 長いコンテキストの料金（OpenAI。なければ nil）
	Key   string      // 当たった料金表のキー
	Exact bool        // 版の印（日付・-v1:0 など）を除くとキーと同じ。false は先頭一致だけで当てたもの（新しいモデルに古い料金を当てているかもしれない）
}

var (
	// bedrockPrefix は Bedrock の anthropic. と、その前の地域（us. eu. apac. global. など）。
	bedrockPrefix = regexp.MustCompile(`^(?:[a-z]+(?:-[a-z]+)*\.)?anthropic\.`)
	// versionSuffix は同じモデルの版の印: 日付（-20251001、Vertex の @20251001、OpenAI の -2025-08-07）、Bedrock の -v1:0、-latest、[1m]。
	versionSuffix = regexp.MustCompile(`(?:[-@]\d{8}|-\d{4}-\d{2}-\d{2}|-v\d+(?::\d+)?|-latest|\[[0-9a-z]+\])$`)
)

// priceID は料金表を引くためのモデル ID（小文字、Bedrock の地域と anthropic. を除き、Claude の 4.5 の形は 4-5 にする）。
func priceID(model string) string {
	m := bedrockPrefix.ReplaceAllString(strings.ToLower(strings.TrimSpace(model)), "")
	if strings.HasPrefix(m, "claude-") {
		m = strings.ReplaceAll(m, ".", "-")
	}
	for {
		s := versionSuffix.ReplaceAllString(m, "")
		if s == m {
			return m
		}
		m = s
	}
}

// prefixOf は key が m の先頭に区切りよく一致するか（gpt-5 は gpt-5-codex に当たるが、gpt-5.7 や gpt-50 には当てない）。
func prefixOf(m, key string) bool {
	if !strings.HasPrefix(m, key) {
		return false
	}
	if len(m) == len(key) {
		return true
	}
	c := m[len(key)]
	return !(c >= '0' && c <= '9' || c == '.')
}

// PriceOf はモデルの料金。版の印を除いた ID と同じキーがあればそれ（Exact）、なければ先頭に区切りよく一致する最も長いキー。
// --prices で足した Prices のキーは、同じ名前の OpenAIPrices より優先する。
func PriceOf(model string) (Price, bool) {
	m := priceID(model)
	if m == "" {
		return Price{}, false
	}
	best, fromOpenAI := "", false
	for k := range Prices {
		if prefixOf(m, k) && len(k) > len(best) {
			best = k
		}
	}
	for k := range OpenAIPrices {
		if _, own := Prices[k]; !own && prefixOf(m, k) && len(k) > len(best) {
			best, fromOpenAI = k, true
		}
	}
	if best == "" {
		return Price{}, false
	}
	p := Price{Key: best, Exact: m == best}
	if fromOpenAI {
		p.Rates = OpenAIPrices[best]
		if l, ok := OpenAILongPrices[best]; ok {
			p.Long = &l
		}
	} else {
		p.Rates = Prices[best]
	}
	return p, true
}

// CostOf は API 換算の目安コスト。料金表にないモデルは ok=false。
// 長いコンテキストの料金は使わない（応答 1 回分かどうか分からないとき用。CostOfRequest を参照）。
func CostOf(model string, u Tokens) (float64, bool) {
	p, ok := PriceOf(model)
	if !ok {
		return 0, false
	}
	return costAt(p.Rates, u), true
}

// CostOfRequest は、u が応答 1 回分と分かっているときの目安コスト。OpenAI のモデルで入力が OpenAILongContext を超えたら、
// その応答の全部を長いコンテキストの料金にする。
func CostOfRequest(model string, u Tokens) (float64, bool) {
	p, ok := PriceOf(model)
	if !ok {
		return 0, false
	}
	if p.Long != nil && u.Input() > OpenAILongContext {
		return costAt(*p.Long, u), true
	}
	return costAt(p.Rates, u), true
}

func costAt(p [5]float64, u Tokens) float64 {
	v := u.vals()
	c := 0.0
	for i := range v {
		c += v[i] * p[i]
	}
	return c / 1e6
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
	Req   bool    // U が応答 1 回分（入力の量で長いコンテキストの料金を選べる）。false は何回分かの合計かもしれない
}

// EventCost は e の目安コスト（料金表にないモデルは ok=false）。倍率は掛けない。
func EventCost(e Event) (float64, bool) {
	if e.Req {
		return CostOfRequest(e.Model, e.U)
	}
	return CostOf(e.Model, e.U)
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
		if c, ok := EventCost(e); ok {
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
