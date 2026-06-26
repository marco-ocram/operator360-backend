package OperatorDetailView

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetOperatorRiskDetails(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	optID := c.Query("opt_id")
	if optID == "" {
		respond.Error(c, http.StatusBadRequest, "opt_id query parameter is required", nil, nil)
		return
	}

	data, fileName, ok := fetchOperatorJSONFile[interface{}](c,
		"[GetOperatorRiskDetails]", optID, user.ADID, user.RegionalOffice,
		"risk_details.json", "Operator risk details file not found")
	if !ok {
		return
	}

	log.Printf("[GetOperatorRiskDetails] Serving key=%s user=%s", fileName, user.ADID)
	respond.OK(c, gin.H{
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"file":            fileName,
		"data":            data,
		"requested_by":    user.ADID,
	})
}
