package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/handlers"
	"whatsapp-bot/ui"
	"whatsapp-bot/utils"
)

var client *whatsmeow.Client
var telegramBot *tgbotapi.BotAPI
var isPairingInProgress = false // Flag untuk menghindari duplikasi notifikasi

// ensureDatabaseWritable memastikan database bisa ditulis
func ensureDatabaseWritable(dbPath string) error {
	// Cek apakah file exist
	if _, err := os.Stat(dbPath); err == nil {
		// File exist, cek apakah bisa ditulis
		file, err := os.OpenFile(dbPath, os.O_RDWR, 0666)
		if err != nil {
			return fmt.Errorf("tidak bisa membuka database untuk write: %v", err)
		}
		file.Close()
	} else if os.IsNotExist(err) {
		// File tidak exist, pastikan directory bisa ditulis
		dir := filepath.Dir(dbPath)
		if dir == "" || dir == "." {
			dir = "."
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("tidak bisa membuat directory: %v", err)
		}
		// Test write dengan membuat file temp
		testFile := filepath.Join(dir, ".write_test")
		file, err := os.Create(testFile)
		if err != nil {
			return fmt.Errorf("directory tidak bisa ditulis: %v", err)
		}
		file.Close()
		os.Remove(testFile)
	} else {
		return fmt.Errorf("error cek database: %v", err)
	}

	return nil
}

func main() {
	// Inisialisasi logger untuk grup
	utils.InitGrupLogger(true) // true = debug mode

	fmt.Println("🚀 Starting WhatsApp Bot dengan Telegram Integration...")
	fmt.Println("📝 Created by @yourname\n")

	// 1. Load config Telegram
	telegramConfig, err := utils.LoadTelegramConfig()
	if err != nil {
		log.Fatal("❌ Gagal load config Telegram! File akses.json diperlukan!")
	}

	// 2. Setup Telegram bot
	telegramBot, err = tgbotapi.NewBotAPI(telegramConfig.TelegramToken)
	if err != nil {
		log.Fatal("❌ Gagal setup Telegram bot! Token tidak valid!")
	}

	// Set config ke handlers
	handlers.SetTelegramConfig(telegramConfig)

	// 3. Tampilkan Welcome Message
	ui.ShowWelcome(telegramBot, telegramConfig.UserAllowedID)
	time.Sleep(2 * time.Second) // Delay untuk efek yang lebih baik

	// 4. Setup database WhatsApp
	// Cek apakah sudah ada database dengan format baru (setelah pairing sebelumnya)
	whatsappDBPath := "whatsapp.db"

	if utils.LoadDBConfigFromFile() {
		// Database dengan format baru ditemukan
		whatsappDBPath = utils.GetWhatsAppDBPath()
		botDataDBPath := utils.GetBotDataDBPath()
		fmt.Printf("✅ Database yang sudah ada ditemukan:\n   WhatsApp DB: %s\n   Bot Data DB: %s\n", whatsappDBPath, botDataDBPath)
	} else {
		// Belum ada database dengan format baru, gunakan default
		fmt.Printf("📝 Menggunakan database default (akan di-rename setelah pairing berhasil)\n")
	}

	// Pastikan database bisa ditulis dengan menambahkan mode=rwc
	dbLog := waLog.Stdout("Database", "ERROR", true)

	// Cek permission file database terlebih dahulu
	if err := ensureDatabaseWritable(whatsappDBPath); err != nil {
		log.Fatalf("❌ Database tidak bisa ditulis: %v\nPastikan file %s memiliki permission write (chmod 644 atau 666)", err, whatsappDBPath)
	}

	// Gunakan mode=rwc (read-write-create) dan WAL mode untuk memastikan database bisa ditulis
	// Tambahkan cache=shared untuk meningkatkan performa dan mengurangi lock issues
	dbConnectionString := fmt.Sprintf("file:%s?_foreign_keys=on&mode=rwc&_journal_mode=WAL&cache=shared", whatsappDBPath)
	container, err := sqlstore.New(context.Background(), "sqlite3", dbConnectionString, dbLog)
	if err != nil {
		log.Fatal("❌ Gagal setup database:", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatal("❌ Gagal get device store:", err)
	}

	// 5. Clear app_state jika perlu
	if err := utils.ClearAppState(); err != nil {
		handlers.SendToTelegram(fmt.Sprintf("⚠️ Gagal clear app_state: %v", err))
	}

	// 6. Setup WhatsApp client
	baseLog := waLog.Stdout("Client", "ERROR", true)
	clientLog := &utils.FilteredLogger{Logger: baseLog}
	client = whatsmeow.NewClient(deviceStore, clientLog)

	// Set clients ke handlers
	handlers.SetClients(client, telegramBot)

	// 7. Register event handler
	client.AddEventHandler(eventHandler)

	// 8. Connect ke WhatsApp
	if err := client.Connect(); err != nil {
		handlers.SendToTelegram(fmt.Sprintf("❌ Gagal connect: %v", err))
		log.Fatal("❌ Gagal connect:", err)
	}

	// 9. Setup database bot
	if err := utils.SetupBotDB(); err != nil {
		handlers.SendToTelegram(fmt.Sprintf("⚠️ Gagal setup bot database: %v", err))
	} else {
		handlers.SendToTelegram("✅ Bot database siap")
	}

	// 10. Jika sudah login, update konfigurasi database
	if client.Store.ID != nil {
		whatsappNumber := client.Store.ID.User
		// Set konfigurasi database jika belum di-set (untuk kompatibilitas dengan database lama)
		if utils.GetDBConfig() == nil {
			utils.SetDBConfig(telegramConfig.UserAllowedID, whatsappNumber)
		}
	}

	// 11. Cek status login dan tampilkan UI sesuai kondisi
	time.Sleep(1 * time.Second)
	if client.Store.ID == nil {
		// Belum login - tampilkan prompt login
		handlers.SendToTelegram("🔄 Memeriksa status login...")
		time.Sleep(1 * time.Second)
		ui.ShowLoginPrompt(telegramBot, telegramConfig.UserAllowedID)
	} else {
		// Sudah login - tampilkan menu utama
		handlers.SendToTelegram("✅ Bot WhatsApp sudah terhubung!")
		time.Sleep(1 * time.Second)
		ui.ShowMainMenu(telegramBot, telegramConfig.UserAllowedID, client)
	}

	// 11. Jalankan Telegram bot handler di background
	go startTelegramBot(telegramConfig)

	// 12. Tunggu signal shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	handlers.SendToTelegram("\n👋 Menutup bot...")
	if telegramBot != nil {
		telegramBot.StopReceivingUpdates()
	}
	client.Disconnect()
}

// Start Telegram bot handler
func startTelegramBot(config *utils.TelegramConfig) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := telegramBot.GetUpdatesChan(u)

	fmt.Println("✅ Telegram bot handler aktif!")

	for update := range updates {
		// Handle inline keyboard callback (tombol di menu)
		if update.CallbackQuery != nil {
			// Cek akses user
			userID := update.CallbackQuery.From.ID
			if int64(userID) != config.UserAllowedID {
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "❌ Anda tidak memiliki akses.")
				telegramBot.Request(callback)
				continue
			}

			// Handle callback berdasarkan data
			handlers.HandleCallbackQuery(update.CallbackQuery, client, telegramBot)
			continue
		}

		// Handle pesan text
		if update.Message == nil {
			continue
		}

		// Cek akses user
		userID := update.Message.From.ID
		if int64(userID) != config.UserAllowedID {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Anda tidak memiliki akses untuk menggunakan bot ini.")
			telegramBot.Send(msg)
			continue
		}

		// Handle commands
		if update.Message.IsCommand() {
			handlers.HandleTelegramCommand(update.Message, client, telegramBot)
			continue
		}

		// Handle pesan text biasa
		text := update.Message.Text
		chatID := update.Message.Chat.ID

		// Cek apakah sedang menunggu input nomor telepon
		if handlers.WaitingForPhoneNumber[chatID] {
			phoneNumber := strings.TrimSpace(text)
			handlers.HandlePhoneNumberInput(phoneNumber, chatID, client, telegramBot)
			continue
		}

		// Default response
		msg := tgbotapi.NewMessage(chatID, "ℹ️ Gunakan /menu untuk melihat menu utama atau /help untuk bantuan.")
		telegramBot.Send(msg)
	}
}

// All handlers moved to handlers/telegram.go

// Event handler untuk WhatsApp
func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		handleMessage(v)
	case *events.Connected:
		// Client terhubung - tidak ada aksi yang diperlukan
	case *events.Disconnected:
		handlers.SendToTelegram("❌ Disconnected from WhatsApp!")
	case *events.LoggedOut:
		handlers.SendToTelegram("🚪 Logged out!")
	case *events.PairSuccess:
		// Pair success sudah ditangani di PairDeviceViaTelegram, skip di sini untuk menghindari duplikasi
		return
	}
}

func handleMessage(msg *events.Message) {
	if msg.Info.IsFromMe {
		return
	}

	messageText := extractMessageText(msg.Message)
	sender := msg.Info.Sender.String()
	chatType := "Personal"
	if msg.Info.IsGroup {
		chatType = "Group"
		// Simpan grup ke database
		groupJID := msg.Info.Chat.String()
		// Coba ambil nama grup
		groupName := ""
		if client != nil {
			groupInfo, err := client.GetGroupInfo(context.Background(), msg.Info.Chat)
			if err == nil && groupInfo != nil {
				groupName = groupInfo.Name
			}
		}
		go utils.SaveGroupToDB(groupJID, groupName)
	}

	fmt.Printf("📨 [%s] From: %s, Message: %s\n", chatType, sender, messageText)
	go utils.SaveMessageToDB(sender, messageText, msg.Info.Timestamp.Format(time.RFC3339))
	go notifyTelegram(sender, messageText, chatType)

	response := handleCommands(messageText, msg.Info.IsGroup, msg.Info.Sender, msg.Info.Chat)
	if response != "" {
		sendMessage(msg.Info.Chat.String(), response)
	}
}

func extractMessageText(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Conversation != nil {
		return *msg.Conversation
	}
	if msg.ExtendedTextMessage != nil {
		return msg.ExtendedTextMessage.GetText()
	}
	if msg.ImageMessage != nil {
		caption := msg.ImageMessage.GetCaption()
		if caption != "" {
			return "[Gambar] " + caption
		}
		return "[Gambar]"
	}
	if msg.VideoMessage != nil {
		caption := msg.VideoMessage.GetCaption()
		if caption != "" {
			return "[Video] " + caption
		}
		return "[Video]"
	}
	if msg.DocumentMessage != nil {
		fileName := msg.DocumentMessage.GetFileName()
		if fileName != "" {
			return "[Document] " + fileName
		}
		return "[Document]"
	}
	return "[Media/Unknown]"
}

func handleCommands(cmd string, isGroup bool, sender types.JID, chat types.JID) string {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	switch cmd {
	case "!ping", "ping", "test":
		return "🏓 Pong! Bot aktif!"
	case "!info", "info":
		return fmt.Sprintf("🤖 Bot Info\nUser: %s\nGroup: %v", sender.String(), isGroup)
	case "!time", "time":
		return fmt.Sprintf("🕐 %s", time.Now().Format("15:04:05"))
	case "!help", "help":
		return "📋 Menu: !ping, !info, !time, !help"
	}
	return ""
}

func sendMessage(to, text string) {
	jid, err := types.ParseJID(to)
	if err != nil {
		return
	}
	client.SendMessage(context.Background(), jid, &waProto.Message{
		Conversation: proto.String(text),
	})
}

func notifyTelegram(sender, messageText, chatType string) {
	if telegramBot == nil || handlers.TelegramConfig == nil {
		return
	}
	notification := fmt.Sprintf("📨 Pesan WhatsApp\n\nType: %s\nFrom: %s\nMessage: %s\nTime: %s",
		chatType, sender, messageText, time.Now().Format("15:04:05"))
	msg := tgbotapi.NewMessage(handlers.TelegramConfig.UserAllowedID, notification)
	telegramBot.Send(msg)
}
