package SidReview

import (

	"encoding/json"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"strconv"
	"strings"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"

	
)

// toCamelCase converts a string with spaces to PascalCase
func toCamelCaseSid(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	
	result := ""
	for _, word := range words {
		result += strings.Title(strings.ToLower(word))
	}
	return result
}


func GetAnamolousSIDs(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get pagination parameters
	page := 1
	pageSize := 20
	
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		} else {
			page = 1
		}
	}
	
	if pageSizeParam := c.Query("page_size"); pageSizeParam != "" {
		if ps, err := strconv.Atoi(pageSizeParam); err == nil && ps > 0 && ps <= 1000 {
			pageSize = ps
		} else {
			pageSize = 20
		}
	}

	// Get required query parameters
	optState := c.Query("opt_state")       // e.g., "Maharashtra"
	optDistrict := c.Query("opt_district") // e.g., "Sangli"
	optID := c.Query("opt_id")             // e.g., "MH_WMIT_SN_NS046496"
	
	// Get optional filter parameter
	anomalyCategoryFilter := c.Query("anomaly_category") // e.g., "work", "hardware", "suspicious", "document", "biometrics" (optional)

	// Validate required parameters
	if optState == "" || optDistrict == "" || optID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_state, opt_district, and opt_id query parameters are required",
		})
		return
	}

	// Convert spaces to PascalCase for S3 path compatibility
	optStateForPath := toCamelCaseSid(optState)
	optDistrictForPath := toCamelCaseSid(optDistrict)
	optIDForPath := optID 

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/anomaly_sid.json
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/anomaly_sid.json"

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Download the JSON file from S3
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "Anomalous SIDs file not found",
			"regional_office": user.RegionalOffice,
			"operator_id":     optID,
			"state":           optState,
			"district":        optDistrict,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read JSON data from S3
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Parse the nested JSON structure
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse JSON data",
			"details": err.Error(),
		})
		return
	}

	// Flatten the nested structure to extract all SIDs
	allData := make([]map[string]interface{}, 0)
	
	// Iterate through operator IDs (e.g., "WCD_RJ_UD_NS887326")
	for operatorID, operatorData := range rawData {
		if operatorMap, ok := operatorData.(map[string]interface{}); ok {
			// Iterate through anomaly categories (e.g., "Biometrics")
			for category, categoryData := range operatorMap {
				if categoryMap, ok := categoryData.(map[string]interface{}); ok {
					// Iterate through SIDs (e.g., "0862533020198020251030134051")
					for sid, sidData := range categoryMap {
						if sidMap, ok := sidData.(map[string]interface{}); ok {
							// Create a flattened record with SID as key
							record := map[string]interface{}{
								"sid":               sid,
								"operator_id":       operatorID,
								"anomaly_category":  category,
							}
							// Add all SID data fields
							for key, value := range sidMap {
								record[key] = value
							}
							allData = append(allData, record)
						}
					}
				}
			}
		}
	}

	// Apply anomaly_category filter if provided
	filteredData := allData
	if anomalyCategoryFilter != "" {
		filteredData = make([]map[string]interface{}, 0)
		for _, record := range allData {
			if category, ok := record["anomaly_category"].(string); ok {
				// Case-insensitive comparison
				if strings.EqualFold(category, anomalyCategoryFilter) {
					filteredData = append(filteredData, record)
				}
			}
		}
	}

	// Calculate pagination
	totalRecords := len(filteredData)
	totalPages := (totalRecords + pageSize - 1) / pageSize
	
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	
	if startIndex >= totalRecords {
		startIndex = 0
		endIndex = 0
	} else if endIndex > totalRecords {
		endIndex = totalRecords
	}
	
	paginatedData := []map[string]interface{}{}
	if startIndex < endIndex {
		paginatedData = filteredData[startIndex:endIndex]
	}

	// Return data with pagination
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"state":           optState,
		"district":        optDistrict,
		"file":            fileName,
		"anomaly_category_filter": anomalyCategoryFilter,
		"pagination": gin.H{
			"page":          page,
			"page_size":     pageSize,
			"total_records": totalRecords,
			"total_pages":   totalPages,
			"has_next":      page < totalPages,
			"has_previous":  page > 1,
		},
		"count":        len(paginatedData),
		"data":         paginatedData,
		"requested_by": user.ADID,
	})
}