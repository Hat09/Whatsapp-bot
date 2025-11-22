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
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// LeaveGroupState manages state for leave group feature
type LeaveGroupState struct {
	WaitingForGroupName    bool
	WaitingForDelay        bool
	WaitingForMode         bool
	WaitingForNotification bool
	SelectedGroups         []GroupLinkInfo
	DelaySeconds           int
	LeaveMode              string // "one_by_one" or "batch"
	NotificationMessage    string // Optional message to send before leaving
	SendNotification       bool   // Whether to send notification
}

var leaveGroupStates = make(map[int64]*LeaveGroupState)

// ShowLeaveGroupMenu menampilkan menu keluar grup otomatis
func ShowLeaveGroupMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `🚪 **KELUAR GRUP OTOMATIS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan membuat bot keluar dari grup WhatsApp secara otomatis.

**📋 Cara Input:**

**Opsi 1: Input Text (Multi-line)**
Ketik nama grup, setiap baris = 1 grup

**Opsi 2: Upload File .txt**
Kirim file .txt yang berisi nama grup (satu per baris)

**Contoh:**
GRUP A
GRUP B
GRUP C

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus sudah bergabung ke grup tersebut
• Delay membantu menghindari rate limit
• Proses mungkin memakan waktu untuk banyak grup
• Setelah keluar, bot tidak bisa menerima pesan dari grup tersebut

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nama grup atau kirim file .txt...`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_leave_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowLeaveGroupMenuEdit menampilkan menu (EDIT, NO SPAM!)
func ShowLeaveGroupMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `🚪 **KELUAR GRUP OTOMATIS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan membuat bot keluar dari grup WhatsApp secara otomatis.

**📋 Cara Input:**

**Opsi 1: Input Text (Multi-line)**
Ketik nama grup, setiap baris = 1 grup

**Opsi 2: Upload File .txt**
Kirim file .txt yang berisi nama grup (satu per baris)

**Contoh:**
GRUP A
GRUP B
GRUP C

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus sudah bergabung ke grup tersebut
• Delay membantu menghindari rate limit
• Proses mungkin memakan waktu untuk banyak grup
• Setelah keluar, bot tidak bisa menerima pesan dari grup tersebut

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nama grup atau kirim file .txt...`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_leave_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartLeaveGroupProcess memulai proses keluar grup
func StartLeaveGroupProcess(chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := &LeaveGroupState{
		WaitingForGroupName:    true,
		WaitingForDelay:        false,
		WaitingForMode:         false,
		WaitingForNotification: false,
		SelectedGroups:         []GroupLinkInfo{},
		DelaySeconds:           0,
		LeaveMode:              "",
		NotificationMessage:    "",
		SendNotification:       false,
	}
	leaveGroupStates[chatID] = state

	promptMsg := `📋 **INPUT NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Ketik nama grup yang ingin Anda keluar (satu per baris), atau kirim file .txt yang berisi nama grup.

**Contoh:**
GRUP A
GRUP B
GRUP C

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nama grup atau kirim file .txt...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForLeave memproses input nama grup untuk keluar grup
func HandleGroupNameInputForLeave(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := leaveGroupStates[chatID]
	if state == nil || !state.WaitingForGroupName {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nama grup tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Parse multi-line group names
	lines := strings.Split(input, "\n")
	groupNames := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			groupNames = append(groupNames, line)
		}
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada nama grup yang valid!")
		telegramBot.Send(errorMsg)
		return
	}

	// Search groups
	groupsMap, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	if len(groupsMap) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang ditemukan!")
		telegramBot.Send(errorMsg)
		return
	}

	// Store selected groups
	state.SelectedGroups = []GroupLinkInfo{}
	for jid, name := range groupsMap {
		state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
			JID:  jid,
			Name: name,
		})
	}

	// Update state
	state.WaitingForGroupName = false
	state.WaitingForMode = true

	// Ask for mode selection
	modeMsg := fmt.Sprintf(`✅ **GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 **Total Grup:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔀 **PILIH MODE KELUAR GRUP**

**1️⃣ Mode 1/1 (Satu Per Satu)**
• Keluar dari grup satu per satu
• Ada delay antar grup
• Lebih aman dari rate limit
• Cocok untuk banyak grup

**2️⃣ Mode Batch (Sekaligus)**
• Keluar dari semua grup sekaligus
• Lebih cepat
• Cocok untuk sedikit grup (< 10)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih mode dengan klik tombol di bawah`, len(state.SelectedGroups))

	msg := tgbotapi.NewMessage(chatID, modeMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1️⃣ Mode 1/1", "leave_mode_one_by_one"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("2️⃣ Mode Batch", "leave_mode_batch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleFileInputForLeave handles file upload untuk leave group
func HandleFileInputForLeave(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := leaveGroupStates[chatID]
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
		errorMsg := utils.FormatUserError(utils.ErrorConnection, err, "Gagal mengunduh konten file")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}
	defer fileResp2.Body.Close()

	// Parse .txt file
	var groupNames []string
	scanner := bufio.NewScanner(fileResp2.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			groupNames = append(groupNames, line)
		}
	}

	if err := scanner.Err(); err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error membaca file: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ File .txt tidak berisi nama grup!")
		telegramBot.Send(errorMsg)
		return
	}

	// Search groups
	groupsMap, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return
	}

	if len(groupsMap) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang ditemukan dari file!")
		telegramBot.Send(errorMsg)
		return
	}

	// Store selected groups
	state.SelectedGroups = []GroupLinkInfo{}
	for jid, name := range groupsMap {
		state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
			JID:  jid,
			Name: name,
		})
	}

	state.WaitingForGroupName = false
	state.WaitingForMode = true

	// Ask for mode selection
	modeMsg := fmt.Sprintf(`✅ **FILE DITERIMA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 **Total Grup:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔀 **PILIH MODE KELUAR GRUP**

**1️⃣ Mode 1/1 (Satu Per Satu)**
• Keluar dari grup satu per satu
• Ada delay antar grup
• Lebih aman dari rate limit
• Cocok untuk banyak grup

**2️⃣ Mode Batch (Sekaligus)**
• Keluar dari semua grup sekaligus
• Lebih cepat
• Cocok untuk sedikit grup (< 10)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih mode dengan klik tombol di bawah`, len(state.SelectedGroups))

	msg := tgbotapi.NewMessage(chatID, modeMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1️⃣ Mode 1/1", "leave_mode_one_by_one"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("2️⃣ Mode Batch", "leave_mode_batch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleModeInputForLeave memproses pemilihan mode keluar grup
func HandleModeInputForLeave(mode string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := leaveGroupStates[chatID]
	if state == nil || !state.WaitingForMode {
		return
	}

	state.LeaveMode = mode
	state.WaitingForMode = false

	if mode == "one_by_one" {
		// For one_by_one mode, ask for delay
		state.WaitingForDelay = true

		delayMsg := `⏱️ **TENTUKAN DELAY**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Masukkan berapa detik delay antar grup saat keluar dari grup.

**Rekomendasi:**
• 2-3 detik: < 10 grup
• 3-5 detik: 10-50 grup
• 5-10 detik: > 50 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 3)`

		msg := tgbotapi.NewMessage(chatID, delayMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	} else {
		// For batch mode, ask for notification
		state.WaitingForNotification = true

		notificationMsg := `📢 **NOTIFIKASI SEBELUM KELUAR**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Apakah Anda ingin mengirim pesan notifikasi ke grup sebelum bot keluar?

**Contoh pesan:**
"Bot akan keluar dari grup ini. Terima kasih!"

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih opsi di bawah`

		msg := tgbotapi.NewMessage(chatID, notificationMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Kirim Notifikasi", "leave_notification_yes"),
				tgbotapi.NewInlineKeyboardButtonData("❌ Tidak", "leave_notification_no"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	}
}

// HandleDelayInputForLeave memproses input delay untuk leave group
func HandleDelayInputForLeave(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := leaveGroupStates[chatID]
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
	state.WaitingForNotification = true

	// Ask for notification
	notificationMsg := `📢 **NOTIFIKASI SEBELUM KELUAR**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Apakah Anda ingin mengirim pesan notifikasi ke grup sebelum bot keluar?

**Contoh pesan:**
"Bot akan keluar dari grup ini. Terima kasih!"

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih opsi di bawah`

	msg := tgbotapi.NewMessage(chatID, notificationMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Kirim Notifikasi", "leave_notification_yes"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Tidak", "leave_notification_no"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleNotificationChoiceForLeave memproses pilihan notifikasi
func HandleNotificationChoiceForLeave(sendNotification bool, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := leaveGroupStates[chatID]
	if state == nil || !state.WaitingForNotification {
		return
	}

	state.SendNotification = sendNotification
	state.WaitingForNotification = false

	if sendNotification {
		// Ask for notification message
		msgPrompt := `📝 **PESAN NOTIFIKASI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Ketik pesan yang ingin dikirim ke grup sebelum bot keluar.

**Contoh:**
Bot akan keluar dari grup ini. Terima kasih!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik pesan notifikasi...`

		msg := tgbotapi.NewMessage(chatID, msgPrompt)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_leave_group"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	} else {
		// No notification, start processing
		state.NotificationMessage = ""
		startProcessing(chatID, telegramBot)
	}
}

// HandleNotificationMessageInputForLeave memproses input pesan notifikasi
func HandleNotificationMessageInputForLeave(message string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := leaveGroupStates[chatID]
	if state == nil {
		return
	}

	message = strings.TrimSpace(message)
	if message == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Pesan notifikasi tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	state.NotificationMessage = message
	startProcessing(chatID, telegramBot)
}

// startProcessing memulai proses keluar grup
func startProcessing(chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := leaveGroupStates[chatID]
	if state == nil {
		return
	}

	// Get client
	client := GetWhatsAppClient()
	if client == nil {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(errorMsg)
		delete(leaveGroupStates, chatID)
		return
	}

	// Start message
	modeText := "1/1 (Satu Per Satu)"
	if state.LeaveMode == "batch" {
		modeText = "Batch (Sekaligus)"
	}

	notificationText := "Tidak"
	if state.SendNotification {
		notificationText = fmt.Sprintf("Ya: \"%s\"", state.NotificationMessage)
	}

	startMsg := fmt.Sprintf(`✅ **KONFIGURASI SELESAI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Grup:** %d grup
🔀 **Mode:** %s
⏱️ **Delay:** %d detik/grup
📢 **Notifikasi:** %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 **Memulai proses keluar grup...**

⏳ Mohon tunggu, proses sedang berjalan...`, len(state.SelectedGroups), modeText, state.DelaySeconds, notificationText)

	msg := tgbotapi.NewMessage(chatID, startMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Process in goroutine
	go ProcessLeaveGroups(state, chatID, client, telegramBot)
}

// ProcessLeaveGroups memproses keluar dari grup
func ProcessLeaveGroups(state *LeaveGroupState, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	if client == nil || client.Store.ID == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(msg)
		delete(leaveGroupStates, chatID)
		return
	}

	// Get own JID (to leave the group)
	ownJID := client.Store.ID.ToNonAD()
	if ownJID == (types.JID{}) {
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mendapatkan JID bot sendiri.")
		telegramBot.Send(msg)
		delete(leaveGroupStates, chatID)
		return
	}

	totalGroups := len(state.SelectedGroups)
	successCount := 0
	failedCount := 0
	var failedGroups []string
	var successGroups []string

	var progressMsgSent *tgbotapi.Message

	if state.LeaveMode == "batch" {
		// Batch mode: Leave all groups at once
		// Collect all group JIDs and own JID
		groupJIDs := []types.JID{}
		groupNames := []string{}

		for _, group := range state.SelectedGroups {
			groupJID, err := parseJIDFromString(group.JID)
			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (invalid JID)", group.Name))
				continue
			}
			groupJIDs = append(groupJIDs, groupJID)
			groupNames = append(groupNames, group.Name)
		}

		if len(groupJIDs) == 0 {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup valid yang ditemukan!")
			telegramBot.Send(errorMsg)
			delete(leaveGroupStates, chatID)
			return
		}

		// Validate client before batch operation
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessLeaveGroups", 0, totalGroups)
		if shouldStop {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(errorMsg)
			delete(leaveGroupStates, chatID)
			return
		}

		// Get own JID
		currentOwnJID := validClient.Store.ID.ToNonAD()
		if currentOwnJID == (types.JID{}) {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Gagal mendapatkan JID bot sendiri.")
			telegramBot.Send(errorMsg)
			delete(leaveGroupStates, chatID)
			return
		}

		// Send notification messages if enabled (one by one before batch leave)
		if state.SendNotification && state.NotificationMessage != "" {
			for i, groupJID := range groupJIDs {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel() // FIXED: Use defer to ensure cancellation
				_, err := validClient.SendMessage(ctx, groupJID, &waProto.Message{
					Conversation: proto.String(state.NotificationMessage),
				})

				if err == nil {
					// Small delay between notifications
					if i < len(groupJIDs)-1 {
						time.Sleep(500 * time.Millisecond)
					}
				}
			}

			// Wait a bit after all notifications before leaving
			time.Sleep(1 * time.Second)
		}

		// Leave all groups at once (batch operation)
		// Process each group individually but without delay
		for i, groupJID := range groupJIDs {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := validClient.UpdateGroupParticipants(ctx, groupJID, []types.JID{currentOwnJID}, whatsmeow.ParticipantChangeRemove)
			cancel()

			groupName := groupNames[i]
			if err != nil {
				failedCount++
				errorDetail := "Tidak dapat keluar dari grup"
				if strings.Contains(err.Error(), "not in group") || strings.Contains(err.Error(), "not found") {
					errorDetail = "Bot tidak berada di grup"
				} else if strings.Contains(err.Error(), "not-authorized") {
					errorDetail = "Tidak diizinkan keluar"
				}
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s\n   💡 %s", groupName, errorDetail))
			} else {
				successCount++
				successGroups = append(successGroups, fmt.Sprintf("✅ %s", groupName))
			}
		}
	} else {
		// One by one mode: Leave groups one by one with delay
		for i, group := range state.SelectedGroups {
			// MEDIUM FIX: Ambil active client di setiap iterasi!
			validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessLeaveGroups", i, totalGroups)
			if shouldStop {
				// Client disconnect - stop proses
				disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada grup %d/%d\n\n✅ Berhasil: %d\n❌ Gagal: %d", i+1, totalGroups, successCount, failedCount)
				notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
				notifMsg.ParseMode = "Markdown"
				telegramBot.Send(notifMsg)
				break
			}

			// Get own JID from valid client (might have changed)
			currentOwnJID := validClient.Store.ID.ToNonAD()
			if currentOwnJID == (types.JID{}) {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (gagal mendapatkan JID)", group.Name))
				continue
			}

			// Parse group JID
			groupJID, err := parseJIDFromString(group.JID)
			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (invalid JID)", group.Name))
				continue
			}

			// Send notification message if enabled
			if state.SendNotification && state.NotificationMessage != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel() // FIXED: Use defer to ensure cancellation
				// FIXED: err tidak digunakan, tapi tidak perlu di-handle karena hanya notification
				_, _ = validClient.SendMessage(ctx, groupJID, &waProto.Message{
					Conversation: proto.String(state.NotificationMessage),
				})

				// Small delay after sending notification before leaving
				time.Sleep(1 * time.Second)
			}

			// Leave group using UpdateGroupParticipants with ParticipantChangeRemove
			// Use own JID to leave the group
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel() // FIXED: Use defer to ensure cancellation
			_, err = validClient.UpdateGroupParticipants(ctx, groupJID, []types.JID{currentOwnJID}, whatsmeow.ParticipantChangeRemove)
			// FIXED: Handle error untuk leave group operation
			if err != nil {
				utils.GetGrupLogger().Warn("ProcessLeaveGroups: Failed to leave group %s: %v", group.Name, err)
			}

			if err != nil {
				failedCount++
				errorDetail := "Tidak dapat keluar dari grup"
				if strings.Contains(err.Error(), "not in group") || strings.Contains(err.Error(), "not found") {
					errorDetail = "Bot tidak berada di grup"
				} else if strings.Contains(err.Error(), "not-authorized") {
					errorDetail = "Tidak diizinkan keluar"
				}
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s\n   💡 %s", group.Name, errorDetail))
			} else {
				successCount++
				successGroups = append(successGroups, fmt.Sprintf("✅ %s", group.Name))
			}

			// Show progress
			if totalGroups > 1 {
				progressPercent := ((i + 1) * 100) / totalGroups
				progressBar := generateProgressBar(progressPercent)

				progressMsg := fmt.Sprintf(`⏳ **PROGRESS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s **%d%%**
�� **Diproses:** %d/%d grup
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

			// Delay between leaves (only for one_by_one mode)
			if state.LeaveMode == "one_by_one" && i < totalGroups-1 {
				time.Sleep(time.Duration(state.DelaySeconds) * time.Second)
			}
		}
	}

	// Delete progress message if exists
	if progressMsgSent != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMsgSent.MessageID)
		telegramBot.Request(deleteMsg)
	}

	// Final summary
	summaryMsg := fmt.Sprintf(`�� **SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
�� **RINGKASAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **Total Grup:** %d grup
✅ **Berhasil:** %d grup
❌ **Gagal:** %d grup
⏱️ **Delay:** %d detik/grup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, totalGroups, successCount, failedCount, state.DelaySeconds)

	if successCount > 0 {
		summaryMsg += "\n\n**✅ Grup yang Berhasil:**\n\n"
		for i, success := range successGroups {
			if i < 20 { // Limit display
				summaryMsg += success + "\n"
			}
		}
		if len(successGroups) > 20 {
			summaryMsg += fmt.Sprintf("\n... dan %d grup lainnya\n", len(successGroups)-20)
		}
	}

	if failedCount > 0 {
		summaryMsg += "\n\n**❌ Grup yang Gagal:**\n\n"
		for i, failed := range failedGroups {
			if i < 20 { // Limit display
				summaryMsg += failed + "\n"
			}
		}
		if len(failedGroups) > 20 {
			summaryMsg += fmt.Sprintf("\n... dan %d grup lainnya\n", len(failedGroups)-20)
		}
	}

	msg := tgbotapi.NewMessage(chatID, summaryMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Log activity
	utils.LogActivity("leave_group", fmt.Sprintf("Leave %d groups: %d success, %d failed", totalGroups, successCount, failedCount), chatID)

	// Clear state
	delete(leaveGroupStates, chatID)
}

// CancelLeaveGroup membatalkan proses leave group
func CancelLeaveGroup(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(leaveGroupStates, chatID)
	msg := tgbotapi.NewMessage(chatID, "❌ Proses keluar grup dibatalkan.")
	telegramBot.Send(msg)
}

// GetLeaveGroupState mendapatkan state untuk chatID tertentu
func GetLeaveGroupState(chatID int64) *LeaveGroupState {
	return leaveGroupStates[chatID]
}

// IsWaitingForLeaveGroupInput checks if user is waiting to input leave group-related data
func IsWaitingForLeaveGroupInput(chatID int64) bool {
	state := leaveGroupStates[chatID]
	return state != nil && (state.WaitingForGroupName || state.WaitingForDelay || (state.WaitingForNotification && state.SendNotification))
}

// GetLeaveGroupInputType returns the current input type
func GetLeaveGroupInputType(chatID int64) string {
	state := leaveGroupStates[chatID]
	if state == nil {
		return ""
	}

	if state.WaitingForGroupName {
		return "group_name"
	}
	if state.WaitingForDelay {
		return "delay"
	}
	if state.WaitingForNotification && state.SendNotification {
		return "notification_message"
	}

	return ""
}
