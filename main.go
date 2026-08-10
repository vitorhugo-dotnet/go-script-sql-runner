package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/bootstrap"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/cli"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/ui"
	wailsui "github.com/vitorhugo-dotnet/go-script-sql-runner/internal/ui/wails"
)

//go:embed all:frontend/dist
var assets embed.FS

var currentBuildTag = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	runtime, err := bootstrap.NewDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "Error closing runtime:", err)
		}
	}()

	if len(os.Args) > 1 {
		return cli.Execute(context.Background(), runtime.Service, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	}

	wailsui.HideOwnConsoleForGUI()
	dialogs := wailsui.DialogAdapter{}
	events := wailsui.EventAdapter{}
	bridge := ui.NewBridge(runtime.Service, dialogs, events)
	desktop := wailsui.NewDesktopApp(bridge)
	desktop.SetCurrentBuildTag(currentBuildTag)

	err = wails.Run(&options.App{
		Title:         "Go Script SQL Runner",
		Width:         680,
		Height:        540,
		MinWidth:      560,
		MinHeight:     420,
		DisableResize: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: desktop.Startup,
		Bind:      []interface{}{desktop},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting desktop UI:", err)
		return 1
	}
	return 0
}
