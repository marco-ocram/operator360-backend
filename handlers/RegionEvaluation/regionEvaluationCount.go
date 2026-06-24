package RegionEvaluation

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/config"
	"opt360-portal-backend/models"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
)

// toCamelCase converts a string with spaces to PascalCase
func toCamelCase(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	
	// If no spaces, return as-is (already formatted)
	if len(words) == 1 {
		return s
	}

	result := ""
	for _, word := range words {
		result += strings.Title(strings.ToLower(word))
	}
	return result
}

func GetRegionEvaluationCount(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Get required query parameters
	regionalOffice := c.Query("regional_office")
	optState := c.Query("opt_state")
	optDistrict := c.Query("opt_district")

	// Validate required parameter
	if regionalOffice == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "regional_office query parameter is required",
		})
		return
	}

	// Convert to PascalCase for S3 path
	regionalOfficeForPath := toCamelCase(regionalOffice)

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on provided parameters
	var fileName string
	if optState != "" && optDistrict != "" {
		// Format: opt360Store/{RegionalOffice}/{State}/{District}/audit.json
		optStateForPath := toCamelCase(optState)
		optDistrictForPath := toCamelCase(optDistrict)
		fileName = "opt360Store/" + regionalOfficeForPath + "/" + optStateForPath + "/" + optDistrictForPath + "/audit.json"
	} else if optState != "" {
		// Format: opt360Store/{RegionalOffice}/{State}/audit.json
		optStateForPath := toCamelCase(optState)
		fileName = "opt360Store/" + regionalOfficeForPath + "/" + optStateForPath + "/audit.json"
	} else {
		// Format: opt360Store/{RegionalOffice}/audit.json
		fileName = "opt360Store/" + regionalOfficeForPath + "/audit.json"
	}

	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		log.Printf("[GetRegionEvaluationCount] S3 client error user=%s: %v", user.ADID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create S3 client", "details": err.Error()})
		return
	}

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		log.Printf("[GetRegionEvaluationCount] S3 fetch failed key=%s user=%s: %v", fileName, user.ADID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Audit file not found", "regional_office": regionalOffice,
			"file_path": fileName, "details": err.Error(),
		})
		return
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("[GetRegionEvaluationCount] Read body failed key=%s: %v", fileName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read S3 data", "details": err.Error()})
		return
	}

	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		log.Printf("[GetRegionEvaluationCount] JSON parse failed key=%s: %v", fileName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JSON data", "details": err.Error()})
		return
	}

	log.Printf("[GetRegionEvaluationCount] Serving key=%s user=%s", fileName, user.ADID)
	c.JSON(http.StatusOK, gin.H{
		"regional_office": regionalOffice,
		"opt_state":       optState,
		"opt_district":    optDistrict,
		"file":            fileName,
		"data":            jsonData,
		"requested_by":    user.ADID,
	})
}
