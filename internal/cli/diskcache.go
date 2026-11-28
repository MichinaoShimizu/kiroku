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
// （フォルダ 0700、ファイル 0600）。kiroku の実行ファイル・時間帯・--gap・--prices が変わったら使わない。
// 中身を出さない会話（Kiro Crew の incognito・temporary）の写しは残さない。--no-cache で使わず、残っていれば消す。

const (
	diskCacheFile  = "serve-cache"
	diskCacheMagic = "kiroku serve-cache 2"
	// diskCacheMax は読むファイルの大きさの上限（壊れたファイルでメモリを使い切らないように）。
	// 塊ごとに読むので、一度にメモリに置くのは読み終えたセッションと、並べて読んでいる数個の塊だけ。
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

// ファイルは 1 つの gob の流れで、diskHeader のあとに diskShardMsg が続き、Last の印で終わる。
// 1 つにまとめた値にすると、書くときも読むときもファイル全体をメモリに置くので、塊（diskShard 個の Unit）ごとの値にする。
// 塊の中身は別の gob（[]diskUnit）にしてあり、並べて読む（gob の 1 つの流れは 1 つの CPU でしか読めない）。
type diskHeader struct {
	Magic string
	Build string // kiroku の実行ファイルの SHA-256 と、時間帯・--prices（違えば使わない）
	Gap   int
}

type diskShardMsg struct {
	Source string // sourceID
	Data   []byte // []diskUnit の gob
	Last   bool   // ファイルの終わり（途中で切れたファイルを、そろったものとして使わない）
}

// diskShard は 1 つの塊の Unit の数。
const diskShard = 32

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

// diskFile は書くもの: 見出しと、Source ごとの Unit（塊に分けるのは書くとき）。
type diskFile struct {
	head    diskHeader
	sources []diskSource
}

type diskSource struct {
	id    string
	units []diskUnit
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
	gen     int // remove のたびに増やす（消す前に作り始めた中身を、あとから置かない）
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
	// 時間帯のない時刻はローカル時刻として読むので、時間帯が変わったら使わない
	zone, off := time.Now().Zone()
	build += fmt.Sprintf("+tz=%s/%s/%d", os.Getenv("TZ"), zone, off)
	if _, err := os.Lstat(dir); err == nil { // 前からあって、ほかの人も開けるフォルダも自分だけにする（書かないときも）
		if err := core.PrivateDir(dir); err != nil {
			return nil
		}
	}
	d := &diskCache{path: filepath.Join(dir, diskCacheFile), build: build, logf: logf, gap: gap}
	if old, _ := filepath.Glob(filepath.Join(dir, diskCacheFile+".*.tmp")); len(old) > 0 { // 書いている途中で止まった残り
		for _, p := range old {
			os.Remove(p)
		}
	}
	loaded, memos, err := d.read()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) && logf != nil {
			logf("not using the saved cache of history (%v); kiroku reads all history and saves it again\n", err)
		}
		return d
	}
	d.loaded, d.memos = loaded, memos
	return d
}

// read はファイルを塊ごとに読み、Source ごとの結果にする。読めない塊・形の合わない Unit は捨てる（読み直す）。
// 終わりの印まで読めなかったファイルは使わない。
func (d *diskCache) read() (map[string]*sourceCache, map[string]map[string][]byte, error) {
	f, err := core.OpenRegular(d.path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0 {
		return nil, nil, errors.New("the file is readable by other users")
	}
	if st.Size() > diskCacheMax {
		return nil, nil, errors.New("the file is too large")
	}
	dec := gob.NewDecoder(bufio.NewReaderSize(io.LimitReader(f, diskCacheMax), 1<<20))
	var head diskHeader
	if err := dec.Decode(&head); err != nil {
		return nil, nil, fmt.Errorf("could not read it: %w", err)
	}
	if head.Magic != diskCacheMagic || head.Build != d.build || head.Gap != d.gap {
		return nil, nil, os.ErrNotExist // 別の版・時間帯・--gap のもの。黙って作り直す
	}
	type decoded struct {
		id    string
		units []diskUnit
	}
	workers := max(1, runtime.GOMAXPROCS(0))
	in, out := make(chan diskShardMsg, workers), make(chan decoded, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range in {
				var units []diskUnit
				if gob.NewDecoder(bytes.NewReader(m.Data)).Decode(&units) != nil {
					units = nil // 読めない塊の Unit は読み直す
				}
				out <- decoded{m.Source, units}
			}
		}()
	}
	loaded, memos := map[string]*sourceCache{}, map[string]map[string][]byte{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for dc := range out {
			sc := loaded[dc.id]
			if sc == nil {
				sc = &sourceCache{units: map[string]unitCache{}}
				loaded[dc.id], memos[dc.id] = sc, map[string][]byte{}
			}
			for _, u := range dc.units {
				if outs, ok := restoreUnit(u); ok {
					sc.units[u.Key] = unitCache{stamp: u.Stamp, outs: outs}
					if len(u.Memo) > 0 {
						memos[dc.id][u.Key] = u.Memo
					}
				}
			}
		}
	}()
	var readErr error
	complete := false
	for {
		var m diskShardMsg
		if err := dec.Decode(&m); err != nil {
			readErr = err
			break
		}
		if m.Last {
			complete = true
			break
		}
		in <- m
	}
	close(in)
	wg.Wait()
	close(out)
	<-done
	if !complete {
		return nil, nil, fmt.Errorf("could not read it: %w", readErr)
	}
	return loaded, memos, nil
}

// restoreUnit は、ファイルから読んだ Unit を読み込みの結果に戻す。形が合わなければ false。
func restoreUnit(u diskUnit) ([]loaded, bool) {
	outs := make([]loaded, 0, len(u.Outs))
	for _, o := range u.Outs {
		if o.Sess != nil && (!plainAny(o.Sess) || !restoreEmpty(reflect.ValueOf(o.Sess).Elem(), o.Empty) || !o.Sess.SetFulls(o.Fulls, o.Replies)) {
			return nil, false
		}
		outs = append(outs, loaded{key: o.Key, claim: o.Claim, yield: o.Yield, sess: o.Sess, format: o.Format})
	}
	return outs, true
}

// plainAny は、[名前, 数] の欄（Tools・Models・Tiers）が、読み込みで作る形（文字列と数）だけか。
// gob は any に登録した型ならなんでも戻すので、画面の JSON にできない値（complex128 など）の入ったファイルを使わない。
func plainAny(s *core.Session) bool {
	for _, rows := range [][][2]any{s.Tools, s.Models, s.Tiers} {
		for _, r := range rows {
			if _, ok := r[0].(string); !ok {
				return false
			}
			switch r[1].(type) {
			case int, float64:
			default:
				return false
			}
		}
	}
	return true
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

// diskSnapshot は、読み込みの結果からファイルの中身を作る（読めなかった Unit と、隠した会話の写しは残さない）。
// 使うのは、読み込みを終えたときに setWithheld が取っておいた、読んだ結果と隠した会話の組
// （読み込みの途中の、新しい会話と前の読み込みの隠す会話の組み合わせで書かないように）。
func (c *loadCache) diskSnapshot(build string) (*diskFile, error) {
	c.mu.Lock()
	sources := c.saved
	df := &diskFile{head: diskHeader{Magic: diskCacheMagic, Build: build, Gap: c.gap}}
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
		ds := diskSource{id: sourceID(s)}
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
			ds.units = append(ds.units, du)
		}
		sort.Slice(ds.units, func(i, j int) bool { return ds.units[i].Key < ds.units[j].Key })
		df.sources = append(df.sources, ds)
	}
	sort.Slice(df.sources, func(i, j int) bool { return df.sources[i].id < df.sources[j].id })
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
			snap, gen := d.next, d.gen
			d.last = time.Now()
			d.mu.Unlock()
			df, err := snap()
			if err == nil {
				err = d.write(df, gen)
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
// 塊は CPU の数ずつ並べて gob にして順に書くので、一度にメモリに置くのはその数の塊だけ。
// 書いている間に remove されたら（gen が変わったら）置きかえない。
func (d *diskCache) write(df *diskFile, gen int) error {
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
	fail := func(err error) error {
		tmp.Close()
		return err
	}
	w := bufio.NewWriterSize(tmp, 1<<20)
	enc := gob.NewEncoder(w)
	if err := enc.Encode(df.head); err != nil {
		return fail(err)
	}
	workers := max(1, runtime.GOMAXPROCS(0))
	for _, ds := range df.sources {
		n := (len(ds.units) + diskShard - 1) / diskShard
		for start := 0; start < n; start += workers {
			batch := make([][]byte, min(workers, n-start))
			errs := make([]error, len(batch))
			parallel(len(batch), func(i int) {
				k := start + i
				var b bytes.Buffer
				errs[i] = gob.NewEncoder(&b).Encode(ds.units[k*diskShard : min(len(ds.units), (k+1)*diskShard)])
				batch[i] = b.Bytes()
			})
			if err := errors.Join(errs...); err != nil {
				return fail(err)
			}
			for _, b := range batch {
				if err := enc.Encode(diskShardMsg{Source: ds.id, Data: b}); err != nil {
					return fail(err)
				}
			}
		}
	}
	if err := enc.Encode(diskShardMsg{Last: true}); err != nil {
		return fail(err)
	}
	if err := w.Flush(); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gen != gen {
		return nil
	}
	return os.Rename(name, d.path)
}

// remove は残した結果を消す（隠すものを調べきれなかったとき。前に書いたものに、いまは隠す会話が入っているかもしれない）。
func (d *diskCache) remove() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gen++
	if err := os.Remove(d.path); err != nil && !errors.Is(err, os.ErrNotExist) && d.logf != nil {
		d.logf("could not delete the cache of history (%v)\n", err)
	}
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
