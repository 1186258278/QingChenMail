// Package cert 提供 SSL 证书管理功能
package cert

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"time"
)

// CertInfo 证书解析后的信息
type CertInfo struct {
	Domains   []string  // 证书包含的域名 (CN + SANs)
	Issuer    string    // 颁发机构
	NotBefore time.Time // 生效时间
	NotAfter  time.Time // 到期时间
	IsCA      bool      // 是否为 CA 证书
}

// ParseCertPEM 解析 PEM 格式的证书，返回证书信息
func ParseCertPEM(certPEM string) (*CertInfo, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errors.New("无法解析 PEM 格式，请确保证书格式正确")
	}

	if block.Type != "CERTIFICATE" {
		return nil, errors.New("PEM 类型不是 CERTIFICATE")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("证书解析失败: " + err.Error())
	}

	info := &CertInfo{
		Issuer:    cert.Issuer.CommonName,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		IsCA:      cert.IsCA,
	}

	// 收集所有域名
	domainsMap := make(map[string]bool)
	if cert.Subject.CommonName != "" {
		domainsMap[strings.ToLower(cert.Subject.CommonName)] = true
	}
	for _, dns := range cert.DNSNames {
		domainsMap[strings.ToLower(dns)] = true
	}

	for domain := range domainsMap {
		info.Domains = append(info.Domains, domain)
	}

	// 验证证书是否在有效期内
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return info, errors.New("证书尚未生效")
	}
	if now.After(cert.NotAfter) {
		return info, errors.New("证书已过期")
	}

	return info, nil
}

// ValidateKeyPEM 验证私钥 PEM 格式是否有效
func ValidateKeyPEM(keyPEM string) error {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return errors.New("无法解析私钥 PEM 格式")
	}

	// 支持多种私钥类型
	switch block.Type {
	case "RSA PRIVATE KEY":
		_, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return errors.New("RSA 私钥解析失败: " + err.Error())
		}
	case "EC PRIVATE KEY":
		_, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return errors.New("EC 私钥解析失败: " + err.Error())
		}
	case "PRIVATE KEY":
		_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return errors.New("PKCS8 私钥解析失败: " + err.Error())
		}
	default:
		return errors.New("不支持的私钥类型: " + block.Type)
	}

	return nil
}

// MatchDomain 检查证书是否匹配指定域名
// 支持通配符匹配 (*.example.com 匹配 mail.example.com)
func MatchDomain(certDomains []string, targetDomain string) bool {
	targetDomain = strings.ToLower(targetDomain)

	for _, certDomain := range certDomains {
		certDomain = strings.ToLower(certDomain)

		// 精确匹配
		if certDomain == targetDomain {
			return true
		}

		// 通配符匹配
		if strings.HasPrefix(certDomain, "*.") {
			// *.example.com 匹配 mail.example.com, 但不匹配 a.b.example.com
			suffix := certDomain[1:] // .example.com
			if strings.HasSuffix(targetDomain, suffix) {
				prefix := strings.TrimSuffix(targetDomain, suffix)
				// 确保前缀不包含点（只匹配一级子域名）
				if !strings.Contains(prefix, ".") && prefix != "" {
					return true
				}
			}
		}
	}

	return false
}

// DaysUntilExpiry 计算证书距离到期还有多少天
func DaysUntilExpiry(notAfter time.Time) int {
	duration := time.Until(notAfter)
	return int(duration.Hours() / 24)
}

// GetExpiryStatus 获取证书到期状态
// 返回: "valid", "warning" (30天内), "critical" (7天内), "expired"
func GetExpiryStatus(notAfter time.Time) string {
	days := DaysUntilExpiry(notAfter)
	if days < 0 {
		return "expired"
	}
	if days <= 7 {
		return "critical"
	}
	if days <= 30 {
		return "warning"
	}
	return "valid"
}
