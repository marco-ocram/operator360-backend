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

func GetInactiveOperatorList(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)

	operators, err := db.GetInactiveOperators(user.RegionalOffice)
	if err != nil {
		log.Printf("[GetInactiveOperatorList] DB error ro=%s: %v", user.RegionalOffice, err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve inactive operators", err, gin.H{
			"regional_office": user.RegionalOffice,
		})
		return
	}

	result := pageparam.Slice(operators, page, pageSize)

	log.Printf("[GetInactiveOperatorList] Returning %d/%d inactive operators for ro=%s (page %d)",
		len(result.Items), result.Total, user.RegionalOffice, result.Page)
	respond.OK(c, gin.H{
		"regional_office": user.RegionalOffice,
		"total_count":     result.Total,
		"pagination":      result.JSON(),
		"count":           len(result.Items),
		"data":            result.Items,
	})
}
