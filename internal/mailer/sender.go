package mailer

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"sort"
	"strings"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/wneessen/go-mail"
)

// Attachment 附件结构
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"` // e.g. "application/pdf"
	Content     string `json:"content"`      // Base64 encoded content
	URL         string `json:"url"`          // Optional: Download from URL
}

// SendRequest 定义发送请求结构
type SendRequest struct {
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body"`
	Attachments []Attachment           `json:"attachments"`
	ChannelID   uint                   `json:"channel_id"` // 0 = Direct, >0 = SMTP Config ID
	TemplateID  uint                   `json:"template_id"`
	Variables   map[string]interface{} `json:"variables"`
	TrackingID  string                 `json:"tracking_id"` // 用于追踪
}

// SendEmail 统一发送入口
func SendEmail(req SendRequest) error {
	// 1. 准备发件人
	fromAddr := req.From
	if fromAddr == "" {
		fromAddr = fmt.Sprintf("noreply@%s", config.AppConfig.Domain)
	}

	// 2. 使用 go-mail 构建标准 MIME 消息
	m := mail.NewMsg()
	if err := m.From(fromAddr); err != nil {
		return logAndReturnError(req, "invalid_from", err)
	}
	if err := m.To(req.To); err != nil {
		return logAndReturnError(req, "invalid_to", err)
	}
	m.Subject(req.Subject)
	m.SetBodyString(mail.TypeTextHTML, req.Body)
	m.SetDate()      // 显式设置日期，确保签名时一致
	m.SetMessageID() // 显式设置 Message-ID

	// 处理附件
	for _, att := range req.Attachments {
		var data []byte
		var err error

		if att.Content != "" {
			// 1. 优先使用 Base64 内容
			data, err = base64.StdEncoding.DecodeString(att.Content)
			if err != nil {
				return logAndReturnError(req, "invalid_attachment_base64", err)
			}
		} else if att.URL != "" {
			// 2. 检查是否为本地文件 (由 Handler 预处理并保存)
			if strings.HasPrefix(att.URL, "local://") {
				localPath := strings.TrimPrefix(att.URL, "local://")
				// 读取本地文件
				// 注意：这里没有复杂的安全校验，因为假定 local:// 只能由后端内部生成
				fileData, err := os.ReadFile(localPath)
				if err != nil {
					return logAndReturnError(req, fmt.Sprintf("failed_read_local_attachment: %s", localPath), err)
				}
				data = fileData
			} else {
				// 3. 尝试从远程 URL 下载
				resp, err := http.Get(att.URL)
				if err != nil {
					return logAndReturnError(req, fmt.Sprintf("failed_download_attachment: %s", att.URL), err)
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					return logAndReturnError(req, fmt.Sprintf("failed_download_attachment_status_%d", resp.StatusCode), fmt.Errorf("status %d", resp.StatusCode))
				}
				
				data, err = io.ReadAll(resp.Body)
				if err != nil {
					return logAndReturnError(req, "failed_read_attachment_body", err)
				}
			}
		} else {
			continue // 跳过无效附件
		}
		
		// 自动推断 ContentType 或使用提供的
		contentType := mail.TypeAppOctetStream
		if att.ContentType != "" {
			contentType = mail.ContentType(att.ContentType)
		}
		
		m.AttachReader(att.Filename, bytes.NewReader(data), mail.WithFileContentType(contentType))
	}

	// 3. 获取原始字节流
	var msgBuffer bytes.Buffer
	if _, err := m.WriteTo(&msgBuffer); err != nil {
		return logAndReturnError(req, "msg_build_failed", err)
	}
	msgBytes := msgBuffer.Bytes()

	// 4. DKIM 签名 (仅当 Direct Send 时，且配置了域名私钥)
	senderDomain := extractDomain(fromAddr)
	var dkimPrivKeyPEM string
	var dkimSelector string

	// 尝试从数据库查找该域名的配置
	if req.ChannelID == 0 { // 仅直连模式需要自己签名
		var domainConfig database.Domain
		if err := database.DB.Where("name = ?", senderDomain).First(&domainConfig).Error; err == nil && domainConfig.DKIMPrivateKey != "" {
			dkimPrivKeyPEM = domainConfig.DKIMPrivateKey
			dkimSelector = domainConfig.DKIMSelector
		} else if senderDomain == config.AppConfig.Domain && config.AppConfig.DKIMPrivateKey != "" {
			// 兜底：使用配置文件中的默认 DKIM
			dkimPrivKeyPEM = config.AppConfig.DKIMPrivateKey
			dkimSelector = config.AppConfig.DKIMSelector
		}

		if dkimPrivKeyPEM != "" {
			// 解析私钥
			block, _ := pem.Decode([]byte(dkimPrivKeyPEM))
			if block != nil {
				privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err == nil {
					// 配置 DKIM 签名选项
					options := &dkim.SignOptions{
						Domain:   senderDomain,
						Selector: dkimSelector,
						Signer:   privKey,
					}
					
					var signedBuffer bytes.Buffer
					// dkim.Sign 读取 reader，计算签名，并将结果（Header + Body）写入 writer
					// 注意：dkim.Sign 函数签名通常是 Sign(w io.Writer, r io.Reader, options *SignOptions) error
					if err := dkim.Sign(&signedBuffer, bytes.NewReader(msgBytes), options); err == nil {
						msgBytes = signedBuffer.Bytes() // 替换为已签名内容
					} else {
						// 记录 DKIM 签名失败，但不阻止发送
						// 在实际生产中应该记录到日志文件
						// fmt.Printf("DKIM sign failed: %v\n", err)
					}
				}
			}
		}
	}

	// 5. 选择发送通道 (含故障转移)
	if req.ChannelID > 0 {
		// 指定通道
		return sendByRelay(req, fromAddr, req.To, msgBytes, req.ChannelID)
	} else {
		// 自动路由：优先尝试默认通道，失败则尝试 Direct
		var defaultSMTP database.SMTPConfig
		if err := database.DB.Where("is_default = ?", true).First(&defaultSMTP).Error; err == nil {
			if err := sendWithSMTPConfig(req, fromAddr, req.To, msgBytes, defaultSMTP); err == nil {
				return nil
			}
			// 默认通道失败，继续尝试 Direct
		}
		// Direct Send
		return sendByDirect(req, fromAddr, req.To, msgBytes)
	}
}

// sendByRelay 包装器
func sendByRelay(req SendRequest, from, to string, msg []byte, channelID uint) error {
	var cfg database.SMTPConfig
	if err := database.DB.First(&cfg, channelID).Error; err != nil {
		return logAndReturnError(req, "smtp_config_not_found", err)
	}
	return sendWithSMTPConfig(req, from, to, msg, cfg)
}

// sendWithSMTPConfig 核心 SMTP 发送逻辑
func sendWithSMTPConfig(req SendRequest, from, to string, msg []byte, cfg database.SMTPConfig) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	// 处理 SSL/TLS
	if cfg.SSL {
		// 隐式 SSL (通常端口 465)
		tlsConfig := &tls.Config{InsecureSkipVerify: true, ServerName: cfg.Host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return logAndReturnError(req, "smtp_tls_dial_failed", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return logAndReturnError(req, "smtp_client_create_failed", err)
		}
		defer c.Quit()

		if err = c.Auth(auth); err != nil {
			return logAndReturnError(req, "smtp_auth_failed", err)
		}
		if err = c.Mail(from); err != nil {
			return logAndReturnError(req, "smtp_mail_from_failed", err)
		}
		if err = c.Rcpt(to); err != nil {
			return logAndReturnError(req, "smtp_rcpt_to_failed", err)
		}
		w, err := c.Data()
		if err != nil {
			return logAndReturnError(req, "smtp_data_failed", err)
		}
		if _, err = w.Write(msg); err != nil {
			return logAndReturnError(req, "smtp_write_failed", err)
		}
		if err = w.Close(); err != nil {
			return logAndReturnError(req, "smtp_close_failed", err)
		}
	} else {
		// StartTLS 或 明文 (通常端口 25/587)
		if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
			return logAndReturnError(req, "smtp_send_error", err)
		}
	}

	logSuccess(req, fmt.Sprintf("smtp_%d", cfg.ID))
	return nil
}

// sendByDirect 直接投递
func sendByDirect(req SendRequest, from, to string, msg []byte) error {
	domain := extractDomain(to)
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return logAndReturnError(req, "mx_lookup_failed", err)
	}

	sort.Slice(mxRecords, func(i, j int) bool { return mxRecords[i].Pref < mxRecords[j].Pref })

	var lastErr error
	for _, mx := range mxRecords {
		host := strings.TrimSuffix(mx.Host, ".")
		addr := fmt.Sprintf("%s:25", host) // 直连通常只走 25

		// 建立连接
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			lastErr = err
			continue
		}
		
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			lastErr = err
			continue
		}

		// [优化] 发送正确的 HELO/EHLO 主机名
		// 使用发件人域名作为 HELO 主机名，这有助于通过 SPF/DMARC 检查
		// 如果是子域名发信 (如 support@mail.example.com)，这里会自动使用 mail.example.com
		senderDomain := extractDomain(from)
		if senderDomain != "" {
			if err := c.Hello(senderDomain); err != nil {
				// 如果 Hello 失败，尝试继续（虽然后面可能会被拒）
				// fmt.Printf("HELO failed: %v\n", err)
			}
		}

		// 尝试 StartTLS，但不强制
		if ok, _ := c.Extension("STARTTLS"); ok {
			_ = c.StartTLS(&tls.Config{InsecureSkipVerify: true, ServerName: host})
		}

		if err = c.Mail(from); err != nil { c.Close(); lastErr = err; continue }
		if err = c.Rcpt(to); err != nil { c.Close(); lastErr = err; continue }
		w, err := c.Data()
		if err != nil { c.Close(); lastErr = err; continue }
		_, err = w.Write(msg)
		if err != nil { c.Close(); lastErr = err; continue }
		err = w.Close()
		c.Quit()
		
		if err == nil {
			logSuccess(req, "direct")
			return nil
		}
		lastErr = err
	}

	// 错误处理优化
	if lastErr != nil && strings.Contains(lastErr.Error(), "timeout") {
		lastErr = fmt.Errorf("%v (Firewall blocked port 25)", lastErr)
	}
	return logAndReturnError(req, "direct_send_failed", lastErr)
}

func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func logAndReturnError(req SendRequest, reason string, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	// 简单记录 channel 类型，不必太精确
	channel := "unknown"
	if req.ChannelID > 0 {
		channel = fmt.Sprintf("smtp_%d", req.ChannelID)
	} else {
		channel = "auto"
	}

	database.DB.Create(&database.EmailLog{
		Recipient:  req.To,
		Subject:    req.Subject,
		Body:       req.Body, // 保存正文
		Status:     "failed",
		ErrorMsg:   fmt.Sprintf("%s: %s", reason, msg),
		Channel:    channel,
		TrackingID: req.TrackingID, // [新增] 保存 TrackingID
	})
	return fmt.Errorf("%s: %v", reason, err)
}

func logSuccess(req SendRequest, channel string) {
	database.DB.Create(&database.EmailLog{
		Recipient:  req.To,
		Subject:    req.Subject,
		Body:       req.Body, // 保存正文
		Status:     "success",
		Channel:    channel,
		TrackingID: req.TrackingID, // [新增] 保存 TrackingID
	})
}
