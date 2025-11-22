# Perbaikan Pairing untuk User Baru

## Masalah

Dari log dan gambar:
1. User baru (7793345217) mengklik "start_pairing" dan memasukkan nomor WhatsApp
2. Terjadi error: "websocket disconnected before info query (retry) returned response"
3. Error terjadi karena websocket terputus saat proses pairing

**Root Cause:**
- `PairDeviceViaTelegram` menggunakan `GetWhatsAppClient()` yang mengembalikan client dari user lain (current active client)
- Client tersebut sedang digunakan oleh user lain, sehingga terjadi konflik websocket
- Multiple user tidak bisa melakukan pairing bersamaan karena menggunakan client yang sama

## Solusi yang Diterapkan

### 1. Selalu Buat Client Baru untuk Pairing

**File: `handlers/pairing.go`**

**Sebelum:**
```go
client := GetWhatsAppClient() // Menggunakan client dari user lain

if client == nil {
    // Hanya buat client baru jika client nil
    // ...
}
```

**Sesudah:**
```go
// CRITICAL FIX: Selalu buat client baru untuk pairing
// Ini mencegah konflik websocket saat multiple user melakukan pairing bersamaan
utils.GetLogger().Info("Creating new client for pairing: TelegramID=%d, Phone=%s", chatID, phone)

// Generate database path untuk user ini
dbPath := utils.GenerateDBName(chatID, cleanPhone, "whatsmeow")

// Setup WhatsApp database store untuk pairing baru
// Create WhatsApp client baru untuk pairing
client := whatsmeow.NewClient(deviceStore, clientLog)

// JANGAN set sebagai global client karena ini client khusus untuk pairing user ini
```

### 2. Gunakan Database Path dengan Folder User

**File: `handlers/pairing.go`**

**Sebelum:**
```go
dbPath := "whatsapp.db" // Default untuk pairing baru
```

**Sesudah:**
```go
// Generate database path untuk user ini
// Gunakan path dengan folder user: DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db
dbPath := utils.GenerateDBName(chatID, cleanPhone, "whatsmeow")
```

### 3. Hapus Rename Database

**File: `handlers/pairing.go`**

**Sebelum:**
```go
// Rename database files dari nama default ke nama dinamis
err := utils.RenameDatabaseFiles("whatsapp.db", newWhatsAppDB, "bot_data.db", newBotDataDB)
```

**Sesudah:**
```go
// CRITICAL FIX: Database sudah dibuat dengan path yang benar dari awal
// Tidak perlu rename karena database sudah di folder user dengan nama yang benar
utils.GetLogger().Info("Database sudah dibuat dengan path yang benar: WhatsAppDB=%s, BotDataDB=%s", newWhatsAppDB, newBotDataDB)
```

## Cara Kerja

### Saat User Baru Melakukan Pairing:
1. `PairDeviceViaTelegram` dipanggil dengan `chatID` (Telegram ID user)
2. **BARU:** Selalu buat client baru untuk pairing (tidak menggunakan client yang sudah ada)
3. **BARU:** Database dibuat dengan path yang benar di folder user: `DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db`
4. Client baru connect ke WhatsApp
5. Generate pairing code menggunakan client baru
6. Setelah pairing berhasil, client disimpan ke AccountManager
7. Database sudah di folder user dengan nama yang benar, tidak perlu rename

### Keuntungan:
1. **Isolasi**: Setiap user memiliki client sendiri untuk pairing
2. **Tidak Ada Konflik**: Multiple user bisa melakukan pairing bersamaan
3. **Path Benar**: Database langsung dibuat di folder user yang benar
4. **Tidak Ada Rename**: Database tidak perlu di-rename karena sudah dibuat dengan path yang benar

## Hasil

Dengan perbaikan ini:
- User baru bisa melakukan pairing tanpa konflik websocket
- Multiple user bisa melakukan pairing bersamaan
- Database langsung dibuat di folder user yang benar
- Tidak ada error "websocket disconnected"

## Files yang Diubah

1. **handlers/pairing.go**
   - `PairDeviceViaTelegram()`: Selalu buat client baru untuk pairing
   - Gunakan database path dengan folder user dari awal
   - Hapus rename database karena sudah dibuat dengan path yang benar

## Kesimpulan

Dengan perbaikan ini, user baru bisa melakukan pairing tanpa error websocket. Setiap user memiliki client sendiri untuk pairing, sehingga tidak ada konflik saat multiple user melakukan pairing bersamaan.

