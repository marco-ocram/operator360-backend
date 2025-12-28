package AnamolyIndicators


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