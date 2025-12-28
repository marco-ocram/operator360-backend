package Feedback

import (

	"encoding/json"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"time"
	"bytes"
	"strings"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	

)


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