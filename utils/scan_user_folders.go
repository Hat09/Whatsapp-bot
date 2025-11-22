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

// ScanUserFoldersAndRegisterAccounts memindai folder user dan mendaftarkan account yang sudah ada
// Fungsi ini dipanggil saat startup untuk mendaftarkan account yang sudah ada di folder user
// tetapi belum terdaftar di database master
func ScanUserFoldersAndRegisterAccounts() error {
	logger := GetLogger()
	logger.Info("Memulai scan folder user untuk mendaftarkan account yang sudah ada...")

	// Buka database master
	dbPath := "bot_data.db"
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=DELETE&_cache=shared")
	if err != nil {
		return fmt.Errorf("gagal membuka database master: %v", err)
	}
	defer db.Close()

	// Pastikan tabel ada
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS whatsapp_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			phone_number TEXT UNIQUE NOT NULL,
			db_path TEXT NOT NULL,
			bot_data_db_path TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("gagal membuat tabel: %v", err)
	}

	// Pattern untuk database: whatsmeow-{telegramID}-{phoneNumber}.db
	pattern := regexp.MustCompile(`^whatsmeow-(\d+)-(\d+)\.db$`)

	// Scan semua folder di "DB USER TELEGRAM"
	userFolders, err := filepath.Glob("DB USER TELEGRAM/*")
	if err != nil {
		return fmt.Errorf("gagal scan folder user: %v", err)
	}

	registeredCount := 0
	for _, userFolder := range userFolders {
		// Cek apakah ini folder (bukan file)
		info, err := os.Stat(userFolder)
		if err != nil || !info.IsDir() {
			continue
		}

		// Parse Telegram ID dari nama folder
		telegramIDStr := filepath.Base(userFolder)
		_, err = strconv.ParseInt(telegramIDStr, 10, 64)
		if err != nil {
			logger.Debug("Skip folder yang tidak valid: %s", userFolder)
			continue
		}

		// Cari semua file whatsmeow-*.db di folder user ini
		whatsappFiles, err := filepath.Glob(filepath.Join(userFolder, "whatsmeow-*.db"))
		if err != nil {
			continue
		}

		for _, whatsappFile := range whatsappFiles {
			matches := pattern.FindStringSubmatch(filepath.Base(whatsappFile))
			if len(matches) != 3 {
				continue
			}

			fileTelegramID := matches[1]
			phoneNumber := matches[2]

			// Pastikan Telegram ID dari file sama dengan Telegram ID dari folder
			if fileTelegramID != telegramIDStr {
				logger.Warn("Telegram ID tidak match: folder=%s, file=%s", telegramIDStr, fileTelegramID)
				continue
			}

			// Cek apakah file database valid (ada device store) - cek dulu sebelum query database
			botDataDBPath := filepath.Join(userFolder, fmt.Sprintf("bot_data-%s-%s.db", fileTelegramID, phoneNumber))
			if !isValidAccountDatabase(whatsappFile) {
				logger.Warn("Database tidak valid untuk nomor %s: %s", phoneNumber, whatsappFile)
				continue
			}

			// Cek apakah account sudah terdaftar di database master (cek berdasarkan nomor telepon)
			var existingID int
			var existingDBPath, existingBotDataDBPath string
			err = db.QueryRow(`
				SELECT id, db_path, bot_data_db_path FROM whatsapp_accounts 
				WHERE phone_number = ?
			`, phoneNumber).Scan(&existingID, &existingDBPath, &existingBotDataDBPath)

			if err == nil {
				// Account sudah terdaftar, cek apakah path berbeda
				if existingDBPath != whatsappFile || existingBotDataDBPath != botDataDBPath {
					// Path berbeda, update path
					_, updateErr := db.Exec(`
						UPDATE whatsapp_accounts 
						SET db_path = ?, bot_data_db_path = ?, updated_at = CURRENT_TIMESTAMP
						WHERE phone_number = ?
					`, whatsappFile, botDataDBPath, phoneNumber)
					if updateErr != nil {
						logger.Warn("Gagal update path account untuk nomor %s: %v", phoneNumber, updateErr)
						continue
					}
					logger.Info("✅ Berhasil update path account untuk nomor %s: %s -> %s", phoneNumber, existingDBPath, whatsappFile)
					registeredCount++
				} else {
					// Path sama, skip
					logger.Debug("Account untuk nomor %s sudah terdaftar dengan path yang sama, skip", phoneNumber)
				}
			} else if err == sql.ErrNoRows {
				// Account belum terdaftar, daftarkan
				_, err = db.Exec(`
					INSERT INTO whatsapp_accounts (phone_number, db_path, bot_data_db_path, status)
					VALUES (?, ?, ?, 'active')
				`, phoneNumber, whatsappFile, botDataDBPath)
				if err != nil {
					logger.Warn("Gagal mendaftarkan account untuk nomor %s: %v", phoneNumber, err)
					continue
				}
				logger.Info("✅ Berhasil mendaftarkan account untuk nomor %s: %s", phoneNumber, whatsappFile)
				registeredCount++
			} else {
				// Error lain
				logger.Warn("Gagal cek account untuk nomor %s: %v", phoneNumber, err)
				continue
			}
		}
	}

	if registeredCount > 0 {
		logger.Info("✅ Scan selesai: %d account berhasil didaftarkan dari folder user", registeredCount)
	} else {
		logger.Info("ℹ️ Tidak ada account baru yang perlu didaftarkan")
	}

	return nil
}

// isValidAccountDatabase mengecek apakah database account valid (ada device store)
func isValidAccountDatabase(dbPath string) bool {
	// Cek apakah file ada
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false
	}

	// Buka database untuk cek apakah ada device store
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=DELETE&_cache=shared")
	if err != nil {
		return false
	}
	defer db.Close()

	// Cek apakah ada tabel whatsmeow_device (indikasi account sudah login)
	// Database whatsmeow menggunakan tabel whatsmeow_device, bukan device atau auth_keys
	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='whatsmeow_device'
	`).Scan(&count)
	if err != nil {
		return false
	}

	if count > 0 {
		// Cek apakah ada data di tabel whatsmeow_device (ada device ID berarti sudah login)
		var deviceCount int
		err = db.QueryRow(`SELECT COUNT(*) FROM whatsmeow_device`).Scan(&deviceCount)
		if err != nil {
			return false
		}
		return deviceCount > 0
	}

	return false
}
