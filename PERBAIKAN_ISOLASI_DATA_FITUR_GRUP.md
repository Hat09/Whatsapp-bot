# Perbaikan Isolasi Data untuk Fitur Grup

## Masalah

User melaporkan bahwa:
1. User 1 dan User 2 membuka fitur "daftar grup" dan melihat semua nomor yang terdaftar termasuk dari kedua user tersebut
2. User khawatir fitur lain juga memiliki masalah isolasi data yang sama

**Root Cause:**
- Fungsi-fungsi yang mengakses database grup (`GetAllGroupsFromDB()`, `SearchGroups()`, `GetGroupsPaginated()`) tidak memastikan `dbConfig` sudah di-update dengan database user yang benar sebelum mengambil data
- Callback handler yang menampilkan grup masih menggunakan `activeClient` yang bisa dari user lain
- Fungsi-fungsi `ShowGroupListFor*Edit` tidak memastikan menggunakan database user yang benar

## Solusi yang Diterapkan

### 1. Perbaikan Fungsi `ShowGroupListFor*Edit`

Semua fungsi yang menampilkan daftar grup sekarang memastikan menggunakan database user yang benar:

**Files yang diperbaiki:**
- `handlers/grup_list_select.go`: `ShowGroupListForLinkEdit()`, `ShowGroupListForLink()`, `GetAllLinksDirectly()`
- `handlers/grup_change_ephemeral.go`: `ShowGroupListForEphemeralEdit()`
- `handlers/grup_change_join_approval.go`: `ShowGroupListForJoinApprovalEdit()`
- `handlers/grup_change_edit.go`: `ShowGroupListForEditEdit()`
- `handlers/grup_change_all_settings.go`: `ShowGroupListForAllSettingsEdit()`

**Perubahan:**
```go
// Sebelum:
func ShowGroupListForLinkEdit(...) {
    groupsMap, err := utils.GetAllGroupsFromDB()
    // ...
}

// Sesudah:
func ShowGroupListForLinkEdit(...) {
    // CRITICAL FIX: Pastikan menggunakan database user yang benar
    am := GetAccountManager()
    userAccount := am.GetAccountByTelegramID(chatID)
    if userAccount != nil {
        EnsureDBConfigForUser(chatID, userAccount)
    }
    
    groupsMap, err := utils.GetAllGroupsFromDB()
    // ...
}
```

### 2. Perbaikan Callback Handler di `telegram.go`

Semua callback handler yang menampilkan grup sekarang menggunakan client dari session user yang benar:

**Perubahan:**
```go
// Sebelum:
case "show_group_list_ephemeral":
    if activeClient == nil || activeClient.Store.ID == nil {
        // ...
    }
    ShowGroupListForEphemeralEdit(...)

// Sesudah:
case "show_group_list_ephemeral":
    // CRITICAL FIX: Gunakan client dari session user yang benar
    if userClient == nil || userClient.Store == nil || userClient.Store.ID == nil {
        // ...
    }
    ShowGroupListForEphemeralEdit(...)
```

**Callbacks yang diperbaiki:**
- `show_group_list_ephemeral`
- `show_group_list_join_approval`
- `show_group_list_all_settings`
- `show_group_list_edit`
- Pagination callbacks: `join_approval_page_X`, `ephemeral_page_X`, `edit_page_X`, `all_settings_page_X`

### 3. Perbaikan Fitur Search Grup

**File: `handlers/grup_search.go`**

**Perubahan:**
```go
// Sebelum:
func HandleSearchInput(...) {
    groups, err := utils.SearchGroups(keyword)
    // ...
}

// Sesudah:
func HandleSearchInput(...) {
    // CRITICAL FIX: Pastikan menggunakan database user yang benar
    am := GetAccountManager()
    userAccount := am.GetAccountByTelegramID(chatID)
    if userAccount != nil {
        EnsureDBConfigForUser(chatID, userAccount)
    }
    
    groups, err := utils.SearchGroups(keyword)
    // ...
}
```

### 4. Perbaikan Fitur Export Grup

**File: `handlers/grup_export.go`**

**Perubahan:**
```go
// Sebelum:
if err := SwitchAccount(userAccount.ID, telegramBot, chatID); err != nil {
    // ...
}
groups, err := utils.GetAllGroupsFromDB()

// Sesudah:
if err := SwitchAccount(userAccount.ID, telegramBot, chatID); err != nil {
    // ...
}

// CRITICAL FIX: Pastikan dbConfig di-update setelah switch
EnsureDBConfigForUser(chatID, userAccount)

groups, err := utils.GetAllGroupsFromDB()
```

## Cara Kerja

### Saat User Mengakses Fitur Grup:
1. Callback handler dipanggil dengan `chatID` (Telegram ID user)
2. `GetUserSession()` dipanggil untuk mendapatkan session user
3. `userAccount` dan `userClient` diambil dari session
4. **BARU:** `EnsureDBConfigForUser()` dipanggil untuk memastikan `dbConfig` menggunakan database user yang benar
5. Fungsi database (`GetAllGroupsFromDB()`, `SearchGroups()`, dll) dipanggil
6. `GetBotDBPool()` mengembalikan database pool dari `dbConfig` yang sudah di-update
7. Data grup yang ditampilkan hanya dari database user yang benar

### Keuntungan:
1. **Isolasi Data**: Setiap user hanya melihat grup dari database mereka sendiri
2. **Konsistensi**: Semua fitur grup menggunakan mekanisme isolasi yang sama
3. **Keamanan**: User tidak bisa mengakses data grup dari user lain
4. **Efisiensi**: Tidak perlu query semua database, hanya database user yang benar

## Files yang Diubah

1. **handlers/grup_list_select.go**
   - `ShowGroupListForLinkEdit()`: Tambah `EnsureDBConfigForUser()`
   - `ShowGroupListForLink()`: Tambah `EnsureDBConfigForUser()`
   - `GetAllLinksDirectly()`: Tambah `EnsureDBConfigForUser()`

2. **handlers/grup_search.go**
   - `HandleSearchInput()`: Tambah `EnsureDBConfigForUser()`

3. **handlers/grup_export.go**
   - `ExportGroupList()`: Tambah `EnsureDBConfigForUser()` setelah switch account

4. **handlers/grup_change_ephemeral.go**
   - `ShowGroupListForEphemeralEdit()`: Tambah `EnsureDBConfigForUser()`

5. **handlers/grup_change_join_approval.go**
   - `ShowGroupListForJoinApprovalEdit()`: Tambah `EnsureDBConfigForUser()`

6. **handlers/grup_change_edit.go**
   - `ShowGroupListForEditEdit()`: Tambah `EnsureDBConfigForUser()`

7. **handlers/grup_change_all_settings.go**
   - `ShowGroupListForAllSettingsEdit()`: Tambah `EnsureDBConfigForUser()`

8. **handlers/telegram.go**
   - Semua callback handler yang menampilkan grup: Ganti `activeClient` dengan `userClient`
   - Semua pagination callbacks: Ganti `activeClient` dengan `userClient`

## Hasil

Dengan perbaikan ini:
- User 1 hanya melihat grup dari database mereka sendiri
- User 2 hanya melihat grup dari database mereka sendiri
- Tidak ada lagi kebocoran data antar user
- Semua fitur grup (daftar, search, export, settings) menggunakan isolasi data yang benar

## Testing

Silakan uji dengan 2 user berbeda:
1. User 1 buka fitur "daftar grup" → Harus menampilkan grup dari database user 1
2. User 2 buka fitur "daftar grup" → Harus menampilkan grup dari database user 2 (tidak mengganggu user 1)
3. User 1 dan User 2 buka fitur "search grup" → Harus mencari di database masing-masing
4. User 1 dan User 2 buka fitur "export grup" → Harus export grup dari database masing-masing

## Catatan

- `GetGroupList()` di `handlers/grup.go` sudah memiliki perbaikan isolasi data sebelumnya
- Semua fungsi database di `utils/bot_database.go` menggunakan `GetBotDBPool()` yang sudah menggunakan `dbConfig` per user
- Perbaikan ini memastikan `dbConfig` selalu di-update sebelum mengakses database

