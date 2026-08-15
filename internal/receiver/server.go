package receiver

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"
	"goemail/internal/mailer"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// SMTPSession 表示一个 SMTP 会话
type SMTPSession struct {
	conn       net.Conn
	reader     *bufio.Reader
	remoteIP   string
	from       string
	to         []string
	data       strings.Builder
	inData     bool
	tlsEnabled bool
}

// RateLimiter IP 速率限制器
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int           // 每分钟最大请求数
	window   time.Duration // 时间窗口
	stop     chan struct{}
	stopOnce sync.Once
}

var (
	rateLimiterPtr atomic.Pointer[RateLimiter]
	blacklistIPs   map[string]bool
	blacklistMu    sync.RWMutex
	tlsConfigPtr   atomic.Pointer[tls.Config]
)

// NewRateLimiter 创建速率限制器
func NewRateLimiter(limit int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   time.Minute,
		stop:     make(chan struct{}),
	}
	// 定期清理过期记录 (可通过 Stop 取消，防止 ReloadConfig 反复创建导致 goroutine 泄漏)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rl.stop:
				return
			case <-ticker.C:
				rl.cleanup()
			}
		}
	}()
	return rl
}

// Stop 停止速率限制器的后台清理 goroutine
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

// Allow 检查 IP 是否允许连接
func (rl *RateLimiter) Allow(ip string) bool {
	if rl.limit <= 0 {
		return true // 不限制
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// 清理旧记录
	var valid []time.Time
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.requests[ip] = valid

	// 检查是否超限
	if len(rl.requests[ip]) >= rl.limit {
		return false
	}

	rl.requests[ip] = append(rl.requests[ip], now)
	return true
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for ip, times := range rl.requests {
		var valid []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = valid
		}
	}
}

// 更新黑名单
func updateBlacklist() {
	blacklistMu.Lock()
	defer blacklistMu.Unlock()

	blacklistIPs = make(map[string]bool)
	if config.AppConfig.ReceiverBlacklist == "" {
		return
	}

	for _, ip := range strings.Split(config.AppConfig.ReceiverBlacklist, ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			blacklistIPs[ip] = true
		}
	}
}

// 检查 IP 是否在黑名单
func isBlacklisted(ip string) bool {
	// 提取纯 IP（去掉端口）
	host, _, _ := net.SplitHostPort(ip)
	if host == "" {
		host = ip
	}

	blacklistMu.RLock()
	defer blacklistMu.RUnlock()
	return blacklistIPs[host]
}

// 加载 TLS 配置
func loadTLSConfig() *tls.Config {
	if !config.AppConfig.ReceiverTLS {
		return nil
	}

	certFile := config.AppConfig.ReceiverTLSCert
	keyFile := config.AppConfig.ReceiverTLSKey

	// 如果没有配置独立证书，尝试使用 Web 服务器的证书
	if certFile == "" {
		certFile = config.AppConfig.CertFile
	}
	if keyFile == "" {
		keyFile = config.AppConfig.KeyFile
	}

	if certFile == "" || keyFile == "" {
		log.Println("[Receiver] TLS enabled but no certificate configured")
		return nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Printf("[Receiver] Failed to load TLS certificate: %v", err)
		return nil
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}

// StartReceiver 启动 SMTP 接收服务
func StartReceiver() {
	if !config.AppConfig.EnableReceiver {
		log.Println("[Receiver] Disabled, skipping...")
		return
	}

	// 初始化速率限制器
	rateLimiterPtr.Store(NewRateLimiter(config.AppConfig.ReceiverRateLimit))

	// 加载黑名单
	updateBlacklist()

	// 加载 TLS 配置
	tlsConfigPtr.Store(loadTLSConfig())
	if tc := tlsConfigPtr.Load(); tc != nil {
		log.Println("[Receiver] STARTTLS enabled")
	}

	port := config.AppConfig.ReceiverPort
	if port == "" {
		port = "25"
	}

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Receiver] Failed to start on %s: %v", addr, err)
		if strings.Contains(err.Error(), "address already in use") {
			checkPortOccupancy(port)
		}
		return
	}

	log.Printf("[Receiver] SMTP receiver started on %s (rate limit: %d/min)", addr, config.AppConfig.ReceiverRateLimit)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("[Receiver] Accept error: %v", err)
				continue
			}
			go handleConnection(conn)
		}
	}()
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteIP := conn.RemoteAddr().String()

	// 检查黑名单
	if isBlacklisted(remoteIP) {
		log.Printf("[Receiver] Blocked blacklisted IP: %s", remoteIP)
		conn.Write([]byte("554 Your IP is blocked\r\n"))
		return
	}

	// 检查速率限制
	if rl := rateLimiterPtr.Load(); rl != nil && !rl.Allow(remoteIP) {
		log.Printf("[Receiver] Rate limit exceeded for IP: %s", remoteIP)
		conn.Write([]byte("421 Too many connections, try again later\r\n"))
		return
	}

	session := &SMTPSession{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		remoteIP: remoteIP,
		to:       make([]string, 0),
	}

	// 设置超时
	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	// 发送欢迎消息
	session.send("220 GoEmail SMTP Ready")

	for {
		line, err := session.reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[Receiver] Read error from %s: %v", session.remoteIP, err)
			}
			return
		}

		// 如果在 DATA 模式：正文行只剥离行尾 \r\n，保留行内空白和空行，
		// 避免破坏邮件正文字节 (影响 DKIM 校验与 quoted-printable 内容)
		if session.inData {
			bodyLine := strings.TrimRight(line, "\r\n")
			if bodyLine == "." {
				// 数据结束，处理邮件
				session.inData = false
				if err := session.processEmail(); err != nil {
					session.send("550 Failed to process email: " + err.Error())
				} else {
					session.send("250 OK: Message queued for forwarding")
				}
				// 重置会话
				session.from = ""
				session.to = make([]string, 0)
				session.data.Reset()
			} else {
				// 检查邮件大小限制 (配置 <=0 时使用默认 10MB，防止关闭限制)
				maxSize := config.AppConfig.ReceiverMaxMsgSize * 1024
				if maxSize <= 0 {
					maxSize = 10240 * 1024
				}
				if session.data.Len()+len(bodyLine) > maxSize {
					session.inData = false
					session.send("552 Message size exceeds limit")
					session.data.Reset()
					continue
				}
				// 处理透明点 (dot stuffing)
				if strings.HasPrefix(bodyLine, "..") {
					bodyLine = bodyLine[1:]
				}
				session.data.WriteString(bodyLine)
				session.data.WriteString("\r\n")
			}
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析命令
		cmd := strings.ToUpper(line)
		if strings.HasPrefix(cmd, "HELO") || strings.HasPrefix(cmd, "EHLO") {
			session.handleHelo(line)
		} else if strings.HasPrefix(cmd, "MAIL FROM:") {
			session.handleMailFrom(line)
		} else if strings.HasPrefix(cmd, "RCPT TO:") {
			session.handleRcptTo(line)
		} else if cmd == "DATA" {
			session.handleData()
		} else if cmd == "STARTTLS" {
			session.handleStartTLS()
		} else if cmd == "QUIT" {
			session.send("221 Bye")
			return
		} else if cmd == "RSET" {
			session.from = ""
			session.to = make([]string, 0)
			session.data.Reset()
			session.send("250 OK")
		} else if cmd == "NOOP" {
			session.send("250 OK")
		} else {
			session.send("502 Command not implemented")
		}
	}
}

func (s *SMTPSession) send(msg string) {
	s.conn.Write([]byte(msg + "\r\n"))
}

func (s *SMTPSession) handleHelo(line string) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		s.send("501 Syntax error")
		return
	}
	
	cmd := strings.ToUpper(parts[0])
	if cmd == "EHLO" {
		s.send("250-GoEmail")
		s.send(fmt.Sprintf("250-SIZE %d", config.AppConfig.ReceiverMaxMsgSize*1024))
		s.send("250-8BITMIME")
		if tc := tlsConfigPtr.Load(); tc != nil && !s.tlsEnabled {
			s.send("250-STARTTLS")
		}
		s.send("250 OK")
	} else {
		s.send("250 GoEmail")
	}
}

func (s *SMTPSession) handleStartTLS() {
	tc := tlsConfigPtr.Load()
	if tc == nil {
		s.send("454 TLS not available")
		return
	}
	if s.tlsEnabled {
		s.send("503 TLS already active")
		return
	}

	s.send("220 Ready to start TLS")

	// 升级到 TLS
	tlsConn := tls.Server(s.conn, tc)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[Receiver] TLS handshake failed from %s: %v", s.remoteIP, err)
		return
	}

	s.conn = tlsConn
	s.reader = bufio.NewReader(tlsConn)
	s.tlsEnabled = true

	// 重置会话状态
	s.from = ""
	s.to = make([]string, 0)
	s.data.Reset()

	log.Printf("[Receiver] TLS connection established from %s", s.remoteIP)
}

func (s *SMTPSession) handleMailFrom(line string) {
	// 检查是否强制要求 TLS
	if config.AppConfig.ReceiverRequireTLS && !s.tlsEnabled {
		s.send("530 Must issue STARTTLS command first")
		return
	}

	addr := extractEmail(line[10:])
	if addr == "" {
		s.send("501 Syntax error in MAIL FROM")
		return
	}
	s.from = addr
	// 新事务开始：清空上一事务的收件人，避免串事务 (RFC 5321)
	s.to = make([]string, 0)
	s.send("250 OK")
}

func (s *SMTPSession) handleRcptTo(line string) {
	// 必须先执行 MAIL FROM (RFC 5321 事务顺序)
	if s.from == "" {
		s.send("503 Bad sequence of commands: MAIL FROM required first")
		return
	}

	addr := extractEmail(line[8:])
	if addr == "" {
		s.send("501 Syntax error in RCPT TO")
		return
	}

	// 检查是否有匹配的转发规则
	rule, domain := findForwardRule(addr)
	if rule == nil {
		s.send("550 Recipient not accepted")
		return
	}

	s.to = append(s.to, addr)
	_ = domain
	s.send("250 OK")
}

func (s *SMTPSession) handleData() {
	if s.from == "" {
		s.send("503 Need MAIL command first")
		return
	}
	if len(s.to) == 0 {
		s.send("503 Need RCPT command first")
		return
	}
	s.inData = true
	s.send("354 Start mail input; end with <CRLF>.<CRLF>")
}

func (s *SMTPSession) processEmail() error {
	rawData := s.data.String()
	
	// 解析 MIME 邮件
	parsed := parseMIMEMessage(rawData)

	// 垃圾邮件检测
	isSpam := false
	spamReason := ""
	if config.AppConfig.ReceiverSpamFilter {
		isSpam, spamReason = detectSpam(s.from, parsed.Subject, parsed.Body)
		if isSpam {
			log.Printf("[Receiver] Spam detected from %s: %s", s.from, spamReason)
		}
	}
	
	// 对每个收件人进行处理
	for _, rcpt := range s.to {
		// 1. 保存到 Inbox (垃圾邮件也保存，但标记 Tags)
		tags := ""
		if isSpam {
			tags = `["spam"]`
		}
		inboxItem := database.Inbox{
			FromAddr: s.from,
			ToAddr:   rcpt,
			Subject:  parsed.Subject,
			Body:     parsed.Body,
			RawData:  rawData,
			RemoteIP: s.remoteIP,
			IsRead:   false,
			Tags:     tags,
		}
		database.DB.Create(&inboxItem)

		// 保存附件
		for _, att := range parsed.Attachments {
			saveInboxAttachment(inboxItem.ID, att)
		}

		// 2. 查找转发规则并转发
		rule, _ := findForwardRule(rcpt)
		if rule == nil || !rule.Enabled {
			continue
		}

		// 创建转发请求
		forwardReq := mailer.SendRequest{
			From:    s.from,
			To:      rule.ForwardTo,
			Subject: fmt.Sprintf("[转发] %s", parsed.Subject),
			Body:    formatForwardBody(s.from, rcpt, parsed.Body),
		}

		_, err := mailer.SendEmailAsync(forwardReq)
		
		logEntry := database.ForwardLog{
			RuleID:    rule.ID,
			FromAddr:  s.from,
			ToAddr:    rcpt,
			ForwardTo: rule.ForwardTo,
			Subject:   parsed.Subject,
			RemoteIP:  s.remoteIP,
		}

		if err != nil {
			logEntry.Status = "failed"
			logEntry.ErrorMsg = err.Error()
		} else {
			logEntry.Status = "success"
		}

		database.DB.Create(&logEntry)
	}

	return nil
}

// ParsedEmail 解析后的邮件结构
type ParsedEmail struct {
	Subject     string
	Body        string
	ContentType string
	Attachments []ParsedAttachment
}

// ParsedAttachment 解析后的附件
type ParsedAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// parseMIMEMessage 解析 MIME 格式邮件
func parseMIMEMessage(rawData string) ParsedEmail {
	result := ParsedEmail{}

	// 分离头部和正文
	parts := strings.SplitN(rawData, "\r\n\r\n", 2)
	if len(parts) != 2 {
		parts = strings.SplitN(rawData, "\n\n", 2)
	}
	if len(parts) != 2 {
		return result
	}

	headerPart := parts[0]
	bodyPart := parts[1]

	// 解析头部
	headers := parseHeaders(headerPart)
	result.Subject = decodeRFC2047(headers["subject"])
	result.ContentType = headers["content-type"]

	// 解析正文
	contentType := strings.ToLower(headers["content-type"])
	transferEncoding := strings.ToLower(headers["content-transfer-encoding"])

	if strings.HasPrefix(contentType, "multipart/") {
		// 解析多部分邮件
		boundary := extractBoundary(contentType)
		if boundary != "" {
			parts, attachments := parseMultipart(bodyPart, boundary)
			result.Body = parts
			result.Attachments = attachments
		}
	} else {
		// 单部分邮件
		result.Body = decodeBody(bodyPart, transferEncoding, getCharset(contentType))
	}

	return result
}

// parseHeaders 解析邮件头
func parseHeaders(headerPart string) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(headerPart, "\n")

	var currentKey, currentValue string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		
		// 折叠行（以空白开头）
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			currentValue += " " + strings.TrimSpace(line)
			continue
		}

		// 保存上一个头部
		if currentKey != "" {
			headers[strings.ToLower(currentKey)] = currentValue
		}

		// 解析新头部
		idx := strings.Index(line, ":")
		if idx > 0 {
			currentKey = line[:idx]
			currentValue = strings.TrimSpace(line[idx+1:])
		}
	}

	// 保存最后一个头部
	if currentKey != "" {
		headers[strings.ToLower(currentKey)] = currentValue
	}

	return headers
}

// decodeRFC2047 解码 RFC 2047 编码的头部
func decodeRFC2047(s string) string {
	decoder := new(mime.WordDecoder)
	decoded, err := decoder.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// extractBoundary 从 Content-Type 中提取 boundary
func extractBoundary(contentType string) string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return params["boundary"]
}

// getCharset 从 Content-Type 中提取字符集
func getCharset(contentType string) string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "utf-8"
	}
	charset := params["charset"]
	if charset == "" {
		return "utf-8"
	}
	return strings.ToLower(charset)
}

// parseMultipart 解析多部分邮件
func parseMultipart(body, boundary string) (string, []ParsedAttachment) {
	return parseMultipartDepth(body, boundary, 0)
}

// maxMultipartDepth 嵌套 multipart 最大递归深度，防止深层嵌套造成栈溢出/OOM (zip-bomb 类攻击)
const maxMultipartDepth = 20

func parseMultipartDepth(body, boundary string, depth int) (string, []ParsedAttachment) {
	var textContent string
	var attachments []ParsedAttachment

	if depth > maxMultipartDepth {
		return textContent, attachments
	}

	reader := multipart.NewReader(strings.NewReader(body), boundary)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		contentType := part.Header.Get("Content-Type")
		contentDisp := part.Header.Get("Content-Disposition")
		transferEncoding := strings.ToLower(part.Header.Get("Content-Transfer-Encoding"))

		data, _ := io.ReadAll(part)
		decodedData := decodeBodyBytes(data, transferEncoding)

		// 判断是附件还是正文
		if strings.Contains(contentDisp, "attachment") || strings.Contains(contentDisp, "filename") {
			filename := extractFilename(contentDisp, contentType)
			attachments = append(attachments, ParsedAttachment{
				Filename:    filename,
				ContentType: contentType,
				Data:        decodedData,
			})
		} else if strings.HasPrefix(strings.ToLower(contentType), "text/") {
			charset := getCharset(contentType)
			textContent += decodeCharset(string(decodedData), charset)
		} else if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
			// 嵌套多部分 (限制递归深度)
			nestedBoundary := extractBoundary(contentType)
			if nestedBoundary != "" {
				nestedText, nestedAtts := parseMultipartDepth(string(data), nestedBoundary, depth+1)
				textContent += nestedText
				attachments = append(attachments, nestedAtts...)
			}
		}
	}

	return textContent, attachments
}

// extractFilename 从 Content-Disposition 或 Content-Type 提取文件名
func extractFilename(contentDisp, contentType string) string {
	// 尝试从 Content-Disposition 提取
	_, params, err := mime.ParseMediaType(contentDisp)
	if err == nil {
		if name := params["filename"]; name != "" {
			return decodeRFC2047(name)
		}
	}

	// 尝试从 Content-Type 提取
	_, params, err = mime.ParseMediaType(contentType)
	if err == nil {
		if name := params["name"]; name != "" {
			return decodeRFC2047(name)
		}
	}

	return "attachment"
}

// decodeBody 解码正文
func decodeBody(body, encoding, charset string) string {
	decoded := decodeBodyBytes([]byte(body), encoding)
	return decodeCharset(string(decoded), charset)
}

// decodeBodyBytes 解码传输编码
func decodeBodyBytes(data []byte, encoding string) []byte {
	switch encoding {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return data
		}
		return decoded
	case "quoted-printable":
		reader := quotedprintable.NewReader(bytes.NewReader(data))
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return data
		}
		return decoded
	default:
		return data
	}
}

// decodeCharset 解码字符集
func decodeCharset(s, charset string) string {
	switch strings.ToLower(charset) {
	case "gb2312", "gbk", "gb18030":
		decoded, err := simplifiedchinese.GBK.NewDecoder().String(s)
		if err != nil {
			return s
		}
		return decoded
	case "iso-8859-1", "latin1":
		decoded, err := charmap.ISO8859_1.NewDecoder().String(s)
		if err != nil {
			return s
		}
		return decoded
	case "windows-1252":
		decoded, err := charmap.Windows1252.NewDecoder().String(s)
		if err != nil {
			return s
		}
		return decoded
	default:
		return s
	}
}

// saveInboxAttachment 保存收件箱附件
func saveInboxAttachment(inboxID uint, att ParsedAttachment) {
	if len(att.Data) == 0 {
		return
	}

	// 创建存储目录
	saveDir := "data/inbox_attachments"
	os.MkdirAll(saveDir, 0755)

	// 生成安全文件名
	ext := filepath.Ext(att.Filename)
	if ext == "" {
		ext = ".dat"
	}
	newFilename := fmt.Sprintf("%d_%d%s", inboxID, time.Now().UnixNano(), ext)
	localPath := filepath.Join(saveDir, newFilename)

	if err := os.WriteFile(localPath, att.Data, 0644); err != nil {
		log.Printf("[Receiver] Failed to save attachment: %v", err)
		return
	}

	// 记录到数据库
	dbFile := database.AttachmentFile{
		Filename:    att.Filename,
		FilePath:    localPath,
		FileSize:    int64(len(att.Data)),
		ContentType: att.ContentType,
		Source:      "inbox",
		RelatedTo:   fmt.Sprintf("inbox:%d", inboxID),
	}
	database.DB.Create(&dbFile)
}

// findForwardRule 查找匹配的转发规则
func findForwardRule(email string) (*database.ForwardRule, *database.Domain) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return nil, nil
	}
	localPart := strings.ToLower(parts[0])
	domainName := strings.ToLower(parts[1])

	var domain database.Domain
	if err := database.DB.Where("LOWER(name) = ?", domainName).First(&domain).Error; err != nil {
		return nil, nil
	}

	var rules []database.ForwardRule
	database.DB.Where("domain_id = ? AND enabled = ?", domain.ID, true).Find(&rules)

	// 精确匹配
	for _, r := range rules {
		if r.MatchType == "exact" && strings.ToLower(r.MatchAddr) == localPart {
			return &r, &domain
		}
	}

	// 前缀匹配
	for _, r := range rules {
		if r.MatchType == "prefix" && strings.HasPrefix(localPart, strings.ToLower(r.MatchAddr)) {
			return &r, &domain
		}
	}

	// 全部匹配
	for _, r := range rules {
		if r.MatchType == "all" {
			return &r, &domain
		}
	}

	return nil, nil
}

// extractEmail 从 SMTP 命令中提取邮箱地址
func extractEmail(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	if idx := strings.Index(s, " "); idx > 0 {
		s = s[:idx]
	}
	if !strings.Contains(s, "@") {
		return ""
	}
	return strings.ToLower(s)
}

// formatForwardBody 格式化转发邮件正文
func formatForwardBody(from, originalTo, body string) string {
	return fmt.Sprintf(`<div style="background:#f5f5f5; padding:15px; margin-bottom:20px; border-left:4px solid #2563eb; font-size:14px; color:#666;">
<p><strong>📧 转发邮件</strong></p>
<p>原始发件人: %s<br>
原始收件人: %s</p>
</div>
<div style="padding:10px 0;">
%s
</div>`, from, originalTo, body)
}

// isValidPort 验证端口号是否为纯数字
func isValidPort(port string) bool {
	if port == "" {
		return false
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func checkPortOccupancy(port string) {
	if !isValidPort(port) {
		log.Printf("[Receiver] Invalid port number: %s", port)
		return
	}

	log.Printf("[Receiver] Checking port %s usage...", port)
	if runtime.GOOS == "windows" {
		cmd := exec.Command("netstat", "-ano")
		out, err := cmd.Output()
		if err != nil {
			log.Printf("[Receiver] Failed to run netstat: %v", err)
			return
		}
		
		lines := strings.Split(string(out), "\n")
		targetPort := ":" + port
		var matchedLines []string
		for _, line := range lines {
			if strings.Contains(line, targetPort) && strings.Contains(line, "LISTENING") {
				matchedLines = append(matchedLines, strings.TrimSpace(line))
			}
		}
		
		if len(matchedLines) > 0 {
			log.Printf("[Receiver] Port occupied details:\n%s", strings.Join(matchedLines, "\n"))
		}
	} else {
		cmd := exec.Command("lsof", "-i", ":"+port)
		out, _ := cmd.Output()
		if len(out) > 0 {
			log.Printf("[Receiver] Port occupied details:\n%s", string(out))
		}
	}
}

// ReloadConfig 重新加载配置（供外部调用）
func ReloadConfig() {
	// 停止旧的速率限制器后台 goroutine，防止泄漏
	if old := rateLimiterPtr.Swap(NewRateLimiter(config.AppConfig.ReceiverRateLimit)); old != nil {
		old.Stop()
	}
	updateBlacklist()
	tlsConfigPtr.Store(loadTLSConfig())
	log.Println("[Receiver] Configuration reloaded")
}

// detectSpam 检测垃圾邮件
// 返回 (是否垃圾邮件, 原因)
func detectSpam(from, subject, body string) (bool, string) {
	// 常见垃圾邮件关键词 (中英文)
	spamKeywords := []string{
		// 英文关键词
		"viagra", "cialis", "lottery", "winner", "congratulations",
		"nigerian prince", "inheritance", "million dollars",
		"click here", "act now", "limited time", "free money",
		"make money fast", "work from home", "earn cash",
		"no obligation", "risk free", "credit card",
		"penis enlargement", "weight loss", "diet pills",
		// 中文关键词
		"彩票中奖", "恭喜您获得", "免费赠送", "点击领取",
		"低价出售", "发票代开", "刷单兼职", "网赚项目",
		"色情", "赌博", "博彩", "六合彩",
	}

	// 转小写进行匹配
	lowerSubject := strings.ToLower(subject)
	lowerBody := strings.ToLower(body)
	lowerFrom := strings.ToLower(from)

	// 检查关键词
	for _, keyword := range spamKeywords {
		if strings.Contains(lowerSubject, keyword) {
			return true, "subject contains spam keyword: " + keyword
		}
		if strings.Contains(lowerBody, keyword) {
			return true, "body contains spam keyword: " + keyword
		}
	}

	// 检查可疑发件人模式
	suspiciousPatterns := []string{
		"noreply@", "no-reply@", "donotreply@",
		"admin@", "support@", "info@",
	}
	for _, pattern := range suspiciousPatterns {
		if strings.HasPrefix(lowerFrom, pattern) {
			// 这些模式不一定是垃圾邮件，只是可疑，跳过
			break
		}
	}

	// 检查大量链接 (超过 5 个链接视为可疑)
	linkCount := strings.Count(lowerBody, "http://") + strings.Count(lowerBody, "https://")
	if linkCount > 10 {
		return true, fmt.Sprintf("too many links: %d", linkCount)
	}

	// 检查全大写主题 (营销邮件特征)
	if len(subject) > 10 && subject == strings.ToUpper(subject) {
		return true, "subject is all uppercase"
	}

	return false, ""
}
