package OperatorTab

import (
	"log"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

// GetLowRiskOperators fetches operator_low.parquet file based on user's regional office
func GetLowRiskOperators(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)

	fileName := s3store.OperatorFilePath(user.RegionalOffice, "operator_low.parquet")
	rows, err := fetchRiskOperatorRows(c, "[GetLowRiskOperators]", user.ADID, fileName,
		"Low risk operators file not found", "file", user.RegionalOffice)
	if err != nil {
		return
	}

	result := pageparam.Slice(rows, page, pageSize)

	log.Printf("[GetLowRiskOperators] Returning %d/%d low-risk operators for ro=%s (page %d)",
		len(result.Items), result.Total, user.RegionalOffice, result.Page)
	respond.OK(c, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"pagination":      result.JSON(),
		"count":           len(result.Items),
		"data":            result.Items,
	})
}
