package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

var ErrNotFound = errors.New("profile not found")

var canonicalProfileID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Repository struct {
	root string
}

func NewRepository(root string) *Repository {
	return &Repository{root: root}
}

func (r *Repository) ensure() error {
	if err := os.MkdirAll(filepath.Join(r.root, "profiles"), 0o700); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}
	if err := os.MkdirAll(r.LogsDir(), 0o700); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}
	return nil
}

func (r *Repository) Save(p profile.Profile) error {
	if !validLookupID(p.ID) {
		return fmt.Errorf("invalid profile id")
	}
	if err := profile.Validate(p); err != nil {
		return err
	}
	if info, err := os.Lstat(r.ProfileDir(p.ID)); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("profile %q path is not a directory", p.ID)
		}
		if _, err := r.Get(p.ID); err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("load existing profile %q before save: %w", p.ID, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check existing profile %q: %w", p.ID, err)
	}
	if err := r.ensure(); err != nil {
		return err
	}
	if err := os.MkdirAll(r.ScriptRoot(p.ID), 0o700); err != nil {
		return fmt.Errorf("create script directory: %w", err)
	}

	var encoded bytes.Buffer
	if err := profile.Encode(&encoded, p); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(r.ProfileDir(p.ID), "profile.yaml"), encoded.Bytes(), 0o600)
}

func (r *Repository) Get(id string) (profile.Profile, error) {
	if !validLookupID(id) {
		return profile.Profile{}, fmt.Errorf("invalid profile id")
	}
	file, err := os.Open(filepath.Join(r.ProfileDir(id), "profile.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return profile.Profile{}, ErrNotFound
	}
	if err != nil {
		return profile.Profile{}, fmt.Errorf("open profile: %w", err)
	}
	defer file.Close()
	p, err := profile.Decode(file)
	if err != nil {
		return profile.Profile{}, err
	}
	if p.ID != id {
		return profile.Profile{}, fmt.Errorf("profile id mismatch: requested %q, stored %q", id, p.ID)
	}
	return p, nil
}

func (r *Repository) List() ([]profile.Profile, error) {
	if err := r.ensure(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(r.root, "profiles"))
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	profiles := make([]profile.Profile, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".clone-") {
			continue
		}
		p, err := r.Get(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("load profile %q: %w", entry.Name(), err)
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Name == profiles[j].Name {
			return profiles[i].ID < profiles[j].ID
		}
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

func (r *Repository) Delete(id string) error {
	if !validLookupID(id) {
		return fmt.Errorf("invalid profile id")
	}
	if _, err := r.Get(id); err != nil {
		return fmt.Errorf("load profile %q before delete: %w", id, err)
	}
	if err := os.RemoveAll(r.ProfileDir(id)); err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	return nil
}

// Clone copies a profile's managed SQL files before publishing the destination.
func (r *Repository) Clone(sourceID string, destination profile.Profile) error {
	if !validLookupID(sourceID) || !validLookupID(destination.ID) {
		return fmt.Errorf("invalid profile id")
	}
	if err := profile.Validate(destination); err != nil {
		return fmt.Errorf("validate clone destination: %w", err)
	}
	if sourceID == destination.ID {
		return fmt.Errorf("clone destination %q is the source profile", destination.ID)
	}
	source, err := r.Get(sourceID)
	if err != nil {
		return fmt.Errorf("load clone source %q: %w", sourceID, err)
	}
	if !reflect.DeepEqual(destination.Scripts, source.Scripts) {
		return fmt.Errorf("clone destination scripts differ from source %q", sourceID)
	}
	if err := r.ensure(); err != nil {
		return err
	}
	profilesRoot := filepath.Join(r.root, "profiles")
	destinationDir := r.ProfileDir(destination.ID)
	if _, err := os.Lstat(destinationDir); err == nil {
		return fmt.Errorf("clone destination %q already exists", destination.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check clone destination %q: %w", destination.ID, err)
	}

	sourceDir := r.ProfileDir(sourceID)
	sourceInfo, err := os.Lstat(sourceDir)
	if err != nil {
		return fmt.Errorf("stat clone source %q: %w", sourceID, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("clone source %q is not a directory", sourceID)
	}
	resolvedSource, err := filepath.EvalSymlinks(sourceDir)
	if err != nil {
		return fmt.Errorf("resolve clone source %q: %w", sourceID, err)
	}
	stagingDir, err := os.MkdirTemp(profilesRoot, ".clone-")
	if err != nil {
		return fmt.Errorf("create clone staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	for _, script := range source.Scripts {
		if !strings.EqualFold(filepath.Ext(script.File), ".sql") {
			return fmt.Errorf("clone script %q is not a SQL file", script.ID)
		}
		sourcePath := filepath.Join(sourceDir, filepath.FromSlash(script.File))
		rel, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("clone script %q escapes source profile", script.ID)
		}
		resolvedPath, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			return fmt.Errorf("resolve clone script %q: %w", script.ID, err)
		}
		resolvedRel, err := filepath.Rel(resolvedSource, resolvedPath)
		if err != nil || resolvedRel == "." || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("clone script %q escapes source profile", script.ID)
		}
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return fmt.Errorf("stat clone script %q: %w", script.ID, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("clone script %q is not a regular file", script.ID)
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read clone script %q: %w", script.ID, err)
		}
		if err := atomicWrite(filepath.Join(stagingDir, rel), data, 0o600); err != nil {
			return fmt.Errorf("write clone script %q: %w", script.ID, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(stagingDir, "scripts"), 0o700); err != nil {
		return fmt.Errorf("create clone script directory: %w", err)
	}
	var encoded bytes.Buffer
	if err := profile.Encode(&encoded, destination); err != nil {
		return fmt.Errorf("encode clone destination: %w", err)
	}
	if err := atomicWrite(filepath.Join(stagingDir, "profile.yaml"), encoded.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write clone profile: %w", err)
	}
	if err := os.Rename(stagingDir, destinationDir); err != nil {
		return fmt.Errorf("publish clone destination %q: %w", destination.ID, err)
	}
	return nil
}

func (r *Repository) AddScript(profileID string, script profile.Script, sourcePath string) error {
	p, err := r.Get(profileID)
	if err != nil {
		return err
	}
	if !validLookupID(script.ID) {
		return fmt.Errorf("invalid script id")
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source script: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat source script: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source script is not a regular file")
	}

	script.File = filepath.ToSlash(filepath.Join("scripts", script.ID+".sql"))
	p.Scripts = append(p.Scripts, script)
	if err := profile.Validate(p); err != nil {
		return err
	}

	if err := os.MkdirAll(r.ScriptRoot(profileID), 0o700); err != nil {
		return fmt.Errorf("create script directory: %w", err)
	}
	target := filepath.Join(r.ScriptRoot(profileID), script.ID+".sql")
	data, err := io.ReadAll(source)
	if err != nil {
		return fmt.Errorf("read source script: %w", err)
	}
	if err := atomicWrite(target, data, 0o600); err != nil {
		return fmt.Errorf("copy source script: %w", err)
	}
	if err := r.Save(p); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func (r *Repository) RemoveScript(profileID, scriptID string) error {
	p, err := r.Get(profileID)
	if err != nil {
		return err
	}
	index := -1
	var file string
	for i, script := range p.Scripts {
		if script.ID == scriptID {
			index = i
			file = script.File
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("script %q not found", scriptID)
	}

	original := append([]profile.Script(nil), p.Scripts...)
	p.Scripts = append(p.Scripts[:index], p.Scripts[index+1:]...)
	if err := r.Save(p); err != nil {
		p.Scripts = original
		return err
	}
	if err := os.Remove(filepath.Join(r.ProfileDir(profileID), filepath.FromSlash(file))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove script file: %w", err)
	}
	return nil
}

func (r *Repository) ReorderScripts(profileID string, orderedIDs []string) error {
	p, err := r.Get(profileID)
	if err != nil {
		return err
	}
	if len(orderedIDs) != len(p.Scripts) {
		return fmt.Errorf("reorder requires every script exactly once")
	}
	byID := make(map[string]*profile.Script, len(p.Scripts))
	for i := range p.Scripts {
		byID[p.Scripts[i].ID] = &p.Scripts[i]
	}
	seen := make(map[string]struct{}, len(orderedIDs))
	for i, id := range orderedIDs {
		script, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown script id %q", id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("duplicate script id %q in reorder", id)
		}
		seen[id] = struct{}{}
		script.Order = (i + 1) * 10
	}
	return r.Save(p)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".runner-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	defer cleanup()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func validLookupID(id string) bool {
	if !canonicalProfileID.MatchString(id) || strings.HasSuffix(id, ".") {
		return false
	}
	stem := strings.ToUpper(strings.SplitN(id, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return false
	}
	if len(stem) == 4 && (stem[:3] == "COM" || stem[:3] == "LPT") && stem[3] >= '1' && stem[3] <= '9' {
		return false
	}
	return filepath.Base(id) == id && filepath.Clean(id) == id
}
