package auth

import (
	"encoding/json"
	"os"

	"opt360-portal-backend/models"
)

var UsersConfig models.UsersConfig

// LoadUsersConfig loads the users configuration from users.json
func LoadUsersConfig() error {
	file, err := os.ReadFile("users.json")
	if err != nil {
		return err
	}

	if err := json.Unmarshal(file, &UsersConfig); err != nil {
		return err
	}

	return nil
}

// GetUserByADID retrieves user information from users config
func GetUserByADID(adID string) (*models.User, bool) {
	for _, user := range UsersConfig.Users {
		if user.ADID == adID {
			return &user, true
		}
	}
	return nil, false
}
