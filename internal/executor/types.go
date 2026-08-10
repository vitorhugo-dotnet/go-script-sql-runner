package executor

import (
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Event struct {
	Time     time.Time `json:"time"`
	Level    Level     `json:"level"`
	ScriptID string    `json:"scriptId,omitempty"`
	Message  string    `json:"message"`
	Detail   string    `json:"detail,omitempty"`
}

type Sink interface {
	Emit(Event)
}

type SinkFunc func(Event)

func (f SinkFunc) Emit(event Event) {
	if f != nil {
		f(event)
	}
}

type ScriptResult struct {
	ScriptID  string    `json:"scriptId"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

type Summary struct {
	Results   []ScriptResult `json:"results"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Aborted   bool           `json:"aborted"`
}

type RunOptions struct {
	// Schema is the runtime database/schema selected for this execution.
	Schema string `json:"schema"`
	// OnError overrides the profile only when non-empty.
	OnError profile.OnError `json:"onError,omitempty"`
	// TransactionMode overrides the profile default only when non-empty.
	// Per-script overrides still take precedence.
	TransactionMode profile.TransactionMode `json:"transactionMode,omitempty"`
}
