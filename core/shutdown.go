package core

import (
	"context"
	"time"

	"whatsapp-bot/handlers"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// ShutdownManager mengelola proses shutdown aplikasi
type ShutdownManager struct {
	waClient    *whatsmeow.Client
	telegramBot *tgbotapi.BotAPI
	logger      *utils.AppLogger
}

// NewShutdownManager membuat ShutdownManager baru
func NewShutdownManager(waClient *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) *ShutdownManager {
	return &ShutdownManager{
		waClient:    waClient,
		telegramBot: telegramBot,
		logger:      utils.GetLogger(),
	}
}

// Shutdown melakukan graceful shutdown aplikasi
func (sm *ShutdownManager) Shutdown(ctx context.Context) error {
	sm.logger.Phase("Initiating graceful shutdown...")

	// Notify user
	handlers.SendToTelegram("👋 Menutup bot...")

	// Stop Telegram bot updates
	if sm.telegramBot != nil {
		sm.logger.Info("Stopping Telegram bot updates...")
		sm.telegramBot.StopReceivingUpdates()
		sm.logger.Success("Telegram bot stopped")
	}

	// Disconnect WhatsApp client
	if sm.waClient != nil {
		sm.logger.Info("Disconnecting WhatsApp client...")
		sm.waClient.Disconnect()
		sm.logger.Success("WhatsApp client disconnected")
	}

	sm.logger.Success("Shutdown completed")
	return nil
}

// ShutdownWithTimeout melakukan shutdown dengan timeout
func (sm *ShutdownManager) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return sm.Shutdown(ctx)
}
