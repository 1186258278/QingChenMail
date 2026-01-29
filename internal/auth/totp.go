package auth

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// TOTP 配置
	TOTPIssuer = "QingChenMail"  // 在验证器 App 中显示的发行者名称
	TOTPPeriod = 30             // 验证码有效期（秒）
	TOTPDigits = otp.DigitsSix  // 验证码位数
)

// GenerateTOTPSecret 为用户生成 TOTP 密钥
// 返回 otp.Key 对象，包含密钥和用于生成二维码的 URL
func GenerateTOTPSecret(username string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: username,
		Period:      TOTPPeriod,
		Digits:      TOTPDigits,
		Algorithm:   otp.AlgorithmSHA1, // 大多数 Authenticator App 默认使用 SHA1
	})
	if err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateQRCodeDataURL 生成二维码的 Data URL (base64 PNG)
// 可直接嵌入 <img src="data:image/png;base64,..."> 标签中
func GenerateQRCodeDataURL(key *otp.Key) (string, error) {
	// 生成二维码图片
	img, err := key.Image(200, 200) // 200x200 像素
	if err != nil {
		return "", err
	}

	// 编码为 PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	// 转换为 Base64 Data URL
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	dataURL := "data:image/png;base64," + b64

	return dataURL, nil
}

// ValidateTOTP 验证用户输入的 TOTP 码是否正确
// secret: Base32 编码的密钥
// code: 用户输入的 6 位验证码
func ValidateTOTP(secret, code string) bool {
	// 验证时允许前后 1 个时间窗口的偏移（±30秒）
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    TOTPPeriod,
		Skew:      1, // 允许 ±1 个时间周期的偏移
		Digits:    TOTPDigits,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return valid
}

// GetTOTPProvisioningURI 获取用于手动输入的 TOTP URI
// 格式: otpauth://totp/QingChenMail:username?secret=XXX&issuer=QingChenMail
func GetTOTPProvisioningURI(secret, username string) string {
	return "otpauth://totp/" + TOTPIssuer + ":" + username + "?secret=" + secret + "&issuer=" + TOTPIssuer
}
