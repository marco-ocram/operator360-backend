package RegionEvaluation

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

func GetRegionEvaluationCount(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	regionalOffice := c.Query("regional_office")
	optState := c.Query("opt_state")
	optDistrict := c.Query("opt_district")

	if regionalOffice == "" {
		respond.Error(c, http.StatusBadRequest, "regional_office query parameter is required", nil, nil)
		return
	}

	roForPath := utils.ToPascalCase(regionalOffice)
	var key string
	if optState != "" && optDistrict != "" {
		key = "opt360Store/" + roForPath + "/" + utils.ToPascalCase(optState) + "/" + utils.ToPascalCase(optDistrict) + "/audit.json"
	} else if optState != "" {
		key = "opt360Store/" + roForPath + "/" + utils.ToPascalCase(optState) + "/audit.json"
	} else {
		key = "opt360Store/" + roForPath + "/audit.json"
	}

	s3Cfg := config.GetDefaultS3Config()

	var jsonData interface{}
	if err := s3store.FetchJSON(s3Cfg, key, &jsonData); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[GetRegionEvaluationCount] S3 fetch failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "Audit file not found", err, gin.H{
				"regional_office": regionalOffice,
				"file_path":       key,
			})
		} else {
			log.Printf("[GetRegionEvaluationCount] Parse failed key=%s user=%s: %v", key, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse JSON data", err, nil)
		}
		return
	}

	log.Printf("[GetRegionEvaluationCount] Serving key=%s user=%s", key, user.ADID)
	respond.OK(c, gin.H{
		"regional_office": regionalOffice,
		"opt_state":       optState,
		"opt_district":    optDistrict,
		"file":            key,
		"data":            jsonData,
		"requested_by":    user.ADID,
	})
}
