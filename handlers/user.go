package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"os"
	"path/filepath"
	"strings"

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

	// Create a temporary file to store the parquet data
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "temp_operator_high.parquet")

	outFile, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tempFile) // Clean up temp file
	defer outFile.Close()

	// Write S3 content to temp file
	_, err = io.Copy(outFile, result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write parquet data",
			"details": err.Error(),
		})
		return
	}
	outFile.Close() // Close before reading

	// Read the parquet file
	fr, err := local.NewLocalFileReader(tempFile)
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
	data := make([]interface{}, len(strs))
	for i, str := range strs {
		var row map[string]interface{}
		jsonStr := fmt.Sprintf("%v", str)
		// Try to parse as JSON
		if err := json.Unmarshal([]byte(jsonStr), &row); err == nil {
			data[i] = row
		} else {
			// If not valid JSON, return the raw interface
			data[i] = str
		}
	}

	// Return the data as JSON
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"count":           len(data),
		"data":            data,
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

	// Create a temporary file to store the parquet data
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "temp_operator_medium.parquet")

	outFile, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tempFile)
	defer outFile.Close()

	// Write S3 content to temp file
	_, err = io.Copy(outFile, result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write parquet data",
			"details": err.Error(),
		})
		return
	}
	outFile.Close()

	// Read the parquet file
	fr, err := local.NewLocalFileReader(tempFile)
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
	data := make([]interface{}, len(strs))
	for i, str := range strs {
		var row map[string]interface{}
		jsonStr := fmt.Sprintf("%v", str)
		if err := json.Unmarshal([]byte(jsonStr), &row); err == nil {
			data[i] = row
		} else {
			data[i] = str
		}
	}

	// Return the data as JSON
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"count":           len(data),
		"data":            data,
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

	// Create a temporary file to store the parquet data
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "temp_low_operators.parquet")

	outFile, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tempFile)
	defer outFile.Close()

	// Write S3 content to temp file
	_, err = io.Copy(outFile, result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write parquet data",
			"details": err.Error(),
		})
		return
	}
	outFile.Close()

	// Read the parquet file
	fr, err := local.NewLocalFileReader(tempFile)
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
	data := make([]interface{}, len(strs))
	for i, str := range strs {
		var row map[string]interface{}
		jsonStr := fmt.Sprintf("%v", str)
		if err := json.Unmarshal([]byte(jsonStr), &row); err == nil {
			data[i] = row
		} else {
			data[i] = str
		}
	}

	// Return the data as JSON
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"count":           len(data),
		"data":            data,
	})
}

// GetOperatorDetails fetches opt_details.parquet file for a specific operator
func GetOperatorDetails(c *gin.Context) {
	// Parse query parameters
	operatorID := c.Query("operator_id")
	state := c.Query("state")
	district := c.Query("district")
	city := c.Query("city")

	// Validate required parameters
	if operatorID == "" || state == "" || district == "" || city == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required query parameters: operator_id, state, district, city",
		})
		return
	}

	// Get user from context (optional - for logging/validation)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Validate that the city in the query parameter matches the user's regional office
	if user.RegionalOffice != city {
		c.JSON(http.StatusForbidden, gin.H{
			"error":           "Access denied",
			"message":         "You can only access operator details for your regional office",
			"your_office":     user.RegionalOffice,
			"requested_city":  city,
		})
		return
	}

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path: opt360Store/{City}/{State}/{District}/{OperatorID}/opt_details.parquet
	fileName := fmt.Sprintf("opt360Store/%s/%s/%s/%s/opt_details.parquet",
		city, state, district, operatorID)

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
			"error":       "Operator details file not found",
			"operator_id": operatorID,
			"file_path":   fileName,
			"details":     err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Create a temporary file to store the parquet data
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("temp_opt_details_%s.parquet", operatorID))

	outFile, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tempFile)
	defer outFile.Close()

	// Write S3 content to temp file
	_, err = io.Copy(outFile, result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write parquet data",
			"details": err.Error(),
		})
		return
	}
	outFile.Close()

	// Read the parquet file
	fr, err := local.NewLocalFileReader(tempFile)
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
	data := make([]interface{}, len(strs))
	for i, str := range strs {
		var row map[string]interface{}
		jsonStr := fmt.Sprintf("%v", str)
		if err := json.Unmarshal([]byte(jsonStr), &row); err == nil {
			data[i] = row
		} else {
			data[i] = str
		}
	}

	// Return the data as JSON
	c.JSON(http.StatusOK, gin.H{
		"operator_id":     operatorID,
		"state":           state,
		"district":        district,
		"city":            city,
		"file":            fileName,
		"count":           len(data),
		"data":            data,
		"requested_by":    user.ADID,
		"regional_office": user.RegionalOffice,
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

	// Create a temporary file to store the parquet data
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "temp_operator_list.parquet")

	outFile, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tempFile)
	defer outFile.Close()

	// Write S3 content to temp file
	_, err = io.Copy(outFile, result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write parquet data",
			"details": err.Error(),
		})
		return
	}
	outFile.Close()

	// Read the parquet file
	fr, err := local.NewLocalFileReader(tempFile)
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
	data := make([]map[string]interface{}, 0)
	for _, str := range strs {
		rowMap := make(map[string]interface{})
		jsonBytes, err := json.Marshal(str)
		if err == nil {
			json.Unmarshal(jsonBytes, &rowMap)
		}
		if len(rowMap) > 0 {
			data = append(data, rowMap)
		}
	}

	// Apply filters if query parameters are provided
	filteredData := make([]map[string]interface{}, 0)
	for _, rowMap := range data {
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

	// Return the filtered data as JSON
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"file":            fileName,
		"total_count":     len(data),
		"filtered_count":  len(filteredData),
		"filters": gin.H{
			"opt_ea":       optEa,
			"opt_reg":      optReg,
			"opt_district": optDistrict,
			"opt_state":    optState,
			"opt_id":       optID,
		},
		"data": filteredData,
	})
}
