
package db

import (
	"database/sql"
	"fmt"
	"log"
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

	opt360DB     *sql.DB
	opt360DBOnce sync.Once
	opt360DBErr  error
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

// InitOpt360DB initializes the operator360 database connection
func InitOpt360DB(config DBConfig) error {
	opt360DBOnce.Do(func() {
		cfg := mysql.Config{
			User:                 config.User,
			Passwd:               config.Password,
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%d", config.Host, config.Port),
			DBName:               config.Database,
			AllowNativePasswords: true,
			ParseTime:            true,
		}

		opt360DB, opt360DBErr = sql.Open("mysql", cfg.FormatDSN())
		if opt360DBErr != nil {
			opt360DBErr = fmt.Errorf("failed to open operator360 database connection: %w", opt360DBErr)
			return
		}

		if err := opt360DB.Ping(); err != nil {
			opt360DBErr = fmt.Errorf("failed to ping operator360 database: %w", err)
			opt360DB.Close()
			opt360DB = nil
			return
		}

		log.Println("Operator360 Database connection established successfully")
	})

	return opt360DBErr
}

// GetOpt360DB returns the operator360 database connection
func GetOpt360DB() (*sql.DB, error) {
	if opt360DB == nil {
		return nil, fmt.Errorf("operator360 database not initialized")
	}
	return opt360DB, nil
}

// GetUserByADID retrieves user information from database
func GetUserByADID(adID string) (*models.User, error) {
	database, err := GetDB()
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
	database, err := GetDB()
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
	database, err := GetDB()
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
	database, err := GetDB()
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

// GetActiveOperators retrieves active operators from both databases
func GetActiveOperators(regionalOffice string) ([]ActiveOperator, error) {
	database, err := GetOpt360DB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		return nil, err
	}

	query := `
		SELECT 
			t1.opt_id,
			t2.user_status,
			t2.user_name,
			t1.risk_score, 
			t1.reg_org_name,
			t1.ea_org_name,
			t1.reg_ro_name
		FROM operator360.OptDetails AS t1
		INNER JOIN uidmasterv1_1.user AS t2 
			ON t1.opt_id = UPPER(t2.user_code)
		WHERE t1.reg_ro_name = ?
			AND t2.user_status = '1'
	`

	log.Printf("Querying active operators for regional office: %s", regionalOffice)

	rows, err := database.Query(query, regionalOffice)
	if err != nil {
		log.Printf("Failed to query active operators: %v", err)
		return nil, fmt.Errorf("failed to query active operators: %w", err)
	}
	defer rows.Close()

	var operators []ActiveOperator
	for rows.Next() {
		var op ActiveOperator
		var userStatus string
		var riskScore sql.NullFloat64
		
		err := rows.Scan(
			&op.OptID,
			&userStatus,
			&op.UserName,
			&riskScore,
			&op.RegOrgName,
			&op.EAOrgName,
			&op.RegROName,
		)
		
		if err != nil {
			log.Printf("Failed to scan operator row: %v", err)
			continue
		}
		
		// Convert user_status string to int
		if userStatus == "1" {
			op.UserStatus = 1
		} else {
			op.UserStatus = 0
		}
		
		// Handle NULL risk_score
		if riskScore.Valid {
			op.RiskScore = &riskScore.Float64
		} else {
			op.RiskScore = nil
		}
		
		operators = append(operators, op)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Successfully retrieved %d active operators", len(operators))
	return operators, nil
}

// GetInactiveOperators retrieves inactive operators from both databases
func GetInactiveOperators(regionalOffice string) ([]ActiveOperator, error) {
	database, err := GetOpt360DB()
	if err != nil {
		log.Printf("Database connection error: %v", err)
		return nil, err
	}

	query := `
		SELECT 
			t1.opt_id,
			t2.user_status,
			t2.user_name,
			t1.risk_score, 
			t1.reg_org_name,
			t1.ea_org_name,
			t1.reg_ro_name
		FROM operator360.OptDetails AS t1
		INNER JOIN uidmasterv1_1.user AS t2 
			ON t1.opt_id = UPPER(t2.user_code)
		WHERE t1.reg_ro_name = ?
			AND t2.user_status != '1'
	`

	log.Printf("Querying inactive operators for regional office: %s", regionalOffice)

	rows, err := database.Query(query, regionalOffice)
	if err != nil {
		log.Printf("Failed to query inactive operators: %v", err)
		return nil, fmt.Errorf("failed to query inactive operators: %w", err)
	}
	defer rows.Close()

	var operators []ActiveOperator
	for rows.Next() {
		var op ActiveOperator
		var userStatus string
		var riskScore sql.NullFloat64
		
		err := rows.Scan(
			&op.OptID,
			&userStatus,
			&op.UserName,
			&riskScore,
			&op.RegOrgName,
			&op.EAOrgName,
			&op.RegROName,
		)
		
		if err != nil {
			log.Printf("Failed to scan operator row: %v", err)
			continue
		}
		
		// Convert user_status string to int
		if userStatus == "1" {
			op.UserStatus = 1
		} else {
			op.UserStatus = 0
		}
		
		// Handle NULL risk_score
		if riskScore.Valid {
			op.RiskScore = &riskScore.Float64
		} else {
			op.RiskScore = nil
		}
		
		operators = append(operators, op)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Successfully retrieved %d inactive operators", len(operators))
	return operators, nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	if opt360DB != nil {
		opt360DB.Close()
	}
	return nil
}
