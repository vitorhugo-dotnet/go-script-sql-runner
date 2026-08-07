package database

import "testing"

func TestParseServerVersion(t *testing.T) {
	tests := []struct {
		raw     string
		comment string
		vendor  Vendor
		major   int
		minor   int
		patch   int
		label   string
	}{
		{"5.6.51", "MySQL Community Server", VendorMySQL, 5, 6, 51, "MySQL 5.6"},
		{"5.7.44-log", "MySQL Community Server", VendorMySQL, 5, 7, 44, "MySQL 5.7"},
		{"8.0.43", "MySQL Community Server - GPL", VendorMySQL, 8, 0, 43, "MySQL 8.x"},
		{"10.11.8-MariaDB", "mariadb.org binary distribution", VendorMariaDB, 10, 11, 8, "MariaDB"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseServerVersion(tt.raw, tt.comment)
			if err != nil {
				t.Fatalf("ParseServerVersion() error: %v", err)
			}
			if got.Vendor != tt.vendor || got.Major != tt.major || got.Minor != tt.minor || got.Patch != tt.patch || got.VersionLabel != tt.label {
				t.Fatalf("ParseServerVersion() = %#v", got)
			}
		})
	}
}

func TestParseServerVersionRejectsUnsupported(t *testing.T) {
	for _, raw := range []string{"", "banana", "5.5.62", "9.0.1"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseServerVersion(raw, "MySQL"); err == nil {
				t.Fatalf("expected error for %q", raw)
			}
		})
	}
}
