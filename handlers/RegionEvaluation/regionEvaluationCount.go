package RegionEvaluation

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"opt360-portal-backend/config"
	"opt360-portal-backend/models"
	"opt360-portal-backend/utils"
	"opt360-portal-backend/cache"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
)

var regionCache = cache.NewFileCache(cache.CacheConfig{})


func GetRegionEvaluationCount(c *gin.Context) {

	// ------------------------------------------------------------------
	// Get authenticated user
	// ------------------------------------------------------------------
	// userInterface, exists := c.Get("user")
	// if !exists {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
	// 	return
	// }

	// user := userInterface.(*models.User)

	user := &models.User{
		ADID:           "TESTUSER001",
		RegionalOffice: "Lucknow",
	}

	c.Set("user", user)
    

	// ------------------------------------------------------------------
	// Request parameters
	// ------------------------------------------------------------------
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

	log.Printf(
		"[GetRegionEvaluationCount] Request received ro=%s state=%s district=%s user=%s",
		regionalOffice,
		optState,
		optDistrict,
		user.ADID,
	)

	// ------------------------------------------------------------------
	// Build S3 filename (used for fallback and response)
	// ------------------------------------------------------------------
	roForPath := utils.ToPascalCase(regionalOffice)
	s3Cfg := config.GetDefaultS3Config()

	var fileName string

	if optState != "" && optDistrict != "" {

		fileName = "opt360Store/" +
			roForPath + "/" +
			utils.ToPascalCase(optState) + "/" +
			utils.ToPascalCase(optDistrict) +
			"/audit.json"

	} else if optState != "" {

		fileName = "opt360Store/" +
			roForPath + "/" +
			utils.ToPascalCase(optState) +
			"/audit.json"

	} else {

		fileName = "opt360Store/" +
			roForPath +
			"/audit.json"
	}

	// ------------------------------------------------------------------
	// ClickHouse (RO + State only)
	// ------------------------------------------------------------------

	cacheKey := cache.GenerateKey(regionalOffice, optState, optDistrict)

	var cachedData interface{}
    if found, err := regionCache.Get("region_evaluation", cacheKey, 24*time.Hour, &cachedData); err != nil {
        log.Printf("[GetRegionEvaluationCount] Cache read failed: %v", err)
    } else if found {
        log.Printf("[GetRegionEvaluationCount] Cache HIT ro=%s state=%s district=%s", regionalOffice, optState, optDistrict)
        
        c.JSON(http.StatusOK, gin.H{
            "regional_office": regionalOffice,
            "opt_state":       optState,
            "opt_district":    optDistrict,
            "file":            "Clickhouse (cached)",
            "data":            cachedData,
            "requested_by":    user.ADID,
        })
        return
    }

	log.Printf(
		"[GetRegionEvaluationCount] Trying ClickHouse first (ro=%s state=%s district=%s)",
		regionalOffice,
		optState,
		optDistrict,
	)

	

	data, found, err := GetRegionEvaluationFromClickHouse(
		strings.ToUpper(regionalOffice),
		optState,
		optDistrict,
	)

	if err != nil {

		log.Printf(
			"[GetRegionEvaluationCount] ClickHouse lookup failed: %v. Falling back to S3.",
			err,
		)

	} else if found {

		if cacheErr := regionCache.Set("region_evaluation", cacheKey, data); cacheErr != nil {
            log.Printf("[GetRegionEvaluationCount] Cache write failed: %v", cacheErr)
        }

        c.JSON(http.StatusOK, gin.H{
            "regional_office": regionalOffice,
            "opt_state":       optState,
            "opt_district":    optDistrict,
            "file":            "Clickhouse",
            "data":            data,
            "requested_by":    user.ADID,
        })
        return

		

	} else {

		log.Printf(
			"[GetRegionEvaluationCount] District request detected. Skipping ClickHouse and using S3.",
		)
	}

	// ------------------------------------------------------------------
	// Existing S3 fallback
	// ------------------------------------------------------------------

	s3Client, err := config.NewS3Client(s3Cfg)
	if err != nil {
		log.Printf(
			"[GetRegionEvaluationCount] S3 client error user=%s: %v",
			user.ADID,
			err,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create S3 client",
			"details": err.Error(),
		})

		return
	}

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s3Cfg.BucketName),
		Key:    aws.String(fileName),
	})

	if err != nil {

		log.Printf(
			"[GetRegionEvaluationCount] S3 fetch failed key=%s user=%s: %v",
			fileName,
			user.ADID,
			err,
		)

		c.JSON(http.StatusNotFound, gin.H{
			"error":           "Audit file not found",
			"regional_office": regionalOffice,
			"file_path":       fileName,
			"details":         err.Error(),
		})

		return
	}

	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {

		log.Printf(
			"[GetRegionEvaluationCount] Read body failed key=%s: %v",
			fileName,
			err,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read S3 data",
			"details": err.Error(),
		})

		return
	}

	var jsonData interface{}

	if err := json.Unmarshal(body, &jsonData); err != nil {

		log.Printf(
			"[GetRegionEvaluationCount] JSON parse failed key=%s: %v",
			fileName,
			err,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse JSON data",
			"details": err.Error(),
		})

		return
	}

	log.Printf(
		"[GetRegionEvaluationCount] Returning S3 response key=%s user=%s",
		fileName,
		user.ADID,
	)

	c.JSON(http.StatusOK, gin.H{
		"regional_office": regionalOffice,
		"opt_state":       optState,
		"opt_district":    optDistrict,
		"file":            fileName,
		"data":            jsonData,
		"requested_by":    user.ADID,
	})
}
