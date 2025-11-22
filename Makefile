.PHONY: run dev build clean install-air

# Install Air untuk auto-restart
install-air:
	@echo "📦 Installing Air..."
	go install github.com/cosmtrek/air@latest
	@echo "✅ Air installed! Run 'make dev' to start development mode"

# Development mode dengan auto-restart
dev:
	@echo "🚀 Starting in development mode (auto-restart enabled)..."
	@if command -v air &> /dev/null || [ -f ~/go/bin/air ]; then \
		echo "✅ Using Air for auto-restart"; \
		if [ -f ~/go/bin/air ]; then ~/go/bin/air; else air; fi \
	elif command -v inotifywait &> /dev/null || [ -f ./watch.sh ]; then \
		echo "✅ Using file watcher (watch.sh)"; \
		./watch.sh \
	else \
		echo "⚠️ Auto-restart tools tidak ditemukan!"; \
		echo "💡 Install Air: make install-air"; \
		echo "💡 Atau gunakan watch.sh: ./watch.sh"; \
		echo ""; \
		echo "📝 Running in normal mode (no auto-restart)..."; \
		go run main.go; \
	fi

# Run normal (tanpa auto-restart)
run:
	@echo "🚀 Starting WhatsApp Bot..."
	go run main.go

# Build binary
build:
	@echo "🔨 Building binary..."
	go build -o whatsapp-bot main.go
	@echo "✅ Built: ./whatsapp-bot"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf tmp/
	rm -f whatsapp-bot
	rm -f build-errors.log
	@echo "✅ Cleaned!"

