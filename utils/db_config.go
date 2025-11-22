package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DBConfig menyimpan konfigurasi nama database
type DBConfig struct {
	TelegramID     int64
	WhatsAppNumber string
	WhatsAppDBName string
	BotDataDBName  string
}

var dbConfig *DBConfig

// GetDBConfig mendapatkan konfigurasi database
func GetDBConfig() *DBConfig {
	return dbConfig
}

// GetUserDBFolder mendapatkan path folder database untuk user tertentu
// Format: DB USER TELEGRAM/{telegramID}/
func GetUserDBFolder(telegramID int64) string {
	return filepath.Join("DB USER TELEGRAM", strconv.FormatInt(telegramID, 10))
}

// EnsureUserDBFolder memastikan folder database untuk user sudah ada
// Jika belum ada, buat folder tersebut
func EnsureUserDBFolder(telegramID int64) error {
	folderPath := GetUserDBFolder(telegramID)
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return fmt.Errorf("gagal membuat folder database untuk user %d: %v", telegramID, err)
	}
	return nil
}

// SetDBConfig mengatur konfigurasi database berdasarkan Telegram ID dan nomor WhatsApp
// Format baru: DB USER TELEGRAM/{telegramID}/whatsmeow-{userid}-{nomorwhatsapp}.db
func SetDBConfig(telegramID int64, whatsappNumber string) {
	// Bersihkan nomor WhatsApp dari karakter non-digit
	cleanNumber := strings.ReplaceAll(whatsappNumber, "+", "")
	cleanNumber = strings.ReplaceAll(cleanNumber, "-", "")
	cleanNumber = strings.ReplaceAll(cleanNumber, " ", "")

	// Pastikan folder database untuk user sudah ada
	if err := EnsureUserDBFolder(telegramID); err != nil {
		GetLogger().Warn("Failed to ensure user DB folder: %v", err)
	}

	// Path database dalam folder user
	userFolder := GetUserDBFolder(telegramID)
	whatsappDBName := fmt.Sprintf("whatsmeow-%d-%s.db", telegramID, cleanNumber)
	botDataDBName := fmt.Sprintf("bot_data-%d-%s.db", telegramID, cleanNumber)

	dbConfig = &DBConfig{
		TelegramID:     telegramID,
		WhatsAppNumber: cleanNumber,
		WhatsAppDBName: filepath.Join(userFolder, whatsappDBName),
		BotDataDBName:  filepath.Join(userFolder, botDataDBName),
	}
}

// GetWhatsAppDBPath mendapatkan path database WhatsApp
func GetWhatsAppDBPath() string {
	if dbConfig != nil && dbConfig.WhatsAppDBName != "" {
		return dbConfig.WhatsAppDBName
	}
	// Default fallback
	return "whatsapp.db"
}

// GetBotDataDBPath mendapatkan path database bot
func GetBotDataDBPath() string {
	if dbConfig != nil && dbConfig.BotDataDBName != "" {
		return dbConfig.BotDataDBName
	}
	// Default fallback
	return "bot_data.db"
}

// RenameDatabaseFiles merename database files dari nama lama ke nama baru
func RenameDatabaseFiles(oldWhatsAppDB, newWhatsAppDB, oldBotDataDB, newBotDataDB string) error {
	// List file database yang perlu di-rename (termasuk -shm dan -wal)
	filesToRename := []struct {
		old string
		new string
	}{
		{oldWhatsAppDB, newWhatsAppDB},
		{oldWhatsAppDB + "-shm", newWhatsAppDB + "-shm"},
		{oldWhatsAppDB + "-wal", newWhatsAppDB + "-wal"},
		{oldBotDataDB, newBotDataDB},
		{oldBotDataDB + "-shm", newBotDataDB + "-shm"},
		{oldBotDataDB + "-wal", newBotDataDB + "-wal"},
	}

	for _, file := range filesToRename {
		if _, err := os.Stat(file.old); err == nil {
			// File exists, rename it
			if err := os.Rename(file.old, file.new); err != nil {
				return fmt.Errorf("gagal rename %s ke %s: %v", file.old, file.new, err)
			}
		}
	}

	return nil
}

// SanitizeFilename membersihkan karakter yang tidak valid untuk nama file
func SanitizeFilename(filename string) string {
	// Replace karakter yang tidak valid untuk nama file
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	sanitized := filename
	for _, char := range invalidChars {
		sanitized = strings.ReplaceAll(sanitized, char, "_")
	}
	return sanitized
}

// GenerateDBName menghasilkan nama database berdasarkan format yang diminta
// Format baru: DB USER TELEGRAM/{telegramID}/whatsmeow-{userid}-{nomorwhatsapp}.db
func GenerateDBName(telegramID int64, whatsappNumber string, dbType string) string {
	// Bersihkan nomor WhatsApp dari karakter non-digit
	cleanNumber := strings.ReplaceAll(whatsappNumber, "+", "")
	cleanNumber = strings.ReplaceAll(cleanNumber, "-", "")
	cleanNumber = strings.ReplaceAll(cleanNumber, " ", "")

	// Pastikan folder database untuk user sudah ada
	if err := EnsureUserDBFolder(telegramID); err != nil {
		GetLogger().Warn("Failed to ensure user DB folder: %v", err)
	}

	// Path database dalam folder user
	userFolder := GetUserDBFolder(telegramID)
	var dbName string
	if dbType == "bot_data" {
		dbName = fmt.Sprintf("bot_data-%d-%s.db", telegramID, cleanNumber)
	} else {
		dbName = fmt.Sprintf("whatsmeow-%d-%s.db", telegramID, cleanNumber)
	}

	return filepath.Join(userFolder, dbName)
}

// FindExistingDatabases mencari database yang sudah ada dengan format baru
// Format baru: DB USER TELEGRAM/{telegramID}/whatsmeow-{userid}-{nomorwhatsapp}.db
// Juga mencari di root directory untuk backward compatibility
func FindExistingDatabases() (string, string, error) {
	// Cek format baru terlebih dahulu (di folder user)
	patternNew := `^whatsmeow-(\d+)-(\d+)\.db$`
	regexNew, err := regexp.Compile(patternNew)
	if err != nil {
		return "", "", err
	}

	// Cari di folder user terlebih dahulu
	userFolders, err := filepath.Glob("DB USER TELEGRAM/*")
	if err == nil {
		for _, userFolder := range userFolders {
			// Cek apakah ini folder (bukan file)
			if info, err := os.Stat(userFolder); err != nil || !info.IsDir() {
				continue
			}

			// Cari whatsmeow database di folder user ini
			filesNew, err := filepath.Glob(filepath.Join(userFolder, "whatsmeow-*.db"))
			if err == nil && len(filesNew) > 0 {
				// Ambil file pertama yang match format baru
				for _, dbFile := range filesNew {
					matches := regexNew.FindStringSubmatch(filepath.Base(dbFile))
					if len(matches) == 3 {
						telegramID := matches[1]
						whatsappNumber := matches[2]

						whatsappDB := dbFile
						botDataDB := filepath.Join(userFolder, fmt.Sprintf("bot_data-%s-%s.db", telegramID, whatsappNumber))

						// Cek apakah bot_data.db juga ada
						if _, err := os.Stat(botDataDB); os.IsNotExist(err) {
							// Coba cari bot_data dengan pattern yang sama
							botFiles, _ := filepath.Glob(filepath.Join(userFolder, fmt.Sprintf("bot_data-%s-%s.db", telegramID, whatsappNumber)))
							if len(botFiles) > 0 {
								botDataDB = botFiles[0]
							} else {
								botDataDB = filepath.Join(userFolder, fmt.Sprintf("bot_data-%s-%s.db", telegramID, whatsappNumber))
							}
						}

						return whatsappDB, botDataDB, nil
					}
				}
			}
		}
	}

	// Fallback: cari di root directory (untuk backward compatibility)
	filesNew, err := filepath.Glob("whatsmeow-*.db")
	if err == nil && len(filesNew) > 0 {
		// Ambil file pertama yang match format baru
		for _, dbFile := range filesNew {
			// Skip jika file sudah di dalam folder user
			if filepath.Dir(dbFile) != "." {
				continue
			}

			matches := regexNew.FindStringSubmatch(filepath.Base(dbFile))
			if len(matches) == 3 {
				telegramID := matches[1]
				whatsappNumber := matches[2]

				whatsappDB := dbFile
				botDataDB := fmt.Sprintf("bot_data-%s-%s.db", telegramID, whatsappNumber)

				// Cek apakah bot_data.db juga ada
				if _, err := os.Stat(botDataDB); os.IsNotExist(err) {
					// Coba cari bot_data dengan pattern yang sama
					botFiles, _ := filepath.Glob(fmt.Sprintf("bot_data-%s-%s.db", telegramID, whatsappNumber))
					if len(botFiles) > 0 {
						botDataDB = botFiles[0]
					} else {
						botDataDB = fmt.Sprintf("bot_data-%s-%s.db", telegramID, whatsappNumber)
					}
				}

				return whatsappDB, botDataDB, nil
			}
		}
	}

	// DEPRECATED: Fallback format lama sudah tidak didukung
	// Semua database harus sudah di-migrate ke format baru: whatsmeow-{userid}-{phone}.db
	return "", "", fmt.Errorf("database tidak ditemukan")
}

// ResetDBConfig mereset konfigurasi database (untuk logout)
func ResetDBConfig() {
	dbConfig = nil
}

// LoadDBConfigFromFile mencoba load konfigurasi database dari file yang ada
// Hanya support format baru: whatsmeow-{userid}-{phone}.db
func LoadDBConfigFromFile() bool {
	whatsappDB, _, err := FindExistingDatabases()
	if err == nil {
		// Hanya support format baru
		patternNew := `whatsmeow-(\d+)-(\d+)\.db$`
		regexNew, err := regexp.Compile(patternNew)
		if err == nil {
			matches := regexNew.FindStringSubmatch(whatsappDB)
			if len(matches) == 3 {
				var telegramID int64
				fmt.Sscanf(matches[1], "%d", &telegramID)
				whatsappNumber := matches[2]

				SetDBConfig(telegramID, whatsappNumber)
				return true
			}
		}
	}
	return false
}
