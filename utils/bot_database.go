package utils

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database connection pools
var (
	botDBPool        *sql.DB
	whatsappDBPool   *sql.DB
	dbPoolOnce       sync.Once
	dbPoolMutex      sync.Mutex
	currentBotDBPath string // Menyimpan path database yang sedang digunakan oleh pool
)

// GetBotDBPool mendapatkan connection pool untuk bot_data.db (menggunakan nama dinamis)
func GetBotDBPool() (*sql.DB, error) {
	dbPoolMutex.Lock()
	defer dbPoolMutex.Unlock()

	dbName := GetBotDataDBPath()

	// Cek apakah pool sudah ada dan menggunakan database yang sama
	if botDBPool != nil && currentBotDBPath == dbName {
		// Pool sudah ada dan menggunakan database yang benar
		return botDBPool, nil
	}

	// Pool tidak ada atau menggunakan database yang berbeda
	// Log perubahan path database untuk debug
	if currentBotDBPath != "" && currentBotDBPath != dbName {
		GetLogger().Info("GetBotDBPool: Database path changed from '%s' to '%s', rebuilding pool", currentBotDBPath, dbName)
		// IMPORTANT: Setup database (create tables) untuk database baru
		// Pastikan tabel groups dan messages ada sebelum menggunakan pool
		if err := SetupBotDB(); err != nil {
			GetLogger().Warn("GetBotDBPool: Failed to setup bot database for new path '%s': %v", dbName, err)
			// Continue anyway, mungkin sudah ada
		} else {
			GetLogger().Info("GetBotDBPool: ✅ Database tables created/verified for path: %s", dbName)
		}
	} else if botDBPool == nil {
		GetLogger().Info("GetBotDBPool: Creating new pool with path: %s", dbName)
		// IMPORTANT: Setup database (create tables) untuk pool pertama kali
		// Pastikan tabel groups dan messages ada sebelum menggunakan pool
		if err := SetupBotDB(); err != nil {
			GetLogger().Warn("GetBotDBPool: Failed to setup bot database for path '%s': %v", dbName, err)
			// Continue anyway, mungkin sudah ada
		} else {
			GetLogger().Info("GetBotDBPool: ✅ Database tables created/verified for path: %s", dbName)
		}
	}

	// FIXED: Tutup pool lama dengan lebih aman untuk mencegah race condition
	if botDBPool != nil {
		GetLogger().Info("GetBotDBPool: Closing old pool with path: %s", currentBotDBPath)
		// FIXED: Set pool ke nil dulu sebelum close untuk mencegah race condition
		oldPool := botDBPool
		botDBPool = nil
		// Close pool lama di luar lock untuk mencegah deadlock
		// Pool akan di-close setelah semua goroutine selesai menggunakan
		go func() {
			// Small delay untuk memastikan semua goroutine selesai menggunakan pool
			time.Sleep(100 * time.Millisecond)
			if err := oldPool.Close(); err != nil {
				GetLogger().Warn("GetBotDBPool: Error closing old pool: %v", err)
			}
		}()
	}

	// FIXED: Reset sync.Once dengan cara yang lebih aman
	// Jangan reset sync.Once karena bisa menyebabkan double initialization
	// Biarkan sync.Once tetap, tapi buat pool baru langsung

	// Buat pool baru dengan database path yang benar
	db, err := sql.Open("sqlite3", dbName+"?_journal_mode=WAL&_cache=shared")
	if err != nil {
		return nil, fmt.Errorf("gagal membuka database: %w", err)
	}

	// FIXED: SetMaxOpenConns dan SetMaxIdleConns tidak return error, tapi tetap validasi
	// dengan ping untuk memastikan connection pool berfungsi
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// FIXED: Test connection dengan ping untuk memastikan pool berfungsi
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("gagal ping database setelah setup pool: %w", err)
	}

	botDBPool = db
	currentBotDBPath = dbName // Simpan path yang digunakan

	GetLogger().Info("GetBotDBPool: Pool rebuilt successfully with path: %s", currentBotDBPath)

	return botDBPool, nil
}

// GetWhatsAppDBPool mendapatkan connection pool untuk whatsapp.db (menggunakan nama dinamis)
func GetWhatsAppDBPool() (*sql.DB, error) {
	if whatsappDBPool == nil {
		dbPoolMutex.Lock()
		defer dbPoolMutex.Unlock()

		if whatsappDBPool == nil {
			dbName := GetWhatsAppDBPath()
			db, err := sql.Open("sqlite3", dbName+"?_foreign_keys=on&_journal_mode=WAL&_cache=shared")
			if err != nil {
				return nil, err
			}
			db.SetMaxOpenConns(10)
			db.SetMaxIdleConns(5)
			whatsappDBPool = db
		}
	}
	return whatsappDBPool, nil
}

// CloseDBPools menutup semua connection pool database
func CloseDBPools() {
	dbPoolMutex.Lock()
	defer dbPoolMutex.Unlock()

	if whatsappDBPool != nil {
		whatsappDBPool.Close()
		whatsappDBPool = nil
	}

	if botDBPool != nil {
		botDBPool.Close()
		botDBPool = nil
	}

	// Reset sync.Once untuk memungkinkan re-initialization
	dbPoolOnce = sync.Once{}

	// Reset current database path
	currentBotDBPath = ""
}

// ClearAppState membersihkan app_state untuk memperbaiki LTHash error
func ClearAppState() error {
	dbName := GetWhatsAppDBPath()
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	// Hapus semua data dari tabel-tabel app_state yang bisa menyebabkan LTHash error
	tables := []string{
		"whatsmeow_app_state_mutation_macs",
		"whatsmeow_app_state_sync_keys",
		"whatsmeow_app_state_version",
	}

	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table)
		if err != nil {
			// Jika tabel tidak ada, lanjutkan ke tabel berikutnya
			if strings.Contains(err.Error(), "no such table") {
				continue
			}
			// Log error tapi jangan gagalkan (bisa jadi tabel sudah kosong)
			fmt.Printf("⚠️ Warning saat clear %s: %v\n", table, err)
		}
	}

	return nil
}

// SetupBotDB menyiapkan database untuk bot (menggunakan nama dinamis)
func SetupBotDB() error {
	dbName := GetBotDataDBPath()
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create messages table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sender TEXT NOT NULL,
			message TEXT,
			timestamp TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	// Create groups table untuk menyimpan daftar grup
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_jid TEXT UNIQUE NOT NULL,
			group_name TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create whatsapp_accounts table untuk multi-account
	// Juga buat di database master (bot_data.db) untuk memastikan konsistensi
	masterDBPath := "bot_data.db"
	masterDB, err := sql.Open("sqlite3", masterDBPath+"?_journal_mode=WAL")
	if err == nil {
		_, _ = masterDB.Exec(`
			CREATE TABLE IF NOT EXISTS whatsapp_accounts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				phone_number TEXT UNIQUE NOT NULL,
				db_path TEXT NOT NULL,
				bot_data_db_path TEXT NOT NULL,
				status TEXT DEFAULT 'active',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)

		// Create activity_logs table untuk audit trail
		_, _ = masterDB.Exec(`
			CREATE TABLE IF NOT EXISTS activity_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				action TEXT NOT NULL,
				description TEXT,
				telegram_chat_id INTEGER,
				success INTEGER DEFAULT 1,
				error_message TEXT,
				metadata TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)

		// Create statistics table untuk monitoring
		_, _ = masterDB.Exec(`
			CREATE TABLE IF NOT EXISTS statistics (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				stat_key TEXT UNIQUE NOT NULL,
				stat_value TEXT NOT NULL,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)

		masterDB.Close()
	}

	return err
}

// SaveGroupToDB menyimpan grup ke database (menggunakan connection pool)
func SaveGroupToDB(groupJID, groupName string) error {
	db, err := GetBotDBPool()
	if err != nil {
		return err
	}

	// Insert or update group
	_, err = db.Exec(`
		INSERT OR REPLACE INTO groups (group_jid, group_name, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, groupJID, groupName)

	return err
}

// BatchSaveGroupsToDB menyimpan multiple grup sekaligus (lebih efisien)
func BatchSaveGroupsToDB(groups map[string]string) error {
	db, err := GetBotDBPool()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO groups (group_jid, group_name, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for jid, name := range groups {
		if _, err := stmt.Exec(jid, name); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAllGroupsFromDB mengambil semua grup dari database (menggunakan connection pool)
func GetAllGroupsFromDB() (map[string]string, error) {
	groups := make(map[string]string)

	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}

	// Get all groups without ordering (will sort naturally in caller)
	rows, err := db.Query("SELECT group_jid, group_name FROM groups")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var jid, name string
		if err := rows.Scan(&jid, &name); err != nil {
			continue
		}
		groups[jid] = name
	}

	return groups, nil
}

// OpenWhatsAppDB membuka koneksi ke database WhatsApp (menggunakan connection pool)
func OpenWhatsAppDB() (*sql.DB, error) {
	return GetWhatsAppDBPool()
}

// SearchGroups mencari grup berdasarkan nama (with natural sorting in caller)
func SearchGroups(keyword string) (map[string]string, error) {
	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}

	groups := make(map[string]string)
	// Remove ORDER BY - will sort naturally in the caller
	query := "SELECT group_jid, group_name FROM groups WHERE group_name LIKE ?"
	rows, err := db.Query(query, "%"+keyword+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var jid, name string
		if err := rows.Scan(&jid, &name); err != nil {
			continue
		}
		groups[jid] = name
	}

	return groups, nil
}

// SearchGroupsFlexible mencari grup dengan matching lebih flexible (per kata, natural sorted in caller)
func SearchGroupsFlexible(keyword string) (map[string]string, error) {
	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}

	groups := make(map[string]string)

	// Split keyword menjadi words
	words := strings.Fields(strings.ToLower(keyword))
	if len(words) == 0 {
		return groups, nil
	}

	// Get all groups (no ORDER BY - will sort naturally in caller)
	query := "SELECT group_jid, group_name FROM groups"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var jid, name string
		if err := rows.Scan(&jid, &name); err != nil {
			continue
		}

		// Check if any word matches
		nameLower := strings.ToLower(name)
		matched := false
		for _, word := range words {
			if strings.Contains(nameLower, word) {
				matched = true
				break
			}
		}

		if matched {
			groups[jid] = name
		}
	}

	return groups, nil
}

// SearchGroupsExact mencari grup dengan exact match atau very close match (natural sorted in caller)
func SearchGroupsExact(keyword string) (map[string]string, error) {
	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}

	groups := make(map[string]string)
	keywordLower := strings.ToLower(strings.TrimSpace(keyword))

	// Try exact match first (no ORDER BY)
	query := "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) = ?"
	rows, err := db.Query(query, keywordLower)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var jid, name string
		if err := rows.Scan(&jid, &name); err != nil {
			continue
		}
		groups[jid] = name
	}

	// If no exact match, try substring match with high similarity
	if len(groups) == 0 {
		query := "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) LIKE ?"
		rows, err := db.Query(query, "%"+keywordLower+"%")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var jid, name string
			if err := rows.Scan(&jid, &name); err != nil {
				continue
			}
			nameLower := strings.ToLower(name)

			// Only add if similarity is high (keyword is significant part of name)
			keywordWords := strings.Fields(keywordLower)
			nameWords := strings.Fields(nameLower)

			if len(keywordWords) >= 3 { // Require at least 3 words for specific search
				matchCount := 0
				for _, kw := range keywordWords {
					if len(kw) > 2 { // Ignore very short words
						for _, nw := range nameWords {
							if strings.Contains(nw, kw) || strings.Contains(kw, nw) {
								matchCount++
								break
							}
						}
					}
				}

				// At least 80% of significant words must match
				if float64(matchCount) >= float64(len(keywordWords))*0.8 {
					groups[jid] = name
				}
			}
		}
	}

	return groups, nil
}

// SearchGroupsExactMultiple mencari multiple grup dengan exact match untuk setiap line
func SearchGroupsExactMultiple(keywords []string) (map[string]string, error) {
	db, err := GetBotDBPool()
	if err != nil {
		return nil, err
	}

	groups := make(map[string]string)

	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}

		keywordLower := strings.ToLower(keyword)

		// Try exact match first
		query := "SELECT group_jid, group_name FROM groups WHERE LOWER(group_name) = ?"
		row := db.QueryRow(query, keywordLower)

		var jid, name string
		err := row.Scan(&jid, &name)
		if err == nil {
			groups[jid] = name
			continue
		}

		// If not exact, try very close match (same words, different order or minor differences)
		query = "SELECT group_jid, group_name FROM groups"
		rows, err := db.Query(query)
		if err != nil {
			continue
		}

		keywordWords := strings.Fields(keywordLower)
		bestMatch := ""
		bestMatchJID := ""
		bestMatchScore := 0.0

		for rows.Next() {
			var jid, name string
			if err := rows.Scan(&jid, &name); err != nil {
				continue
			}

			nameLower := strings.ToLower(name)
			nameWords := strings.Fields(nameLower)

			// Calculate match score
			matchCount := 0
			for _, kw := range keywordWords {
				for _, nw := range nameWords {
					if kw == nw {
						matchCount++
						break
					}
				}
			}

			score := float64(matchCount) / float64(len(keywordWords))

			// Require at least 90% match for multi-line input (very specific)
			if score >= 0.9 && score > bestMatchScore {
				bestMatchScore = score
				bestMatch = name
				bestMatchJID = jid
			}
		}
		rows.Close()

		if bestMatchScore >= 0.9 {
			groups[bestMatchJID] = bestMatch
		}
	}

	return groups, nil
}

// GetGroupsPaginated mengambil grup dengan pagination (natural sorted)
func GetGroupsPaginated(page, perPage int) (map[string]string, int, error) {
	db, err := GetBotDBPool()
	if err != nil {
		return nil, 0, err
	}

	// Get total count
	var totalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get ALL groups first (no pagination in SQL)
	allGroups := make(map[string]string)
	query := "SELECT group_jid, group_name FROM groups"
	rows, err := db.Query(query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var jid, name string
		if err := rows.Scan(&jid, &name); err != nil {
			continue
		}
		allGroups[jid] = name
	}

	// Sort naturally
	sortedGroups := SortGroupsNaturally(allGroups)

	// Apply pagination AFTER sorting
	groups := make(map[string]string)
	start := (page - 1) * perPage
	end := start + perPage

	if start >= len(sortedGroups) {
		return groups, totalCount, nil
	}

	if end > len(sortedGroups) {
		end = len(sortedGroups)
	}

	for i := start; i < end; i++ {
		groups[sortedGroups[i].JID] = sortedGroups[i].Name
	}

	return groups, totalCount, nil
}
