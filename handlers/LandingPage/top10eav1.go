package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

type Top10EAv1Entry struct {
	HighRiskCount int `json:"high_risk_count"`
	MedRiskCount  int `json:"med_risk_count"`
	LowRiskCount  int `json:"low_risk_count"`
}

func GetTop10EAsV1(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetTop10EAsV1] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	query := `
		SELECT
			ea AS ea_name,
			SUM(CASE WHEN risk_bucket = 'High'   THEN 1 ELSE 0 END) AS high_risk,
			SUM(CASE WHEN risk_bucket = 'Medium' THEN 1 ELSE 0 END) AS med_risk,
			SUM(CASE WHEN risk_bucket = 'Low'    THEN 1 ELSE 0 END) AS low_risk
		FROM operator360.opt_master
		WHERE ro = ?
		GROUP BY ea
		ORDER BY (
			SUM(CASE WHEN risk_bucket = 'High'   THEN 1 ELSE 0 END) +
			SUM(CASE WHEN risk_bucket = 'Medium' THEN 1 ELSE 0 END) +
			SUM(CASE WHEN risk_bucket = 'Low'    THEN 1 ELSE 0 END)
		) DESC
		LIMIT 10
	`

	rows, err := database.Query(query, user.RegionalOffice)
	if err != nil {
		log.Printf("[GetTop10EAsV1] Query error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch top 10 EA data", err, nil)
		return
	}
	defer rows.Close()

	eaMap := make(map[string]Top10EAv1Entry)
	for rows.Next() {
		var eaName string
		var entry Top10EAv1Entry
		if err := rows.Scan(&eaName, &entry.HighRiskCount, &entry.MedRiskCount, &entry.LowRiskCount); err != nil {
			log.Printf("[GetTop10EAsV1] Row scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process EA record", err, nil)
			return
		}
		eaMap[eaName] = entry
	}
	if err := rows.Err(); err != nil {
		log.Printf("[GetTop10EAsV1] Row iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading EA records", err, nil)
		return
	}

	log.Printf("[GetTop10EAsV1] Returning top %d EAs for ro=%s", len(eaMap), user.RegionalOffice)
	respond.OK(c, gin.H{
		"data":            gin.H{"ea": eaMap},
		"regional_office": user.RegionalOffice,
	})
}
