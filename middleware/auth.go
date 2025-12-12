package middleware

import (
	"net/http"
	"opt360-portal-backend/auth"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the user and attaches user info to context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		adID := c.GetHeader("X-User-AD-ID")

		if adID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing X-User-AD-ID header",
			})
			c.Abort()
			return
		}

		user, found := auth.GetUserByADID(adID)
		if !found {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "User not authorized",
				"message": "AD-ID not found in system",
			})
			c.Abort()
			return
		}

		// Attach user to context
		c.Set("user", user)
		c.Next()
	}
}
