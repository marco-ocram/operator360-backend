package Feedback

import (
	"log"
	"net/http"
	"strings"
	"time"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/config"
	"opt360-portal-backend/db"
	"opt360-portal-backend/respond"
	"opt360-portal-backend/s3store"

	"github.com/gin-gonic/gin"
)

func SubmitFeedback(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	var feedbackData map[string]interface{}
	if err := c.ShouldBindJSON(&feedbackData); err != nil {
		respond.Error(c, http.StatusBadRequest, "Invalid JSON in request body", err, nil)
		return
	}

	optID, optIDExists := feedbackData["opt_id"].(string)
	if !optIDExists || optID == "" {
		respond.Error(c, http.StatusBadRequest, "opt_id is required in request body", nil, nil)
		return
	}

	dataPath, err := db.GetDataPathByOptID(optID)
	if err != nil {
		log.Printf("[SubmitFeedback] DataPath lookup failed opt_id=%s user=%s: %v", optID, user.ADID, err)
		respond.Error(c, http.StatusNotFound, "Operator data path not found", err, gin.H{"operator_id": optID})
		return
	}

	s3Cfg := config.GetDefaultS3Config()
	currentDate := time.Now().Format("2006_01_02")
	fileName := strings.TrimSuffix(dataPath, "/") + "/" + currentDate + "_" + user.ADID + ".json"

	log.Printf("[SubmitFeedback] Uploading feedback key=%s user=%s", fileName, user.ADID)
	if err := s3store.PutJSON(s3Cfg, fileName, feedbackData); err != nil {
		log.Printf("[SubmitFeedback] S3 upload failed key=%s user=%s: %v", fileName, user.ADID, err)
		respond.Error(c, http.StatusInternalServerError, "Failed to upload feedback file to S3", err, nil)
		return
	}

	log.Printf("[SubmitFeedback] Feedback stored key=%s user=%s", fileName, user.ADID)
	respond.OK(c, gin.H{
		"message":         "Feedback submitted successfully",
		"regional_office": user.RegionalOffice,
		"operator_id":     optID,
		"file_path":       fileName,
		"date":            currentDate,
		"submitted_by":    user.ADID,
	})
}
