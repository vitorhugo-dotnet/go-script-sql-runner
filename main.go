package main

import (
	"fmt"
	"testing/fstest"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var scaffoldAssets = fstest.MapFS{
	"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><html><body><main>Go Script SQL Runner</main></body></html>`)},
}

func main() {
	err := wails.Run(&options.App{
		Title:     "Go Script SQL Runner",
		Width:     680,
		Height:    540,
		MinWidth:  560,
		MinHeight: 420,
		AssetServer: &assetserver.Options{
			Assets: scaffoldAssets,
		},
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}
