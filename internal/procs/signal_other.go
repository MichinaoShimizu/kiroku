//go:build !linux && !darwin

package procs

func signal(pid int, force bool) error { return ErrUnsupported }
