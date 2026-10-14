// Fixture for tools/seclint (not built: it is under testdata). The line numbers are checked by main_test.go.
package fixture

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
)

func shells(ctx context.Context, s string) {
	exec.Command("sh", "-c", s)                     // bad 13
	exec.CommandContext(ctx, "/bin/bash", "x.sh")   // bad 14
	exec.Command("cmd.exe", "/c", s)                // bad 15
	exec.Command("python3", "-c", s)                // bad 16
	exec.Command("git", "log", "--format=%H", "--") // fine
	exec.Command(s)                                 // fine: not a literal shell
}

func modes(p string, b []byte, mode os.FileMode) {
	os.WriteFile(p, b, 0o644)              // bad 22
	os.WriteFile(p, b, 0600)               // fine
	os.OpenFile(p, os.O_CREATE, 0o666)     // bad 24
	os.OpenFile(p, os.O_RDONLY, 0)         // fine
	os.Mkdir(p, 0o755)                     // bad 26
	os.MkdirAll(p, 0o700)                  // fine
	os.MkdirAll(p, mode)                   // bad 28: not a constant
	os.WriteFile(p, b, os.FileMode(0o640)) // bad 29
	os.WriteFile(p, b, os.FileMode(0o400)) // fine
}

func listens(addr string, srv *http.Server) {
	http.ListenAndServe(":8080", nil)       // bad 34
	net.Listen("tcp", "0.0.0.0:8485")       // bad 35
	net.Listen("tcp", addr)                 // bad 36: not a constant
	net.Listen("tcp", "127.0.0.1:0")        // fine
	net.Listen("tcp", "[::1]:0")            // fine
	net.Listen("tcp", "localhost:80")       // fine
	srv.ListenAndServe()                    // bad 40
	http.ListenAndServe("10.0.0.1:80", nil) // bad 41
}
