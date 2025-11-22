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

// GroupCreateState manages the state for creating groups automatically
type GroupCreateState struct {
	Mode                string // "single" or "multiline"
	WaitingForGroupName bool
	WaitingForCount     bool // Only for single mode
	WaitingForNumbers   bool
	WaitingForDelay     bool
	WaitingForSettings  bool
	CurrentSettingIndex int
	GroupNames          []string
	GroupCount          int    // Only for single mode
	BaseName            string // Only for single mode
	PhoneNumbers        []string
	DelaySeconds        int

	// Settings to apply (same as all_settings)
	MessageLogging *bool  // nil = skip, true = ON, false = OFF
	MemberAdd      *bool  // nil = skip, true = ON, false = OFF
	JoinApproval   *bool  // nil = skip, true = ON, false = OFF
	Ephemeral      *int64 // nil = skip, 0 = OFF, 86400 = 24h, 604800 = 7d, 7776000 = 90d
	EditSettings   *bool  // nil = skip, true = ON, false = OFF
}

var groupCreateStates = make(map[int64]*GroupCreateState)

// Settings list in order
var createGroupSettingsList = []string{
	"message_logging",
	"member_add",
	"join_approval",
	"ephemeral",
	"edit_settings",
}

// ShowCreateGroupMenu menampilkan menu buat grup otomatis
func ShowCreateGroupMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `🚀 **BUAT GRUP OTOMATIS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan membuat grup WhatsApp secara otomatis dengan pengaturan yang Anda tentukan.

**📋 Pilihan Mode:**

**Opsi 1: Nama + Jumlah**
• Input nama grup (contoh: "Grup baru 1")
• Input jumlah grup yang akan dibuat
• Program akan membuat grup dengan nama "Grup baru 1", "Grup baru 2", dst

**Opsi 2: Multi-line**
• Input nama grup per baris
• Setiap baris = 1 grup
• Contoh:
  GRUP
  GRUP
  GRUP

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📝 Alur Proses:**
1. Pilih mode (Opsi 1 atau 2)
2. Input nama grup sesuai mode
3. Input nomor telepon (atau skip)
4. Input pengaturan grup (ON/OFF untuk 5 pengaturan)
5. Program akan membuat grup dan menerapkan pengaturan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih mode yang Anda inginkan`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1️⃣ Opsi 1: Nama + Jumlah", "create_group_mode_single"),
			tgbotapi.NewInlineKeyboardButtonData("2️⃣ Opsi 2: Multi-line", "create_group_mode_multiline"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowCreateGroupMenuEdit menampilkan menu (EDIT, NO SPAM!)
func ShowCreateGroupMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `🚀 **BUAT GRUP OTOMATIS**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan membuat grup WhatsApp secara otomatis dengan pengaturan yang Anda tentukan.

**📋 Pilihan Mode:**

**Opsi 1: Nama + Jumlah**
• Input nama grup (contoh: "Grup baru 1")
• Input jumlah grup yang akan dibuat
• Program akan membuat grup dengan nama "Grup baru 1", "Grup baru 2", dst

**Opsi 2: Multi-line**
• Input nama grup per baris
• Setiap baris = 1 grup
• Contoh:
  GRUP
  GRUP
  GRUP

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📝 Alur Proses:**
1. Pilih mode (Opsi 1 atau 2)
2. Input nama grup sesuai mode
3. Input nomor telepon (atau skip)
4. Input pengaturan grup (ON/OFF untuk 5 pengaturan)
5. Program akan membuat grup dan menerapkan pengaturan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih mode yang Anda inginkan`

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1️⃣ Opsi 1: Nama + Jumlah", "create_group_mode_single"),
			tgbotapi.NewInlineKeyboardButtonData("2️⃣ Opsi 2: Multi-line", "create_group_mode_multiline"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "menu_grup"),
		),
	)
	editMsg.ReplyMarkup = &keyboard

	telegramBot.Send(editMsg)
}

// StartCreateGroupProcess memulai proses buat grup (mode single)
func StartCreateGroupProcessSingle(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	groupCreateStates[chatID] = &GroupCreateState{
		Mode:                "single",
		WaitingForGroupName: true,
		WaitingForCount:     false,
		WaitingForNumbers:   false,
		WaitingForSettings:  false,
		CurrentSettingIndex: -1,
		GroupNames:          []string{},
		GroupCount:          0,
		BaseName:            "",
		PhoneNumbers:        []string{},
		DelaySeconds:        2,
		MessageLogging:      nil,
		MemberAdd:           nil,
		JoinApproval:        nil,
		Ephemeral:           nil,
		EditSettings:        nil,
	}

	promptMsg := `📝 **MASUKKAN NAMA GRUP (Opsi 1)**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Ketik nama untuk grup yang akan dibuat.

**Contoh:**
• Grup baru 1
• Test Group
• My Group

**Catatan:**
• Program akan membuat grup dengan nama "Grup baru 1", "Grup baru 2", dst
• Anda akan diminta jumlah grup setelah ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nama grup...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_create_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// StartCreateGroupProcessMultiline memulai proses buat grup (mode multiline)
func StartCreateGroupProcessMultiline(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	groupCreateStates[chatID] = &GroupCreateState{
		Mode:                "multiline",
		WaitingForGroupName: true,
		WaitingForCount:     false,
		WaitingForNumbers:   false,
		WaitingForSettings:  false,
		CurrentSettingIndex: -1,
		GroupNames:          []string{},
		GroupCount:          0,
		BaseName:            "",
		PhoneNumbers:        []string{},
		DelaySeconds:        2,
		MessageLogging:      nil,
		MemberAdd:           nil,
		JoinApproval:        nil,
		Ephemeral:           nil,
		EditSettings:        nil,
	}

	promptMsg := `📝 **MASUKKAN NAMA GRUP (Opsi 2: Multi-line)**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Ketik nama grup, setiap baris = 1 grup.

**Contoh:**
GRUP
GRUP
GRUP
GRUP

**Catatan:**
• Setiap baris akan menjadi nama 1 grup
• Minimal 1 grup
• Maksimal 100 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nama grup (multi-line)...`

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_create_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForCreate memproses input nama grup
func HandleGroupNameInputForCreate(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
	if state == nil || !state.WaitingForGroupName {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nama grup tidak boleh kosong!")
		telegramBot.Send(errorMsg)
		return
	}

	if state.Mode == "single" {
		// Opsi 1: Simpan base name, lalu tanya jumlah
		state.BaseName = input
		state.WaitingForGroupName = false
		state.WaitingForCount = true

		promptMsg := fmt.Sprintf(`🔢 **MASUKKAN JUMLAH GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Nama grup: **%s**

Berapa grup yang akan dibuat?

**Contoh:**
• 5 (akan membuat: %s, %s 2, %s 3, %s 4, %s 5)
• 10
• 20

**Catatan:**
• Minimal 1 grup
• Maksimal 100 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka jumlah grup...`, state.BaseName, state.BaseName, state.BaseName, state.BaseName, state.BaseName, state.BaseName)

		msg := tgbotapi.NewMessage(chatID, promptMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_create_group"),
			),
		)
		msg.ReplyMarkup = keyboard

		telegramBot.Send(msg)
	} else {
		// Opsi 2: Parse multi-line
		lines := strings.Split(input, "\n")
		var names []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				names = append(names, line)
			}
		}

		if len(names) == 0 {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Setidaknya harus ada 1 nama grup!")
			telegramBot.Send(errorMsg)
			return
		}

		if len(names) > 100 {
			errorMsg := tgbotapi.NewMessage(chatID, "❌ Maksimal 100 grup per proses!")
			telegramBot.Send(errorMsg)
			return
		}

		state.GroupNames = names
		state.WaitingForGroupName = false

		// Langsung ke input nomor
		askForPhoneNumbers(chatID, telegramBot, state)
	}
}

// HandleCountInputForCreate memproses input jumlah grup (opsi 1 saja)
func HandleCountInputForCreate(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
	if state == nil || !state.WaitingForCount {
		return
	}

	count, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || count < 1 || count > 100 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Jumlah grup harus antara 1-100!")
		telegramBot.Send(errorMsg)
		return
	}

	state.GroupCount = count

	// Generate group names
	state.GroupNames = []string{}
	for i := 1; i <= count; i++ {
		groupName := state.BaseName
		if count > 1 {
			groupName = fmt.Sprintf("%s %d", state.BaseName, i)
		}
		state.GroupNames = append(state.GroupNames, groupName)
	}

	state.WaitingForCount = false

	// Langsung ke input nomor
	askForPhoneNumbers(chatID, telegramBot, state)
}

// askForPhoneNumbers meminta input nomor telepon
func askForPhoneNumbers(chatID int64, telegramBot *tgbotapi.BotAPI, state *GroupCreateState) {
	totalGroups := len(state.GroupNames)

	promptMsg := fmt.Sprintf(`📱 **MASUKKAN NOMOR TELEPON**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total grup:** %d grup

**Cara Input:**
• Ketik nomor telepon (satu per baris)
• Contoh:
  628123456789
  628987654321
  628111222333

**Format Nomor:**
• Gunakan format internasional (62XXXXXXXXX)
• Atau format lokal (08XXXXXXXXX) - akan dikonversi otomatis
• Minimal 1 nomor
• Maksimal 256 anggota per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik nomor telepon (multi-line) atau klik **SKIP** untuk membuat grup tanpa menambahkan anggota

`, totalGroups)

	msg := tgbotapi.NewMessage(chatID, promptMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "create_group_skip_numbers"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_create_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
	state.WaitingForNumbers = true
}

// HandlePhoneNumbersInputForCreate memproses input nomor telepon
func HandlePhoneNumbersInputForCreate(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
	if state == nil || !state.WaitingForNumbers {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Nomor telepon tidak boleh kosong! Atau klik SKIP jika tidak ingin menambahkan anggota.")
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

	if len(numbers) > 256 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Maksimal 256 nomor per grup!")
		telegramBot.Send(errorMsg)
		return
	}

	state.PhoneNumbers = numbers
	state.WaitingForNumbers = false
	state.WaitingForDelay = true

	// Confirm and ask for delay
	confirmMsg := fmt.Sprintf(`✅ **NOMOR TERDAFTAR**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📱 **Total nomor:** %d nomor
📊 **Total grup:** %d grup

**Catatan:**
Semua nomor ini akan ditambahkan ke SEMUA grup yang dibuat.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay antar grup saat pembuatan.

**Rekomendasi:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`, len(numbers), len(state.GroupNames))

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_create_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleSkipPhoneNumbers menangani skip nomor telepon
func HandleSkipPhoneNumbers(chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
	if state == nil || !state.WaitingForNumbers {
		return
	}

	state.PhoneNumbers = []string{}
	state.WaitingForNumbers = false
	state.WaitingForDelay = true

	confirmMsg := fmt.Sprintf(`⏭️ **NOMOR DI-SKIP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Grup akan dibuat TANPA menambahkan anggota lain.

📊 **Total grup:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay antar grup saat pembuatan.

**Rekomendasi:**
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 2)`, len(state.GroupNames))

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_create_group"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// askForNextCreateSetting menanyakan pengaturan berikutnya
func askForNextCreateSetting(chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
	if state == nil {
		return
	}

	if state.CurrentSettingIndex >= len(createGroupSettingsList) {
		// All settings asked - start creating groups
		state.WaitingForSettings = false
		return
	}

	settingName := createGroupSettingsList[state.CurrentSettingIndex]

	var settingMsg string
	var keyboard tgbotapi.InlineKeyboardMarkup

	switch settingName {
	case "message_logging":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 1/5: ATUR PESAN GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📢 **Send New Messages**

Mengatur siapa yang bisa mengirim pesan di grup.

**Penjelasan:**
• ✅ **ON** - Semua anggota bisa kirim pesan
• ❌ **OFF** - Hanya admin yang bisa kirim pesan
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "create_group_setting_msg_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "create_group_setting_msg_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "create_group_setting_msg_skip"),
			),
		)

	case "member_add":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 2/5: ATUR TAMBAH ANGGOTA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

👥 **Who Can Add Members**

Mengatur siapa yang bisa menambahkan anggota baru.

**Penjelasan:**
• ✅ **ON** - Semua anggota bisa tambah anggota
• ❌ **OFF** - Hanya admin yang bisa tambah anggota
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "create_group_setting_member_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "create_group_setting_member_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "create_group_setting_member_skip"),
			),
		)

	case "join_approval":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 3/5: ATUR PERSETUJUAN ANGGOTA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Join Approval**

Mengatur apakah anggota baru perlu persetujuan admin.

**Penjelasan:**
• ✅ **ON** - Perlu persetujuan admin
• ❌ **OFF** - Tidak perlu persetujuan
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "create_group_setting_approval_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "create_group_setting_approval_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "create_group_setting_approval_skip"),
			),
		)

	case "ephemeral":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 4/5: ATUR PESAN SEMENTARA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **Disappearing Messages**

Mengatur durasi pesan sementara.

**Penjelasan:**
• ❌ **OFF** - Pesan tidak hilang
• ⏱️ **24 JAM** - Pesan hilang dalam 24 jam
• ⏱️ **7 HARI** - Pesan hilang dalam 7 hari
• ⏱️ **90 HARI** - Pesan hilang dalam 90 hari
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "create_group_setting_ephemeral_off"),
				tgbotapi.NewInlineKeyboardButtonData("⏱️ 24 JAM", "create_group_setting_ephemeral_24h"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏱️ 7 HARI", "create_group_setting_ephemeral_7d"),
				tgbotapi.NewInlineKeyboardButtonData("⏱️ 90 HARI", "create_group_setting_ephemeral_90d"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "create_group_setting_ephemeral_skip"),
			),
		)

	case "edit_settings":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 5/5: ATUR EDIT GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔧 **Edit Group Settings**

Mengatur siapa yang bisa mengedit pengaturan grup.

**Penjelasan:**
• ✅ **ON** - Semua anggota bisa edit
• ❌ **OFF** - Hanya admin yang bisa edit
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "create_group_setting_edit_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "create_group_setting_edit_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "create_group_setting_edit_skip"),
			),
		)
	}

	msg := tgbotapi.NewMessage(chatID, settingMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = &keyboard
	telegramBot.Send(msg)
}

// HandleCreateGroupSettingChoice menangani pilihan pengaturan
func HandleCreateGroupSettingChoice(settingName, choice string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
	if state == nil || !state.WaitingForSettings {
		return
	}

	// Save setting choice
	switch settingName {
	case "message_logging":
		if choice == "on" {
			val := true
			state.MessageLogging = &val
		} else if choice == "off" {
			val := false
			state.MessageLogging = &val
		}
	case "member_add":
		if choice == "on" {
			val := true
			state.MemberAdd = &val
		} else if choice == "off" {
			val := false
			state.MemberAdd = &val
		}
	case "join_approval":
		if choice == "on" {
			val := true
			state.JoinApproval = &val
		} else if choice == "off" {
			val := false
			state.JoinApproval = &val
		}
	case "ephemeral":
		if choice == "off" {
			val := int64(0)
			state.Ephemeral = &val
		} else if choice == "24h" {
			val := int64(86400)
			state.Ephemeral = &val
		} else if choice == "7d" {
			val := int64(604800)
			state.Ephemeral = &val
		} else if choice == "90d" {
			val := int64(7776000)
			state.Ephemeral = &val
		}
	case "edit_settings":
		if choice == "on" {
			val := true
			state.EditSettings = &val
		} else if choice == "off" {
			val := false
			state.EditSettings = &val
		}
	}

	// Move to next setting
	state.CurrentSettingIndex++

	if state.CurrentSettingIndex >= len(createGroupSettingsList) {
		// All settings done - start creating groups
		state.WaitingForSettings = false

		// Count settings
		settingsCount := 0
		if state.MessageLogging != nil {
			settingsCount++
		}
		if state.MemberAdd != nil {
			settingsCount++
		}
		if state.JoinApproval != nil {
			settingsCount++
		}
		if state.Ephemeral != nil {
			settingsCount++
		}
		if state.EditSettings != nil {
			settingsCount++
		}

		startMsg := fmt.Sprintf(`✅ **MEMULAI PROSES**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚙️ **Pengaturan:** %d pengaturan per grup
📱 **Anggota:** %d nomor (akan ditambahkan ke semua grup)
⏱️ **Delay:** %d detik per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 Memproses pembuatan grup...`, settingsCount, len(state.PhoneNumbers), state.DelaySeconds)

		msg := tgbotapi.NewMessage(chatID, startMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)

		// Process in goroutine
		go ProcessCreateGroups(state, chatID, client, telegramBot)
	} else {
		// Ask for next setting
		askForNextCreateSetting(chatID, telegramBot)
	}
}

// ProcessCreateGroups memproses pembuatan grup
func ProcessCreateGroups(state *GroupCreateState, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	if client == nil || client.Store.ID == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(msg)
		return
	}

	totalGroups := len(state.GroupNames)
	successCount := 0
	failedCount := 0
	var failedGroups []string
	var createdGroups []*types.GroupInfo
	var createdGroupsWithLinks []struct {
		Group *types.GroupInfo
		Link  string
		Name  string
	}

	var progressMsgSent *tgbotapi.Message

	// Convert phone numbers to JIDs
	participants := []types.JID{}
	for _, phone := range state.PhoneNumbers {
		jid, err := parseJIDFromString(phone + "@s.whatsapp.net")
		if err == nil {
			participants = append(participants, jid)
		}
	}

	for i, groupName := range state.GroupNames {
		// HIGH FIX: Ambil active client di setiap iterasi (create grup = operasi BERAT!)
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessCreateGroups", i, totalGroups)
		if shouldStop {
			// Client disconnect - stop proses
			disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada grup %d/%d\n\n✅ Berhasil: %d\n❌ Gagal: %d", i+1, totalGroups, successCount, failedCount)
			notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
			notifMsg.ParseMode = "Markdown"
			telegramBot.Send(notifMsg)
			break
		}

		// Create group WITH settings applied at creation time
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// Build ReqCreateGroup with settings embedded (applied BEFORE group is created)
		req := buildCreateGroupRequest(groupName, participants, state)

		groupInfo, err := validClient.CreateGroup(ctx, req)
		cancel()

		if err != nil {
			failedCount++
			failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (%v)", groupName, err))

			// Show progress
			if totalGroups > 1 {
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

			time.Sleep(time.Duration(state.DelaySeconds) * time.Second)
			continue
		}

		// Group created successfully WITH settings already applied
		// FIXED: result of append digunakan, jadi tidak ada bug
		createdGroups = append(createdGroups, groupInfo)
		successCount++

		// Apply MemberAdd setting IMMEDIATELY after creation (not supported in CreateGroup)
		// Using the same context to ensure it's part of the same operation
		// This minimizes notification appearance
		if state.MemberAdd != nil {
			jid := groupInfo.JID
			// Apply immediately without creating new context delay
			var addMode types.GroupMemberAddMode
			if *state.MemberAdd {
				addMode = types.GroupMemberAddModeAllMember
			} else {
				addMode = types.GroupMemberAddModeAdmin
			}
			// Use very short timeout context to apply as fast as possible
			ctxMember, cancelMember := context.WithTimeout(context.Background(), 5*time.Second)
			validClient.SetGroupMemberAddMode(ctxMember, jid, addMode)
			cancelMember()
		}

		// Get group invite link immediately after creation
		// IMPORTANT: Gunakan validClient untuk mencegah client stale
		jid := groupInfo.JID
		link := ""
		ctxLink, cancelLink := context.WithTimeout(context.Background(), 10*time.Second)
		inviteLink, errLink := validClient.GetGroupInviteLink(ctxLink, jid, false)
		cancelLink()
		if errLink == nil {
			link = inviteLink
		} else {
			// Log error untuk debugging
			utils.GetGrupLogger().Warn("Gagal mengambil link grup %s (ID: %s): %v", groupName, jid.String(), errLink)
		}

		// Store group with link for summary
		createdGroupsWithLinks = append(createdGroupsWithLinks, struct {
			Group *types.GroupInfo
			Link  string
			Name  string
		}{
			Group: groupInfo,
			Link:  link,
			Name:  groupName,
		})

		// Show progress
		if totalGroups > 1 {
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

		// Delay between groups
		if i < totalGroups-1 {
			time.Sleep(time.Duration(state.DelaySeconds) * time.Second)
		}
	}

	// Final summary
	summaryMsg := fmt.Sprintf(`🎉 **SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 **RINGKASAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ **Berhasil:** %d grup
❌ **Gagal:** %d grup
⏱️ **Delay:** %d detik/grup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, successCount, failedCount, state.DelaySeconds)

	if failedCount > 0 {
		summaryMsg += fmt.Sprintf("\n\n**Grup yang Gagal:**\n")
		for i, failed := range failedGroups {
			if i < 20 { // Limit display
				summaryMsg += failed + "\n"
			}
		}
		if len(failedGroups) > 20 {
			summaryMsg += fmt.Sprintf("\n... dan %d grup lainnya\n", len(failedGroups)-20)
		}
	}

	if successCount > 0 {
		summaryMsg += fmt.Sprintf("\n\n**✅ Grup yang Berhasil Dibuat:**\n\n")

		// Tampilkan max 10 grup di pesan summary utama
		maxDisplayInSummary := 10
		for i, groupData := range createdGroupsWithLinks {
			if i < maxDisplayInSummary {
				if groupData.Link != "" {
					summaryMsg += fmt.Sprintf("**%s**\n🔗 %s\n\n", groupData.Name, groupData.Link)
				} else {
					summaryMsg += fmt.Sprintf("**%s**\n⚠️ Link tidak tersedia\n\n", groupData.Name)
				}
			}
		}
	}

	finalMsg := tgbotapi.NewMessage(chatID, summaryMsg)
	finalMsg.ParseMode = "Markdown"
	telegramBot.Send(finalMsg)

	// Jika ada lebih dari 10 grup, kirim sisa grup di pesan terpisah
	if len(createdGroupsWithLinks) > 10 {
		additionalGroupsMsg := "**📋 Lanjutan Grup yang Berhasil Dibuat:**\n\n"
		for i := 10; i < len(createdGroupsWithLinks); i++ {
			groupData := createdGroupsWithLinks[i]
			if groupData.Link != "" {
				additionalGroupsMsg += fmt.Sprintf("**%s**\n🔗 %s\n\n", groupData.Name, groupData.Link)
			} else {
				additionalGroupsMsg += fmt.Sprintf("**%s**\n⚠️ Link tidak tersedia\n\n", groupData.Name)
			}
		}

		additionalMsg := tgbotapi.NewMessage(chatID, additionalGroupsMsg)
		additionalMsg.ParseMode = "Markdown"
		telegramBot.Send(additionalMsg)
	}

	// Save all created groups to database
	if len(createdGroupsWithLinks) > 0 {
		groupsToSave := make(map[string]string)
		for _, groupData := range createdGroupsWithLinks {
			if groupData.Group != nil {
				jidStr := groupData.Group.JID.String()
				groupName := groupData.Name
				groupsToSave[jidStr] = groupName
			}
		}

		if len(groupsToSave) > 0 {
			if err := utils.BatchSaveGroupsToDB(groupsToSave); err != nil {
				utils.GetGrupLogger().Error("ProcessCreateGroups: Gagal save grup ke database: %v", err)
				// Fallback: save individual jika batch gagal
				for jid, name := range groupsToSave {
					go utils.SaveGroupToDB(jid, name)
				}
			} else {
				utils.GetGrupLogger().Info("ProcessCreateGroups: Berhasil save %d grup ke database", len(groupsToSave))
			}
		}
	}

	// Clear state
	delete(groupCreateStates, chatID)
}

// buildCreateGroupRequest membangun ReqCreateGroup dengan settings yang diterapkan SEBELUM grup dibuat
func buildCreateGroupRequest(groupName string, participants []types.JID, state *GroupCreateState) whatsmeow.ReqCreateGroup {
	req := whatsmeow.ReqCreateGroup{
		Name:         groupName,
		Participants: participants,
	}

	// Apply Message Logging (GroupAnnounce) - applied at creation
	if state.MessageLogging != nil {
		req.GroupAnnounce = types.GroupAnnounce{
			IsAnnounce:        !*state.MessageLogging, // false = ON (all can send), true = OFF (only admin)
			AnnounceVersionID: "",
		}
	}

	// Apply Join Approval (GroupMembershipApprovalMode) - applied at creation
	if state.JoinApproval != nil {
		req.GroupMembershipApprovalMode = types.GroupMembershipApprovalMode{
			IsJoinApprovalRequired: *state.JoinApproval,
		}
	}

	// Apply Ephemeral Messages (GroupEphemeral) - applied at creation
	if state.Ephemeral != nil {
		isEphemeral := *state.Ephemeral > 0
		req.GroupEphemeral = types.GroupEphemeral{
			IsEphemeral:       isEphemeral,
			DisappearingTimer: uint32(*state.Ephemeral),
		}
	}

	// Apply Edit Settings (GroupLocked) - applied at creation
	if state.EditSettings != nil {
		req.GroupLocked = types.GroupLocked{
			IsLocked: !*state.EditSettings, // false = ON (all can edit), true = OFF (only admin)
		}
	}

	return req
}

// Note: MemberAdd setting cannot be applied at group creation time
// because ReqCreateGroup does not support GroupMemberAddMode field.
// It must be applied immediately after creation with minimal delay.

// cleanPhoneNumber membersihkan dan memvalidasi nomor telepon
func cleanPhoneNumber(phone string) string {
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

// CancelCreateGroup membatalkan proses buat grup
func CancelCreateGroup(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(groupCreateStates, chatID)

	msg := tgbotapi.NewMessage(chatID, "❌ Proses buat grup otomatis dibatalkan.")
	telegramBot.Send(msg)
}

// HandleDelayInputForCreate memproses input delay
func HandleDelayInputForCreate(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupCreateStates[chatID]
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
	state.WaitingForSettings = true
	state.CurrentSettingIndex = 0

	// Start asking for settings
	askForNextCreateSetting(chatID, telegramBot)
}

// IsWaitingForCreateGroupInput checks if user is waiting to input
func IsWaitingForCreateGroupInput(chatID int64) bool {
	state := groupCreateStates[chatID]
	return state != nil && (state.WaitingForGroupName || state.WaitingForCount || state.WaitingForNumbers || state.WaitingForDelay)
}

// GetCreateGroupInputType returns the current input type
func GetCreateGroupInputType(chatID int64) string {
	state := groupCreateStates[chatID]
	if state == nil {
		return ""
	}

	if state.WaitingForGroupName {
		return "group_name"
	}
	if state.WaitingForCount {
		return "count"
	}
	if state.WaitingForNumbers {
		return "phone_numbers"
	}
	if state.WaitingForDelay {
		return "delay"
	}

	return ""
}
