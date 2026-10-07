package source

import (
	"bytes"
	"encoding/json"
)

// topFields は JSON のオブジェクト b から、keys の項目の値だけを取り出す（json.Unmarshal で map[string]any に読んだときと同じ値）。
// ほかの項目の値は組み立てずに読み飛ばすので、大きな値（Kiro IDE の実行ファイルの会話の全文など）があっても速い。
// 読み飛ばす値は括弧と文字列の終わりを追うだけで、中身が JSON として正しいかは確かめない。
// b がオブジェクトでないか、形が崩れていたら（取り出す値が JSON として読めないときも）ok = false。
// 同じ項目が 2 度あれば、json.Unmarshal と同じく後のものを使う。
func topFields(b []byte, keys []string) (out map[string]any, ok bool) {
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	out = map[string]any{}
	i := skipSpace(b, 0)
	if i >= len(b) || b[i] != '{' {
		return nil, false
	}
	i = skipSpace(b, i+1)
	if i < len(b) && b[i] == '}' {
		return out, skipSpace(b, i+1) == len(b)
	}
	for {
		if i >= len(b) || b[i] != '"' {
			return nil, false
		}
		end := skipString(b, i)
		if end < 0 {
			return nil, false
		}
		var key string
		if json.Unmarshal(b[i:end], &key) != nil {
			return nil, false
		}
		i = skipSpace(b, end)
		if i >= len(b) || b[i] != ':' {
			return nil, false
		}
		i = skipSpace(b, i+1)
		end = skipValue(b, i)
		if end < 0 {
			return nil, false
		}
		if want[key] {
			var v any
			if json.Unmarshal(b[i:end], &v) != nil {
				return nil, false
			}
			out[key] = v
		}
		i = skipSpace(b, end)
		if i >= len(b) {
			return nil, false
		}
		switch b[i] {
		case ',':
			i = skipSpace(b, i+1)
		case '}':
			return out, skipSpace(b, i+1) == len(b)
		default:
			return nil, false
		}
	}
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i
}

// skipString は b[i] の " から始まる文字列の終わりの次の位置。閉じていなければ -1。
func skipString(b []byte, i int) int {
	i++
	for {
		j := bytes.IndexByte(b[i:], '"')
		if j < 0 {
			return -1
		}
		i += j
		n := 0 // 直前に続く \ の数。奇数なら、この " は文字列の中のもの
		for k := i - 1; k >= 0 && b[k] == '\\'; k-- {
			n++
		}
		i++
		if n%2 == 0 {
			return i
		}
	}
}

// skipValue は b[i] から始まる値の終わりの次の位置。形が崩れていたら -1。
func skipValue(b []byte, i int) int {
	if i >= len(b) {
		return -1
	}
	switch b[i] {
	case '"':
		return skipString(b, i)
	case '{', '[':
		depth := 0
		for i < len(b) {
			switch b[i] {
			case '"':
				if i = skipString(b, i); i < 0 {
					return -1
				}
				continue
			case '{', '[':
				depth++
			case '}', ']':
				if depth--; depth == 0 {
					return i + 1
				}
			}
			i++
		}
		return -1
	}
	start := i // 数・true・false・null
	for i < len(b) && !endsScalar(b[i]) {
		i++
	}
	if i == start {
		return -1
	}
	return i
}

func endsScalar(c byte) bool {
	return c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
