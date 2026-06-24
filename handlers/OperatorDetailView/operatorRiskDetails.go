package OperatorDetailView

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
func toCamelCaseRisk(s string) string {
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

	
	optStateForPath := toCamelCaseRisk(optState)
	optDistrictForPath := toCamelCaseRisk(optDistrict)
	optIDForPath := optID 

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	
	// Format: opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/opt_details.json
	fileName := "opt360Store/" + user.RegionalOffice + "/" + optStateForPath + "/" + optDistrictForPath + "/" + optIDForPath + "/risk_details.json"

	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		log.Printf("[GetOperatorRiskDetails] S3 client error opt_id=%s user=%s: %v", optID, user.ADID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create S3 client", "details": err.Error()})
		return
	}

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		log.Printf("[GetOperatorRiskDetails] S3 fetch failed key=%s user=%s: %v", fileName, user.ADID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "Operator risk details file not found",
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

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("[GetOperatorRiskDetails] Read body failed key=%s: %v", fileName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read S3 data", "details": err.Error()})
		return
	}

	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		log.Printf("[GetOperatorRiskDetails] JSON parse failed key=%s: %v", fileName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JSON data", "details": err.Error()})
		return
	}

	log.Printf("[GetOperatorRiskDetails] Serving key=%s user=%s", fileName, user.ADID)
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
