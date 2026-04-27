package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/db"
	"opt360-portal-backend/models"

	"github.com/gin-gonic/gin"
)
// Top10EAv1Entry holds risk counts for a single EA.
type Top10REGv1Entry struct {
	HighRiskCount int `json:"high_risk_count"`
	MedRiskCount  int `json:"med_risk_count"`
	LowRiskCount  int `json:"low_risk_count"`
}

// GetTop10EAsV1 handles GET /api/top10eav1.
// Returns the top 10 EAs by total operator count (high + medium + low)
// for the logged-in user's regional office, querying data_platform.opt_master.
func GetTop10regV1(c *gin.Context) {

	// ── 1. Auth guard ──────────────────────────────────────────────────────────
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	user := userInterface.(*models.User)

	// ── 2. DB connection ───────────────────────────────────────────────────────
	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetTop10regV1] DB connection error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database connection unavailable",
			"details": err.Error(),
		})
		return
	}

	// ── 3. Query ───────────────────────────────────────────────────────────────
	// Fetch risk counts per EA for the user's RO, ordered by total descending,
	// limited to top 10.
	query := `
		SELECT
			reg AS reg_name,
			SUM(CASE WHEN risk_bucket = 'High'   THEN 1 ELSE 0 END) AS high_risk,
			SUM(CASE WHEN risk_bucket = 'Medium' THEN 1 ELSE 0 END) AS med_risk,
			SUM(CASE WHEN risk_bucket = 'Low'    THEN 1 ELSE 0 END) AS low_risk
		FROM data_platform.opt_master
		WHERE ro = ?
		GROUP BY reg
		ORDER BY (
			SUM(CASE WHEN risk_bucket = 'High'   THEN 1 ELSE 0 END) +
			SUM(CASE WHEN risk_bucket = 'Medium' THEN 1 ELSE 0 END) +
			SUM(CASE WHEN risk_bucket = 'Low'    THEN 1 ELSE 0 END)
		) DESC
		LIMIT 10
	`

	rows, err := database.Query(query, user.RegionalOffice)
	if err != nil {
		log.Printf("[GetTop10regV1] Query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch top 10 registrar data",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	// ── 4. Scan rows ───────────────────────────────────────────────────────────
	regMap := make(map[string]Top10EAv1Entry)
	for rows.Next() {
		var regName string
		var entry Top10EAv1Entry
		if err := rows.Scan(&regName, &entry.HighRiskCount, &entry.MedRiskCount, &entry.LowRiskCount); err != nil {
			log.Printf("[GetTop10regV1] Row scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to process registrar record",
				"details": err.Error(),
			})
			return
		}
		regMap[regName] = entry
	}
	if err := rows.Err(); err != nil {
		log.Printf("[GetTop10regV1] Row iteration error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error while reading registrar records",
			"details": err.Error(),
		})
		return
	}

	// ── 5. Respond ─────────────────────────────────────────────────────────────
	log.Printf("[GetTop10regV1] Returning top %d registrars for ro=%s", len(regMap), user.RegionalOffice)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"registrars": regMap,
		},
		"regional_office": user.RegionalOffice,
	})
}
