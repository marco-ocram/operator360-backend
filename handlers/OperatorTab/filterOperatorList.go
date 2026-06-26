package OperatorTab

import (
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/authctx"
	"opt360-portal-backend/db"
	"opt360-portal-backend/models"
	"opt360-portal-backend/pageparam"
	"opt360-portal-backend/respond"

	"github.com/gin-gonic/gin"
)

// GetFilteredOperatorList handles GET /api/filter_operator_list
//
// Optional query params:
//
//	state        – filter by state
//	district     – filter by district
//	id           – filter by id
//	ea           – filter by ea
//	user_status  – filter by user status (active/inactive)
//	page         – page number (default 1)
//	page_size    – records per page (default 20, max 1000)
//
// See db.GetFilteredOperators for how the cross-cluster user_status filter
// is resolved (opt_master and uidmasterv1_1.user live on separate DB servers).
func GetFilteredOperatorList(c *gin.Context) {
	user, ok := authctx.RequireUser(c)
	if !ok {
		return
	}

	filters := models.FilteredOperatorAppliedFilters{
		RO:         user.RegionalOffice,
		State:      strings.TrimSpace(c.Query("state")),
		District:   strings.TrimSpace(c.Query("district")),
		ID:         strings.TrimSpace(c.Query("id")),
		EA:         strings.TrimSpace(c.Query("ea")),
		Reg:        strings.TrimSpace(c.Query("reg")),
		RiskBucket: strings.TrimSpace(c.Query("risk_bucket")),
		UserStatus: strings.TrimSpace(c.Query("user_status")),
	}

	page, pageSize := pageparam.Parse(c, 20, 1000)
	offset := (page - 1) * pageSize

	operators, total, err := db.GetFilteredOperators(db.OperatorFilterParams{
		RegionalOffice: user.RegionalOffice,
		IsAdmin:        strings.EqualFold(user.Role, "admin"),
		State:          filters.State,
		District:       filters.District,
		ID:             filters.ID,
		EA:             filters.EA,
		Reg:            filters.Reg,
		RiskBucket:     filters.RiskBucket,
		UserStatus:     filters.UserStatus,
		PageSize:       pageSize,
		Offset:         offset,
	})
	if err != nil {
		log.Printf("[GetFilteredOperatorList] %v", err)
		respond.Error(c, http.StatusInternalServerError, "Failed to retrieve operator records", err, nil)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = total / pageSize
		if total%pageSize != 0 {
			totalPages++
		}
	}

	log.Printf(
		"[GetFilteredOperatorList] ro=%s state=%q district=%q id=%q ea=%q reg=%q risk_bucket=%q user_status=%q → %d/%d records (page %d/%d)",
		user.RegionalOffice, filters.State, filters.District, filters.ID, filters.EA, filters.Reg, filters.RiskBucket, filters.UserStatus,
		len(operators), total, page, totalPages,
	)

	c.JSON(http.StatusOK, models.FilteredOperatorResponse{
		Data:           operators,
		Total:          total,
		Page:           page,
		PageSize:       pageSize,
		TotalPages:     totalPages,
		AppliedFilters: filters,
	})
}
