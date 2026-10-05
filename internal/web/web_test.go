package web

import "testing"

// 置き場の行は、改行が LF でも CRLF（Windows のチェックアウト）でも、行ごと中身に入れかわる。
func TestAssemble(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		got := assemble("a"+nl+"/*@x*/"+nl+"b"+nl+"//@y"+nl+"c", "/*@x*/", "X"+nl, "//@y", "Y"+nl)
		if want := "a" + nl + "X" + nl + "b" + nl + "Y" + nl + "c"; got != want {
			t.Errorf("%q: got %q, want %q", nl, got, want)
		}
	}
}
