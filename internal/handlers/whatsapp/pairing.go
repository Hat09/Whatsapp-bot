package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

var WaClient *whatsmeow.Client
var TgBot *tgbotapi.BotAPI

// SetClients mengatur client WhatsApp dan Telegram
func SetClients(waClient *whatsmeow.Client, tgBot *tgbotapi.BotAPI) {
	WaClient = waClient
	TgBot = tgBot
}

// ValidatePhoneNumber memvalidasi format nomor telepon
func ValidatePhoneNumber(phone string) error {
	phone = strings.TrimSpace(phone)

	// Hapus karakter non-digit
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, " ", "")

	// Cek panjang minimal
	if len(phone) < 10 {
		return fmt.Errorf("nomor terlalu pendek (minimal 10 digit)")
	}

	// Cek apakah hanya angka
	for _, char := range phone {
		if char < '0' || char > '9' {
			return fmt.Errorf("nomor hanya boleh mengandung angka")
		}
	}

	// Cek panjang maksimal
	if len(phone) > 15 {
		return fmt.Errorf("nomor terlalu panjang (maksimal 15 digit)")
	}

	return nil
}

// ensureConnection memastikan client terhubung, reconnect jika perlu
func ensureConnection() error {
	if WaClient == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Cek apakah client sudah terhubung
	if !WaClient.IsConnected() {
		// Disconnect dulu jika masih ada koneksi yang bermasalah
		if WaClient.IsLoggedIn() {
			WaClient.Disconnect()
		}

		// Reconnect
		err := WaClient.Connect()
		if err != nil {
			return fmt.Errorf("gagal reconnect: %v", err)
		}

		// Tunggu sampai benar-benar terhubung
		maxWait := 10 * time.Second
		checkInterval := 500 * time.Millisecond
		waited := time.Duration(0)

		for !WaClient.IsConnected() && waited < maxWait {
			time.Sleep(checkInterval)
			waited += checkInterval
		}

		if !WaClient.IsConnected() {
			return fmt.Errorf("timeout: client tidak terhubung setelah reconnect")
		}
	}

	return nil
}

// PairDeviceViaTelegram melakukan pairing WhatsApp melalui Telegram bot dengan flow yang lebih terstruktur
func PairDeviceViaTelegram(phone string, chatID int64) error {
	if WaClient == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Validasi nomor telepon
	if err := ValidatePhoneNumber(phone); err != nil {
		return fmt.Errorf("nomor tidak valid: %v", err)
	}

	// Format nomor (hapus karakter non-digit)
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, " ", "")

	// Kirim pesan sedang memproses
	processingMsg := tgbotapi.NewMessage(chatID, "🔄 **MEMPERSIAPKAN PAIRING...**\n\nMemastikan koneksi ke server WhatsApp...")
	processingMsg.ParseMode = "Markdown"
	processingMsgSent, _ := TgBot.Send(processingMsg)

	// Pastikan client terhubung sebelum pairing
	if err := ensureConnection(); err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **ERROR KONEKSI**\n\n%s\n\nSilakan coba lagi dalam beberapa saat.", err))
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)

		// Hapus pesan processing
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		TgBot.Request(deleteMsg)

		return fmt.Errorf("gagal memastikan koneksi: %v", err)
	}

	// Update pesan processing
	updateMsg := tgbotapi.NewEditMessageText(chatID, processingMsgSent.MessageID, "🔄 **MEMPERSIAPKAN PAIRING...**\n\n✅ Koneksi terhubung\n⏳ Menunggu koneksi fully established...")
	updateMsg.ParseMode = "Markdown"
	TgBot.Send(updateMsg)

	// Menunggu koneksi fully established sebelum generate pairing code
	// Sesuai dokumentasi whatsmeow: perlu wait untuk QR event atau sleep 1 detik setelah Connect
	// Kita akan wait sedikit untuk memastikan connection ready
	ctxPair, cancelPair := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPair()

	// Tunggu beberapa detik untuk memastikan koneksi sudah fully established
	// Dokumentasi whatsmeow: "sleeping for a second after calling Connect will probably work too"
	time.Sleep(2 * time.Second)

	// Pastikan masih connected sebelum generate pairing code
	if !WaClient.IsConnected() {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **ERROR KONEKSI**\n\nKoneksi terputus. Silakan coba lagi.")
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)

		// Hapus pesan processing
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		TgBot.Request(deleteMsg)

		return fmt.Errorf("koneksi terputus sebelum generate pairing code")
	}

	// Update pesan processing
	updateMsg2 := tgbotapi.NewEditMessageText(chatID, processingMsgSent.MessageID, "🔄 **MEMPERSIAPKAN PAIRING...**\n\n✅ Koneksi terhubung\n✅ Koneksi fully established\n📱 Menggenerate kode pairing...")
	updateMsg2.ParseMode = "Markdown"
	TgBot.Send(updateMsg2)

	// Generate pairing code dengan context timeout
	pairingCode, err := WaClient.PairPhone(ctxPair, phone, true, whatsmeow.PairClientChrome, "Chrome (Windows)")
	if err != nil {
		// Cek apakah error terkait database read-only
		errorText := err.Error()
		var errorDetail string
		if strings.Contains(errorText, "readonly database") || strings.Contains(errorText, "read-only") {
			errorDetail = "**Masalah:** Database tidak bisa ditulis\n\n**Solusi:**\n1. Pastikan file whatsapp.db memiliki permission write\n2. Pastikan tidak ada proses lain yang menggunakan database\n3. Coba restart program\n4. Jika masih error, hapus file whatsapp.db dan coba pairing ulang"
		} else if strings.Contains(errorText, "timeout") || strings.Contains(errorText, "context deadline exceeded") {
			errorDetail = "**Masalah:** Timeout saat generate pairing code\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Pastikan server WhatsApp dapat diakses\n3. Coba lagi dalam beberapa saat"
		} else if strings.Contains(errorText, "not connected") || strings.Contains(errorText, "connection") {
			errorDetail = "**Masalah:** Koneksi terputus\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Coba restart program\n3. Coba pairing lagi"
		} else {
			errorDetail = fmt.Sprintf("**Kemungkinan penyebab:**\n• Koneksi internet bermasalah\n• Server WhatsApp sedang sibuk\n• Nomor tidak valid\n• Error: %s\n\n**Solusi:**\nSilakan coba lagi dalam beberapa saat.", err.Error())
		}

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **ERROR**\n\nGagal generate pairing code: %v\n\n%s", err, errorDetail))
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)

		// Hapus pesan processing
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		TgBot.Request(deleteMsg)

		return fmt.Errorf("gagal generate pairing code: %v", err)
	}

	// Validasi pairing code
	if pairingCode == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **ERROR**\n\nPairing code kosong. Silakan coba lagi.")
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)

		// Hapus pesan processing
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		TgBot.Request(deleteMsg)

		return fmt.Errorf("pairing code kosong")
	}

	// Log pairing code untuk debugging (tidak ditampilkan ke user untuk security)
	fmt.Printf("✅ Pairing code berhasil di-generate: %s\n", pairingCode)

	// Hapus pesan processing
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
	TgBot.Request(deleteMsg)

	// Kirim instruksi pairing menggunakan UI helper dengan nomor
	pairingInstructions := ui.FormatPairingInstructions(pairingCode, phone)
	msg := tgbotapi.NewMessage(chatID, pairingInstructions)
	msg.ParseMode = "Markdown"

	// Tambahkan tombol cancel
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan Pairing", "cancel_pairing"),
		),
	)
	msg.ReplyMarkup = keyboard
	TgBot.Send(msg)

	// Tunggu maksimal 2 menit untuk pairing dengan progress update
	startTime := time.Now()
	ticker := time.NewTicker(10 * time.Second) // Update setiap 10 detik
	defer ticker.Stop()

	var progressMessageID int

	for time.Since(startTime) < 120*time.Second {
		<-ticker.C

		// Update progress
		elapsed := int(time.Since(startTime).Seconds())
		remaining := 120 - elapsed
		progressBar := getProgressBar(elapsed, 120)

		progressText := fmt.Sprintf("⏳ **Menunggu pairing...**\n\n%s\n\n⏱️ Waktu tersisa: %d detik", progressBar, remaining)

		if progressMessageID == 0 {
			// Kirim pesan progress baru
			progressMsg := tgbotapi.NewMessage(chatID, progressText)
			progressMsg.ParseMode = "Markdown"
			sent, _ := TgBot.Send(progressMsg)
			progressMessageID = sent.MessageID
		} else {
			// Edit pesan progress
			editMsg := tgbotapi.NewEditMessageText(chatID, progressMessageID, progressText)
			editMsg.ParseMode = "Markdown"
			TgBot.Send(editMsg)
		}

		if WaClient.Store.ID != nil {
			// Hapus pesan progress
			if progressMessageID > 0 {
				deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMessageID)
				TgBot.Request(deleteMsg)
			}

			// Rename database setelah pairing berhasil
			// Format: whatsmeow(id telegram)>(nomor whatsapp).db
			whatsappNumber := WaClient.Store.ID.User
			newWhatsAppDB := utils.GenerateDBName(chatID, whatsappNumber, "whatsapp")
			newBotDataDB := utils.GenerateDBName(chatID, whatsappNumber, "bot_data")

			// Set config database
			utils.SetDBConfig(chatID, whatsappNumber)

			// Rename database files dari nama default ke nama dinamis
			err := utils.RenameDatabaseFiles("whatsapp.db", newWhatsAppDB, "bot_data.db", newBotDataDB)
			if err != nil {
				// Log error tapi tidak gagalkan pairing
				fmt.Printf("⚠️ Warning: Gagal rename database: %v\n", err)
				fmt.Printf("⚠️ Database tetap menggunakan nama default\n")
			} else {
				fmt.Printf("✅ Database berhasil di-rename:\n")
				fmt.Printf("   WhatsApp DB: %s\n", newWhatsAppDB)
				fmt.Printf("   Bot Data DB: %s\n", newBotDataDB)
			}

			// Kirim pesan success dengan format yang menarik
			successMsg := tgbotapi.NewMessage(chatID, ui.FormatPairingSuccess())
			successMsg.ParseMode = "Markdown"
			TgBot.Send(successMsg)

			// Tampilkan menu utama setelah pairing berhasil (hanya sekali)
			time.Sleep(2 * time.Second) // Tunggu sebentar sebelum menampilkan menu
			ui.ShowMainMenu(TgBot, chatID, WaClient)
			return nil
		}
	}

	// Hapus pesan progress jika masih ada
	if progressMessageID > 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMessageID)
		TgBot.Request(deleteMsg)
	}

	timeoutMsg := fmt.Sprintf("❌ **PAIRING TIMEOUT**\n\n"+
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"+
		"⏱️ Waktu pairing telah habis (2 menit)\n\n"+
		"**Kemungkinan masalah:**\n"+
		"• Kode pairing tidak dimasukkan tepat waktu\n"+
		"• Koneksi internet bermasalah\n"+
		"• Nomor telepon tidak valid\n\n"+
		"**Solusi:**\n"+
		"Silakan coba lagi dengan command:\n"+
		"`/pair %s`\n\n"+
		"Atau gunakan tombol \"🔗 Mulai Pairing\" untuk memulai ulang.", phone)

	msgTimeout := tgbotapi.NewMessage(chatID, timeoutMsg)
	msgTimeout.ParseMode = "Markdown"

	// Tambahkan tombol retry
	retryKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Coba Lagi", "start_pairing"),
		),
	)
	msgTimeout.ReplyMarkup = retryKeyboard
	TgBot.Send(msgTimeout)

	return fmt.Errorf("pairing timeout, coba lagi")
}

// getProgressBar membuat progress bar visual
func getProgressBar(current, total int) string {
	if total == 0 {
		return ""
	}

	barLength := 20
	filled := int(float64(current) / float64(total) * float64(barLength))
	if filled > barLength {
		filled = barLength
	}

	bar := ""
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	return bar
}
