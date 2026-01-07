package OperatorDetailView

import (
	"encoding/json"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
)

// toCamelCase converts a string with spaces to PascalCase
func toCamelCaseFeatures(s string) string {
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

func GetOperatorFeatures(c *gin.Context) {

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

	optStateForPath := toCamelCaseFeatures(optState)
	optDistrictForPath := toCamelCaseFeatures(optDistrict)
	optIDForPath := optID 

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/opt_details.json
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/operator_features.json"

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
	// log.Println("Operator Features JSON Data:", string(body))

	// Parse the original JSON structure
	var originalData map[string]interface{}
	if err := json.Unmarshal(body, &originalData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse JSON data",
			"details": err.Error(),
		})
		return
	}

	// Transform the data structure
	transformedData := make(map[string]interface{})
	transformedData["opt_id"] = originalData["opt_id"]
	
	// Copy metadata if it exists
	if metadata, ok := originalData["metadata"]; ok {
		transformedData["metadata"] = metadata
	}

	// Transform kpis from nested object to flat array
	var kpisArray []map[string]interface{}
	if kpis, ok := originalData["kpis"].(map[string]interface{}); ok {
		for categoryName, categoryFeatures := range kpis {
			if featuresArray, ok := categoryFeatures.([]interface{}); ok && len(featuresArray) > 0 {
				categoryItem := map[string]interface{}{
					categoryName: featuresArray,
				}
				kpisArray = append(kpisArray, categoryItem)
			}
		}
	}
	transformedData["kpis"] = kpisArray
	// log.Println("Transformed Operator Features Data:", transformedData)

	// Return transformed data
	c.JSON(http.StatusOK, transformedData)
}