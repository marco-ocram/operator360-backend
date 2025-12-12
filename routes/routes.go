package routes

import (
	"opt360-portal-backend/handlers"
	"opt360-portal-backend/middleware"

	"github.com/gin-gonic/gin"
)


func SetupRoutes(router *gin.Engine) {

	// Apply CORS middleware globally
	router.Use(middleware.CORS())

	// Protected routes - require authentication
	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
	
		api.GET("/user/info", handlers.GetUserInfo)

	
		api.GET("/kpi", handlers.GetKPIData)
		api.GET("/operator_list", handlers.GetOperatorList)
		api.GET("/high_risk_operator", handlers.GetHighRiskOperators)
		api.GET("/med_risk_operator", handlers.GetMediumRiskOperators)
        api.GET("/low_risk_operator", handlers.GetLowRiskOperators)
		api.GET("/operator_details", handlers.GetOperatorDetails)

		
	}
}
