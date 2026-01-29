package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"goemail/internal/auth"
	"goemail/internal/config"
	"goemail/internal/database"
)

// 临时存储 TOTP 设置会话 (用于绑定确认)
// key: username, value: secret (Base32)
var totpSetupSessions = struct {
	sync.RWMutex
	sessions map[string]totpSetupSession
}{sessions: make(map[string]totpSetupSession)}

type totpSetupSession struct {
	Secret    string
	ExpiresAt time.Time
}

// TOTPSetupHandler 生成 TOTP 密钥和二维码
// GET /api/v1/totp/setup
func TOTPSetupHandler(c *gin.Context) {
	// 从 JWT 中获取用户名
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	usernameStr := username.(string)

	// 检查用户是否已启用 TOTP
	var user database.User
	if err := database.DB.Where("username = ?", usernameStr).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	if user.TOTPEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两步验证已启用，如需重新绑定请先关闭"})
		return
	}

	// 生成新的 TOTP 密钥
	key, err := auth.GenerateTOTPSecret(usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成密钥失败"})
		return
	}

	// 生成二维码 Data URL
	qrCode, err := auth.GenerateQRCodeDataURL(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成二维码失败"})
		return
	}

	// 存储临时会话 (5分钟有效期)
	totpSetupSessions.Lock()
	totpSetupSessions.sessions[usernameStr] = totpSetupSession{
		Secret:    key.Secret(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	totpSetupSessions.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"secret":  key.Secret(), // Base32 格式，供手动输入
		"qr_code": qrCode,       // Data URL，可直接放入 img src
		"uri":     key.URL(),    // otpauth:// URI
	})
}

// TOTPEnableHandler 验证并启用两步验证
// POST /api/v1/totp/enable
func TOTPEnableHandler(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"` // 用户输入的 6 位验证码
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入验证码"})
		return
	}

	// 获取用户名
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	usernameStr := username.(string)

	// 获取临时会话中的密钥
	totpSetupSessions.RLock()
	session, exists := totpSetupSessions.sessions[usernameStr]
	totpSetupSessions.RUnlock()

	if !exists || time.Now().After(session.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "设置会话已过期，请重新获取二维码"})
		return
	}

	// 验证 TOTP 码
	code := strings.TrimSpace(req.Code)
	if !auth.ValidateTOTP(session.Secret, code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误，请检查后重试"})
		return
	}

	// 验证通过，保存密钥并启用 TOTP
	var user database.User
	if err := database.DB.Where("username = ?", usernameStr).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	user.TOTPSecret = session.Secret
	user.TOTPEnabled = true

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	// 清理临时会话
	totpSetupSessions.Lock()
	delete(totpSetupSessions.sessions, usernameStr)
	totpSetupSessions.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "两步验证已启用"})
}

// TOTPDisableHandler 关闭两步验证
// POST /api/v1/totp/disable
func TOTPDisableHandler(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"` // 需要密码确认
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入密码"})
		return
	}

	// 获取用户名
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	usernameStr := username.(string)

	// 查询用户
	var user database.User
	if err := database.DB.Where("username = ?", usernameStr).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	// 验证密码
	if !verifyUserPassword(user.Password, req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码错误"})
		return
	}

	// 关闭 TOTP
	user.TOTPEnabled = false
	user.TOTPSecret = ""

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "两步验证已关闭"})
}

// TOTPStatusHandler 获取两步验证状态
// GET /api/v1/totp/status
func TOTPStatusHandler(c *gin.Context) {
	// 获取用户名
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	usernameStr := username.(string)

	// 查询用户
	var user database.User
	if err := database.DB.Where("username = ?", usernameStr).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": user.TOTPEnabled,
	})
}

// TOTPVerifyHandler 登录时验证 TOTP (公开接口，无需 JWT 认证)
// POST /api/v1/totp/verify
func TOTPVerifyHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Code     string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供用户名和验证码"})
		return
	}

	// 查询用户
	var user database.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "用户不存在",
			"reset_hint": "如忘记两步验证，请在服务器执行: ./goemail -reset-totp",
		})
		return
	}

	// 检查是否启用了 TOTP
	if !user.TOTPEnabled || user.TOTPSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户未启用两步验证"})
		return
	}

	// 验证 TOTP 码
	code := strings.TrimSpace(req.Code)
	if !auth.ValidateTOTP(user.TOTPSecret, code) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "验证码错误",
			"reset_hint": "如忘记两步验证，请在服务器执行: ./goemail -reset-totp",
		})
		return
	}

	// 验证通过，生成 JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	// 设置 Cookie
	secureCookie := config.AppConfig.EnableSSL
	c.SetCookie("token", tokenString, 3600*24, "/", "", secureCookie, true)

	c.JSON(http.StatusOK, gin.H{
		"token":   tokenString,
		"message": "登录成功",
	})
}

// verifyUserPassword 验证密码 (支持 bcrypt、明文、SHA256)
func verifyUserPassword(stored, input string) bool {
	// 如果是 bcrypt 哈希
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") {
		return database.CheckPasswordHash(input, stored)
	}
	// 兼容明文
	if stored == input {
		return true
	}
	// 兼容 SHA256
	hash := sha256.Sum256([]byte(input))
	return stored == hex.EncodeToString(hash[:])
}
