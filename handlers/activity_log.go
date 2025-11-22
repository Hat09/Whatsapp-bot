package handlers

import (
	"fmt"
	"time"

	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ShowActivityLog menampilkan activity log
func ShowActivityLog(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	// SECURITY: Validasi bahwa user memiliki akun terdaftar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)

	if userAccount == nil {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar, akses activity log ditolak", chatID)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan /pair untuk melakukan pairing terlebih dahulu.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	// Pastikan menggunakan database user yang benar
	if err := SwitchAccount(userAccount.ID, telegramBot, chatID); err != nil {
		utils.GetLogger().Warn("Security: Failed to switch to user account for activity log: %v", err)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	// ✅ AMAN: Pass chatID untuk filter per user (keamanan multi-user)
	logs, err := utils.GetActivityLogs(chatID, 20)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal memuat activity log")
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, errorMsg)
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	if len(logs) == 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "📜 **ACTIVITY LOG**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n📭 Belum ada aktivitas yang tercatat.\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	logMsg := fmt.Sprintf(`📜 **ACTIVITY LOG**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Menampilkan %d aktivitas terakhir**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

`, len(logs))

	for i, log := range logs {
		if i >= 20 {
			break
		}

		statusIcon := "✅"
		if !log.Success {
			statusIcon = "❌"
		}

		timeStr := log.CreatedAt.Format("02/01 15:04")
		actionName := getActionName(log.Action)

		logMsg += fmt.Sprintf("%s **%s**\n", statusIcon, actionName)
		if log.Description != "" {
			if len(log.Description) > 50 {
				logMsg += fmt.Sprintf("   %s...\n", log.Description[:50])
			} else {
				logMsg += fmt.Sprintf("   %s\n", log.Description)
			}
		}
		logMsg += fmt.Sprintf("   🕐 %s\n\n", timeStr)
	}

	logMsg += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	logMsg += "💡 Log menunjukkan aktivitas 7 hari terakhir"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "activity_log"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Statistics", "activity_stats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, logMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowActivityStats menampilkan statistik aktivitas
func ShowActivityStats(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	// SECURITY: Validasi bahwa user memiliki akun terdaftar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)

	if userAccount == nil {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar, akses activity stats ditolak", chatID)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan /pair untuk melakukan pairing terlebih dahulu.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	// Pastikan menggunakan database user yang benar
	if err := SwitchAccount(userAccount.ID, telegramBot, chatID); err != nil {
		utils.GetLogger().Warn("Security: Failed to switch to user account for activity stats: %v", err)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	// ✅ AMAN: Pass chatID untuk filter per user (keamanan multi-user)
	stats, err := utils.GetActivityStats(chatID, 7)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal memuat statistik")
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, errorMsg)
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

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

	successRate := 0.0
	if totalActivities > 0 {
		successRate = float64(successCount) / float64(totalActivities) * 100
	}

	statsMsg := fmt.Sprintf(`📊 **STATISTIK AKTIVITAS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📅 **Periode:** 7 Hari Terakhir

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📈 **RINGKASAN**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📋 Total Aktivitas: %d
✅ Berhasil: %d
❌ Gagal: %d
📊 Success Rate: %.1f%%

`, totalActivities, successCount, failedCount, successRate)

	// Top actions
	if topActions, ok := stats["top_actions"].(map[string]int); ok && len(topActions) > 0 {
		statsMsg += "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓\n"
		statsMsg += "┃ 🔝 **TOP AKTIVITAS**\n"
		statsMsg += "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛\n\n"

		count := 0
		for action, countVal := range topActions {
			if count >= 5 {
				break
			}
			actionName := getActionName(action)
			statsMsg += fmt.Sprintf("• %s: %d\n", actionName, countVal)
			count++
		}
		statsMsg += "\n"
	}

	statsMsg += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"
	statsMsg += fmt.Sprintf("🕐 Diperbarui: %s", time.Now().Format("02/01/2006 15:04:05"))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📜 Activity Log", "activity_log"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "activity_stats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, statsMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// getActionName mengembalikan nama yang lebih user-friendly untuk action
func getActionName(action string) string {
	actionMap := map[string]string{
		"delete_account":         "Hapus Akun",
		"delete_account_confirm": "Konfirmasi Hapus Akun",
		"delete_account_cancel":  "Batal Hapus Akun",
		"delete_account_error":   "Error Hapus Akun",
		"delete_account_success": "Sukses Hapus Akun",
		"backup_account":         "Backup Akun",
		"admin_unadmin":          "Atur Admin",
		"create_group":           "Buat Grup",
		"join_group":             "Join Grup",
		"leave_group":            "Leave Grup",
		"change_group_photo":     "Ganti Foto Grup",
		"change_group_desc":      "Ubah Deskripsi Grup",
		"change_group_settings":  "Ubah Pengaturan Grup",
		"get_group_link":         "Ambil Link Grup",
		"pair_device":            "Pairing Device",
		"logout":                 "Logout",
		"reset_program":          "Reset Program",
		"switch_account":         "Ganti Akun",
		"login_account":          "Login Akun",
	}

	if name, ok := actionMap[action]; ok {
		return name
	}

	// Default: capitalize first letter
	if len(action) > 0 {
		return fmt.Sprintf("%c%s", action[0]-32, action[1:])
	}

	return action
}
