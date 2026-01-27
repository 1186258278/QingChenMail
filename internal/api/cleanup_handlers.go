package api

import (
	"net/http"

	"goemail/internal/cleanup"

	"github.com/gin-gonic/gin"
)

// RunCleanupHandler 手动执行数据清理
func RunCleanupHandler(c *gin.Context) {
	// 检查是否已有清理任务在运行
	if cleanup.IsRunning() {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Cleanup task is already running",
			"running": true,
		})
		return
	}

	// 异步执行清理，但等待结果
	result := cleanup.RunCleanup()

	c.JSON(http.StatusOK, gin.H{
		"message":     "Cleanup completed",
		"email_logs":  result.EmailLogs,
		"inbox_items": result.InboxItems,
		"queue_items": result.QueueItems,
		"forward_logs": result.ForwardLogs,
		"attachments": result.Attachments,
		"freed_bytes": result.FreedBytes,
		"freed_mb":    float64(result.FreedBytes) / 1024 / 1024,
		"duration_ms": result.Duration,
	})
}

// GetCleanupStatusHandler 获取清理任务状态
func GetCleanupStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running": cleanup.IsRunning(),
	})
}
