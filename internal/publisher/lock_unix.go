//go:build darwin || linux

package publisher

import (
	"os"
	"path/filepath"
	"syscall"
)

func lockPublisher(root string) (func(), error) {
	// Keep the lock inode between owners, including across process restarts.
	f, e := os.OpenFile(filepath.Join(root, ".publish.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		f.Close()
		return nil, e
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
