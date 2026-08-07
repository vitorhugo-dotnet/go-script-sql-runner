package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDirectoryName = "GoScriptSQLRunner"

func DefaultRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, appDirectoryName), nil
}

func (r *Repository) Root() string {
	return r.root
}

func (r *Repository) ProfileDir(profileID string) string {
	return filepath.Join(r.root, "profiles", profileID)
}

func (r *Repository) ScriptRoot(profileID string) string {
	return filepath.Join(r.ProfileDir(profileID), "scripts")
}

func (r *Repository) LogsDir() string {
	return filepath.Join(r.root, "logs")
}
