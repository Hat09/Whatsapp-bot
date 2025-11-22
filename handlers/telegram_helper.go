package handlers

import (
	"fmt"

	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var TelegramConfig *utils.TelegramConfig

// SetTelegramConfig mengatur konfigurasi Telegram
func SetTelegramConfig(config *utils.TelegramConfig) {
	TelegramConfig = config
}

// SendToTelegram mengirim pesan ke Telegram user yang diizinkan
func SendToTelegram(message string) {
	if TgBot != nil && TelegramConfig != nil {
		msg := tgbotapi.NewMessage(TelegramConfig.UserAllowedID, message)
		TgBot.Send(msg)
	}
	// Tetap print ke console untuk logging
	fmt.Println(message)
}
