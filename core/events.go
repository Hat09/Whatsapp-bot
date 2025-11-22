package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"whatsapp-bot/handlers"
	"whatsapp-bot/utils"
)

var (
	globalClient      *whatsmeow.Client
	globalTelegramBot *tgbotapi.BotAPI // Used in future updates for per-client event handling
	globalClientMutex sync.RWMutex     // FIXED: Mutex untuk thread-safe access
)

// SetGlobalClients mengatur global client references
func SetGlobalClients(client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	globalClientMutex.Lock()
	defer globalClientMutex.Unlock()
	globalClient = client
	globalTelegramBot = telegramBot
}

// GetGlobalClient mendapatkan global client dengan thread-safe access
func GetGlobalClient() *whatsmeow.Client {
	globalClientMutex.RLock()
	defer globalClientMutex.RUnlock()
	return globalClient
}

// updateAccountStatusFromClient memperbarui status akun di database berdasarkan client yang triggered event
func updateAccountStatusFromClient(client *whatsmeow.Client, status string) {
	if client == nil || client.Store.ID == nil {
		return
	}

	// Cari account ID berdasarkan phone number dari client
	am := handlers.GetAccountManager()
	accounts := am.GetAllAccounts()

	for _, acc := range accounts {
		accClient := am.GetClient(acc.ID)
		if accClient == client {
			// Found matching client, update status
			if err := am.UpdateAccountStatus(acc.ID, status); err != nil {
				utils.GetLogger().Warn("Failed to update account status for ID %d: %v", acc.ID, err)
			} else {
				utils.GetLogger().Info("Real-time status update: Account %d (%s) -> %s", acc.ID, acc.PhoneNumber, status)
			}
			return
		}
	}

	// Fallback: update global client jika tidak ditemukan di AccountManager
	if GetGlobalClient() == client {
		utils.GetLogger().Debug("Global client status updated to %s", status)
	}
}

// deleteBlockedAccountByID menghapus file database akun yang terblokir berdasarkan ID
// Menggunakan RemoveAccount untuk menghapus akun, file database (switch sudah dihandle sebelumnya)
// FIXED: Sekarang dipanggil secara realtime saat 404, disconnect, atau logout
func deleteBlockedAccountByID(accountID int) {
	am := handlers.GetAccountManager()
	account := am.GetAccount(accountID)

	if account == nil {
		utils.GetLogger().Warn("deleteBlockedAccountByID: Akun %d tidak ditemukan, mungkin sudah dihapus", accountID)
		return
	}

	utils.GetLogger().Info("deleteBlockedAccountByID: [REALTIME CLEANUP] Menghapus akun %d (%s) beserta file database-nya", account.ID, account.PhoneNumber)

	// Gunakan RemoveAccount untuk menghapus akun dan file database
	// Catatan: Switch ke akun lain sudah dihandle di handleAccountDisconnection sebelumnya
	// RemoveAccount akan:
	// 1. Disconnect client
	// 2. Hapus client dari AccountManager
	// 3. Hapus dari database master
	// 4. Hapus file database (db, -shm, -wal)
	if err := am.RemoveAccount(account.ID); err != nil {
		utils.GetLogger().Warn("deleteBlockedAccountByID: Gagal RemoveAccount untuk akun %d: %v", account.ID, err)
	} else {
		utils.GetLogger().Info("deleteBlockedAccountByID: ✅ [REALTIME CLEANUP] Akun %d (%s) berhasil dihapus beserta file database-nya", account.ID, account.PhoneNumber)

		// Kirim notifikasi ke Telegram
		notification := fmt.Sprintf("🗑️ **REALTIME CLEANUP - FILE DIHAPUS**\n\nAkun +%s telah terputus/logout/404.\n\n📁 **File yang dihapus:**\n• WhatsApp database (db, -shm, -wal)\n• Bot data database (db, -shm, -wal)\n\n✅ File database telah dihapus secara otomatis dari server.",
			account.PhoneNumber)
		handlers.SendToTelegram(notification)
	}
}

// cleanupAccountDBOnError menghapus database account saat terjadi error tertentu (404, disconnect, logout)
// FIXED: Fungsi baru untuk cleanup realtime berdasarkan error
func cleanupAccountDBOnError(client *whatsmeow.Client, errorReason string) {
	if client == nil || client.Store == nil || client.Store.ID == nil {
		return
	}

	am := handlers.GetAccountManager()
	accounts := am.GetAllAccounts()
	var accountID int = -1

	// Cari account ID berdasarkan client
	for _, acc := range accounts {
		accClient := am.GetClient(acc.ID)
		if accClient == client {
			accountID = acc.ID
			utils.GetLogger().Info("cleanupAccountDBOnError: [REALTIME CLEANUP] Account %d (%s) - Error: %s", acc.ID, acc.PhoneNumber, errorReason)
			break
		}
	}

	if accountID == -1 {
		utils.GetLogger().Warn("cleanupAccountDBOnError: Account tidak ditemukan untuk client yang error")
		return
	}

	// Hapus database account secara realtime
	deleteBlockedAccountByID(accountID)
}

// handleAccountDisconnection menangani disconnection/logout akun dengan auto-switch ke akun lain yang aktif
func handleAccountDisconnection(disconnectedClient *whatsmeow.Client) {
	if disconnectedClient == nil || disconnectedClient.Store == nil || disconnectedClient.Store.ID == nil {
		return
	}

	am := handlers.GetAccountManager()
	currentAccount := am.GetCurrentAccount()
	currentClient := am.GetCurrentClient()

	// Cek apakah akun yang terputus adalah current account
	isCurrentAccount := false
	disconnectedAccountID := -1

	if currentAccount != nil && currentClient != nil && currentClient == disconnectedClient {
		isCurrentAccount = true
		disconnectedAccountID = currentAccount.ID
		utils.GetLogger().Info("handleAccountDisconnection: Current account %d (%s) disconnected/logged out", currentAccount.ID, currentAccount.PhoneNumber)
	} else {
		// Cari account ID yang sesuai dengan disconnected client
		accounts := am.GetAllAccounts()
		for _, acc := range accounts {
			accClient := am.GetClient(acc.ID)
			if accClient == disconnectedClient {
				disconnectedAccountID = acc.ID
				// Cek apakah ini current account
				if currentAccount != nil && currentAccount.ID == acc.ID {
					isCurrentAccount = true
					utils.GetLogger().Info("handleAccountDisconnection: Current account %d (%s) disconnected/logged out", acc.ID, acc.PhoneNumber)
				}
				break
			}
		}
	}

	// Hanya lakukan auto-switch jika yang terputus adalah current account
	if !isCurrentAccount || disconnectedAccountID == -1 {
		return
	}

	// Cari akun lain yang masih aktif
	allAccounts := am.GetAllAccounts()
	var nextActiveAccount *handlers.WhatsAppAccount

	for _, acc := range allAccounts {
		if acc.ID != disconnectedAccountID {
			// Cek status koneksi aktual
			accClient := am.GetClient(acc.ID)
			if accClient != nil && accClient.IsConnected() && accClient.Store != nil && accClient.Store.ID != nil {
				nextActiveAccount = acc
				utils.GetLogger().Info("handleAccountDisconnection: Found active account %d (%s) untuk auto-switch", acc.ID, acc.PhoneNumber)
				break
			}
		}
	}

	// Jika ditemukan akun aktif, lakukan auto-switch
	if nextActiveAccount != nil {
		utils.GetLogger().Info("handleAccountDisconnection: Auto-switching dari akun %d ke akun %d", disconnectedAccountID, nextActiveAccount.ID)

		// Set sebagai current account
		if err := am.SetCurrentAccount(nextActiveAccount.ID); err != nil {
			utils.GetLogger().Warn("handleAccountDisconnection: Gagal set current account: %v", err)
			return
		}

		// Update global client
		nextClient := am.GetClient(nextActiveAccount.ID)
		if nextClient != nil {
			SetGlobalClients(nextClient, globalTelegramBot)
			handlers.SetClients(nextClient, globalTelegramBot)

			// Kirim notifikasi ke Telegram
			notification := fmt.Sprintf("⚠️ **AKUN TERPUTUS**\n\nAkun +%s telah terputus/logout.\n\n✅ **Auto-switch** ke akun aktif:\n📱 +%s",
				func() string {
					if currentAccount != nil {
						return currentAccount.PhoneNumber
					}
					return "Unknown"
				}(),
				nextActiveAccount.PhoneNumber)

			handlers.SendToTelegram(notification)
			utils.GetLogger().Info("handleAccountDisconnection: ✅ Auto-switch berhasil ke akun %d (%s)", nextActiveAccount.ID, nextActiveAccount.PhoneNumber)
		}
	} else {
		// Tidak ada akun aktif lain, clear current account dan kirim notifikasi
		// Coba set ke akun yang tidak ada untuk clear current (trick untuk set currentID = -1)
		// Atau lebih baik, cari akun yang masih ada tapi tidak aktif untuk dijadikan placeholder
		// Tapi lebih aman, kita hanya tidak set current account dan biarkan currentID tetap, tapi set global client ke nil

		// Set global client ke nil karena tidak ada akun aktif
		SetGlobalClients(nil, globalTelegramBot)
		handlers.SetClients(nil, globalTelegramBot)

		notification := fmt.Sprintf("⚠️ **SEMUA AKUN TERPUTUS**\n\nAkun +%s telah terputus/logout.\n\n❌ Tidak ada akun aktif lainnya.\n\n🔗 Gunakan fitur 'Login Baru' untuk menambahkan akun baru.",
			func() string {
				if currentAccount != nil {
					return currentAccount.PhoneNumber
				}
				return "Unknown"
			}())

		handlers.SendToTelegram(notification)
		utils.GetLogger().Warn("handleAccountDisconnection: Tidak ada akun aktif lain, global client di-set ke nil")
	}
}

// EventHandler adalah event handler untuk WhatsApp events
func EventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Simpan grup ke database jika pesan dari grup (untuk fitur list grup)
		// CRITICAL: Real-time refresh grup saat ada pesan baru (bukan 5 menit sekali)
		if v.Info.IsGroup && !v.Info.IsFromMe {
			groupJID := v.Info.Chat.String()
			groupName := ""

			client := GetGlobalClient() // FIXED: Use thread-safe getter
			if client != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				groupInfo, err := client.GetGroupInfo(ctx, v.Info.Chat)
				if err == nil && groupInfo != nil {
					groupName = groupInfo.Name
				}
				// FIXED: Removed double cancel() - defer already handles cancellation
			}

			// Save group to database secara real-time
			go utils.SaveGroupToDB(groupJID, groupName)

			// REAL-TIME REFRESH: Ambil semua grup dari WhatsApp dan update database
			// Ini memastikan database selalu up-to-date, bukan hanya setiap 5 menit
			go func() {
				// FIXED: Add timeout untuk goroutine untuk mencegah leak
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				// FIXED: Get client fresh dan validate dengan timeout
				client := handlers.GetWhatsAppClient()
				if client == nil || client.Store == nil || client.Store.ID == nil {
					utils.GetLogger().Debug("Real-time group refresh: Client invalid, skipping")
					return
				}

				// FIXED: Check connection dengan timeout
				select {
				case <-ctx.Done():
					utils.GetLogger().Debug("Real-time group refresh: Context timeout before connection check")
					return
				default:
					if !client.IsConnected() {
						utils.GetLogger().Debug("Real-time group refresh: Client not connected, skipping")
						return
					}
				}

				// Fetch semua grup dari WhatsApp API (real-time update)
				fetchCtx, fetchCancel := context.WithTimeout(ctx, 30*time.Second)
				defer fetchCancel()

				joinedGroups, err := client.GetJoinedGroups(fetchCtx)
				if err != nil {
					utils.GetLogger().Debug("Real-time group refresh: Gagal fetch groups: %v", err)
					return
				}

				if len(joinedGroups) == 0 {
					return
				}

				// Convert dan simpan ke database (real-time update)
				groupsToSave := make(map[string]string)
				for _, group := range joinedGroups {
					if group != nil {
						jidStr := group.JID.String()
						if strings.HasSuffix(jidStr, "@g.us") {
							groupName := group.Name
							if groupName == "" {
								groupName = fmt.Sprintf("Grup %s", group.JID.User)
							}
							groupsToSave[jidStr] = groupName
						}
					}
				}

				if len(groupsToSave) > 0 {
					if err := utils.BatchSaveGroupsToDB(groupsToSave); err != nil {
						utils.GetLogger().Debug("Real-time group refresh: Gagal batch save: %v", err)
					} else {
						utils.GetLogger().Debug("Real-time group refresh: ✅ Updated %d groups in database", len(groupsToSave))
					}
				}
			}()
		}
	case *events.Connected:
		// Client connected - update status di database secara real-time
		utils.GetLogger().Debug("WhatsApp client connected")

		// Coba update status dari globalClient jika ada
		client := GetGlobalClient() // FIXED: Use thread-safe getter
		if client != nil {
			go updateAccountStatusFromClient(client, "active")
		}
	case *events.Disconnected:
		handlers.SendToTelegram("❌ Disconnected from WhatsApp!")
		utils.GetLogger().Warn("WhatsApp client disconnected")

		// FIXED: REALTIME CLEANUP - Update status ke inactive, handle auto-switch, dan hapus database
		client := GetGlobalClient() // FIXED: Use thread-safe getter
		if client != nil {
			go func(disconnectedClient *whatsmeow.Client) {
				updateAccountStatusFromClient(disconnectedClient, "inactive")

				// Simpan account ID sebelum switch (karena setelah switch, globalClient akan berubah)
				am := handlers.GetAccountManager()
				accounts := am.GetAllAccounts()
				var disconnectedAccountID int = -1
				for _, acc := range accounts {
					accClient := am.GetClient(acc.ID)
					if accClient == disconnectedClient {
						disconnectedAccountID = acc.ID
						break
					}
				}

				// Handle disconnection dulu (auto-switch jika perlu)
				handleAccountDisconnection(disconnectedClient)

				// REALTIME CLEANUP: Hapus database setelah disconnect
				// Delay kecil untuk memastikan switch selesai
				time.Sleep(1 * time.Second)

				// Hapus account berdasarkan ID (bukan client, karena client sudah berubah)
				if disconnectedAccountID != -1 {
					utils.GetLogger().Info("events.Disconnected: [REALTIME CLEANUP] Menghapus database untuk account %d karena disconnect", disconnectedAccountID)
					deleteBlockedAccountByID(disconnectedAccountID)
				}
			}(client) // Pass as parameter to avoid race condition
		}
	case *events.LoggedOut:
		handlers.SendToTelegram("🚪 Logged out!")
		utils.GetLogger().Warn("WhatsApp client logged out")

		// FIXED: REALTIME CLEANUP - Update status ke inactive saat logout, handle auto-switch, dan hapus file database
		// Urutan: 1. Update status, 2. Simpan account ID, 3. Handle disconnection (switch), 4. Hapus file database
		client := GetGlobalClient() // FIXED: Use thread-safe getter
		if client != nil {
			go func(blockedClient *whatsmeow.Client) {
				updateAccountStatusFromClient(blockedClient, "inactive")

				// Simpan account ID sebelum switch (karena setelah switch, globalClient akan berubah)
				am := handlers.GetAccountManager()
				accounts := am.GetAllAccounts()
				var blockedAccountID int = -1
				for _, acc := range accounts {
					accClient := am.GetClient(acc.ID)
					if accClient == blockedClient {
						blockedAccountID = acc.ID
						break
					}
				}

				// Handle disconnection dulu (auto-switch jika perlu)
				handleAccountDisconnection(blockedClient)

				// REALTIME CLEANUP: Hapus database setelah logout
				// Delay kecil untuk memastikan switch selesai
				time.Sleep(1 * time.Second)

				// Hapus account berdasarkan ID (bukan client, karena client sudah berubah)
				if blockedAccountID != -1 {
					utils.GetLogger().Info("events.LoggedOut: [REALTIME CLEANUP] Menghapus database untuk account %d karena logout", blockedAccountID)
					deleteBlockedAccountByID(blockedAccountID)
				}
			}(client) // Pass client sebagai parameter untuk closure
		}
	case *events.PairSuccess:
		// Pair success handled in PairDeviceViaTelegram, skip here to avoid duplication
		return
	}
}

// HandleWhatsAppError menangani error dari operasi WhatsApp dan melakukan cleanup jika perlu
// FIXED: Fungsi baru untuk deteksi 404 dan error lainnya, trigger cleanup realtime
func HandleWhatsAppError(client *whatsmeow.Client, err error, operation string) {
	if err == nil || client == nil {
		return
	}

	errorText := strings.ToLower(err.Error())

	// Deteksi error yang memerlukan cleanup database (404, 401, device_removed, dll)
	criticalErrors := []string{
		"404",
		"401",
		"unauthorized",
		"device_removed",
		"not logged in",
		"session expired",
		"authentication failed",
		"logged out",
	}

	needsCleanup := false
	for _, criticalErr := range criticalErrors {
		if strings.Contains(errorText, criticalErr) {
			needsCleanup = true
			utils.GetLogger().Warn("HandleWhatsAppError: [REALTIME CLEANUP] Critical error detected: %s (operation: %s)", criticalErr, operation)
			break
		}
	}

	if needsCleanup {
		// Trigger cleanup database realtime
		cleanupAccountDBOnError(client, fmt.Sprintf("%s: %s", operation, errorText))
	}
}
