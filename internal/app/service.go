package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/id"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

type serverConnectorFunc func(context.Context, profile.Connection) (*database.Client, database.ConnectionResult, error)

type Service struct {
	repository     *storage.Repository
	connectServer  serverConnectorFunc
	persistentSink executor.Sink

	connectionMu    sync.Mutex
	activeClient    *database.Client
	activeProfileID string
	closed          bool
}

// NewService creates the shared application service. A persistent execution
// sink may be supplied by production bootstrap; tests and lightweight callers
// may omit it.
func NewService(repository *storage.Repository, persistentSink ...executor.Sink) *Service {
	var sink executor.Sink
	if len(persistentSink) > 0 {
		sink = persistentSink[0]
	}
	return &Service{
		repository:     repository,
		connectServer:  database.ConnectServer,
		persistentSink: sink,
	}
}

func (s *Service) Connect(ctx context.Context, profileID string) (database.ConnectionResult, error) {
	s.connectionMu.Lock()
	closed := s.closed
	s.connectionMu.Unlock()
	if closed {
		return database.ConnectionResult{}, fmt.Errorf("service is closed")
	}
	p, err := s.repository.Get(profileID)
	if err != nil {
		return database.ConnectionResult{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	client, result, err := s.connectServer(ctx, p.Connection)
	if err != nil {
		return database.ConnectionResult{}, err
	}

	s.connectionMu.Lock()
	if s.closed {
		s.connectionMu.Unlock()
		return database.ConnectionResult{}, errors.Join(fmt.Errorf("service is closed"), client.Close())
	}
	previous := s.activeClient
	s.activeClient = client
	s.activeProfileID = profileID
	s.connectionMu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	return result, nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.connectionMu.Lock()
	s.closed = true
	client := s.activeClient
	s.activeClient = nil
	s.activeProfileID = ""
	s.connectionMu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}

func (s *Service) CreateProfile(_ context.Context, p profile.Profile) (profile.Profile, error) {
	if s == nil || s.repository == nil {
		return profile.Profile{}, fmt.Errorf("profile repository is not configured")
	}
	if p.ID == "" {
		generated, err := id.New()
		if err != nil {
			return profile.Profile{}, fmt.Errorf("generate profile id: %w", err)
		}
		p.ID = generated
	}
	if p.Version == 0 {
		p.Version = 1
	}
	if p.Connection.Port == 0 {
		p.Connection.Port = 3306
	}
	p.Connection.Database = ""
	if p.Execution.OnError == "" {
		p.Execution.OnError = profile.OnErrorContinue
	}
	if p.Execution.TransactionMode == "" {
		p.Execution.TransactionMode = profile.TransactionAutoCommit
	}
	if err := s.repository.Save(p); err != nil {
		return profile.Profile{}, fmt.Errorf("save profile: %w", err)
	}
	return p, nil
}

func (s *Service) ListProfiles(_ context.Context) ([]profile.Profile, error) {
	profiles, err := s.repository.List()
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	return profiles, nil
}

func (s *Service) GetProfile(_ context.Context, profileID string) (profile.Profile, error) {
	p, err := s.repository.Get(profileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	return p, nil
}

func (s *Service) AddScript(_ context.Context, profileID, sourcePath string) (profile.Script, error) {
	p, err := s.repository.Get(profileID)
	if err != nil {
		return profile.Script{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	if !strings.EqualFold(filepath.Ext(sourcePath), ".sql") {
		return profile.Script{}, fmt.Errorf("script must use the .sql extension")
	}
	generated, err := id.New()
	if err != nil {
		return profile.Script{}, fmt.Errorf("generate script id: %w", err)
	}
	order := 10
	for _, existing := range p.Scripts {
		if existing.Order >= order {
			order = existing.Order + 10
		}
	}
	base := filepath.Base(sourcePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.TrimSpace(name) == "" {
		name = base
	}
	script := profile.Script{
		ID:      generated,
		Name:    name,
		File:    filepath.ToSlash(filepath.Join("scripts", generated+".sql")),
		Enabled: true,
		Order:   order,
	}
	if err := s.repository.AddScript(profileID, script, sourcePath); err != nil {
		return profile.Script{}, fmt.Errorf("add script: %w", err)
	}
	return script, nil
}

func (s *Service) RemoveScript(_ context.Context, profileID, scriptID string) error {
	if err := s.repository.RemoveScript(profileID, scriptID); err != nil {
		return fmt.Errorf("remove script: %w", err)
	}
	return nil
}

func (s *Service) RunProfile(ctx context.Context, profileID string, opts executor.RunOptions, sink executor.Sink) (executor.Summary, error) {
	p, err := s.repository.Get(profileID)
	if err != nil {
		return executor.Summary{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	schema := strings.TrimSpace(opts.Schema)
	if schema == "" {
		return executor.Summary{}, fmt.Errorf("schema is required")
	}
	opts.Schema = schema
	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()
	client := s.activeClient
	if client == nil || s.activeProfileID != profileID {
		return executor.Summary{}, fmt.Errorf("connect profile %q before running", profileID)
	}
	if err := client.UseSchema(ctx, schema); err != nil {
		return executor.Summary{}, err
	}
	combined := fanOutSink{s.persistentSink, sink}
	return executor.Run(ctx, client, p, s.repository.ScriptRoot(profileID), opts, combined), nil
}

type fanOutSink []executor.Sink

func (sinks fanOutSink) Emit(event executor.Event) {
	for _, sink := range sinks {
		if sink != nil {
			sink.Emit(event)
		}
	}
}
