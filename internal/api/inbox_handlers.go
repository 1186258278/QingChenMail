package api

import (
	"net/http"
	"strconv"

	"goemail/internal/database"

	"github.com/gin-gonic/gin"
)

// 分页限制常量
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100 // [安全修复] 最大分页限制，防止资源滥用
)

// ListInboxHandler 获取收件箱列表
// GET /api/v1/inbox?page=1&limit=20
func ListInboxHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// [安全修复] 参数校验
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
