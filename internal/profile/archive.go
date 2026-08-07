package profile

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

const ArchiveFormatVersion = 1

type ArchiveManifest struct {
	FormatVersion int    `yaml:"format_version"`
	ProfileID     string `yaml:"profile_id"`
	ProfileName   string `yaml:"profile_name"`
}

type ArchiveInspection struct {
	Manifest ArchiveManifest
	Profile  Profile
}

func InspectArchive(archivePath string) (ArchiveInspection, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return ArchiveInspection{}, fmt.Errorf("open profile archive: %w", err)
	}
	defer reader.Close()

	entries := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		if !safeZipName(file.Name) {
			return ArchiveInspection{}, fmt.Errorf("unsafe archive entry %q", file.Name)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return ArchiveInspection{}, fmt.Errorf("archive entry %q is a symlink", file.Name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if _, duplicate := entries[file.Name]; duplicate {
			return ArchiveInspection{}, fmt.Errorf("duplicate archive entry %q", file.Name)
		}
		entries[file.Name] = file
	}

	manifestFile, ok := entries["manifest.yaml"]
	if !ok {
		return ArchiveInspection{}, fmt.Errorf("archive is missing manifest.yaml")
	}
	profileFile, ok := entries["profile.yaml"]
	if !ok {
		return ArchiveInspection{}, fmt.Errorf("archive is missing profile.yaml")
	}

	manifest, err := decodeArchiveManifest(manifestFile)
	if err != nil {
		return ArchiveInspection{}, err
	}
	if manifest.FormatVersion != ArchiveFormatVersion {
		return ArchiveInspection{}, fmt.Errorf("unsupported archive format version %d", manifest.FormatVersion)
	}
	if !safeID.MatchString(strings.TrimSpace(manifest.ProfileID)) {
		return ArchiveInspection{}, fmt.Errorf("archive manifest has invalid profile id")
	}
	if strings.TrimSpace(manifest.ProfileName) == "" {
		return ArchiveInspection{}, fmt.Errorf("archive manifest profile name is required")
	}

	p, err := decodeProfileEntry(profileFile)
	if err != nil {
		return ArchiveInspection{}, err
	}
	if manifest.ProfileID != p.ID {
		return ArchiveInspection{}, fmt.Errorf("archive manifest profile id %q does not match profile id %q", manifest.ProfileID, p.ID)
	}
	if manifest.ProfileName != p.Name {
		return ArchiveInspection{}, fmt.Errorf("archive manifest profile name %q does not match profile name %q", manifest.ProfileName, p.Name)
	}

	expected := make(map[string]struct{}, len(p.Scripts))
	for _, script := range p.Scripts {
		want := path.Join("scripts", script.ID+".sql")
		if script.File != want {
			return ArchiveInspection{}, fmt.Errorf("script %q must reference exactly %q", script.ID, want)
		}
		if _, exists := entries[want]; !exists {
			return ArchiveInspection{}, fmt.Errorf("archive is missing referenced script %q", want)
		}
		expected[want] = struct{}{}
	}

	for name := range entries {
		if name == "manifest.yaml" || name == "profile.yaml" {
			continue
		}
		if _, ok := expected[name]; !ok {
			return ArchiveInspection{}, fmt.Errorf("unexpected archive entry %q", name)
		}
	}

	return ArchiveInspection{Manifest: manifest, Profile: p}, nil
}

func safeZipName(name string) bool {
	if name == "" || strings.Contains(name, "\\") || strings.Contains(name, ":") {
		return false
	}
	if path.IsAbs(name) {
		return false
	}
	clean := path.Clean(name)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

func decodeArchiveManifest(file *zip.File) (ArchiveManifest, error) {
	reader, err := file.Open()
	if err != nil {
		return ArchiveManifest{}, fmt.Errorf("open manifest.yaml: %w", err)
	}
	defer reader.Close()

	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var manifest ArchiveManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ArchiveManifest{}, fmt.Errorf("decode manifest.yaml: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ArchiveManifest{}, fmt.Errorf("manifest.yaml contains multiple YAML documents")
		}
		return ArchiveManifest{}, fmt.Errorf("decode trailing manifest document: %w", err)
	}
	return manifest, nil
}

func decodeProfileEntry(file *zip.File) (Profile, error) {
	reader, err := file.Open()
	if err != nil {
		return Profile{}, fmt.Errorf("open profile.yaml: %w", err)
	}
	defer reader.Close()
	p, err := Decode(reader)
	if err != nil {
		return Profile{}, fmt.Errorf("decode archived profile.yaml: %w", err)
	}
	return p, nil
}
