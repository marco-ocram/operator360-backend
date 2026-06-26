package LandingPage

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"
	"opt360-portal-backend/utils"

	"github.com/gin-gonic/gin"
)

func GetFeatureAnalysis(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	ro := c.Query("RO")
	if ro == "" {
		ro = user.RegionalOffice
	}
	ro = utils.ToPascalCase(ro)

	s3Cfg := config.GetDefaultS3Config()
	key := s3store.OperatorFilePath(ro, "featureAnalysis.json")

	var data interface{}
	if err := s3store.FetchJSON(s3Cfg, key, &data); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[GetFeatureAnalysis] S3 fetch failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "Feature analysis file not found", err, gin.H{
				"regional_office": ro,
				"file_path":       key,
			})
		} else {
			log.Printf("[GetFeatureAnalysis] Parse failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse JSON data", err, nil)
		}
		return
	}

	log.Printf("[GetFeatureAnalysis] Serving key=%s user=%s", key, user.ADID)
	respond.OK(c, gin.H{
		"regional_office": ro,
		"requested_by":    user.ADID,
		"data":            data,
	})
}
