package AnamolyIndicators

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

func GetAnamolyIndicators(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	s3Cfg := config.GetDefaultS3Config()
	key := s3store.OperatorFilePath(user.RegionalOffice, "anomalies_insights.json")

	bodyBytes, err := s3store.FetchBytes(s3Cfg, key)
	if err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[GetAnamolyIndicators] S3 fetch failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "Anomalies insights file not found", err, gin.H{
				"regional_office": user.RegionalOffice,
				"file_path":       key,
			})
		} else {
			log.Printf("[GetAnamolyIndicators] Fetch failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to fetch file", err, nil)
		}
		return
	}

	// File uses single quotes in some places; normalize before JSON parsing.
	bodyStr := strings.ReplaceAll(string(bodyBytes), "'", "\"")

	var jsonData interface{}
	if err := json.Unmarshal([]byte(bodyStr), &jsonData); err != nil {
		log.Printf("[GetAnamolyIndicators] JSON parse failed key=%s: %v", key, err)
		respond.Error(c, http.StatusInternalServerError, "Invalid JSON in file", err, gin.H{"file_path": key})
		return
	}

	log.Printf("[GetAnamolyIndicators] Serving key=%s user=%s", key, user.ADID)
	respond.OK(c, gin.H{
		"file":            key,
		"requested_by":    user.ADID,
		"regional_office": user.RegionalOffice,
		"data":            jsonData,
	})
}
