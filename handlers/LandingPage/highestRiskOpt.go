package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetHighestRiskOperator(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetHighestRiskOperator] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	query := `
		SELECT id, NAME, risk_score
		FROM operator360.opt_master
		WHERE ro = ?
		ORDER BY risk_score DESC
		LIMIT 1`

	row := database.QueryRow(query, user.RegionalOffice)

	var id, name string
	var riskScore float64
	if err := row.Scan(&id, &name, &riskScore); err != nil {
		log.Printf("[GetHighestRiskOperator] Scan error for ro=%s: %v", user.RegionalOffice, err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve highest risk operator", err, nil)
		return
	}

	countQuery := `SELECT COUNT(*) FROM operator360.opt_master WHERE ro = ? AND risk_bucket = 'High'`
	var highOptCount int
	if err := database.QueryRow(countQuery, user.RegionalOffice).Scan(&highOptCount); err != nil {
		log.Printf("[GetHighestRiskOperator] Count scan error for ro=%s: %v", user.RegionalOffice, err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve high risk count", err, nil)
		return
	}

	respond.OK(c, gin.H{
		"regional_office": user.RegionalOffice,
		"data": gin.H{
			"id":             id,
			"name":           name,
			"risk_score":     riskScore,
			"high_opt_count": highOptCount,
		},
	})
}
