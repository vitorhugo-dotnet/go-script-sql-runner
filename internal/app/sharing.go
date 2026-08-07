package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

type ProfileConflictError struct {
	Existing profile.Profile
	Incoming profile.Profile
}

func (e *ProfileConflictError) Error() string {
	return fmt.Sprintf("profile %q (%s) already exists", e.Incoming.Name, e.Incoming.ID)
}

func (s *Service) InspectProfileArchive(ctx context.Context, archivePath string) (profile.ArchiveInspection, error) {
	if err := ctx.Err(); err != nil {
		return profile.ArchiveInspection{}, err
	}
	inspection, err := profile.InspectArchive(archivePath)
	if err != nil {
		return profile.ArchiveInspection{}, fmt.Errorf("inspect profile archive: %w", err)
	}
	return inspection, nil
}

func (s *Service) ExportProfile(ctx context.Context, profileID, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.repository.Get(profileID); err != nil {
		return fmt.Errorf("get profile %q for export: %w", profileID, err)
	}
	if err := profile.ExportArchive(s.repository.ProfileDir(profileID), destination); err != nil {
		return fmt.Errorf("export profile %q: %w", profileID, err)
	}
	return nil
}

func (s *Service) ImportProfile(ctx context.Context, archivePath string, overwrite bool) (profile.Profile, error) {
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	inspection, err := profile.InspectArchive(archivePath)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("inspect profile archive: %w", err)
	}

	existing, getErr := s.repository.Get(inspection.Profile.ID)
	switch {
	case getErr == nil && !overwrite:
		return profile.Profile{}, &ProfileConflictError{Existing: existing, Incoming: inspection.Profile}
	case getErr != nil && !errors.Is(getErr, storage.ErrNotFound):
		return profile.Profile{}, fmt.Errorf("check existing profile %q: %w", inspection.Profile.ID, getErr)
	}
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}

	stagingRoot := filepath.Join(s.repository.Root(), ".imports")
	staged, _, err := profile.StageArchive(archivePath, stagingRoot)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("stage profile archive: %w", err)
	}
	cleanupStaged := true
	defer func() {
		if cleanupStaged {
			_ = os.RemoveAll(staged)
		}
		_ = os.Remove(stagingRoot)
	}()

	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	if err := storage.ReplaceDir(staged, s.repository.ProfileDir(inspection.Profile.ID)); err != nil {
		return profile.Profile{}, fmt.Errorf("replace profile %q: %w", inspection.Profile.ID, err)
	}
	cleanupStaged = false

	imported, err := s.repository.Get(inspection.Profile.ID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("load imported profile %q: %w", inspection.Profile.ID, err)
	}
	return imported, nil
}
