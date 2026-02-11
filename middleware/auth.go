package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"fmt"
    "opt360-portal-backend/auth"
    "opt360-portal-backend/session"
	"github.com/gin-gonic/gin"
)

// TokenClaims represents the decoded JWT token structure
type TokenClaims struct {
	Sub   string `json:"sub"`   
	Aut   string `json:"aut"`
	Aud   string `json:"aud"`
	Nbf   int64  `json:"nbf"`
	Azp   string `json:"azp"`
	Scope string `json:"scope"`
	Iss   string `json:"iss"`
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
	Jti   string `json:"jti"`
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c. GetHeader("Authorization")
		
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing Authorization header",
			})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization header format.  Expected: Bearer <token>",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Decode the JWT token
		claims, err := decodeJWT(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":    "Invalid token",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Extract AD ID from the "sub" field
		adID := claims.Sub
		if adID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token does not contain user identifier (sub)",
			})
			c.Abort()
			return
		}

		// Validate token expiration
		if ! isTokenValid(claims) {
			c.JSON(http.StatusUnauthorized, gin. H{
				"error": "Token has expired",
			})
			c.Abort()
			return
		}

		// Get user by AD ID
		user, found := auth.GetUserByADID(adID)
		if !found {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "User not authorized",
				"message": "AD-ID not found in system",
				"ad_id":   adID,
			})
			c.Abort()
			return
		}

		// Check if user has a session-stored regional office override
		sessionManager := session.GetSessionManager()
		if sessionRegionalOffice, exists := sessionManager.GetUserRegionalOffice(adID); exists {
			// Override the user's regional office with session value
			user.RegionalOffice = sessionRegionalOffice
		}

		// Attach user to context
		c.Set("user", user)
		c.Set("token_claims", claims) // Optional: store claims for later use
		c.Next()
	}
}

// decodeJWT decodes a JWT token without verification (basic Base64 decode)
// WARNING: This does NOT verify the token signature! 
// For production, you should verify the token signature using the issuer's public key




func decodeJWT(token string) (*TokenClaims, error) {
	// JWT structure: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload := parts[1]
	
	// Add padding if necessary (Base64 URL encoding may omit padding)
	if l := len(payload) % 4; l > 0 {
		payload += strings.Repeat("=", 4-l)
	}

	// Decode from Base64 URL encoding
	decodedBytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token payload: %w", err)
	}

	// Parse JSON
	var claims TokenClaims
	if err := json. Unmarshal(decodedBytes, &claims); err != nil {
		return nil, fmt. Errorf("failed to parse token claims: %w", err)
	}

	return &claims, nil
}

// isTokenValid checks if the token is still valid (not expired)
func isTokenValid(claims *TokenClaims) bool {
	currentTime := time.Now().Unix()
	return claims.Exp > currentTime
}