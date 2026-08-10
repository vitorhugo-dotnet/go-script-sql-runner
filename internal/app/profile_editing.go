package app

import (
	"context"
	"fmt"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func (s *Service) UpdateProfile(ctx context.Context, updated profile.Profile) (profile.Profile, error) {
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	existing, err := s.repository.Get(updated.ID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("get profile %q for update: %w", updated.ID, err)
	}
	updated.ID = existing.ID
	updated.Version = existing.Version
	updated.Connection.Database = ""
	updated.Scripts = append([]profile.Script(nil), existing.Scripts...)
	if err := s.repository.Save(updated); err != nil {
		return profile.Profile{}, fmt.Errorf("save updated profile %q: %w", existing.ID, err)
	}
	return s.repository.Get(existing.ID)
}

func (s *Service) ReorderScripts(ctx context.Context, profileID string, orderedIDs []string) (profile.Profile, error) {
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	if err := s.repository.ReorderScripts(profileID, orderedIDs); err != nil {
		return profile.Profile{}, fmt.Errorf("reorder scripts: %w", err)
	}
	p, err := s.repository.Get(profileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("reload reordered profile: %w", err)
	}
	return p, nil
}

func (s *Service) SetScriptEnabled(ctx context.Context, profileID, scriptID string, enabled bool) (profile.Profile, error) {
	return s.updateScript(ctx, profileID, scriptID, func(script *profile.Script) {
		script.Enabled = enabled
	})
}

func (s *Service) SetScriptTransactionMode(ctx context.Context, profileID, scriptID string, mode profile.TransactionMode) (profile.Profile, error) {
	return s.updateScript(ctx, profileID, scriptID, func(script *profile.Script) {
		script.TransactionMode = mode
	})
}

func (s *Service) updateScript(ctx context.Context, profileID, scriptID string, mutate func(*profile.Script)) (profile.Profile, error) {
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	p, err := s.repository.Get(profileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("get profile %q: %w", profileID, err)
	}
	found := false
	for i := range p.Scripts {
		if p.Scripts[i].ID == scriptID {
			mutate(&p.Scripts[i])
			found = true
			break
		}
	}
	if !found {
		return profile.Profile{}, fmt.Errorf("script %q not found in profile %q", scriptID, profileID)
	}
	if err := s.repository.Save(p); err != nil {
		return profile.Profile{}, fmt.Errorf("save script update: %w", err)
	}
	return s.repository.Get(profileID)
}
