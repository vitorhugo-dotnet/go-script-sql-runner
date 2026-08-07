package executor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func executeSQL(ctx context.Context, db *sql.DB, sqlText string, mode profile.TransactionMode) error {
	switch mode {
	case profile.TransactionAutoCommit, profile.TransactionScriptManaged:
		if _, err := db.ExecContext(ctx, sqlText); err != nil {
			return err
		}
		return nil
	case profile.TransactionRunnerManaged:
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		if _, err = tx.ExecContext(ctx, sqlText); err != nil {
			rollbackErr := tx.Rollback()
			return errors.Join(err, rollbackErr)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported transaction mode %q", mode)
	}
}
