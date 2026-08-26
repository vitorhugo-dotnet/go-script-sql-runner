package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Vendor string

const (
	VendorMySQL   Vendor = "mysql"
	VendorMariaDB Vendor = "mariadb"
)

type ServerCapabilities struct {
	Vendor       Vendor `json:"vendor"`
	Major        int    `json:"major"`
	Minor        int    `json:"minor"`
	Patch        int    `json:"patch"`
	RawVersion   string `json:"rawVersion"`
	VersionLabel string `json:"versionLabel"`
}

type ConnectionResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	Schemas      []string           `json:"schemas"`
}

type Client struct {
	DB           *sql.DB
	Capabilities ServerCapabilities
}

func (c *Client) Close() error {
	if c == nil || c.DB == nil {
		return nil
	}
	return c.DB.Close()
}

func (c *Client) UseSchema(ctx context.Context, schema string) error {
	if c == nil || c.DB == nil {
		return fmt.Errorf("database client is not available")
	}
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return fmt.Errorf("schema is required")
	}
	quoted := strings.ReplaceAll(schema, "`", "``")
	if _, err := c.DB.ExecContext(ctx, "USE `"+quoted+"`"); err != nil {
		return fmt.Errorf("select MySQL database %q: %w", schema, err)
	}
	return nil
}
