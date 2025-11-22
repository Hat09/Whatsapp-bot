# 🔒 AUDIT KEAMANAN FITUR LOGIN WHATSAPP BOT
## Login Baru, Daftar Akun, dan Ganti Akun

**Tanggal Audit:** 2025-01-XX  
**Status:** ⚠️ **KRITIS - DITEMUKAN 5 KERENTANAN KEAMANAN**

---

## 📋 RINGKASAN EKSEKUTIF

Audit menyeluruh telah dilakukan pada fitur login WhatsApp bot, termasuk **login baru, daftar akun, dan ganti akun**. Ditemukan **5 kerentanan kritis** yang dapat menyebabkan **kebocoran data dan akses tidak sah antar user**:

1. ⚠️ **SwitchAccount()** - Tidak validasi ownership, user bisa switch ke akun user lain
2. ⚠️ **ShowAccountList()** - Menampilkan SEMUA akun tanpa filter by TelegramID
3. ⚠️ **AddAccount()** - Tidak validasi bahwa akun yang dibuat harus milik user yang memanggil
4. ⚠️ **HandleMultiAccountPhoneInput()** - Cek duplikasi nomor di SEMUA akun, bukan hanya milik user
5. ⚠️ **DeleteAccount()** - Tidak validasi ownership sebelum delete

**Risiko:** User dapat mengakses, switch, dan delete akun WhatsApp milik user lain, melanggar privasi dan keamanan data.

---

## 🔍 TEMUAN AUDIT

### ✅ **YANG SUDAH AMAN**

#### 1. Database Path Generation ✅
- **Lokasi:** `handlers/pairing.go:150`, `handlers/multi_account.go:1272-1273`
- **Status:** ✅ **AMAN**
- **Penjelasan:**
  - Database path menggunakan format: `DB USER TELEGRAM/{telegramID}/whatsmeow-{telegramID}-{phoneNumber}.db`
  - Setiap user memiliki database terpisah berdasarkan TelegramID
  - Path generation sudah benar dan terisolasi per user

#### 2. GetAccountByTelegramID() ✅
- **Lokasi:** `handlers/multi_account.go:490-525`
- **Status:** ✅ **AMAN**
- **Penjelasan:**
  - Fungsi ini sudah benar mencari akun berdasarkan TelegramID dari `BotDataDBPath`
  - Menggunakan regex untuk parse TelegramID dari path database
  - Mendukung format lama dan baru

#### 3. EnsureUserAccountActive() ✅
- **Lokasi:** `handlers/multi_account.go:530-553`
- **Status:** ✅ **AMAN**
- **Penjelasan:**
  - Fungsi ini sudah benar memastikan user hanya mengakses akun miliknya
  - Auto-switch ke akun user jika belum aktif
  - Menggunakan `GetAccountByTelegramID()` untuk validasi

---

### ⚠️ **KERENTANAN KRITIS YANG DITEMUKAN**

#### **KERENTANAN #1: SwitchAccount() - Tidak Validasi Ownership**

**Lokasi File:** `handlers/multi_account.go:1767-1833`

**Fungsi:**
```go
func SwitchAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error
```

**Masalah:**
```go
func SwitchAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
    am := GetAccountManager()
    
    account := am.GetAccount(accountID)
    if account == nil {
        return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
    }
    
    // ❌ TIDAK ADA VALIDASI OWNERSHIP!
    // User bisa switch ke akun user lain dengan hanya mengetahui accountID
    
    // Set sebagai current
    if err := am.SetCurrentAccount(accountID); err != nil {
        return err
    }
    // ... rest of code
}
```

**Risiko:**
- 🔴 **KRITIS:** User dapat switch ke akun WhatsApp milik user lain
- 🔴 User hanya perlu mengetahui `accountID` untuk switch ke akun user lain
- 🔴 Tidak ada validasi bahwa `accountID` tersebut milik `chatID` (TelegramID)
- 🔴 Dampak: **1000+ user dapat saling switch akun satu sama lain**

**Skenario Serangan:**
1. User A (TelegramID: 123) memiliki akun WhatsApp dengan AccountID: 1
2. User B (TelegramID: 456) memiliki akun WhatsApp dengan AccountID: 2
3. User B memanggil `SwitchAccount(1, telegramBot, 456)` dengan AccountID milik User A
4. User B berhasil switch ke akun WhatsApp milik User A
5. User B dapat mengakses semua data WhatsApp milik User A

**Bukti:**
- Tidak ada validasi `GetAccountByTelegramID(chatID)` sebelum switch
- Tidak ada pengecekan bahwa `account.BotDataDBPath` mengandung TelegramID yang sesuai dengan `chatID`

---

#### **KERENTANAN #2: ShowAccountList() - Menampilkan SEMUA Akun**

**Lokasi File:** `handlers/multi_account.go:1558-1661`, `1663-1764`

**Fungsi:**
```go
func ShowAccountList(telegramBot *tgbotapi.BotAPI, chatID int64)
func ShowAccountListEdit(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int)
```

**Masalah:**
```go
func ShowAccountList(telegramBot *tgbotapi.BotAPI, chatID int64) {
    am := GetAccountManager()
    
    // ... reload accounts ...
    
    accounts := am.GetAllAccounts() // ❌ MENGAMBIL SEMUA AKUN, TIDAK FILTER BY TELEGRAMID
    
    // ... menampilkan semua akun ke user ...
    for i, acc := range accounts {
        // User bisa melihat akun user lain!
    }
}
```

**Risiko:**
- 🔴 **KRITIS:** User dapat melihat daftar akun WhatsApp milik **SEMUA USER** di server
- 🔴 User dapat melihat nomor telepon user lain
- 🔴 User dapat melihat AccountID user lain (yang bisa digunakan untuk switch/delete)
- 🔴 Dampak: **1000+ user dapat saling melihat akun satu sama lain**

**Skenario Serangan:**
1. User A memanggil `/multi_account_menu` → `ShowAccountList()`
2. Sistem menampilkan **SEMUA akun** dari semua user (1000+ user)
3. User A melihat AccountID dan nomor telepon milik User B, C, D, dst.
4. User A dapat menggunakan AccountID tersebut untuk switch/delete akun user lain

**Bukti:**
- `GetAllAccounts()` mengembalikan semua akun tanpa filter
- Tidak ada filter berdasarkan TelegramID sebelum menampilkan ke user

---

#### **KERENTANAN #3: AddAccount() - Tidak Validasi Ownership**

**Lokasi File:** `handlers/multi_account.go:233-325`

**Fungsi:**
```go
func (am *AccountManager) AddAccount(phoneNumber, dbPath, botDataDBPath string) (*WhatsAppAccount, error)
```

**Masalah:**
```go
func (am *AccountManager) AddAccount(phoneNumber, dbPath, botDataDBPath string) (*WhatsAppAccount, error) {
    // ... cek limit dan duplikasi ...
    
    // ❌ TIDAK ADA VALIDASI bahwa dbPath dan botDataDBPath mengandung TelegramID yang sesuai
    // ❌ TIDAK ADA PARAMETER telegramID untuk validasi ownership
    
    // Insert ke database
    _, err := db.Exec(`
        INSERT INTO whatsapp_accounts (id, phone_number, db_path, bot_data_db_path, status)
        VALUES (?, ?, ?, ?, 'active')
    `, availableID, phoneNumber, dbPath, botDataDBPath)
    
    // ... rest of code
}
```

**Risiko:**
- 🔴 **KRITIS:** User dapat membuat akun dengan path database yang tidak sesuai dengan TelegramID mereka
- 🔴 Jika ada bug di caller, user bisa membuat akun dengan path milik user lain
- 🔴 Tidak ada validasi bahwa `dbPath` dan `botDataDBPath` mengandung TelegramID yang benar

**Skenario Serangan:**
1. User A memanggil `AddAccount()` dengan `dbPath` yang mengandung TelegramID milik User B
2. Akun dibuat dengan path database milik User B
3. User A dapat mengakses database milik User B

**Bukti:**
- Tidak ada parameter `telegramID` untuk validasi
- Tidak ada pengecekan bahwa `dbPath` dan `botDataDBPath` mengandung TelegramID yang sesuai

---

#### **KERENTANAN #4: HandleMultiAccountPhoneInput() - Cek Duplikasi di SEMUA Akun**

**Lokasi File:** `handlers/multi_account.go:1237-1277`

**Fungsi:**
```go
func HandleMultiAccountPhoneInput(phoneNumber string, chatID int64, telegramBot *tgbotapi.BotAPI)
```

**Masalah:**
```go
func HandleMultiAccountPhoneInput(phoneNumber string, chatID int64, telegramBot *tgbotapi.BotAPI) {
    // ... validasi nomor ...
    
    // Cek apakah nomor sudah terdaftar
    am := GetAccountManager()
    for _, acc := range am.GetAllAccounts() { // ❌ CEK DI SEMUA AKUN, BUKAN HANYA MILIK USER
        if acc.PhoneNumber == phoneNumber {
            errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Nomor %s sudah terdaftar!...", phoneNumber))
            telegramBot.Send(errorMsg)
            return
        }
    }
    
    // ... rest of code
}
```

**Risiko:**
- 🔴 **KRITIS:** User tidak bisa login dengan nomor yang sudah digunakan oleh user lain
- 🔴 Ini sebenarnya bisa jadi fitur (mencegah duplikasi nomor), tapi bisa jadi masalah jika:
  - User A sudah login dengan nomor X
  - User B tidak bisa login dengan nomor X (padahal seharusnya bisa jika nomor X milik User B)
- 🔴 Informasi bocor: User bisa tahu nomor mana yang sudah terdaftar (meskipun tidak tahu milik siapa)

**Skenario:**
1. User A sudah login dengan nomor 628123456789
2. User B mencoba login dengan nomor 628123456789 (yang sebenarnya milik User B)
3. Sistem menolak karena nomor sudah terdaftar (padahal seharusnya User B bisa login dengan nomor miliknya sendiri)

**Catatan:**
- Ini bisa jadi fitur yang diinginkan (mencegah duplikasi nomor di seluruh server)
- Tapi perlu validasi ownership: jika nomor sudah terdaftar, cek apakah milik user yang sama
- Jika milik user yang sama, izinkan (untuk re-login)
- Jika milik user lain, tolak

---

#### **KERENTANAN #5: DeleteAccount() - Tidak Validasi Ownership**

**Lokasi File:** `handlers/multi_account.go:1893-1946`

**Fungsi:**
```go
func DeleteAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error
```

**Masalah:**
```go
func DeleteAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
    am := GetAccountManager()
    
    account := am.GetAccount(accountID)
    if account == nil {
        return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
    }
    
    // ❌ TIDAK ADA VALIDASI OWNERSHIP!
    // User bisa delete akun user lain dengan hanya mengetahui accountID
    
    // ... hapus akun ...
}
```

**Risiko:**
- 🔴 **KRITIS:** User dapat menghapus akun WhatsApp milik user lain
- 🔴 User hanya perlu mengetahui `accountID` untuk delete akun user lain
- 🔴 Tidak ada validasi bahwa `accountID` tersebut milik `chatID` (TelegramID)
- 🔴 Dampak: **1000+ user dapat saling menghapus akun satu sama lain**

**Skenario Serangan:**
1. User A (TelegramID: 123) memiliki akun WhatsApp dengan AccountID: 1
2. User B (TelegramID: 456) memanggil `DeleteAccount(1, telegramBot, 456)` dengan AccountID milik User A
3. User B berhasil menghapus akun WhatsApp milik User A
4. User A kehilangan akses ke akun WhatsApp mereka

**Bukti:**
- Tidak ada validasi `GetAccountByTelegramID(chatID)` sebelum delete
- Tidak ada pengecekan bahwa `account.BotDataDBPath` mengandung TelegramID yang sesuai dengan `chatID`

---

## 🔧 PATCH PERBAIKAN OTOMATIS

### **PATCH #1: Perbaikan SwitchAccount() - Tambahkan Validasi Ownership**

**File:** `handlers/multi_account.go`

**Sebelum (Rawan):**
```go
func SwitchAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
    am := GetAccountManager()
    
    account := am.GetAccount(accountID)
    if account == nil {
        return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
    }
    
    // ❌ TIDAK ADA VALIDASI OWNERSHIP
    // ... rest of code
}
```

**Sesudah (Aman):**
```go
// SwitchAccount mengganti akun aktif dengan validasi ownership
// SECURITY: Validasi bahwa accountID milik chatID (TelegramID) sebelum switch
func SwitchAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
    am := GetAccountManager()
    
    account := am.GetAccount(accountID)
    if account == nil {
        return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
    }
    
    // ✅ AMAN: Validasi ownership - cek apakah akun milik user yang memanggil
    userAccount := am.GetAccountByTelegramID(chatID)
    if userAccount == nil || userAccount.ID != accountID {
        utils.GetLogger().Warn("Security: User %d mencoba switch ke akun %d yang bukan miliknya", chatID, accountID)
        return fmt.Errorf("akses ditolak: akun ini bukan milik Anda")
    }
    
    // ✅ AMAN: Double-check dengan parse TelegramID dari BotDataDBPath
    // Parse TelegramID dari BotDataDBPath untuk memastikan ownership
    reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
    reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)
    
    accountTelegramID := int64(0)
    matchesNew := reNew.FindStringSubmatch(account.BotDataDBPath)
    if len(matchesNew) >= 2 {
        if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
            accountTelegramID = parsedID
        }
    } else {
        matchesOld := reOld.FindStringSubmatch(account.BotDataDBPath)
        if len(matchesOld) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
                accountTelegramID = parsedID
            }
        }
    }
    
    if accountTelegramID != 0 && accountTelegramID != chatID {
        utils.GetLogger().Warn("Security: TelegramID mismatch - User %d mencoba switch ke akun milik User %d", chatID, accountTelegramID)
        return fmt.Errorf("akses ditolak: akun ini bukan milik Anda")
    }
    
    // Set sebagai current
    if err := am.SetCurrentAccount(accountID); err != nil {
        return err
    }
    
    // ... rest of code (sama seperti sebelumnya)
}
```

---

### **PATCH #2: Perbaikan ShowAccountList() - Filter by TelegramID**

**File:** `handlers/multi_account.go`

**Sebelum (Rawan):**
```go
func ShowAccountList(telegramBot *tgbotapi.BotAPI, chatID int64) {
    am := GetAccountManager()
    
    // ... reload accounts ...
    
    accounts := am.GetAllAccounts() // ❌ SEMUA AKUN
    
    // ... menampilkan semua akun ...
}
```

**Sesudah (Aman):**
```go
// ShowAccountList menampilkan daftar akun untuk user tertentu
// SECURITY: Hanya menampilkan akun milik user yang memanggil (filter by TelegramID)
func ShowAccountList(telegramBot *tgbotapi.BotAPI, chatID int64) {
    am := GetAccountManager()
    
    // IMPORTANT: Simpan currentID sebelum reload untuk mencegah reset!
    savedCurrentID := -1
    if currentAcc := am.GetCurrentAccount(); currentAcc != nil {
        savedCurrentID = currentAcc.ID
    }
    
    // Reload accounts dari database untuk memastikan data terbaru
    if err := am.LoadAccounts(); err != nil {
        errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Gagal memuat daftar akun: %v", err))
        telegramBot.Send(errorMsg)
        return
    }
    
    // IMPORTANT: Restore currentID setelah reload jika masih valid!
    if savedCurrentID != -1 {
        if acc := am.GetAccount(savedCurrentID); acc != nil {
            am.SetCurrentAccount(savedCurrentID)
            utils.GetLogger().Info("ShowAccountList: Restored currentID to %d after LoadAccounts", savedCurrentID)
        }
    }
    
    // ✅ AMAN: Filter akun berdasarkan TelegramID user yang memanggil
    allAccounts := am.GetAllAccounts()
    userAccounts := []*WhatsAppAccount{}
    
    for _, acc := range allAccounts {
        // Parse TelegramID dari BotDataDBPath
        reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
        reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)
        
        accountTelegramID := int64(0)
        matchesNew := reNew.FindStringSubmatch(acc.BotDataDBPath)
        if len(matchesNew) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
                accountTelegramID = parsedID
            }
        } else {
            matchesOld := reOld.FindStringSubmatch(acc.BotDataDBPath)
            if len(matchesOld) >= 2 {
                if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
                    accountTelegramID = parsedID
                }
            }
        }
        
        // Hanya tambahkan akun milik user yang memanggil
        if accountTelegramID == chatID {
            userAccounts = append(userAccounts, acc)
        }
    }
    
    currentAccount := am.GetCurrentAccount()
    
    if len(userAccounts) == 0 {
        msg := tgbotapi.NewMessage(chatID, "📭 **BELUM ADA AKUN**\n\nAnda belum memiliki akun WhatsApp yang terdaftar.\n\nGunakan 'Login Baru' untuk menambahkan akun pertama.")
        msg.ParseMode = "Markdown"
        telegramBot.Send(msg)
        return
    }
    
    // ... rest of code menggunakan userAccounts bukan accounts ...
}
```

**Catatan:** Fungsi `ShowAccountListEdit()` juga perlu diperbaiki dengan cara yang sama.

---

### **PATCH #3: Perbaikan AddAccount() - Tambahkan Validasi Ownership**

**File:** `handlers/multi_account.go`

**Sebelum (Rawan):**
```go
func (am *AccountManager) AddAccount(phoneNumber, dbPath, botDataDBPath string) (*WhatsAppAccount, error)
```

**Sesudah (Aman):**
```go
// AddAccount menambahkan akun baru dengan validasi ownership
// SECURITY: Validasi bahwa dbPath dan botDataDBPath mengandung TelegramID yang sesuai
func (am *AccountManager) AddAccount(phoneNumber, dbPath, botDataDBPath string, telegramID int64) (*WhatsAppAccount, error) {
    am.mutex.Lock()
    defer am.mutex.Unlock()
    
    // Cek limit
    if len(am.accounts) >= MaxAccounts {
        return nil, fmt.Errorf("maksimal %d akun WhatsApp telah tercapai", MaxAccounts)
    }
    
    // ✅ AMAN: Validasi bahwa dbPath dan botDataDBPath mengandung TelegramID yang sesuai
    // Parse TelegramID dari dbPath dan botDataDBPath
    reNew := regexp.MustCompile(`whatsmeow-(\d+)-(\d+)\.db`)
    reOld := regexp.MustCompile(`whatsmeow\((\d+)\)>`)
    
    dbPathTelegramID := int64(0)
    matchesNew := reNew.FindStringSubmatch(dbPath)
    if len(matchesNew) >= 2 {
        if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
            dbPathTelegramID = parsedID
        }
    } else {
        matchesOld := reOld.FindStringSubmatch(dbPath)
        if len(matchesOld) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
                dbPathTelegramID = parsedID
            }
        }
    }
    
    botDataDBPathTelegramID := int64(0)
    reNewBotData := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
    reOldBotData := regexp.MustCompile(`bot_data\((\d+)\)>`)
    
    matchesNewBotData := reNewBotData.FindStringSubmatch(botDataDBPath)
    if len(matchesNewBotData) >= 2 {
        if parsedID, err := strconv.ParseInt(matchesNewBotData[1], 10, 64); err == nil {
            botDataDBPathTelegramID = parsedID
        }
    } else {
        matchesOldBotData := reOldBotData.FindStringSubmatch(botDataDBPath)
        if len(matchesOldBotData) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesOldBotData[1], 10, 64); err == nil {
                botDataDBPathTelegramID = parsedID
            }
        }
    }
    
    // Validasi ownership
    if dbPathTelegramID != 0 && dbPathTelegramID != telegramID {
        return nil, fmt.Errorf("akses ditolak: dbPath tidak sesuai dengan TelegramID")
    }
    if botDataDBPathTelegramID != 0 && botDataDBPathTelegramID != telegramID {
        return nil, fmt.Errorf("akses ditolak: botDataDBPath tidak sesuai dengan TelegramID")
    }
    
    // Cek apakah nomor sudah ada (hanya untuk user yang sama)
    for _, acc := range am.accounts {
        if acc.PhoneNumber == phoneNumber {
            // Cek apakah akun ini milik user yang sama
            accTelegramID := int64(0)
            // Parse TelegramID dari BotDataDBPath
            // ... (sama seperti di atas)
            
            if accTelegramID == telegramID {
                return nil, fmt.Errorf("nomor %s sudah terdaftar untuk akun Anda", phoneNumber)
            }
            // Jika milik user lain, izinkan (untuk re-login atau nomor yang sama digunakan user berbeda)
        }
    }
    
    // ... rest of code (sama seperti sebelumnya)
}
```

**Catatan:** Semua caller `AddAccount()` perlu di-update untuk pass `telegramID`:
- `handlers/multi_account.go:1500` - `processMultiAccountPairing()`
- `handlers/pairing.go` - `PairDeviceViaTelegram()`

---

### **PATCH #4: Perbaikan HandleMultiAccountPhoneInput() - Cek Duplikasi Hanya untuk User yang Sama**

**File:** `handlers/multi_account.go`

**Sebelum (Rawan):**
```go
// Cek apakah nomor sudah terdaftar
am := GetAccountManager()
for _, acc := range am.GetAllAccounts() { // ❌ SEMUA AKUN
    if acc.PhoneNumber == phoneNumber {
        errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Nomor %s sudah terdaftar!...", phoneNumber))
        telegramBot.Send(errorMsg)
        return
    }
}
```

**Sesudah (Aman):**
```go
// ✅ AMAN: Cek apakah nomor sudah terdaftar untuk user yang sama
am := GetAccountManager()
allAccounts := am.GetAllAccounts()

for _, acc := range allAccounts {
    if acc.PhoneNumber == phoneNumber {
        // Parse TelegramID dari BotDataDBPath untuk cek ownership
        reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
        reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)
        
        accountTelegramID := int64(0)
        matchesNew := reNew.FindStringSubmatch(acc.BotDataDBPath)
        if len(matchesNew) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
                accountTelegramID = parsedID
            }
        } else {
            matchesOld := reOld.FindStringSubmatch(acc.BotDataDBPath)
            if len(matchesOld) >= 2 {
                if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
                    accountTelegramID = parsedID
                }
            }
        }
        
        // Jika nomor sudah terdaftar untuk user yang sama, tolak
        if accountTelegramID == chatID {
            errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Nomor %s sudah terdaftar untuk akun Anda!\n\nGunakan fitur 'Ganti Akun' untuk menggunakan akun ini.", phoneNumber))
            telegramBot.Send(errorMsg)
            delete(multiAccountLoginStates, chatID)
            return
        }
        // Jika milik user lain, izinkan (untuk re-login atau nomor yang sama digunakan user berbeda)
    }
}
```

---

### **PATCH #5: Perbaikan DeleteAccount() - Tambahkan Validasi Ownership**

**File:** `handlers/multi_account.go`

**Sebelum (Rawan):**
```go
func DeleteAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
    am := GetAccountManager()
    
    account := am.GetAccount(accountID)
    if account == nil {
        return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
    }
    
    // ❌ TIDAK ADA VALIDASI OWNERSHIP
    // ... hapus akun ...
}
```

**Sesudah (Aman):**
```go
// DeleteAccount menghapus akun dengan validasi ownership
// SECURITY: Validasi bahwa accountID milik chatID (TelegramID) sebelum delete
func DeleteAccount(accountID int, telegramBot *tgbotapi.BotAPI, chatID int64) error {
    am := GetAccountManager()
    
    account := am.GetAccount(accountID)
    if account == nil {
        return fmt.Errorf("akun dengan ID %d tidak ditemukan", accountID)
    }
    
    // ✅ AMAN: Validasi ownership - cek apakah akun milik user yang memanggil
    userAccount := am.GetAccountByTelegramID(chatID)
    if userAccount == nil || userAccount.ID != accountID {
        utils.GetLogger().Warn("Security: User %d mencoba delete akun %d yang bukan miliknya", chatID, accountID)
        return fmt.Errorf("akses ditolak: akun ini bukan milik Anda")
    }
    
    // ✅ AMAN: Double-check dengan parse TelegramID dari BotDataDBPath
    reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
    reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)
    
    accountTelegramID := int64(0)
    matchesNew := reNew.FindStringSubmatch(account.BotDataDBPath)
    if len(matchesNew) >= 2 {
        if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
            accountTelegramID = parsedID
        }
    } else {
        matchesOld := reOld.FindStringSubmatch(account.BotDataDBPath)
        if len(matchesOld) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
                accountTelegramID = parsedID
            }
        }
    }
    
    if accountTelegramID != 0 && accountTelegramID != chatID {
        utils.GetLogger().Warn("Security: TelegramID mismatch - User %d mencoba delete akun milik User %d", chatID, accountTelegramID)
        return fmt.Errorf("akses ditolak: akun ini bukan milik Anda")
    }
    
    // ... rest of code (sama seperti sebelumnya)
}
```

---

## 📊 DAFTAR LENGKAP TEMUAN

### **Fungsi yang Sudah Aman ✅**

| No | File | Fungsi | Status |
|---|---|---|---|
| 1 | `handlers/multi_account.go` | `GetAccountByTelegramID()` | ✅ **AMAN** - Sudah benar mencari akun berdasarkan TelegramID |
| 2 | `handlers/multi_account.go` | `EnsureUserAccountActive()` | ✅ **AMAN** - Sudah benar memastikan user hanya mengakses akun miliknya |
| 3 | `handlers/pairing.go` | `PairDeviceViaTelegram()` | ✅ **AMAN** - Database path sudah benar menggunakan TelegramID |
| 4 | `handlers/multi_account.go` | `processMultiAccountPairing()` | ✅ **AMAN** - Database path sudah benar menggunakan TelegramID |

### **Fungsi yang Rawan ⚠️**

| No | File | Fungsi | Risiko | Prioritas |
|---|---|---|---|---|
| 1 | `handlers/multi_account.go` | `SwitchAccount()` | 🔴 **KRITIS** - Tidak validasi ownership | **TINGGI** |
| 2 | `handlers/multi_account.go` | `ShowAccountList()` | 🔴 **KRITIS** - Menampilkan semua akun | **TINGGI** |
| 3 | `handlers/multi_account.go` | `ShowAccountListEdit()` | 🔴 **KRITIS** - Menampilkan semua akun | **TINGGI** |
| 4 | `handlers/multi_account.go` | `AddAccount()` | 🔴 **KRITIS** - Tidak validasi ownership | **TINGGI** |
| 5 | `handlers/multi_account.go` | `HandleMultiAccountPhoneInput()` | 🟡 **MENENGAH** - Cek duplikasi di semua akun | **MENENGAH** |
| 6 | `handlers/multi_account.go` | `DeleteAccount()` | 🔴 **KRITIS** - Tidak validasi ownership | **TINGGI** |

---

## 🔐 REKOMENDASI TAMBAHAN

### 1. **Helper Function untuk Validasi Ownership**

Buat helper function untuk memudahkan validasi ownership di berbagai tempat:

```go
// ValidateAccountOwnership memvalidasi bahwa account milik telegramID
// Return: true jika valid, false jika tidak valid
func ValidateAccountOwnership(account *WhatsAppAccount, telegramID int64) bool {
    if account == nil {
        return false
    }
    
    // Parse TelegramID dari BotDataDBPath
    reNew := regexp.MustCompile(`bot_data-(\d+)-(\d+)\.db`)
    reOld := regexp.MustCompile(`bot_data\((\d+)\)>`)
    
    accountTelegramID := int64(0)
    matchesNew := reNew.FindStringSubmatch(account.BotDataDBPath)
    if len(matchesNew) >= 2 {
        if parsedID, err := strconv.ParseInt(matchesNew[1], 10, 64); err == nil {
            accountTelegramID = parsedID
        }
    } else {
        matchesOld := reOld.FindStringSubmatch(account.BotDataDBPath)
        if len(matchesOld) >= 2 {
            if parsedID, err := strconv.ParseInt(matchesOld[1], 10, 64); err == nil {
                accountTelegramID = parsedID
            }
        }
    }
    
    return accountTelegramID == telegramID
}
```

### 2. **Helper Function untuk Parse TelegramID dari Path**

Buat helper function untuk parse TelegramID dari database path:

```go
// ParseTelegramIDFromPath memparse TelegramID dari database path
// Support format lama dan baru
func ParseTelegramIDFromPath(dbPath string) (int64, error) {
    reNew := regexp.MustCompile(`(?:whatsmeow|bot_data)-(\d+)-(\d+)\.db`)
    reOld := regexp.MustCompile(`(?:whatsmeow|bot_data)\((\d+)\)>`)
    
    matchesNew := reNew.FindStringSubmatch(dbPath)
    if len(matchesNew) >= 2 {
        return strconv.ParseInt(matchesNew[1], 10, 64)
    }
    
    matchesOld := reOld.FindStringSubmatch(dbPath)
    if len(matchesOld) >= 2 {
        return strconv.ParseInt(matchesOld[1], 10, 64)
    }
    
    return 0, fmt.Errorf("tidak dapat parse TelegramID dari path: %s", dbPath)
}
```

### 3. **Audit Log untuk Operasi Kritis**

Tambahkan audit log untuk semua operasi kritis (switch, delete, add account):

```go
// LogAccountOperation mencatat operasi akun untuk audit
func LogAccountOperation(operation string, accountID int, telegramID int64, success bool, errorMsg string) {
    utils.LogActivityWithMetadata(
        fmt.Sprintf("account_%s", operation),
        fmt.Sprintf("User %d melakukan %s pada account %d", telegramID, operation, accountID),
        telegramID,
        map[string]interface{}{
            "account_id": accountID,
            "operation": operation,
            "success": success,
            "error": errorMsg,
        },
        success,
    )
}
```

### 4. **Unit Test untuk Validasi Ownership**

Buat unit test untuk memastikan validasi ownership bekerja dengan benar:

```go
func TestSwitchAccountOwnership(t *testing.T) {
    // Test bahwa user tidak bisa switch ke akun user lain
}

func TestShowAccountListFilter(t *testing.T) {
    // Test bahwa ShowAccountList hanya menampilkan akun milik user
}

func TestDeleteAccountOwnership(t *testing.T) {
    // Test bahwa user tidak bisa delete akun user lain
}
```

---

## ✅ CHECKLIST PERBAIKAN

- [ ] **PATCH #1:** Update `SwitchAccount()` dengan validasi ownership
- [ ] **PATCH #2:** Update `ShowAccountList()` dan `ShowAccountListEdit()` dengan filter by TelegramID
- [ ] **PATCH #3:** Update `AddAccount()` dengan parameter `telegramID` dan validasi ownership
- [ ] **PATCH #4:** Update `HandleMultiAccountPhoneInput()` dengan cek duplikasi hanya untuk user yang sama
- [ ] **PATCH #5:** Update `DeleteAccount()` dengan validasi ownership
- [ ] **UPDATE CALLER:** Update semua caller `AddAccount()` untuk pass `telegramID`
- [ ] **HELPER FUNCTION:** Buat helper function untuk validasi ownership dan parse TelegramID
- [ ] **AUDIT LOG:** Tambahkan audit log untuk operasi kritis
- [ ] **VERIFIKASI:** Test dengan 2+ user untuk memastikan isolasi data
- [ ] **DOKUMENTASI:** Update dokumentasi fungsi yang diubah

---

## 📝 CATATAN PENTING

1. **Database Master:** Tabel `whatsapp_accounts` di `bot_data.db` adalah database master yang digunakan oleh semua user. Ini tidak masalah karena setiap user memiliki database terpisah untuk data mereka sendiri (`bot_data-{telegramID}-{phone}.db`).

2. **AccountID vs TelegramID:** `AccountID` adalah ID unik di database master, sedangkan `TelegramID` adalah ID user Telegram. Setiap akun harus memiliki `BotDataDBPath` yang mengandung `TelegramID` untuk validasi ownership.

3. **Defense in Depth:** Validasi ownership dilakukan di 2 level:
   - Level 1: Cek dengan `GetAccountByTelegramID()` untuk memastikan akun milik user
   - Level 2: Parse `TelegramID` dari `BotDataDBPath` untuk double-check

4. **Backward Compatibility:** Perubahan signature fungsi `AddAccount()` akan mempengaruhi semua caller. Pastikan semua caller di-update.

5. **Testing:** Setelah perbaikan, lakukan testing dengan multiple user untuk memastikan:
   - User A hanya bisa switch/delete akun miliknya
   - User B hanya bisa switch/delete akun miliknya
   - User A tidak bisa melihat akun milik User B
   - Tidak ada data leakage antar user

---

## 🚨 PRIORITAS PERBAIKAN

**PRIORITAS TINGGI (Segera):**
1. ✅ Perbaiki `SwitchAccount()` - Tambahkan validasi ownership
2. ✅ Perbaiki `ShowAccountList()` dan `ShowAccountListEdit()` - Filter by TelegramID
3. ✅ Perbaiki `AddAccount()` - Tambahkan parameter `telegramID` dan validasi ownership
4. ✅ Perbaiki `DeleteAccount()` - Tambahkan validasi ownership

**PRIORITAS MENENGAH:**
1. Perbaiki `HandleMultiAccountPhoneInput()` - Cek duplikasi hanya untuk user yang sama
2. Buat helper function untuk validasi ownership
3. Tambahkan audit log untuk operasi kritis
4. Buat unit test untuk validasi ownership

---

**Status Audit:** ✅ **SELESAI**  
**Total Kerentanan Ditemukan:** 5 (4 Kritis, 1 Menengah)  
**Total Fungsi yang Diperiksa:** 10  
**Total Fungsi yang Aman:** 4  
**Total Fungsi yang Rawan:** 6

---

**Dibuat oleh:** AI-CODER-EXTREME+  
**Tanggal:** 2025-01-XX  
**Versi:** 1.0

