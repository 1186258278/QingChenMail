package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"

	"github.com/google/uuid"
)

// 营销任务处理配置常量
const (
	// CampaignProcessTimeout 单个营销任务处理的最大超时时间
	CampaignProcessTimeout = 30 * time.Minute
	// CampaignBatchSize 每批处理的联系人数量
	CampaignBatchSize = 100
)

// ProcessCampaign 执行营销任务的发送逻辑 (入队)
func ProcessCampaign(campaign *database.Campaign) error {
	if campaign.Status == "processing" || campaign.Status == "completed" {
		return fmt.Errorf("campaign already processed")
	}

	// 1. 获取目标联系人
	var contacts []database.Contact
	if campaign.TargetType == "group" {
		database.DB.Where("group_id = ? AND status = 'active'", campaign.TargetGroupID).Find(&contacts)
	} else if campaign.TargetType == "manual" {
		// Parse JSON list
		var emails []string
		json.Unmarshal([]byte(campaign.TargetList), &emails)
		for _, e := range emails {
			contacts = append(contacts, database.Contact{Email: e})
		}
	}

	if len(contacts) == 0 {
		database.DB.Model(campaign).Update("status", "failed")
		return fmt.Errorf("no contacts found")
	}

	// 2. 获取发件人配置
	var smtpConfig database.SMTPConfig
	if err := database.DB.First(&smtpConfig, campaign.SenderID).Error; err != nil {
		database.DB.Model(campaign).Update("status", "failed")
		return fmt.Errorf("invalid sender configuration")
	}

	// 3. 更新状态并批量创建队列任务
	database.DB.Model(campaign).Updates(map[string]interface{}{
		"status":      "processing",
		"total_count": len(contacts),
		"sent_count":  0,
	})

	// [安全修复] 使用带 context 的 goroutine，支持超时和取消
	ctx, cancel := context.WithTimeout(context.Background(), CampaignProcessTimeout)
	
	go func() {
		// [安全修复] 添加 panic 恢复，防止 goroutine panic 导致程序崩溃
		defer func() {
			cancel() // 确保 context 被取消
			if r := recover(); r != nil {
				log.Printf("[Campaign] Panic recovered in campaign %d: %v", campaign.ID, r)
				database.DB.Model(campaign).Update("status", "failed")
			}
		}()

		for _, contact := range contacts {
			// 检查 context 是否已取消或超时
			select {
			case <-ctx.Done():
				log.Printf("[Campaign] Campaign %d processing cancelled or timed out", campaign.ID)
				database.DB.Model(campaign).Update("status", "failed")
				return
			default:
				// 继续处理
			}

			// Generate Tracking ID
			trackingID := uuid.New().String()

			// [安全修复] 对用户输入进行 HTML 转义，防止 XSS 攻击
			safeName := html.EscapeString(contact.Name)
			safeEmail := html.EscapeString(contact.Email)

			// Replace variables with escaped values
			body := strings.ReplaceAll(campaign.Body, "{name}", safeName)
			body = strings.ReplaceAll(body, "{email}", safeEmail)

			// 注入追踪像素 (Tracking Pixel)
			baseURL := strings.TrimSuffix(config.AppConfig.BaseURL, "/") // 假设 config 中有 BaseURL
			if baseURL == "" {
				baseURL = fmt.Sprintf("http://%s:%s", config.AppConfig.Host, config.AppConfig.Port) // Fallback
			}

			pixel := fmt.Sprintf(`<img src="%s/api/v1/track/open/%s" width="1" height="1" style="display:none;" />`, baseURL, trackingID)
			
			// 注入退订链接 (Unsubscribe Link)
			unsubscribeLink := fmt.Sprintf("%s/api/v1/track/unsubscribe/%s", baseURL, trackingID)
			unsubscribeHTML := fmt.Sprintf(`<br/><br/><hr/><p style="font-size:12px;color:#888;">If you do not wish to receive these emails, <a href="%s">unsubscribe here</a>.</p>`, unsubscribeLink)

			// 如果是 HTML 邮件，在 </body> 前插入
			if strings.Contains(body, "</body>") {
				body = strings.Replace(body, "</body>", pixel+unsubscribeHTML+"</body>", 1)
			} else {
				// 简单的追加
				body = body + pixel + unsubscribeHTML
			}

			// 点击追踪替换 (Click Tracking)
			// 查找所有 <a href="...">
			re := regexp.MustCompile(`(?i)<a\s+[^>]*href=["']([^"']+)["'][^>]*>`)
			body = re.ReplaceAllStringFunc(body, func(match string) string {
				// 提取 URL
				matches := re.FindStringSubmatch(match)
				if len(matches) < 2 {
					return match
				}
				originalURL := matches[1]

				// 跳过退订链接和已经是追踪链接的
				if strings.Contains(originalURL, "/api/v1/track/") {
					return match
				}
				// 仅追踪 http/https
				if !strings.HasPrefix(originalURL, "http") {
					return match
				}

				encodedURL := base64.URLEncoding.EncodeToString([]byte(originalURL))
				trackingURL := fmt.Sprintf("%s/api/v1/track/click/%s?url=%s", baseURL, trackingID, encodedURL)

				// 替换原链接
				return strings.Replace(match, originalURL, trackingURL, 1)
			})

			task := database.EmailQueue{
				From:       smtpConfig.Username, // default from username
				To:         contact.Email,
				Subject:    campaign.Subject,
				Body:       body,
				ChannelID:  smtpConfig.ID,
				Status:     "pending",
				CampaignID: campaign.ID,
				TrackingID: trackingID, // [新增]
			}
			database.DB.Create(&task)
		}
	}()

	return nil
}

// StartCampaignScheduler 启动营销任务调度器
func StartCampaignScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			checkScheduledCampaigns()
		}
	}()
}

func checkScheduledCampaigns() {
	var campaigns []database.Campaign
	now := time.Now()
	
	// 查找状态为 scheduled 且计划时间 <= 当前时间的任务
	if err := database.DB.Where("status = ? AND scheduled_at <= ?", "scheduled", now).Find(&campaigns).Error; err != nil {
		log.Printf("[Scheduler] Error checking campaigns: %v", err)
		return
	}

	for _, campaign := range campaigns {
		log.Printf("[Scheduler] Starting scheduled campaign: %s (ID: %d)", campaign.Name, campaign.ID)
		if err := ProcessCampaign(&campaign); err != nil {
			log.Printf("[Scheduler] Failed to process campaign %d: %v", campaign.ID, err)
		}
	}
}
