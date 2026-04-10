package LandingPage

import (
	"encoding/json"
	"io"
	"net/http"
	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
)

// Request structure for selected EAs
type SelectedEAsRequest struct {
	SelectedEAs []string `json:"selected_eas" binding:"required"`
}

// EA distribution structure
type EADistribution struct {
	HighRisk int `json:"high_risk"`
	MedRisk  int `json:"med_risk"`
	LowRisk  int `json:"low_risk"`
	NoRisk   int `json:"no_risk"`
}

// Audit data structure
type AuditData struct {
	EADistribution map[string]EADistribution `json:"ea_distribution"`
}

func GetSelectedEAs(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Parse request body
	var req SelectedEAsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate that selected_eas is not empty
	if len(req.SelectedEAs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "selected_eas array cannot be empty",
		})
		return
	}

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/audit.json
	fileName := "opt360Store/" + user.RegionalOffice + "/audit.json"

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
			"error":           "Audit file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read the response body
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON data
	var auditData AuditData
	err = json.Unmarshal(body, &auditData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse audit data",
			"details": err.Error(),
		})
		return
	}

	// Filter ea_distribution based on selected_eas
	filteredDistribution := make(map[string]EADistribution)
	for _, eaName := range req.SelectedEAs {
		if distribution, exists := auditData.EADistribution[eaName]; exists {
			filteredDistribution[eaName] = distribution
		}
	}

	// Return filtered data
	c.JSON(http.StatusOK, gin.H{
		"ea_distribution": filteredDistribution,
	})
}

func GetTop10EAs(c *gin.Context) {
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
	// Format: opt360Store/{RegionalOffice}/audit.json
	fileName := "opt360Store/" + user.RegionalOffice + "/audit.json"

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
			"error":           "Audit file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read the response body
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON data
	var auditData AuditData
	err = json.Unmarshal(body, &auditData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse audit data",
			"details": err.Error(),
		})
		return
	}

	// Calculate total risk count for each EA
	type eaWithTotal struct {
		name         string
		distribution EADistribution
		totalRisk    int
	}

	var eaList []eaWithTotal
	for eaName, distribution := range auditData.EADistribution {
		totalRisk := distribution.LowRisk + distribution.MedRisk + distribution.HighRisk
		eaList = append(eaList, eaWithTotal{
			name:         eaName,
			distribution: distribution,
			totalRisk:    totalRisk,
		})
	}

	// Sort by total risk in descending order
	sort.Slice(eaList, func(i, j int) bool {
		return eaList[i].totalRisk > eaList[j].totalRisk
	})

	// Get top 10 EAs (or less if there are fewer than 10)
	top10Count := 10
	if len(eaList) < 10 {
		top10Count = len(eaList)
	}

	// Build the response with ea_distribution structure
	top10Distribution := make(map[string]EADistribution)
	for i := 0; i < top10Count; i++ {
		top10Distribution[eaList[i].name] = eaList[i].distribution
	}

	// Return top 10 EAs in ea_distribution format
	c.JSON(http.StatusOK, gin.H{
		"ea_distribution": top10Distribution,
	})
}

func GetAllEAs(c *gin.Context) {
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
	// Format: opt360Store/{RegionalOffice}/audit.json
	fileName := "opt360Store/" + user.RegionalOffice + "/audit.json"

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
			"error":           "Audit file not found",
			"regional_office": user.RegionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})
		return
	}
	defer result.Body.Close()

	// Read the response body
	body, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})
		return
	}

	// Parse JSON data
	var auditData AuditData
	err = json.Unmarshal(body, &auditData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse audit data",
			"details": err.Error(),
		})
		return
	}

	// Extract all EA names from the distribution map
	eaNames := make([]string, 0, len(auditData.EADistribution))
	for eaName := range auditData.EADistribution {
		eaNames = append(eaNames, eaName)
	}

	// Sort EA names alphabetically for consistent ordering in dropdown
	sort.Strings(eaNames)

	// Return list of all EA names
	c.JSON(http.StatusOK, gin.H{
		"eas":  eaNames,
		"count": len(eaNames),
	})
}
