package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"opt360-portal-backend/config"
	"opt360-portal-backend/db"
	"opt360-portal-backend/routes"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func checkSIDStoreConnectivity(baseURL string, timeoutSeconds int) {
	timeout := time.Duration(timeoutSeconds) * time.Second

	// 1. Is the server reachable? (TCP dial)
	parsed, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("[SID Store] Invalid base URL %q: %v", baseURL, err)
		return
	}
	host := parsed.Host
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		log.Printf("[SID Store] Server UNREACHABLE at %s: %v", host, err)
	} else {
		conn.Close()
		log.Printf("[SID Store] Server reachable at %s", host)
	}

	// 2. Is the API working? (probe the actual SID endpoint with a dummy value)
	apiURL := strings.TrimRight(baseURL, "/") + "/api/opt_details/sid/__probe__"
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("[SID Store] API endpoint UNREACHABLE at %s: %v", apiURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("[SID Store] API endpoint reachable but SID not found (HTTP 404) — endpoint is live at %s", apiURL)
	} else {
		log.Printf("[SID Store] API responding at %s (HTTP %d)", apiURL, resp.StatusCode)
	}
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config.json: ", err)
	}

	// Initialize database connection (operator360)
	dbConfig := db.DBConfig{
		User:     cfg.Opt360Database.User,
		Password: cfg.Opt360Database.Password,
		Host:     cfg.Opt360Database.Host,
		Port:     cfg.Opt360Database.Port,
		Database: cfg.Opt360Database.Database,
	}

	if err := db.InitDB(dbConfig); err != nil {
		log.Fatal("Failed to initialize database: ", err)
	}
	defer db.Close()

	fmt.Println("Database connection established")

	// Initialize UID database connection
	uidDBConfig := db.DBConfig{
		User:     cfg.UIDDatabase.User,
		Password: cfg.UIDDatabase.Password,
		Host:     cfg.UIDDatabase.Host,
		Port:     cfg.UIDDatabase.Port,
		Database: cfg.UIDDatabase.Database,
	}

	if err := db.InitUIDDB(uidDBConfig); err != nil {
		log.Fatal("Failed to initialize UID database: ", err)
	}

	fmt.Println("UID Database connection established")

	// Initialize portal database connection (strot_services — user auth)
	portalDBConfig := db.DBConfig{
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Database: cfg.Database.Database,
	}

	if err := db.InitPortalDB(portalDBConfig); err != nil {
		log.Fatal("Failed to initialize portal database: ", err)
	}

	fmt.Println("Portal Database connection established")

	// Non-fatal connectivity check for SID Store
	checkSIDStoreConnectivity(cfg.SIDStore.BaseURL, cfg.SIDStore.TimeoutSeconds)

	// Non-fatal ClickHouse connection (logs result; server starts regardless)
	db.InitClickHouseDB(db.ClickHouseConfig{
		Host:     cfg.ClickHouse.Host,
		Port:     cfg.ClickHouse.Port,
		Database: cfg.ClickHouse.Database,
		Username: cfg.ClickHouse.Username,
		Password: cfg.ClickHouse.Password,
	})
	defer db.CloseClickHouseDB()

	// Non-fatal Trino connection (logs result; server starts regardless)
	db.InitTrinoDB(db.TrinoConfig{
		Host:     cfg.Trino.Host,
		Port:     cfg.Trino.Port,
		Catalog:  cfg.Trino.Catalog,
		Schema:   cfg.Trino.Schema,
		Username: cfg.Trino.Username,
	})
	defer db.CloseTrinoDB()

	// Initialize Gin router
	router := gin.Default()

	// Setup all routes
	routes.SetupRoutes(router)

	// Start server - always bind to 0.0.0.0 in container environments
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}
	serverAddr := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Printf("Server starting on http://%s\n", serverAddr)
	router.Run(serverAddr)
}


