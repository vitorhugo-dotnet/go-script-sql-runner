package database

import "database/sql"

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
