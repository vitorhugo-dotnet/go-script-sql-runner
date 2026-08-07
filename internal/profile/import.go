package profile

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/id"
)

var ErrUnsafeArchivePath = errors.New("unsafe archive path")

// StageArchive validates the whole archive before creating any staging files,
// then extracts it beneath a unique staging directory.
func StageArchive(archivePath, stagingRoot string) (stagedProfileDir string, inspection ArchiveInspection, returnErr error) {
	inspection, err := InspectArchive(archivePath)
	if err != nil {
		return "", ArchiveInspection{}, err
	}
	if err := os.MkdirAll(stagingRoot, 0o700); err != nil {
		return "", inspection, fmt.Errorf("create import staging root: %w", err)
	}
	suffix, err := id.New()
	if err != nil {
		return "", inspection, fmt.Errorf("generate import staging id: %w", err)
	}
	stagedProfileDir = filepath.Join(stagingRoot, inspection.Profile.ID+"-"+suffix)
	if err := os.MkdirAll(stagedProfileDir, 0o700); err != nil {
		return "", inspection, fmt.Errorf("create import staging directory: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = os.RemoveAll(stagedProfileDir)
		}
	}()

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", inspection, fmt.Errorf("reopen validated profile archive: %w", err)
	}
	defer reader.Close()

	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		// manifest.yaml validates the transport envelope but is not part of the
		// canonical AppData profile directory.
		if entry.Name == "manifest.yaml" {
			continue
		}
		destination := filepath.Join(stagedProfileDir, filepath.FromSlash(entry.Name))
		relative, err := filepath.Rel(stagedProfileDir, destination)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return "", inspection, ErrUnsafeArchivePath
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", inspection, fmt.Errorf("create staged archive directory: %w", err)
		}
		if err := extractArchiveFile(entry, destination); err != nil {
			return "", inspection, err
		}
	}
	return stagedProfileDir, inspection, nil
}

func extractArchiveFile(entry *zip.File, destination string) error {
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open archive entry %q: %w", entry.Name, err)
	}
	defer source.Close()
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create staged archive entry %q: %w", entry.Name, err)
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return fmt.Errorf("extract archive entry %q: %w", entry.Name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close staged archive entry %q: %w", entry.Name, closeErr)
	}
	return nil
}
