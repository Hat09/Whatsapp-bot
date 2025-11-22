package handlers

import (
	"regexp"
	"strconv"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// GetClientForUser mendapatkan client yang benar untuk user berdasarkan Telegram ID
// Helper function untuk memastikan setiap user menggunakan client mereka sendiri
func GetClientForUser(telegramID int64, telegramBot *tgbotapi.BotAPI, fallbackClient *whatsmeow.Client) *whatsmeow.Client {
	// Coba ambil dari session terlebih dahulu
	session, err := GetUserSession(telegramID, telegramBot)
	if err == nil && session != nil && session.Client != nil {
		return session.Client
	}

	// Fallback: coba ambil dari account manager
	am := GetAccountManager()
	account := am.GetAccountByTelegramID(telegramID)
	if account != nil {
		client := am.GetClient(account.ID)
		if client != nil {
			return client
		}
	}

	// Fallback terakhir: gunakan fallbackClient
	return fallbackClient
}

// GetAccountForUser mendapatkan account yang benar untuk user berdasarkan Telegram ID
func GetAccountForUser(telegramID int64, telegramBot *tgbotapi.BotAPI) *WhatsAppAccount {
	// Coba ambil dari session terlebih dahulu
	session, err := GetUserSession(telegramID, telegramBot)
	if err == nil && session != nil && session.Account != nil {
		return session.Account
	}

	// Fallback: coba ambil dari account manager
	am := GetAccountManager()
	return am.GetAccountByTelegramID(telegramID)
}

// EnsureDBConfigForUser memastikan dbConfig di-update dengan database user yang benar
func EnsureDBConfigForUser(telegramID int64, account *WhatsAppAccount) {
	if account == nil || account.BotDataDBPath == "" {
		return
	}

	// Parse Telegram ID dari BotDataDBPath
	re := regexp.MustCompile(`bot_data\((\d+)\)>`)
	matches := re.FindStringSubmatch(account.BotDataDBPath)
	if len(matches) >= 2 {
		if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			telegramID = parsedID
		}
	}

	// Update dbConfig
	utils.SetDBConfig(telegramID, account.PhoneNumber)
	// Reset database pool untuk memastikan menggunakan database yang benar
	utils.CloseDBPools()

	utils.GetLogger().Info("EnsureDBConfigForUser: TelegramID=%d, Phone=%s, DBPath=%s",
		telegramID, account.PhoneNumber, account.BotDataDBPath)
}
