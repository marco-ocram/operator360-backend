
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

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
