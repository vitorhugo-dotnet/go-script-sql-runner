package main

import (
	"context"
	"fmt"
	"os"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/cli"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("Go Script SQL Runner GUI will be enabled in the desktop UI implementation stage.")
		return
	}
	root, err := storage.DefaultRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	service := app.NewService(storage.NewRepository(root))
	os.Exit(cli.Execute(context.Background(), service, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
