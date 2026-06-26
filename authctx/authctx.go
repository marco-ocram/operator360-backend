// Package authctx provides access to the authenticated user that
// middleware.AuthMiddleware attaches to the gin context, replacing the
// c.Get("user") + type-assert + 401 boilerplate repeated in every handler.
package authctx

import (
	"opt360-portal-backend/models"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

// RequireUser fetches the authenticated user set by middleware.AuthMiddleware.
// On failure it writes the standard 401 JSON response itself and returns
// ok=false; callers should return immediately when ok is false:
//
//	user, ok := authctx.RequireUser(c)
//	if !ok {
//		return
//	}
func RequireUser(c *gin.Context) (*models.User, bool) {
	userInterface, exists := c.Get("user")
	if !exists {
		respond.Unauthorized(c, "User not found in context")
		return nil, false
	}

	user, ok := userInterface.(*models.User)
	if !ok {
		respond.Unauthorized(c, "User not found in context")
		return nil, false
	}

	return user, true
}
