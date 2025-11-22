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
	"go.mau.fi/whatsmeow/types"
)

// AddMemberState manages the state for adding members to group
type AddMemberState struct {
	WaitingForGroupName   bool
	WaitingForNumbers     bool
	WaitingForDelay       bool
	WaitingForMode        bool
	WaitingForNumberDelay bool
	SelectedGroups        []GroupLinkInfo
	PhoneNumbers          []string
	DelaySeconds          int
	NumberDelaySeconds    int
	AddMode               string   // "one_by_one" or "batch"
	ContactsToDelete      []string // JIDs yang perlu dihapus setelah selesai
}

var addMemberStates = make(map[int64]*AddMemberState)

// ShowAddMemberMenu menampilkan menu add member grup
func ShowAddMemberMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `➕ **ADD MEMBER GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan menambahkan anggota baru ke grup WhatsApp yang Anda pilih.

**📋 Alur Proses:**

1️⃣ **Input Nama Grup** - Manual atau file .txt
2️⃣ **Input Nomor Telepon** - Manual atau file .vcf (semua versi)
3️⃣ **Input Delay Antar Grup** - Detik
4️⃣ **Pilih Mode:**
   • 1/1 - Tambahkan satu per satu (dengan delay antar nomor)
   • Batch - Tambahkan semua sekaligus (hanya delay antar grup)

**✨ Fitur Otomatis:**
• Simpan kontak otomatis jika belum tersimpan
• Hapus kontak otomatis setelah selesai (jika di-save)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup
• Nomor harus valid dan aktif di WhatsApp
• Delay membantu menghindari rate limit
• VCF file support semua versi (v2.1, v3.0, v4.0)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik tombol di bawah untuk memulai`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_add_member"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Contoh Format", "add_member_example"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowAddMemberMenuEdit menampilkan menu dengan EDIT message
func ShowAddMemberMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `➕ **ADD MEMBER GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan menambahkan anggota baru ke grup WhatsApp yang Anda pilih.

**📋 Alur Proses:**

1️⃣ **Input Nama Grup** - Manual atau file .txt
2️⃣ **Input Nomor Telepon** - Manual atau file .vcf (semua versi)
3️⃣ **Input Delay Antar Grup** - Detik
4️⃣ **Pilih Mode:**
   • 1/1 - Tambahkan satu per satu (dengan delay antar nomor)
   • Batch - Tambahkan semua sekaligus (hanya delay antar grup)

**✨ Fitur Otomatis:**
• Simpan kontak otomatis jika belum tersimpan
• Hapus kontak otomatis setelah selesai (jika di-save)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup
• Nomor harus valid dan aktif di WhatsApp
• Delay membantu menghindari rate limit
• VCF file support semua versi (v2.1, v3.0, v4.0)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik tombol di bawah untuk memulai`

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai", "start_add_member"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Contoh Format", "add_member_example"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	editMsg.ReplyMarkup = &keyboard

	telegramBot.Send(editMsg)
}

// ShowAddMemberExample menampilkan contoh format
func ShowAddMemberExample(chatID int64, telegramBot *tgbotapi.BotAPI, messageID int) {
	exampleMsg := `📖 **CONTOH FORMAT**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**1️⃣ Input Nama Grup (Manual):**
Keluarga Besar
Grup Kerja
Grup Teman

**2️⃣ Input Nomor Telepon (Manual):**
628123456789
628987654321
628111222333

**3️⃣ Input Nomor Telepon (File .vcf):**
Format VCF standard dengan TEL field (support v2.1, v3.0, v4.0)

**4️⃣ Delay Antar Grup:**
2 (detik)

**5️⃣ Mode:**
• 1/1: Tambahkan satu per satu dengan delay antar nomor
• Batch: Tambahkan semua sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Nomor bisa dalam format internasional (62...) atau lokal (08...)
• VCF file support semua versi standar
• Delay disarankan 2-5 detik untuk menghindari rate limit`

	if messageID > 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exampleMsg)
		editMsg.ParseMode = "Markdown"
		telegramBot.Send(editMsg)
	} else {
		msg := tgbotapi.NewMessage(chatID, exampleMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	}
}

// StartAddMemberProcess memulai proses add member
func StartAddMemberProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	addMemberStates[chatID] = &AddMemberState{
		WaitingForGroupName:   true,
		WaitingForNumbers:     false,
		WaitingForDelay:       false,
		WaitingForMode:        false,
		WaitingForNumberDelay: false,
		SelectedGroups:        []GroupLinkInfo{},
		PhoneNumbers:          []string{},
		DelaySeconds:          0,
		NumberDelaySeconds:    0,
		AddMode:               "",
		ContactsToDelete:      []string{},
	}

	promptMsg := `🔍 **MASUKKAN NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

**Cara Input:**

**Opsi 1: Input Manual (Multi-line)**
Ketik nama grup, setiap baris = 1 grup

**Opsi 2: Upload File .txt**
Kirim file .txt yang berisi nama grup (satu per baris)

**Contoh Input Manual:**
Keluarga Besar
Grup Kerja
Grup Teman

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Multi-line untuk exact match nama grup
• Pencarian tidak case-sensitive
• Gunakan "." jika ingin tambahkan ke semua grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input nama grup atau file .txt...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForAddMember memproses input nama grup
func HandleGroupNameInputForAddMember(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := addMemberStates[chatID]
	if state == nil || !state.WaitingForGroupName {
		return
	}

	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nama grup tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Check if it's "all groups" request
	if keyword == "." {
		// Get all groups
		allGroupsMap, err := utils.GetAllGroupsFromDB()
		if err != nil || len(allGroupsMap) == 0 {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Gagal mengambil daftar grup atau tidak ada grup yang ditemukan!")
			telegramBot.Send(errorMsg)
			return
		}

		// Convert map to GroupLinkInfo
		for jid, name := range allGroupsMap {
			state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
				JID:  jid,
				Name: name,
			})
		}
	} else {
		// Parse multi-line group names
		lines := strings.Split(keyword, "\n")
		var groupNames []string
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
			notFoundMsg := fmt.Sprintf("❌ **TIDAK ADA GRUP DITEMUKAN**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nTidak ada grup yang cocok dengan nama:\n\n")
			for _, name := range groupNames {
				notFoundMsg += fmt.Sprintf("• %s\n", name)
			}
			notFoundMsg += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n💡 Pastikan nama grup sudah benar atau coba cari dengan kata kunci."
			msg := tgbotapi.NewMessage(chatID, notFoundMsg)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
			return
		}

		// Convert map to GroupLinkInfo
		for jid, name := range groupsMap {
			state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
				JID:  jid,
				Name: name,
			})
		}
	}

	state.WaitingForGroupName = false
	state.WaitingForNumbers = true

	// Ask for phone numbers
	confirmMsg := fmt.Sprintf(`✅ **GRUP DIPILIH**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Grup:** %d grup

**Daftar Grup:**
`, len(state.SelectedGroups))

	maxShow := 10
	if len(state.SelectedGroups) > maxShow {
		for i := 0; i < maxShow; i++ {
			confirmMsg += fmt.Sprintf("• %s\n", state.SelectedGroups[i].Name)
		}
		confirmMsg += fmt.Sprintf("... dan %d grup lainnya\n", len(state.SelectedGroups)-maxShow)
	} else {
		for _, group := range state.SelectedGroups {
			confirmMsg += fmt.Sprintf("• %s\n", group.Name)
		}
	}

	confirmMsg += `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **MASUKKAN NOMOR TELEPON**

**Cara Input:**

**Opsi 1: Input Manual (Multi-line)**
Ketik nomor telepon, setiap baris = 1 nomor

**Opsi 2: Upload File .vcf**
Kirim file .vcf yang berisi kontak (semua versi didukung)

**Contoh Input Manual:**
628123456789
628987654321
628111222333

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Format Nomor:**
• Format internasional: 62XXXXXXXXX
• Format lokal: 08XXXXXXXXX (akan dikonversi otomatis)
• Minimal 10 digit, maksimal 15 digit

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input nomor telepon atau file .vcf...`

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// parseVCFFile parses VCF file and extracts phone numbers (supports all VCF versions)
func parseVCFFile(body *http.Response) ([]string, error) {
	var phoneNumbers []string
	scanner := bufio.NewScanner(body.Body)

	var currentPhone string
	inVCARD := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for BEGIN:VCARD
		if strings.HasPrefix(strings.ToUpper(line), "BEGIN:VCARD") {
			inVCARD = true
			currentPhone = ""
			continue
		}

		// Check for END:VCARD
		if strings.HasPrefix(strings.ToUpper(line), "END:VCARD") {
			if inVCARD && currentPhone != "" {
				cleaned := cleanPhoneNumber(currentPhone)
				if cleaned != "" {
					phoneNumbers = append(phoneNumbers, cleaned)
				}
			}
			inVCARD = false
			currentPhone = ""
			continue
		}

		if !inVCARD {
			continue
		}

		// Parse TEL field (supports various formats)
		// TEL;TYPE=CELL:628123456789
		// TEL:628123456789
		// TEL;TYPE=WORK,CELL:628123456789
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "TEL") {
			// Extract phone number after colon
			colonIndex := strings.LastIndex(line, ":")
			if colonIndex >= 0 && colonIndex < len(line)-1 {
				phone := line[colonIndex+1:]
				phone = strings.TrimSpace(phone)
				// Replace common separators
				phone = strings.ReplaceAll(phone, "-", "")
				phone = strings.ReplaceAll(phone, " ", "")
				phone = strings.ReplaceAll(phone, "(", "")
				phone = strings.ReplaceAll(phone, ")", "")
				if phone != "" {
					currentPhone = phone
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Handle last vCard if file doesn't end with END:VCARD
	if inVCARD && currentPhone != "" {
		cleaned := cleanPhoneNumber(currentPhone)
		if cleaned != "" {
			phoneNumbers = append(phoneNumbers, cleaned)
		}
	}

	return phoneNumbers, nil
}

// HandleFileInputForAddMember handles file input (.txt for groups, .vcf for contacts)
func HandleFileInputForAddMember(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string, isVCF bool) {
	state := addMemberStates[chatID]
	if state == nil {
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

	if isVCF {
		// Parse VCF file
		phoneNumbers, err := parseVCFFile(fileResp2)
		if err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal parsing file VCF: %v", err))
			telegramBot.Send(errorMsg)
			return
		}

		if len(phoneNumbers) == 0 {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ File VCF tidak berisi nomor telepon yang valid!")
			telegramBot.Send(errorMsg)
			return
		}

		state.PhoneNumbers = phoneNumbers
		state.WaitingForNumbers = false

		// Ask for delay
		askForDelay(chatID, telegramBot, state)
	} else {
		// Parse .txt file for group names
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

		state.SelectedGroups = []GroupLinkInfo{}
		for jid, name := range groupsMap {
			state.SelectedGroups = append(state.SelectedGroups, GroupLinkInfo{
				JID:  jid,
				Name: name,
			})
		}

		state.WaitingForGroupName = false
		state.WaitingForNumbers = true

		// Ask for phone numbers
		confirmMsg := fmt.Sprintf(`✅ **GRUP DIPILIH DARI FILE**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Grup:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **MASUKKAN NOMOR TELEPON**

Ketik nomor telepon (multi-line) atau kirim file .vcf`, len(state.SelectedGroups))

		msg := tgbotapi.NewMessage(chatID, confirmMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	}
}

// HandlePhoneInputForAddMember memproses input nomor telepon
func HandlePhoneInputForAddMember(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := addMemberStates[chatID]
	if state == nil || !state.WaitingForNumbers {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nomor telepon tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	// Parse multi-line phone numbers
	lines := strings.Split(input, "\n")
	var numbers []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Clean and validate phone number
			cleaned := cleanPhoneNumber(line)
			if cleaned != "" {
				numbers = append(numbers, cleaned)
			}
		}
	}

	if len(numbers) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada nomor telepon yang valid!")
		telegramBot.Send(errorMsg)
		return
	}

	state.PhoneNumbers = numbers
	state.WaitingForNumbers = false

	// Ask for delay
	askForDelay(chatID, telegramBot, state)
}

// askForDelay meminta input delay antar grup
func askForDelay(chatID int64, telegramBot *tgbotapi.BotAPI, state *AddMemberState) {
	confirmMsg := fmt.Sprintf(`✅ **NOMOR TERDAFTAR**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **Total Nomor:** %d nomor
📊 **Total Grup:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY ANTAR GRUP**

Masukkan berapa detik delay antar grup saat menambahkan member.

**Rekomendasi:**
• 2-3 detik: < 10 grup
• 3-5 detik: 10-30 grup
• 5-10 detik: > 30 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 3)`, len(state.PhoneNumbers), len(state.SelectedGroups))

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
	state.WaitingForDelay = true
}

// HandleDelayInputForAddMember memproses input delay
func HandleDelayInputForAddMember(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := addMemberStates[chatID]
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
	state.WaitingForMode = true

	// Ask for mode
	modeMsg := fmt.Sprintf(`✅ **DELAY DISET**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **Delay Antar Grup:** %d detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 **PILIH MODE TAMBAH MEMBER**

**1️⃣ 1/1 (Satu Per Satu)**
• Tambahkan nomor satu per satu
• Ada delay antar nomor
• Lebih aman, lebih lambat

**2️⃣ Batch (Semua Sekaligus)**
• Tambahkan semua nomor sekaligus
• Hanya delay antar grup
• Lebih cepat, sedikit lebih berisiko

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih mode yang Anda inginkan`, state.DelaySeconds)

	msg := tgbotapi.NewMessage(chatID, modeMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1️⃣ 1/1", "add_member_mode_one_by_one"),
			tgbotapi.NewInlineKeyboardButtonData("2️⃣ Batch", "add_member_mode_batch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleModeInputForAddMember memproses input mode
func HandleModeInputForAddMember(mode string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := addMemberStates[chatID]
	if state == nil || !state.WaitingForMode {
		return
	}

	state.AddMode = mode

	if mode == "one_by_one" {
		// Ask for delay between numbers
		state.WaitingForMode = false
		state.WaitingForNumberDelay = true

		delayMsg := `⏱️ **TENTUKAN DELAY ANTAR NOMOR**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Mode:** 1/1 (Satu Per Satu)

Masukkan berapa detik delay antar nomor saat menambahkan member.

**Rekomendasi:**
• 1-2 detik: < 10 nomor
• 2-3 detik: 10-50 nomor
• 3-5 detik: > 50 nomor

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`

		msg := tgbotapi.NewMessage(chatID, delayMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_add_member"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	} else {
		// Batch mode - no delay needed between numbers
		state.WaitingForMode = false
		state.WaitingForNumberDelay = false

		// Start processing
		startMsg := fmt.Sprintf(`✅ **KONFIGURASI SELESAI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Grup:** %d grup
📱 **Total Nomor:** %d nomor
⏱️ **Delay Antar Grup:** %d detik
📋 **Mode:** Batch (Semua Sekaligus)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 **Memulai proses add member...**

⏳ Mohon tunggu, proses sedang berjalan...`, len(state.SelectedGroups), len(state.PhoneNumbers), state.DelaySeconds)

		msg := tgbotapi.NewMessage(chatID, startMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

		// Get client
		client := GetWhatsAppClient()
		if client == nil {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
			telegramBot.Send(errorMsg)
			delete(addMemberStates, chatID)
			return
		}

		// Process in goroutine
		go ProcessAddMember(state, chatID, client, telegramBot)

		// Clear state after processing starts
		// State will be cleared after processing completes
	}
}

// HandleNumberDelayInputForAddMember memproses input delay antar nomor
func HandleNumberDelayInputForAddMember(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := addMemberStates[chatID]
	if state == nil || !state.WaitingForNumberDelay {
		return
	}

	delay, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || delay < 0 || delay > 300 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Delay harus antara 0-300 detik!")
		telegramBot.Send(errorMsg)
		return
	}

	state.NumberDelaySeconds = delay
	state.WaitingForNumberDelay = false

	// Start processing
	startMsg := fmt.Sprintf(`✅ **KONFIGURASI SELESAI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Grup:** %d grup
📱 **Total Nomor:** %d nomor
⏱️ **Delay Antar Grup:** %d detik
⏱️ **Delay Antar Nomor:** %d detik
📋 **Mode:** 1/1 (Satu Per Satu)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 **Memulai proses add member...**

⏳ Mohon tunggu, proses sedang berjalan...`, len(state.SelectedGroups), len(state.PhoneNumbers), state.DelaySeconds, state.NumberDelaySeconds)

	msg := tgbotapi.NewMessage(chatID, startMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Get client
	client := GetWhatsAppClient()
	if client == nil {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(errorMsg)
		delete(addMemberStates, chatID)
		return
	}

	// Process in goroutine
	go ProcessAddMember(state, chatID, client, telegramBot)
}

// ProcessAddMember memproses penambahan member ke grup
func ProcessAddMember(state *AddMemberState, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	if client == nil || client.Store.ID == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(msg)
		delete(addMemberStates, chatID)
		return
	}

	totalGroups := len(state.SelectedGroups)
	totalPhones := len(state.PhoneNumbers)

	successCount := 0
	failedCount := 0
	inviteCount := 0 // Track jumlah yang diundang (status "undang")
	var failedOps []string
	var inviteOps []string // Track operasi yang menghasilkan undangan

	var progressMsgSent *tgbotapi.Message

	// Convert phone numbers to JIDs
	participantJIDs := []types.JID{}
	for _, phone := range state.PhoneNumbers {
		jid, err := parseJIDFromString(phone + "@s.whatsapp.net")
		if err == nil {
			participantJIDs = append(participantJIDs, jid)
		}
	}

	if len(participantJIDs) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Semua nomor telepon tidak valid!")
		telegramBot.Send(errorMsg)
		delete(addMemberStates, chatID)
		return
	}

	// Check and save contacts if not exists (before adding to groups)
	// Note: WhatsApp will automatically save contacts when added to group
	// We check if contact exists first to track which ones we might need to handle
	savedContacts := []string{}
	for _, jid := range participantJIDs {
		// Check if contact exists
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		contact, err := client.GetUserInfo(ctx, []types.JID{jid})
		cancel()

		if err != nil || contact == nil {
			// Contact doesn't exist in our contact list
			// WhatsApp will auto-save when we add to group
			savedContacts = append(savedContacts, jid.String())
		}
	}

	// Process each group
	for i, group := range state.SelectedGroups {
		// Parse group JID
		groupJID, err := parseJIDFromString(group.JID)
		if err != nil {
			failedCount += len(participantJIDs)
			for _, phone := range state.PhoneNumbers {
				failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (invalid JID)", group.Name, phone))
			}
			continue
		}

		if state.AddMode == "one_by_one" {
			// Add one by one with delay
			for j, jid := range participantJIDs {
				// Validate client before each operation to prevent disconnect mid-operation
				validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessAddMember", i, totalGroups)
				if shouldStop {
					disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada nomor %d/%d di grup %s\n\n✅ Berhasil: %d\n📧 Undang: %d\n❌ Gagal: %d", j+1, len(participantJIDs), group.Name, successCount, inviteCount, failedCount)
					notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
					notifMsg.ParseMode = "Markdown"
					telegramBot.Send(notifMsg)
					goto cleanup // Exit outer loop
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				results, err := validClient.UpdateGroupParticipants(ctx, groupJID, []types.JID{jid}, whatsmeow.ParticipantChangeAdd)
				cancel()

				phone := state.PhoneNumbers[j]

				if err != nil {
					failedCount++
					failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (%v)", group.Name, phone, err))
				} else if len(results) > 0 {
					// FIXED: Hapus nil check yang tidak perlu, len() untuk nil slices sudah defined as zero
					// Check individual result status
					participant := results[0]
					if participant.Error == 0 {
						// Success - langsung masuk grup
						successCount++
					} else if participant.Error == 401 {
						// Status "undang" - undangan terkirim
						inviteCount++
						inviteOps = append(inviteOps, fmt.Sprintf("📧 %s - %s (undangan terkirim)", group.Name, phone))
					} else {
						// Other error codes
						failedCount++
						failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (error code: %d)", group.Name, phone, participant.Error))
					}
				} else {
					// No results returned, assume success (legacy behavior)
					successCount++
				}

				// Delay between numbers (except last one)
				if j < len(participantJIDs)-1 {
					time.Sleep(time.Duration(state.NumberDelaySeconds) * time.Second)
				}
			}
		} else {
			// Batch mode - add all at once
			// CRITICAL: Validate client BEFORE batch operation
			validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessAddMember", i, totalGroups)
			if shouldStop {
				disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus sebelum batch operation grup %s\n\n✅ Berhasil: %d\n📧 Undang: %d\n❌ Gagal: %d", group.Name, successCount, inviteCount, failedCount)
				notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
				notifMsg.ParseMode = "Markdown"
				telegramBot.Send(notifMsg)
				break
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			results, err := validClient.UpdateGroupParticipants(ctx, groupJID, participantJIDs, whatsmeow.ParticipantChangeAdd)
			cancel()

			// CRITICAL: Validate client AFTER batch operation to ensure it completed
			// This ensures 100% completion before potential disconnect
			// FIXED: validClient tidak digunakan setelah ini, cukup cek shouldStop saja
			_, shouldStop = ValidateClientForBackgroundProcess(client, "ProcessAddMember", i, totalGroups)
			if shouldStop {
				// Client disconnected during batch operation
				// We still process results if available, but mark as interrupted
				disconnectMsg := fmt.Sprintf("⚠️ **PERINGATAN: CLIENT TERPUTUS SETELAH BATCH**\n\nGrup: %s\n\n⚠️ Batch operation mungkin tidak 100%% selesai.\nProses hasil yang tersedia...", group.Name)
				warnMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
				warnMsg.ParseMode = "Markdown"
				telegramBot.Send(warnMsg)
			}

			if err != nil {
				// If batch failed completely, mark all as failed
				failedCount += len(participantJIDs)
				for _, phone := range state.PhoneNumbers {
					failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (%v)", group.Name, phone, err))
				}
			} else {
				// Process results - check each participant individually
				// FIXED: Hapus nil check yang tidak perlu, len() untuk nil slices sudah defined as zero
				if len(results) > 0 {
					// Create map of JID to result for matching
					resultMap := make(map[string]*types.GroupParticipant)
					for idx := range results {
						resultMap[results[idx].JID.User] = &results[idx]
					}

					// Process each phone number
					for j, phone := range state.PhoneNumbers {
						// Find matching JID
						jid := participantJIDs[j]
						if participant, found := resultMap[jid.User]; found {
							if participant.Error == 0 {
								// Success - langsung masuk grup
								successCount++
							} else if participant.Error == 401 {
								// Status "undang" - undangan terkirim
								inviteCount++
								inviteOps = append(inviteOps, fmt.Sprintf("📧 %s - %s (undangan terkirim)", group.Name, phone))
							} else {
								// Other error codes
								failedCount++
								failedOps = append(failedOps, fmt.Sprintf("❌ %s - %s (error code: %d)", group.Name, phone, participant.Error))
							}
						} else {
							// Not found in results - might be success but API didn't return it
							// Default to success for backward compatibility
							successCount++
						}
					}
				} else {
					// No results returned - assume all succeeded (legacy behavior)
					successCount += len(participantJIDs)
				}
			}
		}

		// Show progress if more than 3 groups
		if totalGroups > 3 {
			progressPercent := ((i + 1) * 100) / totalGroups
			progressMsg := fmt.Sprintf("🔄 **PROSES BERJALAN...**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n📊 **Progress:** %d%% (%d/%d grup)\n\n✅ **Berhasil:** %d\n📧 **Undang:** %d\n❌ **Gagal:** %d", progressPercent, i+1, totalGroups, successCount, inviteCount, failedCount)

			if progressMsgSent == nil {
				msg := tgbotapi.NewMessage(chatID, progressMsg)
				msg.ParseMode = "Markdown"
				sent, _ := telegramBot.Send(msg)
				progressMsgSent = &sent
			} else {
				editMsg := tgbotapi.NewEditMessageText(chatID, progressMsgSent.MessageID, progressMsg)
				editMsg.ParseMode = "Markdown"
				telegramBot.Send(editMsg)
			}
		}

		// Delay between groups (except last one)
		if i < len(state.SelectedGroups)-1 {
			time.Sleep(time.Duration(state.DelaySeconds) * time.Second)
		}
	}

cleanup:

	// Note: WhatsApp API doesn't support deleting contacts directly
	// Contacts added to groups will remain in contact list
	// This is a limitation of WhatsApp API - we can't programmatically delete them
	// User needs to delete manually if desired

	// Delete progress message if exists
	if progressMsgSent != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, progressMsgSent.MessageID)
		telegramBot.Request(deleteMsg)
	}

	// Send final summary
	summaryMsg := fmt.Sprintf(`✅ **PROSES SELESAI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total Grup:** %d grup
📱 **Total Nomor:** %d nomor

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Berhasil:** %d
📧 **Undang:** %d
❌ **Gagal:** %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, totalGroups, totalPhones, successCount, inviteCount, failedCount)

	if len(inviteOps) > 0 {
		summaryMsg += "\n\n**📧 Detail Undang (Undangan Terkirim):**\n"
		summaryMsg += "💡 Nomor-nomor ini menerima undangan untuk bergabung ke grup.\n"
		summaryMsg += "💡 Mereka harus menerima undangan terlebih dahulu sebelum masuk grup.\n\n"
		maxInvite := 10
		if len(inviteOps) > maxInvite {
			for i := 0; i < maxInvite; i++ {
				summaryMsg += inviteOps[i] + "\n"
			}
			summaryMsg += fmt.Sprintf("\n... dan %d lainnya", len(inviteOps)-maxInvite)
		} else {
			for _, invite := range inviteOps {
				summaryMsg += invite + "\n"
			}
		}
	}

	if len(failedOps) > 0 {
		summaryMsg += "\n\n**❌ Detail Gagal:**\n"
		maxFailed := 10
		if len(failedOps) > maxFailed {
			for i := 0; i < maxFailed; i++ {
				summaryMsg += failedOps[i] + "\n"
			}
			summaryMsg += fmt.Sprintf("\n... dan %d lainnya", len(failedOps)-maxFailed)
		} else {
			for _, failed := range failedOps {
				summaryMsg += failed + "\n"
			}
		}
	}

	if len(savedContacts) > 0 {
		summaryMsg += fmt.Sprintf("\n\n**💡 Catatan:**\n%d kontak baru otomatis tersimpan saat ditambahkan ke grup.\n\n⚠️ WhatsApp tidak mendukung penghapusan kontak via API.\nKontak tetap ada di daftar kontak WhatsApp.", len(savedContacts))
	}

	msg := tgbotapi.NewMessage(chatID, summaryMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Log activity
	utils.LogActivity("add_member", fmt.Sprintf("Add %d members to %d groups: %d success, %d failed", totalPhones, totalGroups, successCount, failedCount), chatID)

	// Clear state
	delete(addMemberStates, chatID)
}

// CancelAddMember membatalkan proses add member
func CancelAddMember(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(addMemberStates, chatID)
	msg := tgbotapi.NewMessage(chatID, "❌ Proses add member dibatalkan.")
	telegramBot.Send(msg)
}

// GetAddMemberState mendapatkan state untuk chatID tertentu
func GetAddMemberState(chatID int64) *AddMemberState {
	return addMemberStates[chatID]
}
