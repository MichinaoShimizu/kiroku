// Command html writes kiroku's view as one static HTML file. It is not part of the released kiroku:
// the e2e tests, the screenshots and the GitHub Pages demo use it (kiroku 5 removed "kiroku html").
//
//	go run ./tools/html --no-open --sources claude --week 2026-10-05 -o out.html
//
// The flags are those of "kiroku html" in kiroku 4 (-o, --week, --month, --lang, --no-open and the history locations).
package main

import (
	"fmt"
	"os"

	"github.com/MichinaoShimizu/kiroku/internal/cli"
)

func main() {
	if err := cli.WriteHTML(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
