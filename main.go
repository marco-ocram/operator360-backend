package main

import (
	"fmt"
	"opt360-portal-backend/auth"
	"opt360-portal-backend/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load users configuration
	if err := auth.LoadUsersConfig(); err != nil {
		panic("Failed to load users.json: " + err.Error())
	}

	fmt.Printf("Loaded %d users from users.json\n", len(auth.UsersConfig.Users))

	// Initialize Gin router
	router := gin.Default()

	// Setup all routes
	routes.SetupRoutes(router)

	// Start server
	fmt.Println("Server starting on http://0.0.0.0:31151")
	router.Run("0.0.0.0:31151")
}


