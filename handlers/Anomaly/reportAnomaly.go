package Anomaly

import (
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/db"
	"opt360-portal-backend/models"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func ReportAnomaly(c *gin.Context) {
	var reqData models.AnomalyReportRequest
	if err := c.ShouldBindJSON(&reqData); err != nil {
		log.Printf("Invalid JSON in request body: %v", err)
		respond.Error(c, http.StatusBadRequest, "Invalid JSON in request body", err, nil)
		return
	}

	if reqData.EID == "" {
		respond.Error(c, http.StatusBadRequest, "eid is required", nil, nil)
		return
	}
	if reqData.OptID == "" {
		respond.Error(c, http.StatusBadRequest, "opt_id is required", nil, nil)
		return
	}

	markAnomaly := &models.MarkAnomaly{
		EID:                reqData.EID,
		AnomalyCategory:    reqData.AnomalyCategory,
		DateCreated:        reqData.DateCreated,
		EnrolmentType:      reqData.EnrolmentType,
		OptDistrict:        reqData.OptDistrict,
		OptState:           reqData.OptState,
		OptID:              reqData.OptID,
		PktSource:          reqData.PktSource,
		StationMachineCode: reqData.StationMachineCode,
		StationNo:          reqData.StationNo,
	}

	if len(reqData.AnomalyType) > 0 {
		markAnomaly.AnomalyCode = reqData.AnomalyType[0].AnomalyCode
		markAnomaly.AnomalyName = reqData.AnomalyType[0].AnomalyName
		markAnomaly.ErrorCategory = reqData.AnomalyType[0].Reason.ErrorCategory
	}
	if len(reqData.PktUpdtType) > 0 {
		markAnomaly.PktUpdtType = strings.Join(reqData.PktUpdtType, ", ")
	}
	if len(reqData.Remarks) > 0 {
		markAnomaly.Remarks = strings.Join(reqData.Remarks, ", ")
	}

	if err := db.InsertMarkAnomaly(markAnomaly); err != nil {
		log.Printf("Failed to insert anomaly: %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to insert anomaly record", err, nil)
		return
	}

	respond.OK(c, gin.H{
		"message": "Anomaly reported successfully",
		"eid":     reqData.EID,
	})
}
