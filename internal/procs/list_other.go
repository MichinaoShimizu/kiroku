//go:build !linux && !darwin

package procs

// Supported reports whether this OS can list processes.
func Supported() bool { return false }

// List is not supported on this OS.
func List() ([]Proc, error) { return nil, ErrUnsupported }
