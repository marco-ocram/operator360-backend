package SidReview

import (
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/db"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

func flattenAnomalyRecords(rawData map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for operatorID, operatorData := range rawData {
		operatorMap, ok := operatorData.(map[string]interface{})
		if !ok {
			continue
		}
		for category, categoryData := range operatorMap {
			categoryMap, ok := categoryData.(map[string]interface{})
			if !ok {
				continue
			}
			for sid, sidData := range categoryMap {
				sidMap, ok := sidData.(map[string]interface{})
				if !ok {
					continue
				}
				record := map[string]interface{}{
					"sid":              sid,
					"operator_id":      operatorID,
					"anomaly_category": category,
				}
				for key, value := range sidMap {
					record[key] = value
				}
				out = append(out, record)
			}
		}
	}
	return out
}

func GetAnamolousSIDs(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)
	optID := c.Query("opt_id")
	anomalyCategoryFilter := c.Query("anomaly_category")

	if optID == "" {
		respond.Error(c, http.StatusBadRequest, "opt_id query parameter is required", nil, nil)
		return
	}

	dataPath, err := db.GetDataPathByOptID(optID)
	if err != nil {
		log.Printf("[GetAnamolousSIDs] DataPath lookup failed opt_id=%s user=%s: %v", optID, user.ADID, err)
		respond.Error(c, http.StatusNotFound, "Operator data path not found", err, gin.H{"operator_id": optID})
		return
	}

	s3Cfg := config.GetDefaultS3Config()
	fileName := strings.TrimSuffix(dataPath, "/") + "/anomaly_sid.json"

	var rawData map[string]interface{}
	if err := s3store.FetchJSON(s3Cfg, fileName, &rawData); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[GetAnamolousSIDs] S3 fetch failed key=%s user=%s: %v", fileName, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "Anomalous SIDs file not found", err, gin.H{
				"regional_office": user.RegionalOffice,
				"operator_id":     optID,
				"file_path":       fileName,
			})
		} else {
			log.Printf("[GetAnamolousSIDs] Parse failed key=%s user=%s: %v", fileName, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse JSON data", err, nil)
		}
		return
	}

	allData := flattenAnomalyRecords(rawData)

	filteredData := allData
	if anomalyCategoryFilter != "" {
		filteredData = make([]map[string]interface{}, 0)
		for _, record := range allData {
			if category, ok := record["anomaly_category"].(string); ok {
				if strings.EqualFold(category, anomalyCategoryFilter) {
					filteredData = append(filteredData, record)
				}
			}
		}
	}

	result := pageparam.Slice(filteredData, page, pageSize)

	log.Printf("[GetAnamolousSIDs] Returning %d/%d anomalous SIDs for opt_id=%s user=%s (page %d)",
		len(result.Items), result.Total, optID, user.ADID, result.Page)
	respond.OK(c, gin.H{
		"regional_office":         user.RegionalOffice,
		"operator_id":             optID,
		"file":                    fileName,
		"anomaly_category_filter": anomalyCategoryFilter,
		"pagination":              result.JSON(),
		"count":                   len(result.Items),
		"data":                    result.Items,
		"requested_by":            user.ADID,
	})
}
