package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/id"
)

var rename = os.Rename

// ReplaceDir replaces target with staged using same-volume renames. If target
// exists it is first moved to a sibling backup and restored if publishing the
// staged directory fails.
func ReplaceDir(staged, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create target parent: %w", err)
	}
	stagedInfo, err := os.Stat(staged)
	if err != nil {
		return fmt.Errorf("inspect staged directory: %w", err)
	}
	if !stagedInfo.IsDir() {
		return fmt.Errorf("staged path is not a directory")
	}

	_, statErr := os.Stat(target)
	if errors.Is(statErr, os.ErrNotExist) {
		if err := rename(staged, target); err != nil {
			return fmt.Errorf("publish staged profile: %w", err)
		}
		return nil
	}
	if statErr != nil {
		return fmt.Errorf("inspect target profile: %w", statErr)
	}

	suffix, err := id.New()
	if err != nil {
		return fmt.Errorf("generate profile backup id: %w", err)
	}
	backup := target + ".backup-" + suffix
	if err := rename(target, backup); err != nil {
		return fmt.Errorf("backup existing profile: %w", err)
	}
	if err := rename(staged, target); err != nil {
		restoreErr := rename(backup, target)
		if restoreErr != nil {
			return errors.Join(
				fmt.Errorf("publish staged profile: %w", err),
				fmt.Errorf("restore previous profile: %w", restoreErr),
			)
		}
		return fmt.Errorf("publish staged profile: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous profile backup: %w", err)
	}
	return nil
}
