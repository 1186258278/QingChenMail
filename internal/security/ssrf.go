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

	// 仅允许 http/https 协议
	if u.Scheme != "http" && u.Scheme != "https" {
		return true
	}

	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // DNS 解析失败视为不安全
	}

	for _, ip := range ips {
		if IsInternalIP(ip) {
			return true
		}
	}
	return false
}

// IsInternalIP 检查单个 IP 是否属于内网/保留地址
func IsInternalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || // 0.0.0.0 / ::
		ip.IsMulticast()
}
