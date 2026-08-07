package profile

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/id"
	"go.yaml.in/yaml/v3"
)

var exportRename = os.Rename

// ExportArchive writes a self-contained profile archive. Connection credentials
// intentionally remain in profile.yaml because exported profiles are designed
// for controlled internal sharing.
func ExportArchive(profileDir, destination string) error {
	profileBytes, p, err := loadCanonicalProfile(profileDir)
	if err != nil {
		return err
	}

	scripts := append([]Script(nil), p.Scripts...)
	sort.Slice(scripts, func(i, j int) bool {
		if scripts[i].Order == scripts[j].Order {
			return scripts[i].ID < scripts[j].ID
		}
		return scripts[i].Order < scripts[j].Order
	})
	for _, script := range scripts {
		want := path.Join("scripts", script.ID+".sql")
		if script.File != want {
			return fmt.Errorf("script %q must reference exactly %q", script.ID, want)
		}
		if err := requireRegularFile(filepath.Join(profileDir, filepath.FromSlash(script.File))); err != nil {
			return fmt.Errorf("validate script %q: %w", script.ID, err)
		}
	}

	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}
	tempPath := destination + ".tmp"
	_ = os.Remove(tempPath)
	defer os.Remove(tempPath)

	if err := writeArchive(tempPath, profileBytes, p, scripts, profileDir); err != nil {
		return err
	}
	if _, err := InspectArchive(tempPath); err != nil {
		return fmt.Errorf("validate generated profile archive: %w", err)
	}
	if err := replaceExportDestination(tempPath, destination); err != nil {
		return err
	}
	return nil
}

func loadCanonicalProfile(profileDir string) ([]byte, Profile, error) {
	profilePath := filepath.Join(profileDir, "profile.yaml")
	if err := requireRegularFile(profilePath); err != nil {
		return nil, Profile{}, fmt.Errorf("validate profile.yaml: %w", err)
	}
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, Profile{}, fmt.Errorf("read profile.yaml: %w", err)
	}
	p, err := Decode(bytes.NewReader(data))
	if err != nil {
		return nil, Profile{}, err
	}
	return data, p, nil
}

func requireRegularFile(filename string) error {
	info, err := os.Lstat(filename)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", filename)
	}
	return nil
}

func writeArchive(filename string, profileBytes []byte, p Profile, scripts []Script, profileDir string) (returnErr error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary profile archive: %w", err)
	}
	writer := zip.NewWriter(file)
	defer func() {
		if err := writer.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close profile archive: %w", err)
		}
		if err := file.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close profile archive file: %w", err)
		}
	}()

	manifestBytes, err := yaml.Marshal(ArchiveManifest{
		FormatVersion: ArchiveFormatVersion,
		ProfileID:     p.ID,
		ProfileName:   p.Name,
	})
	if err != nil {
		return fmt.Errorf("encode archive manifest: %w", err)
	}
	if err := writeZipFile(writer, "manifest.yaml", manifestBytes); err != nil {
		return err
	}
	if err := writeZipFile(writer, "profile.yaml", profileBytes); err != nil {
		return err
	}
	for _, script := range scripts {
		data, err := os.ReadFile(filepath.Join(profileDir, filepath.FromSlash(script.File)))
		if err != nil {
			return fmt.Errorf("read script %q: %w", script.ID, err)
		}
		if err := writeZipFile(writer, script.File, data); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(writer *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	header.SetMode(0o600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create archive entry %q: %w", name, err)
	}
	if _, err := entry.Write(data); err != nil {
		return fmt.Errorf("write archive entry %q: %w", name, err)
	}
	return nil
}

func replaceExportDestination(tempPath, destination string) error {
	_, statErr := os.Stat(destination)
	if errors.Is(statErr, os.ErrNotExist) {
		if err := exportRename(tempPath, destination); err != nil {
			return fmt.Errorf("publish profile archive: %w", err)
		}
		return nil
	}
	if statErr != nil {
		return fmt.Errorf("inspect export destination: %w", statErr)
	}

	suffix, err := id.New()
	if err != nil {
		return fmt.Errorf("generate export backup id: %w", err)
	}
	backup := destination + ".bak-" + suffix
	if err := exportRename(destination, backup); err != nil {
		return fmt.Errorf("backup existing profile archive: %w", err)
	}
	if err := exportRename(tempPath, destination); err != nil {
		restoreErr := exportRename(backup, destination)
		return errors.Join(fmt.Errorf("publish replacement profile archive: %w", err), restoreErr)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("remove previous profile archive backup: %w", err)
	}
	return nil
}
