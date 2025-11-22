package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
)

// JoinGroupState manages state for join group feature
type JoinGroupState struct {
	WaitingForLink  bool
	WaitingForDelay bool
	GroupLinks      []string
	DelaySeconds    int
}

var joinGroupStates = make(map[int64]*JoinGroupState)

// ShowJoinGroupMenu menampilkan menu join grup otomatis
func ShowJoinGroupMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `🚪 **JOIN GRUP OTOMATIS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan membuat bot bergabung ke grup WhatsApp secara otomatis menggunakan link undangan.

**📋 Cara Input:**

**Opsi 1: Input Text (Multi-line)**
Ketik link grup, setiap baris = 1 link

**Opsi 2: Upload File .txt**
Kirim file .txt yang berisi link grup (satu per baris)

**Contoh Link:**
https://chat.whatsapp.com/ABC123
https://chat.whatsapp.com/XYZ789

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Link harus valid dan masih aktif
• Bot harus belum bergabung ke grup tersebut
• Delay membantu menghindari rate limit
• Proses mungkin memakan waktu untuk banyak link

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik link grup atau kirim file .txt...`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowJoinGroupMenuEdit menampilkan menu (EDIT, NO SPAM!)
func ShowJoinGroupMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `🚪 **JOIN GRUP OTOMATIS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan membuat bot bergabung ke grup WhatsApp secara otomatis menggunakan link undangan.

**📋 Cara Input:**

**Opsi 1: Input Text (Multi-line)**
Ketik link grup, setiap baris = 1 link

**Opsi 2: Upload File .txt**
Kirim file .txt yang berisi link grup (satu per baris)

**Contoh Link:**
https://chat.whatsapp.com/ABC123
https://chat.whatsapp.com/XYZ789

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Link harus valid dan masih aktif
• Bot harus belum bergabung ke grup tersebut
• Delay membantu menghindari rate limit
• Proses mungkin memakan waktu untuk banyak link

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik link grup atau kirim file .txt...`

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	editMsg.ReplyMarkup = &keyboard

	telegramBot.Send(editMsg)
}

// StartJoinGroupProcess memulai proses join grup
func StartJoinGroupProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	joinGroupStates[chatID] = &JoinGroupState{
		WaitingForLink:  true,
		WaitingForDelay: false,
		GroupLinks:      []string{},
		DelaySeconds:    0,
	}

	promptMsg := `🔗 **MASUKKAN LINK GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Cara Input:**

**Opsi 1: Input Text (Multi-line)**
Ketik link grup, setiap baris = 1 link

**Contoh:**
https://chat.whatsapp.com/ABC123
https://chat.whatsapp.com/XYZ789
https://chat.whatsapp.com/DEF456

**Opsi 2: Upload File .txt**
Kirim file .txt yang berisi link grup (satu per baris)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik link grup atau kirim file .txt...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleLinkInputForJoin memproses input link grup (text)
func HandleLinkInputForJoin(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := joinGroupStates[chatID]
	if state == nil || !state.WaitingForLink {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Link grup tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Extract links from input (supports format with group names)
	links := extractLinksFromText(input)

	if len(links) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada link yang valid!\n\nPastikan input berisi link WhatsApp dengan format:\nhttps://chat.whatsapp.com/...\n\nContoh format yang didukung:\n• Nama Grup\n🔗 https://chat.whatsapp.com/...\n• Atau langsung link saja")
		telegramBot.Send(errorMsg)
		return
	}

	state.GroupLinks = links
	state.WaitingForLink = false
	state.WaitingForDelay = true

	// Ask for delay
	askForDelayInput(chatID, telegramBot, state)
}

// HandleFileInputForJoin memproses input file .txt
func HandleFileInputForJoin(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := joinGroupStates[chatID]
	if state == nil || !state.WaitingForLink {
		return
	}

	// Download file
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileID)
	resp, err := http.Get(fileURL)
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
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
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Gagal mengambil informasi file!")
		telegramBot.Send(errorMsg)
		return
	}

	// Download file content
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	fileResp2, err := http.Get(downloadURL)
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		return
	}
	defer fileResp2.Body.Close()

	// Read file content
	fileContent := ""
	scanner := bufio.NewScanner(fileResp2.Body)
	for scanner.Scan() {
		fileContent += scanner.Text() + "\n"
	}

	if err := scanner.Err(); err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error membaca file: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	// Extract links from file content (supports format with group names)
	links := extractLinksFromText(fileContent)

	if len(links) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada link yang valid di file!\n\nPastikan file berisi link WhatsApp dengan format:\nhttps://chat.whatsapp.com/...\n\nContoh format yang didukung:\n• Nama Grup\n🔗 https://chat.whatsapp.com/...\n• Atau langsung link saja")
		telegramBot.Send(errorMsg)
		return
	}

	state.GroupLinks = links
	state.WaitingForLink = false
	state.WaitingForDelay = true

	// Confirm and ask for delay
	confirmMsg := fmt.Sprintf(`✅ **FILE DITERIMA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📁 **Total link:** %d link

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, len(links))

	confirm := tgbotapi.NewMessage(chatID, confirmMsg)
	confirm.ParseMode = "Markdown"
	telegramBot.Send(confirm)

	// Ask for delay
	askForDelayInput(chatID, telegramBot, state)
}

// askForDelayInput meminta input delay
func askForDelayInput(chatID int64, telegramBot *tgbotapi.BotAPI, state *JoinGroupState) {
	promptMsg := fmt.Sprintf(`⏱️ **TENTUKAN DELAY**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total link:** %d link

Masukkan berapa detik delay antar join grup.

**Rekomendasi:**
• 2-3 detik: < 10 link
• 3-5 detik: 10-30 link
• 5-10 detik: > 30 link

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 3)`, len(state.GroupLinks))

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_join_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInputForJoin memproses input delay
func HandleDelayInputForJoin(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := joinGroupStates[chatID]
	if state == nil || !state.WaitingForDelay {
		return
	}

	delay, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || delay < 0 || delay > 300 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Delay harus antara 0-300 detik!")
		telegramBot.Send(errorMsg)
		return
	}

	state.DelaySeconds = delay
	state.WaitingForDelay = false

	// Note: ProcessJoinGroups will be called with client from handler
	// State is ready, waiting for client to process
}

// ProcessJoinGroups memproses join grup
func ProcessJoinGroups(state *JoinGroupState, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	if client == nil || client.Store.ID == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(msg)
		return
	}

	// Start message
	startMsg := fmt.Sprintf(`✅ **MEMULAI PROSES**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Link:** %d link
⏱️ **Delay:** %d detik/link
⏳ **Estimasi:** ~%d detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 Memproses join grup...`, len(state.GroupLinks), state.DelaySeconds, len(state.GroupLinks)*state.DelaySeconds)

	msg := tgbotapi.NewMessage(chatID, startMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	totalLinks := len(state.GroupLinks)
	successCount := 0
	failedCount := 0
	var failedLinks []string
	var successLinks []string

	var progressMsgSent *tgbotapi.Message

	for i, link := range state.GroupLinks {
		// MEDIUM FIX: Ambil active client di setiap iterasi (join bisa timeout!)
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessJoinGroups", i, len(state.GroupLinks))
		if shouldStop {
			// Client disconnect - stop proses
			disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada link %d/%d\n\n✅ Berhasil: %d\n❌ Gagal: %d", i+1, len(state.GroupLinks), successCount, failedCount)
			notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
			notifMsg.ParseMode = "Markdown"
			telegramBot.Send(notifMsg)
			break
		}

		// Join group using link dengan validClient
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel() // FIXED: Use defer to ensure cancellation
		jid, err := validClient.JoinGroupWithLink(ctx, link)

		if err != nil {
			failedCount++
			errorDetail := "Tidak dapat bergabung"
			if strings.Contains(err.Error(), "already in group") {
				errorDetail = "Sudah bergabung"
			} else if strings.Contains(err.Error(), "expired") {
				errorDetail = "Link kedaluwarsa"
			} else if strings.Contains(err.Error(), "not found") {
				errorDetail = "Link tidak valid"
			}
			failedLinks = append(failedLinks, fmt.Sprintf("❌ %s\n   💡 %s", link, errorDetail))
		} else {
			successCount++
			successLinks = append(successLinks, fmt.Sprintf("✅ %s\n   🆔 %s", link, jid.String()))
		}

		// Show progress
		if totalLinks > 1 {
			progressPercent := ((i + 1) * 100) / totalLinks
			progressBar := generateProgressBar(progressPercent)

			progressMsg := fmt.Sprintf(`⏳ **PROGRESS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s **%d%%**
📊 **Diproses:** %d/%d link
✅ **Berhasil:** %d
❌ **Gagal:** %d
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⏳ Sedang memproses...`, progressBar, progressPercent, i+1, totalLinks, successCount, failedCount)

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

		// Delay between joins
		if i < totalLinks-1 {
			time.Sleep(time.Duration(state.DelaySeconds) * time.Second)
		}
	}

	// Final summary
	summaryMsg := fmt.Sprintf(`🎉 **SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 **RINGKASAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **Total Link:** %d link
✅ **Berhasil:** %d link
❌ **Gagal:** %d link
⏱️ **Delay:** %d detik/link
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, totalLinks, successCount, failedCount, state.DelaySeconds)

	if successCount > 0 {
		summaryMsg += fmt.Sprintf("\n\n**✅ Link yang Berhasil:**\n\n")
		for i, success := range successLinks {
			if i < 20 { // Limit display
				summaryMsg += success + "\n\n"
			}
		}
		if len(successLinks) > 20 {
			summaryMsg += fmt.Sprintf("... dan %d link lainnya\n", len(successLinks)-20)
		}
	}

	if failedCount > 0 {
		summaryMsg += fmt.Sprintf("\n\n**❌ Link yang Gagal:**\n\n")
		for i, failed := range failedLinks {
			if i < 20 { // Limit display
				summaryMsg += failed + "\n\n"
			}
		}
		if len(failedLinks) > 20 {
			summaryMsg += fmt.Sprintf("... dan %d link lainnya\n", len(failedLinks)-20)
		}
	}

	finalMsg := tgbotapi.NewMessage(chatID, summaryMsg)
	finalMsg.ParseMode = "Markdown"
	telegramBot.Send(finalMsg)

	// Clear state
	delete(joinGroupStates, chatID)
}

// isValidWhatsAppLink memvalidasi format link WhatsApp
func isValidWhatsAppLink(link string) bool {
	link = strings.TrimSpace(link)
	return strings.HasPrefix(link, "https://chat.whatsapp.com/") ||
		strings.HasPrefix(link, "http://chat.whatsapp.com/")
}

// extractLinksFromText mengekstrak link WhatsApp dari teks
// Mendukung berbagai format:
// - Link langsung: https://chat.whatsapp.com/...
// - Nama grup + link: Nama Grup\n🔗 https://chat.whatsapp.com/...
// - Link di tengah teks: ...https://chat.whatsapp.com/...
func extractLinksFromText(text string) []string {
	var links []string

	// Split by newline to process line by line
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if line contains WhatsApp link
		// Look for https://chat.whatsapp.com/ or http://chat.whatsapp.com/
		linkStart := strings.Index(line, "https://chat.whatsapp.com/")
		if linkStart == -1 {
			linkStart = strings.Index(line, "http://chat.whatsapp.com/")
		}

		if linkStart != -1 {
			// Extract link from this position
			// Link typically ends at space, newline, or end of string
			link := line[linkStart:]

			// Trim any trailing characters (space, emoji, etc.)
			// Find where the link actually ends (usually after the invite code)
			// WhatsApp invite codes are typically alphanumeric and can contain some special chars
			endPos := len(link)
			for i, char := range link {
				if char == ' ' || char == '\n' || char == '\r' || char == '\t' {
					endPos = i
					break
				}
				// Stop at certain special characters that shouldn't be in URL
				if char == ')' || char == ']' || char == '}' || char == '<' {
					// Check if this char is actually part of the link or after it
					if i > 0 && link[i-1] != '/' && link[i-1] != '-' && link[i-1] != '_' {
						endPos = i
						break
					}
				}
			}

			extractedLink := strings.TrimSpace(link[:endPos])

			// Validate extracted link
			if isValidWhatsAppLink(extractedLink) {
				// Remove duplicates
				isDuplicate := false
				for _, existingLink := range links {
					if existingLink == extractedLink {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					links = append(links, extractedLink)
				}
			}
		}
	}

	return links
}

// CancelJoinGroup membatalkan proses join grup
func CancelJoinGroup(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(joinGroupStates, chatID)

	msg := tgbotapi.NewMessage(chatID, "❌ Proses join grup otomatis dibatalkan.")
	telegramBot.Send(msg)
}

// IsWaitingForJoinGroupInput checks if user is waiting to input
func IsWaitingForJoinGroupInput(chatID int64) bool {
	state := joinGroupStates[chatID]
	return state != nil && (state.WaitingForLink || state.WaitingForDelay)
}

// GetJoinGroupInputType returns the current input type
func GetJoinGroupInputType(chatID int64) string {
	state := joinGroupStates[chatID]
	if state == nil {
		return ""
	}

	if state.WaitingForLink {
		return "link"
	}
	if state.WaitingForDelay {
		return "delay"
	}

	return ""
}

// GetJoinGroupState returns the join group state for a chat
func GetJoinGroupState(chatID int64) *JoinGroupState {
	return joinGroupStates[chatID]
}
