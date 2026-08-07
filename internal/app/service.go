package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/id"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

type connectorFunc func(context.Context, profile.Connection) (*database.Client, error)

type Service struct {
	repository *storage.Repository
	connect    connectorFunc
}

func NewService(repository *storage.Repository) *Service {
	return &Service{repository: repository, connect: database.Connect}
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

func (s *Service) TestConnection(ctx context.Context, profileID string) (database.ServerCapabilities, error) {
	p, err := s.repository.Get(profileID)
	if err != nil {
		return database.ServerCapabilities{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	client, err := s.connect(ctx, p.Connection)
	if err != nil {
		return database.ServerCapabilities{}, err
	}
	defer client.Close()
	return client.Capabilities, nil
}

func (s *Service) RunProfile(ctx context.Context, profileID string, opts executor.RunOptions, sink executor.Sink) (executor.Summary, error) {
	p, err := s.repository.Get(profileID)
	if err != nil {
		return executor.Summary{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	client, err := s.connect(ctx, p.Connection)
	if err != nil {
		return executor.Summary{}, err
	}
	defer client.Close()
	return executor.Run(ctx, client, p, s.repository.ScriptRoot(profileID), opts, sink), nil
}
