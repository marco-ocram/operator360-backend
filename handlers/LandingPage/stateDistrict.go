package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetStateDistrict(c *gin.Context) {
	if _, ok := authctx.RequireUser(c); !ok {
		return
	}

	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetStateDistrict] DB connection error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Database connection unavailable", err, nil)
		return
	}

	stateRows, err := database.Query(`SELECT DISTINCT state FROM operator360.opt_master WHERE state IS NOT NULL ORDER BY state`)
	if err != nil {
		log.Printf("[GetStateDistrict] State query error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch states", err, nil)
		return
	}
	defer stateRows.Close()

	states := make([]string, 0)
	for stateRows.Next() {
		var state string
		if err := stateRows.Scan(&state); err != nil {
			log.Printf("[GetStateDistrict] State scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process state record", err, nil)
			return
		}
		states = append(states, state)
	}
	if err := stateRows.Err(); err != nil {
		log.Printf("[GetStateDistrict] State iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading state records", err, nil)
		return
	}

	districtRows, err := database.Query(`SELECT DISTINCT district FROM operator360.opt_master WHERE district IS NOT NULL ORDER BY district`)
	if err != nil {
		log.Printf("[GetStateDistrict] District query error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to fetch districts", err, nil)
		return
	}
	defer districtRows.Close()

	districts := make([]string, 0)
	for districtRows.Next() {
		var district string
		if err := districtRows.Scan(&district); err != nil {
			log.Printf("[GetStateDistrict] District scan error: %v", err)
			respond.Error(c, http.StatusInternalServerError, "Failed to process district record", err, nil)
			return
		}
		districts = append(districts, district)
	}
	if err := districtRows.Err(); err != nil {
		log.Printf("[GetStateDistrict] District iteration error: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Error while reading district records", err, nil)
		return
	}

	log.Printf("[GetStateDistrict] Returning %d states and %d districts", len(states), len(districts))
	respond.OK(c, gin.H{
		"states":    states,
		"districts": districts,
	})
}
