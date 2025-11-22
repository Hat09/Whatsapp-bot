package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// AdminState manages state for admin/unadmin feature
type AdminState struct {
	IsAdminMode      bool // true = promote to admin, false = demote from admin
	WaitingForGroups bool
	WaitingForDelay  bool
	WaitingForPhones bool
	SelectedGroups   []GroupLinkInfo
	PhoneNumbers     []string
	DelaySeconds     int
}

var adminStates = make(map[int64]*AdminState)

// ShowAdminMenu menampilkan menu auto admin
func ShowAdminMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `👑 **AUTO ADMIN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengangkat nomor menjadi admin di grup WhatsApp secara otomatis.

**📋 Alur Proses:**
1️⃣ Input nama grup (multi-line)
2️⃣ Input delay antar grup (dalam detik)
3️⃣ Input nomor yang akan diangkat menjadi admin (multi-line)
4️⃣ Program memproses permintaan

**⚠️ Catatan Penting:**
• Bot harus menjadi admin grup
• Nomor harus sudah menjadi anggota grup
• Delay membantu menghindari rate limit WhatsApp (jeda antar grup)
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik tombol di bawah untuk memulai...`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_admin_process"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowAdminMenuEdit menampilkan menu auto admin dengan EDIT
func ShowAdminMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `👑 **AUTO ADMIN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengangkat nomor menjadi admin di grup WhatsApp secara otomatis.

**📋 Alur Proses:**
1️⃣ Input nama grup (multi-line)
2️⃣ Input delay antar grup (dalam detik)
3️⃣ Input nomor yang akan diangkat menjadi admin (multi-line)
4️⃣ Program memproses permintaan

**⚠️ Catatan Penting:**
• Bot harus menjadi admin grup
• Nomor harus sudah menjadi anggota grup
• Delay membantu menghindari rate limit WhatsApp (jeda antar grup)
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik tombol di bawah untuk memulai...`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_admin_process"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowUnadminMenu menampilkan menu auto unadmin
func ShowUnadminMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `👤 **AUTO UNADMIN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan menurunkan admin menjadi member di grup WhatsApp secara otomatis.

**📋 Alur Proses:**
1️⃣ Input nama grup (multi-line)
2️⃣ Input delay antar grup (dalam detik)
3️⃣ Input nomor yang akan diturunkan dari admin (multi-line)
4️⃣ Program memproses permintaan

**⚠️ Catatan Penting:**
• Bot harus menjadi admin grup
• Nomor harus sudah menjadi admin di grup
• Delay membantu menghindari rate limit WhatsApp (jeda antar grup)
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik tombol di bawah untuk memulai...`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_unadmin_process"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowUnadminMenuEdit menampilkan menu auto unadmin dengan EDIT
func ShowUnadminMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `👤 **AUTO UNADMIN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan menurunkan admin menjadi member di grup WhatsApp secara otomatis.

**📋 Alur Proses:**
1️⃣ Input nama grup (multi-line)
2️⃣ Input delay antar grup (dalam detik)
3️⃣ Input nomor yang akan diturunkan dari admin (multi-line)
4️⃣ Program memproses permintaan

**⚠️ Catatan Penting:**
• Bot harus menjadi admin grup
• Nomor harus sudah menjadi admin di grup
• Delay membantu menghindari rate limit WhatsApp (jeda antar grup)
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik tombol di bawah untuk memulai...`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_unadmin_process"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartAdminProcess memulai proses auto admin
func StartAdminProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	adminStates[chatID] = &AdminState{
		IsAdminMode:      true,
		WaitingForGroups: true,
		WaitingForDelay:  false,
		WaitingForPhones: false,
		SelectedGroups:   []GroupLinkInfo{},
		PhoneNumbers:     []string{},
		DelaySeconds:     0,
	}

	promptMsg := `👑 **INPUT NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

Ketik nama grup atau kata kunci untuk mencari grup.
Anda bisa input beberapa grup sekaligus (multi-line).

**Contoh Input:**
• "Keluarga" - Cari grup dengan kata keluarga
• "Kerja\nTim\nProjek" - Input beberapa grup sekaligus
• "." - Ambil SEMUA grup (hati-hati!)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan kata kunci spesifik untuk hasil akurat
• Gunakan multi-line untuk beberapa grup sekaligus
• Pencarian case-insensitive

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_admin"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// StartUnadminProcess memulai proses auto unadmin
func StartUnadminProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	adminStates[chatID] = &AdminState{
		IsAdminMode:      false,
		WaitingForGroups: true,
		WaitingForDelay:  false,
		WaitingForPhones: false,
		SelectedGroups:   []GroupLinkInfo{},
		PhoneNumbers:     []string{},
		DelaySeconds:     0,
	}

	promptMsg := `👤 **INPUT NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

Ketik nama grup atau kata kunci untuk mencari grup.
Anda bisa input beberapa grup sekaligus (multi-line).

**Contoh Input:**
• "Keluarga" - Cari grup dengan kata keluarga
• "Kerja\nTim\nProjek" - Input beberapa grup sekaligus
• "." - Ambil SEMUA grup (hati-hati!)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan kata kunci spesifik untuk hasil akurat
• Gunakan multi-line untuk beberapa grup sekaligus
• Pencarian case-insensitive

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_unadmin"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForAdmin handles input nama grup untuk admin/unadmin
func HandleGroupNameInputForAdmin(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := adminStates[chatID]
	if state == nil || !state.WaitingForGroups {
		return
	}

	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nama grup tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Show loading
	loadingMsg := tgbotapi.NewMessage(chatID, "🔍 Mencari grup...")
	loadingMsgSent, _ := telegramBot.Send(loadingMsg)

	// Search groups
	var groups map[string]string
	var err error

	lines := strings.Split(keyword, "\n")
	if len(lines) > 1 {
		// Multi-line: exact match for each line
		groups, err = utils.SearchGroupsExactMultiple(lines)
	} else if keyword == "." {
		// Get all groups
		groups, err = utils.GetAllGroupsFromDB()
	} else {
		// Single line: ALWAYS try exact match first
		// This ensures that if user inputs a specific group name,
		// only that exact group is selected, not all groups containing the keyword
		groups, err = utils.SearchGroupsExact(keyword)
		if err == nil && len(groups) == 0 {
			// Fallback to flexible only if exact match not found
			groups, err = utils.SearchGroupsFlexible(keyword)
		}
	}

	if err != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)

		// Reset state
		delete(adminStates, chatID)
		return
	}

	// Delete loading message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgSent.MessageID)
	telegramBot.Request(deleteMsg)

	if len(groups) == 0 {
		noResultMsg := fmt.Sprintf(`❌ **TIDAK ADA GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Kata kunci: "%s"

Tidak ada grup yang cocok dengan kata kunci tersebut.

**Saran:**
• Coba kata kunci lebih pendek (1-2 kata)
• Gunakan kata yang pasti ada di nama grup
• Atau klik "📋 Lihat Daftar" untuk melihat semua`, keyword)

		msg := tgbotapi.NewMessage(chatID, noResultMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔍 Coba Lagi", map[bool]string{true: "start_admin_process", false: "start_unadmin_process"}[state.IsAdminMode]),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Menu", "grup"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)

		// Reset state
		delete(adminStates, chatID)
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
	state.WaitingForGroups = false
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

Masukkan berapa detik delay ANTAR GRUP untuk menghindari rate limit.

**Rekomendasi:**
• 1-2 detik: Untuk grup sedikit (< 10)
• 2-3 detik: Untuk grup sedang (10-30)
• 3-5 detik: Untuk grup banyak (> 30)

**💡 Catatan Penting:**
• Delay digunakan untuk jeda ANTAR GRUP
• Semua nomor dalam 1 grup diproses SEKALIGUS (tanpa delay)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	var cancelCallback string
	if state.IsAdminMode {
		cancelCallback = "cancel_admin"
	} else {
		cancelCallback = "cancel_unadmin"
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", cancelCallback),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInputForAdmin handles input delay untuk admin/unadmin
func HandleDelayInputForAdmin(delayStr string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := adminStates[chatID]
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

	// Store delay
	state.DelaySeconds = delay

	// Update state
	state.WaitingForDelay = false
	state.WaitingForPhones = true

	// Show prompt for phone numbers
	modeText := "mengangkat admin"
	if !state.IsAdminMode {
		modeText = "menurunkan admin"
	}

	resultMsg := fmt.Sprintf(`✅ **DELAY DISET**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **Delay:** %d detik/grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **MASUKKAN NOMOR TELEPON**

Ketik nomor telepon yang akan %s.
Anda bisa input beberapa nomor sekaligus (multi-line).

**Format Nomor:**
• 628123456789
• 6281234567890
• +628123456789

**Contoh Multi-line:**
628123456789
628987654321
628555555555

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan format tanpa spasi dan tanda baca
• Nomor harus sudah menjadi anggota grup
• Untuk admin mode: nomor harus anggota grup
• Untuk unadmin mode: nomor harus admin grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input nomor...`, delay, modeText)

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	var cancelCallback string
	if state.IsAdminMode {
		cancelCallback = "cancel_admin"
	} else {
		cancelCallback = "cancel_unadmin"
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", cancelCallback),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandlePhoneInputForAdmin handles input nomor untuk admin/unadmin
func HandlePhoneInputForAdmin(phoneInput string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := adminStates[chatID]
	if state == nil || !state.WaitingForPhones {
		return
	}

	phoneInput = strings.TrimSpace(phoneInput)
	if phoneInput == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nomor telepon tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Parse multi-line phone numbers
	lines := strings.Split(phoneInput, "\n")
	phoneNumbers := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Clean phone number
		phone := cleanPhoneNumber(line)
		if phone != "" {
			phoneNumbers = append(phoneNumbers, phone)
		}
	}

	if len(phoneNumbers) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada nomor telepon yang valid!")
		telegramBot.Send(errorMsg)
		return
	}

	// Store phone numbers
	state.PhoneNumbers = phoneNumbers

	// Update state
	state.WaitingForPhones = false

	// Start processing in goroutine
	actionText := "Mengangkat Admin"
	if !state.IsAdminMode {
		actionText = "Menurunkan Admin"
	}

	// Get client from global
	if WaClient == nil {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(errorMsg)
		delete(adminStates, chatID)
		return
	}

	// Send confirmation
	confirmMsg := fmt.Sprintf(`✅ **INPUT DITERIMA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **Total Nomor:** %d nomor

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 **Memulai proses %s...**

⏳ Mohon tunggu, proses sedang berjalan...`, len(phoneNumbers), actionText)

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// FIXED: Get client fresh, jangan gunakan global WaClient
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)
	var client *whatsmeow.Client
	if userAccount != nil {
		client = am.GetClient(userAccount.ID)
		if client == nil {
			client, _ = am.CreateClient(userAccount.ID)
		}
	}
	if client == nil {
		client = GetWhatsAppClient() // Fallback
	}
	// Process in goroutine
	go ProcessAdminUnadmin(state, chatID, client, telegramBot)
}

// normalizePhoneForComparison menormalisasi nomor telepon untuk perbandingan
// Logika sama dengan cleanPhoneNumber di grup_create.go
func normalizePhoneForComparison(phone string) string {
	// Remove all non-digit characters
	re := regexp.MustCompile(`\D`)
	cleaned := re.ReplaceAllString(phone, "")

	if cleaned == "" {
		return ""
	}

	// Convert local format (08...) to international (62...)
	if strings.HasPrefix(cleaned, "08") {
		cleaned = "62" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "8") {
		cleaned = "62" + cleaned
	} else if !strings.HasPrefix(cleaned, "62") {
		// Assume it's local Indonesian number
		if len(cleaned) >= 9 && len(cleaned) <= 12 {
			cleaned = "62" + cleaned
		}
	}

	// Validate length (Indonesian numbers should be 10-13 digits with country code)
	if len(cleaned) < 10 || len(cleaned) > 15 {
		return ""
	}

	return cleaned
}

// ProcessAdminUnadmin memproses admin/unadmin
func ProcessAdminUnadmin(state *AdminState, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	if client == nil || client.Store.ID == nil {
		errorMsg := utils.FormatUserError(utils.ErrorConnection, fmt.Errorf("WhatsApp client tidak terhubung"), "Bot WhatsApp belum terhubung")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("admin_unadmin", "WhatsApp client belum terhubung", chatID, fmt.Errorf("client is nil or not logged in"))
		return
	}

	totalGroups := len(state.SelectedGroups)
	totalPhones := len(state.PhoneNumbers)

	successCount := 0
	failedCount := 0
	var failedOps []string

	var progressMsgSent *tgbotapi.Message

	action := whatsmeow.ParticipantChangePromote
	actionText := "Mengangkat Admin"
	if !state.IsAdminMode {
		action = whatsmeow.ParticipantChangeDemote
		actionText = "Menurunkan Admin"
	}

	// Convert phone numbers to JIDs
	participantJIDs := []types.JID{}
	for _, phone := range state.PhoneNumbers {
		jid, err := parseJIDFromString(phone + "@s.whatsapp.net")
		if err == nil {
			participantJIDs = append(participantJIDs, jid)
		}
	}

	if len(participantJIDs) == 0 {
		errorMsg := utils.FormatUserError(utils.ErrorValidation, fmt.Errorf("tidak ada nomor valid"), "Semua nomor telepon tidak valid")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("admin_unadmin", "Tidak ada nomor telepon yang valid", chatID, fmt.Errorf("empty participantJIDs"))
		delete(adminStates, chatID)
		return
	}

	for i, group := range state.SelectedGroups {
		// HIGH FIX: Ambil active client di setiap iterasi (admin/unadmin bisa pakai WaClient global!)
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessAdminUnadmin", i, totalGroups)
		if shouldStop {
			// Client disconnect - stop proses
			disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada grup %d/%d\n\n✅ Berhasil: %d\n❌ Gagal: %d", i+1, totalGroups, successCount, failedCount)
			notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
			notifMsg.ParseMode = "Markdown"
			telegramBot.Send(notifMsg)
			break
		}

		// Parse JID
		groupJID, err := parseJIDFromString(group.JID)
		if err != nil {
			failedCount += len(participantJIDs)
			for _, phone := range state.PhoneNumbers {
				failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (invalid JID)", group.Name, phone))
			}
			continue
		}

		// Process ALL phone numbers for this group SIMULTANEOUSLY (batch)
		// Delay hanya digunakan antar grup, bukan antar nomor
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		results, err := validClient.UpdateGroupParticipants(ctx, groupJID, participantJIDs, action)
		cancel()

		// Process results dengan verifikasi real-time
		if err != nil {
			// If batch failed, mark all as failed
			failedCount += len(participantJIDs)
			for _, phone := range state.PhoneNumbers {
				failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (%v)", group.Name, phone, err))
			}
		} else {
			// Delay kecil untuk memastikan WhatsApp sudah update status admin
			time.Sleep(500 * time.Millisecond)

			// Verifikasi real-time: Ambil info grup setelah operasi untuk memastikan status admin aktual
			verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 15*time.Second)
			groupInfo, verifyErr := validClient.GetGroupInfo(verifyCtx, groupJID)
			verifyCancel()

			if verifyErr == nil && groupInfo != nil && len(groupInfo.Participants) > 0 {
				// Buat map peserta grup dengan status admin aktual
				// IMPORTANT: Normalisasi nomor saat membuat map untuk memastikan perbandingan yang konsisten
				// Gunakan map yang menyimpan SEMUA variasi nomor untuk matching yang lebih robust
				actualAdminMap := make(map[string]bool)

				for _, participant := range groupInfo.Participants {
					// Coba ambil nomor dari beberapa sumber: User, PhoneNumber, LID
					var phoneNumbers []string

					// 1. Dari JID.User
					if participant.JID.User != "" {
						phoneNumbers = append(phoneNumbers, participant.JID.User)
					}

					// 2. Dari PhoneNumber (jika ada)
					if !participant.PhoneNumber.IsEmpty() && participant.PhoneNumber.User != "" {
						phoneNumbers = append(phoneNumbers, participant.PhoneNumber.User)
					}

					// 3. Dari LID (jika ada)
					if !participant.LID.IsEmpty() && participant.LID.User != "" {
						phoneNumbers = append(phoneNumbers, participant.LID.User)
					}

					// Normalisasi dan simpan semua variasi nomor
					normalizedPhones := make(map[string]bool) // Untuk menghindari duplikasi
					for _, phone := range phoneNumbers {
						if phone == "" {
							continue
						}

						// Normalisasi nomor
						normalized := normalizePhoneForComparison(phone)
						if normalized == "" {
							normalized = phone // Fallback ke nomor asli
						}

						if normalizedPhones[normalized] {
							continue // Skip jika sudah ada
						}
						normalizedPhones[normalized] = true

						// Simpan variasi nomor (dengan dan tanpa 62 untuk matching yang lebih baik)
						variations := []string{normalized}

						// Tambahkan variasi tanpa 62
						if strings.HasPrefix(normalized, "62") && len(normalized) > 2 {
							variations = append(variations, normalized[2:])
						}

						// Tambahkan variasi dengan 62 jika belum ada
						if !strings.HasPrefix(normalized, "62") && len(normalized) >= 9 {
							variations = append(variations, "62"+normalized)
						}

						// Untuk Promote: cek apakah IsAdmin = true
						// Untuk Demote: cek apakah IsAdmin = false
						statusValue := participant.IsAdmin
						if !state.IsAdminMode {
							// Demote: harus IsAdmin = false
							statusValue = !participant.IsAdmin
						}

						// Simpan semua variasi nomor dengan status yang sama
						for _, variation := range variations {
							actualAdminMap[variation] = statusValue
						}
					}
				}

				// Verifikasi setiap nomor berdasarkan status admin aktual di grup
				for _, phone := range state.PhoneNumbers {
					// Phone sudah dinormalisasi saat input
					// Cari di map dengan berbagai variasi format (sudah disimpan di map dengan semua variasi)
					found := false
					var actualStatus bool

					// 1. Coba dengan format utama (yang sudah dinormalisasi)
					if status, exists := actualAdminMap[phone]; exists {
						actualStatus = status
						found = true
					}

					// 2. Coba tanpa "62" prefix (untuk nomor Indonesia)
					if !found && strings.HasPrefix(phone, "62") && len(phone) > 2 {
						without62 := phone[2:]
						if status, exists := actualAdminMap[without62]; exists {
							actualStatus = status
							found = true
						}
					}

					// 3. Coba dengan "62" prefix jika nomor tidak ada prefix
					if !found && !strings.HasPrefix(phone, "62") {
						with62 := "62" + phone
						if status, exists := actualAdminMap[with62]; exists {
							actualStatus = status
							found = true
						}
					}

					if !found {
						// Nomor benar-benar tidak ditemukan di grup
						// Mungkin nomor belum terupdate di GetGroupInfo atau format benar-benar berbeda
						failedCount++
						failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (tidak ditemukan di grup)", group.Name, phone))
						continue
					}

					// Verifikasi status admin sesuai yang diinginkan
					if actualStatus {
						// Success - status admin sesuai yang diinginkan (baik berhasil diubah maupun sudah sesuai sebelumnya)
						successCount++
					} else {
						// Failed - status admin tidak sesuai dengan yang diinginkan
						failedCount++
						failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (status tidak sesuai)", group.Name, phone))
					}
				}
			} else {
				// Jika verifikasi gagal, gunakan hasil dari UpdateGroupParticipants
				// Periksa field Error pada setiap participant dalam results
				resultErrorMap := make(map[string]int)
				for _, participant := range results {
					user := participant.JID.User
					if user != "" {
						// Error field menunjukkan kode error (0 = success)
						resultErrorMap[user] = participant.Error
					}
				}

				// Check each phone number
				for _, phone := range state.PhoneNumbers {
					errorCode, exists := resultErrorMap[phone]
					if exists && errorCode == 0 {
						// Success - Error code is 0
						successCount++
					} else if !exists {
						// Not in results - might be success but API didn't return it
						// Default to success if API call didn't error
						successCount++
					} else {
						// Failed - Error code is not 0
						failedCount++
						failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (error code: %d)", group.Name, phone, errorCode))
					}
				}
			}
		}

		// Show progress if many groups
		if totalGroups > 1 {
			progressPercent := ((i + 1) * 100) / totalGroups
			progressBar := generateProgressBar(progressPercent)

			progressMsg := fmt.Sprintf(`⏳ **PROGRESS %s**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Progress:** %d/%d grup
📋 **Grup:** %s (%d/%d)
📱 **Total Nomor:** %d nomor (diproses sekaligus)
✅ **Berhasil:** %d operasi
❌ **Gagal:** %d operasi

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔄 Memproses grup %d/%d...`, actionText, progressBar, i+1, totalGroups, group.Name, i+1, totalGroups, totalPhones, successCount, failedCount, i+1, totalGroups)

			if progressMsgSent == nil {
				progressMsgObj := tgbotapi.NewMessage(chatID, progressMsg)
				progressMsgObj.ParseMode = "Markdown"
				sentMsg, _ := telegramBot.Send(progressMsgObj)
				progressMsgSent = &sentMsg
			} else {
				editMsg := tgbotapi.NewEditMessageText(chatID, progressMsgSent.MessageID, progressMsg)
				editMsg.ParseMode = "Markdown"
				telegramBot.Send(editMsg)
			}
		}

		// Delay between groups (except last group)
		if i < totalGroups-1 && state.DelaySeconds > 0 {
			time.Sleep(time.Duration(state.DelaySeconds) * time.Second)
		}
	}

	// Delete progress message after completion
	if progressMsgSent != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMsgSent.MessageID)
		telegramBot.Request(deleteMsg)
	}

	// Final summary
	totalOps := totalGroups * totalPhones
	summaryMsg := fmt.Sprintf(`🎉 **SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 **RINGKASAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 **Total Operasi:** %d operasi
📱 **Total Nomor:** %d nomor
⏱️ **Delay:** %d detik/grup
✅ **Berhasil:** %d operasi
❌ **Gagal:** %d operasi
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, totalOps, totalPhones, state.DelaySeconds, successCount, failedCount)

	if successCount > 0 {
		summaryMsg += fmt.Sprintf("\n\n**✅ Operasi Berhasil:** %d operasi\n", successCount)
	}

	if failedCount > 0 {
		summaryMsg += "\n\n**❌ Operasi Gagal (Batch 1):**\n\n"
		maxDisplay := 20
		if len(failedOps) < maxDisplay {
			maxDisplay = len(failedOps)
		}
		for i := 0; i < maxDisplay; i++ {
			summaryMsg += failedOps[i] + "\n\n"
		}
		if len(failedOps) > maxDisplay {
			summaryMsg += fmt.Sprintf("... dan %d operasi gagal lainnya\n", len(failedOps)-maxDisplay)
		}
	}

	summaryMsgObj := tgbotapi.NewMessage(chatID, summaryMsg)
	summaryMsgObj.ParseMode = "Markdown"
	telegramBot.Send(summaryMsgObj)

	// Clean up state
	delete(adminStates, chatID)
}

// CancelAdmin membatalkan proses admin
func CancelAdmin(telegramBot *tgbotapi.BotAPI, chatID int64) {
	delete(adminStates, chatID)
	msg := tgbotapi.NewMessage(chatID, "❌ Proses auto admin dibatalkan.")
	telegramBot.Send(msg)
}

// CancelAdminEdit membatalkan proses auto admin dengan EDIT (no spam!)
func CancelAdminEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	delete(adminStates, chatID)
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **PROSES DIBATALKAN**\n\nProses auto admin telah dibatalkan.\n\nAnda dapat memulai kembali dari menu Auto Admin.")
	editMsg.ParseMode = "Markdown"
	telegramBot.Send(editMsg)
}

// CancelUnadmin membatalkan proses unadmin
func CancelUnadmin(telegramBot *tgbotapi.BotAPI, chatID int64) {
	delete(adminStates, chatID)
	msg := tgbotapi.NewMessage(chatID, "❌ Proses auto unadmin dibatalkan.")
	telegramBot.Send(msg)
}

// CancelUnadminEdit membatalkan proses unadmin dengan EDIT (no spam!)
func CancelUnadminEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	delete(adminStates, chatID)
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ **PROSES DIBATALKAN**\n\nProses auto unadmin telah dibatalkan.\n\nAnda dapat memulai kembali dari menu Auto Unadmin.")
	editMsg.ParseMode = "Markdown"
	telegramBot.Send(editMsg)
}

// IsWaitingForAdminInput checks if user is waiting for admin input
func IsWaitingForAdminInput(chatID int64) bool {
	state := adminStates[chatID]
	return state != nil && (state.WaitingForGroups || state.WaitingForDelay || state.WaitingForPhones)
}

// GetAdminInputType returns the type of input expected
func GetAdminInputType(chatID int64) string {
	state := adminStates[chatID]
	if state == nil {
		return ""
	}
	if state.WaitingForGroups {
		return "groups"
	}
	if state.WaitingForDelay {
		return "delay"
	}
	if state.WaitingForPhones {
		return "phones"
	}
	return ""
}
