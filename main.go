package main

import (
	"fmt"
	"log"
	"opt360-portal-backend/config"
	"opt360-portal-backend/db"
	"opt360-portal-backend/routes"

	"github.com/gin-gonic/gin"
)

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


