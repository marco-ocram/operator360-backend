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

	// Initialize database connection
	dbConfig := db.DBConfig{
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Database: cfg.Database.Database,
	}

	if err := db.InitDB(dbConfig); err != nil {
		log.Fatal("Failed to initialize database: ", err)
	}
	defer db.Close()

	fmt.Println("Database connection established")

	// Initialize Gin router
	router := gin.Default()

	// Setup all routes
	routes.SetupRoutes(router)

	// Start server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.Host == "" {
		serverAddr = fmt.Sprintf("0.0.0.0:%d", cfg.Server.Port)
	}
	fmt.Printf("Server starting on http://%s\n", serverAddr)
	router.Run(serverAddr)
}


