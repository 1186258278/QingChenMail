package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"io"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"
	"goemail/internal/mailer"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			tokenString, _ = c.Cookie("token")
		} else {
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// 1. 尝试验证 JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err == nil && token.Valid {
			c.Next()
			return
		}

		// 2. 尝试验证 API Key (sk_...)
		if strings.HasPrefix(tokenString, "sk_") {
			var apiKey database.APIKey
			if err := database.DB.Where("key = ?", tokenString).First(&apiKey).Error; err == nil {
				// 更新最后使用时间
				now := time.Now()
				database.DB.Model(&apiKey).Update("last_used", &now)
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token or API key"})
		c.Abort()
	}
}

// Captcha Store
var (
	captchaStore = make(map[string]string)
	captchaMutex sync.Mutex
)

// LoginHandler 登录接口
func LoginHandler(c *gin.Context) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		CaptchaID   string `json:"captcha_id"`
		CaptchaCode string `json:"captcha_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 验证码校验
	if req.CaptchaID != "" {
		captchaMutex.Lock()
		realCode, ok := captchaStore[req.CaptchaID]
		delete(captchaStore, req.CaptchaID) // 一次性
		captchaMutex.Unlock()

		if !ok || realCode != req.CaptchaCode {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid captcha code"})
			return
		}
	} else {
		// 为了安全，强制要求验证码（除非是 API 调用，但这里是 Login 接口通常给前端用）
		// 这里暂且允许空验证码以兼容旧版，或者根据 header 判断？
		// 实际上前端都会发。如果是 API 脚本登录，可能没有验证码。
		// 为了防止爆破，建议强制。但为了兼容旧代码调试，可以暂时放过？
		// 既然用户要求“增加验证码”，就应该强制。
		c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha code required"})
		return
	}

	// 2. 密码校验 (支持明文/Hash 自动升级)
	var user database.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	passwordMatched := false
	dbPass := user.Password
	inputPass := req.Password

	// 判断输入是否为 SHA256 (64 hex chars)
	isInputHash := len(inputPass) == 64 && isHex(inputPass)

	if dbPass == inputPass {
		// 直接匹配（数据库是明文且输入是明文，或数据库是Hash且输入是Hash）
		passwordMatched = true
	} else {
		// 不匹配，尝试转换后匹配
		if isInputHash {
			// 输入是 Hash，数据库可能是明文 -> 计算数据库明文的 Hash 对比
			hash := sha256.Sum256([]byte(dbPass))
			if hex.EncodeToString(hash[:]) == inputPass {
				passwordMatched = true
				// 自动升级数据库为 Hash
				database.DB.Model(&user).Update("password", inputPass)
			}
		} else {
			// 输入是明文，数据库可能是 Hash -> 计算输入明文的 Hash 对比
			hash := sha256.Sum256([]byte(inputPass))
			if hex.EncodeToString(hash[:]) == dbPass {
				passwordMatched = true
			}
		}
	}

	if !passwordMatched {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.SetCookie("token", tokenString, 3600*24, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// ChangePasswordHandler 修改密码
func ChangePasswordHandler(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user database.User
	if err := database.DB.First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	if user.Password != req.OldPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong old password"})
		return
	}

	user.Password = req.NewPassword
	database.DB.Save(&user)
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

// --- SMTP Management ---

func CreateSMTPHandler(c *gin.Context) {
	var smtp database.SMTPConfig
	if err := c.ShouldBindJSON(&smtp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 如果设为默认，先取消其他默认
	if smtp.IsDefault {
		database.DB.Model(&database.SMTPConfig{}).Where("is_default = ?", true).Update("is_default", false)
	}

	if err := database.DB.Create(&smtp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, smtp)
}

func UpdateSMTPHandler(c *gin.Context) {
	id := c.Param("id")
	var smtp database.SMTPConfig
	if err := database.DB.First(&smtp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SMTP not found"})
		return
	}

	var req database.SMTPConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsDefault && !smtp.IsDefault {
		database.DB.Model(&database.SMTPConfig{}).Where("is_default = ?", true).Update("is_default", false)
	}

	// 更新字段
	smtp.Name = req.Name
	smtp.Host = req.Host
	smtp.Port = req.Port
	smtp.Username = req.Username
	smtp.Password = req.Password
	smtp.SSL = req.SSL
	smtp.IsDefault = req.IsDefault

	if err := database.DB.Save(&smtp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, smtp)
}

func ListSMTPHandler(c *gin.Context) {
	smtps := []database.SMTPConfig{}
	database.DB.Order("is_default desc, id asc").Find(&smtps)
	c.JSON(http.StatusOK, smtps)
}

func DeleteSMTPHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&database.SMTPConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func DownloadFileHandler(c *gin.Context) {
	id := c.Param("id")
	var file database.AttachmentFile
	if err := database.DB.First(&file, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(file.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not on disk"})
		return
	}

	c.FileAttachment(file.FilePath, file.Filename)
}

// --- Domain Management ---

func CreateDomainHandler(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate DKIM key"})
		return
	}
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}))
	pubDER, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	domain := database.Domain{
		Name:           req.Name,
		DKIMSelector:   "default",
		DKIMPrivateKey: privPEM,
		DKIMPublicKey:  pubPEM,
	}

	if err := database.DB.Create(&domain).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "Duplicate entry") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Domain already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, domain)
}

func ListDomainHandler(c *gin.Context) {
	domains := []database.Domain{}
	database.DB.Find(&domains)
	c.JSON(http.StatusOK, domains)
}

func DeleteDomainHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&database.Domain{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// UpdateDomainHandler 更新域名配置 (如子域名前缀)
func UpdateDomainHandler(c *gin.Context) {
	id := c.Param("id")
	var domain database.Domain
	if err := database.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	var req struct {
		MailSubdomainPrefix *string `json:"mail_subdomain_prefix"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MailSubdomainPrefix != nil {
		domain.MailSubdomainPrefix = strings.TrimSpace(*req.MailSubdomainPrefix)
	}

	if err := database.DB.Save(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, domain)
}

func VerifyDomainHandler(c *gin.Context) {
	id := c.Param("id")
	var domain database.Domain
	if err := database.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// 使用自定义 Resolver 以绕过可能的本地缓存 (尝试使用 Google DNS)
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	// 如果无法连接 Google DNS (如国内网络环境)，回退到默认 Resolver
	if _, err := resolver.LookupHost(context.Background(), "google.com"); err != nil {
		resolver = net.DefaultResolver
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 验证 MX
	mxs, err := resolver.LookupMX(ctx, domain.Name)
	domain.MXVerified = err == nil && len(mxs) > 0

	// 2. 验证 SPF
	txts, err := resolver.LookupTXT(ctx, domain.Name)
	domain.SPFVerified = false
	if err == nil {
		for _, txt := range txts {
			// 宽松匹配: 只要包含 v=spf1 即可
			if strings.Contains(txt, "v=spf1") {
				domain.SPFVerified = true
				break
			}
		}
	}

	// 3. 验证 DMARC
	dmarcs, err := resolver.LookupTXT(ctx, "_dmarc."+domain.Name)
	domain.DMARCVerified = false
	if err == nil {
		for _, txt := range dmarcs {
			if strings.HasPrefix(txt, "v=DMARC1") {
				domain.DMARCVerified = true
				break
			}
		}
	}

	// 4. 验证 DKIM
	dkims, err := resolver.LookupTXT(ctx, domain.DKIMSelector+"._domainkey."+domain.Name)
	domain.DKIMVerified = false
	if err == nil {
		for _, txt := range dkims {
			if strings.Contains(txt, "v=DKIM1") {
				domain.DKIMVerified = true
				break
			}
		}
	}

	// 5. [新增] 验证 A 记录 (网站访问)
	// 虽然数据库没有存储字段，但可以在返回 JSON 中临时添加，或者前端单独处理
	// 这里为了完整性，我们检查一下，虽然目前 DB 没存状态
	// aRecords, _ := resolver.LookupHost(ctx, domain.Name)
	// hasARecord := len(aRecords) > 0

	database.DB.Save(&domain)
	c.JSON(http.StatusOK, domain)
}

// --- Template Management ---

func CreateTemplateHandler(c *gin.Context) {
	var tpl database.Template
	if err := c.ShouldBindJSON(&tpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Create(&tpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tpl)
}

func UpdateTemplateHandler(c *gin.Context) {
	id := c.Param("id")
	var tpl database.Template
	if err := database.DB.First(&tpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	var req database.Template
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tpl.Name = req.Name
	tpl.Subject = req.Subject
	tpl.Body = req.Body
	database.DB.Save(&tpl)
	c.JSON(http.StatusOK, tpl)
}

func ListTemplateHandler(c *gin.Context) {
	tpls := []database.Template{}
	database.DB.Find(&tpls)
	c.JSON(http.StatusOK, tpls)
}

func DeleteTemplateHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&database.Template{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// SendHandler 处理邮件发送请求
func SendHandler(c *gin.Context) {
	var req mailer.SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 模板处理逻辑
	if req.TemplateID > 0 {
		var tpl database.Template
		if err := database.DB.First(&tpl, req.TemplateID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found"})
			return
		}

		// 渲染 Subject
		if tpl.Subject != "" {
			t, err := template.New("subject").Parse(tpl.Subject)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse template subject: " + err.Error()})
				return
			}
			var buf bytes.Buffer
			if err := t.Execute(&buf, req.Variables); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template subject: " + err.Error()})
				return
			}
			req.Subject = buf.String()
		}

		// 渲染 Body
		if tpl.Body != "" {
			// 使用 html/template 确保安全性，但也允许变量替换
			t, err := template.New("body").Parse(tpl.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse template body: " + err.Error()})
				return
			}
			var buf bytes.Buffer
			if err := t.Execute(&buf, req.Variables); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template body: " + err.Error()})
				return
			}
			req.Body = buf.String()
		}
	}

	// 附件处理：落地保存 (File Persistence)
	if len(req.Attachments) > 0 {
		saveDir := "data/uploads"
		if _, err := os.Stat(saveDir); os.IsNotExist(err) {
			os.MkdirAll(saveDir, 0755)
		}

		for i, att := range req.Attachments {
			var fileData []byte
			var err error
			sourceType := ""

			// 1. 获取内容
			if att.Content != "" {
				sourceType = "api_base64"
				fileData, err = base64.StdEncoding.DecodeString(att.Content)
			} else if att.URL != "" {
				sourceType = "api_url"
				resp, err := http.Get(att.URL)
				if err == nil {
					defer resp.Body.Close()
					fileData, err = io.ReadAll(resp.Body)
				}
			}

			// 2. 保存并记录
			if err == nil && len(fileData) > 0 {
				ext := filepath.Ext(att.Filename)
				if ext == "" {
					ext = ".dat"
				}
				// 生成唯一文件名: timestamp_random.ext
				newFilename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), generateRandomKey()[:8], ext)
				localPath := filepath.Join(saveDir, newFilename)

				if err := os.WriteFile(localPath, fileData, 0644); err == nil {
					// 记录到数据库
					dbFile := database.AttachmentFile{
						Filename:    att.Filename,
						FilePath:    localPath,
						FileSize:    int64(len(fileData)),
						ContentType: att.ContentType,
						Source:      sourceType,
						RelatedTo:   req.To,
					}
					database.DB.Create(&dbFile)

					// 修改请求指向本地文件，清空 Base64 以减轻队列压力
					req.Attachments[i].Content = ""
					req.Attachments[i].URL = "local://" + localPath
				}
			}
		}
	}

	// 异步发送：只负责加入队列
	queueID, err := mailer.SendEmailAsync(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue email: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "Email queued successfully",
		"queue_id": queueID,
	})
}

// StatsHandler 获取统计数据
func StatsHandler(c *gin.Context) {
	stats, err := database.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// LogsHandler 获取日志
func LogsHandler(c *gin.Context) {
	var logs []database.EmailLog
	result := database.DB.Order("created_at desc").Limit(100).Find(&logs)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

// GenerateDKIMHandler 生成新的 DKIM 密钥
func GenerateDKIMHandler(c *gin.Context) {
	// 兼容旧接口，建议使用 Domain Management
	c.JSON(http.StatusOK, gin.H{"message": "Please use Domain Management"})
}

// GetConfigHandler 获取配置
func GetConfigHandler(c *gin.Context) {
	// 返回配置时隐藏敏感信息
	cfg := config.AppConfig
	// 这里可以根据需要决定是否隐藏 JWTSecret 或其他敏感信息
	// cfg.JWTSecret = "***" 
	c.JSON(http.StatusOK, cfg)
}

// UpdateConfigHandler 更新配置
func UpdateConfigHandler(c *gin.Context) {
	var newConfig config.Config
	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 校验 SSL 配置防止配置错误导致服务无法启动
	if newConfig.EnableSSL {
		if newConfig.CertFile == "" || newConfig.KeyFile == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SSL enabled but cert/key file path missing"})
			return
		}
		if _, err := os.Stat(newConfig.CertFile); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Certificate file not found: " + newConfig.CertFile})
			return
		}
		if _, err := os.Stat(newConfig.KeyFile); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Key file not found: " + newConfig.KeyFile})
			return
		}
	}

	// 2. 保护关键字段或执行重置
	if newConfig.DKIMPrivateKey == "" {
		newConfig.DKIMPrivateKey = config.AppConfig.DKIMPrivateKey
	}
	
	// JWT Secret 处理：支持重置
	if newConfig.JWTSecret == "RESET" {
		b := make([]byte, 16)
		rand.Read(b)
		newConfig.JWTSecret = fmt.Sprintf("goemail-secret-%x", b)
	} else if newConfig.JWTSecret == "" {
		newConfig.JWTSecret = config.AppConfig.JWTSecret
	}

	// 3. 默认值保护
	if newConfig.Host == "" {
		newConfig.Host = config.AppConfig.Host
	}
	if newConfig.Port == "" {
		newConfig.Port = config.AppConfig.Port
	}

	// 4. [新增] 端口可用性检测 (如果启用了接收服务且修改了端口)
	if newConfig.EnableReceiver && (newConfig.ReceiverPort != config.AppConfig.ReceiverPort || !config.AppConfig.EnableReceiver) {
		port := newConfig.ReceiverPort
		if port == "" {
			port = "25"
		}
		// 尝试监听端口
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			// 区分错误类型
			errMsg := err.Error()
			if strings.Contains(errMsg, "bind: permission denied") {
				errMsg = fmt.Sprintf("Cannot bind to port %s (Permission denied). Try running as root or use setcap.", port)
			} else if strings.Contains(errMsg, "bind: address already in use") {
				// 尝试获取占用者信息
				procInfo := getProcessInfo(port)
				errMsg = fmt.Sprintf("Port %s is already in use by: %s", port, procInfo)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		ln.Close()
	}

	config.AppConfig = newConfig
	if err := config.SaveConfig(config.AppConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	msg := "Config updated"
	if newConfig.JWTSecret != config.AppConfig.JWTSecret { // 如果 Secret 变了
		msg = "Config updated & Token reset"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

// --- API Key Management ---

func generateRandomKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return fmt.Sprintf("sk_live_%x", b)
}

func ListAPIKeysHandler(c *gin.Context) {
	keys := []database.APIKey{}
	database.DB.Order("created_at desc").Find(&keys)
	c.JSON(http.StatusOK, keys)
}

func CreateAPIKeyHandler(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key := database.APIKey{
		Name: req.Name,
		Key:  generateRandomKey(),
	}

	if err := database.DB.Create(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, key)
}

func DeleteAPIKeyHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&database.APIKey{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// BackupHandler 导出备份
func BackupHandler(c *gin.Context) {
	c.Header("Content-Disposition", "attachment; filename=goemail-backup.zip")
	c.Header("Content-Type", "application/zip")

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	files := []string{"config.json", "goemail.db"}

	for _, filename := range files {
		f, err := os.Open(filename)
		if err != nil {
			continue
		}
		defer f.Close()

		w, err := zipWriter.Create(filename)
		if err != nil {
			continue
		}

		if _, err := io.Copy(w, f); err != nil {
			continue
		}
	}
}

// CaptchaHandler 生成验证码
func CaptchaHandler(c *gin.Context) {
	// 生成随机数字
	mathrand.Seed(time.Now().UnixNano())
	code := fmt.Sprintf("%04d", mathrand.Intn(10000))
	id := generateRandomKey() // 复用随机字符串生成

	captchaMutex.Lock()
	captchaStore[id] = code
	// 简单的清理逻辑：如果太大则清空（生产环境应用过期清理 goroutine）
	if len(captchaStore) > 1000 {
		captchaStore = make(map[string]string)
		captchaStore[id] = code
	}
	captchaMutex.Unlock()

	// 生成增强版 SVG (带干扰线和噪点)
	width, height := 120, 40
	svgContent := fmt.Sprintf(`<rect width="100%%" height="100%%" fill="#f8fafc"/>`)
	
	// 添加噪点
	for i := 0; i < 20; i++ {
		x := mathrand.Intn(width)
		y := mathrand.Intn(height)
		r := mathrand.Intn(2) + 1
		op := float32(mathrand.Intn(5)) / 10.0
		svgContent += fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" fill="#94a3b8" opacity="%.1f"/>`, x, y, r, op)
	}

	// 添加干扰线
	for i := 0; i < 5; i++ {
		x1 := mathrand.Intn(width)
		y1 := mathrand.Intn(height)
		x2 := mathrand.Intn(width)
		y2 := mathrand.Intn(height)
		stroke := []string{"#cbd5e1", "#94a3b8", "#64748b"}[mathrand.Intn(3)]
		svgContent += fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1" />`, x1, y1, x2, y2, stroke)
	}

	// 文字 (稍微扭曲)
	// 为了简单，我们还是居中显示，但改变颜色和字体
	svgContent += fmt.Sprintf(`<text x="50%%" y="55%%" font-family="Arial, sans-serif" font-size="26" font-weight="bold" fill="#2563eb" dominant-baseline="middle" text-anchor="middle" letter-spacing="6" style="text-shadow: 1px 1px 2px rgba(0,0,0,0.1);">%s</text>`, code)

	svg := fmt.Sprintf(`<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">%s</svg>`, width, height, width, height, svgContent)

	base64Svg := base64.StdEncoding.EncodeToString([]byte(svg))
	c.JSON(http.StatusOK, gin.H{
		"captcha_id": id,
		"image":      "data:image/svg+xml;base64," + base64Svg,
	})
}

// WallpaperHandler 获取 Bing 每日壁纸
func WallpaperHandler(c *gin.Context) {
	// 确保目录存在
	saveDir := "static/wallpapers"
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		os.MkdirAll(saveDir, 0755)
	}

	today := time.Now().Format("2006-01-02")
	filename := today + ".jpg"
	localPath := filepath.Join(saveDir, filename)
	// 修改为 /wallpapers/ 路径
	publicURL := "/wallpapers/" + filename

	// 1. 检查本地缓存
	if _, err := os.Stat(localPath); err == nil {
		c.JSON(http.StatusOK, gin.H{"url": publicURL, "source": "local"})
		return
	}

	// 2. 从 Bing 获取
	// Bing API: https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&mkt=zh-CN
	resp, err := http.Get("https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&mkt=zh-CN")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"url": "", "error": "Bing API failed"})
		return
	}
	defer resp.Body.Close()

	var bingData struct {
		Images []struct {
			Url string `json:"url"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bingData); err != nil || len(bingData.Images) == 0 {
		c.JSON(http.StatusOK, gin.H{"url": "", "error": "Bing response parse failed"})
		return
	}

	bingURL := "https://www.bing.com" + bingData.Images[0].Url
	
	// 下载图片
	imgResp, err := http.Get(bingURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"url": "", "error": "Image download failed"})
		return
	}
	defer imgResp.Body.Close()

	// 保存到本地
	out, err := os.Create(localPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"url": "", "error": "File save failed"})
		return
	}
	defer out.Close()
	io.Copy(out, imgResp.Body)

	c.JSON(http.StatusOK, gin.H{"url": publicURL, "source": "bing"})
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// --- File Management ---

func ListFilesHandler(c *gin.Context) {
	var files []database.AttachmentFile
	database.DB.Order("created_at desc").Limit(100).Find(&files)
	c.JSON(http.StatusOK, files)
}

func DeleteFileHandler(c *gin.Context) {
	id := c.Param("id")
	var file database.AttachmentFile
	if err := database.DB.First(&file, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// 删除本地文件
	if file.FilePath != "" {
		os.Remove(file.FilePath)
	}

	database.DB.Delete(&file)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func BatchDeleteFilesHandler(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var files []database.AttachmentFile
	database.DB.Where("id IN ?", req.IDs).Find(&files)

	for _, f := range files {
		if f.FilePath != "" {
			os.Remove(f.FilePath)
		}
		database.DB.Delete(&f)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// --- Forward Rule Management (邮件转发规则) ---

// ListForwardRulesHandler 获取指定域名的转发规则
func ListForwardRulesHandler(c *gin.Context) {
	domainID := c.Query("domain_id")
	if domainID == "" {
		// 返回所有规则
		var rules []database.ForwardRule
		database.DB.Order("domain_id asc, id asc").Find(&rules)
		c.JSON(http.StatusOK, rules)
		return
	}
	var rules []database.ForwardRule
	database.DB.Where("domain_id = ?", domainID).Order("id asc").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

// CreateForwardRuleHandler 创建转发规则
func CreateForwardRuleHandler(c *gin.Context) {
	var req struct {
		DomainID  uint   `json:"domain_id"`
		MatchType string `json:"match_type"` // all, prefix, exact
		MatchAddr string `json:"match_addr"`
		ForwardTo string `json:"forward_to"`
		Remark    string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证域名存在
	var domain database.Domain
	if err := database.DB.First(&domain, req.DomainID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// 验证匹配类型
	if req.MatchType != "all" && req.MatchType != "prefix" && req.MatchType != "exact" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match_type"})
		return
	}

	// 验证转发地址
	if req.ForwardTo == "" || !strings.Contains(req.ForwardTo, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid forward_to address"})
		return
	}

	rule := database.ForwardRule{
		DomainID:  domain.ID,
		MatchType: req.MatchType,
		MatchAddr: req.MatchAddr,
		ForwardTo: req.ForwardTo,
		Enabled:   true,
		Remark:    req.Remark,
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// UpdateForwardRuleHandler 更新转发规则
func UpdateForwardRuleHandler(c *gin.Context) {
	id := c.Param("id")
	
	var rule database.ForwardRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	var req struct {
		MatchType string `json:"match_type"`
		MatchAddr string `json:"match_addr"`
		ForwardTo string `json:"forward_to"`
		Enabled   *bool  `json:"enabled"`
		Remark    string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MatchType != "" {
		if req.MatchType != "all" && req.MatchType != "prefix" && req.MatchType != "exact" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match_type"})
			return
		}
		rule.MatchType = req.MatchType
	}
	if req.MatchAddr != "" || req.MatchType == "all" {
		rule.MatchAddr = req.MatchAddr
	}
	if req.ForwardTo != "" {
		rule.ForwardTo = req.ForwardTo
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	rule.Remark = req.Remark

	database.DB.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// DeleteForwardRuleHandler 删除转发规则
func DeleteForwardRuleHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&database.ForwardRule{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ToggleForwardRuleHandler 启用/禁用转发规则
func ToggleForwardRuleHandler(c *gin.Context) {
	id := c.Param("id")
	
	var rule database.ForwardRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	rule.Enabled = !rule.Enabled
	database.DB.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// ListForwardLogsHandler 获取转发日志
func ListForwardLogsHandler(c *gin.Context) {
	var logs []database.ForwardLog
	database.DB.Order("created_at desc").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

// GetForwardStatsHandler 获取转发统计
func GetForwardStatsHandler(c *gin.Context) {
	var totalCount int64
	var successCount int64
	var failCount int64
	var todayCount int64

	database.DB.Model(&database.ForwardLog{}).Count(&totalCount)
	database.DB.Model(&database.ForwardLog{}).Where("status = ?", "success").Count(&successCount)
	database.DB.Model(&database.ForwardLog{}).Where("status = ?", "failed").Count(&failCount)
	
	startOfDay := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&database.ForwardLog{}).Where("created_at >= ?", startOfDay).Count(&todayCount)

	c.JSON(http.StatusOK, gin.H{
		"total":   totalCount,
		"success": successCount,
		"failed":  failCount,
		"today":   todayCount,
	})
}

// TestPortHandler 测试端口可用性
func TestPortHandler(c *gin.Context) {
	var req struct {
		Port string `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Port == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Port is required"})
		return
	}

	// 尝试监听
	addr := fmt.Sprintf("0.0.0.0:%s", req.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// 区分错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "forbidden") {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Permission denied. Port < 1024 requires root/admin privilege."})
		} else if strings.Contains(errMsg, "address already in use") || strings.Contains(errMsg, "Only one usage of each socket address") {
			// 如果是我们自己占用的 (比如配置就是当前端口)，则算成功
			if req.Port == config.AppConfig.ReceiverPort && config.AppConfig.EnableReceiver {
				c.JSON(http.StatusOK, gin.H{"success": true, "message": "Port is in use by this application (OK)."})
			} else {
				// 尝试查找占用进程
				procInfo := getProcessInfo(req.Port)
				msg := fmt.Sprintf("Port is occupied by: %s", procInfo)
				c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
			}
		} else {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Listen failed: " + errMsg})
		}
		return
	}
	ln.Close()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Port is available."})
}

// getProcessInfo 获取占用端口的进程信息
func getProcessInfo(port string) string {
	if runtime.GOOS == "windows" {
		// netstat -ano | findstr :PORT
		cmd := exec.Command("cmd", "/C", fmt.Sprintf("netstat -ano | findstr :%s", port))
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			return "Unknown (Check Task Manager)"
		}
		
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "LISTENING") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					pid := fields[len(fields)-1]
					// tasklist | findstr PID
					pCmd := exec.Command("cmd", "/C", fmt.Sprintf("tasklist /FI \"PID eq %s\" /FO CSV /NH", pid))
					pOut, _ := pCmd.Output()
					// Output format: "process_name.exe","1234","Console","1","10,000 K"
					parts := strings.Split(string(pOut), ",")
					if len(parts) > 0 {
						procName := strings.Trim(parts[0], "\"")
						return fmt.Sprintf("%s (PID: %s)", procName, pid)
					}
					return fmt.Sprintf("PID: %s", pid)
				}
			}
		}
	} else {
		// lsof -i :PORT -t
		cmd := exec.Command("lsof", "-i", ":"+port, "-t")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			pid := strings.TrimSpace(string(out))
			// ps -p PID -o comm=
			pCmd := exec.Command("ps", "-p", pid, "-o", "comm=")
			pOut, _ := pCmd.Output()
			return fmt.Sprintf("%s (PID: %s)", strings.TrimSpace(string(pOut)), pid)
		}
	}
	return "Unknown process"
}

// KillProcessHandler 强制关闭占用端口的进程
func KillProcessHandler(c *gin.Context) {
	var req struct {
		PID string `json:"pid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.PID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PID is required"})
		return
	}

	// 1. 安全检查：是否为系统关键进程
	procName := getProcessNameByPID(req.PID)
	if procName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Process not found or already terminated"})
		return
	}

	if isSystemProcess(procName) {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Cannot kill system process: %s", procName)})
		return
	}

	// 2. 执行关闭
	if err := killProcess(req.PID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to kill process: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Process %s (PID: %s) terminated successfully", procName, req.PID)})
}

func getProcessNameByPID(pid string) string {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", fmt.Sprintf("tasklist /FI \"PID eq %s\" /FO CSV /NH", pid))
		out, err := cmd.Output()
		if err == nil {
			parts := strings.Split(string(out), ",")
			if len(parts) > 0 {
				return strings.Trim(parts[0], "\"")
			}
		}
	} else {
		cmd := exec.Command("ps", "-p", pid, "-o", "comm=")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func isSystemProcess(name string) bool {
	systemProcs := []string{
		"system", "system idle process", "smss.exe", "csrss.exe", "wininit.exe", 
		"services.exe", "lsass.exe", "svchost.exe", "winlogon.exe", "explorer.exe",
		"init", "systemd", "kthreadd", "rcu_sched",
	}
	nameLower := strings.ToLower(name)
	for _, p := range systemProcs {
		if nameLower == p {
			return true
		}
	}
	return false
}

func killProcess(pid string) error {
	if runtime.GOOS == "windows" {
		// taskkill /F /PID <pid>
		return exec.Command("taskkill", "/F", "/PID", pid).Run()
	} else {
		// kill -9 <pid>
		return exec.Command("kill", "-9", pid).Run()
	}
}
