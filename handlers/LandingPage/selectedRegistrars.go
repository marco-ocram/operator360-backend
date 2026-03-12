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

// Request structure for selected Registrars
type SelectedRegistrarsRequest struct {
	SelectedRegistrars []string `json:"selected_registrars" binding:"required"`
}

// Registrar distribution structure
type RegistrarDistribution struct {
	HighRisk int `json:"high_risk"`
	MedRisk  int `json:"med_risk"`
	LowRisk  int `json:"low_risk"`
	NoRisk   int `json:"no_risk"`
}

// Audit data structure for registrars
type AuditDataRegistrar struct {
	RegDistribution map[string]RegistrarDistribution `json:"reg_distribution"`
}

func GetSelectedRegistrars(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Parse request body
	var req SelectedRegistrarsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate that selected_registrars is not empty
	if len(req.SelectedRegistrars) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "selected_registrars array cannot be empty",
		})
		return
	}

	// Get S3 configuration
	s3Cfg := config.GetDefaultS3Config()

	// Build file path based on user's regional office
	// Format: opt360Store/{RegionalOffice}/audit1.json
	fileName := "opt360Store/" + user.RegionalOffice + "/audit1.json"

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
	var auditData AuditDataRegistrar
	err = json.Unmarshal(body, &auditData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse audit data",
			"details": err.Error(),
		})
		return
	}

	// Filter reg_distribution based on selected_registrars
	filteredDistribution := make(map[string]RegistrarDistribution)
	for _, registrarName := range req.SelectedRegistrars {
		if distribution, exists := auditData.RegDistribution[registrarName]; exists {
			filteredDistribution[registrarName] = distribution
		}
	}

	// Return filtered data
	c.JSON(http.StatusOK, gin.H{
		"reg_distribution": filteredDistribution,
	})
}

func GetTop10Registrars(c *gin.Context) {
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
	// Format: opt360Store/{RegionalOffice}/audit1.json
	fileName := "opt360Store/" + user.RegionalOffice + "/audit1.json"

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
	var auditData AuditDataRegistrar
	err = json.Unmarshal(body, &auditData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse audit data",
			"details": err.Error(),
		})
		return
	}

	// Calculate total risk count for each Registrar
	type registrarWithTotal struct {
		name         string
		distribution RegistrarDistribution
		totalRisk    int
	}

	var registrarList []registrarWithTotal
	for registrarName, distribution := range auditData.RegDistribution {
		totalRisk := distribution.LowRisk + distribution.MedRisk + distribution.HighRisk
		registrarList = append(registrarList, registrarWithTotal{
			name:         registrarName,
			distribution: distribution,
			totalRisk:    totalRisk,
		})
	}

	// Sort by total risk in descending order
	sort.Slice(registrarList, func(i, j int) bool {
		return registrarList[i].totalRisk > registrarList[j].totalRisk
	})

	// Get top 10 Registrars (or less if there are fewer than 10)
	top10Count := 10
	if len(registrarList) < 10 {
		top10Count = len(registrarList)
	}

	// Build the response with reg_distribution structure
	top10Distribution := make(map[string]RegistrarDistribution)
	for i := 0; i < top10Count; i++ {
		top10Distribution[registrarList[i].name] = registrarList[i].distribution
	}

	// Return top 10 Registrars in reg_distribution format
	c.JSON(http.StatusOK, gin.H{
		"reg_distribution": top10Distribution,
	})
}

func GetAllRegistrars(c *gin.Context) {
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
	// Format: opt360Store/{RegionalOffice}/audit1.json
	fileName := "opt360Store/" + user.RegionalOffice + "/audit1.json"

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
	var auditData AuditDataRegistrar
	err = json.Unmarshal(body, &auditData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse audit data",
			"details": err.Error(),
		})
		return
	}

	// Extract all Registrar names from the distribution map
	registrarNames := make([]string, 0, len(auditData.RegDistribution))
	for registrarName := range auditData.RegDistribution {
		registrarNames = append(registrarNames, registrarName)
	}

	// Sort Registrar names alphabetically for consistent ordering in dropdown
	sort.Strings(registrarNames)

	// Return list of all Registrar names
	c.JSON(http.StatusOK, gin.H{
		"registrars": registrarNames,
		"count":      len(registrarNames),
	})
}
