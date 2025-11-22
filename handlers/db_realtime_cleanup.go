package handlers

import (
	"os"
	"strings"
	"sync"
	"time"

	"whatsapp-bot/utils"
)

// accountLastUsed menyimpan waktu terakhir setiap account digunakan
var accountLastUsed = make(map[int]time.Time)
var accountLastUsedMutex sync.RWMutex

// UpdateAccountLastUsed memperbarui waktu terakhir account digunakan
func UpdateAccountLastUsed(accountID int) {
	accountLastUsedMutex.Lock()
	defer accountLastUsedMutex.Unlock()
	accountLastUsed[accountID] = time.Now()
	utils.GetLogger().Debug("UpdateAccountLastUsed: Account %d last used updated to %v", accountID, time.Now())
}

// GetAccountLastUsed mendapatkan waktu terakhir account digunakan
func GetAccountLastUsed(accountID int) (time.Time, bool) {
	accountLastUsedMutex.RLock()
	defer accountLastUsedMutex.RUnlock()
	lastUsed, exists := accountLastUsed[accountID]
	return lastUsed, exists
}

// CleanupInactiveAccountDBs menghapus database untuk akun yang tidak aktif secara realtime
// Parameter:
//   - inactiveThreshold: Durasi tidak aktif sebelum dianggap inactive (contoh: 7 hari)
//   - activeAccountIDs: Daftar account ID yang masih aktif (tidak akan dihapus)
//
// Return: jumlah file database yang dihapus
func CleanupInactiveAccountDBs(inactiveThreshold time.Duration, activeAccountIDs []int) int {
	utils.GetLogger().Info("CleanupInactiveAccountDBs: Memulai cleanup database untuk akun tidak aktif (threshold: %v)", inactiveThreshold)

	am := GetAccountManager()
	allAccounts := am.GetAllAccounts()

	// Buat map untuk active accounts
	activeAccountMap := make(map[int]bool)
	for _, id := range activeAccountIDs {
		activeAccountMap[id] = true
	}

	now := time.Now()
	deletedCount := 0
	inactiveAccounts := []*WhatsAppAccount{}

	// Identifikasi akun yang tidak aktif
	for _, account := range allAccounts {
		// Skip jika account ada di daftar aktif
		if activeAccountMap[account.ID] {
			utils.GetLogger().Debug("CleanupInactiveAccountDBs: Account %d (%s) ada di daftar aktif, skip", account.ID, account.PhoneNumber)
			continue
		}

		// Cek waktu terakhir digunakan
		accountLastUsedMutex.RLock()
		lastUsed, hasLastUsed := accountLastUsed[account.ID]
		accountLastUsedMutex.RUnlock()

		// Jika tidak ada record lastUsed, gunakan UpdatedAt dari account
		if !hasLastUsed {
			lastUsed = account.UpdatedAt
		}

		// Cek apakah sudah melewati threshold
		if now.Sub(lastUsed) > inactiveThreshold {
			// Account tidak aktif, tambahkan ke daftar
			inactiveAccounts = append(inactiveAccounts, account)
			utils.GetLogger().Info("CleanupInactiveAccountDBs: Account %d (%s) tidak aktif sejak %v (sudah %v)",
				account.ID, account.PhoneNumber, lastUsed, now.Sub(lastUsed))
		} else {
			utils.GetLogger().Debug("CleanupInactiveAccountDBs: Account %d (%s) masih aktif (last used: %v, threshold: %v)",
				account.ID, account.PhoneNumber, lastUsed, inactiveThreshold)
		}
	}

	// Hapus database untuk akun yang tidak aktif
	for _, account := range inactiveAccounts {
		// CRITICAL: Pastikan client tidak sedang digunakan sebelum hapus database
		am.mutex.RLock()
		client := am.clients[account.ID]
		am.mutex.RUnlock()

		// Jika client masih ada dan connected, disconnect dulu
		if client != nil {
			if client.IsConnected() {
				utils.GetLogger().Info("CleanupInactiveAccountDBs: Disconnecting client untuk account %d sebelum hapus database", account.ID)
				client.Disconnect()
			}
			// Hapus client dari map
			am.mutex.Lock()
			delete(am.clients, account.ID)
			am.mutex.Unlock()
		}

		// Hapus database files
		dbFiles := []string{}
		if account.DBPath != "" {
			dbFiles = append(dbFiles, account.DBPath)
			dbFiles = append(dbFiles, account.DBPath+"-shm")
			dbFiles = append(dbFiles, account.DBPath+"-wal")
		}
		if account.BotDataDBPath != "" {
			dbFiles = append(dbFiles, account.BotDataDBPath)
			dbFiles = append(dbFiles, account.BotDataDBPath+"-shm")
			dbFiles = append(dbFiles, account.BotDataDBPath+"-wal")
		}

		// Hapus file database
		for _, dbFile := range dbFiles {
			if _, err := os.Stat(dbFile); err == nil {
				// File exists, hapus
				if err := os.Remove(dbFile); err == nil {
					deletedCount++
					utils.GetLogger().Info("CleanupInactiveAccountDBs: ✅ Berhasil hapus database file: %s (account: %s)",
						dbFile, account.PhoneNumber)
				} else {
					utils.GetLogger().Warn("CleanupInactiveAccountDBs: ❌ Gagal hapus database file: %s (error: %v)", dbFile, err)
				}
			}
		}

		// Hapus account dari database master
		if err := am.RemoveAccount(account.ID); err != nil {
			utils.GetLogger().Warn("CleanupInactiveAccountDBs: Gagal hapus account %d dari database master: %v", account.ID, err)
		} else {
			utils.GetLogger().Info("CleanupInactiveAccountDBs: ✅ Berhasil hapus account %d (%s) dari database master",
				account.ID, account.PhoneNumber)
		}

		// Hapus dari accountLastUsed map
		accountLastUsedMutex.Lock()
		delete(accountLastUsed, account.ID)
		accountLastUsedMutex.Unlock()
	}

	if len(inactiveAccounts) > 0 {
		utils.GetLogger().Info("CleanupInactiveAccountDBs: ✅ Menghapus %d database untuk akun tidak aktif (%d files total)",
			len(inactiveAccounts), deletedCount)
		return deletedCount
	} else {
		utils.GetLogger().Info("CleanupInactiveAccountDBs: ✅ Tidak ada akun tidak aktif ditemukan")
		return 0
	}
}

// StartRealtimeDBCleanup memulai background cleanup untuk database akun tidak aktif
// Parameter:
//   - cleanupInterval: Interval cleanup (contoh: 1 jam)
//   - inactiveThreshold: Durasi tidak aktif sebelum dianggap inactive (contoh: 7 hari)
//   - getActiveAccountIDs: Fungsi untuk mendapatkan daftar account ID yang masih aktif
func StartRealtimeDBCleanup(cleanupInterval time.Duration, inactiveThreshold time.Duration, getActiveAccountIDs func() []int) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		utils.GetLogger().Info("StartRealtimeDBCleanup: Background cleanup dimulai (interval: %v, threshold: %v)",
			cleanupInterval, inactiveThreshold)

		for range ticker.C {
			// Dapatkan daftar account aktif
			activeAccountIDs := getActiveAccountIDs()

			// Jalankan cleanup
			deletedCount := CleanupInactiveAccountDBs(inactiveThreshold, activeAccountIDs)

			if deletedCount > 0 {
				utils.GetLogger().Info("StartRealtimeDBCleanup: Cleanup selesai, %d file database dihapus", deletedCount)

				// Kirim notifikasi ke Telegram jika ada
				am := GetAccountManager()
				am.mutex.RLock()
				telegramBot := am.telegramBot
				am.mutex.RUnlock()

				if telegramBot != nil {
					// Cari user pertama yang punya akun untuk notifikasi
					allAccounts := am.GetAllAccounts()
					if len(allAccounts) > 0 {
						// Ambil Telegram ID dari account pertama (atau bisa dikirim ke semua user)
						// Untuk sementara, skip notifikasi karena perlu Telegram ID
						utils.GetLogger().Debug("StartRealtimeDBCleanup: Cleanup selesai, %d file dihapus", deletedCount)
					}
				}
			}
		}
	}()
}

// GetActiveAccountIDsFromSessions mendapatkan daftar account ID yang masih aktif dari sessions
func GetActiveAccountIDsFromSessions() []int {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	activeAccountIDs := make(map[int]bool)
	for _, session := range userSessions {
		if session.AccountID > 0 {
			activeAccountIDs[session.AccountID] = true
		}
	}

	result := []int{}
	for id := range activeAccountIDs {
		result = append(result, id)
	}

	return result
}

// CleanupInactiveDBsByPhoneNumbers menghapus database untuk nomor yang tidak ada di daftar aktif
// Fungsi ini untuk cleanup berdasarkan daftar nomor aktif yang diberikan user
func CleanupInactiveDBsByPhoneNumbers(activePhoneNumbers []string) (int, []string, []string) {
	utils.GetLogger().Info("CleanupInactiveDBsByPhoneNumbers: Memulai cleanup berdasarkan daftar nomor aktif (%d nomor)",
		len(activePhoneNumbers))

	am := GetAccountManager()
	allAccounts := am.GetAllAccounts()

	// Buat map untuk nomor aktif
	activePhoneMap := make(map[string]bool)
	for _, phone := range activePhoneNumbers {
		// Normalize phone number (hapus +, spasi, dll)
		normalized := normalizePhoneNumber(phone)
		activePhoneMap[normalized] = true
		utils.GetLogger().Debug("CleanupInactiveDBsByPhoneNumbers: Nomor aktif: %s (normalized: %s)", phone, normalized)
	}

	deletedCount := 0
	deletedAccounts := []string{}
	keptAccounts := []string{}

	// Identifikasi dan hapus database untuk akun yang tidak aktif
	for _, account := range allAccounts {
		normalizedPhone := normalizePhoneNumber(account.PhoneNumber)

		if activePhoneMap[normalizedPhone] {
			// Account aktif, pertahankan
			keptAccounts = append(keptAccounts, account.PhoneNumber)
			utils.GetLogger().Debug("CleanupInactiveDBsByPhoneNumbers: Account %d (%s) aktif, pertahankan",
				account.ID, account.PhoneNumber)
			continue
		}

		// Account tidak aktif, hapus database
		utils.GetLogger().Info("CleanupInactiveDBsByPhoneNumbers: Account %d (%s) tidak aktif, hapus database",
			account.ID, account.PhoneNumber)

		// CRITICAL: Pastikan client tidak sedang digunakan sebelum hapus database
		am.mutex.RLock()
		client := am.clients[account.ID]
		am.mutex.RUnlock()

		// Jika client masih ada dan connected, disconnect dulu
		if client != nil {
			if client.IsConnected() {
				utils.GetLogger().Info("CleanupInactiveDBsByPhoneNumbers: Disconnecting client untuk account %d sebelum hapus database", account.ID)
				client.Disconnect()
			}
			// Hapus client dari map
			am.mutex.Lock()
			delete(am.clients, account.ID)
			am.mutex.Unlock()
		}

		// Hapus database files
		dbFiles := []string{}
		if account.DBPath != "" {
			dbFiles = append(dbFiles, account.DBPath)
			dbFiles = append(dbFiles, account.DBPath+"-shm")
			dbFiles = append(dbFiles, account.DBPath+"-wal")
		}
		if account.BotDataDBPath != "" {
			dbFiles = append(dbFiles, account.BotDataDBPath)
			dbFiles = append(dbFiles, account.BotDataDBPath+"-shm")
			dbFiles = append(dbFiles, account.BotDataDBPath+"-wal")
		}

		// Hapus file database
		for _, dbFile := range dbFiles {
			if _, err := os.Stat(dbFile); err == nil {
				// File exists, hapus
				if err := os.Remove(dbFile); err == nil {
					deletedCount++
					utils.GetLogger().Info("CleanupInactiveDBsByPhoneNumbers: ✅ Berhasil hapus database file: %s", dbFile)
				} else {
					utils.GetLogger().Warn("CleanupInactiveDBsByPhoneNumbers: ❌ Gagal hapus database file: %s (error: %v)", dbFile, err)
				}
			}
		}

		// Hapus account dari database master
		if err := am.RemoveAccount(account.ID); err != nil {
			utils.GetLogger().Warn("CleanupInactiveDBsByPhoneNumbers: Gagal hapus account %d dari database master: %v", account.ID, err)
		} else {
			utils.GetLogger().Info("CleanupInactiveDBsByPhoneNumbers: ✅ Berhasil hapus account %d (%s) dari database master",
				account.ID, account.PhoneNumber)
			deletedAccounts = append(deletedAccounts, account.PhoneNumber)
		}

		// Hapus dari accountLastUsed map
		accountLastUsedMutex.Lock()
		delete(accountLastUsed, account.ID)
		accountLastUsedMutex.Unlock()
	}

	utils.GetLogger().Info("CleanupInactiveDBsByPhoneNumbers: ✅ Cleanup selesai - %d account dipertahankan, %d account dihapus, %d file database dihapus",
		len(keptAccounts), len(deletedAccounts), deletedCount)

	return deletedCount, keptAccounts, deletedAccounts
}

// normalizePhoneNumber menormalisasi nomor telepon (hapus +, spasi, dll)
func normalizePhoneNumber(phone string) string {
	normalized := phone
	normalized = strings.ReplaceAll(normalized, "+", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}
