package database

import (
	"time"

	"gorm.io/gorm"
)

// EmailLog 记录每一封发送的邮件
type EmailLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Status    string `json:"status"` // "success" or "failed"
	ErrorMsg  string `json:"error_msg"`
	ClientIP  string `json:"client_ip"`
	Channel    string `json:"channel"` // "direct" or "smtp_config_id"
	CampaignID uint   `json:"campaign_id" gorm:"index"`

	// 追踪字段
	TrackingID   string     `json:"tracking_id" gorm:"index"`
	Opened       bool       `json:"opened"`
	OpenedAt     *time.Time `json:"opened_at"`
	ClickedCount int        `json:"clicked_count"`
	Unsubscribed bool       `json:"unsubscribed"`
}

// EmailQueue 邮件发送队列
type EmailQueue struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	From        string    `json:"from"`
	To          string    `json:"to"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body"`
	Attachments string    `json:"attachments"` // JSON encoded []Attachment
	ChannelID   uint      `json:"channel_id"`
	Status      string    `json:"status" gorm:"index"` // pending, processing, failed, completed
	Retries     int       `json:"retries"`
	NextRetry   time.Time `json:"next_retry" gorm:"index"`
	ErrorMsg    string    `json:"error_msg"`
	CampaignID  uint      `json:"campaign_id" gorm:"index"`
	TrackingID  string    `json:"tracking_id"`              // 预生成的追踪ID
}

// ContactGroup 联系人分组
type ContactGroup struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name        string `json:"name"`
	Description string `json:"description"`
	Count       int64  `json:"count" gorm:"-"` // 动态统计，不存库
}

// Contact 联系人
type Contact struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Email    string `json:"email" gorm:"index"`
	Name     string `json:"name"`
	GroupID  uint   `json:"group_id" gorm:"index"`
	Status   string `json:"status" gorm:"default:'active'"` // active, unsubscribed, bounced
	MetaData string `json:"meta_data"`                      // JSON string for custom fields
}

// Campaign 营销任务
type Campaign struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name       string `json:"name"`
	Subject    string `json:"subject"`
	TemplateID uint   `json:"template_id"` // 可选
	Body       string `json:"body"`        // HTML内容
	SenderID   uint   `json:"sender_id"`   // SMTP Config ID
	SenderName string `json:"sender_name"` // 发件人显示名称

	TargetType    string `json:"target_type"`     // "group" or "manual"
	TargetGroupID uint   `json:"target_group_id"` // 关联的分组ID
	TargetList    string `json:"target_list"`     // 如果是manual，这里存JSON数组字符串

	Status      string     `json:"status"`       // draft, scheduled, processing, completed, paused, failed
	ScheduledAt *time.Time `json:"scheduled_at"` // 计划发送时间

	// 统计快照 (任务完成后更新，或定期更新)
	TotalCount   int `json:"total_count"`
	SentCount    int `json:"sent_count"`
	SuccessCount int `json:"success_count"`
	FailCount    int `json:"fail_count"`
	
	// 进阶统计
	OpenCount        int `json:"open_count"`
	ClickCount       int `json:"click_count"`
	UnsubscribeCount int `json:"unsubscribe_count"`
}

// SMTPConfig 邮件发送通道配置
type SMTPConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	SSL       bool   `json:"ssl"`
	IsDefault bool   `json:"is_default"` // 默认通道
}

// Sender 发件人别名 (可选，用于下拉选择 From 地址)
type Sender struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Email string `json:"email"` // support@example.com
	Name  string `json:"name"`  // "Customer Support"
}

// User 管理员用户
type User struct {
	ID       uint   `gorm:"primaryKey"`
	Username string `gorm:"uniqueIndex"`
	Password string // 简单起见，暂存明文或简单哈希，实际应使用 bcrypt
}

// APIKey API访问密钥
type APIKey struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Key      string     `json:"key" gorm:"uniqueIndex"`
	Name     string     `json:"name"`
	LastUsed *time.Time `json:"last_used"`
}

// Domain 发信域名配置
type Domain struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name           string `json:"name" gorm:"uniqueIndex"` // example.com
	DKIMSelector   string `json:"dkim_selector"`           // default
	DKIMPrivateKey string `json:"dkim_private_key"`        // PEM format
	DKIMPublicKey  string `json:"dkim_public_key"`         // PEM format

	// 高级配置
	MailSubdomainPrefix string `json:"mail_subdomain_prefix"` // e.g., "mail", "smtp", "sec-mail". If empty, use root domain.

	// 验证状态 (缓存)
	SPFVerified   bool `json:"spf_verified"`
	DKIMVerified  bool `json:"dkim_verified"`
	DMARCVerified bool `json:"dmarc_verified"`
	MXVerified    bool `json:"mx_verified"`

	// 关联的 SSL 证书 (用于 STARTTLS)
	CertificateID *uint        `json:"certificate_id" gorm:"index"`
	Certificate   *Certificate `json:"certificate,omitempty" gorm:"foreignKey:CertificateID"`
}

// Certificate SSL证书（独立管理）
type Certificate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name     string `json:"name" gorm:"size:100"`    // 证书名称/备注
	Domains  string `json:"domains" gorm:"size:500"` // 证书包含的域名，逗号分隔 (如 "mail.example.com,*.example.com")
	CertPEM  string `json:"-" gorm:"type:text"`      // 证书内容 (PEM格式，不返回给前端)
	KeyPEM   string `json:"-" gorm:"type:text"`      // 私钥内容 (PEM格式，加密存储，不返回给前端)
	CertPath string `json:"cert_path"`               // 证书文件路径
	KeyPath  string `json:"key_path"`                // 私钥文件路径

	Issuer    string    `json:"issuer" gorm:"size:100"` // 颁发机构 (如 "Let's Encrypt", "DigiCert", "Self-Signed")
	NotBefore time.Time `json:"not_before"`             // 生效时间
	NotAfter  time.Time `json:"not_after"`              // 到期时间

	Source      string `json:"source" gorm:"size:20"`       // 来源: manual / letsencrypt
	AutoRenew   bool   `json:"auto_renew"`                  // 是否自动续期 (仅 letsencrypt)
	DNSProvider string `json:"dns_provider" gorm:"size:30"` // DNS提供商 (manual/cloudflare/aliyun/dnspod)
	DNSConfig   string `json:"-" gorm:"type:text"`          // DNS API配置 (加密存储，不返回给前端)

	// ACME 相关 (Let's Encrypt)
	ACMEAccountKey string `json:"-" gorm:"type:text"` // ACME 账户私钥
	ACMEEmail      string `json:"acme_email"`         // ACME 注册邮箱
}

// Template 邮件模板
type Template struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"` // HTML content
}

// Stats 统计数据结构
type Stats struct {
	TotalSent      int64         `json:"total_sent"`
	TodaySent      int64         `json:"today_sent"`
	SuccessCount   int64         `json:"success_count"`
	FailureCount   int64         `json:"failure_count"`
	LastSentTime   *time.Time    `json:"last_sent_time"`
	Trend          []TrendPoint  `json:"trend"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Time  string `json:"time"`  // 格式 "HH:00"
	Count int64  `json:"count"`
}

// AttachmentFile 附件文件记录 (用于文件管理和留痕)
type AttachmentFile struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Filename    string `json:"filename"`     // 原始文件名
	FilePath    string `json:"file_path"`    // 本地存储路径 (相对路径)
	FileSize    int64  `json:"file_size"`    // 字节数
	ContentType string `json:"content_type"` // MIME 类型
	Source      string `json:"source"`       // "api_base64", "api_url"
	RelatedTo   string `json:"related_to"`   // 关联的收件人或 QueueID (备注)
}

// ForwardRule 邮件转发规则
type ForwardRule struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	DomainID  uint   `json:"domain_id" gorm:"index"`            // 关联的域名ID
	MatchType string `json:"match_type"`                        // "all" (接收所有) / "prefix" (匹配前缀) / "exact" (精确匹配)
	MatchAddr string `json:"match_addr"`                        // 匹配地址，如 "support" 表示 support@domain.com (all模式留空)
	ForwardTo string `json:"forward_to"`                        // 转发目标邮箱，如 "admin@gmail.com"
	Enabled   bool   `json:"enabled" gorm:"default:true"`       // 是否启用
	Remark    string `json:"remark"`                            // 备注
}

// ForwardLog 转发日志
type ForwardLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	RuleID      uint   `json:"rule_id" gorm:"index"`   // 关联的规则ID
	FromAddr    string `json:"from_addr"`              // 原始发件人
	ToAddr      string `json:"to_addr"`                // 原始收件人 (域名邮箱)
	ForwardTo   string `json:"forward_to"`             // 转发到
	Subject     string `json:"subject"`                // 邮件主题
	Status      string `json:"status"`                 // "success" / "failed"
	ErrorMsg    string `json:"error_msg"`              // 错误信息
	RemoteIP    string `json:"remote_ip"`              // 来源IP
}

// Inbox 收件箱
type Inbox struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	FromAddr string `json:"from_addr"`
	ToAddr   string `json:"to_addr"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`      // 存储原始邮件体，或者解析后的正文
	RawData  string `json:"raw_data"`  // 完整原始数据 (可选，用于排查问题)
	IsRead   bool   `json:"is_read"`   // 已读状态
	Tags     string `json:"tags"`      // JSON 标签 (例如 ["reply", "support"])
	RemoteIP string `json:"remote_ip"` // 来源 IP
}

// SchemaVersion 数据库版本控制
type SchemaVersion struct {
	ID        uint      `gorm:"primaryKey"`
	Version   int       `gorm:"uniqueIndex"`
	AppliedAt time.Time
	Description string
}
