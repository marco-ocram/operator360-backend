package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	
	"strconv"
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

	var kpiData models.KPIResponse
	if err := json.Unmarshal(body, &kpiData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Invalid JSON in file",
			"details":  err.Error(),
		})
		return
	}

	// Return the strongly-typed KPI data
	c.JSON(http.StatusOK, kpiData)
}

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

// fetch opt specific details 

func GetOperatorDetails(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

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

	
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/opt_details.json
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/opt_details.json"

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

	// Read JSON data from S3
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON data
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse JSON data",
			"details": err.Error(),
		})
		return
	}

	// Return all data without pagination
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"state":           optState,
		"district":        optDistrict,
		"file":            fileName,
		"data":            jsonData,
		"requested_by":    user.ADID,
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
	
	// Get optional filter parameters
	searchSID := c.Query("sid")            // e.g., "123456789012" (optional)
	anomalyFilter := c.Query("anomaly_filter") // "anomalous" or "non-anomalous" (optional)
	enrollmentTypeFilter := c.Query("enrollment_type") // "new_enrollment" or "update" (optional)
	dateFilter := c.Query("date")          // e.g., "2025-12-25" or "2025_12_25" (optional)

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

	// Create S3 client
	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})
		return
	}

	// Build file path: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/sid.parquet
	filePath := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/sid.parquet"

	// Download parquet file from S3
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(filePath),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "Failed to fetch sid.parquet file",
			"regional_office": user.RegionalOffice,
			"operator_id":     optID,
			"state":           optState,
			"district":        optDistrict,
			"file_path":       filePath,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read parquet data from S3 response
	parquetBytes, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read parquet data from S3",
			"details": err.Error(),
		})
		return
	}

	// Create temporary file to store parquet data
	tempFile, err := os.CreateTemp("", "sid_*.parquet")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temporary file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tempFile.Name())

	// Write parquet data to temporary file
	_, err = tempFile.Write(parquetBytes)
	if err != nil {
		tempFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write parquet data to temporary file",
			"details": err.Error(),
		})
		return
	}
	tempFile.Close()

	// Open parquet file reader
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

	// Read all rows from parquet file
	numRows := int(pr.GetNumRows())
	strs, err := pr.ReadByNumber(numRows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read parquet data",
			"details": err.Error(),
		})
		return
	}

	// Convert parquet data to JSON format
	allData := make([]map[string]interface{}, 0)
	for _, str := range strs {
		// Marshal to JSON and unmarshal back to get map
		jsonBytes, err := json.Marshal(str)
		if err != nil {
			continue
		}

		var rowMap map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &rowMap); err != nil {
			continue
		}

		// Skip empty rows
		if len(rowMap) == 0 {
			continue
		}

		// Convert date fields to normal format (YYYY-MM-DD HH:MM:SS)
		for key, val := range rowMap {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "date") || strings.Contains(lowerKey, "time") {
				var timestamp float64
				
				// Handle different numeric types
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
					// Try parsing string as number
					valStr := strings.TrimSpace(v)
					// Check if it's a date string format
					if strings.Contains(valStr, "-") || strings.Contains(valStr, "_") {
						// Already in string date format, normalize it
						normalizedDate := strings.ReplaceAll(valStr, "_", "-")
						rowMap[key] = normalizedDate
						continue
					}
					// Try parsing as number
					fmt.Sscanf(valStr, "%f", &timestamp)
				}
				
				// If we have a timestamp value, convert it
				if timestamp > 0 {
					// Detect timestamp precision:
					// Nanoseconds: > 1e15 (typically 19 digits like 1761396321000000000)
					// Microseconds: > 1e12 and < 1e15
					// Milliseconds: > 1e9 and < 1e12
					// Seconds: < 1e9
					
					if timestamp > 1e15 {
						// Nanoseconds - convert to seconds
						timestamp = timestamp / 1e9
					} else if timestamp > 1e12 {
						// Milliseconds - convert to seconds
						timestamp = timestamp / 1000.0
					}
					
					// Validate timestamp is in reasonable range (between 2000 and 2100)
					// Unix timestamp for 2000-01-01: 946684800
					// Unix timestamp for 2100-01-01: 4102444800
					if timestamp >= 946684800 && timestamp <= 4102444800 {
						t := time.Unix(int64(timestamp), 0)
						formattedDate := t.Format("2006-01-02 15:04:05")
						rowMap[key] = formattedDate
					}
				}
			}
		}

		allData = append(allData, rowMap)
	}

	// Apply filters if provided
	filteredData := make([]map[string]interface{}, 0)
	for _, rowMap := range allData {
		// Filter by SID search (check "eid" field)
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

		// Filter by anomaly status
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

		// Filter by enrollment type
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

		// Filter by date
		if dateFilter != "" {
			matchFound := false
			// Normalize date filter (support both - and _ separators)
			normalizedDateFilter := strings.ReplaceAll(dateFilter, "-", "_")
			
			// Check common date fields: date, Date, packet_date, Date_packet, etc.
			for key, val := range rowMap {
				lowerKey := strings.ToLower(key)
				if strings.Contains(lowerKey, "date") {
					valStr := fmt.Sprintf("%v", val)
					// Normalize value (replace - with _)
					normalizedVal := strings.ReplaceAll(valStr, "-", "_")
					if strings.Contains(normalizedVal, normalizedDateFilter) {
						matchFound = true
						break
					}
				}
			}
			if !matchFound {
				continue
			}
		}

		filteredData = append(filteredData, rowMap)
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

	// Return paginated data
	c.JSON(http.StatusOK, gin.H{
		"regional_office":        user.RegionalOffice,
		"operator_id":            optID,
		"state":                  optState,
		"district":               optDistrict,
		"file_path":              filePath,
		"search_sid":             searchSID,
		"anomaly_filter":         anomalyFilter,
		"enrollment_type_filter": enrollmentTypeFilter,
		"date_filter":            dateFilter,
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


func GetOperatorRiskDetails(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

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

	
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/opt_details.json
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/risk_details.json"

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

	// Read JSON data from S3
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON data
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse JSON data",
			"details": err.Error(),
		})
		return
	}

	// Return all data without pagination
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"state":           optState,
		"district":        optDistrict,
		"file":            fileName,
		"data":            jsonData,
		"requested_by":    user.ADID,
	})
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

	// Convert spaces to underscores for S3 path compatibility
	optStateForPath := strings.ReplaceAll(optState, " ", "_")
	optDistrictForPath := strings.ReplaceAll(optDistrict, " ", "_")
	optIDForPath := strings.ReplaceAll(optID, " ", "_")

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