package receiver

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"
	"goemail/internal/mailer"
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
}

// StartReceiver 启动 SMTP 接收服务
func StartReceiver() {
	if !config.AppConfig.EnableReceiver {
		log.Println("[Receiver] Disabled, skipping...")
		return
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

	log.Printf("[Receiver] SMTP receiver started on %s", addr)

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
	
	session := &SMTPSession{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		remoteIP: conn.RemoteAddr().String(),
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

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 如果在 DATA 模式
		if session.inData {
			if line == "." {
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
				// 处理透明点 (dot stuffing)
				if strings.HasPrefix(line, "..") {
					line = line[1:]
				}
				session.data.WriteString(line)
				session.data.WriteString("\r\n")
			}
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
	// 简单响应 EHLO/HELO
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		s.send("501 Syntax error")
		return
	}
	
	cmd := strings.ToUpper(parts[0])
	if cmd == "EHLO" {
		s.send("250-GoEmail")
		s.send("250-SIZE 10485760")
		s.send("250 8BITMIME")
	} else {
		s.send("250 GoEmail")
	}
}

func (s *SMTPSession) handleMailFrom(line string) {
	// 解析 MAIL FROM:<address>
	addr := extractEmail(line[10:])
	if addr == "" {
		s.send("501 Syntax error in MAIL FROM")
		return
	}
	s.from = addr
	s.send("250 OK")
}

func (s *SMTPSession) handleRcptTo(line string) {
	// 解析 RCPT TO:<address>
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

	// 记录收件人，带上 domain 信息以便后续处理
	s.to = append(s.to, addr)
	_ = domain // 后续在 processEmail 中使用
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
	
	// 解析邮件头
	subject := extractHeader(rawData, "Subject")
	
	// 对每个收件人进行处理
	for _, rcpt := range s.to {
		// 1. 保存到 Inbox (始终保存，除非被黑名单拦截 - 这里暂无黑名单)
		inboxItem := database.Inbox{
			FromAddr: s.from,
			ToAddr:   rcpt,
			Subject:  subject,
			Body:     formatInboxBody(rawData), // 简单提取正文
			RawData:  rawData,
			RemoteIP: s.remoteIP,
			IsRead:   false,
		}
		database.DB.Create(&inboxItem)

		// 2. 查找转发规则并转发
		rule, _ := findForwardRule(rcpt)
		if rule == nil || !rule.Enabled {
			continue
		}

		// 创建转发请求
		forwardReq := mailer.SendRequest{
			From:    s.from,
			To:      rule.ForwardTo,
			Subject: fmt.Sprintf("[转发] %s", subject),
			Body:    formatForwardBody(s.from, rcpt, rawData),
		}

		// 加入发送队列
		_, err := mailer.SendEmailAsync(forwardReq)
		
		// 记录转发日志
		logEntry := database.ForwardLog{
			RuleID:    rule.ID,
			FromAddr:  s.from,
			ToAddr:    rcpt,
			ForwardTo: rule.ForwardTo,
			Subject:   subject,
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

// findForwardRule 查找匹配的转发规则
func findForwardRule(email string) (*database.ForwardRule, *database.Domain) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return nil, nil
	}
	localPart := strings.ToLower(parts[0])
	domainName := strings.ToLower(parts[1])

	// 查找域名
	var domain database.Domain
	if err := database.DB.Where("LOWER(name) = ?", domainName).First(&domain).Error; err != nil {
		return nil, nil
	}

	// 查找规则 (按优先级: exact > prefix > all)
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
	// 去掉 < > 包裹
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	// 处理可能的参数 (SIZE=xxx)
	if idx := strings.Index(s, " "); idx > 0 {
		s = s[:idx]
	}
	// 验证基本格式
	if !strings.Contains(s, "@") {
		return ""
	}
	return strings.ToLower(s)
}

// extractHeader 从原始邮件中提取头部字段
func extractHeader(rawData, header string) string {
	lines := strings.Split(rawData, "\n")
	headerLower := strings.ToLower(header + ":")
	
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), headerLower) {
			value := strings.TrimSpace(line[len(header)+1:])
			// 处理多行头部 (folding)
			for j := i + 1; j < len(lines); j++ {
				next := lines[j]
				if len(next) > 0 && (next[0] == ' ' || next[0] == '\t') {
					value += " " + strings.TrimSpace(next)
				} else {
					break
				}
			}
			return value
		}
		// 遇到空行表示头部结束
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	return ""
}

// formatForwardBody 格式化转发邮件正文
func formatForwardBody(from, originalTo, rawData string) string {
	body := extractBody(rawData)

	return fmt.Sprintf(`<div style="background:#f5f5f5; padding:15px; margin-bottom:20px; border-left:4px solid #2563eb; font-size:14px; color:#666;">
<p><strong>📧 转发邮件</strong></p>
<p>原始发件人: %s<br>
原始收件人: %s</p>
</div>
<div style="padding:10px 0;">
%s
</div>`, from, originalTo, body)
}

func checkPortOccupancy(port string) {
	log.Printf("[Receiver] Checking port %s usage...", port)
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", fmt.Sprintf("netstat -ano | findstr :%s", port))
		out, _ := cmd.Output()
		if len(out) > 0 {
			log.Printf("[Receiver] Port occupied details:\n%s", string(out))
			log.Println("[Receiver] Tip: Use 'tasklist | findstr <PID>' to find the process name.")
		}
	} else {
		cmd := exec.Command("lsof", "-i", ":"+port)
		out, _ := cmd.Output()
		if len(out) > 0 {
			log.Printf("[Receiver] Port occupied details:\n%s", string(out))
		}
	}
}

func formatInboxBody(rawData string) string {
	return extractBody(rawData)
}

// extractBody 简单提取邮件正文
func extractBody(rawData string) string {
	parts := strings.SplitN(rawData, "\r\n\r\n", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	parts = strings.SplitN(rawData, "\n\n", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
