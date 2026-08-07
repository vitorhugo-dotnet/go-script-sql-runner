package executor

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

var ddlPattern = regexp.MustCompile(`(?im)^\s*(CREATE|ALTER|DROP|TRUNCATE|RENAME)\b`)
var delimiterPattern = regexp.MustCompile(`(?i)^\s*DELIMITER\b`)

type Analysis struct {
	HasImplicitCommitDDL bool
}

// Analyze performs intentionally small preflight checks. It is not a SQL parser.
func Analyze(sqlText string) (Analysis, error) {
	scanner := bufio.NewScanner(strings.NewReader(sqlText))
	for scanner.Scan() {
		if delimiterPattern.MatchString(scanner.Text()) {
			return Analysis{}, fmt.Errorf("unsupported MySQL client directive DELIMITER; use server-executable SQL without client directives")
		}
	}
	if err := scanner.Err(); err != nil {
		return Analysis{}, fmt.Errorf("scan SQL preflight: %w", err)
	}
	return Analysis{HasImplicitCommitDDL: ddlPattern.MatchString(sqlText)}, nil
}
