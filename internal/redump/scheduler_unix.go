//go:build darwin || linux

package redump

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func lockState(root string) (func(), error) {
	f, e := os.OpenFile(filepath.Join(root, ".source.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		if errors.Is(e, syscall.EWOULDBLOCK) || errors.Is(e, syscall.EAGAIN) {
			return nil, ErrBusy
		}
		return nil, e
	}
	// Keep the inode so later processes cannot lock a replacement file concurrently.
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
