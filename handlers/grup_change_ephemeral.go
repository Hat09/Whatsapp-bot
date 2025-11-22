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

// GroupEphemeralState manages the state for changing group ephemeral message settings
type GroupEphemeralState struct {
	WaitingForGroupName bool
	WaitingForDelay     bool
	WaitingForDuration  bool
	SelectedGroups      []GroupLinkInfo
	Keyword             string
	DelaySeconds        int
	DurationSeconds     int64 // 0 = OFF, 86400 = 24h, 604800 = 7d, 7776000 = 90d
}

var groupEphemeralStates = make(map[int64]*GroupEphemeralState)

// ShowChangeEphemeralMenu menampilkan menu atur pesan sementara grup
func ShowChangeEphemeralMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `⏱️ **ATUR PESAN SEMENTARA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengatur durasi pesan sementara (disappearing messages) untuk grup WhatsApp yang Anda pilih. Pesan akan otomatis terhapus setelah durasi yang ditentukan.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Atur Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk mengatur ini
• Delay membantu menghindari rate limit WhatsApp
• Setting akan sama untuk semua grup yang dipilih
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Pilihan Durasi:**
• **OFF** - Nonaktifkan pesan sementara
• **24 Jam** - Pesan terhapus setelah 24 jam
• **7 Hari** - Pesan terhapus setelah 7 hari
• **90 Hari** - Pesan terhapus setelah 90 hari

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_ephemeral"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_ephemeral"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Atur Semua", "change_all_ephemeral"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "ephemeral_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// ShowChangeEphemeralMenuEdit menampilkan menu atur pesan sementara dengan EDIT message (no spam!)
func ShowChangeEphemeralMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `⏱️ **ATUR PESAN SEMENTARA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengatur durasi pesan sementara (disappearing messages) untuk grup WhatsApp yang Anda pilih. Pesan akan otomatis terhapus setelah durasi yang ditentukan.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Atur Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk mengatur ini
• Delay membantu menghindari rate limit WhatsApp
• Setting akan sama untuk semua grup yang dipilih
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Pilihan Durasi:**
• **OFF** - Nonaktifkan pesan sementara
• **24 Jam** - Pesan terhapus setelah 24 jam
• **7 Hari** - Pesan terhapus setelah 7 hari
• **90 Hari** - Pesan terhapus setelah 90 hari

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_ephemeral"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_ephemeral"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Atur Semua", "change_all_ephemeral"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "ephemeral_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowEphemeralExampleEdit menampilkan contoh penggunaan dengan EDIT message
func ShowEphemeralExampleEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	exampleMsg := `📖 **CONTOH PENGGUNAAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Metode 1: Lihat & Pilih**
1️⃣ Klik "📋 Lihat & Pilih"
2️⃣ Bot tampilkan daftar grup
3️⃣ Ketik nomor grup (misal: 1,3,5)
4️⃣ Tentukan delay (misal: 2 detik)
5️⃣ Klik button durasi (OFF/24 Jam/7 Hari/90 Hari)
6️⃣ Selesai! Setting diatur

**🔍 Metode 2: Cari Manual**
1️⃣ Klik "🔍 Cari Manual"
2️⃣ Ketik kata kunci (misal: "Keluarga")
3️⃣ Bot tampilkan hasil pencarian
4️⃣ Tentukan delay
5️⃣ Klik button durasi
6️⃣ Selesai! Setting diatur

**⚡ Metode 3: Atur Semua**
1️⃣ Klik "⚡ Atur Semua"
2️⃣ Konfirmasi total grup
3️⃣ Tentukan delay (rekomendasi: 3-5 detik)
4️⃣ Klik button durasi
5️⃣ Bot proses semua grup
6️⃣ Hasil dikirim per batch

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips Delay:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

✅ **Tips Durasi:**
• Klik **❌ OFF** untuk nonaktifkan pesan sementara
• Klik **⏰ 24 Jam** untuk 24 jam
• Klik **📅 7 Hari** untuk 7 hari
• Klik **🗓️ 90 Hari** untuk 90 hari
• Tidak perlu ketik, langsung klik button!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_ephemeral_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exampleMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartChangeEphemeralProcess memulai proses atur pesan sementara
func StartChangeEphemeralProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	groupEphemeralStates[chatID] = &GroupEphemeralState{
		WaitingForGroupName: true,
		WaitingForDelay:     false,
		WaitingForDuration:  false,
		SelectedGroups:      []GroupLinkInfo{},
		Keyword:             "",
		DelaySeconds:        0,
		DurationSeconds:     0,
	}

	promptMsg := `🔍 **MASUKKAN NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

Ketik nama grup atau kata kunci untuk mencari grup yang ingin diatur pengaturan pesan sementaranya.

**Contoh Input:**
• "Keluarga" - Cari grup dengan kata keluarga
• "Kerja" - Cari grup dengan kata kerja
• "." - Atur SEMUA grup (hati-hati!)

**Multi-line Input (Exact Match):**
GROUP ANGKATAN 1
GROUP ANGKATAN 2
GROUP ANGKATAN 3

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan kata kunci spesifik untuk hasil akurat
• Multi-line untuk exact match nama grup
• Gunakan "." jika ingin atur semua grup
• Pencarian tidak case-sensitive

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_ephemeral"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForEphemeral memproses input nama grup
func HandleGroupNameInputForEphemeral(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupEphemeralStates[chatID]
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

	// Smart search logic (same as other features)
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
				tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Lagi", "start_change_ephemeral"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_ephemeral_menu"),
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

**Daftar grup yang akan diatur:**

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

Ketik delay (dalam detik) untuk setiap grup yang berhasil diatur.

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
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_ephemeral"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInputForEphemeral memproses input delay
func HandleDelayInputForEphemeral(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupEphemeralStates[chatID]
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
	state.WaitingForDuration = true

	// Ask for duration with buttons
	durationMsg := fmt.Sprintf(`✅ **ATUR DURASI PESAN SEMENTARA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Grup dipilih:** %d grup
⏱️ **Delay:** %d detik per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **LANGKAH TERAKHIR**

Klik button durasi untuk mengatur pesan sementara.

**Penjelasan:**
• **❌ OFF** - Nonaktifkan pesan sementara
• **⏰ 24 Jam** - Pesan terhapus setelah 24 jam
• **📅 7 Hari** - Pesan terhapus setelah 7 hari
• **🗓️ 90 Hari** - Pesan terhapus setelah 90 hari

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Klik button di bawah (tidak perlu ketik!)
• Durasi akan sama untuk semua grup yang dipilih
• Pesan akan otomatis terhapus setelah durasi

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Klik button durasi di bawah...`, len(state.SelectedGroups), delay)

	msg := tgbotapi.NewMessage(chatID, durationMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "ephemeral_duration_off"),
			tgbotapi.NewInlineKeyboardButtonData("⏰ 24 Jam", "ephemeral_duration_24h"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 7 Hari", "ephemeral_duration_7d"),
			tgbotapi.NewInlineKeyboardButtonData("🗓️ 90 Hari", "ephemeral_duration_90d"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_ephemeral"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDurationInputForEphemeral memproses input durasi dari button
func HandleDurationInputForEphemeral(durationSeconds int64, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := groupEphemeralStates[chatID]
	if state == nil || !state.WaitingForDuration {
		return
	}

	state.DurationSeconds = durationSeconds
	state.WaitingForDuration = false

	// Format duration text
	var durationText string
	switch durationSeconds {
	case 0:
		durationText = "OFF"
	case 86400:
		durationText = "24 Jam"
	case 604800:
		durationText = "7 Hari"
	case 7776000:
		durationText = "90 Hari"
	default:
		durationText = fmt.Sprintf("%d detik", durationSeconds)
	}

	startMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Durasi diterima!\n\n⏱️ **Durasi:** %s\n\n🚀 Memulai proses atur pesan sementara untuk %d grup...",
		durationText, len(state.SelectedGroups)))
	startMsg.ParseMode = "Markdown"
	telegramBot.Send(startMsg)

	// Process in goroutine
	go ProcessChangeEphemeral(state.SelectedGroups, state.DelaySeconds, state.DurationSeconds, chatID, client, telegramBot)

	// Clear state
	delete(groupEphemeralStates, chatID)
}

// ProcessChangeEphemeral memproses pengaturan pesan sementara grup
func ProcessChangeEphemeral(groups []GroupLinkInfo, delay int, durationSeconds int64, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	totalGroups := len(groups)
	successCount := 0
	failedCount := 0
	var failedGroups []string

	var progressMsgSent *tgbotapi.Message

	// Format duration text
	var durationText string
	switch durationSeconds {
	case 0:
		durationText = "OFF"
	case 86400:
		durationText = "24 Jam"
	case 604800:
		durationText = "7 Hari"
	case 7776000:
		durationText = "90 Hari"
	default:
		durationText = fmt.Sprintf("%d detik", durationSeconds)
	}

	for i, group := range groups {
		// MEDIUM FIX: Ambil active client di setiap iterasi!
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessChangeEphemeral", i, totalGroups)
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

		// Set group ephemeral/disappearing messages
		// Duration: 0 = OFF, 86400 = 24h, 604800 = 7d, 7776000 = 90d
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel() // FIXED: Use defer to ensure cancellation

		// SetDisappearingTimer requires time.Duration and settingTS
		timer := time.Duration(durationSeconds) * time.Second
		settingTS := time.Now()

		err = validClient.SetDisappearingTimer(ctx, jid, timer, settingTS)

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
⏱️ **Durasi:** %s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⏳ Sedang memproses...`, progressBar, progressPercent, i+1, totalGroups, successCount, failedCount, durationText)

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
⏱️ **Durasi:** %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, successCount, failedCount, delay, durationText)

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
			tgbotapi.NewInlineKeyboardButtonData("✅ Atur Lagi", "change_ephemeral_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
		),
	)

	completionMsg := tgbotapi.NewMessage(chatID, "💡 Apa yang ingin Anda lakukan selanjutnya?")
	completionMsg.ReplyMarkup = keyboard
	telegramBot.Send(completionMsg)
}

// CancelChangeEphemeral membatalkan proses atur pesan sementara
func CancelChangeEphemeral(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(groupEphemeralStates, chatID)

	msg := tgbotapi.NewMessage(chatID, "❌ Proses atur pesan sementara grup dibatalkan.")
	telegramBot.Send(msg)
}

// IsWaitingForEphemeralInput checks if user is waiting to input ephemeral-related data
func IsWaitingForEphemeralInput(chatID int64) bool {
	state := groupEphemeralStates[chatID]
	return state != nil && (state.WaitingForGroupName || state.WaitingForDelay)
}

// GetEphemeralInputType returns the current input type
func GetEphemeralInputType(chatID int64) string {
	state := groupEphemeralStates[chatID]
	if state == nil {
		return ""
	}

	if state.WaitingForGroupName {
		return "group_name"
	}
	if state.WaitingForDelay {
		return "delay"
	}

	return ""
}

// ProcessSelectedGroupsForEphemeral processes selected groups untuk atur pesan sementara
func ProcessSelectedGroupsForEphemeral(selection string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	selectedGroups := HandleGroupSelection(selection, chatID, telegramBot)

	if len(selectedGroups) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Pilihan tidak valid!\n\nContoh: 1, 1-5, 1,3,5, atau 'all'")
		telegramBot.Send(errorMsg)
		return
	}

	// Clear ephemeral selection marker if exists
	delete(ephemeralSelection, chatID)

	// Initialize state
	groupEphemeralStates[chatID] = &GroupEphemeralState{
		WaitingForGroupName: false,
		WaitingForDelay:     true,
		WaitingForDuration:  false,
		SelectedGroups:      selectedGroups,
		Keyword:             "",
		DelaySeconds:        0,
		DurationSeconds:     0,
	}

	// Confirm selection dan tanya delay
	confirmMsg := fmt.Sprintf(`✅ **GRUP TERPILIH**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total terpilih:** %d grup

**Daftar grup:**
`, len(selectedGroups))

	for i, group := range selectedGroups {
		if i < 10 {
			confirmMsg += fmt.Sprintf("%d. %s\n", i+1, group.Name)
		}
	}

	if len(selectedGroups) > 10 {
		confirmMsg += fmt.Sprintf("\n... dan %d grup lainnya\n", len(selectedGroups)-10)
	}

	confirmMsg += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay antar permintaan.

**Rekomendasi:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_ephemeral"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleChangeAllEphemeral handles "Atur Semua" untuk pesan sementara
func HandleChangeAllEphemeral(chatID int64, telegramBot *tgbotapi.BotAPI) {
	// Get all groups
	groupsMap, err := utils.GetAllGroupsFromDB()
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	if len(groupsMap) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang tersedia.")
		telegramBot.Send(errorMsg)
		return
	}

	// Convert to GroupLinkInfo with natural sorting
	sortedGroups := utils.SortGroupsNaturally(groupsMap)
	selectedGroups := []GroupLinkInfo{}
	for _, group := range sortedGroups {
		selectedGroups = append(selectedGroups, GroupLinkInfo{
			JID:  group.JID,
			Name: group.Name,
		})
	}

	// Initialize state
	groupEphemeralStates[chatID] = &GroupEphemeralState{
		WaitingForGroupName: false,
		WaitingForDelay:     true,
		WaitingForDuration:  false,
		SelectedGroups:      selectedGroups,
		Keyword:             "",
		DelaySeconds:        0,
		DurationSeconds:     0,
	}

	confirmMsg := fmt.Sprintf(`⚡ **ATUR SEMUA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total grup:** %d grup

⚠️ **PERINGATAN:**
Anda akan mengatur pengaturan pesan sementara untuk SEMUA grup sekaligus!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay antar permintaan.

**Rekomendasi:**
• 3-5 detik untuk menghindari rate limit
• Proses mungkin memakan waktu lama

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 4)`, len(selectedGroups))

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_ephemeral"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowGroupListForEphemeralEdit menampilkan daftar grup dengan pagination untuk atur pesan sementara (EDIT, NO SPAM!)
func ShowGroupListForEphemeralEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int, page int) {
	// CRITICAL FIX: Pastikan menggunakan database user yang benar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)
	if userAccount != nil {
		EnsureDBConfigForUser(chatID, userAccount)
	}

	// Get all groups
	groupsMap, err := utils.GetAllGroupsFromDB()
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(editMsg)
		return
	}

	if len(groupsMap) == 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Tidak ada grup yang terdeteksi.\n\nGrup akan otomatis terdeteksi saat bot menerima pesan dari grup.")
		telegramBot.Send(editMsg)
		return
	}

	// Convert to slice with natural sorting
	groups := []GroupLinkInfo{}

	// Use natural sorting
	sortedGroups := utils.SortGroupsNaturally(groupsMap)
	for _, group := range sortedGroups {
		groups = append(groups, GroupLinkInfo{
			JID:  group.JID,
			Name: group.Name,
		})
	}

	// Pagination
	groupsPerPage := 10
	totalPages := (len(groups) + groupsPerPage - 1) / groupsPerPage
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * groupsPerPage
	end := start + groupsPerPage
	if end > len(groups) {
		end = len(groups)
	}

	pageGroups := groups[start:end]

	// Build message
	msg := fmt.Sprintf(`📋 **DAFTAR GRUP - ATUR PESAN SEMENTARA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total:** %d grup
📄 **Halaman:** %d dari %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Ketik nomor grup untuk memilih:**
(Contoh: 1,3,5 atau 1-10)

`, len(groups), page, totalPages)

	for i, group := range pageGroups {
		num := start + i + 1
		msg += fmt.Sprintf("**%d.** %s\n", num, group.Name)
	}

	msg += fmt.Sprintf(`
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Cara Pilih:**
• Ketik: **1** (pilih 1 grup)
• Ketik: **1,3,5** (pilih beberapa)
• Ketik: **1-10** (pilih range)
• Ketik: **all** (pilih semua)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`)

	// Build keyboard
	var keyboard tgbotapi.InlineKeyboardMarkup

	// Navigation buttons
	navRow := []tgbotapi.InlineKeyboardButton{}
	if page > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("ephemeral_page_%d", page-1)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📄 %d/%d", page, totalPages), "noop"))
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️ Next", fmt.Sprintf("ephemeral_page_%d", page+1)))
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)

	// Quick action buttons
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Pilih Semua", "change_all_ephemeral"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_ephemeral_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, msg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)

	// Store state with custom marker to distinguish from other selections
	SetListSelectStateForEphemeral(chatID, page, totalPages, groupsPerPage, groups)
}

// SetListSelectStateForEphemeral sets the list select state with a marker for ephemeral
func SetListSelectStateForEphemeral(chatID int64, page, totalPages, groupsPerPage int, groups []GroupLinkInfo) {
	listSelectStates[chatID] = &ListSelectState{
		CurrentPage:    page,
		TotalPages:     totalPages,
		GroupsPerPage:  groupsPerPage,
		AllGroups:      groups,
		SelectedGroups: make(map[int]bool),
	}
	ephemeralSelection[chatID] = true
}

// Map to track if selection is for ephemeral
var ephemeralSelection = make(map[int64]bool)

// IsWaitingForEphemeralSelection checks if user is selecting groups for ephemeral
func IsWaitingForEphemeralSelection(chatID int64) bool {
	return ephemeralSelection[chatID] && listSelectStates[chatID] != nil
}

// HandleFileInputForChangeEphemeral - Handle file .txt untuk atur pesan sementara grup
func HandleFileInputForChangeEphemeral(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := groupEphemeralStates[chatID]
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
		utils.LogActivityError("change_ephemeral", "Gagal download file", chatID, err)
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
		utils.LogActivityError("change_ephemeral", "File tidak valid", chatID, err)
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
		utils.LogActivityError("change_ephemeral", "Gagal membaca file", chatID, err)
		return
	}
	defer fileResp2.Body.Close()

	// Read file content
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
		utils.LogActivityError("change_ephemeral", "Error membaca file", chatID, err)
		return
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **FILE KOSONG**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nFile `.txt` yang Anda kirim tidak berisi nama grup.\n\n**Format yang benar:**\nSatu nama grup per baris.\n\n**Contoh:**\nKeluarga Besar\nGrup Kerja\nGrup Teman")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Log activity
	utils.LogActivity("change_ephemeral_file", fmt.Sprintf("File .txt diterima dengan %d nama grup", len(groupNames)), chatID)

	// Search groups
	groups, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_ephemeral", "Gagal mencari grup dari file", chatID, err)
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
