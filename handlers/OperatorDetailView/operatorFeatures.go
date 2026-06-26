package OperatorDetailView

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetOperatorFeatures(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	optID := c.Query("opt_id")
	if optID == "" {
		respond.Error(c, http.StatusBadRequest, "opt_id query parameter is required", nil, nil)
		return
	}

	raw, fileName, ok := fetchOperatorJSONFile[map[string]interface{}](c,
		"[GetOperatorFeatures]", optID, user.ADID, user.RegionalOffice,
		"operator_features.json", "Operator features file not found")
	if !ok {
		return
	}

	log.Printf("[GetOperatorFeatures] Serving key=%s user=%s", fileName, user.ADID)
	respond.OK(c, transformFeatureKPIs(raw))
}

func transformFeatureKPIs(original map[string]interface{}) gin.H {
	out := gin.H{"opt_id": original["opt_id"]}
	if meta, ok := original["metadata"]; ok {
		out["metadata"] = meta
	}
	var arr []map[string]interface{}
	if kpis, ok := original["kpis"].(map[string]interface{}); ok {
		for cat, feats := range kpis {
			if items, ok := feats.([]interface{}); ok && len(items) > 0 {
				arr = append(arr, map[string]interface{}{cat: items})
			}
		}
	}
	out["kpis"] = arr
	return out
}
