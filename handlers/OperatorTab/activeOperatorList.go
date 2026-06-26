package OperatorTab

import (
	"log"
	"net/http"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

func GetActiveOperatorList(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)

	operators, err := db.GetActiveOperators(user.RegionalOffice)
	if err != nil {
		log.Printf("[GetActiveOperatorList] DB error ro=%s: %v", user.RegionalOffice, err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve active operators", err, gin.H{
			"regional_office": user.RegionalOffice,
		})
		return
	}

	result := pageparam.Slice(operators, page, pageSize)

	log.Printf("[GetActiveOperatorList] Returning %d/%d active operators for ro=%s (page %d)",
		len(result.Items), result.Total, user.RegionalOffice, result.Page)
	respond.OK(c, gin.H{
		"regional_office": user.RegionalOffice,
		"total_count":     result.Total,
		"pagination":      result.JSON(),
		"count":           len(result.Items),
		"data":            result.Items,
	})
}
