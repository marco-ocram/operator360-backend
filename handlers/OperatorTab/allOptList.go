package OperatorTab

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"opt360-portal-backend/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)




func GetOperatorList(c *gin.Context) {
	
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	page := 1
	pageSize := 20
	
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := fmt.Sscanf(pageParam, "%d", &page); err == nil && p == 1 && page > 0 {
			
		} else {
			page = 1
		}
	}
	
	if pageSizeParam := c.Query("page_size"); pageSizeParam != "" {
		if ps, err := fmt.Sscanf(pageSizeParam, "%d", &pageSize); err == nil && ps == 1 && pageSize > 0 && pageSize <= 1000 {
		
		} else {
			pageSize = 20
		}
	}

	// Get optional query parameters for filtering
	optEa := c.Query("opt_ea")             // Filter by EA
	optReg := c.Query("opt_reg")           // Filter by region
	optDistrict := c.Query("opt_district") // Filter by district
	optState := c.Query("opt_state")       // Filter by state
	optID := c.Query("opt_id")             // Search by operator ID
	activeStatus := c.Query("active_status") // Filter by active status
	risk := c.Query("risk")                  // Filter by risk level (high, med, low)

	
	s3Cfg := config.GetDefaultS3Config()


	fileName := "opt360Store/" + utils.ToPascalCase(user.RegionalOffice) + "/operator.parquet"


	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		log.Printf("[GetOperatorList] S3 client error user=%s: %v", user.ADID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create S3 client", "details": err.Error()})
		return
	}

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		log.Printf("[GetOperatorList] S3 fetch failed key=%s user=%s: %v", fileName, user.ADID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Operator list file not found", "regional_office": user.RegionalOffice,
			"file_path": fileName, "details": err.Error(),
		})
		return
	}
	defer result.Body.Close()

	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("[GetOperatorList] Read body failed key=%s: %v", fileName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read S3 data", "details": err.Error()})
		return
	}

	tempFile, err := os.CreateTemp("", "parquet_*.parquet")
	if err != nil {
		log.Printf("[GetOperatorList] CreateTemp error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file", "details": err.Error()})
		return
	}
	if _, err = tempFile.Write(parquetBytes); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		log.Printf("[GetOperatorList] Write temp error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write to temp file", "details": err.Error()})
		return
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	fr, err := local.NewLocalFileReader(tempFile.Name())
	if err != nil {
		log.Printf("[GetOperatorList] Open parquet error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open parquet file", "details": err.Error()})
		return
	}
	defer fr.Close()

	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		log.Printf("[GetOperatorList] Parquet reader error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create parquet reader", "details": err.Error()})
		return
	}
	defer pr.ReadStop()

	numRows := int(pr.GetNumRows())
	strs, err := pr.ReadByNumber(numRows)
	if err != nil {
		log.Printf("[GetOperatorList] Read parquet data error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read parquet data", "details": err.Error()})
		return
	}

	totalCount := 0
	filteredData := make([]map[string]interface{}, 0)
	
	for _, str := range strs {
		totalCount++
		
		rowMap := make(map[string]interface{})
		jsonBytes, err := json.Marshal(str)
		if err != nil {
			continue
		}
		json.Unmarshal(jsonBytes, &rowMap)
		if len(rowMap) == 0 {
			continue
		}
		
		match := true

		
		if optState != "" {
			matched := false
			for key, val := range rowMap {
				if strings.EqualFold(key, "Opt_state") || strings.EqualFold(key, "opt_state") {
					valStr := fmt.Sprintf("%v", val)
					if strings.EqualFold(valStr, optState) {
						matched = true
						break
					}
				}
			}
			if !matched {
				match = false
			}
		}

		
		if optID != "" && match {
			matched := false
			for key, val := range rowMap {
				if strings.EqualFold(key, "Opt_id") || strings.EqualFold(key, "opt_id") {
					valStr := fmt.Sprintf("%v", val)
					if strings.Contains(strings.ToLower(valStr), strings.ToLower(optID)) {
						matched = true
						break
					}
				}
			}
			if !matched {
				match = false
			}
		}

		
		if optEa != "" && match {
			matched := false
			for key, val := range rowMap {
				if strings.EqualFold(key, "Opt_ea") || strings.EqualFold(key, "opt_ea") {
					valStr := fmt.Sprintf("%v", val)
					if strings.EqualFold(valStr, optEa) {
						matched = true
						break
					}
				}
			}
			if !matched {
				match = false
			}
		}

	
		if optReg != "" && match {
			matched := false
			for key, val := range rowMap {
				if strings.EqualFold(key, "Opt_reg") || strings.EqualFold(key, "opt_reg") {
					valStr := fmt.Sprintf("%v", val)
					if strings.EqualFold(valStr, optReg) {
						matched = true
						break
					}
				}
			}
			if !matched {
				match = false
			}
		}

		
		if optDistrict != "" && match {
			matched := false
			for key, val := range rowMap {
				if strings.EqualFold(key, "Opt_district") || strings.EqualFold(key, "opt_district") {
					valStr := fmt.Sprintf("%v", val)
					if strings.EqualFold(valStr, optDistrict) {
						matched = true
						break
					}
				}
			}
			if !matched {
				match = false
			}
		}

		// Filter by active status
		if activeStatus != "" && match {
			matched := false
			var expectedValue interface{}
			
			// Map string parameter to numeric value
			if strings.EqualFold(activeStatus, "active") {
				expectedValue = 1
			} else if strings.EqualFold(activeStatus, "inactive") {
				expectedValue = 0
			} else {
				// If neither "active" nor "inactive", skip this filter
				matched = true
			}
			
			if !matched {
				for key, val := range rowMap {
					if strings.EqualFold(key, "Active_status") || strings.EqualFold(key, "active_status") {
						// Convert val to int for comparison
						switch v := val.(type) {
						case int:
							if v == expectedValue {
								matched = true
							}
						case float64:
							if int(v) == expectedValue {
								matched = true
							}
						case string:
							if valStr := v; valStr == fmt.Sprintf("%v", expectedValue) {
								matched = true
							}
						}
						break
					}
				}
			}
			
			if !matched {
				match = false
			}
		}

		// Filter by risk level
		if risk != "" && match {
			matched := false
			for key, val := range rowMap {
				if strings.EqualFold(key, "Opt_risk_score") || strings.EqualFold(key, "opt_risk_score") {
					var riskScore float64
					switch v := val.(type) {
					case float64:
						riskScore = v
					case float32:
						riskScore = float64(v)
					case int:
						riskScore = float64(v)
					case string:
						fmt.Sscanf(v, "%f", &riskScore)
					default:
						continue
					}
					
					if strings.EqualFold(risk, "high") && riskScore > 0.7 {
						matched = true
					} else if strings.EqualFold(risk, "med") && riskScore >= 0.4 && riskScore <= 0.7 {
						matched = true
					} else if strings.EqualFold(risk, "low") && riskScore < 0.4 {
						matched = true
					}
					break
				}
			}
			if !matched {
				match = false
			}
		}

		
		if match {
			filteredData = append(filteredData, rowMap)
		}
	}

	totalFiltered := len(filteredData)
	totalPages := (totalFiltered + pageSize - 1) / pageSize
	
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	
	if startIndex >= totalFiltered {
		startIndex = 0
		endIndex = 0
	} else if endIndex > totalFiltered {
		endIndex = totalFiltered
	}
	
	paginatedData := []map[string]interface{}{}
	if startIndex < endIndex {
		paginatedData = filteredData[startIndex:endIndex]
	}


	log.Printf("[GetOperatorList] Returning %d/%d operators for ro=%s (page %d)",
		len(paginatedData), totalFiltered, user.RegionalOffice, page)
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"total_count":     totalCount,
		"filtered_count":  totalFiltered,
		"pagination": gin.H{
			"page":          page,
			"page_size":     pageSize,
			"total_records": totalFiltered,
			"total_pages":   totalPages,
			"has_next":      page < totalPages,
			"has_previous":  page > 1,
		},
		"filters": gin.H{
			"opt_ea":       optEa,
			"opt_reg":      optReg,
			"opt_district": optDistrict,
			"opt_state":    optState,
			"opt_id":       optID,
			"active_status": activeStatus,
			"risk":          risk,
		},
		"count": len(paginatedData),
		"data":  paginatedData,
	})
}