package cli

import (
	"fmt"
	"strings"
)

// cmdOpen は kiroku open: 動いている kiroku serve の画面を、鍵をつけてブラウザで開く。
// --print は開かずに、使い捨てのコードつきの URL を出す（ほかの機器で開くとき）。
func cmdOpen(args []string) error {
	fs := newFS("open", "open [flags] [ADDR]\n\nOpens the view of a running \"kiroku serve\" in your browser with its key.\nkiroku serve shows your history only to browsers that have the key, so that other users of\nthis computer can't read it. A browser needs this once; after that, the plain address works.\nADDR defaults to the address \"kiroku autostart\" uses, or "+defaultAddr+".")
	printURL := fs.Bool("print", false, "print a one-time address instead of opening a browser (to open the view on another device); it works once, within 10 minutes, and anyone who opens it first can read your history")
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	addr := listenAddr(defaultAddr)
	if len(pos) == 1 {
		if !addrRe.MatchString(pos[0]) {
			return fmt.Errorf("the address must look like :8485 or 127.0.0.1:8485: %s", pos[0])
		}
		addr = listenAddr(pos[0])
	} else if p, err := currentAutostart(nil); err == nil {
		if a, ok := autostartAddr(p); ok {
			addr = a
		}
	}
	if _, err := serveKey(false); err != nil {
		return err
	}
	url := viewURL(addr)
	if *printURL {
		to, err := codeURL(url)
		if err != nil {
			return err
		}
		fmt.Println(to)
		if strings.HasPrefix(url, "http://localhost:") {
			fmt.Println("(on another device, replace localhost with this computer's address; kiroku serve must listen on it, e.g. \"kiroku serve 0.0.0.0:8484\")")
		}
		return nil
	}
	if probe(probeAddr(addr)) == noAnswer {
		return fmt.Errorf("nothing answers at %s; start it with \"kiroku serve\" (or \"kiroku autostart on\")", url)
	}
	if err := openWithKey(url); err != nil {
		return err
	}
	fmt.Printf("opening %s in your browser\n", url)
	return nil
}

// probeAddr は、すべてのネットワークで待ち受けるアドレスを、手元から問い合わせるアドレスにする。
func probeAddr(addr string) string {
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}
	if strings.HasPrefix(addr, "[::]:") {
		return "127.0.0.1:" + strings.TrimPrefix(addr, "[::]:")
	}
	return addr
}
