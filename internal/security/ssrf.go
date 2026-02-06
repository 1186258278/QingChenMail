package security

import (
	"net"
	"net/url"
)

// IsInternalURL 检查 URL 是否指向内网 (SSRF 防护)
func IsInternalURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true // 解析失败视为不安全
	}

	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // DNS 解析失败视为不安全
	}

	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}
	return false
}
