package indexer

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// workerCount is the number of concurrent parse workers in the pool.
// Scaled to the number of logical CPUs, with a minimum of 4 to ensure
// parallelism even on low-core machines.
var workerCount = func() int {
	n := runtime.NumCPU()
	if n < 4 {
		return 4
	}
	return n
}()

// skipDirs is the set of directory names that are never descended into.
// Any directory whose name starts with '.' is also skipped (see walk loop).
var skipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
}

// fileJob is dispatched by the walker to the worker pool.
type fileJob struct {
	path string // absolute path
	rel  string // relative to absRoot
	ext  string // e.g. ".go"
}

// fileResult is sent back by each worker to the collector.
type fileResult struct {
	rel      string
	entry    FileEntry
	err      error // non-nil: read or parse failure
	skip     bool  // true: file unchanged, no write needed
	wasAdded bool  // true: file was not previously in the index
}

// Run walks root, compares each supported file against the index stored in db,
// re-parses only changed or new files using a concurrent worker pool, prunes
// deleted entries, and persists all changes.
//
// Per-file read/parse failures are collected in IndexStats.FileErrors and
// never abort the run. Fatal errors (unresolvable root, walk failure) are
// returned as the error return value.
func Run(root string, db *sql.DB) (IndexStats, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return IndexStats{}, fmt.Errorf("cannot resolve root path: %w", err)
	}

	// Single shared facade — grammars are read-only after init, safe for
	// concurrent use across all workers.
	muncher := NewMuncher()

	now := time.Now().UTC()

	// Snapshot the currently indexed paths before the walk so we can detect
	// deletions afterwards. Read-only, safe from any goroutine.
	indexed, err := IndexedPaths(db)
	if err != nil {
		return IndexStats{}, fmt.Errorf("cannot load indexed paths: %w", err)
	}

	jobs := make(chan fileJob, workerCount*2)
	results := make(chan fileResult, workerCount*2)

	// --- worker pool ---
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(muncher, db, now, jobs, results)
		}()
	}

	// Close results once all workers are done.
	go func() {
		wg.Wait()
		close(results)
	}()

	// --- walker goroutine ---
	// visited tracks relative paths seen during this walk so we can
	// detect deletions. Sent back via channel once the walk completes.
	visited := make(chan map[string]bool, 1)

	var walkErr error
	go func() {
		v := make(map[string]bool)
		walkErr = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if _, supported := muncher.langs[filepath.Ext(path)]; !supported {
				return nil
			}
			rel, err := filepath.Rel(absRoot, path)
			if err != nil {
				return err
			}
			v[rel] = true
			jobs <- fileJob{path: path, rel: rel, ext: filepath.Ext(path)}
			return nil
		})
		close(jobs)
		visited <- v
	}()

	// --- collector (runs on calling goroutine — sole DB writer) ---
	var stats IndexStats

	for res := range results {
		if res.err != nil {
			stats.Errors++
			stats.FileErrors = append(stats.FileErrors, FileError{Path: res.rel, Err: res.err})
			continue
		}
		if res.skip {
			stats.Unchanged++
			continue
		}
		if err := WriteFile(db, res.rel, res.entry); err != nil {
			stats.Errors++
			stats.FileErrors = append(stats.FileErrors, FileError{Path: res.rel, Err: err})
			continue
		}
		if res.wasAdded {
			stats.Added++
		} else {
			stats.Updated++
		}
	}

	// Drain visited map — walker goroutine has finished (jobs closed).
	// Receiving from visited establishes a happens-before relationship:
	// the walker goroutine sent on visited only after completing the WalkDir call
	// (and assigning the result to walkErr). Reading walkErr here is therefore safe —
	// the send on visited synchronizes-with this receive, ensuring all writes to
	// walkErr in the walker goroutine are visible to this goroutine.
	v := <-visited

	// At this point, walkErr has been assigned by the walker goroutine.
	// The channel receive above guarantees visibility of that assignment.
	if walkErr != nil {
		return IndexStats{}, fmt.Errorf("walk error: %w", walkErr)
	}

	// Prune files present in the index but no longer on disk.
	var deleted []string
	for rel := range indexed {
		if !v[rel] {
			deleted = append(deleted, rel)
		}
	}
	if err := PruneFiles(db, deleted); err != nil {
		return stats, fmt.Errorf("cannot prune deleted files: %w", err)
	}
	stats.Removed = len(deleted)

	// Stamp the index with the time this run completed so that ShouldRefresh
	// can cheaply decide whether a re-walk is needed on the next query.
	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return stats, fmt.Errorf("cannot update last_indexed_at: %w", err)
	}

	return stats, nil
}

// worker reads, hashes, and parses each job it receives.
// It sends exactly one fileResult per job.
func worker(
	muncher *MuncherFacade,
	db *sql.DB,
	now time.Time,
	jobs <-chan fileJob,
	results chan<- fileResult,
) {
	for job := range jobs {
		results <- processJob(muncher, db, now, job)
	}
}

// processJob performs the read → hash → compare → parse sequence for one file.
func processJob(
	muncher *MuncherFacade,
	db *sql.DB,
	now time.Time,
	job fileJob,
) fileResult {
	// Stage 1: stat the file to get mtime + size cheaply.
	fi, err := os.Stat(job.path)
	if err != nil {
		return fileResult{rel: job.rel, err: fmt.Errorf("cannot stat file: %w", err)}
	}
	statMtime := fi.ModTime().UTC().Format(time.RFC3339)
	statSize := fi.Size()

	// Stage 2: fetch stored meta (hash + mtime + size) from DB.
	stored, err := GetFileMeta(db, job.rel)
	if err != nil {
		return fileResult{rel: job.rel, err: fmt.Errorf("cannot query meta: %w", err)}
	}

	// Fast path: if mtime AND size both match the stored values, the file is
	// unchanged — skip the read, hash, and parse entirely.
	if stored.Hash != "" && stored.Mtime == statMtime && stored.Size == statSize {
		return fileResult{rel: job.rel, skip: true}
	}

	// Slow path: read, hash, and (if hash changed) re-parse.
	code, err := readFile(job.path)
	if err != nil {
		return fileResult{rel: job.rel, err: fmt.Errorf("cannot read file: %w", err)}
	}

	sum := fileSHA256(code)

	// Even if mtime/size differ, the content hash may still match (e.g. touch).
	if stored.Hash == sum {
		return fileResult{rel: job.rel, skip: true}
	}

	symbols, err := muncher.GetSymbols(job.path, code)
	if err != nil {
		return fileResult{rel: job.rel, err: fmt.Errorf("cannot parse file: %w", err)}
	}

	lang := extensionLanguage(job.ext)

	calls, err := ExtractCalls(lang, code)
	if err != nil {
		// Non-fatal: log and continue without call sites for this file.
		calls = nil
	}

	return fileResult{
		rel: job.rel,
		entry: FileEntry{
			Language:  lang,
			SHA256:    sum,
			Mtime:     statMtime,
			Size:      statSize,
			IndexedAt: now,
			Symbols:   symbols,
			Calls:     calls,
		},
		wasAdded: stored.Hash == "",
	}
}

// extensionLanguage maps a file extension to a human-readable language name.
func extensionLanguage(ext string) string {
	switch ext {
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".cs":
		return "csharp"
	case ".rs":
		return "rust"
	default:
		return ext
	}
}
