package database

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

func ParseServerVersion(rawVersion, versionComment string) (ServerCapabilities, error) {
	rawVersion = strings.TrimSpace(rawVersion)
	if rawVersion == "" {
		return ServerCapabilities{}, fmt.Errorf("server version is blank")
	}
	matches := versionPattern.FindStringSubmatch(rawVersion)
	if len(matches) != 4 {
		return ServerCapabilities{}, fmt.Errorf("unrecognized server version %q", rawVersion)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return ServerCapabilities{}, fmt.Errorf("parse server major version: %w", err)
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return ServerCapabilities{}, fmt.Errorf("parse server minor version: %w", err)
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return ServerCapabilities{}, fmt.Errorf("parse server patch version: %w", err)
	}

	vendor := VendorMySQL
	label := ""
	vendorText := strings.ToLower(rawVersion + " " + versionComment)
	if strings.Contains(vendorText, "mariadb") {
		vendor = VendorMariaDB
		label = "MariaDB"
	} else {
		switch {
		case major == 5 && minor == 6:
			label = "MySQL 5.6"
		case major == 5 && minor == 7:
			label = "MySQL 5.7"
		case major == 8:
			label = "MySQL 8.x"
		default:
			return ServerCapabilities{}, fmt.Errorf("unsupported MySQL version %d.%d", major, minor)
		}
	}

	return ServerCapabilities{
		Vendor:       vendor,
		Major:        major,
		Minor:        minor,
		Patch:        patch,
		RawVersion:   rawVersion,
		VersionLabel: label,
	}, nil
}
