package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// ResetProgramRequest menampilkan konfirmasi reset program
func ResetProgramRequest(telegramBot *tgbotapi.BotAPI, chatID int64) {
	fmt.Printf("[DEBUG] ResetProgramRequest called for chatID=%d\n", chatID)

	warningMsg := `⚠️ RESET PROGRAM

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Perhatian! Fitur ini akan menghapus SEMUA data:

🗑️ Yang akan dihapus:
• Semua database WhatsApp (multi-account)
• Semua database bot_data
• Semua konfigurasi aplikasi
• Semua data grup yang tersimpan
• Semua akun yang sudah terdaftar

⚠️ PERINGATAN:
• Tindakan ini TIDAK DAPAT DIBATALKAN
• Semua akun WhatsApp akan logout
• Anda perlu login ulang semua akun
• Data yang dihapus tidak dapat dikembalikan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Apakah Anda yakin ingin mereset program?`

	msg := tgbotapi.NewMessage(chatID, warningMsg)
	// Tidak menggunakan parse mode untuk menghindari parsing error

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚠️ Ya, Reset Program", "reset_confirm"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batal", "reset_cancel"),
		),
	)
	msg.ReplyMarkup = keyboard

	sentMsg, err := telegramBot.Send(msg)
	if err != nil {
		fmt.Printf("[ERROR] Failed to send reset confirmation message: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] Reset confirmation message sent successfully for chatID=%d, messageID=%d\n", chatID, sentMsg.MessageID)
	}
}

// ResetProgramRequestEdit menampilkan konfirmasi reset program dengan EDIT (no spam!)
func ResetProgramRequestEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	fmt.Printf("[DEBUG] ResetProgramRequestEdit called for chatID=%d, messageID=%d\n", chatID, messageID)

	warningMsg := `⚠️ RESET PROGRAM

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Perhatian! Fitur ini akan menghapus SEMUA data:

🗑️ Yang akan dihapus:
• Semua database WhatsApp (multi-account)
• Semua database bot_data
• Semua konfigurasi aplikasi
• Semua data grup yang tersimpan
• Semua akun yang sudah terdaftar

⚠️ PERINGATAN:
• Tindakan ini TIDAK DAPAT DIBATALKAN
• Semua akun WhatsApp akan logout
• Anda perlu login ulang semua akun
• Data yang dihapus tidak dapat dikembalikan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Apakah Anda yakin ingin mereset program?`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚠️ Ya, Reset Program", "reset_confirm"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batal", "reset_cancel"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, warningMsg)
	editMsg.ReplyMarkup = &keyboard

	_, err := telegramBot.Send(editMsg)
	if err != nil {
		fmt.Printf("[ERROR] Failed to edit reset confirmation message: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] Reset confirmation message edited successfully for chatID=%d, messageID=%d\n", chatID, messageID)
	}
}

// ConfirmResetProgram melakukan reset program lengkap
func ConfirmResetProgram(telegramBot *tgbotapi.BotAPI, chatID int64) error {
	// SECURITY: Validasi bahwa user memiliki akun terdaftar atau adalah admin
	// Reset program adalah operasi yang sangat berbahaya, hanya boleh dilakukan oleh user yang memiliki akun
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)

	// Cek apakah user adalah admin (untuk backward compatibility, admin bisa reset)
	config, err := utils.LoadTelegramConfig()
	isAdmin := false
	if err == nil {
		isAdmin = config.IsAdmin(chatID)
	}

	// Hanya izinkan jika user memiliki akun ATAU adalah admin
	if userAccount == nil && !isAdmin {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar dan bukan admin, akses reset ditolak", chatID)
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nAnda tidak memiliki izin untuk melakukan reset program.\n\nHanya admin atau user yang memiliki akun terdaftar yang dapat melakukan reset.")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return fmt.Errorf("user %d tidak memiliki izin untuk reset program", chatID)
	}

	progressMsg := tgbotapi.NewMessage(chatID, "🔄 **MEMULAI RESET PROGRAM...**\n\nMohon tunggu, proses mungkin memakan waktu beberapa detik...")
	progressMsg.ParseMode = "Markdown"
	telegramBot.Send(progressMsg)

	deletedFiles := []string{}
	errors := []string{}

	// 1. Disconnect dan logout semua client WhatsApp
	// am sudah dideklarasikan di atas
	am.mutex.Lock()
	allClients := make(map[int]*whatsmeow.Client)
	for id, client := range am.clients {
		allClients[id] = client
	}
	am.mutex.Unlock()

	// Logout semua akun dari AccountManager
	for id, client := range allClients {
		if client != nil && client.Store.ID != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			client.Logout(ctx)
			cancel()
			client.Disconnect()
			time.Sleep(500 * time.Millisecond)

			// Hapus client dari map
			am.mutex.Lock()
			delete(am.clients, id)
			am.mutex.Unlock()
		}
	}

	// Logout client utama jika ada
	if WaClient != nil {
		if WaClient.Store.ID != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			WaClient.Logout(ctx)
			cancel()
		}
		WaClient.Disconnect()
		time.Sleep(500 * time.Millisecond)
	}

	// 2. Tutup semua connection pool database
	utils.CloseDBPools()

	// 3. Reset konfigurasi
	utils.ResetDBConfig()

	// 4. Hapus semua database file
	dbPatterns := []string{
		"*.db",
		"*.db-shm",
		"*.db-wal",
	}

	for _, pattern := range dbPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error glob %s: %v", pattern, err))
			continue
		}

		for _, file := range matches {
			// Skip file yang sedang digunakan atau read-only
			if err := os.Remove(file); err != nil {
				// Coba lagi setelah delay
				time.Sleep(200 * time.Millisecond)
				if err2 := os.Remove(file); err2 == nil {
					deletedFiles = append(deletedFiles, file)
				} else {
					errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", file, err2))
				}
			} else {
				deletedFiles = append(deletedFiles, file)
			}
		}
	}

	// 5. Reset AccountManager
	am.mutex.Lock()
	am.accounts = make(map[int]*WhatsAppAccount)
	am.clients = make(map[int]*whatsmeow.Client)
	am.currentID = -1
	am.mutex.Unlock()

	// 6. Reset WaClient dan TgBot references
	WaClient = nil

	// 7. Re-initialize database setelah reset (penting untuk membuat tabel whatsapp_accounts)
	// Pastikan tabel whatsapp_accounts ada untuk login selanjutnya
	if err := InitAccountDB(); err != nil {
		errors = append(errors, fmt.Sprintf("Warning: Failed to re-init account DB after reset: %v", err))
		utils.GetLogger().Warn("Failed to re-init account DB after reset: %v", err)
	} else {
		utils.GetLogger().Info("Account DB re-initialized successfully after reset")
	}

	// 8. Re-setup bot database untuk memastikan semua tabel ada
	if err := utils.SetupBotDB(); err != nil {
		errors = append(errors, fmt.Sprintf("Warning: Failed to re-setup bot DB after reset: %v", err))
		utils.GetLogger().Warn("Failed to re-setup bot DB after reset: %v", err)
	} else {
		utils.GetLogger().Info("Bot DB re-setup successfully after reset")
	}

	// 9. Format pesan hasil
	var resultMsg string
	if len(deletedFiles) > 0 {
		resultMsg = fmt.Sprintf(`✅ **RESET PROGRAM BERHASIL!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🗑️ **File yang dihapus:** %d file

📋 **Detail:**
%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Status:**
• Semua database telah dihapus
• Semua konfigurasi telah direset
• Semua akun telah logout

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📝 **Langkah Selanjutnya:**
1. Gunakan "📱 Login WhatsApp Baru" untuk login ulang
2. Atau gunakan /pair <nomor> untuk pairing

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
			len(deletedFiles),
			func() string {
				fileList := ""
				for i, file := range deletedFiles {
					if i < 20 { // Limit display to 20 files
						fileList += fmt.Sprintf("• %s\n", file)
					}
				}
				if len(deletedFiles) > 20 {
					fileList += fmt.Sprintf("• ... dan %d file lainnya\n", len(deletedFiles)-20)
				}
				return fileList
			}(),
		)

		if len(errors) > 0 {
			resultMsg += fmt.Sprintf("\n⚠️ **Warning:**\n%s\n", strings.Join(errors[:min(5, len(errors))], "\n"))
		}
	} else {
		resultMsg = `✅ **RESET PROGRAM SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Tidak ada file database yang ditemukan untuk dihapus.

Semua konfigurasi telah direset.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📝 **Langkah Selanjutnya:**
Gunakan "📱 Login WhatsApp Baru" atau /pair <nomor> untuk login ulang.`
	}

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	// Tambahkan keyboard untuk login
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Login WhatsApp Baru", "multi_account_login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Menu Utama", "menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)

	// Tampilkan login prompt setelah 2 detik
	// Tapi pastikan client tidak nil untuk menghindari error "WhatsApp client belum diinisialisasi"
	time.Sleep(2 * time.Second)

	// Pastikan client di-set ke nil secara eksplisit sebelum show login prompt
	// GetWhatsAppClient() akan return nil jika tidak ada akun aktif (yang benar setelah reset)
	// Tapi ui.ShowLoginPrompt tidak memerlukan client, jadi ini seharusnya tidak masalah

	// Coba tampilkan login prompt
	// Jika ada error, ignore karena setelah reset memang belum ada client
	ui.ShowLoginPrompt(telegramBot, chatID)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
