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

		// High risk operators route - fetches parquet file based on user's regional office
		api.GET("/high_risk_operator", handlers.GetHighRiskOperators)

		// Medium risk operators route
		api.GET("/med_risk_operator", handlers.GetMediumRiskOperators)

		// Low risk operators route
		api.GET("/low_risk_operator", handlers.GetLowRiskOperators)

		// Add more protected routes here
		// api.GET("/data", handlers.GetData)
	}
}
