# 🔗 FITUR AMBIL LINK GRUP

## 📋 OVERVIEW

Fitur baru untuk mengambil link undangan dari grup WhatsApp dengan sistem delay yang dapat dikustomisasi untuk menghindari rate limit.

---

## ✨ FITUR UTAMA

### 1. **Smart Group Search**
- Cari grup berdasarkan nama/kata kunci
- Gunakan "." untuk mengambil SEMUA grup
- Pencarian case-insensitive
- Menampilkan jumlah grup yang ditemukan

### 2. **Customizable Delay**
- User tentukan delay sendiri (1-60 detik)
- Rekomendasi otomatis berdasarkan jumlah grup
- Estimasi waktu total proses
- Progress update real-time

### 3. **Batch Processing**
- Proses multiple grup sekaligus
- Progress update setiap 5 grup
- Tracking success/failed
- Hasil detail untuk setiap grup

### 4. **Error Handling**
- Graceful error handling per grup
- Tidak stop proses jika 1 grup gagal
- Detail error message
- Retry suggestions

---

## 🎯 FLOW PENGGUNAAN

### **Step 1: Akses Menu**
```
User: /grup
Bot: Tampilkan menu grup
User: Klik "🔗 Ambil Link"
Bot: Tampilkan info & panduan
```

### **Step 2: Input Nama Grup**
```
User: Klik "🔍 Cari Grup"
Bot: "Masukkan nama grup..."

User input contoh:
- "Keluarga" → Cari grup dengan kata "keluarga"
- "Kerja"     → Cari grup dengan kata "kerja"
- "."         → Ambil SEMUA grup
```

### **Step 3: Pilih Delay**
```
Bot: "Ditemukan X grup. Tentukan delay..."

User input:
- "2" → 2 detik delay per grup
- "3" → 3 detik delay per grup
- "5" → 5 detik delay per grup

Rekomendasi:
• 1-2 detik: < 10 grup
• 2-3 detik: 10-30 grup
• 3-5 detik: > 30 grup
```

### **Step 4: Proses & Hasil**
```
Bot: Memproses dengan progress update
Bot: Tampilkan hasil:
  ✅ Grup Keluarga Besar
     🔗 https://chat.whatsapp.com/xxxxx
  
  ✅ Grup Kerja Tim
     🔗 https://chat.whatsapp.com/yyyyy
  
  ❌ Grup Teman
     Error: Tidak dapat mengambil link
```

---

## 💻 TECHNICAL DETAILS

### **New File: `handlers/grup_link.go`**

#### **Key Functions:**

1. **ShowGetLinkMenu()**
   - Menampilkan menu ambil link
   - Info & panduan penggunaan
   - Tombol aksi

2. **StartGetLinkProcess()**
   - Inisialisasi state
   - Prompt input nama grup

3. **HandleGroupNameInput()**
   - Proses input nama grup
   - Search grup dari database
   - Validasi hasil
   - Prompt delay

4. **HandleDelayInput()**
   - Validasi delay (1-60 detik)
   - Start batch processing

5. **ProcessGetLinks()**
   - Main processing function
   - Loop through groups
   - Get invite link via WhatsApp API
   - Apply delay
   - Progress tracking
   - Error handling per grup

#### **State Management:**
```go
type LinkGrupState struct {
    WaitingForGroupName bool
    WaitingForDelay     bool
    SelectedGroups      []GroupLinkInfo
    Keyword             string
}
```

#### **WhatsApp API Used:**
```go
client.GetGroupInviteLink(ctx, jid, false)
```

---

## 📊 USE CASES

### **Use Case 1: Admin Grup Multi**
**Skenario:** Admin 20 grup, perlu share semua link

**Flow:**
1. Klik "🔗 Ambil Link"
2. Input "." (semua grup)
3. Delay "3" detik
4. Dapat 20 link dalam ~1 menit
5. Copy & share ke user

### **Use Case 2: Organisasi Event**
**Skenario:** Punya 5 grup event, perlu link untuk promosi

**Flow:**
1. Klik "🔗 Ambil Link"
2. Input "Event"
3. Ditemukan 5 grup
4. Delay "2" detik
5. Dapat 5 link untuk dipromosikan

### **Use Case 3: Backup Link**
**Skenario:** Backup semua link grup untuk dokumentasi

**Flow:**
1. Ambil semua link
2. Screenshot hasil
3. Atau export ke file (future feature)

---

## ⚠️ LIMITATIONS & REQUIREMENTS

### **Requirements:**
1. ✅ Bot harus login ke WhatsApp
2. ✅ Bot harus menjadi **ADMIN** di grup
3. ✅ Grup harus sudah terdeteksi (ada di database)
4. ✅ Koneksi internet stabil

### **Limitations:**
1. ❌ Tidak bisa ambil link jika bukan admin
2. ❌ Maksimal delay 60 detik
3. ❌ Timeout per request: 15 detik
4. ⚠️ Rate limit WhatsApp (maka perlu delay)

### **Error Cases:**
- **"Tidak dapat mengambil link"**
  - Bot bukan admin
  - Grup sudah tidak ada
  - API error
  
- **"Timeout"**
  - Koneksi lambat
  - Server WhatsApp sibuk

---

## 🎨 UI/UX FEATURES

### **1. Menu Informatif**
```
🔗 AMBIL LINK GRUP

Fitur ini akan mengambil link undangan dari grup WhatsApp...

📋 Cara Penggunaan:
1️⃣ Masukkan nama grup...
2️⃣ Bot akan mencari grup...
3️⃣ Tentukan delay...
4️⃣ Bot akan mengambil semua link

⚠️ Catatan Penting:
• Bot harus menjadi admin grup...
```

### **2. Progress Updates**
```
⏳ PROGRESS

📊 Diproses: 15/25 grup
✅ Berhasil: 12
❌ Gagal: 3
⏱️ Progress: 60%

Sedang memproses...
```

### **3. Final Results**
```
🎉 PROSES SELESAI!

📊 Ringkasan:
• Total: 25 grup
• Berhasil: 22
• Gagal: 3
• Kata Kunci: "."

Detail Hasil:
✅ Grup A
   🔗 https://chat.whatsapp.com/...
...
```

---

## 🚀 INTEGRATION POINTS

### **Modified Files:**

1. **`handlers/grup.go`**
   - Added "🔗 Ambil Link" button
   - Updated menu description

2. **`handlers/telegram.go`**
   - Added callbacks:
     - `get_link_menu`
     - `start_get_link`
     - `link_example`
     - `cancel_get_link`
   - Updated help text

3. **`main.go`**
   - Added input handler for link feature
   - Check `IsWaitingForLinkInput()`
   - Route to appropriate handler

### **New Callbacks:**
```go
"get_link_menu"    → ShowGetLinkMenu()
"start_get_link"   → StartGetLinkProcess()
"link_example"     → ShowLinkExample()
"cancel_get_link"  → CancelGetLink()
```

---

## 📈 PERFORMANCE

### **Processing Time:**
```
Jumlah Grup | Delay | Estimasi Waktu
------------|-------|----------------
10 grup     | 2s    | ~20 detik
25 grup     | 3s    | ~75 detik
50 grup     | 5s    | ~250 detik (~4 menit)
```

### **Memory Usage:**
- Minimal: State per user (~1KB)
- Groups cached: ~10KB per 100 groups

### **Network:**
- 1 API call per grup
- Bandwidth: ~500 bytes per request

---

## 💡 TIPS & BEST PRACTICES

### **For Users:**
1. ✅ Gunakan kata kunci spesifik untuk hasil akurat
2. ✅ Set delay 2-5 detik untuk safe processing
3. ✅ Proses saat jaringan stabil
4. ✅ Pastikan bot adalah admin di grup
5. ⚠️ Jangan set delay terlalu kecil (< 1s)

### **For Developers:**
1. ✅ Always use context with timeout
2. ✅ Handle errors gracefully per grup
3. ✅ Don't stop entire process on single error
4. ✅ Provide detailed progress updates
5. ✅ Cleanup state after process

---

## 🔮 FUTURE ENHANCEMENTS

### **Planned Features:**
- [ ] Export links to file (TXT/CSV)
- [ ] Filter by admin status
- [ ] Bulk link regeneration
- [ ] Link analytics (expiry, usage)
- [ ] Scheduled link extraction
- [ ] Link history tracking

### **Improvements:**
- [ ] Parallel processing (with rate limit)
- [ ] Resume on failure
- [ ] Link validation
- [ ] Custom link format

---

## 📝 EXAMPLE OUTPUT

### **Success Case:**
```
🎉 PROSES SELESAI!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 Ringkasan:
• Total: 5 grup
• Berhasil: 5
• Gagal: 0
• Kata Kunci: "Keluarga"

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Detail Hasil:

✅ Keluarga Besar
   🔗 https://chat.whatsapp.com/ABC123xyz

✅ Keluarga Kecil
   🔗 https://chat.whatsapp.com/DEF456uvw

✅ Keluarga Extended
   🔗 https://chat.whatsapp.com/GHI789rst

✅ Grup Keluarga 2024
   🔗 https://chat.whatsapp.com/JKL012mno

✅ Family Group
   🔗 https://chat.whatsapp.com/PQR345stu
```

### **Mixed Result Case:**
```
🎉 PROSES SELESAI!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 Ringkasan:
• Total: 10 grup
• Berhasil: 7
• Gagal: 3
• Kata Kunci: "."

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Detail Hasil:

✅ Grup Kerja
   🔗 https://chat.whatsapp.com/...

❌ Grup Lama
   Error: Tidak dapat mengambil link

✅ Grup Teman
   🔗 https://chat.whatsapp.com/...

❌ Grup Tertutup
   Error: Tidak dapat mengambil link

... (dan seterusnya)
```

---

## 🎯 SUMMARY

Fitur **Ambil Link Grup** adalah solusi lengkap untuk:
- ✅ Batch extraction link grup
- ✅ Customizable delay anti rate-limit
- ✅ Real-time progress tracking
- ✅ Robust error handling
- ✅ User-friendly interface
- ✅ Smart group filtering

**Perfect for:** Admin grup, event organizer, community manager, dan siapa saja yang kelola multiple WhatsApp groups.

---

**Version:** 1.0
**Added:** November 1, 2025
**Status:** ✅ Production Ready

