package source

import (
	"bufio"
	"encoding/json"
	"hash/maphash"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 別の会話から写した行（/branch・--fork-session・エージェントの一覧の /fork）を見分ける。
// 写した行は、同じ行（uuid）が元の会話のファイルに、元の会話が数える自身の行としてあるときだけ数えない。
// 履歴は信用できない入力なので、時間とメモリに上限をつけ、上限を超えたら数える側に倒す（行を消さない）:
//   - 1 つの会話のファイルで読む元の会話のファイルは maxOriginsPerUnit 個まで
//   - 元の会話のファイルは、読むのを maxOriginBytes（ほどいたあと）でやめ、印は maxOriginKeys 個まで覚える
//   - 印は 8 バイトのハッシュで持ち、読んだ印は maxOriginSets 個のファイル・合わせて maxCachedKeys 個まで覚えておく
//     （使っていない順に捨てる）。メモリは、覚えておく分（約 maxCachedKeys × 16 バイト）と、読んでいる途中の分
//     （並べて読む会話ごとに maxOriginsPerUnit × maxOriginKeys 個まで）を超えない
const (
	maxOriginsPerUnit = 8       // 1 つの会話のファイルで読む元の会話のファイルの数。超えた分の会話を元とする行は数える
	maxOriginSets     = 256     // 覚えておく元の会話のファイルの数
	maxCachedKeys     = 1 << 22 // 覚えておく印の合計
	maxKeyLen         = 256     // 行の印にする uuid などの長さの上限（Claude Code の uuid は 36 文字）。長い行は突き合わせず数える
)

var (
	maxOriginKeys  = 1 << 18          // 1 つの元の会話のファイルから覚える行の印の数（テストで小さくする）
	maxOriginBytes = int64(256 << 20) // 1 つの元の会話のファイルから読むバイト数（.zst はほどいたあと。テストで小さくする）
	originReads    atomic.Int64       // 元の会話のファイルを読んだ回数（テスト用）
)

// claudeOrigins は、Claude が会話をまたいで持つもの（Units と LoadUnit は並べて呼ばれるので mu で守る）。
type claudeOrigins struct {
	mu    sync.Mutex
	index map[string][]string    // 会話の ID ごとの、kiroku が読むファイル（Units が作る）
	sets  map[string]*originSet  // 元の会話のファイル（パスと大きさ・更新時刻）ごとの行の印
	order []string               // sets のキーを、使っていない順に
	total int                    // sets の印の合計
	used  map[string]usedOrigins // 会話のファイル（Unit.Key）ごとの、写した行を数えなかった元の会話（Units が Tag に使う）
	seed  maphash.Seed           // 行の印のハッシュの種（プロセスごとに変わる）
	init  sync.Once
}

// originSet は、元の会話のファイル 1 つにある、元の会話自身の行の印（ハッシュ）。読むのは 1 回だけ。
type originSet struct {
	once sync.Once
	keys map[uint64]struct{}
	n    int // 読み終えて total に足した印の数
}

// usedOrigins は、1 つの会話のファイルが突き合わせた元の会話: ID と、読んだファイルの印（Stamp）。
type usedOrigins struct {
	ids    []string
	stamps map[string]string
}

// sessionIDPattern は、行の sessionId として受け付ける形（会話のファイル名と突き合わせるので、パスや glob の記号を入れない）。
var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// copiedFrom は、行が別の会話から写したものかもしれないとき、元の会話の ID。行の forkedFrom.sessionId か sessionId が
// このファイルの会話（stem）と違えば、その会話を元とみなす。どちらも公式の文書には書かれていないので、
// これは「どの会話のファイルを確かめるか」の手がかりにだけ使う。sessionId のない行（summary・タイトルなど）は、この会話の行。
func copiedFrom(e core.Obj, stem string) string {
	id := core.Str(core.Map(e["forkedFrom"])["sessionId"])
	if id == "" || id == stem {
		id = core.Str(e["sessionId"])
	}
	if id == "" || id == stem || !sessionIDPattern.MatchString(id) {
		return ""
	}
	return id
}

// lineKey は、写した行を元の会話の行と突き合わせる印: uuid。uuid のない行は、assistant の message.id と requestId の組
// （replyKey）。どちらもない・長すぎるときは空（突き合わせず、数える）。
func lineKey(e core.Obj) string {
	if u := core.Str(e["uuid"]); u != "" {
		if len(u) > maxKeyLen {
			return ""
		}
		return "u\x00" + u
	}
	return replyKey(e)
}

// replyKey は、assistant の行の message.id と requestId の組（どちらもあり、長すぎないとき。なければ空）。
func replyKey(e core.Obj) string {
	if core.Str(e["type"]) == "assistant" {
		id, req := core.Str(core.Map(e["message"])["id"]), core.Str(e["requestId"])
		if id != "" && req != "" && len(id) <= maxKeyLen && len(req) <= maxKeyLen {
			return "m\x00" + id + "\x00" + req
		}
	}
	return ""
}

func (o *claudeOrigins) hash(key string) uint64 {
	o.init.Do(func() { o.seed = maphash.MakeSeed() })
	return maphash.String(o.seed, key)
}

// setIndex は、Units が並べた会話のファイルから、会話の ID ごとのファイルの一覧を作って覚える。
func (o *claudeOrigins) setIndex(units []Unit) {
	index := make(map[string][]string, len(units))
	for _, u := range units {
		id := stemOf(u.Key)
		index[id] = append(index[id], u.Key)
	}
	o.mu.Lock()
	o.index = index
	o.mu.Unlock()
}

// files は、会話 id のファイルのうち kiroku が読むもの（最後の Units のとき）。
func (o *claudeOrigins) files(id string) []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.index[id]
}

func (o *claudeOrigins) hasIndex() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.index != nil
}

// keys は、元の会話のファイル path（会話の ID は id）にある、元の会話自身の行の印と、読んだときのファイルの印（Stamp）。
// 同じファイル（大きさと更新時刻も同じ）は、会話をまたいで 1 回だけ読む。
func (o *claudeOrigins) keys(path, id string) (map[uint64]struct{}, string) {
	stamp := Stamp([]string{path})
	k := path + "\x00" + stamp
	o.mu.Lock()
	s := o.sets[k]
	if s == nil {
		if o.sets == nil {
			o.sets = map[string]*originSet{}
		}
		s = &originSet{}
		o.sets[k] = s
	}
	o.touch(k)
	o.mu.Unlock()
	s.once.Do(func() {
		keys := o.readKeys(path, id)
		o.mu.Lock()
		s.keys = keys
		if o.sets[k] == s { // 読んでいる間に捨てられていなければ、合計に足す
			s.n = len(keys)
			o.total += s.n
			o.evict(k)
		}
		o.mu.Unlock()
	})
	return s.keys, stamp
}

// touch は k をいちばん最近使ったものにする（o.mu を持って呼ぶ）。
func (o *claudeOrigins) touch(k string) {
	for i, x := range o.order {
		if x == k {
			o.order = append(o.order[:i], o.order[i+1:]...)
			break
		}
	}
	o.order = append(o.order, k)
}

// evict は、覚えておくファイルの数と印の合計が上限に収まるまで、使っていない順に捨てる（keep は残す。o.mu を持って呼ぶ）。
func (o *claudeOrigins) evict(keep string) {
	for i := 0; i < len(o.order) && (len(o.order) > maxOriginSets || o.total > maxCachedKeys); {
		k := o.order[i]
		if k == keep {
			i++
			continue
		}
		o.total -= o.sets[k].n
		delete(o.sets, k)
		o.order = append(o.order[:i], o.order[i+1:]...)
	}
}

// originFields は、元の会話の行から読む項目（ほかの項目は組み立てずに読み飛ばす。大きな行でもメモリを使わない）。
var originFields = []string{"uuid", "type", "requestId", "sessionId", "forkedFrom", "message", "timestamp"}

// readKeys は、元の会話のファイルにある行のうち、元の会話が数える自身の行の印。元の会話の LoadUnit と同じく、
// JSON として読めて（core.ReadJSONL が受け付ける）長すぎず、時刻があり、ほかの会話から写した行でない（copiedFrom が空）行だけ。
// 2 つの会話が互いを元と書いた同じ行を持っていても、どちらか（その行を自分の行として持つほう）では必ず数える。
// ふつうのファイルでない・読めないところは印が入らず、写した先で数える。maxOriginKeys 個・maxOriginBytes を超えたら、
// そこで読むのをやめる。
func (o *claudeOrigins) readKeys(path, id string) map[uint64]struct{} {
	originReads.Add(1)
	keys := map[uint64]struct{}{}
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return keys
	}
	f, err := os.Open(path)
	if err != nil {
		return keys
	}
	defer f.Close()
	var src io.Reader = f
	if strings.HasSuffix(path, ".zst") {
		d, err := core.NewZstdReader(f)
		if err != nil {
			return keys
		}
		defer d.Close()
		src = d
	}
	r := bufio.NewReaderSize(io.LimitReader(src, maxOriginBytes), 1<<16)
	for len(keys) < maxOriginKeys {
		line, long, err := core.ReadLine(r, core.MaxLine)
		if !long {
			if e := originLine(line); e != nil && copiedFrom(e, id) == "" && ts(e["timestamp"]) != nil {
				// uuid のない写した行とも突き合わせられるよう、組も入れる。行全体が JSON として正しいかは、印があるときだけ確かめる
				ks := []string{lineKey(e), replyKey(e)}
				if (ks[0] != "" || ks[1] != "") && json.Valid(line) {
					for _, k := range ks {
						if k != "" {
							keys[o.hash(k)] = struct{}{}
						}
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	return keys
}

// originLine は、行から突き合わせに使う項目だけを取り出す。長い uuid などは組み立てない（lineKey が使わない）。
// 取り出さない値が JSON として正しいかは確かめない（readKeys が json.Valid で確かめる）。
func originLine(line []byte) core.Obj {
	raw, ok := topRaw(line, originFields)
	if !ok {
		return nil
	}
	e := core.Obj{}
	for _, k := range []string{"uuid", "type", "requestId", "sessionId", "timestamp"} {
		if v := raw[k]; len(v) > 0 && len(v) <= maxKeyLen+2 {
			var s any
			if json.Unmarshal(v, &s) == nil {
				e[k] = s
			}
		}
	}
	for _, f := range [][2]string{{"forkedFrom", "sessionId"}, {"message", "id"}} { // 中の sessionId・id だけを読む
		if v := raw[f[0]]; len(v) > 0 {
			if m, ok := topFields(v, []string{f[1]}); ok {
				e[f[0]] = m
			}
		}
	}
	return e
}

// originCheck は、1 つの会話のファイルを読む間、写した行を元の会話と突き合わせる。
type originCheck struct {
	o      *claudeOrigins
	sets   map[string][]map[uint64]struct{} // 元の会話の ID ごとの行の印（ファイルごと）。確かめないものは nil
	found  map[string]bool                  // 読んだ元の会話（tag 用）
	stamps map[string]string                // 読んだ元の会話のファイルと、読んだときの印（tag 用）
	reads  int                              // 読んだ元の会話のファイルの数
}

// copied は、行 e（このファイルの会話は stem）が、元の会話にある行の写しか。わからなければ false（数える）。
func (c *originCheck) copied(e core.Obj, stem string) bool {
	from := copiedFrom(e, stem)
	if from == "" {
		return false
	}
	key := lineKey(e)
	if key == "" {
		return false
	}
	sets, ok := c.sets[from]
	if !ok { // ない会話の ID は一覧を引くだけ。読むのは、このファイルで合わせて maxOriginsPerUnit 個のファイルまで
		for _, f := range c.o.files(from) {
			if c.reads >= maxOriginsPerUnit {
				break
			}
			c.reads++
			keys, stamp := c.o.keys(f, from)
			sets = append(sets, keys)
			c.stamps[f] = stamp
		}
		c.sets[from] = sets
		if len(sets) > 0 {
			c.found[from] = true
		}
	}
	h := c.o.hash(key)
	for _, s := range sets {
		if _, ok := s[h]; ok {
			return true
		}
	}
	return false
}

// tag は Unit.Tag: 前に読んだときに突き合わせた元の会話のうち、今はないもの・ファイルが変わったもの。
// kiroku serve で、元の会話が消えたり書き換わったりしたら、写した先を読み直すため。まだ読んでいない会話と、
// 元の会話が前のままのものは空。
func (o *claudeOrigins) tag(key string) string {
	o.mu.Lock()
	u := o.used[key]
	var out []string
	for _, id := range u.ids {
		if len(o.index[id]) == 0 {
			out = append(out, "gone:"+id)
		}
	}
	o.mu.Unlock()
	paths := make([]string, 0, len(u.stamps))
	for p := range u.stamps {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if now := Stamp([]string{p}); now != u.stamps[p] {
			out = append(out, "changed:"+now)
		}
	}
	return strings.Join(out, " ")
}

// remember は、会話のファイル key が突き合わせた元の会話（ID と、読んだファイルの印）を覚える（tag 用）。
// 読んだときになかった元の会話は、写した行をもう数えているので覚えない。
func (o *claudeOrigins) remember(key string, c *originCheck) {
	var ids []string
	for id, ok := range c.found {
		if ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(ids) == 0 {
		delete(o.used, key)
		return
	}
	if o.used == nil {
		o.used = map[string]usedOrigins{}
	}
	o.used[key] = usedOrigins{ids: ids, stamps: c.stamps}
}
