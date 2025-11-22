# Perbaikan Session Reconnect untuk User yang Sudah Login

## Masalah

User yang sudah login dan memiliki database di server diminta login ulang oleh program setelah restart.

## Analisis

Masalah terjadi karena:

1. **Client belum terhubung setelah restart**: Setelah program restart, client yang sudah ada di `AccountManager` mungkin belum terhubung lagi ke WhatsApp
2. **Session menggunakan client yang belum terhubung**: `GetUserSession` menggunakan `am.GetClient()` yang bisa mengembalikan client lama yang belum terhubung
3. **Pengecekan login terlalu ketat**: Handler mengecek `userClient.Store.ID == nil` dan langsung menampilkan login prompt, padahal client hanya perlu reconnect

## Solusi

### 1. Auto-Reconnect di GetUserSession

**Sebelum:**
```go
client := am.GetClient(account.ID)
if client == nil {
    client, err = am.CreateClient(account.ID)
}
// Tidak ada pengecekan apakah client sudah terhubung
```

**Sesudah:**
```go
client := am.GetClient(account.ID)
if client != nil {
    // Cek apakah client masih valid dan terhubung
    if client.Store == nil || client.Store.ID == nil || !client.IsConnected() {
        // Client ada tapi belum terhubung, buat client baru
        client, err = am.CreateClient(account.ID)
    }
} else {
    // Client tidak ada, buat baru
    client, err = am.CreateClient(account.ID)
}
```

### 2. Verifikasi Client Sebelum Menyimpan ke Session

Setelah mendapatkan/membuat client, verifikasi apakah client sudah terhubung:
- Jika client `nil` atau `Store.ID == nil`, set client ke `nil` agar handler tahu user perlu login
- Handler akan menampilkan login prompt jika client `nil`

### 3. Update Session yang Sudah Ada

Saat session sudah ada dan masih valid, update client dengan pengecekan koneksi:
- Jika client belum terhubung, buat client baru dengan `CreateClient`
- `CreateClient` akan otomatis connect ke WhatsApp

## Files Modified

1. **handlers/user_session.go**
   - `GetUserSession()`: Menambahkan pengecekan koneksi dan auto-reconnect
   - Update session yang sudah ada dengan client yang terhubung

## Cara Kerja

1. **User mengakses `/start` atau callback**
2. **`GetUserSession()` dipanggil**
   - Cek apakah session sudah ada
   - Jika ada, cek apakah client sudah terhubung
   - Jika belum terhubung, buat client baru dengan `CreateClient`
3. **Handler mengecek `userClient.Store.ID == nil`**
   - Jika client sudah terhubung, tampilkan menu utama
   - Jika client `nil` atau belum terhubung, tampilkan login prompt

## Testing

Untuk memverifikasi perbaikan:

1. ✅ **User yang sudah login mengakses `/start` setelah restart** → Client otomatis reconnect, menu utama ditampilkan
2. ✅ **User yang sudah login mengakses callback setelah restart** → Client otomatis reconnect, callback berfungsi
3. ✅ **User yang account-nya terblokir** → Client tidak bisa connect, login prompt ditampilkan

## Kesimpulan

Dengan auto-reconnect di `GetUserSession`, user yang sudah login tidak perlu login ulang setelah restart. Program akan otomatis reconnect client ke WhatsApp jika client belum terhubung.

