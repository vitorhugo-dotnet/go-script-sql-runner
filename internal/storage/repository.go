package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

var ErrNotFound = errors.New("profile not found")

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
	if err := profile.Validate(p); err != nil {
		return err
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
		if !entry.IsDir() {
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
	if err := os.RemoveAll(r.ProfileDir(id)); err != nil {
		return fmt.Errorf("delete profile: %w", err)
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
	if id == "" || id == "." || id == ".." {
		return false
	}
	return filepath.Base(id) == id && filepath.Clean(id) == id
}
