package core

import (
	"fmt"
	"testing"
)

// 編集のツール呼び出しから、変えた行の目安を数える（エージェントごとの引数の形）。
func TestEditLines(t *testing.T) {
	cases := []struct {
		name     string
		args     map[string]any
		add, del float64
	}{
		{"Edit", map[string]any{"old_string": "a\nb\nc", "new_string": "a\nB\nB2\nc"}, 2, 1},
		{"MultiEdit", map[string]any{"edits": []any{map[string]any{"old_string": "x", "new_string": "y"}, map[string]any{"old_string": "", "new_string": "p\nq"}}}, 3, 1},
		{"Write", map[string]any{"content": "1\n2\n3\n"}, 3, 0},
		{"NotebookEdit", map[string]any{"new_source": "print(1)"}, 1, 0},
		{"fs_write", map[string]any{"command": "create", "file_text": "a\nb"}, 2, 0},
		{"fs_write", map[string]any{"command": "str_replace", "old_str": "a", "new_str": "b\nc"}, 2, 1},
		{"fs_write", map[string]any{"command": "append", "new_str": "z"}, 1, 0},
		{"fsWrite", map[string]any{"text": "a\nb\nc"}, 3, 0},
		{"strReplace", map[string]any{"oldStr": "a\nb", "newStr": "a"}, 0, 1},
		{"editCode", map[string]any{"path": "x"}, 0, 0}, // 中身が引数にない編集は数えない
		{"Read", map[string]any{"file_path": "x"}, 0, 0},
	}
	for _, c := range cases {
		a, d := EditLines(c.name, c.args)
		if a != c.add || d != c.del {
			t.Errorf("%s %v: got +%v -%v, want +%v -%v", c.name, c.args, a, d, c.add, c.del)
		}
	}
}

// apply_patch のパッチから、ファイルごとの追加・削除を数える。移したファイルは移した先の名前、字下げされた見出しも読む。
func TestPatchLines(t *testing.T) {
	p := "*** Begin Patch\n*** Add File: new.go\n+package x\n+\n*** Update File: a.go\n@@ func f\n-old\n+new\n+new2\n ctx\n  *** Update File: b.go\n*** Move to: c.go\n-x\n*** Delete File: d.go\n*** End Patch"
	got := PatchLines(p)
	want := map[string][2]float64{"new.go": {2, 0}, "a.go": {2, 1}, "c.go": {0, 1}, "d.go": {0, 0}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("PatchLines = %v, want %v", got, want)
	}
}

// セッションには、ファイルごとの編集の回数と行（files と同じ並び）と、全部のファイルの行の合計が入る。
// ファイルが 30 を超えるときは、多く変えたファイルを残す。中身を出さない会話（HideText）では消える。
func TestBuilderFileEdits(t *testing.T) {
	at := func(v float64) *float64 { return &v }
	b := NewBuilder("Claude Code", "s1")
	b.Tick(at(1000))
	b.Tool("Edit", map[string]any{"file_path": "/r/a.go", "old_string": "x", "new_string": "y\nz"})
	b.Tool("Edit", map[string]any{"file_path": "/r/a.go", "old_string": "y", "new_string": "w"})
	b.Tool("Write", map[string]any{"file_path": "/r/b.go", "content": "1\n2\n3"})
	b.Tool("Read", map[string]any{"file_path": "/r/c.go"})
	b.Tick(at(2000))
	s := b.Finish(15)
	if fmt.Sprint(s.Files) != "[/r/a.go /r/b.go]" || fmt.Sprint(s.FileEdits) != "[[2 3 2] [1 3 0]]" || s.EditAdded != 6 || s.EditRemoved != 2 {
		t.Errorf("files %v edits %v +%v -%v", s.Files, s.FileEdits, s.EditAdded, s.EditRemoved)
	}

	b = NewBuilder("Claude Code", "s2")
	b.Tick(at(1000))
	for i := 0; i < 40; i++ {
		b.Tool("Write", map[string]any{"file_path": fmt.Sprintf("/r/f%02d.go", i), "content": "x"})
	}
	b.Tool("Write", map[string]any{"file_path": "/r/zz.go", "content": "1\n2\n3\n4\n5"})
	s = b.Finish(15)
	if len(s.Files) != 30 || s.NFiles != 41 || s.Files[len(s.Files)-1] != "/r/zz.go" || s.EditAdded != 45 {
		t.Errorf("files %d/%d last %q +%v", len(s.Files), s.NFiles, s.Files[len(s.Files)-1], s.EditAdded)
	}

	b = NewBuilder("Kiro Crew", "s3")
	b.Tick(at(1000))
	b.Tool("Edit", map[string]any{"file_path": "/r/secret.go", "old_string": "a", "new_string": "b"})
	b.HideText("(hidden)")
	s = b.Finish(15)
	if len(s.Files) != 0 || s.FileEdits != nil || s.EditAdded != 0 {
		t.Errorf("hidden: files %v edits %v +%v", s.Files, s.FileEdits, s.EditAdded)
	}
}
