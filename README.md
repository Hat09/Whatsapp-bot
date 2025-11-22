# WhatsApp Bot dengan Telegram Integration

Bot WhatsApp yang dikontrol melalui Telegram dengan fitur auto-restart saat development.

## 🚀 Cara Menjalankan

### Mode Development (dengan auto-restart)

```bash
air
```

Program akan otomatis restart ketika ada perubahan pada file `.go`.

### Mode Production

```bash
go run main.go
```

atau

```bash
go build -o whatsapp-bot main.go
./whatsapp-bot
```

## 📁 Struktur Project

```
.
├── commands/          # Command handlers
├── handlers/          # Handler untuk pairing dan Telegram
├── ui/                # UI components (welcome, menu)
├── utils/             # Utilities (config, database, logger)
├── main.go            # Entry point
├── akses.json         # Konfigurasi Telegram (token, user ID)
├── .air.toml          # Konfigurasi Air (auto-restart)
└── README.md          # Dokumentasi
```

## ⚙️ Konfigurasi

1. Buat file `akses.json`:
```json
{
  "telegram_token": "YOUR_TELEGRAM_BOT_TOKEN",
  "user_allowed_id": YOUR_TELEGRAM_USER_ID
}
```

2. Jalankan program dan ikuti instruksi pairing WhatsApp.

## 🔧 Development Tools

### Auto-Restart dengan Air

Program akan otomatis restart ketika ada perubahan pada kode.

**Install Air:**
```bash
go install github.com/cosmtrek/air@latest
```

**Cara menggunakan:**

1. **Dengan Makefile (Recommended):**
```bash
make dev          # Development mode dengan auto-restart
make run          # Run normal (tanpa auto-restart)
make build        # Build binary
make install-air  # Install Air tool
```

2. **Dengan File Watcher Script:**
```bash
./watch.sh        # Auto-restart menggunakan file watcher (tidak perlu Air)
```

3. **Dengan Script Air:**
```bash
./run_dev.sh
```

4. **Langsung dengan Air (jika terinstall):**
```bash
air               # Jika air sudah di PATH
~/go/bin/air      # Jika air di ~/go/bin
```

**Catatan:** 
- Air memerlukan Go 1.25+. Jika menggunakan Go versi lebih lama, gunakan `./watch.sh`
- File watcher (`watch.sh`) menggunakan `inotifywait` (Linux) atau polling method sebagai fallback
- Install inotify-tools untuk performa lebih baik: `apt-get install inotify-tools`

## 📋 Fitur

- ✅ Kontrol WhatsApp melalui Telegram
- ✅ Notifikasi pesan WhatsApp masuk
- ✅ Kirim pesan WhatsApp dari Telegram
- ✅ Menu interaktif
- ✅ Welcome message
- ✅ Auto-restart saat development

