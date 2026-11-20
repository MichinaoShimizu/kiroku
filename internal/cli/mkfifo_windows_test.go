//go:build windows

package cli

import "errors"

func syscallMkfifo(string) error { return errors.New("no FIFOs on Windows") }
