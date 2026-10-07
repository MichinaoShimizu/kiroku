package source

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/klauspost/compress/zstd"
)

// testdata/codex は Codex CLI の作り（rollout-*.jsonl、session_index.jsonl）に合わせた合成データ。
// 新しい版の圧縮ファイル（.jsonl.zst）はテストの中で作る。
func codexHome(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "codex")
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(filepath.Join(dst, "thr-new.jsonl.src"))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	dir := filepath.Join(dst, "sessions", "2026", "09", "30")
	os.MkdirAll(dir, 0o755)
	out, _ := os.Create(filepath.Join(dir, "rollout-2026-09-30T14-00-00-thr-new.jsonl.zst"))
	enc, _ := zstd.NewWriter(out)
	io.Copy(enc, in)
	enc.Close()
	out.Close()
	return dst
}

func TestCodex(t *testing.T) {
	bs := load(t, &Codex{Home: codexHome(t)})
	if len(bs) != 2 {
		t.Fatalf("セッション数 = %d, want 2（サブエージェントは親にまとめる）", len(bs))
	}
	m := find(bs, "thr-main")
	if m == nil {
		t.Fatal("thr-main がない")
	}
	if m.Title != "ヘッダーのボタンの色" || m.Project != "/Users/me/web" || m.Branch != "feat/button" {
		t.Errorf("title/project/branch = %q %q %q", m.Title, m.Project, m.Branch)
	}
	if len(m.Prompts) != 2 || len(m.FixTS) != 1 {
		t.Errorf("依頼 = %d（環境の説明は数えない）, 言い直し = %d", len(m.Prompts), len(m.FixTS))
	}
	f := m.Finish(15)
	if f.Usage.In != 200+300 || f.Usage.CR != 800+1200 || f.Usage.Out != 200+150 {
		t.Errorf("トークン = %+v（同じ値の書き直しは 1 回、cached は入力から分ける）", f.Usage.Tokens)
	}
	// gpt-5-codex は料金表にないので、先頭一致の gpt-5 の料金（入力 1.25、キャッシュ済み 0.125、出力 10）
	if f.Usage.Unpriced != 0 || f.Usage.Cost != core.Round((500*1.25+2000*0.125+350*10)/1e6, 4) {
		t.Errorf("gpt-5-codex は gpt-5 の料金で見積もる: %+v", f.Usage)
	}
	if f.Models[0][0] != "gpt-5-codex" || f.Resume == nil || *f.Resume != "cd /Users/me/web && codex resume thr-main" {
		t.Errorf("model/resume = %v %v", f.Models, f.Resume)
	}
	if len(f.Subagents) != 1 {
		t.Fatalf("サブエージェント = %d", len(f.Subagents))
	}
	sa := f.Subagents[0]
	if sa.Type != "explorer" || sa.Desc != "ボタンのスタイルがどこで決まっているか調べる" || *sa.Model != "gpt-5-codex-mini" || sa.Tools != 1 {
		t.Errorf("サブエージェント = %+v", sa)
	}
	if sa.Usage.In != 300 || sa.Usage.CR != 100 { // 親の写しの分は数えない
		t.Errorf("サブエージェントのトークン = %+v", sa.Usage.Tokens)
	}
	n := find(bs, "thr-new") // 圧縮ファイル、token_usage_record だけを使う
	if n == nil {
		t.Fatal("圧縮ファイル（.jsonl.zst）が読めていない")
	}
	if fn := n.Finish(15); fn.Usage.In != 2000+100 || fn.Usage.Out != 600 || len(fn.Prompts) != 1 {
		t.Errorf("新しい版のトークン = %+v", fn.Usage.Tokens)
	}
}

func nativeOf(f *core.Session) map[string]core.NativeValue {
	out := map[string]core.NativeValue{}
	for _, v := range f.Native {
		out[v.Label] = v
	}
	return out
}

func TestCodexNativeMetrics(t *testing.T) {
	bs := load(t, &Codex{Home: codexHome(t)})
	n := nativeOf(find(bs, "thr-main").Finish(15))
	// 親の 2 回 + サブエージェントの 1 回（同じ値の書き直しは数えない）
	if n["応答の数"].V != 3 || n["ツール呼び出し"].V != 3 {
		t.Errorf("応答/ツール = %+v %+v", n["応答の数"], n["ツール呼び出し"])
	}
	// Codex の表示と同じく、いつも入っている 12000 トークンを除いて数える。1650 トークンでは 0%
	if v := n["コンテキストの最大使用率"]; v.N == 0 || v.V != 0 {
		t.Errorf("コンテキストの使用率 = %+v", v)
	}
	if _, ok := n["レート制限の最大使用率"]; ok {
		t.Error("記録がなければ出さない")
	}
}

// writeCodex は sessions/2026/10/06 に rollout ファイルを 1 つ書く（1 行に 1 つの JSON）。
func writeCodex(t *testing.T, home, id string, lines ...string) {
	t.Helper()
	dir := filepath.Join(home, "sessions", "2026", "10", "06")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-10-06T00-00-00-"+id+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// paginated の版は、発言を item_completed の TurnItem で書く。TurnItem は #[serde(tag = "type")] で
// 名前を変えていないので type は UserMessage / AgentMessage。UserMessage の content は UserInput
// （{"type":"text","text":…} のほか画像など）、AgentMessage の content は {"type":"Text","text":…}。
func TestCodexPaginatedUserMessage(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-paginated",
		`{"timestamp":"2026-10-06T00:00:00.000Z","ordinal":0,"type":"session_meta","payload":{"id":"thr-paginated","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/project","history_mode":"paginated"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","ordinal":1,"type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1","model_context_window":null}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","ordinal":2,"type":"event_msg","payload":{"type":"item_completed","thread_id":"thr-paginated","turn_id":"turn-1","item":{"type":"UserMessage","id":"u1","content":[{"type":"text","text":"Fix the parser","text_elements":[]},{"type":"local_image","path":"/Users/me/shot.png"}]},"completed_at_ms":0}}`,
		`{"timestamp":"2026-10-06T00:00:02.000Z","ordinal":3,"type":"turn_context","payload":{"model":"gpt-6-sol"}}`,
		`{"timestamp":"2026-10-06T00:00:09.000Z","ordinal":4,"type":"event_msg","payload":{"type":"item_completed","thread_id":"thr-paginated","turn_id":"turn-1","item":{"type":"AgentMessage","id":"a1","phase":"final_answer","content":[{"type":"Text","text":"Fixed it"}]},"completed_at_ms":0}}`,
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	if len(s.Prompts) != 1 || s.Prompts[0].Text != "Fix the parser" {
		t.Fatalf("paginated user prompt = %+v（画像は文に入れない）", s.Prompts)
	}
	if r := s.Prompts[0].Reply; r == nil || r.Text != "Fixed it" {
		t.Errorf("paginated reply = %+v", r)
	}
}

// cache_write_input_tokens は cached_input_tokens と同じく input_tokens の内訳なので、入力から引く。
func TestCodexCacheWriteTokens(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-cw",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-cw","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","type":"turn_context","payload":{"model":"gpt-5.2-codex"}}`,
		`{"timestamp":"2026-10-06T00:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"テストを足して"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"token_usage_record","payload":{"thread_id":"thr-cw","response_id":"r1","usage":{"input_tokens":1000,"cached_input_tokens":600,"cache_write_input_tokens":300,"output_tokens":50,"reasoning_output_tokens":0,"total_tokens":1050}}}`,
		`{"timestamp":"2026-10-06T00:00:40.000Z","type":"token_usage_record","payload":{"thread_id":"thr-cw","response_id":"r2","usage":{"input_tokens":100,"cached_input_tokens":80,"cache_write_input_tokens":40,"output_tokens":10,"reasoning_output_tokens":0,"total_tokens":110}}}`,
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	u := bs[0].Finish(15).Usage.Tokens
	// 2 回目は cached + cache_write が input を超える（壊れた値）ので、入力は 0 に止める
	if u.In != 100 || u.CR != 600+80 || u.CW != 300+40 || u.Out != 60 {
		t.Errorf("トークン = %+v, want in 100 cr 680 cw 340 out 60", u)
	}
}

// OpenAI の料金: In はキャッシュなしの入力、CR はキャッシュ済み入力、CW はキャッシュ書き込み。
// 1 回の応答（token_usage_record・last_token_usage）の入力が 272K を超えたら、その応答は長いコンテキストの料金。
// 古い形（前回の合計との差）は何回分かの合計なので、短いコンテキストの料金のまま。
func TestCodexOpenAIPrices(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-price",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-price","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","type":"turn_context","payload":{"model":"gpt-6-sol"}}`,
		`{"timestamp":"2026-10-06T00:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"token_usage_record","payload":{"thread_id":"thr-price","response_id":"r1","usage":{"input_tokens":1000000,"cached_input_tokens":600000,"cache_write_input_tokens":300000,"output_tokens":100000,"total_tokens":1100000}}}`,
		`{"timestamp":"2026-10-06T00:00:40.000Z","type":"token_usage_record","payload":{"thread_id":"thr-price","response_id":"r2","usage":{"input_tokens":200000,"cached_input_tokens":100000,"output_tokens":10000,"total_tokens":210000}}}`,
	)
	writeCodex(t, home, "thr-old",
		`{"timestamp":"2026-10-06T01:00:00.000Z","type":"session_meta","payload":{"id":"thr-old","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T01:00:01.000Z","type":"turn_context","payload":{"model":"gpt-5.5"}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T01:00:10.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100000,"output_tokens":1000,"total_tokens":101000}}}}`,
		`{"timestamp":"2026-10-06T01:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":500000,"output_tokens":2000,"total_tokens":502000}}}}`,
		`{"timestamp":"2026-10-06T01:00:30.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":800000,"output_tokens":3000,"total_tokens":803000},"last_token_usage":{"input_tokens":300000,"output_tokens":1000,"total_tokens":301000}}}}`,
	)
	bs := load(t, &Codex{Home: home})
	cost := map[string]float64{}
	for _, b := range bs {
		cost[b.ID] = b.Finish(15).Usage.Cost
	}
	// r1: 入力 1M（> 272K）なので長いコンテキスト: 0.1M*4 + 0.3M*5 + 0.6M*0.4 + 0.1M*15
	// r2: 入力 200K: 0.1M*2 + 0.1M*0.2 + 0.01M*10
	if want := core.Round(0.4+1.5+0.24+1.5+0.2+0.02+0.1, 4); cost["thr-price"] != want {
		t.Errorf("thr-price cost = %v, want %v", cost["thr-price"], want)
	}
	// 1 回目 100K（合計のまま）・2 回目 400K（差。何回分か分からないので短い料金）・3 回目 300K（last_token_usage なので長い料金）
	want := (100000*5+1000*30)/1e6 + (400000*5+1000*30)/1e6 + (300000*10+1000*45)/1e6
	if cost["thr-old"] != core.Round(want, 4) {
		t.Errorf("thr-old cost = %v, want %v", cost["thr-old"], core.Round(want, 4))
	}
}

// paginated のサブエージェントは、親の履歴を自分の session_meta のあとに写す（時刻は写したとき）。
// subagent_history_start_ordinal より前の ordinal の行は写しなので数えない。
func TestCodexSubagentHistoryStartOrdinal(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-p",
		`{"timestamp":"2026-10-06T01:00:00.000Z","ordinal":0,"type":"session_meta","payload":{"id":"thr-p","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web","history_mode":"paginated"}}`,
		`{"timestamp":"2026-10-06T01:00:01.000Z","ordinal":1,"type":"turn_context","payload":{"model":"gpt-5.2-codex"}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","ordinal":2,"type":"event_msg","payload":{"type":"item_completed","thread_id":"thr-p","turn_id":"t1","item":{"type":"UserMessage","id":"u1","content":[{"type":"text","text":"親の依頼"}]}}}`,
		`{"timestamp":"2026-10-06T01:00:20.000Z","ordinal":3,"type":"token_usage_record","payload":{"thread_id":"thr-p","response_id":"p1","usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100}}}`,
	)
	writeCodex(t, home, "thr-c",
		`{"timestamp":"2026-10-06T01:01:00.000Z","ordinal":0,"type":"session_meta","payload":{"id":"thr-c","timestamp":"2026-10-06T01:01:00.000Z","cwd":"/Users/me/web","history_mode":"paginated","subagent_history_start_ordinal":5,"source":{"subagent":{"thread_spawn":{"parent_thread_id":"thr-p","depth":1,"agent_role":"explorer"}}}}}`,
		// 1〜4 は親から写した行（時刻は子の session_meta より後）
		`{"timestamp":"2026-10-06T01:01:00.100Z","ordinal":1,"type":"session_meta","payload":{"id":"thr-p","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web","history_mode":"paginated"}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","ordinal":2,"type":"turn_context","payload":{"model":"gpt-5.2-codex"}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","ordinal":3,"type":"event_msg","payload":{"type":"item_completed","thread_id":"thr-p","turn_id":"t1","item":{"type":"UserMessage","id":"u1","content":[{"type":"text","text":"親の依頼"}]}}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","ordinal":4,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"model_context_window":272000}}}`,
		// 5 からが子の行
		`{"timestamp":"2026-10-06T01:01:00.200Z","ordinal":5,"type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-c","thread_settings":{"model":"gpt-5.2-codex-mini"}}}`,
		`{"timestamp":"2026-10-06T01:01:01.000Z","ordinal":6,"type":"turn_context","payload":{"model":"gpt-5.2-codex-mini"}}`,
		`{"timestamp":"2026-10-06T01:01:02.000Z","ordinal":7,"type":"event_msg","payload":{"type":"item_completed","thread_id":"thr-c","turn_id":"t2","item":{"type":"UserMessage","id":"u2","content":[{"type":"text","text":"子の依頼"}]}}}`,
		`{"timestamp":"2026-10-06T01:01:30.000Z","ordinal":8,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1300,"cached_input_tokens":0,"output_tokens":120,"total_tokens":1420},"last_token_usage":{"input_tokens":300,"cached_input_tokens":0,"output_tokens":20,"total_tokens":320},"model_context_window":272000}}}`,
	)
	checkCodexChild(t, home, "thr-p", "子の依頼", 300, 20)
}

// フォークや古い形（legacy）のサブエージェントは、写しの直後に自分のスレッド ID の thread_settings_applied を書く。
// それより前の行は写しなので数えない（写した親の thread_settings_applied は親のスレッド ID のまま）。
func TestCodexForkSettingsMarker(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-p",
		`{"timestamp":"2026-10-06T01:00:00.000Z","type":"session_meta","payload":{"id":"thr-p","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T01:00:00.100Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-p","thread_settings":{"model":"gpt-5.2-codex"}}}`,
		`{"timestamp":"2026-10-06T01:00:01.000Z","type":"turn_context","payload":{"model":"gpt-5.2-codex"}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		`{"timestamp":"2026-10-06T01:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"model_context_window":272000}}}`,
	)
	writeCodex(t, home, "thr-c",
		`{"timestamp":"2026-10-06T01:01:00.000Z","type":"session_meta","payload":{"id":"thr-c","timestamp":"2026-10-06T01:01:00.000Z","cwd":"/Users/me/web","source":{"subagent":{"thread_spawn":{"parent_thread_id":"thr-p","depth":1,"agent_role":"explorer"}}}}}`,
		// 親から写した行（時刻は子の session_meta より後）
		`{"timestamp":"2026-10-06T01:01:00.100Z","type":"session_meta","payload":{"id":"thr-p","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-p","thread_settings":{"model":"gpt-5.2-codex"}}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","type":"turn_context","payload":{"model":"gpt-5.2-codex"}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		`{"timestamp":"2026-10-06T01:01:00.100Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"model_context_window":272000}}}`,
		// 子の行
		`{"timestamp":"2026-10-06T01:01:00.100Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-c","thread_settings":{"model":"gpt-5.2-codex-mini"}}}`,
		`{"timestamp":"2026-10-06T01:01:01.000Z","type":"turn_context","payload":{"model":"gpt-5.2-codex-mini"}}`,
		`{"timestamp":"2026-10-06T01:01:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"子の依頼"}}`,
		`{"timestamp":"2026-10-06T01:01:30.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1300,"cached_input_tokens":0,"output_tokens":120,"total_tokens":1420},"last_token_usage":{"input_tokens":300,"cached_input_tokens":0,"output_tokens":20,"total_tokens":320},"model_context_window":272000}}}`,
	)
	// /fork したスレッド（親ではなく forked_from_id）は別のセッションで、写した依頼とトークンは数えない
	writeCodex(t, home, "thr-f",
		`{"timestamp":"2026-10-06T02:00:00.000Z","type":"session_meta","payload":{"id":"thr-f","forked_from_id":"thr-p","timestamp":"2026-10-06T02:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T02:00:00.100Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		`{"timestamp":"2026-10-06T02:00:00.100Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"model_context_window":272000}}}`,
		`{"timestamp":"2026-10-06T02:00:00.100Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-f","thread_settings":{"model":"gpt-5.2-codex"}}}`,
		`{"timestamp":"2026-10-06T02:00:05.000Z","type":"event_msg","payload":{"type":"user_message","message":"別の案を試す"}}`,
		`{"timestamp":"2026-10-06T02:00:30.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1500,"cached_input_tokens":0,"output_tokens":130,"total_tokens":1630},"last_token_usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":30,"total_tokens":530},"model_context_window":272000}}}`,
	)
	checkCodexChild(t, home, "thr-p", "子の依頼", 300, 20)
	f := find(load(t, &Codex{Home: home}), "thr-f")
	if f == nil {
		t.Fatal("thr-f がない")
	}
	fs := f.Finish(15)
	if len(fs.Prompts) != 1 || fs.Prompts[0].Text != "別の案を試す" || fs.Usage.In != 500 || fs.Usage.Out != 30 {
		t.Errorf("フォーク: 依頼 = %+v, トークン = %+v（写した分は数えない）", fs.Prompts, fs.Usage.Tokens)
	}
}

// 印のない古い版で作ったサブエージェントのファイルを新しい版で再開すると、自分の行のあとに
// 自分のスレッド ID の thread_settings_applied を書く。これは写しの終わりではないので、それより前の自分の行も数える
// （写しは古い版の作りどおり、session_meta の時刻で分ける）。
func TestCodexResumedLegacySubagent(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-p",
		`{"timestamp":"2026-10-06T01:00:00.000Z","type":"session_meta","payload":{"id":"thr-p","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		`{"timestamp":"2026-10-06T01:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"model_context_window":272000}}}`,
	)
	writeCodex(t, home, "thr-c",
		`{"timestamp":"2026-10-06T01:01:00.000Z","type":"session_meta","payload":{"id":"thr-c","timestamp":"2026-10-06T01:01:00.000Z","cwd":"/Users/me/web","source":{"subagent":{"thread_spawn":{"parent_thread_id":"thr-p","depth":1,"agent_role":"explorer"}}}}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		`{"timestamp":"2026-10-06T01:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100,"total_tokens":1100},"model_context_window":272000}}}`,
		`{"timestamp":"2026-10-06T01:01:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"子の依頼"}}`,
		`{"timestamp":"2026-10-06T01:01:30.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1300,"cached_input_tokens":0,"output_tokens":120,"total_tokens":1420},"last_token_usage":{"input_tokens":300,"cached_input_tokens":0,"output_tokens":20,"total_tokens":320},"model_context_window":272000}}}`,
		// 翌日、新しい版で再開した
		`{"timestamp":"2026-10-07T09:00:00.000Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-c","thread_settings":{"model":"gpt-5.2-codex-mini"}}}`,
		`{"timestamp":"2026-10-07T09:00:10.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1400,"cached_input_tokens":0,"output_tokens":125,"total_tokens":1525},"last_token_usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":5,"total_tokens":105},"model_context_window":272000}}}`,
	)
	checkCodexChild(t, home, "thr-p", "子の依頼", 300+100, 20+5)
}

// checkCodexChild は、親のセッションの依頼とトークンが親の分だけで、サブエージェントには子の分だけが入っていることを確かめる。
func checkCodexChild(t *testing.T, home, parent, desc string, in, out float64) {
	t.Helper()
	p := find(load(t, &Codex{Home: home}), parent)
	if p == nil {
		t.Fatalf("%s がない", parent)
	}
	s := p.Finish(15)
	if len(s.Prompts) != 1 || s.Prompts[0].Text != "親の依頼" {
		t.Errorf("親の依頼 = %+v", s.Prompts)
	}
	if len(s.Subagents) != 1 {
		t.Fatalf("サブエージェント = %d", len(s.Subagents))
	}
	sa := s.Subagents[0]
	if sa.Desc != desc || sa.Usage.In != in || sa.Usage.Out != out {
		t.Errorf("サブエージェント = %q %+v, want %q in %v out %v（親から写した分は数えない）", sa.Desc, sa.Usage.Tokens, desc, in, out)
	}
	if s.Usage.In != 1000 || s.Usage.Out != 100 {
		t.Errorf("親のトークン = %+v（親の分だけ。サブエージェントの分は Subagents に入る）", s.Usage.Tokens)
	}
	if sa.Start == nil || *sa.Start < 1e9 {
		t.Errorf("サブエージェントの開始 = %v", sa.Start)
	}
}

// Units は、サブエージェントのファイルを親と同じまとまりにし、スレッド名を Tag に入れる。
// 見張るのは会話が入っている場所だけ（ログなどは見ない）。
func TestCodexUnits(t *testing.T) {
	c := &Codex{Home: codexHome(t)}
	us := c.Units()
	if len(us) != 2 {
		t.Fatalf("まとまり = %d, want 2（thr-main と thr-sub は一緒）", len(us))
	}
	if len(us[0].Files) != 2 || us[0].Tag != "ヘッダーのボタンの色" {
		t.Errorf("thr-main のまとまり = %+v", us[0])
	}
	if len(us[1].Files) != 1 || !strings.HasSuffix(us[1].Key, ".zst") {
		t.Errorf("thr-new のまとまり = %+v", us[1])
	}
	for _, p := range WatchPaths(c) {
		if p == c.Home || strings.HasPrefix(filepath.Base(p), "log") {
			t.Errorf("見張る場所に %s が入っている", p)
		}
	}
	// 親のファイルがなければ、子が自分のまとまりになる
	os.Remove(us[0].Files[0])
	us = c.Units()
	if len(us) != 2 || len(us[0].Files) != 1 || !strings.Contains(us[0].Key, "thr-sub") {
		t.Errorf("親がないとき = %+v", us)
	}
}

// Codex の応答: event_msg の agent_message と、新しい形（item_completed）・古い形（response_item）の
// どれからも、1 つの依頼につき最後の文を拾う。
func TestCodexReplies(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "09", "30")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"timestamp":"2026-09-30T01:00:00.000Z","type":"session_meta","payload":{"id":"thr-r","timestamp":"2026-09-30T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-09-30T01:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"ボタンの色を直して"}}`,
		`{"timestamp":"2026-09-30T01:00:20.000Z","type":"event_msg","payload":{"type":"agent_message","message":"見てみます"}}`,
		`{"timestamp":"2026-09-30T01:00:40.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"直しました"}]}}`,
		// developer は Codex が足す指示で、応答ではない
		`{"timestamp":"2026-09-30T01:00:50.000Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions instructions>sandbox</permissions instructions>"}]}}`,
		`{"timestamp":"2026-09-30T01:01:00.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","id":"u2","content":[{"type":"text","text":"テストも足して","text_elements":[]}]}}}`,
		`{"timestamp":"2026-09-30T01:01:30.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"AgentMessage","id":"a2","content":[{"type":"Text","text":"足して通しました"}]}}}`,
		`{"timestamp":"2026-09-30T01:01:40.000Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<collaboration_mode>default</collaboration_mode>"}]}}`,
	}
	os.WriteFile(filepath.Join(dir, "rollout-2026-09-30T10-00-00-thr-r.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	if len(s.Prompts) != 2 {
		t.Fatalf("依頼 = %d, want 2", len(s.Prompts))
	}
	if r := s.Prompts[0].Reply; r == nil || r.Text != "直しました" {
		t.Errorf("1 件目の応答 = %+v, want 直しました（同じターンの前の文は残さない）", r)
	}
	if r := s.Prompts[1].Reply; r == nil || r.Text != "足して通しました" {
		t.Errorf("2 件目の応答 = %+v", r)
	}
}

// nativeEn は参考指標を英語のラベルで引けるようにする。
func nativeEn(f *core.Session) map[string]core.NativeValue {
	out := map[string]core.NativeValue{}
	for _, v := range f.Native {
		out[v.LabelEn] = v
	}
	return out
}

// 利用上限に当たったことは 2 か所に残る。token_count.rate_limits.rate_limit_reached_type（429 の usage_limit_reached）と、
// 止まったターンの task_complete.error.codex_error_info（usage_limit_exceeded・rate_limit_exceeded）。
// 会話が長すぎる（context_window_exceeded）や Codex 自身の予算（session_budget_exceeded）は数えない。
// サブエージェントが当たったものは親のセッションに入れる。
func TestCodexLimitHits(t *testing.T) {
	home := t.TempDir()
	reached := `"rate_limits":{"limit_id":"codex","primary":{"used_percent":100,"window_minutes":300,"resets_at":1791262800},"secondary":null,"credits":null,"plan_type":"plus","rate_limit_reached_type":"rate_limit_reached"}`
	writeCodex(t, home, "thr-lim",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-lim","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:01:00.000Z","type":"event_msg","payload":{"type":"token_count","info":null,`+reached+`}}`,
		`{"timestamp":"2026-10-06T00:01:01.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"error":{"message":"You've hit your usage limit.","codex_error_info":"usage_limit_exceeded"}}}`,
		// 同じスナップショットの書き直し（1 分より後でも数えない）
		`{"timestamp":"2026-10-06T00:03:00.000Z","type":"event_msg","payload":{"type":"token_count","info":null,`+reached+`}}`,
		`{"timestamp":"2026-10-06T00:10:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t2","error":{"message":"too long","codex_error_info":"context_window_exceeded"}}}`,
		`{"timestamp":"2026-10-06T00:12:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t3","error":{"message":"budget","codex_error_info":"session_budget_exceeded"}}}`,
		`{"timestamp":"2026-10-06T00:14:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t4","error":{"message":"retry","codex_error_info":{"response_too_many_failed_attempts":{"http_status_code":503}}}}}`,
		`{"timestamp":"2026-10-06T00:20:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t5","error":{"message":"Rate limit reached","codex_error_info":"rate_limit_exceeded"}}}`,
	)
	writeCodex(t, home, "thr-lim-sub",
		`{"timestamp":"2026-10-06T00:04:00.000Z","type":"session_meta","payload":{"id":"thr-lim-sub","timestamp":"2026-10-06T00:04:00.000Z","cwd":"/Users/me/web","source":{"subagent":{"thread_spawn":{"parent_thread_id":"thr-lim","agent_role":"explorer"}}}}}`,
		`{"timestamp":"2026-10-06T00:05:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"s1","error":{"message":"limit","codex_error_info":"usage_limit_exceeded"}}}`,
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	got := bs[0].Finish(15).Limits
	base := float64(1791244800) // 2026-10-06T00:00:00Z
	want := []float64{base + 60, base + 300, base + 1200}
	if len(got) != len(want) {
		t.Fatalf("limits = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("limits = %v, want %v", got, want)
			break
		}
	}
}

// turn_aborted の reason が interrupted のときだけ、人が止めたとして数える。
func TestCodexInterrupted(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-int",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-int","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:30.000Z","type":"event_msg","payload":{"type":"turn_aborted","turn_id":"t1","reason":"interrupted","duration_ms":20000}}`,
		`{"timestamp":"2026-10-06T00:01:00.000Z","type":"event_msg","payload":{"type":"turn_aborted","turn_id":"t2","reason":"replaced"}}`,
		`{"timestamp":"2026-10-06T00:02:00.000Z","type":"event_msg","payload":{"type":"turn_aborted","turn_id":"t3","reason":"review_ended"}}`,
		`{"timestamp":"2026-10-06T00:03:00.000Z","type":"event_msg","payload":{"type":"turn_aborted","reason":"budget_limited"}}`,
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	if s.Interrupts != 1 || len(s.InterruptsAt) != 1 || s.InterruptsAt[0] != 1791244800+30 {
		t.Errorf("interrupts = %d %v, want 1 at +30s", s.Interrupts, s.InterruptsAt)
	}
	if len(s.Prompts) != 1 {
		t.Errorf("prompts = %d, want 1（中断は依頼に数えない）", len(s.Prompts))
	}
}

// rate_limits の primary と secondary を、枠の長さ（window_minutes）ごとの指標にする。
// limit_id が codex 以外のもの（モデルごとの別の枠）は使わない。window_minutes がなければ primary / secondary の名前で出す。
func TestCodexRateLimitWindows(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-rl",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-rl","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"limit_id":"codex","primary":{"used_percent":40,"window_minutes":300},"secondary":{"used_percent":12,"window_minutes":10080}}}}`,
		`{"timestamp":"2026-10-06T00:00:30.000Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"limit_id":"codex_other","primary":{"used_percent":95,"window_minutes":300},"secondary":{"used_percent":90,"window_minutes":10080}}}}`,
		// 古い版は limit_id を書かない
		`{"timestamp":"2026-10-06T00:00:40.000Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"primary":{"used_percent":55,"window_minutes":299},"secondary":{"used_percent":20}}}}`,
		`{"timestamp":"2026-10-06T00:00:50.000Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"primary":{"used_percent":7}}}}`,
	)
	bs := load(t, &Codex{Home: home})
	n := nativeEn(bs[0].Finish(15))
	for label, want := range map[string]float64{
		"Peak 5-hour limit usage":         55,
		"Peak weekly limit usage":         12,
		"Peak rate-limit usage":           7,
		"Peak secondary rate-limit usage": 20,
	} {
		if v, ok := n[label]; !ok || v.V != want {
			t.Errorf("%s = %+v, want %v", label, v, want)
		}
	}
	if _, ok := n["Peak daily limit usage"]; ok {
		t.Error("記録のない枠は出さない")
	}
}

// task_complete（turn_complete とも書く）の time_to_first_token_ms と duration_ms から、ターンの時間の中央値を出す。
// エラーで止まったターンは数えない。古い版は値がないので出さない。
func TestCodexTurnTiming(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-time",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-time","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":"ok","started_at":1791244810,"completed_at":1791244820,"duration_ms":10000,"time_to_first_token_ms":1500}}`,
		`{"timestamp":"2026-10-06T00:01:00.000Z","type":"event_msg","payload":{"type":"turn_complete","turn_id":"t2","duration_ms":20000,"time_to_first_token_ms":2500}}`,
		`{"timestamp":"2026-10-06T00:02:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t3","duration_ms":30000,"time_to_first_token_ms":3500}}`,
		`{"timestamp":"2026-10-06T00:03:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t4","duration_ms":100,"error":{"message":"bad","codex_error_info":"bad_request"}}}`,
		`{"timestamp":"2026-10-06T00:04:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t5","last_agent_message":"old"}}`,
	)
	bs := load(t, &Codex{Home: home})
	n := nativeEn(bs[0].Finish(15))
	if v := n["Time to first token (median)"]; v.V != 2.5 || v.N != 3 {
		t.Errorf("ttft = %+v, want 2.5s from 3", v)
	}
	if v := n["Turn duration (median)"]; v.V != 20 || v.N != 3 {
		t.Errorf("duration = %+v, want 20s from 3", v)
	}

	old := t.TempDir()
	writeCodex(t, old, "thr-old",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-old","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":"ok"}}`,
	)
	if n := nativeEn(load(t, &Codex{Home: old})[0].Finish(15)); len(n) != 0 {
		t.Errorf("値のない版では出さない: %+v", n)
	}
}

// コンテキストの使用率は Codex の表示と同じく (last.total_tokens - 12000) / (上限 - 12000)。
// あふれたあとに Codex が書く token_count（total が上限と同じで、ほかは 0）は 100% とし、応答には数えない。
func TestCodexContextUsage(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-ctx",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-ctx","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":140000,"cached_input_tokens":0,"output_tokens":2000,"reasoning_output_tokens":0,"total_tokens":142000},"last_token_usage":{"input_tokens":140000,"cached_input_tokens":0,"output_tokens":2000,"reasoning_output_tokens":0,"total_tokens":142000},"model_context_window":272000},"rate_limits":null}}`,
	)
	n := nativeEn(load(t, &Codex{Home: home})[0].Finish(15))
	if v := n["Peak context usage"]; v.V != 50 { // (142000 - 12000) / (272000 - 12000)
		t.Errorf("context = %+v, want 50%%", v)
	}
	if w := load(t, &Codex{Home: home})[0].Finish(15).CtxWindow; w != 272000 {
		t.Errorf("ctxWindow = %v, want 272000（model_context_window。「長い会話」の目安に使う）", w)
	}

	full := t.TempDir()
	writeCodex(t, full, "thr-full",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-full","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":140000,"cached_input_tokens":0,"output_tokens":2000,"reasoning_output_tokens":0,"total_tokens":142000},"last_token_usage":{"input_tokens":140000,"cached_input_tokens":0,"output_tokens":2000,"reasoning_output_tokens":0,"total_tokens":142000},"model_context_window":272000},"rate_limits":null}}`,
		// fill_to_context_window: total は上限だけ、last は上限までの残りだけ
		`{"timestamp":"2026-10-06T00:00:30.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":272000},"last_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":130000},"model_context_window":272000},"rate_limits":null}}`,
		`{"timestamp":"2026-10-06T00:00:31.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","error":{"message":"too long","codex_error_info":"context_window_exceeded"}}}`,
	)
	f := load(t, &Codex{Home: full})[0].Finish(15)
	n = nativeEn(f)
	if v := n["Peak context usage"]; v.V != 100 {
		t.Errorf("context after overflow = %+v, want 100%%", v)
	}
	if v := n["Responses"]; v.V != 1 {
		t.Errorf("responses = %+v, want 1（満杯の印は応答ではない）", v)
	}
	if f.Usage.In != 140000 || f.Usage.Out != 2000 {
		t.Errorf("tokens = %+v", f.Usage.Tokens)
	}
	if len(f.Limits) != 0 {
		t.Errorf("limits = %v（会話が長すぎるのは利用上限ではない）", f.Limits)
	}
}

// Codex はファイルを apply_patch で変える。引数はパッチの文なので、その中のファイル名を編集したファイルにする。
// 自由形式のツール（custom_tool_call の input）、関数（arguments の input）、シェルから（argv とヒアドキュメント）の形がある。
func TestCodexApplyPatchFiles(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-patch",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-patch","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:00:11.000Z","type":"response_item","payload":{"type":"custom_tool_call","call_id":"c1","name":"apply_patch","input":"*** Begin Patch\n*** Update File: src/a.go\n@@\n-x\n+y\n*** Move to: src/b.go\n*** Add File: docs/new.md\n+hi\n*** End Patch"}}`,
		`{"timestamp":"2026-10-06T00:00:12.000Z","type":"response_item","payload":{"type":"function_call","call_id":"c2","name":"apply_patch","arguments":"{\"input\":\"*** Begin Patch\\n*** Delete File: old.txt\\n*** End Patch\"}"}}`,
		`{"timestamp":"2026-10-06T00:00:13.000Z","type":"response_item","payload":{"type":"function_call","call_id":"c3","name":"shell","arguments":"{\"command\":[\"bash\",\"-lc\",\"apply_patch <<'EOF'\\n*** Begin Patch\\n*** Update File: README.md\\n@@\\n-a\\n+b\\n*** End Patch\\nEOF\"]}"}}`,
		`{"timestamp":"2026-10-06T00:00:14.000Z","type":"response_item","payload":{"type":"function_call","call_id":"c4","name":"shell","arguments":"{\"command\":[\"apply_patch\",\"*** Begin Patch\\n*** Add File: x.txt\\n+1\\n*** End Patch\"]}"}}`,
		// パッチでないもの（apply_patch の名前を探すだけのコマンド、Begin Patch のない文）は数えない
		`{"timestamp":"2026-10-06T00:00:15.000Z","type":"response_item","payload":{"type":"function_call","call_id":"c5","name":"shell","arguments":"{\"command\":[\"bash\",\"-lc\",\"rg 'Update File: nope.go' apply_patch\"]}"}}`,
		`{"timestamp":"2026-10-06T00:00:16.000Z","type":"response_item","payload":{"type":"custom_tool_call","call_id":"c6","name":"other","input":"*** Begin Patch\n*** Add File: nope2.go\n*** End Patch"}}`,
	)
	f := load(t, &Codex{Home: home})[0].Finish(15)
	want := []string{"README.md", "docs/new.md", "old.txt", "src/a.go", "src/b.go", "x.txt"}
	if strings.Join(f.Files, ",") != strings.Join(want, ",") || f.NFiles != len(want) {
		t.Errorf("files = %v (%d), want %v", f.Files, f.NFiles, want)
	}
}
