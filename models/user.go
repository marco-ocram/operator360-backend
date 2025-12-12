package models

// User represents a user in the system
type User struct {
	ADID           string `json:"ad_id"`
	Name           string `json:"name"`
	RegionalOffice string `json:"regional_office"`
}

// UsersConfig holds the complete users configuration
type UsersConfig struct {
	Users []User `json:"users"`
}
