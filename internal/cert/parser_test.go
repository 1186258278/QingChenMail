package cert

import (
	"testing"
	"time"
)

// 测试用的自签名证书 (仅用于测试，实际已过期)
const testCertPEM = `-----BEGIN CERTIFICATE-----
MIICpDCCAYwCCQDU+pQ4P1OMGzANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwHhcNMjQwMTAxMDAwMDAwWhcNMjUwMTAxMDAwMDAwWjAUMRIwEAYD
VQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC7
o5e7Tz7VrRXJp9WmZqF0z0W1b8K7KWBqLM4wQXTL7f7dTn5kXQ8dJ9zLJ2X9K7l+
pKc7F1xWZ6NHr9V8X4K3X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4
X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4
X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4
X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4
X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4X4AgMBAAEwDQYJKoZIhvcNAQEL
BQADggEBAFJ9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9J9
-----END CERTIFICATE-----`

const testKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAu6OXu08+1a0VyafVpmahAM9FtW/CuylgaizOMEF0y+3+3U5+
ZF0PHSfcyydl/Su5fqSnOxdcVmejR6/VfF+Ct1+F+F+F+F+F+F+F+F+F+F+F+F+F
+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F
+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F
+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F
+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+F+AgMBAAEC
-----END RSA PRIVATE KEY-----`

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		name         string
		certDomains  []string
		targetDomain string
		expected     bool
	}{
		{
			name:         "精确匹配",
			certDomains:  []string{"mail.example.com"},
			targetDomain: "mail.example.com",
			expected:     true,
		},
		{
			name:         "精确匹配 - 大小写不敏感",
			certDomains:  []string{"Mail.Example.Com"},
			targetDomain: "mail.example.com",
			expected:     true,
		},
		{
			name:         "通配符匹配 - 一级子域名",
			certDomains:  []string{"*.example.com"},
			targetDomain: "mail.example.com",
			expected:     true,
		},
		{
			name:         "通配符匹配 - 另一个一级子域名",
			certDomains:  []string{"*.example.com"},
			targetDomain: "smtp.example.com",
			expected:     true,
		},
		{
			name:         "通配符不匹配 - 二级子域名",
			certDomains:  []string{"*.example.com"},
			targetDomain: "a.b.example.com",
			expected:     false,
		},
		{
			name:         "通配符不匹配 - 根域名",
			certDomains:  []string{"*.example.com"},
			targetDomain: "example.com",
			expected:     false,
		},
		{
			name:         "多域名证书 - 匹配其中一个",
			certDomains:  []string{"example.com", "*.example.com", "mail.example.org"},
			targetDomain: "mail.example.org",
			expected:     true,
		},
		{
			name:         "不匹配",
			certDomains:  []string{"mail.example.com"},
			targetDomain: "smtp.example.com",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchDomain(tt.certDomains, tt.targetDomain)
			if result != tt.expected {
				t.Errorf("MatchDomain(%v, %s) = %v, want %v", tt.certDomains, tt.targetDomain, result, tt.expected)
			}
		})
	}
}

func TestDaysUntilExpiry(t *testing.T) {
	tests := []struct {
		name     string
		notAfter time.Time
		minDays  int
		maxDays  int
	}{
		{
			name:     "30天后到期",
			notAfter: time.Now().AddDate(0, 0, 30),
			minDays:  29,
			maxDays:  31,
		},
		{
			name:     "已过期",
			notAfter: time.Now().AddDate(0, 0, -1),
			minDays:  -2,
			maxDays:  0,
		},
		{
			name:     "今天到期",
			notAfter: time.Now(),
			minDays:  -1,
			maxDays:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days := DaysUntilExpiry(tt.notAfter)
			if days < tt.minDays || days > tt.maxDays {
				t.Errorf("DaysUntilExpiry() = %d, want between %d and %d", days, tt.minDays, tt.maxDays)
			}
		})
	}
}

func TestGetExpiryStatus(t *testing.T) {
	tests := []struct {
		name     string
		notAfter time.Time
		expected string
	}{
		{
			name:     "有效 - 90天后到期",
			notAfter: time.Now().AddDate(0, 0, 90),
			expected: "valid",
		},
		{
			name:     "警告 - 25天后到期",
			notAfter: time.Now().AddDate(0, 0, 25),
			expected: "warning",
		},
		{
			name:     "严重 - 5天后到期",
			notAfter: time.Now().AddDate(0, 0, 5),
			expected: "critical",
		},
		{
			name:     "过期",
			notAfter: time.Now().AddDate(0, 0, -1),
			expected: "expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := GetExpiryStatus(tt.notAfter)
			if status != tt.expected {
				t.Errorf("GetExpiryStatus() = %s, want %s", status, tt.expected)
			}
		})
	}
}

func TestValidateKeyPEM_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		keyPEM  string
		wantErr bool
	}{
		{
			name:    "空字符串",
			keyPEM:  "",
			wantErr: true,
		},
		{
			name:    "非 PEM 格式",
			keyPEM:  "this is not a pem",
			wantErr: true,
		},
		{
			name:    "无效的 PEM 块类型",
			keyPEM:  "-----BEGIN INVALID-----\ntest\n-----END INVALID-----",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKeyPEM(tt.keyPEM)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKeyPEM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseCertPEM_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		certPEM string
		wantErr bool
	}{
		{
			name:    "空字符串",
			certPEM: "",
			wantErr: true,
		},
		{
			name:    "非 PEM 格式",
			certPEM: "this is not a certificate",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCertPEM(tt.certPEM)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCertPEM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
