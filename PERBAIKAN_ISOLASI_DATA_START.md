# Perbaikan Isolasi Data pada Command /start

## Masalah yang Ditemukan

Setelah user ke-2 mengklik `/start`, program menampilkan semua database yang dimiliki user ID 1. Ini terjadi karena:

1. **Handler `/start` menggunakan `chatID` langsung**: Di `handlers/telegram.go` line 66, kode menggunakan `int64(chatID)` langsung sebagai Telegram ID untuk `SetDBConfig`, padahal seharusnya menggunakan Telegram ID yang di-parse dari `currentAccount.BotDataDBPath`.

2. **Fungsi activity log menggunakan hardcoded database**: Fungsi-fungsi di `utils/activity_log.go` (`LogActivityWithMetadata`, `GetActivityLogs`, `GetActivityStats`) menggunakan hardcoded `"bot_data.db"` bukan menggunakan `GetBotDBPool()` yang dinamis berdasarkan `dbConfig`.

## Solusi yang Diterapkan

### 1. Perbaikan Handler `/start` di `handlers/telegram.go`

**Sebelum:**
```go
if currentAccount != nil {
    // Update dbConfig untuk memastikan GetBotDataDBPath() mengembalikan path yang benar
    utils.SetDBConfig(int64(chatID), currentAccount.PhoneNumber)
}
```

**Sesudah:**
```go
if currentAccount != nil {
    // CRITICAL FIX: Parse Telegram ID dari BotDataDBPath untuk memastikan dbConfig benar
    // Jangan gunakan chatID langsung karena bisa berbeda dengan Telegram ID yang sebenarnya
    telegramID := int64(chatID) // Default: gunakan chatID
    if currentAccount.BotDataDBPath != "" {
        re := regexp.MustCompile(`bot_data\((\d+)\)>`)
        matches := re.FindStringSubmatch(currentAccount.BotDataDBPath)
        if len(matches) >= 2 {
            if parsedID, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
                telegramID = parsedID
            }
        }
    }
    // Update dbConfig dengan Telegram ID yang benar untuk memastikan isolasi data per user
    utils.SetDBConfig(telegramID, currentAccount.PhoneNumber)
    // Reset database pool untuk memastikan menggunakan database yang benar
    utils.CloseDBPools()
}
```

**Penjelasan:**
- Parse Telegram ID dari `BotDataDBPath` yang berformat `bot_data(telegramID)>(phoneNumber).db`
- Gunakan Telegram ID yang di-parse (bukan `chatID` langsung) untuk `SetDBConfig`
- Reset database pool setelah update `dbConfig` untuk memastikan pool menggunakan database yang benar

### 2. Perbaikan Fungsi Activity Log di `utils/activity_log.go`

**Sebelum:**
```go
func LogActivityWithMetadata(...) error {
    db, err := sql.Open("sqlite3", "bot_data.db?_journal_mode=WAL")
    if err != nil {
        return err
    }
    defer db.Close()
    // ...
}
```

**Sesudah:**
```go
func LogActivityWithMetadata(...) error {
    // CRITICAL FIX: Gunakan GetBotDBPool() untuk memastikan menggunakan database yang benar per user
    db, err := GetBotDBPool()
    if err != nil {
        return err
    }
    // Jangan close pool, biarkan pool management handle
    // ...
}
```

**Fungsi yang diperbaiki:**
1. `LogActivityWithMetadata` - Line 44
2. `GetActivityLogs` - Line 77
3. `GetActivityStats` - Line 123

**Penjelasan:**
- Semua fungsi sekarang menggunakan `GetBotDBPool()` yang membaca dari `dbConfig` yang sudah di-set dengan benar
- Tidak perlu `defer db.Close()` karena pool management handle sendiri
- Setiap user akan membaca dari database mereka sendiri berdasarkan Telegram ID

## Cara Kerja Setelah Perbaikan

1. **Saat user mengirim `/start`**:
   - Sistem memanggil `EnsureUserAccountActive()` untuk switch ke akun user
   - Parse Telegram ID dari `BotDataDBPath` akun aktif
   - Update `dbConfig` dengan Telegram ID yang benar
   - Reset database pool untuk menggunakan database yang benar
   - Tampilkan menu dengan data dari database user yang benar

2. **Saat membaca data dari database**:
   - `GetAllGroupsFromDB()` menggunakan `GetBotDBPool()` → membaca dari database user yang benar
   - `GetActivityStats()` menggunakan `GetBotDBPool()` → membaca dari database user yang benar
   - `GetActivityLogs()` menggunakan `GetBotDBPool()` → membaca dari database user yang benar
   - Semua fungsi membaca dari database yang sesuai dengan Telegram ID user

## Testing Checklist

Untuk memverifikasi perbaikan:

1. ✅ User 1 (Telegram ID: X) melakukan pairing dan menggunakan bot
2. ✅ User 2 (Telegram ID: Y) ditambahkan ke allowed IDs dan melakukan pairing
3. ✅ User 2 mengirim `/start` → Harus menampilkan data dari database User 2, BUKAN User 1
4. ✅ User 1 mengirim `/start` → Harus menampilkan data dari database User 1, BUKAN User 2
5. ✅ Activity stats di menu utama menampilkan data yang benar untuk masing-masing user
6. ✅ Group list menampilkan grup yang benar untuk masing-masing user

## Files Modified

1. `handlers/telegram.go` - Perbaikan handler `/start` untuk parse Telegram ID dari `BotDataDBPath`
2. `utils/activity_log.go` - Perbaikan semua fungsi activity log untuk menggunakan `GetBotDBPool()`

## Catatan Penting

- **Database Isolation**: Setiap user memiliki database terpisah dengan format `bot_data(telegramID)>(phoneNumber).db`
- **Pool Management**: Database pool di-rebuild setiap kali switch account untuk memastikan menggunakan database yang benar
- **Backward Compatibility**: Perbaikan ini tidak mempengaruhi fungsionalitas yang sudah ada, hanya memperbaiki isolasi data

## Referensi

- Masalah serupa telah dibahas di komunitas pengembang tentang pentingnya manajemen sesi untuk menangani banyak pengguna secara bersamaan
- Best practice: Setiap user harus memiliki sesi independen untuk memastikan operasi yang efisien dan aman

