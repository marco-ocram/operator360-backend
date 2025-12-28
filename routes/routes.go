package routes

import (

	"opt360-portal-backend/middleware"

	"github.com/gin-gonic/gin"
	"opt360-portal-backend/handlers/LandingPage"
	"opt360-portal-backend/handlers/OperatorTab"
	"opt360-portal-backend/handlers/OperatorDetailView"
	"opt360-portal-backend/handlers/SidReview"
	"opt360-portal-backend/handlers/Feedback"
	"opt360-portal-backend/handlers/AnamolyIndicators"
	"opt360-portal-backend/handlers/User"
)


func SetupRoutes(router *gin.Engine) {

	// Apply CORS middleware globally
	router.Use(middleware.CORS())

	// Protected routes - require authentication
	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
	
		api.GET("/user/info", User.GetUserInfo)
		api.GET("/kpi", LandingPage.GetKPIData)
		api.GET("/ro_risk_distribution", LandingPage.GetROQRiskDistribution)
		api.GET("/operator_list", OperatorTab.GetOperatorList)
		api.GET("/high_risk_operator", OperatorTab.GetHighRiskOperators)
		api.GET("/med_risk_operator", OperatorTab.GetMediumRiskOperators)
        api.GET("/low_risk_operator", OperatorTab.GetLowRiskOperators)
		api.GET("/operator_details", OperatorDetailView.GetOperatorDetails)
		api.GET("/search_operator_packets", SidReview.SearchOperatorPacketsBySID)
		api.GET("/anamolous_sids", SidReview.GetAnamolousSIDs)
		api.POST("/feedback", Feedback.SubmitFeedback)
		api.GET("/anamoly_indicators", AnamolyIndicators.GetAnamolyIndicators)
		api.GET("/operator_risk_details", OperatorDetailView.GetOperatorRiskDetails)
		

		
	}
}
