package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// UserTimezoneInfo menyimpan timezone info untuk setiap user
type UserTimezoneInfo struct {
	Timezone *time.Location
	Country  string
	City     string
	LastSync time.Time
}

// TimezoneCache menyimpan timezone yang sudah dideteksi per user
type TimezoneCache struct {
	users map[int64]*UserTimezoneInfo
	mu    sync.RWMutex
}

var (
	timezoneCache = &TimezoneCache{
		users: make(map[int64]*UserTimezoneInfo),
	}
	cacheDuration = 24 * time.Hour // Cache untuk 24 jam (per user)
)

// GetTimezoneForUser mendapatkan timezone berdasarkan Telegram user ID
// Note: Karena Telegram Bot API tidak menyediakan akses langsung ke IP user,
// fungsi ini akan fetch dari IP server dengan asumsi user berada di lokasi yang sama dengan server
func GetTimezoneForUser(userID int64) (*UserTimezoneInfo, error) {
	timezoneCache.mu.RLock()

	// Cek apakah cache untuk user ini masih valid
	if info, exists := timezoneCache.users[userID]; exists && info != nil {
		if time.Since(info.LastSync) < cacheDuration {
			loc := info.Timezone
			country := info.Country
			city := info.City
			timezoneCache.mu.RUnlock()
			return &UserTimezoneInfo{
				Timezone: loc,
				Country:  country,
				City:     city,
			}, nil
		}
	}
	timezoneCache.mu.RUnlock()

	// Cache invalid atau belum ada, fetch baru dengan write lock
	timezoneCache.mu.Lock()

	// Double check setelah acquire write lock
	if info, exists := timezoneCache.users[userID]; exists && info != nil {
		if time.Since(info.LastSync) < cacheDuration {
			loc := info.Timezone
			country := info.Country
			city := info.City
			timezoneCache.mu.Unlock()
			return &UserTimezoneInfo{
				Timezone: loc,
				Country:  country,
				City:     city,
			}, nil
		}
	}

	// Fetch timezone baru (lock masih dipegang)
	info, err := fetchTimezoneWithLocation()

	// Update cache
	if err == nil {
		timezoneCache.users[userID] = &UserTimezoneInfo{
			Timezone: info.Timezone,
			Country:  info.Country,
			City:     info.City,
			LastSync: time.Now(),
		}
		GetLogger().Info("GetTimezoneForUser: Sukses fetch timezone untuk user %d: %s, %s (%s)", userID, info.City, info.Country, info.Timezone.String())
	} else {
		GetLogger().Warn("GetTimezoneForUser: Gagal fetch timezone untuk user %d: %v", userID, err)
	}
	timezoneCache.mu.Unlock()

	return info, err
}

// fetchTimezoneWithLocation melakukan fetch timezone dan lokasi dari API eksternal
// NOTE: Lock harus dipegang oleh caller
func fetchTimezoneWithLocation() (*UserTimezoneInfo, error) {
	// Gunakan API gratis untuk mendapatkan timezone berdasarkan IP
	// ip-api.com gratis, max 45 requests/min tanpa API key
	url := "http://ip-api.com/json/?fields=status,message,timezone,country,city"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("gagal fetch timezone: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal read response: %w", err)
	}

	var result struct {
		Status   string `json:"status"`
		Message  string `json:"message,omitempty"`
		Timezone string `json:"timezone"`
		Country  string `json:"country"`
		City     string `json:"city"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gagal parse response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	// Parse timezone string ke time.Location
	loc, err := time.LoadLocation(result.Timezone)
	if err != nil {
		return nil, fmt.Errorf("gagal load location: %w", err)
	}

	return &UserTimezoneInfo{
		Timezone: loc,
		Country:  result.Country,
		City:     result.City,
	}, nil
}

// GetCurrentTimeForUser mengembalikan waktu sekarang di timezone user
func GetCurrentTimeForUser(userID int64) (time.Time, error) {
	info, err := GetTimezoneForUser(userID)
	if err != nil {
		// Fallback ke UTC jika gagal
		GetLogger().Warn("Gagal mendapatkan timezone untuk user %d: %v, fallback ke UTC", userID, err)
		return time.Now().UTC(), nil
	}
	return time.Now().In(info.Timezone), nil
}

// GetCurrentTimeForUserSafe mengembalikan waktu sekarang tanpa error (auto-fallback)
func GetCurrentTimeForUserSafe(userID int64) time.Time {
	info, err := GetTimezoneForUser(userID)
	if err != nil {
		// Fallback ke local time jika gagal
		return time.Now()
	}
	return time.Now().In(info.Timezone)
}

// FormatTimeForUser mengembalikan format waktu yang sudah disesuaikan timezone user
func FormatTimeForUser(userID int64, format string) string {
	t, err := GetCurrentTimeForUser(userID)
	if err != nil {
		return time.Now().Format(format)
	}
	return t.Format(format)
}

// FormatTimeForUserSafe mengembalikan format waktu tanpa error handling
func FormatTimeForUserSafe(userID int64, format string) string {
	return GetCurrentTimeForUserSafe(userID).Format(format)
}

// GetLocationForUserSafe mengembalikan info lokasi user (country, city) tanpa error
func GetLocationForUserSafe(userID int64) (country, city string) {
	info, err := GetTimezoneForUser(userID)
	if err != nil {
		return "Unknown", "Unknown"
	}
	return info.Country, info.City
}

// DEPRECATED: Gunakan GetTimezoneForUser atau fungsi per-user lainnya
// GetTimezoneFromIP hanya untuk backward compatibility, akan menggunakan default user ID 0
var deprecatedUserID int64 = 0

func GetTimezoneFromIP() (*time.Location, error) {
	info, err := GetTimezoneForUser(deprecatedUserID)
	if err != nil {
		return nil, err
	}
	return info.Timezone, nil
}

func GetCurrentTime() (time.Time, error) {
	return GetCurrentTimeForUser(deprecatedUserID)
}

func GetCurrentTimeSafe() time.Time {
	return GetCurrentTimeForUserSafe(deprecatedUserID)
}

func FormatTime(format string) string {
	return FormatTimeForUser(deprecatedUserID, format)
}

func FormatTimeSafe(format string) string {
	return FormatTimeForUserSafe(deprecatedUserID, format)
}
