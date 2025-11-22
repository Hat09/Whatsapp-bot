package handlers

import (
	"whatsapp-bot/utils"

	"go.mau.fi/whatsmeow"
)

// GetActiveClientOrFallback mendapatkan client aktif atau fallback ke parameter
// Fungsi ini digunakan oleh semua proses background untuk mencegah client stale
func GetActiveClientOrFallback(fallbackClient *whatsmeow.Client) *whatsmeow.Client {
	// Ambil client aktif dari AccountManager
	activeClient := GetWhatsAppClient()

	// Jika tidak ada, fallback ke parameter
	if activeClient == nil {
		activeClient = fallbackClient
	}

	return activeClient
}

// IsClientConnected mengecek apakah client masih terhubung
// Return true jika connected, false jika tidak
func IsClientConnected(client *whatsmeow.Client) bool {
	if client == nil {
		return false
	}

	return client.IsConnected()
}

// ValidateClientForBackgroundProcess memvalidasi client sebelum digunakan di background process
// Return client yang valid atau nil jika tidak valid
// shouldStop return true jika proses harus dihentikan
func ValidateClientForBackgroundProcess(fallbackClient *whatsmeow.Client, processName string, currentIndex, totalCount int) (validClient *whatsmeow.Client, shouldStop bool) {
	// Ambil client aktif
	client := GetActiveClientOrFallback(fallbackClient)

	// Check connection
	if !IsClientConnected(client) {
		utils.GetLogger().Warn("%s: Client disconnected at item %d/%d. Stopping process.", processName, currentIndex+1, totalCount)
		return nil, true
	}

	return client, false
}
