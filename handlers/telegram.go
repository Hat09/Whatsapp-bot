package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

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

	// CRITICAL FIX: Gunakan UserSession untuk isolasi data per user
	// Ini memastikan setiap user memiliki session terpisah dan tidak saling mengganggu
	userSession, err := GetUserSession(int64(chatID), telegramBot)
	if err != nil {
		utils.GetLogger().Warn("Failed to get user session (TelegramID: %d): %v", chatID, err)
		// Untuk command /start, /menu, dan /pair, tetap izinkan akses
		if command != "start" && command != "menu" && command != "pair" {
			msg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
			return
		}
	}

	// Fallback ke metode lama jika session tidak tersedia (untuk backward compatibility)
	var userAccount *WhatsAppAccount
	if userSession == nil {
		// Coba gunakan EnsureUserAccountActive sebagai fallback
		userAccount, err = EnsureUserAccountActive(int64(chatID), telegramBot)
		if err != nil {
			am := GetAccountManager()
			userAccount = am.GetAccountByTelegramID(int64(chatID))
			if userAccount == nil && command != "start" && command != "menu" && command != "pair" {
				msg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
				msg.ParseMode = "Markdown"
				telegramBot.Send(msg)
				return
			}
		}
	} else {
		// Gunakan account dari session
		userAccount = userSession.Account
	}

	// CRITICAL: Tolak akses jika user belum memiliki akun terdaftar
	// Jangan fallback ke current account (admin) untuk keamanan
	// KECUALI untuk command /pair, /start, dan /menu yang memang untuk pairing/login
	// Command /start dan /menu harus bisa diakses untuk menampilkan login prompt
	if userAccount == nil && command != "pair" && command != "start" && command != "menu" {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar, akses ditolak untuk command: %s", chatID, command)
		msg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan /pair untuk melakukan pairing terlebih dahulu.")
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	// IMPORTANT: Selalu gunakan client dari akun aktif
	activeClient := GetWhatsAppClient()
	if activeClient == nil {
		activeClient = client // Fallback hanya jika benar-benar tidak ada
	}

	switch command {
	case "start", "menu":
		// CRITICAL FIX: Handle user yang belum punya akun dengan benar
		// Jika user belum punya akun (userAccount == nil), langsung tampilkan login prompt
		if userAccount == nil {
			// User belum punya akun - tampilkan login prompt
			ui.ShowLoginPrompt(telegramBot, chatID)
			return
		}

		// CRITICAL FIX: Gunakan session untuk isolasi data per user
		// Jika session tersedia, gunakan client dan account dari session
		var userClient *whatsmeow.Client
		var accountToUse *WhatsAppAccount

		if userSession != nil && userSession.Account != nil {
			// Gunakan data dari session (lebih aman dan terisolasi)
			userClient = userSession.Client
			accountToUse = userSession.Account
			utils.GetLogger().Info("Using session data: TelegramID=%d, AccountID=%d, Phone=%s",
				userSession.TelegramID, userSession.AccountID, userSession.Account.PhoneNumber)
		} else if userAccount != nil {
			// Fallback: gunakan userAccount dari EnsureUserAccountActive
			am := GetAccountManager()
			userClient = am.GetClient(userAccount.ID)
			if userClient == nil {
				var err error
				userClient, err = am.CreateClient(userAccount.ID)
				if err != nil {
					utils.GetLogger().Warn("Failed to create client for user account %d: %v", userAccount.ID, err)
					userClient = activeClient
				}
			}
			accountToUse = userAccount
		}

		// User sudah punya akun - cek status login
		if userClient == nil || userClient.Store.ID == nil {
			// Belum login - tampilkan prompt login
			ui.ShowLoginPrompt(telegramBot, chatID)
		} else {
			// Sudah login - tampilkan menu utama
			// CRITICAL FIX: Gunakan account dari session atau userAccount
			telegramID := int64(chatID) // Default: gunakan chatID
			if accountToUse != nil && accountToUse.BotDataDBPath != "" {
				re := regexp.MustCompile(`bot_data\((\d+)\)>`)
				matches := re.FindStringSubmatch(accountToUse.BotDataDBPath)
				if len(matches) >= 2 {
					if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
						telegramID = parsedID
					}
				}
				// Update dbConfig dengan Telegram ID yang benar untuk memastikan isolasi data per user
				utils.SetDBConfig(telegramID, accountToUse.PhoneNumber)
				// Reset database pool untuk memastikan menggunakan database yang benar
				utils.CloseDBPools()

				// Log untuk debug
				utils.GetLogger().Info("ShowMainMenu for user: TelegramID=%d, AccountID=%d, Phone=%s, DBPath=%s",
					telegramID, accountToUse.ID, accountToUse.PhoneNumber, accountToUse.BotDataDBPath)
			}

			// Gunakan userClient dari session atau account yang benar
			ui.ShowMainMenu(telegramBot, chatID, userClient)
		}

	case "help":
		helpText := `📖 **BANTUAN LENGKAP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **FITUR YANG TERSEDIA**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

👥 **GRUP**
   /grup - Manajemen grup WhatsApp

🔧 **PENGATURAN**
   /pair <nomor> - Pairing WhatsApp
   /logout - Logout akun saat ini
   /reset - Reset program (hapus semua data)
   /help - Bantuan lengkap

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **CARA PENGGUNAAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

• Gunakan /menu untuk melihat menu utama
• Klik tombol inline untuk navigasi cepat
• Gunakan command di atas sesuai kebutuhan

⚠️ **CATATAN:**
• /logout - Hanya logout akun aktif saat ini
• /reset - Hapus SEMUA data (semua akun, semua database)`
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

		// CRITICAL FIX: Cek apakah user ini sudah punya account dan sudah login
		// Gunakan userAccount dan userClient dari session user yang benar
		var userClientForPair *whatsmeow.Client
		if userSession != nil && userSession.Account != nil {
			userClientForPair = userSession.Client
		} else if userAccount != nil {
			am := GetAccountManager()
			userClientForPair = am.GetClient(userAccount.ID)
		}

		if userAccount != nil && userClientForPair != nil && userClientForPair.Store != nil && userClientForPair.Store.ID != nil {
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

	case "grup":
		// Handler untuk fitur grup
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.\n\nGunakan /pair <nomor> untuk melakukan pairing terlebih dahulu.")
			telegramBot.Send(msg)
			return
		}

		// Tampilkan menu grup dengan inline keyboard
		showGroupMenu(telegramBot, chatID, activeClient)

	case "logout":
		// Tampilkan konfirmasi logout dengan inline keyboard
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum login.")
			telegramBot.Send(msg)
			return
		}
		// Tampilkan konfirmasi logout
		if err := LogoutWhatsApp(chatID); err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
			telegramBot.Send(errorMsg)
		}

	case "stopchat":
		// Stop broadcast jika sedang berjalan
		if IsBroadcastRunning(chatID) {
			StopBroadcast(chatID)
			msg := tgbotapi.NewMessage(chatID, "⏹️ **Broadcast dihentikan**\n\nBroadcast akan berhenti setelah selesai mengirim pesan saat ini.")
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "ℹ️ Tidak ada broadcast yang sedang berjalan.")
			telegramBot.Send(msg)
		}

	case "mulai":
		// Handle /mulai command untuk broadcast
		state := GetBroadcastState(chatID)
		if state != nil {
			// Jika sedang dalam state broadcast manual, proses input /mulai
			if state.WaitingForMessageManual {
				HandleManualMessageInput("/mulai", chatID, telegramBot)
				return
			}
			// Jika sedang dalam state target groups (manual atau file), proses input /mulai
			if state.WaitingForTargetGroups {
				HandleTargetGroupsInput("/mulai", chatID, telegramBot)
				return
			}
		}
		// Jika tidak dalam state broadcast, beri pesan error
		msg := tgbotapi.NewMessage(chatID, "ℹ️ Command ini hanya dapat digunakan saat setup broadcast.")
		telegramBot.Send(msg)

	default:
		msg := tgbotapi.NewMessage(chatID, "❌ Command tidak dikenali. Gunakan /help untuk bantuan.")
		telegramBot.Send(msg)
	}
}

// HandleCallbackQuery memproses callback dari inline keyboard
func HandleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	chatID := callbackQuery.Message.Chat.ID
	messageID := callbackQuery.Message.MessageID
	data := callbackQuery.Data

	// Debug: log callback data
	fmt.Printf("[DEBUG] Callback received: chatID=%d, messageID=%d, data=%s\n", chatID, messageID, data)

	// Acknowledge callback query terlebih dahulu
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	telegramBot.Request(callback)

	// CRITICAL FIX: Gunakan UserSession untuk isolasi data per user di callback handler
	// Ini memastikan setiap user memiliki session terpisah dan tidak saling mengganggu
	userSession, err := GetUserSession(int64(chatID), telegramBot)
	if err != nil {
		utils.GetLogger().Warn("Failed to get user session (TelegramID: %d): %v", chatID, err)
		// Untuk callback pairing, tetap izinkan akses
		if data != "start_pairing" && data != "back_to_login" && data != "login_info" && data != "login_help" {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
			editMsg.ParseMode = "Markdown"
			telegramBot.Send(editMsg)
			return
		}
	}

	// Fallback ke metode lama jika session tidak tersedia (untuk backward compatibility)
	var userAccount *WhatsAppAccount
	var userClient *whatsmeow.Client
	if userSession == nil {
		// Coba gunakan EnsureUserAccountActive sebagai fallback
		userAccount, err = EnsureUserAccountActive(int64(chatID), telegramBot)
		if err != nil {
			am := GetAccountManager()
			userAccount = am.GetAccountByTelegramID(int64(chatID))
		}
		if userAccount != nil {
			am := GetAccountManager()
			userClient = am.GetClient(userAccount.ID)
			if userClient == nil {
				var err error
				userClient, err = am.CreateClient(userAccount.ID)
				if err != nil {
					utils.GetLogger().Warn("Failed to create client for user account %d: %v", userAccount.ID, err)
					userClient = client // Fallback
				}
			}
		}
	} else {
		// Gunakan data dari session (lebih aman dan terisolasi)
		userAccount = userSession.Account
		userClient = userSession.Client
		utils.GetLogger().Info("Callback using session: TelegramID=%d, AccountID=%d, Phone=%s",
			userSession.TelegramID, userSession.AccountID, userSession.Account.PhoneNumber)
	}

	// CRITICAL: Tolak akses jika user belum memiliki akun terdaftar
	// KECUALI untuk callback yang memang untuk pairing baru
	if userAccount == nil && data != "start_pairing" && data != "back_to_login" && data != "login_info" && data != "login_help" {
		utils.GetLogger().Warn("Security: User %d tidak memiliki akun terdaftar, akses ditolak untuk callback: %s", chatID, data)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan tombol \"🔗 Mulai Pairing\" untuk melakukan pairing terlebih dahulu.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
		return
	}

	// CRITICAL FIX: Gunakan userClient dari session, bukan activeClient yang bisa dari user lain
	// Fallback ke client parameter hanya jika benar-benar tidak ada
	activeClient := userClient
	if activeClient == nil {
		activeClient = GetWhatsAppClient()
		if activeClient == nil {
			activeClient = client // Fallback terakhir
		}
	}

	switch data {
	case "menu":
		// CRITICAL FIX: Gunakan session untuk isolasi data per user
		var menuClient *whatsmeow.Client
		var accountForMenu *WhatsAppAccount

		if userSession != nil && userSession.Account != nil {
			// Gunakan data dari session
			menuClient = userSession.Client
			accountForMenu = userSession.Account
		} else if userAccount != nil {
			// Fallback: gunakan userAccount
			menuClient = userClient
			accountForMenu = userAccount
		}

		if menuClient == nil || menuClient.Store.ID == nil {
			// Belum login - tampilkan prompt login
			ui.ShowLoginPromptEdit(telegramBot, chatID, messageID)
		} else {
			// Sudah login - tampilkan menu utama
			if accountForMenu != nil {
				// Parse Telegram ID dari BotDataDBPath untuk memastikan dbConfig benar
				telegramID := int64(chatID) // Default: gunakan chatID
				if accountForMenu.BotDataDBPath != "" {
					re := regexp.MustCompile(`bot_data\((\d+)\)>`)
					matches := re.FindStringSubmatch(accountForMenu.BotDataDBPath)
					if len(matches) >= 2 {
						if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
							telegramID = parsedID
						}
					}
				}
				utils.SetDBConfig(telegramID, accountForMenu.PhoneNumber)
				// Reset database pool untuk realtime
				utils.CloseDBPools()
			}
			// Gunakan menuClient dari session atau account yang benar
			ui.ShowMainMenuEdit(telegramBot, chatID, messageID, menuClient)
		}

	case "activity_log":
		// Tampilkan activity log
		ShowActivityLog(telegramBot, chatID, messageID)

	case "activity_stats":
		// Tampilkan statistik aktivitas
		ShowActivityStats(telegramBot, chatID, messageID)

	case "refresh":
		// CRITICAL FIX: Gunakan session untuk isolasi data per user
		var refreshClient *whatsmeow.Client
		var accountForRefresh *WhatsAppAccount

		if userSession != nil && userSession.Account != nil {
			// Gunakan data dari session
			refreshClient = userSession.Client
			accountForRefresh = userSession.Account
		} else if userAccount != nil {
			// Fallback: gunakan userAccount
			refreshClient = userClient
			accountForRefresh = userAccount
		}

		if refreshClient == nil || refreshClient.Store.ID == nil {
			// Belum login - tampilkan prompt login
			ui.ShowLoginPromptEdit(telegramBot, chatID, messageID)
		} else {
			// Sudah login - tampilkan menu utama
			if accountForRefresh != nil {
				// Parse Telegram ID dari BotDataDBPath untuk memastikan dbConfig benar
				telegramID := int64(chatID) // Default: gunakan chatID
				if accountForRefresh.BotDataDBPath != "" {
					re := regexp.MustCompile(`bot_data\((\d+)\)>`)
					matches := re.FindStringSubmatch(accountForRefresh.BotDataDBPath)
					if len(matches) >= 2 {
						if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
							telegramID = parsedID
						}
					}
				}
				utils.SetDBConfig(telegramID, accountForRefresh.PhoneNumber)
				// Reset database pool untuk realtime
				utils.CloseDBPools()
			}
			// Gunakan refreshClient dari session atau account yang benar
			ui.ShowMainMenuEdit(telegramBot, chatID, messageID, refreshClient)
		}

	case "grup", "menu_grup":
		// CRITICAL FIX: Gunakan client dari session user yang benar
		var grupClient *whatsmeow.Client
		if userSession != nil && userSession.Client != nil {
			grupClient = userSession.Client
		} else if userClient != nil {
			grupClient = userClient
		} else {
			grupClient = activeClient
		}

		if grupClient == nil || grupClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.\n\nGunakan /pair <nomor> untuk melakukan pairing terlebih dahulu.")
			telegramBot.Send(editMsg)
			return
		}

		// CRITICAL FIX: Update dbConfig sebelum menampilkan menu grup
		if userAccount != nil {
			telegramID := int64(chatID)
			if userAccount.BotDataDBPath != "" {
				re := regexp.MustCompile(`bot_data\((\d+)\)>`)
				matches := re.FindStringSubmatch(userAccount.BotDataDBPath)
				if len(matches) >= 2 {
					if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
						telegramID = parsedID
					}
				}
			}
			utils.SetDBConfig(telegramID, userAccount.PhoneNumber)
			utils.CloseDBPools()
		}

		// Tampilkan menu grup - GUNAKAN grupClient dari session user yang benar!
		ShowGroupManagementMenuEdit(telegramBot, chatID, messageID, grupClient)

	case "list_grup":
		// CRITICAL FIX: Gunakan client dari session user yang benar
		var listClient *whatsmeow.Client
		if userSession != nil && userSession.Client != nil {
			listClient = userSession.Client
		} else if userClient != nil {
			listClient = userClient
		} else {
			listClient = activeClient
		}

		if listClient == nil || listClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}

		// CRITICAL FIX: Update dbConfig sebelum mengambil daftar grup
		if userAccount != nil {
			telegramID := int64(chatID)
			if userAccount.BotDataDBPath != "" {
				re := regexp.MustCompile(`bot_data\((\d+)\)>`)
				matches := re.FindStringSubmatch(userAccount.BotDataDBPath)
				if len(matches) >= 2 {
					if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
						telegramID = parsedID
					}
				}
			}
			utils.SetDBConfig(telegramID, userAccount.PhoneNumber)
			utils.CloseDBPools()
		}

		// Ambil dan tampilkan daftar grup - GUNAKAN listClient dari session user yang benar!
		go func() {
			if err := GetGroupList(listClient, telegramBot, chatID); err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
				telegramBot.Send(errorMsg)
			}
		}()

	case "search_grup":
		// Handler untuk search grup (EDIT, NO SPAM!)
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowSearchPromptEdit(telegramBot, chatID, messageID)

	case "cancel_search":
		// Batalkan search
		WaitingForSearch[chatID] = false
		msg := tgbotapi.NewMessage(chatID, "❌ Pencarian dibatalkan.")
		telegramBot.Send(msg)

	case "export_grup":
		// CRITICAL FIX: Update dbConfig sebelum export
		if userAccount != nil {
			EnsureDBConfigForUser(int64(chatID), userAccount)
		}
		ShowExportMenuEdit(telegramBot, chatID, messageID)

	case "broadcast_menu":
		ShowBroadcastMenuEdit(telegramBot, chatID, messageID)

	case "broadcast_start":
		StartBroadcastSetup(telegramBot, chatID, messageID)

	case "broadcast_msg_file":
		HandleMessageModeSelection("file", chatID, telegramBot, messageID)

	case "broadcast_msg_manual":
		HandleMessageModeSelection("manual", chatID, telegramBot, messageID)

	case "broadcast_target_file":
		HandleTargetModeSelectionCallback("file", chatID, telegramBot, messageID)

	case "broadcast_target_manual":
		HandleTargetModeSelectionCallback("manual", chatID, telegramBot, messageID)

	case "broadcast_confirm_yes":
		StartBroadcast(telegramBot, chatID)

	case "broadcast_confirm_no":
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **BROADCAST DIBATALKAN**\n\nSetup broadcast telah dibatalkan.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "broadcast_guide":
		guideMsg := `📖 **PANDUAN BROADCAST**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**🎯 Strategi: Staggered Parallel**
Semua akun broadcast secara bersamaan dengan offset waktu antar akun.

**📝 Mode Input Pesan:**
• File TXT: Upload file berisi kalimat (satu per baris)
• Manual: Input pesan untuk setiap akun secara manual

**🎯 Target Grup:**
• Manual: Ketik nama grup (pisah dengan koma)
• File TXT: Upload file berisi nama grup

**🔄 Mode: Loop**
Broadcast akan terus berulang sampai dihentikan dengan /stopchat

**✨ Auto-Variation:**
Pesan akan divariasikan otomatis untuk menghindari spam detection

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan delay yang wajar (5-15 detik)
• Pastikan semua akun terhubung
• Monitor progress secara berkala
• Gunakan /stopchat untuk menghentikan`
		msg := tgbotapi.NewMessage(chatID, guideMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "export_txt":
		// CRITICAL FIX: Update dbConfig sebelum export
		if userAccount != nil {
			EnsureDBConfigForUser(int64(chatID), userAccount)
		}
		// Export sebagai TXT
		go ExportGroupList(telegramBot, chatID, "txt")

	case "export_csv":
		// CRITICAL FIX: Update dbConfig sebelum export
		if userAccount != nil {
			EnsureDBConfigForUser(int64(chatID), userAccount)
		}
		// Export sebagai CSV
		go ExportGroupList(telegramBot, chatID, "csv")

	case "get_link_menu":
		// CRITICAL FIX: Update dbConfig sebelum get link menu
		if userAccount != nil {
			EnsureDBConfigForUser(int64(chatID), userAccount)
		}
		// Handler untuk menu ambil link grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowGetLinkMenuEdit(telegramBot, chatID, messageID)

	case "start_get_link":
		// CRITICAL FIX: Update dbConfig dan gunakan client dari session
		if userAccount != nil {
			EnsureDBConfigForUser(int64(chatID), userAccount)
		}
		linkClient := GetClientForUser(int64(chatID), telegramBot, activeClient)
		if linkClient == nil || linkClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartGetLinkProcess(telegramBot, chatID)

	case "link_example":
		// Tampilkan contoh penggunaan - EDIT existing message
		ShowLinkExampleEdit(telegramBot, chatID, messageID)

	case "cancel_get_link":
		// Batalkan proses ambil link
		CancelGetLink(chatID, telegramBot)

	case "change_photo_menu":
		// Handler untuk menu ganti foto profil grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangePhotoMenuEdit(telegramBot, chatID, messageID)

	case "start_change_photo":
		// Mulai proses ganti foto
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangePhotoProcess(telegramBot, chatID)

	case "photo_example":
		// Tampilkan contoh penggunaan ganti foto - EDIT existing message
		ShowPhotoExampleEdit(telegramBot, chatID, messageID)

	case "cancel_change_photo":
		// Batalkan proses ganti foto
		CancelChangePhoto(chatID, telegramBot)

	case "change_description_menu":
		// Handler untuk menu atur deskripsi grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeDescriptionMenuEdit(telegramBot, chatID, messageID)

	case "start_change_description":
		// Mulai proses ubah deskripsi
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeDescriptionProcess(telegramBot, chatID)

	case "description_example":
		// Tampilkan contoh penggunaan ubah deskripsi - EDIT existing message
		ShowDescriptionExampleEdit(telegramBot, chatID, messageID)

	case "cancel_change_description":
		// Batalkan proses ubah deskripsi
		CancelChangeDescription(chatID, telegramBot)

	case "change_logging_menu":
		// Handler untuk menu atur pesan grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeMessageLoggingMenuEdit(telegramBot, chatID, messageID)

	case "start_change_logging":
		// Mulai proses atur pesan
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeLoggingProcess(telegramBot, chatID)

	case "logging_example":
		// Tampilkan contoh penggunaan atur pesan - EDIT existing message
		ShowLoggingExampleEdit(telegramBot, chatID, messageID)

	case "logging_toggle_on":
		// Quick toggle ON via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInput("ON", chatID, activeClient, telegramBot)

	case "logging_toggle_off":
		// Quick toggle OFF via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInput("OFF", chatID, activeClient, telegramBot)

	case "cancel_change_logging":
		// Batalkan proses atur pesan
		CancelChangeLogging(chatID, telegramBot)

	case "change_member_add_menu":
		// Handler untuk menu atur tambah anggota grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeMemberAddMenuEdit(telegramBot, chatID, messageID)

	case "start_change_member_add":
		// Mulai proses atur tambah anggota
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeMemberAddProcess(telegramBot, chatID)

	case "member_add_example":
		// Tampilkan contoh penggunaan atur tambah anggota - EDIT existing message
		ShowMemberAddExampleEdit(telegramBot, chatID, messageID)

	case "member_add_toggle_on":
		// Quick toggle ON via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInputForMemberAdd(true, chatID, activeClient, telegramBot)

	case "member_add_toggle_off":
		// Quick toggle OFF via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInputForMemberAdd(false, chatID, activeClient, telegramBot)

	case "cancel_change_member_add":
		// Batalkan proses atur tambah anggota
		CancelChangeMemberAdd(chatID, telegramBot)

	case "change_join_approval_menu":
		// Handler untuk menu atur persetujuan anggota baru - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeJoinApprovalMenuEdit(telegramBot, chatID, messageID)

	case "start_change_join_approval":
		// Mulai proses atur persetujuan anggota
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeJoinApprovalProcess(telegramBot, chatID)

	case "join_approval_example":
		// Tampilkan contoh penggunaan atur persetujuan - EDIT existing message
		ShowJoinApprovalExampleEdit(telegramBot, chatID, messageID)

	case "join_approval_toggle_on":
		// Quick toggle ON via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInputForJoinApproval(true, chatID, activeClient, telegramBot)

	case "join_approval_toggle_off":
		// Quick toggle OFF via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInputForJoinApproval(false, chatID, activeClient, telegramBot)

	case "cancel_change_join_approval":
		// Batalkan proses atur persetujuan anggota
		CancelChangeJoinApproval(chatID, telegramBot)

	case "change_ephemeral_menu":
		// Handler untuk menu atur pesan sementara - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeEphemeralMenuEdit(telegramBot, chatID, messageID)

	case "start_change_ephemeral":
		// Mulai proses atur pesan sementara
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeEphemeralProcess(telegramBot, chatID)

	case "ephemeral_example":
		// Tampilkan contoh penggunaan atur pesan sementara - EDIT existing message
		ShowEphemeralExampleEdit(telegramBot, chatID, messageID)

	case "ephemeral_duration_off":
		// Quick toggle OFF (nonaktifkan pesan sementara) via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleDurationInputForEphemeral(0, chatID, activeClient, telegramBot) // OFF = 0

	case "ephemeral_duration_24h":
		// Quick toggle 24 jam via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleDurationInputForEphemeral(86400, chatID, activeClient, telegramBot) // 24 jam

	case "ephemeral_duration_7d":
		// Quick toggle 7 hari via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleDurationInputForEphemeral(604800, chatID, activeClient, telegramBot) // 7 hari

	case "ephemeral_duration_90d":
		// Quick toggle 90 hari via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleDurationInputForEphemeral(7776000, chatID, activeClient, telegramBot) // 90 hari

	case "cancel_change_ephemeral":
		// Batalkan proses atur pesan sementara
		CancelChangeEphemeral(chatID, telegramBot)

	case "show_group_list_ephemeral":
		// CRITICAL FIX: Gunakan client dari session user yang benar
		if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowGroupListForEphemeralEdit(telegramBot, chatID, messageID, 1)

	case "change_all_ephemeral":
		// Proses semua grup untuk atur pesan sementara
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleChangeAllEphemeral(chatID, telegramBot)

	case "show_group_list_join_approval":
		// CRITICAL FIX: Gunakan client dari session user yang benar
		if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowGroupListForJoinApprovalEdit(telegramBot, chatID, messageID, 1)

	case "change_all_join_approval":
		// Proses semua grup untuk atur persetujuan
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleChangeAllJoinApproval(chatID, telegramBot)

	case "change_edit_menu":
		// Handler untuk menu atur edit grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeEditMenuEdit(telegramBot, chatID, messageID)

	case "start_change_edit":
		// Mulai proses atur edit grup
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeEditProcess(telegramBot, chatID)

	case "edit_example":
		// Tampilkan contoh penggunaan atur edit - EDIT existing message
		ShowEditExampleEdit(telegramBot, chatID, messageID)

	case "edit_toggle_on":
		// Quick toggle ON via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInputForEdit(true, chatID, client, telegramBot)

	case "edit_toggle_off":
		// Quick toggle OFF via button
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleToggleInputForEdit(false, chatID, client, telegramBot)

	case "cancel_change_edit":
		// Batalkan proses atur edit grup
		CancelChangeEdit(chatID, telegramBot)

	case "change_all_settings_menu":
		// Handler untuk menu atur semua pengaturan - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowChangeAllSettingsMenuEdit(telegramBot, chatID, messageID)

	case "start_change_all_settings":
		// Mulai proses atur semua pengaturan
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartChangeAllSettingsProcess(telegramBot, chatID)

	case "all_settings_example":
		// Tampilkan contoh penggunaan - EDIT existing message
		ShowAllSettingsExampleEdit(telegramBot, chatID, messageID)

	case "all_settings_msg_on":
		HandleSettingChoiceForAllSettings("message_logging", "on", chatID, client, telegramBot)

	case "all_settings_msg_off":
		HandleSettingChoiceForAllSettings("message_logging", "off", chatID, client, telegramBot)

	case "all_settings_msg_skip":
		HandleSettingChoiceForAllSettings("message_logging", "skip", chatID, client, telegramBot)

	case "all_settings_member_on":
		HandleSettingChoiceForAllSettings("member_add", "on", chatID, client, telegramBot)

	case "all_settings_member_off":
		HandleSettingChoiceForAllSettings("member_add", "off", chatID, client, telegramBot)

	case "all_settings_member_skip":
		HandleSettingChoiceForAllSettings("member_add", "skip", chatID, client, telegramBot)

	case "all_settings_approval_on":
		HandleSettingChoiceForAllSettings("join_approval", "on", chatID, client, telegramBot)

	case "all_settings_approval_off":
		HandleSettingChoiceForAllSettings("join_approval", "off", chatID, client, telegramBot)

	case "all_settings_approval_skip":
		HandleSettingChoiceForAllSettings("join_approval", "skip", chatID, client, telegramBot)

	case "all_settings_ephemeral_off":
		HandleSettingChoiceForAllSettings("ephemeral", "off", chatID, client, telegramBot)

	case "all_settings_ephemeral_24h":
		HandleSettingChoiceForAllSettings("ephemeral", "24h", chatID, client, telegramBot)

	case "all_settings_ephemeral_7d":
		HandleSettingChoiceForAllSettings("ephemeral", "7d", chatID, client, telegramBot)

	case "all_settings_ephemeral_90d":
		HandleSettingChoiceForAllSettings("ephemeral", "90d", chatID, client, telegramBot)

	case "all_settings_ephemeral_skip":
		HandleSettingChoiceForAllSettings("ephemeral", "skip", chatID, client, telegramBot)

	case "all_settings_edit_on":
		HandleSettingChoiceForAllSettings("edit_settings", "on", chatID, client, telegramBot)

	case "all_settings_edit_off":
		HandleSettingChoiceForAllSettings("edit_settings", "off", chatID, client, telegramBot)

	case "all_settings_edit_skip":
		HandleSettingChoiceForAllSettings("edit_settings", "skip", chatID, client, telegramBot)

	case "cancel_change_all_settings":
		CancelChangeAllSettings(chatID, telegramBot)

	case "change_all_settings_all":
		// Handle "Atur Semua"
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleChangeAllSettingsAll(chatID, telegramBot)

	case "show_group_list_all_settings":
		// CRITICAL FIX: Gunakan client dari session user yang benar
		if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowGroupListForAllSettingsEdit(telegramBot, chatID, messageID, 1)

	case "show_group_list_edit":
		// CRITICAL FIX: Gunakan client dari session user yang benar
		if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowGroupListForEditEdit(telegramBot, chatID, messageID, 1)

	case "change_all_edit":
		// Proses semua grup untuk atur edit
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		HandleChangeAllEdit(chatID, telegramBot)

	case "show_group_list_link":
		// Tampilkan list grup untuk dipilih (EDIT, NO SPAM!)
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowGroupListForLinkEdit(telegramBot, chatID, messageID, 1)

	case "select_all_link":
		// Pilih semua grup langsung
		GetAllLinksDirectly(chatID, telegramBot)

	case "create_group_menu":
		// Handler untuk menu buat grup otomatis - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowCreateGroupMenuEdit(telegramBot, chatID, messageID)

	case "create_group_mode_single":
		// Opsi 1: Nama + Jumlah
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartCreateGroupProcessSingle(telegramBot, chatID)

	case "create_group_mode_multiline":
		// Opsi 2: Multi-line
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartCreateGroupProcessMultiline(telegramBot, chatID)

	case "create_group_skip_numbers":
		// Skip nomor telepon
		HandleSkipPhoneNumbers(chatID, telegramBot)

	case "create_group_setting_msg_on":
		HandleCreateGroupSettingChoice("message_logging", "on", chatID, client, telegramBot)

	case "create_group_setting_msg_off":
		HandleCreateGroupSettingChoice("message_logging", "off", chatID, client, telegramBot)

	case "create_group_setting_msg_skip":
		HandleCreateGroupSettingChoice("message_logging", "skip", chatID, client, telegramBot)

	case "create_group_setting_member_on":
		HandleCreateGroupSettingChoice("member_add", "on", chatID, client, telegramBot)

	case "create_group_setting_member_off":
		HandleCreateGroupSettingChoice("member_add", "off", chatID, client, telegramBot)

	case "create_group_setting_member_skip":
		HandleCreateGroupSettingChoice("member_add", "skip", chatID, client, telegramBot)

	case "create_group_setting_approval_on":
		HandleCreateGroupSettingChoice("join_approval", "on", chatID, client, telegramBot)

	case "create_group_setting_approval_off":
		HandleCreateGroupSettingChoice("join_approval", "off", chatID, client, telegramBot)

	case "create_group_setting_approval_skip":
		HandleCreateGroupSettingChoice("join_approval", "skip", chatID, client, telegramBot)

	case "create_group_setting_ephemeral_off":
		HandleCreateGroupSettingChoice("ephemeral", "off", chatID, client, telegramBot)

	case "create_group_setting_ephemeral_24h":
		HandleCreateGroupSettingChoice("ephemeral", "24h", chatID, client, telegramBot)

	case "create_group_setting_ephemeral_7d":
		HandleCreateGroupSettingChoice("ephemeral", "7d", chatID, client, telegramBot)

	case "create_group_setting_ephemeral_90d":
		HandleCreateGroupSettingChoice("ephemeral", "90d", chatID, client, telegramBot)

	case "create_group_setting_ephemeral_skip":
		HandleCreateGroupSettingChoice("ephemeral", "skip", chatID, client, telegramBot)

	case "create_group_setting_edit_on":
		HandleCreateGroupSettingChoice("edit_settings", "on", chatID, client, telegramBot)

	case "create_group_setting_edit_off":
		HandleCreateGroupSettingChoice("edit_settings", "off", chatID, client, telegramBot)

	case "create_group_setting_edit_skip":
		HandleCreateGroupSettingChoice("edit_settings", "skip", chatID, client, telegramBot)

	case "cancel_create_group":
		CancelCreateGroup(chatID, telegramBot)

	case "join_group_menu":
		// Handler untuk menu join grup otomatis - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowJoinGroupMenuEdit(telegramBot, chatID, messageID)

	case "add_member_menu":
		// Handler untuk menu add member grup - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowAddMemberMenuEdit(telegramBot, chatID, messageID)

	case "start_add_member":
		StartAddMemberProcess(telegramBot, chatID)

	case "add_member_example":
		ShowAddMemberExample(chatID, telegramBot, messageID)

	case "add_member_mode_one_by_one":
		HandleModeInputForAddMember("one_by_one", chatID, telegramBot)

	case "add_member_mode_batch":
		HandleModeInputForAddMember("batch", chatID, telegramBot)

	case "cancel_add_member":
		CancelAddMember(chatID, telegramBot)

	case "admin_menu":
		// Handler untuk menu auto admin - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowAdminMenuEdit(telegramBot, chatID, messageID)

	case "unadmin_menu":
		// Handler untuk menu auto unadmin - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowUnadminMenuEdit(telegramBot, chatID, messageID)

	case "start_admin_process":
		// Handler untuk memulai proses auto admin
		if activeClient == nil || activeClient.Store.ID == nil {
			callback := tgbotapi.NewCallback(callbackQuery.ID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Request(callback)
			return
		}
		StartAdminProcess(telegramBot, chatID)
		callback := tgbotapi.NewCallback(callbackQuery.ID, "")
		telegramBot.Request(callback)

	case "start_unadmin_process":
		// Handler untuk memulai proses auto unadmin
		if activeClient == nil || activeClient.Store.ID == nil {
			callback := tgbotapi.NewCallback(callbackQuery.ID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Request(callback)
			return
		}
		StartUnadminProcess(telegramBot, chatID)
		callback := tgbotapi.NewCallback(callbackQuery.ID, "")
		telegramBot.Request(callback)

	case "cancel_admin":
		// Handler untuk membatalkan proses auto admin (EDIT, NO SPAM!)
		CancelAdminEdit(telegramBot, chatID, messageID)
		callback := tgbotapi.NewCallback(callbackQuery.ID, "❌ Proses dibatalkan.")
		telegramBot.Request(callback)

	case "cancel_unadmin":
		// Handler untuk membatalkan proses auto unadmin (EDIT, NO SPAM!)
		CancelUnadminEdit(telegramBot, chatID, messageID)
		callback := tgbotapi.NewCallback(callbackQuery.ID, "❌ Proses dibatalkan.")
		telegramBot.Request(callback)

	case "start_join_group":
		// Mulai proses join grup
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartJoinGroupProcess(telegramBot, chatID)

	case "cancel_join_group":
		CancelJoinGroup(chatID, telegramBot)

	case "leave_group_menu":
		// Handler untuk menu keluar grup otomatis - EDIT existing message
		if activeClient == nil || activeClient.Store.ID == nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(editMsg)
			return
		}
		ShowLeaveGroupMenuEdit(telegramBot, chatID, messageID)

	case "start_leave_group":
		// Mulai proses keluar grup
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		StartLeaveGroupProcess(chatID, telegramBot)

	case "cancel_leave_group":
		CancelLeaveGroup(chatID, telegramBot)

	case "leave_mode_one_by_one":
		HandleModeInputForLeave("one_by_one", chatID, telegramBot)

	case "leave_mode_batch":
		HandleModeInputForLeave("batch", chatID, telegramBot)

	case "leave_notification_yes":
		HandleNotificationChoiceForLeave(true, chatID, telegramBot)

	case "leave_notification_no":
		HandleNotificationChoiceForLeave(false, chatID, telegramBot)

	case "process_join_group":
		// Process join groups dengan client
		if activeClient == nil || activeClient.Store.ID == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(msg)
			return
		}
		state := joinGroupStates[chatID]
		if state != nil && !state.WaitingForLink && !state.WaitingForDelay {
			// State ready, process with client
			go ProcessJoinGroups(state, chatID, client, telegramBot)
		}

	case "help":
		helpText := `📖 **BANTUAN LENGKAP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **FITUR YANG TERSEDIA**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

👥 **GRUP**
   /grup - Manajemen grup WhatsApp

🔧 **PENGATURAN**
   /pair <nomor> - Pairing WhatsApp
   /logout - Logout akun saat ini
   /reset - Reset program (hapus semua data)
   /help - Bantuan lengkap

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **CARA PENGGUNAAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

• Gunakan /menu untuk melihat menu utama
• Klik tombol inline untuk navigasi cepat
• Gunakan command di atas sesuai kebutuhan

⚠️ **CATATAN:**
• /logout - Hanya logout akun aktif saat ini
• /reset - Hapus SEMUA data (semua akun, semua database)`
		msg := tgbotapi.NewMessage(chatID, helpText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

	case "logout":
		// Logout dengan konfirmasi (dari tombol menu)
		if err := LogoutWhatsApp(chatID); err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
			telegramBot.Send(errorMsg)
		}

	case "reset":
		// Reset program (hapus semua data)
		ResetProgramRequest(telegramBot, chatID)

	case "logout_confirm":
		// Konfirmasi logout - hapus database
		go func() {
			if err := ConfirmLogout(chatID); err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error saat logout: %v", err))
				telegramBot.Send(errorMsg)
			}
		}()

	case "logout_cancel":
		// Batal logout (EDIT, NO SPAM!)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **LOGOUT DIBATALKAN**\n\nLogout telah dibatalkan.\n\nAkun WhatsApp tetap terhubung.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "reset_confirm":
		// Konfirmasi reset program
		go func() {
			if err := ConfirmResetProgram(telegramBot, chatID); err != nil {
				errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error reset program: %v", err))
				telegramBot.Send(errorMsg)
			}
		}()

	case "reset_cancel":
		// Batalkan reset program (EDIT, NO SPAM!)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **RESET DIBATALKAN**\n\nReset program telah dibatalkan.\n\nSemua data tetap aman.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "reset_program":
		// Reset program (dari callback) - Edit message untuk konfirmasi
		fmt.Printf("[DEBUG] reset_program callback triggered for chatID=%d\n", chatID)
		// Edit message yang ada menjadi konfirmasi reset
		ResetProgramRequestEdit(telegramBot, chatID, messageID)
		fmt.Printf("[DEBUG] ResetProgramRequestEdit called for chatID=%d\n", chatID)

	case "start_pairing":
		// Mulai pairing - set state dan tampilkan instruksi
		WaitingForPhoneNumber[chatID] = true

		// CRITICAL FIX: Cek apakah user ini sudah punya account dan sudah login
		// Gunakan userClient dari session user yang benar, bukan client parameter yang bisa dari user lain
		if userAccount != nil && userClient != nil && userClient.Store != nil && userClient.Store.ID != nil {
			WaitingForPhoneNumber[chatID] = false
			msg := tgbotapi.NewMessage(chatID, "✅ Bot WhatsApp sudah login!\n\nGunakan /logout untuk logout terlebih dahulu jika ingin mengganti akun.")
			telegramBot.Send(msg)
			return
		}

		// User belum punya account atau belum login, izinkan pairing
		showPhoneInputPrompt(telegramBot, chatID)

	case "back_to_login":
		// Kembali ke login prompt (EDIT, NO SPAM!)
		WaitingForPhoneNumber[chatID] = false
		ui.ShowLoginPromptEdit(telegramBot, chatID, messageID)

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

	case "multi_account_menu":
		// Menu login WhatsApp baru
		ShowMultiAccountMenuEdit(telegramBot, chatID, messageID)

	case "multi_account_login":
		// Mulai login akun WhatsApp baru (EDIT, NO SPAM!)
		StartMultiAccountLoginEdit(telegramBot, chatID, messageID)

	case "multi_account_cancel_login":
		// Batalkan login akun baru (EDIT, NO SPAM!)
		delete(multiAccountLoginStates, chatID)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **LOGIN DIBATALKAN**\n\nLogin akun WhatsApp baru telah dibatalkan.\n\nGunakan 'Login Baru' untuk memulai kembali.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "multi_account_list":
		// Tampilkan daftar akun (EDIT, NO SPAM!)
		ShowAccountListEdit(telegramBot, chatID, messageID)

	case "multi_account_switch":
		// Tampilkan daftar akun untuk switch (EDIT, NO SPAM!)
		ShowAccountListEdit(telegramBot, chatID, messageID)

	case "multi_account_cancel_pairing":
		// Batalkan pairing multi-account (EDIT, NO SPAM!)
		delete(multiAccountLoginStates, chatID)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **PAIRING DIBATALKAN**\n\nProses pairing telah dibatalkan.\n\nGunakan 'Login Baru' untuk memulai kembali.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "login_help":
		// Help tentang login (EDIT, NO SPAM!)
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
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, helpMsg)
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "cancel_pairing":
		// Batalkan pairing yang sedang berjalan (EDIT, NO SPAM!)
		WaitingForPhoneNumber[chatID] = false
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **PAIRING DIBATALKAN**\n\nPairing WhatsApp telah dibatalkan.\n\nGunakan tombol \"🔗 Mulai Pairing\" atau command `/pair <nomor>` untuk memulai ulang.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)

	case "cancel_phone_input":
		// Batalkan input nomor (EDIT, NO SPAM!) - Kembali ke login prompt
		WaitingForPhoneNumber[chatID] = false
		ui.ShowLoginPromptEdit(telegramBot, chatID, messageID)

	default:
		// Handle pagination callbacks (link_page_X)
		if strings.HasPrefix(data, "link_page_") {
			pageStr := strings.TrimPrefix(data, "link_page_")
			page, err := strconv.Atoi(pageStr)
			if err == nil {
				ShowGroupListForLinkEdit(telegramBot, chatID, messageID, page)
				return
			}
		}

		// Handle pagination callbacks (join_approval_page_X)
		if strings.HasPrefix(data, "join_approval_page_") {
			pageStr := strings.TrimPrefix(data, "join_approval_page_")
			page, err := strconv.Atoi(pageStr)
			if err == nil {
				// CRITICAL FIX: Gunakan client dari session user yang benar
				if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
					editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
					telegramBot.Send(editMsg)
					return
				}
				ShowGroupListForJoinApprovalEdit(telegramBot, chatID, messageID, page)
				return
			}
		}

		// Handle pagination callbacks (ephemeral_page_X)
		if strings.HasPrefix(data, "ephemeral_page_") {
			pageStr := strings.TrimPrefix(data, "ephemeral_page_")
			page, err := strconv.Atoi(pageStr)
			if err == nil {
				// CRITICAL FIX: Gunakan client dari session user yang benar
				if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
					editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
					telegramBot.Send(editMsg)
					return
				}
				ShowGroupListForEphemeralEdit(telegramBot, chatID, messageID, page)
				return
			}
		}

		// Handle pagination callbacks (edit_page_X)
		if strings.HasPrefix(data, "edit_page_") {
			pageStr := strings.TrimPrefix(data, "edit_page_")
			page, err := strconv.Atoi(pageStr)
			if err == nil {
				// CRITICAL FIX: Gunakan client dari session user yang benar
				if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
					editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
					telegramBot.Send(editMsg)
					return
				}
				ShowGroupListForEditEdit(telegramBot, chatID, messageID, page)
				return
			}
		}

		// Handle pagination callbacks (all_settings_page_X)
		if strings.HasPrefix(data, "all_settings_page_") {
			pageStr := strings.TrimPrefix(data, "all_settings_page_")
			page, err := strconv.Atoi(pageStr)
			if err == nil {
				// CRITICAL FIX: Gunakan client dari session user yang benar
				if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
					editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
					telegramBot.Send(editMsg)
					return
				}
				ShowGroupListForAllSettingsEdit(telegramBot, chatID, messageID, page)
				return
			}
		}

		// Check if it's a switch account callback (multi_account_switch_<id>)
		if strings.HasPrefix(data, "multi_account_switch_") {
			var accountID int
			if _, err := fmt.Sscanf(data, "multi_account_switch_%d", &accountID); err == nil {
				if err := SwitchAccount(accountID, telegramBot, chatID); err != nil {
					errorMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("❌ **ERROR**\n\nGagal mengganti akun: %v", err))
					errorMsg.ParseMode = "Markdown"
					telegramBot.Send(errorMsg)
				} else {
					// Refresh menu utama setelah switch (EDIT, NO SPAM!)
					// IMPORTANT: Selalu gunakan GetCurrentClient() setelah switch, JANGAN fallback ke client lama!
					am := GetAccountManager()
					currentClient := am.GetCurrentClient()
					currentAccount := am.GetCurrentAccount()

					if currentClient == nil {
						// Jika client belum ada, coba buat client
						if currentAccount != nil {
							var err error
							currentClient, err = am.CreateClient(currentAccount.ID)
							if err != nil {
								errorMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("❌ **ERROR**\n\nGagal membuat client untuk akun %s: %v", currentAccount.PhoneNumber, err))
								errorMsg.ParseMode = "Markdown"
								telegramBot.Send(errorMsg)
								return
							}
						} else {
							errorMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **ERROR**\n\nTidak ada akun aktif ditemukan setelah switch.")
							errorMsg.ParseMode = "Markdown"
							telegramBot.Send(errorMsg)
							return
						}
					}

					// Update dbConfig untuk realtime - PASTIKAN menggunakan nomor dari account aktif
					if currentAccount != nil {
						// Parse Telegram ID dari BotDataDBPath untuk memastikan dbConfig benar
						telegramID := int64(chatID) // Default: gunakan chatID
						if currentAccount.BotDataDBPath != "" {
							re := regexp.MustCompile(`bot_data\((\d+)\)>`)
							matches := re.FindStringSubmatch(currentAccount.BotDataDBPath)
							if len(matches) >= 2 {
								if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
									telegramID = parsedID
								}
							}
						}

						utils.SetDBConfig(telegramID, currentAccount.PhoneNumber)
						// Reset database pool agar menggunakan database baru (realtime)
						utils.CloseDBPools()

						// Force rebuild pool dengan database baru (dengan delay kecil)
						time.Sleep(100 * time.Millisecond)
						_, err := utils.GetBotDBPool()
						if err != nil {
							utils.GetLogger().Error("Account switch callback: Failed to rebuild database pool: %v", err)
						} else {
							utils.GetLogger().Info("Account switch callback: Database pool rebuilt with path: %s", utils.GetBotDataDBPath())
						}

						// Log untuk debug
						utils.GetLogger().Info("Account switched: ID=%d, Phone=%s, TelegramID=%d, DBPath=%s", currentAccount.ID, currentAccount.PhoneNumber, telegramID, currentAccount.BotDataDBPath)
					}

					// Update global client agar konsisten
					SetClients(currentClient, telegramBot)

					// Kirim notifikasi sukses
					successMsg := fmt.Sprintf(`✅ **SWITCH AKUN BERHASIL!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **Akun Aktif:** +%s
✅ **Status:** Terhubung
🔄 **Koneksi:** Siap digunakan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Daftar akun akan diperbarui...`, currentAccount.PhoneNumber)

					successEdit := tgbotapi.NewEditMessageText(chatID, messageID, successMsg)
					successEdit.ParseMode = "Markdown"
					telegramBot.Send(successEdit)

					// Tunggu sebentar agar user baca notifikasi
					time.Sleep(1 * time.Second)

					// Refresh daftar akun untuk menampilkan status aktif yang baru
					ShowAccountListEdit(telegramBot, chatID, messageID)
				}
				return
			}
		}

		// Check if it's a delete account callback (multi_account_delete_<id>)
		if strings.HasPrefix(data, "multi_account_delete_") {
			var accountID int
			if _, err := fmt.Sscanf(data, "multi_account_delete_%d", &accountID); err == nil {
				// Tampilkan konfirmasi delete
				ShowDeleteAccountConfirmation(telegramBot, chatID, messageID, accountID)
				return
			}
		}

		if strings.HasPrefix(data, "multi_account_delete_confirm_") {
			var accountID int
			if _, err := fmt.Sscanf(data, "multi_account_delete_confirm_%d", &accountID); err == nil {
				if err := DeleteAccount(accountID, telegramBot, chatID); err != nil {
					errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal menghapus akun")
					editMsg := tgbotapi.NewEditMessageText(chatID, messageID, errorMsg)
					editMsg.ParseMode = "Markdown"
					telegramBot.Send(editMsg)
				} else {
					// Success message
					successMsg := tgbotapi.NewEditMessageText(chatID, messageID, "✅ **AKUN BERHASIL DIHAPUS**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nAkun telah dihapus dari sistem.\n\n**Catatan:**\n• Database telah dibackup ke folder backup/\n• Anda bisa restore jika diperlukan\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					successMsg.ParseMode = "Markdown"
					telegramBot.Send(successMsg)

					// Refresh daftar akun setelah delete (EDIT, NO SPAM!)
					time.Sleep(1 * time.Second)
					ShowAccountListEdit(telegramBot, chatID, messageID)
				}
				return
			}
		}

		if strings.HasPrefix(data, "multi_account_delete_cancel_") {
			var accountID int
			if _, err := fmt.Sscanf(data, "multi_account_delete_cancel_%d", &accountID); err == nil {
				CancelDeleteAccount(telegramBot, chatID, messageID)
				return
			}
		}

		// Handle other pagination callbacks that might not be caught
		// (create_page_X, join_page_X, admin_page_X, unadmin_page_X)
		if strings.HasPrefix(data, "create_page_") ||
			strings.HasPrefix(data, "join_page_") ||
			strings.HasPrefix(data, "admin_page_") ||
			strings.HasPrefix(data, "unadmin_page_") {
			// These are handled in their respective handlers
			return
		}

		// Unknown callback - show error
		fmt.Printf("⚠️ Unknown callback data from user %d: %s\n", chatID, data)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **Tombol tidak dikenali.**\n\nSilakan gunakan menu yang tersedia atau klik tombol 'Kembali' untuk kembali ke menu utama.")
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
	}
}

// HandlePhoneNumberInput memproses input nomor telepon dari user
func HandlePhoneNumberInput(phoneNumber string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	// Reset state
	WaitingForPhoneNumber[chatID] = false

	// CRITICAL FIX: Cek apakah user ini sudah punya account dan sudah login
	// Gunakan GetUserSession untuk mendapatkan account user yang benar
	userSession, _ := GetUserSession(chatID, telegramBot)
	var userAccount *WhatsAppAccount
	var userClient *whatsmeow.Client
	if userSession != nil {
		userAccount = userSession.Account
		userClient = userSession.Client
	} else {
		am := GetAccountManager()
		userAccount = am.GetAccountByTelegramID(chatID)
		if userAccount != nil {
			userClient = am.GetClient(userAccount.ID)
		}
	}

	if userAccount != nil && userClient != nil && userClient.Store != nil && userClient.Store.ID != nil {
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
