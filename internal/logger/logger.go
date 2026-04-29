package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileLogger struct {
	mu   sync.Mutex
	file *os.File
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
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileLogger{file: file}, nil
}

func (l *FileLogger) Info(event string, details map[string]any) {
	l.write("info", event, details)
}

func (l *FileLogger) Error(event string, details map[string]any) {
	l.write("error", event, details)
}

func (l *FileLogger) write(level, eventName string, details map[string]any) {
	if l == nil || l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_ = json.NewEncoder(l.file).Encode(entry{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Level:   level,
		Event:   eventName,
		Details: details,
	})
}

func (l *FileLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}
