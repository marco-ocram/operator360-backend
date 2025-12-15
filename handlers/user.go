package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"strings"
	"os"

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

// GetOperatorDetails searches operator.parquet file with filters
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

	// Get optional query parameters for filtering (same as GetOperatorList)
	optEa := c.Query("opt_ea")             // Filter by EA
	optReg := c.Query("opt_reg")           // Filter by region
	optDistrict := c.Query("opt_district") // Filter by district
	optState := c.Query("opt_state")       // Filter by state
	optID := c.Query("opt_id")             // Search by operator ID (required)

	// Validate that at least opt_id is provided
	if optID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_id query parameter is required",
		})
		return
	}

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
			"error":           "Operator file not found",
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
		if err != nil || len(rowMap) == 0 {
			continue
		}
		json.Unmarshal(jsonBytes, &rowMap)
		
		match := true

		// Search by opt_id (case-insensitive partial match) - REQUIRED
		if optID != "" {
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

		// Filter by opt_state (exact match, case-insensitive)
		if optState != "" && match {
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

		// Only keep matching rows in memory
		if match {
			filteredData = append(filteredData, rowMap)
		}
		// Non-matching rows are immediately discarded, not stored
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
			"opt_id":       optID,
			"opt_ea":       optEa,
			"opt_reg":      optReg,
			"opt_district": optDistrict,
			"opt_state":    optState,
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

// GetOperatorPackets fetches all sid_*.parquet files for an operator
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

	// Validate required parameters
	if optState == "" || optDistrict == "" || optID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "opt_state, opt_district, and opt_id query parameters are required",
		})
		return
	}

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
	folderPath := "opt360Store/" + user.RegionalOffice + "/" + optState + "/" + optDistrict + "/" + optID + "/"

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

	// Filter for sid_*.parquet files
	var parquetFiles []string
	for _, obj := range listResult.Contents {
		fileName := *obj.Key
		// Extract just the filename from the full path
		parts := strings.Split(fileName, "/")
		baseName := parts[len(parts)-1]
		
		// Check if it matches sid_*.parquet pattern
		if strings.HasPrefix(baseName, "sid_") && strings.HasSuffix(baseName, ".parquet") {
			parquetFiles = append(parquetFiles, fileName)
		}
	}

	// If no parquet files found
	if len(parquetFiles) == 0 {
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

	// Helper function to read a single parquet file from S3
	readParquetFile := func(fileName string) ([]map[string]interface{}, error) {
		// Download the parquet file from S3
		result, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(s3Cfg.BucketName),
			Key:    aws.String(fileName),
		})
		if err != nil {
			return nil, err
		}
		defer result.Body.Close()

		// Stream parquet data from S3
		parquetBytes, err := io.ReadAll(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read S3 data: %w", err)
		}

		// Write to temp file
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

		// Read all data
		strs, err := pr.ReadByNumber(numRows)
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet data: %w", err)
		}

		// Convert to map format
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

	// Read all parquet files and combine data
	combinedData := make([]map[string]interface{}, 0)
	filesProcessed := make([]gin.H, 0)
	totalRecords := 0

	for _, filePath := range parquetFiles {
		data, err := readParquetFile(filePath)
		
		// Extract filename and packet type from path
		parts := strings.Split(filePath, "/")
		fileName := parts[len(parts)-1]
		
		// Extract packet type from filename (U or N at the end before .parquet)
		packetType := "unknown"
		if strings.Contains(fileName, "_U.parquet") {
			packetType = "update"
		} else if strings.Contains(fileName, "_N.parquet") {
			packetType = "new_enrollment"
		}

		fileInfo := gin.H{
			"file_name":    fileName,
			"file_path":    filePath,
			"packet_type":  packetType,
			"record_count": 0,
			"status":       "failed",
		}

		if err != nil {
			fileInfo["error"] = err.Error()
		} else {
			fileInfo["record_count"] = len(data)
			fileInfo["status"] = "success"
			totalRecords += len(data)

			// Add metadata to each row
			for _, row := range data {
				row["source_file"] = fileName
				row["packet_type"] = packetType
				combinedData = append(combinedData, row)
			}
		}

		filesProcessed = append(filesProcessed, fileInfo)
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
		"regional_office":   user.RegionalOffice,
		"operator_id":       optID,
		"state":             optState,
		"district":          optDistrict,
		"folder_path":       folderPath,
		"files_found":       len(parquetFiles),
		"files_processed":   filesProcessed,
		"total_records":     totalRecords,
		"pagination": gin.H{
			"page":          page,
			"page_size":     pageSize,
			"total_records": totalRecords,
			"total_pages":   totalPages,
			"has_next":      page < totalPages,
			"has_previous":  page > 1,
		},
		"count":             len(paginatedData),
		"data":              paginatedData,
		"requested_by":      user.ADID,
	})
}
