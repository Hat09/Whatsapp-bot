package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"whatsapp-bot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// enrichGroupNamesFromAPIConcurrent mengambil nama grup dengan concurrent API calls
func enrichGroupNamesFromAPIConcurrent(
	client *whatsmeow.Client,
	groups []GroupInfo,
	telegramBot *tgbotapi.BotAPI,
	chatID int64,
	loadingMsgID int,
	progressCallback func(current, total, updated, failed int),
) []GroupInfo {
	if client == nil || !client.IsConnected() {
		return groups
	}

	logger := utils.GetGrupLogger()
	logger.Info("Mengambil nama grup dari API dengan concurrent calls")

	// Hitung berapa grup yang perlu API call
	var needsAPIGroups []int
	for i := range groups {
		groupName := strings.TrimSpace(groups[i].Name)
		needsAPI := groupName == "" ||
			groupName == groups[i].JID.String() ||
			groupName == groups[i].JID.User ||
			(strings.HasPrefix(groupName, "Grup ") && len(groupName) > 5 && isNumeric(strings.TrimSpace(groupName[5:])))

		if needsAPI {
			needsAPIGroups = append(needsAPIGroups, i)
		}
	}

	totalNeedsAPI := len(needsAPIGroups)
	logger.Info("Total grup yang perlu API call: %d dari %d", totalNeedsAPI, len(groups))

	if totalNeedsAPI == 0 {
		return groups
	}

	// Concurrent processing dengan worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, MaxConcurrentAPICalls)
	var mu sync.Mutex
	totalProcessed := 0
	totalUpdated := 0
	totalFailed := 0

	// Progress update goroutine (update setiap 2 detik)
	progressTicker := time.NewTicker(2 * time.Second)
	progressDone := make(chan bool)

	go func() {
		for {
			select {
			case <-progressTicker.C:
				mu.Lock()
				current := totalProcessed
				updated := totalUpdated
				failed := totalFailed
				mu.Unlock()

				if progressCallback != nil && current < totalNeedsAPI {
					progressCallback(current, totalNeedsAPI, updated, failed)
				}

				if current >= totalNeedsAPI {
					progressTicker.Stop()
					progressDone <- true
					return
				}
			case <-progressDone:
				return
			}
		}
	}()

	// Process groups dengan worker pool (dengan batch delay untuk menghindari rate limit)
	batchSize := MaxConcurrentAPICalls * 2 // Process dalam batch

	for batchStart := 0; batchStart < len(needsAPIGroups); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(needsAPIGroups) {
			batchEnd = len(needsAPIGroups)
		}

		// Process batch
		for idx := batchStart; idx < batchEnd; idx++ {
			wg.Add(1)
			semaphore <- struct{}{} // Acquire semaphore

			go func(groupIdx int) {
				defer wg.Done()
				defer func() { <-semaphore }() // Release semaphore

				// Buat context dengan timeout
				ctx, cancel := context.WithTimeout(context.Background(), APICallTimeout)
				defer cancel()

				// Coba ambil nama dari API dengan retry untuk rate limit
				var groupInfo *types.GroupInfo
				var err error
				maxRetries := 3

				for retry := 0; retry < maxRetries; retry++ {
					groupInfo, err = client.GetGroupInfo(ctx, groups[groupIdx].JID)

					if err == nil && groupInfo != nil && groupInfo.Name != "" {
						// Success!
						break
					}

					// Check if rate limit error
					errStr := ""
					if err != nil {
						errStr = err.Error()
					}

					// Jika bukan rate limit atau "not participating", tidak perlu retry
					if !strings.Contains(errStr, "429") &&
						!strings.Contains(errStr, "rate-overlimit") &&
						!strings.Contains(errStr, "rate limit") {
						// Error lain atau tidak participating - tidak perlu retry
						break
					}

					// Rate limit - retry dengan exponential backoff
					if retry < maxRetries-1 {
						backoff := time.Duration(retry+1) * 2 * time.Second
						logger.Debug("Rate limit terdeteksi, retry %d setelah %v", retry+1, backoff)
						time.Sleep(backoff)

						// Buat context baru untuk retry
						ctx, cancel = context.WithTimeout(context.Background(), APICallTimeout)
					}
				}

				mu.Lock()
				totalProcessed++
				mu.Unlock()

				if err == nil && groupInfo != nil && groupInfo.Name != "" {
					groups[groupIdx].Name = groupInfo.Name
					mu.Lock()
					totalUpdated++
					mu.Unlock()
					logger.Debug("Berhasil mengambil nama grup %s: %s", groups[groupIdx].JID.User, groupInfo.Name)
				} else {
					// Jika gagal, gunakan nama default
					if groups[groupIdx].Name == "" || groups[groupIdx].Name == groups[groupIdx].JID.String() || groups[groupIdx].Name == groups[groupIdx].JID.User {
						groups[groupIdx].Name = fmt.Sprintf("Grup %s", groups[groupIdx].JID.User)
					}
					mu.Lock()
					totalFailed++
					mu.Unlock()

					// Hanya log error yang penting (skip "not participating" dan rate limit setelah retry)
					if err != nil {
						errStr := err.Error()
						// Skip log untuk error yang normal/expected
						if !strings.Contains(errStr, "not participating") &&
							!strings.Contains(errStr, "429") &&
							!strings.Contains(errStr, "rate-overlimit") {
							logger.Debug("Gagal mengambil nama grup %s: %v", groups[groupIdx].JID.User, err)
						}
					}
				}

				// Progress update setiap interval
				mu.Lock()
				current := totalProcessed
				updated := totalUpdated
				failed := totalFailed
				mu.Unlock()

				if current%ProgressUpdateInterval == 0 || current == totalNeedsAPI {
					if progressCallback != nil {
						progressCallback(current, totalNeedsAPI, updated, failed)
					}
				}
			}(needsAPIGroups[idx])
		}

		// Wait for batch to complete sebelum batch berikutnya (untuk menghindari rate limit)
		if batchEnd < len(needsAPIGroups) {
			// Wait untuk batch saat ini selesai
			wg.Wait()
			logger.Debug("Batch %d-%d selesai, delay sebelum batch berikutnya", batchStart, batchEnd)
			time.Sleep(BatchDelay)
		}
	}

	// Wait for all remaining goroutines to complete
	wg.Wait()
	progressTicker.Stop()
	progressDone <- true

	// Final progress update
	mu.Lock()
	finalProcessed := totalProcessed
	finalUpdated := totalUpdated
	finalFailed := totalFailed
	mu.Unlock()

	if progressCallback != nil {
		progressCallback(finalProcessed, totalNeedsAPI, finalUpdated, finalFailed)
	}

	logger.Info("Selesai: %d nama grup berhasil diambil dari API, %d gagal", finalUpdated, finalFailed)
	return groups
}

// updateProgressMessage mengupdate pesan loading dengan progress
func updateProgressMessage(telegramBot *tgbotapi.BotAPI, chatID int64, messageID int, current, total, updated, failed int) {
	if current == 0 || total == 0 {
		return
	}

	percentage := (current * 100) / total
	progressBar := generateProgressBar(percentage)

	// Estimasi waktu sisa
	remaining := total - current
	estimatedTime := ""
	if current > 10 {
		// Rata-rata waktu per grup (asumsi 2 detik per grup dengan concurrent calls)
		avgTimePerGroup := 2.0 / MaxConcurrentAPICalls
		estimatedSeconds := int(float64(remaining) * avgTimePerGroup)
		if estimatedSeconds > 60 {
			estimatedTime = fmt.Sprintf("\n⏱️ Estimasi waktu: ~%d menit", estimatedSeconds/60)
		} else {
			estimatedTime = fmt.Sprintf("\n⏱️ Estimasi waktu: ~%d detik", estimatedSeconds)
		}
	}

	progressMsg := fmt.Sprintf(`🔄 **Memuat Daftar Grup WhatsApp**

%s

**Progress:** %d/%d grup diproses (%d%%)%s

**Status:**
✅ Berhasil: %d
❌ Gagal: %d

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏳ Mohon tunggu...`, progressBar, current, total, percentage, estimatedTime, updated, failed)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, progressMsg)
	editMsg.ParseMode = "Markdown"
	telegramBot.Send(editMsg)
}

// generateProgressBar membuat progress bar visual
func generateProgressBar(percentage int) string {
	const barLength = 20
	filled := (percentage * barLength) / 100
	if filled > barLength {
		filled = barLength
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLength-filled)
	return fmt.Sprintf("`%s` %d%%", bar, percentage)
}
