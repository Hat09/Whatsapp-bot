package handlers

import (
	"context"
	"fmt"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// LogoutWhatsApp melakukan logout dari WhatsApp dan menghapus database
func LogoutWhatsApp(chatID int64) error {
	client := GetWhatsAppClient()
	if client == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Cek apakah sudah login
	if client.Store.ID == nil {
		return fmt.Errorf("bot WhatsApp belum login")
	}

	// Ambil nomor yang akan di-logout untuk informasi
	phoneNumber := ""
	if client.Store.ID != nil {
		phoneNumber = client.Store.ID.User
	}

	// Kirim konfirmasi ke user dengan inline keyboard
	confirmMsg := fmt.Sprintf(`⚠️ **KONFIRMASI LOGOUT**

Anda akan logout dari WhatsApp dan menghapus semua data database.

**Nomor:** %s

⚠️ **PERINGATAN:**
• Semua data WhatsApp akan dihapus
• Anda perlu pairing ulang untuk login kembali
• Tindakan ini tidak dapat dibatalkan

Gunakan tombol di bawah untuk konfirmasi:`, phoneNumber)

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	// Tambahkan inline keyboard untuk konfirmasi
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Logout", "logout_confirm"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batal", "logout_cancel"),
		),
	)
	msg.ReplyMarkup = keyboard

	TgBot.Send(msg)

	return nil
}

// ConfirmLogout melakukan logout dan menghapus database setelah konfirmasi
func ConfirmLogout(chatID int64) error {
	// SECURITY: Validasi bahwa user hanya bisa logout akun mereka sendiri
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)

	if userAccount == nil {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar, akses logout ditolak", chatID)
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nTidak ada akun yang bisa di-logout.")
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)
		return fmt.Errorf("user %d tidak memiliki akun terdaftar", chatID)
	}

	// Pastikan user hanya bisa logout akun mereka sendiri
	// Switch ke akun user terlebih dahulu
	if err := SwitchAccount(userAccount.ID, TgBot, chatID); err != nil {
		utils.GetLogger().Warn("Security: Failed to switch to user account for logout: %v", err)
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)
		return fmt.Errorf("failed to switch to user account: %w", err)
	}

	// Coba ambil dari AccountManager terlebih dahulu
	client := am.GetCurrentClient()
	if client == nil {
		client = WaClient
	}

	if client == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Validasi bahwa client yang digunakan sesuai dengan akun user
	if client.Store.ID != nil {
		clientPhoneNumber := client.Store.ID.User
		if clientPhoneNumber != userAccount.PhoneNumber {
			utils.GetLogger().Warn("Security: Client phone mismatch for logout. User: %d, Expected: %s, Got: %s", chatID, userAccount.PhoneNumber, clientPhoneNumber)
			errorMsg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nAnda hanya bisa logout akun WhatsApp Anda sendiri.")
			errorMsg.ParseMode = "Markdown"
			TgBot.Send(errorMsg)
			return fmt.Errorf("user %d tidak memiliki izin untuk logout akun %s", chatID, clientPhoneNumber)
		}
	}

	// Ambil nomor sebelum logout
	phoneNumber := ""
	if client.Store.ID != nil {
		phoneNumber = client.Store.ID.User
	}

	// Kirim notifikasi mulai logout
	progressMsg := tgbotapi.NewMessage(chatID, "🔄 Memproses logout...")
	TgBot.Send(progressMsg)

	// Logout dari WhatsApp
	client.Logout(context.Background())

	// Tunggu sebentar untuk memastikan logout selesai
	time.Sleep(2 * time.Second)

	// Disconnect client
	client.Disconnect()

	// Hapus dari AccountManager jika ada
	currentAccount := am.GetCurrentAccount()
	if currentAccount != nil && currentAccount.PhoneNumber == phoneNumber {
		// Remove client dari AccountManager
		am.mutex.Lock()
		delete(am.clients, currentAccount.ID)
		am.currentID = -1
		am.mutex.Unlock()

		// Hapus dari database (RemoveAccount akan otomatis menghapus file database)
		if err := am.RemoveAccount(currentAccount.ID); err != nil {
			utils.GetLogger().Warn("ConfirmLogout: Gagal RemoveAccount: %v", err)
		} else {
			utils.GetLogger().Info("ConfirmLogout: Akun %d berhasil dihapus beserta file database-nya", currentAccount.ID)
		}
	}

	// Reset DB config setelah menghapus akun
	utils.ResetDBConfig()

	// Tutup connection pool database jika ada
	utils.CloseDBPools()

	// Format pesan success
	successMsg := fmt.Sprintf(`✅ **LOGOUT BERHASIL!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Nomor: %s

🗑️ **Data yang dihapus:**
• WhatsApp database (db, -shm, -wal)
• Bot data database (db, -shm, -wal)

📝 **Catatan:**
• Bot WhatsApp sudah logout
• Semua file database akun telah dihapus
• Gunakan /pair <nomor> untuk login kembali

Gunakan /menu untuk melihat menu utama.`,
		phoneNumber)

	msg := tgbotapi.NewMessage(chatID, successMsg)
	msg.ParseMode = "Markdown"
	TgBot.Send(msg)

	// Tampilkan login prompt
	time.Sleep(1 * time.Second)
	ui.ShowLoginPrompt(TgBot, chatID)

	return nil
}
