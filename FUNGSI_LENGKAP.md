# 📋 DAFTAR LENGKAP FUNGSI PROGRAM

## 🔧 **TELEGRAM COMMANDS** (Chat Commands)
Command yang bisa dipanggil dengan `/command` di Telegram:

### 📱 **Informasi & Status**
- `/start` atau `/menu` - Menampilkan menu utama atau login prompt
- `/status` - Menampilkan status bot WhatsApp (terhubung/belum)
- `/info` - Menampilkan informasi lengkap bot
- `/time` - Menampilkan waktu sekarang
- `/ping` - Test bot aktif (menjawab "Pong!")

### 👥 **Grup**
- `/grup` - Menampilkan menu manajemen grup WhatsApp

### 🔐 **Pengaturan**
- `/pair <nomor>` - Melakukan pairing WhatsApp dengan nomor tertentu
- `/logout` - Logout dari WhatsApp dan hapus database
- `/help` - Menampilkan bantuan lengkap

---

## 🔘 **INLINE KEYBOARD BUTTONS** (Tombol di Menu)
Tombol callback yang dipanggil saat user klik tombol inline keyboard:

### 📊 **Informasi**
- `status` - Tombol "📊 Status" - Menampilkan status bot
- `info` - Tombol "ℹ️ Info" - Menampilkan info bot
- `refresh` - Tombol "🔄 Refresh" - Refresh menu utama
- `help` - Tombol "❓ Help" - Menampilkan bantuan

### 👥 **Grup**
- `grup` - Tombol "👥 Grup" - Menampilkan menu grup
- `list_grup` - Tombol "Lihat Daftar Grup" - Menampilkan daftar semua grup

### 🔐 **Login & Pairing**
- `start_pairing` - Tombol "🔗 Mulai Pairing" - Memulai proses pairing
- `cancel_pairing` - Tombol "❌ Batalkan Pairing" - Membatalkan pairing
- `back_to_login` - Tombol "Kembali ke Login" - Kembali ke login prompt
- `login_info` - Tombol "ℹ️ Info" (di login) - Info tentang pairing
- `login_help` - Tombol "❓ Help" (di login) - Bantuan tentang pairing
- `cancel_phone_input` - Tombol "❌ Batalkan" - Membatalkan input nomor

### 🚪 **Logout**
- `logout` - Tombol "🚪 Logout" - Memulai proses logout
- `logout_confirm` - Tombol "✅ Ya, Logout" - Konfirmasi logout
- `logout_cancel` - Tombol "❌ Batal" - Membatalkan logout

---

## 📨 **WHATSAPP EVENT HANDLERS** (Event dari WhatsApp)
Fungsi yang dipanggil saat ada event dari WhatsApp:

### 📩 **Message Handler**
- `handleMessage()` - Memproses pesan yang masuk dari WhatsApp
  - Menyimpan pesan ke database
  - Menyimpan grup ke database (jika pesan dari grup)
  - Mengirim notifikasi ke Telegram
  - Memproses command dari WhatsApp (jika ada)

### 🔌 **Connection Events**
- `Connected` - Event saat WhatsApp client terhubung
- `Disconnected` - Event saat WhatsApp client terputus
- `LoggedOut` - Event saat user logout
- `PairSuccess` - Event saat pairing berhasil (sudah ditangani di pairing.go)

---

## 💬 **WHATSAPP COMMANDS** (Command di Chat WhatsApp)
Command yang bisa diketik di chat WhatsApp:

- `!ping`, `ping`, `test` - Bot menjawab "🏓 Pong! Bot aktif!"
- `!info`, `info` - Bot menampilkan info (User, Group)
- `!time`, `time` - Bot menampilkan waktu sekarang
- `!help`, `help` - Bot menampilkan menu command

---

## 🔐 **PAIRING & AUTHENTICATION**
Fungsi-fungsi terkait pairing WhatsApp:

- `PairDeviceViaTelegram()` - Proses pairing WhatsApp via Telegram
  - Validasi nomor telepon
  - Generate pairing code
  - Menampilkan instruksi pairing
  - Menunggu konfirmasi pairing (timeout 2 menit)
  - Rename database setelah pairing berhasil
  
- `ValidatePhoneNumber()` - Validasi format nomor telepon
- `ensureConnection()` - Memastikan WhatsApp client terhubung
- `HandlePhoneNumberInput()` - Memproses input nomor dari user
- `showPhoneInputPrompt()` - Menampilkan prompt input nomor

---

## 🚪 **LOGOUT**
Fungsi-fungsi terkait logout:

- `LogoutWhatsApp()` - Memulai proses logout (dengan konfirmasi)
- `ConfirmLogout()` - Melakukan logout dan hapus database setelah konfirmasi

---

## 👥 **GRUP MANAGEMENT**
Fungsi-fungsi terkait manajemen grup:

### 📋 **List Grup**
- `GetGroupList()` - Mengambil dan menampilkan daftar semua grup
- `showGroupMenu()` - Menampilkan menu manajemen grup

### 🔍 **Fetch Grup**
- `fetchAllGroups()` - Mengambil semua grup dari database/API
- `fetchGroupsFromWhatsAppDBFast()` - Mengambil grup dari database WhatsApp (JOIN query)
- `fetchGroupsFromWhatsAppDBSimple()` - Fallback: mengambil grup dengan query sederhana
- `enrichGroupNamesFromAPI()` - Mengambil nama grup dari API WhatsApp

### ⚙️ **Processing Grup**
- `enrichGroupNamesFromAPIConcurrent()` - Mengambil nama grup dengan concurrent API calls
- `filterValidGroups()` - Filter grup yang valid untuk ditampilkan
- `sortGroupsByName()` - Sort grup berdasarkan nama (angka dulu, lalu alfabet)
- `compareNames()` - Helper untuk membandingkan nama grup
- `sendGroupListInChunks()` - Mengirim daftar grup dalam beberapa pesan (chunking)

### 📊 **Progress & UI**
- `updateProgressMessage()` - Update progress saat mengambil nama grup
- `generateProgressBar()` - Generate progress bar visual
- `escapeMarkdown()` - Escape karakter Markdown
- `isNumeric()` - Cek apakah string adalah angka

---

## 🎨 **UI FUNCTIONS** (User Interface)
Fungsi-fungsi untuk menampilkan UI:

- `ShowWelcome()` - Menampilkan pesan welcome saat program pertama kali dinyalakan
- `ShowLoginPrompt()` - Menampilkan prompt login jika belum login
- `ShowMainMenu()` - Menampilkan menu utama dengan semua tombol
- `FormatPairingInstructions()` - Format instruksi pairing dengan styling
- `FormatPairingSuccess()` - Format pesan success setelah pairing

---

## ⚙️ **UTILITY FUNCTIONS**
Fungsi-fungsi utility:

### 🔧 **Handler Setup**
- `SetClients()` - Set WhatsApp client dan Telegram bot
- `SetTelegramConfig()` - Set konfigurasi Telegram
- `SendToTelegram()` - Mengirim pesan ke Telegram

### 📊 **Progress & Display**
- `getProgressBar()` - Membuat progress bar visual (untuk pairing)

---

## 🗄️ **DATABASE FUNCTIONS** (Utils)
Fungsi-fungsi database:

- `SaveMessageToDB()` - Menyimpan pesan ke database
- `SaveGroupToDB()` - Menyimpan grup ke database
- `BatchSaveGroupsToDB()` - Batch save grup ke database
- `GetAllGroupsFromDB()` - Mengambil semua grup dari database
- `SetupBotDB()` - Setup database bot
- `ClearAppState()` - Clear app state (untuk fix LTHash error)
- `GetBotDBPool()` - Get connection pool untuk bot database
- `GetWhatsAppDBPool()` - Get connection pool untuk WhatsApp database

---

## 🎯 **CORE FUNCTIONS** (Startup & Lifecycle)
Fungsi-fungsi core untuk lifecycle aplikasi:

### 🚀 **Startup**
- `Initialize()` - Inisialisasi aplikasi (StartupManager)
  - Phase 1: Load Configuration
  - Phase 2: Initialize Telegram Bot
  - Phase 3: Initialize Database
  - Phase 4: Initialize WhatsApp Client
  - Phase 5: Finalize Setup

### 🔌 **Shutdown**
- `Shutdown()` - Graceful shutdown aplikasi
- `ShutdownWithTimeout()` - Shutdown dengan timeout

### 📨 **Event Handling**
- `EventHandler()` - Main event handler untuk WhatsApp events
- `extractMessageText()` - Ekstrak teks dari berbagai tipe pesan WhatsApp
- `handleCommands()` - Handle command dari WhatsApp
- `sendMessage()` - Kirim pesan WhatsApp
- `notifyTelegram()` - Kirim notifikasi ke Telegram

---

## 📝 **RINGKASAN**

**Total Fungsi:**
- Telegram Commands: **8 commands**
- Inline Keyboard Buttons: **13 callback buttons**
- WhatsApp Commands: **4 commands** (!ping, !info, !time, !help)
- Grup Management Functions: **12 functions**
- UI Functions: **5 functions**
- Utility Functions: **3 functions**
- Database Functions: **8 functions**
- Core Functions: **7 functions**

**Total: ~60+ functions**

---

## 🗑️ **FUNGSI YANG BISA DIHAPUS** (Opsional)

Jika Anda ingin menyederhanakan program, berikut fungsi yang **optional** dan bisa dihapus:

### 🔴 **Fungsi yang Bisa Dihapus:**
1. `/ping` - Hanya untuk test bot aktif
2. `/time` - Hanya menampilkan waktu (bisa diganti dengan info di status)
3. `!ping`, `!info`, `!time`, `!help` - Command WhatsApp yang mungkin tidak perlu
4. `notifyTelegram()` - Notifikasi setiap pesan WhatsApp ke Telegram (bisa terlalu banyak notifikasi)
5. `handleCommands()` di WhatsApp - Handler command dari WhatsApp chat
6. `saveMessageToDB()` - Penyimpanan pesan ke database (jika tidak diperlukan)
7. Progress bar functions - Jika ingin UI lebih sederhana

### 🟡 **Fungsi yang Bisa Disederhanakan:**
1. Grup enrichment - Bisa dihilangkan concurrent API calls jika tidak perlu nama grup real-time
2. Database config dynamic naming - Bisa pakai nama database statis jika hanya 1 user

Silakan sebutkan fungsi mana yang ingin Anda hapus!

