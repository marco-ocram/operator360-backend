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

// validRiskBuckets holds the accepted values for the risk_bucket parameter
var validRiskBuckets = map[string]bool{
	"Low":    true,
	"Medium": true,
	"High":   true,
	"No":     true,
}

// GetOperatorListWithRisk queries data_platform.opt_master filtered by risk_bucket.
// Required query param : risk_bucket  (Low | Medium | High | No)
// Optional query params: page (default 1), page_size (default 20, max 1000)
func GetOperatorListWithRisk(c *gin.Context) {

	// ── 1. Auth guard ──────────────────────────────────────────────────────────
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	user, ok := userInterface.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user from context"})
		return
	}
	userRO := strings.TrimSpace(user.RegionalOffice)

	// ── 2. Validate risk_bucket ────────────────────────────────────────────────
	riskBucket := strings.TrimSpace(c.Query("risk_bucket"))
	if riskBucket == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "risk_bucket query parameter is required",
			"accepted_values": []string{"Low", "Medium", "High", "No"},
		})
		return
	}

	if !validRiskBuckets[riskBucket] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           fmt.Sprintf("invalid risk_bucket value: %q", riskBucket),
			"accepted_values": []string{"Low", "Medium", "High", "No"},
		})
		return
	}

	// ── 3. Pagination params ───────────────────────────────────────────────────
	page := 1
	pageSize := 20

	if pageParam := c.Query("page"); pageParam != "" {
		if _, err := fmt.Sscanf(pageParam, "%d", &page); err != nil || page < 1 {
			page = 1
		}
	}

	if pageSizeParam := c.Query("page_size"); pageSizeParam != "" {
		if _, err := fmt.Sscanf(pageSizeParam, "%d", &pageSize); err != nil || pageSize < 1 || pageSize > 1000 {
			pageSize = 20
		}
	}

	offset := (page - 1) * pageSize

	// ── 4. Get DB connection ───────────────────────────────────────────────────
	database, err := db.GetDB()
	if err != nil {
		log.Printf("[GetOperatorListWithRisk] DB connection error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Database connection unavailable",
			"details": err.Error(),
		})
		return
	}

	// ── 5. Count total matching rows ───────────────────────────────────────────
	countQuery := `SELECT COUNT(*) FROM data_platform.opt_master WHERE risk_bucket = ? AND ro = ?`
	var total int
	if err := database.QueryRow(countQuery, riskBucket, userRO).Scan(&total); err != nil {
		log.Printf("[GetOperatorListWithRisk] Count query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to count records",
			"details": err.Error(),
		})
		return
	}

	// ── 6. Fetch paginated rows from opt_master (no JOIN – separate cluster) ────
	dataQuery := `
		SELECT
			id,
			uid,
			name,
			phone,
			email,
			risk_score,
			risk_bucket,
			data_path,
			updated_at,
			reg,
			ea,
			district,
			state,
			last_sync_timestamp
		FROM data_platform.opt_master
		WHERE ro = ?
			AND risk_bucket = ?
		ORDER BY id
		LIMIT ? OFFSET ?
	`

	rows, err := database.Query(dataQuery, userRO, riskBucket, pageSize, offset)
	if err != nil {
		log.Printf("[GetOperatorListWithRisk] Data query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch operator records",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	// ── 7. Scan rows into model ────────────────────────────────────────────────
	operators := make([]models.OperatorWithRisk, 0, pageSize)

	for rows.Next() {
		var op models.OperatorWithRisk

		var (
			riskScore         sql.NullFloat64
			riskBucketCol     sql.NullString
			dataPath          sql.NullString
			updatedAt         sql.NullTime
			reg               sql.NullString
			ea                sql.NullString
			district          sql.NullString
			state             sql.NullString
			lastSyncTimestamp sql.NullTime
		)

		if err := rows.Scan(
			&op.ID, &op.UID, &op.Name, &op.Phone, &op.Email,
			&riskScore, &riskBucketCol,
			&dataPath, &updatedAt,
			&reg, &ea,
			&district, &state,
			&lastSyncTimestamp,
		); err != nil {
			log.Printf("[GetOperatorListWithRisk] Row scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to process operator record",
				"details": err.Error(),
			})
			return
		}

		if riskScore.Valid {
			op.RiskScore = &riskScore.Float64
		}
		if riskBucketCol.Valid {
			op.RiskBucket = &riskBucketCol.String
		}
		if dataPath.Valid {
			op.DataPath = &dataPath.String
		}
		if updatedAt.Valid {
			op.UpdatedAt = &updatedAt.Time
		}
		if reg.Valid {
			op.Reg = &reg.String
		}
		if ea.Valid {
			op.EA = &ea.String
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

		operators = append(operators, op)
	}

	// ── 8. Check for row-iteration errors ─────────────────────────────────────
	if err := rows.Err(); err != nil {
		log.Printf("[GetOperatorListWithRisk] Row iteration error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Error while reading operator records",
			"details": err.Error(),
		})
		return
	}
	// ── 9. Enrich with user_status & user_name from UID DB (separate cluster) ───
	if len(operators) > 0 {
		if uidDB, enrichErr := db.GetUIDDB(); enrichErr == nil {
			ids := make([]interface{}, 0, len(operators))
			phs := make([]string, 0, len(operators))
			for _, op := range operators {
				ids = append(ids, op.ID)
				phs = append(phs, "?")
			}
			uidRows, enrichErr := uidDB.Query(
				`SELECT UPPER(user_code), user_status, user_name FROM user WHERE user_code IN (`+strings.Join(phs, ",")+`)`,
				ids...,
			)
			if enrichErr == nil {
				defer uidRows.Close()
				type userInfo struct{ status, name string }
				userMap := make(map[string]userInfo)
				for uidRows.Next() {
					var code, status, name string
					if uidRows.Scan(&code, &status, &name) == nil {
						userMap[code] = userInfo{status, name}
					}
				}
				for i, op := range operators {
					if info, ok := userMap[op.ID]; ok {
						s := info.status
						n := info.name
						operators[i].UserStatus = &s
						operators[i].UserName = &n
					}
				}
			}
		} else {
			log.Printf("[GetOperatorListWithRisk] UID DB enrichment error (non-fatal): %v", enrichErr)
		}
	}
	// ── 9. Calculate total pages ───────────────────────────────────────────────
	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	if total == 0 {
		totalPages = 0
	}

	// ── 10. Return response ────────────────────────────────────────────────────
	log.Printf("[GetOperatorListWithRisk] Returning %d/%d records for risk_bucket=%s (page %d/%d)",
		len(operators), total, riskBucket, page, totalPages)

	c.JSON(http.StatusOK, models.OperatorWithRiskListResponse{
		Data:       operators,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		RiskBucket: riskBucket,
	})
}
