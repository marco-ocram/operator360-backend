package OperatorTab

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"opt360-portal-backend/config"
	"opt360-portal-backend/models"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

// GetLowRiskOperators fetches operator_low.parquet file based on user's regional office
func GetLowRiskOperators(c *gin.Context) {
	// Get user from context (set by auth middleware)
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
		if p, err := fmt.Sscanf(pageParam, "%d", &page); err == nil && p == 1 && page > 0 {
			// page is valid
		} else {
			page = 1
		}
	}
	
	if pageSizeParam := c.Query("page_size"); pageSizeParam != "" {
		if ps, err := fmt.Sscanf(pageSizeParam, "%d", &pageSize); err == nil && ps == 1 && pageSize > 0 && pageSize <= 1000 {
			// pageSize is valid
		} else {
			pageSize = 20
		}
	}

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/operator_low.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/operator_low.parquet"

	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		log.Printf("[GetLowRiskOperators] S3 client error user=%s: %v", user.ADID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create S3 client", "details": err.Error()})
		return
	}

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		log.Printf("[GetLowRiskOperators] S3 fetch failed key=%s user=%s: %v", fileName, user.ADID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Low risk operators file not found", "regional_office": user.RegionalOffice,
			"file": fileName, "details": err.Error(),
		})
		return
	}
	defer result.Body.Close()

	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("[GetLowRiskOperators] Read body failed key=%s: %v", fileName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read S3 data", "details": err.Error()})
		return
	}

	tempFile, err := os.CreateTemp("", "parquet_*.parquet")
	if err != nil {
		log.Printf("[GetLowRiskOperators] CreateTemp error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file", "details": err.Error()})
		return
	}
	if _, err = tempFile.Write(parquetBytes); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		log.Printf("[GetLowRiskOperators] Write temp error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write to temp file", "details": err.Error()})
		return
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	fr, err := local.NewLocalFileReader(tempFile.Name())
	if err != nil {
		log.Printf("[GetLowRiskOperators] Open parquet error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open parquet file", "details": err.Error()})
		return
	}
	defer fr.Close()

	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		log.Printf("[GetLowRiskOperators] Parquet reader error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create parquet reader", "details": err.Error()})
		return
	}
	defer pr.ReadStop()

	numRows := int(pr.GetNumRows())
	strs, err := pr.ReadByNumber(numRows)
	if err != nil {
		log.Printf("[GetLowRiskOperators] Read parquet data error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read parquet data", "details": err.Error()})
		return
	}

	// Convert to JSON-friendly format
	allData := make([]interface{}, 0, len(strs))
	for _, str := range strs {
		var row map[string]interface{}
		jsonStr := fmt.Sprintf("%v", str)
		if err := json.Unmarshal([]byte(jsonStr), &row); err == nil {
			allData = append(allData, row)
		} else {
			allData = append(allData, str)
		}
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
	
	paginatedData := []interface{}{}
	if startIndex < endIndex {
		paginatedData = allData[startIndex:endIndex]
	}

	log.Printf("[GetLowRiskOperators] Returning %d/%d low-risk operators for ro=%s (page %d)",
		len(paginatedData), totalRecords, user.RegionalOffice, page)
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
		"count": len(paginatedData),
		"data":  paginatedData,
	})
}