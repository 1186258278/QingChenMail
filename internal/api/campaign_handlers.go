package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goemail/internal/database"

	"github.com/gin-gonic/gin"
)

// =======================
// Unsubscribed Users Handler
// =======================

// ListUnsubscribedHandler 获取退订用户列表
func ListUnsubscribedHandler(c *gin.Context) {
	page := 1
	pageSize := 50

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	var total int64
	database.DB.Model(&database.Contact{}).Where("status = 'unsubscribed'").Count(&total)

	var contacts []database.Contact
	offset := (page - 1) * pageSize
	database.DB.Where("status = 'unsubscribed'").
		Order("updated_at desc").
		Limit(pageSize).
		Offset(offset).
		Find(&contacts)

	// 获取分组名称
	type ContactWithGroup struct {
		database.Contact
		GroupName string `json:"group_name"`
	}

	results := make([]ContactWithGroup, 0, len(contacts))
	for _, contact := range contacts {
		var group database.ContactGroup
		groupName := ""
		if err := database.DB.First(&group, contact.GroupID).Error; err == nil {
			groupName = group.Name
		}
		results = append(results, ContactWithGroup{
			Contact:   contact,
			GroupName: groupName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"data":  results,
		"page":  page,
	})
}

// ResubscribeHandler 重新订阅
func ResubscribeHandler(c *gin.Context) {
	id := c.Param("id")
	var contact database.Contact
	if err := database.DB.First(&contact, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}

	if contact.Status != "unsubscribed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contact is not unsubscribed"})
		return
	}

	database.DB.Model(&contact).Update("status", "active")
	c.JSON(http.StatusOK, gin.H{"message": "Contact resubscribed"})
}

// =======================
// Contact Group Handlers
// =======================

// ListContactGroups 获取联系人分组列表
func ListContactGroupsHandler(c *gin.Context) {
	var groups []database.ContactGroup
	if err := database.DB.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch groups"})
		return
	}

	// 统计每个组的联系人数量
	for i := range groups {
		var count int64
		database.DB.Model(&database.Contact{}).Where("group_id = ?", groups[i].ID).Count(&count)
		groups[i].Count = count
	}

	c.JSON(http.StatusOK, groups)
}

// CreateContactGroup 创建分组
func CreateContactGroupHandler(c *gin.Context) {
	var group database.ContactGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	if err := database.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}
	c.JSON(http.StatusCreated, group)
}

// UpdateContactGroup 更新分组
func UpdateContactGroupHandler(c *gin.Context) {
	id := c.Param("id")
	var group database.ContactGroup
	if err := database.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	var input database.ContactGroup
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	group.Name = input.Name
	group.Description = input.Description
	
	if err := database.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
		return
	}
	c.JSON(http.StatusOK, group)
}

// DeleteContactGroup 删除分组
func DeleteContactGroupHandler(c *gin.Context) {
	id := c.Param("id")
	
	// 检查是否有联系人
	var count int64
	database.DB.Model(&database.Contact{}).Where("group_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete group with contacts"})
		return
	}

	if err := database.DB.Delete(&database.ContactGroup{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Group deleted"})
}

// =======================
// Contact Handlers
// =======================

// ListContactsHandler 获取联系人列表
func ListContactsHandler(c *gin.Context) {
	groupID := c.Query("group_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	query := database.DB.Model(&database.Contact{})
	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if keyword != "" {
		query = query.Where("email LIKE ? OR name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	query.Count(&total)

	var contacts []database.Contact
	offset := (page - 1) * pageSize
	query.Order("id desc").Limit(pageSize).Offset(offset).Find(&contacts)

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"data":  contacts,
		"page":  page,
	})
}

// CreateContactHandler 创建联系人
func CreateContactHandler(c *gin.Context) {
	var contact database.Contact
	if err := c.ShouldBindJSON(&contact); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Check duplicates in group
	var exists int64
	database.DB.Model(&database.Contact{}).
		Where("group_id = ? AND email = ?", contact.GroupID, contact.Email).
		Count(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Contact already exists in this group"})
		return
	}

	if err := database.DB.Create(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create contact"})
		return
	}
	c.JSON(http.StatusCreated, contact)
}

// ImportContactsHandler 批量导入联系人
func ImportContactsHandler(c *gin.Context) {
	var input struct {
		GroupID int    `json:"group_id"`
		Data    string `json:"data"` // text format: email,name per line
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	lines := strings.Split(input.Data, "\n")
	success := 0
	failed := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		parts := strings.SplitN(line, ",", 2)
		email := strings.TrimSpace(parts[0])
		name := ""
		if len(parts) > 1 {
			name = strings.TrimSpace(parts[1])
		}

		if email == "" {
			failed++
			continue
		}

		// Upsert logic: if exists, update name; else create
		var contact database.Contact
		result := database.DB.Where("group_id = ? AND email = ?", input.GroupID, email).First(&contact)
		
		if result.Error == nil {
			// Update
			if name != "" {
				contact.Name = name
				database.DB.Save(&contact)
			}
		} else {
			// Create
			contact = database.Contact{
				GroupID: uint(input.GroupID),
				Email:   email,
				Name:    name,
				Status:  "active",
			}
			if err := database.DB.Create(&contact).Error; err == nil {
				success++
			} else {
				failed++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Imported %d contacts, failed %d", success, failed),
		"success": success,
		"failed":  failed,
	})
}

// DeleteContactHandler 删除联系人
func DeleteContactHandler(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&database.Contact{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete contact"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Contact deleted"})
}

// UpdateContactHandler 更新联系人
func UpdateContactHandler(c *gin.Context) {
	id := c.Param("id")
	var contact database.Contact
	if err := database.DB.First(&contact, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}

	var input struct {
		Email  string `json:"email"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 检查邮箱是否重复（同分组内）
	if input.Email != "" && input.Email != contact.Email {
		var exists int64
		database.DB.Model(&database.Contact{}).
			Where("group_id = ? AND email = ? AND id != ?", contact.GroupID, input.Email, contact.ID).
			Count(&exists)
		if exists > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists in this group"})
			return
		}
		contact.Email = input.Email
	}

	if input.Name != "" {
		contact.Name = input.Name
	}
	if input.Status != "" {
		contact.Status = input.Status
	}

	if err := database.DB.Save(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update contact"})
		return
	}
	c.JSON(http.StatusOK, contact)
}

// ExportContactsHandler 导出联系人
func ExportContactsHandler(c *gin.Context) {
	groupID := c.Query("group_id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id is required"})
		return
	}

	var contacts []database.Contact
	if err := database.DB.Where("group_id = ?", groupID).Find(&contacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contacts"})
		return
	}

	// 生成 CSV 内容
	var builder strings.Builder
	builder.WriteString("email,name,status\n")
	for _, contact := range contacts {
		// CSV 转义：如果包含逗号、引号或换行，需要用引号包裹
		name := contact.Name
		if strings.ContainsAny(name, ",\"\n") {
			name = "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
		}
		builder.WriteString(fmt.Sprintf("%s,%s,%s\n", contact.Email, name, contact.Status))
	}

	// 获取分组名称用于文件名
	var group database.ContactGroup
	groupName := "contacts"
	if err := database.DB.First(&group, groupID).Error; err == nil {
		groupName = group.Name
	}

	// 设置响应头
	filename := fmt.Sprintf("%s_%s.csv", groupName, time.Now().Format("20060102"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.String(http.StatusOK, builder.String())
}

// BatchDeleteContactsHandler 批量删除联系人
func BatchDeleteContactsHandler(c *gin.Context) {
	var input struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if len(input.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No IDs provided"})
		return
	}

	result := database.DB.Delete(&database.Contact{}, input.IDs)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete contacts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Deleted %d contacts", result.RowsAffected),
		"count":   result.RowsAffected,
	})
}

// =======================
// Campaign Handlers
// =======================

// ListCampaignsHandler 获取营销活动列表
func ListCampaignsHandler(c *gin.Context) {
	var campaigns []database.Campaign
	if err := database.DB.Order("id desc").Find(&campaigns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaigns"})
		return
	}
	c.JSON(http.StatusOK, campaigns)
}

// CreateCampaignHandler 创建营销活动
func CreateCampaignHandler(c *gin.Context) {
	var campaign database.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	campaign.Status = "draft"
	if err := database.DB.Create(&campaign).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
		return
	}
	c.JSON(http.StatusCreated, campaign)
}

// UpdateCampaignHandler 更新营销活动
func UpdateCampaignHandler(c *gin.Context) {
	id := c.Param("id")
	var campaign database.Campaign
	if err := database.DB.First(&campaign, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status != "draft" && campaign.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot edit campaign in current status"})
		return
	}

	var input database.Campaign
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	campaign.Name = input.Name
	campaign.Subject = input.Subject
	campaign.Body = input.Body
	campaign.SenderID = input.SenderID
	campaign.TargetType = input.TargetType
	campaign.TargetGroupID = input.TargetGroupID
	campaign.TargetList = input.TargetList
	campaign.ScheduledAt = input.ScheduledAt
	
	if err := database.DB.Save(&campaign).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update campaign"})
		return
	}
	c.JSON(http.StatusOK, campaign)
}

// StartCampaignHandler 启动营销活动
func StartCampaignHandler(c *gin.Context) {
	id := c.Param("id")
	var campaign database.Campaign
	if err := database.DB.First(&campaign, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status == "processing" || campaign.Status == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign already processed"})
		return
	}

	// 检查是否定时发送
	if campaign.ScheduledAt != nil && campaign.ScheduledAt.After(time.Now()) {
		// 更新状态为 scheduled
		database.DB.Model(&campaign).Update("status", "scheduled")
		c.JSON(http.StatusOK, gin.H{"message": "Campaign scheduled", "scheduled_at": campaign.ScheduledAt})
		return
	}

	// 立即发送
	if err := ProcessCampaign(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaign started", "total_count": campaign.TotalCount})
}

// DeleteCampaignHandler 删除营销活动
func DeleteCampaignHandler(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&database.Campaign{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete campaign"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Campaign deleted"})
}

// PauseCampaignHandler 暂停营销活动
func PauseCampaignHandler(c *gin.Context) {
	id := c.Param("id")
	var campaign database.Campaign
	if err := database.DB.First(&campaign, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status != "processing" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only processing campaigns can be paused"})
		return
	}

	database.DB.Model(&campaign).Update("status", "paused")
	c.JSON(http.StatusOK, gin.H{"message": "Campaign paused"})
}

// ResumeCampaignHandler 恢复营销活动
func ResumeCampaignHandler(c *gin.Context) {
	id := c.Param("id")
	var campaign database.Campaign
	if err := database.DB.First(&campaign, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status != "paused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only paused campaigns can be resumed"})
		return
	}

	database.DB.Model(&campaign).Update("status", "processing")
	c.JSON(http.StatusOK, gin.H{"message": "Campaign resumed"})
}

// GetCampaignProgressHandler 获取营销活动进度
func GetCampaignProgressHandler(c *gin.Context) {
	id := c.Param("id")
	var campaign database.Campaign
	if err := database.DB.First(&campaign, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	// 获取队列中的实时统计
	var pendingCount, processingCount, completedCount, failedCount int64
	database.DB.Model(&database.EmailQueue{}).Where("campaign_id = ? AND status = 'pending'", id).Count(&pendingCount)
	database.DB.Model(&database.EmailQueue{}).Where("campaign_id = ? AND status = 'processing'", id).Count(&processingCount)
	database.DB.Model(&database.EmailQueue{}).Where("campaign_id = ? AND status = 'completed'", id).Count(&completedCount)
	database.DB.Model(&database.EmailQueue{}).Where("campaign_id = ? AND status IN ('failed', 'dead')", id).Count(&failedCount)

	c.JSON(http.StatusOK, gin.H{
		"id":               campaign.ID,
		"status":           campaign.Status,
		"total_count":      campaign.TotalCount,
		"sent_count":       campaign.SentCount,
		"success_count":    campaign.SuccessCount,
		"fail_count":       campaign.FailCount,
		"open_count":       campaign.OpenCount,
		"click_count":      campaign.ClickCount,
		"unsubscribe_count": campaign.UnsubscribeCount,
		"queue": gin.H{
			"pending":    pendingCount,
			"processing": processingCount,
			"completed":  completedCount,
			"failed":     failedCount,
		},
	})
}

// TestCampaignHandler 发送测试邮件
func TestCampaignHandler(c *gin.Context) {
	id := c.Param("id")
	var campaign database.Campaign
	if err := database.DB.First(&campaign, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	var input struct {
		TestEmail string `json:"test_email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test email"})
		return
	}

	// 获取发件人配置
	var smtpConfig database.SMTPConfig
	if err := database.DB.First(&smtpConfig, campaign.SenderID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sender configuration"})
		return
	}

	// 替换变量（使用测试数据）
	body := strings.ReplaceAll(campaign.Body, "{name}", "测试用户")
	body = strings.ReplaceAll(body, "{email}", input.TestEmail)

	// 添加测试标记
	subject := "[测试] " + campaign.Subject

	// 直接发送（不经过队列）
	task := database.EmailQueue{
		From:      smtpConfig.Username,
		To:        input.TestEmail,
		Subject:   subject,
		Body:      body,
		ChannelID: smtpConfig.ID,
		Status:    "pending",
	}

	// 使用 SendEmail 直接发送
	if err := database.DB.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue test email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Test email queued",
		"test_email": input.TestEmail,
	})
}
