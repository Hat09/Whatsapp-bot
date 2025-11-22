package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDatabaseWritable memastikan database bisa ditulis
func EnsureDatabaseWritable(dbPath string) error {
	// Cek apakah file exist
	if _, err := os.Stat(dbPath); err == nil {
		// File exist, cek apakah bisa ditulis
		file, err := os.OpenFile(dbPath, os.O_RDWR, 0666)
		if err != nil {
			return fmt.Errorf("tidak bisa membuka database untuk write: %v", err)
		}
		file.Close()
	} else if os.IsNotExist(err) {
		// File tidak exist, pastikan directory bisa ditulis
		dir := filepath.Dir(dbPath)
		if dir == "" || dir == "." {
			dir = "."
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("tidak bisa membuat directory: %v", err)
		}
		// Test write dengan membuat file temp
		testFile := filepath.Join(dir, ".write_test")
		file, err := os.Create(testFile)
		if err != nil {
			return fmt.Errorf("directory tidak bisa ditulis: %v", err)
		}
		file.Close()
		os.Remove(testFile)
	} else {
		return fmt.Errorf("error cek database: %v", err)
	}

	return nil
}
