package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

// GetSIDList fetches sid.parquet file for a specific operator and returns paginated data
func GetSIDList(c *gin.Context) {
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
	optState := c.Query("opt_state")       // e.g., "Rajasthan"
	optDistrict := c.Query("opt_district") // e.g., "Udaipur"
	optID := c.Query("opt_id")             // e.g., "WCD_RJ_UD_NS887326"

	// Validate required parameters
	if optState == "" || optDistrict == "" || optID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_state, opt_district, and opt_id query parameters are required",
		})
		return
	}

	// Convert spaces to underscores for S3 path compatibility
	optStateForPath := strings.ReplaceAll(optState, " ", "_")
	optDistrictForPath := strings.ReplaceAll(optDistrict, " ", "_")
	optIDForPath := strings.ReplaceAll(optID, " ", "_")

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path for sid.parquet
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/sid.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/sid.parquet"

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Download the parquet file from S3
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "SID list file not found",
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

	// Read parquet data from S3
	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Write parquetBytes to a temp file
	tempFile, err := os.CreateTemp("", "parquet_*.parquet")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	_, err = tempFile.Write(parquetBytes)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write to temp file",
			"details": err.Error(),
		})
		return
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Open parquet file
	fr, err := local.NewLocalFileReader(tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to open parquet file",
			"details": err.Error(),
		})
		return
	}
	defer fr.Close()

	// Create parquet reader
	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create parquet reader",
			"details": err.Error(),
		})
		return
	}
	defer pr.ReadStop()

	numRows := int(pr.GetNumRows())

	// Read all data
	strs, err := pr.ReadByNumber(numRows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read parquet data",
			"details": err.Error(),
		})
		return
	}

	// Convert to JSON-friendly format
	allData := make([]map[string]interface{}, 0, len(strs))
	for _, str := range strs {
		rowMap := make(map[string]interface{})
		jsonBytes, err := json.Marshal(str)
		if err != nil {
			continue
		}
		json.Unmarshal(jsonBytes, &rowMap)
		if len(rowMap) == 0 {
			continue
		}
		allData = append(allData, rowMap)
	}

	// Calculate pagination
	totalRecords := len(allData)
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
		paginatedData = allData[startIndex:endIndex]
	}

	// Return the data as JSON with pagination
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"state":           optState,
		"district":        optDistrict,
		"file":            fileName,
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
