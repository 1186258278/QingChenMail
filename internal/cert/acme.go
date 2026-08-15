package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"goemail/internal/database"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// DNSProviderType DNS 提供商类型
type DNSProviderType string

const (
	DNSProviderManual     DNSProviderType = "manual"
	DNSProviderCloudflare DNSProviderType = "cloudflare"
	DNSProviderAliyun     DNSProviderType = "aliyun"
	DNSProviderDNSPod     DNSProviderType = "dnspod"
)

// ACMEUser 实现 lego 的 User 接口
type ACMEUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *ACMEUser) GetEmail() string                        { return u.Email }
func (u *ACMEUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// ManualDNSProvider 手动 DNS 验证提供商
type ManualDNSProvider struct {
	challenges map[string]string // domain -> token
	mu         sync.Mutex
	onPresent  func(domain, token string) // 回调函数，通知用户需要添加的记录
}

func NewManualDNSProvider(onPresent func(domain, token string)) *ManualDNSProvider {
	return &ManualDNSProvider{
		challenges: make(map[string]string),
		onPresent:  onPresent,
	}
}

func (p *ManualDNSProvider) Present(domain, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 计算 TXT 记录值
	txtValue := dns01.GetChallengeInfo(domain, keyAuth).Value
	p.challenges[domain] = txtValue

	if p.onPresent != nil {
		p.onPresent(domain, txtValue)
	}

	return nil
}

func (p *ManualDNSProvider) CleanUp(domain, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.challenges, domain)
	return nil
}

func (p *ManualDNSProvider) GetChallenge(domain string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.challenges[domain]
}

// ACMEClient ACME 客户端
type ACMEClient struct {
	manager     *Manager
	useStaging  bool // 是否使用测试环境
	challenges  map[string]*PendingChallenge
	challengeMu sync.Mutex
}

// PendingChallenge 待验证的挑战
type PendingChallenge struct {
	Domain       string    `json:"domain"`
	TXTRecord    string    `json:"txt_record"`    // _acme-challenge.domain
	TXTValue     string    `json:"txt_value"`     // TXT 记录值
	CreatedAt    time.Time `json:"created_at"`
	Email        string    `json:"email"`
	DNSProvider  string    `json:"dns_provider"`
	DNSConfig    string    `json:"dns_config"`    // DNS API 配置 (加密)
	AccountKey   string    `json:"account_key"`   // ACME 账户私钥 (PEM)
}

// NewACMEClient 创建 ACME 客户端
func NewACMEClient(manager *Manager, staging bool) *ACMEClient {
	return &ACMEClient{
		manager:    manager,
		useStaging: staging,
		challenges: make(map[string]*PendingChallenge),
	}
}

// InitChallenge 初始化证书申请挑战
// 返回需要添加的 DNS TXT 记录信息
func (c *ACMEClient) InitChallenge(domain, email string, dnsProvider DNSProviderType, dnsConfig map[string]string) (*PendingChallenge, error) {
	// 1. 生成 ACME 账户密钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成账户密钥失败: %w", err)
	}

	// 序列化私钥
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("序列化私钥失败: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	// 2. 创建 ACME 用户
	user := &ACMEUser{
		Email: email,
		key:   privateKey,
	}

	// 3. 创建 ACME 配置
	acmeConfig := lego.NewConfig(user)
	if c.useStaging {
		acmeConfig.CADirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	} else {
		acmeConfig.CADirURL = "https://acme-v02.api.letsencrypt.org/directory"
	}

	// 4. 创建 ACME 客户端
	client, err := lego.NewClient(acmeConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 ACME 客户端失败: %w", err)
	}

	// 5. 设置手动 DNS 提供商
	var txtRecord, txtValue string
	var txtValueMu sync.Mutex
	presentCh := make(chan string, 1)
	provider := NewManualDNSProvider(func(d, token string) {
		txtValueMu.Lock()
		txtRecord = "_acme-challenge." + d
		txtValue = token
		txtValueMu.Unlock()
		select {
		case presentCh <- token:
		default:
		}
	})

	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return nil, fmt.Errorf("设置 DNS 提供商失败: %w", err)
	}

	// 6. 注册账户
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("注册 ACME 账户失败: %w", err)
	}
	user.Registration = reg

	// 7. 触发真实挑战流程以获取正确的 DNS-01 TXT 值
	// 正确的 TXT 值 = base64url(sha256(token + "." + 账户密钥指纹))，只能在真实挑战回调中拿到。
	// 由于 DNS 尚未配置，Obtain 最终会失败退出 (lego 有内部超时)，但 Present 回调
	// 会在挑战开始时同步触发，让我们通过 channel 拿到正确的 TXT 值。
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	go func() {
		_, _ = client.Certificate.Obtain(request)
	}()

	// 等待 Present 回调 (有超时上限，不再依赖固定 sleep 竞态)
	select {
	case token := <-presentCh:
		txtValueMu.Lock()
		if txtRecord == "" {
			txtRecord = "_acme-challenge." + domain
		}
		if txtValue == "" {
			txtValue = token
		}
		txtValueMu.Unlock()
	case <-time.After(90 * time.Second):
		log.Printf("[ACME] 等待挑战回调超时: domain=%s", domain)
		txtRecord = "_acme-challenge." + domain
	}

	log.Printf("[ACME] 初始化挑战: domain=%s, email=%s", domain, email)

	// 加密 DNS 配置
	dnsConfigJSON, _ := json.Marshal(dnsConfig)
	encryptedConfig, _ := c.manager.encrypt(string(dnsConfigJSON))

	// 创建待验证挑战
	txtValueMu.Lock()
	challenge := &PendingChallenge{
		Domain:      domain,
		TXTRecord:   txtRecord,
		TXTValue:    txtValue,
		CreatedAt:   time.Now(),
		Email:       email,
		DNSProvider: string(dnsProvider),
		DNSConfig:   encryptedConfig,
		AccountKey:  string(keyPEM),
	}
	txtValueMu.Unlock()

	// 保存挑战信息
	c.challengeMu.Lock()
	c.challenges[domain] = challenge
	c.challengeMu.Unlock()

	return challenge, nil
}

// VerifyAndObtain 验证 DNS 记录并获取证书
func (c *ACMEClient) VerifyAndObtain(domain string) (*database.Certificate, error) {
	c.challengeMu.Lock()
	challenge, ok := c.challenges[domain]
	c.challengeMu.Unlock()

	if !ok {
		return nil, errors.New("找不到该域名的挑战信息，请先初始化")
	}

	// 1. 恢复 ACME 账户
	keyBlock, _ := pem.Decode([]byte(challenge.AccountKey))
	if keyBlock == nil {
		return nil, errors.New("无法解析账户密钥")
	}

	privateKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析账户密钥失败: %w", err)
	}

	user := &ACMEUser{
		Email: challenge.Email,
		key:   privateKey,
	}

	// 2. 创建 ACME 配置
	acmeConfig := lego.NewConfig(user)
	if c.useStaging {
		acmeConfig.CADirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	} else {
		acmeConfig.CADirURL = "https://acme-v02.api.letsencrypt.org/directory"
	}

	// 3. 创建 ACME 客户端
	client, err := lego.NewClient(acmeConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 ACME 客户端失败: %w", err)
	}

	// 4. 设置 DNS 提供商
	provider := NewManualDNSProvider(nil)
	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return nil, fmt.Errorf("设置 DNS 提供商失败: %w", err)
	}

	// 5. 注册账户
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		// 可能已经注册过
		reg, err = client.Registration.ResolveAccountByKey()
		if err != nil {
			return nil, fmt.Errorf("注册/恢复 ACME 账户失败: %w", err)
		}
	}
	user.Registration = reg

	// 6. 申请证书
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书失败: %w", err)
	}

	// 7. 保存证书
	cert := &database.Certificate{
		Name:        domain,
		Source:      "letsencrypt",
		AutoRenew:   true,
		DNSProvider: challenge.DNSProvider,
		DNSConfig:   challenge.DNSConfig,
		ACMEEmail:   challenge.Email,
	}

	if err := c.manager.SaveCertificate(cert, string(certificates.Certificate), string(certificates.PrivateKey)); err != nil {
		return nil, err
	}

	// 8. 清理挑战信息
	c.challengeMu.Lock()
	delete(c.challenges, domain)
	c.challengeMu.Unlock()

	log.Printf("[ACME] 证书申请成功: domain=%s, expires=%s", domain, cert.NotAfter.Format("2006-01-02"))

	return cert, nil
}

// GetPendingChallenge 获取待验证的挑战
func (c *ACMEClient) GetPendingChallenge(domain string) *PendingChallenge {
	c.challengeMu.Lock()
	defer c.challengeMu.Unlock()
	return c.challenges[domain]
}

// CancelChallenge 取消挑战
func (c *ACMEClient) CancelChallenge(domain string) {
	c.challengeMu.Lock()
	defer c.challengeMu.Unlock()
	delete(c.challenges, domain)
}

// RenewCertificate 续期证书
func (c *ACMEClient) RenewCertificate(certID uint) (*database.Certificate, error) {
	cert, err := c.manager.GetCertificateByID(certID)
	if err != nil {
		return nil, err
	}

	if cert.Source != "letsencrypt" {
		return nil, errors.New("只有 Let's Encrypt 证书支持自动续期")
	}

	// 获取域名
	domains := strings.Split(cert.Domains, ",")
	if len(domains) == 0 {
		return nil, errors.New("证书没有关联域名")
	}

	// 使用保存的配置重新申请
	// 注意：这需要 DNS 配置仍然有效

	log.Printf("[ACME] 开始续期证书: id=%d, domains=%s", certID, cert.Domains)

	// TODO: 实现自动续期逻辑（需要 DNS API 配置）
	// 目前返回错误提示用户手动续期

	return nil, errors.New("自动续期功能暂未完全实现，请手动重新申请证书")
}
