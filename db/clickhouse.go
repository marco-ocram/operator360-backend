package db

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

var (
	chDB     *sql.DB
	chDBOnce sync.Once
)

type ClickHouseConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Secure   bool
}

// InitClickHouseDB initialises the ClickHouse connection. Non-fatal: errors are
// logged but do not stop the server. Call GetClickHouseDB later to check if the
// client is available.
//
// Previously this built a raw "clickhouse://user:pass@host:port/db" DSN string by
// hand with fmt.Sprintf. That's broken for any username/password containing
// URL-reserved characters (":", "@", "/", "%", "#", ...) — they'd corrupt the
// DSN's parsing (wrong host/port, truncated password, etc.) and produce exactly
// a "connection test failed" symptom even when the host is genuinely reachable
// and the credentials are correct. This now builds the connection via the
// driver's typed clickhouse.Options struct instead, which takes the username/
// password as plain fields with no string-escaping involved.
func InitClickHouseDB(cfg ClickHouseConfig) {
	chDBOnce.Do(func() {
		if cfg.Host == "" {
			log.Printf("[ClickHouse] Skipping init: no host configured")
			return
		}

		addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

		// Pre-flight raw TCP reachability check, independent of the ClickHouse
		// wire protocol/TLS/auth handshake, purely to disambiguate failure causes
		// in the logs: a dial failure here means network/firewall; a dial success
		// followed by a Ping failure below means the network path is fine and the
		// problem is protocol, TLS, or credentials.
		if rawConn, dialErr := net.DialTimeout("tcp", addr, 5*time.Second); dialErr != nil {
			log.Printf("[ClickHouse] TCP dial to %s FAILED (network/firewall issue): %v", addr, dialErr)
		} else {
			rawConn.Close()
			log.Printf("[ClickHouse] TCP reachable at %s", addr)
		}

		opts := &clickhouse.Options{
			Addr: []string{addr},
			Auth: clickhouse.Auth{
				Database: cfg.Database,
				Username: cfg.Username,
				Password: cfg.Password,
			},
			DialTimeout: 5 * time.Second,
			ReadTimeout: 10 * time.Second,
		}
		if cfg.Secure {
			opts.TLS = &tls.Config{}
		}

		conn := clickhouse.OpenDB(opts)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := conn.PingContext(ctx); err != nil {
			log.Printf("[ClickHouse] Connection test FAILED (%s, secure=%v): %v", addr, cfg.Secure, err)
			if !cfg.Secure {
				log.Printf("[ClickHouse] Hint: if the TCP dial above succeeded but this ping still failed, the cluster may require TLS on the native protocol port — set clickhouse.secure=true in config.json")
			}
			conn.Close()
			return
		}

		chDB = conn
		log.Printf("[ClickHouse] Connection established (%s/%s, secure=%v)", addr, cfg.Database, cfg.Secure)
	})
}

// GetClickHouseDB returns the ClickHouse client, or an error if it was never
// successfully initialised.
func GetClickHouseDB() (*sql.DB, error) {
	if chDB == nil {
		return nil, fmt.Errorf("ClickHouse client not available (connection failed or not initialised)")
	}
	return chDB, nil
}

// CloseClickHouseDB closes the ClickHouse connection.
func CloseClickHouseDB() {
	if chDB != nil {
		chDB.Close()
	}
}
