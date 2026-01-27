package mailer

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"goemail/internal/database"
)

const (
	MaxRetries    = 3
	RetryInterval = 5 * time.Minute // 简单策略：失败后5分钟重试
	WorkerPool    = 5               // 并发 Worker 数量
)

// SendEmailAsync 将邮件请求加入队列
func SendEmailAsync(req SendRequest) (uint, error) {
	// 序列化附件
	attachmentsJSON, err := json.Marshal(req.Attachments)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal attachments: %v", err)
	}

	task := database.EmailQueue{
		From:        req.From,
		To:          req.To,
		Subject:     req.Subject,
		Body:        req.Body,
		Attachments: string(attachmentsJSON),
		ChannelID:   req.ChannelID,
		Status:      "pending",
		Retries:     0,
		NextRetry:   time.Now(),
		TrackingID:  req.TrackingID,
	}

	if err := database.DB.Create(&task).Error; err != nil {
		return 0, err
	}
	return task.ID, nil
}

// StartQueueWorker 启动后台队列处理器
func StartQueueWorker() {
	log.Println("Starting Email Queue Worker...")
	
	// 使用 Ticker 定期轮询
	// 生产环境可能需要更复杂的触发机制（如 Channel 通知），但对于此规模，轮询足够
	ticker := time.NewTicker(2 * time.Second)
	
	go func() {
		for range ticker.C {
			processQueue()
		}
	}()
}

func processQueue() {
	var tasks []database.EmailQueue
	
	// 查找待处理任务：Pending 或 Failed 且到达重试时间
	// 排除暂停中的 Campaign 的任务
	now := time.Now()
	
	// 获取暂停中的 Campaign IDs
	var pausedCampaignIDs []uint
	database.DB.Model(&database.Campaign{}).
		Where("status = 'paused'").
		Pluck("id", &pausedCampaignIDs)
	
	query := database.DB.Where(
		"(status = 'pending') OR (status = 'failed' AND retries < ? AND next_retry <= ?)", 
		MaxRetries, now,
	)
	
	// 排除暂停的 Campaign 的任务
	if len(pausedCampaignIDs) > 0 {
		query = query.Where("campaign_id NOT IN ? OR campaign_id = 0", pausedCampaignIDs)
	}
	
	err := query.Limit(WorkerPool).Find(&tasks).Error

	if err != nil {
		log.Printf("Error fetching queue tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	for _, task := range tasks {
		// 使用原子更新防止竞争条件
		// 只有当 status 仍为 pending/failed 时才更新为 processing
		// 这可以防止多个 worker (如果部署了多个实例) 处理同一任务
		result := database.DB.Model(&database.EmailQueue{}).
			Where("id = ? AND (status = 'pending' OR status = 'failed')", task.ID).
			Update("status", "processing")
		
		if result.RowsAffected == 0 {
			continue // 已经被其他 worker 抢占
		}
		
		// 重新赋值 task 以确保 goroutine 使用正确的数据 (尽管这里已经是拷贝的 task)
		t := task
		go func(t database.EmailQueue) {
			if err := executeTask(t); err != nil {
				// 失败处理
				newRetries := t.Retries + 1
				status := "failed"
				isFinalFailure := false
				if newRetries >= MaxRetries {
					// 超过重试次数，永久失败
					status = "dead"
					isFinalFailure = true
				}
				
				database.DB.Model(&t).Updates(map[string]interface{}{
					"status":     status,
					"retries":    newRetries,
					"next_retry": time.Now().Add(RetryInterval * time.Duration(newRetries)),
					"error_msg":  err.Error(),
				})

				// 只有最终失败（超过重试次数）才计入统计
				if isFinalFailure && t.CampaignID > 0 {
					updateCampaignStats(t.CampaignID, false)
				}
			} else {
				// 成功
				database.DB.Model(&t).Updates(map[string]interface{}{
					"status":    "completed",
					"error_msg": "",
				})

				// 更新 Campaign 统计
				if t.CampaignID > 0 {
					updateCampaignStats(t.CampaignID, true)
				}
			}
		}(t)
	}
}

func executeTask(task database.EmailQueue) error {
	// 反序列化附件
	var attachments []Attachment
	if task.Attachments != "" {
		if err := json.Unmarshal([]byte(task.Attachments), &attachments); err != nil {
			return fmt.Errorf("failed to unmarshal attachments: %v", err)
		}
	}

	req := SendRequest{
		From:        task.From,
		To:          task.To,
		Subject:     task.Subject,
		Body:        task.Body,
		Attachments: attachments,
		ChannelID:   task.ChannelID,
		TrackingID:  task.TrackingID,
	}

	// 调用同步发送逻辑
	return SendEmail(req)
}

// updateCampaignStats 更新营销任务的统计数据
func updateCampaignStats(campaignID uint, success bool) {
	if campaignID == 0 {
		return
	}

	// 根据发送结果更新对应计数
	if success {
		database.DB.Model(&database.Campaign{}).
			Where("id = ?", campaignID).
			UpdateColumn("success_count", database.DB.Raw("success_count + 1"))
	} else {
		database.DB.Model(&database.Campaign{}).
			Where("id = ?", campaignID).
			UpdateColumn("fail_count", database.DB.Raw("fail_count + 1"))
	}

	// 更新已发送计数
	database.DB.Model(&database.Campaign{}).
		Where("id = ?", campaignID).
		UpdateColumn("sent_count", database.DB.Raw("sent_count + 1"))

	// 检查是否所有邮件都已处理完成
	checkCampaignCompletion(campaignID)
}

// checkCampaignCompletion 检查营销任务是否已全部完成
func checkCampaignCompletion(campaignID uint) {
	var campaign database.Campaign
	if err := database.DB.First(&campaign, campaignID).Error; err != nil {
		return
	}

	// 只有 processing 状态的任务才需要检查（paused 状态也不自动完成）
	if campaign.Status != "processing" {
		return
	}

	// 检查队列中是否还有未完成的任务
	// 注意：failed 状态如果还有重试机会，不算完成；dead 状态才是最终失败
	var pendingCount int64
	database.DB.Model(&database.EmailQueue{}).
		Where("campaign_id = ? AND status IN ('pending', 'processing')", campaignID).
		Count(&pendingCount)

	// 检查是否有可重试的失败任务
	var retryableCount int64
	database.DB.Model(&database.EmailQueue{}).
		Where("campaign_id = ? AND status = 'failed' AND retries < ?", campaignID, MaxRetries).
		Count(&retryableCount)

	// 如果没有待处理和可重试的任务，则标记为完成
	if pendingCount == 0 && retryableCount == 0 {
		// 重新获取最新的统计数据
		database.DB.First(&campaign, campaignID)
		database.DB.Model(&campaign).Update("status", "completed")
		log.Printf("[Campaign] Campaign %d completed: total=%d, success=%d, failed=%d",
			campaignID, campaign.TotalCount, campaign.SuccessCount, campaign.FailCount)
	}
}
