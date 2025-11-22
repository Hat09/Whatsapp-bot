# 🖼️ FITUR GANTI FOTO PROFIL GRUP - COMPLETE!

## 🎯 DESKRIPSI FITUR

Fitur baru untuk mengganti foto profil grup WhatsApp secara batch dengan 3 metode pilihan:
1. **🔍 Cari Manual** - Ketik nama grup untuk mencari
2. **📋 Lihat & Pilih** - Browse daftar grup dan pilih
3. **⚡ Ubah Semua** - Ganti foto semua grup sekaligus

---

## 📊 FLOW DIAGRAM

```
┌─────────────────────────────────────────────────┐
│  User Klik: 🖼️ Ganti Foto                      │
└─────────────────┬───────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │   Menu Ganti Foto  │
        └─────────┬─────────┘
                  │
        ┌─────────┴─────────────────────┐
        │                               │
┌───────▼────────┐          ┌──────────▼─────────┐
│ 🔍 Cari Manual │          │ 📋 Lihat & Pilih   │
└───────┬────────┘          └──────────┬─────────┘
        │                              │
┌───────▼──────────┐          ┌────────▼────────┐
│ Ketik Nama Grup  │          │ Pilih Nomor Grup│
└───────┬──────────┘          └────────┬────────┘
        │                              │
        └────────┬──────────┬──────────┘
                 │          │
        ┌────────▼──────────▼─────────┐
        │  Ketik Delay (detik)        │
        └────────┬────────────────────┘
                 │
        ┌────────▼────────────────────┐
        │  Kirim Foto (JPG/PNG)       │
        └────────┬────────────────────┘
                 │
        ┌────────▼────────────────────┐
        │  Processing dengan Progress │
        └────────┬────────────────────┘
                 │
        ┌────────▼────────────────────┐
        │  Result Summary + Failed    │
        └─────────────────────────────┘
```

---

## 🔧 IMPLEMENTASI TEKNIS

### 1️⃣ File Baru: `handlers/grup_change_photo.go` (691 lines)

#### State Management:
```go
type GroupPhotoState struct {
    WaitingForGroupName bool
    WaitingForDelay     bool
    WaitingForPhoto     bool
    SelectedGroups      []GroupLinkInfo
    Keyword             string
    DelaySeconds        int
    PhotoPath           string  // Temp file path
}
```

#### Fungsi Utama:

1. **ShowChangePhotoMenu()** / **ShowChangePhotoMenuEdit()**
   - Menampilkan menu ganti foto dengan 3 metode
   - Total grup tersedia, catatan penting, tips

2. **ShowPhotoExampleEdit()**
   - Contoh penggunaan step-by-step
   - Tips delay dan tips foto (format, ukuran, resolusi)

3. **StartChangePhotoProcess()**
   - Inisialisasi state untuk ganti foto
   - Prompt input nama grup dengan contoh

4. **HandleGroupNameInputForPhoto()**
   - Smart search logic (sama seperti ambil link)
   - Multi-line: exact match multiple
   - Long single-line (>30 chars): exact → flexible
   - Short keyword: flexible search
   - Natural sorting results

5. **HandleDelayInputForPhoto()**
   - Validasi delay (0-60 detik)
   - Prompt untuk upload foto
   - Tips persyaratan foto

6. **HandlePhotoUpload()**
   - Download foto dari Telegram
   - Simpan ke temp file
   - Trigger `ProcessChangePhotos()`

7. **ProcessChangePhotos()**
   - **Progress tracking**: Edit single message dengan progress bar
   - **Batch processing**: Ganti foto per grup dengan delay
   - **Error handling**: Track success/failed groups
   - **Result summary**: Total, berhasil, gagal
   - **Failed details**: Batch 10 grup per message
   - **Cleanup**: Hapus temp file setelah selesai

8. **CancelChangePhoto()**
   - Cleanup temp file jika ada
   - Reset state

---

### 2️⃣ Update `handlers/grup.go`

#### Perubahan:

```go
// Tambah menu item di showGroupMenu()
📋 **Lihat Daftar** - Tampilkan semua grup
🔍 **Cari Grup** - Filter berdasarkan nama
🔗 **Ambil Link** - Get link undangan grup
🖼️ **Ganti Foto** - Ubah foto profil grup  ← NEW!
📥 **Export** - Download daftar ke file

// Tambah button di keyboard
tgbotapi.NewInlineKeyboardRow(
    tgbotapi.NewInlineKeyboardButtonData("🔗 Ambil Link", "get_link_menu"),
    tgbotapi.NewInlineKeyboardButtonData("🖼️ Ganti Foto", "change_photo_menu"),
),
```

**Berlaku untuk**:
- `showGroupMenu()` - New message version
- `ShowGroupManagementMenuEdit()` - Edit message version (NO SPAM!)

---

### 3️⃣ Update `handlers/telegram.go`

#### Routing Callbacks:

```go
case "change_photo_menu":
    // Handler untuk menu ganti foto profil grup - EDIT existing message
    if client == nil || client.Store.ID == nil {
        editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Bot WhatsApp belum terhubung.")
        telegramBot.Send(editMsg)
        return
    }
    ShowChangePhotoMenuEdit(telegramBot, chatID, messageID)

case "start_change_photo":
    // Mulai proses ganti foto
    if client == nil || client.Store.ID == nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Bot WhatsApp belum terhubung.")
        telegramBot.Send(msg)
        return
    }
    StartChangePhotoProcess(telegramBot, chatID)

case "photo_example":
    // Tampilkan contoh penggunaan ganti foto - EDIT existing message
    ShowPhotoExampleEdit(telegramBot, chatID, messageID)

case "cancel_change_photo":
    // Batalkan proses ganti foto
    CancelChangePhoto(chatID, telegramBot)
```

---

### 4️⃣ Update `main.go`

#### Handler untuk Photo Upload:

```go
// Handle change photo input (text part)
if handlers.IsWaitingForPhotoInput(chatID) {
    inputType := handlers.GetPhotoInputType(chatID)

    // Handle photo upload
    if update.Message.Photo != nil && len(update.Message.Photo) > 0 && inputType == "photo" {
        photo := update.Message.Photo[len(update.Message.Photo)-1]  // Get largest
        handlers.HandlePhotoUpload(&photo, chatID, waClient, telegramBot)
        continue
    }

    // Handle text input (group name or delay)
    input := strings.TrimSpace(update.Message.Text)
    if inputType == "group_name" {
        handlers.HandleGroupNameInputForPhoto(input, chatID, telegramBot)
    } else if inputType == "delay" {
        handlers.HandleDelayInputForPhoto(input, chatID, telegramBot)
    }
    continue
}
```

---

## 📋 FLOW LENGKAP

### **Metode 1: Cari Manual**

```
1. User: Klik "🖼️ Ganti Foto" (dari Menu Grup)
   Bot: [EDIT] Menu Ganti Foto

2. User: Klik "🔍 Cari Manual"
   Bot: [NEW] Prompt input nama grup

3. User: Ketik "Keluarga Besar"
   Bot: [DELETE loading] [NEW] Daftar grup ditemukan (3 grup)

4. Bot: Minta delay
   User: Ketik "2"

5. Bot: Minta foto
   User: [Kirim foto JPG]

6. Bot: [NEW] "Foto diterima! Memulai proses..."
   Bot: [NEW] Progress 33% → [EDIT] Progress 66% → [EDIT] Progress 100%
   Bot: [DELETE progress]

7. Bot: [NEW] Result Summary:
   ✅ Berhasil: 2 grup
   ❌ Gagal: 1 grup

8. Bot: [NEW] Detail Failed:
   ❌ Keluarga Besar 3 (bot not admin)

9. Bot: [NEW] Keyboard:
   [🖼️ Ganti Lagi] [🔙 Menu Grup]
```

### **Metode 2: Lihat & Pilih**

```
1. User: Klik "📋 Lihat & Pilih"
   Bot: [EDIT] Daftar Grup Halaman 1

2. User: Ketik "1,3,5"
   Bot: [NEW] Grup dipilih (3 grup), minta delay

3. User: Ketik "3"
   Bot: [NEW] Minta foto

4. User: [Kirim foto]
   Bot: Processing... → Result
```

### **Metode 3: Ubah Semua**

```
1. User: Klik "⚡ Ubah Semua"
   Bot: [NEW] Konfirmasi (50 grup akan diubah)

2. User: Konfirmasi
   Bot: [NEW] Minta delay

3. User: Ketik "5"
   Bot: [NEW] Minta foto

4. User: [Kirim foto]
   Bot: Processing (may take a while)... → Result
```

---

## 🔑 FITUR KHUSUS

### 1. **Smart Search**
- **Multi-line input** → Exact match multiple groups
- **Long keyword (>30 chars)** → Exact then flexible
- **Short keyword** → Flexible search
- **"." (dot)** → Select all groups

### 2. **Progress Tracking**
```
⏳ PROGRESS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
████████████░░░░░░░░ 60%
📊 Diproses: 6/10 grup
✅ Berhasil: 5
❌ Gagal: 1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⏳ Sedang memproses...
```

### 3. **No Spam!**
- Progress bar → EDIT single message
- Menu navigation → EDIT existing message
- Only send NEW message for:
  - Prompt input (first time)
  - Final result summary
  - Failed groups detail

### 4. **Batch Result**
- If >10 failed groups → Split into batches
- 10 groups per batch message
- Auto delay between batches (1 second)

### 5. **Photo Handling**
- Download from Telegram API
- Save to temp file with unique name
- Auto cleanup after processing
- Support JPG, PNG, WEBP
- Max 5MB (Telegram limit)

### 6. **Error Handling**
- Bot not admin → Skip with error
- Invalid JID → Skip with error
- Photo read error → Skip with error
- Network timeout → Skip with error
- All errors tracked and reported

---

## 🎨 UI/UX

### Menu Ganti Foto:
```
🖼️ **GANTI FOTO PROFIL GRUP**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Fitur ini akan mengganti foto profil grup WhatsApp yang Anda pilih.

📊 **Total grup tersedia:** 53 grup

**📋 Pilihan Metode:**

🔍 **Cari Manual** - Ketik nama/kata kunci grup
📋 **Lihat & Pilih** - Lihat daftar lalu pilih
⚡ **Ubah Semua** - Proses semua grup sekaligus

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⚠️ **Catatan Penting:**
• Bot harus menjadi admin grup untuk ganti foto
• Delay membantu menghindari rate limit WhatsApp
• Foto akan sama untuk semua grup yang dipilih
• Proses mungkin memakan waktu untuk banyak grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Pilih metode yang Anda inginkan
```

### Prompt Foto:
```
📸 **KIRIM FOTO**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ **Grup dipilih:** 5 grup
⏱️ **Delay:** 2 detik per grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🖼️ **LANGKAH TERAKHIR**

Kirim foto yang ingin Anda gunakan sebagai foto profil grup.

**Persyaratan Foto:**
• Format: JPG, PNG, WEBP
• Ukuran maks: 5MB
• Resolusi: 640x640 atau lebih tinggi
• Foto harus jelas dan berkualitas

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 **Tips:**
• Gunakan foto dengan resolusi tinggi
• Pastikan foto tidak buram
• Foto akan di-crop otomatis jadi persegi
• Gunakan foto yang represent grup Anda

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Menunggu foto dari Anda...
```

### Result Summary:
```
🎉 **SELESAI!**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 **RINGKASAN**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 **Total Grup:** 5 grup
✅ **Berhasil:** 4 grup
❌ **Gagal:** 1 grup
⏱️ **Delay:** 2 detik/grup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Grup yang Gagal (Batch 1):**

❌ Keluarga Besar 3 (403 forbidden - not admin)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 Apa yang ingin Anda lakukan selanjutnya?

[🖼️ Ganti Lagi] [🔙 Menu Grup]
```

---

## 🔍 WhatsApp API

### Function Used:
```go
client.SetGroupPhoto(ctx, jid, photoBytes)
```

### Parameters:
- `ctx`: Context with timeout (30 seconds)
- `jid`: Group JID (types.JID)
- `photoBytes`: Photo data ([]byte)

### Return:
- `pictureID`: Picture ID if success
- `error`: Error if failed

### Common Errors:
- `403 forbidden` - Bot not admin
- `404 not found` - Group not found
- `timeout` - Network issue
- `invalid JID` - Malformed JID

---

## 📊 COMPARISON

### VS Ambil Link:

| Feature | Ambil Link | Ganti Foto |
|---------|-----------|------------|
| Input | Group name/keyword | Group name/keyword + Photo |
| Output | Group invite links | Photo change confirmation |
| API | `GetGroupInviteLink()` | `SetGroupPhoto()` |
| Requirement | Bot in group | Bot admin in group |
| File handling | No | Yes (temp file) |
| Cleanup | No | Yes (delete temp) |

### Similarities:
- ✅ Same smart search logic
- ✅ Same 3 methods (Manual, List, All)
- ✅ Same delay mechanism
- ✅ Same progress tracking
- ✅ Same result batching
- ✅ Same NO SPAM approach

---

## 🧪 TESTING CHECKLIST

### Test 1: Cari Manual
```
1. /menu → Grup → Ganti Foto
2. Klik "🔍 Cari Manual"
3. Ketik "Test"
4. Ketik delay "2"
5. Kirim foto JPG
6. Verify: Progress bar muncul dan di-edit
7. Verify: Result summary benar
8. Verify: Failed groups (if any) ditampilkan
```

### Test 2: Lihat & Pilih
```
1. Klik "📋 Lihat & Pilih"
2. Ketik "1,3,5"
3. Ketik delay "3"
4. Kirim foto PNG
5. Verify: Processing smooth
6. Verify: Result benar
```

### Test 3: Error Handling
```
1. Cari grup where bot NOT admin
2. Try to change photo
3. Verify: Error tracked correctly
4. Verify: Shows "403 forbidden - not admin"
```

### Test 4: Large Batch
```
1. Select 50+ groups
2. Set delay 3 seconds
3. Send photo
4. Verify: Progress bar works
5. Verify: No spam messages
6. Verify: Failed groups batched (10 per message)
```

### Test 5: Cancel Flow
```
1. Start process
2. Click "❌ Batalkan" at any step
3. Verify: State cleared
4. Verify: Temp file deleted (if any)
```

---

## ✅ CHECKLIST IMPLEMENTASI

- ✅ File `grup_change_photo.go` created (691 lines)
- ✅ State management (`GroupPhotoState`)
- ✅ Smart search integration
- ✅ Photo download & temp file handling
- ✅ WhatsApp API integration (`SetGroupPhoto`)
- ✅ Progress tracking with Edit message
- ✅ Result summary & batching
- ✅ Error handling & cleanup
- ✅ Menu integration in `grup.go`
- ✅ Routing in `telegram.go`
- ✅ Photo handler in `main.go`
- ✅ Build successful (22MB)
- ✅ NO SPAM! (all menus use Edit)

---

## 🚀 READY TO USE!

Program siap digunakan untuk mengganti foto profil grup WhatsApp!

```bash
cd /root/Projel
./bot
```

**Test flow:**
1. `/menu`
2. Klik "👥 Grup"
3. Klik "🖼️ Ganti Foto"
4. Pilih metode
5. Input group name
6. Input delay
7. Kirim foto
8. Lihat hasil!

---

## 📝 NOTES

**Requirements:**
- Bot harus **admin** di grup untuk ganti foto
- Foto max 5MB (Telegram limit)
- Format supported: JPG, PNG, WEBP
- Resolusi recommended: 640x640 atau lebih

**Best Practices:**
- Gunakan delay 2-5 detik untuk avoid rate limit
- Test dengan 1-2 grup dulu sebelum batch besar
- Pastikan bot sudah admin sebelum jalankan
- Gunakan foto berkualitas tinggi

**Known Limitations:**
- Semua grup yang dipilih akan punya foto yang sama
- Tidak bisa undo setelah foto diganti
- Bot must be admin (no workaround)

---

**Status**: ✅ **100% COMPLETE & READY!**  
**Build**: ✅ **SUCCESS (22MB)**  
**Date**: November 1, 2025  
**Lines of Code**: 691 lines

