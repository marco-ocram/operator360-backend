package LandingPage

import (
	"database/sql"
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/models"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

var regionalOffices = []string{
	"Bangalore", "Chandigarh", "Delhi", "Guwahati",
	"Hyderabad", "Lucknow", "Mumbai", "Ranchi",
}

func GetRORiskDist(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetRORiskDist] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	query := `
		SELECT
			ro,
			risk_bucket,
			COUNT(*)
		FROM operator360.opt_master
		WHERE risk_bucket IS NOT NULL
		GROUP BY ro, risk_bucket
		ORDER BY ro
	`

	rows, err := database.Query(query)
	if err != nil {
		log.Printf("[GetRORiskDist] Query error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch RO risk distribution", err, nil)
		return
	}
	defer rows.Close()

	data := make(map[string]models.RORiskCounts, len(regionalOffices))
	for _, ro := range regionalOffices {
		data[ro] = models.RORiskCounts{}
	}

	for rows.Next() {
		var ro, riskBucket sql.NullString
		var count int
		if err := rows.Scan(&ro, &riskBucket, &count); err != nil {
			log.Printf("[GetRORiskDist] Row scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process risk distribution record", err, nil)
			return
		}
		if !ro.Valid || !riskBucket.Valid {
			continue
		}
		counts := data[ro.String]
		switch riskBucket.String {
		case "High":
			counts.HighRiskCount += count
		case "Medium":
			counts.MedRiskCount += count
		case "Low":
			counts.LowRiskCount += count
		default:
			counts.NoRiskCount += count
		}
		data[ro.String] = counts
	}

	if err := rows.Err(); err != nil {
		log.Printf("[GetRORiskDist] Row iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading risk distribution records", err, nil)
		return
	}

	log.Printf("[GetRORiskDist] Returning risk distribution for %d ROs, requested by %s", len(data), user.ADID)
	c.JSON(http.StatusOK, models.RORiskDistResponse{
		Data:           data,
		File:           "opt360Store/kpi.json",
		RegionalOffice: user.RegionalOffice,
		RequestedBy:    user.ADID,
	})
}
