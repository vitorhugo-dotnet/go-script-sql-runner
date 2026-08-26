package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
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

// ConnectServer opens a persistent server session without selecting a default
// database, detects server capabilities, and returns visible schemas.
func ConnectServer(ctx context.Context, c profile.Connection) (*Client, ConnectionResult, error) {
	db, err := open(buildConfig(c, ""))
	if err != nil {
		return nil, ConnectionResult{}, fmt.Errorf("open MySQL server connection: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, ConnectionResult{}, fmt.Errorf("connect to MySQL server: %w", err)
	}

	capabilities, err := detectCapabilities(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, ConnectionResult{}, err
	}
	schemas, err := listSchemas(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, ConnectionResult{}, err
	}
	client := &Client{DB: db, Capabilities: capabilities}
	return client, ConnectionResult{Capabilities: capabilities, Schemas: schemas}, nil
}

// Connect keeps the legacy package API for integration callers. New runtime
// execution paths should call ConnectToDatabase with an explicit schema.
func Connect(ctx context.Context, c profile.Connection) (*Client, error) {
	return ConnectToDatabase(ctx, c, c.Database)
}

// ConnectToDatabase opens a connection using the explicitly selected runtime schema.
func ConnectToDatabase(ctx context.Context, c profile.Connection, schema string) (*Client, error) {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return nil, fmt.Errorf("database/schema is required")
	}

	db, err := open(buildConfig(c, schema))
	if err != nil {
		return nil, fmt.Errorf("open MySQL database %q: %w", schema, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("select MySQL database %q: %w", schema, err)
	}
	capabilities, err := detectCapabilities(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Client{DB: db, Capabilities: capabilities}, nil
}

func detectCapabilities(ctx context.Context, db *sql.DB) (ServerCapabilities, error) {
	var rawVersion string
	var versionComment string
	if err := db.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment").Scan(&rawVersion, &versionComment); err != nil {
		return ServerCapabilities{}, fmt.Errorf("detect MySQL server version: %w", err)
	}
	capabilities, err := ParseServerVersion(rawVersion, versionComment)
	if err != nil {
		return ServerCapabilities{}, err
	}
	return capabilities, nil
}

func listSchemas(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("list MySQL schemas: %w", err)
	}
	defer rows.Close()

	schemas := make([]string, 0)
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, fmt.Errorf("scan MySQL schema: %w", err)
		}
		schemas = append(schemas, schema)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate MySQL schemas: %w", err)
	}
	return schemas, nil
}

func open(cfg *mysql.Config) (*sql.DB, error) {
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}
