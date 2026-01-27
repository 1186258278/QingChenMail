package cleanup

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"
)

// CleanupResult 清理结果统计
type CleanupResult struct {
	EmailLogs   int64 `json:"email_logs"`   // 清理的发送日志数
	InboxItems  int64 `json:"inbox_items"`  // 清理的收件数
	QueueItems  int64 `json:"queue_items"`  // 清理的队列数
	ForwardLogs int64 `json:"forward_logs"` // 清理的转发日志数
	Attachments int64 `json:"attachments"`  // 清理的附件数
	FreedBytes  int64 `json:"freed_bytes"`  // 释放的磁盘空间 (字节)
	Duration    int64 `json:"duration_ms"`  // 执行耗时 (毫秒)
}

// DataStats 数据统计
type DataStats struct {
	EmailLogs   int64 `json:"email_logs"`
	InboxItems  int64 `json:"inbox_items"`
	QueueItems  int64 `json:"queue_items"`
	ForwardLogs int64 `json:"forward_logs"`
	Attachments int64 `json:"attachments"`
	TotalSize   int64 `json:"total_size"` // 附件总大小 (字节)
}

var (
	cleanupMutex sync.Mutex
	isRunning    bool
	stopChan     chan struct{}
)

// GetStats 获取各表数据量统计
func GetStats() DataStats {
	var stats DataStats

	database.DB.Model(&database.EmailLog{}).Count(&stats.EmailLogs)
	database.DB.Model(&database.Inbox{}).Count(&stats.InboxItems)
	database.DB.Model(&database.EmailQueue{}).Count(&stats.QueueItems)
	database.DB.Model(&database.ForwardLog{}).Count(&stats.ForwardLogs)
	database.DB.Model(&database.AttachmentFile{}).Count(&stats.Attachments)

	// 统计附件总大小
	var totalSize struct {
		Total int64
	}
	database.DB.Model(&database.AttachmentFile{}).Select("COALESCE(SUM(file_size), 0) as total").Scan(&totalSize)
	stats.TotalSize = totalSize.Total

	return stats
}

// RunCleanup 执行数据清理
func RunCleanup() CleanupResult {
	cleanupMutex.Lock()
	if isRunning {
		cleanupMutex.Unlock()
		return CleanupResult{}
	}
	isRunning = true
	cleanupMutex.Unlock()

	defer func() {
		cleanupMutex.Lock()
		isRunning = false
		cleanupMutex.Unlock()
	}()

	startTime := time.Now()
	result := CleanupResult{}

	cfg := config.AppConfig

	log.Printf("[Cleanup] 开始数据清理任务...")

	// 1. 清理发送日志
	if cfg.CleanupEmailLogDays > 0 {
		result.EmailLogs = cleanEmailLogs(cfg.CleanupEmailLogDays)
		log.Printf("[Cleanup] 清理发送日志: %d 条", result.EmailLogs)
	}

	// 2. 清理收件箱
	if cfg.CleanupInboxDays > 0 {
		result.InboxItems = cleanInbox(cfg.CleanupInboxDays)
		log.Printf("[Cleanup] 清理收件箱: %d 条", result.InboxItems)
	}

	// 3. 清理队列记录 (仅清理已完成/失败的)
	if cfg.CleanupQueueDays > 0 {
		result.QueueItems = cleanQueue(cfg.CleanupQueueDays)
		log.Printf("[Cleanup] 清理队列记录: %d 条", result.QueueItems)
	}

	// 4. 清理转发日志
	if cfg.CleanupForwardDays > 0 {
		result.ForwardLogs = cleanForwardLogs(cfg.CleanupForwardDays)
		log.Printf("[Cleanup] 清理转发日志: %d 条", result.ForwardLogs)
	}

	// 5. 清理附件 (同时删除磁盘文件)
	if cfg.CleanupAttachDays > 0 {
		result.Attachments, result.FreedBytes = cleanAttachments(cfg.CleanupAttachDays)
		log.Printf("[Cleanup] 清理附件: %d 个, 释放 %.2f MB", result.Attachments, float64(result.FreedBytes)/1024/1024)
	}

	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Cleanup] 数据清理完成，耗时 %d ms", result.Duration)

	return result
}

// cleanEmailLogs 分批清理发送日志
func cleanEmailLogs(days int) int64 {
	cutoff := time.Now().AddDate(0, 0, -days)
	var total int64

	for {
		// 使用子查询方式分批删除，兼容 SQLite
		var ids []uint
		database.DB.Model(&database.EmailLog{}).
			Where("created_at < ?", cutoff).
			Limit(1000).
			Pluck("id", &ids)

		if len(ids) == 0 {
			break
		}

		result := database.DB.Unscoped().
			Where("id IN ?", ids).
			Delete(&database.EmailLog{})

		total += result.RowsAffected
		time.Sleep(50 * time.Millisecond) // 短暂休息防止锁表
	}

	return total
}

// cleanInbox 分批清理收件箱
func cleanInbox(days int) int64 {
	cutoff := time.Now().AddDate(0, 0, -days)
	var total int64

	for {
		var ids []uint
		database.DB.Model(&database.Inbox{}).
			Where("created_at < ?", cutoff).
			Limit(1000).
			Pluck("id", &ids)

		if len(ids) == 0 {
			break
		}

		result := database.DB.Unscoped().
			Where("id IN ?", ids).
			Delete(&database.Inbox{})

		total += result.RowsAffected
		time.Sleep(50 * time.Millisecond)
	}

	return total
}

// cleanQueue 分批清理已完成/失败的队列记录
func cleanQueue(days int) int64 {
	cutoff := time.Now().AddDate(0, 0, -days)
	var total int64

	for {
		var ids []uint
		database.DB.Model(&database.EmailQueue{}).
			Where("created_at < ? AND status IN ?", cutoff, []string{"completed", "failed"}).
			Limit(1000).
			Pluck("id", &ids)

		if len(ids) == 0 {
			break
		}

		result := database.DB.Unscoped().
			Where("id IN ?", ids).
			Delete(&database.EmailQueue{})

		total += result.RowsAffected
		time.Sleep(50 * time.Millisecond)
	}

	return total
}

// cleanForwardLogs 分批清理转发日志
func cleanForwardLogs(days int) int64 {
	cutoff := time.Now().AddDate(0, 0, -days)
	var total int64

	for {
		var ids []uint
		database.DB.Model(&database.ForwardLog{}).
			Where("created_at < ?", cutoff).
			Limit(1000).
			Pluck("id", &ids)

		if len(ids) == 0 {
			break
		}

		result := database.DB.Unscoped().
			Where("id IN ?", ids).
			Delete(&database.ForwardLog{})

		total += result.RowsAffected
		time.Sleep(50 * time.Millisecond)
	}

	return total
}

// cleanAttachments 清理附件 (同时删除磁盘文件)
func cleanAttachments(days int) (int64, int64) {
	cutoff := time.Now().AddDate(0, 0, -days)
	var freedBytes int64
	var count int64

	// 分批处理附件
	for {
		var files []database.AttachmentFile
		database.DB.Where("created_at < ?", cutoff).Limit(500).Find(&files)

		if len(files) == 0 {
			break
		}

		var ids []uint
		for _, f := range files {
			ids = append(ids, f.ID)

			// 删除磁盘文件
			fullPath := f.FilePath
			if !filepath.IsAbs(fullPath) {
				fullPath = filepath.Join(".", fullPath)
			}

			if info, err := os.Stat(fullPath); err == nil {
				freedBytes += info.Size()
				if err := os.Remove(fullPath); err != nil {
					log.Printf("[Cleanup] 删除文件失败 %s: %v", fullPath, err)
				}
			}
		}

		// 删除数据库记录
		database.DB.Unscoped().Where("id IN ?", ids).Delete(&database.AttachmentFile{})
		count += int64(len(ids))

		time.Sleep(50 * time.Millisecond)
	}

	// 尝试清理空目录
	cleanEmptyDirs("data/attachments")
	cleanEmptyDirs("data/inbox_attachments")

	return count, freedBytes
}

// cleanEmptyDirs 递归清理空目录
func cleanEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			cleanEmptyDirs(subDir)

			// 检查目录是否为空
			subEntries, _ := os.ReadDir(subDir)
			if len(subEntries) == 0 {
				os.Remove(subDir)
			}
		}
	}
}

// StartScheduler 启动定时清理任务
func StartScheduler() {
	stopChan = make(chan struct{})

	go func() {
		// 启动时执行一次清理
		if config.AppConfig.CleanupEnabled {
			log.Println("[Cleanup] 服务启动，执行初始清理...")
			RunCleanup()
		}

		// 计算下次凌晨 3 点的时间
		nextRun := getNextScheduleTime(3, 0)
		log.Printf("[Cleanup] 下次定时清理: %s", nextRun.Format("2006-01-02 15:04:05"))

		timer := time.NewTimer(time.Until(nextRun))
		defer timer.Stop()

		for {
			select {
			case <-stopChan:
				log.Println("[Cleanup] 定时任务已停止")
				return
			case <-timer.C:
				if config.AppConfig.CleanupEnabled {
					log.Println("[Cleanup] 执行定时清理任务...")
					RunCleanup()
				}
				// 重置定时器到下一个凌晨 3 点
				nextRun = getNextScheduleTime(3, 0)
				timer.Reset(time.Until(nextRun))
				log.Printf("[Cleanup] 下次定时清理: %s", nextRun.Format("2006-01-02 15:04:05"))
			}
		}
	}()
}

// StopScheduler 停止定时清理任务
func StopScheduler() {
	if stopChan != nil {
		close(stopChan)
	}
}

// getNextScheduleTime 获取下一个指定时间点
func getNextScheduleTime(hour, minute int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	// 如果今天的时间已过，则设为明天
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}

	return next
}

// IsRunning 检查清理任务是否正在运行
func IsRunning() bool {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()
	return isRunning
}
