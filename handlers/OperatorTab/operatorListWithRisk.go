package OperatorTab

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/models"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

// validRiskBuckets holds the accepted values for the risk_bucket parameter
var validRiskBuckets = map[string]bool{
	"Low":    true,
	"Medium": true,
	"High":   true,
	"No":     true,
}

// GetOperatorListWithRisk queries data_platform.opt_master filtered by risk_bucket.
// Required query param : risk_bucket  (Low | Medium | High | No)
// Optional query params: page (default 1), page_size (default 20, max 1000)
func GetOperatorListWithRisk(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}
	userRO := strings.TrimSpace(user.RegionalOffice)

	riskBucket, ok := parseRiskBucket(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)
	offset := (page - 1) * pageSize

	operators, total, err := db.GetOperatorsByRiskBucket(userRO, riskBucket, pageSize, offset)
	if err != nil {
		log.Printf("[GetOperatorListWithRisk] %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve operator records", err, nil)
		return
	}

	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	if total == 0 {
		totalPages = 0
	}

	log.Printf("[GetOperatorListWithRisk] Returning %d/%d records for risk_bucket=%s (page %d/%d)",
		len(operators), total, riskBucket, page, totalPages)

	c.JSON(http.StatusOK, models.OperatorWithRiskListResponse{
		Data:       operators,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		RiskBucket: riskBucket,
	})
}

// parseRiskBucket validates the required risk_bucket query param, writing
// the 400 response itself when missing or invalid.
func parseRiskBucket(c *gin.Context) (string, bool) {
	riskBucket := strings.TrimSpace(c.Query("risk_bucket"))
	if riskBucket == "" {
		respond.Error(c, http.StatusBadRequest, "risk_bucket query parameter is required", nil, gin.H{
			"accepted_values": []string{"Low", "Medium", "High", "No"},
		})
		return "", false
	}
	if !validRiskBuckets[riskBucket] {
		respond.Error(c, http.StatusBadRequest, fmt.Sprintf("invalid risk_bucket value: %q", riskBucket), nil, gin.H{
			"accepted_values": []string{"Low", "Medium", "High", "No"},
		})
		return "", false
	}
	return riskBucket, true
}
