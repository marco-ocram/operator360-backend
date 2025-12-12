package routes

import (
	"opt360-portal-backend/handlers"
	"opt360-portal-backend/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all application routes
func SetupRoutes(router *gin.Engine) {
	// Apply CORS middleware globally
	router.Use(middleware.CORS())

	// Public routes
	router.GET("/ping", handlers.Ping)

	// Protected routes - require authentication
	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		// User routes
		api.GET("/user/info", handlers.GetUserInfo)

		// KPI data route - fetches data based on user's regional office
		api.GET("/kpi", handlers.GetKPIData)

		// Add more protected routes here
		// api.GET("/data", handlers.GetData)
	}
}
