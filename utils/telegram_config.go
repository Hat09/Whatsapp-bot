package utils

import (
	"encoding/json"
	"os"
)

type TelegramConfig struct {
	TelegramToken  string          `json:"telegram_token"`
	UserAllowedID  int64           `json:"user_allowed_id"`    // DEPRECATED: Gunakan AllowedUserIDs
	AdminIDs       []int64         `json:"admin_ids"`          // List admin users
	AllowedUserIDs []int64         `json:"allowed_user_ids"`   // List allowed users
	Settings       *ConfigSettings `json:"settings,omitempty"` // Optional settings
}

type ConfigSettings struct {
	MaxAccounts    int    `json:"max_accounts,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	RetryAttempts  int    `json:"retry_attempts,omitempty"`
	LogLevel       string `json:"log_level,omitempty"`
}

// LoadConfig memuat konfigurasi Telegram dari file akses.json
func LoadTelegramConfig() (*TelegramConfig, error) {
	// Try config/config.json first (new format)
	file, err := os.ReadFile("config/config.json")
	if err != nil {
		// Fallback to old akses.json (backward compatibility)
		file, err = os.ReadFile("akses.json")
		if err != nil {
			return nil, err
		}
	}

	var config TelegramConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	// Backward compatibility: if UserAllowedID is set but AllowedUserIDs is empty,
	// add UserAllowedID to AllowedUserIDs
	if config.UserAllowedID != 0 && len(config.AllowedUserIDs) == 0 {
		config.AllowedUserIDs = []int64{config.UserAllowedID}
	}

	// Backward compatibility: if no admin is set, use allowed users as admin
	if len(config.AdminIDs) == 0 && len(config.AllowedUserIDs) > 0 {
		config.AdminIDs = config.AllowedUserIDs
	}

	return &config, nil
}

// IsAdmin checks if a user ID is an admin
func (tc *TelegramConfig) IsAdmin(userID int64) bool {
	for _, adminID := range tc.AdminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}

// IsAllowed checks if a user ID is allowed to use the bot
func (tc *TelegramConfig) IsAllowed(userID int64) bool {
	for _, allowedID := range tc.AllowedUserIDs {
		if allowedID == userID {
			return true
		}
	}
	return false
}

// CheckAccess checks if user has access (either admin or allowed user)
func (tc *TelegramConfig) CheckAccess(userID int64) bool {
	return tc.IsAdmin(userID) || tc.IsAllowed(userID)
}
