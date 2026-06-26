package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

func GetKPIData(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	s3Cfg := config.GetDefaultS3Config()
	key := s3store.OperatorFilePath(user.RegionalOffice, "kpi.json")

	var kpiData models.KPIResponse
	if err := s3store.FetchJSON(s3Cfg, key, &kpiData); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[GetKPIData] S3 fetch failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "KPI file not found", err, gin.H{
				"regional_office": user.RegionalOffice,
				"file_path":       key,
			})
		} else {
			log.Printf("[GetKPIData] Parse failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse JSON data", err, nil)
		}
		return
	}

	log.Printf("[GetKPIData] Serving key=%s user=%s", key, user.ADID)
	c.JSON(http.StatusOK, kpiData)
}
