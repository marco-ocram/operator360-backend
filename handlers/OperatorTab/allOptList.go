package OperatorTab

import (
	"log"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

func GetOperatorList(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)
	filters := operatorListFilters{
		OptEa:        c.Query("opt_ea"),
		OptReg:       c.Query("opt_reg"),
		OptDistrict:  c.Query("opt_district"),
		OptState:     c.Query("opt_state"),
		OptID:        c.Query("opt_id"),
		ActiveStatus: c.Query("active_status"),
		Risk:         c.Query("risk"),
	}

	// Allow overriding the caller's own RO, e.g. when navigating here from a
	// search result (packet search, feature analysis) for an operator in a
	// different regional office.
	ro := c.Query("RO")
	if ro == "" {
		ro = user.RegionalOffice
	}

	fileName := s3store.OperatorFilePath(ro, "operator.parquet")
	rows, err := fetchOperatorParquetRows[map[string]interface{}](c, "[GetOperatorList]", user.ADID, fileName,
		"Operator list file not found", "file_path", ro)
	if err != nil {
		return
	}

	filtered := filterOperatorRows(rows, filters)
	result := pageparam.Slice(filtered, page, pageSize)

	log.Printf("[GetOperatorList] Returning %d/%d operators for ro=%s (page %d)",
		len(result.Items), result.Total, ro, result.Page)
	respond.OK(c, gin.H{
		"regional_office": ro,
		"file":            fileName,
		"total_count":     len(rows),
		"filtered_count":  result.Total,
		"pagination":      result.JSON(),
		"filters": gin.H{
			"opt_ea":        filters.OptEa,
			"opt_reg":       filters.OptReg,
			"opt_district":  filters.OptDistrict,
			"opt_state":     filters.OptState,
			"opt_id":        filters.OptID,
			"active_status": filters.ActiveStatus,
			"risk":          filters.Risk,
		},
		"count": len(result.Items),
		"data":  result.Items,
	})
}
