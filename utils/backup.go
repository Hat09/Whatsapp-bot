package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BackupAccountDatabase membuat backup database akun sebelum dihapus
func BackupAccountDatabase(accountID int, phoneNumber, dbPath string) string {
	// Buat folder backup jika belum ada
	backupDir := "backup"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		fmt.Printf("[WARNING] Gagal membuat folder backup: %v\n", err)
		return ""
	}

	// Generate nama backup file dengan timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFileName := fmt.Sprintf("account_%s_%d_%s.db", phoneNumber, accountID, timestamp)
	backupPath := filepath.Join(backupDir, backupFileName)

	// Copy database file
	if dbPath != "" {
		sourceFile, err := os.Open(dbPath)
		if err != nil {
			fmt.Printf("[WARNING] Gagal membuka file database untuk backup: %v\n", err)
			return ""
		}
		defer sourceFile.Close()

		destFile, err := os.Create(backupPath)
		if err != nil {
			fmt.Printf("[WARNING] Gagal membuat file backup: %v\n", err)
			return ""
		}
		defer destFile.Close()

		// Copy file
		buf := make([]byte, 1024*1024) // 1MB buffer
		for {
			n, err := sourceFile.Read(buf)
			if err != nil && n == 0 {
				break
			}
			if _, err := destFile.Write(buf[:n]); err != nil {
				fmt.Printf("[WARNING] Gagal menulis backup: %v\n", err)
				return ""
			}
		}

		// Juga backup file -shm dan -wal jika ada
		if _, err := os.Stat(dbPath + "-shm"); err == nil {
			copyFile(dbPath+"-shm", backupPath+"-shm")
		}
		if _, err := os.Stat(dbPath + "-wal"); err == nil {
			copyFile(dbPath+"-wal", backupPath+"-wal")
		}

		return backupPath
	}

	return ""
}

// copyFile menyalin file dari source ke destination
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
