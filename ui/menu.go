package ui

import (
	"fmt"
	"strings"

	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// ShowMainMenu menampilkan menu utama dengan semua fitur
func ShowMainMenu(bot *tgbotapi.BotAPI, chatID int64, waClient *whatsmeow.Client) {
	var status, phoneNumber string
	var statusIcon string

	if waClient != nil && waClient.Store.ID != nil {
		status = "✅ Terhubung"
		statusIcon = "🟢"
		phoneNumber = waClient.Store.ID.User
	} else {
		status = "❌ Belum Terhubung"
		statusIcon = "🔴"
		phoneNumber = "-"
	}

	// ✅ AMAN: Get activity statistics untuk user tertentu (last 7 days)
	// FIXED: Pass chatID untuk filter per user (keamanan multi-user)
	stats, _ := utils.GetActivityStats(chatID, 7)
	totalActivities := 0
	successCount := 0
	failedCount := 0
	if val, ok := stats["total_activities"]; ok {
		totalActivities = val.(int)
	}
	if val, ok := stats["success_count"]; ok {
		successCount = val.(int)
	}
	if val, ok := stats["failed_count"]; ok {
		failedCount = val.(int)
	}

	// Get connection status
	connectionStatus := "❌ Terputus"
	if waClient != nil && waClient.IsConnected() {
		connectionStatus = "🟢 Terhubung"
	}

	// Get location info untuk user
	country, city := utils.GetLocationForUserSafe(chatID)

	// Get time info berdasarkan timezone user
	timeStr := utils.FormatTimeForUserSafe(chatID, "15:04:05")
	dateStr := utils.FormatTimeForUserSafe(chatID, "02 Jan 2006")

	menu := fmt.Sprintf(`╔═══════════════════════════════╗
║      🎯 **DASHBOARD UTAMA**      ║
╚═══════════════════════════════╝

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📊 **STATUS & STATISTIK**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

%s **WhatsApp Bot:** %s
%s **Nomor:** +%s
%s **Koneksi:** %s
🟢 **Telegram Bot:** Aktif
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📈 **AKTIVITAS (7 Hari)**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📋 Total: %d aktivitas
✅ Berhasil: %d
❌ Gagal: %d

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ ⚡ **QUICK ACTIONS**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

👥 Kelola grup WhatsApp
🔍 Cari & filter grup
📥 Export daftar grup
📜 Activity Log
❓ Bantuan & dokumentasi

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🕐 %s | 📅 %s
🌍 %s, %s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
		statusIcon, status,
		statusIcon, phoneNumber,
		statusIcon, connectionStatus,
		totalActivities,
		successCount,
		failedCount,
		timeStr,
		dateStr,
		city,
		country)

	msg := tgbotapi.NewMessage(chatID, menu)
	msg.ParseMode = "Markdown"

	// Tambahkan inline keyboard untuk navigasi cepat
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Grup", "grup"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "refresh"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Login WhatsApp Baru", "multi_account_menu"),
			tgbotapi.NewInlineKeyboardButtonData("📜 Activity Log", "activity_log"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ Help", "help"),
		),
	)

	// Tambahkan tombol logout dan reset jika sudah login
	if waClient != nil && waClient.Store.ID != nil {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🚪 Logout", "logout"),
				tgbotapi.NewInlineKeyboardButtonData("🔄 Reset Program", "reset_program"),
			),
		)
	}
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

// ShowMainMenuEdit menampilkan menu utama dengan EDIT message (no spam!)
func ShowMainMenuEdit(bot *tgbotapi.BotAPI, chatID int64, messageID int, waClient *whatsmeow.Client) {
	var status, phoneNumber string
	var statusIcon string

	if waClient != nil && waClient.Store.ID != nil {
		status = "✅ Terhubung"
		statusIcon = "🟢"
		phoneNumber = waClient.Store.ID.User
	} else {
		status = "❌ Belum Terhubung"
		statusIcon = "🔴"
		phoneNumber = "-"
	}

	// ✅ AMAN: Get activity statistics untuk user tertentu (last 7 days)
	// FIXED: Pass chatID untuk filter per user (keamanan multi-user)
	stats, _ := utils.GetActivityStats(chatID, 7)
	totalActivities := 0
	successCount := 0
	failedCount := 0
	if val, ok := stats["total_activities"]; ok {
		totalActivities = val.(int)
	}
	if val, ok := stats["success_count"]; ok {
		successCount = val.(int)
	}
	if val, ok := stats["failed_count"]; ok {
		failedCount = val.(int)
	}

	// Get connection status
	connectionStatus := "❌ Terputus"
	if waClient != nil && waClient.IsConnected() {
		connectionStatus = "🟢 Terhubung"
	}

	// Get location info untuk user
	country, city := utils.GetLocationForUserSafe(chatID)

	// Get time info berdasarkan timezone user
	timeStr := utils.FormatTimeForUserSafe(chatID, "15:04:05")
	dateStr := utils.FormatTimeForUserSafe(chatID, "02 Jan 2006")

	menu := fmt.Sprintf(`╔═══════════════════════════════╗
║      🎯 **DASHBOARD UTAMA**      ║
╚═══════════════════════════════╝

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📊 **STATUS & STATISTIK**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

%s **WhatsApp Bot:** %s
%s **Nomor:** +%s
%s **Koneksi:** %s
🟢 **Telegram Bot:** Aktif
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📈 **AKTIVITAS (7 Hari)**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📋 Total: %d aktivitas
✅ Berhasil: %d
❌ Gagal: %d

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ ⚡ **QUICK ACTIONS**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

👥 Kelola grup WhatsApp
🔍 Cari & filter grup
📥 Export daftar grup
📜 Activity Log
❓ Bantuan & dokumentasi

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🕐 %s | 📅 %s
🌍 %s, %s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
		statusIcon, status,
		statusIcon, phoneNumber,
		statusIcon, connectionStatus,
		totalActivities,
		successCount,
		failedCount,
		timeStr,
		dateStr,
		city,
		country)

	// Tambahkan inline keyboard untuk navigasi cepat
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Grup", "grup"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "refresh"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📱 Login WhatsApp Baru", "multi_account_menu"),
			tgbotapi.NewInlineKeyboardButtonData("📜 Activity Log", "activity_log"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ Help", "help"),
		),
	)

	// Tambahkan tombol logout dan reset jika sudah login
	if waClient != nil && waClient.Store.ID != nil {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🚪 Logout", "logout"),
				tgbotapi.NewInlineKeyboardButtonData("🔄 Reset Program", "reset_program"),
			),
		)
	}

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menu)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	bot.Send(editMsg)
}

// ShowLoginPromptEdit menampilkan prompt login dengan EDIT message (no spam!)
func ShowLoginPromptEdit(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	welcomeMsg := `╔═══════════════════════════════╗
║   🤖 **WHATSAPP BOT MANAGER**   ║
╚═══════════════════════════════╝

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📱 **STATUS**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

🔴 **WhatsApp:** Belum Terhubung
🟢 **Telegram:** Aktif

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 🚀 **MULAI**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

Hubungkan bot dengan akun WhatsApp Anda untuk mengakses semua fitur manajemen grup.

💡 **Fitur Tersedia:**
• Kelola grup WhatsApp
• Export daftar grup
• Ambil link undangan grup
• Dan masih banyak lagi!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

👇 Klik tombol di bawah untuk memulai pairing`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 Mulai Pairing", "start_pairing"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Info Login", "login_info"),
			tgbotapi.NewInlineKeyboardButtonData("❓ Bantuan", "login_help"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "refresh"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, welcomeMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	bot.Send(editMsg)
}

// FormatPairingInstructions memformat instruksi pairing dengan lebih menarik
func FormatPairingInstructions(pairingCode string, phoneNumber string) string {
	return fmt.Sprintf(`🔗 **PAIRING WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📱 **LANGKAH-LANGKAH PAIRING**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Nomor:** %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **INSTRUKSI:**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1️⃣ Buka aplikasi WhatsApp di HP Anda
2️⃣ Ketuk menu (⋮) di pojok kanan atas
3️⃣ Pilih **Settings** → **Linked Devices**
4️⃣ Ketuk **Link a Device**
5️⃣ Pindai QR code ATAU ketuk **Link with phone number instead**
6️⃣ Masukkan kode pairing berikut:

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔑 **KODE PAIRING:**

   ┌─────────────┐
   │  %s  │
   └─────────────┘
   
💡 **Format kode:** XXX-XXX-XXX

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⏳ **STATUS:** Menunggu pairing...
⏱️ **TIMEOUT:** 2 menit

💡 **Tips:**
• Pastikan HP dan server terhubung internet
• Kode hanya berlaku selama 2 menit
• Jangan tutup chat ini selama proses pairing`, phoneNumber, strings.ToUpper(pairingCode))
}

// FormatPairingSuccess memformat pesan success pairing
func FormatPairingSuccess() string {
	return `✅ **PAIRING BERHASIL!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🎉 Bot WhatsApp sudah terhubung dan siap digunakan!

Anda sekarang dapat:
• Menerima notifikasi pesan WhatsApp
• Mengirim pesan melalui Telegram
• Menggunakan semua fitur bot

Gunakan /menu untuk melihat menu utama.`
}
