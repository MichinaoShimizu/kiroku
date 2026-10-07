package source

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var topKeys = []string{"a", "b", "executionId", "context", "x\"y", "é"}

// wantTop は json.Unmarshal で map[string]any に読んだときの、keys の項目。
func wantTop(b []byte, keys []string) (map[string]any, bool) {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m == nil {
		return nil, false
	}
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out, true
}

func TestTopFields(t *testing.T) {
	ok := []string{
		`{}`,
		` { } `,
		`{"a":1}`,
		`{"a":"x","b":[1,{"c":"]}"}],"skip":{"d":"\"}\\","e":[[[]]]},"executionId":"e-1"}`,
		`{"skip":"a\\\\","a":true,"b":null}`,
		`{"a":1,"a":2}`, // 後のものを使う（json.Unmarshal と同じ）
		`{"A":1,"a":2}`, // 大文字と小文字は区別する
		"{\n\t\"a\" : -1.5e3 ,\r\n\"context\":{\"messages\":[\"long\"]}\n}",
		`{"x\"y":1,"é":2}`,
		`{"skip":"😀","b":"あ"}`,
	}
	for _, s := range ok {
		got, gotOK := topFields([]byte(s), topKeys)
		want, wantOK := wantTop([]byte(s), topKeys)
		if !gotOK || !wantOK || !reflect.DeepEqual(got, want) {
			t.Errorf("topFields(%s) = %v, %v; want %v, %v", s, got, gotOK, want, wantOK)
		}
	}
	bad := []string{
		``, `null`, `[]`, `"a"`, `1`, `{`, `{"a"}`, `{"a":}`, `{"a":1,}`, `{"a":1 "b":2}`,
		`{"a":"x}`, `{"skip":"x\"}`, `{"skip":[1,2}`, `{"a":1}x`, `{"a":1}{}`, `{"a":tru}`, `{"a":[1,]}`, `{a:1}`,
	}
	for _, s := range bad {
		if got, gotOK := topFields([]byte(s), topKeys); gotOK {
			t.Errorf("topFields(%q) = %v, true; want false", s, got)
		}
	}
}

// 実行ファイルは大きい（会話の全文が入る）ので、使わない項目は組み立てずに読み飛ばす。
func BenchmarkTopFieldsExec(b *testing.B) {
	msg := `{"role":"user","content":"` + strings.Repeat(`text with \"quotes\" and \\n `, 80) + `"}`
	file := []byte(`{"executionId":"e-1","chatSessionId":"s-1","startTime":1760000000000,"endTime":1760000030000,"modelId":"m",` +
		`"usageSummary":[{"usage":0.4,"unit":"credit"}],"context":{"messages":[` + strings.Repeat(msg+",", 100) + msg + `]}}`)
	b.SetBytes(int64(len(file)))
	for b.Loop() {
		if _, ok := topFields(file, kiroExecKeys); !ok {
			b.Fatal("not read")
		}
	}
}

// どんな中身でも止まらず、正しい JSON のオブジェクトなら json.Unmarshal と同じ値を返す。
func FuzzTopFields(f *testing.F) {
	f.Add([]byte(`{"a":"x","b":[1,{"c":"]}"}],"skip":{"d":"\"}\\"},"executionId":"e-1"}`))
	f.Add([]byte(`{"executions":[{"id":1}],"context":{"messages":[]}}`))
	f.Add([]byte(`{"a":"\\","b":"\\\""}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, ok := topFields(data, topKeys)
		want, wantOK := wantTop(data, topKeys)
		if wantOK && (!ok || !reflect.DeepEqual(got, want)) {
			t.Fatalf("topFields(%q) = %v, %v; want %v", data, got, ok, want)
		}
		if ok {
			for k := range got {
				if _, in := want[k]; wantOK && !in {
					t.Fatalf("topFields(%q) has extra key %q", data, k)
				}
			}
		}
	})
}
