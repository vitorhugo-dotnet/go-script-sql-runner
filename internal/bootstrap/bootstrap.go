package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
	applog "github.com/vitorhugo-dotnet/go-script-sql-runner/internal/logging"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

type Runtime struct {
	Service *app.Service
	closers []io.Closer
}

func NewDefault() (*Runtime, error) {
	root, err := storage.DefaultRoot()
	if err != nil {
		return nil, err
	}
	return newRuntime(root)
}

func newRuntime(root string) (*Runtime, error) {
	for _, dir := range []string{root, filepath.Join(root, "profiles"), filepath.Join(root, "logs")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create runtime directory %q: %w", dir, err)
		}
	}
	repository := storage.NewRepository(root)
	fileSink, err := applog.NewFileSink(filepath.Join(root, "logs"))
	if err != nil {
		return nil, err
	}
	service := app.NewService(repository, fileSink)
	return &Runtime{Service: service, closers: []io.Closer{service, fileSink}}, nil
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var errs []error
	for i := len(r.closers) - 1; i >= 0; i-- {
		if r.closers[i] == nil {
			continue
		}
		if err := r.closers[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
