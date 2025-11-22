package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // Register GIF decoder
	"image/jpeg"
	_ "image/png" // Register PNG decoder
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // Register WEBP decoder
)

// resizeImage resizes image to square format (size x size)
func resizeImage(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If already the right size, return as is
	if width == size && height == size {
		return img
	}

	// Calculate new dimensions to maintain aspect ratio
	var newWidth, newHeight int
	if width > height {
		// Landscape: fit height
		newHeight = size
		newWidth = (width * size) / height
	} else {
		// Portrait or square: fit width
		newWidth = size
		newHeight = (height * size) / width
	}

	// Create resized image
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	xdraw.NearestNeighbor.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// Crop to square (center crop)
	var cropX, cropY int
	if newWidth > newHeight {
		// Crop width (landscape)
		cropX = (newWidth - size) / 2
		cropY = 0
	} else {
		// Crop height (portrait)
		cropX = 0
		cropY = (newHeight - size) / 2
	}

	// Create final cropped square image
	cropped := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(cropped, cropped.Bounds(), resized, image.Point{cropX, cropY}, draw.Src)

	return cropped
}

// parseJIDFromString parses a string to types.JID
func parseJIDFromString(jidStr string) (types.JID, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return types.JID{}, fmt.Errorf("invalid JID: %v", err)
	}
	return jid, nil
}

// GroupPhotoState manages the state for changing group photos
type GroupPhotoState struct {
	WaitingForGroupName bool
	WaitingForDelay     bool
	WaitingForPhoto     bool
	SelectedGroups      []GroupLinkInfo
	Keyword             string
	DelaySeconds        int
	PhotoPath           string
}

var groupPhotoStates = make(map[int64]*GroupPhotoState)

// ShowChangePhotoMenu menampilkan menu ganti foto profil grup
func ShowChangePhotoMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `🖼️ **GANTI FOTO PROFIL GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengganti foto profil grup WhatsApp yang Anda pilih.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ubah Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk ganti foto
• Delay membantu menghindari rate limit WhatsApp
• Foto akan sama untuk semua grup yang dipilih
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_photo"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_photo"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Ubah Semua", "change_all_photos"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "photo_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// ShowChangePhotoMenuEdit menampilkan menu ganti foto dengan EDIT message (no spam!)
func ShowChangePhotoMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `🖼️ **GANTI FOTO PROFIL GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengganti foto profil grup WhatsApp yang Anda pilih.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ubah Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk ganti foto
• Delay membantu menghindari rate limit WhatsApp
• Foto akan sama untuk semua grup yang dipilih
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_photo"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_photo"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Ubah Semua", "change_all_photos"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "photo_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowPhotoExampleEdit menampilkan contoh penggunaan dengan EDIT message
func ShowPhotoExampleEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	exampleMsg := `📖 **CONTOH PENGGUNAAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Metode 1: Lihat & Pilih**
1️⃣ Klik "📋 Lihat & Pilih"
2️⃣ Bot tampilkan daftar grup
3️⃣ Ketik nomor grup (misal: 1,3,5)
4️⃣ Tentukan delay (misal: 2 detik)
5️⃣ Kirim foto yang ingin digunakan
6️⃣ Selesai! Foto diganti

**🔍 Metode 2: Cari Manual**
1️⃣ Klik "🔍 Cari Manual"
2️⃣ Ketik kata kunci (misal: "Keluarga")
3️⃣ Bot tampilkan hasil pencarian
4️⃣ Tentukan delay
5️⃣ Kirim foto
6️⃣ Selesai! Foto diganti

**⚡ Metode 3: Ubah Semua**
1️⃣ Klik "⚡ Ubah Semua"
2️⃣ Konfirmasi total grup
3️⃣ Tentukan delay (rekomendasi: 3-5 detik)
4️⃣ Kirim foto
5️⃣ Bot proses semua grup
6️⃣ Hasil dikirim per batch

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips Delay:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

🖼️ **Tips Foto:**
• Format: JPG, PNG
• Ukuran maks: 5MB
• Resolusi: 640x640 atau lebih
• Foto harus jelas dan berkualitas

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_photo_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exampleMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartChangePhotoProcess memulai proses ganti foto
func StartChangePhotoProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	groupPhotoStates[chatID] = &GroupPhotoState{
		WaitingForGroupName: true,
		WaitingForDelay:     false,
		WaitingForPhoto:     false,
		SelectedGroups:      []GroupLinkInfo{},
		Keyword:             "",
		DelaySeconds:        0,
		PhotoPath:           "",
	}

	promptMsg := `🔍 **MASUKKAN NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

Ketik nama grup atau kata kunci untuk mencari grup yang ingin diganti fotonya.

**Contoh Input:**
• "Keluarga" - Cari grup dengan kata keluarga
• "Kerja" - Cari grup dengan kata kerja
• "." - Ubah SEMUA grup (hati-hati!)

**Multi-line Input (Exact Match):**
GROUP ANGKATAN 1
GROUP ANGKATAN 2
GROUP ANGKATAN 3

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan kata kunci spesifik untuk hasil akurat
• Multi-line untuk exact match nama grup
• Gunakan "." jika ingin ubah semua grup
• Pencarian tidak case-sensitive

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_photo"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForPhoto memproses input nama grup
func HandleGroupNameInputForPhoto(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupPhotoStates[chatID]
	if state == nil || !state.WaitingForGroupName {
		return
	}

	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nama grup tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	state.Keyword = keyword

	loadingMsg := tgbotapi.NewMessage(chatID, "🔍 Mencari grup...")
	loadingMsgSent, _ := telegramBot.Send(loadingMsg)

	var groups map[string]string
	var err error

	// Smart search logic (same as link feature)
	if keyword == "." {
		groups, err = utils.GetAllGroupsFromDB()
	} else {
		lines := strings.Split(keyword, "\n")
		if len(lines) > 1 {
			// Multi-line: exact match
			groups, err = utils.SearchGroupsExactMultiple(lines)
		} else if len(keyword) > 30 {
			// Long single-line: try exact first
			groups, err = utils.SearchGroupsExact(keyword)
			if err == nil && len(groups) == 0 {
				groups, err = utils.SearchGroupsFlexible(keyword)
			}
		} else {
			// Short keyword: flexible search
			groups, err = utils.SearchGroupsFlexible(keyword)
		}
	}

	// Delete loading message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
	telegramBot.Request(deleteMsg)

	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		state.WaitingForGroupName = false
		return
	}

	if len(groups) == 0 {
		noResultMsg := fmt.Sprintf(`❌ **TIDAK DITEMUKAN**

Tidak ada grup yang cocok dengan kata kunci: **"%s"**

**Saran:**
• Coba kata kunci yang berbeda
• Periksa ejaan kata kunci
• Gunakan kata kunci yang lebih umum

Silakan coba lagi atau klik tombol di bawah.`, keyword)

		msg := tgbotapi.NewMessage(chatID, noResultMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Lagi", "start_change_photo"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_photo_menu"),
			),
		)
		msg.ReplyMarkup = keyboard
		telegramBot.Send(msg)

		state.WaitingForGroupName = false
		return
	}

	// Store selected groups with natural sorting
	state.SelectedGroups = []GroupLinkInfo{}
	sortedGroups := utils.SortGroupsNaturally(groups)
	for _, group := range sortedGroups {
		state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
			JID:  group.JID,
			Name: group.Name,
		})
	}

	// Show found groups and ask for delay
	state.WaitingForGroupName = false
	state.WaitingForDelay = true

	resultMsg := fmt.Sprintf(`✅ **GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total:** %d grup

**Daftar grup yang akan diubah:**

`, len(state.SelectedGroups))

	for i, group := range state.SelectedGroups {
		resultMsg += fmt.Sprintf("%d. %s\n", i+1, group.Name)
		if i >= 9 && len(state.SelectedGroups) > 10 {
			resultMsg += fmt.Sprintf("... dan %d grup lainnya\n", len(state.SelectedGroups)-10)
			break
		}
	}

	resultMsg += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **LANGKAH SELANJUTNYA**

Ketik delay (dalam detik) untuk setiap grup yang berhasil diubah.

**Contoh:**
• Ketik: **2** (delay 2 detik per grup)
• Ketik: **5** (delay 5 detik per grup)

💡 **Rekomendasi:**
• < 10 grup: 1-2 detik
• 10-30 grup: 2-3 detik
• > 30 grup: 3-5 detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input delay...`

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_photo"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInputForPhoto memproses input delay
func HandleDelayInputForPhoto(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupPhotoStates[chatID]
	if state == nil || !state.WaitingForDelay {
		return
	}

	input = strings.TrimSpace(input)

	// Parse delay
	var delay int
	_, err := fmt.Sscanf(input, "%d", &delay)
	if err != nil || delay < 0 || delay > 60 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Input tidak valid!\n\nDelay harus berupa angka antara 0-60 detik.\n\nContoh: 2, 5, 10")
		telegramBot.Send(errorMsg)
		return
	}

	state.DelaySeconds = delay
	state.WaitingForDelay = false
	state.WaitingForPhoto = true

	// Ask for photo
	photoMsg := fmt.Sprintf(`📸 **KIRIM FOTO**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Grup dipilih:** %d grup
⏱️ **Delay:** %d detik per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🖼️ **LANGKAH TERAKHIR**

Kirim foto yang ingin Anda gunakan sebagai foto profil grup.

**Persyaratan Foto:**
• Format: JPG, PNG, WEBP
• Ukuran maks: 5MB
• Resolusi: 640x640 atau lebih tinggi
• Foto harus jelas dan berkualitas

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan foto dengan resolusi tinggi
• Pastikan foto tidak buram
• Foto akan di-crop otomatis jadi persegi
• Gunakan foto yang represent grup Anda

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu foto dari Anda...`, len(state.SelectedGroups), delay)

	msg := tgbotapi.NewMessage(chatID, photoMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_photo"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandlePhotoUpload memproses foto yang dikirim user
func HandlePhotoUpload(photo *tgbotapi.PhotoSize, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := groupPhotoStates[chatID]
	if state == nil || !state.WaitingForPhoto {
		return
	}

	processingMsg := tgbotapi.NewMessage(chatID, "⏳ Mengunduh foto...")
	processingMsgSent, _ := telegramBot.Send(processingMsg)

	// Get file
	fileConfig := tgbotapi.FileConfig{FileID: photo.FileID}
	file, err := telegramBot.GetFile(fileConfig)
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error mengunduh foto: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// Download file
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", telegramBot.Token, file.FilePath)
	resp, err := http.Get(fileURL)
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error download: %v", err))
		telegramBot.Send(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Read image data
	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error membaca foto: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// Decode image (supports JPG, PNG, GIF, WEBP)
	img, format, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Foto tidak valid: %v\n\nFormat terdeteksi: %s\n\nGunakan foto JPG/PNG yang valid.", err, format))
		telegramBot.Send(errorMsg)
		return
	}

	// Resize to 640x640 (WhatsApp requirement)
	resizedImg := resizeImage(img, 640)

	// Re-encode to JPEG with quality 90 (balance between quality and compatibility)
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 90})
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error convert foto: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// Save JPEG to temp file
	tempFile, err := os.CreateTemp("", "group_photo_*.jpg")
	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error simpan foto: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	_, err = tempFile.Write(buf.Bytes())
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error simpan foto: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	tempFile.Close()

	state.PhotoPath = tempFile.Name()
	state.WaitingForPhoto = false

	// Log success
	logger := utils.GetLogger()
	logger.Info("Foto berhasil diproses: format=%s, size=%d bytes (resized to 640x640), path=%s", format, buf.Len(), tempFile.Name())

	// Delete processing message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, processingMsgSent.MessageID)
	telegramBot.Request(deleteMsg)

	// Start processing
	startMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Foto diterima!\n\n🚀 Memulai proses ganti foto untuk %d grup...", len(state.SelectedGroups)))
	telegramBot.Send(startMsg)

	// Process in goroutine
	go ProcessChangePhotos(state.SelectedGroups, state.DelaySeconds, state.PhotoPath, chatID, client, telegramBot)

	// Clear state
	delete(groupPhotoStates, chatID)
}

// ProcessChangePhotos memproses penggantian foto grup
func ProcessChangePhotos(groups []GroupLinkInfo, delay int, photoPath string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	defer os.Remove(photoPath) // Cleanup temp file

	totalGroups := len(groups)
	successCount := 0
	failedCount := 0
	var failedGroups []string

	var progressMsgSent *tgbotapi.Message

	for i, group := range groups {
		// HIGH FIX: Ambil active client di setiap iterasi (upload foto = operasi SANGAT lambat!)
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessChangePhotos", i, totalGroups)
		if shouldStop {
			// Client disconnect - stop proses
			disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada grup %d/%d\n\n✅ Berhasil: %d\n❌ Gagal: %d", i+1, totalGroups, successCount, failedCount)
			notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
			notifMsg.ParseMode = "Markdown"
			telegramBot.Send(notifMsg)
			break
		}

		// Read photo file
		photoBytes, err := os.ReadFile(photoPath)
		if err != nil {
			failedCount++
			failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (error baca foto)", group.Name))
			continue
		}

		// Parse JID
		jid, err := parseJIDFromString(group.JID)
		if err != nil {
			failedCount++
			failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (invalid JID)", group.Name))
			continue
		}

		// Set group photo dengan validClient
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err = validClient.SetGroupPhoto(ctx, jid, photoBytes)
		cancel()

		if err != nil {
			failedCount++
			failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (%v)", group.Name, err))
		} else {
			successCount++
		}

		// Show progress if more than 3 groups
		if totalGroups > 3 {
			progressPercent := ((i + 1) * 100) / totalGroups
			progressBar := generateProgressBar(progressPercent)

			progressMsg := fmt.Sprintf(`⏳ **PROGRESS**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s **%d%%**
📊 **Diproses:** %d/%d grup
✅ **Berhasil:** %d
❌ **Gagal:** %d
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⏳ Sedang memproses...`, progressBar, progressPercent, i+1, totalGroups, successCount, failedCount)

			if progressMsgSent == nil {
				updateMsg := tgbotapi.NewMessage(chatID, progressMsg)
				updateMsg.ParseMode = "Markdown"
				sent, _ := telegramBot.Send(updateMsg)
				progressMsgSent = &sent
			} else {
				editMsg := tgbotapi.NewEditMessageText(chatID, progressMsgSent.MessageID, progressMsg)
				editMsg.ParseMode = "Markdown"
				telegramBot.Send(editMsg)
			}
		}

		// Delay
		if delay > 0 && i < len(groups)-1 {
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	// Delete progress message
	if progressMsgSent != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMsgSent.MessageID)
		telegramBot.Request(deleteMsg)
	}

	// Send final results
	resultMsg := fmt.Sprintf(`🎉 **SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **RINGKASAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Berhasil:** %d grup
❌ **Gagal:** %d grup
⏱️ **Delay:** %d detik/grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, successCount, failedCount, delay)

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Send failed groups if any (batching)
	if len(failedGroups) > 0 {
		batchSize := 10
		for i := 0; i < len(failedGroups); i += batchSize {
			end := i + batchSize
			if end > len(failedGroups) {
				end = len(failedGroups)
			}

			batch := failedGroups[i:end]
			failedMsg := fmt.Sprintf("**Grup yang Gagal (Batch %d):**\n\n%s", (i/batchSize)+1, strings.Join(batch, "\n"))

			msg := tgbotapi.NewMessage(chatID, failedMsg)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)

			if end < len(failedGroups) {
				time.Sleep(1 * time.Second)
			}
		}
	}

	// Send completion keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🖼️ Ganti Lagi", "change_photo_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
		),
	)

	completionMsg := tgbotapi.NewMessage(chatID, "💡 Apa yang ingin Anda lakukan selanjutnya?")
	completionMsg.ReplyMarkup = keyboard
	telegramBot.Send(completionMsg)
}

// CancelChangePhoto membatalkan proses ganti foto
func CancelChangePhoto(chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupPhotoStates[chatID]
	if state != nil {
		// Cleanup temp file if exists
		if state.PhotoPath != "" {
			os.Remove(state.PhotoPath)
		}
		delete(groupPhotoStates, chatID)
	}

	msg := tgbotapi.NewMessage(chatID, "❌ Proses ganti foto profil grup dibatalkan.")
	telegramBot.Send(msg)
}

// IsWaitingForPhotoInput checks if user is waiting to input photo-related data
func IsWaitingForPhotoInput(chatID int64) bool {
	state := groupPhotoStates[chatID]
	return state != nil && (state.WaitingForGroupName || state.WaitingForDelay || state.WaitingForPhoto)
}

// GetPhotoInputType returns the current input type
func GetPhotoInputType(chatID int64) string {
	state := groupPhotoStates[chatID]
	if state == nil {
		return ""
	}

	if state.WaitingForGroupName {
		return "group_name"
	}
	if state.WaitingForDelay {
		return "delay"
	}
	if state.WaitingForPhoto {
		return "photo"
	}

	return ""
}

// HandleFileInputForChangePhoto - Handle file .txt untuk ganti foto grup
func HandleFileInputForChangePhoto(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := groupPhotoStates[chatID]
	if state == nil || !state.WaitingForGroupName {
		return
	}

	// Download file
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileID)
	resp, err := http.Get(fileURL)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorConnection, err, "Gagal mengunduh file")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_photo", "Gagal download file", chatID, err)
		return
	}
	defer resp.Body.Close()

	var fileResp struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil || !fileResp.OK {
		errorMsg := utils.FormatUserError(utils.ErrorValidation, fmt.Errorf("file tidak valid"), "Gagal mengambil informasi file")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_photo", "File tidak valid", chatID, err)
		return
	}

	// Check if file is .txt
	if !strings.HasSuffix(strings.ToLower(fileResp.Result.FilePath), ".txt") {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **FILE TIDAK VALID**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nFile harus berupa format `.txt`.\n\nSilakan kirim file dengan format `.txt` yang berisi barisan nama grup.")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Download file content
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	fileResp2, err := http.Get(downloadURL)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorConnection, err, "Gagal membaca file")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_photo", "Gagal membaca file", chatID, err)
		return
	}
	defer fileResp2.Body.Close()

	// Read file content - extract group names (one per line)
	var groupNames []string
	scanner := bufio.NewScanner(fileResp2.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			groupNames = append(groupNames, line)
		}
	}

	if err := scanner.Err(); err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Error membaca file")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_photo", "Error membaca file", chatID, err)
		return
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **FILE KOSONG**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nFile `.txt` yang Anda kirim tidak berisi nama grup.\n\n**Format yang benar:**\nSatu nama grup per baris.\n\n**Contoh:**\nKeluarga Besar\nGrup Kerja\nGrup Teman")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Log activity
	utils.LogActivity("change_photo_file", fmt.Sprintf("File .txt diterima dengan %d nama grup", len(groupNames)), chatID)

	// Search groups using exact match for each name
	groups, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_photo", "Gagal mencari grup dari file", chatID, err)
		return
	}

	if len(groups) == 0 {
		notFoundMsg := fmt.Sprintf(`❌ **GRUP TIDAK DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Tidak ada grup yang cocok dengan nama-nama di file.

📋 **Nama yang dicari:** %d grup

💡 **Tips:**
• Pastikan nama grup di file sama persis dengan nama di database
• Gunakan menu "📋 Lihat & Pilih" untuk melihat daftar grup yang tersedia`, len(groupNames))

		msg := tgbotapi.NewMessage(chatID, notFoundMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	// Store selected groups
	state.SelectedGroups = []GroupLinkInfo{}
	sortedGroups := utils.SortGroupsNaturally(groups)
	for _, group := range sortedGroups {
		state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
			JID:  group.JID,
			Name: group.Name,
		})
	}

	// Show found groups and ask for delay
	state.WaitingForGroupName = false
	state.WaitingForDelay = true

	resultMsg := fmt.Sprintf(`✅ **GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total:** %d grup (dari %d yang dicari)

**Daftar grup yang akan diubah:**

`, len(state.SelectedGroups), len(groupNames))

	for i, group := range state.SelectedGroups {
		resultMsg += fmt.Sprintf("%d. %s\n", i+1, group.Name)
		if i >= 9 && len(state.SelectedGroups) > 10 {
			resultMsg += fmt.Sprintf("... dan %d grup lainnya\n", len(state.SelectedGroups)-10)
			break
		}
	}

	resultMsg += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **LANGKAH SELANJUTNYA**

Ketik delay (dalam detik) untuk setiap grup yang berhasil diubah.

**Contoh:**
• Ketik: **2** (delay 2 detik per grup)
• Ketik: **5** (delay 5 detik per grup)

💡 **Rekomendasi:**
• < 10 grup: 1-2 detik
• 10-30 grup: 2-3 detik
• > 30 grup: 3-5 detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)
}
