package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/db"
	"opt360-portal-backend/models"

	"github.com/gin-gonic/gin"
)

// regionalOffices is the fixed list of ROs whose counts are always returned.
var regionalOffices = []string{
	"Bangalore", "Chandigarh", "Delhi", "Guwahati",
	"Hyderabad", "Lucknow", "Mumbai", "Ranchi",
}

// GetRORiskDist handles GET /api/ro_risk_dist.
// Returns high / medium / low / no-risk counts for every regional office.
func GetRORiskDist(c *gin.Context) {

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
		log.Printf("[GetRORiskDist] DB connection error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database connection unavailable",
			"details": err.Error(),
		})
		return
	}

	// ── 3. Query all ROs in one pass ───────────────────────────────────────────
	// SUM(col = 'val') uses MySQL boolean arithmetic (0/1) — faster than CASE.
	// no_count is derived from COUNT(*) to avoid a 4th conditional per row.
	query := `
		SELECT
			ro,
			SUM(risk_bucket = 'High')                                    AS high_count,
			SUM(risk_bucket = 'Medium')                                  AS med_count,
			SUM(risk_bucket = 'Low')                                     AS low_count,
			COUNT(*) - SUM(risk_bucket IN ('High', 'Medium', 'Low'))     AS no_count
		FROM data_platform.opt_master
		WHERE ro IN (
			'Bangalore', 'Chandigarh', 'Delhi', 'Guwahati',
			'Hyderabad', 'Lucknow', 'Mumbai', 'Ranchi'
		)
		GROUP BY ro
	`

	rows, err := database.Query(query)
	if err != nil {
		log.Printf("[GetRORiskDist] Query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch RO risk distribution",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	// ── 4. Pre-populate map so every RO appears even if no rows returned ───────
	data := make(map[string]models.RORiskCounts, len(regionalOffices))
	for _, ro := range regionalOffices {
		data[ro] = models.RORiskCounts{}
	}

	// ── 5. Scan rows ───────────────────────────────────────────────────────────
	for rows.Next() {
		var ro string
		var counts models.RORiskCounts
		if err := rows.Scan(&ro, &counts.HighRiskCount, &counts.MedRiskCount, &counts.LowRiskCount, &counts.NoRiskCount); err != nil {
			log.Printf("[GetRORiskDist] Row scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to process risk distribution record",
				"details": err.Error(),
			})
			return
		}
		data[ro] = counts
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetRORiskDist] Row iteration error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error while reading risk distribution records",
			"details": err.Error(),
		})
		return
	}

	// ── 6. Respond ─────────────────────────────────────────────────────────────
	log.Printf("[GetRORiskDist] Returning risk distribution for %d ROs, requested by %s", len(data), user.ADID)

	c.JSON(http.StatusOK, models.RORiskDistResponse{
		Data:           data,
		File:           "opt360Store/kpi.json",
		RegionalOffice: user.RegionalOffice,
		RequestedBy:    user.ADID,
	})
}
