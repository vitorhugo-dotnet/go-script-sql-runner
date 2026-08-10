package wailsui

import (
	"context"
	"fmt"
	"sync"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/ui"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/updatecheck"
)

type DesktopApp struct {
	bridge *ui.Bridge

	mu              sync.RWMutex
	ctx             context.Context
	currentBuildTag string
}

func NewDesktopApp(bridge *ui.Bridge) *DesktopApp {
	return &DesktopApp{
		bridge:          bridge,
		currentBuildTag: "dev",
	}
}

func (a *DesktopApp) Startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
}

func (a *DesktopApp) SetCurrentBuildTag(tag string) {
	if tag == "" {
		tag = "dev"
	}
	a.mu.Lock()
	a.currentBuildTag = tag
	a.mu.Unlock()
}

func (a *DesktopApp) appContext() (context.Context, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ctx == nil {
		return nil, fmt.Errorf("desktop runtime is not ready")
	}
	return a.ctx, nil
}

func (a *DesktopApp) CreateProfile(p profile.Profile) (profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return profile.Profile{}, err }
	return a.bridge.CreateProfile(ctx, p)
}

func (a *DesktopApp) ListProfiles() ([]profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return nil, err }
	return a.bridge.ListProfiles(ctx)
}

func (a *DesktopApp) GetProfile(profileID string) (profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return profile.Profile{}, err }
	return a.bridge.GetProfile(ctx, profileID)
}

func (a *DesktopApp) UpdateProfile(p profile.Profile) (profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return profile.Profile{}, err }
	return a.bridge.UpdateProfile(ctx, p)
}

func (a *DesktopApp) AddScriptFromDialog(profileID string) (*profile.Script, error) {
	ctx, err := a.appContext()
	if err != nil { return nil, err }
	return a.bridge.AddScriptFromDialog(ctx, profileID)
}

func (a *DesktopApp) AddScriptsFromDialog(profileID string) ([]profile.Script, error) {
	ctx, err := a.appContext()
	if err != nil { return nil, err }
	return a.bridge.AddScriptsFromDialog(ctx, profileID)
}

func (a *DesktopApp) RemoveScript(profileID, scriptID string) error {
	ctx, err := a.appContext()
	if err != nil { return err }
	return a.bridge.RemoveScript(ctx, profileID, scriptID)
}

func (a *DesktopApp) ReorderScripts(profileID string, orderedIDs []string) (profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return profile.Profile{}, err }
	return a.bridge.ReorderScripts(ctx, profileID, orderedIDs)
}

func (a *DesktopApp) SetScriptEnabled(profileID, scriptID string, enabled bool) (profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return profile.Profile{}, err }
	return a.bridge.SetScriptEnabled(ctx, profileID, scriptID, enabled)
}

func (a *DesktopApp) SetScriptTransactionMode(profileID, scriptID string, mode profile.TransactionMode) (profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return profile.Profile{}, err }
	return a.bridge.SetScriptTransactionMode(ctx, profileID, scriptID, mode)
}

func (a *DesktopApp) TestConnection(profileID string) (database.ConnectionResult, error) {
	ctx, err := a.appContext()
	if err != nil { return database.ConnectionResult{}, err }
	return a.bridge.TestConnection(ctx, profileID)
}

func (a *DesktopApp) RunProfile(profileID string, options executor.RunOptions) (executor.Summary, error) {
	ctx, err := a.appContext()
	if err != nil { return executor.Summary{}, err }
	return a.bridge.RunProfile(ctx, profileID, options)
}

func (a *DesktopApp) StopRun() bool {
	return a.bridge.StopRun()
}

func (a *DesktopApp) ImportProfileFromDialog() (*profile.Profile, error) {
	ctx, err := a.appContext()
	if err != nil { return nil, err }
	return a.bridge.ImportProfileFromDialog(ctx)
}

func (a *DesktopApp) ExportProfileToDialog(profileID string) (string, error) {
	ctx, err := a.appContext()
	if err != nil { return "", err }
	return a.bridge.ExportProfileToDialog(ctx, profileID)
}

func (a *DesktopApp) CheckForUpdates() (updatecheck.Result, error) {
	ctx, err := a.appContext()
	if err != nil {
		return updatecheck.Result{}, err
	}

	a.mu.RLock()
	currentBuildTag := a.currentBuildTag
	a.mu.RUnlock()

	return updatecheck.New(currentBuildTag).Check(ctx)
}
