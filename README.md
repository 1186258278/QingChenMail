<p align="center">
  <img src="docs/image/zh/01.png" alt="晴辰云邮" width="600">
</p>

<h1 align="center">晴辰云邮 QingChen Mail</h1>

<p align="center">
  <strong>🚀 企业级自建邮件系统 · 发送/接收/营销一站式解决方案</strong>
</p>

<p align="center">
  <a href="https://github.com/1186258278/QingChenMail/releases"><img src="https://img.shields.io/github/v/release/1186258278/QingChenMail?style=flat-square&color=blue" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg?style=flat-square" alt="License"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://github.com/1186258278/QingChenMail/actions"><img src="https://img.shields.io/github/actions/workflow/status/1186258278/QingChenMail/release.yml?style=flat-square" alt="Build"></a>
  <a href="https://qingchencloud.com"><img src="https://img.shields.io/badge/晴辰云-官网-orange?style=flat-square" alt="Website"></a>
</p>

<p align="center">
  <a href="README.md">中文</a> · <a href="docs/README_en.md">English</a> · <a href="docs/INSTALL_zh-CN.md">部署指南</a> · <a href="https://github.com/1186258278/QingChenMail/releases">下载</a>
</p>

---

## 💡 为什么选择晴辰云邮？

| 痛点 | 传统方案 | 晴辰云邮 |
|:---:|:---:|:---:|
| **成本** | 第三方 EDM 按量计费，邮件越多越贵 | **一次部署，永久免费** |
| **隐私** | 邮件内容经第三方服务器，存在泄露风险 | **数据 100% 自有掌控** |
| **灵活性** | API 受限，无法定制 | **开源可改，RESTful API 全开放** |
| **送达率** | 共享 IP 易被标记垃圾 | **独立 IP + DKIM/SPF/DMARC 自动配置** |

---

## ✨ 核心能力

<table>
<tr>
<td width="50%">

### 📤 智能发信引擎

- **双模式投递**: 直连发送 + SMTP 中继智能切换
- **高送达率**: 自动 DKIM 签名 + SPF/DMARC 记录生成
- **子域名隔离**: 营销邮件与事务邮件分离，保护主域信誉
- **异步队列**: 内置高性能队列，支持失败重试与并发控制
- **营销任务**: 支持暂停/恢复、实时进度追踪、打开率统计

</td>
<td width="50%">

### 📥 邮件网关

- **SMTP 收信**: 内置 SMTP Server，接收域名邮箱邮件
- **STARTTLS 加密**: 支持 TLS 加密传输，防窃听
- **智能转发**: 通配符/前缀匹配，自动转发至 Gmail/QQ
- **MIME 解析**: 自动解码 Base64/QP，支持中文无乱码
- **附件处理**: 自动提取保存，支持在线预览

</td>
</tr>
<tr>
<td>

### 🛡️ 安全防护

- **速率限制**: IP 级连接限制，防 DDoS/暴力攻击
- **IP 黑名单**: 一键封禁恶意 IP
- **JWT 认证**: 安全 Token + API Key 双重验证
- **密码加密**: bcrypt 哈希存储
- **HTTPS 支持**: 全站 SSL 加密

</td>
<td>

### 🔧 开发者友好

- **RESTful API**: 标准接口，支持 Bearer Token
- **永久密钥**: `sk_live_...` 格式，集成方便
- **模板引擎**: `{{.name}}` 变量替换，千人千面
- **Webhook 回调**: 发送状态实时推送
- **交互式文档**: 内置 API 文档 + AI 提示词

</td>
</tr>
</table>

---

## 🖼️ 系统预览

<table>
<tr>
<td align="center"><b>仪表盘</b><br><img src="docs/image/zh/02.png" width="380"></td>
<td align="center"><b>营销任务</b><br><img src="docs/image/zh/06.png" width="380"></td>
</tr>
<tr>
<td align="center"><b>联系人管理</b><br><img src="docs/image/zh/05.png" width="380"></td>
<td align="center"><b>邮件模板</b><br><img src="docs/image/zh/10.png" width="380"></td>
</tr>
<tr>
<td align="center"><b>域名配置</b><br><img src="docs/image/zh/08.png" width="380"></td>
<td align="center"><b>收件箱</b><br><img src="docs/image/zh/07.png" width="380"></td>
</tr>
</table>

<details>
<summary>📸 查看更多截图</summary>

| 发送通道 | 密钥管理 |
|:---:|:---:|
| ![SMTP](docs/image/zh/04.png) | ![Keys](docs/image/zh/09.png) |

| 发送日志 | 系统设置 |
|:---:|:---:|
| ![Logs](docs/image/zh/11.png) | ![Settings](docs/image/zh/15.png) |

</details>

---

## 🚀 快速开始

### 1️⃣ 下载运行

```bash
# 从 Releases 下载对应平台二进制文件
# https://github.com/1186258278/QingChenMail/releases

# Linux/macOS
chmod +x goemail && ./goemail

# Windows
goemail.exe
```

### 2️⃣ 访问后台

浏览器打开 `http://localhost:9901`

| 项目 | 值 |
|:---:|:---:|
| 默认账号 | `admin` |
| 默认密码 | `123456` |

> ⚠️ **首次登录后请立即修改密码！**

### 3️⃣ 发送第一封邮件

```bash
curl -X POST http://localhost:9901/api/v1/send \
  -H "Authorization: Bearer sk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "test@example.com",
    "subject": "Hello from QingChen Mail",
    "body": "<h1>欢迎使用晴辰云邮！</h1>"
  }'
```

---

## 📦 功能清单

| 模块 | 功能 | 状态 |
|:---|:---|:---:|
| **发送中心** | 单封/批量发送、附件支持、HTML 模板 | ✅ |
| **营销任务** | 定时发送、暂停恢复、进度追踪、统计分析 | ✅ |
| **联系人** | 分组管理、导入导出、退订管理 | ✅ |
| **收件箱** | SMTP 收信、MIME 解析、附件提取、批量操作 | ✅ |
| **转发规则** | 精确/前缀/通配符匹配、多目标转发 | ✅ |
| **域名管理** | 多域名支持、DKIM 自动生成、DNS 验证 | ✅ |
| **发送通道** | SMTP 中继配置、直连发送、负载均衡 | ✅ |
| **安全防护** | STARTTLS、速率限制、IP 黑名单、JWT | ✅ |
| **系统设置** | HTTPS、端口配置、备份恢复 | ✅ |
| **API** | RESTful 接口、永久密钥、交互文档 | ✅ |

---

## ⚙️ 配置说明

<details>
<summary>📝 config.json 示例</summary>

```json
{
  "domain": "mail.example.com",
  "host": "0.0.0.0",
  "port": "9901",
  "base_url": "https://mail.example.com",
  "enable_ssl": false,
  "enable_receiver": true,
  "receiver_port": "25",
  "receiver_tls": true,
  "receiver_rate_limit": 30,
  "receiver_max_msg_size": 10240
}
```

</details>

<details>
<summary>🔐 DNS 记录配置</summary>

```
# MX 记录 (收件)
@    MX    10    mail.example.com.

# SPF 记录 (发件验证)
@    TXT   "v=spf1 ip4:YOUR_SERVER_IP ~all"

# DKIM 记录 (签名验证)
default._domainkey    TXT    "v=DKIM1; k=rsa; p=YOUR_PUBLIC_KEY"

# DMARC 记录 (策略)
_dmarc    TXT    "v=DMARC1; p=quarantine; rua=mailto:admin@example.com"
```

</details>

---

## 🏗️ 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                      晴辰云邮 架构图                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐  │
│  │  Web UI │    │   API   │    │  SMTP   │    │  Queue  │  │
│  │ (HTML5) │    │  (Gin)  │    │ Server  │    │ Worker  │  │
│  └────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘  │
│       │              │              │              │        │
│       └──────────────┴──────────────┴──────────────┘        │
│                          │                                   │
│                    ┌─────┴─────┐                            │
│                    │   GORM    │                            │
│                    │  SQLite   │                            │
│                    └───────────┘                            │
└─────────────────────────────────────────────────────────────┘
```

| 层级 | 技术选型 |
|:---:|:---|
| **后端** | Go 1.21+ · Gin · GORM · SQLite |
| **前端** | HTML5 · TailwindCSS · Chart.js |
| **邮件** | go-mail · go-msgauth (DKIM) · STARTTLS |
| **安全** | bcrypt · JWT · Rate Limiter |

---

## 🤝 参与贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支: `git checkout -b feature/amazing-feature`
3. 提交更改: `git commit -m 'feat: add amazing feature'`
4. 推送分支: `git push origin feature/amazing-feature`
5. 提交 PR

详见 [贡献指南](CONTRIBUTING.md)

---

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 许可证，可免费商用。

---

<p align="center">
  <b>© 2026 武汉晴辰天下网络科技有限公司</b><br>
  <a href="https://qingchencloud.com">官网</a> · 
  <a href="https://github.com/1186258278/QingChenMail/issues">反馈</a> · 
  <a href="docs/INSTALL_zh-CN.md">文档</a>
</p>

<p align="center">
  如果这个项目对您有帮助，请给一个 ⭐ Star 支持一下！
</p>
