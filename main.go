package main

import (
	"context"
	"fmt"
	"os"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/bootstrap"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/cli"
)

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

	if len(os.Args) == 1 {
		fmt.Println("Go Script SQL Runner GUI will be enabled in the desktop UI implementation stage.")
		return 0
	}
	return cli.Execute(context.Background(), runtime.Service, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
