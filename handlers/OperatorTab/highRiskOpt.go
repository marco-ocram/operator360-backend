package OperatorTab

import (

	"encoding/json"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"strconv"
	"os"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)







// GetHighRiskOperators fetches operator_high.parquet file based on user's regional office
func GetHighRiskOperators(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error":  "User not found in context"})
		return
	}

	user := userInterface.(*models. User)

	// Get pagination parameters
	page := 1
	pageSize := 20 // Default page size
	
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

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/operator_high.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/operator_high.parquet"

	// Create S3 client
	s3Client, err := config. NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Download the parquet file from S3
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket:  aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":            "Parquet file not found",
			"regional_office":  user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Stream parquet data directly from S3 into memory
	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		c. JSON(http.StatusInternalServerError, gin.H{
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
	_, err = tempFile. Write(parquetBytes)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		c.JSON(http.StatusInternalServerError, gin. H{
			"error":   "Failed to write to temp file",
			"details": err.Error(),
		})
		return
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	fr, err := local.NewLocalFileReader(tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to open parquet file",
			"details": err.Error(),
		})
		return
	}
	defer fr.Close()

	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create parquet reader",
			"details":  err.Error(),
		})
		return
	}
	defer pr.ReadStop()

	numRows := int(pr.GetNumRows())

	// Read all data at once using ReadByNumber
	strs, err := pr.ReadByNumber(numRows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read parquet data",
			"details": err.Error(),
		})
		return
	}

	// Convert parquet data to Operator structs
	allOperators := make([]models.Operator, 0, len(strs))
	for _, str := range strs {
		// Convert the interface to JSON bytes first
		jsonBytes, err := json.Marshal(str)
		if err != nil {
			continue 
		}

		
		var operator models.Operator
		if err := json.Unmarshal(jsonBytes, &operator); err != nil {
			continue 
		}

		allOperators = append(allOperators, operator)
	}

	// Calculate pagination
	totalRecords := len(allOperators)
	totalPages := (totalRecords + pageSize - 1) / pageSize
	
	// Validate page number
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	
	// Calculate start and end indices
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	
	if startIndex >= totalRecords {
		startIndex = 0
		endIndex = 0
	} else if endIndex > totalRecords {
		endIndex = totalRecords
	}
	
	// Get paginated data
	paginatedOperators := []models.Operator{}
	if startIndex < endIndex {
		paginatedOperators = allOperators[startIndex:endIndex]
	}

	// Return the data as JSON with pagination info
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"pagination": gin.H{
			"page":          page,
			"page_size":     pageSize,
			"total_records": totalRecords,
			"total_pages":   totalPages,
			"has_next":      page < totalPages,
			"has_previous":  page > 1,
		},
		"count":  len(paginatedOperators),
		"data":  paginatedOperators,
	})
}