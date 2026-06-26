package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetEARegistrar(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetEARegistrar] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	rows, err := database.Query(`
		SELECT DISTINCT reg, ea
		FROM operator360.opt_master
		WHERE ro = ?
		ORDER BY reg, ea
	`, user.RegionalOffice)
	if err != nil {
		log.Printf("[GetEARegistrar] Query error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch EA registrar data", err, nil)
		return
	}
	defer rows.Close()

	data := make(map[string][]string)
	totalPairs := 0
	for rows.Next() {
		var reg, ea string
		if err := rows.Scan(&reg, &ea); err != nil {
			log.Printf("[GetEARegistrar] Row scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process EA registrar record", err, nil)
			return
		}
		data[reg] = append(data[reg], ea)
		totalPairs++
	}
	if err := rows.Err(); err != nil {
		log.Printf("[GetEARegistrar] Row iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading EA registrar records", err, nil)
		return
	}

	log.Printf("[GetEARegistrar] Returning %d reg/ea pairs across %d registrars for ro=%s",
		totalPairs, len(data), user.RegionalOffice)
	respond.OK(c, gin.H{
		"data":            data,
		"total":           totalPairs,
		"regional_office": user.RegionalOffice,
	})
}
