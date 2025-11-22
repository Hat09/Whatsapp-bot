package core

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	"whatsapp-bot/handlers"
	"whatsapp-bot/ui"
	"whatsapp-bot/utils"
)

// StartupConfig menyimpan konfigurasi untuk startup
type StartupConfig struct {
	TelegramConfig *utils.TelegramConfig
	WhatsAppDBPath string
	BotDataDBPath  string
}

// StartupManager mengelola proses startup aplikasi
type StartupManager struct {
	config       *StartupConfig
	logger       *utils.AppLogger
	telegramBot  *tgbotapi.BotAPI
	waClient     *whatsmeow.Client
	deviceStore  interface{} // sqlstore.Device (interface untuk compatibility)
	eventHandler func(interface{})
}

// NewStartupManager membuat StartupManager baru
func NewStartupManager() *StartupManager {
	return &StartupManager{
		logger: utils.GetLogger(),
	}
}

// SetEventHandler mengatur event handler untuk WhatsApp
func (sm *StartupManager) SetEventHandler(handler func(interface{})) {
	sm.eventHandler = handler
}

// Initialize melakukan inisialisasi awal aplikasi
func (sm *StartupManager) Initialize() error {
	sm.logger.Phase("Initializing Application...")

	// Phase 1: Load Configuration
	if err := sm.loadConfiguration(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Phase 2: Initialize Telegram Bot
	if err := sm.initializeTelegram(); err != nil {
		return fmt.Errorf("failed to initialize Telegram: %w", err)
	}

	// Phase 3: Initialize Database
	if err := sm.initializeDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Phase 4: Initialize WhatsApp Client
	if err := sm.initializeWhatsApp(); err != nil {
		return fmt.Errorf("failed to initialize WhatsApp: %w", err)
	}

	// Phase 5: Finalize Setup
	if err := sm.finalizeSetup(); err != nil {
		return fmt.Errorf("failed to finalize setup: %w", err)
	}

	sm.logger.Success("Application initialized successfully")
	return nil
}

// loadConfiguration memuat konfigurasi aplikasi
func (sm *StartupManager) loadConfiguration() error {
	sm.logger.Phase("Loading configuration...")

	telegramConfig, err := utils.LoadTelegramConfig()
	if err != nil {
		return fmt.Errorf("failed to load Telegram config: %w", err)
	}

	// Detect database paths
	// IMPORTANT: Jangan load dbConfig dari file jika multi-account akan digunakan
	// dbConfig akan di-set berdasarkan currentAccount di finalizeSetup()
	whatsappDBPath := "whatsapp.db"
	botDataDBPath := "bot_data.db"

	// Skip LoadDBConfigFromFile() jika akan menggunakan multi-account
	// dbConfig akan di-set di finalizeSetup() berdasarkan currentAccount
	// Hanya load jika benar-benar tidak ada account (single account mode)
	// Tapi kita tidak tahu apakah ada account atau tidak sebelum LoadAccounts()
	// Solusi: Jangan load sekarang, biarkan di-set di finalizeSetup()

	// Load config hanya untuk backward compatibility dengan single account
	// Jika nanti ada multi-account, dbConfig akan di-overwrite di finalizeSetup()
	if utils.LoadDBConfigFromFile() {
		whatsappDBPath = utils.GetWhatsAppDBPath()
		botDataDBPath = utils.GetBotDataDBPath()
		sm.logger.Info("Using existing database: %s (will be overridden by multi-account if available)", whatsappDBPath)
	} else {
		sm.logger.Info("Using default database (will be renamed after pairing)")
	}

	sm.config = &StartupConfig{
		TelegramConfig: telegramConfig,
		WhatsAppDBPath: whatsappDBPath,
		BotDataDBPath:  botDataDBPath,
	}

	sm.logger.Success("Configuration loaded")
	return nil
}

// initializeTelegram menginisialisasi Telegram bot
func (sm *StartupManager) initializeTelegram() error {
	sm.logger.Phase("Initializing Telegram bot...")

	telegramBot, err := tgbotapi.NewBotAPI(sm.config.TelegramConfig.TelegramToken)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	sm.telegramBot = telegramBot
	handlers.SetTelegramConfig(sm.config.TelegramConfig)

	// Welcome message akan dikirim di finalizeSetup() bersamaan dengan dashboard
	// untuk menghindari spam message saat startup

	sm.logger.Success("Telegram bot initialized")
	return nil
}

// initializeDatabase menginisialisasi database
func (sm *StartupManager) initializeDatabase() error {
	sm.logger.Phase("Initializing database...")

	// Ensure database is writable
	if err := utils.EnsureDatabaseWritable(sm.config.WhatsAppDBPath); err != nil {
		return fmt.Errorf("database not writable: %w", err)
	}

	// Setup bot database
	if err := utils.SetupBotDB(); err != nil {
		sm.logger.Warn("Failed to setup bot database: %v", err)
		// Continue anyway, will be retried later
	}

	sm.logger.Success("Database initialized")
	return nil
}

// initializeWhatsApp menginisialisasi WhatsApp client
func (sm *StartupManager) initializeWhatsApp() error {
	sm.logger.Phase("Initializing WhatsApp client...")

	// Cek apakah multi-account sudah ada (load lebih awal untuk auto-login)
	// Jika ada multi-account, skip initializeWhatsApp() di sini
	// Biarkan finalizeSetup() handle dengan CreateClient() untuk setiap account
	handlers.InitAccountDB()
	am := handlers.GetAccountManager()
	am.SetTelegramBot(sm.telegramBot)

	if err := am.LoadAccounts(); err == nil {
		accountCount := am.GetAccountCount()
		if accountCount > 0 {
			sm.logger.Info("Multi-account: Found %d existing accounts, will use multi-account system for auto-login", accountCount)
			// Skip initializeWhatsApp() karena finalizeSetup() akan handle dengan CreateClient()
			// Return nil untuk melanjutkan ke finalizeSetup()
			return nil
		}
	}

	// Jika tidak ada multi-account, lanjutkan dengan single account mode
	// Setup WhatsApp database store
	dbLog := waLog.Stdout("Database", "ERROR", true)

	// Untuk pairing baru: pastikan menggunakan database default yang kosong
	// Jika DB config sudah direset (setelah logout), gunakan default
	dbPath := sm.config.WhatsAppDBPath
	if utils.GetDBConfig() == nil {
		// Reset: gunakan database default untuk pairing baru
		// Ini memastikan pairing baru tidak menggunakan database lama
		dbPath = "whatsapp.db"
		sm.config.WhatsAppDBPath = dbPath // Update config juga
	}

	// FIXED: Tambahkan _busy_timeout dan _locking_mode untuk mencegah "database table is locked" saat concurrent access
	// Gunakan DELETE mode untuk menghilangkan -shm dan -wal files
	dbConnectionString := fmt.Sprintf("file:%s?_foreign_keys=on&mode=rwc&_journal_mode=DELETE&cache=shared&_busy_timeout=10000&_sync=1&_locking_mode=EXCLUSIVE",
		dbPath)

	container, err := sqlstore.New(context.Background(), "sqlite3", dbConnectionString, dbLog)
	if err != nil {
		return fmt.Errorf("failed to create SQL store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get device store: %w", err)
	}

	sm.deviceStore = deviceStore

	// Create WhatsApp client
	baseLog := waLog.Stdout("Client", "ERROR", true)
	clientLog := &utils.FilteredLogger{Logger: baseLog}
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	sm.waClient = waClient
	handlers.SetClients(waClient, sm.telegramBot)

	// Register event handler (will be set from main)
	if sm.eventHandler != nil {
		waClient.AddEventHandler(sm.eventHandler)
	}

	// Connect to WhatsApp
	sm.logger.Info("Connecting to WhatsApp...")
	if err := waClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Wait for connection to be established
	if err := sm.waitForConnection(); err != nil {
		return fmt.Errorf("connection timeout: %w", err)
	}

	sm.logger.Success("WhatsApp client connected")
	return nil
}

// waitForConnection menunggu koneksi WhatsApp established
func (sm *StartupManager) waitForConnection() error {
	timeout := 15 * time.Second
	checkInterval := 500 * time.Millisecond
	elapsed := time.Duration(0)

	for !sm.waClient.IsConnected() && elapsed < timeout {
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	if !sm.waClient.IsConnected() {
		return fmt.Errorf("connection not established after %v", timeout)
	}

	// Additional wait to ensure fully established (recommended by whatsmeow)
	time.Sleep(1 * time.Second)

	return nil
}

// finalizeSetup menyelesaikan setup aplikasi
func (sm *StartupManager) finalizeSetup() error {
	sm.logger.Phase("Finalizing setup...")

	// Update database config if logged in (hanya jika waClient ada)
	if sm.waClient != nil && sm.waClient.Store.ID != nil {
		whatsappNumber := sm.waClient.Store.ID.User
		if utils.GetDBConfig() == nil {
			// Use first admin, first allowed user, or UserAllowedID for backward compatibility
			var telegramID int64
			if len(sm.config.TelegramConfig.AdminIDs) > 0 {
				telegramID = sm.config.TelegramConfig.AdminIDs[0]
			} else if len(sm.config.TelegramConfig.AllowedUserIDs) > 0 {
				telegramID = sm.config.TelegramConfig.AllowedUserIDs[0]
			} else {
				telegramID = sm.config.TelegramConfig.UserAllowedID // Backward compatibility
			}
			utils.SetDBConfig(telegramID, whatsappNumber)
		}
	}

	// Setup bot database (retry if needed)
	if err := utils.SetupBotDB(); err != nil {
		sm.logger.Warn("Failed to setup bot database: %v", err)
	} else {
		handlers.SendToTelegram("✅ Bot database siap")
	}

	// Initialize multi-account manager (jika belum di-init di initializeWhatsApp)
	am := handlers.GetAccountManager()
	if am == nil || am.GetAccountCount() == 0 {
		// Coba init dan load jika belum
		if err := handlers.InitAccountDB(); err != nil {
			sm.logger.Warn("Failed to initialize account DB: %v", err)
		} else {
			am = handlers.GetAccountManager()
			am.SetTelegramBot(sm.telegramBot)

			// Load accounts dari database master
			if err := am.LoadAccounts(); err != nil {
				sm.logger.Warn("Failed to load accounts: %v", err)
			}
		}
	}

	// Pastikan am sudah ada
	am = handlers.GetAccountManager()
	am.SetTelegramBot(sm.telegramBot)

	accountCount := am.GetAccountCount()
	if accountCount > 0 {
		sm.logger.Info("Multi-account: Loaded %d WhatsApp accounts from database", accountCount)

		// Validasi semua accounts saat startup dan hapus yang terblokir/logout
		sm.logger.Phase("Validating all accounts on startup...")
		allAccounts := am.GetAllAccounts()
		blockedAccounts := []int{}

		for _, acc := range allAccounts {
			isValid, err := am.ValidateAccount(acc.ID)
			if err != nil {
				sm.logger.Warn("Multi-account: Error validating account %d (%s): %v", acc.ID, acc.PhoneNumber, err)
				// Jika error validasi, anggap account tidak valid
				isValid = false
			}

			if !isValid {
				sm.logger.Warn("Multi-account: Account %d (%s) terblokir/logout, akan dihapus", acc.ID, acc.PhoneNumber)
				blockedAccounts = append(blockedAccounts, acc.ID)
			}
		}

		// Hapus semua account yang terblokir/logout
		for _, blockedID := range blockedAccounts {
			blockedAcc := am.GetAccount(blockedID)
			if blockedAcc != nil {
				sm.logger.Info("Multi-account: Menghapus account terblokir %d (%s) beserta file database-nya", blockedID, blockedAcc.PhoneNumber)
				if err := am.RemoveAccount(blockedID); err != nil {
					sm.logger.Warn("Multi-account: Gagal hapus account terblokir %d: %v", blockedID, err)
				} else {
					sm.logger.Info("Multi-account: ✅ Account terblokir %d (%s) berhasil dihapus", blockedID, blockedAcc.PhoneNumber)

					// Kirim notifikasi ke Telegram
					notification := fmt.Sprintf("🗑️ **AKUN TERBLOKIR - FILE DIHAPUS (STARTUP)**\n\nAkun +%s terdeteksi terblokir/logout saat startup.\n\n📁 **File yang dihapus:**\n• WhatsApp database (db, -shm, -wal)\n• Bot data database (db, -shm, -wal)\n\n✅ File database telah dihapus secara otomatis dari server.",
						blockedAcc.PhoneNumber)
					handlers.SendToTelegram(notification)
				}
			}
		}

		// Reload accounts setelah menghapus yang terblokir
		if len(blockedAccounts) > 0 {
			sm.logger.Info("Multi-account: Reloading accounts setelah menghapus %d account terblokir", len(blockedAccounts))
			if err := am.LoadAccounts(); err != nil {
				sm.logger.Warn("Multi-account: Gagal reload accounts: %v", err)
			}
			accountCount = am.GetAccountCount()
			sm.logger.Info("Multi-account: Sisa %d valid accounts setelah cleanup", accountCount)
		}
	}

	// Cleanup file database yang tidak terdaftar di database master (orphaned files)
	// IMPORTANT: Dipanggil SETELAH validasi account dan reload untuk memastikan data terbaru
	// CRITICAL: Dipanggil DI LUAR blok if accountCount > 0 agar cleanup juga berjalan jika tidak ada account
	// (semua file database akan dianggap orphaned jika tidak ada account terdaftar)
	sm.logger.Phase("Cleaning up orphaned database files...")
	orphanedFilesCount := handlers.CleanupOrphanedDBFiles(am)
	if orphanedFilesCount > 0 {
		sm.logger.Info("Multi-account: ✅ Menghapus %d orphaned database files saat startup", orphanedFilesCount)

		// Kirim notifikasi ke Telegram
		var targetUserID int64
		if len(sm.config.TelegramConfig.AdminIDs) > 0 {
			targetUserID = sm.config.TelegramConfig.AdminIDs[0]
		} else if len(sm.config.TelegramConfig.AllowedUserIDs) > 0 {
			targetUserID = sm.config.TelegramConfig.AllowedUserIDs[0]
		} else {
			targetUserID = sm.config.TelegramConfig.UserAllowedID
		}

		notification := fmt.Sprintf("🧹 **CLEANUP ORPHANED FILES (STARTUP)**\n\n✅ %d file database yang tidak terdaftar telah dihapus secara otomatis dari server.\n\n📁 **File yang dihapus:**\n• File database orphaned (tidak terdaftar di database master)\n• File pendukung (-shm, -wal)", orphanedFilesCount)
		msg := tgbotapi.NewMessage(targetUserID, notification)
		msg.ParseMode = "Markdown"
		if sm.telegramBot != nil {
			sm.telegramBot.Send(msg)
		}
	} else {
		sm.logger.Info("Multi-account: Tidak ada orphaned database files ditemukan")
	}

	// Set current client jika ada akun yang loaded (kembali ke dalam blok if accountCount > 0 jika perlu)
	if accountCount > 0 {
		currentAccount := am.GetCurrentAccount()
		if currentAccount != nil {
			sm.logger.Info("Multi-account: Using account ID %d (%s) as current", currentAccount.ID, currentAccount.PhoneNumber)

			// Update dbConfig untuk account yang aktif saat startup
			// IMPORTANT: Set dbConfig SEBELUM membuat client atau menggunakan database
			if currentAccount.BotDataDBPath != "" {
				// Parse Telegram ID dari BotDataDBPath
				// Hanya support format baru: bot_data-{telegramID}-{phoneNumber}.db
				reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
				matchesNew := reNew.FindStringSubmatch(currentAccount.BotDataDBPath)
				if len(matchesNew) >= 2 {
					telegramID, err := strconv.ParseInt(matchesNew[1], 10, 64)
					if err == nil && telegramID > 0 {
						utils.SetDBConfig(telegramID, currentAccount.PhoneNumber)
						sm.logger.Info("Multi-account: Updated dbConfig for account %s with TelegramID=%d", currentAccount.PhoneNumber, telegramID)

						// IMPORTANT: Close pools dan rebuild dengan database yang benar
						utils.CloseDBPools()
						sm.logger.Info("Multi-account: Closed database pools to use new account database")
					}
				}
			}

			// Verifikasi dbConfig sudah benar sebelum membuat client
			expectedDBPath := utils.GetBotDataDBPath()
			if currentAccount.BotDataDBPath != "" && expectedDBPath != currentAccount.BotDataDBPath {
				sm.logger.Warn("Multi-account: dbConfig mismatch! Expected: %s, Got: %s. Phone: %s", currentAccount.BotDataDBPath, expectedDBPath, currentAccount.PhoneNumber)
				// Force update dbConfig lagi
				// Hanya support format baru
				reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
				matchesNew := reNew.FindStringSubmatch(currentAccount.BotDataDBPath)
				if len(matchesNew) >= 2 {
					telegramID, err := strconv.ParseInt(matchesNew[1], 10, 64)
					if err == nil && telegramID > 0 {
						utils.SetDBConfig(telegramID, currentAccount.PhoneNumber)
						sm.logger.Info("Multi-account: Force re-updated dbConfig for account %s", currentAccount.PhoneNumber)
						utils.CloseDBPools()
					}
				}
			} else {
				sm.logger.Info("Multi-account: dbConfig verified - DBPath: %s", expectedDBPath)
			}

			// Hanya create client jika waClient belum ada (dari initializeWhatsApp)
			if sm.waClient == nil {
				client, err := am.CreateClient(currentAccount.ID)
				if err == nil {
					handlers.SetClients(client, sm.telegramBot)
					sm.waClient = client
					sm.logger.Success("Multi-account: Successfully loaded and connected account %s", currentAccount.PhoneNumber)

					// Update status setelah berhasil connect
					if err := am.UpdateAccountStatus(currentAccount.ID, "active"); err != nil {
						sm.logger.Warn("Multi-account: Failed to update status for account %d: %v", currentAccount.ID, err)
					}
				} else {
					sm.logger.Warn("Multi-account: Failed to create client for account %d: %v", currentAccount.ID, err)
					// Update status ke inactive jika gagal connect
					_ = am.UpdateAccountStatus(currentAccount.ID, "inactive")
				}
			} else {
				// Client sudah ada dari initializeWhatsApp, pastikan event handler terdaftar
				if sm.eventHandler != nil {
					sm.waClient.AddEventHandler(sm.eventHandler)
				}
				sm.logger.Info("Multi-account: Using existing client for account %s", currentAccount.PhoneNumber)

				// Sync status berdasarkan koneksi aktual
				_ = am.SyncAccountStatus(currentAccount.ID)
			}

			// Verifikasi dan update paths jika format berubah
			// Generate path sesuai format baru: DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db
			if utils.GetDBConfig() != nil && utils.GetDBConfig().TelegramID > 0 {
				// CRITICAL FIX: Gunakan path dengan folder user
				expectedDBPath := utils.GenerateDBName(utils.GetDBConfig().TelegramID, currentAccount.PhoneNumber, "whatsmeow")
				expectedBotDataDBPath := utils.GenerateDBName(utils.GetDBConfig().TelegramID, currentAccount.PhoneNumber, "bot_data")

				// Hanya update jika path berbeda dan format baru sudah ditetapkan
				if currentAccount.DBPath != expectedDBPath || currentAccount.BotDataDBPath != expectedBotDataDBPath {
					sm.logger.Info("Multi-account: Updating database paths for account %d to new format", currentAccount.ID)
					_ = am.UpdateAccountPaths(currentAccount.ID, expectedDBPath, expectedBotDataDBPath)
				}
			}
		}

		// Start periodic sync untuk memastikan data selalu akurat (setiap 2 menit)
		go func() {
			ticker := time.NewTicker(2 * time.Minute)
			defer ticker.Stop()

			for range ticker.C {
				am.SyncAllAccountsStatus()
			}
		}()
		sm.logger.Info("Multi-account: Started periodic account status sync (every 2 minutes)")
	} else {
		sm.logger.Info("Multi-account: No accounts found in database, will use default pairing")
	}

	// Show appropriate UI based on login status
	// Send to first admin/user
	var targetUserID int64
	if len(sm.config.TelegramConfig.AdminIDs) > 0 {
		targetUserID = sm.config.TelegramConfig.AdminIDs[0]
	} else if len(sm.config.TelegramConfig.AllowedUserIDs) > 0 {
		targetUserID = sm.config.TelegramConfig.AllowedUserIDs[0]
	} else {
		targetUserID = sm.config.TelegramConfig.UserAllowedID // Backward compatibility
	}

	// IMPORTANT: Check multi-account client first, fallback to single account client
	currentClient := am.GetCurrentClient()
	currentAccount := am.GetCurrentAccount()

	// CRITICAL FIX: Coba auto-connect ke SEMUA account (tidak peduli status)
	// Ini mengatasi masalah: account ada di DB tapi status "inactive", program kembali ke pairing menu
	if currentClient == nil {
		allAccounts := am.GetAllAccounts()
		sm.logger.Info("finalizeSetup: Client belum ada, mencoba auto-connect ke %d account yang ditemukan...", len(allAccounts))

		// Coba connect ke semua account secara berurutan
		// Mulai dari currentAccount jika ada, lalu coba yang lain
		accountsToTry := []*handlers.WhatsAppAccount{}

		// Prioritaskan currentAccount jika ada
		if currentAccount != nil {
			accountsToTry = append(accountsToTry, currentAccount)
		}

		// Tambahkan account lain yang belum ada di list
		for _, acc := range allAccounts {
			if currentAccount == nil || acc.ID != currentAccount.ID {
				accountsToTry = append(accountsToTry, acc)
			}
		}

		// Coba connect ke setiap account
		for _, acc := range accountsToTry {
			sm.logger.Info("finalizeSetup: Mencoba auto-connect ke account %d (+%s)...", acc.ID, acc.PhoneNumber)

			testClient, err := am.CreateClient(acc.ID)
			if err != nil {
				sm.logger.Warn("finalizeSetup: Gagal auto-connect ke account %d: %v", acc.ID, err)
				// Update status ke inactive jika gagal
				_ = am.UpdateAccountStatus(acc.ID, "inactive")
				continue
			}

			// Cek apakah client valid
			if testClient != nil && testClient.Store != nil && testClient.Store.ID != nil && testClient.IsConnected() {
				currentClient = testClient
				am.SetCurrentAccount(acc.ID)
				// Update status ke active setelah berhasil connect
				_ = am.UpdateAccountStatus(acc.ID, "active")
				sm.logger.Info("finalizeSetup: ✅ Berhasil auto-connect ke account %d (+%s)", acc.ID, acc.PhoneNumber)
				break
			} else {
				sm.logger.Warn("finalizeSetup: Client untuk account %d tidak valid setelah connect", acc.ID)
				_ = am.UpdateAccountStatus(acc.ID, "inactive")
			}
		}

		if currentClient == nil {
			sm.logger.Warn("finalizeSetup: Tidak ada account yang bisa di-connect, akan tampilkan pairing menu")
		} else {
			// Update currentAccount setelah berhasil connect
			currentAccount = am.GetCurrentAccount()
		}
	}

	// Determine which client to use
	var displayClient *whatsmeow.Client
	if currentClient != nil && currentClient.Store != nil && currentClient.Store.ID != nil {
		displayClient = currentClient
	} else if sm.waClient != nil && sm.waClient.Store != nil && sm.waClient.Store.ID != nil {
		displayClient = sm.waClient
	}

	if displayClient != nil {
		handlers.SendToTelegram("✅ Bot WhatsApp sudah terhubung!")
		time.Sleep(500 * time.Millisecond) // Minimal delay

		// Update dbConfig untuk memastikan database path benar
		if currentAccount != nil {
			utils.SetDBConfig(targetUserID, currentAccount.PhoneNumber)
		}

		ui.ShowMainMenu(sm.telegramBot, targetUserID, displayClient)
	} else {
		// Hanya tampilkan pairing menu jika BENAR-BENAR tidak ada account aktif
		allAccounts := am.GetAllAccounts()
		hasActiveAccount := false
		for _, acc := range allAccounts {
			if acc.Status == "active" {
				hasActiveAccount = true
				break
			}
		}

		if hasActiveAccount {
			// Ada account aktif tapi tidak bisa di-connect (mungkin terblokir semua)
			handlers.SendToTelegram("⚠️ Akun WhatsApp terdeteksi tapi tidak dapat terhubung.\n\nSilakan coba login ulang atau tambah akun baru.")
			time.Sleep(500 * time.Millisecond)
		} else {
			handlers.SendToTelegram("🔄 Memeriksa status login...")
			time.Sleep(500 * time.Millisecond) // Minimal delay
		}
		ui.ShowLoginPrompt(sm.telegramBot, targetUserID)
	}

	// Start periodic group refresh in background setelah UI ditampilkan
	// Refresh setiap 5 menit untuk menjaga database selalu up-to-date
	// Hanya berjalan jika client sudah login
	if sm.waClient != nil && sm.waClient.Store.ID != nil {
		handlers.StartPeriodicGroupRefresh(5 * time.Minute)
	}

	sm.logger.Success("Setup finalized")
	return nil
}

// GetTelegramBot mendapatkan Telegram bot instance
func (sm *StartupManager) GetTelegramBot() *tgbotapi.BotAPI {
	return sm.telegramBot
}

// GetWhatsAppClient mendapatkan WhatsApp client instance
func (sm *StartupManager) GetWhatsAppClient() *whatsmeow.Client {
	return sm.waClient
}

// GetConfig mendapatkan startup config
func (sm *StartupManager) GetConfig() *StartupConfig {
	return sm.config
}
