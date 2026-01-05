package models

// User represents a user in the system
type User struct {
	ADID           string `json:"ad_id" db:"user_id"`
	Name           string `json:"name" db:"user_name"`
	RegionalOffice string `json:"regional_office" db:"group"`
	Role           string `json:"role" db:"role"`
}

// UsersConfig holds the complete users configuration (deprecated - for backward compatibility)
type UsersConfig struct {
	Users []User `json:"users"`
}
