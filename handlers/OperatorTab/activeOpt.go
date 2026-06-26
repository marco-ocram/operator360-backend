package OperatorTab

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

// GetActiveOpt handles GET /api/active_opt
// Returns all users with user_status = 1 from uidmasterv1_1.user
func GetActiveOpt(c *gin.Context) {
	if _, ok := authctx.RequireUser(c); !ok {
		return
	}

	users, err := db.GetActiveUserCodes()
	if err != nil {
		log.Printf("[GetActiveOpt] %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch active operators", err, nil)
		return
	}

	log.Printf("[GetActiveOpt] Returning %d active operators", len(users))
	respond.OK(c, gin.H{"data": users, "count": len(users)})
}
