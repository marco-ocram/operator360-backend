package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

// Top10REGv1Entry is defined here but the map below uses Top10EAv1Entry
// (same struct shape) — preserving the original file's behaviour exactly.
type Top10REGv1Entry struct {
	HighRiskCount int `json:"high_risk_count"`
	MedRiskCount  int `json:"med_risk_count"`
	LowRiskCount  int `json:"low_risk_count"`
}

func GetTop10regV1(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetTop10regV1] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	query := `
		SELECT
			reg AS reg_name,
			SUM(CASE WHEN risk_bucket = 'High'   THEN 1 ELSE 0 END) AS high_risk,
			SUM(CASE WHEN risk_bucket = 'Medium' THEN 1 ELSE 0 END) AS med_risk,
			SUM(CASE WHEN risk_bucket = 'Low'    THEN 1 ELSE 0 END) AS low_risk
		FROM operator360.opt_master
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
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch top 10 registrar data", err, nil)
		return
	}
	defer rows.Close()

	regMap := make(map[string]Top10EAv1Entry)
	for rows.Next() {
		var regName string
		var entry Top10EAv1Entry
		if err := rows.Scan(&regName, &entry.HighRiskCount, &entry.MedRiskCount, &entry.LowRiskCount); err != nil {
			log.Printf("[GetTop10regV1] Row scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process registrar record", err, nil)
			return
		}
		regMap[regName] = entry
	}
	if err := rows.Err(); err != nil {
		log.Printf("[GetTop10regV1] Row iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading registrar records", err, nil)
		return
	}

	log.Printf("[GetTop10regV1] Returning top %d registrars for ro=%s", len(regMap), user.RegionalOffice)
	respond.OK(c, gin.H{
		"data":            gin.H{"registrars": regMap},
		"regional_office": user.RegionalOffice,
	})
}
