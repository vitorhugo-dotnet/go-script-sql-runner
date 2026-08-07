package executor

import "testing"

func TestAnalyzeDetectsImplicitCommitDDL(t *testing.T) {
	for _, statement := range []string{
		"CREATE TABLE demo(id INT);",
		"  ALTER TABLE demo ADD name VARCHAR(10);",
		"DROP TABLE demo;",
		"TRUNCATE TABLE demo;",
		"RENAME TABLE demo TO demo2;",
	} {
		analysis, err := Analyze(statement)
		if err != nil {
			t.Fatalf("Analyze(%q) error: %v", statement, err)
		}
		if !analysis.HasImplicitCommitDDL {
			t.Fatalf("Analyze(%q) did not detect DDL", statement)
		}
	}
}

func TestAnalyzeRejectsDelimiterDirective(t *testing.T) {
	if _, err := Analyze("DELIMITER //\nCREATE PROCEDURE p() SELECT 1//"); err == nil {
		t.Fatal("Analyze() expected DELIMITER error")
	}
}

func TestAnalyzeAllowsOrdinaryDML(t *testing.T) {
	analysis, err := Analyze("INSERT INTO demo(id) VALUES (1); UPDATE demo SET id=2;")
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}
	if analysis.HasImplicitCommitDDL {
		t.Fatal("DML incorrectly classified as DDL")
	}
}
