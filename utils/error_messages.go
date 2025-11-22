package utils

import (
	"fmt"
	"strings"
)

// ErrorType represents the type of error
type ErrorType int

const (
	ErrorDatabase ErrorType = iota
	ErrorConnection
	ErrorPermission
	ErrorTimeout
	ErrorValidation
	ErrorUnknown
)

// FormatUserError formats error messages in a user-friendly way
func FormatUserError(errType ErrorType, err error, context string) string {
	var icon, title, description, solutions string

	switch errType {
	case ErrorDatabase:
		icon = "💾"
		title = "MASALAH DATABASE"
		description = "Terjadi kesalahan saat mengakses database."
		solutions = `**Solusi:**
• Coba refresh dengan tombol di bawah
• Restart bot jika masalah berlanjut
• Pastikan file database tidak corrupt`

	case ErrorConnection:
		icon = "🔌"
		title = "MASALAH KONEKSI"
		description = "Koneksi ke WhatsApp terputus atau bermasalah."
		solutions = `**Solusi:**
• Periksa koneksi internet Anda
• Coba reconnect dengan /menu
• Tunggu beberapa saat dan coba lagi
• Jika masalah berlanjut, lakukan /logout dan pair ulang`

	case ErrorPermission:
		icon = "🔒"
		title = "AKSES DITOLAK"
		description = "Bot tidak memiliki izin untuk melakukan operasi ini."
		solutions = `**Solusi:**
• Pastikan bot sudah login
• Periksa izin bot di WhatsApp
• Coba logout dan pair ulang`

	case ErrorTimeout:
		icon = "⏱️"
		title = "TIMEOUT"
		description = "Operasi memakan waktu terlalu lama."
		solutions = `**Solusi:**
• Koneksi internet mungkin lambat
• Coba lagi dalam beberapa saat
• Periksa status server WhatsApp
• Gunakan tombol retry di bawah`

	case ErrorValidation:
		icon = "⚠️"
		title = "INPUT TIDAK VALID"
		description = "Data yang Anda masukkan tidak sesuai format."
		solutions = `**Solusi:**
• Periksa kembali format input Anda
• Lihat contoh yang benar
• Gunakan tombol bantuan untuk info lebih lanjut`

	default:
		icon = "❌"
		title = "TERJADI KESALAHAN"
		description = "Terjadi kesalahan yang tidak diketahui."
		solutions = `**Solusi:**
• Coba operasi kembali
• Restart bot jika perlu
• Hubungi admin jika masalah berlanjut`
	}

	// Build error message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("%s **%s**\n\n", icon, title))
	msg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	msg.WriteString(fmt.Sprintf("%s\n\n", description))

	if context != "" {
		msg.WriteString(fmt.Sprintf("**Konteks:** %s\n\n", context))
	}

	if err != nil {
		errorMsg := err.Error()
		// Sanitize technical error messages
		if len(errorMsg) > 100 {
			errorMsg = errorMsg[:100] + "..."
		}
		msg.WriteString(fmt.Sprintf("**Detail:** `%s`\n\n", errorMsg))
	}

	msg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	msg.WriteString(fmt.Sprintf("%s\n", solutions))
	msg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	msg.WriteString("💡 Gunakan /help untuk bantuan lebih lanjut")

	return msg.String()
}

// FormatError is a smart error formatter that auto-detects error type
func FormatError(err error) string {
	if err == nil {
		return "Terjadi kesalahan yang tidak diketahui"
	}

	errStr := err.Error()

	// Detect error type from error message
	if strings.Contains(errStr, "database") || strings.Contains(errStr, "sql") {
		return FormatUserError(ErrorDatabase, err, "")
	}
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "disconnect") {
		return FormatUserError(ErrorConnection, err, "")
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return FormatUserError(ErrorTimeout, err, "")
	}
	if strings.Contains(errStr, "permission") || strings.Contains(errStr, "forbidden") {
		return FormatUserError(ErrorPermission, err, "")
	}

	return FormatUserError(ErrorUnknown, err, "")
}

// SuccessMsg formats success messages
func SuccessMsg(title, description string) string {
	return fmt.Sprintf(`✅ **%s**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, title, description)
}

// InfoMsg formats info messages
func InfoMsg(title, description string) string {
	return fmt.Sprintf(`ℹ️ **%s**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, title, description)
}

// WarningMsg formats warning messages
func WarningMsg(title, description string) string {
	return fmt.Sprintf(`⚠️ **%s**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, title, description)
}
