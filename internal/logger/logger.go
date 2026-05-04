package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	defaultRotateBytes  int64 = 2 * 1024 * 1024
	defaultTailBytes    int64 = 128 * 1024
	defaultArchiveCount       = 10
)

type FileLogger struct {
	mu           sync.Mutex
	path         string
	archiveDir   string
	file         *os.File
	rotateBytes  int64
	archiveCount int
}

type entry struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Event   string         `json:"event"`
	Details map[string]any `json:"details,omitempty"`
}

func NewFileLogger(path string) (*FileLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	logger := &FileLogger{
		path:         path,
		archiveDir:   filepath.Join(filepath.Dir(path), "archive"),
		rotateBytes:  defaultRotateBytes,
		archiveCount: defaultArchiveCount,
	}
	if err := os.MkdirAll(logger.archiveDir, 0o755); err != nil {
		return nil, err
	}
	if err := logger.openCurrentFile(); err != nil {
		return nil, err
	}
	return logger, nil
}

func (l *FileLogger) Info(event string, details map[string]any) {
	l.write("info", event, details)
}

func (l *FileLogger) Error(event string, details map[string]any) {
	l.write("error", event, details)
}

func (l *FileLogger) write(level, eventName string, details map[string]any) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		if err := l.openCurrentFile(); err != nil {
			return
		}
	}
	if err := l.rotateIfNeededLocked(); err != nil {
		return
	}

	_ = json.NewEncoder(l.file).Encode(entry{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Level:   level,
		Event:   eventName,
		Details: details,
	})
}

func (l *FileLogger) ReadTail(maxBytes int64) ([]byte, error) {
	if l == nil {
		return nil, nil
	}
	if maxBytes <= 0 {
		maxBytes = defaultTailBytes
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.readTailLocked(maxBytes)
}

func (l *FileLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *FileLogger) openCurrentFile() error {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.file = file
	return nil
}

func (l *FileLogger) rotateIfNeededLocked() error {
	info, err := l.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < l.rotateBytes {
		return nil
	}

	if err := l.file.Close(); err != nil {
		return err
	}

	archiveName := fmt.Sprintf("app-%s.jsonl", time.Now().UTC().Format("20060102-150405"))
	archivePath := filepath.Join(l.archiveDir, archiveName)
	if err := os.Rename(l.path, archivePath); err != nil {
		return err
	}

	if err := l.trimArchivesLocked(); err != nil {
		return err
	}

	l.file = nil
	return l.openCurrentFile()
}

func (l *FileLogger) trimArchivesLocked() error {
	entries, err := os.ReadDir(l.archiveDir)
	if err != nil {
		return err
	}

	type archiveEntry struct {
		name    string
		modTime time.Time
	}
	var archives []archiveEntry
	for _, item := range entries {
		if item.IsDir() {
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		archives = append(archives, archiveEntry{
			name:    filepath.Join(l.archiveDir, item.Name()),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		return archives[i].modTime.After(archives[j].modTime)
	})

	for idx := l.archiveCount; idx < len(archives); idx++ {
		_ = os.Remove(archives[idx].name)
	}
	return nil
}

func (l *FileLogger) readTailLocked(maxBytes int64) ([]byte, error) {
	file, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return []byte{}, nil
	}

	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if start > 0 {
		for idx, b := range data {
			if b == '\n' {
				data = data[idx+1:]
				break
			}
		}
	}

	archiveSummary := l.archiveSummaryLocked()
	if archiveSummary == "" {
		return data, nil
	}

	prefix := []byte(archiveSummary + "\n\n")
	return append(prefix, data...), nil
}

func (l *FileLogger) archiveSummaryLocked() string {
	entries, err := os.ReadDir(l.archiveDir)
	if err != nil {
		return ""
	}

	count := 0
	var newest string
	var newestTime time.Time
	for _, item := range entries {
		if item.IsDir() {
			continue
		}
		count++
		info, err := item.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newest = item.Name()
		}
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("[Nhật ký cũ đã được tách lưu: %d file trong logs/archive, mới nhất: %s]", count, newest)
}
