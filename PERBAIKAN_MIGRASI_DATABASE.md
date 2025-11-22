# Perbaikan Migrasi Database ke Folder User

## Masalah

Setelah implementasi folder database per user, terjadi masalah:

1. ✅ Database berhasil dipindahkan ke folder `DB USER TELEGRAM/{telegramID}/`
2. ❌ Path di database master masih menyimpan path lama (`whatsmeow-{telegramID}-{phoneNumber}.db`)
3. ❌ `ValidateAccount` tidak menemukan file karena mencari di path lama
4. ❌ Account dianggap terblokir dan dihapus, padahal file database masih ada di folder user

**Log Error:**
```
⚠️ ValidateAccount: File database tidak ditemukan untuk akun 3: whatsmeow-6069200226-639382163248.db
⚠️ Multi-account: Account 3 (639382163248) terblokir/logout, akan dihapus
```

## Solusi yang Diterapkan

### 1. Update Path di Database Master Setelah Migrasi

**File: `utils/db_migration.go`**

Ditambahkan fungsi `updateAccountPathsInMasterDB()` yang:
- Memperbarui path di database master setelah memindahkan file
- Mencari account berdasarkan nomor telepon
- Update `db_path` dan `bot_data_db_path` ke path baru di folder user

**Kode:**
```go
// CRITICAL: Update path di database master setelah migrasi
if err := updateAccountPathsInMasterDB(phoneNumber, whatsappDBDest, botDataDBDest); err != nil {
    logger.Warn("Gagal update path di database master untuk nomor %s: %v", phoneNumber, err)
    // Continue anyway, path akan di-update saat LoadAccounts
} else {
    logger.Info("✅ Berhasil update path di database master untuk nomor %s", phoneNumber)
}
```

### 2. Fallback di ValidateAccount

**File: `handlers/multi_account.go`**

Ditambahkan logika fallback di `ValidateAccount()`:
- Jika file tidak ditemukan di path lama, coba cari di folder user
- Jika ditemukan di folder user, update path di database
- Gunakan path baru untuk validasi

**Kode:**
```go
// Cek apakah file database ada
if _, err := os.Stat(account.DBPath); os.IsNotExist(err) {
    // CRITICAL FIX: Coba cari di folder user jika file tidak ditemukan di path lama
    pattern := regexp.MustCompile(`whatsmeow-(\d+)-(\d+)\.db$`)
    matches := pattern.FindStringSubmatch(account.DBPath)
    if len(matches) == 3 {
        telegramIDStr := matches[1]
        // Coba cari di folder user
        userFolder := filepath.Join("DB USER TELEGRAM", telegramIDStr)
        expectedPath := filepath.Join(userFolder, filepath.Base(account.DBPath))
        if _, err := os.Stat(expectedPath); err == nil {
            // File ditemukan di folder user, update path di database
            // ... update path dan gunakan path baru
        }
    }
}
```

## Cara Kerja

### Saat Migrasi (Startup):
1. `MigrateDatabaseToUserFolder()` dipanggil
2. File database dipindahkan ke folder user
3. **BARU:** Path di database master di-update ke path baru
4. Account tetap valid dengan path baru

### Saat Validasi Account:
1. `ValidateAccount()` mengecek file di path yang tersimpan
2. Jika tidak ditemukan, coba cari di folder user
3. Jika ditemukan di folder user, update path di database
4. Gunakan path baru untuk validasi selanjutnya

## Keuntungan

1. **Double Protection**: Path di-update saat migrasi DAN saat validasi
2. **Backward Compatible**: Masih mencari di path lama, kemudian fallback ke folder user
3. **Auto-Fix**: Path otomatis di-update jika ditemukan di folder user
4. **Tidak Ada Data Loss**: Account tidak dihapus jika file masih ada di folder user

## Testing

Untuk memverifikasi perbaikan:

1. ✅ **Migrasi database**: Path di database master otomatis di-update
2. ✅ **Validasi account**: Account tetap valid meskipun path lama tidak ditemukan
3. ✅ **User yang sudah login**: Tidak perlu login ulang setelah migrasi
4. ✅ **Account tidak dihapus**: Account tetap ada meskipun path lama tidak ditemukan

## Files yang Diubah

1. **utils/db_migration.go**
   - Tambah `updateAccountPathsInMasterDB()` untuk update path di database master
   - Panggil fungsi ini setelah memindahkan file database

2. **handlers/multi_account.go**
   - Update `ValidateAccount()` untuk mencari di folder user jika file tidak ditemukan
   - Auto-update path di database jika ditemukan di folder user

## Kesimpulan

Dengan perbaikan ini, account tidak akan dihapus setelah migrasi database ke folder user. Path di database master akan otomatis di-update, dan `ValidateAccount` akan mencari di folder user jika file tidak ditemukan di path lama.

