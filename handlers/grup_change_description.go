package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// GroupDescriptionState manages the state for changing group descriptions
type GroupDescriptionState struct {
	WaitingForGroupName   bool
	WaitingForDelay       bool
	WaitingForDescription bool
	SelectedGroups        []GroupLinkInfo
	Keyword               string
	DelaySeconds          int
	Description           string
}

var groupDescriptionStates = make(map[int64]*GroupDescriptionState)

// ShowChangeDescriptionMenu menampilkan menu atur deskripsi grup
func ShowChangeDescriptionMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `📝 **ATUR DESKRIPSI GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengubah deskripsi grup WhatsApp yang Anda pilih.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ubah Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk ubah deskripsi
• Delay membantu menghindari rate limit WhatsApp
• Deskripsi akan sama untuk semua grup yang dipilih
• Maksimal 1024 karakter per deskripsi
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_description"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_description"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Ubah Semua", "change_all_descriptions"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "description_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// ShowChangeDescriptionMenuEdit menampilkan menu atur deskripsi dengan EDIT message (no spam!)
func ShowChangeDescriptionMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `📝 **ATUR DESKRIPSI GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengubah deskripsi grup WhatsApp yang Anda pilih.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ubah Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk ubah deskripsi
• Delay membantu menghindari rate limit WhatsApp
• Deskripsi akan sama untuk semua grup yang dipilih
• Maksimal 1024 karakter per deskripsi
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_description"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_description"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Ubah Semua", "change_all_descriptions"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "description_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowDescriptionExampleEdit menampilkan contoh penggunaan dengan EDIT message
func ShowDescriptionExampleEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	exampleMsg := `📖 **CONTOH PENGGUNAAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Metode 1: Lihat & Pilih**
1️⃣ Klik "📋 Lihat & Pilih"
2️⃣ Bot tampilkan daftar grup
3️⃣ Ketik nomor grup (misal: 1,3,5)
4️⃣ Tentukan delay (misal: 2 detik)
5️⃣ Ketik deskripsi yang diinginkan
6️⃣ Selesai! Deskripsi diganti

**🔍 Metode 2: Cari Manual**
1️⃣ Klik "🔍 Cari Manual"
2️⃣ Ketik kata kunci (misal: "Keluarga")
3️⃣ Bot tampilkan hasil pencarian
4️⃣ Tentukan delay
5️⃣ Ketik deskripsi
6️⃣ Selesai! Deskripsi diganti

**⚡ Metode 3: Ubah Semua**
1️⃣ Klik "⚡ Ubah Semua"
2️⃣ Konfirmasi total grup
3️⃣ Tentukan delay (rekomendasi: 3-5 detik)
4️⃣ Ketik deskripsi
5️⃣ Bot proses semua grup
6️⃣ Hasil dikirim per batch

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips Delay:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

📝 **Tips Deskripsi:**
• Maksimal 1024 karakter
• Bisa multi-line
• Emoji dan simbol didukung
• Deskripsi akan sama untuk semua grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_description_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exampleMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartChangeDescriptionProcess memulai proses ubah deskripsi
func StartChangeDescriptionProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	groupDescriptionStates[chatID] = &GroupDescriptionState{
		WaitingForGroupName:   true,
		WaitingForDelay:       false,
		WaitingForDescription: false,
		SelectedGroups:        []GroupLinkInfo{},
		Keyword:               "",
		DelaySeconds:          0,
		Description:           "",
	}

	promptMsg := `🔍 **MASUKKAN NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

Ketik nama grup atau kata kunci untuk mencari grup yang ingin diubah deskripsinya.

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
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_description"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForDescription memproses input nama grup
func HandleGroupNameInputForDescription(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupDescriptionStates[chatID]
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

	// Smart search logic (same as link & photo features)
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
				tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Lagi", "start_change_description"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_description_menu"),
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
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_description"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInputForDescription memproses input delay
func HandleDelayInputForDescription(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupDescriptionStates[chatID]
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
	state.WaitingForDescription = true

	// Ask for description
	descriptionMsg := fmt.Sprintf(`📝 **MASUKKAN DESKRIPSI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Grup dipilih:** %d grup
⏱️ **Delay:** %d detik per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📝 **LANGKAH TERAKHIR**

Ketik deskripsi yang ingin Anda gunakan untuk grup-grup tersebut.

**Persyaratan Deskripsi:**
• Maksimal 1024 karakter
• Bisa multi-line (enter untuk baris baru)
• Emoji dan simbol didukung
• Deskripsi akan sama untuk semua grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan deskripsi yang informatif
• Bisa tambahkan emoji untuk menarik
• Multi-line untuk format rapi
• Pastikan tidak lebih dari 1024 karakter

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Contoh Deskripsi:**
Grup resmi untuk anggota komunitas.
Silakan baca rules sebelum posting.

Rules:
1. Sopan dan saling menghormati
2. No spam dan promo
3. Enjoy!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu deskripsi dari Anda...`, len(state.SelectedGroups), delay)

	msg := tgbotapi.NewMessage(chatID, descriptionMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_description"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDescriptionInput memproses input deskripsi dari user
func HandleDescriptionInput(description string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := groupDescriptionStates[chatID]
	if state == nil || !state.WaitingForDescription {
		return
	}

	description = strings.TrimSpace(description)
	if description == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Deskripsi tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Validate length (1024 characters max)
	if len(description) > 1024 {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Deskripsi terlalu panjang!\n\nMaksimal 1024 karakter.\n\nPanjang saat ini: %d karakter\n\nSilakan perpendek deskripsi Anda.", len(description)))
		telegramBot.Send(errorMsg)
		return
	}

	state.Description = description
	state.WaitingForDescription = false

	// Start processing
	startMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Deskripsi diterima!\n\n📝 **Deskripsi:**\n%s\n\n📊 **Panjang:** %d karakter\n\n🚀 Memulai proses ubah deskripsi untuk %d grup...",
		description, len(description), len(state.SelectedGroups)))
	startMsg.ParseMode = "Markdown"
	telegramBot.Send(startMsg)

	// Process in goroutine
	go ProcessChangeDescriptions(state.SelectedGroups, state.DelaySeconds, state.Description, chatID, client, telegramBot)

	// Clear state
	delete(groupDescriptionStates, chatID)
}

// ProcessChangeDescriptions memproses pengubahan deskripsi grup
func ProcessChangeDescriptions(groups []GroupLinkInfo, delay int, description string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	totalGroups := len(groups)
	successCount := 0
	failedCount := 0
	var failedGroups []string

	var progressMsgSent *tgbotapi.Message

	for i, group := range groups {
		// HIGH FIX: Ambil active client di setiap iterasi untuk proses panjang!
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessChangeDescriptions", i, totalGroups)
		if shouldStop {
			// Client disconnect - stop proses
			disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada grup %d/%d\n\n✅ Berhasil: %d\n❌ Gagal: %d", i+1, totalGroups, successCount, failedCount)
			notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
			notifMsg.ParseMode = "Markdown"
			telegramBot.Send(notifMsg)
			break
		}

		// Parse JID
		jid, err := parseJIDFromString(group.JID)
		if err != nil {
			failedCount++
			failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (invalid JID)", group.Name))
			continue
		}

		// Set group description dengan validClient
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = validClient.SetGroupDescription(ctx, jid, description)
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
📝 **Deskripsi:** %d karakter

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, successCount, failedCount, delay, len(description))

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
			tgbotapi.NewInlineKeyboardButtonData("📝 Ubah Lagi", "change_description_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
		),
	)

	completionMsg := tgbotapi.NewMessage(chatID, "💡 Apa yang ingin Anda lakukan selanjutnya?")
	completionMsg.ReplyMarkup = keyboard
	telegramBot.Send(completionMsg)
}

// CancelChangeDescription membatalkan proses ubah deskripsi
func CancelChangeDescription(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(groupDescriptionStates, chatID)

	msg := tgbotapi.NewMessage(chatID, "❌ Proses ubah deskripsi grup dibatalkan.")
	telegramBot.Send(msg)
}

// IsWaitingForDescriptionInput checks if user is waiting to input description-related data
func IsWaitingForDescriptionInput(chatID int64) bool {
	state := groupDescriptionStates[chatID]
	return state != nil && (state.WaitingForGroupName || state.WaitingForDelay || state.WaitingForDescription)
}

// GetDescriptionInputType returns the current input type
func GetDescriptionInputType(chatID int64) string {
	state := groupDescriptionStates[chatID]
	if state == nil {
		return ""
	}

	if state.WaitingForGroupName {
		return "group_name"
	}
	if state.WaitingForDelay {
		return "delay"
	}
	if state.WaitingForDescription {
		return "description"
	}

	return ""
}

// HandleFileInputForChangeDescription - Handle file .txt untuk atur deskripsi grup
func HandleFileInputForChangeDescription(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := groupDescriptionStates[chatID]
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
		utils.LogActivityError("change_description", "Gagal download file", chatID, err)
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
		utils.LogActivityError("change_description", "File tidak valid", chatID, err)
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
		utils.LogActivityError("change_description", "Gagal membaca file", chatID, err)
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
		utils.LogActivityError("change_description", "Error membaca file", chatID, err)
		return
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **FILE KOSONG**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nFile `.txt` yang Anda kirim tidak berisi nama grup.\n\n**Format yang benar:**\nSatu nama grup per baris.\n\n**Contoh:**\nKeluarga Besar\nGrup Kerja\nGrup Teman")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Log activity
	utils.LogActivity("change_description_file", fmt.Sprintf("File .txt diterima dengan %d nama grup", len(groupNames)), chatID)

	// Search groups using exact match for each name
	groups, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_description", "Gagal mencari grup dari file", chatID, err)
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
