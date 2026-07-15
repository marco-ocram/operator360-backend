package OperatorTab

import (
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/db"
	"opt360-portal-backend/models"

	"github.com/gin-gonic/gin"
)

// regionalOffices is the fixed list of ROs, kept in sync with
// operator360-ui/src/constants.js's REGIONAL_OFFICES. No query needed for this
// one — it doesn't change without a frontend deploy anyway.
var regionalOffices = []string{
	"Bangalore", "Mumbai", "Delhi", "Lucknow", "Hyderabad", "Ranchi", "Guwahati", "Chandigarh",
}

// GetOperatorFilters handles POST /api/operator_filters — everything needed to
// populate the View Operators filter UI.
//
// Body: {"ro": "<optional>"} — when unset (the common case; the frontend
// never sends it today), registrar/EA/risk-bucket lists come from the
// startup-loaded, periodically-refreshed caches (db/nameCache.go,
// db/riskBuckets.go) instead of a live query. When an explicit ro is given,
// this falls back to a live RO-scoped query, since the caches are global.
func GetOperatorFilters(c *gin.Context) {
	_, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var req models.OperatorFiltersRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	ro := strings.TrimSpace(req.RO)

	var registrars, eas []models.NameCode
	var riskBuckets []string

	if ro == "" {
		registrars = db.GetCachedRegistrars()
		eas = db.GetCachedEAs()
		riskBuckets = db.GetRiskBuckets()
	} else {
		database, err := db.GetDB()
		if err != nil {
			log.Printf("[GetOperatorFilters] DB connection error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Database connection unavailable",
				"details": err.Error(),
			})
			return
		}

		registrars, err = fetchNameCodePairs(database, "reg", "reg_code", ro)
		if err != nil {
			log.Printf("[GetOperatorFilters] registrar query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch registrars", "details": err.Error()})
			return
		}

		eas, err = fetchNameCodePairs(database, "ea", "ea_code", ro)
		if err != nil {
			log.Printf("[GetOperatorFilters] ea query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch EAs", "details": err.Error()})
			return
		}

		riskBuckets, err = fetchDistinctColumn(database, "risk_bucket", ro)
		if err != nil {
			log.Printf("[GetOperatorFilters] risk_bucket query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch risk buckets", "details": err.Error()})
			return
		}
	}

	log.Printf("[GetOperatorFilters] ro=%q → %d registrars, %d eas, %d risk buckets", ro, len(registrars), len(eas), len(riskBuckets))

	c.JSON(http.StatusOK, models.OperatorFiltersResponse{
		RegionalOffices: regionalOffices,
		Registrars:      registrars,
		EAs:             eas,
		RiskBuckets:     riskBuckets,
	})
}

// fetchNameCodePairs returns distinct (nameCol, codeCol) pairs from opt_master,
// optionally scoped to ro. nameCol/codeCol are always one of the small fixed set
// of literals passed by callers in this file (never user input), so building the
// query by string concatenation here is safe.
func fetchNameCodePairs(database *db.LoggedDB, nameCol, codeCol, ro string) ([]models.NameCode, error) {
	query := "SELECT DISTINCT " + nameCol + ", " + codeCol + " FROM operator360.opt_master WHERE " + nameCol + " IS NOT NULL AND " + codeCol + " IS NOT NULL"
	args := []interface{}{}
	if ro != "" {
		query += " AND ro = ?"
		args = append(args, ro)
	}
	query += " ORDER BY " + nameCol

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pairs := make([]models.NameCode, 0)
	for rows.Next() {
		var nc models.NameCode
		if err := rows.Scan(&nc.Name, &nc.Code); err != nil {
			return nil, err
		}
		pairs = append(pairs, nc)
	}
	return pairs, rows.Err()
}

// fetchDistinctColumn returns distinct non-null values of col from opt_master,
// optionally scoped to ro. col is always one of a small fixed set of literals
// passed by callers in this file (never user input).
func fetchDistinctColumn(database *db.LoggedDB, col, ro string) ([]string, error) {
	query := "SELECT DISTINCT " + col + " FROM operator360.opt_master WHERE " + col + " IS NOT NULL"
	args := []interface{}{}
	if ro != "" {
		query += " AND ro = ?"
		args = append(args, ro)
	}
	query += " ORDER BY " + col

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]string, 0)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
