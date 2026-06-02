package OperatorTab

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"opt360-portal-backend/db"
	"opt360-portal-backend/models"

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
// Because opt_master (data_platform) and uidmasterv1_1.user (UID cluster)
// are on different database servers, the user_status filter is resolved via an
// application-level join:
//  1. Fetch matching user_codes from UID DB cluster
//  2. Pass them as an IN clause to the opt_master query on data_platform cluster
//  3. Enrich each result page with user_status from UID DB
func GetFilteredOperatorList(c *gin.Context) {

	// ── 1. Auth guard ──────────────────────────────────────────────────────────
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	user := userInterface.(*models.User)

	// ── 2. Read filters ────────────────────────────────────────────────────────
	filterState := strings.TrimSpace(c.Query("state"))
	filterDistrict := strings.TrimSpace(c.Query("district"))
	filterID := strings.TrimSpace(c.Query("id"))
	filterEA := strings.TrimSpace(c.Query("ea"))
	filterReg := strings.TrimSpace(c.Query("reg"))
	filterRiskBucket := strings.TrimSpace(c.Query("risk_bucket"))
	filterUserStatus := strings.TrimSpace(c.Query("user_status"))

	// ── 3. Pagination ──────────────────────────────────────────────────────────
	page := 1
	pageSize := 20

	if p := c.Query("page"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", &page); err != nil || page < 1 {
			page = 1
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if _, err := fmt.Sscanf(ps, "%d", &pageSize); err != nil || pageSize < 1 || pageSize > 1000 {
			pageSize = 20
		}
	}
	offset := (page - 1) * pageSize

	// ── 4. opt360 DB connection ────────────────────────────────────────────────
	opt360DB, err := db.GetDB()
	if err != nil {
		log.Printf("[GetFilteredOperatorList] opt360 DB error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database connection unavailable",
			"details": err.Error(),
		})
		return
	}

	// ── 5. Resolve user_status filter via UID DB (separate cluster) ────────────
	// uidStatusMap: UPPER(user_code) → user_status. Only populated when requested.
	var uidStatusMap map[string]string
	if filterUserStatus != "" {
		uidDB, err := db.GetUIDDB()
		if err != nil {
			log.Printf("[GetFilteredOperatorList] UID DB error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "UID database connection unavailable",
				"details": err.Error(),
			})
			return
		}

		var statusCond string
		switch strings.ToLower(filterUserStatus) {
		case "active":
			statusCond = "user_status = '1'"
		case "inactive":
			statusCond = "user_status != '1'"
		default:
			statusCond = "1=1"
		}

		uidRows, err := uidDB.Query(`SELECT UPPER(user_code), user_status FROM user WHERE user_code IS NOT NULL AND ` + statusCond)
		if err != nil {
			log.Printf("[GetFilteredOperatorList] UID query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to fetch user status data",
				"details": err.Error(),
			})
			return
		}
		defer uidRows.Close()

		uidStatusMap = make(map[string]string)
		for uidRows.Next() {
			var code, status string
			if err := uidRows.Scan(&code, &status); err != nil {
				continue
			}
			uidStatusMap[code] = status
		}
		_ = uidRows.Err()

		// Early exit if no users match the status filter
		if len(uidStatusMap) == 0 {
			appliedFilters := models.FilteredOperatorAppliedFilters{
				RO: user.RegionalOffice, State: filterState, District: filterDistrict,
				ID: filterID, EA: filterEA, Reg: filterReg,
				RiskBucket: filterRiskBucket, UserStatus: filterUserStatus,
			}
			c.JSON(http.StatusOK, models.FilteredOperatorResponse{
				Data: []models.FilteredOperator{}, Total: 0, Page: page,
				PageSize: pageSize, TotalPages: 0, AppliedFilters: appliedFilters,
			})
			return
		}
	}

	// ── 6. Build WHERE for opt_master (base filters only – no UID IN clause) ───
	// When id is provided the ro restriction is skipped (global search).
	// The UID user_status filter is applied in Go memory (step 7) to avoid
	// MySQL Error 1390 (too many placeholders) when the UID result set is large.
	var whereClauses []string
	var args []interface{}
	if filterID == "" {
		whereClauses = append(whereClauses, "ro = ?")
		args = append(args, user.RegionalOffice)
	}
	if filterState != "" {
		whereClauses = append(whereClauses, "state = ?")
		args = append(args, filterState)
	}
	if filterDistrict != "" {
		whereClauses = append(whereClauses, "district = ?")
		args = append(args, filterDistrict)
	}
	if filterID != "" {
		whereClauses = append(whereClauses, "id = ?")
		args = append(args, filterID)
	}
	if filterEA != "" {
		whereClauses = append(whereClauses, "ea = ?")
		args = append(args, filterEA)
	}
	if filterReg != "" {
		whereClauses = append(whereClauses, "reg = ?")
		args = append(args, filterReg)
	}
	if filterRiskBucket != "" {
		whereClauses = append(whereClauses, "risk_bucket = ?")
		args = append(args, filterRiskBucket)
	}

	var whereSQL string
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}
	fromSQL := "FROM operator360.opt_master"
	selectCols := `SELECT id, name, risk_score, risk_bucket, reg, ea, ro, district, state, last_sync_timestamp, data_path`

	// ── 7. Count and determine page IDs ───────────────────────────────────────
	var total int
	var pageIDs []string // populated only in the UID-filtered path

	if len(uidStatusMap) > 0 {
		// user_status filter active: fetch all matching IDs from opt_master,
		// intersect with uidStatusMap in Go memory, then paginate.
		// This avoids a giant IN (...) clause that exceeds MySQL's 65535-placeholder limit.
		idQuery := fmt.Sprintf("SELECT id %s %s ORDER BY id", fromSQL, whereSQL)
		idRows, idErr := opt360DB.Query(idQuery, args...)
		if idErr != nil {
			log.Printf("[GetFilteredOperatorList] ID query error: %v", idErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to fetch operator IDs",
				"details": idErr.Error(),
			})
			return
		}
		defer idRows.Close()

		var filteredIDs []string
		for idRows.Next() {
			var id string
			if err := idRows.Scan(&id); err != nil {
				continue
			}
			if _, ok := uidStatusMap[strings.ToUpper(id)]; ok {
				filteredIDs = append(filteredIDs, id)
			}
		}
		if err := idRows.Err(); err != nil {
			log.Printf("[GetFilteredOperatorList] ID row iteration error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Error reading operator IDs",
				"details": err.Error(),
			})
			return
		}

		total = len(filteredIDs)
		start := offset
		end := offset + pageSize
		if start < total {
			if end > total {
				end = total
			}
			pageIDs = filteredIDs[start:end]
		}
	} else {
		// No user_status filter: standard COUNT query.
		countQuery := fmt.Sprintf("SELECT COUNT(*) %s %s", fromSQL, whereSQL)
		if cntErr := opt360DB.QueryRow(countQuery, args...).Scan(&total); cntErr != nil {
			log.Printf("[GetFilteredOperatorList] Count query error: %v", cntErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to count records",
				"details": cntErr.Error(),
			})
			return
		}
	}

	// ── 8. Fetch paginated data ────────────────────────────────────────────────
	operators := make([]models.FilteredOperator, 0, pageSize)

	var rows *sql.Rows
	if len(uidStatusMap) > 0 {
		// Fetch full records for this page's IDs only (≤ pageSize placeholders, always safe).
		if len(pageIDs) > 0 {
			phs := make([]string, len(pageIDs))
			pageArgs := make([]interface{}, len(pageIDs))
			for i, id := range pageIDs {
				phs[i] = "?"
				pageArgs[i] = id
			}
			pageQuery := fmt.Sprintf(
				"%s %s WHERE id IN (%s) ORDER BY id",
				selectCols, fromSQL, strings.Join(phs, ","),
			)
			r, rErr := opt360DB.Query(pageQuery, pageArgs...)
			if rErr != nil {
				log.Printf("[GetFilteredOperatorList] Page data query error: %v", rErr)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to fetch operator records",
					"details": rErr.Error(),
				})
				return
			}
			defer r.Close()
			rows = r
		}
	} else {
		// Standard paginated query using LIMIT / OFFSET.
		dataQuery := fmt.Sprintf("%s %s %s ORDER BY id LIMIT ? OFFSET ?", selectCols, fromSQL, whereSQL)
		dataArgs := append(args, pageSize, offset)
		r, rErr := opt360DB.Query(dataQuery, dataArgs...)
		if rErr != nil {
			log.Printf("[GetFilteredOperatorList] Data query error: %v", rErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to fetch operator records",
				"details": rErr.Error(),
			})
			return
		}
		defer r.Close()
		rows = r
	}

	// ── 9. Scan rows ───────────────────────────────────────────────────────────
	if rows != nil {
		for rows.Next() {
			var op models.FilteredOperator
			var (
				riskScore         sql.NullFloat64
				riskBucket        sql.NullString
				reg               sql.NullString
				ea                sql.NullString
				ro                sql.NullString
				district          sql.NullString
				state             sql.NullString
				lastSyncTimestamp sql.NullTime
				dataPath          sql.NullString
			)
			if err := rows.Scan(
				&op.ID, &op.Name,
				&riskScore, &riskBucket,
				&reg, &ea, &ro, &district, &state,
				&lastSyncTimestamp, &dataPath,
			); err != nil {
				log.Printf("[GetFilteredOperatorList] Row scan error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to process operator record",
					"details": err.Error(),
				})
				return
			}
			if riskScore.Valid {
				op.RiskScore = &riskScore.Float64
			}
			if riskBucket.Valid {
				op.RiskBucket = &riskBucket.String
			}
			if reg.Valid {
				op.Reg = &reg.String
			}
			if ea.Valid {
				op.EA = &ea.String
			}
			if ro.Valid {
				op.RO = &ro.String
			}
			if district.Valid {
				op.District = &district.String
			}
			if state.Valid {
				op.State = &state.String
			}
			if lastSyncTimestamp.Valid {
				op.LastSyncTimestamp = &lastSyncTimestamp.Time
			}
			if dataPath.Valid {
				op.DataPath = &dataPath.String
			}
			operators = append(operators, op)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[GetFilteredOperatorList] Row iteration error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Error while reading operator records",
				"details": err.Error(),
			})
			return
		}
	}

	// ── 10. Enrich with user_status from UID DB (cross-cluster) ───────────────
	if len(operators) > 0 {
		if uidStatusMap != nil {
			// user_status filter was requested; uidStatusMap already populated
			for i, op := range operators {
				if status, ok := uidStatusMap[strings.ToUpper(op.ID)]; ok {
					s := status
					operators[i].UserStatus = &s
				}
			}
		} else {
			// No user_status filter; targeted enrichment for just this page
			if uidDB, enrichErr := db.GetUIDDB(); enrichErr == nil {
				ids := make([]interface{}, 0, len(operators))
				phs := make([]string, 0, len(operators))
				for _, op := range operators {
					ids = append(ids, op.ID)
					phs = append(phs, "?")
				}
				if uidRows, enrichErr := uidDB.Query(
					`SELECT UPPER(user_code), user_status FROM user WHERE user_code IN (`+strings.Join(phs, ",")+`)`,
					ids...,
				); enrichErr == nil {
					defer uidRows.Close()
					statusMap := make(map[string]string)
					for uidRows.Next() {
						var code, status string
						if uidRows.Scan(&code, &status) == nil {
							statusMap[code] = status
						}
					}
					for i, op := range operators {
						if status, ok := statusMap[strings.ToUpper(op.ID)]; ok {
							s := status
							operators[i].UserStatus = &s
						}
					}
				}
			}
		}
	}

	// ── 11. Calculate total pages ──────────────────────────────────────────────
	totalPages := 0
	if total > 0 {
		totalPages = total / pageSize
		if total%pageSize != 0 {
			totalPages++
		}
	}

	// ── 12. Build applied_filters ──────────────────────────────────────────────
	appliedFilters := models.FilteredOperatorAppliedFilters{
		RO:         user.RegionalOffice,
		State:      filterState,
		District:   filterDistrict,
		ID:         filterID,
		EA:         filterEA,
		Reg:        filterReg,
		RiskBucket: filterRiskBucket,
		UserStatus: filterUserStatus,
	}

	log.Printf(
		"[GetFilteredOperatorList] ro=%s state=%q district=%q id=%q ea=%q reg=%q risk_bucket=%q user_status=%q → %d/%d records (page %d/%d)",
		user.RegionalOffice, filterState, filterDistrict, filterID, filterEA, filterReg, filterRiskBucket, filterUserStatus,
		len(operators), total, page, totalPages,
	)

	// ── 13. Respond ────────────────────────────────────────────────────────────
	c.JSON(http.StatusOK, models.FilteredOperatorResponse{
		Data:           operators,
		Total:          total,
		Page:           page,
		PageSize:       pageSize,
		TotalPages:     totalPages,
		AppliedFilters: appliedFilters,
	})
}
