package handlers

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// migrateAccountsFromDynamicDB mencari dan memigrasikan akun dari database dinamis ke master
func migrateAccountsFromDynamicDB() error {
	masterDB, err := getMasterBotDB()
	if err != nil {
		return fmt.Errorf("failed to get master DB: %w", err)
	}
	defer masterDB.Close()

	// Pastikan table ada di master
	if err := InitAccountDB(); err != nil {
		return fmt.Errorf("failed to init master DB: %w", err)
	}

	// Cari semua file bot_data*.db di current directory
	pattern := "bot_data*.db"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob database files: %w", err)
	}

	migratedCount := 0

	// CRITICAL FIX: Compile regex untuk performance
	perAccountPattern, _ := regexp.Compile(`^bot_data-\d+-\d+\.db$`)
	oldFormatPattern1, _ := regexp.Compile(`^bot_data_account_\d+\.db$`)
	oldFormatPattern2, _ := regexp.Compile(`^bot_data\(\d+\)>\(\d+\)\.db$`)

	// Cari semua database yang mungkin
	allDBFiles, err := scanAllDatabaseFiles()
	if err == nil {
		matches = append(matches, allDBFiles...)
	}

	for _, dbPath := range matches {
		// Skip master database
		if dbPath == "bot_data.db" {
			continue
		}

		// CRITICAL FIX: Skip per-account databases dengan format baru atau lama
		// Tabel whatsapp_accounts TIDAK BOLEH ada di database per-akun!
		// Format yang harus di-skip: bot_data-*-*.db, bot_data_account_*.db, bot_data(*)>(*).db
		if perAccountPattern.MatchString(dbPath) || oldFormatPattern1.MatchString(dbPath) || oldFormatPattern2.MatchString(dbPath) {
			// Ini adalah database per-akun, skip untuk mencegah pollution
			continue
		}

		// Coba buka database ini
		sourceDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_cache=shared")
		if err != nil {
			continue
		}
		defer sourceDB.Close()

		// Cek apakah ada table whatsapp_accounts
		var tableExists int
		err = sourceDB.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master 
			WHERE type='table' AND name='whatsapp_accounts'
		`).Scan(&tableExists)

		if err != nil || tableExists == 0 {
			continue
		}

		fmt.Printf("🔍 Found whatsapp_accounts table in: %s\n", dbPath)

		// Query akun dari database ini
		rows, err := sourceDB.Query(`
			SELECT id, phone_number, db_path, bot_data_db_path, status, created_at, updated_at
			FROM whatsapp_accounts
		`)
		if err != nil {
			sourceDB.Close()
			continue
		}

		for rows.Next() {
			var accountID int
			var phoneNumber, dbPath, botDataDBPath, status, createdAt, updatedAt string

			err := rows.Scan(
				&accountID,
				&phoneNumber,
				&dbPath,
				&botDataDBPath,
				&status,
				&createdAt,
				&updatedAt,
			)
			if err != nil {
				continue
			}

			// Cek apakah akun sudah ada di master (berdasarkan phone_number)
			var exists int
			err = masterDB.QueryRow(`
				SELECT COUNT(*) FROM whatsapp_accounts 
				WHERE phone_number = ?
			`, phoneNumber).Scan(&exists)

			if err != nil || exists > 0 {
				// Sudah ada, skip
				continue
			}

			// Insert ke master database
			_, err = masterDB.Exec(`
				INSERT INTO whatsapp_accounts (phone_number, db_path, bot_data_db_path, status, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?)
			`, phoneNumber, dbPath, botDataDBPath, status, createdAt, updatedAt)

			if err == nil {
				migratedCount++
				fmt.Printf("✅ Migrated account: %s from %s\n", phoneNumber, dbPath)
			}
		}

		rows.Close()
	}

	if migratedCount > 0 {
		fmt.Printf("✅ Migrated %d accounts to master database\n", migratedCount)
	}

	return nil
}

// scanAllDatabaseFiles mencari semua database yang mungkin berisi akun
func scanAllDatabaseFiles() ([]string, error) {
	var dbFiles []string

	// Pattern untuk mencari database
	patterns := []string{
		"bot_data*.db",
		"whatsapp_account_*.db",
		"whatsmeow*.db",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			// Skip master database
			if match == "bot_data.db" {
				continue
			}

			// Cek apakah file benar-benar database SQLite
			file, err := os.Open(match)
			if err != nil {
				continue
			}

			header := make([]byte, 16)
			n, _ := file.Read(header)
			file.Close()

			if n >= 16 && strings.HasPrefix(string(header), "SQLite format 3") {
				dbFiles = append(dbFiles, match)
			}
		}
	}

	return dbFiles, nil
}
