package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

const (
	MaxAccounts = 50 // Maksimal akun WhatsApp yang bisa login
)

// WhatsAppAccount menyimpan info akun WhatsApp
type WhatsAppAccount struct {
	ID            int
	PhoneNumber   string
	DBPath        string
	BotDataDBPath string
	Status        string // "active", "inactive"
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AccountManager mengelola multiple WhatsApp accounts
type AccountManager struct {
	accounts    map[int]*WhatsAppAccount  // Map account ID -> Account info
	clients     map[int]*whatsmeow.Client // Map account ID -> WhatsApp client
	currentID   int                       // Current active account ID
	mutex       sync.RWMutex
	telegramBot *tgbotapi.BotAPI
}

var accountManager *AccountManager
var accountManagerOnce sync.Once

// DeleteConfirmState menyimpan state konfirmasi delete
type DeleteConfirmState struct {
	AccountID   int
	PhoneNumber string
	MessageID   int
}

var deleteConfirmStates = make(map[int64]*DeleteConfirmState)
var deleteConfirmMutex sync.Mutex

// GetAccountManager mendapatkan instance AccountManager
func GetAccountManager() *AccountManager {
	accountManagerOnce.Do(func() {
		accountManager = &AccountManager{
			accounts:  make(map[int]*WhatsAppAccount),
			clients:   make(map[int]*whatsmeow.Client),
			currentID: -1,
		}
	})
	return accountManager
}

// SetTelegramBot mengatur Telegram bot untuk AccountManager
func (am *AccountManager) SetTelegramBot(bot *tgbotapi.BotAPI) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.telegramBot = bot
}

// getMasterBotDB mendapatkan database master untuk menyimpan info akun multi-account
// Selalu menggunakan bot_data.db default, bukan database dinamis
func getMasterBotDB() (*sql.DB, error) {
	// Selalu gunakan bot_data.db sebagai database master untuk multi-account
	// Gunakan DELETE mode untuk menghilangkan -shm dan -wal files
	dbPath := "bot_data.db"
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=DELETE&_cache=shared")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// InitAccountDB menginisialisasi database untuk menyimpan info akun
func InitAccountDB() error {
	db, err := getMasterBotDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Create accounts table
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

	return err
}

// LoadAccounts memuat semua akun dari database
func (am *AccountManager) LoadAccounts() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Reset accounts map terlebih dahulu untuk memastikan data fresh
	am.accounts = make(map[int]*WhatsAppAccount)
	am.currentID = -1

	// IMPORTANT: Migrasikan akun dari database dinamis ke master terlebih dahulu
	if err := migrateAccountsFromDynamicDB(); err != nil {
		fmt.Printf("⚠️ Warning: Failed to migrate accounts: %v\n", err)
		// Continue anyway, mungkin sudah di master
	}

	// Gunakan database master untuk memuat semua akun
	db, err := getMasterBotDB()
	if err != nil {
		return fmt.Errorf("failed to get master DB: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, phone_number, db_path, bot_data_db_path, status, created_at, updated_at
		FROM whatsapp_accounts
		ORDER BY created_at ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	loadedCount := 0
	for rows.Next() {
		var account WhatsAppAccount
		var createdAt, updatedAt string

		err := rows.Scan(
			&account.ID,
			&account.PhoneNumber,
			&account.DBPath,
			&account.BotDataDBPath,
			&account.Status,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			continue // Skip invalid rows
		}

		account.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		account.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		am.accounts[account.ID] = &account
		loadedCount++

		// Set current ID ke akun pertama yang aktif (jika belum ada current)
		if am.currentID == -1 && account.Status == "active" {
			am.currentID = account.ID
		}
	}

	fmt.Printf("✅ Loaded %d accounts from database master\n", loadedCount)
	return nil
}

// GetAccountCount mendapatkan jumlah akun yang sudah terdaftar (SEMUA USER)
// DEPRECATED: Gunakan GetAccountCountByTelegramID() untuk menghitung akun per user
func (am *AccountManager) GetAccountCount() int {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return len(am.accounts)
}

// GetAccountCountByTelegramID mendapatkan jumlah akun untuk user tertentu
// SECURITY: Filter by TelegramID untuk isolasi data per user
func (am *AccountManager) GetAccountCountByTelegramID(telegramID int64) int {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	count := 0
	reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
	reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)

	for _, acc := range am.accounts {
		accountTelegramID := int64(0)
		matchesNew := reNew.FindStringSubmatch(acc.BotDataDBPath)
		if len(matchesNew) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
				accountTelegramID = parsedID
			}
		} else {
			matchesOld := reOld.FindStringSubmatch(acc.BotDataDBPath)
			if len(matchesOld) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
					accountTelegramID = parsedID
				}
			}
		}

		// Hanya hitung akun milik user yang memanggil
		if accountTelegramID == telegramID {
			count++
		}
	}

	return count
}

// findAvailableID mencari ID terkecil yang tidak digunakan (slot kosong)
func findAvailableID(db *sql.DB) (int, error) {
	// Query semua ID yang ada, diurutkan dari kecil ke besar
	rows, err := db.Query(`SELECT id FROM whatsapp_accounts ORDER BY id ASC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	existingIDs := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			continue
		}
		existingIDs = append(existingIDs, id)
	}

	// Jika tidak ada ID yang ada, mulai dari 1
	if len(existingIDs) == 0 {
		return 1, nil
	}

	// Cari gap terkecil
	// Jika ID pertama > 1, maka gap adalah 1
	if existingIDs[0] > 1 {
		return 1, nil
	}

	// Cari gap di antara ID yang ada
	for i := 0; i < len(existingIDs)-1; i++ {
		if existingIDs[i+1]-existingIDs[i] > 1 {
			// Ditemukan gap
			return existingIDs[i] + 1, nil
		}
	}

	// Tidak ada gap, gunakan ID terbesar + 1
	return existingIDs[len(existingIDs)-1] + 1, nil
}

// AddAccount menambahkan akun baru dengan validasi ownership
// SECURITY: Validasi bahwa dbPath dan botDataDBPath mengandung TelegramID yang sesuai
// Akun baru akan menggunakan slot ID yang kosong (reuse ID dari akun yang sudah dihapus)
func (am *AccountManager) AddAccount(phoneNumber, dbPath, botDataDBPath string, telegramID int64) (*WhatsAppAccount, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Cek limit
	if len(am.accounts) >= MaxAccounts {
		return nil, fmt.Errorf("maksimal %d akun WhatsApp telah tercapai", MaxAccounts)
	}

	// ✅ AMAN: Validasi bahwa dbPath dan botDataDBPath mengandung TelegramID yang sesuai
	// Parse TelegramID dari dbPath
	reNew := regexp.MustCompile(`whatsmeow-(\d+)-(\d+)\.db`)
	reOld := regexp.MustCompile(`whatsmeow\((\d+)\)>`)

	dbPathTelegramID := int64(0)
	matchesNew := reNew.FindStringSubmatch(dbPath)
	if len(matchesNew) >= 2 {
		if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
			dbPathTelegramID = parsedID
		}
	} else {
		matchesOld := reOld.FindStringSubmatch(dbPath)
		if len(matchesOld) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
				dbPathTelegramID = parsedID
			}
		}
	}

	// Parse TelegramID dari botDataDBPath
	reNewBotData := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
	reOldBotData := regexp.MustCompile(`bot_data\((\d+)\)>`)

	botDataDBPathTelegramID := int64(0)
	matchesNewBotData := reNewBotData.FindStringSubmatch(botDataDBPath)
	if len(matchesNewBotData) >= 2 {
		if parsedID, err := strconv.ParseInt(matchesNewBotData[1], 10, 64); err == nil {
			botDataDBPathTelegramID = parsedID
		}
	} else {
		matchesOldBotData := reOldBotData.FindStringSubmatch(botDataDBPath)
		if len(matchesOldBotData) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesOldBotData[1], 10, 64); err == nil {
				botDataDBPathTelegramID = parsedID
			}
		}
	}

	// Validasi ownership
	if dbPathTelegramID != 0 && dbPathTelegramID != telegramID {
		utils.GetLogger().Warn("Security: AddAccount - dbPath TelegramID mismatch: expected %d, got %d", telegramID, dbPathTelegramID)
		return nil, fmt.Errorf("akses ditolak: dbPath tidak sesuai dengan TelegramID")
	}
	if botDataDBPathTelegramID != 0 && botDataDBPathTelegramID != telegramID {
		utils.GetLogger().Warn("Security: AddAccount - botDataDBPath TelegramID mismatch: expected %d, got %d", telegramID, botDataDBPathTelegramID)
		return nil, fmt.Errorf("akses ditolak: botDataDBPath tidak sesuai dengan TelegramID")
	}

	// ✅ AMAN: Cek apakah nomor sudah ada (hanya untuk user yang sama)
	for _, acc := range am.accounts {
		if acc.PhoneNumber == phoneNumber {
			// Cek apakah akun ini milik user yang sama
			accTelegramID := int64(0)
			reNewAcc := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
			reOldAcc := regexp.MustCompile(`bot_data\((\d+)\)>`)

			matchesNewAcc := reNewAcc.FindStringSubmatch(acc.BotDataDBPath)
			if len(matchesNewAcc) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesNewAcc[1], 10, 64); err == nil {
					accTelegramID = parsedID
				}
			} else {
				matchesOldAcc := reOldAcc.FindStringSubmatch(acc.BotDataDBPath)
				if len(matchesOldAcc) >= 2 {
					if parsedID, err := strconv.ParseInt(matchesOldAcc[1], 10, 64); err == nil {
						accTelegramID = parsedID
					}
				}
			}

			// Jika nomor sudah terdaftar untuk user yang sama, tolak
			if accTelegramID == telegramID {
				return nil, fmt.Errorf("nomor %s sudah terdaftar untuk akun Anda", phoneNumber)
			}
			// Jika milik user lain, izinkan (untuk re-login atau nomor yang sama digunakan user berbeda)
		}
	}

	// Gunakan database master untuk menyimpan akun
	db, err := getMasterBotDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Cari ID yang tersedia (slot kosong)
	availableID, err := findAvailableID(db)
	if err != nil {
		return nil, fmt.Errorf("gagal mencari ID tersedia: %w", err)
	}

	var accountID int64

	// Cek ID maksimum yang ada di database
	var maxID sql.NullInt64
	err = db.QueryRow(`SELECT MAX(id) FROM whatsapp_accounts`).Scan(&maxID)
	if err != nil {
		return nil, fmt.Errorf("gagal query max ID: %w", err)
	}

	// Jika availableID <= maxID, berarti ada slot kosong (gap), gunakan ID eksplisit
	// Jika tidak ada gap, availableID akan sama dengan maxID + 1, gunakan auto-increment
	if maxID.Valid && availableID <= int(maxID.Int64) {
		// Ada slot kosong, gunakan ID eksplisit untuk reuse
		_, err := db.Exec(`
			INSERT INTO whatsapp_accounts (id, phone_number, db_path, bot_data_db_path, status)
			VALUES (?, ?, ?, ?, 'active')
		`, availableID, phoneNumber, dbPath, botDataDBPath)
		if err != nil {
			return nil, err
		}

		// Update sqlite_sequence untuk memastikan auto-increment tetap sinkron
		// Set sequence ke max(id) yang ada di database
		// Gunakan INSERT OR REPLACE untuk menangani kasus sequence belum ada
		_, _ = db.Exec(`
			INSERT OR REPLACE INTO sqlite_sequence (name, seq)
			SELECT 'whatsapp_accounts', MAX(id) FROM whatsapp_accounts
		`)

		accountID = int64(availableID)
		utils.GetLogger().Info("AddAccount: ✅ Menggunakan slot ID kosong: %d untuk nomor %s (reuse slot)", availableID, phoneNumber)
	} else {
		// Tidak ada slot kosong, gunakan auto-increment
		result, err := db.Exec(`
			INSERT INTO whatsapp_accounts (phone_number, db_path, bot_data_db_path, status)
			VALUES (?, ?, ?, 'active')
		`, phoneNumber, dbPath, botDataDBPath)
		if err != nil {
			return nil, err
		}

		accountID, _ = result.LastInsertId()
		utils.GetLogger().Info("AddAccount: ✅ Menggunakan auto-increment ID: %d untuk nomor %s", accountID, phoneNumber)
	}

	account := &WhatsAppAccount{
		ID:            int(accountID),
		PhoneNumber:   phoneNumber,
		DBPath:        dbPath,
		BotDataDBPath: botDataDBPath,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	am.accounts[account.ID] = account

	// Set sebagai current jika ini akun pertama
	if am.currentID == -1 {
		am.currentID = account.ID
	}

	return account, nil
}

// UpdateAccountStatus memperbarui status akun di database secara real-time
func (am *AccountManager) UpdateAccountStatus(accountID int, status string) error {
	am.mutex.RLock()
	account, exists := am.accounts[accountID]
	am.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// Update di database master
	db, err := getMasterBotDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		UPDATE whatsapp_accounts 
		SET status = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, status, accountID)

	if err != nil {
		return fmt.Errorf("gagal update status di database: %w", err)
	}

	// Update di memory
	am.mutex.Lock()
	account.Status = status
	account.UpdatedAt = time.Now()
	am.mutex.Unlock()

	utils.GetLogger().Info("Multi-account: Updated status for account %d (%s) to %s", accountID, account.PhoneNumber, status)
	return nil
}

// UpdateAccountPaths memperbarui db_path dan bot_data_db_path di database jika format berubah
func (am *AccountManager) UpdateAccountPaths(accountID int, dbPath, botDataDBPath string) error {
	am.mutex.RLock()
	account, exists := am.accounts[accountID]
	am.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// Update di database master
	db, err := getMasterBotDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		UPDATE whatsapp_accounts 
		SET db_path = ?, bot_data_db_path = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, dbPath, botDataDBPath, accountID)

	if err != nil {
		return fmt.Errorf("gagal update paths di database: %w", err)
	}

	// Update di memory
	am.mutex.Lock()
	account.DBPath = dbPath
	account.BotDataDBPath = botDataDBPath
	account.UpdatedAt = time.Now()
	am.mutex.Unlock()

	utils.GetLogger().Info("Multi-account: Updated paths for account %d (%s): DBPath=%s, BotDataDBPath=%s",
		accountID, account.PhoneNumber, dbPath, botDataDBPath)
	return nil
}

// SyncAccountStatus memastikan status di database sesuai dengan status koneksi aktual
func (am *AccountManager) SyncAccountStatus(accountID int) error {
	am.mutex.RLock()
	account, exists := am.accounts[accountID]
	client := am.clients[accountID]
	am.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// Cek status koneksi aktual
	var actualStatus string
	if client != nil && client.IsConnected() && client.Store.ID != nil {
		actualStatus = "active"
	} else {
		actualStatus = "inactive"
	}

	// Update jika berbeda
	if account.Status != actualStatus {
		return am.UpdateAccountStatus(accountID, actualStatus)
	}

	return nil
}

// SyncAllAccountsStatus melakukan sync status semua akun secara berkala
func (am *AccountManager) SyncAllAccountsStatus() {
	am.mutex.RLock()
	accountIDs := make([]int, 0, len(am.accounts))
	for id := range am.accounts {
		accountIDs = append(accountIDs, id)
	}
	am.mutex.RUnlock()

	for _, id := range accountIDs {
		// Sync setiap akun
		if err := am.SyncAccountStatus(id); err != nil {
			utils.GetLogger().Warn("Multi-account: Failed to sync status for account %d: %v", id, err)
		}
	}
}

// GetAccount mendapatkan info akun berdasarkan ID
func (am *AccountManager) GetAccount(id int) *WhatsAppAccount {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return am.accounts[id]
}

// GetAllAccounts mendapatkan semua akun
func (am *AccountManager) GetAllAccounts() []*WhatsAppAccount {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	accounts := make([]*WhatsAppAccount, 0, len(am.accounts))
	for _, acc := range am.accounts {
		accounts = append(accounts, acc)
	}

	// Sort by ID
	for i := 0; i < len(accounts)-1; i++ {
		for j := i + 1; j < len(accounts); j++ {
			if accounts[i].ID > accounts[j].ID {
				accounts[i], accounts[j] = accounts[j], accounts[i]
			}
		}
	}

	return accounts
}

// GetCurrentAccount mendapatkan akun yang sedang aktif
func (am *AccountManager) GetCurrentAccount() *WhatsAppAccount {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	if am.currentID == -1 {
		return nil
	}
	return am.accounts[am.currentID]
}

// GetAccountByTelegramID mendapatkan akun berdasarkan Telegram ID
// Mencari akun yang memiliki BotDataDBPath dengan format:
// - Format lama: bot_data(telegramID)>(phoneNumber).db
// - Format baru: DB USER TELEGRAM/{telegramID}/bot_data-{telegramID}-{phoneNumber}.db
func (am *AccountManager) GetAccountByTelegramID(telegramID int64) *WhatsAppAccount {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	// Pattern untuk parse BotDataDBPath:
	// Format lama: bot_data(telegramID)>(phoneNumber).db
	// Format baru: bot_data-{telegramID}-{phoneNumber}.db (dengan atau tanpa folder)
	reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)
	reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)

	for _, account := range am.accounts {
		if account.BotDataDBPath != "" {
			// Cek format baru terlebih dahulu (dengan atau tanpa folder user)
			matchesNew := reNew.FindStringSubmatch(account.BotDataDBPath)
			if len(matchesNew) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
					if parsedID == telegramID {
						return account
					}
				}
			}

			// Fallback: cek format lama
			matchesOld := reOld.FindStringSubmatch(account.BotDataDBPath)
			if len(matchesOld) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
					if parsedID == telegramID {
						return account
					}
				}
			}
		}
	}

	return nil
}

// EnsureUserAccountActive memastikan akun user aktif berdasarkan Telegram ID
// Fungsi ini akan otomatis switch ke akun user jika belum aktif
// Return: account yang aktif (bisa nil jika user belum punya akun), error jika ada
func EnsureUserAccountActive(telegramID int64, telegramBot *tgbotapi.BotAPI) (*WhatsAppAccount, error) {
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(telegramID)

	if userAccount == nil {
		// User belum memiliki akun terdaftar
		return nil, nil
	}

	// Cek apakah akun user sudah aktif
	currentAccount := am.GetCurrentAccount()
	if currentAccount != nil && currentAccount.ID == userAccount.ID {
		// Akun user sudah aktif
		return userAccount, nil
	}

	// Switch ke akun user
	if err := SwitchAccount(userAccount.ID, telegramBot, telegramID); err != nil {
		return nil, fmt.Errorf("failed to switch to user account: %w", err)
	}

	utils.GetLogger().Info("Auto-switched to user account: ID=%d, Phone=%s, TelegramID=%d", userAccount.ID, userAccount.PhoneNumber, telegramID)
	return userAccount, nil
}

// GetCurrentClient mendapatkan client WhatsApp yang sedang aktif
func (am *AccountManager) GetCurrentClient() *whatsmeow.Client {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	if am.currentID == -1 {
		return nil
	}
	return am.clients[am.currentID]
}

// GetClient mendapatkan client WhatsApp berdasarkan account ID
// FIXED: Bisa return nil, caller harus handle nil check
func (am *AccountManager) GetClient(accountID int) *whatsmeow.Client {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	client := am.clients[accountID]
	// FIXED: Validasi client sebelum return
	if client != nil && client.Store != nil && client.Store.ID != nil {
		// FIXED: Update waktu terakhir account digunakan untuk tracking aktivitas
		UpdateAccountLastUsed(accountID)
		return client
	}
	// Return nil jika client tidak valid
	return nil
}

// SetCurrentAccount mengatur akun yang sedang aktif
// FIXED: Validasi status account sebelum set current
func (am *AccountManager) SetCurrentAccount(id int) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	account, exists := am.accounts[id]
	if !exists {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", id)
	}

	// FIXED: Validasi status account sebelum set current
	// Allow inactive account untuk backward compatibility, tapi log warning
	if account.Status != "active" {
		utils.GetLogger().Warn("SetCurrentAccount: Setting inactive account %d (%s) as current", id, account.PhoneNumber)
	}

	am.currentID = id

	// FIXED: Update waktu terakhir account digunakan untuk tracking aktivitas
	UpdateAccountLastUsed(id)

	return nil
}

// CleanupOrphanedDBFiles menghapus file database yang tidak terdaftar di database master
// Return jumlah file yang dihapus
// Fungsi ini dipanggil saat startup untuk membersihkan file database yang tidak terdaftar
// LOGIKA DETEKSI:
// 1. Ambil semua account dari database master (data terbaru setelah validasi dan reload)
// 2. Buat map semua file database yang terdaftar (dari DBPath dan BotDataDBPath)
// 3. Scan semua file bot_data-*.db dan whatsmeow-*.db di filesystem
// 4. Bandingkan: jika file ada di filesystem TAPI tidak ada di map terdaftar = orphaned
// 5. Hapus file orphaned beserta file pendukungnya (-shm, -wal)
func CleanupOrphanedDBFiles(am *AccountManager) int {
	// Gunakan fmt.Printf untuk memastikan log muncul di terminal (logger mungkin belum setup)
	fmt.Printf("[CLEANUP] Memulai scan untuk orphaned database files...\n")
	utils.GetLogger().Info("CleanupOrphanedDBFiles: Memulai scan untuk orphaned database files...")

	// IMPORTANT: Ambil semua account dari database master (data terbaru setelah validasi dan reload)
	// Pastikan menggunakan data terbaru dengan memanggil GetAllAccounts() yang membaca dari map
	allAccounts := am.GetAllAccounts()
	fmt.Printf("[CLEANUP] Ditemukan %d accounts terdaftar di database master\n", len(allAccounts))
	utils.GetLogger().Info("CleanupOrphanedDBFiles: Ditemukan %d accounts terdaftar di database master", len(allAccounts))

	// Buat map untuk menyimpan semua file database yang terdaftar (dari database master)
	registeredBotDataPaths := make(map[string]bool)
	registeredWhatsAppPaths := make(map[string]bool)

	// Counter untuk file utama (tidak termasuk -shm dan -wal)
	mainBotDataCount := 0
	mainWhatsAppCount := 0

	// Register semua file database yang terdaftar dari accounts
	for _, acc := range allAccounts {
		if acc.BotDataDBPath != "" {
			registeredBotDataPaths[acc.BotDataDBPath] = true
			// Juga register file pendukung (-shm, -wal) untuk mencegah penghapusan
			registeredBotDataPaths[acc.BotDataDBPath+"-shm"] = true
			registeredBotDataPaths[acc.BotDataDBPath+"-wal"] = true
			mainBotDataCount++
			fmt.Printf("[CLEANUP] 🔍 Registered bot_data file: %s (account: %s)\n", acc.BotDataDBPath, acc.PhoneNumber)
			utils.GetLogger().Debug("CleanupOrphanedDBFiles: Registered bot_data file: %s", acc.BotDataDBPath)
		}
		if acc.DBPath != "" {
			registeredWhatsAppPaths[acc.DBPath] = true
			// Juga register file pendukung (-shm, -wal) untuk mencegah penghapusan
			registeredWhatsAppPaths[acc.DBPath+"-shm"] = true
			registeredWhatsAppPaths[acc.DBPath+"-wal"] = true
			mainWhatsAppCount++
			fmt.Printf("[CLEANUP] 🔍 Registered whatsapp file: %s (account: %s)\n", acc.DBPath, acc.PhoneNumber)
			utils.GetLogger().Debug("CleanupOrphanedDBFiles: Registered whatsapp file: %s", acc.DBPath)
		}
	}

	fmt.Printf("[CLEANUP] Total %d bot_data files dan %d whatsapp files terdaftar (dari %d account)\n",
		mainBotDataCount, mainWhatsAppCount, len(allAccounts))
	utils.GetLogger().Info("CleanupOrphanedDBFiles: Total %d bot_data files dan %d whatsapp files terdaftar (dari %d account)",
		mainBotDataCount, mainWhatsAppCount, len(allAccounts))

	// Scan semua file bot_data-*.db dan whatsmeow-*.db di filesystem
	botDataPattern := "bot_data-*.db"
	whatsappPattern := "whatsmeow-*.db"

	botDataFiles, err := filepath.Glob(botDataPattern)
	if err != nil {
		utils.GetLogger().Warn("CleanupOrphanedDBFiles: Gagal scan bot_data files: %v", err)
		botDataFiles = []string{}
	}

	whatsappFiles, err := filepath.Glob(whatsappPattern)
	if err != nil {
		utils.GetLogger().Warn("CleanupOrphanedDBFiles: Gagal scan whatsapp files: %v", err)
		whatsappFiles = []string{}
	}

	// Filter bot_data files (exclude master database)
	actualBotDataFiles := []string{}
	for _, file := range botDataFiles {
		if file != "bot_data.db" {
			actualBotDataFiles = append(actualBotDataFiles, file)
		}
	}

	fmt.Printf("[CLEANUP] Ditemukan %d bot_data files dan %d whatsapp files di filesystem (exclude master DB)\n",
		len(actualBotDataFiles), len(whatsappFiles))
	utils.GetLogger().Info("CleanupOrphanedDBFiles: Ditemukan %d bot_data files dan %d whatsapp files di filesystem (exclude master DB)",
		len(actualBotDataFiles), len(whatsappFiles))

	orphanedFiles := []string{}
	deletedCount := 0

	// Cek bot_data files (sudah di-filter untuk exclude master DB)
	for _, file := range actualBotDataFiles {

		// Skip jika terdaftar di database master
		if registeredBotDataPaths[file] {
			fmt.Printf("[CLEANUP] 🔍 File terdaftar (skip): %s\n", file)
			utils.GetLogger().Debug("CleanupOrphanedDBFiles: File terdaftar (skip): %s", file)
			continue
		}

		// File tidak terdaftar = orphaned (harus dihapus)
		fmt.Printf("[CLEANUP] ⚠️ Ditemukan orphaned file: %s\n", file)
		utils.GetLogger().Info("CleanupOrphanedDBFiles: ⚠️ Ditemukan orphaned file: %s", file)
		orphanedFiles = append(orphanedFiles, file)

		// Hapus file dan file pendukungnya (-shm, -wal)
		dbFiles := []string{file, file + "-shm", file + "-wal"}
		for _, dbFile := range dbFiles {
			if _, err := os.Stat(dbFile); err == nil {
				if err := os.Remove(dbFile); err == nil {
					deletedCount++
					fmt.Printf("[CLEANUP] ✅ Berhasil hapus orphaned file: %s\n", dbFile)
					utils.GetLogger().Info("cleanupOrphanedDBFiles: Berhasil hapus orphaned file: %s", dbFile)
				} else {
					fmt.Printf("[CLEANUP] ❌ Gagal hapus orphaned file: %s (error: %v)\n", dbFile, err)
					utils.GetLogger().Warn("cleanupOrphanedDBFiles: Gagal hapus orphaned file: %s (error: %v)", dbFile, err)
				}
			}
		}
	}

	// Cek whatsmeow files
	for _, file := range whatsappFiles {
		// Skip jika terdaftar di database master
		if registeredWhatsAppPaths[file] {
			fmt.Printf("[CLEANUP] 🔍 File terdaftar (skip): %s\n", file)
			utils.GetLogger().Debug("CleanupOrphanedDBFiles: File terdaftar (skip): %s", file)
			continue
		}

		// File tidak terdaftar = orphaned (harus dihapus)
		fmt.Printf("[CLEANUP] ⚠️ Ditemukan orphaned file: %s\n", file)
		utils.GetLogger().Info("CleanupOrphanedDBFiles: ⚠️ Ditemukan orphaned file: %s", file)
		orphanedFiles = append(orphanedFiles, file)

		// Hapus file dan file pendukungnya (-shm, -wal)
		dbFiles := []string{file, file + "-shm", file + "-wal"}
		for _, dbFile := range dbFiles {
			if _, err := os.Stat(dbFile); err == nil {
				if err := os.Remove(dbFile); err == nil {
					deletedCount++
					fmt.Printf("[CLEANUP] ✅ Berhasil hapus orphaned file: %s\n", dbFile)
					utils.GetLogger().Info("cleanupOrphanedDBFiles: Berhasil hapus orphaned file: %s", dbFile)
				} else {
					fmt.Printf("[CLEANUP] ❌ Gagal hapus orphaned file: %s (error: %v)\n", dbFile, err)
					utils.GetLogger().Warn("cleanupOrphanedDBFiles: Gagal hapus orphaned file: %s (error: %v)", dbFile, err)
				}
			}
		}
	}

	if len(orphanedFiles) > 0 {
		fmt.Printf("[CLEANUP] ✅ Menghapus %d orphaned database files (%d files total termasuk -shm, -wal)\n", len(orphanedFiles), deletedCount)
		fmt.Printf("[CLEANUP] File yang dihapus: %v\n", orphanedFiles)
		utils.GetLogger().Info("CleanupOrphanedDBFiles: ✅ Menghapus %d orphaned database files (%d files total termasuk -shm, -wal)", len(orphanedFiles), deletedCount)
		utils.GetLogger().Info("CleanupOrphanedDBFiles: File yang dihapus: %v", orphanedFiles)
		return deletedCount
	} else {
		fmt.Printf("[CLEANUP] ✅ Tidak ada orphaned database files ditemukan - semua file terdaftar dengan benar\n")
		utils.GetLogger().Info("CleanupOrphanedDBFiles: ✅ Tidak ada orphaned database files ditemukan - semua file terdaftar dengan benar")
		return 0
	}
}

// ValidateAccount memvalidasi apakah account masih valid (tidak terblokir/logout)
// Return true jika account valid, false jika terblokir/logout, error jika terjadi error lainnya
func (am *AccountManager) ValidateAccount(accountID int) (bool, error) {
	am.mutex.RLock()
	account, exists := am.accounts[accountID]
	am.mutex.RUnlock()

	if !exists {
		return false, fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// Cek apakah file database ada
	if _, err := os.Stat(account.DBPath); os.IsNotExist(err) {
		// CRITICAL FIX: Coba cari di folder user jika file tidak ditemukan di path lama
		// Ini untuk handle kasus setelah migrasi database ke folder user
		pattern := regexp.MustCompile(`whatsmeow-(\d+)-(\d+)\.db$`)
		matches := pattern.FindStringSubmatch(account.DBPath)
		if len(matches) == 3 {
			telegramIDStr := matches[1]
			// Coba cari di folder user
			userFolder := filepath.Join("DB USER TELEGRAM", telegramIDStr)
			expectedPath := filepath.Join(userFolder, filepath.Base(account.DBPath))
			if _, err := os.Stat(expectedPath); err == nil {
				// File ditemukan di folder user, update path di database
				utils.GetLogger().Info("ValidateAccount: File ditemukan di folder user, update path untuk akun %d: %s -> %s", accountID, account.DBPath, expectedPath)
				botDataDBPath := account.BotDataDBPath
				if botDataDBPath != "" {
					// Update bot_data path juga
					botDataDBName := filepath.Base(botDataDBPath)
					botDataDBPath = filepath.Join(userFolder, botDataDBName)
				}
				// Update path di database
				if err := am.UpdateAccountPaths(accountID, expectedPath, botDataDBPath); err != nil {
					utils.GetLogger().Warn("ValidateAccount: Gagal update path untuk akun %d: %v", accountID, err)
				}
				// Gunakan path baru untuk validasi
				account.DBPath = expectedPath
			} else {
				utils.GetLogger().Warn("ValidateAccount: File database tidak ditemukan untuk akun %d: %s (juga tidak ditemukan di %s)", accountID, account.DBPath, expectedPath)
				return false, nil // Account tidak valid karena file database tidak ada
			}
		} else {
			utils.GetLogger().Warn("ValidateAccount: File database tidak ditemukan untuk akun %d: %s", accountID, account.DBPath)
			return false, nil // Account tidak valid karena file database tidak ada
		}
	}

	// Setup WhatsApp database store untuk validasi
	// FIXED: Tambahkan _busy_timeout dan _locking_mode untuk mencegah "database table is locked" saat concurrent access
	dbLog := waLog.Stdout("Database", "ERROR", true)
	dbConnectionString := fmt.Sprintf("file:%s?_foreign_keys=on&mode=rwc&_journal_mode=DELETE&cache=shared&_busy_timeout=10000&_sync=1&_locking_mode=EXCLUSIVE",
		account.DBPath)

	container, err := sqlstore.New(context.Background(), "sqlite3", dbConnectionString, dbLog)
	if err != nil {
		return false, fmt.Errorf("failed to create SQL store: %w", err)
	}
	// FIXED: Container tidak perlu di-close karena sqlstore.New tidak membuka connection yang perlu di-close
	// Container hanya menyimpan config, connection dibuat saat GetFirstDevice dipanggil

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		utils.GetLogger().Warn("ValidateAccount: Failed to get device store untuk akun %d: %v", accountID, err)
		return false, nil // Account tidak valid (mungkin terblokir/logout)
	}

	// Cek apakah deviceStore.ID nil (indikasi account belum pernah login atau sudah logout/terblokir)
	if deviceStore.ID == nil {
		utils.GetLogger().Warn("ValidateAccount: deviceStore.ID nil untuk akun %d - account terblokir/logout", accountID)
		return false, nil // Account tidak valid karena ID nil (terblokir/logout)
	}

	// Account valid
	return true, nil
}

// CreateClient membuat WhatsApp client untuk akun
func (am *AccountManager) CreateClient(accountID int) (*whatsmeow.Client, error) {
	am.mutex.RLock()
	account, exists := am.accounts[accountID]
	am.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// Setup WhatsApp database store
	// FIXED: Tambahkan _busy_timeout dan _locking_mode untuk mencegah "database table is locked" saat concurrent access
	// Gunakan DELETE mode untuk menghilangkan -shm dan -wal files
	dbLog := waLog.Stdout("Database", "ERROR", true)
	dbConnectionString := fmt.Sprintf("file:%s?_foreign_keys=on&mode=rwc&_journal_mode=DELETE&cache=shared&_busy_timeout=10000&_sync=1&_locking_mode=EXCLUSIVE",
		account.DBPath)

	container, err := sqlstore.New(context.Background(), "sqlite3", dbConnectionString, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQL store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device store: %w", err)
	}

	// Cek apakah deviceStore.ID nil (indikasi account terblokir/logout)
	if deviceStore.ID == nil {
		// FIXED: REALTIME CLEANUP - Hapus database jika account terblokir/logout saat CreateClient
		utils.GetLogger().Warn("CreateClient: [REALTIME CLEANUP] Account %d terblokir/logout (Store.ID nil), menghapus database", accountID)
		go func(accID int) {
			time.Sleep(500 * time.Millisecond) // Delay kecil untuk memastikan operasi lain selesai
			am := GetAccountManager()
			if err := am.RemoveAccount(accID); err != nil {
				utils.GetLogger().Warn("CreateClient: Gagal hapus account %d: %v", accID, err)
			} else {
				utils.GetLogger().Info("CreateClient: ✅ [REALTIME CLEANUP] Account %d berhasil dihapus karena Store.ID nil", accID)
			}
		}(accountID)
		return nil, fmt.Errorf("account logged out or blocked: device store ID is nil")
	}

	// Create WhatsApp client
	baseLog := waLog.Stdout("Client", "ERROR", true)
	clientLog := &utils.FilteredLogger{Logger: baseLog}
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	// Store client
	am.mutex.Lock()
	am.clients[accountID] = waClient
	am.mutex.Unlock()

	// Connect to WhatsApp
	if err := waClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Wait for connection
	timeout := 15 * time.Second
	checkInterval := 500 * time.Millisecond
	elapsed := time.Duration(0)

	for !waClient.IsConnected() && elapsed < timeout {
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	if !waClient.IsConnected() {
		return nil, fmt.Errorf("connection timeout")
	}

	// Cek apakah Store.ID masih valid setelah connect
	if waClient.Store == nil || waClient.Store.ID == nil {
		return nil, fmt.Errorf("account logged out or blocked: store ID is nil after connection")
	}

	time.Sleep(1 * time.Second) // Additional wait

	// Update status ke active setelah berhasil connect
	if err := am.UpdateAccountStatus(accountID, "active"); err != nil {
		utils.GetLogger().Warn("Failed to update account status after connection: %v", err)
	}

	return waClient, nil
}

// RemoveAccount menghapus akun
func (am *AccountManager) RemoveAccount(id int) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	_, exists := am.accounts[id]
	if !exists {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", id)
	}

	account := am.accounts[id]         // Get account for deletion info
	phoneNumber := account.PhoneNumber // Store for later use

	// PENTING: Simpan paths database SEBELUM menghapus dari map
	dbPath := account.DBPath
	botDataDBPath := account.BotDataDBPath

	// Disconnect dan hapus client jika ada
	if client, hasClient := am.clients[id]; hasClient {
		if client.IsConnected() {
			client.Disconnect()
		}
		delete(am.clients, id)
	}

	// Jika ini current account, set ke akun lain atau -1
	if am.currentID == id {
		// Cari akun aktif lain
		am.currentID = -1
		for accID, acc := range am.accounts {
			if accID != id && acc.Status == "active" {
				am.currentID = accID
				break
			}
		}
	}

	// Hapus dari map
	delete(am.accounts, id)

	// Hapus dari database master
	db, err := getMasterBotDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM whatsapp_accounts WHERE id = ?", id)
	if err != nil {
		return err
	}

	// Hapus file database akun (WhatsApp DB dan Bot Data DB)
	// Paths sudah disimpan di atas sebelum menghapus dari map

	// Tutup connection pool database untuk akun ini terlebih dahulu
	// (jika ada pool untuk account ini)
	utils.CloseDBPools()

	// Hapus file database WhatsApp dan Bot Data beserta file pendukungnya
	dbFiles := []string{
		dbPath,
		dbPath + "-shm",
		dbPath + "-wal",
		botDataDBPath,
		botDataDBPath + "-shm",
		botDataDBPath + "-wal",
	}

	deletedCount := 0
	for _, dbFile := range dbFiles {
		if _, err := os.Stat(dbFile); err == nil {
			if err := os.Remove(dbFile); err == nil {
				deletedCount++
				utils.GetLogger().Info("RemoveAccount: Berhasil hapus file database: %s", dbFile)
			} else {
				utils.GetLogger().Warn("RemoveAccount: Gagal hapus file database: %s (error: %v)", dbFile, err)
			}
		}
	}

	// Log deletion
	utils.GetLogger().Info("RemoveAccount: Akun %d (%s) dihapus, %d file database terhapus", id, phoneNumber, deletedCount)

	return nil
}

// ShowMultiAccountMenu menampilkan menu login WhatsApp baru
// SECURITY: Hanya menampilkan jumlah akun milik user yang memanggil (filter by TelegramID)
func ShowMultiAccountMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	am := GetAccountManager()
	// ✅ AMAN: Hitung akun hanya untuk user yang memanggil (filter by TelegramID)
	accountCount := am.GetAccountCountByTelegramID(chatID)

	menuMsg := fmt.Sprintf(`📱 **LOGIN WHATSAPP BARU**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini memungkinkan Anda untuk login hingga **%d akun WhatsApp** sekaligus.

📊 **Status Akun:**
• **Total Akun:** %d/%d akun
• **Aktif:** Lihat daftar akun

**📋 Fitur:**
• ➕ Login akun WhatsApp baru
• 📋 Lihat daftar semua akun
• 🔄 Ganti akun aktif
• 🗑️ Hapus akun

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Maksimal %d akun WhatsApp
• Setiap akun memiliki database terpisah
• Akun aktif digunakan untuk operasi grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih aksi yang ingin dilakukan...`, MaxAccounts, accountCount, MaxAccounts, MaxAccounts)

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Login Baru", "multi_account_login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Daftar Akun", "multi_account_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Ganti Akun", "multi_account_switch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowMultiAccountMenuEdit menampilkan menu login WhatsApp baru dengan EDIT
// SECURITY: Hanya menampilkan jumlah akun milik user yang memanggil (filter by TelegramID)
func ShowMultiAccountMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	am := GetAccountManager()
	// ✅ AMAN: Hitung akun hanya untuk user yang memanggil (filter by TelegramID)
	accountCount := am.GetAccountCountByTelegramID(chatID)

	menuMsg := fmt.Sprintf(`📱 **LOGIN WHATSAPP BARU**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini memungkinkan Anda untuk login hingga **%d akun WhatsApp** sekaligus.

📊 **Status Akun:**
• **Total Akun:** %d/%d akun
• **Aktif:** Lihat daftar akun

**📋 Fitur:**
• ➕ Login akun WhatsApp baru
• 📋 Lihat daftar semua akun
• 🔄 Ganti akun aktif
• 🗑️ Hapus akun

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Maksimal %d akun WhatsApp
• Setiap akun memiliki database terpisah
• Akun aktif digunakan untuk operasi grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih aksi yang ingin dilakukan...`, MaxAccounts, accountCount, MaxAccounts, MaxAccounts)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Login Baru", "multi_account_login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Daftar Akun", "multi_account_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Ganti Akun", "multi_account_switch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// MultiAccountLoginState menyimpan state untuk login akun baru
type MultiAccountLoginState struct {
	WaitingForPhone bool
	PhoneNumber     string
}

var multiAccountLoginStates = make(map[int64]*MultiAccountLoginState)

// StartMultiAccountLogin memulai proses login akun WhatsApp baru (untuk command baru)
func StartMultiAccountLogin(telegramBot *tgbotapi.BotAPI, chatID int64) {
	am := GetAccountManager()
	// ✅ AMAN: Hitung akun hanya untuk user yang memanggil (filter by TelegramID)
	accountCount := am.GetAccountCountByTelegramID(chatID)

	if accountCount >= MaxAccounts {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **MAKSIMAL AKUN TERCAPAI**\n\nAnda sudah memiliki %d akun WhatsApp (maksimal %d akun).\n\nHapus salah satu akun terlebih dahulu untuk login akun baru.", accountCount, MaxAccounts))
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	// Set state
	multiAccountLoginStates[chatID] = &MultiAccountLoginState{
		WaitingForPhone: true,
	}

	msgText := fmt.Sprintf(`📱 **LOGIN WHATSAPP BARU**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Status:** %d/%d akun terpakai

**📋 Langkah Login:**
1️⃣ Input nomor WhatsApp yang akan di-login
2️⃣ Bot akan generate kode pairing
3️⃣ Masukkan kode di WhatsApp
4️⃣ Akun akan otomatis terdaftar

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Format Nomor:**
• 628123456789
• 6281234567890
• +628123456789

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nomor WhatsApp yang akan di-login...`, accountCount, MaxAccounts)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "multi_account_cancel_login"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// StartMultiAccountLoginEdit memulai proses login akun WhatsApp baru dengan EDIT (no spam!)
func StartMultiAccountLoginEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	am := GetAccountManager()
	// ✅ AMAN: Hitung akun hanya untuk user yang memanggil (filter by TelegramID)
	accountCount := am.GetAccountCountByTelegramID(chatID)

	if accountCount >= MaxAccounts {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("❌ **MAKSIMAL AKUN TERCAPAI**\n\nAnda sudah memiliki %d akun WhatsApp (maksimal %d akun).\n\nHapus salah satu akun terlebih dahulu untuk login akun baru.", accountCount, MaxAccounts))
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	// Set state
	multiAccountLoginStates[chatID] = &MultiAccountLoginState{
		WaitingForPhone: true,
	}

	msgText := fmt.Sprintf(`📱 **LOGIN WHATSAPP BARU**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Status:** %d/%d akun terpakai

**📋 Langkah Login:**
1️⃣ Input nomor WhatsApp yang akan di-login
2️⃣ Bot akan generate kode pairing
3️⃣ Masukkan kode di WhatsApp
4️⃣ Akun akan otomatis terdaftar

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Format Nomor:**
• 628123456789
• 6281234567890
• +628123456789

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nomor WhatsApp yang akan di-login...`, accountCount, MaxAccounts)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "multi_account_cancel_login"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, msgText)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// HandleMultiAccountPhoneInput menangani input nomor untuk login akun baru
func HandleMultiAccountPhoneInput(phoneNumber string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := multiAccountLoginStates[chatID]
	if state == nil || !state.WaitingForPhone {
		return
	}

	// Validasi nomor
	phoneNumber = strings.TrimSpace(phoneNumber)
	phoneNumber = strings.ReplaceAll(phoneNumber, "+", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, " ", "")

	if len(phoneNumber) < 10 || len(phoneNumber) > 15 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nomor tidak valid!\n\nNomor harus 10-15 digit.\n\nContoh: 628123456789")
		telegramBot.Send(errorMsg)
		return
	}

	// ✅ AMAN: Cek apakah nomor sudah terdaftar untuk user yang sama
	am := GetAccountManager()
	allAccounts := am.GetAllAccounts()

	reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
	reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)

	for _, acc := range allAccounts {
		if acc.PhoneNumber == phoneNumber {
			// Parse TelegramID dari BotDataDBPath untuk cek ownership
			accountTelegramID := int64(0)
			matchesNew := reNew.FindStringSubmatch(acc.BotDataDBPath)
			if len(matchesNew) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
					accountTelegramID = parsedID
				}
			} else {
				matchesOld := reOld.FindStringSubmatch(acc.BotDataDBPath)
				if len(matchesOld) >= 2 {
					if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
						accountTelegramID = parsedID
					}
				}
			}

			// Jika nomor sudah terdaftar untuk user yang sama, tolak
			if accountTelegramID == chatID {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Nomor %s sudah terdaftar untuk akun Anda!\n\nGunakan fitur 'Ganti Akun' untuk menggunakan akun ini.", phoneNumber))
				telegramBot.Send(errorMsg)
				delete(multiAccountLoginStates, chatID)
				return
			}
			// Jika milik user lain, izinkan (untuk re-login atau nomor yang sama digunakan user berbeda)
		}
	}

	state.PhoneNumber = phoneNumber
	state.WaitingForPhone = false

	// Generate database paths dengan format baru: whatsmeow-{userid}-{nomorwhatsapp}.db
	// chatID adalah Telegram user ID
	dbPath := utils.GenerateDBName(chatID, phoneNumber, "whatsmeow")
	botDataDBPath := utils.GenerateDBName(chatID, phoneNumber, "bot_data")

	// Start pairing process untuk akun baru
	go processMultiAccountPairing(phoneNumber, dbPath, botDataDBPath, chatID, telegramBot)
}

// processMultiAccountPairing memproses pairing untuk akun baru
func processMultiAccountPairing(phoneNumber, dbPath, botDataDBPath string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	// Setup WhatsApp database store untuk akun baru
	// FIXED: Tambahkan _busy_timeout dan _locking_mode untuk mencegah "database table is locked" saat concurrent access
	// Gunakan DELETE mode untuk menghilangkan -shm dan -wal files
	dbLog := waLog.Stdout("Database", "ERROR", true)
	dbConnectionString := fmt.Sprintf("file:%s?_foreign_keys=on&mode=rwc&_journal_mode=DELETE&cache=shared&_busy_timeout=10000&_sync=1&_locking_mode=EXCLUSIVE", dbPath)

	container, err := sqlstore.New(context.Background(), "sqlite3", dbConnectionString, dbLog)
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal membuat database: %v", err))
		telegramBot.Send(errorMsg)
		delete(multiAccountLoginStates, chatID)
		return
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal mendapatkan device store: %v", err))
		telegramBot.Send(errorMsg)
		delete(multiAccountLoginStates, chatID)
		return
	}

	// Create WhatsApp client untuk akun baru
	baseLog := waLog.Stdout("Client", "ERROR", true)
	clientLog := &utils.FilteredLogger{Logger: baseLog}
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	// Connect to WhatsApp
	if err := waClient.Connect(); err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal connect: %v", err))
		telegramBot.Send(errorMsg)
		delete(multiAccountLoginStates, chatID)
		return
	}

	// Wait for connection dengan retry
	timeout := 15 * time.Second
	checkInterval := 500 * time.Millisecond
	elapsed := time.Duration(0)

	for !waClient.IsConnected() && elapsed < timeout {
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	if !waClient.IsConnected() {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **KONEKSI GAGAL**\n\nKoneksi ke WhatsApp terputus atau timeout.\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Coba lagi dalam beberapa saat\n3. Restart program jika masih bermasalah")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		waClient.Disconnect()
		delete(multiAccountLoginStates, chatID)
		return
	}

	// Additional wait untuk memastikan connection fully established
	time.Sleep(2 * time.Second)

	// Cek koneksi sekali lagi sebelum pairing
	if !waClient.IsConnected() {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **KONEKSI TIDAK STABIL**\n\nKoneksi terputus sebelum pairing.\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Coba lagi")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		waClient.Disconnect()
		delete(multiAccountLoginStates, chatID)
		return
	}

	// Generate pairing code dengan retry mechanism
	var pairingCode string
	var pairErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Timeout lebih lama untuk pairing
		ctxPair, cancelPair := context.WithTimeout(context.Background(), 15*time.Second)

		// Pastikan masih connected sebelum retry
		if !waClient.IsConnected() {
			// Coba reconnect
			if reconnectErr := waClient.Connect(); reconnectErr != nil {
				cancelPair()
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **KONEKSI TERPUTUS**\n\nGagal reconnect: %v\n\nSilakan coba lagi.", reconnectErr))
				errorMsg.ParseMode = "Markdown"
				telegramBot.Send(errorMsg)
				delete(multiAccountLoginStates, chatID)
				return
			}

			// Wait for reconnection
			time.Sleep(2 * time.Second)
		}

		pairingCode, pairErr = waClient.PairPhone(ctxPair, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Windows)")
		cancelPair()

		if pairErr == nil && pairingCode != "" {
			break // Success
		}

		// Log error tapi coba lagi jika belum max retries
		if pairErr != nil {
			errorText := pairErr.Error()
			if attempt < maxRetries && (strings.Contains(errorText, "websocket") ||
				strings.Contains(errorText, "disconnected") ||
				strings.Contains(errorText, "timeout")) {
				// Retry dengan delay
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
		}
	}

	if pairErr != nil {
		// Format error message yang lebih user-friendly
		errorText := pairErr.Error()
		var errorDetail string

		if strings.Contains(errorText, "websocket") || strings.Contains(errorText, "disconnected") {
			errorDetail = "**Masalah:** Koneksi websocket terputus\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Cek firewall atau proxy\n3. Coba lagi dalam beberapa saat\n4. Restart program jika perlu"
		} else if strings.Contains(errorText, "timeout") {
			errorDetail = "**Masalah:** Timeout saat generate pairing code\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Server WhatsApp mungkin sedang sibuk\n3. Coba lagi dalam beberapa saat"
		} else {
			errorDetail = fmt.Sprintf("**Error:** %s\n\n**Solusi:**\n1. Cek koneksi internet\n2. Coba lagi\n3. Restart program jika perlu", pairErr.Error())
		}

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **GAGAL GENERATE PAIRING CODE**\n\n%s\n\n%s", pairErr, errorDetail))
		errorMsg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Coba Lagi", "multi_account_login"),
			),
		)
		errorMsg.ReplyMarkup = keyboard
		telegramBot.Send(errorMsg)

		waClient.Disconnect()
		delete(multiAccountLoginStates, chatID)
		return
	}

	if pairingCode == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Pairing code kosong. Silakan coba lagi.")
		telegramBot.Send(errorMsg)
		delete(multiAccountLoginStates, chatID)
		return
	}

	// Kirim instruksi pairing
	pairingInstructions := ui.FormatPairingInstructions(pairingCode, phoneNumber)
	msg := tgbotapi.NewMessage(chatID, pairingInstructions)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "multi_account_cancel_pairing"),
		),
	)
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)

	// Tunggu pairing (maksimal 2 menit)
	startTime := time.Now()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var progressMessageID int
	pairingDone := false

	for time.Since(startTime) < 120*time.Second && !pairingDone {
		<-ticker.C

		elapsed := int(time.Since(startTime).Seconds())
		remaining := 120 - elapsed
		minutes := remaining / 60
		seconds := remaining % 60

		progressText := fmt.Sprintf(`⏳ **MENUNGGU PAIRING...**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **Nomor:** +%s
⏰ **Countdown:** %02d:%02d
📊 **Status:** Menunggu konfirmasi...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pastikan kode sudah dimasukkan di WhatsApp...`, phoneNumber, minutes, seconds)

		if progressMessageID == 0 {
			progressMsg := tgbotapi.NewMessage(chatID, progressText)
			progressMsg.ParseMode = "Markdown"
			sent, _ := telegramBot.Send(progressMsg)
			progressMessageID = sent.MessageID
		} else {
			editMsg := tgbotapi.NewEditMessageText(chatID, progressMessageID, progressText)
			editMsg.ParseMode = "Markdown"
			telegramBot.Send(editMsg)
		}

		if waClient != nil && waClient.Store != nil && waClient.Store.ID != nil {
			// Pairing berhasil!
			pairingDone = true
			ticker.Stop()

			// Hapus pesan progress
			if progressMessageID > 0 {
				deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMessageID)
				telegramBot.Request(deleteMsg)
			}

			// Pastikan tabel whatsapp_accounts ada sebelum AddAccount
			if err := InitAccountDB(); err != nil {
				utils.GetLogger().Warn("Failed to init account DB before adding account: %v", err)
				// Continue anyway, mungkin sudah ada
			}

			// Tambahkan akun ke database
			am := GetAccountManager()
			am.SetTelegramBot(telegramBot)
			// ✅ AMAN: Pass chatID (TelegramID) untuk validasi ownership
			account, err := am.AddAccount(phoneNumber, dbPath, botDataDBPath, chatID)
			if err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal menyimpan akun: %v", err))
				telegramBot.Send(errorMsg)
				delete(multiAccountLoginStates, chatID)
				return
			}

			// Simpan client
			am.mutex.Lock()
			am.clients[account.ID] = waClient
			am.mutex.Unlock()

			// ✅ AMAN: Set sebagai current jika ini akun pertama untuk user ini
			// Cek jumlah akun hanya untuk user yang memanggil (filter by TelegramID)
			if am.GetAccountCountByTelegramID(chatID) == 1 {
				am.SetCurrentAccount(account.ID)
				SetClients(waClient, telegramBot)
			}

			// Removed auto-fetch groups - user can manually fetch when needed via menu
			// This prevents database access issues during account switching

			successMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf(`✅ **PAIRING BERHASIL!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **Nomor:** +%s
✅ **Status:** Terhubung
🆔 **Account ID:** %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Akun WhatsApp baru telah berhasil di-login!`, phoneNumber, account.ID))
			successMsg.ParseMode = "Markdown"
			telegramBot.Send(successMsg)

			delete(multiAccountLoginStates, chatID)
			return
		}
	}

	// Timeout
	if !pairingDone {
		if progressMessageID > 0 {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMessageID)
			telegramBot.Request(deleteMsg)
		}

		errorMsg := tgbotapi.NewMessage(chatID, "❌ **PAIRING TIMEOUT**\n\nWaktu pairing habis (2 menit).\n\nSilakan coba lagi.")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)

		waClient.Disconnect()
		delete(multiAccountLoginStates, chatID)
	}
}

// ShowAccountList menampilkan daftar semua akun (untuk command baru)
func ShowAccountList(telegramBot *tgbotapi.BotAPI, chatID int64) {
	am := GetAccountManager()

	// IMPORTANT: Simpan currentID sebelum reload untuk mencegah reset!
	savedCurrentID := -1
	if currentAcc := am.GetCurrentAccount(); currentAcc != nil {
		savedCurrentID = currentAcc.ID
	}

	// Reload accounts dari database untuk memastikan data terbaru
	if err := am.LoadAccounts(); err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal memuat daftar akun: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// IMPORTANT: Restore currentID setelah reload jika masih valid!
	if savedCurrentID != -1 {
		if acc := am.GetAccount(savedCurrentID); acc != nil {
			am.SetCurrentAccount(savedCurrentID)
			utils.GetLogger().Info("ShowAccountList: Restored currentID to %d after LoadAccounts", savedCurrentID)
		}
	}

	// ✅ AMAN: Filter akun berdasarkan TelegramID user yang memanggil
	allAccounts := am.GetAllAccounts()
	userAccounts := []*WhatsAppAccount{}

	// Parse TelegramID dari BotDataDBPath untuk setiap akun
	reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
	reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)

	for _, acc := range allAccounts {
		accountTelegramID := int64(0)
		matchesNew := reNew.FindStringSubmatch(acc.BotDataDBPath)
		if len(matchesNew) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
				accountTelegramID = parsedID
			}
		} else {
			matchesOld := reOld.FindStringSubmatch(acc.BotDataDBPath)
			if len(matchesOld) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
					accountTelegramID = parsedID
				}
			}
		}

		// Hanya tambahkan akun milik user yang memanggil
		if accountTelegramID == chatID {
			userAccounts = append(userAccounts, acc)
		}
	}

	currentAccount := am.GetCurrentAccount()

	if len(userAccounts) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📭 **BELUM ADA AKUN**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan 'Login Baru' untuk menambahkan akun pertama.")
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	var currentID int
	if currentAccount != nil {
		// Cek apakah current account milik user
		currentAccountTelegramID := int64(0)
		matchesNew := reNew.FindStringSubmatch(currentAccount.BotDataDBPath)
		if len(matchesNew) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
				currentAccountTelegramID = parsedID
			}
		} else {
			matchesOld := reOld.FindStringSubmatch(currentAccount.BotDataDBPath)
			if len(matchesOld) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
					currentAccountTelegramID = parsedID
				}
			}
		}
		// Hanya set currentID jika current account milik user
		if currentAccountTelegramID == chatID {
			currentID = currentAccount.ID
		}
	}

	listMsg := fmt.Sprintf(`📋 **DAFTAR AKUN WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total:** %d akun

`, len(userAccounts))

	for i, acc := range userAccounts {
		statusIcon := "🔴"
		if acc.Status == "active" {
			statusIcon = "🟢"
		}

		currentMark := ""
		if acc.ID == currentID {
			currentMark = " ⭐ (Aktif)"
		}

		listMsg += fmt.Sprintf("%s **%d. +%s**%s\n", statusIcon, i+1, acc.PhoneNumber, currentMark)
	}

	listMsg += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	listMsg += "💡 Klik tombol untuk mengelola akun..."

	msg := tgbotapi.NewMessage(chatID, listMsg)
	msg.ParseMode = "Markdown"

	// Create buttons for each account
	// Telegram limit: max 8 buttons per row, max 100 buttons total
	var keyboardRows [][]tgbotapi.InlineKeyboardButton
	maxButtonsPerRow := 2 // Gunakan 2 button per row untuk nama nomor yang panjang

	for i, acc := range userAccounts {
		if i >= 50 { // Max 50 akun sesuai konstanta
			break
		}

		buttonText := fmt.Sprintf("%d. +%s", i+1, acc.PhoneNumber)
		if acc.ID == currentID {
			buttonText += " ⭐"
		}

		// Tambahkan button switch dan delete dalam 1 row
		switchBtn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🔄 %s", buttonText), fmt.Sprintf("multi_account_switch_%d", acc.ID))
		deleteBtn := tgbotapi.NewInlineKeyboardButtonData("🗑️", fmt.Sprintf("multi_account_delete_%d", acc.ID))

		rowIndex := i / maxButtonsPerRow
		if rowIndex >= len(keyboardRows) {
			keyboardRows = append(keyboardRows, []tgbotapi.InlineKeyboardButton{})
		}
		keyboardRows[rowIndex] = append(keyboardRows[rowIndex], switchBtn, deleteBtn)
	}

	// Tambahkan tombol kembali
	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "multi_account_menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowAccountListEdit menampilkan daftar semua akun dengan EDIT (no spam!)
func ShowAccountListEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	am := GetAccountManager()

	// IMPORTANT: Simpan currentID sebelum reload untuk mencegah reset!
	savedCurrentID := -1
	if currentAcc := am.GetCurrentAccount(); currentAcc != nil {
		savedCurrentID = currentAcc.ID
	}

	// Reload accounts dari database untuk memastikan data terbaru
	if err := am.LoadAccounts(); err != nil {
		errorMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("❌ Gagal memuat daftar akun: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// IMPORTANT: Restore currentID setelah reload jika masih valid!
	if savedCurrentID != -1 {
		if acc := am.GetAccount(savedCurrentID); acc != nil {
			am.SetCurrentAccount(savedCurrentID)
			utils.GetLogger().Info("ShowAccountListEdit: Restored currentID to %d after LoadAccounts", savedCurrentID)
		}
	}

	// ✅ AMAN: Filter akun berdasarkan TelegramID user yang memanggil
	allAccounts := am.GetAllAccounts()
	userAccounts := []*WhatsAppAccount{}

	// Parse TelegramID dari BotDataDBPath untuk setiap akun
	reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
	reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)

	for _, acc := range allAccounts {
		accountTelegramID := int64(0)
		matchesNew := reNew.FindStringSubmatch(acc.BotDataDBPath)
		if len(matchesNew) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
				accountTelegramID = parsedID
			}
		} else {
			matchesOld := reOld.FindStringSubmatch(acc.BotDataDBPath)
			if len(matchesOld) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
					accountTelegramID = parsedID
				}
			}
		}

		// Hanya tambahkan akun milik user yang memanggil
		if accountTelegramID == chatID {
			userAccounts = append(userAccounts, acc)
		}
	}

	currentAccount := am.GetCurrentAccount()

	if len(userAccounts) == 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "📭 **BELUM ADA AKUN**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan 'Login Baru' untuk menambahkan akun pertama.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	var currentID int
	if currentAccount != nil {
		// Cek apakah current account milik user
		currentAccountTelegramID := int64(0)
		matchesNew := reNew.FindStringSubmatch(currentAccount.BotDataDBPath)
		if len(matchesNew) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
				currentAccountTelegramID = parsedID
			}
		} else {
			matchesOld := reOld.FindStringSubmatch(currentAccount.BotDataDBPath)
			if len(matchesOld) >= 2 {
				if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
					currentAccountTelegramID = parsedID
				}
			}
		}
		// Hanya set currentID jika current account milik user
		if currentAccountTelegramID == chatID {
			currentID = currentAccount.ID
		}
	}

	listMsg := fmt.Sprintf(`📋 **DAFTAR AKUN WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total:** %d akun

`, len(userAccounts))

	for i, acc := range userAccounts {
		statusIcon := "🔴"
		if acc.Status == "active" {
			statusIcon = "🟢"
		}

		currentMark := ""
		if acc.ID == currentID {
			currentMark = " ⭐ (Aktif)"
		}

		listMsg += fmt.Sprintf("%s **%d. +%s**%s\n", statusIcon, i+1, acc.PhoneNumber, currentMark)
	}

	listMsg += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	listMsg += "💡 Klik tombol untuk mengelola akun..."

	// Create buttons for each account
	var keyboardRows [][]tgbotapi.InlineKeyboardButton
	maxButtonsPerRow := 2

	for i, acc := range userAccounts {
		if i >= 50 {
			break
		}

		buttonText := fmt.Sprintf("%d. +%s", i+1, acc.PhoneNumber)
		if acc.ID == currentID {
			buttonText += " ⭐"
		}

		// Tambahkan button switch dan delete dalam 1 row
		switchBtn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🔄 %s", buttonText), fmt.Sprintf("multi_account_switch_%d", acc.ID))
		deleteBtn := tgbotapi.NewInlineKeyboardButtonData("🗑️", fmt.Sprintf("multi_account_delete_%d", acc.ID))

		rowIndex := i / maxButtonsPerRow
		if rowIndex >= len(keyboardRows) {
			keyboardRows = append(keyboardRows, []tgbotapi.InlineKeyboardButton{})
		}
		keyboardRows[rowIndex] = append(keyboardRows[rowIndex], switchBtn, deleteBtn)
	}

	// Tambahkan tombol kembali
	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "multi_account_menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, listMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// SwitchAccount mengganti akun aktif
func SwitchAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
	am := GetAccountManager()

	account := am.GetAccount(accountID)
	if account == nil {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// Set sebagai current
	if err := am.SetCurrentAccount(accountID); err != nil {
		return err
	}

	// Buat client untuk akun ini jika belum ada
	client := am.GetCurrentClient()
	if client == nil {
		var err error
		client, err = am.CreateClient(accountID)
		if err != nil {
			return fmt.Errorf("gagal membuat client: %w", err)
		}
	}

	// Update global client
	SetClients(client, telegramBot)

	// Update dbConfig untuk menggunakan database akun aktif (realtime)
	// Parse Telegram ID dari database path atau gunakan chatID
	if account.BotDataDBPath != "" {
		// Parse Telegram ID dari BotDataDBPath (format: bot_data(telegramID)>(phoneNumber).db)
		telegramID := int64(chatID) // Default: gunakan chatID

		// Coba parse dari BotDataDBPath untuk mendapatkan Telegram ID yang benar
		re := regexp.MustCompile(`bot_data\((\d+)\)>`)
		matches := re.FindStringSubmatch(account.BotDataDBPath)
		if len(matches) >= 2 {
			if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				telegramID = parsedID
			}
		}

		utils.SetDBConfig(telegramID, account.PhoneNumber)

		// Reset database pool agar menggunakan database baru (realtime)
		// IMPORTANT: Close pools SEBELUM update dbConfig selesai
		utils.CloseDBPools()

		// Verifikasi bahwa GetBotDataDBPath() sudah mengembalikan path yang benar
		expectedPath := utils.GetBotDataDBPath()
		utils.GetLogger().Info("SwitchAccount: Updated dbConfig - TelegramID=%d, Phone=%s, ExpectedDBPath=%s, AccountDBPath=%s", telegramID, account.PhoneNumber, expectedPath, account.BotDataDBPath)

		// Force rebuild pool dengan database baru (dengan delay kecil untuk memastikan)
		time.Sleep(100 * time.Millisecond)

		// Force rebuild pool sekarang dengan database yang benar
		_, err := utils.GetBotDBPool()
		if err != nil {
			utils.GetLogger().Error("SwitchAccount: Failed to rebuild database pool: %v", err)
		} else {
			utils.GetLogger().Info("SwitchAccount: Database pool rebuilt successfully with path: %s", utils.GetBotDataDBPath())
		}
	}

	// Success message akan dikirim oleh caller jika perlu
	// Return success agar caller bisa handle edit message
	return nil
}

// ShowDeleteAccountConfirmation menampilkan dialog konfirmasi delete account
func ShowDeleteAccountConfirmation(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int, accountID int) {
	am := GetAccountManager()
	account := am.GetAccount(accountID)
	if account == nil {
		errorMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **ERROR**\n\nAkun tidak ditemukan.")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Simpan state konfirmasi
	deleteConfirmMutex.Lock()
	deleteConfirmStates[chatID] = &DeleteConfirmState{
		AccountID:   accountID,
		PhoneNumber: account.PhoneNumber,
		MessageID:   messageID,
	}
	deleteConfirmMutex.Unlock()

	// Log activity
	utils.LogActivity("delete_account_confirm", fmt.Sprintf("User mengonfirmasi delete akun +%s", account.PhoneNumber), chatID)

	warningMsg := fmt.Sprintf(`⚠️ **HAPUS AKUN WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Akun yang akan dihapus:**
📱 **+%s**

🗑️ **Yang akan dihapus:**
• Data akun WhatsApp ini
• Semua grup yang terhubung
• Database akun ini
• Riwayat aktivitas akun

⚠️ **PERINGATAN:**
• Tindakan ini **TIDAK DAPAT DIBATALKAN**
• Data yang dihapus **TIDAK DAPAT DIPULIHKAN**
• Anda perlu login ulang jika ingin menggunakan nomor ini lagi

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Apakah Anda **yakin** ingin menghapus akun ini?`, account.PhoneNumber)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚠️ Ya, Hapus Akun", fmt.Sprintf("multi_account_delete_confirm_%d", accountID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batal", fmt.Sprintf("multi_account_delete_cancel_%d", accountID)),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, warningMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// DeleteAccount menghapus akun dengan validasi ownership
// SECURITY: Validasi bahwa accountID milik chatID (TelegramID) sebelum delete
func DeleteAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
	am := GetAccountManager()

	account := am.GetAccount(accountID)
	if account == nil {
		return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
	}

	// ✅ AMAN: Validasi ownership - cek apakah akun milik user yang memanggil
	userAccount := am.GetAccountByTelegramID(chatID)
	if userAccount == nil || userAccount.ID != accountID {
		utils.GetLogger().Warn("Security: User %d mencoba delete akun %d yang bukan miliknya", chatID, accountID)
		return fmt.Errorf("akses ditolak: akun ini bukan milik Anda")
	}

	// ✅ AMAN: Double-check dengan parse TelegramID dari BotDataDBPath
	reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
	reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)

	accountTelegramID := int64(0)
	matchesNew := reNew.FindStringSubmatch(account.BotDataDBPath)
	if len(matchesNew) >= 2 {
		if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
			accountTelegramID = parsedID
		}
	} else {
		matchesOld := reOld.FindStringSubmatch(account.BotDataDBPath)
		if len(matchesOld) >= 2 {
			if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
				accountTelegramID = parsedID
			}
		}
	}

	if accountTelegramID != 0 && accountTelegramID != chatID {
		utils.GetLogger().Warn("Security: TelegramID mismatch - User %d mencoba delete akun milik User %d", chatID, accountTelegramID)
		return fmt.Errorf("akses ditolak: akun ini bukan milik Anda")
	}

	phoneNumber := account.PhoneNumber

	// Log activity sebelum delete
	utils.LogActivity("delete_account", fmt.Sprintf("User menghapus akun +%s", phoneNumber), chatID)

	// Buat backup database sebelum delete
	backupPath := utils.BackupAccountDatabase(accountID, phoneNumber, account.BotDataDBPath)
	if backupPath != "" {
		utils.LogActivity("backup_account", fmt.Sprintf("Backup akun +%s dibuat: %s", phoneNumber, backupPath), chatID)
	}

	if err := am.RemoveAccount(accountID); err != nil {
		utils.LogActivity("delete_account_error", fmt.Sprintf("Gagal menghapus akun +%s: %v", phoneNumber, err), chatID)
		return err
	}

	// Log activity setelah delete
	utils.LogActivity("delete_account_success", fmt.Sprintf("Akun +%s berhasil dihapus", phoneNumber), chatID)

	return nil
}

// CancelDeleteAccount membatalkan konfirmasi delete
func CancelDeleteAccount(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	deleteConfirmMutex.Lock()
	delete(deleteConfirmStates, chatID)
	deleteConfirmMutex.Unlock()

	// Log activity
	utils.LogActivity("delete_account_cancel", "User membatalkan delete akun", chatID)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **PENGHAPUSAN DIBATALKAN**\n\nPenghapusan akun telah dibatalkan.\n\nAkun tetap aman.")
	editMsg.ParseMode = "Markdown"
	telegramBot.Send(editMsg)

	// Refresh daftar akun
	ShowAccountListEdit(telegramBot, chatID, messageID)
}

// IsWaitingForMultiAccountInput mengecek apakah user sedang menunggu input untuk multi-account
func IsWaitingForMultiAccountInput(chatID int64) bool {
	state := multiAccountLoginStates[chatID]
	return state != nil && state.WaitingForPhone
}
