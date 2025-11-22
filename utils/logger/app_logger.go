package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// AppLogger adalah logger terpusat untuk aplikasi
type AppLogger struct {
	prefix string
	debug  bool
}

var globalLogger *AppLogger

// InitLogger menginisialisasi logger global
func InitLogger(debug bool) {
	globalLogger = &AppLogger{
		prefix: "[BOT]",
		debug:  debug,
	}
}

// GetLogger mendapatkan logger global
func GetLogger() *AppLogger {
	if globalLogger == nil {
		globalLogger = &AppLogger{
			prefix: "[BOT]",
			debug:  false,
		}
	}
	return globalLogger
}

// Info menampilkan pesan info
func (l *AppLogger) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s ℹ️  %s", l.prefix, message)
}

// Success menampilkan pesan sukses
func (l *AppLogger) Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s ✅ %s", l.prefix, message)
}

// Error menampilkan pesan error
func (l *AppLogger) Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s ❌ %s", l.prefix, message)
}

// Warn menampilkan pesan warning
func (l *AppLogger) Warn(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s ⚠️  %s", l.prefix, message)
}

// Debug menampilkan pesan debug (hanya jika debug mode)
func (l *AppLogger) Debug(format string, args ...interface{}) {
	if l.debug {
		message := fmt.Sprintf(format, args...)
		log.Printf("%s 🔍 %s", l.prefix, message)
	}
}

// Phase menampilkan pesan phase (untuk startup)
func (l *AppLogger) Phase(phase string) {
	timestamp := time.Now().Format("15:04:05")
	log.Printf("%s 🚀 [%s] %s", l.prefix, timestamp, phase)
}

// Fatal menampilkan pesan fatal dan exit
func (l *AppLogger) Fatal(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s ❌ FATAL: %s", l.prefix, message)
	os.Exit(1)
}

// FilteredLogger adalah wrapper untuk waLog.Logger yang memfilter error non-fatal
type FilteredLogger struct {
	Logger waLog.Logger
}

// Errorf menampilkan error jika bukan error yang di-filter
func (fl *FilteredLogger) Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// Filter error non-fatal yang sering muncul
	filteredErrors := []string{
		"mismatching LTHash",
		"retry receipt",
		"status@broadcast",
		"couldn't find message",
		"app state",
	}

	for _, filtered := range filteredErrors {
		if strings.Contains(strings.ToLower(message), strings.ToLower(filtered)) {
			// Skip logging untuk error yang di-filter
			return
		}
	}

	fl.Logger.Errorf(format, args...)
}

// Warnf menampilkan warning
func (fl *FilteredLogger) Warnf(format string, args ...interface{}) {
	fl.Logger.Warnf(format, args...)
}

// Infof menampilkan info
func (fl *FilteredLogger) Infof(format string, args ...interface{}) {
	fl.Logger.Infof(format, args...)
}

// Debugf menampilkan debug
func (fl *FilteredLogger) Debugf(format string, args ...interface{}) {
	fl.Logger.Debugf(format, args...)
}

// Sub membuat sub-logger
func (fl *FilteredLogger) Sub(module string) waLog.Logger {
	return &FilteredLogger{Logger: fl.Logger.Sub(module)}
}
