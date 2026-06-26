package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"opt360-portal-backend/models"
)

const (
	operatorFilterFrom   = "FROM operator360.opt_master"
	operatorFilterSelect = `SELECT id, name, risk_score, risk_bucket, reg, ea, ro, district, state, last_sync_timestamp, data_path`
)

// OperatorFilterParams holds the supported GetFilteredOperatorList filters.
type OperatorFilterParams struct {
	RegionalOffice string
	IsAdmin        bool
	State          string
	District       string
	ID             string
	EA             string
	Reg            string
	RiskBucket     string
	UserStatus     string // "active", "inactive", or "" (no filter)
	PageSize       int
	Offset         int
}

// GetFilteredOperators resolves GetFilteredOperatorList's query against
// operator360.opt_master (data_platform cluster). Because opt_master and
// uidmasterv1_1.user (UID cluster) live on different database servers, the
// user_status filter is resolved via an application-level join:
//  1. Fetch matching user_codes from the UID cluster.
//  2. Intersect them in-memory with opt_master IDs (avoids a giant IN (...)
//     clause that could exceed MySQL's 65535-placeholder limit).
//  3. Enrich each result page with user_status from the UID cluster.
func GetFilteredOperators(p OperatorFilterParams) ([]models.FilteredOperator, int, error) {
	database, err := GetDB()
	if err != nil {
		log.Printf("DB connection error: %v", err)
		return nil, 0, fmt.Errorf("database connection unavailable: %w", err)
	}

	var uidStatusMap map[string]string
	if p.UserStatus != "" {
		uidStatusMap, err = queryUIDStatusMap(p.UserStatus)
		if err != nil {
			return nil, 0, err
		}
		if len(uidStatusMap) == 0 {
			return []models.FilteredOperator{}, 0, nil
		}
	}

	whereSQL, args := buildOperatorFilterWhere(p)

	total, pageIDs, err := countFilteredOperators(database, whereSQL, args, uidStatusMap, p.Offset, p.PageSize)
	if err != nil {
		return nil, 0, err
	}

	operators, err := fetchFilteredOperatorPage(database, whereSQL, args, p, pageIDs, uidStatusMap)
	if err != nil {
		return nil, 0, err
	}

	enrichFilteredOperatorsUserStatus(operators, uidStatusMap)

	return operators, total, nil
}

// queryUIDStatusMap fetches UPPER(user_code) -> user_status pairs from the
// UID cluster matching the requested "active"/"inactive" filter.
func queryUIDStatusMap(userStatus string) (map[string]string, error) {
	uidDB, err := GetUIDDB()
	if err != nil {
		return nil, fmt.Errorf("UID database connection unavailable: %w", err)
	}

	var statusCond string
	switch strings.ToLower(userStatus) {
	case "active":
		statusCond = "user_status = '1'"
	case "inactive":
		statusCond = "user_status != '1'"
	default:
		statusCond = "1=1"
	}

	rows, err := uidDB.Query(`SELECT UPPER(user_code), user_status FROM user WHERE user_code IS NOT NULL AND ` + statusCond)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user status data: %w", err)
	}
	defer rows.Close()

	statusMap := make(map[string]string)
	for rows.Next() {
		var code, status string
		if err := rows.Scan(&code, &status); err != nil {
			continue
		}
		statusMap[code] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading user status data: %w", err)
	}
	return statusMap, nil
}

// buildOperatorFilterWhere builds the WHERE clause shared by the count and
// data queries. Global ID search is only allowed for admins; all other
// roles are scoped to their own regional office.
func buildOperatorFilterWhere(p OperatorFilterParams) (string, []interface{}) {
	var clauses []string
	var args []interface{}

	if p.ID == "" || !p.IsAdmin {
		clauses = append(clauses, "ro = ?")
		args = append(args, p.RegionalOffice)
	}
	if p.State != "" {
		clauses = append(clauses, "state = ?")
		args = append(args, p.State)
	}
	if p.District != "" {
		clauses = append(clauses, "district = ?")
		args = append(args, p.District)
	}
	if p.ID != "" {
		clauses = append(clauses, "id = ?")
		args = append(args, p.ID)
	}
	if p.EA != "" {
		clauses = append(clauses, "ea = ?")
		args = append(args, p.EA)
	}
	if p.Reg != "" {
		clauses = append(clauses, "reg = ?")
		args = append(args, p.Reg)
	}
	if p.RiskBucket != "" {
		clauses = append(clauses, "risk_bucket = ?")
		args = append(args, p.RiskBucket)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// countFilteredOperators returns the total matching record count. When a
// user_status filter is active, it also returns the page's worth of IDs
// (computed by intersecting opt_master IDs with uidStatusMap in memory)
// since pagination can't be pushed down to SQL in that path.
func countFilteredOperators(database *sql.DB, whereSQL string, args []interface{}, uidStatusMap map[string]string, offset, pageSize int) (int, []string, error) {
	if uidStatusMap != nil {
		idQuery := fmt.Sprintf("SELECT id %s %s ORDER BY id", operatorFilterFrom, whereSQL)
		rows, err := database.Query(idQuery, args...)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to fetch operator IDs: %w", err)
		}
		defer rows.Close()

		var filteredIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			if _, ok := uidStatusMap[strings.ToUpper(id)]; ok {
				filteredIDs = append(filteredIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			return 0, nil, fmt.Errorf("error reading operator IDs: %w", err)
		}

		total := len(filteredIDs)
		var pageIDs []string
		start, end := offset, offset+pageSize
		if start < total {
			if end > total {
				end = total
			}
			pageIDs = filteredIDs[start:end]
		}
		return total, pageIDs, nil
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s %s", operatorFilterFrom, whereSQL)
	if err := database.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("failed to count records: %w", err)
	}
	return total, nil, nil
}

// fetchFilteredOperatorPage fetches the page of opt_master rows: by exact
// ID list when a user_status filter narrowed the set, otherwise via a
// standard LIMIT/OFFSET query.
func fetchFilteredOperatorPage(database *sql.DB, whereSQL string, args []interface{}, p OperatorFilterParams, pageIDs []string, uidStatusMap map[string]string) ([]models.FilteredOperator, error) {
	var rows *sql.Rows
	var err error

	if uidStatusMap != nil {
		if len(pageIDs) == 0 {
			return []models.FilteredOperator{}, nil
		}
		phs := make([]string, len(pageIDs))
		pageArgs := make([]interface{}, len(pageIDs))
		for i, id := range pageIDs {
			phs[i] = "?"
			pageArgs[i] = id
		}
		pageQuery := fmt.Sprintf("%s %s WHERE id IN (%s) ORDER BY id", operatorFilterSelect, operatorFilterFrom, strings.Join(phs, ","))
		rows, err = database.Query(pageQuery, pageArgs...)
	} else {
		dataQuery := fmt.Sprintf("%s %s %s ORDER BY id LIMIT ? OFFSET ?", operatorFilterSelect, operatorFilterFrom, whereSQL)
		dataArgs := append(append([]interface{}{}, args...), p.PageSize, p.Offset)
		rows, err = database.Query(dataQuery, dataArgs...)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch operator records: %w", err)
	}
	defer rows.Close()

	operators := make([]models.FilteredOperator, 0, p.PageSize)
	for rows.Next() {
		op, err := scanFilteredOperator(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to process operator record: %w", err)
		}
		operators = append(operators, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while reading operator records: %w", err)
	}
	return operators, nil
}

func scanFilteredOperator(rows *sql.Rows) (models.FilteredOperator, error) {
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
		return op, err
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
	return op, nil
}

// enrichFilteredOperatorsUserStatus fills in UserStatus for each operator.
// If uidStatusMap is already populated (the user_status filter was active),
// it's reused directly; otherwise a targeted lookup is made for just this
// page (best-effort; UID lookup failures are logged but don't fail the
// request).
func enrichFilteredOperatorsUserStatus(operators []models.FilteredOperator, uidStatusMap map[string]string) {
	if len(operators) == 0 {
		return
	}

	if uidStatusMap != nil {
		for i, op := range operators {
			if status, ok := uidStatusMap[strings.ToUpper(op.ID)]; ok {
				s := status
				operators[i].UserStatus = &s
			}
		}
		return
	}

	uidDB, err := GetUIDDB()
	if err != nil {
		log.Printf("[GetFilteredOperators] UID DB enrichment error (non-fatal): %v", err)
		return
	}

	ids := make([]interface{}, 0, len(operators))
	phs := make([]string, 0, len(operators))
	for _, op := range operators {
		ids = append(ids, op.ID)
		phs = append(phs, "?")
	}

	rows, err := uidDB.Query(
		`SELECT UPPER(user_code), user_status FROM user WHERE user_code IN (`+strings.Join(phs, ",")+`)`,
		ids...,
	)
	if err != nil {
		log.Printf("[GetFilteredOperators] UID DB enrichment error (non-fatal): %v", err)
		return
	}
	defer rows.Close()

	statusMap := make(map[string]string)
	for rows.Next() {
		var code, status string
		if rows.Scan(&code, &status) == nil {
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
