package cli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// kiroku serve の読み込みの結果を、次に起動したときのためにディスクに残す（serve-cache）。
// 会話ごとのファイルに分けて読める Source（source.Splitter: Claude Code と Codex）の Unit ごとに、
// 読んだときの印（Stamp と Tag）とセッションを残し、次の起動では印が同じ Unit を読み直さない。
// 履歴の中身（依頼文・応答・パス）が入るので、kiroku が書くほかの履歴のファイルと同じく本人だけが読める
// （フォルダ 0700、ファイル 0600）。kiroku の実行ファイルが変わったら（版を上げた・作り直した）使わない。
// --no-cache で使わず、残っていれば消す。

const (
	diskCacheFile  = "serve-cache"
	diskCacheMagic = "kiroku serve-cache 1"
	// diskCacheMax は読むファイルの大きさの上限（壊れたファイルでメモリを使い切らないように）。
	diskCacheMax = int64(4 << 30)
)

// diskCacheEvery は書き直す間隔の下限（エージェントが動いている間は数秒ごとに読み直すので、そのたびには書かない）。
var diskCacheEvery = 30 * time.Second

// cacheDir は読み込みの結果を残す場所（$KIROKU_CACHE_DIR、なければ OS のキャッシュの場所の kiroku。Windows は kiroku-cache）。
func cacheDir() (string, error) {
	if d := os.Getenv("KIROKU_CACHE_DIR"); d != "" {
		if !filepath.IsAbs(d) {
			return "", fmt.Errorf("KIROKU_CACHE_DIR must be an absolute path: %s", d)
		}
		return d, nil
	}
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(d) { // XDG_CACHE_HOME が相対パスなど。今のフォルダの下には書かない
		return "", errors.New("the cache folder is unknown")
	}
	if runtime.GOOS == "windows" { // %LocalAppData%\kiroku は kiroku serve --archive の場所なので、分ける
		return filepath.Join(d, "kiroku-cache"), nil
	}
	return filepath.Join(d, "kiroku"), nil
}

// diskFile はファイルの中身。
type diskFile struct {
	Magic   string
	Build   string // kiroku の実行ファイルの SHA-256（違えば使わない）
	Gap     int
	Sources []diskSource
}

// diskSource は 1 つの Source の Unit。gob は 1 つの流れを 1 つの CPU でしか読めないので、
// diskShard 個ずつに分けて別々に gob にし、並べて読む。
type diskSource struct {
	ID     string   // sourceID
	Shards [][]byte // []diskUnit の gob
}

// diskShard は 1 つの塊の Unit の数。
const diskShard = 128

type diskUnit struct {
	Key, Stamp string
	Memo       []byte // source.UnitMemoer の UnitMemo（なければ空）
	Outs       []diskLoaded
}

type diskLoaded struct {
	Key, Claim, Yield string
	Sess              *core.Session
	Fulls, Replies    []string // core.Session.Fulls（HTML に入れない全文）
	Empty             [][]int  // gob が nil にする、空の（nil ではない）スライス・マップと、ゼロを指すポインタの場所（emptyMarks）
	Format            *core.Format
}

// sourceID は Source を見分ける名前（エージェントと場所）。
func sourceID(s source.Source) string {
	return s.Family() + "\x00" + s.Name() + "\x00" + s.Where()
}

// diskCache は、ディスクに残した結果の読み書き。
type diskCache struct {
	path, build string
	logf        func(string, ...any)

	mu      sync.Mutex
	loaded  map[string]*sourceCache // 起動したときに読んだもの（Source ごとに 1 回だけ使う）
	memos   map[string]map[string][]byte
	gap     int
	last    time.Time // 最後に書き始めた時刻
	writing bool
	again   bool // 書いている間に、もう一度書く必要ができた
	next    func() (*diskFile, error)
	wg      sync.WaitGroup
}

// buildHash は、いま動いている kiroku の実行ファイルの SHA-256。わからなければ空（残した結果を使わない）。
var buildHash = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := core.OpenRegular(exe)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
})

// openDiskCache は残した結果を読む。読めない・形が違う・古い実行ファイルのものは使わない（次に書くときに置きかわる）。
// prices は --prices のファイル（目安コストが変わるので、中身が違えば使わない）。
func openDiskCache(dir string, gap int, prices string, logf func(string, ...any)) *diskCache {
	build := buildHash()
	if build == "" {
		return nil
	}
	if prices != "" {
		b, err := os.ReadFile(prices)
		if err != nil {
			return nil
		}
		h := sha256.Sum256(b)
		build += "+" + hex.EncodeToString(h[:])
	}
	d := &diskCache{path: filepath.Join(dir, diskCacheFile), build: build, logf: logf, gap: gap}
	if old, _ := filepath.Glob(filepath.Join(dir, diskCacheFile+".*.tmp")); len(old) > 0 { // 書いている途中で止まった残り
		for _, p := range old {
			os.Remove(p)
		}
	}
	f, err := d.read()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) && logf != nil {
			logf("not using the saved cache of history (%v); kiroku reads all history and saves it again\n", err)
		}
		return d
	}
	d.loaded, d.memos = map[string]*sourceCache{}, map[string]map[string][]byte{}
	for _, s := range f.Sources {
		shards := make([][]diskUnit, len(s.Shards))
		parallel(len(s.Shards), func(i int) {
			if gob.NewDecoder(bytes.NewReader(s.Shards[i])).Decode(&shards[i]) != nil {
				shards[i] = nil // 読めない塊の Unit は読み直す
			}
		})
		sc := &sourceCache{units: map[string]unitCache{}}
		memos := map[string][]byte{}
		for _, units := range shards {
			for _, u := range units {
				outs := make([]loaded, 0, len(u.Outs))
				ok := true
				for _, o := range u.Outs {
					if o.Sess != nil && (!restoreEmpty(reflect.ValueOf(o.Sess).Elem(), o.Empty) || !o.Sess.SetFulls(o.Fulls, o.Replies)) {
						ok = false
						break
					}
					outs = append(outs, loaded{key: o.Key, claim: o.Claim, yield: o.Yield, sess: o.Sess, format: o.Format})
				}
				if !ok {
					continue
				}
				sc.units[u.Key] = unitCache{stamp: u.Stamp, outs: outs}
				if len(u.Memo) > 0 {
					memos[u.Key] = u.Memo
				}
			}
		}
		d.loaded[s.ID], d.memos[s.ID] = sc, memos
	}
	return d
}

func (d *diskCache) read() (*diskFile, error) {
	f, err := core.OpenRegular(d.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("the file is readable by other users")
	}
	if st.Size() > diskCacheMax {
		return nil, errors.New("the file is too large")
	}
	var df diskFile
	if err := gob.NewDecoder(bufio.NewReader(io.LimitReader(f, diskCacheMax))).Decode(&df); err != nil {
		return nil, fmt.Errorf("could not read it: %w", err)
	}
	if df.Magic != diskCacheMagic || df.Build != d.build || df.Gap != d.gap {
		return nil, os.ErrNotExist // 別の版・別の --gap のもの。黙って作り直す
	}
	return &df, nil
}

// take は、s について起動したときに読んだ結果（1 回だけ）。Unit の覚え書きを s に戻してから返す。
func (d *diskCache) take(s source.Source, gap int) *sourceCache {
	if d == nil || gap != d.gap {
		return nil
	}
	if _, ok := s.(source.Splitter); !ok {
		return nil
	}
	id := sourceID(s)
	d.mu.Lock()
	sc, memos := d.loaded[id], d.memos[id]
	delete(d.loaded, id)
	delete(d.memos, id)
	d.mu.Unlock()
	if sc == nil {
		return nil
	}
	if m, ok := s.(source.UnitMemoer); ok {
		for key, memo := range memos {
			m.RestoreUnitMemo(key, memo)
		}
	}
	return sc
}

// snapshot は、いまの読み込みの結果からファイルの中身を作る（読めなかった Unit は残さない）。
func (c *loadCache) diskSnapshot(build string) (*diskFile, error) {
	c.mu.Lock()
	sources := make(map[source.Source]*sourceCache, len(c.sources))
	for s, sc := range c.sources { // 中身は読み直しのたびに作り直すので、書き換わらない
		sources[s] = sc
	}
	df := &diskFile{Magic: diskCacheMagic, Build: build, Gap: c.gap}
	withheld, claims, unsure := c.withheld, c.withheldClaims, c.unsure
	c.mu.Unlock()
	if unsure {
		return nil, errors.New("could not fully check which conversations are private")
	}
	for s, sc := range sources {
		if sc == nil || sc.units == nil {
			continue
		}
		memoer, _ := s.(source.UnitMemoer)
		ds := diskSource{ID: sourceID(s)}
		var units []diskUnit
		for key, uc := range sc.units {
			if uc.err != nil {
				continue
			}
			hidden := false
			for _, o := range uc.outs {
				hidden = hidden || o.key != "" && withheld[s.Family()+"\x00"+o.key] || o.claim != "" && claims[o.claim]
			}
			if hidden { // 中身を出さない会話の写しは残さない（次の起動で読み直す）
				continue
			}
			du := diskUnit{Key: key, Stamp: uc.stamp}
			if memoer != nil {
				if m, ok := memoer.UnitMemo(key); ok {
					du.Memo = m
				}
			}
			for _, o := range uc.outs {
				dl := diskLoaded{Key: o.key, Claim: o.claim, Yield: o.yield, Sess: o.sess, Format: o.format}
				if o.sess != nil {
					dl.Fulls, dl.Replies = o.sess.Fulls()
					emptyMarks(reflect.ValueOf(o.sess).Elem(), nil, &dl.Empty)
				}
				du.Outs = append(du.Outs, dl)
			}
			units = append(units, du)
		}
		sort.Slice(units, func(i, j int) bool { return units[i].Key < units[j].Key })
		ds.Shards = make([][]byte, (len(units)+diskShard-1)/diskShard)
		errs := make([]error, len(ds.Shards))
		parallel(len(ds.Shards), func(i int) {
			var b bytes.Buffer
			errs[i] = gob.NewEncoder(&b).Encode(units[i*diskShard : min(len(units), (i+1)*diskShard)])
			ds.Shards[i] = b.Bytes()
		})
		if err := errors.Join(errs...); err != nil {
			return nil, err
		}
		df.Sources = append(df.Sources, ds)
	}
	sort.Slice(df.Sources, func(i, j int) bool { return df.Sources[i].ID < df.Sources[j].ID })
	return df, nil
}

// save は、読み込みの結果を裏で書く。書いている間に呼ばれたら、書き終えてからもう一度だけ書く。
func (d *diskCache) save(snap func() (*diskFile, error)) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.next = snap
	if d.writing {
		d.again = true
		d.mu.Unlock()
		return
	}
	d.writing = true
	d.wg.Add(1)
	d.mu.Unlock()
	go func() {
		defer d.wg.Done()
		for {
			d.mu.Lock()
			wait := diskCacheEvery - time.Since(d.last)
			d.mu.Unlock()
			if wait > 0 {
				time.Sleep(wait)
			}
			d.mu.Lock()
			snap := d.next
			d.last = time.Now()
			d.mu.Unlock()
			df, err := snap()
			if err == nil {
				err = d.write(df)
			}
			if err != nil && d.logf != nil {
				d.logf("could not save the cache of history (%v); the next start reads all history\n", err)
			}
			d.mu.Lock()
			if !d.again {
				d.writing = false
				d.mu.Unlock()
				return
			}
			d.again = false
			d.mu.Unlock()
		}
	}()
}

// wait は、裏で書いているものが終わるのを待つ（テスト用）。
func (d *diskCache) wait() {
	if d != nil {
		d.wg.Wait()
	}
}

// write はファイルを置きかえる（同じフォルダに書いてから名前を変えるので、途中で止まっても前のものが残る）。
func (d *diskCache) write(df *diskFile) error {
	dir := filepath.Dir(d.path)
	if err := core.PrivateDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, diskCacheFile+".*.tmp") // 0600
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // 名前を変えたあとは何もしない
	w := bufio.NewWriterSize(tmp, 1<<20)
	if err := gob.NewEncoder(w).Encode(df); err != nil {
		tmp.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, d.path)
}

// removeDiskCache は残した結果を消す（--no-cache）。消したら true。なければ何もしない。
// 場所がわからない（ホームがない）ときは何もしないが、KIROKU_CACHE_DIR が相対パスならそう言う。
func removeDiskCache() (bool, error) {
	dir, err := cacheDir()
	if err != nil {
		if os.Getenv("KIROKU_CACHE_DIR") != "" {
			return false, err
		}
		return false, nil
	}
	err = os.Remove(filepath.Join(dir, diskCacheFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// emptyMarks は v の中の、gob が送らずに nil に戻すもの（空の nil ではないスライスとマップ、ゼロの値を指すポインタ）の場所を
// out に足す。場所はフィールドの番号とスライスの添字の並び。画面の JSON では [] と null、0 と null で違うので、戻すときに使う。
func emptyMarks(v reflect.Value, path []int, out *[][]int) {
	mark := func() { *out = append(*out, append([]int(nil), path...)) }
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if v.Elem().IsZero() {
			mark()
			return
		}
		emptyMarks(v.Elem(), path, out)
	case reflect.Slice:
		if v.IsNil() {
			return
		}
		if v.Len() == 0 {
			mark()
			return
		}
		for i := range v.Len() {
			emptyMarks(v.Index(i), append(path, i), out)
		}
	case reflect.Map:
		if !v.IsNil() && v.Len() == 0 {
			mark()
		}
	case reflect.Array:
		for i := range v.Len() {
			emptyMarks(v.Index(i), append(path, i), out)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				emptyMarks(v.Field(i), append(path, i), out)
			}
		}
	}
}

// restoreEmpty は emptyMarks の場所を、空のスライス・マップとゼロを指すポインタに戻す。場所が形に合わなければ false。
func restoreEmpty(root reflect.Value, marks [][]int) bool {
	for _, m := range marks {
		v := root
		for _, i := range m {
			for v.Kind() == reflect.Pointer {
				if v.IsNil() {
					return false
				}
				v = v.Elem()
			}
			switch v.Kind() {
			case reflect.Struct:
				if i < 0 || i >= v.NumField() || !v.Type().Field(i).IsExported() {
					return false
				}
				v = v.Field(i)
			case reflect.Slice, reflect.Array:
				if i < 0 || i >= v.Len() {
					return false
				}
				v = v.Index(i)
			default:
				return false
			}
		}
		if !v.CanSet() {
			return false
		}
		switch v.Kind() {
		case reflect.Slice:
			if !v.IsNil() {
				return false
			}
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		case reflect.Map:
			if !v.IsNil() {
				return false
			}
			v.Set(reflect.MakeMap(v.Type()))
		case reflect.Pointer:
			if !v.IsNil() {
				return false
			}
			v.Set(reflect.New(v.Type().Elem()))
		default:
			return false
		}
	}
	return true
}
