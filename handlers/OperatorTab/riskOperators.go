package OperatorTab

import (
	"log"
	"net/http"

	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

// fetchOperatorParquetRows fetches and decodes a per-regional-office parquet
// file shared by GetHighRiskOperators/GetMediumRiskOperators/
// GetLowRiskOperators/GetOperatorList. On failure it writes the response
// itself (404 if the S3 object couldn't be fetched, 500 if it fetched but
// failed to decode) and returns a non-nil error so the caller knows to stop.
func fetchOperatorParquetRows[T any](c *gin.Context, logPrefix, adID, fileName, notFoundMessage, notFoundFileKey, regionalOffice string) ([]T, error) {
	s3Cfg := config.GetDefaultS3Config()

	rows, err := s3store.FetchParquetRows[T](s3Cfg, fileName)
	if err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("%s S3 fetch failed key=%s user=%s: %v", logPrefix, fileName, adID, err)
			respond.Error(c, http.StatusNotFound, notFoundMessage, err, gin.H{
				"regional_office": regionalOffice,
				notFoundFileKey:   fileName,
			})
		} else {
			log.Printf("%s parquet decode failed key=%s user=%s: %v", logPrefix, fileName, adID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to read parquet data", err, nil)
		}
		return nil, err
	}
	return rows, nil
}

// fetchRiskOperatorRows is the models.Operator-typed instantiation used by
// the high/medium/low risk handlers.
func fetchRiskOperatorRows(c *gin.Context, logPrefix, adID, fileName, notFoundMessage, notFoundFileKey, regionalOffice string) ([]models.Operator, error) {
	return fetchOperatorParquetRows[models.Operator](c, logPrefix, adID, fileName, notFoundMessage, notFoundFileKey, regionalOffice)
}
