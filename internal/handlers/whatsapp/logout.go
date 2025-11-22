package handlers

import (
	"context"
	"fmt"
	"os"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// LogoutWhatsApp melakukan logout dari WhatsApp dan menghapus database
func LogoutWhatsApp(chatID int64) error {
	if WaClient == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Cek apakah sudah login
	if WaClient.Store.ID == nil {
		return fmt.Errorf("bot WhatsApp belum login")
	}

	// Ambil nomor yang akan di-logout untuk informasi
	phoneNumber := ""
	if WaClient.Store.ID != nil {
		phoneNumber = WaClient.Store.ID.User
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
	if WaClient == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Ambil nomor sebelum logout
	phoneNumber := ""
	if WaClient.Store.ID != nil {
		phoneNumber = WaClient.Store.ID.User
	}

	// Kirim notifikasi mulai logout
	progressMsg := tgbotapi.NewMessage(chatID, "🔄 Memproses logout...")
	TgBot.Send(progressMsg)

	// Logout dari WhatsApp
	WaClient.Logout(context.Background())

	// Tunggu sebentar untuk memastikan logout selesai
	time.Sleep(2 * time.Second)

	// Disconnect client
	WaClient.Disconnect()

	// Hapus database WhatsApp (menggunakan nama dinamis jika ada)
	whatsappDB := utils.GetWhatsAppDBPath()
	botDataDB := utils.GetBotDataDBPath()

	dbFiles := []string{
		// Database dinamis (jika ada)
		whatsappDB,
		whatsappDB + "-shm",
		whatsappDB + "-wal",
		botDataDB,
		botDataDB + "-shm",
		botDataDB + "-wal",
		// Database default (untuk kompatibilitas)
		"whatsapp.db",
		"whatsapp.db-shm",
		"whatsapp.db-wal",
		"bot_data.db",
		"bot_data.db-shm",
		"bot_data.db-wal",
	}

	deletedFiles := []string{}
	for _, dbFile := range dbFiles {
		if _, err := os.Stat(dbFile); err == nil {
			if err := os.Remove(dbFile); err == nil {
				deletedFiles = append(deletedFiles, dbFile)
			}
		}
	}

	// Format pesan success
	successMsg := fmt.Sprintf(`✅ **LOGOUT BERHASIL!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Nomor: %s

🗑️ **Data yang dihapus:**
%s

📝 **Catatan:**
• Bot WhatsApp sudah logout
• Database telah dihapus
• Gunakan /pair <nomor> untuk login kembali

Gunakan /menu untuk melihat menu utama.`,
		phoneNumber,
		func() string {
			if len(deletedFiles) > 0 {
				result := ""
				for _, file := range deletedFiles {
					result += fmt.Sprintf("• %s\n", file)
				}
				return result
			}
			return "• Tidak ada file database yang ditemukan\n"
		}())

	msg := tgbotapi.NewMessage(chatID, successMsg)
	msg.ParseMode = "Markdown"
	TgBot.Send(msg)

	// Tampilkan login prompt
	time.Sleep(1 * time.Second)
	ui.ShowLoginPrompt(TgBot, chatID)

	return nil
}
