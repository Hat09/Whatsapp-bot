package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// AutoFetchGroupsAfterLogin otomatis mengambil dan menyimpan daftar grup setelah login berhasil
// Fungsi ini dipanggil setelah pairing berhasil atau saat startup jika client sudah login
func AutoFetchGroupsAfterLogin(client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI, chatID int64) {
	if client == nil || client.Store.ID == nil {
		return
	}

	// Jalankan di goroutine agar tidak blocking
	go func() {
		logger := utils.GetGrupLogger()
		logger.Info("Auto-fetching groups after login...")

		// Tunggu sebentar untuk memastikan koneksi stabil
		time.Sleep(2 * time.Second)

		if !client.IsConnected() {
			logger.Warn("Client tidak terhubung, skip auto-fetch groups")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Ambil semua grup menggunakan GetJoinedGroups()
		joinedGroups, err := client.GetJoinedGroups(ctx)
		if err != nil {
			logger.Warn("Gagal mengambil grup dengan GetJoinedGroups(): %v", err)
			return
		}

		if len(joinedGroups) == 0 {
			logger.Info("Tidak ada grup yang ditemukan")
			return
		}

		// Convert dan simpan ke database
		groupsToSave := make(map[string]string)
		for _, group := range joinedGroups {
			if group != nil {
				jidStr := group.JID.String()
				if strings.HasSuffix(jidStr, "@g.us") {
					groupName := group.Name
					if groupName == "" {
						groupName = fmt.Sprintf("Grup %s", group.JID.User)
					}

					// Simpan ke map untuk batch save
					groupsToSave[jidStr] = groupName
				}
			}
		}

		if len(groupsToSave) > 0 {
			if err := utils.BatchSaveGroupsToDB(groupsToSave); err != nil {
				logger.Error("Gagal batch save groups setelah login: %v", err)
			} else {
				logger.Info("✅ Berhasil auto-save %d grup ke database setelah login", len(groupsToSave))

				// Kirim notifikasi ke user (opsional, hanya jika chatID tersedia)
				if chatID != 0 && telegramBot != nil {
					notifMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ **Grup Otomatis Terdeteksi**\n\n📊 %d grup telah ditambahkan ke database.\n\nGunakan menu '👥 Grup' untuk melihat daftar lengkap.", len(groupsToSave)))
					notifMsg.ParseMode = "Markdown"
					telegramBot.Send(notifMsg)
				}
			}
		}
	}()
}

// StartPeriodicGroupRefresh memulai background auto-refresh grup secara berkala
// Refresh akan berjalan setiap intervalTime dan memperbarui database secara otomatis
// Fungsi ini menggunakan GetJoinedGroups() dari whatsmeow untuk efisiensi maksimal
func StartPeriodicGroupRefresh(intervalTime time.Duration) {
	go func() {
		logger := utils.GetGrupLogger()
		logger.Info("🚀 Periodic group refresh started (interval: %v)", intervalTime)

		ticker := time.NewTicker(intervalTime)
		defer ticker.Stop()

		// Jalankan refresh pertama setelah interval (bukan langsung)
		for range ticker.C {
			performPeriodicGroupRefresh()
		}
	}()

	// Jalankan refresh pertama langsung saat start
	performPeriodicGroupRefresh()
}

// performPeriodicGroupRefresh melakukan refresh grup dari whatsmeow ke database
func performPeriodicGroupRefresh() {
	// Import GetWhatsAppClient dari handlers
	client := GetWhatsAppClient()
	if client == nil || client.Store.ID == nil {
		return
	}

	logger := utils.GetGrupLogger()

	// Cek apakah client masih connected
	if !client.IsConnected() {
		logger.Debug("Client tidak terhubung, skip periodic refresh")
		return
	}

	// Ambil semua grup menggunakan GetJoinedGroups()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	joinedGroups, err := client.GetJoinedGroups(ctx)
	cancel()

	if err != nil {
		logger.Debug("Gagal mengambil grup dengan GetJoinedGroups(): %v", err)
		return
	}

	if len(joinedGroups) == 0 {
		logger.Debug("Tidak ada grup yang ditemukan dalam periodic refresh")
		return
	}

	// Convert dan simpan ke database
	groupsToSave := make(map[string]string)
	for _, group := range joinedGroups {
		if group != nil {
			jidStr := group.JID.String()
			if strings.HasSuffix(jidStr, "@g.us") {
				groupName := group.Name
				if groupName == "" {
					groupName = fmt.Sprintf("Grup %s", group.JID.User)
				}

				// Simpan ke map untuk batch save
				groupsToSave[jidStr] = groupName
			}
		}
	}

	if len(groupsToSave) > 0 {
		if err := utils.BatchSaveGroupsToDB(groupsToSave); err != nil {
			logger.Error("Gagal batch save groups (periodic): %v", err)
		} else {
			logger.Info("🔄 Periodic refresh: %d grup updated di database", len(groupsToSave))
		}
	}
}
