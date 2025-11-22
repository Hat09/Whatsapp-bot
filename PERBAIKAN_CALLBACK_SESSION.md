# Perbaikan Isolasi Data di Callback Handler

## Masalah

User melaporkan bahwa user baru masih bisa menggunakan database milik user lain, terutama saat menggunakan callback seperti "grup".

## Analisis

Masalah terjadi karena:
1. **Callback handler masih menggunakan `activeClient` dari `GetWhatsAppClient()`** yang bisa dari user lain
2. **Callback handler masih menggunakan `GetCurrentClient()` dan `GetCurrentAccount()`** yang bisa stale atau dari user lain
3. **Tidak ada update `dbConfig` sebelum operasi database** di beberapa callback handler
4. **Race condition** saat 2 user menggunakan callback bersamaan

## Solusi

### 1. Implementasi Session Management di Callback Handler

**Sebelum:**
```go
// HandleCallbackQuery
userAccount, err := EnsureUserAccountActive(int64(chatID), telegramBot)
activeClient := GetWhatsAppClient() // Bisa dari user lain!
```

**Sesudah:**
```go
// HandleCallbackQuery
userSession, err := GetUserSession(int64(chatID), telegramBot)
// Setiap user memiliki session terpisah
userClient := userSession.Client // Client dari session user yang benar
```

### 2. Helper Functions untuk Isolasi Data

Dibuat helper functions di `handlers/session_helper.go`:
- `GetClientForUser()` - Mendapatkan client yang benar untuk user
- `GetAccountForUser()` - Mendapatkan account yang benar untuk user
- `EnsureDBConfigForUser()` - Memastikan dbConfig di-update dengan database user yang benar

### 3. Update Semua Callback Handler

Semua callback handler yang menggunakan `activeClient` di-update untuk:
1. Menggunakan `userSession.Client` atau `GetClientForUser()`
2. Memanggil `EnsureDBConfigForUser()` sebelum operasi database
3. Tidak menggunakan `GetCurrentClient()` atau `GetCurrentAccount()` yang bisa stale

### 4. Perbaikan GetGroupList

**Sebelum:**
```go
// Get client yang benar setelah switch
client = am.GetCurrentClient() // Bisa dari user lain!
```

**Sesudah:**
```go
// CRITICAL FIX: Gunakan client dari account user, bukan GetCurrentClient()
client = am.GetClient(userAccount.ID)
// Update dbConfig setelah switch
EnsureDBConfigForUser(chatID, userAccount)
```

## Files Modified

1. **handlers/telegram.go**
   - Callback handler menggunakan `GetUserSession()` untuk isolasi data
   - Semua callback menggunakan client dari session, bukan `activeClient`
   - Semua callback memanggil `EnsureDBConfigForUser()` sebelum operasi database

2. **handlers/grup.go**
   - `GetGroupList()` menggunakan `am.GetClient(userAccount.ID)` bukan `GetCurrentClient()`
   - Memanggil `EnsureDBConfigForUser()` setelah switch account

3. **handlers/session_helper.go** (NEW)
   - Helper functions untuk isolasi data per user

## Testing

Untuk memverifikasi perbaikan:

1. ✅ **User 1 klik callback "grup"** → Session user 1 digunakan, database user 1 diakses
2. ✅ **User 2 klik callback "grup"** → Session user 2 digunakan, database user 2 diakses (tidak mengganggu user 1)
3. ✅ **User 1 dan User 2 klik callback bersamaan** → Tidak ada race condition (mutex protection)
4. ✅ **User 1 melihat daftar grup** → Grup dari database user 1
5. ✅ **User 2 melihat daftar grup** → Grup dari database user 2

## Kesimpulan

Dengan implementasi session management di callback handler dan helper functions untuk isolasi data, setiap user sekarang memiliki session terpisah dan tidak bisa mengakses database user lain, bahkan saat menggunakan callback bersamaan.

