package utils

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

// MigrateDatabaseToUserFolder memindahkan database yang sudah ada ke folder user yang sesuai
// Fungsi ini dipanggil saat startup untuk memindahkan database yang ada di root ke folder user
func MigrateDatabaseToUserFolder() error {
	logger := GetLogger()
	logger.Info("Memulai migrasi database ke folder per user...")

	// Pattern untuk database format baru: whatsmeow-{telegramID}-{phoneNumber}.db
	pattern := regexp.MustCompile(`^whatsmeow-(\d+)-(\d+)\.db$`)

	// Cari semua file whatsmeow-*.db di root directory
	files, err := filepath.Glob("whatsmeow-*.db")
	if err != nil {
		return fmt.Errorf("gagal mencari file database: %v", err)
	}

	migratedCount := 0
	for _, file := range files {
		// Skip jika file sudah di dalam folder user
		if filepath.Dir(file) != "." {
			continue
		}

		matches := pattern.FindStringSubmatch(filepath.Base(file))
		if len(matches) != 3 {
			logger.Debug("Skip file yang tidak match pattern: %s", file)
			continue
		}

		telegramIDStr := matches[1]
		phoneNumber := matches[2]

		// Parse Telegram ID
		telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
		if err != nil {
			logger.Warn("Gagal parse Telegram ID dari file %s: %v", file, err)
			continue
		}

		// Pastikan folder user sudah ada
		userFolder := GetUserDBFolder(telegramID)
		if err := EnsureUserDBFolder(telegramID); err != nil {
			logger.Warn("Gagal membuat folder untuk user %d: %v", telegramID, err)
			continue
		}

		// Path tujuan
		whatsappDBDest := filepath.Join(userFolder, filepath.Base(file))
		botDataDBName := fmt.Sprintf("bot_data-%s-%s.db", telegramIDStr, phoneNumber)
		botDataDBDest := filepath.Join(userFolder, botDataDBName)

		// Pindahkan whatsmeow database
		if err := moveDatabaseFile(file, whatsappDBDest); err != nil {
			logger.Warn("Gagal memindahkan %s ke %s: %v", file, whatsappDBDest, err)
			continue
		}
		logger.Info("✅ Berhasil memindahkan %s ke %s", file, whatsappDBDest)

		// Pindahkan bot_data database jika ada
		if _, err := os.Stat(botDataDBName); err == nil {
			if err := moveDatabaseFile(botDataDBName, botDataDBDest); err != nil {
				logger.Warn("Gagal memindahkan %s ke %s: %v", botDataDBName, botDataDBDest, err)
			} else {
				logger.Info("✅ Berhasil memindahkan %s ke %s", botDataDBName, botDataDBDest)
			}
		}

		// CRITICAL: Update path di database master setelah migrasi
		if err := updateAccountPathsInMasterDB(phoneNumber, whatsappDBDest, botDataDBDest); err != nil {
			logger.Warn("Gagal update path di database master untuk nomor %s: %v", phoneNumber, err)
			// Continue anyway, path akan di-update saat LoadAccounts
		} else {
			logger.Info("✅ Berhasil update path di database master untuk nomor %s", phoneNumber)
		}

		migratedCount++
	}

	// Cari juga bot_data-*.db yang mungkin tidak punya pasangan whatsmeow
	botDataFiles, err := filepath.Glob("bot_data-*.db")
	if err == nil {
		botDataPattern := regexp.MustCompile(`^bot_data-(\d+)-(\d+)\.db$`)
		for _, file := range botDataFiles {
			// Skip jika file sudah di dalam folder user
			if filepath.Dir(file) != "." {
				continue
			}

			matches := botDataPattern.FindStringSubmatch(filepath.Base(file))
			if len(matches) != 3 {
				continue
			}

			telegramIDStr := matches[1]
			_ = matches[2] // phoneNumber tidak digunakan di sini, hanya untuk validasi pattern

			// Parse Telegram ID
			telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
			if err != nil {
				logger.Warn("Gagal parse Telegram ID dari file %s: %v", file, err)
				continue
			}

			// Pastikan folder user sudah ada
			if err := EnsureUserDBFolder(telegramID); err != nil {
				logger.Warn("Gagal membuat folder untuk user %d: %v", telegramID, err)
				continue
			}

			// Path tujuan
			userFolder := GetUserDBFolder(telegramID)
			botDataDBDest := filepath.Join(userFolder, filepath.Base(file))

			// Pindahkan bot_data database
			if err := moveDatabaseFile(file, botDataDBDest); err != nil {
				logger.Warn("Gagal memindahkan %s ke %s: %v", file, botDataDBDest, err)
			} else {
				logger.Info("✅ Berhasil memindahkan %s ke %s", file, botDataDBDest)
				migratedCount++
			}
		}
	}

	if migratedCount > 0 {
		logger.Info("✅ Migrasi selesai: %d database berhasil dipindahkan ke folder per user", migratedCount)
	} else {
		logger.Info("ℹ️ Tidak ada database yang perlu dimigrasi")
	}

	return nil
}

// updateAccountPathsInMasterDB memperbarui path database di database master setelah migrasi
func updateAccountPathsInMasterDB(phoneNumber, newDBPath, newBotDataDBPath string) error {
	// Buka database master
	dbPath := "bot_data.db"
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=DELETE&_cache=shared")
	if err != nil {
		return fmt.Errorf("gagal membuka database master: %v", err)
	}
	defer db.Close()

	// Update path untuk account dengan nomor yang sesuai
	// Cek apakah ada account dengan nomor ini
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM whatsapp_accounts WHERE phone_number = ?`, phoneNumber).Scan(&count)
	if err != nil {
		// Tabel mungkin belum ada, tidak apa-apa
		return nil
	}

	if count > 0 {
		// Update path untuk semua account dengan nomor ini
		_, err = db.Exec(`
			UPDATE whatsapp_accounts 
			SET db_path = ?, bot_data_db_path = ?, updated_at = CURRENT_TIMESTAMP 
			WHERE phone_number = ?
		`, newDBPath, newBotDataDBPath, phoneNumber)
		if err != nil {
			return fmt.Errorf("gagal update path di database master: %v", err)
		}
	}

	return nil
}

// moveDatabaseFile memindahkan file database beserta file pendukungnya (-shm, -wal)
func moveDatabaseFile(oldPath, newPath string) error {
	// List file yang perlu dipindahkan
	filesToMove := []string{
		oldPath,
		oldPath + "-shm",
		oldPath + "-wal",
	}

	for _, file := range filesToMove {
		if _, err := os.Stat(file); err == nil {
			// File exists, move it
			destFile := newPath
			if file != oldPath {
				// Untuk file -shm dan -wal, sesuaikan nama tujuan
				destFile = newPath + file[len(oldPath):]
			}

			// Pastikan folder tujuan sudah ada
			if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
				return fmt.Errorf("gagal membuat folder tujuan: %v", err)
			}

			if err := os.Rename(file, destFile); err != nil {
				return fmt.Errorf("gagal memindahkan %s ke %s: %v", file, destFile, err)
			}
		}
	}

	return nil
}
