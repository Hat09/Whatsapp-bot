# Security Fix: User Database Isolation

## Masalah yang Ditemukan

Terdapat kerentanan keamanan di mana user dengan Telegram ID berbeda dapat mengakses database WhatsApp milik user lain. Masalah ini terjadi karena:

1. **Tidak ada auto-switch ke akun user**: Ketika user mengirim callback query atau command, sistem tidak otomatis beralih ke akun yang sesuai dengan Telegram ID mereka
2. **Global client sharing**: Sistem menggunakan client global yang mungkin milik user lain (misalnya admin)
3. **Database isolation tidak terjamin**: Setiap user seharusnya hanya mengakses database mereka sendiri

## Contoh Masalah

- User dengan Telegram ID `7793345217` dapat mengakses database WhatsApp milik admin (Telegram ID `6069200226`)
- Ini terjadi saat pertama kali user `7793345217` membuka program
- Semua akun DB dari user yang terdaftar dapat saling mengakses satu sama lain

## Solusi yang Diterapkan

### 1. Fungsi `GetAccountByTelegramID`
Menambahkan fungsi untuk menemukan akun berdasarkan Telegram ID dengan mem-parse `BotDataDBPath` yang berformat `bot_data(telegramID)>(phoneNumber).db`.

**Lokasi**: `handlers/multi_account.go`

```go
func (am *AccountManager) GetAccountByTelegramID(telegramID int64) *WhatsAppAccount
```

### 2. Fungsi `EnsureUserAccountActive`
Menambahkan fungsi helper yang memastikan akun user aktif sebelum memproses request. Fungsi ini akan:
- Mencari akun berdasarkan Telegram ID
- Otomatis switch ke akun user jika belum aktif
- Memastikan database isolation per user

**Lokasi**: `handlers/multi_account.go`

```go
func EnsureUserAccountActive(telegramID int64, telegramBot *tgbotapi.BotAPI) (*WhatsAppAccount, error)
```

### 3. Modifikasi `HandleCallbackQuery`
Menambahkan auto-switch di awal fungsi untuk memastikan setiap callback query diproses dengan akun yang sesuai dengan Telegram ID user.

**Lokasi**: `handlers/telegram.go`

### 4. Modifikasi `HandleTelegramCommand`
Menambahkan auto-switch di awal fungsi untuk memastikan setiap command diproses dengan akun yang sesuai dengan Telegram ID user.

**Lokasi**: `handlers/telegram.go`

## Cara Kerja

1. **Saat user mengirim callback/command**:
   - Sistem memanggil `EnsureUserAccountActive(telegramID, telegramBot)`
   - Fungsi ini mencari akun yang sesuai dengan Telegram ID
   - Jika ditemukan dan belum aktif, sistem otomatis switch ke akun tersebut
   - `SwitchAccount` akan:
     - Set akun sebagai current account
     - Update `dbConfig` dengan Telegram ID dan nomor WhatsApp yang benar
     - Close dan rebuild database pool dengan database yang benar
     - Memastikan isolasi database per user

2. **Database Isolation**:
   - Setiap user memiliki database terpisah dengan format: `bot_data(telegramID)>(phoneNumber).db`
   - `dbConfig` di-update setiap kali switch account
   - Database pool di-rebuild untuk menggunakan database yang benar

## Testing

Untuk memverifikasi perbaikan:

1. **Test dengan 2 user berbeda**:
   - User A (Telegram ID: 6069200226) melakukan pairing
   - User B (Telegram ID: 7793345217) melakukan pairing
   - User B mengirim callback query (misalnya klik tombol "grup")
   - Verifikasi bahwa User B hanya mengakses database mereka sendiri

2. **Check logs**:
   - Log akan menampilkan: `Auto-switched to user account: ID=X, Phone=Y, TelegramID=Z`
   - Pastikan Telegram ID sesuai dengan user yang mengirim request

3. **Verify database access**:
   - User A tidak dapat melihat data dari database User B
   - User B tidak dapat melihat data dari database User A

## Backward Compatibility

- Jika user belum memiliki akun terdaftar, sistem akan menggunakan current account (untuk backward compatibility)
- Log akan menampilkan: `No account found for TelegramID X, using current account`

## Catatan Penting

- Perbaikan ini memastikan **database isolation per user**
- Setiap user hanya dapat mengakses database mereka sendiri
- Auto-switch terjadi secara otomatis tanpa perlu intervensi manual
- Perbaikan ini tidak mempengaruhi fungsionalitas yang sudah ada

## Perbaikan Tambahan (Validasi Client & Tolak Akses)

Setelah testing, ditemukan bahwa validasi di entry point saja tidak cukup. Masalahnya adalah:

1. **Dua Telegram user berbeda menggunakan nomor WhatsApp yang sama**: Jika dua user menggunakan nomor WhatsApp yang sama, mereka akan sharing WhatsApp client yang sama
2. **Validasi client di fungsi yang mengakses WhatsApp**: Perlu validasi tambahan di fungsi-fungsi yang langsung mengakses WhatsApp client (seperti `GetGroupList`)
3. **Fallback ke current account (admin)**: Jika user belum memiliki akun terdaftar, sistem akan fallback ke current account (admin), yang menyebabkan user lain bisa mengakses database admin

### Perbaikan Kritis: Tolak Akses untuk User Tanpa Akun

**Masalah**: Ketika user belum memiliki akun terdaftar, sistem akan fallback ke current account (admin), yang menyebabkan user lain bisa mengakses database admin.

**Solusi**: 
- **TOLAK akses** jika user belum memiliki akun terdaftar
- Jangan fallback ke current account (admin) untuk keamanan
- KECUALI untuk command/callback yang memang untuk pairing baru (`/pair`, `start_pairing`, `back_to_login`, `login_info`, `login_help`)

**Lokasi**: `handlers/telegram.go` - `HandleTelegramCommand` dan `HandleCallbackQuery`

### Validasi di `GetGroupList`

Menambahkan validasi di `GetGroupList` untuk memastikan:
- User hanya bisa mengakses WhatsApp account yang sesuai dengan Telegram ID mereka
- Jika client yang digunakan tidak sesuai dengan akun user, sistem akan otomatis switch ke akun user yang benar
- Jika user belum memiliki akun terdaftar, akses akan ditolak

**Lokasi**: `handlers/grup.go`

## Files Modified

1. `handlers/multi_account.go` - Menambahkan `GetAccountByTelegramID` dan `EnsureUserAccountActive`
2. `handlers/telegram.go` - Modifikasi `HandleCallbackQuery` dan `HandleTelegramCommand`
3. `handlers/grup.go` - Menambahkan validasi akses di `GetGroupList`
4. `handlers/reset.go` - Menambahkan validasi di `ConfirmResetProgram` (hanya admin/user dengan akun)
5. `handlers/logout.go` - Menambahkan validasi di `ConfirmLogout` (hanya akun sendiri)
6. `handlers/grup_export.go` - Menambahkan validasi di `ExportGroupList`
7. `handlers/activity_log.go` - Menambahkan validasi di `ShowActivityLog`

## Audit Lengkap

Lihat `SECURITY_AUDIT_COMPLETE.md` untuk audit lengkap semua fitur dan status keamanan.

