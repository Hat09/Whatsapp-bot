package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"whatsapp-bot/core"
	"whatsapp-bot/handlers"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Initialize loggers
	utils.InitLogger(true)     // Application logger (debug mode)
	utils.InitGrupLogger(true) // Group logger (debug mode)

	logger := utils.GetLogger()
	logger.Phase("Starting WhatsApp Bot with Telegram Integration...")
	logger.Info("Version: 1.0.0")
	logger.Info("Initializing...")

	// Migrate existing databases to user folders (if any)
	if err := utils.MigrateDatabaseToUserFolder(); err != nil {
		logger.Warn("Failed to migrate databases to user folders: %v", err)
		// Continue anyway, migration is not critical
	}

	// Scan user folders and register existing accounts to master database
	if err := utils.ScanUserFoldersAndRegisterAccounts(); err != nil {
		logger.Warn("Failed to scan user folders and register accounts: %v", err)
		// Continue anyway, scan is not critical
	}

	// Create startup manager
	startupManager := core.NewStartupManager()

	// Set event handler
	startupManager.SetEventHandler(core.EventHandler)

	// Initialize application
	if err := startupManager.Initialize(); err != nil {
		logger.Fatal("Failed to initialize application: %v", err)
	}

	// Set global clients for event handler
	core.SetGlobalClients(
		startupManager.GetWhatsAppClient(),
		startupManager.GetTelegramBot(),
	)

	// IMPORTANT: Set global clients in handlers package too
	// This ensures TgBot is available for PairDeviceViaTelegram and other handlers
	handlers.SetClients(
		startupManager.GetWhatsAppClient(),
		startupManager.GetTelegramBot(),
	)

	// Start Telegram bot handler in background
	go startTelegramBotHandler(startupManager)

	// Start realtime database cleanup untuk akun tidak aktif
	// Cleanup setiap 1 jam, hapus database untuk akun yang tidak aktif selama 7 hari
	handlers.StartRealtimeDBCleanup(
		1*time.Hour,                              // Cleanup interval: setiap 1 jam
		7*24*time.Hour,                           // Inactive threshold: 7 hari
		handlers.GetActiveAccountIDsFromSessions, // Fungsi untuk mendapatkan account aktif
	)
	logger.Info("Realtime database cleanup started (interval: 1 hour, threshold: 7 days)")

	logger.Success("Application started successfully")
	logger.Info("Press Ctrl+C to stop...")

	// Wait for shutdown signal
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)
	<-shutdownChan

	// Graceful shutdown
	logger.Phase("Shutting down...")
	shutdownManager := core.NewShutdownManager(
		startupManager.GetWhatsAppClient(),
		startupManager.GetTelegramBot(),
	)

	if err := shutdownManager.ShutdownWithTimeout(10 * time.Second); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	logger.Success("Application stopped")
}

// startTelegramBotHandler menjalankan Telegram bot handler
func startTelegramBotHandler(startupManager *core.StartupManager) {
	config := startupManager.GetConfig()
	telegramBot := startupManager.GetTelegramBot()
	waClient := startupManager.GetWhatsAppClient()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := telegramBot.GetUpdatesChan(u)

	logger := utils.GetLogger()
	logger.Success("Telegram bot handler active")

	for update := range updates {
		// Handle inline keyboard callback
		if update.CallbackQuery != nil {
			userID := update.CallbackQuery.From.ID
			fmt.Printf("[DEBUG] CallbackQuery received from userID=%d, data=%s\n", userID, update.CallbackQuery.Data)

			if !config.TelegramConfig.CheckAccess(int64(userID)) {
				fmt.Printf("[DEBUG] Access denied for userID=%d\n", userID)
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "❌ Anda tidak memiliki akses.")
				telegramBot.Request(callback)
				continue
			}

			fmt.Printf("[DEBUG] Access granted, calling HandleCallbackQuery with data=%s\n", update.CallbackQuery.Data)
			handlers.HandleCallbackQuery(update.CallbackQuery, waClient, telegramBot)
			continue
		}

		// Handle text messages
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		if !config.TelegramConfig.CheckAccess(int64(userID)) {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Anda tidak memiliki akses untuk menggunakan bot ini.")
			telegramBot.Send(msg)
			continue
		}

		// Handle commands
		if update.Message.IsCommand() {
			handlers.HandleTelegramCommand(update.Message, waClient, telegramBot)
			continue
		}

		// Handle phone number input
		chatID := update.Message.Chat.ID
		if handlers.WaitingForPhoneNumber[chatID] {
			phoneNumber := strings.TrimSpace(update.Message.Text)
			handlers.HandlePhoneNumberInput(phoneNumber, chatID, waClient, telegramBot)
			continue
		}

		// Handle search input
		if handlers.WaitingForSearch[chatID] {
			keyword := strings.TrimSpace(update.Message.Text)
			handlers.HandleSearchInput(keyword, chatID, telegramBot)
			continue
		}

		// Handle group selection from list (for all settings feature)
		if handlers.IsWaitingForAllSettingsSelection(chatID) {
			selection := strings.TrimSpace(update.Message.Text)
			handlers.ProcessSelectedGroupsForAllSettings(selection, chatID, telegramBot)
			continue
		}

		// Handle group selection from list (for edit feature)
		if handlers.IsWaitingForEditSelection(chatID) {
			selection := strings.TrimSpace(update.Message.Text)
			handlers.ProcessSelectedGroupsForEdit(selection, chatID, telegramBot)
			continue
		}

		// Handle group selection from list (for ephemeral feature)
		if handlers.IsWaitingForEphemeralSelection(chatID) {
			selection := strings.TrimSpace(update.Message.Text)
			handlers.ProcessSelectedGroupsForEphemeral(selection, chatID, telegramBot)
			continue
		}

		// Handle group selection from list (for join approval feature)
		if handlers.IsWaitingForJoinApprovalSelection(chatID) {
			selection := strings.TrimSpace(update.Message.Text)
			handlers.ProcessSelectedGroupsForJoinApproval(selection, chatID, telegramBot)
			continue
		}

		// Handle group selection from list (for link feature)
		if handlers.IsWaitingForGroupSelection(chatID) {
			selection := strings.TrimSpace(update.Message.Text)
			handlers.ProcessSelectedGroupsForLink(selection, chatID, telegramBot)
			continue
		}

		// Handle create group input
		if handlers.IsWaitingForCreateGroupInput(chatID) {
			input := strings.TrimSpace(update.Message.Text)
			inputType := handlers.GetCreateGroupInputType(chatID)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForCreate(input, chatID, telegramBot)
			} else if inputType == "count" {
				handlers.HandleCountInputForCreate(input, chatID, telegramBot)
			} else if inputType == "phone_numbers" {
				handlers.HandlePhoneNumbersInputForCreate(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForCreate(input, chatID, telegramBot)
			}
			continue
		}

		// Handle get link input
		if handlers.IsWaitingForLinkInput(chatID) {
			inputType := handlers.GetLinkInputType(chatID)

			// Handle file upload (.txt)
			if inputType == "group_name" && update.Message.Document != nil {
				// Check if it's a .txt file
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForGetLink(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			// Handle text input
			input := strings.TrimSpace(update.Message.Text)
			if inputType == "group_name" {
				if input != "" {
					handlers.HandleGroupNameInput(input, chatID, telegramBot)
				}
			} else if inputType == "delay" {
				handlers.HandleDelayInput(input, chatID, waClient, telegramBot)
			}
			continue
		}

		// Handle change photo input (text part)
		if handlers.IsWaitingForPhotoInput(chatID) {
			inputType := handlers.GetPhotoInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangePhoto(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}
		}

		// Handle Add Member state
		addMemberState := handlers.GetAddMemberState(chatID)
		if addMemberState != nil {
			if addMemberState.WaitingForGroupName {
				input := strings.TrimSpace(update.Message.Text)
				if input != "" {
					handlers.HandleGroupNameInputForAddMember(input, chatID, telegramBot)
				}
				// Handle .txt file for group names
				if update.Message.Document != nil {
					fileName := update.Message.Document.FileName
					if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
						handlers.HandleFileInputForAddMember(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token, false)
					}
				}
				continue
			}
			if addMemberState.WaitingForNumbers {
				input := strings.TrimSpace(update.Message.Text)
				if input != "" {
					handlers.HandlePhoneInputForAddMember(input, chatID, telegramBot)
				}
				// Handle .vcf file for contacts
				if update.Message.Document != nil {
					fileName := update.Message.Document.FileName
					if strings.HasSuffix(strings.ToLower(fileName), ".vcf") {
						handlers.HandleFileInputForAddMember(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token, true)
					}
				}
				continue
			}
			if addMemberState.WaitingForDelay {
				input := strings.TrimSpace(update.Message.Text)
				if input != "" {
					handlers.HandleDelayInputForAddMember(input, chatID, telegramBot)
				}
				continue
			}
			if addMemberState.WaitingForNumberDelay {
				input := strings.TrimSpace(update.Message.Text)
				if input != "" {
					handlers.HandleNumberDelayInputForAddMember(input, chatID, telegramBot)
				}
				continue
			}
		}

		// Handle change photo input (text part)
		if handlers.IsWaitingForPhotoInput(chatID) {
			inputType := handlers.GetPhotoInputType(chatID)

			// Handle photo upload
			if len(update.Message.Photo) > 0 && inputType == "photo" {
				photo := update.Message.Photo[len(update.Message.Photo)-1]
				handlers.HandlePhotoUpload(&photo, chatID, waClient, telegramBot)
				continue
			}

			// Handle text input (group name or delay)
			input := strings.TrimSpace(update.Message.Text)
			if inputType == "group_name" {
				handlers.HandleGroupNameInputForPhoto(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForPhoto(input, chatID, telegramBot)
			}
			continue
		}

		// Handle change description input
		if handlers.IsWaitingForDescriptionInput(chatID) {
			inputType := handlers.GetDescriptionInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangeDescription(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForDescription(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForDescription(input, chatID, telegramBot)
			} else if inputType == "description" {
				handlers.HandleDescriptionInput(input, chatID, waClient, telegramBot)
			}
			continue
		}

		// Handle change logging input
		if handlers.IsWaitingForLoggingInput(chatID) {
			inputType := handlers.GetLoggingInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangeLogging(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForLogging(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForLogging(input, chatID, telegramBot)
			} else if inputType == "toggle" {
				handlers.HandleToggleInput(input, chatID, waClient, telegramBot)
			}
			continue
		}

		// Handle change member add input
		if handlers.IsWaitingForMemberAddInput(chatID) {
			inputType := handlers.GetMemberAddInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangeMemberAdd(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForMemberAdd(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForMemberAdd(input, chatID, telegramBot)
			}
			// Toggle handled via button callbacks (no text input needed)
			continue
		}

		// Handle change join approval input
		if handlers.IsWaitingForJoinApprovalInput(chatID) {
			inputType := handlers.GetJoinApprovalInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangeJoinApproval(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForJoinApproval(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForJoinApproval(input, chatID, telegramBot)
			}
			// Toggle handled via button callbacks (no text input needed)
			continue
		}

		// Handle change ephemeral input
		if handlers.IsWaitingForEphemeralInput(chatID) {
			inputType := handlers.GetEphemeralInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangeEphemeral(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForEphemeral(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForEphemeral(input, chatID, telegramBot)
			}
			// Duration handled via button callbacks (no text input needed)
			continue
		}

		// Handle change edit input
		if handlers.IsWaitingForEditInput(chatID) {
			inputType := handlers.GetEditInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForChangeEdit(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForEdit(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForEdit(input, chatID, telegramBot)
			}
			// Toggle handled via button callbacks (no text input needed)
			continue
		}

		// Handle change all settings input
		if handlers.IsWaitingForAllSettingsInput(chatID) {
			inputType := handlers.GetAllSettingsInputType(chatID)

			// Handle file upload (.txt) for group names
			if inputType == "group_name" && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForAllSettings(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			input := strings.TrimSpace(update.Message.Text)

			if inputType == "group_name" {
				handlers.HandleGroupNameInputForAllSettings(input, chatID, telegramBot)
			} else if inputType == "delay" {
				handlers.HandleDelayInputForAllSettings(input, chatID, telegramBot)
			}
			// Settings choices handled via button callbacks (no text input needed)
			continue
		}

		// Handle join group input
		if handlers.IsWaitingForJoinGroupInput(chatID) {
			inputType := handlers.GetJoinGroupInputType(chatID)

			// Handle file upload (.txt)
			if update.Message.Document != nil && inputType == "link" {
				fileID := update.Message.Document.FileID
				fileName := update.Message.Document.FileName

				// Check if it's a .txt file
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForJoin(fileID, chatID, telegramBot, telegramBot.Token)
					continue
				} else {
					errorMsg := tgbotapi.NewMessage(chatID, "❌ File harus berformat .txt!")
					telegramBot.Send(errorMsg)
					continue
				}
			}

			// Handle text input (links or delay)
			if inputType == "link" {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleLinkInputForJoin(input, chatID, telegramBot)
			} else if inputType == "delay" {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleDelayInputForJoin(input, chatID, telegramBot)

				// After delay input, check if state is ready and process
				state := handlers.GetJoinGroupState(chatID)
				if state != nil && !state.WaitingForLink && !state.WaitingForDelay {
					// State ready, process with client
					go handlers.ProcessJoinGroups(state, chatID, waClient, telegramBot)
				}
			}
			continue
		}

		// Handle leave group input
		if handlers.IsWaitingForLeaveGroupInput(chatID) {
			inputType := handlers.GetLeaveGroupInputType(chatID)

			// Handle file upload (.txt)
			if update.Message.Document != nil && inputType == "group_name" {
				fileID := update.Message.Document.FileID
				fileName := update.Message.Document.FileName

				// Check if it's a .txt file
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForLeave(fileID, chatID, telegramBot, telegramBot.Token)
					continue
				} else {
					errorMsg := tgbotapi.NewMessage(chatID, "❌ File harus berformat .txt!")
					telegramBot.Send(errorMsg)
					continue
				}
			}

			// Handle text input (group names or delay)
			if inputType == "group_name" {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleGroupNameInputForLeave(input, chatID, telegramBot)
			} else if inputType == "delay" {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleDelayInputForLeave(input, chatID, telegramBot)
			} else if inputType == "notification_message" {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleNotificationMessageInputForLeave(input, chatID, telegramBot)
			}
			continue
		}

		// Handle multi-account login input
		if handlers.IsWaitingForMultiAccountInput(chatID) {
			input := strings.TrimSpace(update.Message.Text)
			handlers.HandleMultiAccountPhoneInput(input, chatID, telegramBot)
			continue
		}

		// Handle admin/unadmin input
		if handlers.IsWaitingForAdminInput(chatID) {
			inputType := handlers.GetAdminInputType(chatID)

			if inputType == "groups" {
				// Input group names
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleGroupNameInputForAdmin(input, chatID, telegramBot)

			} else if inputType == "delay" {
				// Input delay
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleDelayInputForAdmin(input, chatID, telegramBot)

			} else if inputType == "phones" {
				// Input phone numbers
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandlePhoneInputForAdmin(input, chatID, telegramBot)
			}
			continue
		}

		// Handle broadcast input
		state := handlers.GetBroadcastState(chatID)
		if state != nil {
			// Handle offset delay input
			if state.WaitingForOffsetDelay {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleOffsetDelayInput(input, chatID, telegramBot)
				continue
			}

			// Handle group delay input
			if state.WaitingForGroupDelay {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleGroupDelayInput(input, chatID, telegramBot)
				continue
			}

			// Handle message file upload
			if state.WaitingForMessageFile && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForBroadcastMessage(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			// Handle manual message input
			if state.WaitingForMessageManual {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleManualMessageInput(input, chatID, telegramBot)
				continue
			}

			// Handle target groups file upload
			if state.WaitingForTargetGroups && update.Message.Document != nil {
				fileName := update.Message.Document.FileName
				if strings.HasSuffix(strings.ToLower(fileName), ".txt") {
					handlers.HandleFileInputForBroadcastTarget(update.Message.Document.FileID, chatID, telegramBot, telegramBot.Token)
					continue
				}
			}

			// Handle target groups manual input
			if state.WaitingForTargetGroups {
				input := strings.TrimSpace(update.Message.Text)
				handlers.HandleTargetGroupsInput(input, chatID, telegramBot)
				continue
			}
		}

		// Default response
		msg := tgbotapi.NewMessage(chatID, "ℹ️ Gunakan /menu untuk melihat menu utama atau /help untuk bantuan.")
		telegramBot.Send(msg)
	}
}
