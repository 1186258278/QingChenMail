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
	// 注意：并发安全问题。如果是多实例部署，这里需要锁或状态更新的原子性。
	// 单实例部署下，简单的 Update 锁定即可。
	// 这里简化处理：一次取出一批，并在内存中分发给 Worker
	
	now := time.Now()
	err := database.DB.Where(
		"(status = 'pending') OR (status = 'failed' AND retries < ? AND next_retry <= ?)", 
		MaxRetries, now,
	).Limit(WorkerPool).Find(&tasks).Error

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
				if newRetries >= MaxRetries {
					// 超过重试次数，永久失败
					status = "dead" // 或者 keep as failed but max retries reached
				}
				
				database.DB.Model(&t).Updates(map[string]interface{}{
					"status":     status,
					"retries":    newRetries,
					"next_retry": time.Now().Add(RetryInterval * time.Duration(newRetries)), // 指数退避示例
					"error_msg":  err.Error(),
				})
			} else {
				// 成功
				database.DB.Model(&t).Updates(map[string]interface{}{
					"status":    "completed",
					"error_msg": "",
				})
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
	// 注意：SendEmail 内部已经处理了 EmailLog 的写入
	return SendEmail(req)
}
