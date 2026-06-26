package SidReview

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/db"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

type searchFilters struct {
	sid            string
	anomalyFilter  string
	enrollmentType string
	date           string
}

func parseSearchFilters(c *gin.Context) searchFilters {
	return searchFilters{
		sid:            c.Query("sid"),
		anomalyFilter:  c.Query("anomaly_filter"),
		enrollmentType: c.Query("enrollment_type"),
		date:           c.Query("date"),
	}
}

// normalizeDateFields converts numeric timestamp values in date/time fields to
// "YYYY-MM-DD HH:MM:SS" and normalizes underscore-separated date strings to
// hyphen-separated ones, in place.
func normalizeDateFields(row map[string]interface{}) {
	for key, val := range row {
		lowerKey := strings.ToLower(key)
		if !strings.Contains(lowerKey, "date") && !strings.Contains(lowerKey, "time") {
			continue
		}
		var timestamp float64
		switch v := val.(type) {
		case float64:
			timestamp = v
		case float32:
			timestamp = float64(v)
		case int64:
			timestamp = float64(v)
		case int:
			timestamp = float64(v)
		case string:
			valStr := strings.TrimSpace(v)
			if strings.Contains(valStr, "-") || strings.Contains(valStr, "_") {
				row[key] = strings.ReplaceAll(valStr, "_", "-")
				continue
			}
			fmt.Sscanf(valStr, "%f", &timestamp)
		}
		if timestamp > 1e15 {
			timestamp /= 1e9
		} else if timestamp > 1e12 {
			timestamp /= 1000.0
		}
		// Accept timestamps in the range 2000–2100.
		if timestamp >= 946684800 && timestamp <= 4102444800 {
			row[key] = time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
		}
	}
}

func applySearchFilters(rows []map[string]interface{}, f searchFilters) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		if f.sid != "" {
			found := false
			for key, val := range row {
				if strings.EqualFold(key, "eid") {
					if strings.Contains(strings.ToLower(fmt.Sprintf("%v", val)), strings.ToLower(f.sid)) {
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}
		}

		if f.anomalyFilter != "" {
			isAnomalous := false
			for key, val := range row {
				if strings.EqualFold(key, "anomaly_type") {
					switch v := val.(type) {
					case []interface{}:
						isAnomalous = len(v) > 0
					case []string:
						isAnomalous = len(v) > 0
					case string:
						v = strings.TrimSpace(v)
						isAnomalous = v != "[]" && v != "" && v != "null"
					default:
						isAnomalous = val != nil
					}
					break
				}
			}
			if strings.EqualFold(f.anomalyFilter, "anomalous") && !isAnomalous {
				continue
			}
			if strings.EqualFold(f.anomalyFilter, "non-anomalous") && isAnomalous {
				continue
			}
		}

		if f.enrollmentType != "" {
			enrollmentType := ""
			for key, val := range row {
				if strings.EqualFold(key, "enrolnment_type") {
					enrollmentType = strings.TrimSpace(fmt.Sprintf("%v", val))
					break
				}
			}
			if strings.EqualFold(f.enrollmentType, "update") && !strings.EqualFold(enrollmentType, "U") {
				continue
			}
			if strings.EqualFold(f.enrollmentType, "new_enrollment") && !strings.EqualFold(enrollmentType, "N") {
				continue
			}
		}

		if f.date != "" {
			normalizedFilter := strings.ReplaceAll(f.date, "-", "_")
			found := false
			for key, val := range row {
				if strings.Contains(strings.ToLower(key), "date") {
					if strings.Contains(strings.ReplaceAll(fmt.Sprintf("%v", val), "-", "_"), normalizedFilter) {
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}
		}

		out = append(out, row)
	}
	return out
}

func SearchOperatorPacketsBySID(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 10, 1000)
	optID := c.Query("opt_id")
	if optID == "" {
		respond.Error(c, http.StatusBadRequest, "opt_id query parameter is required", nil, nil)
		return
	}
	filters := parseSearchFilters(c)

	dataPath, err := db.GetDataPathByOptID(optID)
	if err != nil {
		log.Printf("[SearchOperatorPacketsBySID] DataPath lookup failed opt_id=%s user=%s: %v", optID, user.ADID, err)
		respond.Error(c, http.StatusNotFound, "Operator data path not found", err, gin.H{"operator_id": optID})
		return
	}

	s3Cfg := config.GetDefaultS3Config()
	filePath := strings.TrimSuffix(dataPath, "/") + "/sid.parquet"

	rows, err := s3store.FetchParquetRows[map[string]interface{}](s3Cfg, filePath)
	if err != nil {
		if s3store.IsNotFound(err) {
			log.Printf("[SearchOperatorPacketsBySID] S3 fetch failed key=%s user=%s: %v", filePath, user.ADID, err)
			respond.Error(c, http.StatusNotFound, "Failed to fetch sid.parquet file", err, gin.H{
				"regional_office": user.RegionalOffice,
				"operator_id":     optID,
				"file_path":       filePath,
			})
		} else {
			log.Printf("[SearchOperatorPacketsBySID] Parse failed key=%s user=%s: %v", filePath, user.ADID, err)
			respond.Error(c, http.StatusInternalServerError, "Failed to parse parquet data", err, nil)
		}
		return
	}

	// Drop empty rows (rows where all fields were skipped during decode).
	nonEmpty := rows[:0]
	for _, row := range rows {
		if len(row) > 0 {
			nonEmpty = append(nonEmpty, row)
		}
	}
	rows = nonEmpty

	log.Printf("[SearchOperatorPacketsBySID] Loaded %d rows from key=%s user=%s", len(rows), filePath, user.ADID)

	for _, row := range rows {
		normalizeDateFields(row)
	}

	filteredData := applySearchFilters(rows, filters)
	result := pageparam.Slice(filteredData, page, pageSize)

	log.Printf("[SearchOperatorPacketsBySID] Returning %d/%d rows for opt_id=%s user=%s (page %d)",
		len(result.Items), result.Total, optID, user.ADID, result.Page)
	respond.OK(c, gin.H{
		"regional_office":        user.RegionalOffice,
		"operator_id":            optID,
		"file_path":              filePath,
		"search_sid":             filters.sid,
		"anomaly_filter":         filters.anomalyFilter,
		"enrollment_type_filter": filters.enrollmentType,
		"date_filter":            filters.date,
		"pagination":             result.JSON(),
		"count":                  len(result.Items),
		"data":                   result.Items,
		"requested_by":           user.ADID,
	})
}
