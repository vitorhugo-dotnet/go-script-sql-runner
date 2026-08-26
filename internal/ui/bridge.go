package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

type DialogPort interface {
	OpenSQLFile(ctx context.Context) (string, error)
	OpenProfileZIP(ctx context.Context) (string, error)
	SaveProfileZIP(ctx context.Context, defaultName string) (string, error)
	ConfirmOverwrite(ctx context.Context, name string) (bool, error)
}

type MultiSQLDialogPort interface {
	OpenSQLFiles(ctx context.Context) ([]string, error)
}

type EventPort interface {
	EmitExecutionEvent(ctx context.Context, event executor.Event)
	EmitExecutionFinished(ctx context.Context, summary executor.Summary)
}

type ServicePort interface {
	CreateProfile(context.Context, profile.Profile) (profile.Profile, error)
	ListProfiles(context.Context) ([]profile.Profile, error)
	GetProfile(context.Context, string) (profile.Profile, error)
	UpdateProfile(context.Context, profile.Profile) (profile.Profile, error)
	AddScript(context.Context, string, string) (profile.Script, error)
	RemoveScript(context.Context, string, string) error
	ReorderScripts(context.Context, string, []string) (profile.Profile, error)
	SetScriptEnabled(context.Context, string, string, bool) (profile.Profile, error)
	SetScriptTransactionMode(context.Context, string, string, profile.TransactionMode) (profile.Profile, error)
	Connect(context.Context, string) (database.ConnectionResult, error)
	RunProfile(context.Context, string, executor.RunOptions, executor.Sink) (executor.Summary, error)
	InspectProfileArchive(context.Context, string) (profile.ArchiveInspection, error)
	ImportProfile(context.Context, string, bool) (profile.Profile, error)
	ExportProfile(context.Context, string, string) error
}

type Bridge struct {
	service ServicePort
	dialogs DialogPort
	events  EventPort

	runMu     sync.Mutex
	runCancel context.CancelFunc
}

func NewBridge(service ServicePort, dialogs DialogPort, events EventPort) *Bridge {
	return &Bridge{service: service, dialogs: dialogs, events: events}
}

func (b *Bridge) CreateProfile(ctx context.Context, p profile.Profile) (profile.Profile, error) {
	return b.service.CreateProfile(ctx, p)
}

func (b *Bridge) ListProfiles(ctx context.Context) ([]profile.Profile, error) {
	return b.service.ListProfiles(ctx)
}

func (b *Bridge) GetProfile(ctx context.Context, profileID string) (profile.Profile, error) {
	return b.service.GetProfile(ctx, profileID)
}

func (b *Bridge) UpdateProfile(ctx context.Context, p profile.Profile) (profile.Profile, error) {
	return b.service.UpdateProfile(ctx, p)
}

func (b *Bridge) AddScriptFromDialog(ctx context.Context, profileID string) (*profile.Script, error) {
	filename, err := b.dialogs.OpenSQLFile(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(filename) == "" {
		return nil, nil
	}
	script, err := b.service.AddScript(ctx, profileID, filename)
	if err != nil {
		return nil, err
	}
	return &script, nil
}

func (b *Bridge) AddScriptsFromDialog(ctx context.Context, profileID string) ([]profile.Script, error) {
	multi, ok := b.dialogs.(MultiSQLDialogPort)
	if !ok {
		script, err := b.AddScriptFromDialog(ctx, profileID)
		if err != nil || script == nil {
			return nil, err
		}
		return []profile.Script{*script}, nil
	}

	paths, err := multi.OpenSQLFiles(ctx)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	added := make([]profile.Script, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		script, err := b.service.AddScript(ctx, profileID, path)
		if err != nil {
			return added, err
		}
		added = append(added, script)
	}
	return added, nil
}

func (b *Bridge) RemoveScript(ctx context.Context, profileID, scriptID string) error {
	return b.service.RemoveScript(ctx, profileID, scriptID)
}

func (b *Bridge) ReorderScripts(ctx context.Context, profileID string, orderedIDs []string) (profile.Profile, error) {
	return b.service.ReorderScripts(ctx, profileID, orderedIDs)
}

func (b *Bridge) SetScriptEnabled(ctx context.Context, profileID, scriptID string, enabled bool) (profile.Profile, error) {
	return b.service.SetScriptEnabled(ctx, profileID, scriptID, enabled)
}

func (b *Bridge) SetScriptTransactionMode(ctx context.Context, profileID, scriptID string, mode profile.TransactionMode) (profile.Profile, error) {
	return b.service.SetScriptTransactionMode(ctx, profileID, scriptID, mode)
}

func (b *Bridge) Connect(ctx context.Context, profileID string) (database.ConnectionResult, error) {
	return b.service.Connect(ctx, profileID)
}

func (b *Bridge) RunProfile(ctx context.Context, profileID string, options executor.RunOptions) (executor.Summary, error) {
	b.runMu.Lock()
	if b.runCancel != nil {
		b.runMu.Unlock()
		return executor.Summary{}, fmt.Errorf("a profile execution is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	b.runCancel = cancel
	b.runMu.Unlock()

	defer func() {
		cancel()
		b.runMu.Lock()
		b.runCancel = nil
		b.runMu.Unlock()
	}()

	sink := executor.SinkFunc(func(event executor.Event) {
		if b.events != nil {
			b.events.EmitExecutionEvent(runCtx, event)
		}
	})
	summary, err := b.service.RunProfile(runCtx, profileID, options, sink)
	if b.events != nil {
		b.events.EmitExecutionFinished(ctx, summary)
	}
	return summary, err
}

func (b *Bridge) StopRun() bool {
	b.runMu.Lock()
	defer b.runMu.Unlock()
	if b.runCancel == nil {
		return false
	}
	b.runCancel()
	return true
}

func (b *Bridge) InspectProfileArchive(ctx context.Context, archivePath string) (profile.ArchiveInspection, error) {
	return b.service.InspectProfileArchive(ctx, archivePath)
}

func (b *Bridge) ImportProfileFromDialog(ctx context.Context) (*profile.Profile, error) {
	filename, err := b.dialogs.OpenProfileZIP(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(filename) == "" {
		return nil, nil
	}
	inspection, err := b.service.InspectProfileArchive(ctx, filename)
	if err != nil {
		return nil, err
	}
	imported, err := b.service.ImportProfile(ctx, filename, false)
	if err == nil {
		return &imported, nil
	}
	var conflict *app.ProfileConflictError
	if !errors.As(err, &conflict) {
		return nil, err
	}
	confirmed, err := b.dialogs.ConfirmOverwrite(ctx, inspection.Profile.Name)
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, nil
	}
	imported, err = b.service.ImportProfile(ctx, filename, true)
	if err != nil {
		return nil, err
	}
	return &imported, nil
}

func (b *Bridge) ExportProfileToDialog(ctx context.Context, profileID string) (string, error) {
	p, err := b.service.GetProfile(ctx, profileID)
	if err != nil {
		return "", err
	}
	destination, err := b.dialogs.SaveProfileZIP(ctx, p.ID+".zip")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(destination) == "" {
		return "", nil
	}
	if err := b.service.ExportProfile(ctx, profileID, destination); err != nil {
		return "", err
	}
	return destination, nil
}
