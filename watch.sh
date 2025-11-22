#!/bin/bash

# File watcher sederhana untuk auto-restart
# Menggunakan inotifywait jika tersedia, atau polling sebagai fallback

echo "🔄 Starting file watcher for auto-restart..."
echo "💡 Edit file .go untuk trigger restart"
echo ""

# Cek apakah inotifywait tersedia
if command -v inotifywait &> /dev/null; then
    echo "✅ Using inotifywait (Linux)"
    while true; do
        go run main.go &
        PID=$!
        
        # Watch untuk perubahan file .go
        inotifywait -e modify,create,delete -r --include='\.go$' . 2>/dev/null
        
        echo ""
        echo "🔄 File changed! Restarting..."
        kill $PID 2>/dev/null
        sleep 1
        pkill -f "go run main.go" 2>/dev/null
        sleep 1
    done
else
    echo "⚠️ inotifywait not found, using polling method"
    echo "📝 Polling every 3 seconds for changes..."
    
    LAST_MODIFIED=$(find . -name "*.go" -type f -exec stat -c %Y {} \; | sort -n | tail -1)
    
    while true; do
        go run main.go &
        PID=$!
        
        # Polling setiap 3 detik
        while kill -0 $PID 2>/dev/null; do
            sleep 3
            CURRENT_MODIFIED=$(find . -name "*.go" -type f -exec stat -c %Y {} \; 2>/dev/null | sort -n | tail -1)
            
            if [ "$CURRENT_MODIFIED" != "$LAST_MODIFIED" ]; then
                echo ""
                echo "🔄 File changed! Restarting..."
                kill $PID 2>/dev/null
                pkill -f "go run main.go" 2>/dev/null
                LAST_MODIFIED=$CURRENT_MODIFIED
                sleep 2
                break
            fi
        done
        
        # Tunggu process benar-benar mati
        wait $PID 2>/dev/null
        sleep 1
    done
fi

