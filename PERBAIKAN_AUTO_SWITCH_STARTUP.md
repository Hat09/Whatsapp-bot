# 🔧 PERBAIKAN AUTO-SWITCH & AUTO-CONNECT SAAT STARTUP

## 📋 MASALAH YANG DIPERBAIKI

### Problem:
1. **User memiliki 3 akun**, akun ke-3 terblokir
2. **Program kembali ke pairing menu** (gambar 1) padahal masih ada 2 DB aktif di server
3. **Tidak efisien** karena DB aktif masih ada tapi program tidak auto-connect

### Root Cause:
1. Saat startup, program hanya cek `GetCurrentClient()` tanpa mencoba auto-connect
2. Jika `currentClient == nil`, langsung tampilkan pairing menu
3. Tidak ada logic untuk auto-connect ke account aktif yang ada di database
4. Tidak ada logic untuk auto-switch ke account aktif lain saat account terblokir

## ✅ SOLUSI YANG DITERAPKAN

### 1. **Auto-Connect saat Startup** (`core/startup.go`)

**Sebelum:**
```go
currentClient := am.GetCurrentClient()
if currentClient == nil {
    ui.ShowLoginPrompt() // Langsung pairing menu ❌
}
```

**Sesudah:**
```go
currentAccount := am.GetCurrentAccount()

// CRITICAL FIX: Jika ada account aktif tapi client belum dibuat, coba auto-connect
if currentAccount != nil && currentClient == nil {
    // Coba buat client untuk account aktif
    currentClient, err = am.CreateClient(currentAccount.ID)
    
    if err != nil {
        // Account mungkin terblokir, coba cari account aktif lain
        for _, acc := range allAccounts {
            if acc.Status == "active" {
                testClient, testErr := am.CreateClient(acc.ID)
                if testErr == nil {
                    // Auto-switch ke account aktif ini ✅
                    currentClient = testClient
                    am.SetCurrentAccount(acc.ID)
                    break
                }
            }
        }
    }
}
```

### 2. **Auto-Switch saat Account Terblokir** (`core/events.go`)

**Sudah ada fungsi `handleAccountDisconnection()`** yang:
- Deteksi account yang terputus/terblokir
- Cari account aktif lain dari database
- Auto-switch ke account aktif tersebut
- Hapus file database account terblokir

### 3. **Smart UI Display**

**Sebelum:**
```go
if displayClient != nil {
    ShowMainMenu()
} else {
    ShowLoginPrompt() // Langsung pairing menu ❌
}
```

**Sesudah:**
```go
if displayClient != nil {
    ShowMainMenu()
} else {
    // Cek apakah ada account aktif di database
    if hasActiveAccount {
        // Ada account tapi tidak bisa connect (mungkin terblokir semua)
        SendToTelegram("⚠️ Akun terdeteksi tapi tidak dapat terhubung...")
    }
    ShowLoginPrompt() // Hanya jika BENAR-BENAR tidak ada account
}
```

## 🚀 HASIL PERBAIKAN

### Scenario 1: Startup dengan Account Aktif
1. ✅ Program load accounts dari database
2. ✅ Auto-connect ke account aktif pertama
3. ✅ Jika account pertama terblokir, auto-switch ke account aktif berikutnya
4. ✅ Tampilkan dashboard (bukan pairing menu)

### Scenario 2: Account Terblokir saat Runtime
1. ✅ Event `LoggedOut` atau `Disconnected` terdeteksi
2. ✅ `handleAccountDisconnection()` dipanggil
3. ✅ Auto-switch ke account aktif lain
4. ✅ Hapus file database account terblokir
5. ✅ Program tetap berjalan dengan account aktif lain (tidak kembali ke pairing menu)

### Scenario 3: Multiple Accounts dengan Satu Terblokir
1. ✅ Program tetap menggunakan account aktif
2. ✅ Account terblokir dihapus dari database dan file system
3. ✅ Tidak ada interupsi ke pairing menu

## 📁 FILE YANG DIMODIFIKASI

1. **`core/startup.go`**:
   - Tambah logic auto-connect saat startup
   - Tambah logic auto-switch jika account pertama terblokir
   - Tambah smart UI display (cek account aktif sebelum tampilkan pairing menu)

2. **`utils/bot_database.go`**:
   - Auto-create tables saat database path berubah (fix error "no such table: groups")

## 🧪 CARA TESTING

1. **Test Auto-Connect saat Startup:**
   ```bash
   # Pastikan ada 2-3 account aktif di database
   ./bot
   # Expected: Program auto-connect ke account aktif, tampilkan dashboard
   ```

2. **Test Auto-Switch saat Account Terblokir:**
   - Block account aktif yang sedang digunakan
   - Expected: Program auto-switch ke account aktif lain, tidak kembali ke pairing menu

3. **Test Cleanup DB Terblokir:**
   - Block account, tunggu cleanup
   - Expected: File DB account terblokir terhapus, account aktif tetap ada

## ✅ CHECKLIST

- [x] Auto-connect ke account aktif saat startup
- [x] Auto-switch jika account aktif terblokir
- [x] Tidak kembali ke pairing menu jika masih ada account aktif
- [x] Auto-hapus DB account terblokir
- [x] Smart UI display (cek account sebelum tampilkan pairing menu)

