package app

import (
	"context"
	"fmt"
)

func (s *Service) GetScriptContent(ctx context.Context, profileID, scriptID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("get script %q content: %w", scriptID, err)
	}
	content, err := s.repository.ReadScriptContent(profileID, scriptID)
	if err != nil {
		return "", fmt.Errorf("get script %q content: %w", scriptID, err)
	}
	return content, nil
}

func (s *Service) SaveScriptContent(ctx context.Context, profileID, scriptID, content string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save script %q content: %w", scriptID, err)
	}
	if err := s.repository.WriteScriptContent(profileID, scriptID, content); err != nil {
		return fmt.Errorf("save script %q content: %w", scriptID, err)
	}
	return nil
}
