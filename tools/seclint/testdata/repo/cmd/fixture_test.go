package fixture

import (
	"net"
	"os"
	"os/exec"
)

func helpers(p string) {
	os.WriteFile(p, nil, 0o644)    // fine in tests
	net.Listen("tcp", "0.0.0.0:0") // fine in tests
	exec.Command("bash", "-c", "") // bad 12: no shells, even in tests
}
