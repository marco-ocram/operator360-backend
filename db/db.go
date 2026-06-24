
package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"opt360-portal-backend/models"
)

var (
	db     *sql.DB
	dbOnce sync.Once
	dbErr  error

	uidDB     *sql.DB
	uidDBOnce sync.Once
	uidDBErr  error
	opt360DBErr  error

	portalDB     *sql.DB
	portalDBOnce sync.Once
	portalDBErr  error
)
	


// DBConfig holds database connection configuration
type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Database string
}

// InitDB initializes the database connection
func InitDB(config DBConfig) error {
	dbOnce.Do(func() {
		// Use mysql.Config to safely build connection string
		cfg := mysql.Config{
			User:                 config.User,
			Passwd:               config.Password,
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%d", config.Host, config.Port),
			DBName:               config.Database,
			AllowNativePasswords: true,
			ParseTime:            true,
		}

		db, dbErr = sql.Open("mysql", cfg.FormatDSN())
		if dbErr != nil {
			dbErr = fmt.Errorf("failed to open database connection: %w", dbErr)
			return
		}

		// Verify connection
		if err := db.Ping(); err != nil {
			dbErr = fmt.Errorf("failed to ping database: %w", err)
			db.Close()
			db = nil
			return
		}

		log.Println("Database connection established successfully")
	})

	return dbErr
}

// GetDB returns the database connection
func GetDB() (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return db, nil
}

// InitUIDDB initializes the UID database connection
func InitUIDDB(config DBConfig) error {
	uidDBOnce.Do(func() {
		// Use mysql.Config to safely build connection string
		cfg := mysql.Config{
			User:                 config.User,
			Passwd:               config.Password,
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%d", config.Host, config.Port),
			DBName:               config.Database,
			AllowNativePasswords: true,
			ParseTime:            true,
		}

		uidDB, uidDBErr = sql.Open("mysql", cfg.FormatDSN())
		if uidDBErr != nil {
			uidDBErr = fmt.Errorf("failed to open UID database connection: %w", uidDBErr)
			return
		}

		// Verify connection
		if err := uidDB.Ping(); err != nil {
			uidDBErr = fmt.Errorf("failed to ping UID database: %w", err)
			uidDB.Close()
			uidDB = nil
			return
		}

		log.Println("UID Database connection established successfully")
	})

	return uidDBErr
}

// GetUIDDB returns the UID database connection
func GetUIDDB() (*sql.DB, error) {
	if uidDB == nil {
		return nil, fmt.Errorf("UID database not initialized")
	}
	return uidDB, nil
}

// InitPortalDB initializes the portal (strot_services) database connection
func InitPortalDB(config DBConfig) error {
	portalDBOnce.Do(func() {
		cfg := mysql.Config{
			User:                 config.User,
			Passwd:               config.Password,
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%d", config.Host, config.Port),
			DBName:               config.Database,
			AllowNativePasswords: true,
			ParseTime:            true,
		}

		portalDB, portalDBErr = sql.Open("mysql", cfg.FormatDSN())
		if portalDBErr != nil {
			portalDBErr = fmt.Errorf("failed to open portal database connection: %w", portalDBErr)
			return
		}

		if err := portalDB.Ping(); err != nil {
			portalDBErr = fmt.Errorf("failed to ping portal database: %w", err)
			portalDB.Close()
			portalDB = nil
			return
		}

		log.Println("Portal Database connection established successfully")
	})
	return portalDBErr
}

// GetPortalDB returns the portal (strot_services) database connection
func GetPortalDB() (*sql.DB, error) {
	if portalDB == nil {
		return nil, fmt.Errorf("portal database not initialized")
	}
	return portalDB, nil
}

// GetUserByADID retrieves user information from database
func GetUserByADID(adID string) (*models.User, error) {
	database, err := GetPortalDB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		return nil, err
	}

	query := `
		SELECT user_id, user_name, ` + "`group`" + `, role 
		FROM opt360_portal_users 
		WHERE user_id = ?
	`

	log.Printf("Querying user with ADID: %s", adID)

	var user models.User
	err = database.QueryRow(query, adID).Scan(
		&user.ADID,
		&user.Name,
		&user.RegionalOffice,
		&user.Role,
	)

	if err == sql.ErrNoRows {
		log.Printf("No user found with ADID: %s", adID)
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		log.Printf("Query error for ADID '%s': %v", adID, err)
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	log.Printf("Successfully found user: %s (Role: %s, Group: %s)", user.Name, user.Role, user.RegionalOffice)
	return &user, nil
}

// GetAllUsers retrieves all users from database
func GetAllUsers() ([]models.User, error) {
	database, err := GetPortalDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT user_id, user_name, ` + "`group`" + `, role 
		FROM opt360_portal_users
	`

	rows, err := database.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ADID, &user.Name, &user.RegionalOffice, &user.Role); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}

// UpdateUserGroup updates the group field for a user identified by user_id
func UpdateUserGroup(userID string, group string) error {
	database, err := GetPortalDB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		return err
	}

	query := `UPDATE opt360_portal_users SET ` + "`group`" + ` = ? WHERE user_id = ?`

	log.Printf("Updating group for user_id: %s to: %s", userID, group)

	result, err := database.Exec(query, group, userID)
	if err != nil {
		log.Printf("Failed to update group for user_id '%s': %v", userID, err)
		return fmt.Errorf("failed to update user group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get rows affected: %v", err)
		return fmt.Errorf("failed to verify update: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("No user found with user_id: %s", userID)
		return fmt.Errorf("user not found")
	}

	log.Printf("Successfully updated group for user_id: %s", userID)
	return nil
}

// InsertMarkAnomaly inserts a new anomaly record into mark_anomaly table
func InsertMarkAnomaly(anomaly *models.MarkAnomaly) error {
	database, err := GetPortalDB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		return err
	}

	query := `
		INSERT INTO ` + "`mark_anomaly`" + ` (
			eid, anomaly_category, anomaly_code, anomaly_name, error_category,
			date_created, enrolnment_type, opt_district, opt_state, opt_id,
			pkt_source, pkt_updt_type, remarks, station_machine_code, station_no
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	log.Printf("Inserting anomaly record for EID: %s", anomaly.EID)

	_, err = database.Exec(query,
		anomaly.EID,
		anomaly.AnomalyCategory,
		anomaly.AnomalyCode,
		anomaly.AnomalyName,
		anomaly.ErrorCategory,
		anomaly.DateCreated,
		anomaly.EnrolmentType,
		anomaly.OptDistrict,
		anomaly.OptState,
		anomaly.OptID,
		anomaly.PktSource,
		anomaly.PktUpdtType,
		anomaly.Remarks,
		anomaly.StationMachineCode,
		anomaly.StationNo,
	)

	if err != nil {
		log.Printf("Failed to insert anomaly record for EID '%s': %v", anomaly.EID, err)
		return fmt.Errorf("failed to insert anomaly: %w", err)
	}

	log.Printf("Successfully inserted anomaly record for EID: %s", anomaly.EID)
	return nil
}

// OperatorStatus holds operator status information
type OperatorStatus struct {
	UserStatus string `json:"user_status"`
	UserName   string `json:"user_name"`
	UserUID    int64  `json:"user_uid"`
}

// ActiveOperator holds active operator information
type ActiveOperator struct {
	OptID       string   `json:"opt_id"`
	UserStatus  int      `json:"user_status"`
	UserName    string   `json:"user_name"`
	RiskScore   *float64 `json:"risk_score"`
	EAOrgName   string   `json:"ea_org_name"`
	RegOrgName  string   `json:"reg_org_name"`
	RegROName   string   `json:"reg_ro_name"`
}

// GetOperatorStatusByUserCode retrieves operator status from UID database
func GetOperatorStatusByUserCode(userCode string) (*OperatorStatus, error) {
	database, err := GetUIDDB()
	if err != nil {
		log.Printf("UID Database connection error: %v", err)
		return nil, err
	}

	query := `
		SELECT user_status, user_name, user_uid 
		FROM user 
		WHERE user_code = ?
	`

	log.Printf("Querying operator with user_code: %s", userCode)

	var status OperatorStatus
	var userUID sql.NullInt64
	err = database.QueryRow(query, userCode).Scan(
		&status.UserStatus,
		&status.UserName,
		&userUID,
	)

	if err == sql.ErrNoRows {
		log.Printf("No operator found with user_code: %s", userCode)
		return nil, fmt.Errorf("operator not found")
	}
	if err != nil {
		log.Printf("Query error for user_code '%s': %v", userCode, err)
		return nil, fmt.Errorf("failed to query operator: %w", err)
	}

	// Handle NULL user_uid
	if userUID.Valid {
		status.UserUID = userUID.Int64
	} else {
		status.UserUID = 0
	}

	log.Printf("Successfully found operator: %s (Status: %s, UID: %d)", status.UserName, status.UserStatus, status.UserUID)
	return &status, nil
}

// GetActiveOperators retrieves active operators using an application-level join
// across data_platform and uidmasterv1_1.
func GetActiveOperators(regionalOffice string) ([]ActiveOperator, error) {
	// Step 1: Query data_platform for all operators in this RO
	database, err := GetDB()
	if err != nil {
		log.Printf("DB connection error: %v", err)
		return nil, err
	}

	type optRow struct {
		optID, regOrgName, eaOrgName, regROName string
		riskScore *float64
	}

	optResultRows, err := database.Query(`
		SELECT opt_id, risk_score, reg_org_name, ea_org_name, reg_ro_name
		FROM data_platform.OptDetails
		WHERE reg_ro_name = ?`, regionalOffice)
	if err != nil {
		return nil, fmt.Errorf("failed to query operator details: %w", err)
	}
	defer optResultRows.Close()

	var optRows []optRow
	for optResultRows.Next() {
		var r optRow
		var rs sql.NullFloat64
		if err := optResultRows.Scan(&r.optID, &rs, &r.regOrgName, &r.eaOrgName, &r.regROName); err != nil {
			log.Printf("Failed to scan opt row: %v", err)
			continue
		}
		if rs.Valid {
			v := rs.Float64
			r.riskScore = &v
		}
		optRows = append(optRows, r)
	}
	if err := optResultRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating opt rows: %w", err)
	}
	if len(optRows) == 0 {
		return []ActiveOperator{}, nil
	}

	// Step 2: Query UID DB for user_status/user_name for these opt_ids
	uidDB, err := GetUIDDB()
	if err != nil {
		log.Printf("UID DB connection error: %v", err)
		return nil, err
	}

	ids := make([]interface{}, 0, len(optRows))
	phs := make([]string, 0, len(optRows))
	for _, r := range optRows {
		ids = append(ids, r.optID)
		phs = append(phs, "?")
	}
	uidResultRows, err := uidDB.Query(
		`SELECT UPPER(user_code), user_status, user_name FROM user WHERE user_code IN (`+strings.Join(phs, ",")+`)`,
		ids...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query UID users: %w", err)
	}
	defer uidResultRows.Close()

	type userInfo struct{ status, name string }
	userMap := make(map[string]userInfo)
	for uidResultRows.Next() {
		var code, status, name string
		if err := uidResultRows.Scan(&code, &status, &name); err != nil {
			continue
		}
		userMap[code] = userInfo{status, name}
	}

	// Step 3: Merge — keep only active users (user_status = '1')
	var operators []ActiveOperator
	for _, r := range optRows {
		info, ok := userMap[r.optID]
		if !ok || info.status != "1" {
			continue
		}
		op := ActiveOperator{
			OptID:      r.optID,
			UserStatus: 1,
			UserName:   info.name,
			RiskScore:  r.riskScore,
			RegOrgName: r.regOrgName,
			EAOrgName:  r.eaOrgName,
			RegROName:  r.regROName,
		}
		operators = append(operators, op)
	}

	log.Printf("Successfully retrieved %d active operators", len(operators))
	return operators, nil
}

// GetInactiveOperators retrieves inactive operators using an application-level join
// across data_platform and uidmasterv1_1.
func GetInactiveOperators(regionalOffice string) ([]ActiveOperator, error) {
	// Step 1: Query data_platform for all operators in this RO
	database, err := GetDB()
	if err != nil {
		log.Printf("DB connection error: %v", err)
		return nil, err
	}

	type optRow struct {
		optID, regOrgName, eaOrgName, regROName string
		riskScore *float64
	}

	optResultRows, err := database.Query(`
		SELECT opt_id, risk_score, reg_org_name, ea_org_name, reg_ro_name
		FROM data_platform.OptDetails
		WHERE reg_ro_name = ?`, regionalOffice)
	if err != nil {
		return nil, fmt.Errorf("failed to query operator details: %w", err)
	}
	defer optResultRows.Close()

	var optRows []optRow
	for optResultRows.Next() {
		var r optRow
		var rs sql.NullFloat64
		if err := optResultRows.Scan(&r.optID, &rs, &r.regOrgName, &r.eaOrgName, &r.regROName); err != nil {
			log.Printf("Failed to scan opt row: %v", err)
			continue
		}
		if rs.Valid {
			v := rs.Float64
			r.riskScore = &v
		}
		optRows = append(optRows, r)
	}
	if err := optResultRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating opt rows: %w", err)
	}
	if len(optRows) == 0 {
		return []ActiveOperator{}, nil
	}

	// Step 2: Query UID DB for user_status/user_name for these opt_ids
	uidDB, err := GetUIDDB()
	if err != nil {
		log.Printf("UID DB connection error: %v", err)
		return nil, err
	}

	ids := make([]interface{}, 0, len(optRows))
	phs := make([]string, 0, len(optRows))
	for _, r := range optRows {
		ids = append(ids, r.optID)
		phs = append(phs, "?")
	}
	uidResultRows, err := uidDB.Query(
		`SELECT UPPER(user_code), user_status, user_name FROM user WHERE user_code IN (`+strings.Join(phs, ",")+`)`,
		ids...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query UID users: %w", err)
	}
	defer uidResultRows.Close()

	type userInfo struct{ status, name string }
	userMap := make(map[string]userInfo)
	for uidResultRows.Next() {
		var code, status, name string
		if err := uidResultRows.Scan(&code, &status, &name); err != nil {
			continue
		}
		userMap[code] = userInfo{status, name}
	}

	// Step 3: Merge — keep only inactive users (user_status != '1')
	var operators []ActiveOperator
	for _, r := range optRows {
		info, ok := userMap[r.optID]
		if !ok || info.status == "1" {
			continue
		}
		op := ActiveOperator{
			OptID:      r.optID,
			UserStatus: 0,
			UserName:   info.name,
			RiskScore:  r.riskScore,
			RegOrgName: r.regOrgName,
			EAOrgName:  r.eaOrgName,
			RegROName:  r.regROName,
		}
		operators = append(operators, op)
	}

	log.Printf("Successfully retrieved %d inactive operators", len(operators))
	return operators, nil
}

// GetDataPathByOptID retrieves the data_path for a single operator from opt_master.
// This is the source of truth for where an operator's files live in S3 and
// must be used instead of deriving the path from RO/State/District/OptID.
func GetDataPathByOptID(optID string) (string, error) {
	database, err := GetDB()
	if err != nil {
		log.Printf("DB connection error: %v", err)
		return "", err
	}

	var dataPath sql.NullString
	err = database.QueryRow(`SELECT data_path FROM opt_master WHERE id = ?`, optID).Scan(&dataPath)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("operator not found: %s", optID)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query data_path for operator '%s': %w", optID, err)
	}
	if !dataPath.Valid || dataPath.String == "" {
		return "", fmt.Errorf("data_path not set for operator: %s", optID)
	}

	return dataPath.String, nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
