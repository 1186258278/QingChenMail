package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"
	"goemail/internal/receiver"

	"github.com/gin-gonic/gin"
)

// 分页限制常量
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100 // 最大分页限制
)

// ListInboxHandler 获取收件箱列表
// GET /api/v1/inbox?page=1&limit=20
func ListInboxHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// 参数校验
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}

	offset := (page - 1) * limit

	var total int64
	var messages []database.Inbox

	query := database.DB.Model(&database.Inbox{})

	// 搜索 (可选)
	if q := c.Query("q"); q != "" {
		query = query.Where("subject LIKE ? OR from_addr LIKE ?", "%"+q+"%", "%"+q+"%")
	}

	query.Count(&total)
	
	if err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inbox"})
		return
	}

	// 简化返回内容，不返回完整的 RawData 以节省带宽
	type InboxSummary struct {
		ID        uint   `json:"id"`
		CreatedAt string `json:"created_at"`
		FromAddr  string `json:"from_addr"`
		ToAddr    string `json:"to_addr"`
		Subject   string `json:"subject"`
		IsRead    bool   `json:"is_read"`
	}

	summary := make([]InboxSummary, len(messages))
	for i, m := range messages {
		summary[i] = InboxSummary{
			ID:        m.ID,
			CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
			FromAddr:  m.FromAddr,
			ToAddr:    m.ToAddr,
			Subject:   m.Subject,
			IsRead:    m.IsRead,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items": summary,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetInboxItemHandler 获取邮件详情
// GET /api/v1/inbox/:id
func GetInboxItemHandler(c *gin.Context) {
	id := c.Param("id")
	var msg database.Inbox
	
	if err := database.DB.First(&msg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	// 标记为已读
	if !msg.IsRead {
		database.DB.Model(&msg).Update("is_read", true)
		msg.IsRead = true
	}

	c.JSON(http.StatusOK, msg)
}

// DeleteInboxItemHandler 删除邮件
// DELETE /api/v1/inbox/:id
func DeleteInboxItemHandler(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&database.Inbox{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// BatchMarkReadHandler 批量标记已读
// POST /api/v1/inbox/batch/read
func BatchMarkReadHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
		All bool   `json:"all"` // 是否标记所有未读
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var affected int64
	var err error

	if req.All {
		result := database.DB.Model(&database.Inbox{}).Where("is_read = ?", false).Update("is_read", true)
		affected = result.RowsAffected
		err = result.Error
	} else if len(req.IDs) > 0 {
		result := database.DB.Model(&database.Inbox{}).Where("id IN ?", req.IDs).Update("is_read", true)
		affected = result.RowsAffected
		err = result.Error
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No IDs provided"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d messages marked as read", affected)})
}

// BatchDeleteHandler 批量删除邮件
// POST /api/v1/inbox/batch/delete
func BatchDeleteHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
		All bool   `json:"all"` // 是否删除所有
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var affected int64
	var err error

	if req.All {
		result := database.DB.Where("1 = 1").Delete(&database.Inbox{})
		affected = result.RowsAffected
		err = result.Error
	} else if len(req.IDs) > 0 {
		result := database.DB.Where("id IN ?", req.IDs).Delete(&database.Inbox{})
		affected = result.RowsAffected
		err = result.Error
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No IDs provided"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d messages deleted", affected)})
}

// GetInboxAttachmentsHandler 获取邮件附件列表
// GET /api/v1/inbox/:id/attachments
func GetInboxAttachmentsHandler(c *gin.Context) {
	id := c.Param("id")
	
	var attachments []database.AttachmentFile
	relatedTo := fmt.Sprintf("inbox:%s", id)
	
	if err := database.DB.Where("related_to = ?", relatedTo).Find(&attachments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch attachments"})
		return
	}

	c.JSON(http.StatusOK, attachments)
}

// GetInboxStatsHandler 获取收件箱统计
// GET /api/v1/inbox/stats
func GetInboxStatsHandler(c *gin.Context) {
	var total, unread int64
	
	database.DB.Model(&database.Inbox{}).Count(&total)
	database.DB.Model(&database.Inbox{}).Where("is_read = ?", false).Count(&unread)

	// 今日收件数
	var todayCount int64
	today := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&database.Inbox{}).Where("created_at >= ?", today).Count(&todayCount)

	c.JSON(http.StatusOK, gin.H{
		"total":       total,
		"unread":      unread,
		"today_count": todayCount,
	})
}

// GetReceiverConfigHandler 获取收件配置
// GET /api/v1/receiver/config
func GetReceiverConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enable_receiver":      config.AppConfig.EnableReceiver,
		"receiver_port":        config.AppConfig.ReceiverPort,
		"receiver_tls":         config.AppConfig.ReceiverTLS,
		"receiver_tls_cert":    config.AppConfig.ReceiverTLSCert,
		"receiver_tls_key":     config.AppConfig.ReceiverTLSKey,
		"receiver_rate_limit":  config.AppConfig.ReceiverRateLimit,
		"receiver_max_msg_size": config.AppConfig.ReceiverMaxMsgSize,
		"receiver_spam_filter": config.AppConfig.ReceiverSpamFilter,
		"receiver_blacklist":   config.AppConfig.ReceiverBlacklist,
		"receiver_require_tls": config.AppConfig.ReceiverRequireTLS,
	})
}

// UpdateReceiverConfigHandler 更新收件配置
// PUT /api/v1/receiver/config
func UpdateReceiverConfigHandler(c *gin.Context) {
	var req struct {
		EnableReceiver     *bool   `json:"enable_receiver"`
		ReceiverPort       *string `json:"receiver_port"`
		ReceiverTLS        *bool   `json:"receiver_tls"`
		ReceiverTLSCert    *string `json:"receiver_tls_cert"`
		ReceiverTLSKey     *string `json:"receiver_tls_key"`
		ReceiverRateLimit  *int    `json:"receiver_rate_limit"`
		ReceiverMaxMsgSize *int    `json:"receiver_max_msg_size"`
		ReceiverSpamFilter *bool   `json:"receiver_spam_filter"`
		ReceiverBlacklist  *string `json:"receiver_blacklist"`
		ReceiverRequireTLS *bool   `json:"receiver_require_tls"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 更新配置
	if req.EnableReceiver != nil {
		config.AppConfig.EnableReceiver = *req.EnableReceiver
	}
	if req.ReceiverPort != nil {
		config.AppConfig.ReceiverPort = *req.ReceiverPort
	}
	if req.ReceiverTLS != nil {
		config.AppConfig.ReceiverTLS = *req.ReceiverTLS
	}
	if req.ReceiverTLSCert != nil {
		config.AppConfig.ReceiverTLSCert = *req.ReceiverTLSCert
	}
	if req.ReceiverTLSKey != nil {
		config.AppConfig.ReceiverTLSKey = *req.ReceiverTLSKey
	}
	if req.ReceiverRateLimit != nil {
		config.AppConfig.ReceiverRateLimit = *req.ReceiverRateLimit
	}
	if req.ReceiverMaxMsgSize != nil {
		config.AppConfig.ReceiverMaxMsgSize = *req.ReceiverMaxMsgSize
	}
	if req.ReceiverSpamFilter != nil {
		config.AppConfig.ReceiverSpamFilter = *req.ReceiverSpamFilter
	}
	if req.ReceiverBlacklist != nil {
		config.AppConfig.ReceiverBlacklist = *req.ReceiverBlacklist
	}
	if req.ReceiverRequireTLS != nil {
		config.AppConfig.ReceiverRequireTLS = *req.ReceiverRequireTLS
	}

	// 保存配置
	if err := config.SaveConfig(config.AppConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	// 重新加载收件配置
	receiver.ReloadConfig()

	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated"})
}
