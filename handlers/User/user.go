package User

import (
	"net/http"
	"github.com/gin-gonic/gin"
	
)

// GetUserInfo returns the current user's information and regional office
func GetUserInfo(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	c.JSON(http.StatusOK, user)
}

