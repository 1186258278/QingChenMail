package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
)

const Version = "v1.2.3"

type Config struct {
	Domain         string `json:"domain"`
	DKIMSelector   string `json:"dkim_selector"`
	DKIMPrivateKey string `json:"dkim_private_key"`
	
	// Web Server Config
	Host           string `json:"host"`            // 监听地址，默认 0.0.0.0
	Port           string `json:"port"`            // 监听端口
	BaseURL        string `json:"base_url"`        // 公网访问地址 (用于生成追踪链接)
	EnableSSL      bool   `json:"enable_ssl"`      // 是否开启 HTTPS
	CertFile       string `json:"cert_file"`       // 证书文件路径
	KeyFile        string `json:"key_file"`        // 私钥文件路径

	// SMTP Receiver Config (邮件接收服务)
	EnableReceiver bool   `json:"enable_receiver"` // 是否启用接收服务
	ReceiverPort   string `json:"receiver_port"`   // SMTP 接收端口，默认 25
	ReceiverTLS    bool   `json:"receiver_tls"`    // 是否启用 STARTTLS
	ReceiverTLSCert string `json:"receiver_tls_cert"` // STARTTLS 证书路径
	ReceiverTLSKey  string `json:"receiver_tls_key"`  // STARTTLS 私钥路径

	// 收件安全配置
	ReceiverRateLimit    int  `json:"receiver_rate_limit"`     // 每 IP 每分钟最大连接数，0 表示不限制
	ReceiverMaxMsgSize   int  `json:"receiver_max_msg_size"`   // 最大邮件大小 (KB)，默认 10240 (10MB)
	ReceiverSpamFilter   bool `json:"receiver_spam_filter"`    // 是否启用垃圾邮件过滤
	ReceiverBlacklist    string `json:"receiver_blacklist"`    // IP 黑名单，逗号分隔
	ReceiverRequireTLS   bool `json:"receiver_require_tls"`    // 是否强制要求 TLS

	// 数据清理配置
	CleanupEnabled      bool `json:"cleanup_enabled"`        // 是否启用自动清理
	CleanupEmailLogDays int  `json:"cleanup_email_log_days"` // 发送日志保留天数
	CleanupInboxDays    int  `json:"cleanup_inbox_days"`     // 收件箱保留天数
	CleanupQueueDays    int  `json:"cleanup_queue_days"`     // 队列记录保留天数
	CleanupForwardDays  int  `json:"cleanup_forward_days"`   // 转发日志保留天数
	CleanupAttachDays   int  `json:"cleanup_attach_days"`    // 附件保留天数

	// 自动更新配置
	AutoUpdateEnabled  bool   `json:"auto_update_enabled"`  // 是否启用自动更新
	AutoUpdateInterval int    `json:"auto_update_interval"` // 检查间隔（小时），默认 24
	AutoUpdateTime     string `json:"auto_update_time"`     // 自动更新执行时间，如 "03:00"

	JWTSecret      string `json:"jwt_secret"`
}

var AppConfig Config

func LoadConfig() {
	// 默认配置
	AppConfig = Config{
		Domain:       "example.com",
		DKIMSelector: "default",
		Host:         "0.0.0.0",
		Port:         "9901",
		BaseURL:      "", // 默认留空，运行时自动推断
		EnableSSL:    false,
		JWTSecret:    "", // 默认留空，强制在后续逻辑中生成
	}

	file, err := os.Open("config.json")
	if err != nil {
		// 如果配置文件不存在，则使用默认值
		// 并立即保存一次以持久化随机生成的 Secret
		if AppConfig.JWTSecret == "" {
			AppConfig.JWTSecret = generateRandomKey(32)
		}
		SaveConfig(AppConfig)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	_ = decoder.Decode(&AppConfig)

	needsSave := false

	// --- 自动校准/补全配置 ---

	// 1. JWT Secret
	// 如果为空，或检测到是已知的硬编码/弱密钥，则轮换
	weakKeys := []string{"goemail-secret-NNbCVZcJcaOOTmAm", "change-this-secret", "goemail-secret-"}
	isWeak := false
	for _, k := range weakKeys {
		if AppConfig.JWTSecret == k || (len(AppConfig.JWTSecret) < 20 && len(AppConfig.JWTSecret) > 0) {
			isWeak = true
			break
		}
	}

	if AppConfig.JWTSecret == "" || isWeak {
		AppConfig.JWTSecret = generateRandomKey(32)
		needsSave = true
	}

	// 2. DKIM Key
	if AppConfig.DKIMPrivateKey == "" {
		if key, err := generateDKIMKey(); err == nil {
			AppConfig.DKIMPrivateKey = key
			needsSave = true
		}
	}

	// 3. 接收端口 (如果为空，说明是旧配置，补全默认值)
	if AppConfig.ReceiverPort == "" {
		AppConfig.ReceiverPort = "2525"
		needsSave = true
	}

	// 4. 收件安全默认值
	if AppConfig.ReceiverRateLimit == 0 {
		AppConfig.ReceiverRateLimit = 30 // 每 IP 每分钟 30 个连接
		needsSave = true
	}
	if AppConfig.ReceiverMaxMsgSize == 0 {
		AppConfig.ReceiverMaxMsgSize = 10240 // 10MB
		needsSave = true
	}

	// 4. Web 端口 (双重保险)
	if AppConfig.Port == "" {
		AppConfig.Port = "9901"
		needsSave = true
	}

	// 5. 数据清理默认值
	if AppConfig.CleanupEmailLogDays == 0 {
		AppConfig.CleanupEmailLogDays = 30
		needsSave = true
	}
	if AppConfig.CleanupInboxDays == 0 {
		AppConfig.CleanupInboxDays = 30
		needsSave = true
	}
	if AppConfig.CleanupQueueDays == 0 {
		AppConfig.CleanupQueueDays = 7
		needsSave = true
	}
	if AppConfig.CleanupForwardDays == 0 {
		AppConfig.CleanupForwardDays = 30
		needsSave = true
	}
	if AppConfig.CleanupAttachDays == 0 {
		AppConfig.CleanupAttachDays = 30
		needsSave = true
	}

	if needsSave {
		SaveConfig(AppConfig)
	}
}

func SaveConfig(cfg Config) error {
	// 使用 0600 权限创建文件，仅当前用户可读写
	file, err := os.OpenFile("config.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

// 使用 crypto/rand 生成安全随机字符串
func generateRandomKey(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			// Fallback if crypto/rand fails (unlikely)
			ret[i] = letters[i%len(letters)]
			continue
		}
		ret[i] = letters[num.Int64()]
	}
	return "goemail-secret-" + string(ret)
}

func generateDKIMKey() (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER})
	return string(privPEM), nil
}
