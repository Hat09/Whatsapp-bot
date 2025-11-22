# Implementasi Folder Database Per User

## Masalah

1. ✅ Saat program pertama kali hidup, program menampilkan menu kepada user yang telah login sebelumnya
2. ❌ Tapi ketika user yang telah login input `/start`, program menampilkan dan meminta user login ulang
3. Program belum memiliki folder untuk mengelola DB sesuai dengan jumlah pengguna
4. Database yang ada di root directory perlu dipindahkan ke folder per user

## Solusi yang Diterapkan

### 1. Struktur Folder Database Per User

**Format:**
```
DB USER TELEGRAM/
├── {telegramID1}/
│   ├── whatsmeow-{telegramID1}-{phoneNumber}.db
│   ├── whatsmeow-{telegramID1}-{phoneNumber}.db-shm
│   ├── whatsmeow-{telegramID1}-{phoneNumber}.db-wal
│   ├── bot_data-{telegramID1}-{phoneNumber}.db
│   ├── bot_data-{telegramID1}-{phoneNumber}.db-shm
│   └── bot_data-{telegramID1}-{phoneNumber}.db-wal
├── {telegramID2}/
│   ├── whatsmeow-{telegramID2}-{phoneNumber}.db
│   └── bot_data-{telegramID2}-{phoneNumber}.db
└── ...
```

### 2. Fungsi-Fungsi yang Ditambahkan

#### `GetUserDBFolder(telegramID int64) string`
Mendapatkan path folder database untuk user tertentu.
- Format: `DB USER TELEGRAM/{telegramID}/`

#### `EnsureUserDBFolder(telegramID int64) error`
Memastikan folder database untuk user sudah ada. Jika belum ada, buat folder tersebut.

#### `MigrateDatabaseToUserFolder() error`
Memindahkan database yang sudah ada di root directory ke folder user yang sesuai.
- Mencari semua file `whatsmeow-*.db` dan `bot_data-*.db` di root
- Parse Telegram ID dari nama file
- Pindahkan ke folder `DB USER TELEGRAM/{telegramID}/`
- Juga memindahkan file pendukung (-shm, -wal)

### 3. Update Fungsi Database

#### `SetDBConfig(telegramID int64, whatsappNumber string)`
Sekarang menggunakan path folder per user:
- **Sebelum:** `whatsmeow-{telegramID}-{phoneNumber}.db`
- **Sesudah:** `DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db`

#### `GenerateDBName(telegramID int64, whatsappNumber string, dbType string) string`
Sekarang mengembalikan path lengkap dengan folder user:
- **Sebelum:** `bot_data-{telegramID}-{phoneNumber}.db`
- **Sesudah:** `DB USER TELEGRAM/{telegramID}/bot_data-{telegramID}-{phoneNumber}.db`

#### `FindExistingDatabases() (string, string, error)`
Sekarang mencari database di folder user terlebih dahulu, kemudian fallback ke root directory untuk backward compatibility.

### 4. Migrasi Database di Startup

Di `main.go`, sebelum inisialisasi aplikasi:
```go
// Migrate existing databases to user folders (if any)
if err := utils.MigrateDatabaseToUserFolder(); err != nil {
    logger.Warn("Failed to migrate databases to user folders: %v", err)
    // Continue anyway, migration is not critical
}
```

### 5. Perbaikan Handler `/start`

Handler `/start` sekarang:
1. Menggunakan `GetUserSession()` untuk mendapatkan session user
2. Menggunakan `userSession.Client` yang sudah terhubung dari startup
3. Jika client belum terhubung, reconnect client yang sudah ada (tidak membuat client baru)
4. Menggunakan `accountToUse.BotDataDBPath` untuk update `dbConfig`

## Files yang Diubah

1. **utils/db_config.go**
   - Tambah `GetUserDBFolder()`
   - Tambah `EnsureUserDBFolder()`
   - Update `SetDBConfig()` untuk menggunakan path folder
   - Update `GenerateDBName()` untuk menggunakan path folder
   - Update `FindExistingDatabases()` untuk mencari di folder user

2. **utils/db_migration.go** (NEW)
   - `MigrateDatabaseToUserFolder()` - memindahkan database ke folder user
   - `moveDatabaseFile()` - helper untuk memindahkan file database beserta file pendukungnya

3. **main.go**
   - Tambah panggilan `MigrateDatabaseToUserFolder()` sebelum inisialisasi

4. **handlers/user_session.go**
   - Update logika reconnect untuk menggunakan client yang sudah ada dari startup

## Cara Kerja

### Saat Startup:
1. `MigrateDatabaseToUserFolder()` dipanggil
   - Mencari semua database di root directory
   - Memindahkan ke folder user yang sesuai
2. `Initialize()` dipanggil
   - Load accounts dari database
   - Create client untuk setiap account
   - Client disimpan di `AccountManager`

### Saat User Mengirim `/start`:
1. `GetUserSession()` dipanggil
   - Cek apakah session sudah ada
   - Jika ada, gunakan client yang sudah ada dari `AccountManager`
   - Jika client belum terhubung, reconnect (tidak membuat client baru)
2. Handler mengecek `userClient.Store.ID == nil`
   - Jika client sudah terhubung → tampilkan menu utama
   - Jika client `nil` → tampilkan login prompt

### Saat User Baru Pairing:
1. `GenerateDBName()` dipanggil dengan Telegram ID
2. Folder `DB USER TELEGRAM/{telegramID}/` dibuat otomatis
3. Database dibuat di folder tersebut

## Keuntungan

1. **Organisasi yang Lebih Baik**: Database terorganisir per user dalam folder terpisah
2. **Mudah Dikelola**: Setiap user memiliki folder sendiri, mudah untuk backup/restore
3. **Scalable**: Struktur folder otomatis bertambah sesuai jumlah user
4. **Backward Compatible**: Masih mencari database di root directory jika tidak ditemukan di folder user
5. **Auto-Migration**: Database yang sudah ada otomatis dipindahkan saat startup

## Testing

Untuk memverifikasi implementasi:

1. ✅ **Startup**: Database yang ada di root otomatis dipindahkan ke folder user
2. ✅ **User yang sudah login mengakses `/start`**: Menggunakan client dari startup, menampilkan menu utama
3. ✅ **User baru pairing**: Folder user dibuat otomatis, database dibuat di folder tersebut
4. ✅ **Multiple users**: Setiap user memiliki folder dan database terpisah

## Catatan

- Migrasi database hanya dilakukan sekali saat startup
- Database yang sudah dipindahkan tidak akan dipindahkan lagi
- Folder user dibuat otomatis saat pertama kali digunakan
- Struktur folder otomatis bertambah sesuai jumlah user Telegram yang menggunakan bot

