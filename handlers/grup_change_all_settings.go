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
	"go.mau.fi/whatsmeow/types"
)

// GroupAllSettingsState manages the state for changing all group settings at once
type GroupAllSettingsState struct {
	WaitingForGroupName bool
	WaitingForDelay     bool
	CurrentSettingIndex int
	SelectedGroups      []GroupLinkInfo
	Keyword             string
	DelaySeconds        int

	// Settings to apply
	MessageLogging *bool  // nil = skip, true = ON, false = OFF
	MemberAdd      *bool  // nil = skip, true = ON, false = OFF
	JoinApproval   *bool  // nil = skip, true = ON, false = OFF
	Ephemeral      *int64 // nil = skip, 0 = OFF, 86400 = 24h, 604800 = 7d, 7776000 = 90d
	EditSettings   *bool  // nil = skip, true = ON, false = OFF
}

var groupAllSettingsStates = make(map[int64]*GroupAllSettingsState)

// Settings list in order
var settingsList = []string{
	"message_logging",
	"member_add",
	"join_approval",
	"ephemeral",
	"edit_settings",
}

// ShowChangeAllSettingsMenu menampilkan menu atur semua pengaturan grup
func ShowChangeAllSettingsMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `⚙️ **ATUR SEMUA PENGATURAN GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengatur SEMUA pengaturan grup WhatsApp yang Anda pilih dalam satu proses. Anda akan ditanyakan untuk setiap pengaturan dan bisa memilih ON/OFF atau Skip untuk melewatinya.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Atur Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk mengatur ini
• Delay membantu menghindari rate limit WhatsApp
• Anda bisa Skip pengaturan yang tidak ingin diubah
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Pengaturan yang Tersedia:**
1. 📢 Atur Pesan (Send new messages)
2. 👥 Atur Tambah Anggota
3. ✅ Atur Persetujuan
4. ⏱️ Atur Pesan Sementara
5. 🔧 Atur Edit Grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_all_settings"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_all_settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Atur Semua", "change_all_settings_all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "all_settings_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// ShowChangeAllSettingsMenuEdit menampilkan menu dengan EDIT message (no spam!)
func ShowChangeAllSettingsMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `⚙️ **ATUR SEMUA PENGATURAN GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengatur SEMUA pengaturan grup WhatsApp yang Anda pilih dalam satu proses. Anda akan ditanyakan untuk setiap pengaturan dan bisa memilih ON/OFF atau Skip untuk melewatinya.

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Atur Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk mengatur ini
• Delay membantu menghindari rate limit WhatsApp
• Anda bisa Skip pengaturan yang tidak ingin diubah
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Pengaturan yang Tersedia:**
1. 📢 Atur Pesan (Send new messages)
2. 👥 Atur Tambah Anggota
3. ✅ Atur Persetujuan
4. ⏱️ Atur Pesan Sementara
5. 🔧 Atur Edit Grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat & Pilih", "show_group_list_all_settings"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Manual", "start_change_all_settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Atur Semua", "change_all_settings_all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Lihat Contoh", "all_settings_example"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// ShowAllSettingsExampleEdit menampilkan contoh penggunaan
func ShowAllSettingsExampleEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	exampleMsg := `📖 **CONTOH PENGGUNAAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Metode 1: Lihat & Pilih**
1️⃣ Klik "📋 Lihat & Pilih"
2️⃣ Bot tampilkan daftar grup
3️⃣ Ketik nomor grup (misal: 1,3,5)
4️⃣ Tentukan delay (misal: 2 detik)
5️⃣ Jawab pertanyaan untuk setiap pengaturan:
   • Klik ✅ ON atau ❌ OFF untuk mengatur
   • Klik ⏭️ Skip untuk melewati
6️⃣ Bot proses semua pengaturan
7️⃣ Hasil dikirim

**🔍 Metode 2: Cari Manual**
1️⃣ Klik "🔍 Cari Manual"
2️⃣ Ketik kata kunci (misal: "Keluarga")
3️⃣ Bot tampilkan hasil pencarian
4️⃣ Tentukan delay
5️⃣ Jawab semua pertanyaan pengaturan
6️⃣ Bot proses semua pengaturan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan Skip jika tidak ingin mengubah pengaturan tertentu
• Semua pengaturan akan diproses dalam satu batch
• Delay berlaku untuk setiap pengaturan per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_all_settings_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, exampleMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartChangeAllSettingsProcess memulai proses atur semua pengaturan
func StartChangeAllSettingsProcess(telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Initialize state
	groupAllSettingsStates[chatID] = &GroupAllSettingsState{
		WaitingForGroupName: true,
		WaitingForDelay:     false,
		CurrentSettingIndex: -1,
		SelectedGroups:      []GroupLinkInfo{},
		Keyword:             "",
		DelaySeconds:        0,
		MessageLogging:      nil,
		MemberAdd:           nil,
		JoinApproval:        nil,
		Ephemeral:           nil,
		EditSettings:        nil,
	}

	promptMsg := `🔍 **MASUKKAN NAMA GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Mode Input Aktif**

Ketik nama grup atau kata kunci untuk mencari grup yang ingin diatur semua pengaturannya.

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
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_all_settings"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleGroupNameInputForAllSettings memproses input nama grup
func HandleGroupNameInputForAllSettings(keyword string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupAllSettingsStates[chatID]
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

	// Smart search logic
	if keyword == "." {
		groups, err = utils.GetAllGroupsFromDB()
	} else {
		lines := strings.Split(keyword, "\n")
		if len(lines) > 1 {
			groups, err = utils.SearchGroupsExactMultiple(lines)
		} else if len(keyword) > 30 {
			groups, err = utils.SearchGroupsExact(keyword)
			if err == nil && len(groups) == 0 {
				groups, err = utils.SearchGroupsFlexible(keyword)
			}
		} else {
			groups, err = utils.SearchGroupsFlexible(keyword)
		}
	}

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

Silakan coba lagi atau klik tombol di bawah.`, keyword)

		msg := tgbotapi.NewMessage(chatID, noResultMsg)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Lagi", "start_change_all_settings"),
				tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_all_settings_menu"),
			),
		)
		msg.ReplyMarkup = keyboard
		telegramBot.Send(msg)

		state.WaitingForGroupName = false
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

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay ANTAR GRUP untuk menghindari rate limit.

**Contoh:** 2 (delay 2 detik sebelum lanjut ke grup berikutnya)

💡 **Rekomendasi:**
• < 10 grup: 1-2 detik
• 10-30 grup: 2-3 detik
• > 30 grup: 3-5 detik

**💡 Catatan Penting:**
• Delay digunakan untuk jeda ANTAR GRUP
• Semua pengaturan dalam 1 grup diproses SEKALIGUS (tanpa delay)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu input delay...`

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_all_settings"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleDelayInputForAllSettings memproses input delay
func HandleDelayInputForAllSettings(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupAllSettingsStates[chatID]
	if state == nil || !state.WaitingForDelay {
		return
	}

	input = strings.TrimSpace(input)

	var delay int
	_, err := fmt.Sscanf(input, "%d", &delay)
	if err != nil || delay < 0 || delay > 60 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Input tidak valid!\n\nDelay harus berupa angka antara 0-60 detik.\n\nContoh: 2, 5, 10")
		telegramBot.Send(errorMsg)
		return
	}

	state.DelaySeconds = delay
	state.WaitingForDelay = false
	state.CurrentSettingIndex = 0

	// Start asking for settings
	AskForNextSetting(chatID, telegramBot)
}

// AskForNextSetting menanyakan pengaturan berikutnya
func AskForNextSetting(chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := groupAllSettingsStates[chatID]
	if state == nil {
		return
	}

	if state.CurrentSettingIndex >= len(settingsList) {
		// All settings asked - will be processed when user makes last choice
		// ProcessAllSettings will be called from HandleSettingChoiceForAllSettings
		return
	}

	settingName := settingsList[state.CurrentSettingIndex]

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

💡 Klik button untuk memilih atau Skip...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "all_settings_msg_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "all_settings_msg_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "all_settings_msg_skip"),
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

💡 Klik button untuk memilih atau Skip...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "all_settings_member_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "all_settings_member_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "all_settings_member_skip"),
			),
		)

	case "join_approval":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 3/5: ATUR PERSETUJUAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Join Approval**

Mengatur apakah admin perlu menyetujui anggota baru.

**Penjelasan:**
• ✅ **ON** - Admin harus setujui anggota baru
• ❌ **OFF** - Anggota bisa langsung bergabung
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih atau Skip...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "all_settings_approval_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "all_settings_approval_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "all_settings_approval_skip"),
			),
		)

	case "ephemeral":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 4/5: ATUR PESAN SEMENTARA**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **Disappearing Messages**

Mengatur durasi pesan sementara (pesan akan terhapus otomatis).

**Penjelasan:**
• ❌ **OFF** - Nonaktifkan pesan sementara
• ⏰ **24 Jam** - Pesan terhapus setelah 24 jam
• 📅 **7 Hari** - Pesan terhapus setelah 7 hari
• 🗓️ **90 Hari** - Pesan terhapus setelah 90 hari
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih atau Skip...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "all_settings_ephemeral_off"),
				tgbotapi.NewInlineKeyboardButtonData("⏰ 24 Jam", "all_settings_ephemeral_24h"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📅 7 Hari", "all_settings_ephemeral_7d"),
				tgbotapi.NewInlineKeyboardButtonData("🗓️ 90 Hari", "all_settings_ephemeral_90d"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "all_settings_ephemeral_skip"),
			),
		)

	case "edit_settings":
		settingMsg = fmt.Sprintf(`✅ **PENGATURAN 5/5: ATUR EDIT GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔧 **Edit Group Settings**

Mengatur siapa yang bisa mengedit pengaturan grup.

**Penjelasan:**
• ✅ **ON** - Semua anggota bisa edit pengaturan
• ❌ **OFF** - Hanya admin yang bisa edit pengaturan
• ⏭️ **SKIP** - Lewati pengaturan ini

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Klik button untuk memilih atau Skip...`)

		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ ON", "all_settings_edit_on"),
				tgbotapi.NewInlineKeyboardButtonData("❌ OFF", "all_settings_edit_off"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⏭️ SKIP", "all_settings_edit_skip"),
			),
		)
	}

	msg := tgbotapi.NewMessage(chatID, settingMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// HandleSettingChoiceForAllSettings memproses pilihan pengaturan
func HandleSettingChoiceForAllSettings(settingName string, choice string, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := groupAllSettingsStates[chatID]
	if state == nil || state.CurrentSettingIndex < 0 {
		return
	}

	// Check if this is the current setting being asked
	currentSetting := settingsList[state.CurrentSettingIndex]
	if currentSetting != settingName {
		return // Not the current setting
	}

	// Process choice based on setting type
	switch settingName {
	case "message_logging":
		if choice == "on" {
			val := true
			state.MessageLogging = &val
		} else if choice == "off" {
			val := false
			state.MessageLogging = &val
		} else if choice == "skip" {
			// nil = skip
		}

	case "member_add":
		if choice == "on" {
			val := true
			state.MemberAdd = &val
		} else if choice == "off" {
			val := false
			state.MemberAdd = &val
		} else if choice == "skip" {
			// nil = skip
		}

	case "join_approval":
		if choice == "on" {
			val := true
			state.JoinApproval = &val
		} else if choice == "off" {
			val := false
			state.JoinApproval = &val
		} else if choice == "skip" {
			// nil = skip
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
		} else if choice == "skip" {
			// nil = skip
		}

	case "edit_settings":
		if choice == "on" {
			val := true
			state.EditSettings = &val
		} else if choice == "off" {
			val := false
			state.EditSettings = &val
		} else if choice == "skip" {
			// nil = skip
		}
	}

	// Move to next setting
	state.CurrentSettingIndex++

	// Check if all settings are done
	if state.CurrentSettingIndex >= len(settingsList) {
		// All settings asked, start processing
		ProcessAllSettings(chatID, client, telegramBot)
	} else {
		// Ask next setting
		AskForNextSetting(chatID, telegramBot)
	}
}

// ProcessAllSettings memproses semua pengaturan yang dipilih
func ProcessAllSettings(chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	state := groupAllSettingsStates[chatID]
	if state == nil {
		return
	}

	if client == nil || client.Store.ID == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(msg)
		return
	}

	// Count settings to apply
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

	if settingsCount == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Tidak ada pengaturan yang dipilih. Semua pengaturan di-skip.")
		telegramBot.Send(msg)
		delete(groupAllSettingsStates, chatID)
		return
	}

	startMsg := fmt.Sprintf(`✅ **MEMULAI PROSES**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚙️ **Pengaturan:** %d pengaturan per grup
⏱️ **Delay:** %d detik per pengaturan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🚀 Memproses semua pengaturan...
`, settingsCount, state.DelaySeconds)

	msg := tgbotapi.NewMessage(chatID, startMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Process in goroutine
	go ProcessAllSettingsBatch(state.SelectedGroups, state.DelaySeconds, state, chatID, client, telegramBot)

	// State will be cleared after processing completes in ProcessAllSettingsBatch
}

// ProcessAllSettingsBatch memproses batch semua pengaturan
func ProcessAllSettingsBatch(groups []GroupLinkInfo, delay int, state *GroupAllSettingsState, chatID int64, client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI) {
	totalGroups := len(groups)

	// Count actual settings to apply
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

	totalOps := totalGroups * settingsCount
	currentOp := 0
	successCount := 0
	failedCount := 0
	var failedGroups []string

	var progressMsgSent *tgbotapi.Message

	for i, group := range groups {
		// CRITICAL FIX: Ambil active client di setiap iterasi untuk proses SANGAT panjang!
		// ProcessAllSettingsBatch bisa berjalan 1+ jam untuk banyak grup × banyak settings
		validClient, shouldStop := ValidateClientForBackgroundProcess(client, "ProcessAllSettingsBatch", i, totalGroups)
		if shouldStop {
			// Client disconnect - stop proses dan kirim notifikasi
			disconnectMsg := fmt.Sprintf("⚠️ **PROSES DIHENTIKAN**\n\nClient WhatsApp terputus pada grup %d/%d\n\n✅ Berhasil: %d operasi\n❌ Gagal: %d operasi", i+1, totalGroups, successCount, failedCount)
			notifMsg := tgbotapi.NewMessage(chatID, disconnectMsg)
			notifMsg.ParseMode = "Markdown"
			telegramBot.Send(notifMsg)
			break
		}

		jid, err := parseJIDFromString(group.JID)
		if err != nil {
			failedCount++
			failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (invalid JID)", group.Name))
			continue
		}

		// Apply ALL settings for this group WITHOUT delay between settings
		// Delay hanya digunakan ANTAR GRUP, bukan antar pengaturan

		if state.MessageLogging != nil {
			currentOp++
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel() // FIXED: Use defer to ensure cancellation
			if *state.MessageLogging {
				err = validClient.SetGroupAnnounce(ctx, jid, false)
			} else {
				err = validClient.SetGroupAnnounce(ctx, jid, true)
			}

			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (Pesan: %v)", group.Name, err))
			} else {
				successCount++
			}
		}

		if state.MemberAdd != nil {
			currentOp++
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel() // FIXED: Use defer to ensure cancellation
			var addMode types.GroupMemberAddMode
			if *state.MemberAdd {
				addMode = types.GroupMemberAddModeAllMember
			} else {
				addMode = types.GroupMemberAddModeAdmin
			}
			err = validClient.SetGroupMemberAddMode(ctx, jid, addMode)

			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (Tambah Anggota: %v)", group.Name, err))
			} else {
				successCount++
			}
		}

		if state.JoinApproval != nil {
			currentOp++
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel() // FIXED: Use defer to ensure cancellation
			err = validClient.SetGroupJoinApprovalMode(ctx, jid, *state.JoinApproval)

			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (Persetujuan: %v)", group.Name, err))
			} else {
				successCount++
			}
		}

		if state.Ephemeral != nil {
			currentOp++
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			timer := time.Duration(*state.Ephemeral) * time.Second
			settingTS := time.Now()
			err = validClient.SetDisappearingTimer(ctx, jid, timer, settingTS)
			cancel()

			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (Pesan Sementara: %v)", group.Name, err))
			} else {
				successCount++
			}
		}

		if state.EditSettings != nil {
			currentOp++
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			locked := !*state.EditSettings
			err = validClient.SetGroupLocked(ctx, jid, locked)
			cancel()

			if err != nil {
				failedCount++
				failedGroups = append(failedGroups, fmt.Sprintf("❌ %s (Edit: %v)", group.Name, err))
			} else {
				successCount++
			}
		}

		// Delay ANTAR GRUP (bukan antar pengaturan)
		// Delay hanya diterapkan setelah semua pengaturan untuk grup ini selesai
		// dan sebelum lanjut ke grup berikutnya
		if i < len(groups)-1 && delay > 0 {
			time.Sleep(time.Duration(delay) * time.Second)
		}

		// Show progress (per grup, bukan per operasi)
		if totalGroups > 1 {
			// Calculate which group we're processing
			currentGroup := (i + 1)
			progressPercent := (currentGroup * 100) / totalGroups
			progressBar := generateProgressBar(progressPercent)

			progressMsg := fmt.Sprintf(`⏳ **PROGRESS**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s **%d%%**
📊 **Grup:** %d/%d grup
⚙️ **Pengaturan:** %d pengaturan per grup (diproses sekaligus)
✅ **Berhasil:** %d operasi
❌ **Gagal:** %d operasi
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⏳ Memproses grup %d/%d...`, progressBar, progressPercent, currentGroup, totalGroups, settingsCount, successCount, failedCount, currentGroup, totalGroups)

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

⚙️ **Total Operasi:** %d operasi
✅ **Berhasil:** %d operasi
❌ **Gagal:** %d operasi
⏱️ **Delay:** %d detik/grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, totalOps, successCount, failedCount, delay)

	msg := tgbotapi.NewMessage(chatID, resultMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Send failed operations if any
	if len(failedGroups) > 0 {
		batchSize := 10
		for i := 0; i < len(failedGroups); i += batchSize {
			end := i + batchSize
			if end > len(failedGroups) {
				end = len(failedGroups)
			}

			batch := failedGroups[i:end]
			failedMsg := fmt.Sprintf("**Operasi yang Gagal (Batch %d):**\n\n%s", (i/batchSize)+1, strings.Join(batch, "\n"))

			msg := tgbotapi.NewMessage(chatID, failedMsg)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)

			if end < len(failedGroups) {
				time.Sleep(1 * time.Second)
			}
		}
	}

	// Completion keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Atur Lagi", "change_all_settings_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Grup", "grup"),
		),
	)

	completionMsg := tgbotapi.NewMessage(chatID, "💡 Apa yang ingin Anda lakukan selanjutnya?")
	completionMsg.ReplyMarkup = keyboard
	telegramBot.Send(completionMsg)
}

// CancelChangeAllSettings membatalkan proses
func CancelChangeAllSettings(chatID int64, telegramBot *tgbotapi.BotAPI) {
	delete(groupAllSettingsStates, chatID)

	msg := tgbotapi.NewMessage(chatID, "❌ Proses atur semua pengaturan grup dibatalkan.")
	telegramBot.Send(msg)
}

// IsWaitingForAllSettingsInput checks if user is waiting to input
func IsWaitingForAllSettingsInput(chatID int64) bool {
	state := groupAllSettingsStates[chatID]
	return state != nil && (state.WaitingForGroupName || state.WaitingForDelay)
}

// GetAllSettingsInputType returns the current input type
func GetAllSettingsInputType(chatID int64) string {
	state := groupAllSettingsStates[chatID]
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

// ProcessSelectedGroupsForAllSettings processes selected groups untuk atur semua pengaturan
func ProcessSelectedGroupsForAllSettings(selection string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	selectedGroups := HandleGroupSelection(selection, chatID, telegramBot)

	if len(selectedGroups) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Pilihan tidak valid!\n\nContoh: 1, 1-5, 1,3,5, atau 'all'")
		telegramBot.Send(errorMsg)
		return
	}

	// Clear all settings selection marker if exists
	delete(allSettingsSelection, chatID)

	// Initialize state
	groupAllSettingsStates[chatID] = &GroupAllSettingsState{
		WaitingForGroupName: false,
		WaitingForDelay:     true,
		CurrentSettingIndex: -1,
		SelectedGroups:      selectedGroups,
		Keyword:             "",
		DelaySeconds:        0,
		MessageLogging:      nil,
		MemberAdd:           nil,
		JoinApproval:        nil,
		Ephemeral:           nil,
		EditSettings:        nil,
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

Masukkan berapa detik delay antar permintaan per pengaturan.

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
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_all_settings"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// HandleChangeAllSettingsAll handles "Atur Semua" untuk semua pengaturan
func HandleChangeAllSettingsAll(chatID int64, telegramBot *tgbotapi.BotAPI) {
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
	groupAllSettingsStates[chatID] = &GroupAllSettingsState{
		WaitingForGroupName: false,
		WaitingForDelay:     true,
		CurrentSettingIndex: -1,
		SelectedGroups:      selectedGroups,
		Keyword:             "",
		DelaySeconds:        0,
		MessageLogging:      nil,
		MemberAdd:           nil,
		JoinApproval:        nil,
		Ephemeral:           nil,
		EditSettings:        nil,
	}

	confirmMsg := fmt.Sprintf(`⚡ **ATUR SEMUA GRUP - SEMUA PENGATURAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total grup:** %d grup

⚠️ **PERINGATAN:**
Anda akan mengatur SEMUA pengaturan untuk SEMUA grup sekaligus!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Masukkan berapa detik delay ANTAR GRUP untuk menghindari rate limit.

**Rekomendasi:**
• < 10 grup: 1-2 detik
• 10-30 grup: 2-3 detik
• > 30 grup: 3-5 detik

**💡 Catatan Penting:**
• Delay digunakan untuk jeda ANTAR GRUP
• Semua pengaturan dalam 1 grup diproses SEKALIGUS (tanpa delay)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay dalam detik (contoh: 4)`, len(selectedGroups))

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_change_all_settings"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowGroupListForAllSettingsEdit menampilkan daftar grup dengan pagination (EDIT, NO SPAM!)
func ShowGroupListForAllSettingsEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int, page int) {
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
	msg := fmt.Sprintf(`📋 **DAFTAR GRUP - ATUR SEMUA PENGATURAN**

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
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("all_settings_page_%d", page-1)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📄 %d/%d", page, totalPages), "noop"))
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️ Next", fmt.Sprintf("all_settings_page_%d", page+1)))
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)

	// Quick action buttons
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Pilih Semua", "change_all_settings_all"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "change_all_settings_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, msg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)

	// Store state with custom marker
	SetListSelectStateForAllSettings(chatID, page, totalPages, groupsPerPage, groups)
}

// SetListSelectStateForAllSettings sets the list select state with a marker for all settings
func SetListSelectStateForAllSettings(chatID int64, page, totalPages, groupsPerPage int, groups []GroupLinkInfo) {
	listSelectStates[chatID] = &ListSelectState{
		CurrentPage:    page,
		TotalPages:     totalPages,
		GroupsPerPage:  groupsPerPage,
		AllGroups:      groups,
		SelectedGroups: make(map[int]bool),
	}
	allSettingsSelection[chatID] = true
}

// Map to track if selection is for all settings
var allSettingsSelection = make(map[int64]bool)

// IsWaitingForAllSettingsSelection checks if user is selecting groups for all settings
func IsWaitingForAllSettingsSelection(chatID int64) bool {
	return allSettingsSelection[chatID] && listSelectStates[chatID] != nil
}

// HandleFileInputForAllSettings - Handle file .txt untuk atur semua pengaturan grup
func HandleFileInputForAllSettings(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := groupAllSettingsStates[chatID]
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
		utils.LogActivityError("change_all_settings", "Gagal download file", chatID, err)
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
		utils.LogActivityError("change_all_settings", "File tidak valid", chatID, err)
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
		utils.LogActivityError("change_all_settings", "Gagal membaca file", chatID, err)
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
		utils.LogActivityError("change_all_settings", "Error membaca file", chatID, err)
		return
	}

	if len(groupNames) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ **FILE KOSONG**\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nFile `.txt` yang Anda kirim tidak berisi nama grup.\n\n**Format yang benar:**\nSatu nama grup per baris.\n\n**Contoh:**\nKeluarga Besar\nGrup Kerja\nGrup Teman")
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return
	}

	// Log activity
	utils.LogActivity("change_all_settings_file", fmt.Sprintf("File .txt diterima dengan %d nama grup", len(groupNames)), chatID)

	// Search groups
	groups, err := utils.SearchGroupsExactMultiple(groupNames)
	if err != nil {
		errorMsg := utils.FormatUserError(utils.ErrorDatabase, err, "Gagal mencari grup")
		msg := tgbotapi.NewMessage(chatID, errorMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		utils.LogActivityError("change_all_settings", "Gagal mencari grup dari file", chatID, err)
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
