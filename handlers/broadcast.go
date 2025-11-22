package handlers

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// BroadcastState manages state for broadcast feature
type BroadcastState struct {
	// Input stages
	WaitingForOffsetDelay   bool     // Delay offset antar akun
	WaitingForGroupDelay    bool     // Delay antar grup
	WaitingForMessageMode   bool     // Mode input pesan (file txt atau manual)
	WaitingForMessageFile   bool     // Upload file txt berisi kalimat
	WaitingForMessageManual bool     // Input manual pesan per akun
	WaitingForTargetMode    bool     // Mode target grup (manual atau file txt)
	WaitingForTargetGroups  bool     // Input nama grup target
	WaitingForConfirmation  bool     // Konfirmasi sebelum start
	CurrentAccountID        int      // ID akun yang sedang menunggu input pesan
	PendingGroupNames       []string // Nama grup yang sedang diakumulasi (belum diproses)
	AllAccountsFilled       bool     // Flag menandai semua akun sudah terisi (untuk menghindari spam pesan)
	ReminderSent            bool     // Flag menandai reminder sudah dikirim (untuk mencegah spam reminder)

	// Configuration
	OffsetDelay      int              // Delay offset antar akun (detik)
	GroupDelay       int              // Delay antar grup (detik)
	MessageMode      string           // "file" atau "manual"
	Messages         map[int][]string // Map account ID -> list of messages
	TargetMode       string           // "manual" atau "file"
	TargetGroups     []types.JID      // JID grup target
	TargetGroupNames []string         // Nama grup target (untuk display)

	// Progress tracking
	IsRunning   bool
	CurrentLoop int
	ShouldStop  bool
	StopMutex   sync.Mutex

	// Statistics
	TotalSent        map[int]int                            // Map account ID -> jumlah pesan terkirim
	TotalFailed      map[int]int                            // Map account ID -> jumlah gagal
	AccountStatus    map[int]bool                           // Map account ID -> connected/disconnected
	LastSentMessages map[int]map[types.JID]*types.MessageID // Map account ID -> map group JID -> Last message ID sent (untuk cross-account read)
	LastUpdateTime   time.Time
}

var broadcastStates = make(map[int64]*BroadcastState)
var broadcastMutex sync.Mutex

// BroadcastProgress tracks real-time progress
type BroadcastProgress struct {
	IsRunning   bool
	CurrentLoop int
	Accounts    []AccountProgress
	LastUpdate  time.Time
}

type AccountProgress struct {
	AccountID    int
	PhoneNumber  string
	Status       string // "running", "disconnected", "stopped"
	TotalSent    int
	TotalFailed  int
	CurrentGroup int
	TotalGroups  int
}

// GetBroadcastState gets broadcast state for a chat
func GetBroadcastState(chatID int64) *BroadcastState {
	broadcastMutex.Lock()
	defer broadcastMutex.Unlock()

	if broadcastStates[chatID] == nil {
		broadcastStates[chatID] = &BroadcastState{
			Messages:          make(map[int][]string),
			TotalSent:         make(map[int]int),
			TotalFailed:       make(map[int]int),
			AccountStatus:     make(map[int]bool),
			PendingGroupNames: make([]string, 0),
			CurrentAccountID:  -1,
		}
	}
	return broadcastStates[chatID]
}

// ShowBroadcastMenu menampilkan menu broadcast
func ShowBroadcastMenu(telegramBot *tgbotapi.BotAPI, chatID int64) {
	menuMsg := `📢 **BROADCAST PESAN KE GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini memungkinkan Anda mengirim pesan ke banyak grup sekaligus menggunakan multi-akun WhatsApp.

**🎯 Strategi:** Staggered Parallel
**🔄 Mode:** Loop (berulang sampai di-stop)
**✨ Variasi:** Auto-Variation (Base + Random)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Fitur:**
• Multi-akun broadcast secara bersamaan
• Input pesan via file TXT atau manual
• Target grup manual atau via file TXT
• Progress monitoring real-time
• Auto-variation pesan otomatis

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih opsi untuk memulai`

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai Broadcast", "broadcast_start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Panduan", "broadcast_guide"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// ShowBroadcastMenuEdit menampilkan menu broadcast dengan EDIT
func ShowBroadcastMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	menuMsg := `📢 **BROADCAST PESAN KE GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini memungkinkan Anda mengirim pesan ke banyak grup sekaligus menggunakan multi-akun WhatsApp.

**🎯 Strategi:** Staggered Parallel
**🔄 Mode:** Loop (berulang sampai di-stop)
**✨ Variasi:** Auto-Variation (Base + Random)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Fitur:**
• Multi-akun broadcast secara bersamaan
• Input pesan via file TXT atau manual
• Target grup manual atau via file TXT
• Progress monitoring real-time
• Auto-variation pesan otomatis

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih opsi untuk memulai`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Mulai Broadcast", "broadcast_start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📖 Panduan", "broadcast_guide"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "grup"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}

// StartBroadcastSetup memulai setup broadcast
func StartBroadcastSetup(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int) {
	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	if len(accounts) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Belum ada akun WhatsApp yang terhubung.\n\nGunakan menu \"📱 Login WhatsApp Baru\" untuk menambahkan akun terlebih dahulu.")
		telegramBot.Send(msg)
		return
	}

	// Reset state
	state := GetBroadcastState(chatID)
	*state = BroadcastState{
		WaitingForOffsetDelay: true,
		Messages:              make(map[int][]string),
		TotalSent:             make(map[int]int),
		TotalFailed:           make(map[int]int),
		AccountStatus:         make(map[int]bool),
	}

	msgText := fmt.Sprintf(`🚀 **SETUP BROADCAST**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📱 Akun Tersedia:** %d akun

**📋 Konfigurasi yang Diperlukan:**

1️⃣ **Delay Offset Antar Akun**
   • Waktu jeda antara akun pertama dan akun berikutnya
   • Contoh: Jika offset = 5 detik
     → Akun 1 mulai di detik 0
     → Akun 2 mulai di detik 5
     → Akun 3 mulai di detik 10
   • Rekomendasi: 3-10 detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💬 **Masukkan delay offset (detik):**`, len(accounts))

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)
}

// HandleOffsetDelayInput handles offset delay input
func HandleOffsetDelayInput(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := GetBroadcastState(chatID)

	delay, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || delay < 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Input tidak valid! Masukkan angka positif (detik).\n\nContoh: 5")
		telegramBot.Send(msg)
		return
	}

	state.OffsetDelay = delay
	state.WaitingForOffsetDelay = false
	state.WaitingForGroupDelay = true

	msgText := fmt.Sprintf(`✅ **Offset Delay:** %d detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

2️⃣ **Delay Antar Grup**
   • Waktu jeda antara pengiriman ke grup satu dan grup berikutnya
   • Contoh: Jika delay = 10 detik
     → Kirim ke grup 1, tunggu 10 detik, kirim ke grup 2
   • Rekomendasi: 5-15 detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💬 **Masukkan delay antar grup (detik):**`, delay)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)
}

// HandleGroupDelayInput handles group delay input
func HandleGroupDelayInput(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := GetBroadcastState(chatID)

	delay, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || delay < 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Input tidak valid! Masukkan angka positif (detik).\n\nContoh: 10")
		telegramBot.Send(msg)
		return
	}

	state.GroupDelay = delay
	state.WaitingForGroupDelay = false
	state.WaitingForMessageMode = true

	msgText := fmt.Sprintf(`✅ **Group Delay:** %d detik

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

3️⃣ **Mode Input Pesan**

**📄 File TXT** - Upload file berisi barisan kalimat
   • Satu kalimat per baris
   • Support semua karakter, simbol, emoji, bahasa
   • Tidak ada batasan

**✍️ Manual** - Input pesan untuk setiap akun
   • Program akan menanyakan pesan untuk setiap akun
   • Ketik "/mulai" setelah semua akun diisi

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Pilih mode input pesan:**`, delay)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📄 File TXT", "broadcast_msg_file"),
			tgbotapi.NewInlineKeyboardButtonData("✍️ Manual", "broadcast_msg_manual"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// HandleMessageModeSelection handles message mode selection
func HandleMessageModeSelection(mode string, chatID int64, telegramBot *tgbotapi.BotAPI, messageID int) {
	state := GetBroadcastState(chatID)

	if mode == "file" {
		state.MessageMode = "file"
		state.WaitingForMessageMode = false
		state.WaitingForMessageFile = true

		msgText := `✅ **Mode:** File TXT

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📄 Upload File TXT**

Format file:
• Satu kalimat per baris
• Support semua karakter: simbol, angka, huruf, emoji, bahasa apapun
• Tidak ada batasan panjang atau jumlah baris

**Contoh isi file:**
Selamat pagi semuanya!
Halo, ini adalah pesan broadcast 🎉
Hello, this is a broadcast message 🌟
こんにちは、これはブロードキャストメッセージです

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💬 **Upload file .txt berisi kalimat pesan:**`

		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	} else if mode == "manual" {
		state.MessageMode = "manual"
		state.WaitingForMessageMode = false
		state.WaitingForMessageManual = true

		// Mulai loop untuk input pesan per akun
		StartManualMessageInput(telegramBot, chatID)
	}
}

// StartManualMessageInput starts manual message input loop
func StartManualMessageInput(telegramBot *tgbotapi.BotAPI, chatID int64) {
	am := GetAccountManager()
	accounts := am.GetAllAccounts()
	state := GetBroadcastState(chatID)

	// Reset messages dan flags
	state.Messages = make(map[int][]string)
	state.CurrentAccountID = -1
	state.AllAccountsFilled = false
	state.ReminderSent = false

	// Cari akun pertama yang belum ada pesannya
	for _, acc := range accounts {
		if _, exists := state.Messages[acc.ID]; !exists {
			state.CurrentAccountID = acc.ID
			msgText := fmt.Sprintf(`✍️ **INPUT MANUAL PESAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📱 Pesan untuk Nomor:** +%s

💬 **Masukkan pesan untuk akun ini:**`, acc.PhoneNumber)

			msg := tgbotapi.NewMessage(chatID, msgText)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
			return
		}
	}

	// Semua akun sudah terisi, tunggu user ketik "/mulai"
	// Tapi seharusnya tidak pernah masuk ke sini karena StartManualMessageInput hanya dipanggil saat awal
	// Jika memang semua sudah terisi, set flag dan tunggu
	state.CurrentAccountID = -1
	state.AllAccountsFilled = true
	msg := tgbotapi.NewMessage(chatID, "✅ **Semua akun sudah memiliki pesan!**\n\nKetik **\"/mulai\"** untuk melanjutkan ke konfigurasi target grup.")
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)
}

// HandleManualMessageInput handles manual message input for an account
func HandleManualMessageInput(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := GetBroadcastState(chatID)
	if !state.WaitingForMessageManual {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		// Input kosong, abaikan
		return
	}

	// Terima input "mulai" atau "/mulai" (case insensitive)
	if strings.ToLower(input) == "mulai" || strings.ToLower(input) == "/mulai" {
		// Reset flags jika user ketik "/mulai"
		state.AllAccountsFilled = false
		state.ReminderSent = false

		// Cek apakah semua akun sudah ada pesannya
		am := GetAccountManager()
		accounts := am.GetAllAccounts()

		allFilled := true
		var missingAccounts []string
		for _, acc := range accounts {
			if _, exists := state.Messages[acc.ID]; !exists || len(state.Messages[acc.ID]) == 0 {
				allFilled = false
				missingAccounts = append(missingAccounts, "+"+acc.PhoneNumber)
			}
		}

		if !allFilled {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Belum semua akun memiliki pesan!\n\nAkun yang belum terisi:\n%s\n\nLanjutkan mengisi pesan untuk akun yang belum terisi.", strings.Join(missingAccounts, "\n")))
			telegramBot.Send(msg)
			return
		}

		// Semua akun sudah terisi, lanjut ke target mode
		state.WaitingForMessageManual = false
		state.CurrentAccountID = -1
		state.AllAccountsFilled = false
		state.ReminderSent = false
		HandleTargetModeSelection(telegramBot, chatID)
		return
	}

	// Dapatkan akun yang sedang menunggu input
	am := GetAccountManager()

	// Jika sudah semua akun terisi dan input bukan "/mulai"
	if state.AllAccountsFilled {
		// Jika input adalah "/mulai", sudah di-handle di atas
		// Jika bukan "/mulai", abaikan tanpa mengirim reminder lagi (sudah ada di summary final)
		// TIDAK mengirim pesan apapun - biarkan user ketik "/mulai" dulu
		return
	}

	// Jika CurrentAccountID == -1, berarti ini input pertama atau setelah semua akun terisi
	// Cari akun pertama yang belum ada pesannya
	if state.CurrentAccountID == -1 {
		accounts := am.GetAllAccounts()

		// Cari akun yang belum ada pesannya
		var foundAccount *WhatsAppAccount
		for _, acc := range accounts {
			if _, exists := state.Messages[acc.ID]; !exists || len(state.Messages[acc.ID]) == 0 {
				foundAccount = acc
				state.CurrentAccountID = acc.ID
				break
			}
		}

		// Jika tidak ada akun yang belum terisi, berarti semua sudah terisi
		// Loop kembali ke akun pertama untuk memungkinkan user mengubah pesan
		if foundAccount == nil {
			// Cek ulang apakah benar-benar semua sudah terisi
			allFilled := true
			for _, acc := range accounts {
				if _, exists := state.Messages[acc.ID]; !exists || len(state.Messages[acc.ID]) == 0 {
					allFilled = false
					break
				}
			}

			if allFilled {
				// Semua akun sudah terisi - loop kembali ke akun pertama
				if len(accounts) > 0 {
					firstAccount := accounts[0]
					state.CurrentAccountID = firstAccount.ID

					// Tampilkan pesan bahwa semua akun sudah terisi, tapi loop kembali
					msgText := fmt.Sprintf(`━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Semua akun sudah memiliki pesan!**

💡 Anda dapat:
• Lanjutkan menginput pesan untuk menyimpan pesan tambahan
• Ketik **"/mulai"** untuk melanjutkan ke konfigurasi target grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📱 Pesan untuk Nomor:** +%s

💬 **Masukkan pesan untuk akun ini:**`, firstAccount.PhoneNumber)

					msg := tgbotapi.NewMessage(chatID, msgText)
					msg.ParseMode = "Markdown"
					telegramBot.Send(msg)
					return
				}
			}

			// Jika ada yang belum terisi tapi tidak ditemukan, ada masalah
			msg := tgbotapi.NewMessage(chatID, "❌ Error: Tidak dapat menemukan akun yang belum terisi. Silakan coba lagi.")
			telegramBot.Send(msg)
			return
		}
	}

	// Dapatkan akun yang sedang menunggu input
	currentAccount := am.GetAccount(state.CurrentAccountID)
	if currentAccount == nil {
		// Reset dan coba lagi
		state.CurrentAccountID = -1
		msg := tgbotapi.NewMessage(chatID, "❌ Error: Akun tidak ditemukan! Mencari akun berikutnya...")
		telegramBot.Send(msg)
		// Rekursif panggil lagi untuk mencari akun berikutnya
		HandleManualMessageInput(input, chatID, telegramBot)
		return
	}

	// Simpan pesan untuk akun ini (hanya 1 pesan per input)
	// Jika user ingin multiple messages, bisa pisah dengan newline atau |
	messages := []string{input}
	if strings.Contains(input, "\n") {
		messages = strings.Split(input, "\n")
		// Filter empty lines
		var filteredMessages []string
		for _, msg := range messages {
			trimmed := strings.TrimSpace(msg)
			if trimmed != "" {
				filteredMessages = append(filteredMessages, trimmed)
			}
		}
		messages = filteredMessages
	} else if strings.Contains(input, "|") {
		messages = strings.Split(input, "|")
		// Filter empty
		var filteredMessages []string
		for _, msg := range messages {
			trimmed := strings.TrimSpace(msg)
			if trimmed != "" {
				filteredMessages = append(filteredMessages, trimmed)
			}
		}
		messages = filteredMessages
	}

	if len(messages) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Pesan tidak boleh kosong!")
		telegramBot.Send(msg)
		return
	}

	// Simpan pesan untuk akun ini dengan append (akumulasi pesan)
	// Jika akun ini sudah punya pesan, append ke yang sudah ada
	// Jika belum punya, buat baru
	if existingMessages, exists := state.Messages[currentAccount.ID]; exists && len(existingMessages) > 0 {
		// Append pesan baru ke yang sudah ada
		state.Messages[currentAccount.ID] = append(existingMessages, messages...)
	} else {
		// Buat baru
		state.Messages[currentAccount.ID] = messages
	}

	// Kirim konfirmasi pesan tersimpan dengan summary
	accounts := am.GetAllAccounts()

	// Buat konfirmasi dengan summary (tampilkan jumlah pesan per akun)
	var filledAccountsList []string
	var totalAccounts int
	for _, acc := range accounts {
		totalAccounts++
		if msgs, exists := state.Messages[acc.ID]; exists && len(msgs) > 0 {
			filledAccountsList = append(filledAccountsList, fmt.Sprintf("✅ +%s (%d pesan)", acc.PhoneNumber, len(msgs)))
		}
	}

	// Buat konfirmasi dengan summary
	confirmMsg := fmt.Sprintf(`✅ **Pesan untuk +%s tersimpan!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📊 Progress:** %d/%d akun sudah memiliki pesan

**✅ Akun yang sudah terisi:**
%s`, currentAccount.PhoneNumber, len(filledAccountsList), totalAccounts, strings.Join(filledAccountsList, "\n"))

	sentConfirmMsg := tgbotapi.NewMessage(chatID, confirmMsg)
	sentConfirmMsg.ParseMode = "Markdown"
	telegramBot.Send(sentConfirmMsg)

	// Cari akun berikutnya dengan logika round-robin
	// 1. Jika masih ada akun yang belum punya pesan, cari yang belum punya
	// 2. Jika semua sudah punya pesan, gunakan round-robin (Akun1>Akun2>Akun1>Akun2...)

	var nextAccount *WhatsAppAccount
	var nextAccountID int = -1

	// Cek apakah masih ada akun yang belum punya pesan
	var hasEmptyAccount bool
	for _, acc := range accounts {
		if _, exists := state.Messages[acc.ID]; !exists || len(state.Messages[acc.ID]) == 0 {
			hasEmptyAccount = true
			break
		}
	}

	if hasEmptyAccount {
		// Masih ada akun yang belum punya pesan, cari yang belum terisi
		for _, acc := range accounts {
			if _, exists := state.Messages[acc.ID]; !exists || len(state.Messages[acc.ID]) == 0 {
				nextAccount = acc
				nextAccountID = acc.ID
				break
			}
		}
	} else {
		// Semua akun sudah punya pesan - gunakan round-robin
		// Cari index akun yang baru saja menerima pesan
		currentIndex := -1
		for i, acc := range accounts {
			if acc.ID == currentAccount.ID {
				currentIndex = i
				break
			}
		}

		// Pilih akun berikutnya (round-robin)
		if currentIndex >= 0 {
			// Next account adalah akun berikutnya dalam array
			nextIndex := (currentIndex + 1) % len(accounts)
			nextAccount = accounts[nextIndex]
			nextAccountID = nextAccount.ID
		} else {
			// Jika tidak ditemukan (tidak seharusnya terjadi), pilih akun pertama
			if len(accounts) > 0 {
				nextAccount = accounts[0]
				nextAccountID = nextAccount.ID
			}
		}
	}

	// Set current account ID untuk prompt berikutnya
	if nextAccount != nil && nextAccountID >= 0 {
		state.CurrentAccountID = nextAccountID

		// Tentukan pesan yang akan ditampilkan
		var msgText string
		if hasEmptyAccount {
			// Masih ada akun yang belum terisi
			msgText = fmt.Sprintf(`━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📱 Pesan untuk Nomor:** +%s

💬 **Masukkan pesan untuk akun ini:**`, nextAccount.PhoneNumber)
		} else {
			// Semua akun sudah terisi - round-robin mode
			msgText = fmt.Sprintf(`━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Semua akun sudah memiliki pesan!**

💡 Anda dapat:
• Lanjutkan menginput pesan untuk menyimpan pesan tambahan
• Ketik **"/mulai"** untuk melanjutkan ke konfigurasi target grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📱 Pesan untuk Nomor:** +%s

💬 **Masukkan pesan untuk akun ini:**`, nextAccount.PhoneNumber)
		}

		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	} else {
		// Tidak ada akun, error
		msg := tgbotapi.NewMessage(chatID, "❌ Error: Tidak ada akun yang ditemukan!")
		telegramBot.Send(msg)
	}
}

// GetNextAccountForMessage gets next account that needs message input
func GetNextAccountForMessage(accounts []*WhatsAppAccount, messages map[int][]string) *WhatsAppAccount {
	for _, acc := range accounts {
		if _, exists := messages[acc.ID]; !exists {
			return acc
		}
		if len(messages[acc.ID]) == 0 {
			return acc
		}
	}
	return nil
}

// HandleFileInputForBroadcastMessage handles file upload for broadcast messages
func HandleFileInputForBroadcastMessage(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := GetBroadcastState(chatID)
	if !state.WaitingForMessageFile {
		return
	}

	// Download file dari Telegram
	// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile", botToken)
	var resp *http.Response
	var err error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = http.Get(fileURL + "?file_id=" + fileID)
		if err == nil && resp != nil && resp.StatusCode == 200 {
			break // Success
		}
		if resp != nil {
			resp.Body.Close()
		}

		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Warn("HandleFileInputForBroadcastMessage: Attempt %d/%d failed: %v", attempt+1, maxRetries, err)

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil || resp == nil {
		errorMsg := "unknown error"
		if err != nil {
			errorMsg = err.Error()
		}
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mengunduh file setelah "+fmt.Sprintf("%d", maxRetries)+" percobaan: "+errorMsg)
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Error("HandleFileInputForBroadcastMessage: Gagal download file setelah retry: %v", err)
		return
	}
	defer resp.Body.Close()

	var fileResult struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fileResult); !fileResult.OK || err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mendapatkan file path")
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Error("HandleFileInputForBroadcastMessage: Gagal decode file result: %v", err)
		return
	}

	// Download actual file
	// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResult.Result.FilePath)
	var fileResp *http.Response
	err = nil
	for attempt := 0; attempt < maxRetries; attempt++ {
		fileResp, err = http.Get(downloadURL)
		if err == nil && fileResp != nil && fileResp.StatusCode == 200 {
			break // Success
		}
		if fileResp != nil {
			fileResp.Body.Close()
		}

		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Warn("HandleFileInputForBroadcastMessage: Attempt %d/%d failed download file: %v", attempt+1, maxRetries, err)

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil || fileResp == nil {
		errorMsg := "unknown error"
		if err != nil {
			errorMsg = err.Error()
		}
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mengunduh file setelah "+fmt.Sprintf("%d", maxRetries)+" percobaan: "+errorMsg)
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Error("HandleFileInputForBroadcastMessage: Gagal download actual file setelah retry: %v", err)
		return
	}
	defer fileResp.Body.Close()

	// Baca isi file
	scanner := bufio.NewScanner(fileResp.Body)
	var sentences []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			sentences = append(sentences, line)
		}
	}

	if err := scanner.Err(); err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal membaca file: "+err.Error())
		telegramBot.Send(msg)
		return
	}

	if len(sentences) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ File kosong! File harus berisi minimal 1 kalimat.")
		telegramBot.Send(msg)
		return
	}

	// Distribusikan kalimat ke semua akun
	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	if len(accounts) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Belum ada akun WhatsApp!")
		telegramBot.Send(msg)
		return
	}

	// Distribusi kalimat: setiap akun dapat semua kalimat (tapi akan di-random saat broadcast)
	for _, acc := range accounts {
		state.Messages[acc.ID] = make([]string, len(sentences))
		copy(state.Messages[acc.ID], sentences)
	}

	state.WaitingForMessageFile = false

	msgText := fmt.Sprintf(`✅ **File berhasil dibaca!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📊 Statistik:**
• Total kalimat: %d
• Jumlah akun: %d
• Kalimat per akun: %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Kalimat akan didistribusikan ke semua akun dan divariasikan otomatis saat broadcast.`, len(sentences), len(accounts), len(sentences))

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Lanjut ke target mode
	HandleTargetModeSelection(telegramBot, chatID)
}

// HandleTargetModeSelection handles target mode selection
func HandleTargetModeSelection(telegramBot *tgbotapi.BotAPI, chatID int64) {
	state := GetBroadcastState(chatID)
	state.WaitingForTargetMode = true

	msgText := `4️⃣ **MODE TARGET GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Manual** - Ketik nama grup (pisah dengan koma atau newline)
**📄 File TXT** - Upload file berisi nama grup (satu nama per baris)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Pilih mode target grup:**`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Manual", "broadcast_target_manual"),
			tgbotapi.NewInlineKeyboardButtonData("📄 File TXT", "broadcast_target_file"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// HandleTargetModeSelectionCallback handles target mode selection from callback
func HandleTargetModeSelectionCallback(mode string, chatID int64, telegramBot *tgbotapi.BotAPI, messageID int) {
	state := GetBroadcastState(chatID)

	if mode == "file" {
		state.TargetMode = "file"
		state.WaitingForTargetMode = false
		state.WaitingForTargetGroups = true

		msgText := `✅ **Mode:** File TXT

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📄 Upload File TXT**

Format file:
• Satu nama grup per baris
• Nama harus sama persis dengan nama di database
• Contoh:
Keluarga Besar
Grup Kerja
Grup Teman

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💬 **Upload file .txt berisi nama grup:**`

		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	} else if mode == "manual" {
		state.TargetMode = "manual"
		state.WaitingForTargetMode = false
		state.WaitingForTargetGroups = true
		state.PendingGroupNames = []string{} // Reset pending groups

		msgText := `✅ **Mode:** Manual

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Input Nama Grup**

Masukkan nama grup target, pisahkan dengan:
• Koma (,) untuk satu baris
• Newline untuk beberapa baris

Anda dapat memasukkan grup satu per satu atau sekaligus.

**Contoh:**
Keluarga Besar, Grup Kerja, Grup Teman

atau

Keluarga Besar
Grup Kerja
Grup Teman

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💬 **Masukkan nama grup target** (ketik **"/mulai"** setelah selesai):`

		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	}
}

// HandleTargetGroupsInput handles target groups input (manual)
func HandleTargetGroupsInput(input string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := GetBroadcastState(chatID)
	if !state.WaitingForTargetGroups {
		return
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Cek jika user ketik "Mulai" untuk memproses semua grup yang sudah diakumulasi
	// Terima input "mulai" atau "/mulai" (case insensitive)
	if strings.ToLower(input) == "mulai" || strings.ToLower(input) == "/mulai" {
		if len(state.PendingGroupNames) == 0 {
			msg := tgbotapi.NewMessage(chatID, "❌ Belum ada nama grup yang dimasukkan!\n\nMasukkan nama grup terlebih dahulu.")
			telegramBot.Send(msg)
			return
		}

		// Proses semua grup yang sudah diakumulasi
		ProcessTargetGroups(state.PendingGroupNames, chatID, telegramBot)
		return
	}

	// Parse nama grup dari input
	var groupNames []string
	if strings.Contains(input, ",") {
		groupNames = strings.Split(input, ",")
	} else if strings.Contains(input, "\n") {
		groupNames = strings.Split(input, "\n")
	} else {
		groupNames = []string{input}
	}

	// Trim whitespace dan tambahkan ke pending list
	for _, groupName := range groupNames {
		trimmed := strings.TrimSpace(groupName)
		if trimmed != "" {
			// Cek duplikasi
			exists := false
			for _, pending := range state.PendingGroupNames {
				if strings.EqualFold(pending, trimmed) {
					exists = true
					break
				}
			}
			if !exists {
				state.PendingGroupNames = append(state.PendingGroupNames, trimmed)
			}
		}
	}

	// Tampilkan konfirmasi bahwa grup ditambahkan dan minta input lebih atau "Mulai"
	msgText := fmt.Sprintf(`✅ **Grup ditambahkan ke daftar!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📋 Daftar grup saat ini (%d grup):**
`, len(state.PendingGroupNames))

	for i, name := range state.PendingGroupNames {
		if i < 10 { // Tampilkan max 10 grup pertama
			msgText += fmt.Sprintf("• %s\n", name)
		} else if i == 10 {
			msgText += fmt.Sprintf("• ... dan %d grup lainnya\n", len(state.PendingGroupNames)-10)
			break
		}
	}

	msgText += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💬 **Masukkan nama grup lainnya** (atau ketik **"/mulai"** untuk memproses semua grup):`

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)
}

// ProcessTargetGroups processes accumulated group names and finds them in database
func ProcessTargetGroups(groupNames []string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	state := GetBroadcastState(chatID)

	// Ambil JID dari database
	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	var targetJIDs []types.JID
	var targetNames []string
	var notFoundGroups []string
	seenJIDs := make(map[string]bool) // Untuk mencegah duplikasi JID

	// Gunakan akun pertama untuk query database (biasanya semua akun punya grup yang sama)
	if len(accounts) > 0 {
		firstAccount := accounts[0]

		// Set DB config untuk akun ini
		utils.SetDBConfig(int64(chatID), firstAccount.PhoneNumber)

		// Query database untuk mendapatkan JID
		dbPool, err := utils.GetBotDBPool()
		if err == nil && dbPool != nil {
			// Gunakan case-insensitive search untuk setiap nama grup
			utils.GetLogger().Info("ProcessTargetGroups: Mencari %d grup di database", len(groupNames))
			for idx, groupName := range groupNames {
				if strings.TrimSpace(groupName) == "" {
					utils.GetLogger().Error("ProcessTargetGroups: Skip grup kosong di index %d", idx)
					continue
				}

				trimmedName := strings.TrimSpace(groupName)
				var found bool
				utils.GetLogger().Info("ProcessTargetGroups: [%d/%d] Mencari grup: '%s'", idx+1, len(groupNames), trimmedName)

				// Try exact match first (case-insensitive)
				query := "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) = LOWER(?)"
				row := dbPool.QueryRow(query, trimmedName)

				var jidStr, name string
				err := row.Scan(&jidStr, &name)
				if err == nil && jidStr != "" {
					utils.GetLogger().Info("ProcessTargetGroups: Grup '%s' ditemukan (exact match): JID=%s, Name=%s", trimmedName, jidStr, name)
					// Cek apakah JID sudah pernah ditambahkan (prevent duplicate)
					if !seenJIDs[jidStr] {
						jid, err := parseJIDFromString(jidStr)
						if err == nil {
							// Langsung tambahkan ke list
							targetJIDs = append(targetJIDs, jid)
							targetNames = append(targetNames, name)
							seenJIDs[jidStr] = true
							found = true
							utils.GetLogger().Info("ProcessTargetGroups: Grup '%s' berhasil ditambahkan ke target list (total: %d)", trimmedName, len(targetJIDs))
							continue // Grup ditemukan dan ditambahkan, lanjut ke grup berikutnya
						} else {
							utils.GetLogger().Error("ProcessTargetGroups: Gagal parse JID '%s' untuk grup '%s': %v", jidStr, trimmedName, err)
						}
					} else {
						// JID sudah ada, skip grup ini (grup duplikat dengan nama berbeda)
						utils.GetLogger().Error("ProcessTargetGroups: Grup '%s' di-skip karena JID duplikat: %s", trimmedName, jidStr)
						found = true
						continue
					}
				} else if err != nil {
					utils.GetLogger().Info("ProcessTargetGroups: Grup '%s' tidak ditemukan dengan exact match: %v", trimmedName, err)
				}

				// If exact match failed, try LIKE match (case-insensitive)
				// IMPORTANT: Escape special characters untuk SQL LIKE (%, _, |, etc.)
				if !found {
					utils.GetLogger().Info("ProcessTargetGroups: Mencoba LIKE match untuk grup '%s'", trimmedName)
					// Escape special characters untuk SQL LIKE: %, _, [, ], |, \, ^, $
					escapedName := escapeLikePattern(trimmedName)
					query = "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) LIKE LOWER(?) ESCAPE '\\'"
					rows, err := dbPool.Query(query, "%"+escapedName+"%")
					if err == nil {
						matchCount := 0
						for rows.Next() {
							var jidStr2, name2 string
							if rows.Scan(&jidStr2, &name2) == nil && jidStr2 != "" {
								matchCount++
								utils.GetLogger().Info("ProcessTargetGroups: LIKE match #%d untuk '%s': JID=%s, Name=%s", matchCount, trimmedName, jidStr2, name2)
								// Cek apakah JID sudah pernah ditambahkan
								if !seenJIDs[jidStr2] {
									jid, err := parseJIDFromString(jidStr2)
									if err == nil {
										// Langsung tambahkan ke list
										targetJIDs = append(targetJIDs, jid)
										targetNames = append(targetNames, name2)
										seenJIDs[jidStr2] = true
										found = true
										utils.GetLogger().Info("ProcessTargetGroups: Grup '%s' berhasil ditambahkan via LIKE match (total: %d)", trimmedName, len(targetJIDs))
										break // Ambil yang pertama ditemukan dan tambahkan
									} else {
										utils.GetLogger().Error("ProcessTargetGroups: Gagal parse JID '%s' (LIKE match) untuk grup '%s': %v", jidStr2, trimmedName, err)
									}
								} else {
									// JID sudah ada, skip grup ini (grup duplikat)
									utils.GetLogger().Error("ProcessTargetGroups: Grup '%s' di-skip (LIKE match) karena JID duplikat: %s", trimmedName, jidStr2)
									found = true
									break
								}
							}
						}
						rows.Close()
						if matchCount == 0 {
							utils.GetLogger().Error("ProcessTargetGroups: Tidak ada LIKE match untuk grup '%s'", trimmedName)
						}
					} else {
						utils.GetLogger().Error("ProcessTargetGroups: Error query LIKE untuk grup '%s': %v", trimmedName, err)
					}
				}

				// Jika grup tidak ditemukan, tambahkan ke notFoundGroups
				if !found {
					notFoundGroups = append(notFoundGroups, groupName)
					// Debug: log grup yang tidak ditemukan
					utils.GetLogger().Error("ProcessTargetGroups: Grup '%s' tidak ditemukan di database", trimmedName)
				}
			}
		}
	}

	if len(targetJIDs) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang ditemukan dengan nama tersebut!\n\nPastikan nama grup sama persis dengan nama di database.")
		telegramBot.Send(msg)
		state.PendingGroupNames = []string{} // Reset pending groups
		return
	}

	// Jika ada grup yang tidak ditemukan, tampilkan warning tapi lanjutkan dengan yang ditemukan
	if len(notFoundGroups) > 0 {
		warningMsg := fmt.Sprintf("⚠️ **Beberapa grup tidak ditemukan (%d grup):**\n%s\n\n✅ **Menggunakan %d grup yang ditemukan.**\n\n",
			len(notFoundGroups),
			strings.Join(notFoundGroups, ", "),
			len(targetJIDs))
		msg := tgbotapi.NewMessage(chatID, warningMsg)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	}

	// Debug: Log jumlah grup yang diproses
	utils.GetLogger().Info("ProcessTargetGroups: Input %d grup, ditemukan %d grup, tidak ditemukan %d grup",
		len(groupNames), len(targetJIDs), len(notFoundGroups))

	state.TargetGroups = targetJIDs
	state.TargetGroupNames = targetNames
	state.WaitingForTargetGroups = false
	state.PendingGroupNames = []string{} // Reset setelah diproses

	// Tampilkan konfirmasi
	ShowBroadcastConfirmation(telegramBot, chatID)
}

// HandleFileInputForBroadcastTarget handles file upload for target groups
func HandleFileInputForBroadcastTarget(fileID string, chatID int64, telegramBot *tgbotapi.BotAPI, botToken string) {
	state := GetBroadcastState(chatID)
	if !state.WaitingForTargetGroups {
		return
	}

	// Download file dari Telegram
	// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile", botToken)
	var resp *http.Response
	var err error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = http.Get(fileURL + "?file_id=" + fileID)
		if err == nil && resp != nil && resp.StatusCode == 200 {
			break // Success
		}
		if resp != nil {
			resp.Body.Close()
		}

		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Warn("HandleFileInputForBroadcastTarget: Attempt %d/%d failed: %v", attempt+1, maxRetries, err)

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil || resp == nil {
		errorMsg := "unknown error"
		if err != nil {
			errorMsg = err.Error()
		}
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mengunduh file setelah "+fmt.Sprintf("%d", maxRetries)+" percobaan: "+errorMsg)
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Error("HandleFileInputForBroadcastTarget: Gagal download file setelah retry: %v", err)
		return
	}
	defer resp.Body.Close()

	var fileResult struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fileResult); !fileResult.OK || err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mendapatkan file path")
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Error("HandleFileInputForBroadcastTarget: Gagal decode file result: %v", err)
		return
	}

	// Download actual file
	// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResult.Result.FilePath)
	var fileResp *http.Response
	err = nil
	for attempt := 0; attempt < maxRetries; attempt++ {
		fileResp, err = http.Get(downloadURL)
		if err == nil && fileResp != nil && fileResp.StatusCode == 200 {
			break // Success
		}
		if fileResp != nil {
			fileResp.Body.Close()
		}

		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Warn("HandleFileInputForBroadcastTarget: Attempt %d/%d failed download file: %v", attempt+1, maxRetries, err)

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil || fileResp == nil {
		errorMsg := "unknown error"
		if err != nil {
			errorMsg = err.Error()
		}
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal mengunduh file setelah "+fmt.Sprintf("%d", maxRetries)+" percobaan: "+errorMsg)
		telegramBot.Send(msg)
		// FIXED: Log error untuk operasi kritis
		utils.GetLogger().Error("HandleFileInputForBroadcastTarget: Gagal download actual file setelah retry: %v", err)
		return
	}
	defer fileResp.Body.Close()

	// Baca isi file
	scanner := bufio.NewScanner(fileResp.Body)
	var groupNames []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			groupNames = append(groupNames, line)
		}
	}

	if err := scanner.Err(); err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Gagal membaca file: "+err.Error())
		telegramBot.Send(msg)
		return
	}

	if len(groupNames) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ File kosong! File harus berisi minimal 1 nama grup.")
		telegramBot.Send(msg)
		return
	}

	// Ambil JID dari database
	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	var targetJIDs []types.JID
	var targetNames []string
	seenJIDs := make(map[string]bool) // Untuk mencegah duplikasi JID

	if len(accounts) > 0 {
		firstAccount := accounts[0]
		utils.SetDBConfig(int64(chatID), firstAccount.PhoneNumber)

		dbPool, err := utils.GetBotDBPool()
		if err == nil && dbPool != nil {
			// Gunakan case-insensitive search untuk setiap nama grup
			for _, groupName := range groupNames {
				if strings.TrimSpace(groupName) == "" {
					continue
				}

				trimmedName := strings.TrimSpace(groupName)
				var found bool

				// Try exact match first (case-insensitive)
				query := "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) = LOWER(?)"
				row := dbPool.QueryRow(query, trimmedName)

				var jidStr, name string
				err := row.Scan(&jidStr, &name)
				if err == nil && jidStr != "" {
					// Cek apakah JID sudah pernah ditambahkan (prevent duplicate)
					if !seenJIDs[jidStr] {
						jid, err := parseJIDFromString(jidStr)
						if err == nil {
							targetJIDs = append(targetJIDs, jid)
							targetNames = append(targetNames, name)
							seenJIDs[jidStr] = true
							found = true
							continue
						}
					} else {
						// JID sudah ada, anggap sudah found
						found = true
						continue
					}
				}

				// If exact match failed, try LIKE match (case-insensitive)
				// IMPORTANT: Escape special characters untuk SQL LIKE (%, _, |, etc.)
				if !found {
					// Escape special characters untuk SQL LIKE: %, _, [, ], |, \, ^, $
					escapedName := escapeLikePattern(trimmedName)
					query = "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) LIKE LOWER(?) ESCAPE '\\'"
					rows, err := dbPool.Query(query, "%"+escapedName+"%")
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var jidStr2, name2 string
							if rows.Scan(&jidStr2, &name2) == nil && jidStr2 != "" {
								// Cek apakah JID sudah pernah ditambahkan
								if !seenJIDs[jidStr2] {
									jid, err := parseJIDFromString(jidStr2)
									if err == nil {
										targetJIDs = append(targetJIDs, jid)
										targetNames = append(targetNames, name2)
										seenJIDs[jidStr2] = true
										found = true
										break // Ambil yang pertama ditemukan
									}
								} else {
									// JID sudah ada, anggap sudah found
									found = true
									break
								}
							}
						}
						rows.Close()
					}
				}
			}
		}
	}

	if len(targetJIDs) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang ditemukan dengan nama tersebut!\n\nPastikan nama grup sama persis dengan nama di database.")
		telegramBot.Send(msg)
		return
	}

	state.TargetGroups = targetJIDs
	state.TargetGroupNames = targetNames
	state.WaitingForTargetGroups = false

	// Tampilkan konfirmasi
	ShowBroadcastConfirmation(telegramBot, chatID)
}

// ShowBroadcastConfirmation shows broadcast confirmation dialog
func ShowBroadcastConfirmation(telegramBot *tgbotapi.BotAPI, chatID int64) {
	state := GetBroadcastState(chatID)
	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	var accountList strings.Builder
	for i, acc := range accounts {
		msgCount := len(state.Messages[acc.ID])
		accountList.WriteString(fmt.Sprintf("%d. +%s (%d pesan)\n", i+1, acc.PhoneNumber, msgCount))
	}

	msgText := fmt.Sprintf(`✅ **KONFIGURASI SELESAI**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📊 Ringkasan Konfigurasi:**

**⏱️ Delay:**
• Offset antar akun: %d detik
• Delay antar grup: %d detik

**📱 Akun (%d):**
%s
**🎯 Target Grup:** %d grup

**📝 Mode Pesan:** %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **PERINGATAN:**
• Broadcast akan berjalan dalam mode LOOP
• Program akan terus mengulang sampai Anda stop dengan /stopchat
• Gunakan delay yang wajar untuk menghindari spam detection

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Konfirmasi untuk memulai broadcast?**`,
		state.OffsetDelay,
		state.GroupDelay,
		len(accounts),
		accountList.String(),
		len(state.TargetGroups),
		strings.ToUpper(state.MessageMode))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Mulai!", "broadcast_confirm_yes"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batal", "broadcast_confirm_no"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	telegramBot.Send(msg)
}

// StartBroadcast starts the broadcast process
func StartBroadcast(telegramBot *tgbotapi.BotAPI, chatID int64) {
	state := GetBroadcastState(chatID)

	// Validate state
	if len(state.TargetGroups) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Tidak ada target grup!")
		telegramBot.Send(msg)
		return
	}

	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	if len(accounts) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Tidak ada akun!")
		telegramBot.Send(msg)
		return
	}

	// Initialize statistics
	state.IsRunning = true
	state.CurrentLoop = 0
	state.ShouldStop = false
	state.TotalSent = make(map[int]int)
	state.TotalFailed = make(map[int]int)
	state.AccountStatus = make(map[int]bool)
	state.LastSentMessages = make(map[int]map[types.JID]*types.MessageID) // Initialize for cross-account read

	// Pastikan semua akun punya client dan terhubung sebelum broadcast
	var readyAccounts []*WhatsAppAccount
	for _, acc := range accounts {
		// Cek apakah akun ini punya pesan
		if messages, exists := state.Messages[acc.ID]; !exists || len(messages) == 0 {
			continue // Skip akun tanpa pesan
		}

		// Cek apakah client sudah dibuat
		client := am.GetClient(acc.ID)
		if client == nil {
			// Client belum dibuat, buat sekarang
			newClient, err := am.CreateClient(acc.ID)
			if err != nil {
				// Gagal membuat client, skip akun ini
				state.AccountStatus[acc.ID] = false
				continue
			}
			client = newClient
		}

		// Pastikan client terhubung
		if !client.IsConnected() {
			// Coba reconnect
			if err := client.Connect(); err != nil {
				state.AccountStatus[acc.ID] = false
				continue
			}
			// Tunggu sampai terhubung (max 5 detik)
			timeout := 5 * time.Second
			checkInterval := 500 * time.Millisecond
			elapsed := time.Duration(0)
			for !client.IsConnected() && elapsed < timeout {
				time.Sleep(checkInterval)
				elapsed += checkInterval
			}
		}

		// Akun siap untuk broadcast
		if client.IsConnected() {
			state.AccountStatus[acc.ID] = true
			state.TotalSent[acc.ID] = 0
			state.TotalFailed[acc.ID] = 0
			readyAccounts = append(readyAccounts, acc)
		} else {
			state.AccountStatus[acc.ID] = false
		}
	}

	// Jika tidak ada akun yang siap
	if len(readyAccounts) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Tidak ada akun yang siap untuk broadcast!\n\nPastikan:\n• Semua akun sudah terhubung\n• Semua akun sudah memiliki pesan")
		telegramBot.Send(msg)
		state.IsRunning = false
		return
	}

	// Start broadcast in goroutine dengan akun yang siap
	go RunBroadcastLoop(state, readyAccounts, telegramBot, chatID)

	// Send start notification
	msgText := fmt.Sprintf(`🚀 **BROADCAST DIMULAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📊 Konfigurasi:**
• Akun: %d
• Target grup: %d
• Mode: Loop (berulang sampai di-stop)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**⏹️ Stop Broadcast:**
Ketik /stopchat untuk menghentikan

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📈 Progress akan diupdate secara real-time...`, len(readyAccounts), len(state.TargetGroups))

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)

	// Start progress monitoring
	go MonitorBroadcastProgress(telegramBot, chatID)
}

// RunBroadcastLoop runs the main broadcast loop
// FIXED: Tambahkan context untuk cancellation dan timeout
func RunBroadcastLoop(state *BroadcastState, accounts []*WhatsAppAccount, telegramBot *tgbotapi.BotAPI, chatID int64) {
	// FIXED: Add context dengan timeout untuk mencegah loop tanpa batas
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// FIXED: Add timeout untuk loop (max 24 jam)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 24*time.Hour)
	defer timeoutCancel()

	for {
		// FIXED: Check context cancellation
		select {
		case <-timeoutCtx.Done():
			utils.GetLogger().Info("RunBroadcastLoop: Timeout reached, stopping loop")
			return
		default:
		}

		state.StopMutex.Lock()
		shouldStop := state.ShouldStop
		state.StopMutex.Unlock()

		if shouldStop {
			break
		}

		state.CurrentLoop++

		// Run broadcast untuk semua akun dengan staggered parallel
		var wg sync.WaitGroup

		for i, acc := range accounts {
			// Check if account is still connected BEFORE starting goroutine
			am := GetAccountManager()
			client := am.GetClient(acc.ID)

			// Cek apakah client tidak null
			if client == nil {
				state.AccountStatus[acc.ID] = false
				utils.GetLogger().Warn("RunBroadcastLoop: Client null untuk akun %d, skip", acc.ID)
				continue // Skip jika client null
			}

			// Periksa apakah akun ini punya pesan
			if messages, exists := state.Messages[acc.ID]; !exists || len(messages) == 0 {
				// Akun tidak punya pesan, skip
				continue
			}

			// Periksa koneksi - tapi jangan langsung skip jika IsConnected() false
			// Karena mungkin hanya transient disconnect
			if !client.IsConnected() {
				utils.GetLogger().Warn("RunBroadcastLoop: Client tidak connected untuk akun %d, tapi tetap lanjutkan (transient check)", acc.ID)
				// Jangan langsung set status false atau skip, biarkan goroutine menangani
			}

			// Pastikan status true jika client terhubung dan punya pesan
			state.AccountStatus[acc.ID] = true

			wg.Add(1)
			go func(account *WhatsAppAccount, offset int, allAccounts []*WhatsAppAccount) {
				defer wg.Done()

				// Staggered delay: akun pertama mulai langsung, akun berikutnya dengan offset
				if offset > 0 {
					delayDuration := time.Duration(offset*state.OffsetDelay) * time.Second
					utils.GetLogger().Info("RunBroadcastLoop: Offset delay %d detik untuk akun %d (offset=%d)", offset*state.OffsetDelay, account.ID, offset)
					time.Sleep(delayDuration)
				}

				// Check stop signal
				state.StopMutex.Lock()
				if state.ShouldStop {
					state.StopMutex.Unlock()
					return
				}
				state.StopMutex.Unlock()

				// Periksa koneksi lagi setelah delay (bisa berubah selama delay)
				amCheck := GetAccountManager()
				clientCheck := amCheck.GetClient(account.ID)
				if clientCheck == nil {
					// Client null - skip broadcast untuk akun ini
					state.AccountStatus[account.ID] = false
					utils.GetLogger().Warn("RunBroadcastLoop: Client null setelah delay untuk akun %d, skip", account.ID)
					return
				}

				// Catat status koneksi tapi jangan langsung return jika false
				if !clientCheck.IsConnected() {
					utils.GetLogger().Warn("RunBroadcastLoop: Client tidak connected setelah delay untuk akun %d, tapi lanjutkan", account.ID)
					// Jangan langsung return, biarkan BroadcastToGroupsForAccount yang handle
				}

				// Broadcast ke semua target grup untuk akun ini
				BroadcastToGroupsForAccount(state, account, telegramBot, chatID)

			}(acc, i, accounts)
		}

		wg.Wait()

		// Debug: Log semua LastSentMessages sebelum cross-account read
		utils.GetLogger().Info("RunBroadcastLoop: === DEBUG LastSentMessages ===")
		state.StopMutex.Lock()
		for accID, groupMap := range state.LastSentMessages {
			utils.GetLogger().Info("RunBroadcastLoop: Akun %d memiliki %d grup dengan message ID", accID, len(groupMap))
			for groupJID, msgID := range groupMap {
				if msgID != nil {
					utils.GetLogger().Info("RunBroadcastLoop:   - Grup %s: msgID=%s", groupJID.String(), *msgID)
				} else {
					utils.GetLogger().Warn("RunBroadcastLoop:   - Grup %s: msgID=nil", groupJID.String())
				}
			}
		}
		state.StopMutex.Unlock()
		utils.GetLogger().Info("RunBroadcastLoop: === END DEBUG LastSentMessages ===")

		// CROSS-ACCOUNT READ: Setelah semua broadcast selesai, SETIAP akun baca pesan dari SEMUA akun lain
		amRead := GetAccountManager()
		for i, acc := range accounts {
			client := amRead.GetClient(acc.ID)
			if client == nil {
				utils.GetLogger().Warn("RunBroadcastLoop: client null untuk akun %d (reader), skip", acc.ID)
				continue
			}

			utils.GetLogger().Info("RunBroadcastLoop: === Processing cross-account read untuk akun %d ===", acc.ID)

			// Setiap akun baca pesan dari semua akun lain (kecuali diri sendiri)
			for j, otherAcc := range accounts {
				if i == j {
					continue // Skip diri sendiri
				}

				otherClient := amRead.GetClient(otherAcc.ID)
				if otherClient != nil && otherClient.Store != nil && otherClient.Store.ID != nil {
					state.StopMutex.Lock()
					otherMessages := state.LastSentMessages[otherAcc.ID]
					state.StopMutex.Unlock()

					utils.GetLogger().Info("RunBroadcastLoop: Cek LastSentMessages untuk akun %d -> akun %d (ditemukan %d grup)", otherAcc.ID, acc.ID, len(otherMessages))

					// FIXED: Hapus nil check yang tidak perlu, len() untuk nil maps sudah defined as zero
					if len(otherMessages) == 0 {
						utils.GetLogger().Warn("RunBroadcastLoop: LastSentMessages kosong untuk akun %d (sender), skip", otherAcc.ID)
						continue
					}

					// Baca semua pesan terakhir dari other account untuk setiap grup
					for groupJID, msgID := range otherMessages {
						if msgID != nil && *msgID != "" {
							utils.GetLogger().Info("RunBroadcastLoop: Ditemukan message ID untuk grup %s: msgID=%s (dari akun %d ke akun %d)", groupJID.String(), *msgID, otherAcc.ID, acc.ID)
							go func(targetGroupJID types.JID, targetMsgID types.MessageID, senderPhoneNumber string, fromAccID, toAccID int, readerClient *whatsmeow.Client) {
								// Delay untuk memastikan pesan sudah tersimpan
								time.Sleep(2 * time.Second)

								markReadCtx, markReadCancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer markReadCancel()

								// Sender adalah akun yang mengirim pesan
								senderJID := types.NewJID(senderPhoneNumber, types.DefaultUserServer)
								utils.GetLogger().Info("RunBroadcastLoop: Attempting mark as read dari akun %d ke akun %d untuk grup %s (msgID=%s)", fromAccID, toAccID, targetGroupJID.String(), targetMsgID)
								err := readerClient.MarkRead(markReadCtx, []types.MessageID{targetMsgID}, time.Now(), targetGroupJID, senderJID)
								if err != nil {
									utils.GetLogger().Warn("RunBroadcastLoop: Gagal mark as read dari akun %d ke akun %d untuk grup %s: %v", fromAccID, toAccID, targetGroupJID.String(), err)
								} else {
									utils.GetLogger().Info("RunBroadcastLoop: ✅ Berhasil mark as read dari akun %d ke akun %d untuk grup %s (msgID=%s)", fromAccID, toAccID, targetGroupJID.String(), targetMsgID)
								}
							}(groupJID, *msgID, otherAcc.PhoneNumber, otherAcc.ID, acc.ID, client)
						} else {
							utils.GetLogger().Warn("RunBroadcastLoop: msgID nil atau kosong untuk akun %d, grup %s", otherAcc.ID, groupJID.String())
						}
					}
				} else {
					utils.GetLogger().Warn("RunBroadcastLoop: otherClient null untuk akun %d (sender), skip", otherAcc.ID)
				}
			}

			utils.GetLogger().Info("RunBroadcastLoop: === Selesai processing cross-account read untuk akun %d ===", acc.ID)
		}

		// Tambah delay sebelum loop berikutnya untuk memastikan semua mark as read selesai
		time.Sleep(5 * time.Second)

		// Check stop signal sebelum loop berikutnya
		state.StopMutex.Lock()
		shouldStop = state.ShouldStop
		state.StopMutex.Unlock()

		if shouldStop {
			break
		}

		// Loop berikutnya langsung dimulai (tidak ada jeda)
	}

	// Broadcast selesai
	state.IsRunning = false

	finalMsg := fmt.Sprintf(`✅ **BROADCAST DIHENTIKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**📊 Statistik Final:**
• Total loop: %d
• Total pesan terkirim: %d
• Total gagal: %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Terima kasih telah menggunakan fitur broadcast!`,
		state.CurrentLoop,
		GetTotalSent(state),
		GetTotalFailed(state))

	msg := tgbotapi.NewMessage(chatID, finalMsg)
	msg.ParseMode = "Markdown"
	telegramBot.Send(msg)
}

// BroadcastToGroupsForAccount broadcasts messages to all target groups for a specific account
func BroadcastToGroupsForAccount(state *BroadcastState, account *WhatsAppAccount, telegramBot *tgbotapi.BotAPI, chatID int64) {
	// Get client for this account
	am := GetAccountManager()
	client := am.GetClient(account.ID)

	// Periksa apakah client null
	if client == nil {
		state.AccountStatus[account.ID] = false
		utils.GetLogger().Warn("BroadcastToGroupsForAccount: Client null untuk akun %d, skip", account.ID)
		return
	}

	// Periksa apakah akun ini sudah di-mark sebagai disconnected
	state.StopMutex.Lock()
	isDisconnected := !state.AccountStatus[account.ID]
	state.StopMutex.Unlock()

	if isDisconnected {
		utils.GetLogger().Warn("BroadcastToGroupsForAccount: Akun %d sudah di-mark disconnected, skip", account.ID)
		return // Skip jika sudah di-mark disconnected
	}

	// Periksa koneksi tapi jangan langsung return jika false
	// Karena mungkin hanya transient disconnect yang akan recover
	if !client.IsConnected() {
		utils.GetLogger().Warn("BroadcastToGroupsForAccount: Akun %d tidak connected di awal, tapi lanjutkan", account.ID)
		// Jangan langsung return, biarkan proses broadcast mencoba
	}

	// Get messages for this account
	messages := state.Messages[account.ID]
	if len(messages) == 0 {
		return
	}

	// Broadcast ke setiap target grup
	for _, targetJID := range state.TargetGroups {
		// Check stop signal
		state.StopMutex.Lock()
		if state.ShouldStop {
			state.StopMutex.Unlock()
			return
		}
		state.StopMutex.Unlock()

		// Pilih pesan random dari daftar messages untuk akun ini
		selectedMessage := messages[rand.Intn(len(messages))]

		// Kirim pesan sesuai input user (TANPA modifikasi/emoji)
		// Pesan dikirim persis seperti yang diinput user
		// FIXED: Tambahkan retry logic dengan exponential backoff untuk network operations
		var resp whatsmeow.SendResponse
		var err error
		maxRetries := 3
		for attempt := 0; attempt < maxRetries; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			resp, err = client.SendMessage(ctx, targetJID, &waProto.Message{
				Conversation: proto.String(selectedMessage),
			})
			cancel()

			if err == nil {
				break // Success
			}

			// FIXED: Log error untuk operasi kritis
			utils.GetLogger().Warn("BroadcastToGroupsForAccount: Attempt %d/%d failed untuk akun %d, grup %s: %v",
				attempt+1, maxRetries, account.ID, targetJID.String(), err)

			// Exponential backoff: 1s, 2s, 4s
			if attempt < maxRetries-1 {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				time.Sleep(backoff)
			}
		}

		if err != nil {
			state.TotalFailed[account.ID]++
			// Debug: log error pengiriman
			utils.GetLogger().Error("BroadcastToGroupsForAccount: Gagal mengirim ke grup %s untuk akun %d: %v", targetJID.String(), account.ID, err)

			// Periksa apakah error ini benar-benar disconnect atau hanya transient error
			errorStr := strings.ToLower(err.Error())

			// Daftar error yang mengindikasikan disconnect permanen
			disconnectErrors := []string{
				"not logged in",
				"401 unauthorized",
				"logged out",
				"session expired",
				"authentication failed",
			}

			isDisconnect := false
			for _, disconnectErr := range disconnectErrors {
				if strings.Contains(errorStr, disconnectErr) {
					isDisconnect = true
					break
				}
			}

			// Hanya tandai sebagai disconnect jika:
			// 1. Error mengindikasikan disconnect permanen ATAU
			// 2. IsConnected() false DAN error bukan timeout/transient error
			if isDisconnect || (!client.IsConnected() && !strings.Contains(errorStr, "timeout") && !strings.Contains(errorStr, "context deadline")) {
				state.StopMutex.Lock()
				state.AccountStatus[account.ID] = false
				state.StopMutex.Unlock()
				utils.GetLogger().Warn("BroadcastToGroupsForAccount: Akun %d disconnect permanen, menghentikan broadcast untuk akun ini", account.ID)
				return
			} else {
				// Error transient (timeout, network issue dll) - lanjutkan broadcast
				utils.GetLogger().Warn("BroadcastToGroupsForAccount: Transient error untuk akun %d, melanjutkan broadcast: %v", account.ID, err)
			}
		} else {
			state.TotalSent[account.ID]++
			// Pastikan status tetap true jika berhasil kirim
			state.StopMutex.Lock()
			state.AccountStatus[account.ID] = true
			state.StopMutex.Unlock()
			// Debug: log sukses pengiriman
			utils.GetLogger().Info("BroadcastToGroupsForAccount: Berhasil mengirim ke grup %s untuk akun %d", targetJID.String(), account.ID)

			// Simpan Message ID untuk cross-account read
			if resp.ID != "" {
				state.StopMutex.Lock()
				if state.LastSentMessages[account.ID] == nil {
					state.LastSentMessages[account.ID] = make(map[types.JID]*types.MessageID)
				}
				msgIDCopy := resp.ID
				state.LastSentMessages[account.ID][targetJID] = &msgIDCopy
				state.StopMutex.Unlock()
				utils.GetLogger().Info("BroadcastToGroupsForAccount: Saved LastSentMessages untuk akun %d, grup %s, msgID=%s", account.ID, targetJID.String(), resp.ID)
			} else {
				utils.GetLogger().Warn("BroadcastToGroupsForAccount: resp.ID kosong untuk akun %d, grup %s", account.ID, targetJID.String())
			}
		}

		// Delay antar grup (WAJIB diterapkan untuk setiap grup kecuali grup terakhir)
		if state.GroupDelay > 0 {
			// Debug: log delay
			utils.GetLogger().Info("BroadcastToGroupsForAccount: Delay %d detik sebelum grup berikutnya (akun %d)", state.GroupDelay, account.ID)
			time.Sleep(time.Duration(state.GroupDelay) * time.Second)
		}
	}
}

// ApplyAutoVariation applies auto-variation to a message (Base + Random, no limits)
func ApplyAutoVariation(baseMessage string, accountID int, loopNumber int) string {
	// Generate unique seed berdasarkan account ID + loop + timestamp
	seed := fmt.Sprintf("%d_%d_%d", accountID, loopNumber, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(seed))

	// Random variation elements (NO LIMITS)
	variations := []string{
		"",    // No variation
		" ",   // Extra space
		"\n",  // Line break
		"✨",   // Emoji
		"🌟",   // Emoji
		"💫",   // Emoji
		"🎉",   // Emoji
		"🔥",   // Emoji
		"⭐",   // Emoji
		"❤️",  // Emoji
		"👍",   // Emoji
		"👏",   // Emoji
		"🙌",   // Emoji
		"🚀",   // Emoji
		"💪",   // Emoji
		"😊",   // Emoji
		"😎",   // Emoji
		"🎯",   // Emoji
		"📢",   // Emoji
		"✅",   // Emoji
		"🔥 ",  // Emoji + space
		" ✨",  // Space + emoji
		"\n✨", // Newline + emoji
		"✨\n", // Emoji + newline
		"💫 ",  // Emoji + space
		" 🌟",  // Space + emoji
	}

	// Pick random variation based on hash
	variationIndex := int(hash[0]) % len(variations)
	selectedVariation := variations[variationIndex]

	// Random position: start, middle, end
	position := int(hash[1]) % 3

	var result string
	switch position {
	case 0: // Start
		result = selectedVariation + baseMessage
	case 1: // Middle (if message long enough)
		if len(baseMessage) > 10 {
			mid := len(baseMessage) / 2
			result = baseMessage[:mid] + selectedVariation + baseMessage[mid:]
		} else {
			result = baseMessage + selectedVariation
		}
	case 2: // End
		result = baseMessage + selectedVariation
	}

	// Add random extra characters sometimes (based on hash)
	if hash[2]%3 == 0 {
		extraChars := []string{"!", ".", "?", "~", "…", "✨", "🌟"}
		extraIndex := int(hash[3]) % len(extraChars)
		result += extraChars[extraIndex]
	}

	// Ensure UTF-8 validity
	if !utf8.ValidString(result) {
		return baseMessage // Fallback to original
	}

	return result
}

// MonitorBroadcastProgress monitors and updates broadcast progress
func MonitorBroadcastProgress(telegramBot *tgbotapi.BotAPI, chatID int64) {
	var lastMsgID int

	for {
		state := GetBroadcastState(chatID)

		if !state.IsRunning {
			break
		}

		// Update every 5 seconds
		time.Sleep(5 * time.Second)

		// Build progress message
		progressMsg := BuildProgressMessage(state)

		// Send or update message
		if lastMsgID == 0 {
			msg := tgbotapi.NewMessage(chatID, progressMsg)
			msg.ParseMode = "Markdown"
			sentMsg, err := telegramBot.Send(msg)
			if err == nil && sentMsg.MessageID > 0 {
				lastMsgID = sentMsg.MessageID
			}
		} else {
			editMsg := tgbotapi.NewEditMessageText(chatID, lastMsgID, progressMsg)
			editMsg.ParseMode = "Markdown"
			telegramBot.Send(editMsg)
		}
	}
}

// BuildProgressMessage builds the progress message
func BuildProgressMessage(state *BroadcastState) string {
	am := GetAccountManager()
	accounts := am.GetAllAccounts()

	var builder strings.Builder
	builder.WriteString("📊 **PROGRESS BROADCAST**\n\n")
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	builder.WriteString(fmt.Sprintf("🔄 **Loop:** %d\n", state.CurrentLoop))
	builder.WriteString(fmt.Sprintf("🎯 **Target Grup:** %d\n\n", len(state.TargetGroups)))

	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	builder.WriteString("**📱 Status Akun:**\n\n")

	for i, acc := range accounts {
		statusIcon := "🟢"
		if !state.AccountStatus[acc.ID] {
			statusIcon = "🔴"
		}

		sent := state.TotalSent[acc.ID]
		failed := state.TotalFailed[acc.ID]

		statusText := "Running"
		if !state.AccountStatus[acc.ID] {
			statusText = "Disconnected"
		}

		builder.WriteString(fmt.Sprintf("%s **Akun %d:** +%s\n", statusIcon, i+1, acc.PhoneNumber))
		builder.WriteString(fmt.Sprintf("   Status: %s\n", statusText))
		builder.WriteString(fmt.Sprintf("   ✅ Terkirim: %d | ❌ Gagal: %d\n\n", sent, failed))
	}

	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	builder.WriteString(fmt.Sprintf("**📈 Total:** ✅ %d | ❌ %d\n\n", GetTotalSent(state), GetTotalFailed(state)))
	builder.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	builder.WriteString("⏹️ Stop: Ketik `/stopchat`")

	return builder.String()
}

// GetTotalSent gets total sent messages across all accounts
func GetTotalSent(state *BroadcastState) int {
	total := 0
	for _, count := range state.TotalSent {
		total += count
	}
	return total
}

// GetTotalFailed gets total failed messages across all accounts
func GetTotalFailed(state *BroadcastState) int {
	total := 0
	for _, count := range state.TotalFailed {
		total += count
	}
	return total
}

// StopBroadcast stops the broadcast
func StopBroadcast(chatID int64) {
	state := GetBroadcastState(chatID)

	state.StopMutex.Lock()
	state.ShouldStop = true
	state.StopMutex.Unlock()

	state.IsRunning = false
}

// IsBroadcastRunning checks if broadcast is running
func IsBroadcastRunning(chatID int64) bool {
	state := GetBroadcastState(chatID)
	return state.IsRunning
}

// escapeLikePattern escape karakter khusus untuk SQL LIKE pattern
// Escape: %, _, [, ], |, \, ^, $ menjadi \% \_ \[ \] \| \\ \^ \$
func escapeLikePattern(pattern string) string {
	var result strings.Builder
	for _, char := range pattern {
		switch char {
		case '%', '_', '[', ']', '|', '\\', '^', '$':
			result.WriteRune('\\')
			result.WriteRune(char)
		default:
			result.WriteRune(char)
		}
	}
	return result.String()
}
