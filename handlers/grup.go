package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const (
	// Batas maksimal karakter per pesan Telegram
	MaxTelegramMessageLength = 4096
	// Reserve space untuk header/footer
	ReservedSpace = 500
	// Panjang maksimal per pesan
	MaxMessageLength = MaxTelegramMessageLength - ReservedSpace
	// Concurrent API calls limit untuk performance (dikurangi untuk menghindari rate limit)
	MaxConcurrentAPICalls = 3
	// Progress update interval (update setiap N grup)
	ProgressUpdateInterval = 10
	// Context timeout untuk API calls
	APICallTimeout = 15 * time.Second
	// Delay antar batch untuk menghindari rate limit
	BatchDelay = 500 * time.Millisecond
)

// GetGroupList mengambil semua daftar grup WhatsApp dan mengirimkannya ke Telegram
func GetGroupList(client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI, chatID int64) error {
	if client == nil || client.Store.ID == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
		telegramBot.Send(msg)
		return fmt.Errorf("client belum terhubung")
	}

	// SECURITY: Validasi bahwa user hanya bisa mengakses WhatsApp account yang sesuai dengan Telegram ID mereka
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)

	if userAccount == nil {
		// User belum memiliki akun terdaftar, tidak bisa mengakses
		msg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan /pair untuk melakukan pairing terlebih dahulu.")
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return fmt.Errorf("user %d tidak memiliki akun terdaftar", chatID)
	}

	// Validasi bahwa client yang digunakan sesuai dengan akun user
	// Cek apakah nomor WhatsApp dari client sesuai dengan akun user
	if client.Store.ID != nil {
		clientPhoneNumber := client.Store.ID.User
		if clientPhoneNumber != userAccount.PhoneNumber {
			// Client tidak sesuai dengan akun user, switch ke akun user
			utils.GetLogger().Warn("Security: Client phone mismatch for user %d. Expected: %s, Got: %s. Switching account...", chatID, userAccount.PhoneNumber, clientPhoneNumber)

			// Switch ke akun user
			if err := SwitchAccount(userAccount.ID, telegramBot, chatID); err != nil {
				msg := tgbotapi.NewMessage(chatID, "❌ **AKSES DITOLAK**\n\nGagal mengakses akun WhatsApp Anda.\n\nSilakan coba lagi atau hubungi admin.")
				msg.ParseMode = "Markdown"
				telegramBot.Send(msg)
				return fmt.Errorf("failed to switch to user account: %w", err)
			}

			// CRITICAL FIX: Gunakan client dari account user, bukan GetCurrentClient() yang bisa dari user lain
			client = am.GetClient(userAccount.ID)
			if client == nil {
				// Coba buat client jika belum ada
				var err error
				client, err = am.CreateClient(userAccount.ID)
				if err != nil {
					msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
					telegramBot.Send(msg)
					return fmt.Errorf("failed to create client for user account: %w", err)
				}
			}

			if client == nil || client.Store.ID == nil {
				msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
				telegramBot.Send(msg)
				return fmt.Errorf("client belum terhubung setelah switch")
			}

			// CRITICAL FIX: Update dbConfig setelah switch untuk memastikan menggunakan database user yang benar
			EnsureDBConfigForUser(chatID, userAccount)

			utils.GetLogger().Info("Security: Successfully switched to user account for TelegramID %d", chatID)
		} else {
			// CRITICAL FIX: Pastikan dbConfig di-update bahkan jika client sudah benar
			EnsureDBConfigForUser(chatID, userAccount)
		}
	}

	// Kirim pesan loading
	loadingMsg := tgbotapi.NewMessage(chatID, "🔄 **Memuat daftar grup WhatsApp...**\n\n⏳ Ini mungkin memakan waktu beberapa saat untuk mengambil semua nama grup.")
	loadingMsg.ParseMode = "Markdown"
	loadingMsgSent, _ := telegramBot.Send(loadingMsg)
	loadingMsgID := loadingMsgSent.MessageID

	// Ambil semua grup dari database/store dengan progress callback
	progressCallback := func(current, total, updated, failed int) {
		updateProgressMessage(telegramBot, chatID, loadingMsgID, current, total, updated, failed)
	}

	allGroups, err := fetchAllGroups(client, telegramBot, chatID, loadingMsgID, progressCallback)
	if err != nil {
		// Hapus pesan loading
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgID)
		telegramBot.Request(deleteMsg)

		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ %s", utils.ErrorMsg(err)))
		errorMsg.ParseMode = "Markdown"
		telegramBot.Send(errorMsg)
		return err
	}

	// Hapus pesan loading
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, loadingMsgID)
	telegramBot.Request(deleteMsg)

	// Debug: Log jumlah grup yang diambil
	totalFetched := len(allGroups)
	utils.GetGrupLogger().Info("Total grup diambil: %d", totalFetched)

	if totalFetched == 0 {
		msg := tgbotapi.NewMessage(chatID, `📭 **TIDAK ADA GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Bot belum menemukan grup WhatsApp yang diikuti.

**Kemungkinan penyebab:**
• Bot belum pernah menerima pesan dari grup
• Database belum terisi dengan grup yang diikuti
• Bot tidak terhubung ke WhatsApp

**Solusi:**
1. Pastikan bot sudah login ke WhatsApp
2. Coba kirim pesan di salah satu grup yang diikuti
3. Atau tunggu beberapa saat lalu coba lagi

💡 Bot akan otomatis mendeteksi grup saat ada pesan masuk.`)
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return nil
	}

	// Jangan filter terlalu ketat - tampilkan semua grup yang sudah punya nama
	// Filter hanya grup yang benar-benar hanya UID tanpa nama
	validGroups := filterValidGroups(allGroups)

	// Debug info
	utils.GetGrupLogger().Info("Total grup diambil: %d, Grup dengan nama valid: %d", totalFetched, len(validGroups))

	if len(validGroups) == 0 {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(`📭 **TIDAK ADA GRUP DITEMUKAN**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Total grup yang diambil: %d
Grup dengan nama valid: 0

**Catatan:**
Grup yang hanya memiliki UID (nomor ID) tidak ditampilkan.

💡 Pastikan grup WhatsApp memiliki nama yang jelas.
💡 Grup akan otomatis mendapatkan nama saat bot menerima pesan dari grup tersebut.`, totalFetched))
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
		return nil
	}

	// Urutkan grup berdasarkan nama (alfabet + angka)
	sortedGroups := sortGroupsByName(validGroups)

	// Format dan kirim daftar grup
	sendGroupListInChunks(telegramBot, chatID, sortedGroups)

	return nil
}

// GroupInfo menyimpan informasi grup
type GroupInfo struct {
	JID  types.JID
	Name string
}

// fetchAllGroups mengambil semua grup dari WhatsApp dengan progress callback
// Menggunakan GetJoinedGroups() dari whatsmeow untuk performa optimal
func fetchAllGroups(client *whatsmeow.Client, telegramBot *tgbotapi.BotAPI, chatID int64, loadingMsgID int, progressCallback func(current, total, updated, failed int)) ([]GroupInfo, error) {
	var groups []GroupInfo

	if client == nil || client.Store == nil {
		return nil, fmt.Errorf("client atau store tidak tersedia")
	}

	logger := utils.GetGrupLogger()
	logger.Debug("Mulai fetchAllGroups")

	// Prioritas 1: Coba ambil semua grup sekaligus dari API menggunakan GetJoinedGroups()
	// Ini lebih efisien dan menghindari rate limit dari banyak GetGroupInfo() calls
	if client.IsConnected() {
		logger.Info("Mencoba mengambil grup menggunakan GetJoinedGroups() dari whatsmeow")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		joinedGroups, err := client.GetJoinedGroups(ctx)
		if err == nil && len(joinedGroups) > 0 {
			logger.Info("Berhasil mengambil %d grup dari GetJoinedGroups()", len(joinedGroups))

			// Convert dari whatsmeow types.GroupInfo ke GroupInfo kita
			for _, group := range joinedGroups {
				if group != nil {
					// Cek apakah JID adalah grup (grup selalu berakhiran @g.us)
					jidStr := group.JID.String()
					if strings.HasSuffix(jidStr, "@g.us") {
						groupName := group.Name
						// Jika nama kosong, gunakan fallback
						if groupName == "" {
							groupName = fmt.Sprintf("Grup %s", group.JID.User)
						}

						groups = append(groups, GroupInfo{
							JID:  group.JID,
							Name: groupName,
						})
					}
				}
			}

			// Batch save semua nama ke database untuk penggunaan selanjutnya
			groupsToSave := make(map[string]string)
			for _, group := range groups {
				if group.Name != "" && group.Name != group.JID.User && group.Name != group.JID.String() {
					groupsToSave[group.JID.String()] = group.Name
				}
			}

			if len(groupsToSave) > 0 {
				if err := utils.BatchSaveGroupsToDB(groupsToSave); err != nil {
					logger.Error("Gagal batch save groups: %v", err)
				} else {
					logger.Info("Berhasil batch save %d grup ke database", len(groupsToSave))
				}
			}

			logger.Info("Total %d grup berhasil diambil dari GetJoinedGroups()", len(groups))
			return groups, nil
		}

		// Jika GetJoinedGroups() gagal, log warning dan fallback ke database
		if err != nil {
			logger.Warn("GetJoinedGroups() gagal: %v, fallback ke database", err)
		}
	}

	// Prioritas 2: Fallback ke database WhatsApp (whatsapp.db)
	logger.Info("Mengambil grup dari database WhatsApp sebagai fallback")
	groupsFromWA, err := fetchGroupsFromWhatsAppDBFast()
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil grup dari database: %v", err)
	}

	// Jika ada grup dari database WhatsApp
	if len(groupsFromWA) > 0 {
		logger.Info("%d grup diambil dari database WhatsApp", len(groupsFromWA))

		// Untuk grup yang belum punya nama, ambil dari API WhatsApp (dengan concurrent calls)
		// Tapi hanya untuk grup yang benar-benar belum punya nama
		groups = enrichGroupNamesFromAPIConcurrent(client, groupsFromWA, telegramBot, chatID, loadingMsgID, progressCallback)

		logger.Info("Setelah enrich dari API, total %d grup", len(groups))

		// Pastikan semua grup tetap ada (tidak ada yang hilang selama enrich)
		if len(groups) < len(groupsFromWA) {
			logger.Warn("Jumlah grup berkurang dari %d menjadi %d setelah enrich!", len(groupsFromWA), len(groups))
		}

		// Batch save semua nama ke database bot untuk efisiensi
		groupsToSave := make(map[string]string)
		for _, group := range groups {
			if group.Name != "" && group.Name != group.JID.User && group.Name != group.JID.String() {
				groupsToSave[group.JID.String()] = group.Name
			}
		}

		if len(groupsToSave) > 0 {
			if err := utils.BatchSaveGroupsToDB(groupsToSave); err != nil {
				logger.Error("Gagal batch save groups: %v", err)
				// Fallback: save individual jika batch gagal
				for jid, name := range groupsToSave {
					go utils.SaveGroupToDB(jid, name)
				}
			} else {
				logger.Info("Berhasil batch save %d grup ke database", len(groupsToSave))
			}
		}
		return groups, nil
	}

	// Fallback: ambil dari bot_data.db jika ada
	groupMap, err := utils.GetAllGroupsFromDB()
	if err == nil && len(groupMap) > 0 {
		for jidStr, groupName := range groupMap {
			jid, err := types.ParseJID(jidStr)
			if err != nil {
				continue
			}
			groups = append(groups, GroupInfo{
				JID:  jid,
				Name: groupName,
			})
		}
	}

	return groups, nil
}

// fetchGroupsFromWhatsAppDBFast mengambil grup dari database WhatsApp dengan cepat (tanpa API call)
func fetchGroupsFromWhatsAppDBFast() ([]GroupInfo, error) {
	var groups []GroupInfo

	// Buka database WhatsApp
	db, err := utils.OpenWhatsAppDB()
	if err != nil {
		return groups, err
	}
	defer db.Close()

	// Query langsung dari database dengan JOIN untuk mendapatkan nama grup
	// Menggunakan LEFT JOIN untuk menggabungkan chat_settings dengan contacts
	// dan bot_data.db untuk nama yang sudah disimpan
	// Note: No ORDER BY here - will be sorted naturally in Go
	rows, err := db.Query(`
		SELECT DISTINCT 
			cs.chat_jid,
			COALESCE(
				NULLIF(c.full_name, ''),
				NULLIF(c.first_name, ''),
				cs.chat_jid
			) as group_name
		FROM whatsmeow_chat_settings cs
		LEFT JOIN whatsmeow_contacts c ON c.their_jid = cs.chat_jid
		WHERE cs.chat_jid LIKE '%@g.us'
	`)
	if err != nil {
		// Jika query dengan JOIN gagal, coba query sederhana
		return fetchGroupsFromWhatsAppDBSimple(db)
	}
	defer rows.Close()

	// Ambil nama dari bot_data.db untuk yang sudah disimpan
	botGroupMap, _ := utils.GetAllGroupsFromDB()

	for rows.Next() {
		var jidStr, groupName string
		if err := rows.Scan(&jidStr, &groupName); err != nil {
			continue
		}

		jid, err := types.ParseJID(jidStr)
		if err != nil {
			continue
		}

		// Prioritaskan nama dari bot_data.db (yang sudah pernah disimpan saat ada pesan)
		if savedName, exists := botGroupMap[jidStr]; exists && savedName != "" && savedName != jidStr && savedName != jid.User {
			// Gunakan nama yang sudah disimpan jika valid
			groupName = savedName
		} else {
			// Jika tidak ada nama di bot_data.db, cek dari contacts
			// Jika contacts juga kosong, gunakan format fallback
			if groupName == "" || groupName == jidStr || groupName == jid.User {
				groupName = fmt.Sprintf("Grup %s", jid.User)
			}
		}

		groups = append(groups, GroupInfo{
			JID:  jid,
			Name: groupName,
		})
	}

	utils.GetGrupLogger().Debug("Selesai fetchGroupsFromWhatsAppDBFast, total %d grup", len(groups))
	return groups, nil
}

// fetchGroupsFromWhatsAppDBSimple query sederhana jika JOIN gagal
func fetchGroupsFromWhatsAppDBSimple(db *sql.DB) ([]GroupInfo, error) {
	var groups []GroupInfo

	rows, err := db.Query(`
		SELECT DISTINCT chat_jid 
		FROM whatsmeow_chat_settings 
		WHERE chat_jid LIKE '%@g.us'
	`)
	if err != nil {
		return groups, nil
	}
	defer rows.Close()

	// Ambil nama dari bot_data.db
	botGroupMap, _ := utils.GetAllGroupsFromDB()

	// Query nama dari contacts
	contactMap := make(map[string]string)
	contactRows, err := db.Query(`
		SELECT their_jid, COALESCE(NULLIF(full_name, ''), NULLIF(first_name, ''), their_jid) as name
		FROM whatsmeow_contacts
		WHERE their_jid LIKE '%@g.us'
	`)
	if err == nil {
		defer contactRows.Close()
		for contactRows.Next() {
			var jid, name string
			if contactRows.Scan(&jid, &name) == nil {
				contactMap[jid] = name
			}
		}
	}

	for rows.Next() {
		var jidStr string
		if err := rows.Scan(&jidStr); err != nil {
			continue
		}

		jid, err := types.ParseJID(jidStr)
		if err != nil {
			continue
		}

		// Prioritaskan nama dari bot_data.db, lalu contacts, terakhir JID
		groupName := ""
		if savedName, exists := botGroupMap[jidStr]; exists && savedName != "" {
			groupName = savedName
		} else if contactName, exists := contactMap[jidStr]; exists && contactName != "" && contactName != jidStr {
			groupName = contactName
		} else {
			groupName = fmt.Sprintf("Grup %s", jid.User)
		}

		groups = append(groups, GroupInfo{
			JID:  jid,
			Name: groupName,
		})
	}

	return groups, nil
}

// enrichGroupNamesFromAPI adalah wrapper untuk backward compatibility
// Sebaiknya gunakan enrichGroupNamesFromAPIConcurrent untuk performa lebih baik
func enrichGroupNamesFromAPI(client *whatsmeow.Client, groups []GroupInfo) []GroupInfo {
	// Redirect ke concurrent version (tanpa progress callback untuk backward compat)
	return enrichGroupNamesFromAPIConcurrent(client, groups, nil, 0, 0, nil)
}

// filterValidGroups memfilter grup yang valid
// Tampilkan SEMUA grup yang memiliki nama, termasuk yang formatnya "Grup {UID}"
// Hanya skip grup yang benar-benar tidak memiliki nama sama sekali atau sama dengan JID
func filterValidGroups(groups []GroupInfo) []GroupInfo {
	var validGroups []GroupInfo

	for _, group := range groups {
		groupName := strings.TrimSpace(group.Name)

		// Skip grup yang namanya benar-benar kosong
		if groupName == "" {
			continue
		}

		// Skip jika nama sama dengan JID User atau JID lengkap (belum punya nama yang proper)
		// Tapi jangan skip jika nama sama dengan JID User tapi dalam format "Grup {UID}"
		if groupName == group.JID.String() {
			// Nama sama persis dengan JID lengkap, skip
			continue
		}

		// Jika nama sama dengan JID User (angka saja), cek apakah sudah dalam format "Grup {UID}"
		if groupName == group.JID.User {
			// Jika belum diformat, skip (akan diformat nanti)
			// Tapi karena sudah ada di database dengan format "Grup {UID}", ini seharusnya jarang terjadi
			continue
		}

		// Tampilkan SEMUA grup lainnya, termasuk yang formatnya "Grup {UID}"
		// Karena semua grup WhatsApp seharusnya memiliki nama dari API atau database
		// Atau minimal memiliki format "Grup {UID}" yang menunjukkan grup memang ada
		validGroups = append(validGroups, group)
	}

	logger := utils.GetGrupLogger()
	logger.Debug("Filter: %d grup masuk, %d grup valid setelah filter", len(groups), len(validGroups))

	// Peringatan jika banyak grup terfilter
	if len(validGroups) < len(groups) {
		filteredCount := len(groups) - len(validGroups)
		logger.Warn("%d grup terfilter! Grup yang terfilter kemungkinan tidak memiliki nama atau nama sama dengan JID", filteredCount)
	}

	return validGroups
}

// sortGroupsByName mengurutkan grup berdasarkan nama menggunakan natural sorting
func sortGroupsByName(groups []GroupInfo) []GroupInfo {
	sorted := make([]GroupInfo, len(groups))
	copy(sorted, groups)

	// Use natural sorting algorithm
	sort.Slice(sorted, func(i, j int) bool {
		nameI := strings.TrimSpace(sorted[i].Name)
		nameJ := strings.TrimSpace(sorted[j].Name)

		// Jika nama kosong, letakkan di akhir
		if nameI == "" && nameJ == "" {
			return false
		}
		if nameI == "" {
			return false
		}
		if nameJ == "" {
			return true
		}

		// Use natural comparison from utils
		return utils.NaturalLess(nameI, nameJ)
	})

	return sorted
}

// compareNames is deprecated - use utils.NaturalLess instead
// Kept for backwards compatibility only
func compareNames(name1, name2 string) bool {
	return utils.NaturalLess(name1, name2)
}

// sendGroupListInChunks mengirim daftar grup dalam beberapa chunk jika terlalu panjang
func sendGroupListInChunks(telegramBot *tgbotapi.BotAPI, chatID int64, groups []GroupInfo) {
	if len(groups) == 0 {
		return
	}

	// Header untuk pesan pertama
	header := fmt.Sprintf(`👥 **DAFTAR GRUP WHATSAPP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Total:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

`, len(groups))

	var currentMessage strings.Builder
	currentMessage.WriteString(header)
	currentMessageLength := len(header)

	messageCount := 1

	for _, group := range groups {
		// Format nama grup (escape markdown jika perlu)
		// Note: grup sudah difilter oleh filterValidGroups sebelumnya
		groupName := escapeMarkdown(group.Name)

		// Format baris grup tanpa nomor urut (hanya nama grup)
		line := fmt.Sprintf("%s\n", groupName)

		// Cek apakah menambahkan baris ini akan melebihi batas
		if currentMessageLength+len(line) > MaxMessageLength && currentMessageLength > len(header) {
			// Kirim pesan saat ini
			msg := tgbotapi.NewMessage(chatID, currentMessage.String())
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)

			// Tunggu sebentar untuk menghindari rate limit
			time.Sleep(100 * time.Millisecond)

			// Reset untuk pesan berikutnya
			currentMessage.Reset()
			messageCount++
			header = fmt.Sprintf(`👥 **DAFTAR GRUP (Lanjutan %d)**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

`, messageCount)
			currentMessage.WriteString(header)
			currentMessageLength = len(header)
		}

		currentMessage.WriteString(line)
		currentMessageLength += len(line)
	}

	// Kirim pesan terakhir jika ada
	if currentMessageLength > len(header) {
		footer := "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n✅ *Selesai*"
		currentMessage.WriteString(footer)

		msg := tgbotapi.NewMessage(chatID, currentMessage.String())
		msg.ParseMode = "Markdown"
		telegramBot.Send(msg)
	}
}

// escapeMarkdown mengescape karakter markdown
func escapeMarkdown(text string) string {
	// Escape karakter markdown
	text = strings.ReplaceAll(text, "*", "\\*")
	text = strings.ReplaceAll(text, "_", "\\_")
	text = strings.ReplaceAll(text, "[", "\\[")
	text = strings.ReplaceAll(text, "]", "\\]")
	text = strings.ReplaceAll(text, "`", "\\`")
	return text
}

// isNumeric mengecek apakah string hanya berisi angka
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// showGroupMenu menampilkan menu grup dengan opsi
func showGroupMenu(telegramBot *tgbotapi.BotAPI, chatID int64, client *whatsmeow.Client) {
	menuMsg := fmt.Sprintf(`╔═══════════════════════════════╗
║   👥 **MANAJEMEN GRUP**       ║
╚═══════════════════════════════╝

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📊 **STATISTIK**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📅 **Update:** %s

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ ⚡ **FITUR TERSEDIA**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📋 **Lihat Daftar** - Tampilkan semua grup
🔍 **Cari Grup** - Filter berdasarkan nama
🔗 **Ambil Link** - Get link undangan grup
📥 **Export** - Download daftar ke file

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **Tips:**
• Grup otomatis terdeteksi saat ada pesan
• Gunakan search untuk menemukan grup cepat
• Ambil link untuk grup yang Anda kelola
• Export untuk backup daftar grup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, time.Now().Format("15:04 WIB"))

	msg := tgbotapi.NewMessage(chatID, menuMsg)
	msg.ParseMode = "Markdown"

	// Tambahkan inline keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat Daftar", "list_grup"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Grup", "search_grup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📢 Broadcast Pesan", "broadcast_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 Ambil Link", "get_link_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🖼️ Ganti Foto", "change_photo_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Atur Deskripsi", "change_description_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📢 Atur Pesan", "change_logging_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Atur Tambah Anggota", "change_member_add_menu"),
			tgbotapi.NewInlineKeyboardButtonData("✅ Atur Persetujuan", "change_join_approval_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏱️ Atur Pesan Sementara", "change_ephemeral_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔧 Atur Edit Grup", "change_edit_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Atur Semua Pengaturan", "change_all_settings_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Buat Grup Otomatis", "create_group_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 Join Grup Otomatis", "join_group_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 Auto Admin", "admin_menu"),
			tgbotapi.NewInlineKeyboardButtonData("👤 Auto Unadmin", "unadmin_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 Export Grup", "export_grup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Utama", "refresh"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)
}

// ShowGroupManagementMenuEdit menampilkan menu grup dengan EDIT message (no spam!)
func ShowGroupManagementMenuEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int, client *whatsmeow.Client) {
	menuMsg := fmt.Sprintf(`╔═══════════════════════════════╗
║   👥 **MANAJEMEN GRUP**       ║
╚═══════════════════════════════╝

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ 📊 **STATISTIK**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📅 **Update:** %s

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ ⚡ **FITUR TERSEDIA**
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

📋 **Lihat Daftar** - Tampilkan semua grup
🔍 **Cari Grup** - Filter berdasarkan nama
🔗 **Ambil Link** - Get link undangan grup
🖼️ **Ganti Foto** - Ubah foto profil grup
📝 **Atur Deskripsi** - Ubah deskripsi grup
📢 **Atur Pesan** - Aktifkan/nonaktifkan pesan
👥 **Atur Tambah Anggota** - Atur izin tambah anggota
✅ **Atur Persetujuan** - Atur approval anggota baru
⏱️ **Atur Pesan Sementara** - Atur durasi pesan sementara
🔧 **Atur Edit Grup** - Atur izin edit pengaturan grup
⚙️ **Atur Semua Pengaturan** - Atur semua pengaturan sekaligus
📥 **Export** - Download daftar ke file

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 **Tips:**
• Grup otomatis terdeteksi saat ada pesan
• Gunakan search untuk menemukan grup cepat
• Ambil link untuk grup yang Anda kelola
• Export untuk backup daftar grup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, time.Now().Format("15:04 WIB"))

	// Tambahkan inline keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Lihat Daftar", "list_grup"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Cari Grup", "search_grup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📢 Broadcast Pesan", "broadcast_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 Ambil Link", "get_link_menu"),
			tgbotapi.NewInlineKeyboardButtonData("🖼️ Ganti Foto", "change_photo_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Atur Deskripsi", "change_description_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📢 Atur Pesan", "change_logging_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Atur Tambah Anggota", "change_member_add_menu"),
			tgbotapi.NewInlineKeyboardButtonData("✅ Atur Persetujuan", "change_join_approval_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏱️ Atur Pesan Sementara", "change_ephemeral_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔧 Atur Edit Grup", "change_edit_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Atur Semua Pengaturan", "change_all_settings_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Buat Grup Otomatis", "create_group_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 Join Grup Otomatis", "join_group_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚪 Keluar Grup Otomatis", "leave_group_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Add Member Grup", "add_member_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 Auto Admin", "admin_menu"),
			tgbotapi.NewInlineKeyboardButtonData("👤 Auto Unadmin", "unadmin_menu"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 Export Grup", "export_grup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Menu Utama", "refresh"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, menuMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)
}
