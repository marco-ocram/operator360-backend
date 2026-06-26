package OperatorDetailView

import (
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/config"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

// fetchOperatorJSONFile resolves optID's data_path, then fetches and
// JSON-decodes <dataPath>/<filename> from S3 into a T. Shared by
// GetOperatorDetails/GetOperatorFeatures/GetOperatorRiskDetails, which only
// differ in filename, not-found wording, and what they do with the decoded
// data. On any failure it writes the response itself and returns ok=false.
func fetchOperatorJSONFile[T any](c *gin.Context, logPrefix, optID, adID, regionalOffice, filename, notFoundMessage string) (T, string, bool) {
	var zero T

	dataPath, err := db.GetDataPathByOptID(optID)
	if err != nil {
		log.Printf("%s DataPath lookup failed opt_id=%s user=%s: %v", logPrefix, optID, adID, err)
		respond.Error(c, http.StatusNotFound, "Operator data path not found", err, gin.H{"operator_id": optID})
		return zero, "", false
	}

	s3Cfg := config.GetDefaultS3Config()
	fileName := strings.TrimSuffix(dataPath, "/") + "/" + filename

	var data T
	if err := s3store.FetchJSON(s3Cfg, fileName, &data); err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("%s S3 fetch failed key=%s user=%s: %v", logPrefix, fileName, adID, err)
			respond.Error(c, http.StatusNotFound, notFoundMessage, err, gin.H{
				"regional_office": regionalOffice,
				"operator_id":     optID,
				"file_path":       fileName,
			})
		} else {
			log.Printf("%s parse failed key=%s user=%s: %v", logPrefix, fileName, adID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse JSON data", err, nil)
		}
		return zero, "", false
	}

	return data, fileName, true
}
