package User

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/middleware"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/session"

	"github.com/gin-gonic/gin"
)

func GetUserInfo(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}
	log.Printf("[GetUserInfo] Returning info for user=%s role=%s ro=%s", user.ADID, user.Role, user.RegionalOffice)
	c.JSON(http.StatusOK, user)
}

type UpdateRORequest struct {
	Group string `json:"group" binding:"required"`
}

func UpdateRO(c *gin.Context) {
	var req UpdateRORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, http.StatusBadRequest, "Invalid request body", err, nil)
		return
	}

	claims, exists := c.Get("token_claims")
	if !exists {
		respond.Error(c, http.StatusUnauthorized, "Token claims not found in context", nil, nil)
		return
	}

	tokenClaims, ok := claims.(*middleware.TokenClaims)
	if !ok {
		respond.Error(c, http.StatusInternalServerError, "Failed to parse token claims", nil, nil)
		return
	}

	userID := tokenClaims.Sub
	if userID == "" {
		respond.Error(c, http.StatusUnauthorized, "User ID not found in token", nil, nil)
		return
	}

	session.GetSessionManager().SetUserRegionalOffice(userID, req.Group)

	log.Printf("[UpdateRO] Session RO updated for user=%s to group=%s", userID, req.Group)
	respond.OK(c, gin.H{
		"message": "User regional office updated for session",
		"user_id": userID,
		"group":   req.Group,
	})
}
