package wailsui

import (
	"context"
	"fmt"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
)

const (
	executionEventName    = "execution:event"
	executionFinishedName = "execution:finished"
)

type DialogAdapter struct{}

func (DialogAdapter) OpenSQLFile(ctx context.Context) (string, error) {
	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "Adicionar script SQL",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "SQL (*.sql)", Pattern: "*.sql"},
		},
	})
}

func (DialogAdapter) OpenProfileZIP(ctx context.Context) (string, error) {
	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "Importar profile",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Runner Profile (*.zip)", Pattern: "*.zip"},
		},
	})
}

func (DialogAdapter) SaveProfileZIP(ctx context.Context, defaultName string) (string, error) {
	if _, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.WarningDialog,
		Title:         "Profile contém credenciais",
		Message:       "O arquivo exportado contém host, usuário e senha do banco de dados em texto simples. Compartilhe somente pelo ambiente interno autorizado.",
		Buttons:       []string{"OK"},
		DefaultButton: "OK",
	}); err != nil {
		return "", err
	}
	return wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:           "Exportar profile",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Runner Profile (*.zip)", Pattern: "*.zip"},
		},
	})
}

func (DialogAdapter) ConfirmOverwrite(ctx context.Context, name string) (bool, error) {
	answer, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.WarningDialog,
		Title:         "Sobrescrever profile?",
		Message:       fmt.Sprintf("O profile %q já existe. A configuração e todos os scripts atuais serão substituídos.", name),
		Buttons:       []string{"Sobrescrever", "Cancelar"},
		DefaultButton: "Cancelar",
		CancelButton:  "Cancelar",
	})
	if err != nil {
		return false, err
	}
	return answer == "Sobrescrever", nil
}

type EventAdapter struct{}

func (EventAdapter) EmitExecutionEvent(ctx context.Context, event executor.Event) {
	wailsruntime.EventsEmit(ctx, executionEventName, event)
}

func (EventAdapter) EmitExecutionFinished(ctx context.Context, summary executor.Summary) {
	wailsruntime.EventsEmit(ctx, executionFinishedName, summary)
}
