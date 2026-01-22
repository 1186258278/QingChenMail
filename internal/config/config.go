package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	mathrand "math/rand"
	"os"
	"time"
)

type Config struct {
	Domain         string `json:"domain"`
	DKIMSelector   string `json:"dkim_selector"`
	DKIMPrivateKey string `json:"dkim_private_key"`
	
	// Web Server Config
	Host           string `json:"host"`            // 监听地址，默认 0.0.0.0
	Port           string `json:"port"`            // 监听端口
	BaseURL        string `json:"base_url"`        // [新增] 公网访问地址 (用于生成追踪链接)，如 http://mail.example.com:9901
	EnableSSL      bool   `json:"enable_ssl"`      // 是否开启 HTTPS
	CertFile       string `json:"cert_file"`       // 证书文件路径
	KeyFile        string `json:"key_file"`        // 私钥文件路径

	// SMTP Receiver Config (邮件接收服务)
	EnableReceiver bool   `json:"enable_receiver"` // 是否启用接收服务
	ReceiverPort   string `json:"receiver_port"`   // SMTP 接收端口，默认 25
	ReceiverTLS    bool   `json:"receiver_tls"`    // 是否启用 STARTTLS

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
		JWTSecret:    "goemail-secret-" + generateRandomString(16),
	}

	file, err := os.Open("config.json")
	if err != nil {
		// 如果配置文件不存在，则使用默认值
		// 并立即保存一次以持久化随机生成的 Secret
		SaveConfig(AppConfig)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	_ = decoder.Decode(&AppConfig)

	needsSave := false

	// --- 自动校准/补全配置 ---

	// 1. JWT Secret
	if AppConfig.JWTSecret == "" {
		AppConfig.JWTSecret = "goemail-secret-" + generateRandomString(16)
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
		// 注意：我们不强制开启 EnableReceiver，让用户自己决定，但我们把端口写进去方便修改
		needsSave = true
	}

	// 4. Web 端口 (双重保险)
	if AppConfig.Port == "" {
		AppConfig.Port = "9901"
		needsSave = true
	}

	if needsSave {
		SaveConfig(AppConfig)
	}
}

func SaveConfig(cfg Config) error {
	file, err := os.Create("config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func generateRandomString(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	mathrand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[mathrand.Intn(len(letters))]
	}
	return string(b)
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
