// Package diagnostics owns bounded, user-accessible application diagnostics.
package diagnostics

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxSegmentBytes int64 = 5 << 20
	DefaultMaxSegments           = 8
	filePrefix                   = "application-"
	fileSuffix                   = ".log"
)

// Options configures a retained application log. Zero values select production defaults.
type Options struct {
	Directory       string
	Fallback        io.Writer
	Now             func() time.Time
	RunID           string
	MaxSegmentBytes int64
	MaxSegments     int
	remove          func(string) error
}

type retainedFile interface {
	io.Writer
	io.Closer
}

// RunLog mirrors records to the process fallback and retains a bounded, run-scoped copy.
type RunLog struct {
	mu              sync.RWMutex
	directory       string
	runID           string
	currentPath     string
	file            retainedFile
	segment         int
	size            int64
	maxSegmentBytes int64
	maxSegments     int
	fallback        io.Writer
	now             func() time.Time
	remove          func(string) error
	warned          bool
	closed          bool
}

// Open creates the current run log. The returned writer remains usable through
// the fallback even when retained storage cannot be initialized.
func Open(options Options) (*RunLog, error) {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	runID := options.RunID
	if runID == "" {
		runID = newRunID()
	}
	maxBytes := options.MaxSegmentBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSegmentBytes
	}
	maxSegments := options.MaxSegments
	if maxSegments <= 0 {
		maxSegments = DefaultMaxSegments
	}
	maxSegments = max(maxSegments, 2)
	remove := options.remove
	if remove == nil {
		remove = os.Remove
	}
	result := &RunLog{
		directory: options.Directory, runID: runID, fallback: options.Fallback,
		now: now, maxSegmentBytes: maxBytes, maxSegments: maxSegments, remove: remove,
	}
	if err := result.openSegmentLocked(); err != nil {
		result.warnFallbackLocked()
		return result, err
	}
	return result, nil
}

func newRunID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func (log *RunLog) Directory() string {
	log.mu.RLock()
	defer log.mu.RUnlock()
	return log.directory
}

func (log *RunLog) RunID() string {
	log.mu.RLock()
	defer log.mu.RUnlock()
	return log.runID
}

func (log *RunLog) CurrentPath() string {
	log.mu.RLock()
	defer log.mu.RUnlock()
	return log.currentPath
}

func (log *RunLog) Write(record []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.closed {
		return 0, os.ErrClosed
	}
	fallbackOK := log.fallback == nil
	if log.fallback != nil {
		written, err := log.fallback.Write(record)
		fallbackOK = err == nil && written == len(record)
	}
	if log.file == nil {
		if fallbackOK {
			return len(record), nil
		}
		return 0, io.ErrShortWrite
	}
	if int64(len(record)) > log.maxSegmentBytes {
		if fallbackOK {
			return len(record), nil
		}
		return 0, io.ErrShortWrite
	}
	if log.size > 0 && log.size+int64(len(record)) > log.maxSegmentBytes {
		if err := log.rotateLocked(); err != nil {
			log.degradeLocked()
			if fallbackOK {
				return len(record), nil
			}
			return 0, err
		}
	}
	written, err := log.file.Write(record)
	if err != nil || written != len(record) {
		if err == nil {
			err = io.ErrShortWrite
		}
		log.degradeLocked()
		if fallbackOK {
			return len(record), nil
		}
		return written, err
	}
	log.size += int64(written)
	return len(record), nil
}

func (log *RunLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.closed {
		return nil
	}
	log.closed = true
	if log.file == nil {
		return nil
	}
	err := log.file.Close()
	log.file = nil
	return err
}

func (log *RunLog) rotateLocked() error {
	if err := log.file.Close(); err != nil {
		return err
	}
	log.file = nil
	log.currentPath = ""
	log.segment++
	return log.openSegmentLocked()
}

func (log *RunLog) openSegmentLocked() error {
	if !filepath.IsAbs(log.directory) {
		return errors.New("log directory must be absolute")
	}
	if err := os.MkdirAll(log.directory, 0o700); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	info, err := os.Lstat(log.directory)
	if err != nil {
		return fmt.Errorf("inspect log directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("log directory must be a real directory")
	}
	if err := os.Chmod(log.directory, 0o700); err != nil {
		return fmt.Errorf("secure log directory: %w", err)
	}
	// Reserve capacity before creating a segment. If pruning fails, retaining the
	// existing evidence is safer than admitting another file past the boundary.
	if err := log.pruneLocked(log.maxSegments - 1); err != nil {
		return err
	}
	name := fmt.Sprintf("%s%s-%s-%03d%s", filePrefix, log.now().UTC().Format("20060102T150405.000000000Z"), log.runID, log.segment, fileSuffix)
	path := filepath.Join(log.directory, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create retained log segment: %w", err)
	}
	log.file = file
	log.currentPath = path
	log.size = 0
	return nil
}

type ownedSegment struct {
	path  string
	name  string
	runID string
	mod   time.Time
}

func (log *RunLog) pruneLocked(limit int) error {
	entries, err := os.ReadDir(log.directory)
	if err != nil {
		return fmt.Errorf("list retained logs: %w", err)
	}
	segments := make([]ownedSegment, 0, len(entries))
	for _, entry := range entries {
		runID, ok := parseOwnedName(entry.Name())
		if !ok || entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		segments = append(segments, ownedSegment{path: filepath.Join(log.directory, entry.Name()), name: entry.Name(), runID: runID, mod: info.ModTime()})
	}
	slices.SortFunc(segments, func(left, right ownedSegment) int {
		if order := left.mod.Compare(right.mod); order != 0 {
			return order
		}
		return cmp.Compare(left.name, right.name)
	})
	protectedPrevious := ""
	for _, segment := range slices.Backward(segments) {
		if segment.runID != log.runID {
			protectedPrevious = segment.path
			break
		}
	}
	for len(segments) > limit {
		remove := -1
		for index, segment := range segments {
			if segment.path != log.currentPath && segment.path != protectedPrevious {
				remove = index
				break
			}
		}
		if remove < 0 {
			break
		}
		if err := log.remove(segments[remove].path); err != nil {
			return fmt.Errorf("prune retained log: %w", err)
		}
		segments = append(segments[:remove], segments[remove+1:]...)
	}
	return nil
}

func parseOwnedName(name string) (string, bool) {
	base, ok := strings.CutPrefix(name, filePrefix)
	if !ok {
		return "", false
	}
	base, ok = strings.CutSuffix(base, fileSuffix)
	if !ok {
		return "", false
	}
	identity, ordinal, ok := strings.CutLast(base, "-")
	if !ok || len(ordinal) < 3 {
		return "", false
	}
	for _, digit := range ordinal {
		if digit < '0' || digit > '9' {
			return "", false
		}
	}
	timestamp, runID, ok := strings.Cut(identity, "-")
	if !ok || runID == "" {
		return "", false
	}
	if _, err := time.Parse("20060102T150405.000000000Z", timestamp); err != nil {
		return "", false
	}
	return runID, true
}

func (log *RunLog) degradeLocked() {
	if log.file != nil {
		_ = log.file.Close()
		log.file = nil
	}
	log.currentPath = ""
	log.warnFallbackLocked()
}

func (log *RunLog) warnFallbackLocked() {
	if log.warned || log.fallback == nil {
		return
	}
	log.warned = true
	_, _ = fmt.Fprintf(log.fallback, "level=WARN run_id=%s operation=diagnostics.retention outcome=degraded error_category=storage_unavailable\n", log.runID)
}
