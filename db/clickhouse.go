package db

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/ClickHouse/clickhouse-go/v2"
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
}

// InitClickHouseDB initialises the ClickHouse connection. Non-fatal: errors are
// logged but do not stop the server. Call GetClickHouseDB later to check if the
// client is available.
func InitClickHouseDB(cfg ClickHouseConfig) {
	chDBOnce.Do(func() {
		dsn := fmt.Sprintf(
			"clickhouse://%s:%s@%s:%d/%s?dial_timeout=5s&read_timeout=10s",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
		)

		db, err := sql.Open("clickhouse", dsn)
		if err != nil {
			log.Printf("[ClickHouse] Failed to open connection: %v", err)
			return
		}

		if err := db.Ping(); err != nil {
			log.Printf("[ClickHouse] Connection test FAILED (%s:%d): %v", cfg.Host, cfg.Port, err)
			db.Close()
			return
		}

		chDB = db
		log.Printf("[ClickHouse] Connection established (%s:%d/%s)", cfg.Host, cfg.Port, cfg.Database)
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
