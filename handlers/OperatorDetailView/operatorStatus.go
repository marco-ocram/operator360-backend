package OperatorDetailView

import (
	"log"
	"net/http"

	"opt360-portal-backend/db"

	"github.com/gin-gonic/gin"
)

// OperatorStatusRequest represents the request body for operator status
type OperatorStatusRequest struct {
	UserCode string `json:"user_code" binding:"required"`
}

func GetOperatorStatus(c *gin.Context) {
	// Parse request body
	var req OperatorStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate that user_code is not empty
	if req.UserCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "user_code is required",
		})
		return
	}

	status, err := db.GetOperatorStatusByUserCode(req.UserCode)
	if err != nil {
		if err.Error() == "operator not found" {
			log.Printf("[GetOperatorStatus] Not found user_code=%s", req.UserCode)
			c.JSON(http.StatusNotFound, gin.H{"error": "Operator not found", "user_code": req.UserCode})
			return
		}
		log.Printf("[GetOperatorStatus] DB error user_code=%s: %v", req.UserCode, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve operator status", "details": err.Error()})
		return
	}

	log.Printf("[GetOperatorStatus] Returning status for user_code=%s name=%s", req.UserCode, status.UserName)
	c.JSON(http.StatusOK, gin.H{
		"user_status": status.UserStatus,
		"user_name":   status.UserName,
		"user_uid":    status.UserUID,
	})
}
