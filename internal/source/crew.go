package source

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Kiro Crew（github.com/kirodotdev/KiroCrew）は kiro-cli を ACP で動かすので、会話そのものは
// ~/.kiro/sessions/cli に kiro-cli の履歴として残る。Crew 側のデータは「Crew から動かした会話」という
// 目印とタイトルに使う。
//
// クレジットは、Crew のダッシュボードから動かした会話だと kiro-cli の履歴に残らないことがある。
// Crew は 1 ターンごとのクレジットを usage/tokens/<日付>.jsonl に書くので、そちらも読む。
// 同じ会話は二重に数えないよう、会話ごとに kiro-cli の記録と Crew の記録の多いほうを使う。Crew の記録は会話キーごとなので、
// キーの会話を差しかえたときは、時刻でそのころの kiro-cli の会話に分ける（crewOwner）。
// kiro-cli の会話に結びつかない記録（Crew の裏方の処理 _bg など）は、Crew のセッションとして数える。
//
//	<KIROCREW_HOME か ~/.kiro/crew>/session_map.json        {会話キー: {"sid": kiro-cli の会話 ID, "cwd": …, "provider"?, "discarded_sid"?} か "sid"}
//	<…>/sessions/<キー>.jsonl                               1 行目 {"_type": "metadata", "title", "agent", "model", "memory_mode"?, …}
//	                                                         2 行目から {"role": "user"|"assistant"|…, "content", "ts", "tools"?: [ツール名]}
//	<…>/subagents/<id>/state.json                          {"task", "agent", "parent_session", "session_id"?, "provider"?, …}
//	<…>/usage/tokens/<YYYY-MM-DD>.jsonl                     {"_type": "tokens", "ts", "slot": 会話キー, "provider", "model", "input", "output",
//	                                                         "cache_create", "cache_read", "cost", "credits", "duration_ms", "context_used",
//	                                                         "context_window", "stop_reason", …}
//
// usage/tokens の行は Crew の dashboard/handlers/usage.py の _build_token_record が書く。backend ごとに埋まる項目が違う
// （acp/types.py の TurnUsage）: kiro-cli はクレジットだけ、Claude Code（claude-agent-acp）などはトークンと USD の cost。
// トークンの 4 つは重ならない（Crew 自身も合計を input + output + cache_create + cache_read で出す。usage.py の context_usage の total）。
// cost は backend が会話の累計で返す額の差分で、届いたターンにまとめて載る（acp/types.py の apply_cost_cumulative）。

// CrewInfo は 1 つの kiro-cli の会話についての Crew の情報。
type CrewInfo struct {
	Subagent bool
	Key      string // Crew の会話キー（サブエージェントなら親の会話キー）
	Title    string
	Agent    string
	Task     string
	Cwd      string
	// Provider は Crew の backend（session_map の "provider"、サブエージェントなら state.json の "provider"。
	// 空か "acp" は kiro-cli、"claude_code"・"codex"・"kas" など）
	Provider string
	// UsageSlot はサブエージェントの使用量の記録の slot（subagentSlot）。サブエージェントでなければ空（使用量は Key で記録される）
	UsageSlot string
	// Former は、その会話キーの今の会話ではない kiro-cli の会話（session_map の discarded_sid か、時刻で見つけた前の会話。crewOwner）
	Former bool
	// Private は、中身を残してはいけない会話（incognito・temporary。crewPrivateMode）。数と時刻だけを出す（hideCrew）
	Private bool
}

// usageSlot は、その会話の使用量の記録の slot。
func (i CrewInfo) usageSlot() string {
	if i.Subagent {
		return i.UsageSlot
	}
	return i.Key
}

// Crew の会話は、メタデータ（会話の記録の 1 行目）の memory_mode が incognito か temporary なら、Crew 自身も
// そこから何も学ばない（要約・検索・書き出しを断る。history.py の INCOGNITO_MEMORY_MODES・transcript_withholds_derivation）。
// kiroku もそうした会話は数と時刻だけを出し、依頼・応答の文、それから決めたタイトル、会話に出たファイルは出さない（hideCrew）。
const (
	crewPrivateTitle = "Kiro Crew private conversation"
	crewPrivateText  = "(private)" // 依頼の文の代わり（依頼の数と時刻は残す）
)

// crewPrivateMode は memory_mode の値が、中身を残してはいけない会話を示すか。
// Crew の history.memory_mode_from_header_line と同じく、ない・null は persistent（古い会話）。文字列は前後の ASCII の空白を除き、
// ASCII の文字だけを小文字にして "persistent" と比べる。それ以外は、incognito・temporary のほか、Crew が読めない値
// （数・空の文字列・Python の lower() と Go の小文字が食い違う "PERSİSTENT" など）も中身を残さない側に倒す（Crew の書き出しも断る）。
func crewPrivateMode(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return asciiLower(strings.Trim(x, " \t\r\n\v\f")) != "persistent"
	}
	return true
}

// asciiLower は ASCII の大文字だけを小文字にする（ほかの文字はそのまま）。
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// crewPrivateMeta は、会話の記録のメタデータ（か、サブエージェントの state.json）が中身を残さない会話を示すか。
// memory_mode と、その中の execution_context.memory_mode を見る（Crew はどちらも厳しくする向きにしか書きかえない）。
// execution_context が object でない（null を除く）ときは、Crew が読めない記録なので中身を残さない側に倒す。
func crewPrivateMeta(e core.Obj) bool {
	ec, bad := e["execution_context"], false
	if ec != nil {
		_, ok := ec.(map[string]any)
		bad = !ok
	}
	return bad || crewPrivateMode(e["memory_mode"]) || crewPrivateMode(core.Map(ec)["memory_mode"])
}

// crewPrivateFlags は session_map の "flags" に temporary か incognito がついているか（session_map.py の _privacy_flags_on。
// Slack などのスレッドの印で、会話の記録のメタデータより先にここに書かれる）。
func crewPrivateFlags(v any) bool {
	flags := core.Map(v)
	for _, name := range []string{"temporary", "incognito"} {
		switch x := flags[name].(type) {
		case nil:
		case bool:
			if x {
				return true
			}
		case float64:
			if x != 0 {
				return true
			}
		case string:
			if x != "" {
				return true
			}
		default: // Python で真になる値（空でない list・object）
			return true
		}
	}
	return false
}

// crewPrivacy は、会話が中身を残さないものかを、会話の記録の名前（safeKey）ごとに覚えておく。1 回の読み込みで 1 つを使い回す
// （loadCrew・loadCrewSpawns・KiroCLI.Load）。印は次のどれか:
//   - session_map の flags（sid のない行も。mark）
//   - 会話の記録（sessions/<名前>.jsonl）、退避した記録（sessions/archive/<名前>__<日時>.jsonl）、
//     その kiroku archive のコピーの、どれかの行のメタデータ（crewFilePrivate）
//
// 会話キーは、そのままの名前と、使用量の記録の書き方をそろえた名前（chat-… → dashboard:chat-…。crewSlot）の両方で調べる。
type crewPrivacy struct {
	home, arch string
	stems      map[string]bool
	idx        *crewArchiveIndex // 退避した記録（archive で作る）
}

func newCrewPrivacy(home, arch string) *crewPrivacy {
	return &crewPrivacy{home: home, arch: arch, stems: map[string]bool{}}
}

// archive は退避した記録を並べたもの（はじめて使うときに 1 度だけ並べる）。
func (p *crewPrivacy) archive() *crewArchiveIndex {
	if p.idx == nil {
		p.idx = newCrewArchiveIndex(p.home, p.arch)
	}
	return p.idx
}

// crewStems は会話キーの記録の名前（そのままと、crewSlot でそろえたもの）。
func crewStems(key string) []string {
	a, b := safeKey(key), safeKey(crewSlot(key))
	if a == b {
		return []string{a}
	}
	return []string{a, b}
}

// mark は会話キーを中身を残さない会話にする。
func (p *crewPrivacy) mark(key string) {
	for _, s := range crewStems(key) {
		p.stems[s] = true
	}
}

// key は会話キーの会話が中身を残さないものか。
func (p *crewPrivacy) key(key string) bool {
	if p == nil || key == "" {
		return false
	}
	for _, s := range crewStems(key) {
		if p.stem(s) {
			return true
		}
	}
	return false
}

// stem は、名前が stem の会話の記録（退避した記録とそのコピーも）が中身を残さない会話を示すか。
func (p *crewPrivacy) stem(stem string) bool {
	if p == nil || p.home == "" {
		return false
	}
	v, ok := p.stems[stem]
	if ok {
		return v
	}
	files := []string{filepath.Join(p.home, "sessions", stem+".jsonl")}
	for _, s := range p.archive().segments(stem) {
		files = append(files, s.path)
	}
	for _, f := range files {
		if crewFilePrivate(f) {
			v = true
			break
		}
	}
	p.stems[stem] = v
	return v
}

// crewMetaMark は、メタデータの行にだけある文字。ほかの行（ずっと多い）は JSON として読まない。
var crewMetaMark = []byte(`"metadata"`)

// crewFilePrivate は、Crew の会話の記録のファイル（.zst のコピーも）のどれかの行が、中身を残さない会話のメタデータか。
// 1 行目の BOM は読み飛ばす（Crew の json.loads は BOM のある行を読めず、中身を残さない側に倒す）。
// 1 行目が長すぎて読めない（core.MaxLine を超える）か、memory_mode を含むのに読めないときも、中身を残さない会話とみなす
// （Crew もメタデータの行を読めなければ断る。history.py の memory_mode_from_header_line）。ない・開けないファイルは false。
// 1 行は core.MaxLine までしか持たないので、使うメモリは決まっている。
func crewFilePrivate(path string) bool {
	f, err := openRegular(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var src io.Reader = f
	if strings.HasSuffix(path, ".zst") {
		d, err := core.NewZstdReader(f)
		if err != nil {
			return false
		}
		defer d.Close()
		src = d
	}
	r := bufio.NewReaderSize(src, 1<<16)
	for first := true; ; first = false {
		line, tooLong, err := core.ReadLine(r, core.MaxLine)
		if first {
			if tooLong {
				return true
			}
			line = bytes.TrimPrefix(line, crewBOM)
		}
		if !tooLong && bytes.Contains(line, crewMetaMark) {
			var e core.Obj
			if json.Unmarshal(line, &e) == nil {
				if core.Str(e["_type"]) == "metadata" && crewPrivateMeta(e) {
					return true
				}
			} else if first && bytes.Contains(line, []byte(`memory_mode`)) {
				return true
			}
		}
		if err != nil {
			return false
		}
	}
}

// openRegular はふつうのファイル（シンボリックリンクの先も）だけを開く。名前つきパイプなどは、開くだけで止まってしまうので開かない
// （ないものと同じに扱い、エラーは返さない）。
func openRegular(path string) (*os.File, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, os.ErrNotExist
	}
	return os.Open(path)
}

// crewBOM は UTF-8 の BOM（手で編集したファイルの先頭につくことがある）。
var crewBOM = []byte("\xef\xbb\xbf")

// crewJSONL は Crew の JSONL を 1 行ずつ読む（core.ReadJSONL と同じ。.zst もほどく）。先頭の BOM は読み飛ばす。
func crewJSONL(path string, fn func(core.Obj)) error {
	f, err := openRegular(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var src io.Reader = f
	if strings.HasSuffix(path, ".zst") {
		d, err := core.NewZstdReader(f)
		if err != nil {
			return err
		}
		defer d.Close()
		src = d
	}
	r := bufio.NewReader(src)
	if b, _ := r.Peek(len(crewBOM)); bytes.Equal(b, crewBOM) {
		_, _ = r.Discard(len(crewBOM))
	}
	return core.ReadJSONLFrom(r, fn)
}

// hideCrew は、中身を残さない会話から、会話の中身から読んだものを消し、決まったタイトルにする（core.Builder.HideText）。
func hideCrew(s *core.Builder) {
	s.HideText(crewPrivateText)
	s.Title = crewPrivateTitle
}

// HideWithheld は、Withholder が中身を出してはいけないとした会話（ほかの Source に残る同じ会話）を、
// Crew の中身を残さない会話と同じ形にした写しを返す（数と時刻だけ。s は変えない）。
func HideWithheld(s *core.Session) *core.Session {
	return s.Hidden(crewPrivateText, crewPrivateTitle)
}

// DefaultCrewHome は KIROCREW_HOME か ~/.kiro/crew。
// Crew は KIRO_HOME を見ない（config/paths.py の _default_home は Path.home()/.kiro/crew。KIRO_HOME が動かすのは kiro-cli の履歴だけ）。
func DefaultCrewHome() string {
	if d := crewHomeOverride(os.Getenv("KIROCREW_HOME")); d != "" {
		return d
	}
	return filepath.Join(home(), ".kiro", "crew")
}

// crewHomeOverride は KIROCREW_HOME の値を Crew と同じく読む（config/paths.py の _valid_override_home）。
// 先頭の ~ と ~<ユーザー名> はホームにし（Python の expanduser）、絶対パスにする。
// ルートや /usr・/System・/etc・/private/etc の下は、Crew が無視して既定の場所を使うので、空を返す。
func crewHomeOverride(v string) string {
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "~") {
		name, rest, _ := strings.Cut(filepath.ToSlash(v[1:]), "/") // Windows では \ も区切り
		dir := ""
		if name == "" {
			dir = home()
		} else if u, err := user.Lookup(name); err == nil {
			dir = u.HomeDir
		}
		if dir != "" {
			v = filepath.Join(dir, filepath.FromSlash(rest))
		}
	}
	if a, err := filepath.Abs(v); err == nil {
		v = a
	}
	if unsafeCrewHome(v) {
		return ""
	}
	return v
}

// unsafeCrewHome は、Crew が KIROCREW_HOME として使わない場所か（config/paths.py の _is_unsafe_home）。
func unsafeCrewHome(p string) bool {
	if filepath.Dir(p) == p {
		return true
	}
	if !strings.HasPrefix(p, "/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	switch parts[0] {
	case "usr", "System", "etc":
		return true
	}
	return len(parts) > 1 && parts[0] == "private" && parts[1] == "etc"
}

// unsafeKey は、会話キーをファイル名にするときに _ にする文字。Crew の history._safe_key は Python の re.sub(r"[^\w\-.]", "_", key) で、
// \w は Unicode の文字・数字と _（str.isalnum() か _。結合文字などの記号は入らない）。Go の \w は ASCII だけなので \p{L}・\p{N} で書く。
// 残るのは文字・数字・_・-・. だけなので、/ や \ もなく sessions/ の外は指せず（.. も後ろに .jsonl がつく）、glob の特別な文字（* ? [ \）も残らない。
var unsafeKey = regexp.MustCompile(`[^\p{L}\p{N}_\-.]`)

// safeKey は会話キーの記録のファイル名（拡張子なし。Crew の history._safe_key と同じ）。
func safeKey(key string) string { return unsafeKey.ReplaceAllString(key, "_") }

// bareChatSlot は使用量の記録に残るダッシュボードの会話キー（chat-<連番>-<UNIX 秒>）。
// 会話そのものは dashboard:chat-… のキーで扱われる（Crew の usage.spend_key_for_slot と同じ規則）。
var bareChatSlot = regexp.MustCompile(`^chat-\d+-\d+$`)

// spendKey は使用量の記録の slot を、会話のキー（session_map や会話の記録と同じ形）にそろえる。
func spendKey(slot string) string {
	if bareChatSlot.MatchString(slot) {
		return "dashboard:" + slot
	}
	return slot
}

// memoryRun は記憶の整理（memory consolidation）の 1 回ごとの slot（memory-consolidation:<記憶の場所>:<uuid4 の 32 桁>。
// Crew の llm_helpers.py が回ごとに作る）。
var memoryRun = regexp.MustCompile(`^(memory-consolidation:.*):[0-9a-f]{32}$`)

// crewSlot は使用量の記録の slot を、kiroku がまとめる単位にする。記憶の整理は回ごとに slot が変わるので、
// 記憶の場所ごとにまとめる（crewOnly で、裏方の処理 _bg と同じく 1 日ごとのセッションにする）。
func crewSlot(slot string) string {
	slot = spendKey(slot)
	if m := memoryRun.FindStringSubmatch(slot); m != nil {
		return m[1]
	}
	return slot
}

// crewBackground は、会話ではなく裏方の処理の slot か（1 日ごとにまとめ、会話の記録は読まない）。
func crewBackground(slot string) bool {
	return slot == "_bg" || strings.HasPrefix(slot, "memory-consolidation:")
}

// loadCrew は kiro-cli の会話 ID → Crew の情報。Crew がなければ空。
// 読めなかったファイルは errs に足す（nil なら黙って飛ばす）。priv は中身を残さない会話（session_map の flags をここで足す。nil なら作る）。
func loadCrew(home string, errs *fileErrs, priv *crewPrivacy) map[string]CrewInfo {
	out := map[string]CrewInfo{}
	if home == "" || !isDir(home) {
		return out
	}
	mapPath := filepath.Join(home, "session_map.json")
	sm, err := core.ReadJSONFile(mapPath)
	errs.file(mapPath, err)
	former := map[string]CrewInfo{} // discarded_sid → Crew の情報（今の sid の会話がなければ使う）
	if priv == nil {
		priv = newCrewPrivacy(home, "")
	}
	entries := core.Map(sm)
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys) // 同じ sid が 2 つのキーにあっても、毎回同じほうを使う
	// 印（flags）は、sid のない行（会話の記録や使用量の記録だけが残る会話）のものも先に覚える
	for _, key := range keys {
		if crewPrivateFlags(core.Map(entries[key])["flags"]) {
			priv.mark(key)
		}
	}
	for _, key := range keys {
		v := entries[key]
		sid, cwd, provider, discarded := core.Str(v), "", "", "" // 古い形は文字列だけ
		if m := core.Map(v); m != nil {
			sid, cwd, provider = core.Str(m["sid"]), core.Str(m["cwd"]), core.Str(m["provider"])
			// 会話を差しかえたとき（backend の切りかえ・会話の破棄・記録が消えた sid の片付け）に sid を空にして残す、前の sid
			// （session_map.py の _stash_and_clear_sid。SessionMap.set はこれを消さないので、新しい sid と並んで残ることもある）
			discarded = core.Str(m["discarded_sid"])
		}
		if sid == "" && discarded == "" {
			continue
		}
		info := CrewInfo{Key: key, Cwd: cwd, Provider: provider, Private: priv.key(key)}
		// 会話キーをファイル名にしたもの（Crew の history._safe_key と同じ）
		path := crewTranscriptPath(home, key)
		errs.file(path, crewJSONL(path, func(e core.Obj) {
			if info.Title == "" && core.Str(e["_type"]) == "metadata" {
				info.Title = core.Str(e["title"])
				info.Agent = core.Str(e["agent"])
			}
		}))
		if sid != "" {
			out[sid] = info
		}
		if _, dup := former[discarded]; discarded != "" && discarded != sid && !dup {
			f := info
			f.Former = true
			former[discarded] = f
		}
	}
	for _, p := range glob(filepath.Join(home, "subagents", "*", "state.json")) {
		v, err := core.ReadJSONFile(p)
		errs.file(p, err)
		st := core.Map(v)
		sid := core.Str(st["session_id"])
		if sid == "" {
			continue
		}
		info := CrewInfo{Subagent: true, Key: core.Str(st["parent_session"]), Agent: core.Str(st["agent"]),
			Task: core.Runes(strings.TrimSpace(core.Str(st["task"])), 120), Cwd: core.Str(st["cwd"]), Provider: core.Str(st["provider"]),
			UsageSlot: subagentSlot(st, filepath.Base(filepath.Dir(p)))}
		// あとから incognito・temporary にしたサブエージェント（subagent_persistence.py は state.json の memory_mode と
		// execution_context.memory_mode を書きかえる）と、中身を残さない会話から起動したサブエージェントは、依頼も残さない
		if crewPrivateMeta(st) || priv.key(info.Key) {
			info.Private, info.Task = true, ""
		}
		out[sid] = info
	}
	for sid, info := range former {
		if _, ok := out[sid]; !ok {
			out[sid] = info
		}
	}
	return out
}

// subagentSlot は、サブエージェントの使用量の記録の slot（loadCrewUsage のキーと同じ形）。
// Crew は conversation_key（残す（keep）サブエージェントだけ state.json に書く）か "subagent:<id>" で記録する
// （subagent_manager/run.py の _run_impl）。id は state.json の "id"、なければフォルダの名前（subagents/<id>/。subagent_persistence.py の _agent_dir）。
func subagentSlot(st core.Obj, dir string) string {
	if k := core.Str(st["conversation_key"]); k != "" {
		return crewSlot(k)
	}
	return crewSlot("subagent:" + firstNonEmpty(core.Str(st["id"]), dir))
}

// crewLogSegment は crew-log の 1 つのファイルの名前（log.jsonl か、後ろの分かれ log.<最初の seq>.jsonl。Crew の crew_log/store.py の segment_paths）。
var crewLogSegment = regexp.MustCompile(`^log(?:\.([0-9]{1,18}))?\.jsonl$`)

// crewSpawnedMark は subagent/spawned の行にだけある文字。ほかの行（ターン・ツールなど、ずっと多い）は JSON として読まない。
var crewSpawnedMark = []byte(`"subagent/spawned"`)

// loadCrewSpawns は、crew-log に残るサブエージェントの起動の記録（使用量の記録の slot "subagent:<id>" → Crew の情報）。
// Crew は届け終えたサブエージェントのフォルダ（subagents/<id>/。state.json）を 1 時間で消す（subagent_persistence.py の
// prune_stale_tombstones。異常終了は 7 日）。一時的な（temporary・incognito の）サブエージェントは state.json をそもそも書かない。
// そのあとも使用量の記録は "subagent:<id>" の slot で残るので、親がわからないと、ひとりで Crew のセッションになってしまう。
// 親の会話の crew-log（追記だけで書きかえない）には、親が起動した子が残る:
//
//	<Crew の場所>/crew-log/sessions/<名前>/log.jsonl（と log.<seq>.jsonl）
//	1 行目 {"type": "session", "id": ACP の会話 ID, "slot": 親の会話キー, "cwd", …}（crew_log/schema.py の SessionHeader）
//	2 行目から {"type": "subagent/spawned", "time", "src", "data": {"agent_id", "agent"?, "task"?, …}}（crew_log/emit.py の on_subagent_spawned）
//
// crew-log はダッシュボードの会話にだけある（サブエージェント自身の会話にはない）。同じ id が何度も出たら、最初に読んだものを使う。
// priv は中身を残さない会話（loadCrew と同じものを渡す。nil なら作る）。
func loadCrewSpawns(home string, errs *fileErrs, priv *crewPrivacy) map[string]CrewInfo {
	out := map[string]CrewInfo{}
	if home == "" {
		return out
	}
	if priv == nil {
		priv = newCrewPrivacy(home, "")
	}
	for _, dir := range glob(filepath.Join(home, "crew-log", "sessions", "*")) {
		if !realDir(dir) {
			continue
		}
		var slot, cwd string
		var spawns []crewSpawn
		seen := map[string]bool{} // 同じ会話の中で読んだ id（中身は最初のものだけ持つ）
		for _, p := range crewLogSegments(dir) {
			s, c, err := readCrewLog(p, seen, &spawns)
			errs.file(p, err)
			slot, cwd = firstNonEmpty(slot, s), firstNonEmpty(cwd, c)
		}
		if slot == "" {
			continue
		}
		// crew-log は会話の memory_mode を見ずに書かれるので、親の会話が中身を残さないものなら、子への依頼も残さない
		private := priv.key(slot)
		for _, x := range spawns {
			key := crewSlot("subagent:" + x.id)
			if _, dup := out[key]; dup {
				continue
			}
			info := CrewInfo{Subagent: true, Key: slot, Agent: x.agent, Task: x.task, Cwd: cwd, UsageSlot: key, Private: private}
			if private {
				info.Task = ""
			}
			out[key] = info
		}
	}
	return out
}

// crewSpawn は subagent/spawned の 1 行のうち kiroku が使うもの（読んだときに切りつめ、ほかの項目は持たない）。
type crewSpawn struct{ id, agent, task string }

// crewLogSegments は crew-log の 1 つの会話のファイルを、書いた順（seq の順。log.jsonl が先）に並べる。
func crewLogSegments(dir string) []string {
	type seg struct {
		p string
		n uint64
	}
	var segs []seg
	// glob ではなくフォルダの中を並べる。dir の名前（* や [ など）がパターンとして読まれ、ほかのフォルダのファイルを拾わないように
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		m := crewLogSegment.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.ParseUint(m[1], 10, 64) // log.jsonl は 0
		segs = append(segs, seg{filepath.Join(dir, e.Name()), n})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].n < segs[j].n })
	out := make([]string, len(segs))
	for i, x := range segs {
		out[i] = x.p
	}
	return out
}

// readCrewLog は crew-log のファイル 1 つから、会話キー（1 行目の slot か、session/opened の data.slot）と作業場所を読み、
// subagent/spawned の行を spawns に足す（seen にある id は飛ばす）。シンボリックリンクでないふつうのファイルだけ。
// 長すぎる行は飛ばし、その数をエラーで返す（ほかの履歴と同じく「Data sources」に出る）。
func readCrewLog(p string, seen map[string]bool, spawns *[]crewSpawn) (slot, cwd string, err error) {
	if st, err := os.Lstat(p); err != nil || !st.Mode().IsRegular() {
		return "", "", nil
	}
	f, err := os.Open(p)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<16)
	long := 0
	for first := true; ; first = false {
		line, tooLong, err := core.ReadLine(r, core.MaxLine)
		var e core.Obj
		switch {
		case tooLong:
			long++
		case first && json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "session":
			slot, cwd = core.Str(e["slot"]), core.Str(e["cwd"])
		case slot == "" && bytes.Contains(line, []byte(`"session/opened"`)):
			if json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "session/opened" {
				slot = core.Str(core.Map(e["data"])["slot"])
			}
		case bytes.Contains(line, crewSpawnedMark):
			if json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "subagent/spawned" {
				d := core.Map(e["data"])
				if id := core.Str(d["agent_id"]); id != "" && !seen[id] {
					seen[id] = true
					*spawns = append(*spawns, crewSpawn{id: id, agent: core.Runes(core.Str(d["agent"]), 120),
						task: core.Runes(strings.TrimSpace(core.Str(d["task"])), 120)})
				}
			}
		}
		if err == io.EOF {
			if long > 0 {
				return slot, cwd, fmt.Errorf("skipped %d line(s) longer than %d bytes", long, core.MaxLine)
			}
			return slot, cwd, nil
		}
		if err != nil {
			return slot, cwd, err
		}
	}
}

// subagentTitle はサブエージェントの会話のタイトル（"Subagent <エージェント>: <依頼>"）。
func subagentTitle(info CrewInfo) string {
	title := "Subagent"
	if info.Agent != "" {
		title += " " + info.Agent
	}
	if info.Task != "" {
		title += ": " + info.Task
	}
	return title
}

// crewHeld は、出す前の Crew の会話。サブエージェントの会話は、親の会話（parent_session の会話キー）が見つかれば
// その会話のサブエージェントにまとめる（Claude Code・Codex と同じく、親のセッションの中に出す）。親が見つからなければ、そのまま出す。
type crewHeld struct {
	list    []heldCrew
	parents map[string]*core.Builder // 会話キー（crewSlot でそろえたもの）→ 親になれる会話
	byPath  map[string]*core.Builder // 会話の記録だけがある会話（記録のファイル → 会話）
	private map[*core.Builder]bool   // 中身を残さない会話（出す前に hideCrew する）
}

// hide は、預かった会話 b を中身を残さない会話にする（出す前に hideCrew する）。
func (h *crewHeld) hide(b *core.Builder) {
	if h.private == nil {
		h.private = map[*core.Builder]bool{}
	}
	h.private[b] = true
}

type heldCrew struct {
	b    *core.Builder
	info *CrewInfo // わからなければ nil
}

// add は Crew の会話を 1 つ預かる。key はその会話の会話キー（サブエージェントなら親の会話キー）。
func (h *crewHeld) add(b *core.Builder, info *CrewInfo, key string) {
	h.list = append(h.list, heldCrew{b: b, info: info})
	if info != nil && info.Private {
		h.hide(b)
	}
	if (info == nil || !info.Subagent) && key != "" {
		if k := crewSlot(key); h.parents[k] == nil {
			h.parents[k] = b
		}
	}
}

// flush はサブエージェントを親の会話にまとめてから、全部の会話を出す。
// 中身を残さない会話は、まとめる前（サブエージェントの編集したファイルなどを親に入れないように）と、
// まとめたあと（親に入ったサブエージェントへの依頼を消すように）に hideCrew する。
func (h *crewHeld) flush(home string, emit func(*core.Builder)) {
	for b := range h.private {
		hideCrew(b)
	}
	folded := map[*core.Builder]bool{}
	for _, x := range h.list {
		if x.info == nil || !x.info.Subagent || x.info.Key == "" {
			continue
		}
		parent := h.parents[crewSlot(x.info.Key)]
		if parent == nil && home != "" {
			parent = h.byPath[crewTranscriptPath(home, x.info.Key)]
		}
		if parent == nil || parent == x.b {
			continue
		}
		foldCrewSubagent(parent, x.b, *x.info)
		folded[x.b] = true
	}
	for b := range h.private {
		hideCrew(b)
	}
	for _, x := range h.list {
		if folded[x.b] {
			continue
		}
		sort.SliceStable(x.b.Subagents, func(i, j int) bool { return lessTime(x.b.Subagents[i].Start, x.b.Subagents[j].Start) })
		emit(x.b)
	}
}

// lessTime は、時刻のあるものを先に、早い順に並べる。
func lessTime(a, b *float64) bool {
	if a == nil || b == nil {
		return a != nil && b == nil
	}
	return *a < *b
}

// foldCrewSubagent はサブエージェントの会話を、親の会話のサブエージェントとして入れる。
// クレジット・Crew の参考指標（Crew から動かした会話・うちサブエージェントも）・編集したファイルは親の会話の数字に入れる
// （まとめる前と、合計は変わらない）。トークンとドル額はサブエージェントの使用量として入り、親のセッションの合計に足される。
// サブエージェントが動いた時刻は、親の会話で AI が動いていた時刻にする（Claude Code と同じく、作業時間に入り、待たせ時間にならない）。
func foldCrewSubagent(parent, sub *core.Builder, info CrewInfo) {
	for i := range sub.Times {
		parent.Agent(&sub.Times[i])
	}
	parent.Measures = append(parent.Measures, sub.Measures...)
	parent.Credits = append(parent.Credits, sub.Credits...)
	for _, f := range sub.EditedFiles() {
		parent.Edited(f)
	}
	var start, end *float64
	if len(sub.Times) > 0 {
		ts := append([]float64(nil), sub.Times...)
		sort.Float64s(ts)
		start, end = &ts[0], &ts[len(ts)-1]
	}
	var model *string
	if m := sub.TopModel(); m != "" {
		model = &m
	}
	tools := 0.0
	for _, n := range sub.ToolCounts() {
		tools += float64(n)
	}
	evs := sub.Usage.Events()
	parent.Subagents = append(parent.Subagents, core.Subagent{Type: firstNonEmpty(info.Agent, "subagent"), Desc: info.Task,
		Start: start, End: end, Model: model, Tools: tools, Usage: core.SumUsage(evs), Events: evs})
}

// crewRow は Crew の会話の 1 行。
type crewRow struct {
	t     *float64
	role  string
	text  string
	tools []string
	human bool   // meta.human が true（人が書いた行。Crew の history.HUMAN_TURN_META_KEY）
	mid   string // meta.mid（Crew が行ごとにつける ID。dashboard/state.py の row_mid。なければ空）
	rawTS string // ts の値をそのまま（mid のない行を見分けるため。crewRowSet）
	// hidden は、中身を出さない行（今の会話より前に同じ名前で退避された記録の行。crewEarlierRow）。数と時刻だけを使う
	hidden bool
}

// crewMeta は Crew の会話の記録のメタデータ（1 行目）。
type crewMeta struct {
	title      string
	forkedFrom string   // 会話を分けて（fork して）作ったなら、元の会話キー（"forked_from"）
	created    *float64 // "created_at"（その記録を作った時刻。fork なら分けた時刻）
	private    bool     // 中身を残さない会話（memory_mode。crewPrivateMeta）
	archive    string   // 退避した記録のファイルの 1 行目 {"_type": "archive", "reason"} の reason（そのファイルだけのもの。add では重ねない）
}

// add は、まだ決まっていない項目を m2 で埋める。private はどれかのファイルが示せば true（Crew も厳しくする向きにしか書きかえない）。
func (m *crewMeta) add(m2 crewMeta) {
	m.title = firstNonEmpty(m.title, m2.title)
	m.forkedFrom = firstNonEmpty(m.forkedFrom, m2.forkedFrom)
	if m.created == nil {
		m.created = m2.created
	}
	m.private = m.private || m2.private
}

// ownRows は、fork した会話から、元の会話から写した行を除く。
// Crew は fork するとき、新しい会話（created_at はそのときの時刻）に元の会話の行を元の ts と meta のまま写す
// （dashboard/chat_fork.py。tail fork も同じ）。そのあとの行の ts は、写した行より後で created_at より後になる
// （state.py の _ChatSlot.append と history.py の monotonic_transcript_ts）。なので created_at より前の行は写した行。
// 元の会話は kiroku が別に読むので、写した行も数えると同じ依頼を 2 度数える。
// created_at がない・読めない fork は見分けられないので、全部の行を使う。ts のない行も残す。
func ownRows(m crewMeta, rows []crewRow) []crewRow {
	if m.forkedFrom == "" || m.created == nil {
		return rows
	}
	out := rows[:0:0]
	for _, r := range rows {
		if r.t != nil && *r.t < *m.created {
			continue
		}
		out = append(out, r)
	}
	return out
}

// crewTranscriptPath は会話キーの記録ファイル（Crew の history._safe_key と同じ名前）。
func crewTranscriptPath(home, key string) string {
	return filepath.Join(home, "sessions", safeKey(key)+".jsonl")
}

// readCrewKey は会話キーの記録を読む（readCrewStem）。
func readCrewKey(home, arch, key string, errs *fileErrs) (meta crewMeta, rows []crewRow) {
	return readCrewStem(home, arch, safeKey(key), errs)
}

// crewSeg は退避した記録のファイル 1 つ（Crew の history._archive_lines が書く。同じ秒に重なれば <日時>-<番号>。番号は 1 から。番号なしが先）。
type crewSeg struct {
	path, stamp string
	n           int
}

// readCrewStem は会話の記録（sessions/<stem>.jsonl。stem は safeKey の名前）を読む。Crew は古い行を sessions/archive/<名前>__<日時>.jsonl に退避する
// （残す期間は session.archive_retention_days で決まる。既定 30 日。crewRetentionDays）ので、残っていればそちらも書いた順に読む。
// 退避した記録が消えていても、kiroku archive のコピー（arch の下の同じ並び。.jsonl.zst）があればそれを読む。
// 名前が <stem>__ で始まっても、そのあとが日時でないもの（<stem>__x という別の会話のもの）は読まない。
// 退避した記録は、あふれた古い行（reason "rotate"）だけではない（crewArchiveRows）。
// fork した会話なら、元の会話から写した行は除く（ownRows）。
func readCrewStem(home, arch, stem string, errs *fileErrs) (meta crewMeta, rows []crewRow) {
	return readCrewStemIn(newCrewArchiveIndex(home, arch), stem, errs)
}

// readCrewStemIn は readCrewStem と同じ。退避した記録は idx（1 回の読み込みで 1 度だけ並べたもの）から探す。
func readCrewStemIn(idx *crewArchiveIndex, stem string, errs *fileErrs) (meta crewMeta, rows []crewRow) {
	segs := idx.segments(stem)
	paths := make([]string, 0, len(segs)+1)
	for _, s := range segs {
		paths = append(paths, s.path)
	}
	segRows := make([][]crewRow, 0, len(paths)+1)
	reasons := make([]string, 0, len(paths)+1)
	var live crewMeta // 今の記録のメタデータ
	for i, p := range append(paths, filepath.Join(idx.home, "sessions", stem+".jsonl")) {
		m, rs := readCrewTranscript(p, errs)
		meta.add(m)
		segRows = append(segRows, rs)
		if i == len(paths) {
			m.archive = "" // 今の記録（退避した記録ではない）
			live = m
		}
		reasons = append(reasons, m.archive)
	}
	// 今の会話より前に同じ名前で退避された記録の行は、中身を出さない（crewEarlierRow）
	start := live.created
	if start != nil && *start < crewEpoch {
		start = nil // Crew より前の時刻は、壊れた created_at
	}
	for i, sg := range segs {
		stampAfter := start != nil && crewStampAfter(sg.stamp, *start)
		for j := range segRows[i] {
			segRows[i][j].hidden = crewEarlierRow(segRows[i][j], start, stampAfter)
		}
	}
	return meta, ownRows(meta, crewArchiveRows(segRows, reasons))
}

// crewEpoch は、これより前の created_at を壊れた値とみなす時刻（2020-01-01T00:00:00Z。Kiro Crew はそれより新しい）。
const crewEpoch = 1577836800

// crewEarlierRow は、退避した記録の行 r が、start（今の記録の created_at）に始まった今の会話より前のもので、中身を出さないか。
//
// 会話の記録が消えたあと、同じ会話キーで新しい会話が始まることがある。前の会話が incognito・temporary だったかは、退避した記録
// （1 行目は {"_type": "archive", "reason"} だけ）からはわからないので、前の会話の行は中身を出さない（数と時刻だけ）。見分け方:
//   - 今の記録の created_at（Crew は時差つきの ISO 8601 で書き、書きなおしても残す）がわからない（記録がない・created_at がない・
//     文字列でない・読めない・2020 年より前）なら、どの行が今の会話のものか決められないので、全部の行の中身を出さない
//   - 行に時刻（ts。時差つき）があれば、それが start より前なら前の会話の行（今の会話の行は記録を作ったあとに書かれる）
//   - 時刻のない行は、ファイル名の日時（Crew がそのマシンの時刻で書く。時差はわからない）が、どの時差（-12〜+14 時間）でも start より
//     後のとき（stampAfter）だけ今の会話のものとみなす
//
// fork した会話の、元の会話から写した行（start より前の時刻）も中身を出さないことになるが、ownRows がもともと除く。
func crewEarlierRow(r crewRow, start *float64, stampAfter bool) bool {
	switch {
	case start == nil:
		return true
	case r.t != nil:
		return *r.t < *start
	}
	return !stampAfter
}

// crewStampAfter は、退避した記録のファイル名の日時 stamp（YYYYMMDD-HHMMSS。Crew のマシンの時刻）が、どの時差でも start より後か。
// stamp を UTC として読んだ時刻から 14 時間（いちばん東の時差）引いても start 以降なら後。読めなければ false。
func crewStampAfter(stamp string, start float64) bool {
	t, err := time.ParseInLocation("20060102-150405", stamp, time.UTC)
	return err == nil && float64(t.Unix())-14*3600 >= start
}

// crewArchiveName は退避した記録のファイル名（<名前>__<YYYYMMDD-HHMMSS>[-<番号>].jsonl）。<名前> は、日時の前の最後の __ まで。
var crewArchiveName = regexp.MustCompile(`^(.+)__(\d{8}-\d{6})(?:-(\d{1,9}))?\.jsonl$`)

// crewArchiveIndex は、退避した記録（sessions/archive/ と、元が消えたものは kiroku archive のコピー）を、名前ごとに書いた順に並べたもの。
// フォルダは 1 回の読み込みで 1 度だけ並べる（会話キーごとに探すと、会話の数の 2 乗に比例して遅くなる）。
type crewArchiveIndex struct {
	home   string
	byStem map[string][]crewSeg
}

// newCrewArchiveIndex は home（と arch）の退避した記録を並べる。
func newCrewArchiveIndex(home, arch string) *crewArchiveIndex {
	idx := &crewArchiveIndex{home: home, byStem: map[string][]crewSeg{}}
	if home == "" {
		return idx
	}
	add := func(p, name string) {
		m := crewArchiveName.FindStringSubmatch(name)
		if m == nil {
			return
		}
		n, _ := strconv.Atoi(m[3]) // 番号なしは 0
		idx.byStem[m[1]] = append(idx.byStem[m[1]], crewSeg{path: p, stamp: m[2], n: n})
	}
	dir := filepath.Join(home, "sessions", "archive")
	ents, _ := os.ReadDir(dir)
	orig := map[string]bool{}
	for _, e := range ents {
		if !e.IsDir() {
			orig[e.Name()] = true
			add(filepath.Join(dir, e.Name()), e.Name())
		}
	}
	if arch != "" {
		adir := filepath.Join(arch, "sessions", "archive")
		ents, _ := os.ReadDir(adir)
		for _, e := range ents {
			name, ok := strings.CutSuffix(e.Name(), ".zst")
			if ok && !e.IsDir() && !orig[name] {
				add(filepath.Join(adir, e.Name()), name)
			}
		}
	}
	// 書いた順: 日時の順、同じ日時なら番号の順（名前の順だと "-1" が番号なしより先、"-10" が "-2" より先になる）
	for _, segs := range idx.byStem {
		sort.Slice(segs, func(i, j int) bool {
			if segs[i].stamp != segs[j].stamp {
				return segs[i].stamp < segs[j].stamp
			}
			return segs[i].n < segs[j].n
		})
	}
	return idx
}

// segments は名前が stem の会話の退避した記録（書いた順）。
func (idx *crewArchiveIndex) segments(stem string) []crewSeg {
	if idx == nil {
		return nil
	}
	return idx.byStem[stem]
}

// crewArchiveRows は、退避した記録（書いた順）と今の記録（最後）の行をつなぐ。reasons はファイルごとの reason（今の記録は空）。
// Crew は次のわけで行を退避する（history.py の _archive_lines。ファイルの 1 行目 {"_type": "archive", "reason", …}）:
//   - "rotate": 記録が大きくなりすぎて、古い行をそのまま移したもの（本当の古い会話）。全部読む
//   - "foreign-dedup": ほかのプロセスが書いた行のうち、今の記録に残る行と同じとみなして落としたもの
//     （dashboard/slot_persistence/transcript_merge.py）。同じ行が今の記録にあるので読まない
//   - "compact": 巻き戻し・作り直し・fork で記録を書きなおしたときに落とした行（同じ）。会話を巻き戻して消えたターン
//     （本当にあった依頼）と、書きかえた行の前の形（新しい形が残っている）が入るので、残っている行と同じ行だけを除く（crewRowSet）
//
// Crew 自身の画面は "rotate" だけを読む（history_projection.py の read_rotated_messages。消したものを画面に戻さないため）が、
// kiroku はあったことの記録なので、巻き戻したターンの依頼も残す。reason のないファイル（古い Crew）は "rotate" と同じに読む。
func crewArchiveRows(segs [][]crewRow, reasons []string) []crewRow {
	kept := newCrewRowSet()
	for i, rs := range segs {
		if reasons[i] == "" || reasons[i] == "rotate" {
			kept.addAll(rs)
		}
	}
	var out []crewRow
	for i, rs := range segs {
		switch reasons[i] {
		case "", "rotate":
			out = append(out, rs...)
		case "foreign-dedup":
		default:
			for _, r := range rs {
				if !kept.has(r) {
					kept.add(r)
					out = append(out, r)
				}
			}
		}
	}
	return out
}

// crewRowSet は、行が同じかを Crew と同じ規則で調べるための集まり（history_projection.py の drop_persisted_tail_prefix）:
// どちらの行にも meta.mid があれば mid が同じか、そうでなければ role・ts・内容が全部同じなら同じ行。
type crewRowSet struct {
	mids      map[string]bool
	all, bare map[string]bool // role・ts・内容（bare は mid のない行だけ）
}

func newCrewRowSet() *crewRowSet {
	return &crewRowSet{mids: map[string]bool{}, all: map[string]bool{}, bare: map[string]bool{}}
}

// crewRowKey は、mid で見分けられない行を見分ける role・ts・内容。
func crewRowKey(r crewRow) string { return r.role + "\x00" + r.rawTS + "\x00" + r.text }

func (s *crewRowSet) add(r crewRow) {
	k := crewRowKey(r)
	s.all[k] = true
	if r.mid != "" {
		s.mids[r.mid] = true
	} else {
		s.bare[k] = true
	}
}

func (s *crewRowSet) addAll(rs []crewRow) {
	for _, r := range rs {
		s.add(r)
	}
}

// has は、r と同じ行が s にあるか。
func (s *crewRowSet) has(r crewRow) bool {
	if r.mid != "" {
		return s.mids[r.mid] || s.bare[crewRowKey(r)]
	}
	return s.all[crewRowKey(r)]
}

// readCrewTranscript は Crew の会話の記録を読む。meta は 1 行目のメタデータ（退避した記録なら reason も）。
// 途中で読めなくなっても、読めた行までは返す。fork の写した行は除かない（ownRows）。
func readCrewTranscript(path string, errs *fileErrs) (meta crewMeta, rows []crewRow) {
	errs.file(path, crewJSONL(path, func(e core.Obj) {
		switch core.Str(e["_type"]) {
		case "metadata":
			var created *float64 // Crew は時差つきの ISO 8601 の文字列で書く。ほかの形（数など）は読まない
			if c, ok := e["created_at"].(string); ok {
				created = ts(c)
			}
			meta.add(crewMeta{title: core.Str(e["title"]), forkedFrom: core.Str(e["forked_from"]), created: created, private: crewPrivateMeta(e)})
			return
		case "archive":
			if meta.archive == "" {
				meta.archive = core.Str(e["reason"])
			}
			return
		}
		r := crewRow{t: ts(e["ts"]), role: core.Str(e["role"]), text: core.TextOf(e["content"]), rawTS: fmt.Sprint(e["ts"])}
		m := core.Map(e["meta"])
		r.human, _ = m["human"].(bool) // Crew と同じく true だけ（"true" や 1 は数えない）
		r.mid, _ = m["mid"].(string)   // Crew と同じく空でない文字列だけ（row_mid）
		for _, x := range core.List(e["tools"]) {
			if n := core.Str(x); n != "" {
				r.tools = append(r.tools, n)
			}
		}
		if r.role != "" {
			rows = append(rows, r)
		}
	}))
	return meta, rows
}

// taskRunnerRow は、タスクの実行が会話の記録に書く user の行（taskrunner.py の _log_task: "[Task: <spec の名前>] Task <番号>: <題>"）。
var taskRunnerRow = regexp.MustCompile(`^\[Task: [^\n]*\] Task \d+: `)

// addCrewRows は Crew の会話の記録から、依頼の流れ・時刻・使ったツールを足す。
// kiro-cli の履歴に依頼が残っていない会話（Crew のダッシュボードから動かしたものなど）のため。
//
// role が user の行は、人が書いたとは限らない。Crew は予定の実行（"# Cron Run: …"）・Issue Radar の呼び出し・
// タスクの実行・auto-go なども user の行として書き、人が書いた行にだけ meta.human: true をつける
// （history.py の HUMAN_TURN_META_KEY。目印のない行は人のものと数えない、という allowlist）。
// 目印は新しい Crew だけが書くので、最初に目印のある行からあとは、目印のある行だけを依頼に数え、
// ほかの user の行は仕組みが入れたもの（kind "agent"。Claude Code のほかのエージェントや予定から送られたものと同じ）として残す。
// それより前の行（目印ができる前の記録）は、ほぼ見分けられないので今までどおり依頼に数える。
// ただしタスクの実行の行（taskRunnerRow）は形で見分けられ、目印も書かれない（タスクの実行だけの記録には目印のある行がない）ので、仕組みが入れたものにする。
// Crew 自身は目印のない記録で "[" で始まる user の行をすべて仕組みのものとみなす（dashboard/handlers/members.py）が、
// 人が "[WIP] …" のように書いた依頼まで落とさないよう、kiroku は形の決まったタスクの実行の行だけを除く。
func addCrewRows(s *core.Builder, rows []crewRow) {
	marked := false // 目印を書く Crew の記録に入ったか
	for _, r := range rows {
		marked = marked || r.human
		switch {
		case r.hidden && r.role == "user": // 中身を出さない行: 依頼の数と時刻だけ
			s.Tick(r.t)
			if !marked || r.human {
				s.Prompt(r.t, crewPrivateText)
			}
		case r.hidden:
			s.Agent(r.t)
		case r.role == "user":
			s.Tick(r.t)
			if marked && !r.human || !marked && taskRunnerRow.MatchString(r.text) {
				s.InjectAll(r.t, "agent", r.text)
			} else {
				s.Prompt(r.t, r.text)
			}
		case r.role == "assistant":
			s.Agent(r.t)
			s.Reply(r.t, "", r.text)
		default: // tool / tool_call / tool_result など
			s.Agent(r.t)
		}
		for _, n := range r.tools {
			s.Tool(n, nil)
		}
	}
}

// tagCrew は Crew から動かした kiro-cli の会話に目印とタイトルをつける。つけたら true。
func tagCrew(s *core.Builder, crew map[string]CrewInfo) bool {
	info, ok := crew[s.ID]
	if ok {
		tagCrewInfo(s, info)
	}
	return ok
}

// tagCrewInfo は kiro-cli の会話に、Crew から動かした目印とタイトルをつける。
func tagCrewInfo(s *core.Builder, info CrewInfo) {
	s.Source = "Kiro Crew"
	var first *float64
	for i := range s.Times {
		if first == nil || s.Times[i] < *first {
			first = &s.Times[i]
		}
	}
	s.Measure("crew_sessions", first, 1)
	if info.Subagent {
		s.Measure("crew_subagents", first, 1)
	}
	if info.Subagent {
		s.Title = subagentTitle(info)
	} else if info.Title != "" {
		s.Title = info.Title
	}
}

// crewOwner は、使用量の記録の slot に結びつく kiro-cli の会話。
//
// Crew は会話キーの sid を差しかえる（SessionMap.set。容量がいっぱいになった会話の作り直し・backend の切りかえ・seed）が、
// 使用量の記録は会話キー（slot）で書き、どの kiro-cli の会話のものかは書かない（usage.py の _build_token_record）。
// 前の会話は session_map から消えても、kiro-cli の履歴に自分のクレジットとともに残る（persistent でない会話は Crew が消す）。
// なので slot の行を全部今の会話に足すと 2 度数える。行（と会話の記録の行）は時刻で、そのころに動いていた会話に結びつける（crewOwnerAt）。
type crewOwner struct {
	s          *core.Builder
	info       CrewInfo
	start, end *float64 // 会話の最初と最後の時刻（なければ nil）
	turns      []crewTurn
	rows       []crewRow
}

// newCrewOwner は kiro-cli の会話 s を、info の会話キーの会話にする。
func newCrewOwner(s *core.Builder, info CrewInfo) *crewOwner {
	o := &crewOwner{s: s, info: info}
	o.start, o.end = timeSpan(s.Times)
	return o
}

// timeSpan は時刻の最初と最後（なければ nil）。
func timeSpan(ts []float64) (first, last *float64) {
	for i := range ts {
		if first == nil || ts[i] < *first {
			first = &ts[i]
		}
		if last == nil || ts[i] > *last {
			last = &ts[i]
		}
	}
	return first, last
}

// crewTurnSlack は、kiro-cli の会話の最後の時刻（ターンの終わり）から、Crew がそのターンの使用量の記録を書くまでの余裕（秒）。
const crewTurnSlack = 120

// sortCrewOwners は会話を始まりの順に並べる（時刻のない会話が先）。
func sortCrewOwners(ow []*crewOwner) {
	sort.SliceStable(ow, func(i, j int) bool {
		a, b := ow[i].start, ow[j].start
		if a == nil || b == nil {
			return a == nil && b != nil
		}
		return *a < *b
	})
}

// crewOwnerAt は、時刻 t の行を結びつける会話の番号（ow は sortCrewOwners の順）。どれでもなければ -1。
// 行は、t より前に始まった会話のうち、t がその会話の間に入る最後のものに結びつける。今の会話（Former でない）の間は始まりから先の全部
// （kiro-cli に記録が残らないターンがあるので）、前の会話の間は最後の時刻 + crewTurnSlack まで
// （backend を切りかえたあとの行は、kiro-cli の会話のものではない）。時刻のない前の会話には結びつけない。
func crewOwnerAt(ow []*crewOwner, t float64) int {
	for i := len(ow) - 1; i >= 0; i-- {
		o := ow[i]
		if o.info.Former {
			if o.start == nil || t < *o.start || t > *o.end+crewTurnSlack {
				continue
			}
			return i
		}
		if o.start == nil || t >= *o.start {
			return i
		}
	}
	return -1
}

// crewKiroRow は kiro-cli の行らしいか（トークンもドル額もない。kiro-cli はクレジットだけを返す）。
func crewKiroRow(x crewTurn) bool {
	return x.u.Total() == 0 && x.cost == 0
}

// claimCrewFormer は、session_map から消えた前の kiro-cli の会話（Crew の目印のない会話）を、どの会話にも結びつかない行から見つけて ow に足す。
// 行の時刻が会話の最初から最後 + crewTurnSlack に入り、作業場所（session_map の cwd）が同じ会話が 1 つだけなら、その会話にする。
// 2 つ以上あれば決めない（その行は Crew のセッションになる）。taken は、もう別の会話キーの前の会話にしたもの。
// 作業場所がわからない会話キー（古い session_map）とサブエージェントでは探さない。
func claimCrewFormer(ow []*crewOwner, turns []crewTurn, plain []*core.Builder, info CrewInfo, taken map[*core.Builder]bool) []*crewOwner {
	if info.Subagent || info.Cwd == "" {
		return ow
	}
	spans := make([][2]*float64, len(plain))
	for i, p := range plain {
		spans[i][0], spans[i][1] = timeSpan(p.Times)
	}
	for _, x := range turns {
		if crewOwnerAt(ow, x.t) >= 0 || !crewKiroRow(x) {
			continue
		}
		found := -1
		for i, p := range plain {
			first, last := spans[i][0], spans[i][1]
			if p.Project != info.Cwd || first == nil || x.t < *first || x.t > *last+crewTurnSlack {
				continue
			}
			if found >= 0 {
				found = -2 // 2 つ以上
				break
			}
			found = i
		}
		if found < 0 || taken[plain[found]] {
			continue
		}
		taken[plain[found]] = true
		f := info
		f.Former = true
		tagCrewInfo(plain[found], f)
		ow = append(ow, newCrewOwner(plain[found], f))
		sortCrewOwners(ow)
	}
	return ow
}

// crewTurn は usage/tokens の 1 行（Crew が記録した 1 ターン）。
type crewTurn struct {
	t, start, credits float64
	model, provider   string      // provider は行の "provider"（"acp"・"claude_code"・"codex" など。crewElsewhere）
	u                 core.Tokens // input → In、output → Out、cache_create → CW、cache_read → CR（重ならない）
	cost              float64     // backend が返した USD（kiro-cli の行は 0）
	ctx               float64     // context_used ÷ context_window（わからなければ -1）
	stop              string      // stop_reason（"" は記録なし）
}

// crewCount は行のトークン数やドル額。数でない・負の数・有限でない値は 0。
func crewCount(v any) float64 {
	f, ok := core.Num(v)
	if !ok || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// loadCrewUsage は usage/tokens/*.jsonl を会話キー（slot）ごとにまとめる。時刻の古い順。
func loadCrewUsage(home string, errs *fileErrs) map[string][]crewTurn {
	out := map[string][]crewTurn{}
	if home == "" {
		return out
	}
	for _, p := range glob(filepath.Join(home, "usage", "tokens", "*.jsonl")) {
		errs.file(p, core.ReadJSONL(p, func(e core.Obj) {
			if core.Str(e["_type"]) != "tokens" {
				return
			}
			t := ts(e["ts"])
			c, ok := core.Num(e["credits"])
			if t == nil || !ok || c < 0 {
				return
			}
			start := *t
			if d, ok := core.Num(e["duration_ms"]); ok && d > 0 {
				start -= d / 1000
			}
			x := crewTurn{t: *t, start: start, credits: c, model: core.Str(e["model"]), provider: core.Str(e["provider"]),
				u:    core.Tokens{In: crewCount(e["input"]), Out: crewCount(e["output"]), CW: crewCount(e["cache_create"]), CR: crewCount(e["cache_read"])},
				cost: crewCount(e["cost"]), ctx: -1, stop: core.Str(e["stop_reason"])}
			// コンテキストの使用率。窓がない・0 の行は使わない（Crew の context_occupancy と同じ）
			if used, ok := core.Num(e["context_used"]); ok && used >= 0 && !math.IsInf(used, 0) {
				if w, ok := core.Num(e["context_window"]); ok && w > 0 && !math.IsInf(w, 0) {
					x.ctx = min(used/w, 1)
				}
			}
			slot := crewSlot(core.Str(e["slot"]))
			out[slot] = append(out[slot], x)
		}))
	}
	for k := range out {
		sort.SliceStable(out[k], func(i, j int) bool { return out[k][i].t < out[k][j].t })
	}
	return out
}

// crewElsewhere は、その行のトークンとドル額を、kiroku が別に読む履歴が記録しているか（crewOwnHistory）。
// Crew の Claude Code の seam は claude-agent-acp で Claude Code を動かし、Claude Code が ~/.claude/projects に自分の記録を書く
// （Crew の providers/acp.py の cleanup_session の注記）。Codex の backend も Codex 自身が ~/.codex に記録を書くものとみなす。
// Crew の会話 ID（session_map の sid）とそれらの履歴の会話 ID が同じかは、手元の根拠（Crew と claude-agent-acp の公開ソース）では確かめられない。
// 結びつけられないまま足すと同じトークンとコストを 2 度数えるので、これらの行ではトークンとドル額を足さない（ターン・時刻・モデルは数える）。
//
// 行の provider は書いたところで意味が違う: 裏方の処理・記憶の整理は本当の backend の名前（llm_helpers.py の _provider_label →
// providers/acp.py の provider_label。"acp"（kiro-cli）・"claude_code"・"codex"・"kas"・"goose" など）、ダッシュボードとサブエージェントは
// "claude_code"（Claude の backend）か "acp"（それ以外の全部。Codex も。agent_sdk/capabilities.py の provider_seam、
// dashboard/chat_runner.py、subagent_manager/run.py）、ワークフローは "acp" と書いたまま（workflows/agent_exec.py）、
// タスクの実行・予定の実行・Slack・webhook は設定の agent.provider（"acp" しか選べない）。
// なので "acp" の行は、label（session_map の provider か、サブエージェントの state.json の "provider"。わからなければ空）も見る。
// label は会話キーの今の backend で、行の時刻のころの backend とは限らない（Codex で話したあと kiro-cli に切りかえた会話など）。
// そこで label が kiro-cli（空か "acp"）なら、行そのもので決める（crewCodexRow）。label が kiro-cli・Claude Code・Codex のどれでもない
// （goose など）なら数える。タスクの実行と webhook の行は、Claude Code で動いても "acp" で、会話キーが session_map から
// 消えていると見分けられないので、トークンを数える（2 度数えることがある）。
func crewElsewhere(x crewTurn, label string) bool {
	switch {
	case crewOwnHistory(x.provider):
		return true
	case x.provider != "" && x.provider != "acp": // 本当の backend の名前（kas・goose など）
		return false
	case crewOwnHistory(label):
		return true
	case label != "" && label != "acp":
		return false
	}
	return crewCodexRow(x)
}

// crewCodexModel は Codex（codex-acp）が出すモデル ID（"gpt-5.4"・"gpt-5.4-codex"・"o3"、推論の強さつきの "gpt-6-astra[max]"）。
// opencode・pi・goose・deepseek は "<provider>/<model>" や JSON の組を使い（agent_sdk/backends.py）、この形にはならない。
var crewCodexModel = regexp.MustCompile(`^(?:gpt-|o[1-9](?:$|[-\[])|[^/\[{]*codex)[^/]*$`)

// crewCodexRow は、"acp" の行が Codex で動いたターンのものらしいか。Codex の行はトークン（とドル額）で、クレジットはない。
// kiro-cli も GPT のモデル（gpt-5.6-sol など。model_registry.py）を出すが、kiro-cli の行はクレジットで数える（acp/types.py の TurnUsage）ので、
// クレジットのある行は kiro-cli のものとみなす。Crew の行に backend の名前はほかに残らない（usage.py の _build_token_record）ので、モデル ID で見分ける。
// モデル ID は短いので、crewCodexModelMax バイトより長い値（壊れた行やわざと作った行）は Codex の ID とみなさず、正規表現にもかけない。
func crewCodexRow(x crewTurn) bool {
	m := strings.TrimSpace(x.model)
	return x.credits == 0 && (x.u.Total() > 0 || x.cost > 0) && len(m) <= crewCodexModelMax && crewCodexModel.MatchString(strings.ToLower(m))
}

// crewCodexModelMax は Codex のモデル ID として見る長さの上限（バイト）。
const crewCodexModelMax = 200

// crewOwnHistory は、Crew の backend の名前が、kiroku が自分の履歴を別に読むエージェント（Claude Code と Codex）か。
func crewOwnHistory(provider string) bool {
	return provider == "claude_code" || provider == "codex"
}

func sumCredits(cs []core.Credit) float64 {
	v := 0.0
	for _, c := range cs {
		v += c.V
	}
	return v
}

// useCrewCredits は、Crew の記録のほうが多ければ、その会話のクレジットを Crew の記録に置きかえる。置きかえたら true。
func useCrewCredits(s *core.Builder, turns []crewTurn) bool {
	crew := 0.0
	for _, x := range turns {
		crew += x.credits
	}
	if crew <= sumCredits(s.Credits)+1e-9 {
		return false
	}
	s.Credits = nil
	ms := s.Measures[:0]
	for _, m := range s.Measures {
		if m.Key != "credits" && m.Key != "turns" {
			ms = append(ms, m)
		}
	}
	s.Measures = ms
	addCrewTurns(s, turns, "")
	return true
}

// addCrewTurns は Crew の行をターンとして足す。label は会話の session_map の provider（わからなければ空）。
// トークンかドル額のある行（kiro-cli 以外の backend）は使用量としても足す（crewElsewhere の行は除く）。
// ドル額は会話の累計の差分なので、会話のどこかに cost があれば、cost が 0 の行も 0 ドルとして扱う（料金表で見積もると 2 度数えるため）。
// どの行にも cost がなければ、料金表で見積もる（料金表にないモデルは見積もらない）。
// 戻り値は、crewElsewhere でトークンとドル額を足さなかった行の数。
func addCrewTurns(s *core.Builder, turns []crewTurn, label string) (skipped int) {
	hasCost := false
	for _, x := range turns {
		if x.cost > 0 && !crewElsewhere(x, label) {
			hasCost = true
		}
	}
	for _, x := range turns {
		t, st := x.t, x.start
		s.Tick(&st)
		s.Agent(&t)
		if x.credits != 0 {
			s.Credits = append(s.Credits, core.Credit{T: &t, V: x.credits})
		}
		s.Measure("credits", &t, x.credits)
		s.Measure("turns", &t, 1)
		billed := x.u.Total() > 0 || x.cost > 0
		switch {
		case billed && crewElsewhere(x, label):
			skipped++
			s.Model(x.model)
		case billed:
			e := core.Event{T: &t, Model: x.model, U: x.u}
			if hasCost {
				c := x.cost
				e.Cost = &c
			}
			s.AddEvent(e) // モデルも数える
		default:
			s.Model(x.model)
		}
	}
	return skipped
}

// addCrewNative は Crew の行から、Crew だけが記録している数字（コンテキストの使用率・正常に終わらなかったターン）を足す。
// stop_reason が end_turn 以外（cancelled・refusal・max_tokens・"error: …" など）のターンを数える。stop_reason のない行は数えない。
// Crew は何も使わなかったターンの行を書かない（usage_has_billing）ので、そうしたターンは入らない。
func addCrewNative(s *core.Builder, turns []crewTurn) {
	for _, x := range turns {
		t := x.t
		if x.ctx >= 0 {
			s.Measure("context_used", &t, x.ctx)
		}
		if x.stop != "" {
			v := 0.0
			if x.stop != "end_turn" {
				v = 1
			}
			s.Measure("stopped_early", &t, v)
		}
	}
}

// crewOnly は kiro-cli の会話に結びつかない Crew の記録を、Crew のセッションにする。
// 裏方の処理（_bg と、記憶の場所ごとの記憶の整理。crewBackground）は 1 日ごとにまとめる。skipped は、crewElsewhere でトークンとドル額を足さなかった行の数。
// title と rows は、その slot の会話の記録（のうち kiro-cli の会話に結びつかなかった行。readCrewKey）。裏方の処理では使わない。
func crewOnly(home, slot string, turns []crewTurn, info *CrewInfo, title string, rows []crewRow) (out []*core.Builder, skipped int) {
	bg := crewBackground(slot)
	if bg {
		title, rows = "", nil
	}
	groups := map[string][]crewTurn{}
	var order []string
	for _, x := range turns {
		g := ""
		if bg {
			g = time.Unix(int64(x.t), 0).In(time.Local).Format("2006-01-02")
		}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], x)
	}
	label := ""
	if info != nil {
		label = info.Provider
	}
	for _, g := range order {
		id := "crew:" + slot
		if g != "" {
			id += ":" + g
		}
		s := core.NewBuilder("Kiro Crew", id)
		if !bg && home != "" && len(rows) > 0 {
			s.File = crewTranscriptPath(home, slot)
		}
		s.Key = "kiro-crew:" + id[len("crew:"):]
		switch {
		case slot == "_bg":
			s.Title = "Kiro Crew background work"
		case bg:
			s.Title = "Kiro Crew memory consolidation (" + strings.TrimPrefix(slot, "memory-consolidation:") + ")"
		case info != nil && info.Subagent: // kiro-cli の会話が消えたサブエージェント（残さないものは Crew が消す）
			s.Title = subagentTitle(*info)
		case info != nil && info.Title != "":
			s.Title = info.Title
		case title != "":
			s.Title = title
		default:
			s.Title = "Kiro Crew: " + slot
		}
		s.Project = "(Kiro Crew)"
		if info != nil && info.Cwd != "" {
			s.Project = info.Cwd
		}
		skipped += addCrewTurns(s, groups[g], label)
		addCrewNative(s, groups[g])
		addCrewRows(s, rows)
		first := groups[g][0].start
		s.Measure("crew_sessions", &first, 1)
		if info != nil && info.Subagent {
			s.Measure("crew_subagents", &first, 1)
		}
		out = append(out, s)
	}
	return out, skipped
}

// crewRetentionDefault は Crew の session.archive_retention_days の既定値（kiro_crew/config/sections.py の SessionConfig）。
const crewRetentionDefault = 30

// crewRetentionNever は、これより長い日数を「消さない」と同じに扱う境目（約 2700 年。int にしても溢れない）。
const crewRetentionNever = 1_000_000

// configMax は設定ファイル（Crew の config.json、Claude Code の managed-settings.json など）として読む大きさの上限。
// これより大きいファイルは読まない。
const configMax = 8 << 20

// crewRetentionDays は Crew が使う session.archive_retention_days（日数）を読む。never は、Crew が古い記録を消さない設定のとき。
// Crew と同じく <Crew の場所>/config.json に config.local.json を重ねて読む（config.local.json が勝つ。
// "session" がどちらも object ならキーごとに重ね、そうでなければ上書き）。書き方は {"session": {"archive_retention_days": 30}}。
// Crew の読み方（config/sections.py の _archive_retention_days と config/validation.py）に合わせて:
//   - null か負の数は消さない（Crew は -1 にそろえて、片付けをしない）
//   - 0 以上の整数（30.0 のような小数点つきの整数や、"45" のような整数の文字列も）はその日数
//   - キーがない・整数でない数・数でない値・"session" が object でない・ファイルがないか壊れているときは既定の 30 日
func crewRetentionDays(home string) (days int, never bool) {
	v, ok := crewSessionSetting(home, "archive_retention_days")
	if !ok {
		return crewRetentionDefault, false
	}
	var f float64
	switch x := v.(type) {
	case nil:
		return 0, true
	case float64:
		f = x
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return crewRetentionDefault, false
		}
		f = float64(n)
	default: // true / false や object など。Crew は型が違う値を捨てて既定値にする
		return crewRetentionDefault, false
	}
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f):
		return crewRetentionDefault, false
	case f < 0 || f > crewRetentionNever:
		return 0, true
	}
	return int(f), false
}

// crewSessionSetting は config.json に config.local.json を重ねたときの "session" の key の値。ないなら ok が false。
func crewSessionSetting(home, key string) (v any, ok bool) {
	base := configObject(filepath.Join(home, "config.json"))
	local := configObject(filepath.Join(home, "config.local.json"))
	sec := base["session"]
	if l, has := local["session"]; has {
		lm, lok := l.(map[string]any)
		if bm, bok := sec.(map[string]any); lok && bok {
			if v, ok := lm[key]; ok {
				return v, true
			}
			v, ok := bm[key]
			return v, ok
		}
		sec = l
	}
	m, isMap := sec.(map[string]any)
	if !isMap {
		return nil, false
	}
	v, ok = m[key]
	return v, ok
}

// configObject は設定ファイル（JSON の object。Crew の設定や Claude Code の設定）を読む。ない・ふつうのファイルでない・大きすぎる・
// 読めない・壊れている・object でないなら nil（Crew もそのファイルを無視して既定値で動く）。手で編集したファイルの先頭の BOM は、Crew と同じく読み飛ばす。
func configObject(path string) map[string]any {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > configMax {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, configMax+1))
	if err != nil || len(b) > configMax {
		return nil
	}
	var v any
	if json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &v) != nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}
