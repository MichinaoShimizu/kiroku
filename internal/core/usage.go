package core

import (
	"regexp"
	"strings"
	"sync"
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
	"claude-sonnet-5-5": {2, 10, 2.5, 4, 0.10},
	"claude-sonnet-5":   {2, 10, 2.5, 4, 0.20},
	"claude-sonnet-4-6": {3, 15, 3.75, 6, 0.30},
	"claude-sonnet-4-5": {3, 15, 3.75, 6, 0.30},
	"claude-sonnet-4":   {3, 15, 3.75, 6, 0.30},
	"claude-haiku-5-5":  {0.10, 0.50, 0.125, 0.20, 0.01}, // プロンプトが 100K 以下。超えたら LongPrices
	"claude-haiku-4-5":  {1, 5, 1.25, 2, 0.10},
	"claude-3-5-haiku":  {0.8, 4, 1, 1.6, 0.08},
}

// LongPrice は、1 回の応答のプロンプトが Over トークンを超えたときの料金（その応答の全部にかかる。出力も含む）。並びは Prices と同じ。
type LongPrice struct {
	Over  float64
	Rates [5]float64
}

// LongPrices は、プロンプトの長さで料金が変わる Anthropic のモデル（Prices の同じキーに足す）。出典は Prices と同じページ
// （Model pricing の "for prompts over 100,000 tokens" の行と Long context pricing）。
// ページのとおり、プロンプトの長さはキャッシュの読み書きも含む入力の全部なので、入力 + キャッシュの書き込み + 読み込み
// （Tokens.Input。モデルが読むプロンプト全体で、OpenAI の境目と同じ数え方）で比べる。
// --prices で同じキーを上書きすると、ここからは消す（足した料金をどの長さにも使う）。
var LongPrices = map[string]LongPrice{
	"claude-haiku-5-5": {Over: 100_000, Rates: [5]float64{0.50, 2.50, 0.625, 1, 0.05}},
}

// OpenAIPrices は OpenAI のモデル（Codex が使う gpt-5 以降）の料金。並びは Prices と同じで、
// 入力（キャッシュなし）, 出力, キャッシュ書き込み, キャッシュ書き込み（同じ値。OpenAI に 1 時間の区別はない）, キャッシュ済み入力。
// 出典: https://developers.openai.com/api/docs/pricing の Standard（短いコンテキスト）、Cyber models、Specialized models の Codex（2026-10 時点）。
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
	"gpt-5.6-cyber": {12.5, 75, 15.625, 15.625, 1.25}, // Cyber models の表（長いコンテキストの料金はない）
	"gpt-5.5-cyber": {12.5, 75, 12.5, 12.5, 1.25},
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

// WebSearchPrice は、ウェブ検索 1 回の料金（USD）。トークンの料金とは別に、1,000 回あたり $10 かかる。
// fast モードや US 内だけの推論の倍率は、トークンの料金にだけ掛かる。
// 出典: https://platform.claude.com/docs/en/about-claude/pricing （Web search tool。2026-10 時点）
const WebSearchPrice = 10.0 / 1000

// Tokens は 1 つの応答のトークン。WebSearches はトークンではなく、ウェブ検索の回数（usage.server_tool_use.web_search_requests）。
type Tokens struct {
	In          float64 `json:"in"`
	Out         float64 `json:"out"`
	CW          float64 `json:"cw"`
	CW1h        float64 `json:"cw1h"`
	CR          float64 `json:"cr"`
	WebSearches float64 `json:"webSearches,omitempty"`
}

// Total はトークンの合計（ウェブ検索の回数は入れない）。
func (t Tokens) Total() float64 { return t.In + t.Out + t.CW + t.CW1h + t.CR }

// Input は 1 つの応答の入力（キャッシュの読み書きを含む）。
func (t Tokens) Input() float64 { return t.In + t.CW + t.CW1h + t.CR }

func (t Tokens) vals() [5]float64 { return [5]float64{t.In, t.Out, t.CW, t.CW1h, t.CR} }

// ModelName は claude-haiku-4-5-20251001 → claude-haiku-4-5（日付の版を落としてまとめる）。
// 末尾の "-" と 8 桁の数字を落とす（イベントごとに呼ぶので、正規表現は使わない）。
func ModelName(m string) string {
	n := len(m)
	if n < 9 || m[n-9] != '-' {
		return m
	}
	for i := n - 8; i < n; i++ {
		if m[i] < '0' || m[i] > '9' {
			return m
		}
	}
	return m[:n-9]
}

// Price は料金表で引いた結果。
type Price struct {
	Rates [5]float64  // 並びは Prices と同じ
	Long  *[5]float64 // 長いコンテキストの料金（OpenAI と LongPrices。なければ nil）
	Over  float64     // 1 回の応答の入力（Tokens.Input）がこれを超えたら Long
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
// 履歴ではイベントごとに同じモデル ID を何度も引くので、正規表現にかけた結果を覚えておく（覚えるのは maxPriceIDs 個まで。
// 履歴に変わったモデル ID がたくさん書かれていても、メモリを使い続けないため）。
var priceIDs = struct {
	sync.RWMutex
	m map[string]string
}{m: map[string]string{}}

const maxPriceIDs = 1024

func priceID(model string) string {
	priceIDs.RLock()
	id, ok := priceIDs.m[model]
	priceIDs.RUnlock()
	if ok {
		return id
	}
	id = priceIDOf(model)
	priceIDs.Lock()
	if len(priceIDs.m) < maxPriceIDs {
		priceIDs.m[model] = id
	}
	priceIDs.Unlock()
	return id
}

func priceIDOf(model string) string {
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
			p.Long, p.Over = &l, OpenAILongContext
		}
	} else {
		p.Rates = Prices[best]
		if l, ok := LongPrices[best]; ok {
			p.Long, p.Over = &l.Rates, l.Over
		}
	}
	return p, true
}

// byModel は、料金ではない表（コンテキストの上限など）をモデル ID で引く。料金表と同じく、ID を priceID でそろえ
// （大文字・小文字、Bedrock の地域と anthropic.、版の印、4.8 の形）、先頭に区切りよく一致する最も長いキーを使う
// （claude-opus-4-10 に claude-opus-4-1 は当てない）。
func byModel[V any](table map[string]V, model string) (V, bool) {
	m := priceID(model)
	best := ""
	for k := range table {
		if prefixOf(m, k) && len(k) > len(best) {
			best = k
		}
	}
	if best == "" {
		var zero V
		return zero, false
	}
	return table[best], true
}

// CostOf は API 換算の目安コスト（トークンの料金とウェブ検索の料金の和）。料金表にないモデルは ok=false。
// 長いコンテキストの料金は使わない（応答 1 回分かどうか分からないとき用。CostOfRequest を参照）。
func CostOf(model string, u Tokens) (float64, bool) {
	c, ok := tokenCost(model, u)
	if !ok {
		return 0, false
	}
	return c + u.WebSearches*WebSearchPrice, true
}

// tokenCost は、トークンだけの料金（fast モードなどの倍率を掛ける部分）。
func tokenCost(model string, u Tokens) (float64, bool) {
	p, ok := PriceOf(model)
	if !ok {
		return 0, false
	}
	return costAt(p.Rates, u), true
}

// CostOfRequest は、u が応答 1 回分と分かっているときの目安コスト。入力（キャッシュの読み書きを含む）が境目を超えたら
// （OpenAI は OpenAILongContext、Anthropic は LongPrices の Over）、その応答の全部を長いコンテキストの料金にする。
func CostOfRequest(model string, u Tokens) (float64, bool) { return costOfRequest(model, u, u.Input()) }

// costOfRequest は CostOfRequest と同じだが、境目と比べるプロンプトの大きさを prompt で渡す（Event.Prompt を参照）。
func costOfRequest(model string, u Tokens, prompt float64) (float64, bool) {
	p, ok := PriceOf(model)
	if !ok {
		return 0, false
	}
	if p.Long != nil && prompt > p.Over {
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

// ReadUsage は message.usage → Tokens。ウェブ検索の回数（server_tool_use.web_search_requests）も読む。キャッシュ書き込みの内訳（cache_creation の 5 分・1 時間）があれば分ける。
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
	return Tokens{In: NumOr0(m["input_tokens"]), Out: NumOr0(m["output_tokens"]), CW: cw - cw1h, CW1h: cw1h, CR: NumOr0(m["cache_read_input_tokens"]),
		WebSearches: max(0, NumOr0(Map(m["server_tool_use"])["web_search_requests"]))}
}

// Event は 1 つの応答の使用量。Cost が nil なら料金表にないモデル。
type Event struct {
	T       *float64
	Model   string
	U       Tokens
	Cost    *float64
	Mult    float64 // 料金表の値に掛ける倍率（fast モード・US 内だけの推論）。0 は 1 と同じ
	Req     bool    // U が応答 1 回分（入力の量で長いコンテキストの料金を選べる）。false は何回分かの合計かもしれない
	Prompt  float64 // 1 回の呼び出しでモデルが読んだいちばん大きいプロンプト（usage.iterations に呼び出しが 2 回以上あるとき）。0 なら U.Input()
	Advisor bool    // advisor ツールの呼び出し（Model は advisor のモデル）。応答の数やコンテキストの使用率には入れない
}

// prompt は、e の 1 回の呼び出しのプロンプトの大きさ（入力 + キャッシュの読み書き）。
// advisor ツールを使うと、usage の各項目は executor の何回かの呼び出しの和になるので、いちばん大きい 1 回を使う。
func (e Event) prompt() float64 {
	if e.Prompt > 0 {
		return e.Prompt
	}
	return e.U.Input()
}

// EventCost は e のトークンの目安コスト（料金表にないモデルは ok=false）。倍率は掛けず、ウェブ検索の料金も入れない。
func EventCost(e Event) (float64, bool) {
	if e.Req {
		return costOfRequest(e.Model, e.U, e.prompt())
	}
	return tokenCost(e.Model, e.U)
}

// iteration は usage.iterations の 1 つ（advisor ツールを使った応答だけにある）。
type iteration struct {
	model string // advisor の呼び出しのモデル（executor の呼び出しにはない）
	u     Tokens
	mult  float64
}

// readIterations は usage.iterations を読む。prompt は executor の呼び出し（type が message）が 2 回以上あるときの、
// いちばん大きい 1 回のプロンプト（なければ 0）。advisors は advisor の呼び出し（type が advisor_message）。
// usage の各項目は executor の呼び出しの和で、advisor の分は入らない。advisor の分は advisor のモデルの料金でかかる。
// 出典: https://platform.claude.com/docs/en/agents-and-tools/tool-use/advisor-tool （Usage and billing）
func readIterations(raw any) (prompt float64, advisors []iteration) {
	its := List(Map(raw)["iterations"])
	if len(its) == 0 {
		return 0, nil
	}
	n := 0
	for _, it := range its {
		m := Map(it)
		switch Str(m["type"]) {
		case "message":
			n++
			prompt = max(prompt, ReadUsage(m).Input())
		case "advisor_message":
			advisors = append(advisors, iteration{model: Str(m["model"]), u: ReadUsage(m), mult: rateMult(m)})
		}
	}
	if n < 2 {
		prompt = 0
	}
	return prompt, advisors
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
	u.AddAdvised(mid, t, model, "", raw)
}

// AddAdvised は Add と同じだが、advisor ツールの呼び出し（usage.iterations の advisor_message）も、advisor のモデルの
// 応答として足す。advisor のモデルは呼び出しの model、なければ advisor（Claude Code の行の advisorModel）。
func (u *Usage) AddAdvised(mid string, t *float64, model, advisor string, raw any) {
	if mid == "" {
		mid = "_" + itoa(len(u.byMsg))
	}
	prompt, advs := readIterations(raw)
	u.add(mid, Event{T: t, Model: ModelName(model), U: ReadUsage(raw), Mult: rateMult(raw), Req: true, Prompt: prompt})
	for i, a := range advs { // 同じ応答の何行にも同じ iterations が書かれるので、何番目の呼び出しかで重ねる
		u.add(mid+"\x00advisor"+itoa(i), Event{T: t, Model: ModelName(firstNonEmpty(a.model, advisor)), U: a.u, Mult: a.mult, Req: true, Advisor: true})
	}
}

// add は、メッセージ ID（mid）ごとに項目別の最大値をとって e を重ねる。メッセージ ID ごとにまとめるので応答 1 回分。
func (u *Usage) add(mid string, e Event) {
	cur, ok := u.byMsg[mid]
	if !ok {
		u.byMsg[mid] = &e
		u.order = append(u.order, mid)
		return
	}
	cur.Mult = max(cur.Mult, e.Mult)
	if cur.T == nil || *cur.T == 0 {
		cur.T = e.T
	}
	if cur.Model == "" {
		cur.Model = e.Model
	}
	cur.Prompt = max(cur.Prompt, e.Prompt)
	tok := e.U
	cur.U = Tokens{max(cur.U.In, tok.In), max(cur.U.Out, tok.Out), max(cur.U.CW, tok.CW), max(cur.U.CW1h, tok.CW1h), max(cur.U.CR, tok.CR), max(cur.U.WebSearches, tok.WebSearches)}
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

func (u *Usage) Len() int { return len(u.byMsg) }

// Events は [(t, model, usage, cost)]。コストは料金表で見積もる。
// AddEvent で入れたときにコストが決まっているもの（エージェント自身が記録したドル額。Kiro Crew）はそのまま使う。
func (u *Usage) Events() []Event {
	var out []Event
	for _, id := range u.order {
		e := *u.byMsg[id]
		if e.Model == "<synthetic>" {
			continue
		}
		if e.Cost != nil {
			c := *e.Cost
			e.Cost = &c
		} else if c, ok := EventCost(e); ok {
			if e.Mult > 0 {
				c *= e.Mult // 倍率はトークンの料金にだけ掛かり、ウェブ検索の料金には掛からない
			}
			c += e.U.WebSearches * WebSearchPrice
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
		t.WebSearches += e.U.WebSearches
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
