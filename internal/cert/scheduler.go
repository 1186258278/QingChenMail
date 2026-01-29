package cert

import (
	"log"
	"sync"
	"time"

	"goemail/internal/database"
)

var (
	schedulerOnce    sync.Once
	schedulerStop    chan struct{}
	schedulerRunning bool
)

// StartScheduler 启动证书检查调度器
// 每天检查一次证书到期情况，并记录警告日志
func StartScheduler() {
	schedulerOnce.Do(func() {
		schedulerStop = make(chan struct{})
		schedulerRunning = true
		
		go func() {
			log.Println("[CertScheduler] 证书检查调度器已启动")
			
			// 启动时立即检查一次
			checkCertificates()
			
			// 每天凌晨 4:00 检查
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			
			for {
				select {
				case <-schedulerStop:
					log.Println("[CertScheduler] 调度器已停止")
					schedulerRunning = false
					return
				case <-ticker.C:
					// 检查是否是凌晨 4 点附近
					now := time.Now()
					if now.Hour() >= 3 && now.Hour() <= 5 {
						checkCertificates()
					}
				}
			}
		}()
	})
}

// StopScheduler 停止调度器
func StopScheduler() {
	if schedulerRunning && schedulerStop != nil {
		close(schedulerStop)
	}
}

// checkCertificates 检查所有证书的到期情况
func checkCertificates() {
	log.Println("[CertScheduler] 开始检查证书到期情况...")
	
	var certs []database.Certificate
	if err := database.DB.Find(&certs).Error; err != nil {
		log.Printf("[CertScheduler] 查询证书失败: %v", err)
		return
	}
	
	if len(certs) == 0 {
		log.Println("[CertScheduler] 暂无证书需要检查")
		return
	}
	
	now := time.Now()
	var expiredCount, warningCount, criticalCount int
	
	for _, cert := range certs {
		daysLeft := DaysUntilExpiry(cert.NotAfter)
		status := GetExpiryStatus(cert.NotAfter)
		
		switch status {
		case "expired":
			expiredCount++
			log.Printf("[CertScheduler] ⚠️ 证书已过期: ID=%d, 域名=%s, 到期日=%s",
				cert.ID, cert.Domains, cert.NotAfter.Format("2006-01-02"))
		case "critical":
			criticalCount++
			log.Printf("[CertScheduler] 🔴 证书即将过期 (%d天): ID=%d, 域名=%s",
				daysLeft, cert.ID, cert.Domains)
			// 尝试自动续期 (如果是 Let's Encrypt 且启用了自动续期)
			if cert.Source == "letsencrypt" && cert.AutoRenew {
				log.Printf("[CertScheduler] 尝试自动续期证书 ID=%d...", cert.ID)
				// TODO: 实现自动续期
				// 目前需要 DNS API 配置才能自动续期
			}
		case "warning":
			warningCount++
			log.Printf("[CertScheduler] ⚠️ 证书将在 %d 天后过期: ID=%d, 域名=%s",
				daysLeft, cert.ID, cert.Domains)
		}
	}
	
	log.Printf("[CertScheduler] 检查完成: 共 %d 个证书, 已过期 %d, 即将过期(7天内) %d, 警告(30天内) %d",
		len(certs), expiredCount, criticalCount, warningCount)
	
	_ = now // 避免未使用变量警告
}

// GetCertificateSummary 获取证书状态摘要 (用于仪表盘等)
func GetCertificateSummary() map[string]interface{} {
	var certs []database.Certificate
	if err := database.DB.Find(&certs).Error; err != nil {
		return map[string]interface{}{
			"total":    0,
			"valid":    0,
			"warning":  0,
			"critical": 0,
			"expired":  0,
		}
	}
	
	summary := map[string]int{
		"total":    len(certs),
		"valid":    0,
		"warning":  0,
		"critical": 0,
		"expired":  0,
	}
	
	for _, cert := range certs {
		status := GetExpiryStatus(cert.NotAfter)
		summary[status]++
	}
	
	return map[string]interface{}{
		"total":    summary["total"],
		"valid":    summary["valid"],
		"warning":  summary["warning"],
		"critical": summary["critical"],
		"expired":  summary["expired"],
	}
}
