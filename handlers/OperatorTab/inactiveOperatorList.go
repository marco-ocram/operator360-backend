package OperatorTab

import (
	"fmt"
	"log"
	"net/http"

	"opt360-portal-backend/db"
	"opt360-portal-backend/models"

	"github.com/gin-gonic/gin"
)

func GetInactiveOperatorList(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	user := userInterface.(*models.User)

	// Parse pagination parameters
	page := 1
	pageSize := 20
	
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := fmt.Sscanf(pageParam, "%d", &page); err == nil && p == 1 && page > 0 {
			// Valid page number
		} else {
			page = 1
		}
	}
	
	if pageSizeParam := c.Query("page_size"); pageSizeParam != "" {
		if ps, err := fmt.Sscanf(pageSizeParam, "%d", &pageSize); err == nil && ps == 1 && pageSize > 0 && pageSize <= 1000 {
			// Valid page size
		} else {
			pageSize = 20
		}
	}

	operators, err := db.GetInactiveOperators(user.RegionalOffice)
	if err != nil {
		log.Printf("[GetInactiveOperatorList] DB error ro=%s: %v", user.RegionalOffice, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":           "Failed to retrieve inactive operators",
			"regional_office": user.RegionalOffice,
			"details":         err.Error(),
		})
		return
	}

	totalCount := len(operators)
	totalPages := (totalCount + pageSize - 1) / pageSize
	
	// Adjust page if it exceeds total pages
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	
	// Calculate pagination indices
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	
	if startIndex >= totalCount {
		startIndex = 0
		endIndex = 0
	} else if endIndex > totalCount {
		endIndex = totalCount
	}
	
	// Paginate the data
	paginatedData := []db.ActiveOperator{}
	if startIndex < endIndex {
		paginatedData = operators[startIndex:endIndex]
	}

	log.Printf("[GetInactiveOperatorList] Returning %d/%d inactive operators for ro=%s (page %d)",
		len(paginatedData), totalCount, user.RegionalOffice, page)
	c.JSON(http.StatusOK, gin.H{
		"regional_office": user.RegionalOffice,
		"total_count":     totalCount,
		"pagination": gin.H{
			"page":          page,
			"page_size":     pageSize,
			"total_records": totalCount,
			"total_pages":   totalPages,
			"has_next":      page < totalPages,
			"has_previous":  page > 1,
		},
		"count": len(paginatedData),
		"data":  paginatedData,
	})
}
