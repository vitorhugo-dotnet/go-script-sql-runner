package app

import (
	"context"
	"fmt"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/id"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func (s *Service) CloneProfile(ctx context.Context, profileID string) (profile.Profile, error) {
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, fmt.Errorf("clone profile %q: %w", profileID, err)
	}
	source, err := s.repository.Get(profileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("load profile %q for clone: %w", profileID, err)
	}
	clone := source
	clone.ID = ""
	clone.Name = source.Name + " (copy)"
	generated, err := id.New()
	if err != nil {
		return profile.Profile{}, fmt.Errorf("generate clone id for profile %q: %w", profileID, err)
	}
	clone.ID = generated
	if err := s.repository.Clone(profileID, clone); err != nil {
		return profile.Profile{}, fmt.Errorf("clone profile %q: %w", profileID, err)
	}
	return clone, nil
}

func (s *Service) DeleteProfile(ctx context.Context, profileID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete profile %q: %w", profileID, err)
	}
	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete profile %q: %w", profileID, err)
	}
	if err := s.repository.Delete(profileID); err != nil {
		return fmt.Errorf("delete profile %q: %w", profileID, err)
	}
	if s.activeProfileID == profileID {
		client := s.activeClient
		s.activeClient = nil
		s.activeProfileID = ""
		if err := client.Close(); err != nil {
			return fmt.Errorf("close connection for deleted profile %q: %w", profileID, err)
		}
	}
	return nil
}
