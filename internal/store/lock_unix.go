//go:build !windows

package store

import (
	"errors"
	"os"
	"syscall"
)

// tryFlock attempts a non-blocking flock; ok is false if someone else holds it.
func tryFlock(f *os.File, exclusive bool) (ok bool, err error) {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	for {
		err = syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EWOULDBLOCK):
			return false, nil
		}
		return false, err
	}
}
