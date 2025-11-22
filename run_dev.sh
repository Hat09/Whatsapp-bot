#!/bin/bash

# Script untuk development mode dengan auto-restart
# Menggunakan air jika tersedia, jika tidak menggunakan alternatif sederhana

echo "🚀 Starting WhatsApp Bot in Development Mode..."

# Cek apakah air tersedia
if command -v air &> /dev/null || [ -f ~/go/bin/air ]; then
    echo "✅ Menggunakan Air untuk auto-restart..."
    if [ -f ~/go/bin/air ]; then
        ~/go/bin/air
    else
        air
    fi
else
    echo "⚠️ Air tidak ditemukan, menggunakan mode alternatif..."
    echo "💡 Install Air dengan: go install github.com/cosmtrek/air@latest"
    echo ""
    echo "📝 Menjalankan program (manual restart dengan Ctrl+C)..."
    go run main.go
fi

