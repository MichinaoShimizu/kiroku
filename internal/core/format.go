package core

import (
	"sort"
	"strconv"
	"strings"
)

// Format は、履歴を読んで見つけた形の手がかり（kiroku doctor の「Format check」と「Data sources」に出す）。
// エージェントが履歴の形を変えると、kiroku の知らない記録の種類や読めない行が増え、版も新しくなる。
// 中身は種類の名前（SafeKind で確かめたもの）と数と版（SafeVersion）だけで、パス・プロジェクト名・ID・発言は入れない。
type Format struct {
	Unknown map[string]int // kiroku の知らない記録の種類と、その行の数
	Bad     int            // JSON のオブジェクトとして読めなかった行
	Version string         // 履歴にあった、いちばん新しいエージェントの版
}

// MaxKinds は、1 つのエージェントで名前を出す知らない種類の数。超えた分は OtherKind にまとめる。
const MaxKinds = 20

// OtherKind は、名前を出さない（出せない）知らない種類をまとめる名前。
const OtherKind = "other"

// SafeKind は、履歴から読んだ種類の名前を、そのまま見せてよい形か確かめる（英数字と _.:- の 40 文字まで）。
// それ以外（空・長い・空白や記号・日本語など）は OtherKind にする。名前に発言やパスがまぎれこんでも、外に出さないため。
func SafeKind(name string) string {
	if name == "" || len(name) > 40 {
		return OtherKind
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '_' || c == '.' || c == ':' || c == '-') {
			return OtherKind
		}
	}
	return name
}

// SafeVersion は、履歴から読んだ版を、数字とドットだけの短い形（2.1.300 など）にする。うしろの -alpha.1 や +build は落とす。
// 版の形でないものは "" を返す（使わない）。
func SafeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	if v == "" || len(v) > 20 {
		return ""
	}
	parts := strings.Split(v, ".")
	if len(parts) > 4 {
		return ""
	}
	for _, p := range parts {
		if p == "" || len(p) > 6 {
			return ""
		}
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return ""
			}
		}
	}
	return v
}

// NewerVersion は、a が b より新しい版か（どちらも SafeVersion の形。数字の部分ごとに比べる）。
func NewerVersion(a, b string) bool {
	if b == "" {
		return a != ""
	}
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// Kind は、知らない種類の行を 1 つ数える。名前は SafeKind で確かめ、MaxKinds を超えた分は OtherKind にまとめる。
func (f *Format) Kind(name string) {
	name = SafeKind(name)
	if f.Unknown == nil {
		f.Unknown = map[string]int{}
	}
	if _, ok := f.Unknown[name]; !ok && name != OtherKind && len(f.Unknown) >= MaxKinds {
		name = OtherKind
	}
	f.Unknown[name]++
}

// SawVersion は、履歴にあった版を覚える（いちばん新しいものを残す）。同じ版が続くことが多いので、同じなら比べない。
func (f *Format) SawVersion(v string) {
	if v == "" || v == f.Version {
		return
	}
	if v = SafeVersion(v); v != "" && NewerVersion(v, f.Version) {
		f.Version = v
	}
}

// Merge は o を f に足す。知らない種類は、数の多いものから MaxKinds までを名前で残し、残りを OtherKind にまとめる。
func (f *Format) Merge(o *Format) {
	if o == nil {
		return
	}
	f.Bad += o.Bad
	if NewerVersion(o.Version, f.Version) {
		f.Version = o.Version
	}
	if len(o.Unknown) == 0 {
		return
	}
	all := map[string]int{}
	for k, n := range f.Unknown {
		all[SafeKind(k)] += n
	}
	for k, n := range o.Unknown {
		all[SafeKind(k)] += n
	}
	f.Unknown = capKinds(all)
}

// capKinds は、数の多いもの（同じ数なら名前の順）から MaxKinds までを残し、残りを OtherKind にまとめる。
func capKinds(all map[string]int) map[string]int {
	other := all[OtherKind]
	delete(all, OtherKind)
	if len(all) <= MaxKinds {
		if other > 0 {
			all[OtherKind] = other
		}
		return all
	}
	names := make([]string, 0, len(all))
	for k := range all {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if all[names[i]] != all[names[j]] {
			return all[names[i]] > all[names[j]]
		}
		return names[i] < names[j]
	})
	out := make(map[string]int, MaxKinds+1)
	for i, k := range names {
		if i < MaxKinds {
			out[k] = all[k]
		} else {
			other += all[k]
		}
	}
	if other > 0 {
		out[OtherKind] = other
	}
	return out
}

// Empty は、出すものがないか。
func (f *Format) Empty() bool {
	return f == nil || f.Bad == 0 && f.Version == "" && len(f.Unknown) == 0
}
