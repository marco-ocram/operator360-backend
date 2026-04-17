package OperatorTab

import (
	"database/sql"
	"log"
	"net/http"

	"opt360-portal-backend/db"

	"github.com/gin-gonic/gin"
)

// ActiveOptUser represents the structure returned by the active_opt endpoint
type ActiveOptUser struct {
	UserStatus string `json:"user_status"`
	UserCode   string `json:"user_code"`
}

// GetActiveOpt handles GET /api/active_opt
// Returns all users with user_status = 1 from uidmasterv1_1.user
func GetActiveOpt(c *gin.Context) {

	// ── 1. Auth guard ──────────────────────────────────────────────────────────
	_, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	// ── 2. DB connection ───────────────────────────────────────────────────────
	database, err := db.GetUIDDB()
	if err != nil {
		log.Printf("[GetActiveOpt] DB connection error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database connection unavailable",
			"details": err.Error(),
		})
		return
	}

	// ── 3. Query active users ──────────────────────────────────────────────────
	query := `SELECT user_status, user_code FROM uidmasterv1_1.user WHERE user_status = 1`

	rows, err := database.Query(query)
	if err != nil {
		log.Printf("[GetActiveOpt] Query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch active operators",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	// ── 4. Scan rows ───────────────────────────────────────────────────────────
	users := make([]ActiveOptUser, 0)
	for rows.Next() {
		var userStatus string
		var userCode sql.NullString
		
		if err := rows.Scan(&userStatus, &userCode); err != nil {
			log.Printf("[GetActiveOpt] Row scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to process active operator record",
				"details": err.Error(),
			})
			return
		}
		
		user := ActiveOptUser{
			UserStatus: userStatus,
			UserCode:   "",
		}
		
		if userCode.Valid {
			user.UserCode = userCode.String
		}
		
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetActiveOpt] Row iteration error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error while reading active operator records",
			"details": err.Error(),
		})
		return
	}

	// ── 5. Respond ─────────────────────────────────────────────────────────────
	log.Printf("[GetActiveOpt] Returning %d active operators", len(users))
	c.JSON(http.StatusOK, gin.H{
		"data":  users,
		"count": len(users),
	})
}