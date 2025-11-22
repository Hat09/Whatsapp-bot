package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whatsapp-bot/ui"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var WaClient *whatsmeow.Client
var TgBot *tgbotapi.BotAPI

// SetClients mengatur client WhatsApp dan Telegram
func SetClients(waClient *whatsmeow.Client, tgBot *tgbotapi.BotAPI) {
	WaClient = waClient
	TgBot = tgBot
}

// GetWhatsAppClient mendapatkan client WhatsApp yang aktif (dari multi-account atau global)
// Fungsi ini digunakan oleh fungsi-fungsi lain untuk mendapatkan client aktif
// FIXED: Bisa return nil, caller harus handle nil check
func GetWhatsAppClient() *whatsmeow.Client {
	// Coba ambil dari AccountManager terlebih dahulu
	am := GetAccountManager()
	if am != nil {
		currentClient := am.GetCurrentClient()
		if currentClient != nil {
			return currentClient
		}
	}
	// Fallback ke global client (bisa nil)
	return WaClient
}

// ValidatePhoneNumber memvalidasi format nomor telepon
// FIXED: Tambahkan validasi format internasional dan WhatsApp-specific validation
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

	// FIXED: Validasi format internasional (country code)
	// WhatsApp memerlukan format internasional dengan country code
	// Country code biasanya 1-3 digit diikuti nomor lokal
	if len(phone) < 11 {
		// Nomor terlalu pendek untuk format internasional
		return fmt.Errorf("nomor harus dalam format internasional (contoh: 6281234567890)")
	}

	// FIXED: Validasi country code umum (1-3 digit pertama)
	// Country code harus valid (tidak boleh 0 di awal setelah country code)
	if phone[0] == '0' {
		return fmt.Errorf("nomor tidak boleh dimulai dengan 0, gunakan format internasional (contoh: 6281234567890)")
	}

	return nil
}

// ensureConnection memastikan client terhubung, reconnect jika perlu
// FIXED: Gunakan GetWhatsAppClient() untuk mendapatkan client fresh, bukan global WaClient
func ensureConnection() error {
	client := GetWhatsAppClient()
	if client == nil {
		return fmt.Errorf("WhatsApp client belum diinisialisasi")
	}

	// Cek apakah client sudah terhubung
	if !client.IsConnected() {
		// Disconnect dulu jika masih ada koneksi yang bermasalah
		if client.IsLoggedIn() {
			client.Disconnect()
		}

		// Reconnect
		err := client.Connect()
		if err != nil {
			return fmt.Errorf("gagal reconnect: %v", err)
		}

		// Tunggu sampai benar-benar terhubung
		maxWait := 10 * time.Second
		checkInterval := 500 * time.Millisecond
		waited := time.Duration(0)

		for !client.IsConnected() && waited < maxWait {
			time.Sleep(checkInterval)
			waited += checkInterval
		}

		if !client.IsConnected() {
			return fmt.Errorf("timeout: client tidak terhubung setelah reconnect")
		}
	}

	return nil
}

// PairDeviceViaTelegram melakukan pairing WhatsApp melalui Telegram bot dengan flow yang lebih terstruktur
// FIXED: Tambahkan input validation untuk parameter
func PairDeviceViaTelegram(phone string, chatID int64) error {
	// FIXED: Input validation untuk parameter
	if phone == "" {
		return fmt.Errorf("nomor telepon tidak boleh kosong")
	}
	if chatID == 0 {
		return fmt.Errorf("chat ID tidak valid")
	}
	// CRITICAL FIX: Selalu buat client baru untuk pairing, jangan gunakan client yang sudah ada
	// Ini mencegah konflik websocket saat multiple user melakukan pairing bersamaan
	utils.GetLogger().Info("Creating new client for pairing: TelegramID=%d, Phone=%s", chatID, phone)

	// Format nomor (hapus karakter non-digit)
	cleanPhone := strings.ReplaceAll(phone, "+", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")

	// Generate database path untuk user ini
	// Gunakan path dengan folder user: DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db
	dbPath := utils.GenerateDBName(chatID, cleanPhone, "whatsmeow")

	// CRITICAL FIX: Pastikan folder database ada dan memiliki permission write
	// Ini mencegah error "readonly database" saat WhatsMeow mencoba save device store
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("gagal membuat folder database: %v", err)
	}

	// CRITICAL FIX: Pastikan folder memiliki permission write
	if err := os.Chmod(dbDir, 0755); err != nil {
		utils.GetLogger().Warn("[Pairing] Gagal chmod folder database: %v (continuing anyway)", err)
	}

	// CRITICAL FIX: Verifikasi folder bisa ditulis
	testFile := filepath.Join(dbDir, ".write_test")
	if testF, err := os.Create(testFile); err != nil {
		return fmt.Errorf("folder database tidak bisa ditulis (permission denied): %v", err)
	} else {
		testF.Close()
		os.Remove(testFile)
		utils.GetLogger().Info("[Pairing] Folder database verified: writable, path=%s", dbDir)
	}

	// CRITICAL FIX: Hapus database lama jika ada untuk memastikan pairing fresh
	if _, err := os.Stat(dbPath); err == nil {
		utils.GetLogger().Info("[Pairing] Database lama ditemukan, menghapus untuk pairing fresh: %s", dbPath)
		os.Remove(dbPath)
		os.Remove(dbPath + "-shm")
		os.Remove(dbPath + "-wal")
		utils.GetLogger().Info("[Pairing] Database lama berhasil dihapus, siap untuk pairing fresh")
	}

	// Setup WhatsApp database store untuk pairing baru
	dbLog := waLog.Stdout("Database", "ERROR", true)
	// CRITICAL FIX: Pastikan connection string menggunakan mode write dengan permission yang benar
	// Gunakan DELETE mode untuk menghilangkan -shm dan -wal files
	// Mode rwc = read-write-create, memastikan database bisa ditulis
	// _sync=1 = FULL sync mode untuk memastikan data tersimpan dengan benar
	// _locking_mode=EXCLUSIVE = exclusive locking untuk mencegah concurrent access issues
	dbConnectionString := fmt.Sprintf("file:%s?_foreign_keys=on&mode=rwc&_journal_mode=DELETE&cache=shared&_busy_timeout=10000&_sync=1&_locking_mode=EXCLUSIVE", dbPath)
	utils.GetLogger().Info("[Pairing] Database connection string: %s (mode=rwc, writable, exclusive locking)", dbPath)

	container, err := sqlstore.New(context.Background(), "sqlite3", dbConnectionString, dbLog)
	if err != nil {
		return fmt.Errorf("gagal membuat SQL store: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("gagal mendapatkan device store: %v", err)
	}

	// CRITICAL FIX: Pastikan database file memiliki permission write setelah GetFirstDevice()
	// GetFirstDevice() mungkin membuat database baru, jadi pastikan permission write
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Chmod(dbPath, 0644); err != nil {
			utils.GetLogger().Warn("[Pairing] Gagal chmod database file setelah GetFirstDevice: %v (continuing anyway)", err)
		} else {
			utils.GetLogger().Info("[Pairing] Database file permission set: writable, path=%s", dbPath)
		}
	}

	// Create WhatsApp client baru untuk pairing
	baseLog := waLog.Stdout("Client", "ERROR", true)
	clientLog := &utils.FilteredLogger{Logger: baseLog}
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// CRITICAL FIX: Register event handler untuk PairSuccess dan Connected
	// Ini memastikan permission database di-set tepat sebelum WhatsMeow mencoba save device store
	client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.PairSuccess:
			utils.GetLogger().Info("[Pairing] PairSuccess event received: TelegramID=%d", chatID)

			// CRITICAL FIX: Set permission database file SEBELUM WhatsMeow mencoba save device store
			// Ini mencegah error "readonly database" saat WhatsMeow mencoba save setelah PairSuccess
			if dbPath != "" {
				dbDir := filepath.Dir(dbPath)
				// Pastikan folder memiliki permission write
				if err := os.Chmod(dbDir, 0755); err != nil {
					utils.GetLogger().Warn("[Pairing] Gagal chmod folder database di PairSuccess: %v (continuing anyway)", err)
				}
				// Pastikan database file memiliki permission write
				if err := os.Chmod(dbPath, 0644); err != nil {
					utils.GetLogger().Warn("[Pairing] Gagal chmod database file di PairSuccess: %v (continuing anyway)", err)
				} else {
					utils.GetLogger().Info("[Pairing] Database file permission set di PairSuccess event: writable, path=%s", dbPath)
				}
			}
		case *events.Connected:
			utils.GetLogger().Info("[Pairing] Connected event received: TelegramID=%d", chatID)

			// CRITICAL FIX: Set permission database file juga saat Connected event
			// Connected event berarti pairing berhasil dan client sudah fully connected
			if dbPath != "" {
				dbDir := filepath.Dir(dbPath)
				// Pastikan folder memiliki permission write
				if err := os.Chmod(dbDir, 0755); err != nil {
					utils.GetLogger().Warn("[Pairing] Gagal chmod folder database di Connected: %v (continuing anyway)", err)
				}
				// Pastikan database file memiliki permission write
				if err := os.Chmod(dbPath, 0644); err != nil {
					utils.GetLogger().Warn("[Pairing] Gagal chmod database file di Connected: %v (continuing anyway)", err)
				} else {
					utils.GetLogger().Info("[Pairing] Database file permission set di Connected event: writable, path=%s", dbPath)
				}
			}
		}
	})

	// JANGAN set sebagai global client karena ini client khusus untuk pairing user ini
	// Global client akan di-update setelah pairing berhasil

	// Connect to WhatsApp
	utils.GetLogger().Info("Connecting new client to WhatsApp for pairing: TelegramID=%d, Phone=%s", chatID, cleanPhone)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("gagal connect: %v", err)
	}

	// Wait for connection
	timeout := 15 * time.Second
	checkInterval := 500 * time.Millisecond
	elapsed := time.Duration(0)

	for !client.IsConnected() && elapsed < timeout {
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	if !client.IsConnected() {
		return fmt.Errorf("connection timeout: client tidak terhubung setelah %v", timeout)
	}

	// Additional wait untuk memastikan connection fully established
	time.Sleep(1 * time.Second)

	utils.GetLogger().Info("New WhatsApp client created and connected successfully for pairing: TelegramID=%d, Phone=%s", chatID, cleanPhone)

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
	// Jika client baru saja dibuat, tidak perlu panggil ensureConnection() karena sudah connected
	// ensureConnection() hanya untuk reconnect jika terputus
	if !client.IsConnected() {
		if err := ensureConnection(); err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **ERROR KONEKSI**\n\n%s\n\nSilakan coba lagi dalam beberapa saat.", err))
			errorMsg.ParseMode = "Markdown"
			TgBot.Send(errorMsg)

			// Hapus pesan processing
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
			TgBot.Request(deleteMsg)

			return fmt.Errorf("gagal memastikan koneksi: %v", err)
		}
	}

	// Update pesan processing
	updateMsg := tgbotapi.NewEditMessageText(chatID, processingMsgSent.MessageID, "🔄 **MEMPERSIAPKAN PAIRING...**\n\n✅ Koneksi terhubung\n⏳ Menunggu koneksi fully established...")
	updateMsg.ParseMode = "Markdown"
	TgBot.Send(updateMsg)

	// Menunggu koneksi fully established sebelum generate pairing code
	// Sesuai dokumentasi whatsmeow: perlu wait untuk QR event atau sleep 1 detik setelah Connect
	// Kita akan wait sedikit untuk memastikan connection ready
	// CRITICAL FIX: Context timeout akan dibuat di dalam retry loop dengan timeout yang lebih lama

	// Tunggu beberapa detik untuk memastikan koneksi sudah fully established
	// Dokumentasi whatsmeow: "sleeping for a second after calling Connect will probably work too"
	time.Sleep(2 * time.Second)

	// Pastikan masih connected sebelum generate pairing code
	if !client.IsConnected() {
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

	// CRITICAL FIX: Generate pairing code dengan retry mechanism dan enhanced error handling
	// Berdasarkan issue WhatsMeow: https://github.com/tulir/whatsmeow/issues/984, #584, #877
	// Perlu handle stream error code 516, 401 (device_removed), dan prekey issues
	var pairingCode string
	var pairErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Pastikan masih connected sebelum retry
		if !client.IsConnected() {
			utils.GetLogger().Warn("[Pairing] Client tidak connected pada attempt %d, mencoba reconnect...", attempt)
			if reconnectErr := client.Connect(); reconnectErr != nil {
				pairErr = fmt.Errorf("gagal reconnect: %v", reconnectErr)
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt) * 2 * time.Second)
					continue
				}
				break
			}
			// Wait for reconnection
			time.Sleep(2 * time.Second)
		}

		// CRITICAL FIX: Gunakan context timeout yang lebih lama untuk pairing
		// Issue #984: Pairing timeout dengan versi WhatsApp terbaru
		ctxPairRetry, cancelPairRetry := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancelPairRetry() // FIXED: Use defer to ensure cancellation even on early return
		utils.GetLogger().Info("[Pairing] Memanggil PairPhone() untuk generate pairing code: Attempt=%d/%d", attempt, maxRetries)
		pairingCode, pairErr = client.PairPhone(ctxPairRetry, phone, true, whatsmeow.PairClientChrome, "Chrome (Windows)")

		if pairErr == nil && pairingCode != "" {
			utils.GetLogger().Info("[Pairing] PairPhone() berhasil: PairingCode=%s, Attempt=%d/%d", pairingCode, attempt, maxRetries)
			break // Success
		}

		// Log error dan handle berdasarkan jenis error
		if pairErr != nil {
			errorText := pairErr.Error()
			utils.GetLogger().Warn("[Pairing] Attempt %d/%d gagal: %v", attempt, maxRetries, pairErr)

			// CRITICAL FIX: Handle rate limit error (429: rate-overlimit) dengan delay yang lebih lama
			// Rate limit dari WhatsApp server memerlukan delay yang lebih lama (30-60 detik)
			if strings.Contains(errorText, "429") || strings.Contains(errorText, "rate-overlimit") || strings.Contains(errorText, "rate limit") {
				if attempt < maxRetries {
					// Rate limit memerlukan delay yang lebih lama
					// Exponential backoff: 30s, 60s, 120s
					rateLimitDelay := time.Duration(attempt) * 30 * time.Second
					utils.GetLogger().Warn("[Pairing] Rate limit terdeteksi (429), menunggu %v sebelum retry...", rateLimitDelay)

					// Update pesan ke user tentang rate limit
					updateMsg := tgbotapi.NewEditMessageText(chatID, processingMsgSent.MessageID,
						fmt.Sprintf("⏳ **RATE LIMIT TERDETEKSI**\n\nWhatsApp server membatasi request terlalu cepat.\n\nMenunggu %d detik sebelum mencoba lagi...\n\nAttempt: %d/%d",
							int(rateLimitDelay.Seconds()), attempt+1, maxRetries))
					updateMsg.ParseMode = "Markdown"
					TgBot.Send(updateMsg)

					time.Sleep(rateLimitDelay)
					continue
				} else {
					// Max retries reached untuk rate limit
					utils.GetLogger().Error("[Pairing] Rate limit setelah %d attempts, tidak bisa retry lagi", maxRetries)
					break
				}
			}

			// Handle error lainnya (websocket, timeout, EOF) dengan delay normal
			if attempt < maxRetries && (strings.Contains(errorText, "websocket") ||
				strings.Contains(errorText, "disconnected") ||
				strings.Contains(errorText, "timeout") ||
				strings.Contains(errorText, "EOF") ||
				strings.Contains(errorText, "516") || // Stream error code 516 (Issue #584)
				strings.Contains(errorText, "device_removed")) { // Issue #984
				// Retry dengan delay exponential backoff
				delay := time.Duration(attempt) * 2 * time.Second
				utils.GetLogger().Info("[Pairing] Menunggu %v sebelum retry berikutnya...", delay)
				time.Sleep(delay)
				continue
			}
		}
	}

	// Set error untuk handling di bawah
	if pairErr != nil {
		// Cek apakah error terkait database read-only
		errorText := pairErr.Error()
		var errorDetail string
		if strings.Contains(errorText, "429") || strings.Contains(errorText, "rate-overlimit") || strings.Contains(errorText, "rate limit") {
			errorDetail = "**Masalah:** Rate Limit dari WhatsApp Server (429)\n\n**Penyebab:**\n• Terlalu banyak request pairing dalam waktu singkat\n• WhatsApp server membatasi request untuk keamanan\n\n**Solusi:**\n1. ⏰ **Tunggu 5-10 menit** sebelum mencoba lagi\n2. Jangan melakukan pairing berulang kali dalam waktu singkat\n3. Pastikan tidak ada proses pairing lain yang berjalan\n4. Coba lagi setelah beberapa menit\n\n**Catatan:** Bot sudah mencoba 3x dengan delay, tapi masih kena rate limit. Silakan tunggu beberapa menit sebelum mencoba lagi."
		} else if strings.Contains(errorText, "readonly database") || strings.Contains(errorText, "read-only") {
			errorDetail = "**Masalah:** Database tidak bisa ditulis\n\n**Solusi:**\n1. Pastikan file whatsapp.db memiliki permission write\n2. Pastikan tidak ada proses lain yang menggunakan database\n3. Coba restart program\n4. Jika masih error, hapus file whatsapp.db dan coba pairing ulang"
		} else if strings.Contains(errorText, "516") || strings.Contains(errorText, "stream:error code=\"516\"") {
			// Issue #584: Stream error code 516 setelah scan QR
			errorDetail = "**Masalah:** Stream Error Code 516\n\n**Penyebab:**\n• WhatsApp server menolak koneksi setelah scan QR\n• Mungkin terkait dengan versi WhatsApp atau device yang digunakan\n\n**Solusi:**\n1. Pastikan menggunakan versi WhatsApp terbaru\n2. Coba restart aplikasi WhatsApp di ponsel\n3. Pastikan koneksi internet stabil\n4. Coba pairing lagi setelah beberapa saat\n5. Jika masih error, coba dengan nomor WhatsApp lain"
		} else if strings.Contains(errorText, "401") || strings.Contains(errorText, "device_removed") {
			// Issue #984: Device removed error dengan WhatsApp 2.25.32.70
			errorDetail = "**Masalah:** Device Removed (Error 401)\n\n**Penyebab:**\n• WhatsApp mendeteksi device sudah dihapus\n• Mungkin terkait dengan versi WhatsApp yang digunakan\n\n**Solusi:**\n1. Pastikan menggunakan versi WhatsApp yang kompatibel\n2. Hapus semua device yang terhubung dari WhatsApp Settings\n3. Restart aplikasi WhatsApp di ponsel\n4. Coba pairing lagi\n5. Jika masih error, coba dengan versi WhatsApp yang berbeda"
		} else if strings.Contains(errorText, "timeout") || strings.Contains(errorText, "context deadline exceeded") {
			errorDetail = "**Masalah:** Timeout saat generate pairing code setelah 3x retry\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Pastikan server WhatsApp dapat diakses\n3. Coba lagi dalam beberapa saat\n4. Restart program jika perlu"
		} else if strings.Contains(errorText, "not connected") || strings.Contains(errorText, "connection") {
			errorDetail = "**Masalah:** Koneksi terputus setelah 3x retry\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Coba restart program\n3. Coba pairing lagi"
		} else if strings.Contains(errorText, "websocket") || strings.Contains(errorText, "disconnected") || strings.Contains(errorText, "EOF") {
			errorDetail = "**Masalah:** Koneksi websocket terputus setelah 3x retry\n\n**Solusi:**\n1. Pastikan koneksi internet stabil\n2. Cek firewall atau proxy\n3. Coba lagi dalam beberapa saat\n4. Restart program jika perlu\n\n**Catatan:** Bot sudah mencoba reconnect otomatis 3x tapi masih gagal."
		} else {
			errorDetail = fmt.Sprintf("**Kemungkinan penyebab:**\n• Koneksi internet bermasalah\n• Server WhatsApp sedang sibuk\n• Nomor tidak valid\n• Error: %s\n\n**Solusi:**\nSilakan coba lagi dalam beberapa saat.", errorText)
		}

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ **ERROR**\n\nGagal generate pairing code: %v\n\n%s", pairErr, errorDetail))
		errorMsg.ParseMode = "Markdown"
		TgBot.Send(errorMsg)

		// Hapus pesan processing
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		TgBot.Request(deleteMsg)

		return fmt.Errorf("gagal generate pairing code: %v", pairErr)
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
	ticker := time.NewTicker(5 * time.Second) // Update setiap 5 detik untuk countdown lebih smooth
	defer ticker.Stop()

	var progressMessageID int

	for time.Since(startTime) < 120*time.Second {
		<-ticker.C

		// Update progress
		elapsed := int(time.Since(startTime).Seconds())
		remaining := 120 - elapsed
		progressBar := getProgressBar(elapsed, 120)
		percentage := int((float64(elapsed) / 120.0) * 100)
		minutes := remaining / 60
		seconds := remaining % 60

		// Cek status koneksi
		var statusEmoji string
		if client.IsConnected() {
			statusEmoji = "🟢"
		} else {
			statusEmoji = "🔴"
		}

		progressText := fmt.Sprintf(`⏳ **MENUNGGU PAIRING...**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s %s (%d%%)

%s **Countdown:** %02d:%02d
📱 **Status:** Menunggu konfirmasi...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Pastikan kode sudah dimasukkan di WhatsApp
• Jangan tutup aplikasi WhatsApp
• Koneksi internet harus stabil
• Bot akan otomatis terdeteksi setelah konfirmasi`, progressBar, getProgressEmoji(percentage), percentage, statusEmoji, minutes, seconds)

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

		// CRITICAL FIX: Set permission database file secara berkala selama menunggu pairing
		// Ini memastikan permission tetap benar saat WhatsMeow mencoba save device store
		if dbPath != "" {
			dbDir := filepath.Dir(dbPath)
			if err := os.Chmod(dbDir, 0755); err != nil {
				utils.GetLogger().Warn("[Pairing] Gagal chmod folder database selama wait: %v (continuing anyway)", err)
			}
			if err := os.Chmod(dbPath, 0644); err != nil {
				utils.GetLogger().Warn("[Pairing] Gagal chmod database file selama wait: %v (continuing anyway)", err)
			}
		}

		if client != nil && client.Store != nil && client.Store.ID != nil {
			// CRITICAL FIX: Set permission database file SEBELUM melanjutkan
			// Ini memastikan permission benar saat kita akan menggunakan database
			if dbPath != "" {
				dbDir := filepath.Dir(dbPath)
				if err := os.Chmod(dbDir, 0755); err != nil {
					utils.GetLogger().Warn("[Pairing] Gagal chmod folder database sebelum save: %v (continuing anyway)", err)
				}
				if err := os.Chmod(dbPath, 0644); err != nil {
					utils.GetLogger().Warn("[Pairing] Gagal chmod database file sebelum save: %v (continuing anyway)", err)
				} else {
					utils.GetLogger().Info("[Pairing] Database file permission verified sebelum save: writable, path=%s", dbPath)
				}
			}

			// Hapus pesan progress
			if progressMessageID > 0 {
				deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMessageID)
				TgBot.Request(deleteMsg)
			}

			// Rename database setelah pairing berhasil
			// Format: DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db
			whatsappNumber := client.Store.ID.User
			newWhatsAppDB := utils.GenerateDBName(chatID, whatsappNumber, "whatsmeow")
			newBotDataDB := utils.GenerateDBName(chatID, whatsappNumber, "bot_data")

			// Set config database
			utils.SetDBConfig(chatID, whatsappNumber)

			// Daftarkan akun ke sistem multi-account jika belum ada
			am := GetAccountManager()
			am.SetTelegramBot(TgBot)

			// Cek apakah akun sudah terdaftar
			allAccounts := am.GetAllAccounts()
			accountExists := false
			for _, acc := range allAccounts {
				if acc.PhoneNumber == whatsappNumber {
					accountExists = true
					break
				}
			}

			// Jika belum terdaftar, daftarkan sekarang
			if !accountExists {
				// Pastikan tabel whatsapp_accounts ada sebelum AddAccount
				if err := InitAccountDB(); err != nil {
					utils.GetLogger().Warn("Failed to init account DB before adding account: %v", err)
					// Continue anyway, mungkin sudah ada
				}

				// ✅ AMAN: Pass chatID (TelegramID) untuk validasi ownership
				account, err := am.AddAccount(whatsappNumber, newWhatsAppDB, newBotDataDB, chatID)
				if err == nil {
					// Simpan client ke account manager
					am.mutex.Lock()
					am.clients[account.ID] = client
					am.mutex.Unlock()

					// Set sebagai current jika belum ada current
					if am.GetCurrentAccount() == nil {
						am.SetCurrentAccount(account.ID)
					}

					utils.GetLogger().Info("Account registered to multi-account system: %s (ID: %d)", whatsappNumber, account.ID)
				} else {
					utils.GetLogger().Warn("Failed to register account to multi-account system: %v", err)
				}
			}

			// CRITICAL FIX: Database sudah dibuat dengan path yang benar dari awal
			// Tidak perlu rename karena database sudah di folder user dengan nama yang benar
			utils.GetLogger().Info("Database sudah dibuat dengan path yang benar: WhatsAppDB=%s, BotDataDB=%s", newWhatsAppDB, newBotDataDB)

			// Removed auto-fetch groups - user can manually fetch when needed via menu
			// This prevents database access issues during account switching

			// Kirim pesan sukses
			successMsg := ui.FormatPairingSuccess()
			msg := tgbotapi.NewMessage(chatID, successMsg)
			msg.ParseMode = "Markdown"
			TgBot.Send(msg)

			// Refresh menu utama setelah pairing berhasil
			time.Sleep(1 * time.Second)
			// Gunakan client yang baru saja di-pair
			ui.ShowMainMenu(TgBot, chatID, client)

			return nil
		}
	}

	// Timeout - hapus pesan progress dan kirim error
	if progressMessageID > 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMessageID)
		TgBot.Request(deleteMsg)
	}

	timeoutMsg := tgbotapi.NewMessage(chatID, "❌ **PAIRING TIMEOUT**\n\nWaktu pairing habis (2 menit).\n\nSilakan coba lagi dengan command `/pair <nomor>`.")
	timeoutMsg.ParseMode = "Markdown"
	TgBot.Send(timeoutMsg)

	return fmt.Errorf("pairing timeout")
}

// getProgressBar membuat progress bar visual
func getProgressBar(current, total int) string {
	if total == 0 {
		return "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	}

	filled := int((float64(current) / float64(total)) * 30)
	empty := 30 - filled

	if filled > 30 {
		filled = 30
		empty = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("━", empty)
	return bar
}

// getProgressEmoji mendapatkan emoji berdasarkan persentase
func getProgressEmoji(percentage int) string {
	if percentage < 25 {
		return "🔴"
	} else if percentage < 50 {
		return "🟠"
	} else if percentage < 75 {
		return "🟡"
	} else {
		return "🟢"
	}
}
