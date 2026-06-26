package OperatorDetailView

import (
	"log"
	"net/http"

	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

type OperatorStatusRequest struct {
	UserCode string `json:"user_code" binding:"required"`
}

func GetOperatorStatus(c *gin.Context) {
	var req OperatorStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, http.StatusBadRequest, "Invalid request body", err, nil)
		return
	}

	if req.UserCode == "" {
		respond.Error(c, http.StatusBadRequest, "user_code is required", nil, nil)
		return
	}

	status, err := db.GetOperatorStatusByUserCode(req.UserCode)
	if err != nil {
		if err.Error() == "operator not found" {
			log.Printf("[GetOperatorStatus] Not found user_code=%s", req.UserCode)
			respond.Error(c, http.StatusNotFound, "Operator not found", nil, gin.H{"user_code": req.UserCode})
			return
		}
		log.Printf("[GetOperatorStatus] DB error user_code=%s: %v", req.UserCode, err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve operator status", err, nil)
		return
	}

	log.Printf("[GetOperatorStatus] Returning status for user_code=%s name=%s", req.UserCode, status.UserName)
	respond.OK(c, gin.H{
		"user_status": status.UserStatus,
		"user_name":   status.UserName,
		"user_uid":    status.UserUID,
	})
}
