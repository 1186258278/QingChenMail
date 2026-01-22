package api

import (
	"encoding/base64"
	"net/http"
	"time"

	"goemail/internal/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TrackOpenHandler 处理邮件打开追踪像素
// GET /api/v1/track/open/:id
func TrackOpenHandler(c *gin.Context) {
	trackingID := c.Param("id")

	// 1. 查找邮件日志
	var log database.EmailLog
	if err := database.DB.Where("tracking_id = ?", trackingID).First(&log).Error; err == nil {
		// 2. 更新打开状态 (如果尚未打开)
		if !log.Opened {
			now := time.Now()
			database.DB.Model(&log).Updates(map[string]interface{}{
				"opened":    true,
				"opened_at": &now,
			})

			// 3. 增加 Campaign 的打开计数
			if log.CampaignID > 0 {
				database.DB.Model(&database.Campaign{ID: log.CampaignID}).
					UpdateColumn("open_count", gorm.Expr("open_count + ?", 1))
			}
		}
	}

	// 4. 返回 1x1 透明 GIF
	c.Header("Content-Type", "image/gif")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	// Base64 decoded 1x1 transparent GIF
	gif, _ := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
	c.Data(http.StatusOK, "image/gif", gif)
}

// UnsubscribeHandler 处理退订请求
// GET /api/v1/track/unsubscribe/:id
func UnsubscribeHandler(c *gin.Context) {
	trackingID := c.Param("id")

	// 1. 查找邮件日志
	var log database.EmailLog
	if err := database.DB.Where("tracking_id = ?", trackingID).First(&log).Error; err != nil {
		c.String(http.StatusNotFound, "Invalid unsubscribe link.")
		return
	}

	// 2. 标记日志为已退订
	if !log.Unsubscribed {
		database.DB.Model(&log).Update("unsubscribed", true)

		// 3. 增加 Campaign 的退订计数
		if log.CampaignID > 0 {
			database.DB.Model(&database.Campaign{ID: log.CampaignID}).
				UpdateColumn("unsubscribe_count", gorm.Expr("unsubscribe_count + ?", 1))
		}

		// 4. 将联系人状态标记为 unsubscribed
		// 注意：EmailLog 中只有 recipient 字符串，我们需要找到对应的 Contact
		var contact database.Contact
		if err := database.DB.Where("email = ?", log.Recipient).First(&contact).Error; err == nil {
			database.DB.Model(&contact).Update("status", "unsubscribed")
		}
	}

	c.String(http.StatusOK, "You have been successfully unsubscribed. We're sorry to see you go.")
}

// TrackClickHandler 处理点击追踪
// GET /api/v1/track/click/:id?url=...
func TrackClickHandler(c *gin.Context) {
	trackingID := c.Param("id")
	targetURL64 := c.Query("url")

	// Base64 URL Decode
	targetURLBytes, err := base64.URLEncoding.DecodeString(targetURL64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid URL")
		return
	}
	targetURL := string(targetURLBytes)

	// 1. 查找日志
	var log database.EmailLog
	if err := database.DB.Where("tracking_id = ?", trackingID).First(&log).Error; err == nil {
		// 2. 增加点击数
		database.DB.Model(&log).UpdateColumn("clicked_count", gorm.Expr("clicked_count + ?", 1))

		// 3. 增加 Campaign 点击数
		if log.CampaignID > 0 {
			database.DB.Model(&database.Campaign{ID: log.CampaignID}).
				UpdateColumn("click_count", gorm.Expr("click_count + ?", 1))
		}
	}

	// 4. 重定向到原始链接
	c.Redirect(http.StatusFound, targetURL)
}
