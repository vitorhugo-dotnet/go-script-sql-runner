package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
)

type FileSink struct {
	mu     sync.Mutex
	file   *os.File
	logger *slog.Logger
}

func NewFileSink(logDir string) (*FileSink, error) {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	filename := filepath.Join(logDir, "runner-"+time.Now().Format("2006-01-02")+".log")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open execution log: %w", err)
	}
	logger := slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return &FileSink{file: file, logger: logger}, nil
}

func (s *FileSink) Emit(event executor.Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.logger == nil {
		return
	}
	attrs := []any{
		"event_time", event.Time.Format(time.RFC3339Nano),
		"script_id", event.ScriptID,
		"message", event.Message,
		"detail", event.Detail,
	}
	switch event.Level {
	case executor.LevelError:
		s.logger.Error("execution_event", attrs...)
	case executor.LevelWarn:
		s.logger.Warn("execution_event", attrs...)
	default:
		s.logger.Info("execution_event", attrs...)
	}
}

func (s *FileSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	s.logger = nil
	if err != nil {
		return fmt.Errorf("close execution log: %w", err)
	}
	return nil
}
