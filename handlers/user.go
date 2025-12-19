package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"strings"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

// GetUserInfo returns the current user's information and regional office
func GetUserInfo(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetKPIData fetches kpi.json file based on user's regional office
func GetKPIData(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/kpi.json
	fileName := "opt360Store/" + user.RegionalOffice + "/kpi.json"

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Get the object from S3
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "KPI file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read the file content
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read file content",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON to validate it's valid JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Invalid JSON in file",
			"details": err.Error(),
		})
		return
	}

	// Return the JSON content
	c.JSON(http.StatusOK, jsonData)
}

// GetHighRiskOperators fetches operator_high.parquet file based on user's regional office
func GetHighRiskOperators(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get pagination parameters
	page := 1
	pageSize := 20 // Default page size
	
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
			pageSize = 50
		}
	}

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/operator_high.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/operator_high.parquet"

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
			"error":           "Parquet file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Stream parquet data directly from S3 into memory
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
			"details": err.Error(),
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

	// Convert to JSON-friendly format
	allData := make([]interface{}, 0, len(strs))
	for _, str := range strs {
		var row map[string]interface{}
		jsonStr := fmt.Sprintf("%v", str)
		// Try to parse as JSON
		if err := json.Unmarshal([]byte(jsonStr), &row); err == nil {
			allData = append(allData, row)
		} else {
			// If not valid JSON, return the raw interface
			allData = append(allData, str)
		}
	}

	// Calculate pagination
	totalRecords := len(allData)
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
	paginatedData := []interface{}{}
	if startIndex < endIndex {
		paginatedData = allData[startIndex:endIndex]
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
		"count": len(paginatedData),
		"data":  paginatedData,
	})
}

// GetMediumRiskOperators fetches operator_medium.parquet file based on user's regional office
func GetMediumRiskOperators(c *gin.Context) {
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
	// Format: opt360Store/{RegionalOffice}/operator_medium.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/operator_medium.parquet"

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
			"error":           "Parquet file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Stream parquet data directly from S3 into memory
	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Create in-memory parquet reader
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
			"details": err.Error(),
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

	// Return the data as JSON with pagination
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
			"error":           "Low risk operators file not found",
			"regional_office": user.RegionalOffice,
			"file":            fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Stream parquet data directly from S3 into memory
	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Create in-memory parquet reader
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
			"details": err.Error(),
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

	// Return the data as JSON with pagination
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

// GetOperatorDetails fetches operator-specific opt_details.parquet file
func GetOperatorDetails(c *gin.Context) {
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

	// Get required query parameters
	optState := c.Query("opt_state")       // e.g., "Maharashtra"
	optDistrict := c.Query("opt_district") // e.g., "Sangli"
	optID := c.Query("opt_id")             // e.g., "MH_WMIT_SN_NS046496"

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

	// Build file path for operator-specific details
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/opt_details.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/opt_details.parquet"

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
			"error":           "Operator details file not found",
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

	// Stream parquet data directly from S3 into memory
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
			"details": err.Error(),
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

// GetOperatorList fetches operator.parquet file with optional filters
func GetOperatorList(c *gin.Context) {
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

	// Get optional query parameters for filtering
	optEa := c.Query("opt_ea")             // Filter by EA
	optReg := c.Query("opt_reg")           // Filter by region
	optDistrict := c.Query("opt_district") // Filter by district
	optState := c.Query("opt_state")       // Filter by state
	optID := c.Query("opt_id")             // Search by operator ID

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/operator.parquet
	fileName := "opt360Store/" + user.RegionalOffice + "/operator.parquet"

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
			"error":           "Operator list file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Stream parquet data directly from S3 into memory
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
			"details": err.Error(),
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

	// Convert and filter in single pass to minimize memory usage
	totalCount := 0
	filteredData := make([]map[string]interface{}, 0)
	
	for _, str := range strs {
		totalCount++
		
		// Convert to map
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

		// Filter by opt_state (exact match, case-insensitive)
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

		// Search by opt_id (case-insensitive partial match)
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

		// Filter by opt_ea (exact match, case-insensitive)
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

		// Filter by opt_reg (exact match, case-insensitive)
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

		// Filter by opt_district (exact match, case-insensitive)
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

		// Add to filtered results if all filters match
		if match {
			filteredData = append(filteredData, rowMap)
		}
	}

	// Calculate pagination on filtered data
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

	// Return the filtered data as JSON with pagination
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
		},
		"count": len(paginatedData),
		"data":  paginatedData,
	})
}

// GetROQRiskDistribution fetches kpi.json file from S3
func GetROQRiskDistribution(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path
	// Format: opt360Store/kpi.json
	fileName := "opt360Store/kpi.json"

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Get the object from S3
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "RO risk distribution file not found",
			"file_path": fileName,
			"details":   err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read the file content
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read file content",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON to validate it's valid JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Invalid JSON in file",
			"details": err.Error(),
		})
		return
	}

	// Return the JSON content with user info
	c.JSON(http.StatusOK, gin.H{
		"file":            fileName,
		"requested_by":    user.ADID,
		"regional_office": user.RegionalOffice,
		"data":            jsonData,
	})
}

// GetOperatorPackets fetches the latest date's sid_*.parquet files for an operator
func GetOperatorPackets(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get pagination parameters
	page := 1
	pageSize := 10
	
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
			pageSize = 10
		}
	}

	// Get required query parameters
	optState := c.Query("opt_state")       // e.g., "Gujarat"
	optDistrict := c.Query("opt_district") // e.g., "Vadodara"
	optID := c.Query("opt_id")             // e.g., "GJ_DOP_VDR_NS764416"


	if optState == "" || optDistrict == "" || optID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_state, opt_district, and opt_id query parameters are required",
		})
		return
	}

	
	optStateForPath := strings.ReplaceAll(optState, " ", "_")
	optDistrictForPath := strings.ReplaceAll(optDistrict, " ", "_")
	optIDForPath := strings.ReplaceAll(optID, " ", "_")

	
	s3Cfg := config.GetDefaultS3Config()


	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Build folder path
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/
	folderPath := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/"

	// List all objects in the folder
	listResult, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(s3Cfg.BucketName),
		Prefix: aws.String(folderPath),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list S3 objects",
			"details": err.Error(),
		})
		return
	}

	// Parse files and extract dates
	type FileInfo struct {
		Path       string
		Date       string
		PacketType string // "U" or "N"
	}

	var allFiles []FileInfo
	for _, obj := range listResult.Contents {
		fileName := *obj.Key
		// Extract just the filename from the full path
		parts := strings.Split(fileName, "/")
		baseName := parts[len(parts)-1]

		// Check if it matches sid_YYYY_MM_DD_X.parquet pattern
		if strings.HasPrefix(baseName, "sid_") && strings.HasSuffix(baseName, ".parquet") {
			// Extract date and packet type: sid_2025_12_04_U.parquet
			// Remove "sid_" prefix and ".parquet" suffix
			middle := strings.TrimPrefix(baseName, "sid_")
			middle = strings.TrimSuffix(middle, ".parquet")

			// Split by underscore: [2025, 12, 04, U]
			dateParts := strings.Split(middle, "_")
			if len(dateParts) == 4 {
				dateStr := dateParts[0] + "_" + dateParts[1] + "_" + dateParts[2] // "2025_12_04"
				packetType := dateParts[3]                                         // "U" or "N"

				allFiles = append(allFiles, FileInfo{
					Path:       fileName,
					Date:       dateStr,
					PacketType: packetType,
				})
			}
		}
	}

	// If no parquet files found
	if len(allFiles) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "No packet files found for the operator",
			"regional_office": user.RegionalOffice,
			"operator_id":     optID,
			"state":           optState,
			"district":        optDistrict,
			"folder_path":     folderPath,
		})
		return
	}

	// Find the latest date
	latestDate := ""
	for _, file := range allFiles {
		if file.Date > latestDate {
			latestDate = file.Date
		}
	}

	// Filter files for the latest date
	var latestFiles []FileInfo
	for _, file := range allFiles {
		if file.Date == latestDate {
			latestFiles = append(latestFiles, file)
		}
	}

	// Helper function to read a single parquet file from S3
	readParquetFile := func(filePath string) ([]map[string]interface{}, error) {
		result, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(s3Cfg.BucketName),
			Key:    aws.String(filePath),
		})
		if err != nil {
			return nil, err
		}
		defer result.Body.Close()

		parquetBytes, err := io.ReadAll(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read S3 data: %w", err)
		}

		tempFile, err := os.CreateTemp("", "parquet_*.parquet")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		_, err = tempFile.Write(parquetBytes)
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			return nil, fmt.Errorf("failed to write to temp file: %w", err)
		}
		tempFile.Close()
		defer os.Remove(tempFile.Name())

		fr, err := local.NewLocalFileReader(tempFile.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to open parquet file: %w", err)
		}
		defer fr.Close()

		pr, err := reader.NewParquetReader(fr, nil, 4)
		if err != nil {
			return nil, fmt.Errorf("failed to create parquet reader: %w", err)
		}
		defer pr.ReadStop()

		numRows := int(pr.GetNumRows())

		strs, err := pr.ReadByNumber(numRows)
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet data: %w", err)
		}

		data := make([]map[string]interface{}, 0)
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
			data = append(data, rowMap)
		}

		return data, nil
	}

	// Read all latest date files and combine data
	combinedData := make([]map[string]interface{}, 0)
	filesProcessed := make([]gin.H, 0)
	totalRecords := 0

	for _, fileInfo := range latestFiles {
		data, err := readParquetFile(fileInfo.Path)

		// Extract filename
		parts := strings.Split(fileInfo.Path, "/")
		fileName := parts[len(parts)-1]

		// Determine packet type name
		packetTypeName := "unknown"
		if fileInfo.PacketType == "U" {
			packetTypeName = "update"
		} else if fileInfo.PacketType == "N" {
			packetTypeName = "new_enrollment"
		}

		fileProcessInfo := gin.H{
			"file_name":    fileName,
			"file_path":    fileInfo.Path,
			"date":         fileInfo.Date,
			"packet_type":  packetTypeName,
			"record_count": 0,
			"status":       "failed",
		}

		if err != nil {
			fileProcessInfo["error"] = err.Error()
		} else {
			fileProcessInfo["record_count"] = len(data)
			fileProcessInfo["status"] = "success"
			totalRecords += len(data)

			// Add metadata to each row
			for _, row := range data {
				row["source_file"] = fileName
				row["packet_type"] = packetTypeName
				row["date"] = fileInfo.Date
				combinedData = append(combinedData, row)
			}
		}

		filesProcessed = append(filesProcessed, fileProcessInfo)
	}

	// Calculate pagination
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
		paginatedData = combinedData[startIndex:endIndex]
	}

	// Return combined data with pagination
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"state":           optState,
		"district":        optDistrict,
		"folder_path":     folderPath,
		"latest_date":     latestDate,
		"files_found":     len(latestFiles),
		"files_processed": filesProcessed,
		"total_records":   totalRecords,
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

// SearchOperatorPacketsBySID searches for a specific SID in packets for a given date
func SearchOperatorPacketsBySID(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get pagination parameters
	page := 1
	pageSize := 50
	
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
			pageSize = 50
		}
	}

	// Get required query parameters
	optState := c.Query("opt_state")       // e.g., "Gujarat"
	optDistrict := c.Query("opt_district") // e.g., "Vadodara"
	optID := c.Query("opt_id")             // e.g., "GJ_DOP_VDR_NS764416"
	selectedDate := c.Query("date")        // e.g., "2025_12_12"
	searchSID := c.Query("sid")            // e.g., "123456789012" (optional)
	anomalyFilter := c.Query("anomaly_filter") // "anomalous" or "non-anomalous" (optional)
	enrollmentTypeFilter := c.Query("enrollment_type") // "new_enrollment" or "update" (optional)

	// Validate required parameters
	if optState == "" || optDistrict == "" || optID == "" || selectedDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_state, opt_district, opt_id, and date query parameters are required",
		})
		return
	}

	// Convert spaces to underscores for S3 path compatibility
	optStateForPath := strings.ReplaceAll(optState, " ", "_")
	optDistrictForPath := strings.ReplaceAll(optDistrict, " ", "_")
	optIDForPath := strings.ReplaceAll(optID, " ", "_")

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Build folder path
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/
	folderPath := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/"

	// Build file paths for the selected date
	// Format: all_sid_YYYY_MM_DD_U.parquet and all_sid_YYYY_MM_DD_N.parquet
	updateFile := folderPath + "all_sid_" + selectedDate + "_U.parquet"
	newEnrollmentFile := folderPath + "all_sid_" + selectedDate + "_N.parquet"

	// Helper function to read and filter parquet file
	readAndFilterParquetFile := func(filePath string) ([]map[string]interface{}, error) {
		result, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(s3Cfg.BucketName),
			Key:    aws.String(filePath),
		})
		if err != nil {
			return nil, err
		}
		defer result.Body.Close()

		parquetBytes, err := io.ReadAll(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read S3 data: %w", err)
		}

		tempFile, err := os.CreateTemp("", "parquet_*.parquet")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		_, err = tempFile.Write(parquetBytes)
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			return nil, fmt.Errorf("failed to write to temp file: %w", err)
		}
		tempFile.Close()
		defer os.Remove(tempFile.Name())

		fr, err := local.NewLocalFileReader(tempFile.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to open parquet file: %w", err)
		}
		defer fr.Close()

		pr, err := reader.NewParquetReader(fr, nil, 4)
		if err != nil {
			return nil, fmt.Errorf("failed to create parquet reader: %w", err)
		}
		defer pr.ReadStop()

		numRows := int(pr.GetNumRows())

		strs, err := pr.ReadByNumber(numRows)
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet data: %w", err)
		}

		data := make([]map[string]interface{}, 0)
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

			// If SID search is provided, filter by SID
			if searchSID != "" {
				matchFound := false
				for key, val := range rowMap {
					if strings.EqualFold(key, "eid") {
						valStr := fmt.Sprintf("%v", val)
						if strings.Contains(strings.ToLower(valStr), strings.ToLower(searchSID)) {
							matchFound = true
							break
						}
					}
				}
				if !matchFound {
					continue
				}
			}

			// If anomaly filter is provided, filter by anomaly status
			if anomalyFilter != "" {
				isAnomalous := false
				
				// Check Anomaly_type field
				for key, val := range rowMap {
					if strings.EqualFold(key, "anomaly_type") {
						// Check if the value is a non-empty array
						switch v := val.(type) {
						case []interface{}:
							isAnomalous = len(v) > 0
						case []string:
							isAnomalous = len(v) > 0
						case string:
							// Handle string representation of arrays
							v = strings.TrimSpace(v)
							isAnomalous = v != "[]" && v != "" && v != "null"
						default:
							// If it's some other type, check if it's not nil/empty
							isAnomalous = val != nil
						}
						break
					}
				}
				
				// Apply filter based on anomaly status
				if strings.EqualFold(anomalyFilter, "anomalous") && !isAnomalous {
					continue
				}
				if strings.EqualFold(anomalyFilter, "non-anomalous") && isAnomalous {
					continue
				}
			}

			// If enrollment type filter is provided, filter by enrollment type
			if enrollmentTypeFilter != "" {
				enrollmentType := ""
				
				// Check Enrolnment_type field
				for key, val := range rowMap {
					if strings.EqualFold(key, "enrolnment_type") {
						valStr := fmt.Sprintf("%v", val)
						enrollmentType = strings.TrimSpace(valStr)
						break
					}
				}
				
				// Map the enrollment type: "U" = update, "N" = new_enrollment
				if strings.EqualFold(enrollmentTypeFilter, "update") && !strings.EqualFold(enrollmentType, "U") {
					continue
				}
				if strings.EqualFold(enrollmentTypeFilter, "new_enrollment") && !strings.EqualFold(enrollmentType, "N") {
					continue
				}
			}

			data = append(data, rowMap)
		}

		return data, nil
	}

	// Try to read update file (_U)
	updateData, updateErr := readAndFilterParquetFile(updateFile)

	// Try to read new enrollment file (_N)
	newEnrollmentData, newEnrollmentErr := readAndFilterParquetFile(newEnrollmentFile)

	// Check if both files are missing
	if updateErr != nil && newEnrollmentErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "No packet files found for the selected date",
			"regional_office": user.RegionalOffice,
			"operator_id":     optID,
			"state":           optState,
			"district":        optDistrict,
			"date":            selectedDate,
			"folder_path":     folderPath,
		})
		return
	}

	// Combine data from both files
	combinedData := make([]map[string]interface{}, 0)
	filesProcessed := make([]gin.H, 0)

	if updateErr == nil {
		filesProcessed = append(filesProcessed, gin.H{
			"file_name":    "all_sid_" + selectedDate + "_U.parquet",
			"packet_type":  "update",
			"record_count": len(updateData),
			"status":       "success",
		})
		// Add packet type to each row
		for _, row := range updateData {
			row["packet_type"] = "update"
			row["date"] = selectedDate
			combinedData = append(combinedData, row)
		}
	} else {
		filesProcessed = append(filesProcessed, gin.H{
			"file_name":   "all_sid_" + selectedDate + "_U.parquet",
			"packet_type": "update",
			"status":      "not_found",
			"error":       updateErr.Error(),
		})
	}

	if newEnrollmentErr == nil {
		filesProcessed = append(filesProcessed, gin.H{
			"file_name":    "all_sid_" + selectedDate + "_N.parquet",
			"packet_type":  "new_enrollment",
			"record_count": len(newEnrollmentData),
			"status":       "success",
		})
		// Add packet type to each row
		for _, row := range newEnrollmentData {
			row["packet_type"] = "new_enrollment"
			row["date"] = selectedDate
			combinedData = append(combinedData, row)
		}
	} else {
		filesProcessed = append(filesProcessed, gin.H{
			"file_name":   "all_sid_" + selectedDate + "_N.parquet",
			"packet_type": "new_enrollment",
			"status":      "not_found",
			"error":       newEnrollmentErr.Error(),
		})
	}

	// Calculate pagination
	totalRecords := len(combinedData)
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
		paginatedData = combinedData[startIndex:endIndex]
	}

	// Return combined data with pagination
	c.JSON(http.StatusOK, gin.H{
		"regional_office":      user.RegionalOffice,
		"operator_id":          optID,
		"state":                optState,
		"district":             optDistrict,
		"folder_path":          folderPath,
		"selected_date":        selectedDate,
		"search_sid":           searchSID,
		"anomaly_filter":       anomalyFilter,
		"enrollment_type_filter": enrollmentTypeFilter,
		"files_processed":      filesProcessed,
		"total_records":        totalRecords,
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

// SubmitFeedback receives feedback data and writes it as a JSON file to S3
func SubmitFeedback(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Parse request body
	var feedbackData map[string]interface{}
	if err := c.ShouldBindJSON(&feedbackData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid JSON in request body",
			"details": err.Error(),
		})
		return
	}

	// Get required parameters from request body
	optState, optStateExists := feedbackData["opt_state"].(string)
	optDistrict, optDistrictExists := feedbackData["opt_district"].(string)
	optID, optIDExists := feedbackData["opt_id"].(string)

	// Validate required parameters
	if !optStateExists || !optDistrictExists || !optIDExists || optState == "" || optDistrict == "" || optID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_state, opt_district, and opt_id are required in request body",
		})
		return
	}

	// Convert spaces to underscores for S3 path compatibility
	optStateForPath := strings.ReplaceAll(optState, " ", "_")
	optDistrictForPath := strings.ReplaceAll(optDistrict, " ", "_")
	optIDForPath := strings.ReplaceAll(optID, " ", "_")

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Generate date for filename (YYYY_MM_DD format)
	currentDate := time.Now().Format("2006_01_02")
	
	// Build file path
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/{YYYY_MM_DD}_{ad-id}.json
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/" + currentDate + "_" + user.ADID + ".json"

	// Convert feedback data to JSON
	jsonData, err := json.MarshalIndent(feedbackData, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to marshal feedback data to JSON",
			"details": err.Error(),
		})
		return
	}

	// Upload JSON file to S3
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s3Cfg.BucketName),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to upload feedback file to S3",
			"details": err.Error(),
		})
		return
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"message":         "Feedback submitted successfully",
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"state":           optState,
		"district":        optDistrict,
		"file_path":       fileName,
		"date":            currentDate,
		"submitted_by":    user.ADID,
	})
}

func GetAnamolyIndicators(c *gin.Context) {

	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return 
	}

	user := userInterface.(*models.User)

	s3Cfg := config.GetDefaultS3Config()

	filename := "opt360Store/" + user.RegionalOffice + "/anomalies_insights.json"


	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket : aws.String(s3Cfg.BucketName),
		Key    : aws.String(filename),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":             "Anamolies insights file not found",
			"regional_office":    user.RegionalOffice,
			"file_path":          filename,
			"details":            err.Error(),
		})
		return 
	}
	defer result.Body.Close()

	
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read file content",
			"details": err.Error(),
		})
		return
	}

	// converting the file to double quotes for valid JSON
	bodyStr := strings.ReplaceAll(string(body), "'", "\"")

	var jsonData interface{}
	if err := json.Unmarshal([]byte(bodyStr), &jsonData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Invalid JSON in file",
			"details":    err.Error(),
			"file_path":  filename,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file":            filename,
		"requested_by":    user.ADID,
		"regional_office": user.RegionalOffice,
		"data":            jsonData,
	})

}
