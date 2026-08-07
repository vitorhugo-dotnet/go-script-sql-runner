package profile

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func Validate(p Profile) error {
	var errs []error

	if !safeID.MatchString(strings.TrimSpace(p.ID)) {
		errs = append(errs, errors.New("profile id must be a filesystem-safe identifier"))
	}
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, errors.New("profile name is required"))
	}
	if p.Version != 1 {
		errs = append(errs, fmt.Errorf("unsupported profile version %d", p.Version))
	}
	if strings.TrimSpace(p.Connection.Host) == "" {
		errs = append(errs, errors.New("connection host is required"))
	}
	if p.Connection.Port < 1 || p.Connection.Port > 65535 {
		errs = append(errs, errors.New("connection port must be between 1 and 65535"))
	}
	if strings.TrimSpace(p.Connection.Database) == "" {
		errs = append(errs, errors.New("connection database is required"))
	}
	if strings.TrimSpace(p.Connection.Username) == "" {
		errs = append(errs, errors.New("connection username is required"))
	}
	if !validOnError(p.Execution.OnError) {
		errs = append(errs, fmt.Errorf("invalid on_error value %q", p.Execution.OnError))
	}
	if !validTransactionMode(p.Execution.TransactionMode, false) {
		errs = append(errs, fmt.Errorf("invalid transaction_mode value %q", p.Execution.TransactionMode))
	}

	ids := make(map[string]struct{}, len(p.Scripts))
	orders := make(map[int]struct{}, len(p.Scripts))
	for i, script := range p.Scripts {
		prefix := fmt.Sprintf("script[%d]", i)
		if !safeID.MatchString(strings.TrimSpace(script.ID)) {
			errs = append(errs, fmt.Errorf("%s id must be a filesystem-safe identifier", prefix))
		}
		if _, exists := ids[script.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate script id %q", script.ID))
		}
		ids[script.ID] = struct{}{}

		if strings.TrimSpace(script.Name) == "" {
			errs = append(errs, fmt.Errorf("%s name is required", prefix))
		}
		if !validScriptPath(script.File) {
			errs = append(errs, fmt.Errorf("%s file must be inside scripts/", prefix))
		}
		if _, exists := orders[script.Order]; exists {
			errs = append(errs, fmt.Errorf("duplicate script order %d", script.Order))
		}
		orders[script.Order] = struct{}{}
		if !validTransactionMode(script.TransactionMode, true) {
			errs = append(errs, fmt.Errorf("%s has invalid transaction_mode %q", prefix, script.TransactionMode))
		}
	}

	return errors.Join(errs...)
}

func validOnError(value OnError) bool {
	return value == OnErrorContinue || value == OnErrorStop
}

func validTransactionMode(value TransactionMode, allowEmpty bool) bool {
	if allowEmpty && value == "" {
		return true
	}
	return value == TransactionAutoCommit || value == TransactionRunnerManaged || value == TransactionScriptManaged
}

func validScriptPath(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	normalized := strings.ReplaceAll(value, `\`, "/")
	if path.IsAbs(normalized) || strings.Contains(normalized, ":") {
		return false
	}
	cleaned := path.Clean(normalized)
	return cleaned != "scripts" && strings.HasPrefix(cleaned, "scripts/") && !strings.Contains(cleaned, "../")
}
