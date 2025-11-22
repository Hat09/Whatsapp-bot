# Perbaikan Scan Folder User untuk Mendaftarkan Account

## Masalah

Dari log terlihat:
1. ✅ Database berhasil dipindahkan ke folder user (`DB USER TELEGRAM/6069200226/`)
2. ❌ `Loaded 0 accounts from database master` - tidak ada account yang ter-load
3. ❌ Account tidak terdaftar di database master (`bot_data.db`)
4. ❌ Saat user input `/start`, program meminta login ulang karena account tidak ditemukan

**Root Cause:**
- `LoadAccounts()` hanya membaca dari database master
- Tidak ada fungsi untuk scan folder user dan mendaftarkan account yang sudah ada
- Account yang sudah ada di folder user tidak terdaftar di database master

## Solusi yang Diterapkan

### 1. Fungsi Scan Folder User

**File: `utils/scan_user_folders.go` (NEW)**

Fungsi `ScanUserFoldersAndRegisterAccounts()`:
- Memindai semua folder di `DB USER TELEGRAM/`
- Mencari file `whatsmeow-{telegramID}-{phoneNumber}.db` di setiap folder
- Mengecek apakah account sudah terdaftar di database master
- Mendaftarkan account yang belum terdaftar ke database master
- Memvalidasi database sebelum mendaftarkan (cek device store)

**Fitur:**
- Auto-register account yang sudah ada di folder user
- Validasi database sebelum mendaftarkan
- Update path jika account sudah ada dengan path berbeda
- Skip account yang sudah terdaftar

### 2. Integrasi di Startup

**File: `main.go`**

Ditambahkan panggilan `ScanUserFoldersAndRegisterAccounts()` setelah migrasi:
```go
// Scan user folders and register existing accounts to master database
if err := utils.ScanUserFoldersAndRegisterAccounts(); err != nil {
    logger.Warn("Failed to scan user folders and register accounts: %v", err)
    // Continue anyway, scan is not critical
}
```

## Cara Kerja

### Saat Startup:
1. `MigrateDatabaseToUserFolder()` - memindahkan database ke folder user
2. **BARU:** `ScanUserFoldersAndRegisterAccounts()` - scan folder user dan daftarkan account
3. `LoadAccounts()` - load account dari database master (sekarang sudah ada)

### Proses Scan:
1. Scan semua folder di `DB USER TELEGRAM/`
2. Untuk setiap folder, cari file `whatsmeow-*.db`
3. Parse Telegram ID dan nomor telepon dari nama file
4. Cek apakah account sudah terdaftar di database master
5. Validasi database (cek device store)
6. Daftarkan account jika belum terdaftar
7. Update path jika account sudah ada dengan path berbeda

## Keuntungan

1. **Auto-Register**: Account yang sudah ada otomatis terdaftar
2. **Validasi**: Hanya mendaftarkan database yang valid
3. **Update Path**: Update path jika account sudah ada dengan path berbeda
4. **Tidak Duplikat**: Skip account yang sudah terdaftar

## Testing

Untuk memverifikasi perbaikan:

1. ✅ **Startup**: Account yang ada di folder user otomatis terdaftar
2. ✅ **LoadAccounts**: Account ter-load dari database master
3. ✅ **User input `/start`**: Account ditemukan, tidak perlu login ulang
4. ✅ **Multiple users**: Setiap user memiliki account terpisah

## Files yang Dibuat/Diubah

1. **utils/scan_user_folders.go** (NEW)
   - `ScanUserFoldersAndRegisterAccounts()` - scan dan daftarkan account
   - `isValidAccountDatabase()` - validasi database account

2. **main.go**
   - Tambah panggilan `ScanUserFoldersAndRegisterAccounts()` di startup

## Kesimpulan

Dengan perbaikan ini, account yang sudah ada di folder user akan otomatis terdaftar di database master saat startup. User yang sudah login tidak perlu login ulang saat mengirim `/start`.

