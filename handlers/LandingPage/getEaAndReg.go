package LandingPage

import (
	"database/sql"
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetEaAndReg(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	value := c.Query("value")
	if value != "ea" && value != "reg" {
		respond.Error(c, http.StatusBadRequest, "query parameter 'value' is required", nil, gin.H{
			"accepted_values": []string{"ea", "reg"},
		})
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetEaAndReg] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	// value is validated above (only "ea" or "reg"), safe to interpolate.
	query := "SELECT DISTINCT " + value + " FROM operator360.opt_master WHERE ro = ? AND " + value + " IS NOT NULL ORDER BY " + value

	rows, err := database.Query(query, user.RegionalOffice)
	if err != nil {
		log.Printf("[GetEaAndReg] Query error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch data", err, nil)
		return
	}
	defer rows.Close()

	results := make([]string, 0)
	for rows.Next() {
		var val sql.NullString
		if err := rows.Scan(&val); err != nil {
			log.Printf("[GetEaAndReg] Row scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process record", err, nil)
			return
		}
		if val.Valid {
			results = append(results, val.String)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[GetEaAndReg] Row iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading records", err, nil)
		return
	}

	log.Printf("[GetEaAndReg] Returning %d distinct %s values for ro=%s", len(results), value, user.RegionalOffice)
	respond.OK(c, gin.H{
		"data":            results,
		"count":           len(results),
		"type":            value,
		"regional_office": user.RegionalOffice,
	})
}
