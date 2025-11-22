package ui

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ShowWelcome menampilkan pesan sambutan saat program pertama kali dinyalakan
func ShowWelcome(bot *tgbotapi.BotAPI, chatID int64) {
	welcomeMsg := fmt.Sprintf(`✨ **Selamat Datang!**

🤖 **WhatsApp Bot dengan Telegram Integration**

Bot ini memungkinkan Anda untuk:
• Mengontrol WhatsApp melalui Telegram
• Menerima notifikasi pesan WhatsApp
• Mengirim pesan WhatsApp dari Telegram
• Dan banyak lagi...

🔄 Bot sedang mempersiapkan sistem...`)

	msg := tgbotapi.NewMessage(chatID, welcomeMsg)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// ShowLoginPrompt menampilkan prompt untuk login/pairing jika belum login
func ShowLoginPrompt(bot *tgbotapi.BotAPI, chatID int64) {
	loginMsg := `🔐 **LOGIN REQUIRED**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 **STATUS AKUN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

❌ WhatsApp Bot: Belum Terhubung
✅ Telegram Bot: Terhubung

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **CARA LOGIN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Untuk menggunakan bot, Anda perlu melakukan pairing terlebih dahulu.

**Langkah-langkah:**
1️⃣ Klik tombol "🔗 Mulai Pairing" di bawah
2️⃣ Masukkan nomor WhatsApp Anda
3️⃣ Ikuti instruksi yang diberikan
4️⃣ Selesai! Bot siap digunakan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠️ **FORMAT NOMOR**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

• Gunakan kode negara (tanpa + atau 0)
• Contoh: 628123456789 (untuk Indonesia)
• Contoh: 14155552671 (untuk US)`

	msg := tgbotapi.NewMessage(chatID, loginMsg)
	msg.ParseMode = "Markdown"

	// Tambahkan inline keyboard untuk memulai pairing
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 Mulai Pairing", "start_pairing"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Info", "login_info"),
			tgbotapi.NewInlineKeyboardButtonData("❓ Help", "login_help"),
		),
	)
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}
