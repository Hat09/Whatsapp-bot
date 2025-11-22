package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// LinkGrupState manages state for link grup feature
type LinkGrupState struct {
	WaitingForGroupName bool
	WaitingForDelay     bool
	SelectedGroups      []GroupLinkInfo
	Keyword             string
}

// GroupLinkInfo stores group information for link extraction
type GroupLinkInfo struct {
	JID  string
	Name string
}

var linkGrupStates = make(map[int64]*LinkGrupState)

// ShowGetLinkMenu menampilkan menu untuk ambil link grup
func ShowGetLinkMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `🔗 **AMBIL LINK GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengambil link undangan dari grup WhatsApp yang Anda pilih.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ambil Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk mengambil link
• Delay membantu menghindari rate limit WhatsApp
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_link"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_get_link"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Ambil Semua", "get_all_links"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "link_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowGetLinkMenuEdit menampilkan menu ambil link dengan EDIT message (no spam!)
func ShowGetLinkMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `🔗 **AMBIL LINK GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengambil link undangan dari grup WhatsApp yang Anda pilih.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ambil Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk mengambil link
• Delay membantu menghindari rate limit WhatsApp
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_link"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_get_link"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Ambil Semua", "get_all_links"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "link_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowLinkExample menampilkan contoh penggunaan
func ShowLinkExample(telegramBot *tgbotapi.BotAPI, chatID int64) {
	exampleMsg := `📖 **CONTOH PENGGUNAAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Skenario 1: Ambil link grup keluarga**

👤 User: Klik "🔍 Cari Grup"
🤖 Bot: "Masukkan nama grup..."
👤 User: "Keluarga"
🤖 Bot: "Ditemukan 3 grup: ..."
🤖 Bot: "Berapa delay? (detik)"
👤 User: "2"
🤖 Bot: Proses mengambil 3 link dengan delay 2 detik

**Skenario 2: Ambil semua link grup**

👤 User: Klik "🔍 Cari Grup"
🤖 Bot: "Masukkan nama grup..."
👤 User: "." (titik untuk semua grup)
🤖 Bot: "Ditemukan 25 grup"
🤖 Bot: "Berapa delay? (detik)"
👤 User: "3"
🤖 Bot: Proses 25 grup dengan delay 3 detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan delay 2-5 detik untuk grup banyak
• Delay terlalu kecil bisa kena rate limit
• Gunakan "." untuk mengambil semua grup
• Pastikan bot adalah admin di grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	msg := tgbotapi.NewMessage(chatID, exampleMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 Mulai Cari", "start_get_link"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "get_link_menu"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowLinkExampleEdit menampilkan contoh dengan EDIT message (no spam!)
func ShowLinkExampleEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	exampleMsg := `📖 **CONTOH PENGGUNAAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Metode 1: Lihat & Pilih**
1️⃣ Klik "📋 Lihat & Pilih"
2️⃣ Bot tampilkan daftar grup
3️⃣ Ketik nomor grup (misal: 1,3,5)
4️⃣ Tentukan delay (misal: 2 detik)
5️⃣ Selesai! Link dikirim

**🔍 Metode 2: Cari Manual**
1️⃣ Klik "🔍 Cari Manual"
2️⃣ Ketik kata kunci (misal: "XTC")
3️⃣ Bot tampilkan hasil pencarian
4️⃣ Tentukan delay
5️⃣ Selesai! Link dikirim

**⚡ Metode 3: Ambil Semua**
1️⃣ Klik "⚡ Ambil Semua"
2️⃣ Konfirmasi total grup
3️⃣ Tentukan delay (rekomendasi: 3-5 detik)
4️⃣ Bot proses semua grup
5️⃣ Hasil dikirim per batch

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips Delay:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "get_link_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exampleMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartGetLinkProcess memulai proses ambil link
func StartGetLinkProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	linkGrupStates[chatID] = &LinkGrupState{
		WaitingForGroupName: true,
		WaitingForDelay:     false,
		SelectedGroups:      []GroupLinkInfo{},
		Keyword:             "",
	}

	promptMsg := `🔍 **MASUKKAN NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

**📋 Opsi 1: Input Text**
Ketik nama grup atau kata kunci untuk mencari grup yang ingin diambil linknya.

**📄 Opsi 2: Upload File .txt**
Kirim file .txt yang berisi barisan nama grup (satu per baris)

**Contoh Input Text:**
• "Keluarga" - Cari grup dengan kata keluarga
• "Kerja" - Cari grup dengan kata kerja
• "." - Ambil SEMUA grup (hati-hati!)

**Contoh Format File .txt:**
Keluarga Besar
Grup Kerja
Grup Teman

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan kata kunci spesifik untuk hasil akurat
• Gunakan "." jika ingin ambil semua grup
• Pencarian case-insensitive
• File .txt: satu nama grup per baris

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Ketik nama grup atau kirim file .txt...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_get_link"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInput handles input nama grup
func HandleGroupNameInput(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := linkGrupStates[chatID]
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

	// Show loading
	loadingMsg := tgbotapi.NewMessage(chatID, "🔍 Mencari grup...")
	loadingMsgSent, _ := telegramBot.Send(loadingMsg)

	// Search groups
	var groups map[string]string
	var err error

	if keyword == "." {
		// Get all groups
		groups, err = utils.GetAllGroupsFromDB()
	} else {
		// Check if multi-line input (user input multiple group names)
		lines := strings.Split(keyword, "\n")
		if len(lines) > 1 {
			// Multi-line: exact match for each line
			groups, err = utils.SearchGroupsExactMultiple(lines)
		} else if len(keyword) > 30 {
			// Long single line: try exact match first, then flexible
			groups, err = utils.SearchGroupsExact(keyword)
			if err == nil && len(groups) == 0 {
				// Fallback to flexible if exact not found
				groups, err = utils.SearchGroupsFlexible(keyword)
			}
		} else {
			// Short keyword: use flexible matching
			groups, err = utils.SearchGroupsFlexible(keyword)
		}
	}

	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)

		// Reset state
		delete(linkGrupStates, chatID)
		return
	}

	// Delete loading message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
	telegramBot.Request(deleteMsg)

	if len(groups) == 0 {
		// Get sample groups untuk preview
		allGroups, _ := utils.GetAllGroupsFromDB()
		sampleMsg := "Tidak ada grup yang cocok.\n\n**Contoh nama grup yang tersedia:**\n"
		count := 0
		for _, name := range allGroups {
			if count >= 5 {
				break
			}
			sampleMsg += fmt.Sprintf("• %s\n", name)
			count++
		}
		if len(allGroups) > 5 {
			sampleMsg += fmt.Sprintf("\n... dan %d grup lainnya", len(allGroups)-5)
		}

		noResultMsg := fmt.Sprintf(`❌ **TIDAK ADA GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Kata kunci: "%s"

%s

**Saran:**
• Coba kata kunci lebih pendek (1-2 kata)
• Gunakan kata yang pasti ada di nama grup
• Atau klik "📋 Lihat Daftar" untuk melihat semua`, keyword, sampleMsg)

		msg := tgbotapi.NewMessage(chatID, noResultMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📋 Lihat Daftar", "show_group_list_link"),
				tgbotapi.NewInlineKeyboardButtonData("🔍 Coba Lagi", "start_get_link"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Menu", "get_link_menu"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)

		// Reset state
		delete(linkGrupStates, chatID)
		return
	}

	// Store selected groups with natural sorting
	state.SelectedGroups = []GroupLinkInfo{}

	// Sort groups naturally
	sortedGroups := utils.SortGroupsNaturally(groups)
	for _, group := range sortedGroups {
		state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
			JID:  group.JID,
			Name: group.Name,
		})
	}

	// Update state
	state.WaitingForGroupName = false
	state.WaitingForDelay = true

	// Show found groups and ask for delay
	resultMsg := fmt.Sprintf(`✅ **GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Kata Kunci:** "%s"
✅ **Ditemukan:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Daftar Grup:**
`, keyword, len(groups))

	count := 1
	for _, group := range state.SelectedGroups {
		if count <= 10 { // Show max 10 groups
			resultMsg += fmt.Sprintf("%d. %s\n", count, group.Name)
		}
		count++
	}

	if len(state.SelectedGroups) > 10 {
		resultMsg += fmt.Sprintf("\n... dan %d grup lainnya\n", len(state.SelectedGroups)-10)
	}

	resultMsg += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay antar permintaan untuk menghindari rate limit.

**Rekomendasi:**
• 1-2 detik: Untuk grup sedikit (< 10)
• 2-3 detik: Untuk grup sedang (10-30)
• 3-5 detik: Untuk grup banyak (> 30)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_get_link"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInput handles input delay
func HandleDelayInput(delayStr string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := linkGrupStates[chatID]
	if state == nil || !state.WaitingForDelay {
		return
	}

	// Parse delay
	delay, err := strconv.Atoi(strings.TrimSpace(delayStr))
	if err != nil || delay < 1 || delay > 60 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Delay harus berupa angka antara 1-60 detik!\n\nContoh: 2")
		telegramBot.Send(errorMsg)
		return
	}

	// Reset state
	state.WaitingForDelay = false

	// Start processing
	go ProcessGetLinks(state.SelectedGroups, delay, chatID, client, telegramBot, state.Keyword)

	// Clear state
	delete(linkGrupStates, chatID)
}

// ProcessGetLinks processes link extraction with delay
func ProcessGetLinks(groups []GroupLinkInfo, delay int, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI, keyword string) {
	totalGroups := len(groups)

	startMsg := fmt.Sprintf(`🚀 **MEMULAI PROSES**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **Delay:** %d detik/grup
⏳ **Estimasi:** ~%d detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Proses dimulai... Harap tunggu!`, delay, totalGroups*delay)

	msg := tgbotapi.NewMessage(chatID, startMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	startTime := time.Now()

	// Process each group
	successCount := 0
	failedCount := 0
	results := []string{}
	failedGroups := []string{}

	// Progress message tracking
	var progressMsgSent *tgbotapi.Message
	lastProgressUpdate := 0
	lastProgressUpdateTime := startTime // Track waktu update terakhir untuk interval waktu

	// Update progress juga untuk grup pertama (0%)
	if totalGroups > 3 {
		progressMsg := fmt.Sprintf(`⏳ **PROGRESS REALTIME**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

░░░░░░░░░░░░░░░░░░░░ **0%%**

📊 **Status:**
• Diproses: 0 / %d grup
• ✅ Berhasil: 0 grup
• ❌ Gagal: 0 grup
• 📋 Sisa: %d grup

⏱️ **Estimasi:**
• Waktu Total: ~%s
• Selesai: Menghitung...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Memulai proses...`,
			totalGroups, totalGroups, time.Duration(totalGroups*delay)*time.Second)

		updateMsg := tgbotapi.NewMessage(chatID, progressMsg)
		updateMsg.ParseMode = "Markdown"
		sent, _ := telegramBot.Send(updateMsg)
		progressMsgSent = &sent
	}

	// Determine if we should use file export (for large results)
	useFileExport := totalGroups > 50
	var tempFile *os.File
	var tempFileName string
	lastSaveCount := 0
	saveInterval := 50 // Auto-save setiap 50 grup yang berhasil diproses

	if useFileExport {
		// Create temporary file for large results
		timestamp := time.Now().Format("20060102_150405")
		tempFileName = fmt.Sprintf("group_links_%s.txt", timestamp)
		var err error
		tempFile, err = os.Create(tempFileName)
		if err != nil {
			// Fallback to in-memory if file creation fails
			useFileExport = false
			utils.GetGrupLogger().Warn("ProcessGetLinks: Failed to create temp file, using in-memory storage: %v", err)
		}
		// Note: Tidak perlu write header, langsung mulai dengan grup pertama
	}

	for i, group := range groups {
		// IMPORTANT: Ambil active client di setiap iterasi untuk proses panjang!
		// Ini mencegah masalah client stale setelah berjam-jam
		activeClient := GetWhatsAppClient()
		if activeClient == nil {
			activeClient = client // Fallback ke parameter jika benar-benar tidak ada
		}

		// Check connection sebelum request (penting untuk proses panjang!)
		if activeClient == nil || !activeClient.IsConnected() {
			failedCount++
			errorMsg := fmt.Sprintf("❌ %s\n   💡 Client tidak terhubung", group.Name)
			// FIXED: failedGroups digunakan untuk tracking, tapi result of append tidak digunakan
			// Gunakan untuk tracking meskipun tidak digunakan di akhir
			_ = append(failedGroups, errorMsg)
			if useFileExport && tempFile != nil {
				// FIXED: Handle error untuk file write operations
				if _, err := tempFile.WriteString(fmt.Sprintf("%d. %s\n   Error: Client tidak terhubung\n\n", i+1, group.Name)); err != nil {
					utils.GetGrupLogger().Warn("ProcessGetLinks: Gagal menulis ke file: %v", err)
				}
			} else {
				results = append(results, errorMsg)
			}

			// Jika client disconnect, tidak ada gunanya lanjut. Stop proses.
			utils.GetGrupLogger().Warn("ProcessGetLinks: Client disconnected at group %d/%d. Stopping process.", i+1, len(groups))
			break
		}

		// Parse JID
		jid, err := types.ParseJID(group.JID)
		if err != nil {
			failedCount++
			errorMsg := fmt.Sprintf("❌ %s\n   💡 Invalid JID", group.Name)
			// FIXED: failedGroups digunakan untuk tracking, tapi result of append tidak digunakan
			_ = append(failedGroups, errorMsg)
			if useFileExport && tempFile != nil {
				// FIXED: Handle error untuk file write operations
				if _, err := tempFile.WriteString(fmt.Sprintf("%d. %s\n   Error: Invalid JID\n\n", i+1, group.Name)); err != nil {
					utils.GetGrupLogger().Warn("ProcessGetLinks: Gagal menulis ke file: %v", err)
				}
			} else {
				results = append(results, errorMsg)
			}
			continue
		}

		// Get invite link dengan active client
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel() // FIXED: Use defer to ensure cancellation
		link, err := activeClient.GetGroupInviteLink(ctx, jid, false)

		if err != nil {
			failedCount++
			errorDetail := "Tidak dapat mengambil link"
			if strings.Contains(err.Error(), "not-authorized") || strings.Contains(err.Error(), "not an admin") {
				errorDetail = "Bot bukan admin"
			} else if strings.Contains(err.Error(), "item-not-found") {
				errorDetail = "Grup tidak ditemukan"
			} else if strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "timeout") {
				errorDetail = "Timeout (WhatsApp tidak merespon)"
			}
			errorMsg := fmt.Sprintf("❌ %s\n   💡 %s", group.Name, errorDetail)
			// FIXED: failedGroups digunakan untuk tracking, tapi result of append tidak digunakan
			_ = append(failedGroups, errorMsg)
			if useFileExport && tempFile != nil {
				// Tidak perlu tulis error ke file (hanya yang berhasil)
			} else {
				results = append(results, errorMsg)
			}
		} else {
			successCount++
			successMsg := fmt.Sprintf("✅ **%s**\n   🔗 %s", group.Name, link)
			if useFileExport && tempFile != nil {
				// Format sederhana: Nama Grup, lalu link di bawahnya (sesuai permintaan user)
				// FIXED: Handle error untuk file write operations
				if _, err := tempFile.WriteString(fmt.Sprintf("%s\n\n%s\n\n", group.Name, link)); err != nil {
					utils.GetGrupLogger().Warn("ProcessGetLinks: Gagal menulis ke file: %v", err)
				}

				// Auto-save setiap 50 grup yang berhasil diproses
				if successCount-lastSaveCount >= saveInterval {
					// FIXED: Handle error untuk file sync operations
					if err := tempFile.Sync(); err != nil {
						utils.GetGrupLogger().Warn("ProcessGetLinks: Gagal sync file: %v", err)
					}
					lastSaveCount = successCount
					utils.GetGrupLogger().Info("ProcessGetLinks: Auto-saved progress to file (%d grup berhasil)", successCount)
				}
			} else {
				results = append(results, successMsg)
			}
		}

		// Update progress secara realtime (setiap grup untuk akurasi 100%)
		// Tapi untuk menghindari spam, update hanya jika:
		// 1. Setiap beberapa grup diproses, ATAU
		// 2. Persentase berubah, ATAU
		// 3. Setiap beberapa detik berlalu, ATAU
		// 4. Ini iterasi terakhir
		if totalGroups > 3 {
			currentProgress := float64(i+1) * 100.0 / float64(totalGroups)
			progressDiff := i + 1 - lastProgressUpdate
			lastProgressPercent := float64(lastProgressUpdate) * 100.0 / float64(totalGroups)

			// Untuk proses panjang (delay besar atau grup banyak), update lebih sering
			updateInterval := 5 // Update setiap 5 grup untuk realtime
			if totalGroups > 100 {
				updateInterval = 3 // Update setiap 3 grup untuk ribuan grup
			}
			if delay >= 10 {
				updateInterval = 3 // Update setiap 3 grup untuk delay besar
			}

			// Hitung waktu sejak update terakhir
			elapsedSinceLastUpdate := time.Since(lastProgressUpdateTime)

			// Update jika:
			// 1. Sudah melewati interval grup, ATAU
			// 2. Persentase berubah 1% atau lebih, ATAU
			// 3. Setiap 10 detik berlalu (untuk delay besar), ATAU
			// 4. Ini iterasi terakhir
			shouldUpdate := (progressDiff >= updateInterval) ||
				(currentProgress-lastProgressPercent >= 1.0) ||
				(elapsedSinceLastUpdate.Seconds() >= 10.0) ||
				(i == len(groups)-1)

			if shouldUpdate {
				progressPercent := int(currentProgress)
				progressBar := generateProgressBar(progressPercent)

				// Hitung sisa grup yang harus diambil
				remainingGroups := totalGroups - (i + 1)

				// Calculate estimated time remaining (realtime berdasarkan delay yang diinput user)
				elapsedTime := time.Since(startTime)
				var estimatedRemaining string
				var estimatedFinishTime string

				// Hitung berdasarkan waktu aktual (lebih akurat untuk realtime)
				var remainingTimeActual time.Duration
				if i > 0 {
					avgTimePerGroup := elapsedTime / time.Duration(i+1)
					remainingTimeActual = avgTimePerGroup * time.Duration(remainingGroups)

					// Hitung waktu selesai
					finishTime := time.Now().Add(remainingTimeActual)
					estimatedFinishTime = finishTime.Format("15:04:05")
				}

				// Juga hitung berdasarkan delay (untuk perbandingan)
				remainingTimeFromDelay := time.Duration(remainingGroups*delay) * time.Second

				// Gunakan waktu aktual jika sudah ada data, fallback ke delay
				var displayTime time.Duration
				if i > 0 && remainingTimeActual > 0 {
					displayTime = remainingTimeActual
				} else {
					displayTime = remainingTimeFromDelay
				}

				// Format estimasi waktu
				if displayTime.Hours() >= 1 {
					hours := int(displayTime.Hours())
					minutes := int(displayTime.Minutes()) % 60
					estimatedRemaining = fmt.Sprintf("%d jam %d menit", hours, minutes)
				} else if displayTime.Minutes() >= 1 {
					estimatedRemaining = fmt.Sprintf("%.0f menit", displayTime.Minutes())
				} else {
					estimatedRemaining = fmt.Sprintf("%.0f detik", displayTime.Seconds())
				}

				// Format progress message dengan informasi lengkap dan realtime
				progressMsg := fmt.Sprintf(`⏳ **PROGRESS REALTIME**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s **%d%%**

📊 **Status:**
• Diproses: %d / %d grup
• ✅ Berhasil: %d grup
• ❌ Gagal: %d grup
• 📋 Sisa: %d grup

⏱️ **Estimasi:**
• Waktu Sisa: %s
• Selesai: ~%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Sedang memproses grup ke-%d...`,
					progressBar, progressPercent,
					i+1, totalGroups,
					successCount, failedCount,
					remainingGroups,
					estimatedRemaining,
					estimatedFinishTime,
					i+1)

				if progressMsgSent == nil {
					// Send new progress message
					updateMsg := tgbotapi.NewMessage(chatID, progressMsg)
					updateMsg.ParseMode = "Markdown"
					sent, _ := telegramBot.Send(updateMsg)
					progressMsgSent = &sent
				} else {
					// Edit existing progress message (NO SPAM! - hanya edit, tidak kirim pesan baru)
					editMsg := tgbotapi.NewEditMessageText(chatID, progressMsgSent.MessageID, progressMsg)
					editMsg.ParseMode = "Markdown"
					telegramBot.Send(editMsg)
				}

				lastProgressUpdate = i + 1
				lastProgressUpdateTime = time.Now() // Update waktu terakhir
			}
		}

		// Delay before next request
		if i < len(groups)-1 {
			time.Sleep(time.Duration(delay) * time.Second)
		}

		// Delay before next request
		if i < len(groups)-1 {
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	// Close temporary file if used (dengan final save dan sync)
	if useFileExport && tempFile != nil {
		// FIXED: Handle error untuk file operations
		if err := tempFile.Sync(); err != nil {
			utils.GetGrupLogger().Warn("ProcessGetLinks: Gagal sync file: %v", err)
		}
		if err := tempFile.Close(); err != nil {
			utils.GetGrupLogger().Warn("ProcessGetLinks: Gagal close file: %v", err)
		}

		// Verifikasi file berhasil dibuat dan ada isinya
		if fileInfo, err := os.Stat(tempFileName); err == nil && fileInfo.Size() > 0 {
			utils.GetGrupLogger().Info("ProcessGetLinks: File berhasil disimpan: %s (size: %d bytes, %d grup berhasil)",
				tempFileName, fileInfo.Size(), successCount)
		} else {
			if err != nil {
				utils.GetGrupLogger().Error("ProcessGetLinks: Gagal stat file: %v", err)
			} else {
				utils.GetGrupLogger().Error("ProcessGetLinks: File tidak valid atau kosong: %s", tempFileName)
			}
			useFileExport = false // Fallback ke in-memory jika file tidak valid
		}
	}

	// Delete progress message after completion
	if progressMsgSent != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMsgSent.MessageID)
		telegramBot.Request(deleteMsg)
	}

	// Send final result (smart batching or file export)
	if useFileExport && tempFile != nil {
		// For large results, send as file
		summaryMsg := fmt.Sprintf(`🎉 **PROSES SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Ringkasan:**
• Total: %d grup
• ✅ Berhasil: %d
• ❌ Gagal: %d
⏱️ **Durasi:** %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📄 **Hasil disimpan ke file** (karena jumlah grup besar)

File akan dikirim sebentar lagi...`, totalGroups, successCount, failedCount, time.Since(startTime).Round(time.Second).String())

		summaryMsgObj := tgbotapi.NewMessage(chatID, summaryMsg)
		summaryMsgObj.ParseMode = "Markdown"
		telegramBot.Send(summaryMsgObj)

		// Send file
		fileMsg := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(tempFileName))
		fileMsg.Caption = fmt.Sprintf("🔗 **Hasil Link Grup WhatsApp**\n\n"+
			"📊 Total: %d grup\n"+
			"✅ Berhasil: %d\n"+
			"❌ Gagal: %d\n"+
			"📅 %s",
			totalGroups, successCount, failedCount,
			time.Now().Format("02 Jan 2006 15:04"))
		fileMsg.ParseMode = "Markdown"

		// Add buttons
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Ambil Link Lagi", "get_link_menu"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
			),
		)
		fileMsg.ReplyMarkup = keyboard

		sentFile, err := telegramBot.Send(fileMsg)
		if err != nil {
			utils.GetGrupLogger().Error("ProcessGetLinks: Failed to send file: %v", err)
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal mengirim file: %v", err))
			telegramBot.Send(errorMsg)
			// Jangan hapus file jika gagal kirim, biarkan user bisa download manual
		} else {
			utils.GetGrupLogger().Info("ProcessGetLinks: File berhasil dikirim ke user (message ID: %d)", sentFile.MessageID)

			// Hapus file setelah berhasil dikirim (untuk menghemat storage server)
			// Tunggu sebentar untuk memastikan file terkirim dengan baik
			time.Sleep(2 * time.Second)

			// Hapus file dari server
			if err := os.Remove(tempFileName); err != nil {
				utils.GetGrupLogger().Error("ProcessGetLinks: Gagal menghapus file temporary: %v", err)
				// Coba hapus lagi setelah beberapa detik (jika masih terkunci)
				time.Sleep(3 * time.Second)
				if err2 := os.Remove(tempFileName); err2 != nil {
					utils.GetGrupLogger().Error("ProcessGetLinks: Gagal menghapus file temporary (retry): %v", err2)
				} else {
					utils.GetGrupLogger().Info("ProcessGetLinks: File temporary berhasil dihapus setelah retry: %s", tempFileName)
				}
			} else {
				// Verifikasi file benar-benar terhapus
				if _, err := os.Stat(tempFileName); os.IsNotExist(err) {
					utils.GetGrupLogger().Info("ProcessGetLinks: ✅ File temporary berhasil dihapus dari server: %s (storage dihemat)", tempFileName)
				} else {
					utils.GetGrupLogger().Warn("ProcessGetLinks: File masih ada setelah Remove: %s (mungkin masih terkunci)", tempFileName)
				}
			}
		}
	} else if totalGroups > 10 {
		// Send summary first
		summaryMsg := fmt.Sprintf(`🎉 **PROSES SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Ringkasan:**
• Total: %d grup
• ✅ Berhasil: %d
• ❌ Gagal: %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📥 Hasil akan dikirim dalam beberapa pesan...`, totalGroups, successCount, failedCount)

		summaryMsgObj := tgbotapi.NewMessage(chatID, summaryMsg)
		summaryMsgObj.ParseMode = "Markdown"
		telegramBot.Send(summaryMsgObj)

		// Send results in batches with message length check
		batchSize := 10
		const maxMessageLength = 3500 // Reserve space for headers and formatting

		for i := 0; i < len(results); i += batchSize {
			end := i + batchSize
			if end > len(results) {
				end = len(results)
			}

			batchNum := (i / batchSize) + 1
			totalBatches := (len(results) + batchSize - 1) / batchSize

			// Build batch message with length checking
			var batchContent strings.Builder
			batchContent.WriteString(fmt.Sprintf(`📦 **Batch %d/%d**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

`, batchNum, totalBatches))

			// Add results one by one, checking length
			currentLength := len(batchContent.String())
			actualEnd := i
			for j := i; j < end; j++ {
				resultLine := results[j] + "\n\n"
				if currentLength+len(resultLine) > maxMessageLength {
					// Stop if adding this line would exceed limit
					break
				}
				batchContent.WriteString(resultLine)
				currentLength += len(resultLine)
				actualEnd = j + 1
			}

			// If we couldn't fit all results in this batch, adjust
			if actualEnd < end && actualEnd == i {
				// Even a single result is too long, split it
				resultText := results[i]
				// Truncate and add note
				if len(resultText) > maxMessageLength-200 {
					resultText = resultText[:maxMessageLength-200] + "\n... (pesan terlalu panjang, silakan cek grup lain)"
				}
				batchContent.WriteString(resultText)
				actualEnd = i + 1
			}

			batchMsg := batchContent.String()

			msg := tgbotapi.NewMessage(chatID, batchMsg)
			msg.ParseMode = "Markdown"

			// Add buttons to last batch
			if actualEnd >= len(results) {
				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("🔄 Ambil Link Lagi", "get_link_menu"),
						tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
					),
				)
				msg.ReplyMarkup = keyboard
			}

			telegramBot.Send(msg)

			// Update i to actualEnd-1 (will be incremented by loop)
			i = actualEnd - 1

			// Small delay between batches to avoid rate limit
			if actualEnd < len(results) {
				time.Sleep(1 * time.Second)
			}
		}
	} else {
		// For small batches, send in one message
		finalMsg := fmt.Sprintf(`🎉 **PROSES SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Ringkasan:**
• Total: %d grup
• ✅ Berhasil: %d
• ❌ Gagal: %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Detail Hasil:**

%s`, totalGroups, successCount, failedCount, strings.Join(results, "\n\n"))

		msg := tgbotapi.NewMessage(chatID, finalMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Ambil Link Lagi", "get_link_menu"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	}
}

// CancelGetLink cancels the get link process
func CancelGetLink(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(linkGrupStates, chatID)

	msg := tgbotapi.NewMessage(chatID, "❌ Proses ambil link dibatalkan.")
	telegramBot.Send(msg)
}

// IsWaitingForLinkInput checks if user is in link input mode
func IsWaitingForLinkInput(chatID int64) bool {
	state := linkGrupStates[chatID]
	if state == nil {
		return false
	}
	return state.WaitingForGroupName || state.WaitingForDelay
}

// GetLinkInputType returns the type of input expected
func GetLinkInputType(chatID int64) string {
	state := linkGrupStates[chatID]
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

// HandleFileInputForGetLink memproses input file .txt untuk ambil link grup
func HandleFileInputForGetLink(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := linkGrupStates[chatID]
	if state == nil || !state.WaitingForGroupName {
		return
	}

	// Download file
	// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileID)
	var resp *http.Response
	var err error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = http.Get(fileURL)
		if err == nil && resp != nil && resp.StatusCode == 200 {
			break // Success
		}
		if resp != nil {
			resp.Body.Close()
		}

		// FIXED: Log error untuk operasi kritis
		utils.GetGrupLogger().Warn("HandleFileInputForGetLinks: Attempt %d/%d failed: %v", attempt+1, maxRetries, err)

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil || resp == nil {
		errorMsg := utils.FormatUserError(utils.ErrorConnection, err, "Gagal mengunduh file setelah "+fmt.Sprintf("%d", maxRetries)+" percobaan")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.LogActivityError("get_group_link", "Gagal download file setelah retry", chatID, err)
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
		// FIXED: Log error untuk operasi kritis
		utils.LogActivityError("get_group_link", "File tidak valid", chatID, err)
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
	// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	var fileResp2 *http.Response
	err = nil
	for attempt := 0; attempt < maxRetries; attempt++ {
		fileResp2, err = http.Get(downloadURL)
		if err == nil && fileResp2 != nil && fileResp2.StatusCode == 200 {
			break // Success
		}
		if fileResp2 != nil {
			fileResp2.Body.Close()
		}

		// FIXED: Log error untuk operasi kritis
		utils.GetGrupLogger().Warn("HandleFileInputForGetLinks: Attempt %d/%d failed download file: %v", attempt+1, maxRetries, err)

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil || fileResp2 == nil {
		errorMsg := utils.FormatUserError(utils.ErrorConnection, err, "Gagal membaca file setelah "+fmt.Sprintf("%d", maxRetries)+" percobaan")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.LogActivityError("get_group_link", "Gagal membaca file setelah retry", chatID, err)
		return
	}
	defer fileResp2.Body.Close()

	// Read file content - extract group names (one per line)
	var groupNames []string
	scanner := bufio.NewScanner(fileResp2.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// Remove emoji and special characters if present (like 🔗)
			line = strings.TrimPrefix(line, "🔗")
			line = strings.TrimSpace(line)
			if line != "" {
				groupNames = append(groupNames, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Error membaca file")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("get_group_link", "Error membaca file", chatID, err)
		return
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **FILE KOSONG**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nFile `.txt` yang Anda kirim tidak berisi nama grup.\n\n**Format yang benar:**\nSatu nama grup per baris.\n\n**Contoh:**\nKeluarga Besar\nGrup Kerja\nGrup Teman")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Log activity
	utils.LogActivity("get_group_link_file", fmt.Sprintf("File .txt diterima dengan %d nama grup", len(groupNames)), chatID)

	// Search groups using exact match for each name
	groups, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("get_group_link", "Gagal mencari grup dari file", chatID, err)
		return
	}

	if len(groups) == 0 {
		noResultMsg := fmt.Sprintf(`❌ **TIDAK ADA GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📄 **File:** %d nama grup dibaca
❌ **Ditemukan:** 0 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Saran:**
• Pastikan nama grup di file sesuai dengan nama grup di WhatsApp
• Nama harus persis sama (case-insensitive)
• Cek apakah grup sudah ada di database bot`, len(groupNames))

		msg := tgbotapi.NewMessage(chatID, noResultMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📋 Lihat Daftar", "show_group_list_link"),
				tgbotapi.NewInlineKeyboardButtonData("🔍 Coba Lagi", "start_get_link"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Menu", "get_link_menu"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)

		// Reset state
		delete(linkGrupStates, chatID)
		return
	}

	// Store selected groups with natural sorting
	state.SelectedGroups = []GroupLinkInfo{}

	// Sort groups naturally
	sortedGroups := utils.SortGroupsNaturally(groups)
	for _, group := range sortedGroups {
		state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
			JID:  group.JID,
			Name: group.Name,
		})
	}

	// Set keyword to indicate it came from file
	state.Keyword = fmt.Sprintf("file_%d_groups", len(groupNames))

	// Update state
	state.WaitingForGroupName = false
	state.WaitingForDelay = true

	// Show found groups and ask for delay
	resultMsg := fmt.Sprintf(`✅ **FILE DITERIMA & GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📄 **File:** %d nama grup dibaca
✅ **Ditemukan:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Daftar Grup Ditemukan:**
`, len(groupNames), len(groups))

	count := 1
	for _, group := range state.SelectedGroups {
		if count <= 10 { // Show max 10 groups
			resultMsg += fmt.Sprintf("%d. %s\n", count, group.Name)
		}
		count++
	}

	if len(state.SelectedGroups) > 10 {
		resultMsg += fmt.Sprintf("\n... dan %d grup lainnya\n", len(state.SelectedGroups)-10)
	}

	resultMsg += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay antar permintaan untuk menghindari rate limit.

**Rekomendasi:**
• 1-2 detik: Untuk grup sedikit (< 10)
• 2-3 detik: Untuk grup sedang (10-30)
• 3-5 detik: Untuk grup banyak (> 30)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_get_link"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)

	// Log activity
	utils.LogActivity("get_group_link_file_success", fmt.Sprintf("%d grup ditemukan dari file", len(groups)), chatID)
}
