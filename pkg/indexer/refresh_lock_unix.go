//go:build !windows

package indexer

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockRepoRefreshFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX)
}

func unlockRepoRefreshFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
