package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// GrupLogger untuk logging operasi grup dengan level yang jelas
type GrupLogger struct {
	logger *log.Logger
	debug  bool
}

var grupLogger *GrupLogger

// InitGrupLogger inisialisasi logger untuk grup
func InitGrupLogger(debug bool) {
	grupLogger = &GrupLogger{
		logger: log.New(os.Stdout, "[GRUP] ", log.LstdFlags|log.Lshortfile),
		debug:  debug,
	}
}

// GetGrupLogger mendapatkan instance logger
func GetGrupLogger() *GrupLogger {
	if grupLogger == nil {
		InitGrupLogger(true) // Default debug mode
	}
	return grupLogger
}

// Debug log untuk debugging (hanya tampil jika debug=true)
func (l *GrupLogger) Debug(format string, args ...interface{}) {
	if l.debug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info log untuk informasi umum
func (l *GrupLogger) Info(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

// Warn log untuk warning
func (l *GrupLogger) Warn(format string, args ...interface{}) {
	l.logger.Printf("[WARN] "+format, args...)
}

// Error log untuk error
func (l *GrupLogger) Error(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

// Progress log khusus untuk progress update
func (l *GrupLogger) Progress(current, total int, updated, failed int) {
	l.Info("Progress: %d/%d grup diproses | Berhasil: %d | Gagal: %d", current, total, updated, failed)
}

// ErrorMsg menerjemahkan error ke pesan user-friendly bahasa Indonesia
func ErrorMsg(err error) string {
	if err == nil {
		return "Tidak ada error"
	}

	errStr := err.Error()

	// Mapping error ke pesan Indonesia yang lebih friendly
	errorMap := map[string]string{
		"context deadline exceeded": "⏱️ Waktu habis. Server WhatsApp tidak merespons. Silakan coba lagi.",
		"websocket":                 "🔌 Koneksi terputus. Pastikan koneksi internet stabil.",
		"not connected":             "❌ Bot WhatsApp belum terhubung. Pastikan bot sudah login.",
		"database":                  "💾 Error database. Silakan restart program.",
		"connection":                "🔌 Masalah koneksi. Cek internet Anda.",
		"timeout":                   "⏱️ Proses terlalu lama. Coba lagi nanti.",
	}

	for key, msg := range errorMap {
		if contains(errStr, key) {
			return msg
		}
	}

	// Default error message
	return fmt.Sprintf("❌ Terjadi error: %s", errStr)
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
