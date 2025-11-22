# 📁 Konfigurasi Bot WhatsApp

Folder ini berisi file konfigurasi untuk bot WhatsApp.

## 📋 File: `config.json`

File ini berisi konfigurasi utama bot:

```json
{
  "telegram_token": "YOUR_BOT_TOKEN",
  "admin_ids": [123456789],
  "allowed_user_ids": [123456789, 987654321],
  "settings": {
    "max_accounts": 50,
    "timeout_seconds": 30,
    "retry_attempts": 3,
    "log_level": "INFO"
  }
}
```

### 🔧 Field Configuration

#### `telegram_token` (Required)
Token bot Telegram dari @BotFather

#### `admin_ids` (Array, Recommended)
List user ID Telegram yang memiliki akses admin penuh
- Bisa menggunakan semua fitur bot
- Akses penuh ke semua data

#### `allowed_user_ids` (Array, Recommended)
List user ID Telegram yang diizinkan menggunakan bot
- Akses terbatas sesuai kebutuhan
- Tidak bisa mengakses fitur admin

#### `settings` (Object, Optional)
Pengaturan tambahan bot:
- `max_accounts`: Jumlah maksimal akun WhatsApp yang bisa login (default: 50)
- `timeout_seconds`: Timeout untuk operasi API (default: 30)
- `retry_attempts`: Jumlah retry saat error (default: 3)
- `log_level`: Level logging (DEBUG, INFO, WARN, ERROR)

## 🔐 Backward Compatibility

Program juga membaca file `akses.json` di root directory sebagai fallback:

```json
{
  "telegram_token": "YOUR_BOT_TOKEN",
  "user_allowed_id": 123456789
}
```

Jika `config/config.json` tidak ada, program akan menggunakan `akses.json` dengan:
- `user_allowed_id` otomatis dijadikan admin dan allowed user

## 🚀 Cara Menambahkan User

1. **Menambahkan Admin:**
   ```json
   "admin_ids": [6069200226, 123456789, 987654321]
   ```

2. **Menambahkan User Biasa:**
   ```json
   "allowed_user_ids": [6069200226, 555444333]
   ```

3. **Kombinasi Admin + User:**
   ```json
   {
     "admin_ids": [6069200226],
     "allowed_user_ids": [6069200226, 123456789]
   }
   ```

## 🔍 Cara Mendapatkan User ID

1. Buka [@userinfobot](https://t.me/userinfobot) di Telegram
2. Kirim pesan ke bot tersebut
3. Bot akan mengirim ID Anda

## ⚠️ Keamanan

- **JANGAN** share file config.json ke publik
- **JANGAN** commit file config.json ke Git (sudah ada di .gitignore)
- Pastikan permission file adalah 600 (read-write owner only)

## 📝 Restart Bot

Setelah mengubah config:
```bash
pkill -f whatsapp-bot
./whatsapp-bot
```

