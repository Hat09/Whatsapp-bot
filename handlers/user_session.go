package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// UserSession menyimpan session context untuk setiap user
// Ini memastikan setiap request menggunakan account yang benar
type UserSession struct {
	TelegramID    int64
	AccountID     int
	Account       *WhatsAppAccount
	Client        *whatsmeow.Client
	DBPath        string
	BotDataDBPath string
}

var (
	userSessions = make(map[int64]*UserSession)
	sessionMutex sync.RWMutex
)

// GetUserSession mendapatkan atau membuat session untuk user
// Ini memastikan setiap user memiliki session terpisah
func GetUserSession(telegramID int64, telegramBot *tgbotapi.BotAPI) (*UserSession, error) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	// Cek apakah session sudah ada
	if session, exists := userSessions[telegramID]; exists {
		// Verifikasi account masih valid
		am := GetAccountManager()
		account := am.GetAccount(session.AccountID)
		if account != nil {
			// Update session dengan data terbaru
			session.Account = account
			client := am.GetClient(session.AccountID)

			// CRITICAL FIX: Prioritaskan client yang sudah ada dari startup
			// Jika client sudah ada dan Store.ID valid, gunakan client tersebut (meskipun IsConnected() false)
			// Hanya buat client baru jika benar-benar tidak ada atau Store.ID nil
			if client != nil {
				// Cek apakah client masih valid (Store.ID != nil berarti account masih login)
				if client.Store != nil && client.Store.ID != nil {
					// Client valid, cek apakah perlu reconnect
					if !client.IsConnected() {
						utils.GetLogger().Info("Session client exists but not connected for TelegramID=%d, attempting reconnect...", telegramID)
						// Coba reconnect client yang sudah ada (lebih efisien daripada membuat client baru)
						if err := client.Connect(); err != nil {
							utils.GetLogger().Warn("Failed to reconnect existing client: %v, will create new client", err)
							// Jika reconnect gagal, buat client baru
							newClient, err := am.CreateClient(session.AccountID)
							if err != nil {
								utils.GetLogger().Warn("Failed to create new client: %v", err)
								session.Client = nil
							} else {
								session.Client = newClient
							}
						} else {
							// Wait for connection
							timeout := 5 * time.Second
							checkInterval := 500 * time.Millisecond
							elapsed := time.Duration(0)
							for !client.IsConnected() && elapsed < timeout {
								time.Sleep(checkInterval)
								elapsed += checkInterval
							}
							if !client.IsConnected() {
								utils.GetLogger().Warn("Reconnect timeout, will create new client")
								newClient, err := am.CreateClient(session.AccountID)
								if err != nil {
									utils.GetLogger().Warn("Failed to create new client: %v", err)
									session.Client = nil
								} else {
									session.Client = newClient
								}
							} else {
								utils.GetLogger().Info("Successfully reconnected session client for TelegramID=%d", telegramID)
								session.Client = client
							}
						}
					} else {
						// Client sudah terhubung, gunakan client tersebut
						session.Client = client
					}
				} else {
					// Store.ID nil berarti account terblokir/logout, buat client baru
					utils.GetLogger().Warn("Session client exists but Store.ID is nil for TelegramID=%d, creating new client...", telegramID)
					newClient, err := am.CreateClient(session.AccountID)
					if err != nil {
						utils.GetLogger().Warn("Failed to create new client: %v", err)
						session.Client = nil
					} else {
						session.Client = newClient
					}
				}
			} else {
				// Client tidak ada, coba buat
				utils.GetLogger().Info("No existing client for session TelegramID=%d, creating new client...", telegramID)
				newClient, err := am.CreateClient(session.AccountID)
				if err != nil {
					utils.GetLogger().Warn("Failed to create client for session: %v", err)
					session.Client = nil
				} else {
					session.Client = newClient
				}
			}

			session.DBPath = account.DBPath
			session.BotDataDBPath = account.BotDataDBPath
			return session, nil
		}
		// Account tidak valid, hapus session dan buat baru
		delete(userSessions, telegramID)
	}

	// Buat session baru
	am := GetAccountManager()
	account := am.GetAccountByTelegramID(telegramID)

	if account == nil {
		// User belum punya akun
		return nil, nil
	}

	// Pastikan account aktif
	if err := am.SetCurrentAccount(account.ID); err != nil {
		return nil, fmt.Errorf("failed to set current account: %w", err)
	}

	// Dapatkan atau buat client untuk account ini
	client := am.GetClient(account.ID)

	// CRITICAL FIX: Prioritaskan client yang sudah ada dari startup
	// Jika client sudah ada dan Store.ID valid, gunakan client tersebut (meskipun IsConnected() false)
	// Hanya buat client baru jika benar-benar tidak ada atau Store.ID nil
	if client != nil {
		// Cek apakah client masih valid (Store.ID != nil berarti account masih login)
		if client.Store != nil && client.Store.ID != nil {
			// Client valid, cek apakah perlu reconnect
			if !client.IsConnected() {
				utils.GetLogger().Info("Client exists but not connected for account %d, attempting reconnect...", account.ID)
				// Coba reconnect client yang sudah ada (lebih efisien daripada membuat client baru)
				if err := client.Connect(); err != nil {
					utils.GetLogger().Warn("Failed to reconnect existing client: %v, will create new client", err)
					// Jika reconnect gagal, buat client baru
					var err error
					client, err = am.CreateClient(account.ID)
					if err != nil {
						utils.GetLogger().Warn("Failed to create client: %v, user may need to login again", err)
						client = nil
					}
				} else {
					// Wait for connection
					timeout := 5 * time.Second
					checkInterval := 500 * time.Millisecond
					elapsed := time.Duration(0)
					for !client.IsConnected() && elapsed < timeout {
						time.Sleep(checkInterval)
						elapsed += checkInterval
					}
					if !client.IsConnected() {
						utils.GetLogger().Warn("Reconnect timeout, will create new client")
						var err error
						client, err = am.CreateClient(account.ID)
						if err != nil {
							utils.GetLogger().Warn("Failed to create client: %v", err)
							client = nil
						}
					} else {
						utils.GetLogger().Info("Successfully reconnected client for account %d", account.ID)
					}
				}
			}
			// Client sudah terhubung atau berhasil reconnect, gunakan client tersebut
		} else {
			// Store.ID nil berarti account terblokir/logout, buat client baru
			utils.GetLogger().Warn("Client exists but Store.ID is nil for account %d, creating new client...", account.ID)
			var err error
			client, err = am.CreateClient(account.ID)
			if err != nil {
				utils.GetLogger().Warn("Failed to create client: %v, user may need to login again", err)
				client = nil
			}
		}
	} else {
		// Client tidak ada, buat baru
		utils.GetLogger().Info("No existing client for account %d, creating new client...", account.ID)
		var err error
		client, err = am.CreateClient(account.ID)
		if err != nil {
			utils.GetLogger().Warn("Failed to create client: %v, user may need to login again", err)
			client = nil
		}
	}

	// FIXED: Validasi client setelah create
	if client != nil {
		// Validasi client setelah create
		if client.Store == nil || client.Store.ID == nil {
			utils.GetLogger().Warn("Client created but Store.ID is nil for account %d, user may need to login again", account.ID)
			client = nil
		} else if !client.IsConnected() {
			// Client valid tapi tidak connected, coba connect
			utils.GetLogger().Info("Client created but not connected for account %d, attempting connect...", account.ID)
			if err := client.Connect(); err != nil {
				utils.GetLogger().Warn("Failed to connect client for account %d: %v", account.ID, err)
				// Keep client, mungkin akan connect nanti
			}
		}
	}

	// CRITICAL FIX: Verifikasi client sudah terhubung sebelum menyimpan ke session
	// Jika client nil atau Store.ID nil, tetap simpan session tapi dengan client nil
	// Handler akan menampilkan login prompt jika client nil
	if client != nil && (client.Store == nil || client.Store.ID == nil) {
		utils.GetLogger().Warn("Client not connected for account %d, user may need to login again", account.ID)
		client = nil // Set ke nil agar handler tahu user perlu login
	}

	// Update global client (untuk backward compatibility)
	SetClients(client, telegramBot)

	// Update dbConfig
	telegramIDFromPath := telegramID
	if account.BotDataDBPath != "" {
		// Parse Telegram ID dari BotDataDBPath
		re := regexp.MustCompile(`bot_data\((\d+)\)>`)
		matches := re.FindStringSubmatch(account.BotDataDBPath)
		if len(matches) >= 2 {
			if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				telegramIDFromPath = parsedID
			}
		}
	}
	utils.SetDBConfig(telegramIDFromPath, account.PhoneNumber)
	utils.CloseDBPools()

	// Buat session baru
	session := &UserSession{
		TelegramID:    telegramID,
		AccountID:     account.ID,
		Account:       account,
		Client:        client,
		DBPath:        account.DBPath,
		BotDataDBPath: account.BotDataDBPath,
	}

	userSessions[telegramID] = session

	utils.GetLogger().Info("Created user session: TelegramID=%d, AccountID=%d, Phone=%s",
		telegramID, account.ID, account.PhoneNumber)

	return session, nil
}

// ClearUserSession menghapus session untuk user tertentu
func ClearUserSession(telegramID int64) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	delete(userSessions, telegramID)
}
