package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

const connectionTimeout = 10 * time.Second

func buildConfig(c profile.Connection, dbName string) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = c.Username
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	cfg.DBName = dbName
	cfg.MultiStatements = true
	cfg.Timeout = connectionTimeout
	cfg.ReadTimeout = connectionTimeout
	cfg.WriteTimeout = connectionTimeout
	return cfg
}

func Connect(ctx context.Context, c profile.Connection) (*Client, error) {
	probe, err := open(buildConfig(c, ""))
	if err != nil {
		return nil, fmt.Errorf("open MySQL server probe: %w", err)
	}
	if err := probe.PingContext(ctx); err != nil {
		_ = probe.Close()
		return nil, fmt.Errorf("connect to MySQL server: %w", err)
	}

	var rawVersion string
	var versionComment string
	if err := probe.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment").Scan(&rawVersion, &versionComment); err != nil {
		_ = probe.Close()
		return nil, fmt.Errorf("detect MySQL server version: %w", err)
	}
	capabilities, err := ParseServerVersion(rawVersion, versionComment)
	closeErr := probe.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close MySQL server probe: %w", closeErr)
	}

	db, err := open(buildConfig(c, c.Database))
	if err != nil {
		return nil, fmt.Errorf("open MySQL database %q: %w", c.Database, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("select MySQL database %q: %w", c.Database, err)
	}

	return &Client{DB: db, Capabilities: capabilities}, nil
}

func open(cfg *mysql.Config) (*sql.DB, error) {
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}
