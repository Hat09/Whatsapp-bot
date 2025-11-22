package handlers

import (
	"fmt"
	"os"
	"strings"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ExportGroupList mengexport daftar grup ke file
func ExportGroupList(telegramBot *tgbotapi.BotAPI, chatID int64, format string) {
	// SECURITY: Validasi bahwa user memiliki akun terdaftar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)

	if userAccount == nil {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar, akses export ditolak", chatID)
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan /pair untuk melakukan pairing terlebih dahulu.")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Pastikan menggunakan database user yang benar
	if err := SwitchAccount(userAccount.ID, telegramBot, chatID); err != nil {
		utils.GetLogger().Warn("Security: Failed to switch to user account for export: %v", err)
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// CRITICAL FIX: Pastikan dbConfig di-update setelah switch
	EnsureDBConfigForUser(chatID, userAccount)

	// Show loading
	loadingMsg := tgbotapi.NewMessage(chatID, "📥 Mempersiapkan export...")
	loadingMsgSent, _ := telegramBot.Send(loadingMsg)

	// Get all groups
	groups, err := utils.GetAllGroupsFromDB()
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	if len(groups) == 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		noDataMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup untuk di-export.")
		telegramBot.Send(noDataMsg)
		return
	}

	// Sort groups naturally before export
	sortedGroups := utils.SortGroupsNaturally(groups)

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	var content strings.Builder

	if format == "csv" {
		filename = fmt.Sprintf("whatsapp_groups_%s.csv", timestamp)
		content.WriteString("No,Nama Grup,JID\n")

		count := 1
		for _, group := range sortedGroups {
			// Escape CSV special characters
			escapedName := strings.ReplaceAll(group.Name, "\"", "\"\"")
			if strings.Contains(escapedName, ",") || strings.Contains(escapedName, "\"") {
				escapedName = fmt.Sprintf("\"%s\"", escapedName)
			}
			content.WriteString(fmt.Sprintf("%d,%s,%s\n", count, escapedName, group.JID))
			count++
		}
	} else {
		// TXT format
		filename = fmt.Sprintf("whatsapp_groups_%s.txt", timestamp)
		content.WriteString("=" + strings.Repeat("=", 60) + "\n")
		content.WriteString("          DAFTAR GRUP WHATSAPP\n")
		content.WriteString("=" + strings.Repeat("=", 60) + "\n\n")
		content.WriteString(fmt.Sprintf("Tanggal Export: %s\n\n", time.Now().Format("02 January 2006 15:04:05")))
		content.WriteString("=" + strings.Repeat("=", 60) + "\n\n")

		count := 1
		for _, group := range sortedGroups {
			content.WriteString(fmt.Sprintf("%d. %s\n", count, group.Name))
			content.WriteString(fmt.Sprintf("   JID: %s\n\n", group.JID))
			count++
		}

		content.WriteString("=" + strings.Repeat("=", 60) + "\n")
		content.WriteString("Export by WhatsApp Bot\n")
	}

	// Write to file
	err = os.WriteFile(filename, []byte(content.String()), 0644)
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal membuat file: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// Update loading message
	updateMsg := tgbotapi.NewEditMessageText(chatID, loadingMsgSent.MessageID, "📤 Mengirim file...")
	updateMsg.ParseMode = "Markdown"
	telegramBot.Send(updateMsg)

	// Send file
	fileMsg := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(filename))
	fileMsg.Caption = fmt.Sprintf("📥 **Export Grup WhatsApp**\n\n"+
		"📊 Total: %d grup\n"+
		"📅 %s\n"+
		"📄 Format: %s",
		len(groups),
		time.Now().Format("02 Jan 2006 15:04"),
		strings.ToUpper(format))
	fileMsg.ParseMode = "Markdown"

	_, err = telegramBot.Send(fileMsg)
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal mengirim file: %v", err))
		telegramBot.Send(errorMsg)
	}

	// Delete loading message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
	telegramBot.Request(deleteMsg)

	// Delete temporary file
	os.Remove(filename)

	// Send success message with options
	successMsg := tgbotapi.NewMessage(chatID, "✅ Export berhasil!")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 Export Lagi", "export_grup"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
		),
	)
	successMsg.ReplyMarkup = keyboard
	telegramBot.Send(successMsg)
}

// ShowExportMenu menampilkan menu export dengan pilihan format
func ShowExportMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	exportMsg := `📥 **EXPORT DAFTAR GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Pilih format file yang ingin Anda export:

📄 **TXT** - Format teks biasa
• Mudah dibaca
• Lengkap dengan detail
• Cocok untuk dokumentasi

📊 **CSV** - Format spreadsheet
• Bisa dibuka di Excel/Sheets
• Mudah diolah datanya
• Cocok untuk analisis

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 File akan dikirim langsung ke chat ini
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	msg := tgbotapi.NewMessage(chatID, exportMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📄 Export TXT", "export_txt"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Export CSV", "export_csv"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowExportMenuEdit menampilkan menu export dengan EDIT message (no spam!)
func ShowExportMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	exportMsg := `📥 **EXPORT DAFTAR GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Pilih format file yang ingin Anda export:

📄 **TXT** - Format teks biasa
• Mudah dibaca
• Lengkap dengan detail
• Cocok untuk dokumentasi

📊 **CSV** - Format spreadsheet
• Bisa dibuka di Excel/Sheets
• Mudah diolah datanya
• Cocok untuk analisis

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 File akan dikirim langsung ke chat ini
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📄 Export TXT", "export_txt"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Export CSV", "export_csv"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exportMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}
