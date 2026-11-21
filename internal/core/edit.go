package core

import "strings"

// FileEdit は、1 つのファイルへの編集の回数と、変えた行の目安（編集の前後を比べた追加・削除の行数）。
// 行は lineDiff と同じ数え方で、前後で共通する先頭・末尾の行を除いた残り。ファイルをまるごと書いたときは、書いた行を追加と数える
// （書く前の中身は履歴にないので、削除は数えない）。
type FileEdit struct {
	Edits   int
	Added   float64
	Removed float64
}

// editWeight は、ファイルの並べ替えに使う変えた量（変えた行、同じなら編集の回数）。
func editWeight(e *FileEdit) float64 {
	if e == nil {
		return 0
	}
	return e.Added + e.Removed + float64(e.Edits)/1e6
}

// EditLines は、編集のツール呼び出し 1 回で変えた行の目安（追加・削除）。中身が引数にない編集は 0, 0。
//   - Claude Code: Edit・MultiEdit（old_string → new_string）、Write（content）、NotebookEdit（new_source）
//   - Kiro CLI: fs_write（command が create なら file_text、str_replace なら old_str → new_str、insert・append なら new_str）
//   - Kiro IDE: fsWrite・fsAppend（text）、strReplace（oldStr → newStr）
func EditLines(name string, args any) (added, removed float64) {
	in := Map(args)
	n := func(s string) float64 { return float64(len(lines(s))) }
	switch name {
	case "Edit":
		return lineDiff(Str(in["old_string"]), Str(in["new_string"]))
	case "MultiEdit":
		for _, e := range List(in["edits"]) {
			em := Map(e)
			a, r := lineDiff(Str(em["old_string"]), Str(em["new_string"]))
			added, removed = added+a, removed+r
		}
		return
	case "Write":
		return n(Str(in["content"])), 0
	case "NotebookEdit":
		return n(Str(in["new_source"])), 0
	case "fs_write":
		switch Str(in["command"]) {
		case "create":
			return n(Str(in["file_text"])), 0
		case "str_replace":
			return lineDiff(Str(in["old_str"]), Str(in["new_str"]))
		case "insert", "append":
			return n(Str(in["new_str"])), 0
		}
	case "fsWrite", "fsAppend":
		return n(Str(in["text"])), 0
	case "strReplace":
		return lineDiff(Str(in["oldStr"]), Str(in["newStr"]))
	}
	return 0, 0
}

// PatchLines は、apply_patch のパッチ（*** Begin Patch 形式）から、ファイルごとの追加・削除の行数を数える（Codex）。
// 追加するファイル（*** Add File）は + の行がそのまま中身。移すファイル（*** Move to）は、移した先の名前で数える。
func PatchLines(patch string) map[string][2]float64 {
	out := map[string][2]float64{}
	cur := ""
	for _, l := range strings.Split(patch, "\n") {
		l = strings.TrimSuffix(l, "\r")
		h := strings.TrimSpace(l) // 見出しの行は、シェルのヒアドキュメントの中で字下げされていることがある
		switch {
		case strings.HasPrefix(h, "*** Add File: "), strings.HasPrefix(h, "*** Update File: "), strings.HasPrefix(h, "*** Delete File: "):
			cur = strings.TrimSpace(h[strings.Index(h, ": ")+2:])
			if _, ok := out[cur]; !ok {
				out[cur] = [2]float64{}
			}
		case strings.HasPrefix(h, "*** Move to: "):
			to := strings.TrimSpace(strings.TrimPrefix(h, "*** Move to: "))
			if cur != "" && to != "" {
				out[to] = out[cur]
				delete(out, cur)
				cur = to
			}
		case strings.HasPrefix(h, "***"), strings.HasPrefix(l, "@@"):
		case cur != "" && strings.HasPrefix(l, "+"):
			x := out[cur]
			x[0]++
			out[cur] = x
		case cur != "" && strings.HasPrefix(l, "-"):
			x := out[cur]
			x[1]++
			out[cur] = x
		}
	}
	return out
}
