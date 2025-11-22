package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ListSelectState manages pagination state for list selection
type ListSelectState struct {
	CurrentPage    int
	TotalPages     int
	GroupsPerPage  int
	AllGroups      []GroupLinkInfo
	SelectedGroups map[int]bool
}

var listSelectStates = make(map[int64]*ListSelectState)

// ShowGroupListForLink menampilkan daftar grup dengan pagination
func ShowGroupListForLink(telegramBot *tgbotapi.BotAPI, chatID int64, page int) {
	// CRITICAL FIX: Pastikan menggunakan database user yang benar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)
	if userAccount != nil {
		EnsureDBConfigForUser(chatID, userAccount)
	}

	// Get all groups
	groupsMap, err := utils.GetAllGroupsFromDB()
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	if len(groupsMap) == 0 {
		noDataMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang terdeteksi.\n\nGrup akan otomatis terdeteksi saat bot menerima pesan dari grup.")
		telegramBot.Send(noDataMsg)
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
	msg := fmt.Sprintf(`📋 **DAFTAR GRUP**

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

	msgObj := tgbotapi.NewMessage(chatID, msg)
	msgObj.ParseMode = "Markdown"

	// Build keyboard
	var keyboard tgbotapi.InlineKeyboardMarkup

	// Navigation buttons
	navRow := []tgbotapi.InlineKeyboardButton{}
	if page > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("link_page_%d", page-1)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📄 %d/%d", page, totalPages), "noop"))
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️ Next", fmt.Sprintf("link_page_%d", page+1)))
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)

	// Quick action buttons
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Pilih Semua", "select_all_link"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "get_link_menu"),
		),
	)

	msgObj.ReplyMarkup = keyboard
	telegramBot.Send(msgObj)

	// Store state
	listSelectStates[chatID] = &ListSelectState{
		CurrentPage:    page,
		TotalPages:     totalPages,
		GroupsPerPage:  groupsPerPage,
		AllGroups:      groups,
		SelectedGroups: make(map[int]bool),
	}
}

// ShowGroupListForLinkEdit menampilkan daftar grup dengan pagination (EDIT, NO SPAM!)
func ShowGroupListForLinkEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int, page int) {
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
	msg := fmt.Sprintf(`📋 **DAFTAR GRUP**

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
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("link_page_%d", page-1)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📄 %d/%d", page, totalPages), "noop"))
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️ Next", fmt.Sprintf("link_page_%d", page+1)))
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)

	// Quick action buttons
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Pilih Semua", "select_all_link"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Kembali", "get_link_menu"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, msg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	telegramBot.Send(editMsg)

	// Store state
	listSelectStates[chatID] = &ListSelectState{
		CurrentPage:    page,
		TotalPages:     totalPages,
		GroupsPerPage:  groupsPerPage,
		AllGroups:      groups,
		SelectedGroups: make(map[int]bool),
	}
}

// HandleGroupSelection handles group selection input
func HandleGroupSelection(selection string, chatID int64, telegramBot *tgbotapi.BotAPI) []GroupLinkInfo {
	state := listSelectStates[chatID]
	if state == nil {
		return nil
	}

	selection = strings.TrimSpace(strings.ToLower(selection))
	selectedGroups := []GroupLinkInfo{}

	if selection == "all" || selection == "semua" {
		// Select all groups
		selectedGroups = state.AllGroups
	} else if strings.Contains(selection, "-") {
		// Range selection (e.g., "1-10")
		parts := strings.Split(selection, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

			if err1 == nil && err2 == nil && start > 0 && end <= len(state.AllGroups) && start <= end {
				for i := start - 1; i < end; i++ {
					selectedGroups = append(selectedGroups, state.AllGroups[i])
				}
			}
		}
	} else if strings.Contains(selection, ",") {
		// Multiple selection (e.g., "1,3,5")
		parts := strings.Split(selection, ",")
		for _, part := range parts {
			num, err := strconv.Atoi(strings.TrimSpace(part))
			if err == nil && num > 0 && num <= len(state.AllGroups) {
				selectedGroups = append(selectedGroups, state.AllGroups[num-1])
			}
		}
	} else {
		// Single selection
		num, err := strconv.Atoi(selection)
		if err == nil && num > 0 && num <= len(state.AllGroups) {
			selectedGroups = append(selectedGroups, state.AllGroups[num-1])
		}
	}

	return selectedGroups
}

// ProcessSelectedGroupsForLink processes selected groups untuk ambil link
func ProcessSelectedGroupsForLink(selection string, chatID int64, telegramBot *tgbotapi.BotAPI) {
	selectedGroups := HandleGroupSelection(selection, chatID, telegramBot)

	if len(selectedGroups) == 0 {
		errorMsg := tgbotapi.NewMessage(chatID, "❌ Pilihan tidak valid!\n\nContoh: 1, 1-5, 1,3,5, atau 'all'")
		telegramBot.Send(errorMsg)
		return
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

Masukkan berapa detik delay antar permintaan.

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
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_get_link"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)

	// Update state untuk waiting delay
	linkGrupStates[chatID] = &LinkGrupState{
		WaitingForGroupName: false,
		WaitingForDelay:     true,
		SelectedGroups:      selectedGroups,
		Keyword:             fmt.Sprintf("Selected %d groups", len(selectedGroups)),
	}

	// Clear list select state
	delete(listSelectStates, chatID)
}

// IsWaitingForGroupSelection checks if user is in selection mode
func IsWaitingForGroupSelection(chatID int64) bool {
	return listSelectStates[chatID] != nil
}

// GetAllLinksDirectly processes all groups directly
func GetAllLinksDirectly(chatID int64, telegramBot *tgbotapi.BotAPI) {
	// CRITICAL FIX: Pastikan menggunakan database user yang benar
	am := GetAccountManager()
	userAccount := am.GetAccountByTelegramID(chatID)
	if userAccount != nil {
		EnsureDBConfigForUser(chatID, userAccount)
	}

	// Get all groups
	groupsMap, err := utils.GetAllGroupsFromDB()
	if err != nil {
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		telegramBot.Send(errorMsg)
		return
	}

	if len(groupsMap) == 0 {
		noDataMsg := tgbotapi.NewMessage(chatID, "❌ Tidak ada grup yang terdeteksi.")
		telegramBot.Send(noDataMsg)
		return
	}

	// Convert to slice
	selectedGroups := []GroupLinkInfo{}
	for jid, name := range groupsMap {
		selectedGroups = append(selectedGroups, GroupLinkInfo{
			JID:  jid,
			Name: name,
		})
	}

	// Ask for delay
	confirmMsg := fmt.Sprintf(`⚡ **AMBIL SEMUA LINK**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **Total grup:** %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️ **TENTUKAN DELAY**

Berapa detik delay per grup?

**Rekomendasi:** 3-5 detik untuk %d grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Ketik angka delay (contoh: 3)`, len(selectedGroups), len(selectedGroups))

	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "cancel_get_link"),
		),
	)
	msg.ReplyMarkup = keyboard

	telegramBot.Send(msg)

	// Set state
	linkGrupStates[chatID] = &LinkGrupState{
		WaitingForGroupName: false,
		WaitingForDelay:     true,
		SelectedGroups:      selectedGroups,
		Keyword:             "all",
	}
}
