package cert

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"
)

const (
	// CertsDir 证书存储目录
	CertsDir = "./certs"
)

// Manager 证书管理器
type Manager struct {
	encryptionKey []byte
}

// NewManager 创建证书管理器
func NewManager() *Manager {
	// 使用 JWT Secret 派生加密密钥
	secret := config.AppConfig.JWTSecret
	if secret == "" {
		secret = "default-secret-key"
	}
	hash := sha256.Sum256([]byte(secret))
	return &Manager{
		encryptionKey: hash[:],
	}
}

// encrypt 使用 AES-GCM 加密数据
func (m *Manager) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 解密数据
func (m *Manager) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// EnsureCertsDir 确保证书目录存在
func EnsureCertsDir() error {
	return os.MkdirAll(CertsDir, 0700)
}

// SaveCertificate 保存证书到数据库和文件系统
func (m *Manager) SaveCertificate(cert *database.Certificate, certPEM, keyPEM string) error {
	// 1. 解析证书获取信息
	certInfo, err := ParseCertPEM(certPEM)
	if err != nil {
		return fmt.Errorf("证书解析失败: %w", err)
	}

	// 2. 验证私钥
	if err := ValidateKeyPEM(keyPEM); err != nil {
		return fmt.Errorf("私钥验证失败: %w", err)
	}

	// 3. 确保目录存在
	if err := EnsureCertsDir(); err != nil {
		return fmt.Errorf("创建证书目录失败: %w", err)
	}

	// 4. 生成文件名 (使用域名和时间戳)
	primaryDomain := certInfo.Domains[0]
	if len(certInfo.Domains) > 0 {
		primaryDomain = certInfo.Domains[0]
	}
	safeDomain := strings.ReplaceAll(primaryDomain, "*", "wildcard")
	safeDomain = strings.ReplaceAll(safeDomain, ".", "_")
	timestamp := time.Now().Format("20060102_150405")
	
	certFilename := fmt.Sprintf("%s_%s.crt", safeDomain, timestamp)
	keyFilename := fmt.Sprintf("%s_%s.key", safeDomain, timestamp)
	
	certPath := filepath.Join(CertsDir, certFilename)
	keyPath := filepath.Join(CertsDir, keyFilename)

	// 5. 写入文件
	if err := os.WriteFile(certPath, []byte(certPEM), 0644); err != nil {
		return fmt.Errorf("写入证书文件失败: %w", err)
	}
	if err := os.WriteFile(keyPath, []byte(keyPEM), 0600); err != nil {
		os.Remove(certPath) // 回滚
		return fmt.Errorf("写入私钥文件失败: %w", err)
	}

	// 6. 加密存储敏感数据
	encryptedKey, err := m.encrypt(keyPEM)
	if err != nil {
		return fmt.Errorf("加密私钥失败: %w", err)
	}

	// 7. 填充证书信息
	cert.Domains = strings.Join(certInfo.Domains, ",")
	cert.Issuer = certInfo.Issuer
	cert.NotBefore = certInfo.NotBefore
	cert.NotAfter = certInfo.NotAfter
	cert.CertPEM = certPEM
	cert.KeyPEM = encryptedKey
	cert.CertPath = certPath
	cert.KeyPath = keyPath

	// 8. 保存到数据库
	if cert.ID == 0 {
		if err := database.DB.Create(cert).Error; err != nil {
			// 清理文件
			os.Remove(certPath)
			os.Remove(keyPath)
			return fmt.Errorf("保存证书记录失败: %w", err)
		}
	} else {
		if err := database.DB.Save(cert).Error; err != nil {
			return fmt.Errorf("更新证书记录失败: %w", err)
		}
	}

	return nil
}

// GetCertificateByID 根据 ID 获取证书
func (m *Manager) GetCertificateByID(id uint) (*database.Certificate, error) {
	var cert database.Certificate
	if err := database.DB.First(&cert, id).Error; err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetAllCertificates 获取所有证书
func (m *Manager) GetAllCertificates() ([]database.Certificate, error) {
	var certs []database.Certificate
	if err := database.DB.Order("created_at desc").Find(&certs).Error; err != nil {
		return nil, err
	}
	return certs, nil
}

// GetMatchingCertificates 获取匹配指定域名的证书
func (m *Manager) GetMatchingCertificates(targetDomain string) ([]database.Certificate, error) {
	var allCerts []database.Certificate
	if err := database.DB.Find(&allCerts).Error; err != nil {
		return nil, err
	}

	var matched []database.Certificate
	for _, cert := range allCerts {
		domains := strings.Split(cert.Domains, ",")
		if MatchDomain(domains, targetDomain) {
			matched = append(matched, cert)
		}
	}

	return matched, nil
}

// DeleteCertificate 删除证书
func (m *Manager) DeleteCertificate(id uint) error {
	var cert database.Certificate
	if err := database.DB.First(&cert, id).Error; err != nil {
		return err
	}

	// 检查是否有域名正在使用此证书
	var count int64
	database.DB.Model(&database.Domain{}).Where("certificate_id = ?", id).Count(&count)
	if count > 0 {
		return fmt.Errorf("有 %d 个域名正在使用此证书，请先解除关联", count)
	}

	// 删除文件
	if cert.CertPath != "" {
		os.Remove(cert.CertPath)
	}
	if cert.KeyPath != "" {
		os.Remove(cert.KeyPath)
	}

	// 删除数据库记录
	return database.DB.Delete(&cert).Error
}

// GetDecryptedKey 获取解密后的私钥
func (m *Manager) GetDecryptedKey(cert *database.Certificate) (string, error) {
	return m.decrypt(cert.KeyPEM)
}

// ApplyCertToDomain 将证书应用到域名
func (m *Manager) ApplyCertToDomain(domainID uint, certID *uint) error {
	return database.DB.Model(&database.Domain{}).Where("id = ?", domainID).Update("certificate_id", certID).Error
}

// GetExpiringSoon 获取即将到期的证书 (days 天内)
func (m *Manager) GetExpiringSoon(days int) ([]database.Certificate, error) {
	deadline := time.Now().AddDate(0, 0, days)
	var certs []database.Certificate
	if err := database.DB.Where("not_after <= ? AND not_after > ?", deadline, time.Now()).Find(&certs).Error; err != nil {
		return nil, err
	}
	return certs, nil
}

// ApplyToSTARTTLS 将证书应用到 STARTTLS 配置
func (m *Manager) ApplyToSTARTTLS(certID uint) error {
	cert, err := m.GetCertificateByID(certID)
	if err != nil {
		return fmt.Errorf("获取证书失败: %w", err)
	}

	// 更新配置
	config.AppConfig.ReceiverTLSCert = cert.CertPath
	config.AppConfig.ReceiverTLSKey = cert.KeyPath
	config.AppConfig.ReceiverTLS = true

	// 保存配置
	return config.SaveConfig(config.AppConfig)
}
