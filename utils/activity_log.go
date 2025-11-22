package utils

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ActivityLog menyimpan log aktivitas
type ActivityLog struct {
	ID             int
	Action         string
	Description    string
	TelegramChatID int64
	Success        bool
	ErrorMessage   string
	Metadata       map[string]interface{}
	CreatedAt      time.Time
}

// LogActivity mencatat aktivitas ke database
func LogActivity(action, description string, chatID int64) error {
	return LogActivityWithMetadata(action, description, chatID, nil, true)
}

// LogActivityError mencatat aktivitas dengan error
func LogActivityError(action, description string, chatID int64, err error) error {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
	}
	return LogActivityWithMetadata(action, description, chatID, map[string]interface{}{
		"error": errMsg,
	}, false)
}

// LogActivityWithMetadata mencatat aktivitas dengan metadata tambahan
func LogActivityWithMetadata(action, description string, chatID int64, metadata map[string]interface{}, success bool) error {
	// CRITICAL FIX: Gunakan GetBotDBPool() untuk memastikan menggunakan database yang benar per user
	db, err := GetBotDBPool()
	if err != nil {
		return err
	}
	// Jangan close pool, biarkan pool management handle

	var metadataJSON string
	if metadata != nil {
		jsonBytes, err := json.Marshal(metadata)
		if err == nil {
			metadataJSON = string(jsonBytes)
		}
	}

	successInt := 0
	if success {
		successInt = 1
	}

	if len(description) > 500 {
		description = description[:500] + "..."
	}

	_, err = db.Exec(`
		INSERT INTO activity_logs (action, description, telegram_chat_id, success, error_message, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
	`, action, description, chatID, successInt, "", metadataJSON)

	return err
}

// GetActivityLogs mengambil log aktivitas untuk user tertentu
// FIXED: Tambahkan parameter telegramChatID untuk filter per user (keamanan multi-user)
// SECURITY: Filter by telegram_chat_id untuk mencegah user melihat log user lain
func GetActivityLogs(telegramChatID int64, limit int) ([]ActivityLog, error) {
	// CRITICAL FIX: Gunakan GetBotDBPool() untuk memastikan menggunakan database yang benar per user
	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}
	// Jangan close pool, biarkan pool management handle

	// ✅ AMAN: Filter by telegram_chat_id untuk isolasi data per user
	query := "SELECT id, action, description, telegram_chat_id, success, error_message, metadata, created_at FROM activity_logs WHERE telegram_chat_id = ? ORDER BY created_at DESC LIMIT ?"
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(query, telegramChatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ActivityLog
	for rows.Next() {
		var log ActivityLog
		var metadataJSON sql.NullString
		var errMsg sql.NullString

		err := rows.Scan(&log.ID, &log.Action, &log.Description, &log.TelegramChatID, &log.Success, &errMsg, &metadataJSON, &log.CreatedAt)
		if err != nil {
			continue
		}

		// CRITICAL: Validasi bahwa log benar-benar milik user yang meminta
		// Defense in depth - meskipun sudah filter di query, validasi lagi di aplikasi
		if log.TelegramChatID != telegramChatID {
			GetLogger().Warn("GetActivityLogs: Security warning - log TelegramChatID mismatch: expected %d, got %d", telegramChatID, log.TelegramChatID)
			continue // Skip log yang tidak sesuai
		}

		if errMsg.Valid {
			log.ErrorMessage = errMsg.String
		}

		log.Success = (log.Success == true)

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &log.Metadata)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetActivityStats mengambil statistik aktivitas untuk user tertentu
// FIXED: Tambahkan parameter telegramChatID untuk filter per user (keamanan multi-user)
// SECURITY: Filter by telegram_chat_id untuk mencegah user melihat statistik user lain
func GetActivityStats(telegramChatID int64, days int) (map[string]interface{}, error) {
	// CRITICAL FIX: Gunakan GetBotDBPool() untuk memastikan menggunakan database yang benar per user
	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}
	// Jangan close pool, biarkan pool management handle

	stats := make(map[string]interface{})

	// ✅ AMAN: Filter by telegram_chat_id untuk isolasi data per user
	// Total aktivitas
	var totalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE telegram_chat_id = ? AND created_at >= datetime('now', '-' || ? || ' days')", telegramChatID, days).Scan(&totalCount)
	if err == nil {
		stats["total_activities"] = totalCount
	}

	// Aktivitas berhasil
	var successCount int
	err = db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE telegram_chat_id = ? AND success = 1 AND created_at >= datetime('now', '-' || ? || ' days')", telegramChatID, days).Scan(&successCount)
	if err == nil {
		stats["success_count"] = successCount
	}

	// Aktivitas gagal
	var failedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE telegram_chat_id = ? AND success = 0 AND created_at >= datetime('now', '-' || ? || ' days')", telegramChatID, days).Scan(&failedCount)
	if err == nil {
		stats["failed_count"] = failedCount
	}

	// Aktivitas per action
	actionQuery := `
		SELECT action, COUNT(*) as count 
		FROM activity_logs 
		WHERE telegram_chat_id = ? AND created_at >= datetime('now', '-' || ? || ' days')
		GROUP BY action 
		ORDER BY count DESC 
		LIMIT 10
	`
	rows, err := db.Query(actionQuery, telegramChatID, days)
	if err == nil {
		actionStats := make(map[string]int)
		for rows.Next() {
			var action string
			var count int
			if err := rows.Scan(&action, &count); err == nil {
				actionStats[action] = count
			}
		}
		rows.Close()
		stats["top_actions"] = actionStats
	}

	return stats, nil
}
