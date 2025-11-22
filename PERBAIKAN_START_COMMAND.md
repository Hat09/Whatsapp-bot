# Perbaikan `/start` Command untuk User yang Sudah Login

## Masalah

1. ✅ Saat program pertama kali hidup, program menampilkan menu kepada user yang telah login sebelumnya
2. ❌ Tapi ketika user yang telah login input `/start`, program menampilkan dan meminta user login ulang
3. User menduga program belum dapat membaca db terhadap user yang telah login dan user yang belum login

## Analisis

Masalah terjadi karena:

1. **Pengecekan koneksi terlalu ketat**: `GetUserSession` mengecek `!client.IsConnected()` yang bisa return false sementara meskipun client masih valid dengan `Store.ID != nil`
2. **Tidak menggunakan client dari startup**: Saat user mengirim `/start`, `GetUserSession` membuat client baru dengan `CreateClient` padahal client yang sudah terhubung dari startup masih ada di `AccountManager`
3. **Tidak ada reconnect untuk client yang sudah ada**: Jika client sudah ada tapi belum terhubung, langsung membuat client baru daripada reconnect client yang sudah ada

## Solusi

### 1. Prioritaskan Client yang Sudah Ada dari Startup

**Sebelum:**
```go
if client.Store == nil || client.Store.ID == nil || !client.IsConnected() {
    // Langsung buat client baru
    client, err = am.CreateClient(account.ID)
}
```

**Sesudah:**
```go
if client.Store != nil && client.Store.ID != nil {
    // Client valid, cek apakah perlu reconnect
    if !client.IsConnected() {
        // Coba reconnect client yang sudah ada (lebih efisien)
        if err := client.Connect(); err != nil {
            // Jika reconnect gagal, baru buat client baru
            client, err = am.CreateClient(account.ID)
        }
    }
    // Client sudah terhubung atau berhasil reconnect, gunakan client tersebut
}
```

### 2. Reconnect Client yang Sudah Ada

Jika client sudah ada dan `Store.ID != nil` (berarti account masih login), coba reconnect dulu sebelum membuat client baru:
- Lebih efisien daripada membuat client baru
- Menggunakan client yang sudah terhubung dari startup
- Hanya membuat client baru jika reconnect gagal

### 3. Prioritaskan Store.ID daripada IsConnected()

- `Store.ID != nil` berarti account masih login (data tersimpan di database)
- `IsConnected()` bisa false sementara (misalnya saat reconnect otomatis)
- Jika `Store.ID != nil`, gunakan client tersebut meskipun `IsConnected()` false

## Files Modified

1. **handlers/user_session.go**
   - `GetUserSession()`: Prioritaskan client yang sudah ada dari startup
   - Reconnect client yang sudah ada sebelum membuat client baru
   - Update session yang sudah ada dengan logika yang sama

## Cara Kerja

1. **Startup**: Client dibuat dan terhubung, disimpan di `am.clients[accountID]`
2. **User mengirim `/start`**:
   - `GetUserSession()` dipanggil
   - `am.GetClient(account.ID)` mengembalikan client yang sudah ada dari startup
   - Cek `Store.ID != nil` (berarti account masih login)
   - Jika `!IsConnected()`, coba reconnect client yang sudah ada
   - Jika reconnect berhasil, gunakan client tersebut
   - Jika reconnect gagal, baru buat client baru
3. **Handler mengecek `userClient.Store.ID == nil`**:
   - Jika client sudah terhubung → tampilkan menu utama
   - Jika client `nil` → tampilkan login prompt

## Testing

Untuk memverifikasi perbaikan:

1. ✅ **User yang sudah login mengakses `/start` setelah startup** → Client dari startup digunakan, menu utama ditampilkan
2. ✅ **User yang sudah login mengakses `/start` setelah beberapa saat** → Client reconnect jika perlu, menu utama ditampilkan
3. ✅ **User yang account-nya terblokir** → Client tidak bisa connect, login prompt ditampilkan

## Kesimpulan

Dengan memprioritaskan client yang sudah ada dari startup dan reconnect client yang sudah ada sebelum membuat client baru, user yang sudah login tidak perlu login ulang saat mengirim `/start`. Program akan menggunakan client yang sudah terhubung dari startup atau reconnect client tersebut jika perlu.

