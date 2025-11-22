# 📁 Struktur Folder Proyek

## 📂 Struktur Folder yang Dirapikan

```
/root/Projel/
├── main.go                    # Entry point aplikasi
├── core/                      # Core functionality (lifecycle management)
│   ├── startup.go            # Startup manager
│   ├── shutdown.go           # Shutdown manager  
│   └── events.go             # WhatsApp event handler
│
├── utils/                     # Utilities (organized by category)
│   ├── app_logger.go        # Application logger (source in logger/)
│   ├── grup_logger.go       # Group logger (source in logger/)
│   ├── bot_database.go      # Database operations (source in database/)
│   ├── database_helper.go   # Database helper (source in database/)
│   ├── db_config.go         # Database config (source in database/)
│   ├── telegram_config.go   # Telegram config (source in config/)
│   │
│   ├── logger/               # Logging source files (organized)
│   │   ├── app_logger.go
│   │   └── grup_logger.go
│   ├── database/             # Database source files (organized)
│   │   ├── bot_database.go
│   │   ├── helper.go
│   │   └── config.go
│   └── config/               # Config source files (organized)
│       └── telegram_config.go
│
├── handlers/                  # Handlers (main files for import)
│   ├── telegram.go          # Telegram handler
│   ├── telegram_helper.go   # Telegram helper
│   ├── pairing.go           # WhatsApp pairing
│   ├── logout.go            # WhatsApp logout
│   ├── grup.go              # Group handler
│   └── grup_enrich.go       # Group enrichment
│
├── internal/                 # Internal organized source files
│   └── handlers/            # Source files organized for maintenance
│       ├── telegram/        # Telegram handlers (source)
│       ├── whatsapp/        # WhatsApp handlers (source)
│       └── grup/            # Group handlers (source)
│
├── ui/                        # UI components
│   ├── menu.go               # Main menu UI
│   └── welcome.go            # Welcome message UI
│
└── backup/                    # Backup files
    └── main_old.go          # Backup versi lama main.go
```

## 🔄 Keuntungan Struktur Baru

1. **Lebih Terorganisir**: File dikelompokkan berdasarkan fungsi/kategori
2. **Mudah Dipelihara**: Lokasi file lebih mudah ditemukan
3. **Scalable**: Mudah menambah fitur baru tanpa membuat folder berantakan
4. **Clear Separation**: Pemisahan yang jelas antara core, handlers, utils, dan UI
5. **Dual Structure**: 
   - File aktif di root folder untuk import compatibility
   - File source terorganisir di subfolder untuk maintenance
6. **Backward Compatible**: Import path tetap `whatsapp-bot/utils` dan `whatsapp-bot/handlers`
