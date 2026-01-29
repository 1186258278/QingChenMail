package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"goemail/internal/cert"
	"goemail/internal/database"

	"github.com/gin-gonic/gin"
)

var (
	certManager *cert.Manager
	acmeClient  *cert.ACMEClient
)

// InitCertManager 初始化证书管理器
func InitCertManager() {
	certManager = cert.NewManager()
	// 默认使用生产环境，设置 true 可切换到测试环境
	acmeClient = cert.NewACMEClient(certManager, false)
}

// CertificateResponse 证书响应结构
type CertificateResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Domains     []string  `json:"domains"`
	Issuer      string    `json:"issuer"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	Source      string    `json:"source"`
	AutoRenew   bool      `json:"auto_renew"`
	DNSProvider string    `json:"dns_provider"`
	ACMEEmail   string    `json:"acme_email"`
	CertPath    string    `json:"cert_path"`
	KeyPath     string    `json:"key_path"`
	Status      string    `json:"status"`      // valid, warning, critical, expired
	DaysLeft    int       `json:"days_left"`   // 剩余天数
	CreatedAt   time.Time `json:"created_at"`
}

// toCertResponse 转换为响应结构
func toCertResponse(c *database.Certificate) CertificateResponse {
	domains := []string{}
	if c.Domains != "" {
		domains = strings.Split(c.Domains, ",")
	}

	return CertificateResponse{
		ID:          c.ID,
		Name:        c.Name,
		Domains:     domains,
		Issuer:      c.Issuer,
		NotBefore:   c.NotBefore,
		NotAfter:    c.NotAfter,
		Source:      c.Source,
		AutoRenew:   c.AutoRenew,
		DNSProvider: c.DNSProvider,
		ACMEEmail:   c.ACMEEmail,
		CertPath:    c.CertPath,
		KeyPath:     c.KeyPath,
		Status:      cert.GetExpiryStatus(c.NotAfter),
		DaysLeft:    cert.DaysUntilExpiry(c.NotAfter),
		CreatedAt:   c.CreatedAt,
	}
}

// GetCertificatesHandler 获取所有证书
// GET /api/v1/certs
func GetCertificatesHandler(c *gin.Context) {
	certs, err := certManager.GetAllCertificates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取证书列表失败: " + err.Error()})
		return
	}

	result := make([]CertificateResponse, len(certs))
	for i, cert := range certs {
		result[i] = toCertResponse(&cert)
	}

	c.JSON(http.StatusOK, result)
}

// GetCertificateHandler 获取单个证书
// GET /api/v1/certs/:id
func GetCertificateHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	cert, err := certManager.GetCertificateByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "证书不存在"})
		return
	}

	c.JSON(http.StatusOK, toCertResponse(cert))
}

// UploadCertificateRequest 上传证书请求
type UploadCertificateRequest struct {
	Name    string `json:"name"`     // 证书名称
	CertPEM string `json:"cert_pem"` // 证书内容 (PEM)
	KeyPEM  string `json:"key_pem"`  // 私钥内容 (PEM)
}

// UploadCertificateHandler 上传证书
// POST /api/v1/certs
func UploadCertificateHandler(c *gin.Context) {
	var req UploadCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.CertPEM == "" || req.KeyPEM == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "证书和私钥不能为空"})
		return
	}

	// 创建证书记录
	dbCert := &database.Certificate{
		Name:   req.Name,
		Source: "manual",
	}

	// 保存证书
	if err := certManager.SaveCertificate(dbCert, req.CertPEM, req.KeyPEM); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "证书上传成功",
		"certificate": toCertResponse(dbCert),
	})
}

// DeleteCertificateHandler 删除证书
// DELETE /api/v1/certs/:id
func DeleteCertificateHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	if err := certManager.DeleteCertificate(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "证书已删除"})
}

// GetMatchingCertsHandler 获取匹配指定域名的证书
// GET /api/v1/certs/match/:domain
func GetMatchingCertsHandler(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "域名不能为空"})
		return
	}

	certs, err := certManager.GetMatchingCertificates(domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	result := make([]CertificateResponse, len(certs))
	for i, cert := range certs {
		result[i] = toCertResponse(&cert)
	}

	c.JSON(http.StatusOK, result)
}

// UpdateDomainCertRequest 更新域名证书关联请求
type UpdateDomainCertRequest struct {
	CertificateID *uint `json:"certificate_id"` // 证书 ID，null 表示解除关联
}

// UpdateDomainCertHandler 更新域名的证书关联
// PUT /api/v1/domains/:id/cert
func UpdateDomainCertHandler(c *gin.Context) {
	domainID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的域名 ID"})
		return
	}

	var req UpdateDomainCertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证证书是否存在
	if req.CertificateID != nil && *req.CertificateID > 0 {
		_, err := certManager.GetCertificateByID(*req.CertificateID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指定的证书不存在"})
			return
		}
	}

	// 更新关联
	if err := certManager.ApplyCertToDomain(uint(domainID), req.CertificateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "域名证书关联已更新"})
}

// ApplyCertToSTARTTLSRequest 应用证书到 STARTTLS 请求
type ApplyCertToSTARTTLSRequest struct {
	CertificateID uint `json:"certificate_id" binding:"required"`
}

// ApplyCertToSTARTTLSHandler 将证书应用到 STARTTLS 配置
// POST /api/v1/certs/:id/apply-starttls
func ApplyCertToSTARTTLSHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	if err := certManager.ApplyToSTARTTLS(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "证书已应用到 STARTTLS 配置，请重启服务生效",
	})
}

// ========== ACME (Let's Encrypt) 相关 API ==========

// ACMEInitRequest ACME 初始化请求
type ACMEInitRequest struct {
	Domain      string            `json:"domain" binding:"required"`
	Email       string            `json:"email" binding:"required,email"`
	DNSProvider string            `json:"dns_provider"` // manual, cloudflare, aliyun, dnspod
	DNSConfig   map[string]string `json:"dns_config"`   // DNS API 配置
}

// ACMEInitHandler 初始化 ACME 证书申请
// POST /api/v1/certs/acme/init
func ACMEInitHandler(c *gin.Context) {
	var req ACMEInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.DNSProvider == "" {
		req.DNSProvider = "manual"
	}

	challenge, err := acmeClient.InitChallenge(
		req.Domain,
		req.Email,
		cert.DNSProviderType(req.DNSProvider),
		req.DNSConfig,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "挑战已初始化，请添加 DNS TXT 记录",
		"domain":     challenge.Domain,
		"txt_record": challenge.TXTRecord,
		"txt_value":  challenge.TXTValue,
		"expires_in": "10 分钟",
		"note":       "添加 DNS 记录后，请调用 /api/v1/certs/acme/verify 完成验证",
	})
}

// ACMEVerifyRequest ACME 验证请求
type ACMEVerifyRequest struct {
	Domain string `json:"domain" binding:"required"`
}

// ACMEVerifyHandler 验证并获取证书
// POST /api/v1/certs/acme/verify
func ACMEVerifyHandler(c *gin.Context) {
	var req ACMEVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	certificate, err := acmeClient.VerifyAndObtain(req.Domain)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "证书申请成功",
		"certificate": toCertResponse(certificate),
	})
}

// ACMEChallengeStatusHandler 获取挑战状态
// GET /api/v1/certs/acme/challenge/:domain
func ACMEChallengeStatusHandler(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "域名不能为空"})
		return
	}

	challenge := acmeClient.GetPendingChallenge(domain)
	if challenge == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有找到该域名的待验证挑战"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"domain":     challenge.Domain,
		"txt_record": challenge.TXTRecord,
		"txt_value":  challenge.TXTValue,
		"created_at": challenge.CreatedAt,
	})
}

// ACMECancelHandler 取消挑战
// DELETE /api/v1/certs/acme/challenge/:domain
func ACMECancelHandler(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "域名不能为空"})
		return
	}

	acmeClient.CancelChallenge(domain)
	c.JSON(http.StatusOK, gin.H{"message": "挑战已取消"})
}

// RenewCertificateHandler 续期证书
// POST /api/v1/certs/:id/renew
func RenewCertificateHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的证书 ID"})
		return
	}

	newCert, err := acmeClient.RenewCertificate(uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "证书续期成功",
		"certificate": toCertResponse(newCert),
	})
}

// GetExpiringSoonHandler 获取即将到期的证书
// GET /api/v1/certs/expiring?days=30
func GetExpiringSoonHandler(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 30
	}

	certs, err := certManager.GetExpiringSoon(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	result := make([]CertificateResponse, len(certs))
	for i, cert := range certs {
		result[i] = toCertResponse(&cert)
	}

	c.JSON(http.StatusOK, result)
}
