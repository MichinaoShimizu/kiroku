package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MaxJSONFile より大きい JSON ファイルは、全部を読まずにエラーを返す。上限ちょうどまでは今までどおり読む。
func TestReadJSONFileTooLarge(t *testing.T) {
	old := MaxJSONFile
	MaxJSONFile = 16
	defer func() { MaxJSONFile = old }()
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.json")
	big := filepath.Join(dir, "big.json")
	os.WriteFile(ok, []byte(`{"a":"0123456"}`+"\n"), 0o600) // 16 バイト
	os.WriteFile(big, []byte(`{"a":"01234567"}`+"\n"), 0o600)
	if v, err := ReadJSONFile(ok); err != nil || Str(Map(v)["a"]) != "0123456" {
		t.Errorf("ReadJSONFile(ok) = %v, %v", v, err)
	}
	if v, err := ReadJSONFile(big); err == nil || v != nil || !strings.Contains(err.Error(), "larger than 16 bytes") {
		t.Errorf("ReadJSONFile(big) = %v, %v, want too large", v, err)
	}
}

// フォルダはふつうのファイルでないので開かず、ErrNotRegular を返す。ないファイルは今までどおり fs.ErrNotExist。
func TestOpenRegular(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenRegular(dir); !errors.Is(err, ErrNotRegular) {
		t.Errorf("OpenRegular(dir) err = %v, want ErrNotRegular", err)
	}
	if _, err := OpenRegular(filepath.Join(dir, "none")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("OpenRegular(none) err = %v, want ErrNotExist", err)
	}
	p := filepath.Join(dir, "a.jsonl")
	os.WriteFile(p, []byte("{}\n"), 0o600)
	f, err := OpenRegular(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}
