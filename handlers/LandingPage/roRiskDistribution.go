package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

func GetROQRiskDistribution(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	s3Cfg := config.GetDefaultS3Config()
	key := "opt360Store/kpi.json"

	var jsonData interface{}
	if err := s3store.FetchJSON(s3Cfg, key, &jsonData); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[GetROQRiskDistribution] S3 fetch failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "RO risk distribution file not found", err, gin.H{"file_path": key})
		} else {
			log.Printf("[GetROQRiskDistribution] Parse failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse JSON data", err, nil)
		}
		return
	}

	log.Printf("[GetROQRiskDistribution] Serving key=%s user=%s", key, user.ADID)
	respond.OK(c, gin.H{
		"file":            key,
		"requested_by":    user.ADID,
		"regional_office": user.RegionalOffice,
		"data":            jsonData,
	})
}
