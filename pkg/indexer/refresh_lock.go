package indexer

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type repoRefreshLock struct {
	file *os.File
}

func acquireRepoRefreshLock(root string) (*repoRefreshLock, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve root path: %w", err)
	}

	path, err := refreshLockPath(absRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("cannot create refresh lock directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open refresh lock: %w", err)
	}
	if err := lockRepoRefreshFile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("cannot acquire refresh lock: %w", err)
	}

	return &repoRefreshLock{file: file}, nil
}

func (l *repoRefreshLock) Unlock() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockRepoRefreshFile(l.file)
	closeErr := l.file.Close()
	if err != nil {
		return fmt.Errorf("cannot release refresh lock: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("cannot close refresh lock: %w", closeErr)
	}
	return nil
}

// RunLocked serializes indexing for a repository across CLI processes.
func RunLocked(root string, db *sql.DB) (IndexStats, error) {
	lock, err := acquireRepoRefreshLock(root)
	if err != nil {
		return IndexStats{}, err
	}
	defer lock.Unlock() //nolint:errcheck

	return Run(root, db)
}

func refreshLockPath(absRoot string) (string, error) {
	db, err := dbPath(absRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(db), "refresh.lock"), nil
}
