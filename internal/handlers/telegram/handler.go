package handlers

import (
	"fmt"
	"strings"
	"time"

	"whatsapp-bot/ui"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// WaitingForPhoneNumber state management untuk input nomor
var WaitingForPhoneNumber = make(map[int64]bool)

// HandleTelegramCommand memproses command dari Telegram
func HandleTelegramCommand(message *tgbotapi.Message, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	command := message.Command()
	chatID := message.Chat.ID
	args := message.CommandArguments()

	switch command {
	case "start", "menu":
		// Cek status login terlebih dahulu
		if client == nil || client.Store.ID == nil {
			// Belum login - tampilkan prompt login
			ui.ShowLoginPrompt(telegramBot, chatID)
		} else {
			// Sudah login - tampilkan menu utama
			ui.ShowMainMenu(telegramBot, chatID, client)
		}

	case "help":
		helpText := `📖 **BANTUAN LENGKAP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **FITUR YANG TERSEDIA**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **INFORMASI**
   /status - Status bot WhatsApp
   /info - Informasi lengkap bot
   /time - Waktu sekarang

👥 **GRUP**
   /grup - Manajemen grup WhatsApp

🔧 **PENGATURAN**
   /pair <nomor> - Pairing WhatsApp
   /logout - Logout dan hapus data
   /help - Bantuan lengkap

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **CARA PENGGUNAAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

• Gunakan /menu untuk melihat menu utama
• Klik tombol inline untuk navigasi cepat
• Gunakan command di atas sesuai kebutuhan`
		msg := tgbotapi.NewMessage(chatID, helpText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "pair":
		phoneNumber := strings.TrimSpace(args)
		if phoneNumber == "" {
			// Jika tidak ada nomor, aktifkan mode input
			showPhoneInputPrompt(telegramBot, chatID)
			return
		}

		// Validasi nomor
		if err := ValidatePhoneNumber(phoneNumber); err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **Nomor Tidak Valid**\n\n%s\n\nGunakan format: /pair 628123456789", err))
			errorMsg.ParseMode = "Markdown"
			telegramBot.Send(errorMsg)
			return
		}

		// Cek apakah sudah login
		if client != nil && client.Store.ID != nil {
			msg := tgbotapi.NewMessage(chatID, "✅ Bot WhatsApp sudah login!\n\nGunakan /logout untuk logout terlebih dahulu jika ingin mengganti akun.")
			telegramBot.Send(msg)
			return
		}

		// Jalankan pairing di goroutine
		go func() {
			if err := PairDeviceViaTelegram(phoneNumber, chatID); err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **Error**\n\n%s", err))
				errorMsg.ParseMode = "Markdown"
				telegramBot.Send(errorMsg)
			}
		}()

	case "status":
		if client != nil && client.Store.ID != nil {
			status := fmt.Sprintf("✅ **Status Bot WhatsApp**\n\n• Terhubung: ✅\n• User ID: %s\n• Waktu: %s",
				client.Store.ID.User, time.Now().Format("15:04:05"))
			msg := tgbotapi.NewMessage(chatID, status)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
		}

	case "info":
		info := fmt.Sprintf(`🤖 **INFORMASI BOT**

WhatsApp: %s
Telegram: ✅ Connected
Waktu: %s`,
			func() string {
				if client != nil && client.Store.ID != nil {
					return "✅ Connected"
				}
				return "❌ Not Connected"
			}(), time.Now().Format("15:04:05"))
		msg := tgbotapi.NewMessage(chatID, info)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "ping":
		msg := tgbotapi.NewMessage(chatID, "🏓 Pong! Bot aktif!")
		telegramBot.Send(msg)

	case "time":
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🕐 Waktu: %s", time.Now().Format("15:04:05")))
		telegramBot.Send(msg)

	case "grup":
		// Handler untuk fitur grup
		if client == nil || client.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.\n\nGunakan /pair <nomor> untuk melakukan pairing terlebih dahulu.")
			telegramBot.Send(msg)
			return
		}

		// Tampilkan menu grup dengan inline keyboard
		showGroupMenu(telegramBot, chatID, client)

	case "logout":
		// Tampilkan konfirmasi logout dengan inline keyboard
		if client == nil || client.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum login.")
			telegramBot.Send(msg)
			return
		}
		// Tampilkan konfirmasi logout
		if err := LogoutWhatsApp(chatID); err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
			telegramBot.Send(errorMsg)
		}

	default:
		msg := tgbotapi.NewMessage(chatID, "❌ Command tidak dikenali. Gunakan /help untuk bantuan.")
		telegramBot.Send(msg)
	}
}

// HandleCallbackQuery memproses callback dari inline keyboard
func HandleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	chatID := callbackQuery.Message.Chat.ID
	data := callbackQuery.Data

	// Acknowledge callback query terlebih dahulu
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	telegramBot.Request(callback)

	switch data {
	case "status":
		if client != nil && client.Store.ID != nil {
			status := fmt.Sprintf("✅ **Status Bot WhatsApp**\n\n• Terhubung: ✅\n• User ID: %s\n• Waktu: %s",
				client.Store.ID.User, time.Now().Format("15:04:05"))
			msg := tgbotapi.NewMessage(chatID, status)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
		}

	case "info":
		info := fmt.Sprintf(`🤖 **INFORMASI BOT**

WhatsApp: %s
Telegram: ✅ Connected
Waktu: %s`,
			func() string {
				if client != nil && client.Store.ID != nil {
					return "✅ Connected"
				}
				return "❌ Not Connected"
			}(), time.Now().Format("15:04:05"))
		msg := tgbotapi.NewMessage(chatID, info)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "refresh":
		// Refresh - cek status dan tampilkan UI sesuai kondisi
		if client == nil || client.Store.ID == nil {
			// Belum login - tampilkan prompt login
			ui.ShowLoginPrompt(telegramBot, chatID)
		} else {
			// Sudah login - tampilkan menu utama
			ui.ShowMainMenu(telegramBot, chatID, client)
		}

	case "grup":
		// Handler untuk fitur grup dari inline keyboard
		if client == nil || client.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.\n\nGunakan /pair <nomor> untuk melakukan pairing terlebih dahulu.")
			telegramBot.Send(msg)
			return
		}

		// Tampilkan menu grup
		showGroupMenu(telegramBot, chatID, client)

	case "list_grup":
		// Handler untuk menampilkan daftar grup
		if client == nil || client.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}

		// Ambil dan tampilkan daftar grup
		go func() {
			if err := GetGroupList(client, telegramBot, chatID); err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
				telegramBot.Send(errorMsg)
			}
		}()

	case "help":
		helpText := `📖 **BANTUAN LENGKAP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **FITUR YANG TERSEDIA**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **INFORMASI**
   /status - Status bot WhatsApp
   /info - Informasi lengkap bot
   /time - Waktu sekarang

👥 **GRUP**
   /grup - Manajemen grup WhatsApp

🔧 **PENGATURAN**
   /pair <nomor> - Pairing WhatsApp
   /logout - Logout dan hapus data
   /help - Bantuan lengkap

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **CARA PENGGUNAAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

• Gunakan /menu untuk melihat menu utama
• Klik tombol inline untuk navigasi cepat
• Gunakan command di atas sesuai kebutuhan`
		msg := tgbotapi.NewMessage(chatID, helpText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "logout":
		// Logout dengan konfirmasi (dari tombol menu)
		if err := LogoutWhatsApp(chatID); err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
			telegramBot.Send(errorMsg)
		}

	case "logout_confirm":
		// Konfirmasi logout - hapus database
		go func() {
			if err := ConfirmLogout(chatID); err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error saat logout: %v", err))
				telegramBot.Send(errorMsg)
			}
		}()

	case "logout_cancel":
		// Batal logout
		msg := tgbotapi.NewMessage(chatID, "❌ Logout dibatalkan.")
		telegramBot.Send(msg)

	case "start_pairing":
		// Mulai pairing - set state dan tampilkan instruksi
		WaitingForPhoneNumber[chatID] = true

		// Cek apakah sudah login
		if client != nil && client.Store.ID != nil {
			WaitingForPhoneNumber[chatID] = false
			msg := tgbotapi.NewMessage(chatID, "✅ Bot WhatsApp sudah login!\n\nGunakan /logout untuk logout terlebih dahulu jika ingin mengganti akun.")
			telegramBot.Send(msg)
			return
		}

		showPhoneInputPrompt(telegramBot, chatID)

	case "back_to_login":
		// Kembali ke login prompt
		WaitingForPhoneNumber[chatID] = false
		ui.ShowLoginPrompt(telegramBot, chatID)

	case "login_info":
		// Info tentang login
		infoMsg := `ℹ️ **INFORMASI LOGIN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Apa itu Pairing?**
Pairing adalah proses menghubungkan bot dengan akun WhatsApp Anda.

**Mengapa perlu Pairing?**
• Untuk keamanan akun Anda
• Agar bot dapat mengakses WhatsApp Anda
• Untuk menerima dan mengirim pesan

**Setelah Pairing:**
• Bot akan terhubung ke WhatsApp Anda
• Anda dapat menggunakan semua fitur bot
• Data disimpan secara lokal dan aman`
		msg := tgbotapi.NewMessage(chatID, infoMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "login_help":
		// Help tentang login
		helpMsg := "❓ **BANTUAN LOGIN**\n\n" +
			"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n" +
			"**Cara melakukan Pairing:**\n\n" +
			"1️⃣ Klik \"🔗 Mulai Pairing\"\n" +
			"2️⃣ Masukkan nomor dengan format: `/pair 628123456789`\n" +
			"3️⃣ Ikuti instruksi yang diberikan\n" +
			"4️⃣ Masukkan kode pairing di WhatsApp\n" +
			"5️⃣ Selesai!\n\n" +
			"**Format Nomor:**\n" +
			"• Indonesia: 628123456789\n" +
			"• US: 14155552671\n" +
			"• UK: 447911123456\n\n" +
			"**Troubleshooting:**\n" +
			"• Pastikan nomor aktif\n" +
			"• Cek koneksi internet\n" +
			"• Coba lagi jika timeout"
		msg := tgbotapi.NewMessage(chatID, helpMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "cancel_pairing":
		// Batalkan pairing yang sedang berjalan
		WaitingForPhoneNumber[chatID] = false
		msg := tgbotapi.NewMessage(chatID, "❌ Pairing dibatalkan.\n\nGunakan tombol \"🔗 Mulai Pairing\" atau command `/pair <nomor>` untuk memulai ulang.")
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "cancel_phone_input":
		// Batalkan input nomor
		WaitingForPhoneNumber[chatID] = false
		msg := tgbotapi.NewMessage(chatID, "❌ Input nomor dibatalkan.")
		telegramBot.Send(msg)
		// Tampilkan kembali login prompt
		time.Sleep(500 * time.Millisecond)
		ui.ShowLoginPrompt(telegramBot, chatID)

	default:
		msg := tgbotapi.NewMessage(chatID, "❌ Tombol tidak dikenali.")
		telegramBot.Send(msg)
	}
}

// HandlePhoneNumberInput memproses input nomor telepon dari user
func HandlePhoneNumberInput(phoneNumber string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	// Reset state
	WaitingForPhoneNumber[chatID] = false

	// Cek apakah sudah login
	if client != nil && client.Store.ID != nil {
		msg := tgbotapi.NewMessage(chatID, "✅ Bot WhatsApp sudah login!\n\nGunakan /logout untuk logout terlebih dahulu jika ingin mengganti akun.")
		telegramBot.Send(msg)
		return
	}

	// Validasi nomor
	if err := ValidatePhoneNumber(phoneNumber); err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **Nomor Tidak Valid**\n\n%s\n\nSilakan masukkan nomor yang benar atau klik tombol di bawah untuk mencoba lagi.", err))
		errorMsg.ParseMode = "Markdown"

		// Tampilkan tombol untuk mencoba lagi
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔗 Coba Lagi", "start_pairing"),
			),
		)
		errorMsg.ReplyMarkup = keyboard
		telegramBot.Send(errorMsg)
		return
	}

	// Proses pairing
	go func(phone string, cID int64) {
		if err := PairDeviceViaTelegram(phone, cID); err != nil {
			errorMsg := tgbotapi.NewMessage(cID, fmt.Sprintf("❌ **Error**\n\n%s", err))
			errorMsg.ParseMode = "Markdown"
			telegramBot.Send(errorMsg)
		}
	}(phoneNumber, chatID)
}

// showPhoneInputPrompt menampilkan prompt input nomor
func showPhoneInputPrompt(telegramBot *tgbotapi.BotAPI, chatID int64) {
	inputMsg := "📱 **MASUKKAN NOMOR TELEPON**\n\n" +
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n" +
		"✅ **Mode Input Aktif**\n\n" +
		"Sekarang Anda cukup **mengetik nomor telepon** Anda saja.\n\n" +
		"**Contoh:**\n" +
		"`628123456789`\n" +
		"`14155552671`\n\n" +
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n" +
		"💡 **Format Nomor:**\n" +
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n" +
		"• Gunakan kode negara (tanpa + atau 0)\n" +
		"• Minimal 10 digit, maksimal 15 digit\n" +
		"• Hanya angka\n\n" +
		"**Contoh:**\n" +
		"• Indonesia: `628123456789`\n" +
		"• US: `14155552671`\n" +
		"• UK: `447911123456`\n\n" +
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⏳ Menunggu input nomor..."

	msg := tgbotapi.NewMessage(chatID, inputMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_phone_input"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}
