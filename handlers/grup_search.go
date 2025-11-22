package handlers

import (
	"fmt"
	"strings"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// WaitingForSearch state management untuk input search
var WaitingForSearch = make(map[int64]bool)

// ShowSearchPrompt menampilkan prompt untuk search grup
func ShowSearchPrompt(telegramBot *tgbotapi.BotAPI, chatID int64) {
	searchMsg := `🔍 **CARI GRUP WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Search Aktif**

Ketik nama grup atau kata kunci yang ingin Anda cari.

**Contoh:**
• Keluarga
• Kerja
• Teman

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **Tips:**
• Pencarian tidak case-sensitive
• Cukup ketik sebagian nama grup
• Hasil akan ditampilkan langsung
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input...`

	msg := tgbotapi.NewMessage(chatID, searchMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_search"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
	WaitingForSearch[chatID] = true
}

// ShowSearchPromptEdit menampilkan prompt search dengan EDIT message (no spam!)
func ShowSearchPromptEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	searchMsg := `🔍 **CARI GRUP WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Search Aktif**

Ketik nama grup atau kata kunci yang ingin Anda cari.

**Contoh:**
• Keluarga
• Kerja
• Teman

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **Tips:**
• Pencarian tidak case-sensitive
• Cukup ketik sebagian nama grup
• Hasil akan ditampilkan langsung
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input...`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_search"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, searchMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)

	WaitingForSearch[chatID] = true
}

// HandleSearchInput memproses input search dari user
func HandleSearchInput(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	WaitingForSearch[chatID] = false

	if strings.TrimSpace(keyword) == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Kata kunci tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// CRITICAL FIX: Pastikan menggunakan database user yang benar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)
	if userAccount != nil {
		EnsureDBConfigForUser(chatID, userAccount)
	}

	// Show loading
	loadingMsg := tgbotapi.NewMessage(chatID, "🔍 Mencari grup...")
	loadingMsgSent, _ := telegramBot.Send(loadingMsg)

	// Search groups
	groups, err := utils.SearchGroups(keyword)
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// Delete loading message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
	telegramBot.Request(deleteMsg)

	if len(groups) == 0 {
		noResultMsg := fmt.Sprintf(`🔍 **HASIL PENCARIAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

❌ Tidak ada grup yang cocok dengan kata kunci: **"%s"**

**Saran:**
• Coba kata kunci yang berbeda
• Periksa ejaan kata kunci
• Gunakan kata kunci yang lebih umum

Gunakan /grup untuk kembali ke menu grup.`, keyword)

		msg := tgbotapi.NewMessage(chatID, noResultMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	// Format results
	resultMsg := fmt.Sprintf(`🔍 **HASIL PENCARIAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Kata Kunci:** "%s"
✅ **Ditemukan:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

`, keyword, len(groups))

	count := 1
	for jid, name := range groups {
		groupEntry := fmt.Sprintf("**%d.** %s\n    `%s`\n\n", count, escapeMarkdownV2(name), jid)

		// Check message length
		if len(resultMsg)+len(groupEntry) > 3500 {
			msg := tgbotapi.NewMessage(chatID, resultMsg)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
			resultMsg = ""
		}

		resultMsg += groupEntry
		count++
	}

	if resultMsg != "" {
		msg := tgbotapi.NewMessage(chatID, resultMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Lagi", "search_grup"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	}
}

// escapeMarkdownV2 escapes special characters for Markdown
func escapeMarkdownV2(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}
